package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
	"github.com/li-yongqvan/ai-forum/backend/internal/config"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"github.com/li-yongqvan/ai-forum/backend/user"
	"gorm.io/gorm"
)

// ---- 装配 ----

func newEngine(gdb *gorm.DB) *gin.Engine {
	jwtMgr := auth.NewManager("test-secret", 7*24*time.Hour)
	userSvc := user.NewService(user.NewGormRepo(gdb), jwtMgr)
	return httpapi.NewEngine(config.Config{Env: "test", Port: "8080"}, jwtMgr, userSvc)
}

// seedCode 直插邀请码（auth-flow §7：管理员 DB 直管，MVP 无管理 UI）。
func seedCode(t *testing.T, gdb *gorm.DB, code string) {
	t.Helper()
	if err := gdb.Exec(`INSERT INTO "user".invitation_codes (code, created_by) VALUES (?, ?)`, code, 1).Error; err != nil {
		t.Fatalf("seed 邀请码失败: %v", err)
	}
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("编码请求体失败: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type authResp struct {
	Token string   `json:"token"`
	User  userView `json:"user"`
}

type userView struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

func decodeAuthResp(t *testing.T, w *httptest.ResponseRecorder) authResp {
	t.Helper()
	var res authResp
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	return res
}

// ---- 测试 ----

func TestHealthz(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(gdb)
	w := doJSON(t, r, http.MethodGet, "/healthz", nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", w.Code)
	}
}

// 主流程：注册（自动登录）→ me → 登录 → 注销（auth-flow §3/§8）。
func TestRegisterLoginMeLogoutFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	seedCode(t, gdb, "CODE1")
	r := newEngine(gdb)

	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "email": "alice@x.edu", "password": "secret123", "invite_code": "CODE1",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body=%s", w.Code, w.Body.String())
	}
	reg := decodeAuthResp(t, w)
	if reg.Token == "" {
		t.Error("注册应返回 token（自动登录）")
	}
	if reg.User.Role != "user" {
		t.Errorf("注册返回 role = %q, want user（member→user 边界映射）", reg.User.Role)
	}

	// 邀请码已标记使用：同码再注册应失败
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "bob", "email": "bob@x.edu", "password": "secret123", "invite_code": "CODE1",
	}, "")
	if w2.Code != http.StatusBadRequest {
		t.Errorf("复用邀请码 status = %d, want 400", w2.Code)
	}

	// me
	w3 := doJSON(t, r, http.MethodGet, "/api/v1/auth/me", nil, reg.Token)
	if w3.Code != http.StatusOK {
		t.Fatalf("me status = %d, body=%s", w3.Code, w3.Body.String())
	}
	var me userView
	if err := json.Unmarshal(w3.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Username != "alice" {
		t.Errorf("me username = %q, want alice", me.Username)
	}

	// 登录
	w4 := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "secret123",
	}, "")
	if w4.Code != http.StatusOK {
		t.Fatalf("login status = %d, body=%s", w4.Code, w4.Body.String())
	}
	if decodeAuthResp(t, w4).Token == "" {
		t.Error("登录应返回 token")
	}

	// 注销（空实现）
	w5 := doJSON(t, r, http.MethodPost, "/api/v1/auth/logout", nil, reg.Token)
	if w5.Code != http.StatusOK {
		t.Errorf("logout status = %d, want 200", w5.Code)
	}
}

func TestRegisterValidation(t *testing.T) {
	gdb := testutil.SetupPG(t)
	seedCode(t, gdb, "CODE2")
	seedCode(t, gdb, "CODE2B")
	r := newEngine(gdb)

	// 非法邀请码 → 400
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "email": "alice@x.edu", "password": "secret123", "invite_code": "NOPE",
	}, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("非法邀请码 status = %d, want 400", w.Code)
	}

	// 邮箱格式非法 → 400（binding）
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "email": "not-an-email", "password": "secret123", "invite_code": "CODE2",
	}, "")
	if w2.Code != http.StatusBadRequest {
		t.Errorf("邮箱格式非法 status = %d, want 400", w2.Code)
	}

	// 正常注册
	w3 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "email": "alice@x.edu", "password": "secret123", "invite_code": "CODE2",
	}, "")
	if w3.Code != http.StatusCreated {
		t.Fatalf("正常注册 status = %d, body=%s", w3.Code, w3.Body.String())
	}

	// 用户名重复 → 409（用新邀请码，避免先触发邀请码校验；服务端邀请码优先校验，防用户名枚举）
	w4 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "email": "other@x.edu", "password": "secret123", "invite_code": "CODE2B",
	}, "")
	if w4.Code != http.StatusConflict {
		t.Errorf("用户名重复 status = %d, want 409", w4.Code)
	}
}

func TestLoginFailures(t *testing.T) {
	gdb := testutil.SetupPG(t)
	seedCode(t, gdb, "CODE3")
	r := newEngine(gdb)

	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "bob", "email": "bob@x.edu", "password": "secret123", "invite_code": "CODE3",
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d", w.Code)
	}

	// 密码错误 → 401 同文案
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "bob", "password": "wrongpass",
	}, "")
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("密码错误 status = %d, want 401", w2.Code)
	}

	// 用户不存在 → 同样 401（防枚举）
	w3 := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "ghost", "password": "whatever",
	}, "")
	if w3.Code != http.StatusUnauthorized {
		t.Errorf("用户不存在 status = %d, want 401", w3.Code)
	}
}

func TestMeRequiresToken(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(gdb)
	w := doJSON(t, r, http.MethodGet, "/api/v1/auth/me", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("me 无 token status = %d, want 401", w.Code)
	}
}
