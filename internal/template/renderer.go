// Package template 渲染标准消息为目标格式（text/markdown/html）。
package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	domainmessage "telegram-message-forward/internal/domain/message"
	domaintemplate "telegram-message-forward/internal/domain/template"
)

// Renderer 渲染消息为指定模板格式。
type Renderer struct{}

// NewRenderer 创建渲染器。
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Rendered 是渲染结果。
type Rendered struct {
	Format domaintemplate.Format
	Text   string
}

// Render 用模板渲染消息。tpl 为 nil 时回退为消息原文（text 格式）。
func (r *Renderer) Render(tpl *domaintemplate.Template, msg *domainmessage.NormalizedMessage) (*Rendered, error) {
	if tpl == nil {
		return &Rendered{Format: domaintemplate.FormatText, Text: defaultText(msg)}, nil
	}

	t, err := template.New(fmt.Sprintf("tpl-%d", tpl.ID)).Parse(tpl.Content)
	if err != nil {
		return nil, fmt.Errorf("解析模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, msg); err != nil {
		return nil, fmt.Errorf("渲染模板失败: %w", err)
	}
	return &Rendered{Format: tpl.Format, Text: buf.String()}, nil
}

func defaultText(msg *domainmessage.NormalizedMessage) string {
	if msg == nil {
		return ""
	}
	out := msg.Text
	for _, link := range msg.Links {
		url := strings.TrimSpace(link.URL)
		if url == "" || strings.Contains(out, url) {
			continue
		}
		title := strings.TrimSpace(link.Title)
		line := "链接：" + url
		if title != "" && title != url && !strings.Contains(out, title) {
			line = "链接：" + title + " " + url
		}
		if out == "" {
			out = line
		} else {
			out += "\n" + line
		}
	}
	return out
}
