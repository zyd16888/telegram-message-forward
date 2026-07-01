// Package source 定义 Source 插件接口与编译期注册表。
package source

import (
	"context"
	"fmt"
	"sort"
	"sync"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
)

// Handler 接收标准化后的消息。Source 插件负责把原始消息转成 NormalizedMessage 再回调。
type Handler func(ctx context.Context, msg *domainmessage.NormalizedMessage) error

// Capabilities 声明 Source 插件能力。
type Capabilities struct {
	SupportsSync    bool
	SupportsMedia   bool
	SupportsHistory bool
}

// SyncedPeer 是同步到的一个 chat/channel/user。
type SyncedPeer struct {
	PeerType domainsource.PeerType
	PeerID   int64
	Name     string
	Username string
}

// Plugin 是消息来源插件。gotd/td 等原始依赖只允许出现在其实现内部。
type Plugin interface {
	Name() string
	ValidateConfig(config map[string]any) error
	Capabilities() Capabilities
	Start(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, handler Handler) error
	Stop(ctx context.Context, src *domainsource.Source) error
	SyncSources(ctx context.Context, acc *domainaccount.Account) ([]SyncedPeer, error)
}

// Factory 创建 Source 插件实例。
type Factory func() (Plugin, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register 注册一个 Source 类型。
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("source 插件重复注册: %s", name))
	}
	registry[name] = f
}

// New 按类型创建 Source 插件实例。
func New(name string) (Plugin, error) {
	mu.RLock()
	f, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未注册的 source 插件: %s", name)
	}
	return f()
}

// Names 返回已注册的 Source 类型，按字母排序。
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
