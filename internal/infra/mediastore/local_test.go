package mediastore

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestLocal(t *testing.T, base string) *Local {
	t.Helper()
	l := NewLocal(t.TempDir(), base, []byte("0123456789abcdef0123456789abcdef"), time.Hour)
	return l
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "src.jpg")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLocalPutFileMovesIntoStore(t *testing.T) {
	l := newTestLocal(t, "")
	src := writeTempFile(t, "img-data")

	dst, err := l.PutFile(context.Background(), "telegram/source_1/10_0.jpg", src)
	if err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "img-data" {
		t.Fatalf("目标文件内容不符: %q err=%v", data, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("源文件应已被移除")
	}
}

func TestLocalPutFileRejectsBadKey(t *testing.T) {
	l := newTestLocal(t, "")
	src := writeTempFile(t, "x")
	for _, key := range []string{"", "../etc/passwd", "/abs", "a/../../b", "a\\b"} {
		if _, err := l.PutFile(context.Background(), key, src); err == nil {
			t.Fatalf("key=%q 应被拒绝", key)
		}
	}
}

func TestLocalPublicURLSignAndVerify(t *testing.T) {
	l := newTestLocal(t, "https://tmf.example.com/")
	const key = "telegram/source_1/10_0.jpg"

	raw, err := l.PublicURL(context.Background(), key)
	if err != nil {
		t.Fatalf("PublicURL: %v", err)
	}
	if !strings.HasPrefix(raw, "https://tmf.example.com/media/"+key+"?") {
		t.Fatalf("URL 前缀不符: %s", raw)
	}

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	expires, err := strconv.ParseInt(u.Query().Get("e"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !l.VerifySignedPath(key, expires, u.Query().Get("s")) {
		t.Fatal("合法签名校验失败")
	}
	if l.VerifySignedPath(key, expires, "bad-sig") {
		t.Fatal("非法签名不应通过")
	}
	if l.VerifySignedPath("telegram/source_1/11_0.jpg", expires, u.Query().Get("s")) {
		t.Fatal("签名不应对其他 key 生效")
	}
}

func TestLocalPublicURLEmptyWithoutBase(t *testing.T) {
	l := newTestLocal(t, "")
	u, err := l.PublicURL(context.Background(), "a/b.jpg")
	if err != nil || u != "" {
		t.Fatalf("未配置 public_base_url 时应返回空串, got %q err=%v", u, err)
	}
}

func TestLocalVerifyExpired(t *testing.T) {
	l := newTestLocal(t, "https://tmf.example.com")
	const key = "a/b.jpg"
	expired := time.Now().Add(-time.Minute).Unix()
	if l.VerifySignedPath(key, expired, l.sign(key, expired)) {
		t.Fatal("过期签名不应通过")
	}
}

func TestLocalCleanup(t *testing.T) {
	l := newTestLocal(t, "")
	src := writeTempFile(t, "old")
	if _, err := l.PutFile(context.Background(), "a/old.jpg", src); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(l.dir, "a", "old.jpg"), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	src2 := writeTempFile(t, "new")
	if _, err := l.PutFile(context.Background(), "a/new.jpg", src2); err != nil {
		t.Fatal(err)
	}

	removed, err := l.Cleanup(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if removed != 1 {
		t.Fatalf("应清理 1 个文件, got %d", removed)
	}
	if _, err := os.Stat(filepath.Join(l.dir, "a", "new.jpg")); err != nil {
		t.Fatalf("新文件不应被清理: %v", err)
	}
}
