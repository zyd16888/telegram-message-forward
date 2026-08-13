// Package feishu 实现飞书自定义机器人 Sink。
package feishu

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"telegram-message-forward/internal/domain/formschema"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/httpclient"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func init() {
	pluginsink.Register("feishu_bot", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

// Sink 是飞书自定义机器人 webhook Sink。
type Sink struct {
	client *httpclient.Client
}

// New 创建飞书机器人 Sink。
func New() *Sink {
	return &Sink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*Sink)(nil)

func (s *Sink) Name() string { return "feishu_bot" }

func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "飞书自定义机器人",
		Description: "通过飞书群自定义机器人 webhook 推送文本或富文本消息。",
		ConfigFields: []formschema.FieldSpec{
			{
				Key:         "webhook_url",
				Label:       "Webhook URL",
				Type:        formschema.FieldText,
				Required:    true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/...",
			},
		},
		SecretField:  &formschema.FieldSpec{Key: "secret", Label: "签名密钥", Type: formschema.FieldPassword, Secret: true, Placeholder: "可选；机器人安全设置中的签名密钥"},
		Capabilities: s.Capabilities(),
	}
}

func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		MaxTextLength:    20000,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: false, RequiresUpload: true, SupportsBinary: false, Fallback: "自定义机器人 webhook 不直接上传本地图片，降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: false, RequiresUpload: true, SupportsBinary: false, Fallback: "降级为文件名、大小和原始链接摘要"},
			{Type: "audio", Supported: false, Fallback: "降级为音频文件名、大小和原始链接摘要"},
			{Type: "video", Supported: false, Fallback: "降级为视频文件名、大小和原始链接摘要"},
		},
		Notes: []string{"飞书 webhook 图片消息需要 image_key；本插件不持有飞书应用 token，因此媒体默认按文本降级。"},
	}
}

func (s *Sink) ValidateConfig(config map[string]any) error {
	if url, _ := config["webhook_url"].(string); url == "" {
		return fmt.Errorf("feishu_bot 缺少 webhook_url")
	}
	return nil
}

type apiResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	url, _ := sink.Config["webhook_url"].(string)
	if url == "" {
		return &pluginsink.Result{Success: false, Error: "feishu_bot 缺少 webhook_url"}, nil
	}
	if err := waitRate(ctx, url); err != nil {
		return nil, err
	}
	if len(payload.Media) > 0 {
		payload.Format = "text"
		if payload.FallbackText != "" {
			payload.Text = payload.FallbackText
		}
	}
	body := buildBody(payload)
	if len(sink.Secret) > 0 {
		timestamp := time.Now().Unix()
		body["timestamp"] = timestamp
		body["sign"] = sign(string(sink.Secret), timestamp)
	}
	resp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}
	if !resp.IsSuccess() {
		summary, _ := json.Marshal(map[string]any{"status_code": resp.StatusCode})
		return pluginsink.HTTPFailure(summary, resp.StatusCode, resp.Header, fmt.Sprintf("飞书返回 HTTP %d", resp.StatusCode)), nil
	}
	var r apiResp
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return &pluginsink.Result{Success: false, ResponseSummary: truncate(resp.Body, 512), Error: "响应解析失败: " + err.Error()}, nil
	}
	summary, _ := json.Marshal(map[string]any{"code": r.Code, "msg": r.Msg})
	if r.Code != 0 {
		return pluginsink.PermanentResult(summary, fmt.Sprintf("飞书返回错误 code=%d msg=%s", r.Code, r.Msg)), nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}

func buildBody(payload pluginsink.Payload) map[string]any {
	if payload.Format == "markdown" {
		return map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"header":   map[string]any{"title": map[string]string{"tag": "plain_text", "content": "消息通知"}},
				"elements": []map[string]string{{"tag": "markdown", "content": payload.Text}},
			},
		}
	}
	return map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": payload.Text},
	}
}

func sign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

var (
	limiterMu sync.Mutex
	limiters  = map[string]*rate.Limiter{}
)

func limiterFor(key string) *rate.Limiter {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	l, ok := limiters[key]
	if !ok {
		l = rate.NewLimiter(rate.Every(time.Second), 10)
		limiters[key] = l
	}
	return l
}

func waitRate(ctx context.Context, key string) error {
	return limiterFor(key).Wait(ctx)
}
