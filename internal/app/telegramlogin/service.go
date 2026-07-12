// Package telegramlogin 提供 Telegram 账号登录 flow 的应用编排。
//
// 服务负责：登录状态机、flow 持久化、账号状态更新与 session 保存。
// gotd/td 通过 infra/telegram 的 flowRunner 接口注入，本层与 handler 均不接触
// gotd/td 类型。验证码明文与 2FA 密码只在内存中短暂使用，不落库、不打印。
package telegramlogin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainloginflow "telegram-message-forward/internal/domain/loginflow"
	"telegram-message-forward/internal/infra/clock"
	infratelegram "telegram-message-forward/internal/infra/telegram"
)

// 默认 flow 过期时间。
const defaultFlowTTL = 10 * time.Minute

// 应用层错误。
var (
	// ErrFlowExpired 表示登录 flow 已过期，需重新发起。
	ErrFlowExpired = errors.New("登录流程已过期，请重新发起")
	// ErrWrongStep 表示当前步骤不允许该操作。
	ErrWrongStep = errors.New("当前登录步骤不允许该操作")
	// ErrWrongMethod 表示 flow 登录方式与操作不匹配。
	ErrWrongMethod = errors.New("登录方式不匹配")
	// ErrFlowAccountMismatch 表示 flow 不属于该账号，禁止跨账号操作。
	ErrFlowAccountMismatch = errors.New("登录流程与账号不匹配")
)

// flowRunner 抽象 Telegram 验证码分步登录操作，便于测试注入 fake。
// 实现见 infra/telegram.LoginFlowService。
type flowRunner interface {
	SendCode(ctx context.Context, cfg infratelegram.LoginFlowConfig, phone string) (infratelegram.SendCodeResult, error)
	SignInCode(ctx context.Context, cfg infratelegram.LoginFlowConfig, phone, code, phoneCodeHash string) (infratelegram.SignInResult, error)
	SignInPassword(ctx context.Context, cfg infratelegram.LoginFlowConfig, password string) (infratelegram.SignInResult, error)
}

// qrRunner 抽象 Telegram 扫码登录后台会话，便于测试注入 fake。
// 实现见 infra/telegram.QRSessionManager：QRStart 建立长连接并导出首个
// token，之后在后台监听扫码确认；QRCheck 只读取会话内存快照，不重新
// export、不刷新 token；QRCancel 停止并清理会话。
type qrRunner interface {
	QRStart(ctx context.Context, cfg infratelegram.LoginFlowConfig, sessionID string) (infratelegram.QRExportResult, error)
	QRCheck(ctx context.Context, sessionID string) (infratelegram.QRCheckResult, error)
	QRCancel(sessionID string)
}

type connectionController interface {
	StopAccount(ctx context.Context, accountID int64) error
}

// Service 是 Telegram 登录应用服务。
type Service struct {
	accounts    domainaccount.Repository
	flows       domainloginflow.Repository
	runner      flowRunner
	qr          qrRunner
	clk         clock.Clock
	log         *slog.Logger
	ttl         time.Duration
	connections connectionController
}

// UseConnectionController 在重新登录前停止账号现有连接，避免同 session 并行使用。
func (s *Service) UseConnectionController(controller connectionController) *Service {
	s.connections = controller
	return s
}

// NewService 创建登录服务。runner 处理验证码登录，qr 处理扫码登录。
func NewService(accounts domainaccount.Repository, flows domainloginflow.Repository, runner flowRunner, qr qrRunner, clk clock.Clock, log *slog.Logger) *Service {
	return &Service{
		accounts: accounts,
		flows:    flows,
		runner:   runner,
		qr:       qr,
		clk:      clk,
		log:      log,
		ttl:      defaultFlowTTL,
	}
}

