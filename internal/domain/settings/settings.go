// Package settings 定义页面可管理的系统设置领域模型与仓储接口。
//
// 每组设置一行：Value 存非敏感字段的 JSON 对象，Secret 存敏感字段的
// JSON（领域层持有明文，存储层负责加解密）。设置组优先级高于配置文件，
// 数据库无记录时回退到配置文件默认值。
package settings

import (
	"context"
	"time"
)

// KeyMedia 是媒体存储设置组。
const KeyMedia = "media"

// Setting 是一组系统设置。
type Setting struct {
	Key       string
	Value     map[string]any
	Secret    []byte
	UpdatedAt time.Time
}

// Repository 是系统设置仓储接口。
type Repository interface {
	// Get 按 key 查询设置；不存在时返回 (nil, nil)。
	Get(ctx context.Context, key string) (*Setting, error)
	Upsert(ctx context.Context, s *Setting) error
}
