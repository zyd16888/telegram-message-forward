package sink

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	domainsink "telegram-message-forward/internal/domain/sink"
	_ "telegram-message-forward/internal/plugin/sink/webhook"
)

type fakeSinkRepo struct {
	item            *domainsink.Sink
	lastTestUpdated bool
}

func (r *fakeSinkRepo) Create(_ context.Context, s *domainsink.Sink) error {
	r.item = s
	return nil
}

func (r *fakeSinkRepo) Update(_ context.Context, s *domainsink.Sink) error {
	r.item = s
	return nil
}

func (r *fakeSinkRepo) GetByID(_ context.Context, _ int64) (*domainsink.Sink, error) {
	return r.item, nil
}

func (r *fakeSinkRepo) List(context.Context) ([]*domainsink.Sink, error) {
	if r.item == nil {
		return nil, nil
	}
	return []*domainsink.Sink{r.item}, nil
}

func (r *fakeSinkRepo) Delete(context.Context, int64) error {
	r.item = nil
	return nil
}

func (r *fakeSinkRepo) UpdateTestResult(_ context.Context, _ int64, at time.Time, success bool, errText string) error {
	r.lastTestUpdated = true
	r.item.Observability.LastTestAt = &at
	r.item.Observability.LastTestSuccess = success
	r.item.Observability.LastTestError = errText
	return nil
}

func TestServiceTestUnsavedWebhook(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody.Store(string(body))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	svc := NewService(&fakeSinkRepo{})
	result, err := svc.Test(context.Background(), TestInput{
		Type:   "webhook",
		Config: map[string]any{"url": srv.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("测试消息应投递成功: %+v", result)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, "渠道测试消息") {
		t.Fatalf("测试请求体未包含测试消息: %s", body)
	}
}

func TestServiceTestExistingKeepsSecret(t *testing.T) {
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	repo := &fakeSinkRepo{item: &domainsink.Sink{
		ID:     1,
		Type:   "webhook",
		Config: map[string]any{"url": srv.URL},
		Secret: []byte("old-token"),
	}}
	svc := NewService(repo)
	result, err := svc.Test(context.Background(), TestInput{ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("测试消息应投递成功: %+v", result)
	}
	auth, _ := gotAuth.Load().(string)
	if auth != "Bearer old-token" {
		t.Fatalf("应复用已有密钥，实际 Authorization=%q", auth)
	}
	if !repo.lastTestUpdated || !repo.item.Observability.LastTestSuccess {
		t.Fatalf("应记录最近测试成功: %+v", repo.item.Observability)
	}
}

func TestTestMediaValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   TestMediaInput
	}{
		{name: "unsupported type", in: TestMediaInput{Type: "archive", URL: "https://example.com/a.zip"}},
		{name: "local file", in: TestMediaInput{Type: "file", URL: "file:///tmp/a.txt"}},
		{name: "credentials", in: TestMediaInput{Type: "image", URL: "https://user:pass@example.com/a.jpg"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := testMedia(tc.in); err == nil {
				t.Fatalf("testMedia(%+v) 应失败", tc.in)
			}
		})
	}
	media, err := testMedia(TestMediaInput{Type: " IMAGE ", URL: " https://example.com/a.jpg ", FileName: " a.jpg "})
	if err != nil {
		t.Fatal(err)
	}
	if media.Type != "image" || media.RemoteURL != "https://example.com/a.jpg" || media.FileName != "a.jpg" {
		t.Fatalf("测试媒体规范化结果不正确: %+v", media)
	}
}
