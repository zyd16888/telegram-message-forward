package mediastore

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Local 是本地目录存储：文件保存在 dir 下，可选地通过本服务的
// /media/*key 端点对外提供带 HMAC 签名和过期时间的 URL。
type Local struct {
	dir           string
	publicBaseURL string
	signKey       []byte
	urlTTL        time.Duration
	now           func() time.Time
}

// NewLocal 创建本地存储。publicBaseURL 为空时 PublicURL 恒返回空串。
func NewLocal(dir, publicBaseURL string, signKey []byte, urlTTL time.Duration) *Local {
	if urlTTL <= 0 {
		urlTTL = 24 * time.Hour
	}
	return &Local{
		dir:           dir,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		signKey:       signKey,
		urlTTL:        urlTTL,
		now:           time.Now,
	}
}

var _ Store = (*Local)(nil)

// PutFile 将 srcPath 移动到 dir/key。
func (l *Local) PutFile(_ context.Context, key, srcPath string) (string, error) {
	if !ValidKey(key) {
		return "", fmt.Errorf("非法存储键: %q", key)
	}
	dst := filepath.Join(l.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("创建媒体目录失败: %w", err)
	}
	if err := moveFile(srcPath, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// PublicURL 生成带过期时间与 HMAC 签名的媒体访问 URL。
func (l *Local) PublicURL(_ context.Context, key string) (string, error) {
	if l.publicBaseURL == "" {
		return "", nil
	}
	if !ValidKey(key) {
		return "", fmt.Errorf("非法存储键: %q", key)
	}
	expires := l.now().Add(l.urlTTL).Unix()
	q := url.Values{}
	q.Set("e", strconv.FormatInt(expires, 10))
	q.Set("s", l.sign(key, expires))
	return l.publicBaseURL + "/media/" + escapeKeyPath(key) + "?" + q.Encode(), nil
}

// Open 打开本地文件。
func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if !ValidKey(key) {
		return nil, fmt.Errorf("非法存储键: %q", key)
	}
	return os.Open(filepath.Join(l.dir, filepath.FromSlash(key)))
}

// Cleanup 删除早于 olderThan 的本地文件。
func (l *Local) Cleanup(_ context.Context, olderThan time.Duration) (int, error) {
	if _, err := os.Stat(l.dir); os.IsNotExist(err) {
		return 0, nil
	}
	return cleanupDir(l.dir, l.now().Add(-olderThan))
}

// VerifySignedPath 校验 /media 端点收到的签名与有效期。
func (l *Local) VerifySignedPath(key string, expires int64, sig string) bool {
	if !ValidKey(key) || expires < l.now().Unix() {
		return false
	}
	expect := l.sign(key, expires)
	return subtle.ConstantTimeCompare([]byte(expect), []byte(sig)) == 1
}

// sign 计算 key + 过期时间的 HMAC-SHA256 签名。
func (l *Local) sign(key string, expires int64) string {
	mac := hmac.New(sha256.New, l.signKey)
	fmt.Fprintf(mac, "%s\n%d", key, expires)
	return hex.EncodeToString(mac.Sum(nil))
}

// escapeKeyPath 按路径段转义存储键，保留 / 分隔符。
func escapeKeyPath(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
