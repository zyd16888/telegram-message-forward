package template

import (
	"strings"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domaintemplate "telegram-message-forward/internal/domain/template"
)

func TestRenderDefaultTextPreservesHiddenLinks(t *testing.T) {
	rendered, err := NewRenderer().Render(nil, &domainmessage.NormalizedMessage{
		Text: "游资大V复盘文章汇总20260705",
		Links: []domainmessage.Link{
			{URL: "https://pan.baidu.com/s/1Hg80L07OLfcW99xhco5UQQ?pwd=uscp", Title: "游资大V复盘文章汇总20260705"},
			{URL: "https://example.com/already-visible", Title: "https://example.com/already-visible"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Format != domaintemplate.FormatText {
		t.Fatalf("Format = %s, want text", rendered.Format)
	}
	if !strings.Contains(rendered.Text, "https://pan.baidu.com/s/1Hg80L07OLfcW99xhco5UQQ?pwd=uscp") {
		t.Fatalf("default text should include hidden URL: %q", rendered.Text)
	}
}

func TestRenderDefaultTextDoesNotDuplicateVisibleURL(t *testing.T) {
	url := "https://example.com/already-visible"
	rendered, err := NewRenderer().Render(nil, &domainmessage.NormalizedMessage{
		Text:  "正文 " + url,
		Links: []domainmessage.Link{{URL: url, Title: url}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(rendered.Text, url) != 1 {
		t.Fatalf("visible URL should not be duplicated: %q", rendered.Text)
	}
}

func TestRenderCustomTemplateKeepsTemplateOutput(t *testing.T) {
	rendered, err := NewRenderer().Render(&domaintemplate.Template{
		ID:      1,
		Format:  domaintemplate.FormatText,
		Content: "{{.Text}}",
	}, &domainmessage.NormalizedMessage{
		Text:  "只要正文",
		Links: []domainmessage.Link{{URL: "https://example.com/hidden", Title: "只要正文"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Text != "只要正文" {
		t.Fatalf("custom template output should not auto append links: %q", rendered.Text)
	}
}
