package database

import (
	"log"

	"gorm.io/gorm"

	"github.com/zyd16888/telegram-message-forward/models"
)

// MigrateTables 迁移数据库表结构
func MigrateTables(db *gorm.DB) {
	// 自动迁移模型，确保表结构和模型同步
	err := db.AutoMigrate(&models.PluginConfig{})
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	}

	// 插入测试数据
	pluginSettings := []models.PluginConfig{
		{
			Name:    "printmsg",
			Enabled: true,
			Config:  "{}",
		},
		{
			Name:    "wechat",
			Enabled: true,
			Config:  `{"apikey": "1234567890"}`,
		},
	}

	for _, plugin := range pluginSettings {
		var existingPlugin models.PluginConfig
		if err := db.Where("name = ?", plugin.Name).First(&existingPlugin).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&plugin)
			} else {
				log.Fatalf("failed to query plugin: %v", err)
			}
		} else {
			db.Model(&existingPlugin).Updates(plugin)
		}
	}

}
