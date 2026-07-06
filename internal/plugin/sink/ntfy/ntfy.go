// Package ntfy 实现 ntfy HTTP publish Sink。
package ntfy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

const (
	attachmentModeAuto   = "auto"
	attachmentModeUpload = "upload"
	attachmentModeURL    = "url"
	attachmentModeText   = "text"

	ntfyShUploadLimitMB     = 2
	selfHostedUploadLimitMB = 15
)

func init() {
	pluginsink.Register("ntfy", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sink 是 ntfy 发布 Sink。
type Sink struct {
	client httpDoer
}

// New 创建 ntfy Sink。
func New() *Sink {
	return &Sink{client: &http.Client{}}
}

var _ pluginsink.Plugin = (*Sink)(nil)

func (s *Sink) Name() string { return "ntfy" }

func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "ntfy",
		Description: "通过 ntfy topic 发布通知，支持文本、Markdown 和附件。",
		ConfigFields: []formschema.FieldSpec{
			{Key: "topic_url", Label: "Topic URL", Type: formschema.FieldText, Required: true, Placeholder: "https://ntfy.sh/my-topic"},
			{Key: "title", Label: "默认标题", Type: formschema.FieldText, Default: "Telegram Message Forward"},
			{Key: "priority", Label: "优先级", Type: formschema.FieldSelect, Options: []formschema.Option{
				{Label: "默认", Value: ""},
				{Label: "低", Value: "low"},
				{Label: "普通", Value: "default"},
				{Label: "高", Value: "high"},
				{Label: "紧急", Value: "urgent"},
			}},
			{Key: "attachment_mode", Label: "附件策略", Type: formschema.FieldSelect, Default: attachmentModeAuto, Help: "自动模式会小文件直传、大文件走 URL 或摘要。", Options: []formschema.Option{
				{Label: "自动：小文件直传，大文件 URL", Value: attachmentModeAuto},
				{Label: "优先直传", Value: attachmentModeUpload},
				{Label: "优先 URL", Value: attachmentModeURL},
				{Label: "仅文本摘要", Value: attachmentModeText},
			}},
			{Key: "upload_max_mb", Label: "直传上限 MB", Type: formschema.FieldNumber, Placeholder: "ntfy.sh 默认 2，自建默认 15", Help: "自动模式下，本地附件不超过该大小才直传；留空时按 Topic URL 自动选择默认值。"},
		},
		SecretField: &formschema.FieldSpec{
			Key:         "secret",
			Label:       "Access Token",
			Type:        formschema.FieldPassword,
			Secret:      true,
			Placeholder: "可选；私有 ntfy 服务 token",
		},
		Capabilities: s.Capabilities(),
	}
}

func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		SupportsImage:    true,
		SupportsFile:     true,
		SupportsAudio:    true,
		SupportsVideo:    true,
		MaxFileSizeMB:    15,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 15, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "attach_header_or_upload", Fallback: "超限或不可读时降级为图片摘要和链接"},
			{Type: "file", Supported: true, MaxSizeMB: 15, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "attach_header_or_upload", Fallback: "超限或不可读时降级为文件摘要和链接"},
			{Type: "audio", Supported: true, MaxSizeMB: 15, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "attach_header_or_upload", Fallback: "超限或不可读时降级为音频摘要和链接"},
			{Type: "video", Supported: true, MaxSizeMB: 15, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "attach_header_or_upload", Fallback: "超限或不可读时降级为视频摘要和链接"},
		},
		Notes: []string{"ntfy 远程媒体 URL 通过 Attach header；本地媒体可作为请求体上传，自动模式下小文件直传、大文件走 URL 或摘要。ntfy.sh 公共服务附件直传限制较低，自建服务可在配置中调大直传上限。"},
	}
}

func (s *Sink) ValidateConfig(config map[string]any) error {
	if topicURL, _ := config["topic_url"].(string); topicURL == "" {
		return fmt.Errorf("ntfy 缺少 topic_url")
	}
	return nil
}

func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	topicURL, _ := sink.Config["topic_url"].(string)
	if topicURL == "" {
		return &pluginsink.Result{Success: false, Error: "ntfy 缺少 topic_url"}, nil
	}
	media := firstMedia(payload.Media)
	if len(payload.Media) == 0 {
		return s.publishText(ctx, sink, payload, "")
	}

	attachURL := mediaURL(media)
	switch attachmentMode(sink) {
	case attachmentModeText:
		return s.publishFallbackText(ctx, sink, payload)
	case attachmentModeURL:
		if attachURL != "" {
			return s.publishText(ctx, sink, payload, attachURL)
		}
		if media.LocalPath != "" && shouldUploadLocal(sink, media) {
			return s.publishLocalMedia(ctx, sink, payload, media)
		}
		return s.publishFallbackText(ctx, sink, payload)
	case attachmentModeUpload:
		if media.LocalPath != "" {
			return s.publishLocalMedia(ctx, sink, payload, media)
		}
		if attachURL != "" {
			return s.publishText(ctx, sink, payload, attachURL)
		}
		return s.publishFallbackText(ctx, sink, payload)
	default:
		if media.LocalPath != "" && shouldUploadLocal(sink, media) {
			return s.publishLocalMedia(ctx, sink, payload, media)
		}
		if attachURL != "" {
			return s.publishText(ctx, sink, payload, attachURL)
		}
		return s.publishFallbackText(ctx, sink, payload)
	}
}

