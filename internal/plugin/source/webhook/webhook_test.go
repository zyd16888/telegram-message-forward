package webhook

import (
	"context"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

func TestReceiveJSON(t *testing.T) {
	p := New()
	src := &domainsource.Source{ID: 9, Type: "webhook", PeerID: 1, Name: "hook", Config: map[string]any{"token": "TOKEN"}}
	msg, err := p.Receive(context.Background(), src, pluginsource.WebhookRequest{Body: []byte(`{
		"message_id": "42",
		"text": "hello",
		"sender": "ci",
		"timestamp": "2026-07-03T12:00:00+08:00",
		"links": [{"url":"https://example.com","title":"Example"}],
		"media": [{"type":"image","remote_url":"https://example.com/a.jpg"}]
	}`)})
	if err != nil {
		t.Fatal(err)
	}
	if msg.ExternalMessageID != 42 || msg.MessageType != "image" || msg.SenderName != "ci" || len(msg.Links) != 1 {
		t.Fatalf("标准化结果不正确: %+v", msg)
	}
	if len(p.RunnerStatuses()) != 1 {
		t.Fatalf("接收后应记录运行状态: %+v", p.RunnerStatuses())
	}
}

func TestReceivePlainText(t *testing.T) {
	p := New()
	src := &domainsource.Source{ID: 9, Type: "webhook", PeerID: 1, Name: "hook", Config: map[string]any{"token": "TOKEN"}}
	msg, err := p.Receive(context.Background(), src, pluginsource.WebhookRequest{Body: []byte("plain text")})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Text != "plain text" || msg.ExternalMessageID == 0 {
		t.Fatalf("纯文本应被标准化: %+v", msg)
	}
}

func TestValidateConfig(t *testing.T) {
	p := New()
	if err := p.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少 token 应报错")
	}
	if err := p.ValidateConfig(map[string]any{"token": "TOKEN"}); err != nil {
		t.Fatalf("有 token 不应报错: %v", err)
	}
}

func TestNormalizeKeepsMedia(t *testing.T) {
	src := &domainsource.Source{ID: 1, Type: "webhook"}
	msg := normalize(src, incomingPayload{Text: "x", Media: []domainmessage.Media{{Type: "video", RemoteURL: "https://example.com/v.mp4"}}}, []byte(`{}`))
	if msg.MessageType != "video" || len(msg.Media) != 1 {
		t.Fatalf("媒体未保留: %+v", msg)
	}
}
