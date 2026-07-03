package dingtalk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func TestSendSuccessWithAccessToken(t *testing.T) {
	var gotPath atomic.Value
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.String())
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()
	apiBase = srv.URL + "/robot/send"
	defer func() { apiBase = "https://oapi.dingtalk.com/robot/send" }()

	s := New()
	sink := &domainsink.Sink{Type: "dingtalk_bot", Config: map[string]any{"access_token": "TOKEN123"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "hello dingtalk"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	path, _ := gotPath.Load().(string)
	if !strings.Contains(path, "access_token=TOKEN123") {
		t.Fatalf("请求地址应带 access_token: %s", path)
	}
	if strings.Contains(path, "sign=") {
		t.Fatalf("未配置加签密钥时不应出现 sign 参数: %s", path)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msgtype":"text"`) || !strings.Contains(body, "hello dingtalk") {
		t.Fatalf("请求体不正确: %s", body)
	}
}

func TestSendErrCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errcode":300001,"errmsg":"keyword not in content"}`)
	}))
	defer srv.Close()
	apiBase = srv.URL
	defer func() { apiBase = "https://oapi.dingtalk.com/robot/send" }()

	s := New()
	sink := &domainsink.Sink{Type: "dingtalk_bot", Config: map[string]any{"access_token": "TOKEN123"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "markdown", Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success {
		t.Fatal("errcode!=0 应失败")
	}
	if !strings.Contains(res.Error, "300001") {
		t.Fatalf("错误信息应含 errcode: %s", res.Error)
	}
}

func TestSendWithSignatureAppendsValidSign(t *testing.T) {
	var gotQuery atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery.Store(r.URL.Query())
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()
	apiBase = srv.URL
	defer func() { apiBase = "https://oapi.dingtalk.com/robot/send" }()

	const secret = "SECtestsecret123"
	s := New()
	sink := &domainsink.Sink{
		Type:   "dingtalk_bot",
		Config: map[string]any{"access_token": "TOKEN123"},
		Secret: []byte(secret),
	}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "signed"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}

	q, _ := gotQuery.Load().(url.Values)
	tsStr := q.Get("timestamp")
	gotSign := q.Get("sign")
	if tsStr == "" || gotSign == "" {
		t.Fatalf("加签请求应带 timestamp 和 sign: %+v", q)
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		t.Fatalf("timestamp 不是合法数字: %s", tsStr)
	}
	// 按钉钉官方算法在测试里独立重新计算一遍签名，交叉验证实现正确性。
	stringToSign := fmt.Sprintf("%d\n%s", ts, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	wantSign := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if gotSign != wantSign {
		t.Fatalf("签名不匹配:\n got  %s\n want %s", gotSign, wantSign)
	}
}

func TestSendMarkdownUsesSinkNameAsTitle(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()
	apiBase = srv.URL
	defer func() { apiBase = "https://oapi.dingtalk.com/robot/send" }()

	s := New()
	sink := &domainsink.Sink{Type: "dingtalk_bot", Name: "技术群通知", Config: map[string]any{"access_token": "TOKEN123"}}
	if _, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "markdown", Text: "# hi"}, pluginsink.Options{}); err != nil {
		t.Fatal(err)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"title":"技术群通知"`) {
		t.Fatalf("markdown title 应使用 sink 名称: %s", body)
	}
}

func TestSendImageWithPublicURLUsesMarkdown(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()
	apiBase = srv.URL
	defer func() { apiBase = "https://oapi.dingtalk.com/robot/send" }()

	s := New()
	sink := &domainsink.Sink{Type: "dingtalk_bot", Name: "图片通知", Config: map[string]any{"access_token": "TOKEN123"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Format: "text",
		Text:   "hello",
		Media:  []domainmessage.Media{{Type: "image", RemoteURL: "https://example.com/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msgtype":"markdown"`) || !strings.Contains(body, "![image](https://example.com/a.jpg)") {
		t.Fatalf("公网图片应以 markdown 图片投递: %s", body)
	}
}

func TestSendLocalImageFallsBackToText(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()
	apiBase = srv.URL
	defer func() { apiBase = "https://oapi.dingtalk.com/robot/send" }()

	s := New()
	sink := &domainsink.Sink{Type: "dingtalk_bot", Config: map[string]any{"access_token": "TOKEN123"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Format:       "text",
		Text:         "hello",
		FallbackText: "hello\n[图片消息] image.jpg",
		Media:        []domainmessage.Media{{Type: "image", LocalPath: "C:/tmp/image.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msgtype":"text"`) || !strings.Contains(body, "[图片消息] image.jpg") {
		t.Fatalf("本地图片应降级为文本: %s", body)
	}
	if strings.Contains(body, "![image]") {
		t.Fatalf("本地图片不应生成 markdown 图片: %s", body)
	}
}

func TestValidateConfigRequiresTokenOrURL(t *testing.T) {
	s := New()
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少 access_token 与 webhook_url 应报错")
	}
	if err := s.ValidateConfig(map[string]any{"access_token": "x"}); err != nil {
		t.Fatalf("有 access_token 不应报错: %v", err)
	}
	if err := s.ValidateConfig(map[string]any{"webhook_url": "https://example.com"}); err != nil {
		t.Fatalf("有 webhook_url 不应报错: %v", err)
	}
}

func TestSendMissingTokenFails(t *testing.T) {
	s := New()
	sink := &domainsink.Sink{Type: "dingtalk_bot", Config: map[string]any{}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success {
		t.Fatal("缺少 access_token/webhook_url 应失败")
	}
}