func (s *Sink) publishText(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, attachURL string) (*pluginsink.Result, error) {
	topicURL, _ := sink.Config["topic_url"].(string)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, topicURL, strings.NewReader(payload.Text))
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req, sink, payload.Format, attachURL, "")
	return s.do(req)
}

func (s *Sink) publishLocalMedia(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, media domainmessage.Media) (*pluginsink.Result, error) {
	if payload.Text != "" {
		if res, err := s.publishText(ctx, sink, payload, ""); err != nil || res == nil || !res.Success {
			return res, err
		}
	}
	return s.publishAttachment(ctx, sink, media)
}

func (s *Sink) publishFallbackText(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload) (*pluginsink.Result, error) {
	if payload.FallbackText != "" {
		payload.Text = payload.FallbackText
	}
	return s.publishText(ctx, sink, payload, "")
}

func (s *Sink) publishAttachment(ctx context.Context, sink *domainsink.Sink, media domainmessage.Media) (*pluginsink.Result, error) {
	data, err := os.ReadFile(filepath.Clean(media.LocalPath))
	if err != nil {
		return &pluginsink.Result{Success: false, Error: "读取附件失败: " + err.Error()}, nil
	}
	topicURL, _ := sink.Config["topic_url"].(string)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, topicURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	fileName := media.FileName
	if fileName == "" {
		fileName = filepath.Base(media.LocalPath)
	}
	s.applyHeaders(req, sink, "", "", fileName)
	return s.do(req)
}

func (s *Sink) applyHeaders(req *http.Request, sink *domainsink.Sink, format string, attachURL string, filename string) {
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if title, _ := sink.Config["title"].(string); title != "" {
		req.Header.Set("Title", title)
	}
	if priority, _ := sink.Config["priority"].(string); priority != "" {
		req.Header.Set("Priority", priority)
	}
	if format == "markdown" {
		req.Header.Set("Markdown", "yes")
	}
	if attachURL != "" {
		req.Header.Set("Attach", attachURL)
	}
	if filename != "" {
		req.Header.Set("Filename", filename)
	}
	if len(sink.Secret) > 0 {
		req.Header.Set("Authorization", "Bearer "+string(sink.Secret))
	}
}

func (s *Sink) do(req *http.Request) (*pluginsink.Result, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	summary, _ := json.Marshal(map[string]any{"status_code": resp.StatusCode})
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &pluginsink.Result{Success: false, ResponseSummary: summary, Error: fmt.Sprintf("ntfy 返回非 2xx 状态: %d %s", resp.StatusCode, string(body))}, nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}

func firstMedia(media []domainmessage.Media) domainmessage.Media {
	if len(media) == 0 {
		return domainmessage.Media{}
	}
	return media[0]
}

func mediaURL(media domainmessage.Media) string {
	if media.RemoteURL != "" {
		return media.RemoteURL
	}
	return media.URL
}

func attachmentMode(sink *domainsink.Sink) string {
	mode, _ := sink.Config["attachment_mode"].(string)
	switch mode {
	case attachmentModeUpload, attachmentModeURL, attachmentModeText:
		return mode
	default:
		return attachmentModeAuto
	}
}

func shouldUploadLocal(sink *domainsink.Sink, media domainmessage.Media) bool {
	size := media.Size
	if size <= 0 && media.LocalPath != "" {
		if stat, err := os.Stat(filepath.Clean(media.LocalPath)); err == nil {
			size = stat.Size()
		}
	}
	if size <= 0 {
		return true
	}
	return size <= int64(uploadLimitMB(sink))*1024*1024
}

func uploadLimitMB(sink *domainsink.Sink) int {
	if v, ok := positiveIntConfig(sink.Config["upload_max_mb"]); ok {
		return v
	}
	topicURL, _ := sink.Config["topic_url"].(string)
	if strings.HasPrefix(strings.ToLower(topicURL), "https://ntfy.sh/") || strings.HasPrefix(strings.ToLower(topicURL), "http://ntfy.sh/") {
		return ntfyShUploadLimitMB
	}
	return selfHostedUploadLimitMB
}

func positiveIntConfig(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, v > 0
	case int64:
		return int(v), v > 0
	case float64:
		return int(v), v > 0
	case json.Number:
		i, err := strconv.Atoi(v.String())
		return i, err == nil && i > 0
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		return i, err == nil && i > 0
	default:
		return 0, false
	}
}
