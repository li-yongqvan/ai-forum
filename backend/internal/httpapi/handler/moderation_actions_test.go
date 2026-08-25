package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
)

// ---- #60 审计端点 + 弹窗封禁集成测试 ----

type actionView struct {
	ID                int64  `json:"id"`
	ModeratorID       int64  `json:"moderator_id"`
	ModeratorUsername string `json:"moderator_username"`
	Action            string `json:"action"`
	TargetType        string `json:"target_type"`
	TargetID          int64  `json:"target_id"`
	Reason            string `json:"reason"`
}

type actionList struct {
	Items    []actionView `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func listActions(t *testing.T, r *gin.Engine, token string, query string) (int, actionList) {
	t.Helper()
	w := doJSON(t, r, http.MethodGet, "/api/v1/moderation/actions?"+query, nil, token)
	var l actionList
	json.Unmarshal(w.Body.Bytes(), &l)
	return w.Code, l
}

func handleReport(t *testing.T, r *gin.Engine, token string, reportID int64, action, note string) (int, string) {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/moderation/reports/%d/handle", reportID),
		map[string]any{"action": action, "note": note}, token)
	return w.Code, w.Body.String()
}

// 审计端点权限：mod+ 可读；普通用户 403；游客 401。
func TestListActionsAuthGate(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	registerNotifyUser(t, r, gdb, "alice", "ACT-A1")
	registerNotifyUser(t, r, gdb, "bob", "ACT-A2")
	makeModerator(t, gdb, "bob")
	bob := login(t, r, "bob")

	// 游客
	if code, _ := listActions(t, r, "", ""); code != http.StatusUnauthorized {
		t.Errorf("游客审计 = %d, want 401", code)
	}
	// 普通用户
	alice := login(t, r, "alice")
	if code, _ := listActions(t, r, alice.Token, ""); code != http.StatusForbidden {
		t.Errorf("普通用户审计 = %d, want 403", code)
	}
	// moderator 空列表 200
	if code, l := listActions(t, r, bob.Token, ""); code != http.StatusOK || len(l.Items) != 0 {
		t.Errorf("mod 审计 = %d/%d, want 200/0", code, len(l.Items))
	}
}

// 审计端点过滤、分页、enrich 用户名。
func TestListActionsFiltering(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "ACT-F1") // 作者
	bob := registerNotifyUser(t, r, gdb, "bob", "ACT-F2")     // 举报人
	registerNotifyUser(t, r, gdb, "carol", "ACT-F3")
	makeModerator(t, gdb, "carol")
	carol := login(t, r, "carol")

	post := createPost(t, r, alice.Token, 1, "审计帖")
	if code := reportCode(t, r, bob.Token, "post", post.ID, "垃圾广告", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	rep := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports?status=pending", nil, carol.Token)
	var list reportList
	json.Unmarshal(rep.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Fatalf("pending = %d, want 1", len(list.Items))
	}
	// 先 dismiss（不落审计）再 delete（落审计），模拟多条审计
	if code, _ := handleReport(t, r, carol.Token, list.Items[0].ID, "dismiss", "证据不足"); code != http.StatusOK {
		t.Fatalf("dismiss = %d", code)
	}
	post2 := createPost(t, r, alice.Token, 1, "审计帖2")
	if code := reportCode(t, r, bob.Token, "post", post2.ID, "违法违规", ""); code != http.StatusCreated {
		t.Fatalf("建举报2 = %d", code)
	}
	rep2 := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports?status=pending", nil, carol.Token)
	var list2 reportList
	json.Unmarshal(rep2.Body.Bytes(), &list2)
	if code, _ := handleReport(t, r, carol.Token, list2.Items[0].ID, "delete_post", "违规删除"); code != http.StatusOK {
		t.Fatalf("delete_post = %d", code)
	}

	// 全量审计 = 1 条 delete_post（dismiss 不落审计）
	code, all := listActions(t, r, carol.Token, "")
	if code != http.StatusOK || len(all.Items) != 1 {
		t.Fatalf("全量审计 = %d/%d, want 200/1", code, len(all.Items))
	}
	if all.Items[0].Action != "delete_post" || all.Items[0].ModeratorUsername != "carol" {
		t.Errorf("审计行 = %+v", all.Items[0])
	}

	// 按 target_id 过滤
	code, filtered := listActions(t, r, carol.Token, fmt.Sprintf("target_type=post&target_id=%d", post2.ID))
	if code != http.StatusOK || len(filtered.Items) != 1 || filtered.Items[0].TargetID != post2.ID {
		t.Errorf("按 target 过滤 = %d/%+v", code, filtered.Items)
	}

	// 按 action 过滤（无匹配）
	code, empty := listActions(t, r, carol.Token, "action=warn")
	if code != http.StatusOK || len(empty.Items) != 0 {
		t.Errorf("warn 过滤 = %d/%d, want 200/0", code, len(empty.Items))
	}
}

// 弹窗封禁：admin 可 handleReport(action=ban_user)； moderator 拒绝；reason 必填。
func TestHandleReportBanUserIntegration(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "ACT-B1") // 作者
	bob := registerNotifyUser(t, r, gdb, "bob", "ACT-B2")     // 举报人
	registerNotifyUser(t, r, gdb, "carol", "ACT-B3")
	makeAdmin(t, gdb, "carol")
	admin := login(t, r, "carol")

	post := createPost(t, r, alice.Token, 1, "封禁帖")
	if code := reportCode(t, r, bob.Token, "post", post.ID, "违法违规", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	rep := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports?status=pending", nil, admin.Token)
	var list reportList
	json.Unmarshal(rep.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Fatalf("pending = %d", len(list.Items))
	}

	// reason 空 → 400
	if code, _ := handleReport(t, r, admin.Token, list.Items[0].ID, "ban_user", ""); code != http.StatusBadRequest {
		t.Errorf("ban 空 reason = %d, want 400", code)
	}
	// 成功 ban_user
	if code, body := handleReport(t, r, admin.Token, list.Items[0].ID, "ban_user", "严重违规"); code != http.StatusOK {
		t.Fatalf("ban_user = %d, body=%s", code, body)
	}
	// 作者状态 = banned
	var status string
	gdb.Table("user.users").Select("status").Where("id = ?", alice.User.ID).Scan(&status)
	if status != "banned" {
		t.Errorf("作者 status = %q, want banned", status)
	}
	// 审计行 target_type=user target_id=alice
	if n := countAction(t, gdb, "ban_user", alice.User.ID); n != 1 {
		t.Errorf("ban_user 审计行 = %d, want 1", n)
	}
	// 举报人收到 report_result「已封禁用户」
	wn := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, bob.Token)
	var notifs notifListFull
	json.Unmarshal(wn.Body.Bytes(), &notifs)
	if len(notifs.Items) == 0 || notifs.Items[0].TargetTitle == nil || *notifs.Items[0].TargetTitle != "已封禁用户" {
		t.Errorf("举报人通知 = %+v", notifs.Items)
	}
	// 作者收到 report_handled「已被封禁」
	wa := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, alice.Token)
	var an notifListFull
	json.Unmarshal(wa.Body.Bytes(), &an)
	if len(an.Items) == 0 || an.Items[0].TargetTitle == nil || *an.Items[0].TargetTitle != "你因「违法违规」被举报，已被封禁" {
		t.Errorf("作者通知 = %+v", an.Items)
	}
}

// 弹窗封禁权限墙：moderator 不能 ban_user；普通用户不能访问审计。
func TestBanUserInReportPermissionWall(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "ACT-P1") // 作者
	bob := registerNotifyUser(t, r, gdb, "bob", "ACT-P2")     // 举报人
	registerNotifyUser(t, r, gdb, "carol", "ACT-P3")
	makeModerator(t, gdb, "carol")
	mod := login(t, r, "carol")

	post := createPost(t, r, alice.Token, 1, "权限帖")
	if code := reportCode(t, r, bob.Token, "post", post.ID, "违法违规", ""); code != http.StatusCreated {
		t.Fatalf("建举报 = %d", code)
	}
	rep := doJSON(t, r, http.MethodGet, "/api/v1/moderation/reports?status=pending", nil, mod.Token)
	var list reportList
	json.Unmarshal(rep.Body.Bytes(), &list)

	// moderator 试图 ban_user → 400（服务端二次鉴权拒绝）
	if code, _ := handleReport(t, r, mod.Token, list.Items[0].ID, "ban_user", "越权"); code != http.StatusBadRequest {
		t.Errorf("moderator ban_user = %d, want 400", code)
	}
}
