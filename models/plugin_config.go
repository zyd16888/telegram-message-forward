package models

import (
	"gorm.io/gorm"
)

// PluginConfig 定义插件的数据库表结构
type PluginConfig struct {
	Name    string `gorm:"uniqueIndex" json:"name"`
	Enabled bool   `gorm:"default:false" json:"enabled"`
	Config  string `json:"config"` // 存储 JSON 配置
	gorm.Model
}
