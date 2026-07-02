// Package peer 定义 Telegram peer 缓存的领域模型与仓储接口。
//
// raw gotd/td 解析 peer、拉历史、下载媒体、reply 都依赖持久化的 access_hash，
// 缺失会导致 peer 无法解析。
package peer

import (
	"context"
	"time"
)

// Type 是 peer 类型。
type Type string

const (
	TypeUser    Type = "user"
	TypeChat    Type = "chat"
	TypeChannel Type = "channel"
)

// Peer 是一个缓存的 Telegram peer，保存 access_hash。
type Peer struct {
	ID         int64
	AccountID  int64
	PeerType   Type
	PeerID     int64
	AccessHash int64
	Username   string
	Title      string
	UpdatedAt  time.Time
}

// Repository 是 peer 缓存仓储接口。
type Repository interface {
	// Upsert 按 (account_id, peer_type, peer_id) 插入或更新。
	Upsert(ctx context.Context, p *Peer) error
	// BulkUpsert 批量插入或更新 peer 缓存。
	BulkUpsert(ctx context.Context, peers []*Peer) error
	// Get 按唯一键查询单个 peer；不存在返回 (nil, nil)。
	Get(ctx context.Context, accountID int64, peerType Type, peerID int64) (*Peer, error)
	// ListByAccount 返回某账号下全部 peer 缓存。
	ListByAccount(ctx context.Context, accountID int64) ([]*Peer, error)
}
