// Package dingtalk 实现钉钉自定义机器人 Sink（webhook，可选加签校验）。
package dingtalk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"telegram-message-forward/internal/domain/formschema"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/httpclient"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

// apiBase 是钉钉机器人 webhook 根地址。声明为 var 以便测试注入 mock server。
var apiBase = "https://oapi.dingtalk.com/robot/send"

func init() {
	pluginsink.Register("dingtalk_bot", func() (pluginsink.Plugin, error) {
		return New(), nil
	})
}

// Sink 是钉钉自定义机器人 Sink。
//
// access_token 保存在 config；加签密钥（安全设置里以 SEC 开头）作为可选 secret——
// 钉钉自定义机器人的安全校验三选一（关键词/IP 白名单/加签），加签不是必须项，
// 未配置时按无签名请求发送。
type Sink struct {
	client *httpclient.Client
}

// New 创建钉钉机器人 Sink。
func New() *Sink {
	return &Sink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*Sink)(nil)

// Name 返回插件名。
func (s *Sink) Name() string { return "dingtalk_bot" }

// Descriptor 返回后台表单元数据。
func (s *Sink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "钉钉自定义机器人",
		Description: "通过钉钉群自定义机器人 webhook 推送文本或 Markdown；支持加签安全校验。",
		ConfigFields: []formschema.FieldSpec{
			{
				Key:         "access_token",
				Label:       "Access Token",
				Type:        formschema.FieldText,
				Placeholder: "机器人 webhook 地址中 access_token= 后面的部分",
				Help:        "机器人管理页「Webhook」地址里 access_token 参数的值；也可以直接在下方填写完整 Webhook URL。",
			},
			{
				Key:         "webhook_url",
				Label:       "完整 Webhook URL",
				Type:        formschema.FieldText,
				Placeholder: "可选；填写后优先使用完整 URL",
				Help:        "通常只需要填写上方 Access Token；如果网关地址特殊，可直接填写完整 webhook_url。",
			},
		},
		SecretField: &formschema.FieldSpec{
			Key:         "secret",
			Label:       "加签密钥",
			Type:        formschema.FieldPassword,
			Secret:      true,
			Placeholder: "机器人安全设置「加签」下的 SEC 开头字符串；未启用加签可留空",
		},
		Capabilities: s.Capabilities(),
	}
}

// Capabilities 声明能力。
func (s *Sink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		SupportsImage:    true,
		MaxTextLength:    20000,
		MaxMediaItems:    1,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, SupportsPublicURL: true, RequiresUpload: false, SupportsBinary: false, DeliveryMode: "markdown_public_url", Fallback: "本地图片无法直传时降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: false, Fallback: "自定义机器人不支持文件直传，降级为文件名、大小和原始链接摘要"},
			{Type: "audio", Supported: false, Fallback: "降级为音频文件名、大小和原始链接摘要"},
			{Type: "video", Supported: false, Fallback: "降级为视频文件名、大小和原始链接摘要"},
		},
		Notes: []string{"钉钉自定义机器人可在 markdown 中引用公网图片 URL；本地媒体需降级或另行上传到公网可访问位置。"},
	}
}

// ValidateConfig 校验必须能拿到 access_token 或完整 webhook_url。
func (s *Sink) ValidateConfig(config map[string]any) error {
	if url, _ := config["webhook_url"].(string); url != "" {
		return nil
	}
	if token, _ := config["access_token"].(string); token != "" {
		return nil
	}
	return fmt.Errorf("dingtalk_bot 缺少 access_token 或 webhook_url")
}

// requestURL 组装机器人 webhook 地址；加签密钥存在时附上 timestamp + sign。
func (s *Sink) requestURL(sink *domainsink.Sink) (string, string, error) {
	base, _ := sink.Config["webhook_url"].(string)
	bucket := base
	if base == "" {
		token, _ := sink.Config["access_token"].(string)
		if token == "" {
			return "", "", fmt.Errorf("dingtalk_bot 缺少 access_token 或 webhook_url")
		}
		base = apiBase + "?access_token=" + token
		bucket = "dingtalk_bot:" + token
	}

	secret := string(sink.Secret)
	if secret == "" {
		return base, bucket, nil
	}

	ts := time.Now().UnixMilli()
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", base, sep, ts, url.QueryEscape(sign(secret, ts))), bucket, nil
}

// sign 按钉钉加签规则计算签名：base64(hmacSHA256(secret, "timestamp\nsecret"))。
func sign(secret string, timestampMillis int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestampMillis, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

type textContent struct {
	Content string `json:"content"`
}

type markdownContent struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

func msgTypeFor(format string) string {
	if format == "markdown" {
		return "markdown"
	}
	return "text"
}

// apiResp 是钉钉机器人通用响应。
type apiResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// Send 向钉钉自定义机器人 webhook 发送消息。
func (s *Sink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	reqURL, bucket, err := s.requestURL(sink)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, nil
	}

	if err := waitRate(ctx, bucket); err != nil {
		return nil, err
	}

	msgType := msgTypeFor(payload.Format)
	body := map[string]any{"msgtype": msgType}
	if len(payload.Media) > 0 {
		if imageURL := firstImageURL(payload); imageURL != "" {
			payload.Format = "markdown"
			msgType = "markdown"
			body["msgtype"] = msgType
			if payload.Text == "" {
				payload.Text = "![image](" + imageURL + ")"
			} else {
				payload.Text += "\n\n![image](" + imageURL + ")"
			}
		} else {
			payload.Format = "text"
			payload.Text = fallbackText(payload)
			msgType = "text"
			body["msgtype"] = msgType
		}
	}
	if msgType == "markdown" {
		title := sink.Name
		if title == "" {
			title = "消息通知"
		}
		body["markdown"] = markdownContent{Title: title, Text: payload.Text}
	} else {
		body["text"] = textContent{Content: payload.Text}
	}

	resp, err := s.client.PostJSON(ctx, reqURL, body, nil)
	if err != nil {
		return &pluginsink.Result{Success: false, Error: err.Error()}, err
	}
	if !resp.IsSuccess() {
		summary, _ := json.Marshal(map[string]any{"status_code": resp.StatusCode})
		return pluginsink.HTTPFailure(summary, resp.StatusCode, resp.Header, fmt.Sprintf("钉钉返回 HTTP %d", resp.StatusCode)), nil
	}

	var r apiResp
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return &pluginsink.Result{Success: false, ResponseSummary: truncate(resp.Body, 512), Error: "响应解析失败: " + err.Error()}, nil
	}
	summary, _ := json.Marshal(map[string]any{"errcode": r.ErrCode, "errmsg": r.ErrMsg})
	if r.ErrCode != 0 {
		return pluginsink.PermanentResult(summary, fmt.Sprintf("钉钉返回错误 errcode=%d errmsg=%s", r.ErrCode, r.ErrMsg)), nil
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

func fallbackText(payload pluginsink.Payload) string {
	if payload.FallbackText != "" {
		return payload.FallbackText
	}
	return payload.Text
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

// --- 包级限流器：按渠道标识分桶（钉钉自定义机器人约 20 条/分）。---

var (
	limiterMu sync.Mutex
	limiters  = map[string]*rate.Limiter{}
)

func limiterFor(key string) *rate.Limiter {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	l, ok := limiters[key]
	if !ok {
		l = rate.NewLimiter(rate.Every(3*time.Second), 20) // ~20/min，突发 20
		limiters[key] = l
	}
	return l
}

func waitRate(ctx context.Context, key string) error {
	return limiterFor(key).Wait(ctx)
}
