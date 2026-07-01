package telegram

import (
	"context"
	"fmt"
	"net"

	"github.com/gotd/td/telegram/dcs"
	"golang.org/x/net/proxy"

	domainaccount "telegram-message-forward/internal/domain/account"
)

// resolverFor 根据代理配置构建 dcs.Resolver；未配置代理时返回 nil（使用默认直连）。
func resolverFor(cfg domainaccount.ProxyConfig) (dcs.Resolver, error) {
	if cfg.Type == "" || cfg.Addr == "" {
		return nil, nil
	}
	switch cfg.Type {
	case "socks5":
		var auth *proxy.Auth
		if cfg.Username != "" || cfg.Password != "" {
			auth = &proxy.Auth{User: cfg.Username, Password: cfg.Password}
		}
		d, err := proxy.SOCKS5("tcp", cfg.Addr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("构建 socks5 代理失败: %w", err)
		}
		cd, ok := d.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("socks5 dialer 不支持 context")
		}
		dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
			return cd.DialContext(ctx, network, addr)
		}
		return dcs.Plain(dcs.PlainOptions{Dial: dial}), nil
	case "http", "https":
		return nil, fmt.Errorf("http 代理暂未实现，请使用 socks5 或直连")
	default:
		return nil, fmt.Errorf("不支持的代理类型: %s", cfg.Type)
	}
}
