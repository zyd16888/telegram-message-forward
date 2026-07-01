package processor

import (
	"context"
	"strings"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
)

func TestAppendSource(t *testing.T) {
	p, err := Get("append_source")
	if err != nil {
		t.Fatal(err)
	}
	m := &domainmessage.NormalizedMessage{Text: "hello"}
	if err := p.Process(context.Background(), m, map[string]any{"label": "测试频道"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.Text, "hello") || !strings.Contains(m.Text, "测试频道") {
		t.Fatalf("追加来源失败: %q", m.Text)
	}
}

func TestTruncateText(t *testing.T) {
	p, _ := Get("truncate_text")
	m := &domainmessage.NormalizedMessage{Text: "一二三四五六七八九十"}
	if err := p.Process(context.Background(), m, map[string]any{"max_length": float64(5)}); err != nil {
		t.Fatal(err)
	}
	runes := []rune(m.Text)
	if string(runes[:5]) != "一二三四五" {
		t.Fatalf("截断结果错误: %q", m.Text)
	}
	if !strings.HasSuffix(m.Text, "…") {
		t.Fatalf("应追加省略号: %q", m.Text)
	}

	// 不超长不改动
	short := &domainmessage.NormalizedMessage{Text: "abc"}
	_ = p.Process(context.Background(), short, map[string]any{"max_length": float64(10)})
	if short.Text != "abc" {
		t.Fatalf("短文本不应改动: %q", short.Text)
	}
}
