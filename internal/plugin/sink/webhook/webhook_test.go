package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

func TestSendIncludesPublicMediaMetadata(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := New()
	sink := &domainsink.Sink{Type: "webhook", Config: map[string]any{"url": srv.URL}}
	res, err := s.Send(context.Background(), sink, pluginsink.Payload{
		Text:         "hello",
		Format:       "text",
		FallbackText: "hello\n[图片消息]",
		Media: []domainmessage.Media{{
			Type:           "image",
			FileName:       "image.jpg",
			MimeType:       "image/jpeg",
			Size:           1024,
			Width:          640,
			Height:         480,
			Caption:        "caption",
			LocalPath:      "C:/tmp/private/image.jpg",
			StorageKey:     "telegram/source_1/1_0.jpg",
			DownloadStatus: "downloaded",
		}},
	}, pluginsink.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	for _, want := range []string{`"media"`, `"file_name":"image.jpg"`, `"download_status":"downloaded"`, `"fallback_text"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("请求体缺少 %s: %s", want, body)
		}
	}
	for _, leak := range []string{"local_path", "storage_key", "C:/tmp/private", "telegram/source_1"} {
		if strings.Contains(body, leak) {
			t.Fatalf("Webhook 不应外发本地路径/存储键 %s: %s", leak, body)
		}
	}
}
