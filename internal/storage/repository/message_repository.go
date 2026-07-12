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

type messageListRow struct {
	model.Message
	SourceName     string
	SourceType     string
	SourceUsername string
	DeliveryTotal  int64
	Pending        int64
	Processing     int64
	Success        int64
	Failed         int64
	Retrying       int64
	Dead           int64
	Cancelled      int64
}

const messageListSelect = `
	m.*,
	s.name AS source_name,
	s.source_type AS source_type,
	s.username AS source_username,
	COALESCE(ds.delivery_total, 0) AS delivery_total,
	COALESCE(ds.pending, 0) AS pending,
	COALESCE(ds.processing, 0) AS processing,
	COALESCE(ds.success, 0) AS success,
	COALESCE(ds.failed, 0) AS failed,
	COALESCE(ds.retrying, 0) AS retrying,
	COALESCE(ds.dead, 0) AS dead,
	COALESCE(ds.cancelled, 0) AS cancelled`

const deliverySummaryJoin = `LEFT JOIN (
	SELECT message_id,
		COUNT(*) AS delivery_total,
		COUNT(*) FILTER (WHERE status = 'pending') AS pending,
		COUNT(*) FILTER (WHERE status = 'processing') AS processing,
		COUNT(*) FILTER (WHERE status = 'success') AS success,
		COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		COUNT(*) FILTER (WHERE status = 'retrying') AS retrying,
		COUNT(*) FILTER (WHERE status = 'dead') AS dead,
		COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
	FROM delivery_tasks
	WHERE message_id IS NOT NULL
	GROUP BY message_id
) ds ON ds.message_id = m.id`

// List 为消息中心按 ID 倒序查询消息，并聚合关联投递状态。
func (r *MessageRepository) List(ctx context.Context, q domainmessage.Query) ([]domainmessage.ListItem, bool, error) {
	db := r.messageListQuery(ctx)
	if q.SourceID > 0 {
		db = db.Where("m.source_id = ?", q.SourceID)
	}
	if q.SourceType != "" {
		db = db.Where("s.source_type = ?", q.SourceType)
	}
	if q.MessageType != "" {
		db = db.Where("m.message_type = ?", q.MessageType)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		db = db.Where("m.text ILIKE ? OR m.sender_name ILIKE ?", like, like)
	}
	if q.HasMedia != nil {
		if *q.HasMedia {
			db = db.Where("jsonb_array_length(m.media) > 0")
		} else {
			db = db.Where("jsonb_array_length(m.media) = 0")
		}
	}
	if q.DeliveryStatus != "" {
		if q.DeliveryStatus == "none" {
			db = db.Where("NOT EXISTS (SELECT 1 FROM delivery_tasks dt_filter WHERE dt_filter.message_id = m.id)")
		} else {
			db = db.Where("EXISTS (SELECT 1 FROM delivery_tasks dt_filter WHERE dt_filter.message_id = m.id AND dt_filter.status = ?)", q.DeliveryStatus)
		}
	}
	if q.From != nil {
		db = db.Where("m.received_at >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("m.received_at < ?", *q.To)
	}
	if q.BeforeID > 0 {
		db = db.Where("m.id < ?", q.BeforeID)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 30
	}
	var rows []messageListRow
	if err := db.Order("m.id DESC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items, err := messageRowsToDomain(rows)
	return items, hasMore, err
}

// GetDetail 返回消息详情和精简投递引用，不暴露 raw_payload。
func (r *MessageRepository) GetDetail(ctx context.Context, id int64) (*domainmessage.Detail, error) {
	var row messageListRow
	if err := r.messageListQuery(ctx).Where("m.id = ?", id).Limit(1).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	items, err := messageRowsToDomain([]messageListRow{row})
	if err != nil {
		return nil, err
	}

	type deliveryRow struct {
		ID           int64
		SinkID       int64
		SinkName     string
		SinkType     string
		OriginType   string
		OriginID     int64
		OriginNodeID int64
		FlowName     string
		Status       string
		AttemptCount int
		MaxAttempts  int
		LastError    string
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}
	var deliveryRows []deliveryRow
	if err := r.db.WithContext(ctx).Table("delivery_tasks dt").
		Select(`dt.id, dt.sink_id, sk.name AS sink_name, sk.type AS sink_type,
			dt.origin_type, COALESCE(dt.origin_id, 0) AS origin_id,
			COALESCE(dt.origin_node_id, 0) AS origin_node_id,
			COALESCE(f.name, '') AS flow_name, dt.status, dt.attempt_count,
			dt.max_attempts, dt.last_error, dt.created_at, dt.updated_at`).
		Joins("JOIN sinks sk ON sk.id = dt.sink_id").
		Joins("LEFT JOIN flows f ON dt.origin_type = 'flow' AND f.id = dt.origin_id").
		Where("dt.message_id = ?", id).
		Order("dt.id ASC").
		Scan(&deliveryRows).Error; err != nil {
		return nil, err
	}
	deliveries := make([]domainmessage.DeliveryRef, 0, len(deliveryRows))
	for _, item := range deliveryRows {
		deliveries = append(deliveries, domainmessage.DeliveryRef{
			ID: item.ID, SinkID: item.SinkID, SinkName: item.SinkName, SinkType: item.SinkType,
			OriginType: item.OriginType, OriginID: item.OriginID, OriginNodeID: item.OriginNodeID,
			FlowName: item.FlowName, Status: item.Status, AttemptCount: item.AttemptCount,
			MaxAttempts: item.MaxAttempts, LastError: item.LastError,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	return &domainmessage.Detail{Item: &items[0], Deliveries: deliveries}, nil
}

func (r *MessageRepository) messageListQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("messages m").
		Select(messageListSelect).
		Joins("JOIN sources s ON s.id = m.source_id").
		Joins(deliverySummaryJoin)
}

func messageRowsToDomain(rows []messageListRow) ([]domainmessage.ListItem, error) {
	items := make([]domainmessage.ListItem, 0, len(rows))
	for i := range rows {
		msg, err := toMessageDomain(&rows[i].Message)
		if err != nil {
			return nil, err
		}
		msg.RawPayload = nil
		items = append(items, domainmessage.ListItem{
			Message: msg, SourceName: rows[i].SourceName, SourceType: rows[i].SourceType,
			SourceUsername: rows[i].SourceUsername,
			Deliveries: domainmessage.DeliveryCounts{
				Total: rows[i].DeliveryTotal, Pending: rows[i].Pending, Processing: rows[i].Processing,
				Success: rows[i].Success, Failed: rows[i].Failed, Retrying: rows[i].Retrying,
				Dead: rows[i].Dead, Cancelled: rows[i].Cancelled,
			},
		})
	}
	return items, nil
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
