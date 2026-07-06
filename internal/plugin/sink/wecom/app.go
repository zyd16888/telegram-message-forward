package wecom

import (
	"context"
	"crypto/sha256"
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

const appTemporaryImageMaxBytes int64 = 10 * 1024 * 1024

// --- 包级 access_token 缓存：按企业应用凭据分桶 ---

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
			{
				Key:     "debug",
				Label:   "调试模式",
				Type:    formschema.FieldBoolean,
				Default: false,
				Help:    "开启后企业微信相关请求会追加 debug=1 参数。",
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
		MaxTextLength:    2048,
		MaxFileSizeMB:    20,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 10, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: true, MaxSizeMB: 20, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为文件名、大小和原始链接摘要"},
			{Type: "audio", Supported: true, MaxSizeMB: 20, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media_as_file", Fallback: "无本地文件时降级为音频文件名、大小和原始链接摘要"},
			{Type: "video", Supported: false, Fallback: "官方支持 video 素材，但当前内置实现未接入，降级为视频文件名、大小和原始链接摘要"},
		},
		Notes: []string{"应用消息媒体需先上传临时素材，再用 media_id 发送；音频按普通文件上传发送，视频原生发送后置。"},
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

func tokenCacheKey(corpid, agentid, secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return fmt.Sprintf("%s:%s:%x", corpid, agentid, sum)
}

// getToken 返回有效的 access_token，必要时刷新并缓存。forceRefresh 强制重新获取。
func (s *AppSink) getToken(ctx context.Context, corpid, agentid, secret string, debug bool, forceRefresh bool) (string, error) {
	cacheKey := tokenCacheKey(corpid, agentid, secret)
	tokenMu.Lock()
	if !forceRefresh {
		if c, ok := tokenCache[cacheKey]; ok && time.Now().Before(c.expires) {
			tok := c.token
			tokenMu.Unlock()
			return tok, nil
		}
	}
	tokenMu.Unlock()

	url := withDebugParam(fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", apiBase, corpid, secret), debug)
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
	tokenCache[cacheKey] = &cachedToken{token: tr.AccessToken, expires: time.Now().Add(ttl - tokenExpirySafety)}
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
	debug := debugEnabled(sink.Config)
	token, err := s.getToken(ctx, corpid, agentid, secret, debug, forceRefresh)
	if err != nil {
		return nil, false, err
	}

	if img, ok := firstLocalImage(payload, appTemporaryImageMaxBytes); ok {
		mediaID, summary, err := s.uploadMedia(ctx, token, "image", img.LocalPath, img.FileName, debug)
		if err != nil {
			return failResult(summary, err.Error()), false, err
		}
		res, expired, err := s.sendAppImage(ctx, sink, token, agentid, mediaID)
		if err != nil || expired || res == nil || !res.Success {
			return res, expired, err
		}
		return s.sendAppTextAfterMedia(ctx, sink, token, agentid, payload, res)
	}
	if file, ok := firstLocalFile(payload); ok {
		mediaID, summary, err := s.uploadMedia(ctx, token, "file", file.LocalPath, file.FileName, debug)
		if err != nil {
			return failResult(summary, err.Error()), false, err
		}
		res, expired, err := s.sendAppFile(ctx, sink, token, agentid, mediaID)
		if err != nil || expired || res == nil || !res.Success {
			return res, expired, err
		}
		return s.sendAppTextAfterMedia(ctx, sink, token, agentid, payload, res)
	}
	if len(payload.Media) > 0 {
		payload.Format = "text"
		payload.Text = fallbackText(payload)
	}
	return s.sendAppText(ctx, sink, token, agentid, payload)
}

func (s *AppSink) sendAppTextAfterMedia(ctx context.Context, sink *domainsink.Sink, token, agentid string, payload pluginsink.Payload, mediaResult *pluginsink.Result) (*pluginsink.Result, bool, error) {
	if payload.Text == "" {
		return mediaResult, false, nil
	}
	return s.sendAppText(ctx, sink, token, agentid, payload)
}

func (s *AppSink) sendAppText(ctx context.Context, sink *domainsink.Sink, token, agentid string, payload pluginsink.Payload) (*pluginsink.Result, bool, error) {
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

	url := withDebugParam(apiBase+"/message/send?access_token="+token, debugEnabled(sink.Config))
	resp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return nil, false, err
	}
	summary, ok, expired, errMsg := parseResp(resp.Body)
	if ok {
		return &pluginsink.Result{Success: true, ResponseSummary: summary}, false, nil
	}
	if expired {
		return failResult(summary, errMsg), true, nil
	}
	return failResult(summary, errMsg), false, nil
}

func (s *AppSink) uploadMedia(ctx context.Context, token, mediaType, path, fileName string, debug bool) (string, []byte, error) {
	url := withDebugParam(fmt.Sprintf("%s/media/upload?access_token=%s&type=%s", apiBase, token, mediaType), debug)
	resp, err := s.client.PostMultipartFileNamed(ctx, url, "media", path, fileName, nil)
	if err != nil {
		return "", nil, err
	}
	var r struct {
		apiResp
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return "", truncate(resp.Body, 512), fmt.Errorf("解析上传响应失败: %w", err)
	}
	summary, _ := json.Marshal(map[string]any{"errcode": r.ErrCode, "errmsg": r.ErrMsg, "has_media_id": r.MediaID != ""})
	if r.ErrCode != 0 || r.MediaID == "" {
		return "", summary, fmt.Errorf("上传临时素材失败 errcode=%d errmsg=%s", r.ErrCode, r.ErrMsg)
	}
	return r.MediaID, summary, nil
}

func (s *AppSink) sendAppFile(ctx context.Context, sink *domainsink.Sink, token, agentid, mediaID string) (*pluginsink.Result, bool, error) {
	body := map[string]any{
		"touser":  s.toUser(sink),
		"msgtype": "file",
		"agentid": agentid,
		"file":    fileContent{MediaID: mediaID},
	}
	url := withDebugParam(apiBase+"/message/send?access_token="+token, debugEnabled(sink.Config))
	resp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return nil, false, err
	}
	summary, ok, expired, errMsg := parseResp(resp.Body)
	if ok {
		return &pluginsink.Result{Success: true, ResponseSummary: summary}, false, nil
	}
	return failResult(summary, errMsg), expired, nil
}

func (s *AppSink) sendAppImage(ctx context.Context, sink *domainsink.Sink, token, agentid, mediaID string) (*pluginsink.Result, bool, error) {
	body := map[string]any{
		"touser":  s.toUser(sink),
		"msgtype": "image",
		"agentid": agentid,
		"image":   imageContent{MediaID: mediaID},
	}
	url := withDebugParam(apiBase+"/message/send?access_token="+token, debugEnabled(sink.Config))
	resp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return nil, false, err
	}
	summary, ok, expired, errMsg := parseResp(resp.Body)
	if ok {
		return &pluginsink.Result{Success: true, ResponseSummary: summary}, false, nil
	}
	return failResult(summary, errMsg), expired, nil
}
