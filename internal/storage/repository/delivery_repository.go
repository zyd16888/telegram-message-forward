package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/storage/model"
)

// DeliveryRepository 是 delivery.Repository 的 PostgreSQL 实现。
type DeliveryRepository struct {
	db *gorm.DB
}

// NewDeliveryRepository 创建投递任务仓储。
func NewDeliveryRepository(db *gorm.DB) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

var _ domaindelivery.Repository = (*DeliveryRepository)(nil)

// Create 插入投递任务，按来源类型的唯一键幂等。
func (r *DeliveryRepository) Create(ctx context.Context, t *domaindelivery.Task) error {
	if t.OriginType == "" {
		t.OriginType = "flow"
	}
	m, err := toDeliveryModel(t)
	if err != nil {
		return err
	}
	switch t.OriginType {
	case "flow":
		res := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "message_id"}, {Name: "origin_type"}, {Name: "origin_id"}, {Name: "origin_node_id"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				clause.Expr{SQL: "origin_type = 'flow'"},
			}},
			DoNothing: true,
		}).Create(m)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			t.ID = m.ID
			return nil
		}
		var existing model.DeliveryTask
		if qerr := r.db.WithContext(ctx).
			Select("id").
			Where("message_id = ? AND origin_type = ? AND origin_id = ? AND origin_node_id = ?", t.MessageID, "flow", t.OriginID, t.OriginNodeID).
			First(&existing).Error; qerr != nil {
			return qerr
		}
		t.ID = existing.ID
		return nil
	default:
		if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
			return err
		}
		t.ID = m.ID
		return nil
	}
}

