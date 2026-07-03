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
