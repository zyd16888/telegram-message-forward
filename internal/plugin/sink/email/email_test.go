package email

import (
	"context"
	"encoding/base64"
	"net/smtp"
	"os"
	"strings"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func TestSendTextMail(t *testing.T) {
	var gotAddr, gotFrom string
	var gotTo []string
	var gotMsg string
	s := New()
	s.send = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		gotAddr, gotFrom, gotTo, gotMsg = addr, from, to, string(msg)
		return nil
	}
	sink := &domainsink.Sink{Type: "email", Config: map[string]any{
		"smtp_addr": "smtp.example.com:25",
		"from":      "bot@example.com",
		"to":        []any{"a@example.com", "b@example.com"},
		"subject":   "测试",
	}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "hello email"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	if gotAddr != "smtp.example.com:25" || gotFrom != "bot@example.com" || len(gotTo) != 2 {
		t.Fatalf("SMTP 参数不正确 addr=%s from=%s to=%v", gotAddr, gotFrom, gotTo)
	}
	if !strings.Contains(gotMsg, "Content-Type: text/plain") || !strings.Contains(gotMsg, "aGVsbG8gZW1haWw=") {
		t.Fatalf("邮件正文不正确: %s", gotMsg)
	}
}

func TestSendAttachmentMail(t *testing.T) {
	file := t.TempDir() + "/image.jpg"
	if err := os.WriteFile(file, []byte("fake-image"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotMsg string
	s := New()
	s.send = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		gotMsg = string(msg)
		return nil
	}
	sink := &domainsink.Sink{Type: "email", Config: map[string]any{
		"smtp_addr": "smtp.example.com:25",
		"from":      "bot@example.com",
		"to":        []any{"a@example.com"},
	}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text: "hello",
		Media: []domainmessage.Media{{
			Type:      "image",
			FileName:  "image.jpg",
			MimeType:  "image/jpeg",
			LocalPath: file,
		}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	for _, want := range []string{"multipart/mixed", `filename="image.jpg"`, "Content-Type: image/jpeg", "ZmFrZS1pbWFnZQ=="} {
		if !strings.Contains(gotMsg, want) {
			t.Fatalf("附件邮件缺少 %s: %s", want, gotMsg)
		}
	}
}

func TestSendMediaWithoutLocalPathUsesFallback(t *testing.T) {
	var gotMsg string
	s := New()
	s.send = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		gotMsg = string(msg)
		return nil
	}
	sink := &domainsink.Sink{Type: "email", Config: map[string]any{
		"smtp_addr": "smtp.example.com:25",
		"from":      "bot@example.com",
		"to":        []any{"a@example.com"},
	}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:         "hello",
		FallbackText: "hello\n[图片消息] remote",
		Media:        []domainmessage.Media{{Type: "image", RemoteURL: "https://example.com/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	want := base64.StdEncoding.EncodeToString([]byte("hello\nhello\n[图片消息] remote"))
	if !strings.Contains(gotMsg, want) {
		t.Fatalf("应包含降级文本: %s", gotMsg)
	}
}

func TestValidateConfig(t *testing.T) {
	s := New()
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少配置应失败")
	}
	if err := s.ValidateConfig(map[string]any{"smtp_addr": "smtp:25", "from": "bot@example.com", "to": []any{"a@example.com"}}); err != nil {
		t.Fatalf("完整配置不应失败: %v", err)
	}
	if err := s.ValidateConfig(map[string]any{"smtp_addr": "smtp:25", "security": "insecure", "from": "bot@example.com", "to": []any{"a@example.com"}}); err == nil {
		t.Fatal("未知连接安全模式应失败")
	}
}

func TestSMTPHost(t *testing.T) {
	for input, want := range map[string]string{
		"smtp.example.com:587": "smtp.example.com",
		"[2001:db8::1]:465":    "2001:db8::1",
		"smtp.example.com":     "smtp.example.com",
	} {
		if got := smtpHost(input); got != want {
			t.Fatalf("smtpHost(%q) = %q, want %q", input, got, want)
		}
	}
}
