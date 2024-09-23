package plugin

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/celestix/gotgproto/types"
	"gorm.io/gorm"

	"github.com/zyd16888/telegram-message-forward/models"
	"github.com/zyd16888/telegram-message-forward/plugin/printmsg"
	"github.com/zyd16888/telegram-message-forward/plugin/wechat"
)

type MessageHandler interface {
	Handle(message *types.Message) error
}

type PluginManager struct {
	plugins []MessageHandler
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make([]MessageHandler, 0),
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

			// 根据插件类型进行初始化
			switch config.Name {
			case "printmsg":
				plugin := printmsg.NewPrintMSGPlugin(configMap)
				pm.RegisterPlugin(plugin)
				log.Printf("Successfully loaded plugin: %s", config.Name)
			case "wechat":
				plugin := wechat.NewWeChatPlugin(configMap)
				pm.RegisterPlugin(plugin)
				log.Printf("Successfully loaded plugin: %s", config.Name)
			default:
				return fmt.Errorf("unknown plugin type: %s", config.Name)
			}
		}
	}
	return nil
}
