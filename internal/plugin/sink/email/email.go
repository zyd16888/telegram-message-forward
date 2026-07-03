// Package email 实现 SMTP 邮件 Sink。
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func init() {
	pluginsink.Register("email", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

type smtpSender func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

// Sink 是 SMTP 邮件 Sink。
type Sink struct {
	send smtpSender
}

// New 创建邮件 Sink。
func New() *Sink {
	return &Sink{send: smtp.SendMail}
}

var _ pluginsink.Plugin = (*Sink)(nil)

func (s *Sink) Name() string { return "email" }

func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "邮件 SMTP",
		Description: "通过 SMTP 发送文本、HTML 或附件邮件。",
		ConfigFields: []formschema.FieldSpec{
			{Key: "smtp_addr", Label: "SMTP 地址", Type: formschema.FieldText, Required: true, Placeholder: "smtp.example.com:587"},
			{Key: "username", Label: "用户名", Type: formschema.FieldText, Placeholder: "user@example.com"},
			{Key: "from", Label: "发件人", Type: formschema.FieldText, Required: true, Placeholder: "bot@example.com"},
			{Key: "to", Label: "收件人", Type: formschema.FieldStringList, Required: true, Placeholder: "逐行输入收件人"},
			{Key: "subject", Label: "默认主题", Type: formschema.FieldText, Default: "Telegram Message Forward"},
		},
		SecretField: &formschema.FieldSpec{
			Key:         "secret",
			Label:       "SMTP 密码",
			Type:        formschema.FieldPassword,
			Secret:      true,
			Placeholder: "可选；无用户名时留空",
		},
		Capabilities: s.Capabilities(),
	}
}

func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		SupportsHTML:     true,
		SupportsImage:    true,
		SupportsFile:     true,
		SupportsAudio:    true,
		SupportsVideo:    true,
		MaxFileSizeMB:    20,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 20, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "mime_attachment", Fallback: "无本地文件时降级为图片摘要和链接"},
			{Type: "file", Supported: true, MaxSizeMB: 20, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "mime_attachment", Fallback: "无本地文件时降级为文件名、大小和链接摘要"},
			{Type: "audio", Supported: true, MaxSizeMB: 20, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "mime_attachment", Fallback: "无本地文件时降级为音频摘要和链接"},
			{Type: "video", Supported: true, MaxSizeMB: 20, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "mime_attachment", Fallback: "无本地文件时降级为视频摘要和链接"},
		},
		Notes: []string{"邮件媒体作为 MIME attachment 发送；公网 URL 或下载失败媒体会追加到正文摘要。"},
	}
}

func (s *Sink) ValidateConfig(config map[string]any) error {
	if smtpAddr, _ := config["smtp_addr"].(string); smtpAddr == "" {
		return fmt.Errorf("email 缺少 smtp_addr")
	}
	if from, _ := config["from"].(string); from == "" {
		return fmt.Errorf("email 缺少 from")
	}
	if len(configStringSlice(config, "to")) == 0 {
		return fmt.Errorf("email 缺少 to")
	}
	return nil
}

func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	cfg := sink.Config
	addr, _ := cfg["smtp_addr"].(string)
	from, _ := cfg["from"].(string)
	to := configStringSlice(cfg, "to")
	subject, _ := cfg["subject"].(string)
	if subject == "" {
		subject = "Telegram Message Forward"
	}
	if err := s.ValidateConfig(cfg); err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, nil
	}
	msg, attachCount, err := buildMessage(from, to, subject, payload)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, nil
	}
	var auth smtp.Auth
	username, _ := cfg["username"].(string)
	if username != "" && len(sink.Secret) > 0 {
		auth = smtp.PlainAuth("", username, string(sink.Secret), smtpHost(addr))
	}
	if err := s.send(addr, auth, from, to, msg); err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}
	summary, _ := json.Marshal(map[string]any{"recipients": len(to), "attachments": attachCount})
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}

