// Package condition 定义规则条件接口与注册表。
package condition

import (
	"context"
	"fmt"
	"sync"

	domainmessage "github.com/zyd16888/telegram-message-forward/internal/domain/message"
)

// Condition 判断一条消息是否满足某个条件。
type Condition interface {
	// Evaluate 返回消息是否命中该条件。
	Evaluate(ctx context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Condition{}
)

// Register 注册一个条件类型，如 keyword_contains、regex、message_type 等。
func Register(name string, c Condition) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("condition 重复注册: %s", name))
	}
	registry[name] = c
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
