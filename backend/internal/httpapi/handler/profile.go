package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// ProfileHandler 提供用户公开资料（user + content 组合根装配，#4 进程内调用）。
type ProfileHandler struct {
	users   user.Service
	content content.Service
}

// NewProfileHandler 装配 ProfileHandler。
func NewProfileHandler(users user.Service, content content.Service) *ProfileHandler {
	return &ProfileHandler{users: users, content: content}
}

type profileResp struct {
	ID             int64         `json:"id"`
	Username       string        `json:"username"`
	AvatarURL      *string       `json:"avatar_url"`
	Bio            *string       `json:"bio"`
	JoinedAt       time.Time     `json:"joined_at"`
	PostCount      int           `json:"post_count"`
	FollowerCount  int           `json:"follower_count"`
	FollowingCount int           `json:"following_count"`
	Viewer         *followViewer `json:"viewer,omitempty"` // 登录态附 following
	Banned         *bool         `json:"banned,omitempty"` // #34：仅 admin/self 可见（治理信息不外泄）
}

type followViewer struct {
	Following bool `json:"following"`
}

// GetUserProfile GET /api/v1/users/:id（公开；登录态附 viewer.following，供关注按钮初始态）。
func (h *ProfileHandler) GetUserProfile(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	var viewerID int64
	var viewerRole string
	if claims, authed := middleware.Identity(c); authed {
		viewerID = claims.UserID
		viewerRole = claims.Role
	}
	ctx := c.Request.Context()

	p, err := h.users.PublicProfile(ctx, user.PublicProfileCmd{TargetID: id, ViewerID: viewerID})
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			respondError(c, http.StatusNotFound, "用户不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	postCount, err := h.content.CountPostsByAuthor(ctx, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	resp := profileResp{
		ID:             p.ID,
		Username:       p.Username,
		AvatarURL:      p.AvatarURL,
		Bio:            p.Bio,
		JoinedAt:       p.CreatedAt,
		PostCount:      postCount,
		FollowerCount:  p.FollowerCount,
		FollowingCount: p.FollowingCount,
	}
	if viewerID != 0 {
		resp.Viewer = &followViewer{Following: p.Following}
	}
	if viewerRole == "admin" || (viewerID != 0 && viewerID == id) {
		resp.Banned = &p.Banned // 评审 Q5 附带：admin 或 self-view 才可见封禁状态
	}
	c.JSON(http.StatusOK, resp)
}
