// Command token 生成或吊销管理 API token。
//
// 用法：
//
//	token -config ./configs/config.yaml create -name my-token
//	token -config ./configs/config.yaml list
//	token -config ./configs/config.yaml revoke -id 3
//
// create 会打印一次明文 token，请立即保存；数据库只存哈希。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	"telegram-message-forward/internal/config"
	"telegram-message-forward/internal/security"
	"telegram-message-forward/internal/storage"
	"telegram-message-forward/internal/storage/repository"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	name := flag.String("name", "default", "token 名称（create 用）")
	id := flag.Int64("id", 0, "token id（revoke 用）")
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		command = "create"
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := storage.Open(cfg.Database)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	repo := repository.NewAPITokenRepository(db)
	ctx := context.Background()

	switch command {
	case "create":
		token, err := generateToken()
		if err != nil {
			log.Fatalf("生成 token 失败: %v", err)
		}
		hash := security.HashToken(token)
		newID, err := repo.Create(ctx, *name, hash)
		if err != nil {
			log.Fatalf("写入 token 失败: %v", err)
		}
		fmt.Printf("已创建 API token（id=%d, name=%s）\n", newID, *name)
		fmt.Printf("明文 token（仅显示一次，请立即保存）：\n%s\n", token)
	case "list":
		tokens, err := repo.List(ctx)
		if err != nil {
			log.Fatalf("查询 token 失败: %v", err)
		}
		fmt.Printf("%-5s %-20s %-10s %s\n", "ID", "NAME", "REVOKED", "CREATED")
		for _, t := range tokens {
			revoked := "no"
			if t.RevokedAt != nil {
				revoked = "yes"
			}
			fmt.Printf("%-5d %-20s %-10s %s\n", t.ID, t.Name, revoked, t.CreatedAt.Format("2006-01-02 15:04"))
		}
	case "revoke":
		if *id == 0 {
			log.Fatal("revoke 需要 -id 参数")
		}
		if err := repo.Revoke(ctx, *id); err != nil {
			log.Fatalf("吊销 token 失败: %v", err)
		}
		fmt.Printf("已吊销 token id=%d\n", *id)
	default:
		log.Fatalf("未知命令: %s（支持 create/list/revoke）", command)
	}
}

// generateToken 生成 32 字节随机 token，十六进制编码。
func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
