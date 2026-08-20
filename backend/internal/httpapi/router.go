// Package httpapi 组装 Gin 引擎、中间件与路由。
package httpapi

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
	"github.com/li-yongqvan/ai-forum/backend/internal/config"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/handler"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/moderation"
	"github.com/li-yongqvan/ai-forum/backend/notify"
	"github.com/li-yongqvan/ai-forum/backend/upload"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// userProviderAdapter 使 user.Service 满足 content.UserProvider（组合根适配：#7 Adapter 在 seam；
// 内容域不 import user，作者画像与关注数据经此适配注入）。
type userProviderAdapter struct {
	svc user.Service
}

func (a userProviderAdapter) FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error) {
	return a.svc.FollowedUserIDs(ctx, userID)
}

func (a userProviderAdapter) GetUserView(ctx context.Context, id int64) (content.UserView, error) {
	u, err := a.svc.GetUser(ctx, id)
	if err != nil {
		return content.UserView{}, err
	}
	return content.UserView{ID: u.ID, Username: u.Username, AvatarURL: u.AvatarURL}, nil
}

func (a userProviderAdapter) FollowsUser(ctx context.Context, followerID, targetID int64) (bool, error) {
	return a.svc.FollowsUser(ctx, followerID, targetID)
}

// NewUserProvider 构造 content.UserProvider（供 main 装配 content 服务）。
func NewUserProvider(svc user.Service) content.UserProvider {
	return userProviderAdapter{svc: svc}
}

// NewEngine 组装 Gin 引擎与全部路由。
func NewEngine(cfg config.Config, jwtMgr *auth.Manager, userSvc user.Service, contentSvc content.Service, uploadSvc upload.Service, notifySvc notify.Service, moderationSvc moderation.Service) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	uh := handler.NewUserHandler(userSvc)
	ch := handler.NewContentHandler(contentSvc)
	fh := handler.NewFollowHandler(userSvc, contentSvc, notifySvc)
	ph := handler.NewProfileHandler(userSvc, contentSvc)
	upl := handler.NewUploadHandler(uploadSvc)
	nh := handler.NewNotifyHandler(notifySvc)
	mh := handler.NewModerationHandler(moderationSvc)

	r.GET("/healthz", handler.Health)

	// 开发环境由 Go 托管 /uploads（#12 D1：生产 nginx 接管，图片 GET 不经 Go；
	// 目录由 Store 首次上传时自动创建，此前请求 404 属预期）
	if cfg.Env != "production" && cfg.UploadsDir != "" {
		r.Static("/uploads", cfg.UploadsDir)
	}

	api := r.Group("/api/v1")
	{
		// 鉴权中间件：内容写入/管理操作（#9 §5.0）
		authed := api.Group("")
		authed.Use(middleware.Auth(jwtMgr))

		// auth（auth-flow §9）
		api.POST("/auth/register", uh.Register)
		api.POST("/auth/login", uh.Login)
		authed.POST("/auth/logout", uh.Logout)
		authed.GET("/auth/me", uh.Me)

		// 内容读取（公开可看，可选鉴权以提供个性化 is_liked/is_faved 与关注流，#9 登录墙）
		reads := api.Group("")
		reads.Use(middleware.OptionalAuth(jwtMgr))
		reads.GET("/boards", ch.ListBoards)
		reads.GET("/topics", ch.ListTopics)
		reads.GET("/posts", ch.ListPosts)
		reads.GET("/posts/:id", ch.GetPost)
		reads.GET("/posts/:id/comments", ch.GetComments)
		reads.GET("/users/:id", ph.GetUserProfile)

		// 内容写入（操作需登录）
		authed.POST("/posts", ch.CreatePost)
		authed.DELETE("/posts/:id", ch.DeletePost)
		authed.POST("/comments", ch.CreateComment)
		authed.DELETE("/comments/:id", ch.DeleteComment)
		authed.POST("/likes", ch.Like)
		authed.DELETE("/likes", ch.Unlike)
		authed.POST("/favorites", ch.Favorite)
		authed.DELETE("/favorites", ch.Unfavorite)
		authed.GET("/favorites", ch.ListFavorites) // #23 我的收藏（按收藏时间倒序 + 分页）
		authed.POST("/follows", fh.Follow)
		authed.DELETE("/follows", fh.Unfollow)
		authed.GET("/follows", fh.ListFollows) // #23 我关注的用户/板块/话题（target_type 分派）

		// 通知中心（#32：需登录，#9 登录墙）
		authed.GET("/notifications", nh.ListNotifications)
		authed.GET("/notifications/unread_count", nh.UnreadCount)
		authed.POST("/notifications/:id/read", nh.MarkRead)
		authed.POST("/notifications/read-all", nh.MarkAllRead)

		// 举报创建（#33：需登录）
		authed.POST("/reports", mh.CreateReport)

		// 图片上传（#12：需登录；无 DB，图片 GET 由 nginx/开发态 Go 托管）
		authed.POST("/uploads", upl.Upload)

		// 管理操作（moderator+，双保险：#9 §5.0）
		mod := authed.Group("")
		mod.Use(middleware.RequireRole("moderator", "admin"))
		mod.POST("/posts/:id/pin", ch.PinPost)
		mod.POST("/posts/:id/feature", ch.FeaturePost)
		// 举报处理队列（#33）
		mod.GET("/moderation/reports", mh.ListReports)
		mod.GET("/moderation/reports/count", mh.CountReports)
		mod.POST("/moderation/reports/:id/handle", mh.HandleReport)
	}
	return r
}
