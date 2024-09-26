package plugin

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/types"
	"github.com/gotd/td/tg"

	"github.com/zyd16888/telegram-message-forward/global"
	"github.com/zyd16888/telegram-message-forward/models"
)

type MessageHandler interface {
	Handle(message *types.Message) error
}

type PluginManager struct {
	plugins       map[int64][]MessageHandler
	pluginFactory PluginFactory
}

func NewPluginManager(factory PluginFactory) *PluginManager {
	return &PluginManager{
		plugins:       make(map[int64][]MessageHandler),
		pluginFactory: factory,
	}
}

func (pm *PluginManager) GetPlugins() map[int64][]MessageHandler {
	return pm.plugins
}

func (pm *PluginManager) RegisterPlugin(plugins *[]MessageHandler, plugin MessageHandler) {
	*plugins = append(*plugins, plugin)
}

func (pm *PluginManager) HandleMessage(chatID int64, message *types.Message) error {
	plugins, ok := pm.plugins[chatID]
	if !ok {
		return fmt.Errorf("未找到聊天ID %d 的插件", chatID)
	}

	for _, plugin := range plugins {
		if err := plugin.Handle(message); err != nil {
			return err
		}
	}
	return nil
}

// 从数据库加载插件配置并初始化插件
func (pm *PluginManager) LoadPlugins(chatID int64) error {
	var chatConfigs []models.ChatConfig
	db := global.DB

	if chatID != 0 {
		if err := db.Where(models.ChatConfig{ChatID: chatID}).Find(&chatConfigs).Error; err != nil {
			return fmt.Errorf("查询Chat配置失败: %v", err)
		}
	} else {
		if err := db.Find(&chatConfigs).Error; err != nil {
			return fmt.Errorf("查询Chat配置失败: %v", err)
		}
	}

	for _, chatConfig := range chatConfigs {
		var associations []models.ChatPluginAssociation

		if err := global.DB.Where(models.ChatPluginAssociation{ChatConfigID: chatConfig.ID, Enabled: true}).Find(&associations).Error; err != nil {
			return fmt.Errorf("查询聊天插件关联失败 %v", err)
		}

		chatPlugins := make([]MessageHandler, 0)
		for _, association := range associations {
			var pluginConfig models.PluginConfig
			if err := global.DB.First(&pluginConfig, association.PluginConfigID).Error; err != nil {
				return fmt.Errorf("查询插件配置失败: %v", err)
			}

			if pluginConfig.Enabled {
				var configMap map[string]interface{}
				if err := json.Unmarshal([]byte(pluginConfig.Config), &configMap); err != nil {
					return fmt.Errorf("解析插件配置JSON失败: %v", err)
				}

				plugin, err := pm.pluginFactory.CreatePlugin(pluginConfig.Name, configMap)
				if err != nil {
					return err
				}
				// 注册插件
				pm.RegisterPlugin(&chatPlugins, plugin)
				log.Printf("成功加载插件: %s，聊天ID: %d", pluginConfig.Name, chatConfig.ChatID)
			}
		}

		pm.plugins[chatConfig.ChatID] = chatPlugins
	}

	return nil
}

func (pm *PluginManager) CentralHandler(ctx *ext.Context, update *ext.Update) error {
	message := update.EffectiveMessage
	if message == nil {
		return nil
	}

	chatID := message.GetPeerID().(*tg.PeerChannel).ChannelID
	plugins, ok := pm.plugins[chatID]
	if !ok {
		return nil
	}

	for _, plugin := range plugins {
		if err := plugin.Handle(message); err != nil {
			log.Printf("处理消息失败: %v", err)
		}
	}

	return nil
}
