package plugin

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/celestix/gotgproto/types"
	"gorm.io/gorm"

	"github.com/zyd16888/telegram-message-forward/models"
)

type MessageHandler interface {
	Handle(message *types.Message) error
}

type PluginManager struct {
	plugins       []MessageHandler
	pluginFactory PluginFactory
}

func NewPluginManager(factory PluginFactory) *PluginManager {
	return &PluginManager{
		plugins:       make([]MessageHandler, 0),
		pluginFactory: factory,
	}
}

func (pm *PluginManager) RegisterPlugin(plugin MessageHandler) {
	pm.plugins = append(pm.plugins, plugin)
}

func (pm *PluginManager) HandleMessage(message *types.Message) error {
	for _, plugin := range pm.plugins {
		if err := plugin.Handle(message); err != nil {
			return err
		}
	}
	return nil
}

// LoadPluginsFromDB 从数据库加载插件配置并初始化插件
func (pm *PluginManager) LoadPluginsFromDB(db *gorm.DB) error {
	// 查询所有聊天配置
	var chatConfigs []models.ChatConfig
	if err := db.Find(&chatConfigs).Error; err != nil {
		return fmt.Errorf("查询Chat配置失败: %v", err)
	}

	// 重置插件列表
	pm.plugins = make([]MessageHandler, 0)

	// 遍历每个聊天配置
	for _, chatConfig := range chatConfigs {
		// 查询该聊天配置下启用的插件关联
		var associations []models.ChatPluginAssociation
		if err := db.Where("chat_config_id = ? AND enabled = ?", chatConfig.ID, true).Find(&associations).Error; err != nil {
			return fmt.Errorf("查询聊天插件关联失败 %v", err)
		}

		// 遍历每个插件关联
		for _, association := range associations {
			// 查询插件配置
			var pluginConfig models.PluginConfig
			if err := db.First(&pluginConfig, association.PluginConfigID).Error; err != nil {
				return fmt.Errorf("查询插件配置失败: %v", err)
			}

			// 如果插件配置启用
			if pluginConfig.Enabled {
				// 解析插件配置JSON
				var configMap map[string]interface{}
				if err := json.Unmarshal([]byte(pluginConfig.Config), &configMap); err != nil {
					return fmt.Errorf("解析插件 %s 配置JSON失败: %v", pluginConfig.Name, err)
				}
				
				// 创建插件实例
				plugin, err := pm.pluginFactory.CreatePlugin(pluginConfig.Name, configMap)
				if err != nil {
					return err
				}
				
				// 注册插件
				pm.RegisterPlugin(plugin)
				log.Printf("成功加载插件: %s，聊天ID: %d", pluginConfig.Name, chatConfig.ChatID)
			}
		}
	}

	return nil
}
