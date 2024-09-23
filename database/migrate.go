package database

import (
	"gorm.io/gorm"
	"log"
	"telegram-message-forward/models" // 导入你的模型包
)

// MigrateTables 迁移数据库表结构
func MigrateTables(db *gorm.DB) {
	// 自动迁移模型，确保表结构和模型同步
	err := db.AutoMigrate(&models.PluginConfig{})
	if err != nil {
		log.Fatalf("failed to migrate tables: %v", err)
	}

}