// Start 发起某账号的手机验证码登录，返回当前 flow。
func (s *Service) Start(ctx context.Context, accountID int64) (*domainloginflow.Flow, error) {
	acc, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	// 取消该账号已有的进行中 flow，避免并发悬挂。
	if existing, _ := s.flows.GetActiveByAccount(ctx, accountID); existing != nil {
		existing.Status = domainloginflow.StatusCancelled
		clearSensitive(existing)
		_ = s.flows.Update(ctx, existing)
	}

	if err := s.prepareFreshLogin(ctx, acc); err != nil {
		return nil, err
	}

	now := s.clk.Now()
	flow := &domainloginflow.Flow{
		FlowID:      newFlowID(),
		AccountID:   accountID,
		Method:      domainloginflow.MethodPhoneCode,
		Status:      domainloginflow.StatusSendingCode,
		CurrentStep: string(domainloginflow.StatusSendingCode),
		ExpiresAt:   now.Add(s.ttl),
	}
	if err := s.flows.Create(ctx, flow); err != nil {
		return nil, err
	}

	res, err := s.runner.SendCode(ctx, s.buildConfig(acc), acc.PhoneNumber)
	if err != nil {
		s.failFlow(ctx, flow, err)
		s.markAccountError(ctx, accountID, err)
		return flow, err
	}

	flow.PhoneCodeHash = res.PhoneCodeHash
	flow.Status = domainloginflow.StatusCodeRequired
	flow.CurrentStep = string(domainloginflow.StatusCodeRequired)
	flow.LastError = ""
	if err := s.flows.Update(ctx, flow); err != nil {
		return nil, err
	}
	return flow, nil
}

// SubmitCode 提交验证码。需要 2FA 时进入 password_required。
func (s *Service) SubmitCode(ctx context.Context, accountID int64, flowID, code string) (*domainloginflow.Flow, error) {
	flow, err := s.loadFlowForAccount(ctx, accountID, flowID)
	if err != nil {
		return flow, err
	}
	if flow.Method != domainloginflow.MethodPhoneCode {
		return flow, ErrWrongMethod
	}
	if s.isExpired(flow) {
		return s.markExpired(ctx, flow), ErrFlowExpired
	}
	if flow.Status != domainloginflow.StatusCodeRequired {
		return flow, ErrWrongStep
	}

	acc, err := s.accounts.GetByID(ctx, flow.AccountID)
	if err != nil {
		return nil, err
	}
	res, err := s.runner.SignInCode(ctx, s.buildConfig(acc), acc.PhoneNumber, code, flow.PhoneCodeHash)
	if err != nil {
		// 验证码过期需重新发起；其它错误保持 code_required 允许重试。
		if errors.Is(err, infratelegram.ErrCodeExpired) {
			return s.markExpired(ctx, flow), err
		}
		flow.LastError = err.Error()
		_ = s.flows.Update(ctx, flow)
		return flow, err
	}
	if res.NeedPassword {
		flow.Status = domainloginflow.StatusPasswordRequired
		flow.CurrentStep = string(domainloginflow.StatusPasswordRequired)
		flow.LastError = ""
		if err := s.flows.Update(ctx, flow); err != nil {
			return nil, err
		}
		return flow, nil
	}
	return s.finalize(ctx, flow)
}

// SubmitPassword 提交两步验证密码。password 不落库、不打印。
func (s *Service) SubmitPassword(ctx context.Context, accountID int64, flowID, password string) (*domainloginflow.Flow, error) {
	flow, err := s.loadFlowForAccount(ctx, accountID, flowID)
	if err != nil {
		return flow, err
	}
	if flow.Method != domainloginflow.MethodPhoneCode {
		return flow, ErrWrongMethod
	}
	if s.isExpired(flow) {
		return s.markExpired(ctx, flow), ErrFlowExpired
	}
	if flow.Status != domainloginflow.StatusPasswordRequired {
		return flow, ErrWrongStep
	}

	acc, err := s.accounts.GetByID(ctx, flow.AccountID)
	if err != nil {
		return nil, err
	}
	res, err := s.runner.SignInPassword(ctx, s.buildConfig(acc), password)
	if err != nil {
		flow.LastError = err.Error()
		_ = s.flows.Update(ctx, flow)
		return flow, err
	}
	if !res.Authorized {
		flow.LastError = "两步验证未通过"
		_ = s.flows.Update(ctx, flow)
		return flow, errors.New(flow.LastError)
	}
	return s.finalize(ctx, flow)
}

// Status 返回 flow 当前状态；已过期的非终态 flow 会被标记为 expired。
func (s *Service) Status(ctx context.Context, flowID string) (*domainloginflow.Flow, error) {
	flow, err := s.flows.GetByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	if s.isExpired(flow) {
		return s.markExpired(ctx, flow), nil
	}
	return flow, nil
}

