// Command migrate 执行数据库 migration。
//
// 用法：
//
//	migrate -config ./configs/config.yaml up
//	migrate up | down | status | version
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zyd16888/telegram-message-forward/internal/config"
	storagemigrate "github.com/zyd16888/telegram-message-forward/internal/storage/migrate"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	var extra []string
	if args := flag.Args(); len(args) > 1 {
		extra = args[1:]
	}
	if err := storagemigrate.Run(context.Background(), db, command, extra...); err != nil {
		log.Fatalf("%v", err)
	}
}
