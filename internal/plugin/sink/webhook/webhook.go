// Package webhook 实现通用 Webhook Sink：把渲染后的内容 POST 到目标 URL。
package webhook

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
	pluginsink.Register("webhook", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

// Sink 是 Webhook 渠道插件。它只做渠道适配，不查库、不判规则、不做重试调度。
type Sink struct {
	client *httpclient.Client
}

// New 创建 Webhook Sink。
func New() *Sink {
	return &Sink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*Sink)(nil)

// Name 返回插件名。
func (s *Sink) Name() string { return "webhook" }

// Descriptor 返回后台表单元数据。
func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "通用 Webhook",
		Description: "将渲染后的消息以 JSON POST 到指定地址。",
		ConfigFields: []formschema.FieldSpec{
			{
				Key:         "url",
				Label:       "Webhook URL",
				Type:        formschema.FieldText,
				Required:    true,
				Placeholder: "https://example.com/webhook",
			},
			{
				Key:   "headers",
				Label: "请求头",
				Type:  formschema.FieldKeyValue,
				Help:  "可选，逐项填写 HTTP header；Authorization 留空时会用密钥生成 Bearer token。",
			},
		},
		SecretField: &formschema.FieldSpec{
			Key:         "secret",
			Label:       "Bearer Token",
			Type:        formschema.FieldPassword,
			Secret:      true,
			Placeholder: "可选",
		},
		Capabilities: s.Capabilities(),
	}
}

// Capabilities 声明 Webhook 支持的能力。
func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		SupportsHTML:     true,
	}
}

// ValidateConfig 校验配置，必须包含 url。
func (s *Sink) ValidateConfig(config map[string]any) error {
	if url, _ := config["url"].(string); url == "" {
		return fmt.Errorf("webhook sink 缺少 url 配置")
	}
	return nil
}

// body 是默认的 Webhook 请求体。
type body struct {
	Text   string `json:"text"`
	Format string `json:"format"`
}

// Send 将渲染内容 POST 到目标 URL。
func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	url, _ := sink.Config["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("webhook sink 缺少 url 配置")
	}

	headers := map[string]string{}
	if raw, ok := sink.Config["headers"].(map[string]any); ok {
		for k, v := range raw {
			if sv, ok := v.(string); ok {
				headers[k] = sv
			}
		}
	}
	// secret 作为 Bearer token（可选）。
	if len(sink.Secret) > 0 {
		if _, exists := headers["Authorization"]; !exists {
			headers["Authorization"] = "Bearer " + string(sink.Secret)
		}
	}

	reqBody := body{Text: payload.Text, Format: payload.Format}
	resp, err := s.client.PostJSON(ctx, url, reqBody, headers)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}

	summary := responseSummary(resp)
	if !resp.IsSuccess() {
		return &pluginsink.Result{
			Success:         false,
			ResponseSummary: summary,
			Error:           fmt.Sprintf("webhook 返回非 2xx 状态: %d", resp.StatusCode),
		}, nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}

// responseSummary 生成脱敏后的响应摘要，截断过长 body。
func responseSummary(resp *httpclient.Response) []byte {
	const maxBody = 512
	bodyStr := string(resp.Body)
	if len(bodyStr) > maxBody {
		bodyStr = bodyStr[:maxBody]
	}
	b, _ := json.Marshal(map[string]any{
		"status_code": resp.StatusCode,
		"body":        bodyStr,
	})
	return b
}