// GetActiveByAccount 返回某账号最近的非终态 flow，供 UI 恢复展示；无则返回 nil,nil。
func (s *Service) GetActiveByAccount(ctx context.Context, accountID int64) (*domainloginflow.Flow, error) {
	flow, err := s.flows.GetActiveByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if flow == nil {
		return nil, nil
	}
	if s.isExpired(flow) {
		return s.markExpired(ctx, flow), nil
	}
	s.attachQRURL(flow)
	return flow, nil
}

// Cancel 取消一个进行中的 flow。
func (s *Service) Cancel(ctx context.Context, accountID int64, flowID string) (*domainloginflow.Flow, error) {
	flow, err := s.loadFlowForAccount(ctx, accountID, flowID)
	if err != nil {
		return flow, err
	}
	if flow.Status.IsTerminal() {
		return flow, nil
	}
	if flow.Method == domainloginflow.MethodQR && s.qr != nil {
		// 停止后台监听会话，避免遗留 goroutine 与长连接。
		s.qr.QRCancel(flow.FlowID)
	}
	flow.Status = domainloginflow.StatusCancelled
	clearSensitive(flow)
	if err := s.flows.Update(ctx, flow); err != nil {
		return nil, err
	}
	// 登录中断：若账号仍处于 logging_in，回退为 inactive。
	if acc, err := s.accounts.GetByID(ctx, flow.AccountID); err == nil && acc.Status == domainaccount.StatusLoggingIn {
		acc.Status = domainaccount.StatusInactive
		_ = s.accounts.Update(ctx, acc)
	}
	return flow, nil
}

// RecoverStale 在服务启动时将超期的非终态 flow 标记为 expired 并清空敏感字段。
func (s *Service) RecoverStale(ctx context.Context) error {
	n, err := s.flows.ExpireStale(ctx, s.clk.Now())
	if err != nil {
		return err
	}
	if n > 0 && s.log != nil {
		s.log.Info("清理过期登录 flow", "count", n)
	}
	return nil
}

// StartQR 发起某账号的扫码登录，返回带 qr_url 的 flow。
func (s *Service) StartQR(ctx context.Context, accountID int64) (*domainloginflow.Flow, error) {
	acc, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if existing, _ := s.flows.GetActiveByAccount(ctx, accountID); existing != nil {
		if existing.Method == domainloginflow.MethodQR && s.qr != nil {
			s.qr.QRCancel(existing.FlowID)
		}
		existing.Status = domainloginflow.StatusCancelled
		clearSensitive(existing)
		_ = s.flows.Update(ctx, existing)
	}

	if err := s.prepareFreshLogin(ctx, acc); err != nil {
		return nil, err
	}

	now := s.clk.Now()
	flow := &domainloginflow.Flow{
		FlowID:      newFlowID(),
		AccountID:   accountID,
		Method:      domainloginflow.MethodQR,
		Status:      domainloginflow.StatusWaitingScan,
		CurrentStep: string(domainloginflow.StatusWaitingScan),
		ExpiresAt:   now.Add(s.ttl),
	}
	if err := s.flows.Create(ctx, flow); err != nil {
		return nil, err
	}

	res, err := s.qr.QRStart(ctx, s.buildConfig(acc), flow.FlowID)
	if err != nil {
		s.failFlow(ctx, flow, err)
		s.markAccountError(ctx, accountID, err)
		return flow, err
	}
	s.applyQRToken(flow, res.Token, res.ExpiresAt)
	flow.Status = domainloginflow.StatusWaitingScan
	flow.CurrentStep = string(domainloginflow.StatusWaitingScan)
	flow.LastError = ""
	if err := s.flows.Update(ctx, flow); err != nil {
		return nil, err
	}
	s.attachQRURL(flow)
	return flow, nil
}

