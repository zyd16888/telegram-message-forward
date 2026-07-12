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

// --- fakes ---

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type fakeAccountRepo struct {
	items map[int64]domainaccount.Account
}

func newFakeAccountRepo() *fakeAccountRepo {
	return &fakeAccountRepo{items: map[int64]domainaccount.Account{}}
}

func (r *fakeAccountRepo) Create(_ context.Context, a *domainaccount.Account) error {
	a.ID = int64(len(r.items) + 1)
	r.items[a.ID] = *a
	return nil
}
func (r *fakeAccountRepo) Update(_ context.Context, a *domainaccount.Account) error {
	r.items[a.ID] = *a
	return nil
}
func (r *fakeAccountRepo) GetByID(_ context.Context, id int64) (*domainaccount.Account, error) {
	a, ok := r.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := a
	return &cp, nil
}
func (r *fakeAccountRepo) List(context.Context) ([]*domainaccount.Account, error) { return nil, nil }
func (r *fakeAccountRepo) Delete(context.Context, int64) error                    { return nil }

type fakeFlowRepo struct {
	items  map[string]domainloginflow.Flow
	nextID int64
}

func newFakeFlowRepo() *fakeFlowRepo {
	return &fakeFlowRepo{items: map[string]domainloginflow.Flow{}}
}
func (r *fakeFlowRepo) Create(_ context.Context, f *domainloginflow.Flow) error {
	r.nextID++
	f.ID = r.nextID
	r.items[f.FlowID] = *f
	return nil
}
func (r *fakeFlowRepo) Update(_ context.Context, f *domainloginflow.Flow) error {
	r.items[f.FlowID] = *f
	return nil
}
func (r *fakeFlowRepo) GetByFlowID(_ context.Context, flowID string) (*domainloginflow.Flow, error) {
	f, ok := r.items[flowID]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := f
	return &cp, nil
}
func (r *fakeFlowRepo) GetActiveByAccount(_ context.Context, accountID int64) (*domainloginflow.Flow, error) {
	var latest *domainloginflow.Flow
	for k := range r.items {
		f := r.items[k]
		if f.AccountID == accountID && !f.Status.IsTerminal() {
			if latest == nil || f.ID > latest.ID {
				cp := f
				latest = &cp
			}
		}
	}
	return latest, nil
}
func (r *fakeFlowRepo) ExpireStale(_ context.Context, now time.Time) (int64, error) {
	var n int64
	for k := range r.items {
		f := r.items[k]
		if !f.Status.IsTerminal() && f.ExpiresAt.Before(now) {
			f.Status = domainloginflow.StatusExpired
			f.PhoneCodeHash = ""
			f.QRToken = nil
			r.items[k] = f
			n++
		}
	}
	return n, nil
}

// fakeRunner 模拟 Telegram 分步登录；通过 SaveSession 回调写入 session 明文以便断言。
type fakeRunner struct {
	sendErr      error
	codeErr      error
	needPassword bool
	passwordErr  error
	authorized   bool // password 步骤是否成功
	sendSession  []byte
}

func (f *fakeRunner) SendCode(ctx context.Context, cfg infratelegram.LoginFlowConfig, _ string) (infratelegram.SendCodeResult, error) {
	f.sendSession = append([]byte(nil), cfg.Session...)
	if f.sendErr != nil {
		return infratelegram.SendCodeResult{}, f.sendErr
	}
	// 模拟 gotd 建立连接后持久化未授权 session。
	_ = cfg.SaveSession(ctx, []byte("unauth-session"))
	return infratelegram.SendCodeResult{PhoneCodeHash: "hash-123"}, nil
}

type fakeConnectionController struct {
	stopped  []int64
	started  []int64
	err      error
	startErr error
}

func (f *fakeConnectionController) StopAccount(_ context.Context, accountID int64) error {
	f.stopped = append(f.stopped, accountID)
	return f.err
}

