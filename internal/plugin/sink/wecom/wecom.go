// Package wecom 实现企业微信 Sink：群机器人（wecom_bot）与应用消息（wecom_app）。
//
// 两种子类型共享响应解析与限流；access_token 缓存与限流器为包级、按渠道标识分桶，
// 以便在 worker 每次投递重建插件实例时仍能复用。
package wecom

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/time/rate"

	domainmessage "telegram-message-forward/internal/domain/message"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

// apiBase 是企业微信 API 根地址。声明为 var 以便测试注入 mock server。
var apiBase = "https://qyapi.weixin.qq.com/cgi-bin"

// tokenExpirySafety 提前 60s 视为过期，避免边界失效。
const tokenExpirySafety = 60 * time.Second

// apiResp 是企业微信通用响应。
type apiResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// textPayload / markdownPayload 是消息体。
type textContent struct {
	Content string `json:"content"`
}

type imageContent struct {
	Base64  string `json:"base64,omitempty"`
	MD5     string `json:"md5,omitempty"`
	MediaID string `json:"media_id,omitempty"`
}

type fileContent struct {
	MediaID string `json:"media_id"`
}

// buildMessage 根据渲染格式选择 msgtype，返回 (msgtype, contentField)。
func msgTypeFor(format string) string {
	if format == "markdown" {
		return "markdown"
	}
	return "text"
}

// parseResp 解析响应并判断成功（errcode==0）。返回脱敏摘要与是否 token 失效。
func parseResp(body []byte) (summary []byte, ok bool, tokenExpired bool, errMsg string) {
	var r apiResp
	if err := json.Unmarshal(body, &r); err != nil {
		// 无法解析时保留原始（截断）body 作为摘要。
		return truncate(body, 512), false, false, "响应解析失败: " + err.Error()
	}
	summary, _ = json.Marshal(map[string]any{"errcode": r.ErrCode, "errmsg": r.ErrMsg})
	if r.ErrCode == 0 {
		return summary, true, false, ""
	}
	// 42001: access_token 过期；40014: 不合法的 access_token。
	expired := r.ErrCode == 42001 || r.ErrCode == 40014
	return summary, false, expired, fmt.Sprintf("企业微信返回错误 errcode=%d errmsg=%s", r.ErrCode, r.ErrMsg)
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

// --- 包级限流器：按渠道标识分桶 ---

var (
	limiterMu sync.Mutex
	limiters  = map[string]*rate.Limiter{}
)

// limiterFor 返回某渠道的限流器（企业微信群机器人约 20 条/分）。
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

// waitRate 阻塞直到该渠道允许发送或 ctx 取消（触发限频时排队而非报错）。
func waitRate(ctx context.Context, key string) error {
	return limiterFor(key).Wait(ctx)
}

// failResult 构造失败结果。
func failResult(summary []byte, errMsg string) *pluginsink.Result {
	return &pluginsink.Result{Success: false, ResponseSummary: summary, Error: errMsg}
}

func firstLocalImage(payload pluginsink.Payload) (domainmessage.Media, bool) {
	for _, item := range payload.Media {
		if (item.Type == "photo" || item.Type == "image") && item.LocalPath != "" {
			return item, true
		}
	}
	return domainmessage.Media{}, false
}

func firstLocalFile(payload pluginsink.Payload) (domainmessage.Media, bool) {
	for _, item := range payload.Media {
		if (item.Type == "file" || item.Type == "document") && item.LocalPath != "" {
			return item, true
		}
	}
	return domainmessage.Media{}, false
}

func fallbackText(payload pluginsink.Payload) string {
	if payload.FallbackText != "" {
		return payload.FallbackText
	}
	return payload.Text
}

func imageBase64MD5(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	h := md5.New()
	data, err := io.ReadAll(io.TeeReader(f, h))
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(data), fmt.Sprintf("%x", h.Sum(nil)), nil
}
