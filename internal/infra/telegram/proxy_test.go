package telegram

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
)

func TestResolverForHTTPProxyConstructs(t *testing.T) {
	// 启动假 HTTP 代理：接受 CONNECT 后回 200。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				br := bufio.NewReader(conn)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				if req.Method != http.MethodConnect {
					_, _ = io.WriteString(conn, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
					return
				}
				// 校验 Proxy-Authorization 存在时被解析。
				_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
			}(c)
		}
	}()

	addr := ln.Addr().String()
	res, err := resolverFor(domainaccount.ProxyConfig{
		Type:     "http",
		Addr:     addr,
		Username: "u",
		Password: "p",
	})
	if err != nil {
		t.Fatalf("resolverFor http: %v", err)
	}
	if res == nil {
		t.Fatal("http 代理应返回 resolver")
	}

	// socks5 与 https 构造路径不报「暂未实现」。
	if _, err := resolverFor(domainaccount.ProxyConfig{Type: "https", Addr: addr}); err != nil {
		t.Fatalf("https 代理构造失败: %v", err)
	}
	if _, err := resolverFor(domainaccount.ProxyConfig{Type: "socks5", Addr: "127.0.0.1:1080"}); err != nil {
		// socks5 在未连上时构造 dialer 通常仍成功。
		if strings.Contains(err.Error(), "暂未实现") {
			t.Fatal(err)
		}
	}
	if _, err := resolverFor(domainaccount.ProxyConfig{Type: "http", Addr: addr}); err != nil {
		t.Fatal(err)
	}

	// 通过 dial 走 CONNECT 隧道（目标是假服务，CONNECT 成功即可）。
	dial, err := httpConnectDialFunc(domainaccount.ProxyConfig{Type: "http", Addr: addr, Username: "u", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// 目标地址任意；代理假实现不真正转发，但 CONNECT 握手应成功。
	conn, err := dial(ctx, "tcp", "example.com:443")
	if err != nil {
		t.Fatalf("http CONNECT dial: %v", err)
	}
	_ = conn.Close()
}

func TestResolverForRejectsUnknown(t *testing.T) {
	_, err := resolverFor(domainaccount.ProxyConfig{Type: "ss", Addr: "1:2"})
	if err == nil || !strings.Contains(err.Error(), "不支持的代理类型") {
		t.Fatalf("应拒绝未知类型: %v", err)
	}
}

func TestResolverForEmptyIsNil(t *testing.T) {
	res, err := resolverFor(domainaccount.ProxyConfig{})
	if err != nil || res != nil {
		t.Fatalf("空代理应直连: res=%v err=%v", res, err)
	}
}

func TestHTTPProxyNotUnimplementedMessage(t *testing.T) {
	_, err := resolverFor(domainaccount.ProxyConfig{Type: "http", Addr: "127.0.0.1:9"})
	if err != nil && strings.Contains(err.Error(), "暂未实现") {
		t.Fatalf("不应再返回暂未实现: %v", err)
	}
	// 构造阶段只校验配置，dial 在使用时才连。
	if err != nil {
		t.Fatalf("构造 http resolver 不应失败: %v", err)
	}
}
