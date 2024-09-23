package wechat

import (
	"fmt"

	"github.com/celestix/gotgproto/types"
)

type WeChatPlugin struct {
	// WeChat specific configuration
	apikey string
}

func NewWeChatPlugin(configMap map[string]interface{}) *WeChatPlugin {
	apikey := configMap["apikey"].(string)
	return &WeChatPlugin{apikey: apikey}
}

func (w *WeChatPlugin) Handle(message *types.Message) error {
	// Implement WeChat forwarding logic
	fmt.Println("WeChat API Key:", w.apikey)
	fmt.Println("Forwarding message to WeChat:", message.Message)
	
	return nil
}
