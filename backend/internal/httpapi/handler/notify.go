package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/notify"
)

// NotifyHandler 暴露通知域端点（#9 登录墙：通知接口必须鉴权）。
type NotifyHandler struct {
	svc notify.Service
}

// NewNotifyHandler 装配 NotifyHandler。
func NewNotifyHandler(svc notify.Service) *NotifyHandler {
	return &NotifyHandler{svc: svc}
}

// respondNotifyError 将通知域哨兵错误映射为 HTTP 状态。
func respondNotifyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, notify.ErrNotFound):
		respondError(c, http.StatusNotFound, "通知不存在")
	case errors.Is(err, notify.ErrInvalidType):
		respondError(c, http.StatusBadRequest, "通知类型非法")
	default:
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
	}
}

// ListNotifications GET /api/v1/notifications?page=&page_size=&unread=1
func (h *NotifyHandler) ListNotifications(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	views, err := h.svc.ListNotifications(c.Request.Context(), notify.ListQuery{
		UserID:     claims.UserID,
		Page:       page,
		PageSize:   pageSize,
		UnreadOnly: c.Query("unread") == "1",
	})
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
}

// UnreadCount GET /api/v1/notifications/unread_count（底部 Tab badge）
func (h *NotifyHandler) UnreadCount(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	n, err := h.svc.UnreadCount(c.Request.Context(), claims.UserID)
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

// MarkRead POST /api/v1/notifications/:id/read
func (h *NotifyHandler) MarkRead(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	id, ok := pathID(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), id, claims.UserID); err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// MarkAllRead POST /api/v1/notifications/read-all
func (h *NotifyHandler) MarkAllRead(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	if err := h.svc.MarkAllRead(c.Request.Context(), claims.UserID); err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
