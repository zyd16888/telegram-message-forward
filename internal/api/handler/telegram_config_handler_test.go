package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	appconfig "telegram-message-forward/internal/app/telegramconfig"
	domainconfig "telegram-message-forward/internal/domain/telegramconfig"
)

type proxyHandlerRepository struct {
	createCalls int
}

func (r *proxyHandlerRepository) Create(_ context.Context, _ *domainconfig.Proxy) error {
	r.createCalls++
	return nil
}

func (r *proxyHandlerRepository) Update(_ context.Context, _ *domainconfig.Proxy) error {
	return nil
}

func (r *proxyHandlerRepository) GetByID(_ context.Context, _ int64) (*domainconfig.Proxy, error) {
	return nil, nil
}

func (r *proxyHandlerRepository) List(_ context.Context) ([]*domainconfig.Proxy, error) {
	return nil, nil
}

func (r *proxyHandlerRepository) Delete(_ context.Context, _ int64) error {
	return nil
}

func TestCreateProxyRejectsUnsupportedTypeWithBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &proxyHandlerRepository{}
	h := NewTelegramConfigHandler(appconfig.NewService(nil, repo))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/proxies", strings.NewReader(`{
		"name":"test",
		"type":"ss",
		"addr":"127.0.0.1:1080",
		"enabled":true
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateProxy(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), domainconfig.ErrUnsupportedProxyType.Error()) {
		t.Fatalf("body = %s", w.Body.String())
	}
	if repo.createCalls != 0 {
		t.Fatalf("create calls = %d", repo.createCalls)
	}
}
