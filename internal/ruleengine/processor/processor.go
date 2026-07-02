// Package processor 定义规则处理器接口与注册表。
package processor

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
)

// Processor 对消息做轻量处理，可原地修改消息。
type Processor interface {
	// Process 处理消息，如追加来源、截断文本、保留链接等。
	Process(ctx context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error
}

// Descriptor describes a processor type for rule editor forms.
type Descriptor struct {
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	Fields      []formschema.FieldSpec `json:"fields"`
}

// ConfigValidator is implemented by processors with custom config validation.
type ConfigValidator interface {
	ValidateConfig(config map[string]any) error
}

var (
	mu          sync.RWMutex
	registry    = map[string]Processor{}
	descriptors = map[string]Descriptor{}
)

// Register 注册一个处理器类型，如 append_source、truncate_text 等。
func Register(name string, p Processor) {
	RegisterWithDescriptor(name, p, Descriptor{Type: name, Label: name})
}

// RegisterWithDescriptor 注册处理器类型与后台表单元数据。
func RegisterWithDescriptor(name string, p Processor, desc Descriptor) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("processor 重复注册: %s", name))
	}
	registry[name] = p
	if desc.Type == "" {
		desc.Type = name
	}
	if desc.Label == "" {
		desc.Label = name
	}
	descriptors[name] = desc
}

// Get 返回指定处理器实现。
func Get(name string) (Processor, error) {
	mu.RLock()
	p, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未注册的 processor: %s", name)
	}
	return p, nil
}

// Descriptors 返回全部处理器元数据，按类型排序。
func Descriptors() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(descriptors))
	for name := range descriptors {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Descriptor, 0, len(names))
	for _, name := range names {
		out = append(out, descriptors[name])
	}
	return out
}

// ValidateConfig validates a processor config using descriptor and custom checks.
func ValidateConfig(name string, config map[string]any) error {
	p, err := Get(name)
	if err != nil {
		return err
	}
	mu.RLock()
	desc := descriptors[name]
	mu.RUnlock()
	if err := formschema.Validate(desc.Fields, config); err != nil {
		return err
	}
	if validator, ok := p.(ConfigValidator); ok {
		return validator.ValidateConfig(config)
	}
	return nil
}
