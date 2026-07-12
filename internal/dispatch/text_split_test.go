package dispatch

import (
	"strings"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func TestSplitTextRespectsUTF8ByteLimitWithoutLosingContent(t *testing.T) {
	text := strings.Repeat("摘要内容。", 500)
	chunks := splitText(text, textLimit{maxBytes: 2048})
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want multiple chunks", len(chunks))
	}
	if strings.Join(chunks, "") != text {
		t.Fatal("split text should preserve all content")
	}
	for i, chunk := range chunks {
		part := "[1/9] " + chunk
		if len(part) > 2048 {
			t.Fatalf("chunk %d uses %d bytes, want <= 2048", i, len(part))
		}
	}
}

func TestSplitTextPrefersParagraphBoundary(t *testing.T) {
	text := strings.Repeat("第一段内容", 8) + "\n\n" + strings.Repeat("第二段内容", 8)
	chunks := splitText(text, textLimit{maxRunes: 60})
	if len(chunks) < 2 || !strings.HasSuffix(chunks[0], "\n\n") {
		t.Fatalf("chunks should split at paragraph boundary: %#v", chunks)
	}
	if strings.Join(chunks, "") != text {
		t.Fatal("paragraph split should preserve all content")
	}
}

func TestSplitPayloadOnlyKeepsMediaOnFirstPart(t *testing.T) {
	payload := pluginsink.Payload{
		Format: "text",
		Text:   strings.Repeat("long text ", 20),
		Media:  []domainmessage.Media{{Type: "image", FileName: "cover.jpg"}},
	}
	parts := splitPayload(payload, domainsink.Capabilities{MaxTextLength: 50})
	if len(parts) < 2 {
		t.Fatalf("parts = %d, want multiple parts", len(parts))
	}
	if len(parts[0].Media) != 1 {
		t.Fatal("first part should keep media")
	}
	for i := 1; i < len(parts); i++ {
		if len(parts[i].Media) != 0 {
			t.Fatalf("part %d should not repeat media", i+1)
		}
	}
}
