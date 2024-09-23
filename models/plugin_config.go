package models

import (
	"gorm.io/gorm"
)

// PluginConfig 定义插件的数据库表结构
type PluginConfig struct {
	Name    string `gorm:"uniqueIndex"`
	Enabled bool
	Config  string // 存储 JSON 配置
	gorm.Model
}
