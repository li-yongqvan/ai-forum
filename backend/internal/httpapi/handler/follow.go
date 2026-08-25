package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/notify"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// FollowHandler 统一关注端点（#9 §5.2：关注用户/板块/话题，#4 目标类型拆分 → handler 按类型分派）。
// notify：关注用户成功后生成 follow 通知（#32 跨包聚合在 handler 层，SOP §5.8）。
type FollowHandler struct {
	users   user.Service
	content content.Service
	notify  notify.Service
}

// NewFollowHandler 装配 FollowHandler。
func NewFollowHandler(users user.Service, content content.Service, notify notify.Service) *FollowHandler {
	return &FollowHandler{users: users, content: content, notify: notify}
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
	// 关注用户成功后生成 follow 通知（#32：快照自足，不通知自己——服务端已拒自关；
	// 通知失败不回滚关注成功，吞错仅影响通知落库）。
	if req.TargetType == "user" {
		h.notifyFollow(ctx, claims.UserID, claims.Username, req.TargetID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// notifyFollow 生成「X 关注了你」通知：接收人 = 被关注者，actor = 当前关注者（快照就地算，#4 D4）。
// 跳转锚点按 IA v2 §4：follow → 用户主页（target 指向关注者本人主页）。
func (h *FollowHandler) notifyFollow(ctx context.Context, followerID int64, followerName string, targetID int64) {
	var avatar *string
	if u, err := h.users.GetUser(ctx, followerID); err == nil {
		avatar = u.AvatarURL
	}
	_ = h.notify.CreateNotification(ctx, notify.CreateNotificationCmd{
		RecipientID: targetID,
		Type:        "follow",
		ActorID:     &followerID,
		ActorName:   &followerName,
		ActorAvatar: avatar,
		TargetType:  strPtr("user"),
		TargetID:    &followerID,
	})
}

// strPtr 返回字符串指针（构造快照字段用）。
func strPtr(s string) *string { return &s }

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

// ListFollows GET /api/v1/follows?target_type=user|board|topic（需登录；我关注的用户/板块/话题，
// 按关注时间倒序 + 分页，#23）。target_type 分派与 POST/DELETE /follows 同构。
func (h *FollowHandler) ListFollows(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ctx := c.Request.Context()
	switch c.Query("target_type") {
	case "user":
		views, err := h.users.ListFollowedUsers(ctx, user.ListFollowedUsersQuery{ViewerID: claims.UserID, Page: page, PageSize: pageSize})
		if err != nil {
			respondFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
	case "board":
		views, err := h.content.ListFollowedBoards(ctx, content.ListFollowedBoardsQuery{ViewerID: claims.UserID, Page: page, PageSize: pageSize})
		if err != nil {
			respondFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
	case "topic":
		views, err := h.content.ListFollowedTopics(ctx, content.ListFollowedTopicsQuery{ViewerID: claims.UserID, Page: page, PageSize: pageSize})
		if err != nil {
			respondFollowError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
	default:
		// 缺参与非法值统一 400（与 POST/DELETE 的 default 分支对齐）
		respondError(c, http.StatusBadRequest, "target_type 非法（user|board|topic）")
	}
}

// respondFollowError 映射 user/content 两包的关注相关错误。
func respondFollowError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, user.ErrAuthRequired), errors.Is(err, content.ErrAuthRequired):
		respondError(c, http.StatusUnauthorized, "需要登录")
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
