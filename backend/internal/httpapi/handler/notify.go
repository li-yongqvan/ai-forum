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
	svc   notify.Service
	peers notify.PeerProvider
}

// NewNotifyHandler 装配 NotifyHandler。
func NewNotifyHandler(svc notify.Service, peers notify.PeerProvider) *NotifyHandler {
	return &NotifyHandler{svc: svc, peers: peers}
}

// respondNotifyError 将通知域哨兵错误映射为 HTTP 状态。
func respondNotifyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, notify.ErrNotFound):
		respondError(c, http.StatusNotFound, "通知不存在")
	case errors.Is(err, notify.ErrInvalidType):
		respondError(c, http.StatusBadRequest, "通知类型非法")
	case errors.Is(err, notify.ErrEmptyMessage), errors.Is(err, notify.ErrSelfMessage), errors.Is(err, notify.ErrMessageTooLong):
		respondError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, notify.ErrRateLimited):
		respondError(c, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, notify.ErrPeerNotFound):
		respondError(c, http.StatusNotFound, "用户不存在")
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
// 自 #59 起语义为「通知未读 + 私信未读」合并值。
func (h *NotifyHandler) UnreadCount(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	ctx := c.Request.Context()
	nc, err := h.svc.UnreadCount(ctx, claims.UserID)
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	mc, err := h.svc.UnreadMessageCount(ctx, claims.UserID)
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": nc + mc})
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

// ---- 私信（#59） ----

type sendMessageReq struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

// SendMessage POST /api/v1/messages
func (h *NotifyHandler) SendMessage(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req sendMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	ctx := c.Request.Context()
	// 对端存在性校验（notify 零依赖，画像经 PeerProvider 解析）。
	if _, err := h.peers.GetPeerView(ctx, req.ToUserID); err != nil {
		if errors.Is(err, notify.ErrPeerNotFound) {
			respondNotifyError(c, err)
			return
		}
		respondNotifyError(c, err)
		return
	}
	view, err := h.svc.SendMessage(ctx, notify.SendMessageCmd{
		FromUserID: claims.UserID,
		ToUserID:   req.ToUserID,
		Content:    req.Content,
	})
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// ListConversations GET /api/v1/conversations
func (h *NotifyHandler) ListConversations(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	views, err := h.svc.ListConversations(c.Request.Context(), notify.ListQuery{
		UserID:   claims.UserID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	items := make([]gin.H, 0, len(views))
	ctx := c.Request.Context()
	for _, v := range views {
		peerName := ""
		var peerAvatar *string
		if p, err := h.peers.GetPeerView(ctx, v.PeerID); err == nil {
			peerName = p.Username
			peerAvatar = p.Avatar
		}
		items = append(items, gin.H{
			"peer_id":      v.PeerID,
			"peer_name":    peerName,
			"peer_avatar":  peerAvatar,
			"last_message": v.LastMessage,
			"unread_count": v.UnreadCount,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "page_size": pageSize})
}

// ListConversationMessages GET /api/v1/conversations/:peerID/messages
func (h *NotifyHandler) ListConversationMessages(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	peerID, ok := pathInt64(c, "peerID")
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	views, err := h.svc.ListConversationMessages(c.Request.Context(), claims.UserID, peerID, notify.ListQuery{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
}

// MarkConversationRead POST /api/v1/conversations/:peerID/read
func (h *NotifyHandler) MarkConversationRead(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	peerID, ok := pathInt64(c, "peerID")
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := h.svc.MarkConversationRead(c.Request.Context(), claims.UserID, peerID); err != nil {
		respondNotifyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
