// Command server 是 HTTP 服务进程入口。
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegram-message-forward/internal/bootstrap"
	"telegram-message-forward/internal/config"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径，默认在 ./configs/config.yaml 查找")
	initConfigPath := flag.String("init-config", "", "初始化配置文件后退出；如果文件已存在则只检查是否可启动")
	flag.Parse()

	if *initConfigPath != "" {
		created, err := config.EnsureFile(*initConfigPath)
		if err != nil {
			log.Fatalf("初始化配置失败: %v", err)
		}
		if created {
			log.Printf("已生成配置文件: %s", *initConfigPath)
		}
		if err := config.ValidateFileReady(*initConfigPath); err != nil {
			log.Printf("配置未就绪: %v。请编辑 %s 后重新启动。", err, *initConfigPath)
			os.Exit(2)
		}
		log.Printf("配置已就绪: %s", *initConfigPath)
		return
	}

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