// QRStatus 读取扫码登录会话状态：成功则完成登录；过期或后台会话已丢失
// （例如服务重启）则进入 qr_refresh_required。不会因轮询而重新导出 token，
// 因为状态直接读取 infra 层长连接会话的内存快照。
func (s *Service) QRStatus(ctx context.Context, accountID int64, flowID string) (*domainloginflow.Flow, error) {
	flow, err := s.loadFlowForAccount(ctx, accountID, flowID)
	if err != nil {
		return flow, err
	}
	if flow.Method != domainloginflow.MethodQR {
		return flow, ErrWrongMethod
	}
	if flow.Status.IsTerminal() {
		return flow, nil
	}
	if s.isExpired(flow) {
		return s.markExpired(ctx, flow), nil
	}

	res, err := s.qr.QRCheck(ctx, flow.FlowID)
	if err != nil {
		if errors.Is(err, infratelegram.ErrQRSessionNotFound) {
			flow.Status = domainloginflow.StatusQRRefreshRequired
			flow.CurrentStep = string(domainloginflow.StatusQRRefreshRequired)
			flow.LastError = ""
			if err := s.flows.Update(ctx, flow); err != nil {
				return nil, err
			}
			return flow, nil
		}
		flow.LastError = err.Error()
		_ = s.flows.Update(ctx, flow)
		s.attachQRURL(flow)
		return flow, err
	}
	switch {
	case res.Authorized:
		if res.MigrateDC > 0 {
			flow.DCID = res.MigrateDC
		}
		return s.finalize(ctx, flow)
	case res.Expired:
		flow.Status = domainloginflow.StatusQRRefreshRequired
		flow.CurrentStep = string(domainloginflow.StatusQRRefreshRequired)
		if err := s.flows.Update(ctx, flow); err != nil {
			return nil, err
		}
		return flow, nil
	default: // waiting：token 未变化，仅刷新 updated_at 与 qr_url。
		flow.Status = domainloginflow.StatusWaitingScan
		flow.CurrentStep = string(domainloginflow.StatusWaitingScan)
		flow.LastError = ""
		if err := s.flows.Update(ctx, flow); err != nil {
			return nil, err
		}
		s.attachQRURL(flow)
		return flow, nil
	}
}

// RefreshQR 丢弃旧的后台会话，重新发起一次扫码登录会话，重置等待状态与过期时间。
func (s *Service) RefreshQR(ctx context.Context, accountID int64, flowID string) (*domainloginflow.Flow, error) {
	flow, err := s.loadFlowForAccount(ctx, accountID, flowID)
	if err != nil {
		return flow, err
	}
	if flow.Method != domainloginflow.MethodQR {
		return flow, ErrWrongMethod
	}
	if flow.Status == domainloginflow.StatusAuthorized {
		return flow, ErrWrongStep
	}

	acc, err := s.accounts.GetByID(ctx, flow.AccountID)
	if err != nil {
		return nil, err
	}
	s.qr.QRCancel(flow.FlowID)
	res, err := s.qr.QRStart(ctx, s.buildConfig(acc), flow.FlowID)
	if err != nil {
		flow.LastError = err.Error()
		_ = s.flows.Update(ctx, flow)
		return flow, err
	}
	s.applyQRToken(flow, res.Token, res.ExpiresAt)
	flow.Status = domainloginflow.StatusWaitingScan
	flow.CurrentStep = string(domainloginflow.StatusWaitingScan)
	flow.LastError = ""
	flow.ExpiresAt = s.clk.Now().Add(s.ttl)
	if err := s.flows.Update(ctx, flow); err != nil {
		return nil, err
	}
	s.attachQRURL(flow)
	return flow, nil
}

// applyQRToken 更新 flow 的 QR token 与短过期时间。
func (s *Service) applyQRToken(flow *domainloginflow.Flow, token []byte, expiresAt time.Time) {
	flow.QRToken = token
	flow.QRTokenExpiresAt = expiresAt
}

// attachQRURL 为等待中的 QR flow 即时构造 qr_url（不落库、不持久化）。
func (s *Service) attachQRURL(flow *domainloginflow.Flow) {
	if flow.Method == domainloginflow.MethodQR && len(flow.QRToken) > 0 && flow.Status == domainloginflow.StatusWaitingScan {
		flow.QRURL = infratelegram.QRTokenURL(flow.QRToken)
	}
}

