package gotify

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
	var gotToken atomic.Value
	var gotPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken.Store(r.Header.Get("X-Gotify-Key"))
		gotPath.Store(r.URL.String())
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, `{"id":12,"title":"TMF","message":"hello"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "gotify", Config: map[string]any{"server_url": srv.URL, "app_token": "APP", "title": "TMF", "priority": 8}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "hello"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	if gotToken.Load() != "APP" || gotPath.Load() != "/message" {
		t.Fatalf("应通过 header 鉴权且不把 token 放入 URL token=%v path=%v", gotToken.Load(), gotPath.Load())
	}
	if got["title"] != "TMF" || got["message"] != "hello" || got["priority"] != float64(8) {
		t.Fatalf("请求体不正确: %+v", got)
	}
}

func TestSendMarkdownSetsDisplayExtra(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, `{"id":1}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "gotify", Config: map[string]any{"server_url": srv.URL, "app_token": "APP"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "markdown", Text: "**hello**"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	extras, _ := got["extras"].(map[string]any)
	display, _ := extras["client::display"].(map[string]any)
	if !res.Success || display["contentType"] != "text/markdown" {
		t.Fatalf("Markdown 应声明 display extra res=%+v body=%+v", res, got)
	}
}

func TestSendRemoteImageUsesBigImageExtra(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, `{"id":1}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "gotify", Config: map[string]any{"server_url": srv.URL, "app_token": "APP"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "hello",
		Media: []domainmessage.Media{{Type: "image", RemoteURL: "https://example.com/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	extras, _ := got["extras"].(map[string]any)
	notification, _ := extras["client::notification"].(map[string]any)
	if !res.Success || notification["bigImageUrl"] != "https://example.com/a.jpg" {
		t.Fatalf("远程图片应进入 bigImageUrl res=%+v body=%+v", res, got)
	}
}

func TestSendLocalMediaFallsBackToText(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, `{"id":1}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "gotify", Config: map[string]any{"server_url": srv.URL, "app_token": "APP"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:         "hello",
		FallbackText: "hello\n[图片消息] local",
		Media:        []domainmessage.Media{{Type: "image", LocalPath: "C:/tmp/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || got["message"] != "hello\n[图片消息] local" {
		t.Fatalf("本地媒体应降级为正文 res=%+v body=%+v", res, got)
	}
}

func TestSendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"bad"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "gotify", Config: map[string]any{"server_url": srv.URL, "app_token": "APP"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || !strings.Contains(res.Error, "400") {
		t.Fatalf("非 2xx 应失败: %+v", res)
	}
}

func TestValidateConfig(t *testing.T) {
	s := New()
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少配置应报错")
	}
	if err := s.ValidateConfig(map[string]any{"server_url": "https://gotify.example.com", "app_token": "APP"}); err != nil {
		t.Fatalf("完整配置不应报错: %v", err)
	}
}

func TestMessageEndpoint(t *testing.T) {
	got, err := messageEndpoint("https://gotify.example.com/base/?token=SECRET#x")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://gotify.example.com/base/message" {
		t.Fatalf("endpoint 不正确: %s", got)
	}
}
