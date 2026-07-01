package database

import (
	"errors"
	"fmt"
	"log"

	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/zyd16888/telegram-message-forward/models"
)

// MigrateTables 迁移数据库表结构
func MigrateTables(db *gorm.DB, config *viper.Viper) {
	// 自动迁移模型，确保表结构和模型同步
	err := db.AutoMigrate(&models.PluginConfig{})
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	}

	err = db.AutoMigrate(&models.ChatConfig{})
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	}

	err = db.AutoMigrate(&models.ChatPluginAssociation{})
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	}

	wechatConfig := config.GetStringMap("wecom_application")
	corpid := wechatConfig["corpid"].(string)
	corpsecret := wechatConfig["corpsecret"].(string)
	agentid := wechatConfig["agentid"].(string)

	// 插入测试数据
	pluginSettings := []models.PluginConfig{
		{
			Name:    "printmsg",
			Enabled: true,
			Config:  "{}",
		},
		{
			Name:    "wecom",
			Enabled: true,
			Config:  fmt.Sprintf(`{"corpid": "%s", "corpsecret": "%s", "agentid": "%s"}`, corpid, corpsecret, agentid),
		},
	}

	for _, plugin := range pluginSettings {
		var existingPlugin models.PluginConfig
		if err := db.Where(models.ChatConfig{Name: plugin.Name}).First(&existingPlugin).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				db.Create(&plugin)
			} else {
				log.Fatalf("failed to query plugin: %v", err)
			}
		} else {
			db.Model(&existingPlugin).Updates(plugin)
		}
	}

	// 定义聊天设置
	chatSettings := []models.ChatConfig{
		{
			ChatID: 2161625827,
			Name:   "test",
		},
	}

	// 插入或更新聊天设置
	for _, chat := range chatSettings {
		var existingChat models.ChatConfig
		if err := db.Where(models.ChatConfig{
			ChatID: chat.ChatID,
		}).First(&existingChat).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				db.Create(&chat)
			} else {
				log.Fatalf("查询聊天设置失败: %v", err)
			}
		} else {
			db.Model(&existingChat).Updates(chat)
		}
	}

	// 定义聊天插件关联
	chatAssociations := []models.ChatPluginAssociation{
		{
			ChatConfigID:   1,
			PluginConfigID: 1,
			Enabled:        true,
		},
		{
			ChatConfigID:   1,
			PluginConfigID: 2,
			Enabled:        true,
		},
	}

	// 插入或更新聊天插件关联
	for _, association := range chatAssociations {
		var existingAssociation models.ChatPluginAssociation
		if err := db.Where(models.ChatPluginAssociation{
			ChatConfigID:   association.ChatConfigID,
			PluginConfigID: association.PluginConfigID,
		}).First(&existingAssociation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				db.Create(&association)
			} else {
				log.Fatalf("查询聊天插件关联失败: %v", err)
			}
		} else {
			db.Model(&existingAssociation).Updates(association)
		}
	}
}
