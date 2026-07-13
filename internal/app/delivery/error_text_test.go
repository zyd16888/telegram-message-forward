package delivery

import "testing"

func TestHumanizeError(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"渠道已禁用", "目标渠道已禁用，任务已取消。请启用渠道后重试。"},
		{"媒体因渠道不支持图片，已降级为文本摘要", "媒体已降级处理：媒体因渠道不支持图片，已降级为文本摘要"},
		{"something unknown", "something unknown"},
	}
	for _, tc := range cases {
		got := HumanizeError(tc.in)
		if got != tc.want {
			t.Fatalf("HumanizeError(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
	// 公网 URL 类错误应提示设置页。
	got := HumanizeError("钉钉需要公网 URL 才能发送图片")
	if !containsAll(got, "公网", "设置") {
		t.Fatalf("公网 URL 错误应提示设置: %q", got)
	}
	got = HumanizeError("无公网 URL，已降级为文本")
	if !containsAll(got, "公网", "设置") {
		t.Fatalf("无公网 URL 降级应提示设置: %q", got)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
