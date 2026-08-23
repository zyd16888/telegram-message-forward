package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api"
	"telegram-message-forward/internal/api/handler"
	apptoken "telegram-message-forward/internal/app/apitoken"
	appauth "telegram-message-forward/internal/app/auth"
	appchatarchive "telegram-message-forward/internal/app/chatarchive"
	appsink "telegram-message-forward/internal/app/sink"
	domainarchive "telegram-message-forward/internal/domain/chatarchive"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/security"
)

// ---- 内存仓储 ----

type fakeArchiveRepo struct {
	archive *domainarchive.Archive
}

func (r *fakeArchiveRepo) Ensure(context.Context, *domainarchive.Archive) (*domainarchive.Archive, error) {
	return r.archive, nil
}
func (r *fakeArchiveRepo) GetByID(context.Context, int64) (*domainarchive.Archive, error) {
	return r.archive, nil
}
func (r *fakeArchiveRepo) List(context.Context) ([]*domainarchive.Archive, error) {
	return []*domainarchive.Archive{r.archive}, nil
}
func (r *fakeArchiveRepo) Delete(context.Context, int64) error                  { return nil }
func (r *fakeArchiveRepo) RefreshStats(context.Context, int64, time.Time) error { return nil }

type fakeArchiveMessageRepo struct {
	items []*domainarchive.Message
}

func (r *fakeArchiveMessageRepo) BulkUpsert(context.Context, int64, []*domainarchive.Message) (int64, error) {
	return 0, nil
}
func (r *fakeArchiveMessageRepo) Search(context.Context, domainarchive.MessageQuery) ([]*domainarchive.Message, bool, error) {
	return r.items, false, nil
}
func (r *fakeArchiveMessageRepo) CountByArchive(context.Context, int64) (int64, error) {
	return int64(len(r.items)), nil
}
func (r *fakeArchiveMessageRepo) StreamPage(_ context.Context, q domainarchive.ExportQuery) ([]*domainarchive.Message, error) {
	out := make([]*domainarchive.Message, 0, len(r.items))
	for _, m := range r.items {
		if m.ID > q.AfterID {
			out = append(out, m)
		}
	}
	return out, nil
}

type fakeExportJobRepo struct{ job domainarchive.Job }

func (r *fakeExportJobRepo) Create(_ context.Context, j *domainarchive.Job) error {
	j.ID = 1
	r.job = *j
	return nil
}
func (r *fakeExportJobRepo) Update(_ context.Context, j *domainarchive.Job) error {
	r.job = *j
	return nil
}
func (r *fakeExportJobRepo) GetByID(context.Context, int64) (*domainarchive.Job, error) {
	out := r.job
	return &out, nil
}
func (r *fakeExportJobRepo) ListByArchive(context.Context, int64, int) ([]*domainarchive.Job, error) {
	out := r.job
	return []*domainarchive.Job{&out}, nil
}
func (r *fakeExportJobRepo) ListActive(context.Context) ([]*domainarchive.Job, error) {
	return nil, nil
}
func (r *fakeExportJobRepo) RequestCancel(context.Context, int64) error { return nil }

// 归档消息里就是私人对话原文，接口必须走鉴权。
func TestChatArchiveRoutesRequireAuth(t *testing.T) {
	repo := newFakeTokenRepo()
	r := buildRouterWithChatArchive(t, true, repo, nil)

	for _, path := range []string{
		"/api/v1/chat-archives",
		"/api/v1/chat-archives/1",
		"/api/v1/chat-archives/1/messages",
		"/api/v1/chat-archives/1/download?format=jsonl",
		"/api/v1/chat-exports",
		"/api/v1/chat-exports/1",
	} {
		if code, _ := do(t, r, http.MethodGet, path, "", nil); code != http.StatusUnauthorized {
			t.Fatalf("%s 无 token 应 401，实际 %d", path, code)
		}
	}
	for _, path := range []string{"/api/v1/chat-exports", "/api/v1/chat-exports/1/cancel"} {
		if code, _ := do(t, r, http.MethodPost, path, "", nil); code != http.StatusUnauthorized {
			t.Fatalf("%s 无 token 应 401", path)
		}
	}
	if code, _ := do(t, r, http.MethodDelete, "/api/v1/chat-archives/1", "", nil); code != http.StatusUnauthorized {
		t.Fatal("删除归档无 token 应 401")
	}
}

