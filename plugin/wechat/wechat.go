package wechat

import (
	"fmt"
	"github.com/celestix/gotgproto/types"
)

type WeChatPlugin struct {
	// WeChat specific configuration
}

func (w *WeChatPlugin) Handle(message *types.Message) error {
	// Implement WeChat forwarding logic
	fmt.Println("Forwarding message to WeChat:", message.Message)
	return nil
}
