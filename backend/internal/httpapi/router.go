// Package httpapi 组装 Gin 引擎、中间件与路由。
package httpapi

import (
	"context"
	"errors"

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

// notifyPeerAdapter 使 user.Service 满足 notify.PeerProvider（组合根适配：notify 零依赖，
// 对端画像由 user 域提供，handler 层合并）。
type notifyPeerAdapter struct {
	svc user.Service
}

func (a notifyPeerAdapter) GetPeerView(ctx context.Context, id int64) (notify.PeerView, error) {
	u, err := a.svc.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return notify.PeerView{}, notify.ErrPeerNotFound
		}
		return notify.PeerView{}, err
	}
	return notify.PeerView{ID: u.ID, Username: u.Username, Avatar: u.AvatarURL}, nil
}

// NewNotifyPeerProvider 构造 notify.PeerProvider（供 NotifyHandler 解析对端画像）。
func NewNotifyPeerProvider(svc user.Service) notify.PeerProvider {
	return notifyPeerAdapter{svc: svc}
}

// activeCheckerAdapter 使 user.Service 满足 middleware.ActiveChecker（#34：RequireActive 写拦截 seam）。
type activeCheckerAdapter struct {
	svc user.Service
}

func (a activeCheckerAdapter) IsActive(ctx context.Context, userID int64) (bool, error) {
	return a.svc.IsActive(ctx, userID)
}

// NewActiveChecker 构造 middleware.ActiveChecker（供 RequireActive 写拦截）。
func NewActiveChecker(svc user.Service) middleware.ActiveChecker {
	return activeCheckerAdapter{svc: svc}
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
	nh := handler.NewNotifyHandler(notifySvc, NewNotifyPeerProvider(userSvc))
	mh := handler.NewModerationHandler(moderationSvc, userSvc)

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
		// logout/me 豁免 RequireActive（账户生命周期接口：登出放行、读不禁，#34 D3）
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

		// 登录态只读（#23/#32 我的收藏/关注/通知列表）——写拦截只拦写，读不禁（#34 D3）
		authed.GET("/favorites", ch.ListFavorites)
		authed.GET("/follows", fh.ListFollows)
		authed.GET("/notifications", nh.ListNotifications)
		authed.GET("/notifications/unread_count", nh.UnreadCount)
		authed.GET("/conversations", nh.ListConversations)
		authed.GET("/conversations/:peerID/messages", nh.ListConversationMessages)

		// 全部登录态写操作（#34：RequireActive 拦截被封用户，JWT 7 天内即时生效须查 DB）
		writers := authed.Group("")
		writers.Use(middleware.RequireActive(NewActiveChecker(userSvc)))
		writers.POST("/posts", ch.CreatePost)
		writers.DELETE("/posts/:id", ch.DeletePost)
		writers.POST("/comments", ch.CreateComment)
		writers.DELETE("/comments/:id", ch.DeleteComment)
		writers.POST("/likes", ch.Like)
		writers.DELETE("/likes", ch.Unlike)
		writers.POST("/favorites", ch.Favorite)
		writers.DELETE("/favorites", ch.Unfavorite)
		writers.POST("/follows", fh.Follow)
		writers.DELETE("/follows", fh.Unfollow)
		writers.POST("/notifications/:id/read", nh.MarkRead)
		writers.POST("/notifications/read-all", nh.MarkAllRead)
		writers.POST("/messages", nh.SendMessage)
		writers.POST("/conversations/:peerID/read", nh.MarkConversationRead)
		writers.POST("/reports", mh.CreateReport)
		writers.POST("/uploads", upl.Upload)

		// 管理操作（moderator+，双保险：#9 §5.0；writers 子组 → 被封 mod 也不许管理）
		mod := writers.Group("")
		mod.Use(middleware.RequireRole("moderator", "admin"))
		mod.POST("/posts/:id/pin", ch.PinPost)
		mod.POST("/posts/:id/feature", ch.FeaturePost)
		// 举报处理队列（#33）
		mod.GET("/moderation/reports", mh.ListReports)
		mod.GET("/moderation/reports/count", mh.CountReports)
		mod.POST("/moderation/reports/:id/handle", mh.HandleReport)

		// 治理管理（admin-only：#9 §5.0 封禁/解封仅 admin）
		admin := writers.Group("")
		admin.Use(middleware.RequireRole("admin"))
		admin.POST("/moderation/users/:id/ban", mh.BanUser)
		admin.POST("/moderation/users/:id/unban", mh.UnbanUser)
	}
	return r
}
