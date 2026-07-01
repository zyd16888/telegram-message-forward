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
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.auto_migrate", false)
	v.SetDefault("dispatch.worker_count", 2)
	v.SetDefault("dispatch.poll_interval", "2s")
	v.SetDefault("dispatch.visibility_timeout", "5m")
	v.SetDefault("dispatch.max_attempts", 3)
}
