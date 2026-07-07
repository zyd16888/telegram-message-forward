package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Load 从配置文件和环境变量加载配置。
//
// 环境变量前缀为 TMF，例如 TMF_DATABASE_DSN、TMF_SECURITY_ENCRYPTION_KEY，
// 嵌套字段用下划线分隔，环境变量优先级高于配置文件。
func Load(path string) (*Config, error) {
	v := viper.New()

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	setDefaults(v)

	v.SetEnvPrefix("TMF")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.web_dir", "web/dist")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.auto_migrate", false)
	// 鉴权默认开启；仅开发/本地可通过 TMF_SECURITY_AUTH_ENABLED=false 关闭。
	v.SetDefault("security.auth_enabled", true)
	v.SetDefault("dispatch.worker_count", 2)
	v.SetDefault("dispatch.poll_interval", "2s")
	v.SetDefault("dispatch.visibility_timeout", "5m")
	v.SetDefault("dispatch.max_attempts", 3)
	v.SetDefault("flow_engine.mode", "off")
	v.SetDefault("media.dir", "data/media")
	v.SetDefault("media.url_ttl", "24h")
	v.SetDefault("media.retention", "168h")
	v.SetDefault("media.s3.enabled", false)
	v.SetDefault("media.s3.use_ssl", true)
	v.SetDefault("media.s3.auto_cleanup", false)
	v.SetDefault("media.download.image_max_mb", 20)
	v.SetDefault("media.download.file_max_mb", 50)
	v.SetDefault("media.download.file_types", DefaultDownloadFileTypes())
}

// DefaultDownloadFileTypes 返回文件下载扩展名白名单默认值（文档、表格、压缩包等常见资料类型）。
func DefaultDownloadFileTypes() []string {
	return []string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "csv", "txt", "md", "epub", "zip", "rar", "7z"}
}
