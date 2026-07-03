// Package bootstrap 装配各层依赖并启动应用。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"telegram-message-forward/internal/api"
	"telegram-message-forward/internal/api/handler"
	appaccount "telegram-message-forward/internal/app/account"
	apptoken "telegram-message-forward/internal/app/apitoken"
	appauth "telegram-message-forward/internal/app/auth"
	appdelivery "telegram-message-forward/internal/app/delivery"
	appingest "telegram-message-forward/internal/app/ingest"
	apprule "telegram-message-forward/internal/app/rule"
	appsink "telegram-message-forward/internal/app/sink"
	appsource "telegram-message-forward/internal/app/source"
	apptelegramconfig "telegram-message-forward/internal/app/telegramconfig"
	apptelegramlogin "telegram-message-forward/internal/app/telegramlogin"
	apptemplate "telegram-message-forward/internal/app/template"
	"telegram-message-forward/internal/config"
	"telegram-message-forward/internal/dispatch"
	domainaccount "telegram-message-forward/internal/domain/account"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/infra/logger"
	infratelegram "telegram-message-forward/internal/infra/telegram"
	pluginsource "telegram-message-forward/internal/plugin/source"
	tgsource "telegram-message-forward/internal/plugin/source/telegram"
	"telegram-message-forward/internal/ruleengine"
	"telegram-message-forward/internal/security"
	"telegram-message-forward/internal/storage"
	storagemigrate "telegram-message-forward/internal/storage/migrate"
	"telegram-message-forward/internal/storage/repository"
	tmpl "telegram-message-forward/internal/template"

	// 通过 blank import 触发内置 Sink 插件的编译期注册。
	_ "telegram-message-forward/internal/plugin/sink/dingtalk"
	_ "telegram-message-forward/internal/plugin/sink/webhook"
	_ "telegram-message-forward/internal/plugin/sink/wecom"
)

// App 持有已装配的运行时依赖。
type App struct {
	cfg        *config.Config
	log        *slog.Logger
	server     *http.Server
	workers    []*dispatch.Worker
	srcManager *appsource.Manager
	tgLogin    *apptelegramlogin.Service
	deps       *Deps
}

// Deps 汇总各层已装配的依赖，供 API、CLI 等复用。
type Deps struct {
	Cipher *crypto.Cipher
	Clock  clock.Clock

	Accounts     *repository.AccountRepository
	Sources      *repository.SourceRepository
	Sinks        *repository.SinkRepository
	Templates    *repository.TemplateRepository
	Rules        *repository.RuleRepository
	Messages     *repository.MessageRepository
	Deliveries   *repository.DeliveryRepository
	APITokens    *repository.APITokenRepository
	Admins       *repository.AdminRepository
	TelegramApps *repository.TelegramAppRepository
	Proxies      *repository.ProxyConfigRepository

	Engine   *ruleengine.Engine
	Renderer *tmpl.Renderer
	Queue    *dispatch.Queue
	Ingest   *appingest.Service
	Delivery *appdelivery.Service
}

