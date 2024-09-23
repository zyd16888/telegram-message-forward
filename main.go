package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/dispatcher/handlers/filters"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/td/tg"
	"github.com/zyd16888/telegram-message-forward/global"
)

func main() {

	// 全局初始化
	global.Init()

	client, err := gotgproto.NewClient(
		// Get AppID from https://my.telegram.org/apps
		global.Config.GetInt("app_id"),
		// Get ApiHash from https://my.telegram.org/apps
		global.Config.GetString("app_hash"),
		// ClientType, as we defined above
		gotgproto.ClientTypePhone(global.Config.GetString("phone_number")),
		// Optional parameters of client
		&gotgproto.ClientOpts{
			Session: sessionMaker.SqlSession(sqlite.Open(global.Config.GetString("database"))),
		},
	)
	if err != nil {
		log.Fatalln("failed to start client:", err)
	}

	fmt.Printf("client (@%s) has been started...\n", client.Self.Username)

	clientDispatcher := client.Dispatcher

	// 添加一个处理器来监听所有新消息
	clientDispatcher.AddHandlerToGroup(handlers.NewMessage(filters.Message.Chat(2161625827), handleNewMessage), 1)

	client.Idle()
}

func handleNewMessage(ctx *ext.Context, update *ext.Update) error {
	message := update.EffectiveMessage
	chat, _ := ctx.GetChat(2161625827) // 检查消息是否来自频道
	chatJson, _ := json.Marshal(chat)
	fmt.Println(string(chatJson))
	fmt.Println("----------------------------------------------------")

	channel, err := ctx.Raw.ChannelsGetFullChannel(
		ctx, &tg.InputChannel{
			ChannelID:  2161625827,
			AccessHash: 3276307730339658348,
		})
	if err != nil {
		fmt.Println("Error marshalling to JSON:", err)
	}
	channelJson, _ := json.Marshal(channel)
	fmt.Println(string(channelJson))

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
