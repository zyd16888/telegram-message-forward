package main

import (
	"fmt"
	"log"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/dispatcher/handlers/filters"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"

	"github.com/zyd16888/telegram-message-forward/global"
	"github.com/zyd16888/telegram-message-forward/plugin"
	"github.com/zyd16888/telegram-message-forward/server"
)

func main() {
	// 全局初始化
	global.Init()

	// 创建插件工厂
	pluginFactory := &plugin.DefaultPluginFactory{}

	// 使用工厂创建 PluginManager
	pluginManager := plugin.NewPluginManager(pluginFactory)

	// 从数据库加载插件配置
	if err := pluginManager.LoadPlugins(0); err != nil {
		log.Fatalf("Failed to load plugins from database: %v", err)
	}

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
	clientDispatcher.AddHandler(handlers.NewMessage(filters.Message.All, pluginManager.CentralHandler))

	go server.InitServer(pluginManager)

	defer client.Idle()
}