// Build 根据配置装配 App。
func Build(cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Log.Level, cfg.Log.Format)

	cipher, err := crypto.NewCipher([]byte(cfg.Security.EncryptionKey))
	if err != nil {
		return nil, fmt.Errorf("初始化加密失败: %w", err)
	}

	db, err := storage.Open(cfg.Database)
	if err != nil {
		return nil, err
	}

	if cfg.Database.AutoMigrate {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("获取底层连接失败: %w", err)
		}
		if err := storagemigrate.Run(context.Background(), sqlDB, "up"); err != nil {
			return nil, fmt.Errorf("自动迁移失败: %w", err)
		}
		log.Info("数据库自动迁移完成")
	}

	clk := clock.System{}

	// 仓储装配。
	accounts := repository.NewAccountRepository(db, cipher)
	sources := repository.NewSourceRepository(db)
	sinks := repository.NewSinkRepository(db, cipher)
	templates := repository.NewTemplateRepository(db)
	rules := repository.NewRuleRepository(db)
	messages := repository.NewMessageRepository(db)
	deliveries := repository.NewDeliveryRepository(db)
	peers := repository.NewTelegramPeerRepository(db)
	apiTokens := repository.NewAPITokenRepository(db)
	admins := repository.NewAdminRepository(db)
	telegramApps := repository.NewTelegramAppRepository(db, cipher)
	proxies := repository.NewProxyConfigRepository(db, cipher)
	loginFlows := repository.NewTelegramLoginFlowRepository(db, cipher)

	// 规则引擎、渲染器、投递队列。
	engine := ruleengine.NewEngine()
	renderer := tmpl.NewRenderer()
	deliveryNotifier := dispatch.NewNotifier()
	queue := dispatch.NewQueue(deliveries, cfg.Dispatch.MaxAttempts, deliveryNotifier).UseSinks(sinks)
	ingestSvc := appingest.NewService(messages, rules, engine, queue, clk, log)
	deliverySvc := appdelivery.NewService(deliveries, appdelivery.DisplayDeps{
		Messages:  messages,
		Sources:   sources,
		Sinks:     sinks,
		Rules:     rules,
		Templates: templates,
	})

	// Telegram Source 插件（deps 注入，避免 plugin 直连存储层）。
	tgPlugin := tgsource.NewPlugin(tgsource.Deps{
		Peers: peers,
		Log:   log,
		LoadSession: func(ctx context.Context, accountID int64) ([]byte, error) {
			acc, err := accounts.GetByID(ctx, accountID)
			if err != nil {
				return nil, err
			}
			return acc.Session, nil
		},
		SaveSession: func(ctx context.Context, accountID int64, sess []byte) error {
			acc, err := accounts.GetByID(ctx, accountID)
			if err != nil {
				return err
			}
			acc.Session = sess
			acc.Status = domainaccount.StatusActive
			return accounts.Update(ctx, acc)
		},
	})
	pluginsource.Register("telegram", func() (pluginsource.Plugin, error) { return tgPlugin, nil })
	srcManager := appsource.NewManager(accounts, sources, tgPlugin, ingestSvc, log)

	// worker 装配。
	workerCount := cfg.Dispatch.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}
	workers := make([]*dispatch.Worker, 0, workerCount)
	for i := 0; i < workerCount; i++ {
		id := fmt.Sprintf("worker-%d", i+1)
		workers = append(workers, dispatch.NewWorker(
			id, cfg.Dispatch, deliveries, sinks, templates, messages, renderer, clk, log, deliveryNotifier,
		))
	}

	validator := security.TokenValidator{
		Lookup: func(hash string) (bool, error) {
			now := time.Now()
			sess, user, err := admins.GetActiveSessionByHash(context.Background(), hash, now)
			if err != nil {
				return false, err
			}
			if sess != nil && user != nil {
				_ = admins.TouchSession(context.Background(), sess.ID, now)
				return true, nil
			}
			return apiTokens.ExistsActiveHash(context.Background(), hash)
		},
	}

	// 应用服务装配。
	accountSvc := appaccount.NewService(accounts, telegramApps, proxies)
	sinkSvc := appsink.NewService(sinks)
	templateSvc := apptemplate.NewService(templates)
	ruleSvc := apprule.NewService(rules, apprule.ValidatorDeps{Sinks: sinks, Templates: templates})
	sourceSvc := appsource.NewService(sources, accounts, tgPlugin, srcManager)
	tokenSvc := apptoken.NewService(apiTokens)
	authSvc := appauth.NewService(admins, apiTokens, tokenSvc, cfg.Security.AuthEnabled)
	tgConfigSvc := apptelegramconfig.NewService(telegramApps, proxies)
	tgLoginRunner := infratelegram.LoginFlowService{}
	tgQRRunner := infratelegram.NewQRSessionManager()
	tgLoginSvc := apptelegramlogin.NewService(accounts, loginFlows, tgLoginRunner, tgQRRunner, clk, log)

	router := api.NewRouter(api.Deps{
		Logger:         log,
		TokenValidator: validator,
		AuthEnabled:    cfg.Security.AuthEnabled,
		WebDir:         cfg.Server.WebDir,
		Auth:           handler.NewAuthHandler(authSvc),
		Account:        handler.NewAccountHandler(accountSvc),
		AccountLogin:   handler.NewAccountLoginHandler(tgLoginSvc),
		Sink:           handler.NewSinkHandler(sinkSvc),
		Source:         handler.NewSourceHandler(sourceSvc),
		Template:       handler.NewTemplateHandler(templateSvc),
		Rule:           handler.NewRuleHandler(ruleSvc),
		Delivery:       handler.NewDeliveryHandler(deliverySvc),
		Token:          handler.NewTokenHandler(tokenSvc),
		TelegramConfig: handler.NewTelegramConfigHandler(tgConfigSvc),
	})

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	deps := &Deps{
		Cipher:       cipher,
		Clock:        clk,
		Accounts:     accounts,
		Sources:      sources,
		Sinks:        sinks,
		Templates:    templates,
		Rules:        rules,
		Messages:     messages,
		Deliveries:   deliveries,
		APITokens:    apiTokens,
		Admins:       admins,
		TelegramApps: telegramApps,
		Proxies:      proxies,
		Engine:       engine,
		Renderer:     renderer,
		Queue:        queue,
		Ingest:       ingestSvc,
		Delivery:     deliverySvc,
	}

	return &App{cfg: cfg, log: log, server: server, workers: workers, srcManager: srcManager, tgLogin: tgLoginSvc, deps: deps}, nil
}

// Handler 返回已装配的 HTTP handler，供集成测试使用。
func (a *App) Handler() http.Handler {
	return a.server.Handler
}

// Deps 返回已装配的依赖，供集成测试使用。
func (a *App) Dependencies() *Deps {
	return a.deps
}

// Run 启动投递 worker 与 HTTP 服务，阻塞直到 ctx 取消后优雅关闭。
func (a *App) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, w := range a.workers {
		wg.Add(1)
		go func(worker *dispatch.Worker) {
			defer wg.Done()
			worker.Run(ctx)
		}(w)
	}
	a.log.Info("投递 worker 已启动", "count", len(a.workers))

	// 清理服务重启前遗留的过期登录 flow。
	if err := a.tgLogin.RecoverStale(ctx); err != nil {
		a.log.Error("清理过期登录 flow 失败", "err", err)
	}

	// 启动已启用且账号可用的监听源。
	if err := a.srcManager.StartAll(ctx); err != nil {
		a.log.Error("启动监听源失败", "err", err)
	}

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("HTTP 服务启动", "addr", a.cfg.Server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		a.log.Info("收到关闭信号，正在优雅关闭")
		a.srcManager.StopAll(context.Background())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.server.Shutdown(shutdownCtx)
		wg.Wait() // 等待 worker 收到 ctx 取消后退出
		return err
	}
}
