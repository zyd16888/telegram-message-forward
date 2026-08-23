// Package ingest 编排「标准消息 → 落库 → Flow 求值 → 生成投递任务」主链路。
//
// 它是 Source 插件与投递队列之间的入口，不解析 Telegram 原始消息，
// 只接收已标准化的 NormalizedMessage。
package ingest

import (
	"context"
	"log/slog"

	"telegram-message-forward/internal/dispatch"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
	"telegram-message-forward/internal/flowengine"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/mediastore"
)

// sourceCursor 在成功落库后推进监听源 last_message_id。
type sourceCursor interface {
	AdvanceLastMessageID(ctx context.Context, sourceID, messageID int64) error
}

// Service 是消息 ingest 应用服务。
type Service struct {
	messages   domainmessage.Repository
	flows      domainflow.Repository
	flowEngine *flowengine.Engine
	queue      *dispatch.Queue
	clock      clock.Clock
	log        *slog.Logger
	media      mediastore.Store
	cursors    sourceCursor
}

// NewService 创建 ingest 服务。
func NewService(
	messages domainmessage.Repository,
	flows domainflow.Repository,
	flowEngine *flowengine.Engine,
	queue *dispatch.Queue,
	clk clock.Clock,
	log *slog.Logger,
) *Service {
	return &Service{
		messages: messages, flows: flows, flowEngine: flowEngine,
		queue: queue, clock: clk, log: log,
	}
}

// UseMediaStore 注入媒体存储：Source 下载到临时目录的媒体在落库前收编到存储层。
func (s *Service) UseMediaStore(store mediastore.Store) *Service {
	s.media = store
	return s
}

// UseSourceCursor 注入游标推进（Telegram 历史补拉/实时共用 last_message_id）。
func (s *Service) UseSourceCursor(c sourceCursor) *Service {
	s.cursors = c
	return s
}

// Ingest 处理一条标准化消息：幂等落库 → 求值 Flow → 生成投递任务。
//
// 落库使用 (source_id, external_message_id) 幂等约束；重复消息不会重复投递。
func (s *Service) Ingest(ctx context.Context, msg *domainmessage.NormalizedMessage) error {
	if msg != nil && msg.EventKind == domainmessage.EventEdit {
		return s.ingestEdit(ctx, msg)
	}
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
	// 游标推进独立于是否命中 Flow：只要成功收到并落库就前进，避免重复补拉。
	// SkipCursorAdvance 的手动回捞路径除外：它拉的是「最近 N 条」而非连续区间，
	// 推进游标会让中间未覆盖的部分被自动追平永久跳过。
	if s.cursors != nil && msg.ExternalMessageID > 0 && !msg.SkipCursorAdvance {
		if err := s.cursors.AdvanceLastMessageID(ctx, msg.SourceID, msg.ExternalMessageID); err != nil {
			s.log.Warn("推进 last_message_id 失败", "source_id", msg.SourceID, "external_id", msg.ExternalMessageID, "err", err)
		}
	}

	matches, err := s.evaluate(ctx, msg)
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
		"matched_flows", len(matches),
	)
	return nil
}

func (s *Service) ingestEdit(ctx context.Context, msg *domainmessage.NormalizedMessage) error {
	if msg == nil {
		return nil
	}
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = s.clock.Now()
	}
	if msg.MessageType == "" {
		msg.MessageType = "text"
	}
	s.persistMedia(ctx, msg)
	result, err := s.messages.ApplyEdit(ctx, msg)
	if err != nil {
		return err
	}
	if !result.Found {
		s.log.Debug("忽略未采集原消息的 Telegram 编辑事件",
			"source_id", msg.SourceID, "external_id", msg.ExternalMessageID)
		return nil
	}
	if !result.Changed {
		return nil
	}
	matches, err := s.evaluate(ctx, msg)
	if err != nil {
		return err
	}
	if err := s.queue.EnqueueEdit(ctx, msg, matches, msg.EditTextSuffix); err != nil {
		return err
	}
	s.log.Info("Telegram 编辑消息已入队补发",
		"message_id", msg.ID,
		"source_id", msg.SourceID,
		"external_id", msg.ExternalMessageID,
		"revision", msg.ContentRevision,
	)
	return nil
}

func (s *Service) evaluate(ctx context.Context, msg *domainmessage.NormalizedMessage) ([]domainflow.Match, error) {
	if s.flows == nil || s.flowEngine == nil {
		return nil, nil
	}
	flows, err := s.flows.ListEnabledBySource(ctx, msg.SourceID)
	if err != nil || len(flows) == 0 {
		return nil, err
	}
	return s.flowEngine.Evaluate(ctx, msg, flows)
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
