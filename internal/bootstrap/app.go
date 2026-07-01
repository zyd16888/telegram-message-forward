// Package bootstrap 装配各层依赖并启动应用。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/zyd16888/telegram-message-forward/internal/api"
	"github.com/zyd16888/telegram-message-forward/internal/config"
	"github.com/zyd16888/telegram-message-forward/internal/infra/crypto"
	"github.com/zyd16888/telegram-message-forward/internal/infra/logger"
	"github.com/zyd16888/telegram-message-forward/internal/security"
	"github.com/zyd16888/telegram-message-forward/internal/storage"
	storagemigrate "github.com/zyd16888/telegram-message-forward/internal/storage/migrate"
	"github.com/zyd16888/telegram-message-forward/internal/storage/repository"
)

// App 持有已装配的运行时依赖。
type App struct {
	cfg    *config.Config
	log    *slog.Logger
	server *http.Server
}

// Build 根据配置装配 App。
func Build(cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Log.Level, cfg.Log.Format)

	if _, err := crypto.NewCipher([]byte(cfg.Security.EncryptionKey)); err != nil {
		return nil, fmt.Errorf("初始化加密失败: %w", err)
	}

	db, err := storage.Open(cfg.Database)
	if err != nil {
		return nil, err
	}

	// 开发环境可启用启动时自动迁移；生产环境建议关闭，改用 cmd/migrate。
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

	apiTokens := repository.NewAPITokenRepository(db)
	validator := security.TokenValidator{
		Lookup: func(hash string) (bool, error) {
			return apiTokens.ExistsActiveHash(context.Background(), hash)
		},
	}

	router := api.NewRouter(api.Deps{
		Logger:         log,
		TokenValidator: validator,
	})

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &App{cfg: cfg, log: log, server: server}, nil
}

// Run 启动 HTTP 服务，阻塞直到 ctx 取消后优雅关闭。
func (a *App) Run(ctx context.Context) error {
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a.server.Shutdown(shutdownCtx)
	}
}