// Claim 使用 FOR UPDATE SKIP LOCKED 领取一批可执行任务，置为 processing。
//
// 领取条件：status='pending'，或 status='retrying' 且 next_retry_at<=now。
func (r *DeliveryRepository) Claim(ctx context.Context, workerID string, limit int) ([]*domaindelivery.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	var ms []model.DeliveryTask
	const q = `
UPDATE delivery_tasks
SET status = 'processing', locked_at = now(), locked_by = ?, updated_at = now()
WHERE id IN (
    SELECT id FROM delivery_tasks
    WHERE status = 'pending'
       OR (status = 'retrying' AND (next_retry_at IS NULL OR next_retry_at <= now()))
    ORDER BY created_at
    LIMIT ?
    FOR UPDATE SKIP LOCKED
)
RETURNING *`
	if err := r.db.WithContext(ctx).Raw(q, workerID, limit).Scan(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domaindelivery.Task, 0, len(ms))
	for i := range ms {
		task, err := toDeliveryDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}

// UpdateStatus 更新任务状态与重试信息。
func (r *DeliveryRepository) UpdateStatus(ctx context.Context, t *domaindelivery.Task) error {
	updates := map[string]any{
		"status":        string(t.Status),
		"attempt_count": t.AttemptCount,
		"next_retry_at": t.NextRetryAt,
		"last_error":    t.LastError,
		"updated_at":    time.Now(),
	}
	// 终态清理领取锁。
	switch t.Status {
	case domaindelivery.StatusSuccess, domaindelivery.StatusDead, domaindelivery.StatusCancelled, domaindelivery.StatusRetrying, domaindelivery.StatusFailed:
		updates["locked_at"] = nil
		updates["locked_by"] = ""
	}
	return r.db.WithContext(ctx).
		Model(&model.DeliveryTask{}).
		Where("id = ?", t.ID).
		Updates(updates).Error
}

// RecoverStale 将超过可见性超时仍处于 processing 的任务回退到 retrying。
func (r *DeliveryRepository) RecoverStale(ctx context.Context, olderThan time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&model.DeliveryTask{}).
		Where("status = ? AND locked_at IS NOT NULL AND locked_at < ?", string(domaindelivery.StatusProcessing), olderThan).
		Updates(map[string]any{
			"status":     string(domaindelivery.StatusRetrying),
			"locked_at":  nil,
			"locked_by":  "",
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

// Requeue 将一个终态任务重置为 pending，清零重试计数与锁，供手动重试。
func (r *DeliveryRepository) Requeue(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.DeliveryTask{}).
		Where("id = ? AND status IN ?", id, []string{
			string(domaindelivery.StatusDead),
			string(domaindelivery.StatusFailed),
			string(domaindelivery.StatusCancelled),
		}).
		Updates(map[string]any{
			"status":        string(domaindelivery.StatusPending),
			"attempt_count": 0,
			"next_retry_at": nil,
			"locked_at":     nil,
			"locked_by":     "",
			"last_error":    "",
			"updated_at":    time.Now(),
		}).Error
}

// RequeueByStatus 将指定终态的一批任务重置为 pending，返回影响行数。
func (r *DeliveryRepository) RequeueByStatus(ctx context.Context, status domaindelivery.Status) (int64, error) {
	if status == "" {
		status = domaindelivery.StatusDead
	}
	res := r.db.WithContext(ctx).
		Model(&model.DeliveryTask{}).
		Where("status = ?", string(status)).
		Updates(map[string]any{
			"status":        string(domaindelivery.StatusPending),
			"attempt_count": 0,
			"next_retry_at": nil,
			"locked_at":     nil,
			"locked_by":     "",
			"last_error":    "",
			"updated_at":    time.Now(),
		})
	return res.RowsAffected, res.Error
}

// AddAttempt 追加一次投递尝试记录。
func (r *DeliveryRepository) AddAttempt(ctx context.Context, a *domaindelivery.Attempt) error {
	m := &model.DeliveryAttempt{
		DeliveryTaskID:  a.DeliveryTaskID,
		AttemptNo:       a.AttemptNo,
		Status:          string(a.Status),
		RequestSummary:  a.RequestSummary,
		ResponseSummary: a.ResponseSummary,
		Error:           a.Error,
		StartedAt:       a.StartedAt,
		FinishedAt:      a.FinishedAt,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	a.ID = m.ID
	return nil
}

// ListAttempts 查询某投递任务的全部尝试记录。
func (r *DeliveryRepository) ListAttempts(ctx context.Context, taskID int64) ([]*domaindelivery.Attempt, error) {
	var ms []model.DeliveryAttempt
	if err := r.db.WithContext(ctx).
		Where("delivery_task_id = ?", taskID).
		Order("attempt_no ASC, id ASC").
		Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domaindelivery.Attempt, 0, len(ms))
	for i := range ms {
		out = append(out, toAttemptDomain(&ms[i]))
	}
	return out, nil
}

// GetByID 按 id 查询投递任务。
func (r *DeliveryRepository) GetByID(ctx context.Context, id int64) (*domaindelivery.Task, error) {
	var m model.DeliveryTask
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toDeliveryDomain(&m)
}

// List 按状态分页查询投递任务。status 为空时返回非 cancelled 记录。
func (r *DeliveryRepository) List(ctx context.Context, status domaindelivery.Status, limit, offset int) ([]*domaindelivery.Task, error) {
	if limit <= 0 {
		limit = 50
	}
	q := r.deliveryStatusQuery(ctx, status)
	var ms []model.DeliveryTask
	if err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domaindelivery.Task, 0, len(ms))
	for i := range ms {
		task, err := toDeliveryDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}

// Count 统计投递任务数量。status 为空时不统计 cancelled 记录。
func (r *DeliveryRepository) Count(ctx context.Context, status domaindelivery.Status) (int64, error) {
	var count int64
	err := r.deliveryStatusQuery(ctx, status).Count(&count).Error
	return count, err
}

// ListByQuery 按复合条件分页查询投递任务。
func (r *DeliveryRepository) ListByQuery(ctx context.Context, q domaindelivery.Query) ([]*domaindelivery.Task, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	var ms []model.DeliveryTask
	db := r.deliveryQuery(ctx, q)
	if err := db.Order("delivery_tasks.id DESC").Limit(q.Limit).Offset(q.Offset).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domaindelivery.Task, 0, len(ms))
	for i := range ms {
		task, err := toDeliveryDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}

// CountByQuery 按复合条件统计投递任务。
func (r *DeliveryRepository) CountByQuery(ctx context.Context, q domaindelivery.Query) (int64, error) {
	var count int64
	err := r.deliveryQuery(ctx, q).Count(&count).Error
	return count, err
}

// SinkStatsSince 汇总某时间点之后各 Sink 的投递状态。
func (r *DeliveryRepository) SinkStatsSince(ctx context.Context, since time.Time) (map[int64]domainsink.DeliveryStats, error) {
	type row struct {
		SinkID      int64
		Total       int64
		Success     int64
		LastFailure string
	}
	var rows []row
	const q = `
SELECT
    dt.sink_id,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE dt.status = 'success') AS success,
    COALESCE((
        SELECT dt2.last_error
        FROM delivery_tasks dt2
        WHERE dt2.sink_id = dt.sink_id
          AND dt2.status IN ('failed', 'dead')
          AND dt2.last_error <> ''
        ORDER BY dt2.updated_at DESC, dt2.id DESC
        LIMIT 1
    ), '') AS last_failure
FROM delivery_tasks dt
WHERE dt.created_at >= ?
GROUP BY dt.sink_id`
	if err := r.db.WithContext(ctx).Raw(q, since).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]domainsink.DeliveryStats, len(rows))
	for _, row := range rows {
		out[row.SinkID] = domainsink.DeliveryStats{
			Total:       row.Total,
			Success:     row.Success,
			LastFailure: row.LastFailure,
		}
	}
	return out, nil
}

