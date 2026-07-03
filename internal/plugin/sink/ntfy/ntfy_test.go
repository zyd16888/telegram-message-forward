package ntfy

import (
	"context"
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

func TestSendTextSuccess(t *testing.T) {
	var gotBody atomic.Value
	var gotMarkdown atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		gotMarkdown.Store(r.Header.Get("Markdown"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "ntfy", Config: map[string]any{"topic_url": srv.URL, "title": "TMF"}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Format: "markdown", Text: "**hello**"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	if gotBody.Load() != "**hello**" || gotMarkdown.Load() != "yes" {
		t.Fatalf("请求不正确 body=%v markdown=%v", gotBody.Load(), gotMarkdown.Load())
	}
}

func TestSendRemoteMediaUsesAttachHeader(t *testing.T) {
	var gotAttach atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAttach.Store(r.Header.Get("Attach"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "ntfy", Config: map[string]any{"topic_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "hello",
		Media: []domainmessage.Media{{Type: "image", RemoteURL: "https://example.com/a.jpg"}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || gotAttach.Load() != "https://example.com/a.jpg" {
		t.Fatalf("远程媒体应使用 Attach header res=%+v attach=%v", res, gotAttach.Load())
	}
}

func TestSendLocalAttachment(t *testing.T) {
	file := t.TempDir() + "/a.txt"
	if err := os.WriteFile(file, []byte("attachment"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var gotFilename atomic.Value
	var gotUploadBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 2 {
			b, _ := io.ReadAll(r.Body)
			gotUploadBody.Store(string(b))
			gotFilename.Store(r.Header.Get("Filename"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "ntfy", Config: map[string]any{"topic_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:  "hello",
		Media: []domainmessage.Media{{Type: "file", FileName: "a.txt", LocalPath: file}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || calls.Load() != 2 {
		t.Fatalf("应先发文本再发附件 res=%+v calls=%d", res, calls.Load())
	}
	if gotFilename.Load() != "a.txt" || gotUploadBody.Load() != "attachment" {
		t.Fatalf("附件请求不正确 filename=%v body=%v", gotFilename.Load(), gotUploadBody.Load())
	}
}

func TestSendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "ntfy", Config: map[string]any{"topic_url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{Text: "x"}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || !strings.Contains(res.Error, "400") {
		t.Fatalf("非 2xx 应失败: %+v", res)
	}
}

func TestValidateConfig(t *testing.T) {
	s := New()
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少 topic_url 应报错")
	}
	if err := s.ValidateConfig(map[string]any{"topic_url": "https://ntfy.sh/t"}); err != nil {
		t.Fatalf("有 topic_url 不应报错: %v", err)
	}
}
