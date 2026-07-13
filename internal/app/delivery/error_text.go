package delivery

import (
	"strings"
)

// HumanizeError 把投递 last_error / attempt error 转为更易读的中文说明。
// 保留原始错误片段，便于排障；无法识别时原样返回。
func HumanizeError(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "sink is disabled") || strings.Contains(raw, "渠道已禁用") || strings.Contains(raw, "Sink 已禁用"):
		return "目标渠道已禁用，任务已取消。请启用渠道后重试。"
	case strings.Contains(raw, "公网") && (strings.Contains(raw, "URL") || strings.Contains(raw, "url")):
		return "媒体需要公网 URL 但未配置，已降级或失败。请在「设置 → 媒体存储」配置公网访问地址或 S3。原始：" + raw
	case strings.Contains(lower, "public_base_url") || strings.Contains(lower, "public url"):
		return "缺少媒体公网访问地址。请在设置页配置 public_base_url 或 S3。原始：" + raw
	case strings.Contains(raw, "降级") || strings.Contains(lower, "degrad"):
		return "媒体已降级处理：" + raw
	case strings.Contains(lower, "timeout") || strings.Contains(raw, "超时"):
		return "下游请求超时，可稍后重试。原始：" + raw
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host"):
		return "无法连接下游服务，请检查地址与网络。原始：" + raw
	case strings.Contains(lower, "unauthorized") || strings.Contains(raw, "401") || strings.Contains(lower, "invalid token"):
		return "下游鉴权失败，请检查渠道密钥/Token。原始：" + raw
	case strings.Contains(raw, "配置") && (strings.Contains(raw, "缺少") || strings.Contains(raw, "无效")):
		return "渠道配置不完整或无效：" + raw
	case strings.Contains(lower, "rate limit") || strings.Contains(raw, "限流") || strings.Contains(raw, "FLOOD"):
		return "触发下游限流，请稍后重试。原始：" + raw
	case strings.Contains(raw, "本地文件") || strings.Contains(lower, "local path") || strings.Contains(lower, "no such file"):
		return "本地媒体文件缺失或不可读，可能已清理。原始：" + raw
	case strings.Contains(lower, "context deadline") || strings.Contains(lower, "deadline exceeded"):
		return "请求超时（含 AI Provider 或下游），请检查网络与超时配置。原始：" + raw
	case strings.Contains(lower, "api key") || strings.Contains(lower, "incorrect api key") || strings.Contains(raw, "API Key"):
		return "AI Provider 鉴权失败，请检查 API Key。原始：" + raw
	case strings.Contains(lower, "insufficient_quota") || strings.Contains(lower, "quota") || strings.Contains(raw, "余额"):
		return "AI Provider 额度不足或账单异常。原始：" + raw
	case strings.Contains(raw, "同一 AI Profile 已有运行中"):
		return "同一整理任务已有运行中的实例，请等待完成或取消后再试。"
	default:
		return raw
	}
}
