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

func (pm *PluginManager) LoadPluginsFromDB(db *gorm.DB) error {
	var chatPluginAssociations []models.ChatPluginAssociation
	if err := db.Where("enabled = ?", true).Find(&chatPluginAssociations).Error; err != nil {
		return err
	}

	// 遍历所有启用的聊天插件关联
	for _, association := range chatPluginAssociations {
		var pluginConfig models.PluginConfig
		// 根据关联的插件配置ID查找插件配置
		if err := db.First(&pluginConfig, association.PluginConfigID).Error; err != nil {
			return fmt.Errorf("查找插件配置失败: %v", err)
		}

		// 如果插件配置已启用
		if pluginConfig.Enabled {
			var configMap map[string]interface{}
			// 将插件配置的JSON字符串解析为map
			if err := json.Unmarshal([]byte(pluginConfig.Config), &configMap); err != nil {
				return fmt.Errorf("解析插件配置JSON失败: %v", err)
			}

			// 使用插件工厂创建插件实例
			plugin, err := pm.pluginFactory.CreatePlugin(pluginConfig.Name, configMap)
			if err != nil {
				return err
			}
			// 注册插件到插件管理器
			pm.RegisterPlugin(plugin)
			// 记录成功加载插件的日志
			log.Printf("成功加载插件: %s，聊天ID: %d", pluginConfig.Name, association.ChatConfigID)
		}
	}
	return nil
}
