package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
	"github.com/li-yongqvan/ai-forum/backend/internal/config"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"github.com/li-yongqvan/ai-forum/backend/moderation"
	"github.com/li-yongqvan/ai-forum/backend/notify"
	"github.com/li-yongqvan/ai-forum/backend/upload"
	"github.com/li-yongqvan/ai-forum/backend/user"
	"gorm.io/gorm"
)

// placeholderEmailRe 占位邮箱格式（#76 open 版）：u_<16字节32位小写hex>@local.invalid。
var placeholderEmailRe = regexp.MustCompile(`^u_[0-9a-f]{32}@local\.invalid$`)

// ---- 装配 ----

func newEngine(t *testing.T, gdb *gorm.DB) *gin.Engine {
	t.Helper()
	return newEngineWithUploads(t, gdb, t.TempDir(), 0)
}

// newEngineWithUploads 允许指定上传目录与大小上限（上传集成测试用；maxBytes<=0 走默认 5MB）。
func newEngineWithUploads(t *testing.T, gdb *gorm.DB, uploadDir string, maxBytes int64) *gin.Engine {
	t.Helper()
	jwtMgr := auth.NewManager("test-secret", 7*24*time.Hour)
	userSvc := user.NewService(user.NewGormRepo(gdb), jwtMgr)
	// #72：content 需要 Notifier 包装 notifySvc，须先装配 notifySvc
	notifySvc := notify.NewService(notify.NewGormRepo(gdb))
	contentSvc := content.NewService(content.NewGormRepo(gdb), httpapi.NewUserProvider(userSvc), httpapi.NewContentNotifier(notifySvc))
	uploadSvc := upload.NewService(upload.Config{Dir: uploadDir, MaxBytes: maxBytes})
	moderationSvc := moderation.NewService(
		moderation.NewGormRepo(gdb),
		httpapi.NewContentGateway(contentSvc),
		httpapi.NewUserGateway(userSvc),
		httpapi.NewNotifier(notifySvc),
	)
	return httpapi.NewEngine(
		config.Config{Env: "test", Port: "8080", UploadsDir: uploadDir, MaxUploadBytes: maxBytes},
		jwtMgr, userSvc, contentSvc, uploadSvc, notifySvc, moderationSvc,
	)
}

// seedCode 直插邀请码（auth-flow §7：管理员 DB 直管，MVP 无管理 UI）。
// 0010 迁移后 invitation_codes.created_by 有物理 FK，需先保证 admin 用户存在，并同步序列避免 id 冲突。
func seedCode(t *testing.T, gdb *gorm.DB, code string) {
	t.Helper()
	if err := gdb.Exec(`
		INSERT INTO "user".users (id, email, username, password_hash, role)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (id) DO NOTHING
	`, 1, "admin@example.com", "admin", "hash", "admin").Error; err != nil {
		t.Fatalf("seed admin 用户失败: %v", err)
	}
	if err := gdb.Exec(`
		SELECT setval(pg_get_serial_sequence('"user".users', 'id'), COALESCE((SELECT MAX(id) FROM "user".users), 1), true)
	`).Error; err != nil {
		t.Fatalf("同步 users_id_seq 失败: %v", err)
	}
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
	r := newEngine(t, gdb)
	w := doJSON(t, r, http.MethodGet, "/healthz", nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", w.Code)
	}
}

