package telegram

import (
	"log/slog"
	"time"

	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"golang.org/x/time/rate"

	domainaccount "telegram-message-forward/internal/domain/account"
)

// ClientConfig 是构建 Telegram 客户端所需的配置。
type ClientConfig struct {
	AppID   int
	AppHash string
	Proxy   domainaccount.ProxyConfig

	// SessionStore 为加密 session 存储；必填。
	SessionStore session.Storage

	// UpdateHandler 处理实时 updates；监听时必填，登录时可为 nil。
	UpdateHandler telegram.UpdateHandler

	// RateLimit / RateBurst 控制对 Telegram 的请求速率，零值使用保守默认。
	RateLimit rate.Limit
	RateBurst int

	// Log 用于记录 FLOOD_WAIT 等待事件；为空则使用 slog.Default()。
	Log *slog.Logger
}

// NewClient 按配置构建 gotd 客户端（含 FLOOD_WAIT 处理与限流 middleware）。
func NewClient(cfg ClientConfig) (*telegram.Client, error) {
	resolver, err := resolverFor(cfg.Proxy)
	if err != nil {
		return nil, err
	}

	limit := cfg.RateLimit
	if limit == 0 {
		limit = rate.Every(100 * time.Millisecond) // ~10 req/s，保守默认
	}
	burst := cfg.RateBurst
	if burst <= 0 {
		burst = 5
	}

	opts := telegram.Options{
		SessionStorage: cfg.SessionStore,
		Middlewares: []telegram.Middleware{
			newFloodWaitMiddleware(cfg.Log),
			ratelimit.New(limit, burst),
		},
		Device: telegram.DeviceConfig{
			DeviceModel:   "telegram-message-forward",
			SystemVersion: "1.0",
			AppVersion:    "v1",
		},
	}
	if resolver != nil {
		opts.Resolver = resolver
	}
	if cfg.UpdateHandler != nil {
		opts.UpdateHandler = cfg.UpdateHandler
	}

	return telegram.NewClient(cfg.AppID, cfg.AppHash, opts), nil
}
