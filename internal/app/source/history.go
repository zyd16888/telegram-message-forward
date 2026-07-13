package source

import (
	"context"
	"fmt"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
	tgsource "telegram-message-forward/internal/plugin/source/telegram"
)

// HistoryResult 是历史预览/回捞的应用层结果。
type HistoryResult struct {
	Items      []tgsource.HistoryPreviewItem `json:"items"`
	Fetched    int                           `json:"fetched"`
	Ingested   int                           `json:"ingested"`
	MaxMessage int64                         `json:"max_message_id"`
}

// HistoryPreview 预览源历史消息（不投递、不推进游标）。
func (s *Service) HistoryPreview(ctx context.Context, sourceID int64, limit int) (*HistoryResult, error) {
	src, acc, plugin, err := s.telegramHistoryReady(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	res, err := plugin.PreviewHistory(ctx, acc, src, limit)
	if err != nil {
		return nil, err
	}
	return mapHistoryResult(res), nil
}

// HistoryBackfill 确认执行历史回捞（走标准 ingest，幂等防重投）。
func (s *Service) HistoryBackfill(ctx context.Context, sourceID int64, limit int) (*HistoryResult, error) {
	src, acc, plugin, err := s.telegramHistoryReady(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if s.manager == nil {
		return nil, fmt.Errorf("source manager 未装配")
	}
	res, err := plugin.ExecuteHistoryBackfill(ctx, acc, src, 0, limit, s.manager.IngestHandler())
	if err != nil {
		return nil, err
	}
	return mapHistoryResult(res), nil
}

// CatchUpSource 在启动后对开启补拉且已有游标的源做增量追平。
func (s *Service) CatchUpSource(ctx context.Context, src *domainsource.Source) error {
	if src == nil || !tgsource.HistoryBackfillEnabled(src) || src.LastMessageID <= 0 {
		return nil
	}
	acc, plugin, err := s.telegramHistoryPlugin(ctx, src)
	if err != nil {
		return err
	}
	if s.manager == nil {
		return fmt.Errorf("source manager 未装配")
	}
	return plugin.CatchUpIfNeeded(ctx, acc, src, s.manager.IngestHandler())
}

type historyPlugin interface {
	PreviewHistory(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, limit int) (*tgsource.HistoryFetchResult, error)
	ExecuteHistoryBackfill(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, minID int64, limit int, handler pluginsource.Handler) (*tgsource.HistoryFetchResult, error)
	CatchUpIfNeeded(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, handler pluginsource.Handler) error
}

func (s *Service) telegramHistoryReady(ctx context.Context, sourceID int64) (*domainsource.Source, *domainaccount.Account, historyPlugin, error) {
	src, err := s.sources.GetByID(ctx, sourceID)
	if err != nil {
		return nil, nil, nil, err
	}
	acc, plugin, err := s.telegramHistoryPlugin(ctx, src)
	if err != nil {
		return nil, nil, nil, err
	}
	return src, acc, plugin, nil
}

func (s *Service) telegramHistoryPlugin(ctx context.Context, src *domainsource.Source) (*domainaccount.Account, historyPlugin, error) {
	if sourceType(src) != "telegram" {
		return nil, nil, fmt.Errorf("仅 Telegram 源支持历史补拉")
	}
	plugin, err := s.pluginForSource(src)
	if err != nil {
		return nil, nil, err
	}
	hp, ok := plugin.(historyPlugin)
	if !ok {
		return nil, nil, fmt.Errorf("当前 Telegram 插件未实现历史补拉")
	}
	acc, err := s.accounts.GetByID(ctx, src.AccountID)
	if err != nil {
		return nil, nil, err
	}
	if acc.Status != domainaccount.StatusActive {
		return nil, nil, fmt.Errorf("账号未登录，无法拉取历史")
	}
	return acc, hp, nil
}

func mapHistoryResult(res *tgsource.HistoryFetchResult) *HistoryResult {
	if res == nil {
		return &HistoryResult{Items: []tgsource.HistoryPreviewItem{}}
	}
	return &HistoryResult{
		Items:      res.Items,
		Fetched:    res.Fetched,
		Ingested:   res.Ingested,
		MaxMessage: res.MaxMessage,
	}
}