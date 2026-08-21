package handler_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"gorm.io/gorm"
)

// ---- #34 治理权限矩阵：ban/unban 集成（仿 moderation_test.go 模式） ----

func makeAdmin(t *testing.T, gdb *gorm.DB, username string) {
	t.Helper()
	if err := gdb.Exec(`UPDATE "user".users SET role='admin' WHERE username=?`, username).Error; err != nil {
		t.Fatalf("提权 admin 失败: %v", err)
	}
}

// banReq 发起 ban（unban=false）或 unban（unban=true），返回状态码与响应体。
func banReq(t *testing.T, r *gin.Engine, token string, targetID int64, unban bool, reason string) (int, string) {
	t.Helper()
	action := "ban"
	if unban {
		action = "unban"
	}
	w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/moderation/users/%d/%s", targetID, action),
		map[string]any{"reason": reason}, token)
	return w.Code, w.Body.String()
}

func countAction(t *testing.T, gdb *gorm.DB, action string, targetID int64) int64 {
	t.Helper()
	var n int64
	gdb.Table("moderation.moderation_actions").
		Where("action = ? AND target_type = ? AND target_id = ?", action, "user", targetID).Count(&n)
	return n
}

// admin ban 主链路：200 + status=banned + 审计 ban_user 行（moderator_id/reason 落库）。
func TestAdminBanFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	root := registerNotifyUser(t, r, gdb, "root", "ADM-B1")
	makeAdmin(t, gdb, "root")
	rootTok := login(t, r, "root").Token // 提权后重登拿 admin token
	target := registerNotifyUser(t, r, gdb, "bob", "ADM-B2")

	code, body := banReq(t, r, rootTok, target.User.ID, false, "人身攻击")
	if code != http.StatusOK {
		t.Fatalf("ban = %d, body=%s", code, body)
	}
	// status=banned
	var status string
	gdb.Table("user.users").Select("status").Where("id = ?", target.User.ID).Scan(&status)
	if status != "banned" {
		t.Errorf("封禁后 status = %q, want banned", status)
	}
	// 审计行
	if n := countAction(t, gdb, "ban_user", target.User.ID); n != 1 {
		t.Errorf("ban_user 审计行数 = %d, want 1", n)
	}
	var auditorID int64
	var auditReason string
	gdb.Table("moderation.moderation_actions").Select("moderator_id, reason").
		Where("action = ? AND target_id = ?", "ban_user", target.User.ID).Row().Scan(&auditorID, &auditReason)
	if auditorID != root.User.ID || auditReason != "人身攻击" {
		t.Errorf("审计 operator/reason = %d/%q, want %d/人身攻击", auditorID, auditReason, root.User.ID)
	}
}

// admin unban 主链路：先 ban 再 unban → 200 + status=active + unban_user 审计行。
func TestAdminUnbanFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	registerNotifyUser(t, r, gdb, "root", "ADM-U1")
	makeAdmin(t, gdb, "root")
	rootTok := login(t, r, "root").Token
	target := registerNotifyUser(t, r, gdb, "bob", "ADM-U2")

	if code, _ := banReq(t, r, rootTok, target.User.ID, false, "封"); code != http.StatusOK {
		t.Fatal("预封禁失败")
	}
	code, body := banReq(t, r, rootTok, target.User.ID, true, "误封纠正")
	if code != http.StatusOK {
		t.Fatalf("unban = %d, body=%s", code, body)
	}
	var status string
	gdb.Table("user.users").Select("status").Where("id = ?", target.User.ID).Scan(&status)
	if status != "active" {
		t.Errorf("解封后 status = %q, want active", status)
	}
	if n := countAction(t, gdb, "unban_user", target.User.ID); n != 1 {
		t.Errorf("unban_user 审计行数 = %d, want 1", n)
	}
}

// 权限墙：普通 user / moderator 调 ban → 403；游客 → 401。
func TestBanPermissionWall(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "ADM-P1")
	bob := registerNotifyUser(t, r, gdb, "bob", "ADM-P2")
	makeModerator(t, gdb, "alice")
	modTok := login(t, r, "alice").Token

	// 普通用户（bob）ban alice → 403
	if code, _ := banReq(t, r, bob.Token, alice.User.ID, false, "x"); code != http.StatusForbidden {
		t.Errorf("普通用户 ban = %d, want 403", code)
	}
	// moderator ban → 403（封禁/解封仅 admin，IA §5.0）
	if code, _ := banReq(t, r, modTok, bob.User.ID, false, "x"); code != http.StatusForbidden {
		t.Errorf("moderator ban = %d, want 403", code)
	}
	// 游客 → 401
	w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/moderation/users/%d/ban", bob.User.ID), map[string]any{"reason": "x"}, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("游客 ban = %d, want 401", w.Code)
	}
}

