// Package telegramconfig 定义 Telegram 平台应用与代理配置。
package telegramconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
)

const (
	ProxyTypeSOCKS5 = "socks5"
	ProxyTypeHTTP   = "http"
	ProxyTypeHTTPS  = "https"
)

// ErrUnsupportedProxyType 表示代理类型不在系统支持范围内。
var ErrUnsupportedProxyType = errors.New("不支持的代理类型")

// NormalizeProxyType 规范化并校验代理类型。
func NormalizeProxyType(value string) (string, error) {
	typ := strings.ToLower(strings.TrimSpace(value))
	switch typ {
	case ProxyTypeSOCKS5, ProxyTypeHTTP, ProxyTypeHTTPS:
		return typ, nil
	default:
		return "", fmt.Errorf("%w：%q（支持 socks5 / http / https）", ErrUnsupportedProxyType, strings.TrimSpace(value))
	}
}

// TelegramApp 是 Telegram 官方平台应用配置，多个账号可复用同一组 app_id/app_hash。
type TelegramApp struct {
	ID        int64
	Name      string
	AppID     int
	AppHash   string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Proxy 是可复用代理配置。
type Proxy struct {
	ID        int64
	Name      string
	Type      string
	Addr      string
	Username  string
	Password  string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ToAccountProxy 转换为账号运行时使用的代理配置。
func (p *Proxy) ToAccountProxy() domainaccount.ProxyConfig {
	if p == nil {
		return domainaccount.ProxyConfig{}
	}
	return domainaccount.ProxyConfig{
		Type:     p.Type,
		Addr:     p.Addr,
		Username: p.Username,
		Password: p.Password,
	}
}

// TelegramAppRepository 是 Telegram App 仓储接口。
type TelegramAppRepository interface {
	Create(ctx context.Context, app *TelegramApp) error
	Update(ctx context.Context, app *TelegramApp) error
	GetByID(ctx context.Context, id int64) (*TelegramApp, error)
	List(ctx context.Context) ([]*TelegramApp, error)
	Delete(ctx context.Context, id int64) error
}

// ProxyRepository 是代理配置仓储接口。
type ProxyRepository interface {
	Create(ctx context.Context, p *Proxy) error
	Update(ctx context.Context, p *Proxy) error
	GetByID(ctx context.Context, id int64) (*Proxy, error)
	List(ctx context.Context) ([]*Proxy, error)
	Delete(ctx context.Context, id int64) error
}
