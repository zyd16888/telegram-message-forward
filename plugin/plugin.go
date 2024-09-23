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
	var pluginConfigs []models.PluginConfig
	if err := db.Find(&pluginConfigs).Error; err != nil {
		return err
	}

	for _, config := range pluginConfigs {
		if config.Enabled {
			// config.Config 是 JSON 字符串，需要反序列化为配置项目，然后传入对应的插件进行初始化
			var configMap map[string]interface{}
			if err := json.Unmarshal([]byte(config.Config), &configMap); err != nil {
				return fmt.Errorf("failed to unmarshal plugin config: %v", err)
			}

			plugin, err := pm.pluginFactory.CreatePlugin(config.Name, configMap)
			if err != nil {
				return err
			}
			pm.RegisterPlugin(plugin)
			log.Printf("Successfully loaded plugin: %s", config.Name)
		}
	}
	return nil
}
