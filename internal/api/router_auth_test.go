package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api"
	"telegram-message-forward/internal/api/handler"
	apptoken "telegram-message-forward/internal/app/apitoken"
	appauth "telegram-message-forward/internal/app/auth"
	appsink "telegram-message-forward/internal/app/sink"
	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/security"
)

// fakeTokenRepo 是内存版 apitoken 仓储，供无 DB 的路由鉴权测试使用。
type fakeTokenRepo struct {
	mu     sync.Mutex
	nextID int64
	items  map[int64]*domainapitoken.Token
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{items: map[int64]*domainapitoken.Token{}}
}

func (r *fakeTokenRepo) Create(_ context.Context, name, hash string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	r.items[r.nextID] = &domainapitoken.Token{ID: r.nextID, Name: name, TokenHash: hash, CreatedAt: time.Now()}
	return r.nextID, nil
}

func (r *fakeTokenRepo) List(_ context.Context) ([]*domainapitoken.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domainapitoken.Token, 0, len(r.items))
	for _, t := range r.items {
		out = append(out, t)
	}
	return out, nil
}

func (r *fakeTokenRepo) Revoke(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.items[id]; ok {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}

func (r *fakeTokenRepo) ExistsActiveHash(_ context.Context, hash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.items {
		if t.TokenHash == hash && t.RevokedAt == nil {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTokenRepo) CountActive(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, t := range r.items {
		if t.RevokedAt == nil {
			n++
		}
	}
	return n, nil
}

// fakeSinkRepo 是内存版 sink 仓储，List 返回空即可满足 200 校验。
type fakeSinkRepo struct{}

func (fakeSinkRepo) Create(context.Context, *domainsink.Sink) error         { return nil }
func (fakeSinkRepo) Update(context.Context, *domainsink.Sink) error         { return nil }
func (fakeSinkRepo) GetByID(context.Context, int64) (*domainsink.Sink, error) { return nil, nil }
func (fakeSinkRepo) List(context.Context) ([]*domainsink.Sink, error) {
	return []*domainsink.Sink{}, nil
}
func (fakeSinkRepo) Delete(context.Context, int64) error { return nil }

func buildRouter(t *testing.T, authEnabled bool, repo *fakeTokenRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tokenSvc := apptoken.NewService(repo)
	authSvc := appauth.NewService(repo, tokenSvc, authEnabled)
	sinkSvc := appsink.NewService(fakeSinkRepo{})
	validator := security.TokenValidator{
		Lookup: func(hash string) (bool, error) { return repo.ExistsActiveHash(context.Background(), hash) },
	}
	return api.NewRouter(api.Deps{
		Logger:         nil,
		TokenValidator: validator,
		AuthEnabled:    authEnabled,
		Auth:           handler.NewAuthHandler(authSvc),
		Sink:           handler.NewSinkHandler(sinkSvc),
	})
}

func do(t *testing.T, r http.Handler, method, path, token string, body any) (int, string) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestAuthEnabledRequiresToken 验证 auth_enabled=true 时无 token 请求 /api/v1/sinks 返回 401，
// 有效 token 返回 200。
func TestAuthEnabledRequiresToken(t *testing.T) {
	repo := newFakeTokenRepo()
	plain := "valid-token-123"
	if _, err := repo.Create(context.Background(), "t", security.HashToken(plain)); err != nil {
		t.Fatal(err)
	}
	r := buildRouter(t, true, repo)

	if code, body := do(t, r, http.MethodGet, "/api/v1/sinks", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401，实际 %d: %s", code, body)
	}
	if code, body := do(t, r, http.MethodGet, "/api/v1/sinks", "wrong", nil); code != http.StatusUnauthorized {
		t.Fatalf("错误 token 应 401，实际 %d: %s", code, body)
	}
	if code, body := do(t, r, http.MethodGet, "/api/v1/sinks", plain, nil); code != http.StatusOK {
		t.Fatalf("有效 token 应 200，实际 %d: %s", code, body)
	}
}

// TestAuthDisabledSkipsToken 验证 auth_enabled=false 时无 token 请求 /api/v1/sinks 返回 200。
func TestAuthDisabledSkipsToken(t *testing.T) {
	repo := newFakeTokenRepo()
	r := buildRouter(t, false, repo)
	if code, body := do(t, r, http.MethodGet, "/api/v1/sinks", "", nil); code != http.StatusOK {
		t.Fatalf("免鉴权无 token 应 200，实际 %d: %s", code, body)
	}
}

// TestAuthDisabledMe 验证免鉴权模式下 /auth/me 返回 authenticated=true 且 auth_enabled=false。
func TestAuthDisabledMe(t *testing.T) {
	repo := newFakeTokenRepo()
	r := buildRouter(t, false, repo)
	code, body := do(t, r, http.MethodGet, "/api/v1/auth/me", "", nil)
	if code != http.StatusOK {
		t.Fatalf("me 应 200，实际 %d: %s", code, body)
	}
	var resp struct {
		Data struct {
			AuthEnabled   bool `json:"auth_enabled"`
			Authenticated bool `json:"authenticated"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.AuthEnabled || !resp.Data.Authenticated {
		t.Fatalf("免鉴权模式应 auth_enabled=false 且 authenticated=true，实际 %+v", resp.Data)
	}
}

// TestBootstrapOnlyWhenNoActiveToken 验证 bootstrap 只在无 active token 时可用：
// 初始可创建首个凭证，创建后再次 bootstrap 返回不可初始化且 POST 冲突。
func TestBootstrapOnlyWhenNoActiveToken(t *testing.T) {
	repo := newFakeTokenRepo()
	r := buildRouter(t, true, repo)

	// 初始：可 bootstrap。
	code, body := do(t, r, http.MethodGet, "/api/v1/auth/bootstrap", "", nil)
	if code != http.StatusOK {
		t.Fatalf("bootstrap 状态应 200，实际 %d", code)
	}
	var status struct {
		Data struct {
			CanBootstrap bool `json:"can_bootstrap"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(body), &status)
	if !status.Data.CanBootstrap {
		t.Fatal("初始应可 bootstrap")
	}

	// 创建首个凭证。
	code, body = do(t, r, http.MethodPost, "/api/v1/auth/bootstrap", "", map[string]any{"name": "admin"})
	if code != http.StatusCreated {
		t.Fatalf("首个凭证应 201，实际 %d: %s", code, body)
	}
	var created struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal([]byte(body), &created)
	if created.Data.Token == "" {
		t.Fatal("应返回明文 token")
	}

	// 再次查询：不可 bootstrap。
	_, body = do(t, r, http.MethodGet, "/api/v1/auth/bootstrap", "", nil)
	json.Unmarshal([]byte(body), &status)
	if status.Data.CanBootstrap {
		t.Fatal("已有凭证后不应再可 bootstrap")
	}

	// 再次创建：409 冲突。
	if code, _ := do(t, r, http.MethodPost, "/api/v1/auth/bootstrap", "", map[string]any{"name": "x"}); code != http.StatusConflict {
		t.Fatalf("重复 bootstrap 应 409，实际 %d", code)
	}

	// 用新建凭证登录成功。
	if code, _ := do(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"token": created.Data.Token}); code != http.StatusOK {
		t.Fatalf("有效 token 登录应 200，实际 %d", code)
	}
	// 错误凭证登录失败。
	if code, _ := do(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"token": "nope"}); code != http.StatusUnauthorized {
		t.Fatalf("错误 token 登录应 401，实际 %d", code)
	}
}
