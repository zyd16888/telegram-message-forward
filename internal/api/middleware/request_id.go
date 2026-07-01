// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// HeaderRequestID 是请求 ID 的响应头名。
const HeaderRequestID = "X-Request-Id"

// ContextRequestID 是请求 ID 在 gin.Context 中的键。
const ContextRequestID = "request_id"

// RequestID 为每个请求生成或透传请求 ID。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = newRequestID()
		}
		c.Set(ContextRequestID, id)
		c.Header(HeaderRequestID, id)
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
