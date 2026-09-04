package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// UserHandler 暴露 user 包的 HTTP 端点（auth-flow §9）。
type UserHandler struct {
	svc user.Service
}

// NewUserHandler 装配 UserHandler。
func NewUserHandler(svc user.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

// Health 健康检查（#8：/healthz 返回 200 OK 即健康）。
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// registerReq：#76 open 版一次性改动（随 revert 还原）——email/invite_code 的 binding 标签整条去掉
// （仅去 required 会留空串仍触发 email 校验→400，评审 F5⑤）；入参被 service 忽略（占位邮箱生成、免码）。
// username/password 保留 required；trim 后空 username 由 service 守卫（ErrEmptyUsername→400）。
type registerReq struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"`
}

// Register POST /api/v1/auth/register（auth-flow §3：注册即自动登录；#76 open 版免码）。
func (h *UserHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	res, err := h.svc.Register(c.Request.Context(), user.RegisterCmd{
		Username:   req.Username,
		Email:      req.Email,
		Password:   req.Password,
		InviteCode: req.InviteCode,
	})
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUsernameTaken):
			// #76：撞名 409 + 现算建议名（Q4 铁律：每次现算；suggestion 经 seam 取、不进错误对象，§6.2）。
			// 前端按 code=username_taken 分支做一键采用；建议名探测失败则降级为无 suggestion 的 409。
			sugg, sErr := h.svc.SuggestNextUsername(c.Request.Context(), req.Username)
			if sErr != nil {
				respondError(c, http.StatusConflict, "用户名已被占用")
				return
			}
			c.JSON(http.StatusConflict, gin.H{
				"error":      "用户名已被占用",
				"code":       "username_taken",
				"suggestion": sugg,
			})
		case errors.Is(err, user.ErrEmailTaken):
			respondError(c, http.StatusConflict, "用户名或邮箱已存在")
		case errors.Is(err, user.ErrEmptyUsername):
			respondError(c, http.StatusBadRequest, "用户名必填")
		case errors.Is(err, user.ErrWeakPassword):
			respondError(c, http.StatusBadRequest, "密码至少 8 位")
		default:
			respondError(c, http.StatusInternalServerError, "服务器内部错误")
		}
		return
	}
	c.JSON(http.StatusCreated, res)
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/v1/auth/login（auth-flow §2：失败统一 401）。
func (h *UserHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	res, err := h.svc.Login(c.Request.Context(), user.LoginCmd{Username: req.Username, Password: req.Password})
	if err != nil {
		// ErrBadCredential / ErrBanned 统一 401 同文案，不泄露账号状态（防枚举）
		if errors.Is(err, user.ErrBadCredential) || errors.Is(err, user.ErrBanned) {
			respondError(c, http.StatusUnauthorized, "用户名或密码错误")
			return
		}
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	c.JSON(http.StatusOK, res)
}

// Logout POST /api/v1/auth/logout（auth-flow §8：JWT 无状态，前端清除 token；后端空实现）。
func (h *UserHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// Me GET /api/v1/auth/me：返回当前登录用户信息。
func (h *UserHandler) Me(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "未登录")
		return
	}
	u, err := h.svc.GetUser(c.Request.Context(), claims.UserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	c.JSON(http.StatusOK, u)
}
