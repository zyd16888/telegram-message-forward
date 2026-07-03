// Package bark 实现 Bark 推送 Sink。
package bark

import (
	"context"
	"encoding/json"
	"fmt"

	"telegram-message-forward/internal/domain/formschema"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/httpclient"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func init() {
	pluginsink.Register("bark", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

// Sink 是 Bark 推送 Sink。
type Sink struct {
	client *httpclient.Client
}

// New 创建 Bark Sink。
func New() *Sink {
	return &Sink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*Sink)(nil)

func (s *Sink) Name() string { return "bark" }

func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "Bark",
		Description: "通过 Bark 推送到 iOS 设备，支持文本和远程图片 URL。",
		ConfigFields: []formschema.FieldSpec{
			{Key: "server_url", Label: "Server URL", Type: formschema.FieldText, Default: "https://api.day.app/push", Placeholder: "https://api.day.app/push"},
			{Key: "device_key", Label: "Device Key", Type: formschema.FieldText, Required: true, Placeholder: "Bark 设备 key"},
			{Key: "title", Label: "默认标题", Type: formschema.FieldText, Default: "Telegram Message Forward"},
			{Key: "group", Label: "分组", Type: formschema.FieldText, Placeholder: "Telegram"},
		},
		Capabilities: s.Capabilities(),
	}
}

func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:  true,
		SupportsImage: true,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: false, DeliveryMode: "image_url", Fallback: "本地图片无法直传时降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: false, Fallback: "降级为文件名、大小和链接摘要"},
			{Type: "audio", Supported: false, Fallback: "降级为音频摘要和链接"},
			{Type: "video", Supported: false, Fallback: "降级为视频摘要和链接"},
		},
		Notes: []string{"Bark 媒体仅支持远程图片 URL；本地文件和其它媒体按文本摘要降级。"},
	}
}

func (s *Sink) ValidateConfig(config map[string]any) error {
	if key, _ := config["device_key"].(string); key == "" {
		return fmt.Errorf("bark 缺少 device_key")
	}
	return nil
}

type apiResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	if err := s.ValidateConfig(sink.Config); err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, nil
	}
	serverURL, _ := sink.Config["server_url"].(string)
	if serverURL == "" {
		serverURL = "https://api.day.app/push"
	}
	body := map[string]any{
		"device_key": configString(sink.Config, "device_key"),
		"title":      configDefault(sink.Config, "title", "Telegram Message Forward"),
		"body":       payload.Text,
	}
	if group := configString(sink.Config, "group"); group != "" {
		body["group"] = group
	}
	if imageURL := firstImageURL(payload); imageURL != "" {
		body["image"] = imageURL
	} else if len(payload.Media) > 0 && payload.FallbackText != "" {
		body["body"] = payload.FallbackText
	}
	resp, err := s.client.PostJSON(ctx, serverURL, body, nil)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}
	var r apiResp
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return &pluginsink.Result{Success: false, ResponseSummary: truncate(resp.Body, 512), Error: "响应解析失败: " + err.Error()}, nil
	}
	summary, _ := json.Marshal(map[string]any{"code": r.Code, "message": r.Message})
	if !resp.IsSuccess() || r.Code != 200 {
		return &pluginsink.Result{Success: false, ResponseSummary: summary, Error: fmt.Sprintf("Bark 返回错误 code=%d message=%s", r.Code, r.Message)}, nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
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
	return v
}

func configDefault(config map[string]any, key string, fallback string) string {
	if v := configString(config, key); v != "" {
		return v
	}
	return fallback
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
