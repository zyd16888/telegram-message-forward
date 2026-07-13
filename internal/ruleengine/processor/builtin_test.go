package processor

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestPreserveLinksAndMediaFallbackText(t *testing.T) {
	m := &domainmessage.NormalizedMessage{
		Text:        "hello",
		OriginalURL: "https://t.me/c/1/2",
		Links:       []domainmessage.Link{{URL: "https://example.com/a"}},
		Media:       []domainmessage.Media{{Type: "image", FileName: "a.jpg", Size: 2048, Caption: "cap"}},
	}
	p, _ := Get("preserve_links")
	if err := p.Process(context.Background(), m, map[string]any{"include_original_url": true}); err != nil {
		t.Fatal(err)
	}
	p, _ = Get("media_fallback_text")
	if err := p.Process(context.Background(), m, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"https://example.com/a", "https://t.me/c/1/2", "[图片消息]", "a.jpg", "2.0 KB"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("处理后文本缺少 %s: %q", want, m.Text)
		}
	}
}

func TestMaskSensitiveAndDedupe(t *testing.T) {
	m := &domainmessage.NormalizedMessage{
		Text:  "phone 13812345678\nphone 13812345678\ntoken=abcdef",
		Links: []domainmessage.Link{{URL: "https://a.test"}, {URL: "https://a.test"}},
	}
	p, _ := Get("mask_sensitive")
	if err := p.Process(context.Background(), m, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	p, _ = Get("dedupe")
	if err := p.Process(context.Background(), m, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.Text, "13812345678") || strings.Contains(m.Text, "abcdef") {
		t.Fatalf("敏感信息应被遮罩: %q", m.Text)
	}
	if strings.Count(m.Text, "phone") != 1 {
		t.Fatalf("重复行应去除: %q", m.Text)
	}
	if len(m.Links) != 1 {
		t.Fatalf("重复链接应去除: %+v", m.Links)
	}
}

func TestQuietHoursAndBatchDigest(t *testing.T) {
	sent := time.Date(2026, 7, 3, 23, 0, 0, 0, time.FixedZone("CST", 8*3600))
	m := &domainmessage.NormalizedMessage{Text: "hello", SenderName: "Tech", SentAt: &sent}
	p, _ := Get("quiet_hours")
	if err := p.Process(context.Background(), m, map[string]any{"start": "22:00", "end": "08:00", "timezone": "Asia/Shanghai"}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.Text, "[静默时间]") {
		t.Fatalf("静默时间应追加标记: %q", m.Text)
	}
	p, _ = Get("batch_digest")
	if err := p.Process(context.Background(), m, map[string]any{"title": "日报"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, "【日报】") || !strings.Contains(m.Text, "来源：Tech") {
		t.Fatalf("摘要格式错误: %q", m.Text)
	}
}

func TestHonestProcessorDescriptors(t *testing.T) {
	want := map[string]struct {
		labelContains string
		descContains  []string
	}{
		"quiet_hours": {
			labelContains: "静默时间标记",
			descContains:  []string{"不抑制", "不延迟"},
		},
		"batch_digest": {
			labelContains: "摘要样式格式化",
			descContains:  []string{"单条", "跨消息"},
		},
		"dedupe": {
			labelContains: "本条去重",
			descContains:  []string{"当前消息", "跨消息"},
		},
	}
	byType := map[string]Descriptor{}
	for _, d := range Descriptors() {
		byType[d.Type] = d
	}
	for typ, spec := range want {
		d, ok := byType[typ]
		if !ok {
			t.Fatalf("缺少处理器 descriptor: %s", typ)
		}
		if d.Label != spec.labelContains {
			t.Fatalf("%s label = %q, want %q", typ, d.Label, spec.labelContains)
		}
		for _, part := range spec.descContains {
			if !strings.Contains(d.Description, part) {
				t.Fatalf("%s description 应包含 %q, got %q", typ, part, d.Description)
			}
		}
	}
}
