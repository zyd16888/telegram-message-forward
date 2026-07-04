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
	Media    MediaConfig    `mapstructure:"media"`
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

// MediaConfig 是媒体文件存储配置。
type MediaConfig struct {
	// Dir 是本地媒体目录，Source 下载的媒体统一收编到这里。
	Dir string `mapstructure:"dir"`
	// PublicBaseURL 是本服务对外可访问的根地址（如 https://tmf.example.com）。
	// 配置后本地媒体可通过 /media 端点生成带签名的公网 URL；留空则不生成。
	PublicBaseURL string `mapstructure:"public_base_url"`
	// URLTTL 是签名 URL / 预签名 URL 的有效期。
	URLTTL time.Duration `mapstructure:"url_ttl"`
	// Retention 是本地媒体保留时长，超过后由后台任务清理；0 表示不清理。
	Retention time.Duration `mapstructure:"retention"`
	// S3 是可选的 S3 兼容对象存储配置，启用后媒体额外上传并优先用其 URL。
	S3 S3Config `mapstructure:"s3"`
	// Download 是 Source 侧媒体下载策略（大小上限、文件类型白名单）。
	Download DownloadConfig `mapstructure:"download"`
}

// DownloadConfig 是媒体下载策略配置。
type DownloadConfig struct {
	// ImageMaxMB 是图片（photo 与 image document）下载大小上限，<=0 使用内置默认。
	ImageMaxMB float64 `mapstructure:"image_max_mb"`
	// FileMaxMB 是文件（PDF 等非图片 document）下载大小上限，<=0 使用内置默认。
	FileMaxMB float64 `mapstructure:"file_max_mb"`
	// FileTypes 是文件扩展名白名单（不带点，如 pdf、docx）；空列表表示不限类型。
	FileTypes []string `mapstructure:"file_types"`
}

// S3Config 是 S3 兼容对象存储配置（AWS S3 / Cloudflare R2 / MinIO / OSS / COS）。
type S3Config struct {
	Enabled   bool   `mapstructure:"enabled"`
	Endpoint  string `mapstructure:"endpoint"`
	Region    string `mapstructure:"region"`
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	// KeyPrefix 是对象键前缀，便于与其他数据共用一个桶。
	KeyPrefix string `mapstructure:"key_prefix"`
	// PublicBaseURL 非空时（公开桶或 CDN）直接拼接 URL，否则生成预签名 URL。
	PublicBaseURL string `mapstructure:"public_base_url"`
	// AutoCleanup 开启后，超过 media.retention 的对象由本服务定时删除；
	// 关闭时对象存储侧的过期清理交由桶生命周期规则。
	// 与其他数据共用一个桶时请务必设置 key_prefix，否则会误删无关对象。
	AutoCleanup bool `mapstructure:"auto_cleanup"`
}

// DispatchConfig 是投递 worker 配置。
type DispatchConfig struct {
	WorkerCount       int           `mapstructure:"worker_count"`
	PollInterval      time.Duration `mapstructure:"poll_interval"`
	VisibilityTimeout time.Duration `mapstructure:"visibility_timeout"`
	MaxAttempts       int           `mapstructure:"max_attempts"`
}
