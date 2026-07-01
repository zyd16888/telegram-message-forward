package plugin

import (
	"fmt"

	"telegram-message-forward/plugin/printmsg"
	"telegram-message-forward/plugin/wecom"
)

type PluginFactory interface {
	CreatePlugin(name string, configMap map[string]interface{}) (MessageHandler, error)
}

type DefaultPluginFactory struct{}

func (f *DefaultPluginFactory) CreatePlugin(name string, configMap map[string]interface{}) (MessageHandler, error) {
	switch name {
	case "printmsg":
		return printmsg.NewPrintMSGPlugin(configMap), nil
	case "wecom":
		return wecom.NewWeChatPlugin(configMap), nil
	default:
		return nil, fmt.Errorf("unknown plugin type: %s", name)
	}
}
