// Package source 提供监听源的生命周期编排（启动/停止监听）。
//
// CRUD 应用服务在 M4 补充；本文件先负责把已启用、账号可用的 source 接入 ingest。
package source

import (
	"context"
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
	plugin   pluginsource.Plugin
	ingest   *appingest.Service
	log      *slog.Logger
}

// NewManager 创建 Source 生命周期管理器。
func NewManager(
	accounts domainaccount.Repository,
	sources domainsource.Repository,
	plugin pluginsource.Plugin,
	ingest *appingest.Service,
	log *slog.Logger,
) *Manager {
	return &Manager{accounts: accounts, sources: sources, plugin: plugin, ingest: ingest, log: log}
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
		acc, err := m.accounts.GetByID(ctx, src.AccountID)
		if err != nil {
			m.log.Warn("跳过 source：账号查询失败", "source", src.ID, "err", err)
			continue
		}
		if acc.Status != domainaccount.StatusActive {
			m.log.Info("跳过 source：账号未登录", "source", src.ID, "account", acc.ID, "status", acc.Status)
			continue
		}
		if err := m.plugin.Start(ctx, acc, src, m.ingest.Ingest); err != nil {
			m.log.Error("启动 source 监听失败", "source", src.ID, "err", err)
			continue
		}
		started++
	}
	m.log.Info("Source 监听已启动", "count", started, "total", len(srcs))
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
		if err := m.plugin.Stop(ctx, src); err != nil {
			m.log.Warn("停止 source 失败", "source", src.ID, "err", err)
		}
	}
}
