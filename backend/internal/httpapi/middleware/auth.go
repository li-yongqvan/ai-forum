// Package middleware 提供 Gin 鉴权中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
)

type ctxKey string

const (
	ctxKeyIdentity ctxKey = "auth_identity"
)

// Auth 解析 Authorization: Bearer <token> 并注入身份到 context（auth-flow §1 token 传输约定）。
// token 缺失/非法/过期统一 401。
func Auth(mgr *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		claims, err := mgr.Verify(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			return
		}
		c.Set(string(ctxKeyIdentity), claims)
		c.Next()
	}
}

// OptionalAuth 可选鉴权：有合法 token 则注入身份；无/非法 token 按游客继续（不阻断）。
// 用于公开读接口（feed/详情/评论树）的个性化——is_liked/is_faved 与关注流需要查看者身份，
// 但浏览不设登录墙（#9 §4 登录墙：内容开放浏览）。
func OptionalAuth(mgr *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if ok && token != "" {
			if claims, err := mgr.Verify(token); err == nil {
				c.Set(string(ctxKeyIdentity), claims)
			}
		}
		c.Next()
	}
}

// Identity 从 context 取当前请求身份；未登录返回 false。
func Identity(c *gin.Context) (*auth.Claims, bool) {
	v, ok := c.Get(string(ctxKeyIdentity))
	if !ok {
		return nil, false
	}
	claims, ok := v.(*auth.Claims)
	return claims, ok
}

// RequireRole 拦截不具备任一指定角色的请求（#9 §5.0：前端按角色渲染只是体验层，接口必须独立鉴权）。
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		claims, ok := Identity(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		if !allowed[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			return
		}
		c.Next()
	}
}
