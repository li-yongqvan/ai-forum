package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ActiveChecker 供 RequireActive 查询当前用户是否可执行写操作（#34）。
// 定义在 middleware 包（深模块：middleware 不 import user），实现由组合根 httpapi 适配注入。
type ActiveChecker interface {
	IsActive(ctx context.Context, userID int64) (bool, error)
}

// RequireActive 拦截被封禁用户的请求（#34：JWT claims 最长 7 天，封禁即时生效须查 DB）。
// 顺序：Auth → RequireActive → RequireRole → handler。
// 无 token→401「未登录」；DB 查询失败→500（fail-closed，安全边界）；被封→403 + code（F5：前端匹配 code 而非文案）。
func RequireActive(checker ActiveChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := Identity(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		active, err := checker.IsActive(c.Request.Context(), claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
			return
		}
		if !active {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "账号已被封禁", "code": "account_banned"})
			return
		}
		c.Next()
	}
}
