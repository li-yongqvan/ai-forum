package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
)

// ---- 举报域响应结构（独立定义，不依赖 notify_test 的结构） ----

type reportView struct {
	ID               int64  `json:"id"`
	ReporterID       int64  `json:"reporter_id"`
	ReporterUsername string `json:"reporter_username"`
	TargetType       string `json:"target_type"`
	TargetID         int64  `json:"target_id"`
	TargetTitle      string `json:"target_title"`
	Reason           string `json:"reason"`
	Status           string `json:"status"`
}

type reportList struct {
	Items []reportView `json:"items"`
}

type reportNotifyView struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	TargetType  *string `json:"target_type"`
	TargetID    *int64  `json:"target_id"`
	TargetTitle *string `json:"target_title"`
}

type notifListFull struct {
	Items []reportNotifyView `json:"items"`
}

func reportCode(t *testing.T, r *gin.Engine, token string, targetType string, targetID int64, reason, note string) int {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/v1/reports", map[string]any{
		"target_type": targetType, "target_id": targetID, "reason": reason, "note": note,
	}, token)
	return w.Code
}

// 举报接口登录墙 + 管理接口角色墙（#9 §5.0）。
func TestReportsAuthGates(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	// 游客举报 → 401
	w := doJSON(t, r, http.MethodPost, "/api/v1/reports", map[string]any{
		"target_type": "post", "target_id": 1, "reason": "垃圾广告",
	}, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("游客举报 = %d, want 401", w.Code)
	}

	// 普通用户访问管理队列 → 403
	alice := registerNotifyUser(t, r, gdb, "alice", "RM-A1")
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports", nil, alice.Token)
	if w2.Code != http.StatusForbidden {
		t.Errorf("普通用户访问队列 = %d, want 403", w2.Code)
	}
}

// 创建举报：201 + id、重复 409、自举报 400、目标不存在 404、非法原因 400、「其他」无备注 400。
func TestReportCreateFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "RM-C1") // 帖子作者
	bob := registerNotifyUser(t, r, gdb, "bob", "RM-C2")     // 举报人

	post := createPost(t, r, alice.Token, 1, "被举报的帖子")

	// bob 举报 alice 的帖子 → 201 + 举报 id
	w := doJSON(t, r, http.MethodPost, "/api/v1/reports", map[string]any{
		"target_type": "post", "target_id": post.ID, "reason": "垃圾广告", "note": "广告链接",
	}, bob.Token)
	if w.Code != http.StatusCreated {
		t.Fatalf("举报 = %d, body=%s", w.Code, w.Body.String())
	}
	var created struct{ ID int64 `json:"id"` }
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Error("举报响应应含 id")
	}

	// 重复举报（pending 期）→ 409
	if code := reportCode(t, r, bob.Token, "post", post.ID, "违法违规", ""); code != http.StatusConflict {
		t.Errorf("重复举报 = %d, want 409", code)
	}
	// 自举报 → 400
	if code := reportCode(t, r, alice.Token, "post", post.ID, "垃圾广告", ""); code != http.StatusBadRequest {
		t.Errorf("自举报 = %d, want 400", code)
	}
	// 目标不存在 → 404
	if code := reportCode(t, r, bob.Token, "post", 999999, "垃圾广告", ""); code != http.StatusNotFound {
		t.Errorf("举报不存在目标 = %d, want 404", code)
	}
	// 非法原因 → 400
	if code := reportCode(t, r, bob.Token, "comment", 1, "瞎编的原因", ""); code != http.StatusBadRequest {
		t.Errorf("非法原因 = %d, want 400", code)
	}
	// 「其他」无备注 → 400（对未举报过的 comment 目标，避开 409）
	wc := doJSON(t, r, http.MethodPost, "/api/v1/comments", map[string]any{
		"post_id": post.ID, "content": "一条评论",
	}, alice.Token)
	if wc.Code != http.StatusCreated {
		t.Fatalf("建评论 = %d, body=%s", wc.Code, wc.Body.String())
	}
	var cm struct{ ID int64 `json:"id"` }
	json.Unmarshal(wc.Body.Bytes(), &cm)
	if code := reportCode(t, r, bob.Token, "comment", cm.ID, "其他", ""); code != http.StatusBadRequest {
		t.Errorf("「其他」无备注 = %d, want 400", code)
	}
}

