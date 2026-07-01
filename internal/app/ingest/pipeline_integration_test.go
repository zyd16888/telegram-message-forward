package ingest_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	appingest "telegram-message-forward/internal/app/ingest"
	"telegram-message-forward/internal/config"
	"telegram-message-forward/internal/dispatch"
	domainaccount "telegram-message-forward/internal/domain/account"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	domainsink "telegram-message-forward/internal/domain/sink"
	domainsource "telegram-message-forward/internal/domain/source"
	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/ruleengine"
	"telegram-message-forward/internal/storage"
	"telegram-message-forward/internal/storage/repository"
	tmpl "telegram-message-forward/internal/template"

	_ "telegram-message-forward/internal/plugin/sink/webhook"
)

// findRepoRoot 向上查找包含 go.mod 的目录。
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("未找到 go.mod")
		}
		dir = parent
	}
}

type pipelineEnv struct {
	db         *gorm.DB
	ingest     *appingest.Service
	worker     *dispatch.Worker
	sinks      *repository.SinkRepository
	deliveries *repository.DeliveryRepository
	accountID  int64
	sourceID   int64
	cleanup    func()
}

func setupPipeline(t *testing.T) *pipelineEnv {
	t.Helper()
	if os.Getenv("TMF_RUN_DB_TESTS") == "" {
		t.Skip("设置 TMF_RUN_DB_TESTS=1 运行数据库集成测试")
	}

	root := findRepoRoot(t)
	cfg, err := config.Load(filepath.Join(root, "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	cipher, err := crypto.NewCipher([]byte(cfg.Security.EncryptionKey))
	if err != nil {
		t.Fatalf("初始化加密失败: %v", err)
	}
	db, err := storage.Open(cfg.Database)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.System{}

	accounts := repository.NewAccountRepository(db, cipher)
	sources := repository.NewSourceRepository(db)
	sinks := repository.NewSinkRepository(db, cipher)
	templates := repository.NewTemplateRepository(db)
	rules := repository.NewRuleRepository(db)
	messages := repository.NewMessageRepository(db)
	deliveries := repository.NewDeliveryRepository(db)

	engine := ruleengine.NewEngine()
	renderer := tmpl.NewRenderer()
	queue := dispatch.NewQueue(deliveries, cfg.Dispatch.MaxAttempts)
	ingestSvc := appingest.NewService(messages, rules, engine, queue, clk, log)
	worker := dispatch.NewWorker("test-worker", cfg.Dispatch, deliveries, sinks, templates, messages, renderer, clk, log)

	ctx := context.Background()
	acc := &domainaccount.Account{Name: "it-account", PhoneNumber: "+10000000000", AppID: 1, Status: domainaccount.StatusInactive}
	if err := accounts.Create(ctx, acc); err != nil {
		t.Fatalf("创建账号失败: %v", err)
	}
	src := &domainsource.Source{AccountID: acc.ID, PeerType: domainsource.PeerChannel, PeerID: 999001, Name: "it-source", Enabled: true}
	if err := sources.Create(ctx, src); err != nil {
		t.Fatalf("创建 source 失败: %v", err)
	}

	cleanup := func() {
		// 删除账号级联 sources/messages/delivery_tasks；再清理独立的 sinks/templates/rules。
		accounts.Delete(ctx, acc.ID)
	}

	return &pipelineEnv{
		db: db, ingest: ingestSvc, worker: worker, sinks: sinks, deliveries: deliveries,
		accountID: acc.ID, sourceID: src.ID, cleanup: cleanup,
	}
}

// TestPipelineSuccess 验证 消息→规则→渲染→Webhook 投递成功 全链路。
func TestPipelineSuccess(t *testing.T) {
	env := setupPipeline(t)
	defer env.cleanup()
	ctx := context.Background()

	var received atomic.Int32
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		received.Add(1)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	sinks := env.sinks
	templates := repository.NewTemplateRepository(env.db)
	rules := repository.NewRuleRepository(env.db)

	sk := &domainsink.Sink{Type: "webhook", Name: "it-webhook", Enabled: true,
		Config:       map[string]any{"url": srv.URL},
		Capabilities: domainsink.Capabilities{SupportsText: true}}
	if err := sinks.Create(ctx, sk); err != nil {
		t.Fatalf("创建 sink 失败: %v", err)
	}
	defer sinks.Delete(ctx, sk.ID)

	tpl := &domaintemplate.Template{Name: "it-tpl", Format: domaintemplate.FormatText, Content: "{{.Text}}"}
	if err := templates.Create(ctx, tpl); err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}
	defer templates.Delete(ctx, tpl.ID)

	rl := &domainrule.Rule{
		Name: "it-rule", Enabled: true, Priority: 10,
		Conditions: []domainrule.ConditionConfig{{Type: "keyword_contains", Config: map[string]any{"keywords": []any{"golang"}}}},
		Processors: []domainrule.ProcessorConfig{{Type: "append_source", Config: map[string]any{"label": "it-source"}}},
		SourceIDs:  []int64{env.sourceID},
		Targets:    []domainrule.Target{{SinkID: sk.ID, TemplateID: &tpl.ID}},
	}
	if err := rules.Create(ctx, rl); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	defer rules.Delete(ctx, rl.ID)

	m := &domainmessage.NormalizedMessage{
		SourceID: env.sourceID, ExternalMessageID: 5001, MessageType: "text",
		Text: "hello golang", ReceivedAt: time.Now(),
	}
	if err := env.ingest.Ingest(ctx, m); err != nil {
		t.Fatalf("ingest 失败: %v", err)
	}
	if m.ID == 0 {
		t.Fatal("消息未落库")
	}

	if err := env.worker.Tick(ctx); err != nil {
		t.Fatalf("worker tick 失败: %v", err)
	}

	if received.Load() != 1 {
		t.Fatalf("webhook 应收到 1 次，实际 %d", received.Load())
	}
	body, _ := gotBody.Load().(string)
	if body == "" || !strings.Contains(body, "hello golang") {
		t.Fatalf("webhook body 未含渲染文本: %q", body)
	}

	tasks, err := env.deliveries.List(ctx, domaindelivery.StatusSuccess, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tk := range tasks {
		if tk.MessageID == m.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("未找到 success 状态的投递任务")
	}
}

// TestPipelineDead 验证 Webhook 失败时任务在 max_attempts=1 下直接进入 dead。
func TestPipelineDead(t *testing.T) {
	env := setupPipeline(t)
	defer env.cleanup()
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sinks := env.sinks
	rules := repository.NewRuleRepository(env.db)

	sk := &domainsink.Sink{Type: "webhook", Name: "it-webhook-fail", Enabled: true,
		Config: map[string]any{"url": srv.URL}, Capabilities: domainsink.Capabilities{SupportsText: true}}
	if err := sinks.Create(ctx, sk); err != nil {
		t.Fatal(err)
	}
	defer sinks.Delete(ctx, sk.ID)

	rl := &domainrule.Rule{
		Name: "it-rule-fail", Enabled: true,
		SourceIDs: []int64{env.sourceID},
		Targets:   []domainrule.Target{{SinkID: sk.ID}},
	}
	if err := rules.Create(ctx, rl); err != nil {
		t.Fatal(err)
	}
	defer rules.Delete(ctx, rl.ID)

	// 用 max_attempts=1 的队列，令首次失败即 dead。
	queue := dispatch.NewQueue(env.deliveries, 1)
	m := &domainmessage.NormalizedMessage{SourceID: env.sourceID, ExternalMessageID: 6001, MessageType: "text", Text: "fail me", ReceivedAt: time.Now()}
	messages := repository.NewMessageRepository(env.db)
	if err := messages.Create(ctx, m); err != nil {
		t.Fatal(err)
	}
	matches := []ruleengine.Match{{Rule: rl, Targets: rl.Targets}}
	if err := queue.Enqueue(ctx, m, matches); err != nil {
		t.Fatal(err)
	}

	if err := env.worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}

	tasks, err := env.deliveries.List(ctx, domaindelivery.StatusDead, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tk := range tasks {
		if tk.MessageID == m.ID {
			found = true
			if tk.LastError == "" {
				t.Fatal("dead 任务应记录 last_error")
			}
		}
	}
	if !found {
		t.Fatal("失败任务应进入 dead 状态")
	}
}
