package telegram

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

// ErrQRSessionNotFound 表示指定的扫码登录后台会话不存在或已结束（例如服务重启后
// 内存态丢失）。调用方应引导用户刷新二维码重新发起。
var ErrQRSessionNotFound = errors.New("扫码登录会话不存在，请刷新二维码")

// QRExportResult 是启动一次扫码登录会话后，首个 token 的导出结果。Token 为敏感字段。
type QRExportResult struct {
	Token     []byte
	URL       string
	ExpiresAt time.Time
}

// QRCheckResult 是一次 QR 会话状态快照。
//
// 语义互斥：Authorized / Waiting / Expired 只会命中其一。
type QRCheckResult struct {
	// Authorized 表示扫码已确认并完成登录（含可能发生的 DC 迁移）。
	Authorized bool
	// Waiting 表示仍在等待扫码；token 未变化，不会因轮询而刷新。
	Waiting bool
	// Expired 表示 token 已过期，需要调用方发起刷新。
	Expired bool
	// MigrateDC 大于 0 表示本次登录过程中发生了真实的 DC 迁移（已由 gotd 完成迁移与 token 导入）。
	MigrateDC int
	// Err 记录会话内部失败原因（非 pending 语义的真实错误）。
	Err error
}

// qrExportOutcome 是 QRStart 等待的首个导出结果。
type qrExportOutcome struct {
	res QRExportResult
	err error
}

// qrSession 是一个后台扫码登录会话的共享状态；由 runSession 的 goroutine 写入，
// QRCheck 只读快照，不触碰网络、不刷新 token。
type qrSession struct {
	id     string
	mu     sync.Mutex
	result QRCheckResult
	cancel context.CancelFunc
}

func (s *qrSession) snapshot() QRCheckResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

func (s *qrSession) set(r QRCheckResult) {
	s.mu.Lock()
	s.result = r
	s.mu.Unlock()
}

// QRSessionManager 管理进行中的扫码登录会话。
//
// 每个会话对应一条持久连接，导出 token 后监听 Telegram 推送的
// updateLoginToken，只在 token 真正过期或收到扫码确认推送时才再次访问
// Telegram，从而避免 HTTP 轮询导致 token 被频繁替换。DC migrate 由
// gotd 的 client.MigrateTo 真实执行，不只是记录 dc_id。
//
// 会话仅保存在进程内存中：服务重启后所有会话丢失，QRCheck 返回
// ErrQRSessionNotFound，由 app 层将 flow 置为 qr_refresh_required，
// 引导用户刷新二维码——这是 HTTP 轮询式 UI 适配 gotd 推送语义的方式。
type QRSessionManager struct {
	mu       sync.Mutex
	sessions map[string]*qrSession
}

// NewQRSessionManager 创建会话管理器。
func NewQRSessionManager() *QRSessionManager {
	return &QRSessionManager{sessions: map[string]*qrSession{}}
}

// QRStart 发起一次扫码登录会话：建立长连接、导出首个 token，并在后台持续监听
// 扫码确认，直到过期、完成或被取消。sessionID 通常为登录 flow 的 flow_id。
//
// 返回值只包含首次导出的 token/url/expires；后续状态通过 QRCheck 读取内存
// 快照，不会因轮询而重新 export。
func (m *QRSessionManager) QRStart(ctx context.Context, cfg LoginFlowConfig, sessionID string) (QRExportResult, error) {
	m.QRCancel(sessionID)

	sessCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	sess := &qrSession{id: sessionID, cancel: cancel}
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	ready := make(chan qrExportOutcome, 1)
	go m.runSession(sessCtx, cancel, cfg, sess, ready)

	select {
	case out := <-ready:
		if out.err != nil {
			m.QRCancel(sessionID)
			return QRExportResult{}, classifyLoginErr(out.err)
		}
		return out.res, nil
	case <-ctx.Done():
		m.QRCancel(sessionID)
		return QRExportResult{}, ctx.Err()
	}
}

// QRCheck 读取会话当前状态快照；不发起任何 Telegram 请求，不会刷新 token。
// 会话不存在（未发起、已取消或服务重启后丢失）时返回 ErrQRSessionNotFound。
func (m *QRSessionManager) QRCheck(_ context.Context, sessionID string) (QRCheckResult, error) {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return QRCheckResult{}, ErrQRSessionNotFound
	}

	res := sess.snapshot()
	if res.Authorized || res.Expired || res.Err != nil {
		// 终态：会话已无用，清理内存。
		m.QRCancel(sessionID)
	}
	if res.Err != nil {
		return QRCheckResult{}, classifyLoginErr(res.Err)
	}
	return res, nil
}

// QRCancel 停止并清理指定会话；会话不存在时忽略。
func (m *QRSessionManager) QRCancel(sessionID string) {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
	if ok {
		sess.cancel()
	}
}

