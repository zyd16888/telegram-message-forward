package plugin

import (
	"github.com/celestix/gotgproto/types"
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
