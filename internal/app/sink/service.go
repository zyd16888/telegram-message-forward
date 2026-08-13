// Package sink 提供目标渠道管理应用服务。
package sink

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

// Service 是渠道应用服务。
type Service struct {
	repo          domainsink.Repository
	deliveryStats DeliveryStatsReader
}

// NewService 创建渠道服务。
func NewService(repo domainsink.Repository, statsReader ...DeliveryStatsReader) *Service {
	var reader DeliveryStatsReader
	if len(statsReader) > 0 {
		reader = statsReader[0]
	}
	return &Service{repo: repo, deliveryStats: reader}
}

// DeliveryStatsReader 提供 Sink 维度投递统计。
type DeliveryStatsReader interface {
	SinkStatsSince(ctx context.Context, since time.Time) (map[int64]domainsink.DeliveryStats, error)
}

// List 返回全部渠道。
func (s *Service) List(ctx context.Context) ([]*domainsink.Sink, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return items, s.attachObservability(ctx, items)
}

// Get 查询单个渠道。
func (s *Service) Get(ctx context.Context, id int64) (*domainsink.Sink, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return item, s.attachObservability(ctx, []*domainsink.Sink{item})
}

// CreateInput 是创建渠道的输入。Secret 为明文，存储层加密。
type CreateInput struct {
	Type    string
	Name    string
	Enabled bool
	Config  map[string]any
	Secret  string
}

// Create 创建渠道，校验类型已注册且配置合法，并按插件声明补齐能力。
func (s *Service) Create(ctx context.Context, in CreateInput) (*domainsink.Sink, error) {
	plugin, err := pluginsink.New(in.Type)
	if err != nil {
		return nil, err
	}
	sk := &domainsink.Sink{
		Type: in.Type, Name: in.Name, Enabled: in.Enabled, Config: in.Config, Secret: []byte(in.Secret),
		Capabilities: plugin.Capabilities(),
	}
	if err := pluginsink.Validate(plugin, sk); err != nil {
		return nil, fmt.Errorf("渠道配置校验失败: %w", err)
	}
	if err := s.repo.Create(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

// UpdateInput 是更新渠道的输入。Secret 为 nil 时保留原值。
type UpdateInput struct {
	Name    *string
	Enabled *bool
	Config  map[string]any
	Secret  *string
}

// Update 更新渠道可变字段。
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (*domainsink.Sink, error) {
	sk, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		sk.Name = *in.Name
	}
	if in.Enabled != nil {
		sk.Enabled = *in.Enabled
	}
	if in.Config != nil {
		sk.Config = in.Config
	}
	if in.Secret != nil {
		sk.Secret = []byte(*in.Secret)
	}
	plugin, err := pluginsink.New(sk.Type)
	if err != nil {
		return nil, err
	}
	sk.Capabilities = plugin.Capabilities()
	if err := pluginsink.Validate(plugin, sk); err != nil {
		return nil, fmt.Errorf("渠道配置校验失败: %w", err)
	}
	if err := s.repo.Update(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

// Delete 删除渠道。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// TestInput 是渠道连通性测试输入。
type TestInput struct {
	ID     int64
	Type   string
	Config map[string]any
	Secret *string
	Media  *TestMediaInput
}

type TestMediaInput struct {
	Type     string
	URL      string
	FileName string
}

// Test 用当前配置向渠道发送一条测试消息。
func (s *Service) Test(ctx context.Context, in TestInput) (*pluginsink.Result, error) {
	var sk *domainsink.Sink
	if in.ID > 0 {
		existing, err := s.repo.GetByID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		sk = existing
		if in.Config != nil {
			sk.Config = in.Config
		}
		if in.Secret != nil {
			sk.Secret = []byte(*in.Secret)
		}
	} else {
		secret := ""
		if in.Secret != nil {
			secret = *in.Secret
		}
		sk = &domainsink.Sink{
			Type:    in.Type,
			Name:    "test",
			Enabled: true,
			Config:  in.Config,
			Secret:  []byte(secret),
		}
	}
	plugin, err := pluginsink.New(sk.Type)
	if err != nil {
		return nil, err
	}
	if err := pluginsink.Validate(plugin, sk); err != nil {
		if in.ID > 0 {
			_ = s.repo.UpdateTestResult(ctx, in.ID, time.Now(), false, err.Error())
		}
		return nil, fmt.Errorf("渠道配置校验失败: %w", err)
	}
	payload := pluginsink.Payload{
		Format: "text",
		Text:   "这是一条来自 Telegram Message Forward 的渠道测试消息。",
	}
	if in.Media != nil {
		media, err := testMedia(*in.Media)
		if err != nil {
			return nil, fmt.Errorf("测试媒体无效: %w", err)
		}
		payload.Media = []domainmessage.Media{media}
		payload.FallbackText = payload.Text + "\n[测试媒体] " + media.RemoteURL
	}
	result, sendErr := plugin.Send(ctx, sk, payload, pluginsink.Options{})
	if in.ID > 0 {
		success := result != nil && result.Success && sendErr == nil
		errText := ""
		if sendErr != nil {
			errText = sendErr.Error()
		} else if result != nil {
			errText = result.Error
		} else {
			errText = "渠道未返回测试结果"
		}
		_ = s.repo.UpdateTestResult(ctx, in.ID, time.Now(), success, errText)
	}
	return result, sendErr
}

func testMedia(in TestMediaInput) (domainmessage.Media, error) {
	kind := strings.ToLower(strings.TrimSpace(in.Type))
	switch kind {
	case "image", "file", "audio", "video":
	default:
		return domainmessage.Media{}, fmt.Errorf("不支持的类型 %q", in.Type)
	}
	u, err := url.Parse(strings.TrimSpace(in.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return domainmessage.Media{}, fmt.Errorf("URL 必须是无用户凭据的 http/https 公网地址")
	}
	return domainmessage.Media{Type: kind, RemoteURL: u.String(), FileName: strings.TrimSpace(in.FileName)}, nil
}

// Types 返回已注册的 Sink 类型。
func (s *Service) Types() []string {
	return pluginsink.Names()
}

// Descriptors 返回已注册 Sink 的后台配置元数据。
func (s *Service) Descriptors() []pluginsink.Descriptor {
	return pluginsink.Descriptors()
}

func (s *Service) attachObservability(ctx context.Context, items []*domainsink.Sink) error {
	if s.deliveryStats == nil || len(items) == 0 {
		return nil
	}
	stats, err := s.deliveryStats.SinkStatsSince(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return err
	}
	for _, item := range items {
		stat := stats[item.ID]
		item.Observability.DeliveryTotal24h = stat.Total
		item.Observability.DeliverySuccess24h = stat.Success
		item.Observability.RecentFailure = stat.LastFailure
		if stat.Total > 0 {
			item.Observability.SuccessRate24h = float64(stat.Success) / float64(stat.Total)
		}
	}
	return nil
}
