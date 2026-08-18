// Package httpapi 组装 Gin 引擎、中间件与路由。
package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
	"github.com/li-yongqvan/ai-forum/backend/internal/config"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/handler"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// NewEngine 组装 Gin 引擎与全部路由（auth-flow §9 端点）。
func NewEngine(cfg config.Config, jwtMgr *auth.Manager, userSvc user.Service) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	uh := handler.NewUserHandler(userSvc)

	r.GET("/healthz", handler.Health)

	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", uh.Register) // 公开
			authGroup.POST("/login", uh.Login)       // 公开
			authed := authGroup.Group("")
			authed.Use(middleware.Auth(jwtMgr))
			authed.POST("/logout", uh.Logout) // 空实现（无状态 JWT）
			authed.GET("/me", uh.Me)
		}
		// content / notify / moderation 路由由后续 ticket 挂载
	}
	return r
}
