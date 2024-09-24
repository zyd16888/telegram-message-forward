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

	for _, association := range chatPluginAssociations {
		var pluginConfig models.PluginConfig
		if err := db.First(&pluginConfig, association.PluginConfigID).Error; err != nil {
			return fmt.Errorf("failed to find plugin config: %v", err)
		}

		if pluginConfig.Enabled {
			var configMap map[string]interface{}
			if err := json.Unmarshal([]byte(pluginConfig.Config), &configMap); err != nil {
				return fmt.Errorf("failed to unmarshal plugin config: %v", err)
			}

			plugin, err := pm.pluginFactory.CreatePlugin(pluginConfig.Name, configMap)
			if err != nil {
				return err
			}
			pm.RegisterPlugin(plugin)
			log.Printf("Successfully loaded plugin: %s for chat ID: %d", pluginConfig.Name, association.ChatConfigID)
		}
	}
	return nil
}
