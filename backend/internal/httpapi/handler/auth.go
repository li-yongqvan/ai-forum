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

type registerReq struct {
	Username   string `json:"username" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	InviteCode string `json:"invite_code" binding:"required"`
}

// Register POST /api/v1/auth/register（auth-flow §3：注册即自动登录）。
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
		case errors.Is(err, user.ErrUsernameTaken), errors.Is(err, user.ErrEmailTaken):
			respondError(c, http.StatusConflict, "用户名或邮箱已存在")
		case errors.Is(err, user.ErrInvalidInvite):
			respondError(c, http.StatusBadRequest, "邀请码无效或已使用")
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
