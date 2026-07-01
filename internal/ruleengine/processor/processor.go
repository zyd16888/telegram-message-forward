// Package processor 定义规则处理器接口与注册表。
package processor

import (
	"context"
	"fmt"
	"sync"

	domainmessage "github.com/zyd16888/telegram-message-forward/internal/domain/message"
)

// Processor 对消息做轻量处理，可原地修改消息。
type Processor interface {
	// Process 处理消息，如追加来源、截断文本、保留链接等。
	Process(ctx context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error
}

var (
	mu       sync.RWMutex
	registry = map[string]Processor{}
)

// Register 注册一个处理器类型，如 append_source、truncate_text 等。
func Register(name string, p Processor) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("processor 重复注册: %s", name))
	}
	registry[name] = p
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
