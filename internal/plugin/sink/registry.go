// Package sink 定义 Sink 插件接口与编译期注册表。
package sink

import (
	"context"
	"fmt"
	"sort"
	"sync"

	domainsink "github.com/zyd16888/telegram-message-forward/internal/domain/sink"
)

// Payload 是渲染后待投递的内容。
type Payload struct {
	Format string // text | markdown | html
	Text   string
}

// Options 是投递可选项。
type Options struct {
	// 预留：超时、重试提示等。
}

// Result 是一次投递的标准结果。
type Result struct {
	Success         bool
	ResponseSummary []byte
	Error           string
}

// Plugin 是目标渠道插件。它只做渠道适配，不查数据库、不判断规则、不做重试调度。
type Plugin interface {
	Name() string
	ValidateConfig(config map[string]any) error
	Capabilities() domainsink.Capabilities
	Send(ctx context.Context, s *domainsink.Sink, payload Payload, opts Options) (*Result, error)
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
