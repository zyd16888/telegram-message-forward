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

// func (pm *PluginManager) RegisterPlugin(chatID int64, plugin MessageHandler) {
// 	pm.plugins[chatID] = append(pm.plugins[chatID], plugin)
// }

func (pm *PluginManager) RegisterPlugin(plugins []MessageHandler, plugin MessageHandler) {
	plugins = append(plugins, plugin)
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

// LoadPluginsFromDB 从数据库加载插件配置并初始化插件
func (pm *PluginManager) LoadPluginsFromDB() error {
	// 查询所有聊天配置
	var chatConfigs []models.ChatConfig
	if err := global.DB.Find(&chatConfigs).Error; err != nil {
		return fmt.Errorf("查询Chat配置失败: %v", err)
	}

	// 重置插件列表
	// pm.plugins = make(map[int64][]MessageHandler)

	// 遍历每个聊天配置
	for _, chatConfig := range chatConfigs {
		// 查询该聊天配置下启用的插件关联
		var associations []models.ChatPluginAssociation
		if err := global.DB.Where("chat_config_id = ? AND enabled = ?", chatConfig.ID, true).Find(&associations).Error; err != nil {
			return fmt.Errorf("查询聊天插件关联失败 %v", err)
		}

		chatPlugins := make([]MessageHandler, 0)

		// 遍历每个插件关联
		for _, association := range associations {
			// 查询插件配置
			var pluginConfig models.PluginConfig
			if err := global.DB.First(&pluginConfig, association.PluginConfigID).Error; err != nil {
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
				pm.RegisterPlugin(chatPlugins, plugin)
				log.Printf("成功加载插件: %s，聊天ID: %d", pluginConfig.Name, chatConfig.ChatID)
			}
		}
		pm.plugins[chatConfig.ChatID] = chatPlugins
	}

	return nil
}

func (pm *PluginManager) LoadPluginsForChat(chatID int64) error {

	var chatConfig models.ChatConfig
	if err := global.DB.Where("chat_id = ?", chatID).First(&chatConfig).Error; err != nil {
		return fmt.Errorf("查询聊天配置失败: %v", err)
	}

	var associations []models.ChatPluginAssociation
	if err := global.DB.Where("chat_config_id = ? AND enabled = ?", chatConfig.ID, true).Find(&associations).Error; err != nil {
		return fmt.Errorf("查询聊天插件关联失败 %v", err)
	}

	plugins := make([]MessageHandler, 0)
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

			plugins = append(plugins, plugin)
			log.Printf("成功加载插件: %s，聊天ID: %d", pluginConfig.Name, chatID)
		}
	}

	pm.plugins[chatID] = plugins

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
