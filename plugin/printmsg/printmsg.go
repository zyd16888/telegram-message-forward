package printmsg

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/celestix/gotgproto/types"
)

type PrintMSGPlugin struct{}

func NewPrintMSGPlugin(configMap map[string]interface{}) *PrintMSGPlugin {
	log.Printf("配置：%v", configMap)
	return &PrintMSGPlugin{}
}

func (p *PrintMSGPlugin) Handle(message *types.Message) error {

	fmt.Printf("收到消息---------------------------------\n")

	// 打印消息的关键信息
	if message != nil {
		jsonData, err := json.MarshalIndent(message, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling to JSON:", err)
		}
		// 打印 JSON 格式的消息
		fmt.Println(string(jsonData))
		fmt.Println("消息 ID:", message.ID)
		fmt.Println("来自:", message.FromID)
		fmt.Println("频道 ID:", message.PeerID.String())
		fmt.Println("消息内容:", message.Message.Message)
		fmt.Println("消息日期:", message.Date)

		// 如果消息有实体（如加粗、链接等），也可以解析出来
		if len(message.Entities) > 0 {
			fmt.Println("消息包含的实体:")
			for _, entity := range message.Entities {
				fmt.Printf("- 实体类型: %T\n", entity)
			}
		}

		// 如果消息带有媒体内容
		if message.Media != nil {
			fmt.Println("媒体内容存在")
		}
	} else {
		fmt.Println("未找到有效的消息")
	}

	fmt.Println("----------------------------------------------------")

	return nil
}
