// Package sink 定义 Sink 插件接口与编译期注册表。
package sink

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
)

// Payload 是渲染后待投递的内容。
type Payload struct {
	Format       string // text | markdown | html
	Text         string
	Media        []domainmessage.Media
	FallbackText string
}

// Options 是投递可选项。
type Options struct {
	DeliveryKey string
	StepPrefix  string
	Completed   map[string]bool
	Checkpoint  func(context.Context, string) error
}

func (o Options) StepKey(step string) string {
	if o.StepPrefix == "" {
		return step
	}
	return o.StepPrefix + ":" + step
}

func (o Options) IsCompleted(step string) bool {
	return o.Completed[o.StepKey(step)]
}

func (o Options) MarkCompleted(ctx context.Context, step string) error {
	if o.Checkpoint == nil {
		return nil
	}
	return o.Checkpoint(ctx, o.StepKey(step))
}

type FailureKind string

const (
	FailureUnknown   FailureKind = ""
	FailureTransient FailureKind = "transient"
	FailurePermanent FailureKind = "permanent"
)

// Result 是一次投递的标准结果。
type Result struct {
	Success         bool
	ResponseSummary []byte
	Error           string
	FailureKind     FailureKind
	RetryAfter      time.Duration
}

// Plugin 是目标渠道插件。它只做渠道适配，不查数据库、不判断规则、不做重试调度。
type Plugin interface {
	Name() string
	ValidateConfig(config map[string]any) error
	Capabilities() domainsink.Capabilities
	Send(ctx context.Context, s *domainsink.Sink, payload Payload, opts Options) (*Result, error)
}

// SinkValidator 用于校验同时依赖 config 与 secret 的渠道。
type SinkValidator interface {
	ValidateSink(*domainsink.Sink) error
}

func Validate(plugin Plugin, s *domainsink.Sink) error {
	if err := plugin.ValidateConfig(s.Config); err != nil {
		return err
	}
	if validator, ok := plugin.(SinkValidator); ok {
		return validator.ValidateSink(s)
	}
	if describer, ok := plugin.(Describer); ok {
		field := describer.Descriptor().SecretField
		if field != nil && field.Required && len(s.Secret) == 0 {
			return fmt.Errorf("%s 缺少 %s", plugin.Name(), field.Label)
		}
	}
	return nil
}

// Descriptor describes a registered Sink type for admin UI forms.
type Descriptor struct {
	Type         string                  `json:"type"`
	Label        string                  `json:"label"`
	Description  string                  `json:"description,omitempty"`
	ConfigFields []formschema.FieldSpec  `json:"config_fields"`
	SecretField  *formschema.FieldSpec   `json:"secret_field,omitempty"`
	Capabilities domainsink.Capabilities `json:"capabilities"`
}

// Describer is implemented by Sink plugins that expose UI metadata.
type Describer interface {
	Descriptor() Descriptor
}

// Factory 根据配置创建 Plugin 实例。
type Factory func() (Plugin, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register 注册一个 Sink 类型。重复注册会 panic，用于编译期发现错误。
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("sink 插件重复注册: %s", name))
	}
	registry[name] = f
}

// New 按类型创建 Plugin 实例。
func New(name string) (Plugin, error) {
	mu.RLock()
	f, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未注册的 sink 插件: %s", name)
	}
	return f()
}

// Names 返回已注册的 Sink 类型，按字母排序。
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Descriptors returns registered Sink descriptors sorted by type.
func Descriptors() []Descriptor {
	names := Names()
	out := make([]Descriptor, 0, len(names))
	for _, name := range names {
		plugin, err := New(name)
		if err != nil {
			continue
		}
		if d, ok := plugin.(Describer); ok {
			desc := d.Descriptor()
			if desc.Type == "" {
				desc.Type = name
			}
			if desc.Label == "" {
				desc.Label = name
			}
			out = append(out, desc)
			continue
		}
		out = append(out, Descriptor{
			Type:         name,
			Label:        name,
			ConfigFields: nil,
			Capabilities: plugin.Capabilities(),
		})
	}
	return out
}
