// Package api 装配 HTTP 路由、中间件与 handler。
package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/handler"
	"telegram-message-forward/internal/api/middleware"
)

// Deps 是路由装配所需的依赖。
type Deps struct {
	Logger         *slog.Logger
	TokenValidator middleware.TokenValidator
	// AuthEnabled 为 false 时 /api/v1 不挂 Auth 中间件（开发免鉴权）。
	AuthEnabled bool
	// WebDir 为前端构建产物目录；为空时不托管前端静态文件。
	WebDir string

	Auth         *handler.AuthHandler
	Account      *handler.AccountHandler
	AccountLogin *handler.AccountLoginHandler
	Sink         *handler.SinkHandler
	Source       *handler.SourceHandler
	Template     *handler.TemplateHandler
	Rule         *handler.RuleHandler
	Delivery     *handler.DeliveryHandler
	Token        *handler.TokenHandler
}

// NewRouter 构建 gin 引擎并挂载 /api/v1 资源接口。
func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(deps.Logger))

	health := handler.NewHealthHandler()
	r.GET("/healthz", health.Health)

	// 登录相关端点不经过 Auth 中间件：登录前必须可访问。
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.GET("/bootstrap", deps.Auth.BootstrapStatus)
		authGroup.POST("/bootstrap", deps.Auth.Bootstrap)
		authGroup.POST("/login", deps.Auth.Login)
		authGroup.GET("/me", deps.Auth.Me)
		authGroup.POST("/logout", deps.Auth.Logout)
	}

	// 管理 API 需要 token 鉴权；auth_enabled=false 时跳过（开发免鉴权）。
	v1 := r.Group("/api/v1")
	if deps.AuthEnabled {
		v1.Use(middleware.Auth(deps.TokenValidator))
	}
	{
		accounts := v1.Group("/accounts")
		{
			accounts.GET("", deps.Account.List)
			accounts.POST("", deps.Account.Create)
			accounts.GET("/:id", deps.Account.Get)
			accounts.PUT("/:id", deps.Account.Update)
			accounts.DELETE("/:id", deps.Account.Delete)

			// Telegram 登录 flow（验证码登录）。
			login := accounts.Group("/:id/login")
			{
				login.POST("/start", deps.AccountLogin.Start)
				login.POST("/code", deps.AccountLogin.Code)
				login.POST("/password", deps.AccountLogin.Password)
				login.GET("/status", deps.AccountLogin.Status)
				login.POST("/cancel", deps.AccountLogin.Cancel)

				// 扫码登录。
				qr := login.Group("/qr")
				{
					qr.POST("/start", deps.AccountLogin.QRStart)
					qr.GET("/status", deps.AccountLogin.QRStatus)
					qr.POST("/refresh", deps.AccountLogin.QRRefresh)
					qr.POST("/cancel", deps.AccountLogin.QRCancel)
				}
			}
		}

		sinks := v1.Group("/sinks")
		{
			sinks.GET("", deps.Sink.List)
			sinks.GET("/types", deps.Sink.Types)
			sinks.POST("", deps.Sink.Create)
			sinks.GET("/:id", deps.Sink.Get)
			sinks.PUT("/:id", deps.Sink.Update)
			sinks.DELETE("/:id", deps.Sink.Delete)
		}

		sources := v1.Group("/sources")
		{
			sources.GET("", deps.Source.List)
			sources.POST("", deps.Source.Create)
			sources.POST("/sync", deps.Source.Sync)
			sources.GET("/:id", deps.Source.Get)
			sources.PUT("/:id", deps.Source.Update)
			sources.DELETE("/:id", deps.Source.Delete)
			sources.POST("/:id/start", deps.Source.Start)
			sources.POST("/:id/stop", deps.Source.Stop)
		}

		templates := v1.Group("/templates")
		{
			templates.GET("", deps.Template.List)
			templates.POST("", deps.Template.Create)
			templates.GET("/:id", deps.Template.Get)
			templates.PUT("/:id", deps.Template.Update)
			templates.DELETE("/:id", deps.Template.Delete)
		}

		rules := v1.Group("/rules")
		{
			rules.GET("", deps.Rule.List)
			rules.POST("", deps.Rule.Create)
			rules.GET("/:id", deps.Rule.Get)
			rules.PUT("/:id", deps.Rule.Update)
			rules.DELETE("/:id", deps.Rule.Delete)
		}

		deliveries := v1.Group("/deliveries")
		{
			deliveries.GET("", deps.Delivery.List)
			deliveries.GET("/:id", deps.Delivery.Get)
			deliveries.POST("/:id/retry", deps.Delivery.Retry)
		}

		tokens := v1.Group("/tokens")
		{
			tokens.GET("", deps.Token.List)
			tokens.POST("", deps.Token.Create)
			tokens.DELETE("/:id", deps.Token.Revoke)
		}
	}

	mountWebStatic(r, deps.WebDir, deps.Logger)

	return r
}
