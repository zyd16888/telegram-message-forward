// Package ingest 编排「标准消息 → 落库 → 规则匹配 → 生成投递任务」主链路。
//
// 它是 Source 插件与投递队列之间的入口，不解析 Telegram 原始消息，
// 只接收已标准化的 NormalizedMessage。
package ingest

import (
	"context"
	"log/slog"

	"telegram-message-forward/internal/dispatch"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/mediastore"
	"telegram-message-forward/internal/ruleengine"
)

// Service 是消息 ingest 应用服务。
type Service struct {
	messages domainmessage.Repository
	rules    domainrule.Repository
	engine   *ruleengine.Engine
	queue    *dispatch.Queue
	clock    clock.Clock
	log      *slog.Logger
	media    mediastore.Store
}

// NewService 创建 ingest 服务。
func NewService(
	messages domainmessage.Repository,
	rules domainrule.Repository,
	engine *ruleengine.Engine,
	queue *dispatch.Queue,
	clk clock.Clock,
	log *slog.Logger,
) *Service {
	return &Service{
		messages: messages, rules: rules, engine: engine,
		queue: queue, clock: clk, log: log,
	}
}

// UseMediaStore 注入媒体存储：Source 下载到临时目录的媒体在落库前收编到存储层。
func (s *Service) UseMediaStore(store mediastore.Store) *Service {
	s.media = store
	return s
}

// Ingest 处理一条标准化消息：幂等落库 → 匹配规则 → 生成投递任务。
//
// 落库使用 (source_id, external_message_id) 幂等约束；重复消息不会重复投递。
func (s *Service) Ingest(ctx context.Context, msg *domainmessage.NormalizedMessage) error {
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = s.clock.Now()
	}
	if msg.MessageType == "" {
		msg.MessageType = "text"
	}

	s.persistMedia(ctx, msg)

	if err := s.messages.Create(ctx, msg); err != nil {
		return err
	}

	rules, err := s.rules.ListEnabledBySource(ctx, msg.SourceID)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	matches, err := s.engine.Evaluate(ctx, msg, rules)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	if err := s.queue.Enqueue(ctx, msg, matches); err != nil {
		return err
	}

	s.log.Info("消息已入队投递",
		"message_id", msg.ID,
		"source_id", msg.SourceID,
		"matched_rules", len(matches),
	)
	return nil
}

// persistMedia 把 Source 下载到临时目录的媒体文件移入媒体存储，
// 并把 LocalPath 更新为存储层路径。失败不阻断消息主链路。
func (s *Service) persistMedia(ctx context.Context, msg *domainmessage.NormalizedMessage) {
	if s.media == nil {
		return
	}
	for i := range msg.Media {
		m := &msg.Media[i]
		if m.LocalPath == "" || m.StorageKey == "" {
			continue
		}
		newPath, err := s.media.PutFile(ctx, m.StorageKey, m.LocalPath)
		if newPath != "" {
			// 对象存储上传失败时文件仍已移入本地缓存，保留本地路径降级使用。
			m.LocalPath = newPath
		}
		if err != nil {
			s.log.Warn("媒体收编存储失败",
				"source_id", msg.SourceID,
				"storage_key", m.StorageKey,
				"err", err,
			)
		}
	}
}
