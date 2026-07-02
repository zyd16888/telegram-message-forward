package telegramlogin

import (
	"context"
	"errors"
	"testing"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainloginflow "telegram-message-forward/internal/domain/loginflow"
	infratelegram "telegram-message-forward/internal/infra/telegram"
)

// fakeQRRunner 模拟 infra/telegram.QRSessionManager 的后台会话契约：
// QRStart 建立一次会话并返回首个 token；QRCheck 按预设序列依次返回状态快照，
// 不接收/不依赖 token 参数，天然验证「轮询不会重新导出 token」；QRCancel
// 模拟服务重启或主动取消导致会话丢失。
type fakeQRRunner struct {
	startErr error
	// checkResults 按调用顺序返回；用尽后保持返回最后一个（默认 Waiting）。
	checkResults []infratelegram.QRCheckResult
	calls        int
	sessions     map[string]bool
}

func newFakeQRRunner() *fakeQRRunner {
	return &fakeQRRunner{sessions: map[string]bool{}}
}

func (f *fakeQRRunner) QRStart(ctx context.Context, cfg infratelegram.LoginFlowConfig, sessionID string) (infratelegram.QRExportResult, error) {
	if f.startErr != nil {
		return infratelegram.QRExportResult{}, f.startErr
	}
	f.sessions[sessionID] = true
	f.calls = 0
	// 模拟 gotd 建立长连接后立即持久化 session（onSession 在连接握手时触发，
	// 与是否已授权无关）；扫码成功后无需再次显式保存。
	_ = cfg.SaveSession(ctx, []byte("qr-session-bytes"))
	return infratelegram.QRExportResult{
		Token:     []byte("qr-token-bytes"),
		URL:       "tg://login?token=x",
		ExpiresAt: time.Date(2026, 7, 1, 12, 0, 30, 0, time.UTC),
	}, nil
}

func (f *fakeQRRunner) QRCheck(ctx context.Context, sessionID string) (infratelegram.QRCheckResult, error) {
	if !f.sessions[sessionID] {
		return infratelegram.QRCheckResult{}, infratelegram.ErrQRSessionNotFound
	}
	if len(f.checkResults) == 0 {
		return infratelegram.QRCheckResult{Waiting: true}, nil
	}
	idx := f.calls
	if idx >= len(f.checkResults) {
		idx = len(f.checkResults) - 1
	}
	f.calls++
	res := f.checkResults[idx]
	if res.Authorized || res.Expired || res.Err != nil {
		delete(f.sessions, sessionID)
	}
	return res, nil
}

func (f *fakeQRRunner) QRCancel(sessionID string) {
	delete(f.sessions, sessionID)
}

func setupQR(t *testing.T, qr *fakeQRRunner) (*Service, *fakeAccountRepo, *fakeFlowRepo, *fakeClock, int64) {
	t.Helper()
	accounts := newFakeAccountRepo()
	flows := newFakeFlowRepo()
	clk := &fakeClock{t: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)}
	acc := &domainaccount.Account{Name: "a", PhoneNumber: "+8613800000000", AppID: 1, AppHash: "h", Status: domainaccount.StatusInactive}
	_ = accounts.Create(context.Background(), acc)
	svc := NewService(accounts, flows, nil, qr, clk, nil)
	return svc, accounts, flows, clk, acc.ID
}

