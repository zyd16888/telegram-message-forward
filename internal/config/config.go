// Package config 定义服务配置结构。
package config

import "time"

// Config 是服务的全部配置。
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	Security SecurityConfig `mapstructure:"security"`
	Dispatch DispatchConfig `mapstructure:"dispatch"`
}

// ServerConfig 是 HTTP 服务配置。
type ServerConfig struct {
	Addr   string `mapstructure:"addr"`
	WebDir string `mapstructure:"web_dir"`
}

// LogConfig 是日志配置。
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DatabaseConfig 是数据库配置。
type DatabaseConfig struct {
	DSN          string `mapstructure:"dsn"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	AutoMigrate  bool   `mapstructure:"auto_migrate"`
}

// SecurityConfig 是安全相关配置。
type SecurityConfig struct {
	// EncryptionKey 是敏感字段加密主密钥（KEK），必须为 32 字节。
	EncryptionKey string `mapstructure:"encryption_key"`
	// AuthEnabled 控制 /api/v1 是否启用 Bearer Token 鉴权，默认 true。
	// 仅在开发或单人本地使用时可通过 TMF_SECURITY_AUTH_ENABLED=false 关闭；
	// /healthz 始终无鉴权。
	AuthEnabled bool `mapstructure:"auth_enabled"`
}

// DispatchConfig 是投递 worker 配置。
type DispatchConfig struct {
	WorkerCount       int           `mapstructure:"worker_count"`
	PollInterval      time.Duration `mapstructure:"poll_interval"`
	VisibilityTimeout time.Duration `mapstructure:"visibility_timeout"`
	MaxAttempts       int           `mapstructure:"max_attempts"`
}
