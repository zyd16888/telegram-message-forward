package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainpeer "telegram-message-forward/internal/domain/peer"
	"telegram-message-forward/internal/storage/model"
)

// TelegramPeerRepository 是 peer.Repository 的 PostgreSQL 实现。
type TelegramPeerRepository struct {
	db *gorm.DB
}

// NewTelegramPeerRepository 创建 peer 缓存仓储。
func NewTelegramPeerRepository(db *gorm.DB) *TelegramPeerRepository {
	return &TelegramPeerRepository{db: db}
}

var _ domainpeer.Repository = (*TelegramPeerRepository)(nil)

// Upsert 按 (account_id, peer_type, peer_id) 插入或更新 access_hash 等字段。
func (r *TelegramPeerRepository) Upsert(ctx context.Context, p *domainpeer.Peer) error {
	m := &model.TelegramPeer{
		AccountID:  p.AccountID,
		PeerType:   string(p.PeerType),
		PeerID:     p.PeerID,
		AccessHash: p.AccessHash,
		Username:   p.Username,
		Title:      p.Title,
		UpdatedAt:  time.Now(),
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account_id"}, {Name: "peer_type"}, {Name: "peer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"access_hash", "username", "title", "updated_at"}),
	}).Create(m).Error
	if err != nil {
		return err
	}
	p.ID = m.ID
	return nil
}

// Get 按唯一键查询单个 peer；不存在返回 (nil, nil)。
func (r *TelegramPeerRepository) Get(ctx context.Context, accountID int64, peerType domainpeer.Type, peerID int64) (*domainpeer.Peer, error) {
	var m model.TelegramPeer
	err := r.db.WithContext(ctx).
		Where("account_id = ? AND peer_type = ? AND peer_id = ?", accountID, string(peerType), peerID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toPeerDomain(&m), nil
}

// ListByAccount 返回某账号下全部 peer 缓存。
func (r *TelegramPeerRepository) ListByAccount(ctx context.Context, accountID int64) ([]*domainpeer.Peer, error) {
	var ms []model.TelegramPeer
	if err := r.db.WithContext(ctx).Where("account_id = ?", accountID).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainpeer.Peer, 0, len(ms))
	for i := range ms {
		out = append(out, toPeerDomain(&ms[i]))
	}
	return out, nil
}

func toPeerDomain(m *model.TelegramPeer) *domainpeer.Peer {
	return &domainpeer.Peer{
		ID:         m.ID,
		AccountID:  m.AccountID,
		PeerType:   domainpeer.Type(m.PeerType),
		PeerID:     m.PeerID,
		AccessHash: m.AccessHash,
		Username:   m.Username,
		Title:      m.Title,
		UpdatedAt:  m.UpdatedAt,
	}
}
