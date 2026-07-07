// Package bootstrap 装配各层依赖并启动应用。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"telegram-message-forward/internal/api"
	"telegram-message-forward/internal/api/handler"
	appaccount "telegram-message-forward/internal/app/account"
	appaidigest "telegram-message-forward/internal/app/aidigest"
	apptoken "telegram-message-forward/internal/app/apitoken"
	appauth "telegram-message-forward/internal/app/auth"
	appdelivery "telegram-message-forward/internal/app/delivery"
	appfilter "telegram-message-forward/internal/app/filter"
	appflow "telegram-message-forward/internal/app/flow"
	appingest "telegram-message-forward/internal/app/ingest"
	apprule "telegram-message-forward/internal/app/rule"
	appsettings "telegram-message-forward/internal/app/settings"
	appsink "telegram-message-forward/internal/app/sink"
	appsource "telegram-message-forward/internal/app/source"
	apptelegramconfig "telegram-message-forward/internal/app/telegramconfig"
	apptelegramlogin "telegram-message-forward/internal/app/telegramlogin"
	apptemplate "telegram-message-forward/internal/app/template"
	"telegram-message-forward/internal/config"
	"telegram-message-forward/internal/dispatch"
	domainaccount "telegram-message-forward/internal/domain/account"
	"telegram-message-forward/internal/flowengine"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/infra/logger"
	"telegram-message-forward/internal/infra/mediastore"
	infratelegram "telegram-message-forward/internal/infra/telegram"
	pluginsource "telegram-message-forward/internal/plugin/source"
	rsssource "telegram-message-forward/internal/plugin/source/rss"
	tgsource "telegram-message-forward/internal/plugin/source/telegram"
	webhooksource "telegram-message-forward/internal/plugin/source/webhook"
	"telegram-message-forward/internal/ruleengine"
	"telegram-message-forward/internal/security"
	"telegram-message-forward/internal/storage"
	storagemigrate "telegram-message-forward/internal/storage/migrate"
	"telegram-message-forward/internal/storage/repository"
	tmpl "telegram-message-forward/internal/template"

	// 通过 blank import 触发内置 Sink 插件的编译期注册。
	_ "telegram-message-forward/internal/plugin/sink/bark"
	_ "telegram-message-forward/internal/plugin/sink/dingtalk"
	_ "telegram-message-forward/internal/plugin/sink/email"
	_ "telegram-message-forward/internal/plugin/sink/feishu"
	_ "telegram-message-forward/internal/plugin/sink/gotify"
	_ "telegram-message-forward/internal/plugin/sink/ntfy"
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
	aiSched    *appaidigest.Scheduler
	mediaStore *mediastore.Manager
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
	Flows        *repository.FlowRepository
	Messages     *repository.MessageRepository
	Deliveries   *repository.DeliveryRepository
	APITokens    *repository.APITokenRepository
	Admins       *repository.AdminRepository
	TelegramApps *repository.TelegramAppRepository
	Proxies      *repository.ProxyConfigRepository

	Engine     *ruleengine.Engine
	FlowEngine *flowengine.Engine
	Renderer   *tmpl.Renderer
	Queue      *dispatch.Queue
	Ingest     *appingest.Service
	Delivery   *appdelivery.Service
	AIDigest   *appaidigest.Service
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
	filters := repository.NewFilterRepository(db)
	flows := repository.NewFlowRepository(db)
	messages := repository.NewMessageRepository(db)
	deliveries := repository.NewDeliveryRepository(db)
	peers := repository.NewTelegramPeerRepository(db)
	apiTokens := repository.NewAPITokenRepository(db)
	admins := repository.NewAdminRepository(db)
	telegramApps := repository.NewTelegramAppRepository(db, cipher)
	proxies := repository.NewProxyConfigRepository(db, cipher)
	loginFlows := repository.NewTelegramLoginFlowRepository(db, cipher)
	aiDigests := repository.NewAIDigestRepository(db)

	// 规则引擎、渲染器、投递队列。
	engine := ruleengine.NewEngine()
	flowEngine := flowengine.NewEngine(flowengine.WithFilterResolver(filters))
	renderer := tmpl.NewRenderer()
	deliveryNotifier := dispatch.NewNotifier()
	queue := dispatch.NewQueue(deliveries, cfg.Dispatch.MaxAttempts, deliveryNotifier).UseSinks(sinks)

	// 媒体存储：设置服务提供生效配置（页面保存的设置优先，配置文件作默认），
	// Manager 支持设置保存后热重载，无需重启。
	settingsRepo := repository.NewSettingRepository(db, cipher)
	settingsSvc := appsettings.NewService(settingsRepo, cfg.Media)
	mediaStore := mediastore.NewManager()
	mediaSignKey := []byte(cfg.Security.EncryptionKey)
	// Telegram 下载策略随媒体设置一起热更新：reloadMedia 时刷新，插件通过闭包读取。
	var downloadPolicy atomic.Pointer[tgsource.DownloadPolicy]
	reloadMedia := func(ms appsettings.MediaSettings, s3Secret string) error {
		policy := downloadPolicyFrom(ms)
		downloadPolicy.Store(&policy)
		local := mediastore.NewLocal(ms.Dir, ms.PublicBaseURL, mediaSignKey, ms.URLTTL())
		var store mediastore.Store = local
		if ms.S3.Enabled {
			s3Store, err := mediastore.NewS3(mediaS3Options(ms, s3Secret), local)
			if err != nil {
				return fmt.Errorf("初始化 S3 媒体存储失败: %w", err)
			}
			store = s3Store
		}
		mediaStore.Swap(store, local, ms.Retention())
		return nil
	}
	settingsSvc.SetMediaReloader(reloadMedia)
	settingsSvc.SetMediaTester(func(ctx context.Context, ms appsettings.MediaSettings, s3Secret string) error {
		return mediastore.TestS3(ctx, mediaS3Options(ms, s3Secret))
	})
	initialMedia, initialS3Secret, _, err := settingsSvc.EffectiveMedia(context.Background())
	if err != nil {
		return nil, fmt.Errorf("加载媒体设置失败: %w", err)
	}
	if err := reloadMedia(initialMedia, initialS3Secret); err != nil {
		// 数据库里存了无效的 S3 配置时不阻止启动，退回本地存储，用户可在页面修复。
		log.Error("媒体存储初始化失败，暂时退回本地存储", "err", err)
		local := mediastore.NewLocal(initialMedia.Dir, initialMedia.PublicBaseURL, mediaSignKey, initialMedia.URLTTL())
		mediaStore.Swap(local, local, initialMedia.Retention())
	}

	ingestSvc := appingest.NewService(messages, rules, engine, queue, clk, log).
		UseMediaStore(mediaStore).
		UseFlowEngine(flows, flowEngine, cfg.FlowEngine.Mode)
	deliverySvc := appdelivery.NewService(deliveries, appdelivery.DisplayDeps{
		Messages:  messages,
		Sources:   sources,
		Sinks:     sinks,
		Rules:     rules,
		Flows:     flows,
		Templates: templates,
	})
	aiDigestSvc := appaidigest.NewService(appaidigest.Deps{
		Repo:     aiDigests,
		Settings: settingsRepo,
		Messages: messages,
		Sources:  sources,
		Sinks:    sinks,
		Tasks:    deliveries,
		Filters:  filters,
		Clock:    clk,
		Logger:   log,
		Wake:     deliveryNotifier.Notify,
	})
	aiScheduler := appaidigest.NewScheduler(aiDigestSvc, log)

	// Telegram Source 插件（deps 注入，避免 plugin 直连存储层）。
	tgPlugin := tgsource.NewPlugin(tgsource.Deps{
		Peers: peers,
		Log:   log,
		DownloadPolicy: func() tgsource.DownloadPolicy {
			if p := downloadPolicy.Load(); p != nil {
				return *p
			}
			return tgsource.DownloadPolicy{}
		},
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
	rssPlugin := rsssource.New()
	webhookPlugin := webhooksource.New()
	srcManager := appsource.NewManager(accounts, sources, tgPlugin, ingestSvc, log)
	srcManager.RegisterPlugin("rss", rssPlugin)
	srcManager.RegisterPlugin("webhook", webhookPlugin)

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
		).UseMediaStore(mediaStore))
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
	sinkSvc := appsink.NewService(sinks, deliveries)
	templateSvc := apptemplate.NewService(templates)
	ruleSvc := apprule.NewService(rules, apprule.ValidatorDeps{Sinks: sinks, Templates: templates, Filters: filters})
	flowSvc := appflow.NewService(flows, flowEngine, appflow.ValidatorDeps{Sources: sources, Sinks: sinks, Templates: templates, Filters: filters})
	filterSvc := appfilter.NewService(filters)
	sourceSvc := appsource.NewService(sources, accounts, tgPlugin, srcManager)
	sourceSvc.RegisterPlugin("rss", rssPlugin)
	sourceSvc.RegisterPlugin("webhook", webhookPlugin)
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
		Flow:           handler.NewFlowHandler(flowSvc),
		Filter:         handler.NewFilterHandler(filterSvc),
		Delivery:       handler.NewDeliveryHandler(deliverySvc),
		AIDigest:       handler.NewAIDigestHandler(aiDigestSvc),
		Token:          handler.NewTokenHandler(tokenSvc),
		TelegramConfig: handler.NewTelegramConfigHandler(tgConfigSvc),
		Settings:       handler.NewSettingsHandler(settingsSvc),
		Media:          handler.NewMediaHandler(mediaStore),
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
		Flows:        flows,
		Messages:     messages,
		Deliveries:   deliveries,
		APITokens:    apiTokens,
		Admins:       admins,
		TelegramApps: telegramApps,
		Proxies:      proxies,
		Engine:       engine,
		FlowEngine:   flowEngine,
		Renderer:     renderer,
		Queue:        queue,
		Ingest:       ingestSvc,
		Delivery:     deliverySvc,
		AIDigest:     aiDigestSvc,
	}

	return &App{cfg: cfg, log: log, server: server, workers: workers, srcManager: srcManager, tgLogin: tgLoginSvc, aiSched: aiScheduler, mediaStore: mediaStore, deps: deps}, nil
}

