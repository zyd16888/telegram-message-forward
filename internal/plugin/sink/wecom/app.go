package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"telegram-message-forward/internal/domain/formschema"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/httpclient"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func init() {
	pluginsink.Register("wecom_app", func() (pluginsink.Plugin, error) {
		return NewApp(), nil
	})
}

// --- 包级 access_token 缓存：按 corpid 分桶 ---

type cachedToken struct {
	token   string
	expires time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = map[string]*cachedToken{}
)

// AppSink 是企业微信应用消息 Sink（需 corpid/secret/agentid + access_token 缓存刷新）。
//
// corpsecret 保存在 sink.Secret；corpid / agentid / touser 在 config。
type AppSink struct {
	client *httpclient.Client
}

// NewApp 创建应用消息 Sink。
func NewApp() *AppSink {
	return &AppSink{client: httpclient.New()}
}

var _ pluginsink.Plugin = (*AppSink)(nil)

// Name 返回插件名。
func (s *AppSink) Name() string { return "wecom_app" }

// Descriptor 返回后台表单元数据。
func (s *AppSink) Descriptor() pluginsink.Descriptor {
	return pluginsink.Descriptor{
		Type:        s.Name(),
		Label:       "企业微信应用消息",
		Description: "通过企业微信自建应用发送消息，适合固定成员或部门通知。",
		ConfigFields: []formschema.FieldSpec{
			{
				Key:         "corpid",
				Label:       "企业 ID",
				Type:        formschema.FieldText,
				Required:    true,
				Placeholder: "wwxxxxxxxx",
			},
			{
				Key:         "agentid",
				Label:       "Agent ID",
				Type:        formschema.FieldText,
				Required:    true,
				Placeholder: "1000002",
			},
			{
				Key:         "touser",
				Label:       "接收成员",
				Type:        formschema.FieldText,
				Default:     "@all",
				Placeholder: "@all 或 user1|user2",
				Help:        "留空时默认 @all；多个成员用 | 分隔。",
			},
		},
		SecretField: &formschema.FieldSpec{
			Key:         "secret",
			Label:       "CorpSecret",
			Type:        formschema.FieldPassword,
			Secret:      true,
			Required:    true,
			Placeholder: "应用 Secret",
		},
		Capabilities: s.Capabilities(),
	}
}

// Capabilities 声明能力。
func (s *AppSink) Capabilities() domainsink.Capabilities {
	return domainsink.Capabilities{
		SupportsText:     true,
		SupportsMarkdown: true,
		SupportsImage:    true,
		SupportsFile:     true,
		SupportsAudio:    true,
		SupportsVideo:    true,
		MaxTextLength:    2048,
		MaxFileSizeMB:    20,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 10, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: true, MaxSizeMB: 20, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为文件名、大小和原始链接摘要"},
			{Type: "audio", Supported: true, MaxSizeMB: 2, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为音频文件名、大小和原始链接摘要"},
			{Type: "video", Supported: true, MaxSizeMB: 10, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为视频文件名、大小和原始链接摘要"},
		},
		Notes: []string{"应用消息媒体需先上传临时素材，再用 media_id 发送。"},
	}
}

// ValidateConfig 校验必要配置。
func (s *AppSink) ValidateConfig(config map[string]any) error {
	if corpid, _ := config["corpid"].(string); corpid == "" {
		return fmt.Errorf("wecom_app 缺少 corpid")
	}
	if agentid, _ := config["agentid"].(string); agentid == "" {
		if _, ok := config["agentid"].(float64); !ok {
			return fmt.Errorf("wecom_app 缺少 agentid")
		}
	}
	return nil
}

func (s *AppSink) corpID(sink *domainsink.Sink) string {
	v, _ := sink.Config["corpid"].(string)
	return v
}

func (s *AppSink) agentID(sink *domainsink.Sink) string {
	switch v := sink.Config["agentid"].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%d", int64(v))
	}
	return ""
}

func (s *AppSink) toUser(sink *domainsink.Sink) string {
	if v, _ := sink.Config["touser"].(string); v != "" {
		return v
	}
	return "@all"
}

// getToken 返回有效的 access_token，必要时刷新并缓存。forceRefresh 强制重新获取。
func (s *AppSink) getToken(ctx context.Context, corpid, secret string, forceRefresh bool) (string, error) {
	tokenMu.Lock()
	if !forceRefresh {
		if c, ok := tokenCache[corpid]; ok && time.Now().Before(c.expires) {
			tok := c.token
			tokenMu.Unlock()
			return tok, nil
		}
	}
	tokenMu.Unlock()

	url := fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", apiBase, corpid, secret)
	resp, err := s.client.Get(ctx, url, nil)
	if err != nil {
		return "", err
	}
	var tr struct {
		apiResp
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(resp.Body, &tr); err != nil {
		return "", fmt.Errorf("解析 access_token 响应失败: %w", err)
	}
	if tr.ErrCode != 0 || tr.AccessToken == "" {
		return "", fmt.Errorf("获取 access_token 失败 errcode=%d errmsg=%s", tr.ErrCode, tr.ErrMsg)
	}

	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 7200 * time.Second
	}
	tokenMu.Lock()
	tokenCache[corpid] = &cachedToken{token: tr.AccessToken, expires: time.Now().Add(ttl - tokenExpirySafety)}
	tokenMu.Unlock()
	return tr.AccessToken, nil
}

// Send 通过应用消息接口发送。access_token 失效时刷新并重试一次。
func (s *AppSink) Send(ctx context.Context, sink *domainsink.Sink, payload pluginsink.Payload, _ pluginsink.Options) (*pluginsink.Result, error) {
	corpid := s.corpID(sink)
	agentid := s.agentID(sink)
	secret := string(sink.Secret)
	if corpid == "" || agentid == "" || secret == "" {
		return failResult(nil, "wecom_app 配置不完整：corpid/agentid/secret 必填"), nil
	}

	bucket := "wecom_app:" + corpid + ":" + agentid
	if err := waitRate(ctx, bucket); err != nil {
		return nil, err
	}

	result, tokenExpired, err := s.trySend(ctx, sink, corpid, agentid, secret, payload, false)
	if err != nil {
		return failResult(nil, err.Error()), err
	}
	if tokenExpired {
		// 强制刷新 token 后重试一次。
		result, _, err = s.trySend(ctx, sink, corpid, agentid, secret, payload, true)
		if err != nil {
			return failResult(nil, err.Error()), err
		}
	}
	return result, nil
}

// trySend 发送一次；返回结果、是否因 token 失效需要重试、以及网络错误。
func (s *AppSink) trySend(ctx context.Context, sink *domainsink.Sink, corpid, agentid, secret string, payload pluginsink.Payload, forceRefresh bool) (*pluginsink.Result, bool, error) {
	token, err := s.getToken(ctx, corpid, secret, forceRefresh)
	if err != nil {
		return nil, false, err
	}

	msgType := msgTypeFor(payload.Format)
	body := map[string]any{
		"touser":  s.toUser(sink),
		"msgtype": msgType,
		"agentid": agentid,
	}
	if msgType == "markdown" {
		body["markdown"] = textContent{Content: payload.Text}
	} else {
		body["text"] = textContent{Content: payload.Text}
	}

	url := apiBase + "/message/send?access_token=" + token
	resp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return nil, false, err
	}
	summary, ok, expired, errMsg := parseResp(resp.Body)
	if ok {
		return &pluginsink.Result{Success: true, ResponseSummary: summary}, false, nil
	}
	if expired && !forceRefresh {
		return failResult(summary, errMsg), true, nil
	}
	return failResult(summary, errMsg), false, nil
}
