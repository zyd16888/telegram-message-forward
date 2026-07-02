package telegram

import (
	"context"
	"errors"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	domainaccount "telegram-message-forward/internal/domain/account"
)

// 可读的登录错误分类，供 app 层分支与 UI 展示；不携带敏感明文。
var (
	ErrCodeInvalid     = errors.New("验证码错误")
	ErrCodeExpired     = errors.New("验证码已过期，请重新获取")
	ErrPhoneInvalid    = errors.New("手机号无效")
	ErrPasswordInvalid = errors.New("两步验证密码错误")
	ErrTooManyRequests = errors.New("操作过于频繁，请稍后再试")
)

// LoginFlowConfig 描述执行一步登录所需的客户端配置。
//
// Session 为账号当前 session 明文；SaveSession 在 session 变化时被调用，
// 负责把明文 session 加密落库（加密由存储层完成）。每一步建立一次短连接，
// 依赖 session 中已持久化的 auth key 继续下一步。
type LoginFlowConfig struct {
	AppID       int
	AppHash     string
	Proxy       domainaccount.ProxyConfig
	Session     []byte
	SaveSession func(ctx context.Context, plaintext []byte) error
}

// SendCodeResult 是发送验证码的结果。
type SendCodeResult struct {
	PhoneCodeHash string
}

// SignInResult 是一次登录尝试的结果。
type SignInResult struct {
	Authorized   bool
	NeedPassword bool
}

// LoginFlowService 以分步、可持久化的方式驱动 Telegram 验证码登录。
//
// 与 RunLogin 的同步阻塞式流程不同，本类型把 SendCode / SignIn / 2FA 拆成
// 独立步骤，每步单独建立连接后即断开，适配 UI 分步交互与服务重启恢复。
// gotd/td 完全封装在本层，app/handler 不直接接触。
type LoginFlowService struct{}

// SendCode 发送验证码，返回 phone_code_hash。
func (LoginFlowService) SendCode(ctx context.Context, cfg LoginFlowConfig, phone string) (SendCodeResult, error) {
	var out SendCodeResult
	err := runLoginStep(ctx, cfg, func(ctx context.Context, client *telegram.Client) error {
		sent, err := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		if err != nil {
			return err
		}
		code, ok := sent.(*tg.AuthSentCode)
		if !ok {
			return errors.New("未获得验证码发送结果")
		}
		out.PhoneCodeHash = code.PhoneCodeHash
		return nil
	})
	return out, classifyLoginErr(err)
}

// SignInCode 使用验证码登录。需要 2FA 时 NeedPassword=true。
func (LoginFlowService) SignInCode(ctx context.Context, cfg LoginFlowConfig, phone, code, phoneCodeHash string) (SignInResult, error) {
	var out SignInResult
	err := runLoginStep(ctx, cfg, func(ctx context.Context, client *telegram.Client) error {
		_, err := client.Auth().SignIn(ctx, phone, code, phoneCodeHash)
		if errors.Is(err, auth.ErrPasswordAuthNeeded) {
			out.NeedPassword = true
			return nil
		}
		if err != nil {
			return err
		}
		out.Authorized = true
		return nil
	})
	return out, classifyLoginErr(err)
}

// SignInPassword 完成两步验证。password 只在内存中短暂使用，不落库、不打印。
func (LoginFlowService) SignInPassword(ctx context.Context, cfg LoginFlowConfig, password string) (SignInResult, error) {
	var out SignInResult
	err := runLoginStep(ctx, cfg, func(ctx context.Context, client *telegram.Client) error {
		_, err := client.Auth().Password(ctx, password)
		if err != nil {
			return err
		}
		out.Authorized = true
		return nil
	})
	return out, classifyLoginErr(err)
}

// runLoginStep 建立一次短连接并执行单步登录操作。
func runLoginStep(ctx context.Context, cfg LoginFlowConfig, fn func(context.Context, *telegram.Client) error) error {
	store := &memorySession{data: cfg.Session, save: cfg.SaveSession}
	client, err := NewClient(ClientConfig{
		AppID:        cfg.AppID,
		AppHash:      cfg.AppHash,
		Proxy:        cfg.Proxy,
		SessionStore: store,
	})
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return client.Run(runCtx, func(ctx context.Context) error {
		return fn(ctx, client)
	})
}

// classifyLoginErr 把 Telegram RPC 错误映射为可读的中文错误；未知错误原样返回。
func classifyLoginErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case tgerr.Is(err, "PHONE_CODE_INVALID"):
		return ErrCodeInvalid
	case tgerr.Is(err, "PHONE_CODE_EXPIRED"):
		return ErrCodeExpired
	case tgerr.Is(err, "PHONE_NUMBER_INVALID"):
		return ErrPhoneInvalid
	case tgerr.Is(err, "PASSWORD_HASH_INVALID"):
		return ErrPasswordInvalid
	}
	if _, ok := tgerr.AsFloodWait(err); ok {
		return ErrTooManyRequests
	}
	return err
}
