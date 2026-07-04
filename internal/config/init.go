package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureFile creates a starter config file when it does not exist.
func EnsureFile(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, errors.New("配置文件路径不能为空")
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("配置文件路径是目录: %s", path)
		}
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("检查配置文件失败: %w", err)
	}

	key, err := randomKey()
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("创建配置目录失败: %w", err)
	}

	content := fmt.Sprintf(`# 服务配置。请先填写 database.dsn，再启动服务。

server:
  addr: ":8080"
  web_dir: "web/dist"

log:
  level: "info"
  format: "text"

database:
  dsn: "postgres://tmf:CHANGE_ME_DB_PASSWORD@db.example.com:5432/telegram_forward?sslmode=require"
  max_open_conns: 20
  max_idle_conns: 5
  auto_migrate: true

security:
  # 已自动生成 32 字节密钥。存入 Telegram session 或 secret 后不要再修改。
  encryption_key: "%s"
  auth_enabled: true

dispatch:
  worker_count: 2
  poll_interval: "2s"
  visibility_timeout: "5m"
  max_attempts: 3

# 媒体存储推荐在管理后台「设置」页配置（页面保存后立即生效且优先于本段）；
# 本段仅作为页面未配置时的默认值，可整段留默认。
media:
  # 本地媒体目录；Telegram 下载的图片等媒体保存在这里。
  dir: "data/media"
  # 本服务对外可访问的根地址（如 https://tmf.example.com）。
  # 配置后钉钉/Bark/Gotify 等只认公网 URL 的渠道可直接引用本服务托管的媒体。
  public_base_url: ""
  url_ttl: "24h"
  # 媒体保留时长（如 30 天写 "720h"），超过后由后台任务清理；0 表示不清理。
  retention: "168h"
  # 媒体下载策略：图片/文件大小上限与文件类型白名单（推荐在设置页配置）。
  download:
    image_max_mb: 20
    file_max_mb: 50
    # 文件扩展名白名单（不带点）；留空列表 [] 表示不限类型。
    file_types: ["pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "csv", "txt", "md", "epub", "zip", "rar", "7z"]
  # 可选：S3 兼容对象存储（AWS S3 / Cloudflare R2 / MinIO / OSS / COS）。
  # 启用后媒体额外上传到对象存储，公网 URL 优先使用对象存储地址。
  s3:
    enabled: false
    endpoint: ""
    region: ""
    bucket: ""
    access_key: ""
    secret_key: ""
    use_ssl: true
    key_prefix: ""
    # 公开桶或 CDN 根地址；留空则生成预签名 URL。
    public_base_url: ""
    # 开启后超过 retention 的对象由本服务定时删除（共用桶时务必设置 key_prefix）；
    # 关闭时请用桶生命周期规则清理。
    auto_cleanup: false
`, key)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return false, fmt.Errorf("写入配置文件失败: %w", err)
	}
	return true, nil
}

// ValidateFileReady checks whether the generated starter config was edited.
func ValidateFileReady(path string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Database.DSN) == "" || strings.Contains(cfg.Database.DSN, "CHANGE_ME") {
		return errors.New("database.dsn 仍是占位值")
	}
	if len(cfg.Security.EncryptionKey) != 32 || strings.Contains(cfg.Security.EncryptionKey, "CHANGE_ME") {
		return errors.New("security.encryption_key 必须是 32 字节真实密钥")
	}
	return nil
}

func randomKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成加密密钥失败: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