func (f *fakeConnectionController) StartAccount(_ context.Context, accountID int64) error {
	f.started = append(f.started, accountID)
	return f.startErr
}
func (f *fakeRunner) SignInCode(ctx context.Context, cfg infratelegram.LoginFlowConfig, _, _, _ string) (infratelegram.SignInResult, error) {
	if f.codeErr != nil {
		return infratelegram.SignInResult{}, f.codeErr
	}
	if f.needPassword {
		return infratelegram.SignInResult{NeedPassword: true}, nil
	}
	_ = cfg.SaveSession(ctx, []byte("auth-session"))
	return infratelegram.SignInResult{Authorized: true}, nil
}
func (f *fakeRunner) SignInPassword(ctx context.Context, cfg infratelegram.LoginFlowConfig, _ string) (infratelegram.SignInResult, error) {
	if f.passwordErr != nil {
		return infratelegram.SignInResult{}, f.passwordErr
	}
	if f.authorized {
		_ = cfg.SaveSession(ctx, []byte("auth-session"))
		return infratelegram.SignInResult{Authorized: true}, nil
	}
	return infratelegram.SignInResult{}, nil
}

// --- helpers ---

func setup(t *testing.T, runner *fakeRunner) (*Service, *fakeAccountRepo, *fakeFlowRepo, *fakeClock, int64) {
	t.Helper()
	accounts := newFakeAccountRepo()
	flows := newFakeFlowRepo()
	clk := &fakeClock{t: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)}
	acc := &domainaccount.Account{Name: "a", PhoneNumber: "+8613800000000", AppID: 1, AppHash: "h", Status: domainaccount.StatusInactive}
	_ = accounts.Create(context.Background(), acc)
	svc := NewService(accounts, flows, runner, nil, clk, nil)
	return svc, accounts, flows, clk, acc.ID
}

// --- tests ---

func TestStartStopsRunnerAndClearsOldSession(t *testing.T) {
	runner := &fakeRunner{}
	svc, accounts, _, _, accID := setup(t, runner)
	connections := &fakeConnectionController{}
	svc.UseConnectionController(connections)
	acc, _ := accounts.GetByID(context.Background(), accID)
	acc.Session = []byte("invalid-old-session")
	acc.Status = domainaccount.StatusActive
	_ = accounts.Update(context.Background(), acc)

	if _, err := svc.Start(context.Background(), accID); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if len(connections.stopped) != 1 || connections.stopped[0] != accID {
		t.Fatalf("应先停止账号 runner，实际 %v", connections.stopped)
	}
	if len(runner.sendSession) != 0 {
		t.Fatalf("发送验证码不应复用旧 session，实际 %q", string(runner.sendSession))
	}
}

func TestStartDoesNotClearSessionWhenRunnerCannotStop(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{})
	svc.UseConnectionController(&fakeConnectionController{err: errors.New("stop failed")})
	acc, _ := accounts.GetByID(context.Background(), accID)
	acc.Session = []byte("existing-session")
	_ = accounts.Update(context.Background(), acc)

	if _, err := svc.Start(context.Background(), accID); err == nil {
		t.Fatal("runner 停止失败时应中止登录")
	}
	got, _ := accounts.GetByID(context.Background(), accID)
	if string(got.Session) != "existing-session" {
		t.Fatal("runner 未停止时不能提前清除 session")
	}
}

// TestPhoneLoginHappyPath 验证无 2FA 的完整登录：验证码通过后账号 active、session 为授权后 session。
func TestPhoneLoginHappyPath(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{})
	connections := &fakeConnectionController{}
	svc.UseConnectionController(connections)
	ctx := context.Background()

	flow, err := svc.Start(ctx, accID)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusCodeRequired {
		t.Fatalf("Start 后应为 code_required，实际 %s", flow.Status)
	}

	flow, err = svc.SubmitCode(ctx, accID, flow.FlowID, "12345")
	if err != nil {
		t.Fatalf("SubmitCode 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusAuthorized {
		t.Fatalf("应为 authorized，实际 %s", flow.Status)
	}
	if flow.PhoneCodeHash != "" {
		t.Fatal("成功后应清空 phone_code_hash")
	}
	acc, _ := accounts.GetByID(ctx, accID)
	if acc.Status != domainaccount.StatusActive {
		t.Fatalf("账号应为 active，实际 %s", acc.Status)
	}
	if string(acc.Session) != "auth-session" {
		t.Fatalf("应保存授权后 session，实际 %q", string(acc.Session))
	}
	if acc.LastLoginAt == nil {
		t.Fatal("应记录 last_login_at")
	}
	if len(connections.started) != 1 || connections.started[0] != accID {
		t.Fatalf("登录成功后应恢复账号 Source，实际 %v", connections.started)
	}
}

