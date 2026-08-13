// Package gotify 实现 Gotify 推送 Sink。
package gotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"telegram-message-forward/internal/domain/formschema"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/httpclient"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func init() {
	pluginsink.Register("gotify", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

// Sink 是 Gotify 推送 Sink。
type Sink struct {
	client *httpclient.Client
}

// New 创建 Gotify Sink。
func New() *Sink {
	return &Sink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*Sink)(nil)

func (s *Sink) Name() string { return "gotify" }

func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "Gotify",
		Description: "通过 Gotify 应用推送消息，支持文本、Markdown 和远程图片通知。",
		ConfigFields: []formschema.FieldSpec{
			{Key: "server_url", Label: "Server URL", Type: formschema.FieldText, Required: true, Placeholder: "https://gotify.example.com"},
			{Key: "app_token", Label: "Application Token", Type: formschema.FieldText, Required: true, Placeholder: "Gotify 应用 token"},
			{Key: "title", Label: "默认标题", Type: formschema.FieldText, Default: "Telegram Message Forward"},
			{Key: "priority", Label: "优先级", Type: formschema.FieldNumber, Default: 5, Placeholder: "5"},
		},
		Capabilities: s.Capabilities(),
	}
}

func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		SupportsImage:    true,
		MaxMediaItems:    1,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: false, DeliveryMode: "big_image_url_or_markdown_link", Fallback: "本地图片无法直传时降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: false, Fallback: "降级为文件名、大小和链接摘要"},
			{Type: "audio", Supported: false, Fallback: "降级为音频摘要和链接"},
			{Type: "video", Supported: false, Fallback: "降级为视频摘要和链接"},
		},
		Notes: []string{"Gotify 不提供通用附件上传；远程图片 URL 可通过 notification extras 展示为大图，本地媒体按文本摘要降级。"},
	}
}

func (s *Sink) ValidateConfig(config map[string]any) error {
	if serverURL, _ := config["server_url"].(string); strings.TrimSpace(serverURL) == "" {
		return fmt.Errorf("gotify 缺少 server_url")
	}
	if token, _ := config["app_token"].(string); strings.TrimSpace(token) == "" {
		return fmt.Errorf("gotify 缺少 app_token")
	}
	return nil
}

type apiResp struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	if err := s.ValidateConfig(sink.Config); err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, nil
	}
	endpoint, err := messageEndpoint(configString(sink.Config, "server_url"))
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, nil
	}
	if len(payload.Media) > 0 && firstImageURL(payload) == "" && payload.FallbackText != "" {
		payload.Text = payload.FallbackText
		payload.Format = "text"
	}
	body := buildBody(sink.Config, payload)
	headers := map[string]string{"X-Gotify-Key": configString(sink.Config, "app_token")}
	resp, err := s.client.PostJSON(ctx, endpoint, body, headers)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}
	var r apiResp
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &r); err != nil {
			return &pluginsink.Result{Success: false, ResponseSummary: truncate(resp.Body, 512), Error: "响应解析失败: " + err.Error()}, nil
		}
	}
	summary, _ := json.Marshal(map[string]any{"status_code": resp.StatusCode, "id": r.ID})
	if !resp.IsSuccess() {
		return pluginsink.HTTPFailure(summary, resp.StatusCode, resp.Header, fmt.Sprintf("Gotify 返回非 2xx 状态: %d", resp.StatusCode)), nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}

func buildBody(config map[string]any, payload pluginsink.Payload) map[string]any {
	body := map[string]any{
		"title":    configDefault(config, "title", "Telegram Message Forward"),
		"message":  payload.Text,
		"priority": configInt(config, "priority", 5),
	}
	extras := map[string]any{}
	if payload.Format == "markdown" {
		extras["client::display"] = map[string]any{"contentType": "text/markdown"}
	}
	if imageURL := firstImageURL(payload); imageURL != "" {
		extras["client::notification"] = map[string]any{"bigImageUrl": imageURL}
	}
	if len(extras) > 0 {
		body["extras"] = extras
	}
	return body
}

func messageEndpoint(serverURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil {
		return "", fmt.Errorf("gotify server_url 无效: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("gotify server_url 必须包含协议和主机")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/message"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func firstImageURL(payload pluginsink.Payload) string {
	for _, item := range payload.Media {
		if item.Type != "photo" && item.Type != "image" {
			continue
		}
		if item.RemoteURL != "" {
			return item.RemoteURL
		}
		if item.URL != "" {
			return item.URL
		}
	}
	return ""
}

func configString(config map[string]any, key string) string {
	v, _ := config[key].(string)
	return strings.TrimSpace(v)
}

func configDefault(config map[string]any, key string, fallback string) string {
	if v := configString(config, key); v != "" {
		return v
	}
	return fallback
}

func configInt(config map[string]any, key string, fallback int) int {
	switch v := config[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