func TestChatArchiveListAndSearch(t *testing.T) {
	repo := newFakeTokenRepo()
	date := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	msgs := []*domainarchive.Message{
		{ID: 1, MessageID: 101, Date: &date, Out: true, SenderName: "我", MessageType: "text", Text: "你好"},
	}
	r := buildRouterWithChatArchive(t, true, repo, msgs)
	token := mintToken(t, repo)

	code, body := do(t, r, http.MethodGet, "/api/v1/chat-archives", token, nil)
	if code != http.StatusOK {
		t.Fatalf("列表 = %d: %s", code, body)
	}

	code, body = do(t, r, http.MethodGet, "/api/v1/chat-archives/1/messages?q=%E4%BD%A0%E5%A5%BD", token, nil)
	if code != http.StatusOK {
		t.Fatalf("检索 = %d: %s", code, body)
	}
	var resp struct {
		Data []struct {
			MessageID int64  `json:"message_id"`
			Direction string `json:"direction"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 || resp.Data[0].Direction != "out" {
		t.Fatalf("检索结果异常: %s", body)
	}
}

func TestChatArchiveDownloadStreamsAndSetsHeaders(t *testing.T) {
	repo := newFakeTokenRepo()
	date := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	msgs := []*domainarchive.Message{
		{ID: 1, MessageID: 101, Date: &date, SenderName: "阿达", MessageType: "text", Text: "在吗"},
	}
	r := buildRouterWithChatArchive(t, true, repo, msgs)
	token := mintToken(t, repo)

	code, body, hdr := doWithHeaders(t, r, http.MethodGet, "/api/v1/chat-archives/1/download?format=jsonl", token)
	if code != http.StatusOK {
		t.Fatalf("下载 = %d: %s", code, body)
	}
	if !strings.Contains(hdr.Get("Content-Disposition"), "attachment;") {
		t.Fatalf("缺少下载头: %q", hdr.Get("Content-Disposition"))
	}
	if hdr.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("缺少 nosniff 头")
	}
	if !strings.Contains(body, `"message_id":101`) || !strings.Contains(body, "在吗") {
		t.Fatalf("导出内容异常: %s", body)
	}

	// 非法格式必须在写响应头之前拦掉。
	if code, _, _ = doWithHeaders(t, r, http.MethodGet, "/api/v1/chat-archives/1/download?format=pdf", token); code != http.StatusBadRequest {
		t.Fatalf("非法格式应 400，实际 %d", code)
	}
}

func TestChatArchiveSearchRejectsBadParams(t *testing.T) {
	repo := newFakeTokenRepo()
	r := buildRouterWithChatArchive(t, true, repo, nil)
	token := mintToken(t, repo)

	for _, path := range []string{
		"/api/v1/chat-archives/1/messages?direction=sideways",
		"/api/v1/chat-archives/1/messages?include_service=maybe",
		"/api/v1/chat-archives/1/messages?from=not-a-time",
	} {
		if code, body := do(t, r, http.MethodGet, path, token, nil); code != http.StatusBadRequest {
			t.Fatalf("%s 应 400，实际 %d: %s", path, code, body)
		}
	}
}

func TestChatExportCreateValidates(t *testing.T) {
	repo := newFakeTokenRepo()
	r := buildRouterWithChatArchive(t, true, repo, nil)
	token := mintToken(t, repo)

	code, body := do(t, r, http.MethodPost, "/api/v1/chat-exports", token, map[string]any{
		"account_id": 1, "peer_type": "user", "peer_id": 7, "from_date": "昨天",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("非法时间应 400，实际 %d: %s", code, body)
	}

	code, body = do(t, r, http.MethodPost, "/api/v1/chat-exports", token, map[string]any{
		"peer_type": "user", "peer_id": 7,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("缺少 account_id 应 400，实际 %d: %s", code, body)
	}
}

// mintToken 生成一个有效的明文 API token。
func mintToken(t *testing.T, repo *fakeTokenRepo) string {
	t.Helper()
	plain, err := security.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(context.Background(), "t", security.HashToken(plain)); err != nil {
		t.Fatal(err)
	}
	return plain
}

// doWithHeaders 与 do 相同，但额外返回响应头（下载接口需要校验）。
func doWithHeaders(t *testing.T, r http.Handler, method, path, token string) (int, string, http.Header) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(nil))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String(), w.Header()
}

func buildRouterWithChatArchive(t *testing.T, authEnabled bool, repo *fakeTokenRepo, msgs []*domainarchive.Message) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	adminRepo := newFakeAdminRepo()
	tokenSvc := apptoken.NewService(repo)
	authSvc := appauth.NewService(adminRepo, repo, tokenSvc, authEnabled)
	validator := security.TokenValidator{
		Lookup: func(hash string) (bool, error) {
			sess, user, err := adminRepo.GetActiveSessionByHash(context.Background(), hash, time.Now())
			if err != nil {
				return false, err
			}
			if sess != nil && user != nil {
				return true, nil
			}
			return repo.ExistsActiveHash(context.Background(), hash)
		},
	}

	archive := &domainarchive.Archive{
		ID: 1, AccountID: 1, PeerType: domainarchive.PeerUser, PeerID: 7, PeerName: "阿达",
	}
	svc := appchatarchive.NewService(
		&fakeArchiveRepo{archive: archive},
		&fakeArchiveMessageRepo{items: msgs},
		&fakeExportJobRepo{},
		nil, nil, nil, clock.System{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return api.NewRouter(api.Deps{
		TokenValidator: validator,
		AuthEnabled:    authEnabled,
		Auth:           handler.NewAuthHandler(authSvc),
		Sink:           handler.NewSinkHandler(appsink.NewService(fakeSinkRepo{})),
		ChatArchive:    handler.NewChatArchiveHandler(svc),
	})
}
