package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// FollowHandler 统一关注端点（#9 §5.2：关注用户/板块/话题，#4 目标类型拆分 → handler 按类型分派）。
type FollowHandler struct {
	users   user.Service
	content content.Service
}

// NewFollowHandler 装配 FollowHandler。
func NewFollowHandler(users user.Service, content content.Service) *FollowHandler {
	return &FollowHandler{users: users, content: content}
}

type followReq struct {
	TargetType string `json:"target_type" binding:"required"` // user | board | topic
	TargetID   int64  `json:"target_id" binding:"required"`
}

// Follow POST /api/v1/follows
func (h *FollowHandler) Follow(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req followReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	ctx := c.Request.Context()
	var err error
	switch req.TargetType {
	case "user":
		err = h.users.Follow(ctx, user.FollowCmd{FollowerID: claims.UserID, TargetID: req.TargetID})
	case "board":
		err = h.content.FollowBoard(ctx, content.FollowBoardCmd{FollowerID: claims.UserID, BoardID: req.TargetID})
	case "topic":
		err = h.content.FollowTopic(ctx, content.FollowTopicCmd{FollowerID: claims.UserID, TopicID: req.TargetID})
	default:
		respondError(c, http.StatusBadRequest, "target_type 非法（user|board|topic）")
		return
	}
	if err != nil {
		respondFollowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// Unfollow DELETE /api/v1/follows?target_type=&target_id=
func (h *FollowHandler) Unfollow(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	targetType := c.Query("target_type")
	targetID, err := strconv.ParseInt(c.Query("target_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	ctx := c.Request.Context()
	switch targetType {
	case "user":
		err = h.users.Unfollow(ctx, user.FollowCmd{FollowerID: claims.UserID, TargetID: targetID})
	case "board":
		err = h.content.UnfollowBoard(ctx, content.FollowBoardCmd{FollowerID: claims.UserID, BoardID: targetID})
	case "topic":
		err = h.content.UnfollowTopic(ctx, content.FollowTopicCmd{FollowerID: claims.UserID, TopicID: targetID})
	default:
		respondError(c, http.StatusBadRequest, "target_type 非法（user|board|topic）")
		return
	}
	if err != nil {
		respondFollowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// respondFollowError 映射 user/content 两包的关注相关错误。
func respondFollowError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, user.ErrNotFound), errors.Is(err, content.ErrBoardNotFound),
		errors.Is(err, content.ErrTopicNotFound):
		respondError(c, http.StatusNotFound, "目标不存在")
	case errors.Is(err, user.ErrSelfFollow):
		respondError(c, http.StatusBadRequest, "不能关注自己")
	case errors.Is(err, user.ErrAlreadyFollow), errors.Is(err, content.ErrAlreadyFollowed):
		respondError(c, http.StatusConflict, "已关注")
	case errors.Is(err, content.ErrForbidden):
		respondError(c, http.StatusForbidden, "权限不足")
	default:
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
	}
}
