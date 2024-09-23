package plugin

import (
	"fmt"

	"github.com/zyd16888/telegram-message-forward/plugin/printmsg"
	"github.com/zyd16888/telegram-message-forward/plugin/wechat"

)

type PluginFactory interface {
	CreatePlugin(name string, configMap map[string]interface{}) (MessageHandler, error)
}

type DefaultPluginFactory struct{}

func (f *DefaultPluginFactory) CreatePlugin(name string, configMap map[string]interface{}) (MessageHandler, error) {
	switch name {
	case "printmsg":
		return printmsg.NewPrintMSGPlugin(configMap), nil
	case "wechat":
		return wechat.NewWeChatPlugin(configMap), nil
	default:
		return nil, fmt.Errorf("unknown plugin type: %s", name)
	}
}