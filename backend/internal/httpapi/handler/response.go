package handler

import "github.com/gin-gonic/gin"

// respondError 输出统一错误体 { "error": "<msg>" }。
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}
