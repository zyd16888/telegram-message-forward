package bark

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func TestSendTextSuccess(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		io.WriteString(w, `{"code":200,"message":"success"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "bark", Config: map[string]any{"server_url": srv.URL, "device_key": "KEY", "title": "TMF"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "hello bark"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	if got["device_key"] != "KEY" || got["body"] != "hello bark" || got["title"] != "TMF" {
		t.Fatalf("请求体不正确: %+v", got)
	}
}

func TestSendRemoteImageUsesImageField(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		io.WriteString(w, `{"code":200,"message":"success"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "bark", Config: map[string]any{"server_url": srv.URL, "device_key": "KEY"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "hello",
		Media: []domainmessage.Media{{Type: "image", RemoteURL: "https://example.com/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || got["image"] != "https://example.com/a.jpg" {
		t.Fatalf("远程图片应进入 image 字段 res=%+v body=%+v", res, got)
	}
}

func TestSendLocalMediaFallsBackToText(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		io.WriteString(w, `{"code":200,"message":"success"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "bark", Config: map[string]any{"server_url": srv.URL, "device_key": "KEY"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:         "hello",
		FallbackText: "hello\n[图片消息] local",
		Media:        []domainmessage.Media{{Type: "image", LocalPath: "C:/tmp/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || got["body"] != "hello\n[图片消息] local" {
		t.Fatalf("本地媒体应降级为正文 res=%+v body=%+v", res, got)
	}
}

func TestSendError(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		io.WriteString(w, `{"code":400,"message":"bad"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "bark", Config: map[string]any{"server_url": srv.URL, "device_key": "KEY"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !called.Load() || res.Success || !strings.Contains(res.Error, "400") {
		t.Fatalf("错误响应应失败: %+v", res)
	}
}

func TestValidateConfig(t *testing.T) {
	s := New()
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少 device_key 应报错")
	}
	if err := s.ValidateConfig(map[string]any{"device_key": "KEY"}); err != nil {
		t.Fatalf("有 device_key 不应报错: %v", err)
	}
}