// TestQRStartAndAuthorize 验证扫码登录：start 生成二维码，轮询到 authorized 后账号 active、
// session 已加密落库（在 QRStart 建立长连接时即已保存）。
func TestQRStartAndAuthorize(t *testing.T) {
	qr := newFakeQRRunner()
	qr.checkResults = []infratelegram.QRCheckResult{
		{Waiting: true},
		{Authorized: true},
	}
	svc, accounts, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, err := svc.StartQR(ctx, accID)
	if err != nil {
		t.Fatalf("StartQR 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusWaitingScan {
		t.Fatalf("应为 waiting_scan，实际 %s", flow.Status)
	}
	if flow.QRURL == "" {
		t.Fatal("应返回 qr_url 用于渲染二维码")
	}
	// session 在建立长连接时已保存，不必等到 authorized。
	if acc, _ := accounts.GetByID(ctx, accID); string(acc.Session) != "qr-session-bytes" {
		t.Fatalf("StartQR 后应已保存 session，实际 %q", string(acc.Session))
	}

	// 第一次轮询仍在等待。
	flow, err = svc.QRStatus(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusWaitingScan {
		t.Fatalf("首次轮询应仍等待，实际 %s", flow.Status)
	}

	// 第二次轮询完成登录。
	flow, err = svc.QRStatus(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusAuthorized {
		t.Fatalf("应为 authorized，实际 %s", flow.Status)
	}
	acc, _ := accounts.GetByID(ctx, accID)
	if acc.Status != domainaccount.StatusActive {
		t.Fatalf("账号应 active，实际 %s", acc.Status)
	}
	if string(acc.Session) != "qr-session-bytes" {
		t.Fatalf("应保存 session，实际 %q", string(acc.Session))
	}
}

// TestQRStatusDoesNotRefreshTokenOnEachPoll 验证多次轮询「等待中」状态时，
// qr_url（由 token 派生）保持不变，且底层 QRCheck 不会重新 export 新 token——
// 这正是修复的核心问题：轮询不应每次替换二维码。
func TestQRStatusDoesNotRefreshTokenOnEachPoll(t *testing.T) {
	qr := newFakeQRRunner()
	qr.checkResults = []infratelegram.QRCheckResult{
		{Waiting: true}, {Waiting: true}, {Waiting: true},
	}
	svc, _, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, err := svc.StartQR(ctx, accID)
	if err != nil {
		t.Fatalf("StartQR 失败: %v", err)
	}
	firstURL := flow.QRURL
	if firstURL == "" {
		t.Fatal("应返回 qr_url")
	}

	for i := 0; i < 3; i++ {
		flow, err = svc.QRStatus(ctx, accID, flow.FlowID)
		if err != nil {
			t.Fatalf("第 %d 次轮询失败: %v", i+1, err)
		}
		if flow.Status != domainloginflow.StatusWaitingScan {
			t.Fatalf("第 %d 次轮询应仍等待，实际 %s", i+1, flow.Status)
		}
		if flow.QRURL != firstURL {
			t.Fatalf("第 %d 次轮询后 qr_url 不应变化，原 %q 现 %q", i+1, firstURL, flow.QRURL)
		}
	}
	// QRCheck 只读取会话快照，不会触发 QRStart（QRStart 只在真正 start/refresh 时调用一次）。
	if qr.calls != 3 {
		t.Fatalf("应恰好轮询 3 次 QRCheck，实际 %d", qr.calls)
	}
}

// TestQRMigrateBranch 验证 DC migrate 分支：授权结果携带 MigrateDC 时，
// flow.DCID 被真实记录（对应 infra 层通过 client.MigrateTo 完成的真实迁移与导入），
// 而不只是把请求挂起。
func TestQRMigrateBranch(t *testing.T) {
	qr := newFakeQRRunner()
	qr.checkResults = []infratelegram.QRCheckResult{
		{Authorized: true, MigrateDC: 4},
	}
	svc, accounts, flows, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, _ := svc.StartQR(ctx, accID)
	flow, err := svc.QRStatus(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusAuthorized {
		t.Fatalf("migrate 后应完成登录，实际 %s", flow.Status)
	}
	stored, _ := flows.GetByFlowID(ctx, flow.FlowID)
	if stored.DCID != 4 {
		t.Fatalf("应记录真实迁移目标 dc=4，实际 %d", stored.DCID)
	}
	if acc, _ := accounts.GetByID(ctx, accID); acc.Status != domainaccount.StatusActive {
		t.Fatal("migrate 后账号应 active")
	}
}

// TestQRExpiredRefresh 验证二维码过期与刷新：过期进入 qr_refresh_required，刷新后重新
// 发起会话（旧会话被取消）并回到 waiting_scan。
func TestQRExpiredRefresh(t *testing.T) {
	qr := newFakeQRRunner()
	qr.checkResults = []infratelegram.QRCheckResult{{Expired: true}}
	svc, _, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, _ := svc.StartQR(ctx, accID)
	oldFlowID := flow.FlowID
	flow, err := svc.QRStatus(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusQRRefreshRequired {
		t.Fatalf("过期应为 qr_refresh_required，实际 %s", flow.Status)
	}
	if qr.sessions[oldFlowID] {
		t.Fatal("过期后旧会话应已被清理")
	}

	flow, err = svc.RefreshQR(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatalf("RefreshQR 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusWaitingScan {
		t.Fatalf("刷新后应 waiting_scan，实际 %s", flow.Status)
	}
	if flow.QRURL == "" {
		t.Fatal("刷新后应返回新的 qr_url")
	}
	if !qr.sessions[flow.FlowID] {
		t.Fatal("刷新后应重新发起会话")
	}
}

// TestQRSessionLostAfterRestart 验证服务重启后（内存态会话丢失）：轮询检测到
// ErrQRSessionNotFound 时应将 flow 置为 qr_refresh_required，而不是报错卡死，
// 这是 HTTP 轮询式 UI 恢复 gotd 推送语义会话的方式。
func TestQRSessionLostAfterRestart(t *testing.T) {
	qr := newFakeQRRunner()
	svc, _, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, err := svc.StartQR(ctx, accID)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟进程重启：内存态会话丢失。
	qr.QRCancel(flow.FlowID)

	flow, err = svc.QRStatus(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusQRRefreshRequired {
		t.Fatalf("会话丢失后应为 qr_refresh_required，实际 %s", flow.Status)
	}
}

// TestQRFlowRecovery 验证服务重启后 flow 仍可通过 GetActiveByAccount 恢复展示，
// 超过整体 TTL 后 RecoverStale 清理为 expired。
func TestQRFlowRecovery(t *testing.T) {
	qr := newFakeQRRunner()
	svc, _, _, clk, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, _ := svc.StartQR(ctx, accID)
	got, err := svc.GetActiveByAccount(ctx, accID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Status != domainloginflow.StatusWaitingScan {
		t.Fatal("重启后应恢复等待中的 QR flow")
	}
	if got.QRURL == "" {
		t.Fatal("恢复时应能重建 qr_url")
	}

	// 超过整体 TTL 后 RecoverStale 标记 expired。
	clk.t = clk.t.Add(defaultFlowTTL + time.Minute)
	if err := svc.RecoverStale(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.GetActiveByAccount(ctx, accID)
	if got != nil {
		t.Fatalf("超期后不应再有活跃 flow，实际 %v", got.Status)
	}
	_ = flow
}

// TestQRStatusAccountMismatch 验证账号归属校验：用另一个账号 id 轮询该 flow 必须
// 返回明确的不匹配错误，且不能返回 flow 数据（避免跨账号泄露登录状态）。
func TestQRStatusAccountMismatch(t *testing.T) {
	qr := newFakeQRRunner()
	svc, accounts, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	other := &domainaccount.Account{Name: "b", PhoneNumber: "+8613900000000", AppID: 1, AppHash: "h", Status: domainaccount.StatusInactive}
	_ = accounts.Create(ctx, other)

	flow, err := svc.StartQR(ctx, accID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.QRStatus(ctx, other.ID, flow.FlowID)
	if !errors.Is(err, ErrFlowAccountMismatch) {
		t.Fatalf("应返回账号不匹配错误，实际 %v", err)
	}
	if got != nil {
		t.Fatal("账号不匹配时不应返回 flow 数据")
	}

	// 真正所属账号仍可正常轮询。
	if _, err := svc.QRStatus(ctx, accID, flow.FlowID); err != nil {
		t.Fatalf("所属账号轮询应成功: %v", err)
	}
}

// TestQRRefreshAccountMismatch 验证 RefreshQR 同样校验账号归属，不允许跨账号刷新。
func TestQRRefreshAccountMismatch(t *testing.T) {
	qr := newFakeQRRunner()
	qr.checkResults = []infratelegram.QRCheckResult{{Expired: true}}
	svc, accounts, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	other := &domainaccount.Account{Name: "b", PhoneNumber: "+8613900000000", AppID: 1, AppHash: "h", Status: domainaccount.StatusInactive}
	_ = accounts.Create(ctx, other)

	flow, _ := svc.StartQR(ctx, accID)
	_, _ = svc.QRStatus(ctx, accID, flow.FlowID) // 触发过期

	if _, err := svc.RefreshQR(ctx, other.ID, flow.FlowID); !errors.Is(err, ErrFlowAccountMismatch) {
		t.Fatalf("跨账号刷新应返回不匹配错误，实际 %v", err)
	}
}
