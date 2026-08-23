package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainarchive "telegram-message-forward/internal/domain/chatarchive"
	"telegram-message-forward/internal/storage/model"
)

// ChatArchiveRepository 是 chatarchive.ArchiveRepository 的 PostgreSQL 实现。
type ChatArchiveRepository struct {
	db *gorm.DB
}

// NewChatArchiveRepository 创建归档会话仓储。
func NewChatArchiveRepository(db *gorm.DB) *ChatArchiveRepository {
	return &ChatArchiveRepository{db: db}
}

var _ domainarchive.ArchiveRepository = (*ChatArchiveRepository)(nil)

// Ensure 按 (account_id, peer_type, peer_id) 幂等获取或创建归档。
func (r *ChatArchiveRepository) Ensure(ctx context.Context, a *domainarchive.Archive) (*domainarchive.Archive, error) {
	mo := &model.ChatArchive{
		AccountID:    a.AccountID,
		PeerType:     string(a.PeerType),
		PeerID:       a.PeerID,
		PeerName:     a.PeerName,
		PeerUsername: a.PeerUsername,
	}
	// 已存在时刷新展示名，不覆盖统计与游标。
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}, {Name: "peer_type"}, {Name: "peer_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"peer_name":     mo.PeerName,
			"peer_username": mo.PeerUsername,
			"updated_at":    time.Now(),
		}),
	}).Create(mo).Error; err != nil {
		return nil, err
	}
	if mo.ID > 0 {
		return r.GetByID(ctx, mo.ID)
	}
	var existing model.ChatArchive
	if err := r.db.WithContext(ctx).
		Where("account_id = ? AND peer_type = ? AND peer_id = ?", a.AccountID, string(a.PeerType), a.PeerID).
		First(&existing).Error; err != nil {
		return nil, err
	}
	return toArchiveDomain(&existing), nil
}

// GetByID 按 id 读取归档。
func (r *ChatArchiveRepository) GetByID(ctx context.Context, id int64) (*domainarchive.Archive, error) {
	var mo model.ChatArchive
	if err := r.db.WithContext(ctx).First(&mo, id).Error; err != nil {
		return nil, err
	}
	return toArchiveDomain(&mo), nil
}

// List 返回全部归档，最近同步的在前。
func (r *ChatArchiveRepository) List(ctx context.Context) ([]*domainarchive.Archive, error) {
	var rows []model.ChatArchive
	if err := r.db.WithContext(ctx).
		Order("last_synced_at DESC NULLS LAST, id DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domainarchive.Archive, 0, len(rows))
	for i := range rows {
		out = append(out, toArchiveDomain(&rows[i]))
	}
	return out, nil
}

// Delete 删除归档，关联消息与任务经 FK CASCADE 一并删除。
func (r *ChatArchiveRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ChatArchive{}, id).Error
}

