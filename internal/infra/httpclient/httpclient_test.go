package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestClientErrorRedactsURLQuery(t *testing.T) {
	hc := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errors.New("connection refused")}
	})}
	client := New(WithHTTPClient(hc))
	_, err := client.PostJSON(context.Background(), "https://example.com/hook?key=SECRET_TOKEN", nil, nil)
	if err == nil {
		t.Fatal("请求应失败")
	}
	text := err.Error()
	if strings.Contains(text, "SECRET_TOKEN") || strings.Contains(text, "?key=") {
		t.Fatalf("错误不应包含 URL query: %s", text)
	}
	if !strings.Contains(text, "https://example.com/hook") {
		t.Fatalf("错误应保留非敏感目标信息: %s", text)
	}
}
