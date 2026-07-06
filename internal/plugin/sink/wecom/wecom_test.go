package wecom

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
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

func TestBotSendLocalImage(t *testing.T) {
	img := t.TempDir() + "/image.jpg"
	if err := os.WriteFile(img, []byte("fake-image"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()

	s := NewBot()
	sink := &domainsink.Sink{Type: "wecom_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Media: []domainmessage.Media{{Type: "photo", LocalPath: img}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("图片应发送成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, `"msgtype":"image"`) || !strings.Contains(body, `"base64"`) || !strings.Contains(body, `"md5"`) {
		t.Fatalf("图片请求体不正确: %s", body)
	}
}

func TestBotSendLocalFile(t *testing.T) {
	pdf := t.TempDir() + "/report.pdf"
	if err := os.WriteFile(pdf, []byte("%PDF-fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	var uploadCalled, sendBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/webhook/upload_media"):
			if r.URL.Query().Get("type") != "file" {
				t.Errorf("upload type = %q, want file", r.URL.Query().Get("type"))
			}
			uploadCalled.Store(true)
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","type":"file","media_id":"MEDIA_FILE"}`)
		case strings.Contains(r.URL.Path, "/webhook/send"):
			b, _ := io.ReadAll(r.Body)
			sendBody.Store(string(b))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		default:
			t.Errorf("未预期请求: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	s := NewBot()
	sink := &domainsink.Sink{Type: "wecom_bot", Config: map[string]any{"webhook_url": srv.URL + "/cgi-bin/webhook/send?key=TEST"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Media: []domainmessage.Media{{Type: "document", FileName: "report.pdf", LocalPath: pdf}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("文件应发送成功: %+v", res)
	}
	if got, _ := uploadCalled.Load().(bool); !got {
		t.Fatal("应先调用 upload_media")
	}
	body, _ := sendBody.Load().(string)
	if !strings.Contains(body, `"msgtype":"file"`) || !strings.Contains(body, "MEDIA_FILE") {
		t.Fatalf("文件请求体不正确: %s", body)
	}
}

func TestBotDebugParam(t *testing.T) {
	pdf := t.TempDir() + "/report.pdf"
	if err := os.WriteFile(pdf, []byte("%PDF-fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	var uploadDebug, sendDebug atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/webhook/upload_media"):
			uploadDebug.Store(r.URL.Query().Get("debug"))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","type":"file","media_id":"MEDIA_FILE"}`)
		case strings.Contains(r.URL.Path, "/webhook/send"):
			sendDebug.Store(r.URL.Query().Get("debug"))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		default:
			t.Errorf("未预期请求: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	s := NewBot()
	sink := &domainsink.Sink{Type: "wecom_bot", Config: map[string]any{"webhook_url": srv.URL + "/cgi-bin/webhook/send?key=TEST", "debug": true}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "文件",
		Media: []domainmessage.Media{{Type: "document", LocalPath: pdf}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("开启 debug 后应成功: %+v", res)
	}
	if got, _ := sendDebug.Load().(string); got != "1" {
		t.Fatalf("send debug = %q, want 1", got)
	}
	if got, _ := uploadDebug.Load().(string); got != "1" {
		t.Fatalf("upload debug = %q, want 1", got)
	}
}

func TestBotSendFileUploadURLUnderivable(t *testing.T) {
	pdf := t.TempDir() + "/report.pdf"
	if err := os.WriteFile(pdf, []byte("%PDF-fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()

	s := NewBot()
	// webhook_url 不含 /webhook/send，无法推导 upload_media 地址，应返回可解释错误。
	sink := &domainsink.Sink{Type: "wecom_bot", Config: map[string]any{"webhook_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Media: []domainmessage.Media{{Type: "document", LocalPath: pdf}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || !strings.Contains(res.Error, "upload_media") {
		t.Fatalf("应失败且错误可解释: %+v", res)
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

func TestAppDebugParam(t *testing.T) {
	tokenMu.Lock()
	tokenCache = map[string]*cachedToken{}
	tokenMu.Unlock()

	img := t.TempDir() + "/image.jpg"
	if err := os.WriteFile(img, []byte("fake-image"), 0o644); err != nil {
		t.Fatal(err)
	}

	var tokenDebug, uploadDebug, sendDebug atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gettoken"):
			tokenDebug.Store(r.URL.Query().Get("debug"))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","access_token":"TOK","expires_in":7200}`)
		case strings.Contains(r.URL.Path, "/media/upload"):
			uploadDebug.Store(r.URL.Query().Get("debug"))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","media_id":"MEDIA_ID"}`)
		case strings.Contains(r.URL.Path, "/message/send"):
			sendDebug.Store(r.URL.Query().Get("debug"))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		default:
			t.Errorf("未预期请求: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	apiBase = srv.URL
	defer func() { apiBase = "https://qyapi.weixin.qq.com/cgi-bin" }()

	s := NewApp()
	sink := &domainsink.Sink{
		Type:   "wecom_app",
		Config: map[string]any{"corpid": "corp1", "agentid": "1000002", "debug": true},
		Secret: []byte("secret1"),
	}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "图片",
		Media: []domainmessage.Media{{Type: "image", LocalPath: img}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("开启 debug 后应成功: %+v", res)
	}
	if got, _ := tokenDebug.Load().(string); got != "1" {
		t.Fatalf("gettoken debug = %q, want 1", got)
	}
	if got, _ := uploadDebug.Load().(string); got != "1" {
		t.Fatalf("upload debug = %q, want 1", got)
	}
	if got, _ := sendDebug.Load().(string); got != "1" {
		t.Fatalf("send debug = %q, want 1", got)
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

func TestAppSendLocalImageUploadsMedia(t *testing.T) {
	tokenMu.Lock()
	tokenCache = map[string]*cachedToken{}
	tokenMu.Unlock()

	img := t.TempDir() + "/image.jpg"
	if err := os.WriteFile(img, []byte("fake-image"), 0o644); err != nil {
		t.Fatal(err)
	}

	var uploadCalled, imageSent atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gettoken"):
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","access_token":"TOK","expires_in":7200}`)
		case strings.Contains(r.URL.Path, "/media/upload"):
			uploadCalled.Store(true)
			if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Errorf("应使用 multipart 上传，Content-Type=%s", r.Header.Get("Content-Type"))
			}
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","media_id":"MEDIA_ID"}`)
		case strings.Contains(r.URL.Path, "/message/send"):
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), `"msgtype":"image"`) && strings.Contains(string(b), "MEDIA_ID") {
				imageSent.Store(true)
			}
			io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		}
	}))
	defer srv.Close()

	apiBase = srv.URL
	defer func() { apiBase = "https://qyapi.weixin.qq.com/cgi-bin" }()

	s := NewApp()
	sink := &domainsink.Sink{
		Type:   "wecom_app",
		Config: map[string]any{"corpid": "corp1", "agentid": "1000002"},
		Secret: []byte("secret1"),
	}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Media: []domainmessage.Media{{Type: "image", LocalPath: img}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || !uploadCalled.Load() || !imageSent.Load() {
		t.Fatalf("图片上传/发送未完成 res=%+v upload=%v image=%v", res, uploadCalled.Load(), imageSent.Load())
	}
}

func TestAppSendLocalFileUploadsMedia(t *testing.T) {
	tokenMu.Lock()
	tokenCache = map[string]*cachedToken{}
	tokenMu.Unlock()

	pdf := t.TempDir() + "/report.pdf"
	if err := os.WriteFile(pdf, []byte("%PDF-fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	var uploadType atomic.Value
	var fileSent atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gettoken"):
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","access_token":"TOK","expires_in":7200}`)
		case strings.Contains(r.URL.Path, "/media/upload"):
			uploadType.Store(r.URL.Query().Get("type"))
			io.WriteString(w, `{"errcode":0,"errmsg":"ok","media_id":"MEDIA_FILE"}`)
		case strings.Contains(r.URL.Path, "/message/send"):
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), `"msgtype":"file"`) && strings.Contains(string(b), "MEDIA_FILE") {
				fileSent.Store(true)
			}
			io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		}
	}))
	defer srv.Close()

	apiBase = srv.URL
	defer func() { apiBase = "https://qyapi.weixin.qq.com/cgi-bin" }()

	s := NewApp()
	sink := &domainsink.Sink{
		Type:   "wecom_app",
		Config: map[string]any{"corpid": "corp1", "agentid": "1000002"},
		Secret: []byte("secret1"),
	}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "季度报告",
		Media: []domainmessage.Media{{Type: "document", FileName: "report.pdf", LocalPath: pdf}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || !fileSent.Load() {
		t.Fatalf("文件上传/发送未完成 res=%+v file=%v", res, fileSent.Load())
	}
	if got, _ := uploadType.Load().(string); got != "file" {
		t.Fatalf("上传素材类型 = %q, want file", got)
	}
}