// RefreshStats 依据实际归档消息重算计数与覆盖区间。
//
// 用聚合重算而不是增量累加：批量 upsert 是幂等的，重复执行时增量累加会算多。
func (r *ChatArchiveRepository) RefreshStats(ctx context.Context, archiveID int64, syncedAt time.Time) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE chat_archives a SET
			message_count  = COALESCE(s.cnt, 0),
			media_count    = COALESCE(s.media_cnt, 0),
			min_message_id = COALESCE(s.min_id, 0),
			max_message_id = COALESCE(s.max_id, 0),
			last_synced_at = ?,
			updated_at     = now()
		FROM (
			SELECT COUNT(*) AS cnt,
				COUNT(*) FILTER (WHERE jsonb_array_length(media) > 0) AS media_cnt,
				MIN(message_id) AS min_id,
				MAX(message_id) AS max_id
			FROM chat_archive_messages WHERE archive_id = ?
		) s
		WHERE a.id = ?`, syncedAt, archiveID, archiveID).Error
}

func toArchiveDomain(mo *model.ChatArchive) *domainarchive.Archive {
	return &domainarchive.Archive{
		ID:           mo.ID,
		AccountID:    mo.AccountID,
		PeerType:     domainarchive.PeerType(mo.PeerType),
		PeerID:       mo.PeerID,
		PeerName:     mo.PeerName,
		PeerUsername: mo.PeerUsername,
		MinMessageID: mo.MinMessageID,
		MaxMessageID: mo.MaxMessageID,
		MessageCount: mo.MessageCount,
		MediaCount:   mo.MediaCount,
		LastSyncedAt: mo.LastSyncedAt,
		CreatedAt:    mo.CreatedAt,
		UpdatedAt:    mo.UpdatedAt,
	}
}

// ChatArchiveMessageRepository 是 chatarchive.MessageRepository 的 PostgreSQL 实现。
type ChatArchiveMessageRepository struct {
	db *gorm.DB
}

// NewChatArchiveMessageRepository 创建归档消息仓储。
func NewChatArchiveMessageRepository(db *gorm.DB) *ChatArchiveMessageRepository {
	return &ChatArchiveMessageRepository{db: db}
}

var _ domainarchive.MessageRepository = (*ChatArchiveMessageRepository)(nil)

// chatArchiveUpsertBatch 是单次 INSERT 的行数上限，避免超出参数上限。
const chatArchiveUpsertBatch = 200

// BulkUpsert 按 (archive_id, message_id) 幂等写入一批消息，返回新增条数。
func (r *ChatArchiveMessageRepository) BulkUpsert(ctx context.Context, archiveID int64, msgs []*domainarchive.Message) (int64, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	rows := make([]*model.ChatArchiveMessage, 0, len(msgs))
	for _, m := range msgs {
		mo, err := toArchiveMessageModel(archiveID, m)
		if err != nil {
			return 0, err
		}
		rows = append(rows, mo)
	}
	var inserted int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(rows); start += chatArchiveUpsertBatch {
			end := start + chatArchiveUpsertBatch
			if end > len(rows) {
				end = len(rows)
			}
			// 已归档的消息不覆盖：同一条消息重复拉取应保持幂等。
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "archive_id"}, {Name: "message_id"}},
				DoNothing: true,
			}).Create(rows[start:end])
			if res.Error != nil {
				return res.Error
			}
			inserted += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return inserted, nil
}

// Search 按检索条件倒序分页查询归档消息。
func (r *ChatArchiveMessageRepository) Search(ctx context.Context, q domainarchive.MessageQuery) ([]*domainarchive.Message, bool, error) {
	db := r.db.WithContext(ctx).Model(&model.ChatArchiveMessage{}).
		Where("archive_id = ?", q.ArchiveID)

	if q.Keyword != "" {
		// 走 text 的 trigram GIN 索引；转义 LIKE 通配符，避免用户输入的 % / _ 退化成通配。
		db = db.Where("text ILIKE ?", "%"+escapeLike(q.Keyword)+"%")
	}
	if q.SenderID != 0 {
		db = db.Where("sender_id = ?", q.SenderID)
	}
	if q.Out != nil {
		db = db.Where("outgoing = ?", *q.Out)
	}
	if !q.IncludeService {
		db = db.Where("service_action = ''")
	}
	if q.From != nil {
		db = db.Where("date >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("date < ?", *q.To)
	}
	if q.BeforeID > 0 {
		db = db.Where("id < ?", q.BeforeID)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	var rows []model.ChatArchiveMessage
	if err := db.Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out, err := toArchiveMessagesDomain(rows)
	return out, hasMore, err
}

// StreamPage 按主键升序取一页，供导出渲染分批读取。
func (r *ChatArchiveMessageRepository) StreamPage(ctx context.Context, q domainarchive.ExportQuery) ([]*domainarchive.Message, error) {
	db := r.db.WithContext(ctx).Model(&model.ChatArchiveMessage{}).
		Where("archive_id = ?", q.ArchiveID)
	if !q.IncludeService {
		db = db.Where("service_action = ''")
	}
	if q.From != nil {
		db = db.Where("date >= ?", *q.From)
	}
	if q.To != nil {
		db = db.Where("date < ?", *q.To)
	}
	if q.AfterID > 0 {
		db = db.Where("id > ?", q.AfterID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}
	var rows []model.ChatArchiveMessage
	if err := db.Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return toArchiveMessagesDomain(rows)
}

// CountByArchive 返回归档的消息条数。
func (r *ChatArchiveMessageRepository) CountByArchive(ctx context.Context, archiveID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ChatArchiveMessage{}).
		Where("archive_id = ?", archiveID).Count(&count).Error
	return count, err
}

// escapeLike 转义 LIKE/ILIKE 的通配符，使关键词按字面量匹配。
func escapeLike(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch r {
		case '\\', '%', '_':
			out = append(out, '\\', r)
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func toArchiveMessageModel(archiveID int64, m *domainarchive.Message) (*model.ChatArchiveMessage, error) {
	entities, err := marshalJSONSlice(m.Entities)
	if err != nil {
		return nil, err
	}
	media, err := marshalJSONSlice(m.Media)
	if err != nil {
		return nil, err
	}
	reactions, err := marshalJSONSlice(m.Reactions)
	if err != nil {
		return nil, err
	}
	mo := &model.ChatArchiveMessage{
		ArchiveID:        archiveID,
		MessageID:        m.MessageID,
		GroupedID:        m.GroupedID,
		ReplyToMessageID: m.ReplyToMessageID,
		Outgoing:         m.Out,
		SenderPeerType:   m.SenderPeerType,
		SenderID:         m.SenderID,
		SenderName:       m.SenderName,
		SenderUsername:   m.SenderUsername,
		MessageType:      m.MessageType,
		Text:             m.Text,
		Entities:         entities,
		Media:            media,
		Reactions:        reactions,
		ServiceAction:    m.ServiceAction,
		Views:            m.Views,
		Date:             m.Date,
		EditDate:         m.EditDate,
	}
	if m.Forward != nil {
		// 不能复用 marshalJSONSlice：它把 null 归一成 "[]"，对单个对象语义不对。
		raw, err := json.Marshal(m.Forward)
		if err != nil {
			return nil, err
		}
		mo.FwdFrom = datatypes.JSON(raw)
	}
	return mo, nil
}

func toArchiveMessagesDomain(rows []model.ChatArchiveMessage) ([]*domainarchive.Message, error) {
	out := make([]*domainarchive.Message, 0, len(rows))
	for i := range rows {
		m, err := toArchiveMessageDomain(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func toArchiveMessageDomain(mo *model.ChatArchiveMessage) (*domainarchive.Message, error) {
	m := &domainarchive.Message{
		ID:               mo.ID,
		ArchiveID:        mo.ArchiveID,
		MessageID:        mo.MessageID,
		GroupedID:        mo.GroupedID,
		ReplyToMessageID: mo.ReplyToMessageID,
		Out:              mo.Outgoing,
		SenderPeerType:   mo.SenderPeerType,
		SenderID:         mo.SenderID,
		SenderName:       mo.SenderName,
		SenderUsername:   mo.SenderUsername,
		MessageType:      mo.MessageType,
		Text:             mo.Text,
		ServiceAction:    mo.ServiceAction,
		Views:            mo.Views,
		Date:             mo.Date,
		EditDate:         mo.EditDate,
		CreatedAt:        mo.CreatedAt,
	}
	if err := unmarshalJSON(mo.Entities, &m.Entities); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(mo.Media, &m.Media); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(mo.Reactions, &m.Reactions); err != nil {
		return nil, err
	}
	if len(mo.FwdFrom) > 0 {
		var fwd domainarchive.Forward
		if err := unmarshalJSON(mo.FwdFrom, &fwd); err != nil {
			return nil, err
		}
		m.Forward = &fwd
	}
	return m, nil
}

// ChatExportJobRepository 是 chatarchive.JobRepository 的 PostgreSQL 实现。
type ChatExportJobRepository struct {
	db *gorm.DB
}

// NewChatExportJobRepository 创建导出任务仓储。
func NewChatExportJobRepository(db *gorm.DB) *ChatExportJobRepository {
	return &ChatExportJobRepository{db: db}
}

var _ domainarchive.JobRepository = (*ChatExportJobRepository)(nil)

// ErrActiveJobExists 表示该归档已有在跑的任务。
var ErrActiveJobExists = errors.New("该归档已有进行中的导出任务")

// Create 创建任务。命中「同归档只允许一个活跃任务」的唯一索引时返回 ErrActiveJobExists。
func (r *ChatExportJobRepository) Create(ctx context.Context, j *domainarchive.Job) error {
	mo := toJobModel(j)
	if err := r.db.WithContext(ctx).Create(mo).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrActiveJobExists
		}
		return err
	}
	j.ID = mo.ID
	j.CreatedAt = mo.CreatedAt
	j.UpdatedAt = mo.UpdatedAt
	return nil
}

// Update 全量更新任务状态与进度。
func (r *ChatExportJobRepository) Update(ctx context.Context, j *domainarchive.Job) error {
	return r.db.WithContext(ctx).Model(&model.ChatExportJob{}).
		Where("id = ?", j.ID).
		Updates(map[string]any{
			"status":           string(j.Status),
			"fetched_count":    j.FetchedCount,
			"media_count":      j.MediaCount,
			"cursor_offset_id": j.CursorOffsetID,
			"last_error":       j.LastError,
			"started_at":       j.StartedAt,
			"finished_at":      j.FinishedAt,
			"updated_at":       time.Now(),
		}).Error
}

// GetByID 按 id 读取任务。
func (r *ChatExportJobRepository) GetByID(ctx context.Context, id int64) (*domainarchive.Job, error) {
	var mo model.ChatExportJob
	if err := r.db.WithContext(ctx).First(&mo, id).Error; err != nil {
		return nil, err
	}
	return toJobDomain(&mo), nil
}

// ListByArchive 返回某归档的任务，最新在前。
func (r *ChatExportJobRepository) ListByArchive(ctx context.Context, archiveID int64, limit int) ([]*domainarchive.Job, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []model.ChatExportJob
	if err := r.db.WithContext(ctx).
		Where("archive_id = ?", archiveID).
		Order("created_at DESC, id DESC").
		Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return toJobsDomain(rows), nil
}

// ListActive 返回未到终态的任务，用于服务重启后回收。
func (r *ChatExportJobRepository) ListActive(ctx context.Context) ([]*domainarchive.Job, error) {
	var rows []model.ChatExportJob
	if err := r.db.WithContext(ctx).
		Where("status IN ?", []string{string(domainarchive.JobPending), string(domainarchive.JobRunning)}).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return toJobsDomain(rows), nil
}

// RequestCancel 置位取消标记；执行器在页边界检查后停止。
func (r *ChatExportJobRepository) RequestCancel(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&model.ChatExportJob{}).
		Where("id = ?", id).
		Updates(map[string]any{"cancel_requested": true, "updated_at": time.Now()}).Error
}

func toJobModel(j *domainarchive.Job) *model.ChatExportJob {
	status := j.Status
	if status == "" {
		status = domainarchive.JobPending
	}
	return &model.ChatExportJob{
		ArchiveID:     j.ArchiveID,
		Status:        string(status),
		FromDate:      j.FromDate,
		ToDate:        j.ToDate,
		IncludeMedia:  j.IncludeMedia,
		MediaMaxBytes: j.MediaMaxBytes,
		MaxMessages:   j.MaxMessages,
	}
}

func toJobsDomain(rows []model.ChatExportJob) []*domainarchive.Job {
	out := make([]*domainarchive.Job, 0, len(rows))
	for i := range rows {
		out = append(out, toJobDomain(&rows[i]))
	}
	return out
}

func toJobDomain(mo *model.ChatExportJob) *domainarchive.Job {
	return &domainarchive.Job{
		ID:              mo.ID,
		ArchiveID:       mo.ArchiveID,
		Status:          domainarchive.JobStatus(mo.Status),
		FromDate:        mo.FromDate,
		ToDate:          mo.ToDate,
		IncludeMedia:    mo.IncludeMedia,
		MediaMaxBytes:   mo.MediaMaxBytes,
		MaxMessages:     mo.MaxMessages,
		FetchedCount:    mo.FetchedCount,
		MediaCount:      mo.MediaCount,
		CursorOffsetID:  mo.CursorOffsetID,
		CancelRequested: mo.CancelRequested,
		LastError:       mo.LastError,
		StartedAt:       mo.StartedAt,
		FinishedAt:      mo.FinishedAt,
		CreatedAt:       mo.CreatedAt,
		UpdatedAt:       mo.UpdatedAt,
	}
}