// scheduleCleanup 在会话进入终态一段时间后，若调用方（QRCheck/QRCancel）
// 一直未再访问，主动从内存中移除，避免被放弃的登录尝试无限占用内存。
func (m *QRSessionManager) scheduleCleanup(sess *qrSession) {
	time.AfterFunc(2*time.Minute, func() {
		m.mu.Lock()
		if cur, ok := m.sessions[sess.id]; ok && cur == sess {
			delete(m.sessions, sess.id)
		}
		m.mu.Unlock()
	})
}

// runSession 建立一条长连接，导出 token 后阻塞等待扫码确认推送
// （updateLoginToken）或过期，期间不会重新 export。DC migrate 由
// client.MigrateTo 真实处理：迁移成功后在新 DC 上完成 AuthImportLoginToken。
func (m *QRSessionManager) runSession(ctx context.Context, cancel context.CancelFunc, cfg LoginFlowConfig, sess *qrSession, ready chan<- qrExportOutcome) {
	defer cancel()
	store := &memorySession{data: cfg.Session, save: cfg.SaveSession}

	loggedIn := make(chan struct{}, 1)
	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnLoginToken(func(context.Context, tg.Entities, *tg.UpdateLoginToken) error {
		select {
		case loggedIn <- struct{}{}:
		default:
		}
		return nil
	})

	client, err := NewClient(ClientConfig{
		AppID:         cfg.AppID,
		AppHash:       cfg.AppHash,
		Proxy:         cfg.Proxy,
		SessionStore:  store,
		UpdateHandler: dispatcher,
	})
	if err != nil {
		ready <- qrExportOutcome{err: err}
		return
	}

	sent := false
	_ = client.Run(ctx, func(runCtx context.Context) error {
		var migratedDC int
		migrate := func(mctx context.Context, dcID int) error {
			migratedDC = dcID
			return client.MigrateTo(mctx, dcID)
		}
		q := qrlogin.NewQR(client.API(), cfg.AppID, cfg.AppHash, qrlogin.Options{Migrate: migrate})

		token, err := exportWithMigrate(runCtx, q, migrate)
		if err != nil {
			ready <- qrExportOutcome{err: err}
			sent = true
			return nil
		}

		raw, err := base64.URLEncoding.DecodeString(token.String())
		if err != nil {
			ready <- qrExportOutcome{err: err}
			sent = true
			return nil
		}

		ready <- qrExportOutcome{res: QRExportResult{Token: raw, URL: token.URL(), ExpiresAt: token.Expires()}}
		sent = true
		sess.set(QRCheckResult{Waiting: true})

		timer := time.NewTimer(time.Until(token.Expires()))
		defer timer.Stop()

		select {
		case <-runCtx.Done():
			return nil
		case <-timer.C:
			sess.set(QRCheckResult{Expired: true})
			m.scheduleCleanup(sess)
			return nil
		case <-loggedIn:
		}

		if _, err := q.Import(runCtx); err != nil {
			if tgerr.Is(err, "AUTH_TOKEN_EXPIRED", "AUTH_TOKEN_INVALID") {
				sess.set(QRCheckResult{Expired: true})
				m.scheduleCleanup(sess)
				return nil
			}
			sess.set(QRCheckResult{Err: err})
			m.scheduleCleanup(sess)
			return nil
		}
		sess.set(QRCheckResult{Authorized: true, MigrateDC: migratedDC})
		m.scheduleCleanup(sess)
		return nil
	})

	if !sent {
		ready <- qrExportOutcome{err: errors.New("扫码登录连接未建立")}
	}
}

// exportWithMigrate 导出登录 token；若 Telegram 要求先迁移 DC（AuthLoginTokenMigrateTo），
// 真实执行迁移后重试，而不是仅记录 dc_id。
func exportWithMigrate(ctx context.Context, q qrlogin.QR, migrate func(context.Context, int) error) (qrlogin.Token, error) {
	for attempt := 0; attempt < 3; attempt++ {
		token, err := q.Export(ctx)
		if err == nil {
			return token, nil
		}
		var mig *qrlogin.MigrationNeededError
		if !errors.As(err, &mig) || migrate == nil {
			return qrlogin.Token{}, err
		}
		if merr := migrate(ctx, mig.MigrateTo.DCID); merr != nil {
			return qrlogin.Token{}, merr
		}
	}
	return qrlogin.Token{}, errors.New("导出登录 token 失败：DC 迁移次数过多")
}

// QRTokenURL 由 token 字节构造 tg://login URL，格式与 gotd qrlogin.Token.URL 一致。
func QRTokenURL(token []byte) string {
	return "tg://login?token=" + base64.URLEncoding.EncodeToString(token)
}
