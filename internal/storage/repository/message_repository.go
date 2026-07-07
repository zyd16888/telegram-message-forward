package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainmessage "telegram-message-forward/internal/domain/message"
	"telegram-message-forward/internal/storage/model"
)

// MessageRepository 是 message.Repository 的 PostgreSQL 实现。
type MessageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 创建消息仓储。
func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

var _ domainmessage.Repository = (*MessageRepository)(nil)

// Create 插入消息，按 (source_id, external_message_id) 幂等。
//
// 命中唯一冲突时视为幂等成功，并回填已存在记录的 ID。
func (r *MessageRepository) Create(ctx context.Context, m *domainmessage.NormalizedMessage) error {
	mo, err := toMessageModel(m)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_id"}, {Name: "external_message_id"}},
		DoNothing: true,
	}).Create(mo)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		m.ID = mo.ID
		return nil
	}
	// 幂等：查回已存在的消息 ID。
	var existing model.Message
	if qerr := r.db.WithContext(ctx).
		Select("id").
		Where("source_id = ? AND external_message_id = ?", m.SourceID, m.ExternalMessageID).
		First(&existing).Error; qerr != nil {
		return qerr
	}
	m.ID = existing.ID
	return nil
}

// GetByID 按 id 查询消息。
func (r *MessageRepository) GetByID(ctx context.Context, id int64) (*domainmessage.NormalizedMessage, error) {
	var mo model.Message
	if err := r.db.WithContext(ctx).First(&mo, id).Error; err != nil {
		return nil, err
	}
	return toMessageDomain(&mo)
}

// ListBySourcesAndReceivedAt 查询指定来源在 received_at 窗口内的消息。
func (r *MessageRepository) ListBySourcesAndReceivedAt(ctx context.Context, sourceIDs []int64, start, end time.Time, limit int) ([]*domainmessage.NormalizedMessage, error) {
	if len(sourceIDs) == 0 {
		return []*domainmessage.NormalizedMessage{}, nil
	}
	if limit <= 0 {
		limit = 500
	}
	var ms []model.Message
	if err := r.db.WithContext(ctx).
		Where("source_id IN ? AND received_at >= ? AND received_at < ?", sourceIDs, start, end).
		Order("received_at ASC, id ASC").
		Limit(limit).
		Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainmessage.NormalizedMessage, 0, len(ms))
	for i := range ms {
		msg, err := toMessageDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, nil
}

// ExistsByExternalID 判断消息是否已存在。
func (r *MessageRepository) ExistsByExternalID(ctx context.Context, sourceID, externalMessageID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("source_id = ? AND external_message_id = ?", sourceID, externalMessageID).
		Count(&count).Error
	return count > 0, err
}

func toMessageModel(m *domainmessage.NormalizedMessage) (*model.Message, error) {
	media, err := marshalJSONSlice(m.Media)
	if err != nil {
		return nil, fmt.Errorf("序列化 media 失败: %w", err)
	}
	links, err := marshalJSONSlice(m.Links)
	if err != nil {
		return nil, fmt.Errorf("序列化 links 失败: %w", err)
	}
	var raw datatypes.JSON
	if len(m.RawPayload) > 0 {
		raw = datatypes.JSON(m.RawPayload)
	}
	return &model.Message{
		ID:                m.ID,
		SourceID:          m.SourceID,
		ExternalMessageID: m.ExternalMessageID,
		GroupedID:         m.GroupedID,
		MessageType:       m.MessageType,
		SenderPeerType:    m.SenderPeerType,
		SenderID:          m.SenderID,
		SenderName:        m.SenderName,
		Text:              m.Text,
		Media:             media,
		Links:             links,
		OriginalURL:       m.OriginalURL,
		RawPayload:        raw,
		SentAt:            m.SentAt,
		ReceivedAt:        m.ReceivedAt,
		CreatedAt:         m.CreatedAt,
	}, nil
}

func toMessageDomain(m *model.Message) (*domainmessage.NormalizedMessage, error) {
	var media []domainmessage.Media
	if len(m.Media) > 0 {
		if err := json.Unmarshal(m.Media, &media); err != nil {
			return nil, fmt.Errorf("解析 media 失败: %w", err)
		}
	}
	var links []domainmessage.Link
	if len(m.Links) > 0 {
		if err := json.Unmarshal(m.Links, &links); err != nil {
			return nil, fmt.Errorf("解析 links 失败: %w", err)
		}
	}
	return &domainmessage.NormalizedMessage{
		ID:                m.ID,
		SourceID:          m.SourceID,
		ExternalMessageID: m.ExternalMessageID,
		GroupedID:         m.GroupedID,
		MessageType:       m.MessageType,
		SenderPeerType:    m.SenderPeerType,
		SenderID:          m.SenderID,
		SenderName:        m.SenderName,
		Text:              m.Text,
		Media:             media,
		Links:             links,
		OriginalURL:       m.OriginalURL,
		RawPayload:        []byte(m.RawPayload),
		SentAt:            m.SentAt,
		ReceivedAt:        m.ReceivedAt,
		CreatedAt:         m.CreatedAt,
	}, nil
}

// marshalJSONSlice 序列化切片为 datatypes.JSON，nil/空返回 "[]"。
func marshalJSONSlice(v any) (datatypes.JSON, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return datatypes.JSON([]byte("[]")), nil
	}
	return datatypes.JSON(b), nil
}