// TestPhoneLogin2FA 验证 2FA 分支：验证码后进入 password_required，密码通过后 authorized。
func TestPhoneLogin2FA(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{needPassword: true, authorized: true})
	ctx := context.Background()

	flow, _ := svc.Start(ctx, accID)
	flow, err := svc.SubmitCode(ctx, accID, flow.FlowID, "12345")
	if err != nil {
		t.Fatalf("SubmitCode 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusPasswordRequired {
		t.Fatalf("应为 password_required，实际 %s", flow.Status)
	}
	// 未进入密码步骤前账号不应 active。
	if acc, _ := accounts.GetByID(ctx, accID); acc.Status == domainaccount.StatusActive {
		t.Fatal("2FA 完成前账号不应 active")
	}

	flow, err = svc.SubmitPassword(ctx, accID, flow.FlowID, "secret")
	if err != nil {
		t.Fatalf("SubmitPassword 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusAuthorized {
		t.Fatalf("应为 authorized，实际 %s", flow.Status)
	}
	if acc, _ := accounts.GetByID(ctx, accID); acc.Status != domainaccount.StatusActive {
		t.Fatal("2FA 后账号应 active")
	}
}

// TestSubmitCodeInvalid 验证验证码错误：flow 保持 code_required、账号不 active、不写授权 session。
func TestSubmitCodeInvalid(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{codeErr: infratelegram.ErrCodeInvalid})
	ctx := context.Background()

	flow, _ := svc.Start(ctx, accID)
	flow, err := svc.SubmitCode(ctx, accID, flow.FlowID, "00000")
	if !errors.Is(err, infratelegram.ErrCodeInvalid) {
		t.Fatalf("应返回验证码错误，实际 %v", err)
	}
	if flow.Status != domainloginflow.StatusCodeRequired {
		t.Fatalf("验证码错误后应仍为 code_required，实际 %s", flow.Status)
	}
	if flow.LastError == "" {
		t.Fatal("应记录 last_error")
	}
	acc, _ := accounts.GetByID(ctx, accID)
	if acc.Status == domainaccount.StatusActive {
		t.Fatal("验证码错误后账号不应 active")
	}
	if string(acc.Session) == "auth-session" {
		t.Fatal("验证码错误不应写入授权 session")
	}
}

// TestSubmitCodeExpiredByServer 验证服务端返回验证码过期时 flow 置 expired。
func TestSubmitCodeExpiredByServer(t *testing.T) {
	svc, _, _, _, accID := setup(t, &fakeRunner{codeErr: infratelegram.ErrCodeExpired})
	ctx := context.Background()
	flow, _ := svc.Start(ctx, accID)
	flow, err := svc.SubmitCode(ctx, accID, flow.FlowID, "00000")
	if !errors.Is(err, infratelegram.ErrCodeExpired) {
		t.Fatalf("应返回验证码过期，实际 %v", err)
	}
	if flow.Status != domainloginflow.StatusExpired {
		t.Fatalf("应为 expired，实际 %s", flow.Status)
	}
}

// TestFlowExpiryByClock 验证 flow 超过 expires_at 后 SubmitCode 返回过期。
func TestFlowExpiryByClock(t *testing.T) {
	svc, _, _, clk, accID := setup(t, &fakeRunner{})
	ctx := context.Background()
	flow, _ := svc.Start(ctx, accID)
	// 时钟前进超过 TTL。
	clk.t = clk.t.Add(defaultFlowTTL + time.Minute)
	flow, err := svc.SubmitCode(ctx, accID, flow.FlowID, "12345")
	if !errors.Is(err, ErrFlowExpired) {
		t.Fatalf("应返回 flow 过期，实际 %v", err)
	}
	if flow.Status != domainloginflow.StatusExpired {
		t.Fatalf("应为 expired，实际 %s", flow.Status)
	}
}

// TestCancel 验证取消 flow：flow 置 cancelled，登录中账号回退 inactive。
func TestCancel(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{})
	ctx := context.Background()
	flow, _ := svc.Start(ctx, accID)
	flow, err := svc.Cancel(ctx, accID, flow.FlowID)
	if err != nil {
		t.Fatalf("Cancel 失败: %v", err)
	}
	if flow.Status != domainloginflow.StatusCancelled {
		t.Fatalf("应为 cancelled，实际 %s", flow.Status)
	}
	if acc, _ := accounts.GetByID(ctx, accID); acc.Status != domainaccount.StatusInactive {
		t.Fatalf("取消后账号应回退 inactive，实际 %s", acc.Status)
	}
}