func (r *DeliveryRepository) deliveryStatusQuery(ctx context.Context, status domaindelivery.Status) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&model.DeliveryTask{})
	if status != "" {
		return q.Where("status = ?", string(status))
	}
	return q.Where("status <> ?", string(domaindelivery.StatusCancelled))
}

func (r *DeliveryRepository) deliveryQuery(ctx context.Context, q domaindelivery.Query) *gorm.DB {
	db := r.deliveryStatusQuery(ctx, q.Status)
	if q.SinkID > 0 {
		db = db.Where("delivery_tasks.sink_id = ?", q.SinkID)
	}
	if q.SourceID > 0 {
		db = db.Joins("JOIN messages ON messages.id = delivery_tasks.message_id").
			Where("messages.source_id = ?", q.SourceID)
	}
	if q.Since != nil {
		db = db.Where("delivery_tasks.created_at >= ?", *q.Since)
	}
	return db
}

func toDeliveryModel(t *domaindelivery.Task) (*model.DeliveryTask, error) {
	snapshot, err := marshalMessageSnapshot(t.MessageSnapshot)
	if err != nil {
		return nil, err
	}
	messageID := nullablePositive(t.MessageID)
	ruleID := nullablePositive(t.RuleID)
	originType := t.OriginType
	if originType == "" {
		originType = "flow"
	}
	originID := nullablePositive(t.OriginID)
	originNodeID := nullablePositive(t.OriginNodeID)
	return &model.DeliveryTask{
		ID:              t.ID,
		MessageID:       messageID,
		RuleID:          ruleID,
		SinkID:          t.SinkID,
		TemplateID:      t.TemplateID,
		OriginType:      originType,
		OriginID:        originID,
		OriginNodeID:    originNodeID,
		Status:          string(t.Status),
		AttemptCount:    t.AttemptCount,
		MaxAttempts:     t.MaxAttempts,
		NextRetryAt:     t.NextRetryAt,
		LockedAt:        t.LockedAt,
		LockedBy:        t.LockedBy,
		LastError:       t.LastError,
		MessageSnapshot: snapshot,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}, nil
}

func toDeliveryDomain(m *model.DeliveryTask) (*domaindelivery.Task, error) {
	snapshot, err := unmarshalMessageSnapshot(m.MessageSnapshot)
	if err != nil {
		return nil, err
	}
	messageID := int64(0)
	if m.MessageID != nil {
		messageID = *m.MessageID
	}
	ruleID := int64(0)
	if m.RuleID != nil {
		ruleID = *m.RuleID
	}
	originID := int64(0)
	if m.OriginID != nil {
		originID = *m.OriginID
	}
	originNodeID := int64(0)
	if m.OriginNodeID != nil {
		originNodeID = *m.OriginNodeID
	}
	return &domaindelivery.Task{
		ID:              m.ID,
		MessageID:       messageID,
		RuleID:          ruleID,
		SinkID:          m.SinkID,
		TemplateID:      m.TemplateID,
		OriginType:      m.OriginType,
		OriginID:        originID,
		OriginNodeID:    originNodeID,
		Status:          domaindelivery.Status(m.Status),
		AttemptCount:    m.AttemptCount,
		MaxAttempts:     m.MaxAttempts,
		NextRetryAt:     m.NextRetryAt,
		LockedAt:        m.LockedAt,
		LockedBy:        m.LockedBy,
		LastError:       m.LastError,
		MessageSnapshot: snapshot,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}, nil
}

func nullablePositive(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func toAttemptDomain(m *model.DeliveryAttempt) *domaindelivery.Attempt {
	return &domaindelivery.Attempt{
		ID:              m.ID,
		DeliveryTaskID:  m.DeliveryTaskID,
		AttemptNo:       m.AttemptNo,
		Status:          domaindelivery.AttemptStatus(m.Status),
		RequestSummary:  m.RequestSummary,
		ResponseSummary: m.ResponseSummary,
		Error:           m.Error,
		StartedAt:       m.StartedAt,
		FinishedAt:      m.FinishedAt,
		CreatedAt:       m.CreatedAt,
	}
}

func marshalMessageSnapshot(m *domainmessage.NormalizedMessage) (datatypes.JSON, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("序列化 message_snapshot 失败: %w", err)
	}
	return datatypes.JSON(b), nil
}

func unmarshalMessageSnapshot(raw datatypes.JSON) (*domainmessage.NormalizedMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m domainmessage.NormalizedMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("解析 message_snapshot 失败: %w", err)
	}
	return &m, nil
}
