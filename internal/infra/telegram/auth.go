package telegram

import (
	"context"
	"errors"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// ErrSignUpRequired 表示该手机号尚未注册 Telegram 账号，v1 不支持注册。
var ErrSignUpRequired = errors.New("该手机号未注册，v1 不支持注册流程")

// Authenticator 通过注入的回调完成 phone / code / 2FA 密码认证。
//
// Password 仅在账号开启两步验证时被调用；Code 会拿到 SendCode 返回的元信息。
type Authenticator struct {
	PhoneFunc    func(ctx context.Context) (string, error)
	CodeFunc     func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error)
	PasswordFunc func(ctx context.Context) (string, error)
}

var _ auth.UserAuthenticator = Authenticator{}

// Phone 返回登录手机号。
func (a Authenticator) Phone(ctx context.Context) (string, error) {
	if a.PhoneFunc == nil {
		return "", errors.New("未提供手机号")
	}
	return a.PhoneFunc(ctx)
}

// Password 返回两步验证密码。
func (a Authenticator) Password(ctx context.Context) (string, error) {
	if a.PasswordFunc == nil {
		return "", errors.New("未提供两步验证密码")
	}
	return a.PasswordFunc(ctx)
}

// Code 返回登录验证码。
func (a Authenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	if a.CodeFunc == nil {
		return "", errors.New("未提供验证码")
	}
	return a.CodeFunc(ctx, sentCode)
}

// AcceptTermsOfService 默认接受服务条款。
func (a Authenticator) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return nil
}

// SignUp v1 不支持注册。
func (a Authenticator) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, ErrSignUpRequired
}

// Login 在客户端已连接的前提下执行登录流程（若尚未登录）。
//
// 需在 client.Run 的回调内调用。SendCode / SignIn / 2FA 由 Flow 统一编排。
func Login(ctx context.Context, client *telegram.Client, a Authenticator) error {
	flow := auth.NewFlow(a, auth.SendCodeOptions{})
	return client.Auth().IfNecessary(ctx, flow)
}

// Status 返回当前授权状态（是否已登录、账号信息）。
func Status(ctx context.Context, client *telegram.Client) (*auth.Status, error) {
	return client.Auth().Status(ctx)
}
