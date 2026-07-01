package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TokenValidator 校验 API token 是否有效。
type TokenValidator interface {
	Validate(token string) bool
}

// Auth 校验 Authorization: Bearer <token>。管理 API 必须经过此中间件。
func Auth(v TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" || !v.Validate(token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}
		c.Next()
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}
