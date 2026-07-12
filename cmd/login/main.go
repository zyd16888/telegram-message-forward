// Command login 完成 Telegram 用户账号首次命令行登录（SendCode / SignIn / 2FA）。
//
// 用法：
//
//	# 为已存在账号登录
//	login -config ./configs/config.yaml -account-id 1
//
//	# 新建账号并登录（app-id / app-hash 从 my.telegram.org 获取）
//	login -config ./configs/config.yaml -name my -phone +8613800000000 -app-id 12345 -app-hash xxxx
//
// 登录成功后 session 加密落库，后续监听复用该 session。
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"telegram-message-forward/internal/config"
	domainaccount "telegram-message-forward/internal/domain/account"
	"telegram-message-forward/internal/infra/crypto"
	infratelegram "telegram-message-forward/internal/infra/telegram"
	"telegram-message-forward/internal/storage"
	"telegram-message-forward/internal/storage/repository"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	accountID := flag.Int64("account-id", 0, "已存在账号 id（登录该账号）")
	name := flag.String("name", "", "新账号名称")
	phone := flag.String("phone", "", "手机号（含国家码，如 +8613800000000）")
	appID := flag.Int("app-id", 0, "Telegram app_id")
	appHash := flag.String("app-hash", "", "Telegram app_hash")
	proxyType := flag.String("proxy-type", "", "代理类型 socks5（可选）")
	proxyAddr := flag.String("proxy-addr", "", "代理地址 host:port（可选）")
	proxyUser := flag.String("proxy-user", "", "代理用户名（可选）")
	proxyPass := flag.String("proxy-pass", "", "代理密码（可选）")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	cipher, err := crypto.NewCipher([]byte(cfg.Security.EncryptionKey))
	if err != nil {
		log.Fatalf("初始化加密失败: %v", err)
	}
	db, err := storage.Open(cfg.Database)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取底层数据库连接失败: %v", err)
	}
	ctx := context.Background()
	runtimeLock, acquired, err := storage.TryAcquireRuntimeLock(ctx, sqlDB)
	if err != nil {
		log.Fatalf("获取运行时锁失败: %v", err)
	}
	if !acquired {
		log.Fatal("服务正在运行，请在管理后台完成 Telegram 登录，或先停止服务后再使用 cmd/login")
	}
	defer func() {
		if err := runtimeLock.Release(); err != nil {
			log.Printf("释放运行时锁失败: %v", err)
		}
	}()
	accounts := repository.NewAccountRepository(db, cipher)

	var acc *domainaccount.Account
	if *accountID != 0 {
		acc, err = accounts.GetByID(ctx, *accountID)
		if err != nil {
			log.Fatalf("查询账号失败: %v", err)
		}
		acc.Session = nil
		acc.Status = domainaccount.StatusLoggingIn
		acc.LastError = ""
		if err := accounts.Update(ctx, acc); err != nil {
			log.Fatalf("重置账号登录态失败: %v", err)
		}
	} else {
		if *phone == "" || *appID == 0 || *appHash == "" {
			log.Fatal("新建账号需要 -phone -app-id -app-hash；或使用 -account-id 指定已存在账号")
		}
		acc = &domainaccount.Account{
			Name:        firstNonEmpty(*name, *phone),
			PhoneNumber: *phone,
			AppID:       *appID,
			AppHash:     *appHash,
			Status:      domainaccount.StatusLoggingIn,
			Proxy: domainaccount.ProxyConfig{
				Type: *proxyType, Addr: *proxyAddr, Username: *proxyUser, Password: *proxyPass,
			},
		}
		if err := accounts.Create(ctx, acc); err != nil {
			log.Fatalf("创建账号失败: %v", err)
		}
		fmt.Printf("已创建账号 id=%d\n", acc.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	req := infratelegram.LoginRequest{
		Account: acc,
		Code: func(context.Context) (string, error) {
			fmt.Print("请输入 Telegram 验证码: ")
			return readLine(reader)
		},
		Password: func(context.Context) (string, error) {
			fmt.Print("请输入两步验证密码: ")
			return readLine(reader)
		},
		SaveSession: func(ctx context.Context, plaintext []byte) error {
			acc.Session = plaintext
			return accounts.Update(ctx, acc)
		},
	}

	fmt.Printf("开始登录账号 %s ...\n", maskPhone(acc.PhoneNumber))
	if err := infratelegram.RunLogin(ctx, req); err != nil {
		acc.Status = domainaccount.StatusError
		acc.LastError = err.Error()
		_ = accounts.Update(ctx, acc)
		log.Fatalf("登录失败: %v", err)
	}

	now := time.Now()
	acc.Status = domainaccount.StatusActive
	acc.LastLoginAt = &now
	acc.LastError = ""
	if err := accounts.Update(ctx, acc); err != nil {
		log.Fatalf("更新账号状态失败: %v", err)
	}
	fmt.Println("登录成功，session 已加密保存。")
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// maskPhone 脱敏手机号，仅保留末 4 位。
func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "****" + phone[len(phone)-4:]
}