// TestSubmitCodeAccountMismatch 验证账号归属校验：用另一个账号 id 提交验证码必须
// 返回明确的不匹配错误，且不能修改该 flow（flow 仍保持 code_required 可被真正账号继续）。
func TestSubmitCodeAccountMismatch(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{})
	ctx := context.Background()

	other := &domainaccount.Account{Name: "b", PhoneNumber: "+8613900000000", AppID: 1, AppHash: "h", Status: domainaccount.StatusInactive}
	_ = accounts.Create(ctx, other)

	flow, _ := svc.Start(ctx, accID)

	got, err := svc.SubmitCode(ctx, other.ID, flow.FlowID, "12345")
	if !errors.Is(err, ErrFlowAccountMismatch) {
		t.Fatalf("应返回账号不匹配错误，实际 %v", err)
	}
	if got != nil {
		t.Fatal("账号不匹配时不应返回 flow 数据")
	}

	// flow 未被跨账号请求修改，真正所属账号仍可正常提交验证码。
	flow, err = svc.SubmitCode(ctx, accID, flow.FlowID, "12345")
	if err != nil {
		t.Fatalf("所属账号提交验证码应成功: %v", err)
	}
	if flow.Status != domainloginflow.StatusAuthorized {
		t.Fatalf("应为 authorized，实际 %s", flow.Status)
	}
}

// TestCancelAccountMismatch 验证 Cancel 同样校验账号归属，不允许跨账号取消他人 flow。
func TestCancelAccountMismatch(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{})
	ctx := context.Background()

	other := &domainaccount.Account{Name: "b", PhoneNumber: "+8613900000000", AppID: 1, AppHash: "h", Status: domainaccount.StatusInactive}
	_ = accounts.Create(ctx, other)

	flow, _ := svc.Start(ctx, accID)

	if _, err := svc.Cancel(ctx, other.ID, flow.FlowID); !errors.Is(err, ErrFlowAccountMismatch) {
		t.Fatalf("跨账号取消应返回不匹配错误，实际 %v", err)
	}
	stored, _ := svc.GetActiveByAccount(ctx, accID)
	if stored == nil || stored.Status.IsTerminal() {
		t.Fatal("跨账号取消不应影响真正所属账号的 flow")
	}
}

// TestStartSendCodeError 验证 SendCode 失败：flow 置 failed，账号置 error。
func TestStartSendCodeError(t *testing.T) {
	svc, accounts, _, _, accID := setup(t, &fakeRunner{sendErr: infratelegram.ErrTooManyRequests})
	ctx := context.Background()
	flow, err := svc.Start(ctx, accID)
	if !errors.Is(err, infratelegram.ErrTooManyRequests) {
		t.Fatalf("应返回限流错误，实际 %v", err)
	}
	if flow.Status != domainloginflow.StatusFailed {
		t.Fatalf("应为 failed，实际 %s", flow.Status)
	}
	if acc, _ := accounts.GetByID(ctx, accID); acc.Status != domainaccount.StatusError {
		t.Fatalf("账号应为 error，实际 %s", acc.Status)
	}
}

// TestGetActiveRecovery 验证服务重启恢复：未过期 flow 可查询；ExpireStale 后过期 flow 置 expired。
func TestGetActiveRecovery(t *testing.T) {
	svc, _, flows, clk, accID := setup(t, &fakeRunner{})
	ctx := context.Background()
	flow, _ := svc.Start(ctx, accID)

	// 未过期：可恢复查询。
	got, err := svc.GetActiveByAccount(ctx, accID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.FlowID != flow.FlowID {
		t.Fatal("应能查询到未完成 flow")
	}

	// 过期后 RecoverStale 标记 expired。
	clk.t = clk.t.Add(defaultFlowTTL + time.Minute)
	if err := svc.RecoverStale(ctx); err != nil {
		t.Fatal(err)
	}
	stored, _ := flows.GetByFlowID(ctx, flow.FlowID)
	if stored.Status != domainloginflow.StatusExpired {
		t.Fatalf("RecoverStale 后应为 expired，实际 %s", stored.Status)
	}
	if stored.PhoneCodeHash != "" {
		t.Fatal("过期后应清空 phone_code_hash")
	}
}