// 主流程：注册（免码、自动登录）→ me → 登录 → 注销（auth-flow §3/§8）。
// #76 open 版改写：两字段载荷；邀请码不再参与注册（预置一码验证不被消耗，不变量 #1）。
func TestRegisterLoginMeLogoutFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	seedCode(t, gdb, "CODE1")
	r := newEngine(t, gdb)

	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "secret123",
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
	if !placeholderEmailRe.MatchString(reg.User.Email) {
		t.Errorf("占位邮箱 = %q, want u_<32hex>@local.invalid", reg.User.Email)
	}

	// 免码注册不消耗邀请码：预置码仍可查且未使用（不变量 #1）
	var usedBy *int64
	if err := gdb.Raw(`SELECT used_by FROM "user".invitation_codes WHERE code = ?`, "CODE1").Scan(&usedBy).Error; err != nil {
		t.Fatalf("查询邀请码失败: %v", err)
	}
	if usedBy != nil {
		t.Errorf("免码注册不应消耗邀请码, used_by = %v", *usedBy)
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

// #76 open 版改写（原「非法邀请码/邮箱格式 400」随新语义失效）：注册边界校验——
// binding 仅 username/password required；空白 username 靠 service 守卫 400（F5④）。
func TestRegisterValidation(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	// 缺用户名 → 400（binding）
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"password": "secret123",
	}, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("缺用户名 status = %d, want 400", w.Code)
	}

	// 缺密码 → 400（binding）
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice",
	}, "")
	if w2.Code != http.StatusBadRequest {
		t.Errorf("缺密码 status = %d, want 400", w2.Code)
	}

	// 纯空白用户名 → 400（required 对空格放行，service 守卫，F5④）
	w3 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "   ", "password": "secret123",
	}, "")
	if w3.Code != http.StatusBadRequest {
		t.Errorf("空白用户名 status = %d, want 400", w3.Code)
	}

	// 弱密码 → 400（回归：minPasswordLen=8 保留，D3）
	w4 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "short",
	}, "")
	if w4.Code != http.StatusBadRequest {
		t.Errorf("弱密码 status = %d, want 400", w4.Code)
	}

	// 两字段正常注册 → 201（免码，email/invite_code 不再参与校验）
	w5 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "secret123",
	}, "")
	if w5.Code != http.StatusCreated {
		t.Fatalf("正常注册 status = %d, body=%s", w5.Code, w5.Body.String())
	}

	// 用户名重复 → 409
	w6 := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "secret123",
	}, "")
	if w6.Code != http.StatusConflict {
		t.Errorf("用户名重复 status = %d, want 409", w6.Code)
	}
}

// #76 §8：撞名 409 载荷形状（code=username_taken / suggestion）+ 建议名每次现算（Q4 铁律）。
func TestRegister_ConflictSuggestion(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	reg := func(name string) *httptest.ResponseRecorder {
		return doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
			"username": name, "password": "secret123",
		}, "")
	}

	if c := reg("alice").Code; c != http.StatusCreated {
		t.Fatalf("首次注册 status = %d, want 201", c)
	}

	// 撞名 → 409 {error, code:"username_taken", suggestion:"alice_2"}
	w := reg("alice")
	if w.Code != http.StatusConflict {
		t.Fatalf("撞名 status = %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		Suggestion string `json:"suggestion"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 409 body 失败: %v, body=%s", err, w.Body.String())
	}
	if body.Code != "username_taken" || body.Suggestion != "alice_2" || body.Error == "" {
		t.Errorf("409 载荷 = %+v, want code=username_taken suggestion=alice_2", body)
	}

	// 建议名现算（Q4）：alice_2 被占后再次撞名 → suggestion 前移为 alice_3
	if c := reg("alice_2").Code; c != http.StatusCreated {
		t.Fatalf("alice_2 注册 status = %d, want 201", c)
	}
	w2 := reg("alice")
	if w2.Code != http.StatusConflict {
		t.Fatalf("再次撞名 status = %d, body=%s", w2.Code, w2.Body.String())
	}
	var body2 struct {
		Suggestion string `json:"suggestion"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &body2); err != nil {
		t.Fatal(err)
	}
	if body2.Suggestion != "alice_3" {
		t.Errorf("suggestion = %q, want alice_3（每次现算）", body2.Suggestion)
	}
}

func TestLoginFailures(t *testing.T) {
	gdb := testutil.SetupPG(t)
	seedCode(t, gdb, "CODE3")
	r := newEngine(t, gdb)

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
	r := newEngine(t, gdb)
	w := doJSON(t, r, http.MethodGet, "/api/v1/auth/me", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("me 无 token status = %d, want 401", w.Code)
	}
}
