package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"telegram-message-forward/internal/bootstrap"
	"telegram-message-forward/internal/config"
	"telegram-message-forward/internal/security"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("未找到 go.mod")
		}
		dir = parent
	}
}

// TestAPIAuthAndCRUD 验证：无 token 401；有 token 可完成渠道 CRUD 且 secret 不回显、敏感 config 脱敏。
func TestAPIAuthAndCRUD(t *testing.T) {
	if os.Getenv("TMF_RUN_DB_TESTS") == "" {
		t.Skip("设置 TMF_RUN_DB_TESTS=1 运行数据库集成测试")
	}

	root := findRepoRoot(t)
	cfg, err := config.Load(filepath.Join(root, "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	app, err := bootstrap.Build(cfg)
	if err != nil {
		t.Fatalf("装配应用失败: %v", err)
	}
	handler := app.Handler()
	deps := app.Dependencies()
	ctx := context.Background()

	// 准备一个唯一的有效 token（随机，避免与历史运行的 token_hash 唯一约束冲突）。
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		t.Fatal(err)
	}
	plain := "it-token-" + hex.EncodeToString(rnd)
	tokenID, err := deps.APITokens.Create(ctx, "it-token", security.HashToken(plain))
	if err != nil {
		t.Fatalf("创建 token 失败: %v", err)
	}
	defer deps.APITokens.Revoke(ctx, tokenID)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// 1. 无 token → 401
	resp := doReq(t, srv.URL, http.MethodGet, "/api/v1/sinks", "", nil)
	if resp.code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401，实际 %d", resp.code)
	}

	// 2. 错误 token → 401
	resp = doReq(t, srv.URL, http.MethodGet, "/api/v1/sinks", "wrong-token", nil)
	if resp.code != http.StatusUnauthorized {
		t.Fatalf("错误 token 应 401，实际 %d", resp.code)
	}

	// 3. 有 token → 200
	resp = doReq(t, srv.URL, http.MethodGet, "/api/v1/sinks", plain, nil)
	if resp.code != http.StatusOK {
		t.Fatalf("有 token 应 200，实际 %d: %s", resp.code, resp.body)
	}

	// 4. 创建 webhook sink（带 secret + 敏感 config）
	createBody := map[string]any{
		"type":   "webhook",
		"name":   "it-api-sink",
		"config": map[string]any{"url": "http://example.com/hook", "webhook_url": "http://secret"},
		"secret": "topsecret",
	}
	resp = doReq(t, srv.URL, http.MethodPost, "/api/v1/sinks", plain, createBody)
	if resp.code != http.StatusCreated {
		t.Fatalf("创建 sink 应 201，实际 %d: %s", resp.code, resp.body)
	}
	var created struct {
		Data struct {
			ID        int64          `json:"id"`
			HasSecret bool           `json:"has_secret"`
			Config    map[string]any `json:"config"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	sinkID := created.Data.ID
	defer deps.Sinks.Delete(ctx, sinkID)

	if !created.Data.HasSecret {
		t.Fatal("响应应标记 has_secret=true")
	}
	// secret 明文不应出现在响应中。
	if strings.Contains(resp.body, "topsecret") {
		t.Fatalf("响应不应包含 secret 明文: %s", resp.body)
	}
	// 敏感 config 键应脱敏。
	if created.Data.Config["webhook_url"] != "***" {
		t.Fatalf("webhook_url 应脱敏为 ***，实际 %v", created.Data.Config["webhook_url"])
	}

	// 5. 删除 sink → 204
	resp = doReq(t, srv.URL, http.MethodDelete, "/api/v1/sinks/"+strconv.FormatInt(sinkID, 10), plain, nil)
	if resp.code != http.StatusNoContent {
		t.Fatalf("删除 sink 应 204，实际 %d", resp.code)
	}
}

type apiResp struct {
	code int
	body string
}

func doReq(t *testing.T, base, method, path, token string, body any) apiResp {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, base+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return apiResp{code: resp.StatusCode, body: buf.String()}
}
