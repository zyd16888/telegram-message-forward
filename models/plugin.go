package models

import (
	"gorm.io/gorm"
)

// 数据库表结构
type PluginConfig struct {
	Name    string `gorm:"uniqueIndex" json:"name"`
	Enabled bool   `gorm:"default:false" json:"enabled"`
	Config  string `json:"config"` // 存储 JSON 配置
	gorm.Model
}

type ChatConfig struct {
	gorm.Model
	ChatID int64 `gorm:"uniqueIndex"` // 聊天ID，唯一索引
	Name   string                     // 聊天名称
}

type ChatPluginAssociation struct {
	gorm.Model
	ChatConfigID   uint         // 聊天配置ID，关联ChatConfig表
	PluginConfigID uint         // 插件配置ID，关联PluginConfig表
	Enabled        bool   `gorm:"default:false"` // 是否启用该插件
}