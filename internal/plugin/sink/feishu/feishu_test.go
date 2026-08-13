package feishu

import (
	"context"
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
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"code":0,"msg":"ok"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "feishu_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "hello feishu"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msg_type":"text"`) || !strings.Contains(body, "hello feishu") {
		t.Fatalf("请求体不正确: %s", body)
	}
}

func TestSendMarkdownUsesInteractiveCardAndSignature(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"code":0,"msg":"ok"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "feishu_bot", Config: map[string]any{"webhook_url": srv.URL}, Secret: []byte("secret")}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "markdown", Text: "**hello**"}, pluginsink.Options{})
	if err != nil || !res.Success {
		t.Fatalf("Markdown 应成功: res=%+v err=%v", res, err)
	}
	body, _ := gotBody.Load().(string)
	for _, want := range []string{`"msg_type":"interactive"`, `"tag":"markdown"`, `"timestamp":`, `"sign":`} {
		if !strings.Contains(body, want) {
			t.Fatalf("飞书 Markdown/签名请求缺少 %s: %s", want, body)
		}
	}
}

func TestSendMediaFallsBackToText(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"code":0,"msg":"ok"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "feishu_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:         "hello",
		FallbackText: "hello\n[图片消息] preview.jpg",
		Media:        []domainmessage.Media{{Type: "image", FileName: "preview.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msg_type":"text"`) || !strings.Contains(body, "[图片消息] preview.jpg") {
		t.Fatalf("媒体应降级为文本: %s", body)
	}
}

func TestSendErrCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":19001,"msg":"bad webhook"}`)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "feishu_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || !strings.Contains(res.Error, "19001") {
		t.Fatalf("错误响应应失败: %+v", res)
	}
}

func TestValidateConfig(t *testing.T) {
	s := New()
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少 webhook_url 应报错")
	}
	if err := s.ValidateConfig(map[string]any{"webhook_url": "https://example.com"}); err != nil {
		t.Fatalf("有 webhook_url 不应报错: %v", err)
	}
}
