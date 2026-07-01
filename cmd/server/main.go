// Command server 是 HTTP 服务进程入口。
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/zyd16888/telegram-message-forward/internal/bootstrap"
	"github.com/zyd16888/telegram-message-forward/internal/config"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径，默认在 ./configs/config.yaml 查找")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	app, err := bootstrap.Build(cfg)
	if err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("服务异常退出: %v", err)
	}
}
