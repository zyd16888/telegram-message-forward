// Package source 定义监听源领域模型与仓储接口。
package source

import (
	"context"
	"time"
)

// PeerType 是 Telegram peer 类型。
type PeerType string

const (
	PeerUser    PeerType = "user"
	PeerChat    PeerType = "chat"
	PeerChannel PeerType = "channel"
)

// Source 是一个监听源，指向某账号下的一个 peer。
type Source struct {
	ID            int64
	AccountID     int64
	PeerType      PeerType
	PeerID        int64
	Name          string
	Username      string
	Enabled       bool
	Config        map[string]any
	LastMessageID int64
	LastSyncedAt  *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repository 是监听源仓储接口。
type Repository interface {
	Create(ctx context.Context, s *Source) error
	Update(ctx context.Context, s *Source) error
	GetByID(ctx context.Context, id int64) (*Source, error)
	List(ctx context.Context) ([]*Source, error)
	ListByAccount(ctx context.Context, accountID int64) ([]*Source, error)
	ListEnabled(ctx context.Context) ([]*Source, error)
	Delete(ctx context.Context, id int64) error
}
