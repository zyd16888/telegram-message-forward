package telegramconfig

import (
	"context"
	"errors"
	"testing"

	domainconfig "telegram-message-forward/internal/domain/telegramconfig"
)

type fakeProxyRepository struct {
	created *domainconfig.Proxy
}

func (r *fakeProxyRepository) Create(_ context.Context, p *domainconfig.Proxy) error {
	r.created = p
	return nil
}

func (r *fakeProxyRepository) Update(_ context.Context, _ *domainconfig.Proxy) error {
	return nil
}

func (r *fakeProxyRepository) GetByID(_ context.Context, _ int64) (*domainconfig.Proxy, error) {
	return nil, errors.New("unexpected GetByID call")
}

func (r *fakeProxyRepository) List(_ context.Context) ([]*domainconfig.Proxy, error) {
	return nil, nil
}

func (r *fakeProxyRepository) Delete(_ context.Context, _ int64) error {
	return nil
}

func TestCreateProxyNormalizesHTTPType(t *testing.T) {
	repo := &fakeProxyRepository{}
	svc := NewService(nil, repo)

	proxy, err := svc.CreateProxy(context.Background(), ProxyInput{Type: " HTTP "})
	if err != nil {
		t.Fatal(err)
	}
	if proxy.Type != domainconfig.ProxyTypeHTTP || repo.created != proxy {
		t.Fatalf("proxy = %#v, created = %#v", proxy, repo.created)
	}
}

func TestCreateProxyRejectsUnsupportedTypeBeforeWrite(t *testing.T) {
	repo := &fakeProxyRepository{}
	svc := NewService(nil, repo)

	_, err := svc.CreateProxy(context.Background(), ProxyInput{Type: "ss"})
	if !errors.Is(err, domainconfig.ErrUnsupportedProxyType) {
		t.Fatalf("CreateProxy error = %v", err)
	}
	if repo.created != nil {
		t.Fatalf("unsupported proxy was written: %#v", repo.created)
	}
}