// Handler 返回已装配的 HTTP handler，供集成测试使用。
func (a *App) Handler() http.Handler {
	return a.server.Handler
}

// Deps 返回已装配的依赖，供集成测试使用。
func (a *App) Dependencies() *Deps {
	return a.deps
}

// downloadPolicyFrom 把媒体设置的下载策略段转换为 Telegram 插件的下载策略（MB → 字节）。
func downloadPolicyFrom(ms appsettings.MediaSettings) tgsource.DownloadPolicy {
	toBytes := func(mb float64) int64 {
		if mb <= 0 {
			return 0
		}
		return int64(mb * 1024 * 1024)
	}
	return tgsource.DownloadPolicy{
		ImageMaxBytes: toBytes(ms.Download.ImageMaxMB),
		FileMaxBytes:  toBytes(ms.Download.FileMaxMB),
		FileTypes:     ms.Download.FileTypes,
	}
}

// mediaS3Options 把媒体设置转换为 mediastore 的 S3 连接参数。
func mediaS3Options(ms appsettings.MediaSettings, s3Secret string) mediastore.S3Options {
	return mediastore.S3Options{
		Endpoint:      ms.S3.Endpoint,
		Region:        ms.S3.Region,
		Bucket:        ms.S3.Bucket,
		AccessKey:     ms.S3.AccessKey,
		SecretKey:     s3Secret,
		UseSSL:        ms.S3.UseSSL,
		KeyPrefix:     ms.S3.KeyPrefix,
		PublicBaseURL: ms.S3.PublicBaseURL,
		URLTTL:        ms.URLTTL(),
		AutoCleanup:   ms.S3.AutoCleanup,
	}
}

// runMediaCleanup 启动时先清理一次超过保留期的媒体，之后每小时清理一次。
// 保留期从设置热读取；<=0 时跳过（页面可随时开关，无需重启）。
func (a *App) runMediaCleanup(ctx context.Context) {
	cleanup := func() {
		retention := a.mediaStore.Retention()
		if retention <= 0 {
			return
		}
		removed, err := a.mediaStore.Cleanup(ctx, retention)
		if err != nil {
			a.log.Warn("清理媒体缓存失败", "err", err)
		}
		if removed > 0 {
			a.log.Info("已清理过期媒体文件", "count", removed)
		}
	}
	cleanup()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
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

	// 按保留期定时清理本地媒体缓存（保留期在设置里动态调整）。
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.runMediaCleanup(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		a.aiSched.Run(ctx)
	}()

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