// 管理闭环：队列 enrich → dismiss（不落审计/不发通知）→ delete（软删 + 通知结论）→ warn（审计记作者）。
func TestModerationHandlingFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "RM-M1") // 作者
	bob := registerNotifyUser(t, r, gdb, "bob", "RM-M2")     // 举报人
	registerNotifyUser(t, r, gdb, "carol", "RM-M3")
	makeModerator(t, gdb, "carol")
	carol := login(t, r, "carol") // moderator token

	listPending := func() reportList {
		w := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports?status=pending", nil, carol.Token)
		if w.Code != http.StatusOK {
			t.Fatalf("队列 = %d, body=%s", w.Code, w.Body.String())
		}
		var l reportList
		if err := json.Unmarshal(w.Body.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		return l
	}
	pendingCount := func() int64 {
		w := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports/count?status=pending", nil, carol.Token)
		var c struct{ Count int64 `json:"count"` }
		json.Unmarshal(w.Body.Bytes(), &c)
		return c.Count
	}
	handle := func(id int64, action, note string) int {
		w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/moderation/reports/%d/handle", id), map[string]any{
			"action": action, "note": note,
		}, carol.Token)
		return w.Code
	}
	bobNotifs := func() []reportNotifyView {
		w := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, bob.Token)
		var l notifListFull
		json.Unmarshal(w.Body.Bytes(), &l)
		return l.Items
	}

	// ===== 场景 1：dismiss（忽略）——只改状态，不落审计，不发通知 =====
	post1 := createPost(t, r, alice.Token, 1, "举报帖1")
	if code := reportCode(t, r, bob.Token, "post", post1.ID, "垃圾广告", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	l1 := listPending()
	if len(l1.Items) != 1 || l1.Items[0].ReporterUsername != "bob" || l1.Items[0].TargetTitle != "举报帖1" || l1.Items[0].Reason != "垃圾广告" {
		t.Errorf("队列 enrich = %+v", l1.Items)
	}
	if pendingCount() != 1 {
		t.Errorf("pending 计数 = %d, want 1", pendingCount())
	}
	rep1 := l1.Items[0]
	if code := handle(rep1.ID, "dismiss", "证据不足"); code != http.StatusOK {
		t.Fatalf("dismiss = %d, body=%s", code, "见上")
	}
	if pendingCount() != 0 {
		t.Errorf("dismiss 后 pending = %d, want 0", pendingCount())
	}
	// IA §5.6「所有动作→通知举报人」：dismiss 也发结论通知（未采取处理）
	notifs := bobNotifs()
	if len(notifs) != 1 || notifs[0].Type != "report_result" || notifs[0].TargetTitle == nil || *notifs[0].TargetTitle != "未采取处理" {
		t.Errorf("dismiss 应发结论通知, got %+v", notifs)
	}
	// 重复处理 → 400（已结案）
	if code := handle(rep1.ID, "dismiss", ""); code != http.StatusBadRequest {
		t.Errorf("重复处理 = %d, want 400", code)
	}

	// ===== 场景 2：delete_post——软删内容 + 落审计 + 通知举报人「内容已删除」 =====
	post2 := createPost(t, r, alice.Token, 1, "举报帖2")
	if code := reportCode(t, r, bob.Token, "post", post2.ID, "违法违规", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	rep2 := listPending().Items[0]
	if code := handle(rep2.ID, "delete_post", "违规删除"); code != http.StatusOK {
		t.Fatalf("delete_post = %d", code)
	}
	// 帖子已软删 → 详情 404
	wGet := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/posts/%d", post2.ID), nil, "")
	if wGet.Code != http.StatusNotFound {
		t.Errorf("删除后帖子 = %d, want 404", wGet.Code)
	}
	// 审计行存在
	var auditN int64
	gdb.Table("moderation.moderation_actions").Where("action = ? AND target_type = ? AND target_id = ?", "delete_post", "post", post2.ID).Count(&auditN)
	if auditN != 1 {
		t.Errorf("delete_post 审计行数 = %d, want 1", auditN)
	}
	// 通知举报人：report_result + 结论「内容已删除」
	notifs = bobNotifs()
	if len(notifs) == 0 || notifs[0].Type != "report_result" || notifs[0].TargetTitle == nil || *notifs[0].TargetTitle != "内容已删除" {
		t.Errorf("report_result 通知 = %+v", notifs)
	}

	// ===== 场景 3：warn——审计记被举报内容作者（target_type=user） + 通知「已警告违规用户」 =====
	post3 := createPost(t, r, alice.Token, 1, "举报帖3")
	if code := reportCode(t, r, bob.Token, "post", post3.ID, "人身攻击", "骂人"); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	rep3 := listPending().Items[0]
	if code := handle(rep3.ID, "warn", "警告一次"); code != http.StatusOK {
		t.Fatalf("warn = %d", code)
	}
	var warnN int64
	gdb.Table("moderation.moderation_actions").Where("action = ? AND target_type = ? AND target_id = ?", "warn", "user", alice.User.ID).Count(&warnN)
	if warnN != 1 {
		t.Errorf("warn 审计行数 = %d, want 1（记作者）", warnN)
	}
	notifs = bobNotifs()
	if len(notifs) == 0 || notifs[0].TargetTitle == nil || *notifs[0].TargetTitle != "已警告违规用户" {
		t.Errorf("warn 通知 = %+v", notifs)
	}
}

// #53 被举报人触达：dismiss 不通知作者（0 条）；delete_post/warn 通知作者（累计语义，最新在前）。
func TestReportedUserNotification(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "RN-N1") // 作者
	bob := registerNotifyUser(t, r, gdb, "bob", "RN-N2")     // 举报人
	registerNotifyUser(t, r, gdb, "carol", "RN-N3")
	makeModerator(t, gdb, "carol")
	carol := login(t, r, "carol")

	listPending := func() reportList {
		w := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports?status=pending", nil, carol.Token)
		var l reportList
		json.Unmarshal(w.Body.Bytes(), &l)
		return l
	}
	handle := func(id int64, action, note string) int {
		w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/moderation/reports/%d/handle", id), map[string]any{
			"action": action, "note": note,
		}, carol.Token)
		return w.Code
	}
	aliceNotifs := func() []reportNotifyView {
		w := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, alice.Token)
		var l notifListFull
		json.Unmarshal(w.Body.Bytes(), &l)
		return l.Items
	}

	// 场景 1：dismiss → 作者 0 条（#53 D2 不打扰被举报人）
	post1 := createPost(t, r, alice.Token, 1, "帖1")
	if code := reportCode(t, r, bob.Token, "post", post1.ID, "垃圾广告", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	if code := handle(listPending().Items[0].ID, "dismiss", "证据不足"); code != http.StatusOK {
		t.Fatalf("dismiss = %d", code)
	}
	if got := aliceNotifs(); len(got) != 0 {
		t.Errorf("dismiss 后作者通知 = %+v, want 0 条", got)
	}

	// 场景 2：delete_post → 作者 1 条 report_handled（累计 1）
	post2 := createPost(t, r, alice.Token, 1, "帖2")
	if code := reportCode(t, r, bob.Token, "post", post2.ID, "违法违规", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	if code := handle(listPending().Items[0].ID, "delete_post", "违规删除"); code != http.StatusOK {
		t.Fatalf("delete_post = %d", code)
	}
	got := aliceNotifs()
	if len(got) != 1 || got[0].Type != "report_handled" || got[0].TargetTitle == nil || *got[0].TargetTitle != "你的内容因「违法违规」被举报，已删除" {
		t.Errorf("delete_post 后作者通知 = %+v", got)
	}

	// 场景 3：warn → 作者累计 2 条，最新为 warn 文案（#53 warn 也通知，反转 O4）
	post3 := createPost(t, r, alice.Token, 1, "帖3")
	if code := reportCode(t, r, bob.Token, "post", post3.ID, "人身攻击", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	if code := handle(listPending().Items[0].ID, "warn", "警告一次"); code != http.StatusOK {
		t.Fatalf("warn = %d", code)
	}
	got = aliceNotifs()
	if len(got) != 2 || got[0].Type != "report_handled" || got[0].TargetTitle == nil || *got[0].TargetTitle != "你因「人身攻击」被举报，已警告" {
		t.Errorf("warn 后作者通知（累计 2 条，最新 warn）= %+v", got)
	}
}

// #53 举报频控：同举报人 10 分钟 5 连报全 201；第 6 次（全新目标）→ 429 + 提示，不落库。
func TestReportRateLimit(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "RL-L1") // 作者
	bob := registerNotifyUser(t, r, gdb, "bob", "RL-L2")     // 举报人

	// 5 个不同帖子各报一次 → 201（不同目标避开 uq_reports_pending 409）
	for i := 0; i < 5; i++ {
		p := createPost(t, r, alice.Token, 1, fmt.Sprintf("频控帖%d", i+1))
		if code := reportCode(t, r, bob.Token, "post", p.ID, "垃圾广告", ""); code != http.StatusCreated {
			t.Fatalf("第 %d 次举报 = %d, want 201", i+1, code)
		}
	}
	// 第 6 个全新帖子 → 429（打旧目标会被 409 先拦，无法验证 429）
	p6 := createPost(t, r, alice.Token, 1, "频控帖6")
	w := doJSON(t, r, http.MethodPost, "/api/v1/reports", map[string]any{
		"target_type": "post", "target_id": p6.ID, "reason": "垃圾广告",
	}, bob.Token)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 6 次 = %d, want 429, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "举报过于频繁") {
		t.Errorf("429 body 应含提示, got %s", w.Body.String())
	}
	// 不落库
	var n int64
	gdb.Table("moderation.reports").Where("reporter_id = ?", bob.User.ID).Count(&n)
	if n != 5 {
		t.Errorf("reports 计数 = %d, want 5（超限不落库）", n)
	}
}
