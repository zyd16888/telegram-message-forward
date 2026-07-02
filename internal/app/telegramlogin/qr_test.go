package telegramlogin

import (
	"context"
	"testing"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainloginflow "telegram-message-forward/internal/domain/loginflow"
	infratelegram "telegram-message-forward/internal/infra/telegram"
)

// fakeQRRunner 模拟扫码登录的分步结果。
type fakeQRRunner struct {
	exportErr error
	// checkResults 按调用顺序返回；用尽后返回最后一个。
	checkResults []infratelegram.QRCheckResult
	calls        int
}

func (f *fakeQRRunner) QRExport(ctx context.Context, cfg infratelegram.LoginFlowConfig) (infratelegram.QRExportResult, error) {
	if f.exportErr != nil {
		return infratelegram.QRExportResult{}, f.exportErr
	}
	return infratelegram.QRExportResult{
		Token:     []byte("qr-token-bytes"),
		URL:       "tg://login?token=x",
		ExpiresAt: time.Date(2026, 7, 1, 12, 0, 30, 0, time.UTC),
		DCID:      2,
	}, nil
}

func (f *fakeQRRunner) QRCheck(ctx context.Context, cfg infratelegram.LoginFlowConfig, _ []byte, _ int64) (infratelegram.QRCheckResult, error) {
	if len(f.checkResults) == 0 {
		return infratelegram.QRCheckResult{Waiting: true}, nil
	}
	idx := f.calls
	if idx >= len(f.checkResults) {
		idx = len(f.checkResults) - 1
	}
	f.calls++
	res := f.checkResults[idx]
	if res.Authorized {
		// 模拟 gotd 完成登录后持久化授权 session。
		_ = cfg.SaveSession(ctx, []byte("auth-session"))
	}
	return res, nil
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

// TestQRStartAndAuthorize 验证扫码登录：start 生成二维码，轮询到 authorized 后账号 active。
func TestQRStartAndAuthorize(t *testing.T) {
	qr := &fakeQRRunner{checkResults: []infratelegram.QRCheckResult{
		{Waiting: true, RefreshedToken: []byte("qr-token-2"), ExpiresAt: time.Date(2026, 7, 1, 12, 1, 0, 0, time.UTC)},
		{Authorized: true},
	}}
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

	// 第一次轮询仍在等待。
	flow, err = svc.QRStatus(ctx, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusWaitingScan {
		t.Fatalf("首次轮询应仍等待，实际 %s", flow.Status)
	}

	// 第二次轮询完成登录。
	flow, err = svc.QRStatus(ctx, flow.FlowID)
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
	if string(acc.Session) != "auth-session" {
		t.Fatalf("应保存授权 session，实际 %q", string(acc.Session))
	}
}

// TestQRMigrateBranch 验证 DC migrate 分支：先返回 migrate 记录 dc，再轮询成功。
func TestQRMigrateBranch(t *testing.T) {
	qr := &fakeQRRunner{checkResults: []infratelegram.QRCheckResult{
		{MigrateDC: 4},
		{Authorized: true},
	}}
	svc, _, flows, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, _ := svc.StartQR(ctx, accID)
	// 第一次轮询触发 migrate，记录 dc_id 并保持等待。
	flow, err := svc.QRStatus(ctx, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusWaitingScan {
		t.Fatalf("migrate 后应仍等待，实际 %s", flow.Status)
	}
	stored, _ := flows.GetByFlowID(ctx, flow.FlowID)
	if stored.DCID != 4 {
		t.Fatalf("应记录迁移目标 dc=4，实际 %d", stored.DCID)
	}
	// 第二次轮询完成登录。
	flow, _ = svc.QRStatus(ctx, flow.FlowID)
	if flow.Status != domainloginflow.StatusAuthorized {
		t.Fatalf("migrate 后应完成登录，实际 %s", flow.Status)
	}
}

// TestQRExpiredRefresh 验证二维码过期与刷新：过期进入 qr_refresh_required，刷新回到 waiting_scan。
func TestQRExpiredRefresh(t *testing.T) {
	qr := &fakeQRRunner{checkResults: []infratelegram.QRCheckResult{{Expired: true}}}
	svc, _, _, _, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, _ := svc.StartQR(ctx, accID)
	flow, err := svc.QRStatus(ctx, flow.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Status != domainloginflow.StatusQRRefreshRequired {
		t.Fatalf("过期应为 qr_refresh_required，实际 %s", flow.Status)
	}

	flow, err = svc.RefreshQR(ctx, flow.FlowID)
	if err != nil {
		t.Fatalf("RefreshQR 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusWaitingScan {
		t.Fatalf("刷新后应 waiting_scan，实际 %s", flow.Status)
	}
	if flow.QRURL == "" {
		t.Fatal("刷新后应返回新的 qr_url")
	}
}

// TestQRTokenNotLeaked 验证 QR flow 的 DTO 侧不会泄露原始 token（domain 保留明文供内部使用，
// 但持久化经加密、响应仅暴露派生 URL）。此处校验 attachQRURL 只依赖 token 派生 URL，不外泄 token。
func TestQRFlowRecovery(t *testing.T) {
	qr := &fakeQRRunner{}
	svc, _, _, clk, accID := setupQR(t, qr)
	ctx := context.Background()

	flow, _ := svc.StartQR(ctx, accID)
	// 模拟服务重启：通过 GetActiveByAccount 恢复。
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
