// Package condition 定义规则条件接口与注册表。
package condition

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
)

// Condition 判断一条消息是否满足某个条件。
type Condition interface {
	// Evaluate 返回消息是否命中该条件。
	Evaluate(ctx context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error)
}

// Descriptor describes a condition type for rule editor forms.
type Descriptor struct {
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	Fields      []formschema.FieldSpec `json:"fields"`
}

// ConfigValidator is implemented by conditions with custom config validation.
type ConfigValidator interface {
	ValidateConfig(config map[string]any) error
}

var (
	mu          sync.RWMutex
	registry    = map[string]Condition{}
	descriptors = map[string]Descriptor{}
)

// Register 注册一个条件类型，如 keyword_contains、regex、message_type 等。
func Register(name string, c Condition) {
	RegisterWithDescriptor(name, c, Descriptor{Type: name, Label: name})
}

// RegisterWithDescriptor 注册条件类型与后台表单元数据。
func RegisterWithDescriptor(name string, c Condition, desc Descriptor) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("condition 重复注册: %s", name))
	}
	registry[name] = c
	if desc.Type == "" {
		desc.Type = name
	}
	if desc.Label == "" {
		desc.Label = name
	}
	descriptors[name] = desc
}

// Get 返回指定条件实现。
func Get(name string) (Condition, error) {
	mu.RLock()
	c, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未注册的 condition: %s", name)
	}
	return c, nil
}

// Descriptors 返回全部条件元数据，按类型排序。
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

// ValidateConfig validates a condition config using descriptor and custom checks.
func ValidateConfig(name string, config map[string]any) error {
	c, err := Get(name)
	if err != nil {
		return err
	}
	mu.RLock()
	desc := descriptors[name]
	mu.RUnlock()
	if err := formschema.Validate(desc.Fields, config); err != nil {
		return err
	}
	if validator, ok := c.(ConfigValidator); ok {
		return validator.ValidateConfig(config)
	}
	return nil
}
