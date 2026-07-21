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

	Auth           *handler.AuthHandler
	Account        *handler.AccountHandler
	AccountLogin   *handler.AccountLoginHandler
	Sink           *handler.SinkHandler
	Source         *handler.SourceHandler
	Template       *handler.TemplateHandler
	Flow           *handler.FlowHandler
	Filter         *handler.FilterHandler
	Delivery       *handler.DeliveryHandler
	Message        *handler.MessageHandler
	AIDigest       *handler.AIDigestHandler
	Token          *handler.TokenHandler
	TelegramConfig *handler.TelegramConfigHandler
	Settings       *handler.SettingsHandler
	Backup         *handler.BackupHandler
	Dashboard      *handler.DashboardHandler
	// Media 为 nil 时不挂载 /media 端点。
	Media *handler.MediaHandler
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

	// Webhook Source 使用每个 source 自己的 token 鉴权，不能依赖管理后台会话 token。
	r.POST("/api/v1/sources/:id/webhook", deps.Source.Webhook)

	// 媒体文件访问使用 URL 内嵌的 HMAC 签名鉴权（下游渠道服务器直接拉取）。
	if deps.Media != nil {
		r.GET("/media/*key", deps.Media.Serve)
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
			sinks.GET("/meta", deps.Sink.Meta)
			sinks.POST("/test", deps.Sink.Test)
			sinks.POST("", deps.Sink.Create)
			sinks.GET("/:id", deps.Sink.Get)
			sinks.PUT("/:id", deps.Sink.Update)
			sinks.DELETE("/:id", deps.Sink.Delete)
			sinks.POST("/:id/test", deps.Sink.TestExisting)
		}

		sources := v1.Group("/sources")
		{
			sources.GET("", deps.Source.List)
			sources.POST("", deps.Source.Create)
			sources.POST("/sync", deps.Source.Sync)
			sources.GET("/sync/stream", deps.Source.SyncStream)
			sources.GET("/:id", deps.Source.Get)
			sources.PUT("/:id", deps.Source.Update)
			sources.DELETE("/:id", deps.Source.Delete)
			sources.POST("/:id/start", deps.Source.Start)
			sources.POST("/:id/stop", deps.Source.Stop)
			sources.POST("/:id/history/preview", deps.Source.PreviewHistory)
			sources.POST("/:id/history/backfill", deps.Source.BackfillHistory)
		}

		templates := v1.Group("/templates")
		{
			templates.GET("", deps.Template.List)
			templates.POST("/preview", deps.Template.Preview)
			templates.POST("", deps.Template.Create)
			templates.GET("/:id", deps.Template.Get)
			templates.PUT("/:id", deps.Template.Update)
			templates.DELETE("/:id", deps.Template.Delete)
		}

		if deps.Flow != nil {
			flows := v1.Group("/flows")
			{
				flows.GET("", deps.Flow.List)
				flows.GET("/meta", deps.Flow.Meta)
				flows.POST("/preview", deps.Flow.Preview)
				flows.POST("", deps.Flow.Create)
				flows.GET("/:id", deps.Flow.Get)
				flows.PUT("/:id", deps.Flow.Update)
				flows.DELETE("/:id", deps.Flow.Delete)
			}
		}

		if deps.Filter != nil {
			filters := v1.Group("/filters")
			{
				filters.GET("", deps.Filter.List)
				filters.POST("", deps.Filter.Create)
				filters.GET("/:id", deps.Filter.Get)
				filters.PUT("/:id", deps.Filter.Update)
				filters.DELETE("/:id", deps.Filter.Delete)
			}
		}

		deliveries := v1.Group("/deliveries")
		{
			deliveries.GET("", deps.Delivery.List)
			deliveries.POST("/retry-dead", deps.Delivery.RetryDeadBatch)
			deliveries.GET("/:id", deps.Delivery.Get)
			deliveries.POST("/:id/retry", deps.Delivery.Retry)
		}

		if deps.Message != nil {
			messages := v1.Group("/messages")
			{
				messages.GET("", deps.Message.List)
				messages.GET("/:id", deps.Message.Get)
			}
		}

		if deps.AIDigest != nil {
			ai := v1.Group("/ai")
			{
				ai.GET("/provider", deps.AIDigest.GetProvider)
				ai.PUT("/provider", deps.AIDigest.UpdateProvider)
				ai.POST("/provider/test", deps.AIDigest.TestProvider)
				ai.GET("/providers", deps.AIDigest.ListProviders)
				ai.POST("/providers", deps.AIDigest.CreateProvider)
				ai.POST("/providers/test", deps.AIDigest.TestProviderDraft)
				ai.GET("/providers/:id", deps.AIDigest.GetProviderByID)
				ai.PUT("/providers/:id", deps.AIDigest.UpdateProviderByID)
				ai.DELETE("/providers/:id", deps.AIDigest.DeleteProvider)
				ai.POST("/providers/:id/test", deps.AIDigest.TestProviderByID)
				ai.GET("/presets", deps.AIDigest.ListPresets)

				ai.GET("/output-templates", deps.AIDigest.ListOutputTemplates)
				ai.POST("/output-templates", deps.AIDigest.CreateOutputTemplate)
				ai.GET("/output-templates/:id", deps.AIDigest.GetOutputTemplate)
				ai.PUT("/output-templates/:id", deps.AIDigest.UpdateOutputTemplate)
				ai.DELETE("/output-templates/:id", deps.AIDigest.DeleteOutputTemplate)

				ai.GET("/digests", deps.AIDigest.ListProfiles)
				ai.POST("/digests", deps.AIDigest.CreateProfile)
				ai.POST("/digests/preview", deps.AIDigest.PreviewDraft)
				ai.GET("/digests/:id", deps.AIDigest.GetProfile)
				ai.PUT("/digests/:id", deps.AIDigest.UpdateProfile)
				ai.DELETE("/digests/:id", deps.AIDigest.DeleteProfile)
				ai.POST("/digests/:id/preview", deps.AIDigest.PreviewProfile)
				ai.POST("/digests/:id/run", deps.AIDigest.RunProfile)
				ai.GET("/digests/:id/runs", deps.AIDigest.ListRuns)
				ai.GET("/runs/:id", deps.AIDigest.GetRun)
				ai.POST("/runs/:id/clone-profile", deps.AIDigest.CloneProfileFromRun)
				ai.POST("/runs/:id/cancel", deps.AIDigest.CancelRun)
				ai.POST("/runs/:id/deliver", deps.AIDigest.DeliverRun)
				ai.POST("/runs/cleanup", deps.AIDigest.CleanupRuns)
			}
		}

		tokens := v1.Group("/tokens")
		{
			tokens.GET("", deps.Token.List)
			tokens.POST("", deps.Token.Create)
			tokens.DELETE("/:id", deps.Token.Revoke)
		}

		tgApps := v1.Group("/telegram-apps")
		{
			tgApps.GET("", deps.TelegramConfig.ListApps)
			tgApps.POST("", deps.TelegramConfig.CreateApp)
			tgApps.PUT("/:id", deps.TelegramConfig.UpdateApp)
			tgApps.DELETE("/:id", deps.TelegramConfig.DeleteApp)
		}

		if deps.Settings != nil {
			settings := v1.Group("/settings")
			{
				settings.GET("/media", deps.Settings.GetMedia)
				settings.PUT("/media", deps.Settings.UpdateMedia)
				settings.POST("/media/test-s3", deps.Settings.TestMediaS3)
				settings.POST("/media/cleanup", deps.Settings.CleanupMedia)
				settings.GET("/data-retention", deps.Settings.GetDataRetention)
				settings.PUT("/data-retention", deps.Settings.UpdateDataRetention)
				settings.POST("/data-retention/cleanup", deps.Settings.CleanupDataRetention)
			}
		}

		if deps.Dashboard != nil {
			v1.GET("/dashboard/summary", deps.Dashboard.Summary)
		}

		if deps.Backup != nil {
			backups := v1.Group("/settings/backups")
			{
				backups.POST("/export", deps.Backup.Export)
				backups.POST("/inspect", deps.Backup.Inspect)
				backups.POST("/restore", deps.Backup.Restore)
			}
		}

		proxies := v1.Group("/proxies")
		{
			proxies.GET("", deps.TelegramConfig.ListProxies)
			proxies.POST("", deps.TelegramConfig.CreateProxy)
			proxies.PUT("/:id", deps.TelegramConfig.UpdateProxy)
			proxies.DELETE("/:id", deps.TelegramConfig.DeleteProxy)
		}
	}

	mountWebStatic(r, deps.WebDir, deps.Logger)

	return r
}