func buildMessage(from string, to []string, subject string, payload pluginsink.Payload) ([]byte, int, error) {
	attachments := localAttachments(payload.Media)
	text := payload.Text
	if len(payload.Media) > len(attachments) && payload.FallbackText != "" && !strings.Contains(text, payload.FallbackText) {
		text = joinText(text, payload.FallbackText)
	}
	if len(attachments) == 0 {
		return buildSimpleMessage(from, to, subject, payload.Format, text), 0, nil
	}
	boundary := randomBoundary()
	var b bytes.Buffer
	writeHeaders(&b, map[string]string{
		"From":         from,
		"To":           strings.Join(to, ", "),
		"Subject":      mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version": "1.0",
		"Content-Type": `multipart/mixed; boundary="` + boundary + `"`,
	})
	fmt.Fprintf(&b, "\r\n--%s\r\n", boundary)
	writeBodyPart(&b, payload.Format, text)
	for _, item := range attachments {
		if err := writeAttachment(&b, boundary, item); err != nil {
			return nil, 0, err
		}
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes(), len(attachments), nil
}

func buildSimpleMessage(from string, to []string, subject string, format string, text string) []byte {
	var b bytes.Buffer
	headers := map[string]string{
		"From":         from,
		"To":           strings.Join(to, ", "),
		"Subject":      mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version": "1.0",
	}
	if format == "html" {
		headers["Content-Type"] = `text/html; charset="utf-8"`
	} else {
		headers["Content-Type"] = `text/plain; charset="utf-8"`
	}
	headers["Content-Transfer-Encoding"] = "base64"
	writeHeaders(&b, headers)
	b.WriteString("\r\n")
	writeBase64(&b, []byte(text))
	return b.Bytes()
}

func writeBodyPart(b *bytes.Buffer, format string, text string) {
	contentType := `text/plain; charset="utf-8"`
	if format == "html" {
		contentType = `text/html; charset="utf-8"`
	}
	writeHeaders(b, map[string]string{
		"Content-Type":              contentType,
		"Content-Transfer-Encoding": "base64",
	})
	b.WriteString("\r\n")
	writeBase64(b, []byte(text))
}

func writeAttachment(b *bytes.Buffer, boundary string, item domainmessage.Media) error {
	data, err := os.ReadFile(filepath.Clean(item.LocalPath))
	if err != nil {
		return fmt.Errorf("读取邮件附件失败: %w", err)
	}
	fileName := item.FileName
	if fileName == "" {
		fileName = filepath.Base(item.LocalPath)
	}
	mimeType := item.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	fmt.Fprintf(b, "\r\n--%s\r\n", boundary)
	writeHeaders(b, map[string]string{
		"Content-Type":              mimeType + `; name="` + escapeHeaderParam(fileName) + `"`,
		"Content-Disposition":       `attachment; filename="` + escapeHeaderParam(fileName) + `"`,
		"Content-Transfer-Encoding": "base64",
	})
	b.WriteString("\r\n")
	writeBase64(b, data)
	return nil
}

func writeHeaders(b *bytes.Buffer, headers map[string]string) {
	order := []string{"From", "To", "Subject", "MIME-Version", "Content-Type", "Content-Disposition", "Content-Transfer-Encoding"}
	written := map[string]struct{}{}
	for _, key := range order {
		if value, ok := headers[key]; ok {
			fmt.Fprintf(b, "%s: %s\r\n", key, value)
			written[key] = struct{}{}
		}
	}
	for key, value := range headers {
		if _, ok := written[key]; ok {
			continue
		}
		fmt.Fprintf(b, "%s: %s\r\n", key, value)
	}
}

func writeBase64(b *bytes.Buffer, data []byte) {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(encoded, data)
	for len(encoded) > 76 {
		b.Write(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	b.Write(encoded)
	b.WriteString("\r\n")
}

func localAttachments(media []domainmessage.Media) []domainmessage.Media {
	out := make([]domainmessage.Media, 0, len(media))
	for _, item := range media {
		if item.LocalPath != "" {
			out = append(out, item)
		}
	}
	return out
}

func configStringSlice(config map[string]any, key string) []string {
	raw, ok := config[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	}
	return nil
}

func smtpHost(addr string) string {
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

func randomBoundary() string {
	var b [12]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "tmf-boundary"
	}
	return "tmf-" + base64.RawURLEncoding.EncodeToString(b[:])
}

func escapeHeaderParam(value string) string {
	return strings.ReplaceAll(value, `"`, "")
}

func joinText(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n" + b
}
