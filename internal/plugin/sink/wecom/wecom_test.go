package wecom

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func TestBotSendSuccess(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()

	s := NewBot()
	sink := &domainsink.Sink{Type: "wecom_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "hello wecom"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msgtype":"text"`) || !strings.Contains(body, "hello wecom") {
		t.Fatalf("请求体不正确: %s", body)
	}
}

func TestBotSendErrCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errcode":93000,"errmsg":"invalid webhook"}`)
	}))
	defer srv.Close()

	s := NewBot()
	sink := &domainsink.Sink{Type: "wecom_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "markdown", Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success {
		t.Fatal("errcode!=0 应失败")
	}
	if !strings.Contains(res.Error, "93000") {
		t.Fatalf("错误信息应含 errcode: %s", res.Error)
	}
}

func TestAppTokenCacheAndRefresh(t *testing.T) {
	// 重置包级缓存，避免与其它用例串扰。
	tokenMu.Lock()
	tokenCache = map[string]*cachedToken{}
	tokenMu.Unlock()

	var tokenCalls, sendCalls atomic.Int32
	var firstSend atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gettoken"):
			tokenCalls.Add(1)
			// 每次返回不同 token，便于断言刷新。
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","access_token":"TOK","expires_in":7200}`)
		case strings.Contains(r.URL.Path, "/message/send"):
			n := sendCalls.Add(1)
			b, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(b, &payload)
			if payload["agentid"] != "1000002" {
				t.Errorf("agentid 不正确: %v", payload["agentid"])
			}
			if payload["touser"] != "@all" {
				t.Errorf("touser 默认应为 @all: %v", payload["touser"])
			}
			// 第一次 send 模拟 token 过期，触发刷新重试。
			if n == 1 && !firstSend.Swap(true) {
				io.WriteString(w, `{"errcode":42001,"errmsg":"access_token expired"}`)
				return
			}
			io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		}
	}))
	defer srv.Close()

	apiBase = srv.URL // 注入 mock
	defer func() { apiBase = "https://qyapi.weixin.qq.com/cgi-bin" }()

	s := NewApp()
	sink := &domainsink.Sink{
		Type:   "wecom_app",
		Config: map[string]any{"corpid": "corp1", "agentid": "1000002"},
		Secret: []byte("secret1"),
	}

	// 第一次发送：token 过期 → 刷新重试 → 成功。
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "hi"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("刷新后应成功: %+v", res)
	}
	if sendCalls.Load() != 2 {
		t.Fatalf("应发送 2 次（过期+重试），实际 %d", sendCalls.Load())
	}

	// 第二次发送：token 已缓存，不应再次 gettoken（除非过期刷新）。
	tokBefore := tokenCalls.Load()
	res, err = s.Send(context.Background(), sink, pluginsink.Payload{Format: "text", Text: "hi2"}, pluginsink.Options{})
	if err != nil || !res.Success {
		t.Fatalf("第二次应成功: %+v err=%v", res, err)
	}
	if tokenCalls.Load() != tokBefore {
		t.Fatalf("token 应命中缓存，不应再次 gettoken（before=%d after=%d）", tokBefore, tokenCalls.Load())
	}
}

func TestAppMissingConfig(t *testing.T) {
	s := NewApp()
	sink := &domainsink.Sink{Type: "wecom_app", Config: map[string]any{"corpid": "c"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success {
		t.Fatal("缺少 agentid/secret 应失败")
	}
}
