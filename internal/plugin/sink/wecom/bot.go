package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
		SupportsImage:    true,
		SupportsFile:     true,
		MaxTextLength:    4096,
		MaxFileSizeMB:    20,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 2, SupportsPublicURL: false, RequiresUpload: false, SupportsBinary: true, DeliveryMode: "base64_md5", Fallback: "降级为 [图片消息] + caption + 原始链接"},
			{Type: "file", Supported: true, MaxSizeMB: 20, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media", Fallback: "降级为文件名、大小和原始链接摘要"},
			{Type: "audio", Supported: true, MaxSizeMB: 20, SupportsPublicURL: false, RequiresUpload: true, SupportsBinary: true, DeliveryMode: "upload_media_as_file", Fallback: "无本地文件时降级为音频文件名、大小和原始链接摘要"},
			{Type: "video", Supported: false, Fallback: "降级为视频文件名、大小和原始链接摘要"},
		},
		Notes: []string{"群机器人图片使用 base64 + md5；文件需先上传获取 media_id；音频按普通文件上传发送。"},
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
	debug := debugEnabled(sink.Config)
	if url, _ := sink.Config["webhook_url"].(string); url != "" {
		return withDebugParam(url, debug), url, nil
	}
	key := string(sink.Secret)
	if key == "" {
		return "", "", fmt.Errorf("wecom_bot 缺少 webhook key（secret）或 webhook_url")
	}
	return withDebugParam(apiBase+"/webhook/send?key="+key, debug), "wecom_bot:" + key, nil
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

	if img, ok := firstLocalImage(payload); ok {
		res, err := s.sendImage(ctx, url, img.LocalPath)
		if err != nil || res == nil || !res.Success {
			return res, err
		}
		return s.sendTextAfterMedia(ctx, url, payload, res)
	}
	if file, ok := firstLocalFile(payload); ok {
		res, err := s.sendFile(ctx, url, file.LocalPath, file.FileName)
		if err != nil || res == nil || !res.Success {
			return res, err
		}
		return s.sendTextAfterMedia(ctx, url, payload, res)
	}
	if len(payload.Media) > 0 {
		payload.Format = "text"
		payload.Text = fallbackText(payload)
	}
	return s.sendText(ctx, url, payload)
}

func (s *BotSink) sendText(ctx context.Context, url string, payload pluginsink.Payload) (*pluginsink.Result, error) {
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

func (s *BotSink) sendTextAfterMedia(ctx context.Context, url string, payload pluginsink.Payload, mediaResult *pluginsink.Result) (*pluginsink.Result, error) {
	if payload.Text == "" {
		return mediaResult, nil
	}
	return s.sendText(ctx, url, payload)
}

// botUploadURL 由 webhook send 地址推导 upload_media 地址（沿用同一个 key）。
func botUploadURL(sendURL string) (string, error) {
	if !strings.Contains(sendURL, "/webhook/send") {
		return "", fmt.Errorf("无法从 webhook 地址推导 upload_media 地址，请使用标准 /webhook/send 地址")
	}
	uploadURL := strings.Replace(sendURL, "/webhook/send", "/webhook/upload_media", 1)
	sep := "&"
	if !strings.Contains(uploadURL, "?") {
		sep = "?"
	}
	return uploadURL + sep + "type=file", nil
}

// sendFile 先上传文件获取 media_id，再按 msgtype=file 发送。
func (s *BotSink) sendFile(ctx context.Context, url string, path string, fileName string) (*pluginsink.Result, error) {
	uploadURL, err := botUploadURL(url)
	if err != nil {
		return failResult(nil, err.Error()), nil
	}
	resp, err := s.client.PostMultipartFileNamed(ctx, uploadURL, "media", path, fileName, nil)
	if err != nil {
		return failResult(nil, "上传文件失败: "+err.Error()), err
	}
	var r struct {
		apiResp
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return failResult(truncate(resp.Body, 512), "解析上传响应失败: "+err.Error()), nil
	}
	if r.ErrCode != 0 || r.MediaID == "" {
		summary, _ := json.Marshal(map[string]any{"errcode": r.ErrCode, "errmsg": r.ErrMsg, "has_media_id": r.MediaID != ""})
		return failResult(summary, fmt.Sprintf("上传文件失败 errcode=%d errmsg=%s", r.ErrCode, r.ErrMsg)), nil
	}

	body := map[string]any{
		"msgtype": "file",
		"file":    fileContent{MediaID: r.MediaID},
	}
	sendResp, err := s.client.PostJSON(ctx, url, body, nil)
	if err != nil {
		return failResult(nil, err.Error()), err
	}
	summary, ok, _, errMsg := parseResp(sendResp.Body)
	if !ok {
		return failResult(summary, errMsg), nil
	}
	return &pluginsink.Result{Success: true, ResponseSummary: summary}, nil
}

func (s *BotSink) sendImage(ctx context.Context, url string, path string) (*pluginsink.Result, error) {
	b64, md5sum, err := imageBase64MD5(path)
	if err != nil {
		return failResult(nil, "读取图片失败: "+err.Error()), nil
	}
	body := map[string]any{
		"msgtype": "image",
		"image":   imageContent{Base64: b64, MD5: md5sum},
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
