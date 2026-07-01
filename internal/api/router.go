// Package api 装配 HTTP 路由、中间件与 handler。
package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/zyd16888/telegram-message-forward/internal/api/handler"
	"github.com/zyd16888/telegram-message-forward/internal/api/middleware"
)

// Deps 是路由装配所需的依赖。
type Deps struct {
	Logger         *slog.Logger
	TokenValidator middleware.TokenValidator
}

// NewRouter 构建 gin 引擎。
func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(deps.Logger))

	health := handler.NewHealthHandler()
	r.GET("/healthz", health.Health)

	// 管理 API 需要 token 鉴权。
	api := r.Group("/api/v1")
	api.Use(middleware.Auth(deps.TokenValidator))
	{
		// TODO: 挂载 accounts / sources / sinks / rules / deliveries handler。
		_ = api
	}

	return r
}
