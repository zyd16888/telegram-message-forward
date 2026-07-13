// Package dashboard 提供管理后台首页聚合统计。
package dashboard

import (
	"context"
	"fmt"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainsettings "telegram-message-forward/internal/domain/settings"
	domainsink "telegram-message-forward/internal/domain/sink"
	domainsource "telegram-message-forward/internal/domain/source"
)

// Summary 是 Dashboard 一次请求返回的聚合数据。
type Summary struct {
	SinceHours  int
	Resources   ResourceCounts
	Status      map[string]int64
	WindowTotal int64
	Queue       QueueSummary
	TopFailures TopFailures
	Setup       SetupStatus
	AI          AIStats
}

// AIStats 是近窗 AI 整理概况。
type AIStats struct {
	Profiles int64 `json:"profiles"`
	Runs     int64 `json:"runs"`
	Success  int64 `json:"success"`
	Failed   int64 `json:"failed"`
	Tokens   int64 `json:"tokens"`
}

// AIStatsProvider 提供 AI 运行聚合。
type AIStatsProvider interface {
	GlobalStats(ctx context.Context, sinceHours int) (runs, success, failed, tokens int64, err error)
	CountProfiles(ctx context.Context) (int64, error)
}

// ResourceCounts 是资源实体计数。
type ResourceCounts struct {
	Accounts int64
	Sources  int64
	Sinks    int64
	Flows    int64
}

// QueueSummary 是当前队列积压（不限时间窗）。
type QueueSummary struct {
	Pending    int64
	Processing int64
	Retrying   int64
}

// FailureBucket 是失败 Top 的一项。
type FailureBucket struct {
	Key   string
	Label string
	Count int64
}

// TopFailures 按 sink / flow / source 聚合失败。
type TopFailures struct {
	Sink   []FailureBucket
	Flow   []FailureBucket
	Source []FailureBucket
}

// SetupStatus 是首次配置检查清单布尔项。
type SetupStatus struct {
	HasTelegramApp      bool
	HasActiveAccount    bool
	HasEnabledSource    bool
	HasEnabledSink      bool
	HasEnabledFlow      bool
	HasMediaPublicURL   bool
	MediaURLRecommended bool
}

// DeliveryStats 提供投递状态聚合与失败 Top。
type DeliveryStats interface {
	CountByStatusSince(ctx context.Context, since *time.Time) (map[string]int64, error)
	CountQueue(ctx context.Context) (pending, processing, retrying int64, err error)
	FailureTopSince(ctx context.Context, since time.Time, limit int) (sinks, flows, sources []FailureBucket, err error)
}

// Counter 提供简单的资源计数。
type Counter interface {
	Count(ctx context.Context) (int64, error)
}

// AccountLister 用于判断是否存在 active 账号。
type AccountLister interface {
	List(ctx context.Context) ([]*domainaccount.Account, error)
}

// SourceLister 用于启用源判断。
type SourceLister interface {
	List(ctx context.Context) ([]*domainsource.Source, error)
}

// SinkLister 用于启用渠道与媒体需求判断。
type SinkLister interface {
	List(ctx context.Context) ([]*domainsink.Sink, error)
}

// FlowLister 用于启用 Flow 判断。
type FlowLister interface {
	List(ctx context.Context) ([]*domainflow.Flow, error)
}

// SettingsReader 读取媒体公网 URL 等设置。
type SettingsReader interface {
	Get(ctx context.Context, key string) (*domainsettings.Setting, error)
}

// TelegramAppCounter 统计已配置的 Telegram App。
type TelegramAppCounter interface {
	Count(ctx context.Context) (int64, error)
}

// MediaPublicURL 从设置读取公网媒体 URL 是否已配置。
type MediaPublicURL func(ctx context.Context) (bool, error)

// Deps 是 Dashboard 服务依赖。
type Deps struct {
	Deliveries   DeliveryStats
	Accounts     AccountLister
	Sources      SourceLister
	Sinks        SinkLister
	Flows        FlowLister
	TelegramApps TelegramAppCounter
	MediaURL     MediaPublicURL
	AI           AIStatsProvider
	Now          func() time.Time
}

// Service 聚合 Dashboard 数据。
type Service struct {
	deps Deps
}

// NewService 创建 Dashboard 服务。
func NewService(deps Deps) *Service {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Service{deps: deps}
}

