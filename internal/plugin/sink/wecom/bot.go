package wecom

import (
	"context"
	"fmt"

	"telegram-message-forward/internal/domain/formschema"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/httpclient"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func init() {
	pluginsink.Register("wecom_bot", func() (pluginsink.Plugin, error) {
		return NewBot(), nil
	})
}

// BotSink 是企业微信群机器人 Sink（webhook，无 access_token）。
//
// key 保存在 sink.Secret；config 可选 webhook_url 覆盖默认地址。
type BotSink struct {
	client *httpclient.Client
}

// NewBot 创建群机器人 Sink。
func NewBot() *BotSink {
	return &BotSink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*BotSink)(nil)

// Name 返回插件名。
func (s *BotSink) Name() string { return "wecom_bot" }

// Descriptor 返回后台表单元数据。
func (s *BotSink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "企业微信群机器人",
		Description: "通过企业微信群机器人 webhook 推送文本或 Markdown。",
		ConfigFields: []formschema.FieldSpec{
			{
				Key:         "webhook_url",
				Label:       "完整 Webhook URL",
				Type:        formschema.FieldText,
				Placeholder: "可选；填写后优先使用完整 URL",
				Help:        "通常只需要填写下方机器人 Key；如果网关地址特殊，可直接填写完整 webhook_url。",
			},
		},
		SecretField: &formschema.FieldSpec{
			Key:         "secret",
			Label:       "机器人 Key",
			Type:        formschema.FieldPassword,
			Secret:      true,
			Placeholder: "企业微信群机器人 key",
		},
		Capabilities: s.Capabilities(),
	}
}

// Capabilities 声明能力。
func (s *BotSink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		MaxTextLength:    4096,
	}
}

// ValidateConfig 校验必须能拿到 webhook key。
func (s *BotSink) ValidateConfig(config map[string]any) error {
	if url, _ := config["webhook_url"].(string); url != "" {
		return nil
	}
	// 否则依赖 secret 提供的 key，运行期再校验。
	return nil
}

// webhookURL 组装机器人 webhook 地址。优先 config.webhook_url；否则用 secret 作为 key。
func (s *BotSink) webhookURL(sink *domainsink.Sink) (string, string, error) {
	if url, _ := sink.Config["webhook_url"].(string); url != "" {
		return url, url, nil
	}
	key := string(sink.Secret)
	if key == "" {
		return "", "", fmt.Errorf("wecom_bot 缺少 webhook key（secret）或 webhook_url")
	}
	return apiBase + "/webhook/send?key=" + key, "wecom_bot:" + key, nil
}

// Send 向群机器人 webhook 发送消息。
func (s *BotSink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	url, bucket, err := s.webhookURL(sink)
	if err != nil {
		return failResult(nil, err.Error()), nil
	}

	if err := waitRate(ctx, bucket); err != nil {
		return nil, err
	}

	msgType := msgTypeFor(payload.Format)
	body := map[string]any{"msgtype": msgType}
	if msgType == "markdown" {
		body["markdown"] = textContent{Content: payload.Text}
	} else {
		body["text"] = textContent{Content: payload.Text}
	}

	resp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return failResult(nil, err.Error()), err
	}
	summary, ok, _, errMsg := parseResp(resp.Body)
	if !ok {
		return failResult(summary, errMsg), nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}
