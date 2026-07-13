// Package source 提供监听源的生命周期编排（启动/停止监听）。
//
// CRUD 应用服务在 M4 补充；本文件先负责把已启用、账号可用的 source 接入 ingest。
package source

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	appingest "telegram-message-forward/internal/app/ingest"
	domainaccount "telegram-message-forward/internal/domain/account"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

// Manager 负责启动/停止 Source 监听，并把消息接入 ingest。
type Manager struct {
	accounts domainaccount.Repository
	sources  domainsource.Repository
	plugins  map[string]pluginsource.Plugin
	ingest   *appingest.Service
	log      *slog.Logger
}

// IngestHandler 返回标准消息 ingest 回调，供历史补拉等路径复用。
func (m *Manager) IngestHandler() pluginsource.Handler {
	if m == nil || m.ingest == nil {
		return nil
	}
	return m.ingest.Ingest
}

// NewManager 创建 Source 生命周期管理器。
func NewManager(
	accounts domainaccount.Repository,
	sources domainsource.Repository,
	plugin pluginsource.Plugin,
	ingest *appingest.Service,
	log *slog.Logger,
) *Manager {
	return &Manager{accounts: accounts, sources: sources, plugins: map[string]pluginsource.Plugin{"telegram": plugin}, ingest: ingest, log: log}
}

// RegisterPlugin 注册非 Telegram Source 插件。
func (m *Manager) RegisterPlugin(name string, plugin pluginsource.Plugin) {
	if m.plugins == nil {
		m.plugins = map[string]pluginsource.Plugin{}
	}
	m.plugins[name] = plugin
}

// StartAll 启动全部「已启用且账号处于 active」的监听源。
//
// 单个 source 启动失败只记录日志，不阻断其它 source。
func (m *Manager) StartAll(ctx context.Context) error {
	srcs, err := m.sources.ListEnabled(ctx)
	if err != nil {
		return err
	}
	started := 0
	for _, src := range srcs {
		plugin, err := m.pluginForSource(src)
		if err != nil {
			m.log.Warn("跳过 source：插件未注册", "source", src.ID, "type", sourceType(src), "err", err)
			continue
		}
		var acc *domainaccount.Account
		if sourceRequiresAccount(src) {
			acc, err = m.accounts.GetByID(ctx, src.AccountID)
			if err != nil {
				m.log.Warn("跳过 source：账号查询失败", "source", src.ID, "account", src.AccountID, "err", err)
				continue
			}
			if acc.Status != domainaccount.StatusActive {
				m.log.Info("跳过 source：账号未登录", "source", src.ID, "account", acc.ID, "status", acc.Status)
				continue
			}
		}
		if err := plugin.Start(ctx, acc, src, m.ingest.Ingest); err != nil {
			m.log.Error("启动 source 监听失败", "source", src.ID, "type", sourceType(src), "err", err)
			continue
		}
		// 启动后对开启历史补拉且已有游标的源做增量追平（新源 last_message_id=0 跳过）。
		if catcher, ok := plugin.(interface {
			CatchUpIfNeeded(context.Context, *domainaccount.Account, *domainsource.Source, pluginsource.Handler) error
		}); ok {
			if err := catcher.CatchUpIfNeeded(ctx, acc, src, m.ingest.Ingest); err != nil {
				m.log.Warn("启动历史补漏失败", "source", src.ID, "err", err)
			}
		}
		started++
	}
	m.log.Info("Source 监听已启动", "count", started, "total", len(srcs))
	return nil
}

// StartAccount 恢复指定账号下全部已启用 Source。登录成功后调用，避免必须重启服务。
func (m *Manager) StartAccount(ctx context.Context, accountID int64) error {
	acc, err := m.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if acc.Status != domainaccount.StatusActive {
		return fmt.Errorf("账号未登录: %s", acc.Status)
	}
	srcs, err := m.sources.ListEnabled(ctx)
	if err != nil {
		return err
	}
	started := 0
	var startErrs []error
	for _, src := range srcs {
		if !sourceRequiresAccount(src) || src.AccountID != accountID {
			continue
		}
		plugin, err := m.pluginForSource(src)
		if err != nil {
			m.log.Warn("恢复账号 Source 时插件未注册", "account", accountID, "source", src.ID, "err", err)
			startErrs = append(startErrs, fmt.Errorf("source %d: %w", src.ID, err))
			continue
		}
		if err := plugin.Start(ctx, acc, src, m.ingest.Ingest); err != nil {
			m.log.Error("恢复账号 Source 失败", "account", accountID, "source", src.ID, "err", err)
			startErrs = append(startErrs, fmt.Errorf("source %d: %w", src.ID, err))
			continue
		}
		if catcher, ok := plugin.(interface {
			CatchUpIfNeeded(context.Context, *domainaccount.Account, *domainsource.Source, pluginsource.Handler) error
		}); ok {
			if err := catcher.CatchUpIfNeeded(ctx, acc, src, m.ingest.Ingest); err != nil {
				m.log.Warn("恢复账号后历史补漏失败", "account", accountID, "source", src.ID, "err", err)
			}
		}
		started++
	}
	m.log.Info("账号 Source 监听已恢复", "account", accountID, "count", started)
	return errors.Join(startErrs...)
}

// StopAccount 停止指定账号的长连接，并等待连接完全退出。
func (m *Manager) StopAccount(ctx context.Context, accountID int64) error {
	for _, plugin := range m.plugins {
		if stopper, ok := plugin.(interface {
			StopAccount(context.Context, int64) error
		}); ok {
			if err := stopper.StopAccount(ctx, accountID); err != nil {
				return err
			}
		}
	}
	return nil
}

// StopAll 停止全部监听源。
func (m *Manager) StopAll(ctx context.Context) {
	srcs, err := m.sources.ListEnabled(ctx)
	if err != nil {
		m.log.Warn("停止 source 时查询失败", "err", err)
		return
	}
	for _, src := range srcs {
		plugin, err := m.pluginForSource(src)
		if err != nil {
			m.log.Warn("停止 source 时插件未注册", "source", src.ID, "type", sourceType(src), "err", err)
			continue
		}
		if err := plugin.Stop(ctx, src); err != nil {
			m.log.Warn("停止 source 失败", "source", src.ID, "err", err)
		}
	}
}

func (m *Manager) pluginForSource(src *domainsource.Source) (pluginsource.Plugin, error) {
	sourceType := sourceType(src)
	plugin, ok := m.plugins[sourceType]
	if !ok {
		return nil, fmt.Errorf("未注册的 source 插件: %s", sourceType)
	}
	return plugin, nil
}

func sourceType(src *domainsource.Source) string {
	if src == nil || src.Type == "" {
		return "telegram"
	}
	return src.Type
}