// Summary 返回近 sinceHours 小时的聚合统计；sinceHours<=0 时默认 24。
func (s *Service) Summary(ctx context.Context, sinceHours int) (*Summary, error) {
	if sinceHours <= 0 {
		sinceHours = 24
	}
	since := s.deps.Now().Add(-time.Duration(sinceHours) * time.Hour)

	status, err := s.deps.Deliveries.CountByStatusSince(ctx, &since)
	if err != nil {
		return nil, fmt.Errorf("统计投递状态失败: %w", err)
	}
	var windowTotal int64
	for _, n := range status {
		windowTotal += n
	}

	pending, processing, retrying, err := s.deps.Deliveries.CountQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("统计队列失败: %w", err)
	}

	sinkTop, flowTop, sourceTop, err := s.deps.Deliveries.FailureTopSince(ctx, since, 5)
	if err != nil {
		return nil, fmt.Errorf("统计失败 Top 失败: %w", err)
	}

	resources, setup, err := s.resourceAndSetup(ctx)
	if err != nil {
		return nil, err
	}

	aiStats := AIStats{}
	if s.deps.AI != nil {
		runs, success, failed, tokens, aerr := s.deps.AI.GlobalStats(ctx, sinceHours)
		if aerr != nil {
			return nil, fmt.Errorf("统计 AI 运行失败: %w", aerr)
		}
		profiles, perr := s.deps.AI.CountProfiles(ctx)
		if perr != nil {
			return nil, fmt.Errorf("统计 AI Profile 失败: %w", perr)
		}
		aiStats = AIStats{Profiles: profiles, Runs: runs, Success: success, Failed: failed, Tokens: tokens}
	}

	return &Summary{
		SinceHours:  sinceHours,
		Resources:   resources,
		Status:      status,
		WindowTotal: windowTotal,
		Queue: QueueSummary{
			Pending:    pending,
			Processing: processing,
			Retrying:   retrying,
		},
		TopFailures: TopFailures{
			Sink:   sinkTop,
			Flow:   flowTop,
			Source: sourceTop,
		},
		Setup: setup,
		AI:    aiStats,
	}, nil
}

func (s *Service) resourceAndSetup(ctx context.Context) (ResourceCounts, SetupStatus, error) {
	var res ResourceCounts
	var setup SetupStatus

	if s.deps.Accounts != nil {
		accs, err := s.deps.Accounts.List(ctx)
		if err != nil {
			return res, setup, fmt.Errorf("列出账号失败: %w", err)
		}
		res.Accounts = int64(len(accs))
		for _, a := range accs {
			if a != nil && a.Status == domainaccount.StatusActive {
				setup.HasActiveAccount = true
				break
			}
		}
	}
	if s.deps.Sources != nil {
		srcs, err := s.deps.Sources.List(ctx)
		if err != nil {
			return res, setup, fmt.Errorf("列出监听源失败: %w", err)
		}
		res.Sources = int64(len(srcs))
		for _, src := range srcs {
			if src != nil && src.Enabled {
				setup.HasEnabledSource = true
				break
			}
		}
	}
	if s.deps.Sinks != nil {
		snks, err := s.deps.Sinks.List(ctx)
		if err != nil {
			return res, setup, fmt.Errorf("列出渠道失败: %w", err)
		}
		res.Sinks = int64(len(snks))
		for _, snk := range snks {
			if snk != nil && snk.Enabled {
				setup.HasEnabledSink = true
				break
			}
		}
		// 部分渠道依赖公网媒体 URL（钉钉/Bark/Gotify 等）；有启用渠道时提示检查媒体配置。
		setup.MediaURLRecommended = setup.HasEnabledSink
	}
	if s.deps.Flows != nil {
		fls, err := s.deps.Flows.List(ctx)
		if err != nil {
			return res, setup, fmt.Errorf("列出 Flow 失败: %w", err)
		}
		res.Flows = int64(len(fls))
		for _, f := range fls {
			if f != nil && f.Enabled {
				setup.HasEnabledFlow = true
				break
			}
		}
	}
	if s.deps.TelegramApps != nil {
		n, err := s.deps.TelegramApps.Count(ctx)
		if err != nil {
			return res, setup, fmt.Errorf("统计 Telegram App 失败: %w", err)
		}
		setup.HasTelegramApp = n > 0
	}
	if s.deps.MediaURL != nil {
		ok, err := s.deps.MediaURL(ctx)
		if err != nil {
			return res, setup, err
		}
		setup.HasMediaPublicURL = ok
	}
	return res, setup, nil
}

// 确保 domaindelivery 状态常量被引用（避免未使用包在精简构建中被误删）。
var _ = domaindelivery.StatusPending