// finalize 完成登录：账号置 active，flow 置 authorized，清空敏感字段。
func (s *Service) finalize(ctx context.Context, flow *domainloginflow.Flow) (*domainloginflow.Flow, error) {
	now := s.clk.Now()
	// 重新加载账号，拿到 SaveSession 已保存的授权 session，避免用旧值覆盖。
	acc, err := s.accounts.GetByID(ctx, flow.AccountID)
	if err != nil {
		return nil, err
	}
	acc.Status = domainaccount.StatusActive
	acc.LastLoginAt = &now
	acc.LastError = ""
	if err := s.accounts.Update(ctx, acc); err != nil {
		return nil, err
	}

	flow.Status = domainloginflow.StatusAuthorized
	flow.CurrentStep = string(domainloginflow.StatusAuthorized)
	flow.LastError = ""
	flow.CompletedAt = &now
	clearSensitive(flow)
	if err := s.flows.Update(ctx, flow); err != nil {
		return nil, err
	}
	return flow, nil
}

// buildConfig 构建一步登录所需的客户端配置。
func (s *Service) buildConfig(acc *domainaccount.Account) infratelegram.LoginFlowConfig {
	accountID := acc.ID
	return infratelegram.LoginFlowConfig{
		AppID:   acc.AppID,
		AppHash: acc.AppHash,
		Proxy:   acc.Proxy,
		Session: acc.Session,
		SaveSession: func(ctx context.Context, plaintext []byte) error {
			// 重新加载账号后仅更新 session，避免覆盖其它并发变更。
			fresh, err := s.accounts.GetByID(ctx, accountID)
			if err != nil {
				return err
			}
			fresh.Session = plaintext
			return s.accounts.Update(ctx, fresh)
		},
	}
}

// prepareFreshLogin 停止旧连接并丢弃旧 auth key。每次新登录都从全新 session 开始。
func (s *Service) prepareFreshLogin(ctx context.Context, acc *domainaccount.Account) error {
	if s.connections != nil {
		if err := s.connections.StopAccount(ctx, acc.ID); err != nil {
			return fmt.Errorf("停止账号现有连接失败: %w", err)
		}
	}
	acc.Session = nil
	acc.Status = domainaccount.StatusLoggingIn
	acc.LastError = ""
	return s.accounts.Update(ctx, acc)
}

// loadFlowForAccount 加载 flow 并校验其属于 accountID，避免跨账号误操作。
// 不匹配时不返回 flow，避免向调用方泄露其它账号的登录状态。
func (s *Service) loadFlowForAccount(ctx context.Context, accountID int64, flowID string) (*domainloginflow.Flow, error) {
	flow, err := s.flows.GetByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	if flow.AccountID != accountID {
		return nil, ErrFlowAccountMismatch
	}
	return flow, nil
}

func (s *Service) isExpired(flow *domainloginflow.Flow) bool {
	return !flow.Status.IsTerminal() && s.clk.Now().After(flow.ExpiresAt)
}

func (s *Service) markExpired(ctx context.Context, flow *domainloginflow.Flow) *domainloginflow.Flow {
	flow.Status = domainloginflow.StatusExpired
	clearSensitive(flow)
	_ = s.flows.Update(ctx, flow)
	return flow
}

func (s *Service) failFlow(ctx context.Context, flow *domainloginflow.Flow, cause error) {
	flow.Status = domainloginflow.StatusFailed
	flow.LastError = cause.Error()
	clearSensitive(flow)
	_ = s.flows.Update(ctx, flow)
}

func (s *Service) markAccountError(ctx context.Context, accountID int64, cause error) {
	acc, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return
	}
	acc.Status = domainaccount.StatusError
	acc.LastError = cause.Error()
	if errors.Is(cause, infratelegram.ErrSessionDuplicated) {
		acc.Session = nil
	}
	_ = s.accounts.Update(ctx, acc)
}

// clearSensitive 清空 flow 的敏感字段。
func clearSensitive(flow *domainloginflow.Flow) {
	flow.PhoneCodeHash = ""
	flow.QRToken = nil
	flow.QRTokenExpiresAt = time.Time{}
	flow.QRURL = ""
}

// newFlowID 生成随机 flow id。
func newFlowID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// 确保 infra 实现满足验证码与扫码 runner 接口。
var (
	_ flowRunner = infratelegram.LoginFlowService{}
	_ qrRunner   = (*infratelegram.QRSessionManager)(nil)
)
