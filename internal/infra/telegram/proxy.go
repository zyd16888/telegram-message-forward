package telegram

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"golang.org/x/net/proxy"

	domainaccount "telegram-message-forward/internal/domain/account"
)

// resolverFor 根据代理配置构建 dcs.Resolver；未配置代理时返回 nil（使用默认直连）。
func resolverFor(cfg domainaccount.ProxyConfig) (dcs.Resolver, error) {
	if cfg.Type == "" || cfg.Addr == "" {
		return nil, nil
	}
	typ := strings.ToLower(strings.TrimSpace(cfg.Type))
	switch typ {
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
		// Telegram MTProto 走 TCP；通过 HTTP CONNECT 隧道出站。
		// type=https 表示与代理的控制连接使用 TLS（较少见），CONNECT 语义相同。
		dial, err := httpConnectDialFunc(cfg)
		if err != nil {
			return nil, err
		}
		return dcs.Plain(dcs.PlainOptions{Dial: dial}), nil
	default:
		return nil, fmt.Errorf("不支持的代理类型: %s（支持 socks5 / http / https）", cfg.Type)
	}
}

// httpConnectDialFunc 返回经 HTTP(S) 代理 CONNECT 的 dial 函数。
func httpConnectDialFunc(cfg domainaccount.ProxyConfig) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	proxyAddr := strings.TrimSpace(cfg.Addr)
	if proxyAddr == "" {
		return nil, fmt.Errorf("http 代理地址不能为空")
	}
	// 允许用户填 host:port 或带 scheme 的 URL。
	if strings.Contains(proxyAddr, "://") {
		u, err := url.Parse(proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("解析代理地址失败: %w", err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("代理地址缺少 host")
		}
		proxyAddr = u.Host
		if u.User != nil && cfg.Username == "" {
			cfg.Username = u.User.Username()
			cfg.Password, _ = u.User.Password()
		}
	}
	useTLS := strings.EqualFold(cfg.Type, "https")
	user := cfg.Username
	pass := cfg.Password

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if network != "tcp" && network != "tcp4" && network != "tcp6" {
			return nil, fmt.Errorf("http 代理仅支持 tcp，收到 %s", network)
		}
		var d net.Dialer
		if deadline, ok := ctx.Deadline(); ok {
			d.Deadline = deadline
		} else {
			d.Timeout = 30 * time.Second
		}
		conn, err := d.DialContext(ctx, "tcp", proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("连接 http 代理 %s 失败: %w", proxyAddr, err)
		}
		if useTLS {
			// 与代理建立 TLS 后再发 CONNECT（HTTPS 代理）。
			tlsConn := tls.Client(conn, &tls.Config{ServerName: proxyHost(proxyAddr), MinVersion: tls.VersionTLS12})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				_ = conn.Close()
				return nil, fmt.Errorf("与 https 代理握手失败: %w", err)
			}
			conn = tlsConn
		}
		if err := sendHTTPConnect(conn, addr, user, pass); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}, nil
}

func proxyHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func sendHTTPConnect(conn net.Conn, target, user, pass string) error {
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: make(http.Header),
		Proto:  "HTTP/1.1",
	}
	req.Header.Set("Proxy-Connection", "Keep-Alive")
	if user != "" || pass != "" {
		token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		req.Header.Set("Proxy-Authorization", "Basic "+token)
	}
	if err := req.Write(conn); err != nil {
		return fmt.Errorf("发送 CONNECT 失败: %w", err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return fmt.Errorf("读取代理 CONNECT 响应失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("代理 CONNECT 被拒绝: %s", resp.Status)
	}
	// 缓冲中不应残留业务数据；若有则无法安全交给上层。
	if br.Buffered() > 0 {
		return fmt.Errorf("代理 CONNECT 后仍有缓冲数据，无法建立隧道")
	}
	return nil
}
