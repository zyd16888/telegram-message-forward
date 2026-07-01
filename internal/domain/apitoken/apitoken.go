// Package apitoken 定义管理 API token 的领域模型与仓储接口。
//
// 数据库只存 token 哈希，不存明文。
package apitoken

import (
	"context"
	"time"
)

// Token 是一个管理 API token 记录（不含明文）。
type Token struct {
	ID         int64
	Name       string
	TokenHash  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// Repository 是 API token 仓储接口。
type Repository interface {
	Create(ctx context.Context, name, hash string) (int64, error)
	List(ctx context.Context) ([]*Token, error)
	Revoke(ctx context.Context, id int64) error
	ExistsActiveHash(ctx context.Context, hash string) (bool, error)
}