// 被封用户：写操作 403 + code=account_banned；登录 401；/auth/me 与 logout 豁免。
func TestBannedUserInterception(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	registerNotifyUser(t, r, gdb, "root", "ADM-I1")
	makeAdmin(t, gdb, "root")
	rootTok := login(t, r, "root").Token
	bob := registerNotifyUser(t, r, gdb, "bob", "ADM-I2")

	if code, _ := banReq(t, r, rootTok, bob.User.ID, false, "违规"); code != http.StatusOK {
		t.Fatal("预封禁失败")
	}

	// 写操作（发帖）→ 403 + code=account_banned（bob 的旧 token 仍有效，但被写拦截）
	w := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{"board_id": 1, "title": "x", "content": "y"}, bob.Token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("被封用户发帖 = %d, want 403", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"account_banned"`) {
		t.Errorf("body = %s, want code=account_banned", w.Body.String())
	}

	// 登录拒绝 → 401（同文案防枚举）
	wl := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "bob", "password": "secret123"}, "")
	if wl.Code != http.StatusUnauthorized {
		t.Errorf("被封用户登录 = %d, want 401", wl.Code)
	}

	// 豁免：/auth/me 与 /auth/logout 放行
	wm := doJSON(t, r, http.MethodGet, "/api/v1/auth/me", nil, bob.Token)
	if wm.Code != http.StatusOK {
		t.Errorf("被封用户 me = %d, want 200（读不禁）", wm.Code)
	}
	wo := doJSON(t, r, http.MethodPost, "/api/v1/auth/logout", nil, bob.Token)
	if wo.Code != http.StatusOK {
		t.Errorf("被封用户 logout = %d, want 200", wo.Code)
	}
}

// 边界：自封 400 / 封 admin 400 / 重复 ban 409（审计仅 1 行）/ unban 未封 409 / reason 缺失或超长 400。
func TestBanBoundaries(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	root := registerNotifyUser(t, r, gdb, "root", "ADM-E1")
	makeAdmin(t, gdb, "root")
	rootTok := login(t, r, "root").Token
	other := registerNotifyUser(t, r, gdb, "admin2", "ADM-E2")
	makeAdmin(t, gdb, "admin2") // 第二个 admin
	bob := registerNotifyUser(t, r, gdb, "bob", "ADM-E3")

	// 自封 → 400
	if code, _ := banReq(t, r, rootTok, root.User.ID, false, "x"); code != http.StatusBadRequest {
		t.Errorf("自封 = %d, want 400", code)
	}
	// 封 admin → 400
	if code, _ := banReq(t, r, rootTok, other.User.ID, false, "x"); code != http.StatusBadRequest {
		t.Errorf("封 admin = %d, want 400", code)
	}
	// reason 缺失 → 400
	w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/moderation/users/%d/ban", bob.User.ID), map[string]any{}, rootTok)
	if w.Code != http.StatusBadRequest {
		t.Errorf("reason 缺失 = %d, want 400", w.Code)
	}
	// reason 超长 → 400
	if code, _ := banReq(t, r, rootTok, bob.User.ID, false, strings.Repeat("长", 501)); code != http.StatusBadRequest {
		t.Errorf("reason 超长 = %d, want 400", code)
	}
	// 正常 ban → 200
	if code, _ := banReq(t, r, rootTok, bob.User.ID, false, "违规"); code != http.StatusOK {
		t.Fatalf("ban = %d", code)
	}
	// 重复 ban → 409，且审计仅 1 行
	if code, _ := banReq(t, r, rootTok, bob.User.ID, false, "再封一次"); code != http.StatusConflict {
		t.Errorf("重复 ban = %d, want 409", code)
	}
	if n := countAction(t, gdb, "ban_user", bob.User.ID); n != 1 {
		t.Errorf("重复 ban 后审计行数 = %d, want 1（幂等不重复审计）", n)
	}
	// unban 未封用户（另一个 active 用户）→ 409
	dave := registerNotifyUser(t, r, gdb, "dave", "ADM-E4")
	if code, _ := banReq(t, r, rootTok, dave.User.ID, true, "x"); code != http.StatusConflict {
		t.Errorf("unban 未封 = %d, want 409", code)
	}
}

// profile banned 可见性：admin 与 self-view 可见，普通用户/游客不可见。
func TestProfileBannedVisibility(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	registerNotifyUser(t, r, gdb, "root", "ADM-V1")
	makeAdmin(t, gdb, "root")
	rootTok := login(t, r, "root").Token
	bob := registerNotifyUser(t, r, gdb, "bob", "ADM-V2")
	carol := registerNotifyUser(t, r, gdb, "carol", "ADM-V3")

	// ban bob
	if code, _ := banReq(t, r, rootTok, bob.User.ID, false, "违规"); code != http.StatusOK {
		t.Fatal("预封禁失败")
	}

	// admin viewer 可见 banned:true
	wAdmin := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", bob.User.ID), nil, rootTok)
	if wAdmin.Code != http.StatusOK || !strings.Contains(wAdmin.Body.String(), `"banned":true`) {
		t.Errorf("admin 看 profile = %d, body=%s, want banned:true", wAdmin.Code, wAdmin.Body.String())
	}
	// 普通用户（carol）不可见 banned 字段
	wUser := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", bob.User.ID), nil, carol.Token)
	if wUser.Code != http.StatusOK || strings.Contains(wUser.Body.String(), `"banned"`) {
		t.Errorf("普通用户看 profile = %d, body=%s, want 无 banned 字段", wUser.Code, wUser.Body.String())
	}
	// 游客不可见
	wGuest := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", bob.User.ID), nil, "")
	if strings.Contains(wGuest.Body.String(), `"banned"`) {
		t.Errorf("游客看 profile = %s, want 无 banned 字段", wGuest.Body.String())
	}
	// self-view：被封用户看自己主页 → banned:true（评审 Q5 附带 1）
	wSelf := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", bob.User.ID), nil, bob.Token)
	if wSelf.Code != http.StatusOK || !strings.Contains(wSelf.Body.String(), `"banned":true`) {
		t.Errorf("self 看自己 profile = %d, body=%s, want banned:true", wSelf.Code, wSelf.Body.String())
	}
}
