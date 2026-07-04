package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	"telegram-message-forward/internal/storage/model"
)

// AIDigestRepository 是 AI 整理 profile/run 仓储。
type AIDigestRepository struct {
	db *gorm.DB
}

func NewAIDigestRepository(db *gorm.DB) *AIDigestRepository {
	return &AIDigestRepository{db: db}
}

var _ domainaidigest.Repository = (*AIDigestRepository)(nil)

func (r *AIDigestRepository) CreateProfile(ctx context.Context, p *domainaidigest.Profile) error {
	m, err := toAIDigestProfileModel(p)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	p.ID = m.ID
	p.CreatedAt = m.CreatedAt
	p.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *AIDigestRepository) UpdateProfile(ctx context.Context, p *domainaidigest.Profile) error {
	m, err := toAIDigestProfileModel(p)
	if err != nil {
		return err
	}
	m.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Model(&model.AIDigestProfile{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"name":            m.Name,
			"enabled":         m.Enabled,
			"source_ids":      m.SourceIDs,
			"conditions":      m.Conditions,
			"schedule":        m.Schedule,
			"window":          m.Window,
			"dedupe":          m.Dedupe,
			"prompt_template": m.PromptTemplate,
			"output_format":   m.OutputFormat,
			"target_sink_ids": m.TargetSinkIDs,
			"model_config":    m.ModelConfig,
			"limits":          m.Limits,
			"updated_at":      m.UpdatedAt,
		}).Error; err != nil {
		return err
	}
	p.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *AIDigestRepository) GetProfile(ctx context.Context, id int64) (*domainaidigest.Profile, error) {
	var m model.AIDigestProfile
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	p, err := toAIDigestProfileDomain(&m)
	if err != nil {
		return nil, err
	}
	p.RecentRun, _ = r.latestRun(ctx, p.ID)
	return p, nil
}

func (r *AIDigestRepository) ListProfiles(ctx context.Context) ([]*domainaidigest.Profile, error) {
	var ms []model.AIDigestProfile
	if err := r.db.WithContext(ctx).Order("id DESC").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainaidigest.Profile, 0, len(ms))
	for i := range ms {
		p, err := toAIDigestProfileDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		p.RecentRun, _ = r.latestRun(ctx, p.ID)
		out = append(out, p)
	}
	return out, nil
}

func (r *AIDigestRepository) DeleteProfile(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.AIDigestProfile{}, id).Error
}

func (r *AIDigestRepository) CreateRun(ctx context.Context, run *domainaidigest.Run) error {
	m, err := toAIDigestRunModel(run)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	run.ID = m.ID
	run.CreatedAt = m.CreatedAt
	return nil
}

func (r *AIDigestRepository) UpdateRun(ctx context.Context, run *domainaidigest.Run) error {
	m, err := toAIDigestRunModel(run)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.AIDigestRun{}).
		Where("id = ?", run.ID).
		Updates(map[string]any{
			"status":              m.Status,
			"input_message_count": m.InputMessageCount,
			"included_count":      m.IncludedCount,
			"excluded_count":      m.ExcludedCount,
			"delivery_task_ids":   m.DeliveryTaskIDs,
			"model_name":          m.ModelName,
			"token_usage":         m.TokenUsage,
			"error":               m.Error,
			"started_at":          m.StartedAt,
			"finished_at":         m.FinishedAt,
		}).Error
}

func (r *AIDigestRepository) HasRunningRun(ctx context.Context, profileID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AIDigestRun{}).
		Where("profile_id = ? AND status IN ?", profileID, []string{"pending", "running"}).
		Count(&count).Error
	return count > 0, err
}

func (r *AIDigestRepository) LastSuccessfulRun(ctx context.Context, profileID int64) (*domainaidigest.Run, error) {
	var m model.AIDigestRun
	if err := r.db.WithContext(ctx).
		Where("profile_id = ? AND status = ?", profileID, string(domainaidigest.RunSuccess)).
		Order("window_end DESC, id DESC").
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toAIDigestRunDomain(&m)
}

func (r *AIDigestRepository) ListRuns(ctx context.Context, profileID int64, limit, offset int) ([]*domainaidigest.Run, error) {
	if limit <= 0 {
		limit = 50
	}
	db := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset)
	if profileID > 0 {
		db = db.Where("profile_id = ?", profileID)
	}
	var ms []model.AIDigestRun
	if err := db.Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainaidigest.Run, 0, len(ms))
	for i := range ms {
		run, err := toAIDigestRunDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (r *AIDigestRepository) GetRun(ctx context.Context, id int64) (*domainaidigest.Run, error) {
	var m model.AIDigestRun
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toAIDigestRunDomain(&m)
}

func (r *AIDigestRepository) AddRunItems(ctx context.Context, items []*domainaidigest.RunItem) error {
	if len(items) == 0 {
		return nil
	}
	ms := make([]model.AIDigestRunItem, 0, len(items))
	for _, item := range items {
		ms = append(ms, model.AIDigestRunItem{
			RunID:     item.RunID,
			MessageID: item.MessageID,
			SourceID:  item.SourceID,
			Included:  item.Included,
			Reason:    item.Reason,
			Score:     item.Score,
			SortOrder: item.SortOrder,
		})
	}
	return r.db.WithContext(ctx).Create(&ms).Error
}

func (r *AIDigestRepository) ListRunItems(ctx context.Context, runID int64) ([]*domainaidigest.RunItem, error) {
	var rows []model.AIDigestRunItem
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("sort_order ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domainaidigest.RunItem, 0, len(rows))
	for i := range rows {
		var mm model.Message
		if err := r.db.WithContext(ctx).First(&mm, rows[i].MessageID).Error; err != nil {
			return nil, err
		}
		msg, err := toMessageDomain(&mm)
		if err != nil {
			return nil, err
		}
		out = append(out, &domainaidigest.RunItem{
			RunID:     rows[i].RunID,
			MessageID: rows[i].MessageID,
			SourceID:  rows[i].SourceID,
			Included:  rows[i].Included,
			Reason:    rows[i].Reason,
			Score:     rows[i].Score,
			SortOrder: rows[i].SortOrder,
			Message:   msg,
		})
	}
	return out, nil
}

func (r *AIDigestRepository) UpsertOutput(ctx context.Context, out *domainaidigest.Output) error {
	m := &model.AIDigestOutput{
		ID:          out.ID,
		RunID:       out.RunID,
		Format:      out.Format,
		Title:       out.Title,
		Content:     out.Content,
		RawResponse: datatypes.JSON(out.RawResponse),
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "run_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"format", "title", "content", "raw_response"}),
		}).
		Create(m).Error; err != nil {
		return err
	}
	out.ID = m.ID
	return nil
}

func (r *AIDigestRepository) GetOutputByRunID(ctx context.Context, runID int64) (*domainaidigest.Output, error) {
	var m model.AIDigestOutput
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&m).Error; err != nil {
		return nil, err
	}
	return toAIDigestOutputDomain(&m), nil
}

func (r *AIDigestRepository) CleanupRuns(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("created_at < ?", before).Delete(&model.AIDigestRun{})
	return res.RowsAffected, res.Error
}

func (r *AIDigestRepository) latestRun(ctx context.Context, profileID int64) (*domainaidigest.Run, error) {
	var m model.AIDigestRun
	if err := r.db.WithContext(ctx).
		Where("profile_id = ?", profileID).
		Order("id DESC").
		First(&m).Error; err != nil {
		return nil, err
	}
	return toAIDigestRunDomain(&m)
}

func toAIDigestProfileModel(p *domainaidigest.Profile) (*model.AIDigestProfile, error) {
	sourceIDs, err := marshalJSON(p.SourceIDs)
	if err != nil {
		return nil, err
	}
	conditions, err := marshalJSON(p.Conditions)
	if err != nil {
		return nil, err
	}
	targets, err := marshalJSON(p.TargetSinkIDs)
	if err != nil {
		return nil, err
	}
	schedule, err := marshalJSON(p.Schedule)
	if err != nil {
		return nil, err
	}
	window, err := marshalJSON(p.Window)
	if err != nil {
		return nil, err
	}
	dedupe, err := marshalJSON(p.Dedupe)
	if err != nil {
		return nil, err
	}
	modelCfg, err := marshalJSON(p.ModelConfig)
	if err != nil {
		return nil, err
	}
	limits, err := marshalJSON(p.Limits)
	if err != nil {
		return nil, err
	}
	return &model.AIDigestProfile{
		ID:             p.ID,
		Name:           p.Name,
		Enabled:        p.Enabled,
		SourceIDs:      sourceIDs,
		Conditions:     conditions,
		Schedule:       schedule,
		Window:         window,
		Dedupe:         dedupe,
		PromptTemplate: p.PromptTemplate,
		OutputFormat:   p.OutputFormat,
		TargetSinkIDs:  targets,
		ModelConfig:    modelCfg,
		Limits:         limits,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}, nil
}

func toAIDigestProfileDomain(m *model.AIDigestProfile) (*domainaidigest.Profile, error) {
	var p domainaidigest.Profile
	p.ID = m.ID
	p.Name = m.Name
	p.Enabled = m.Enabled
	p.PromptTemplate = m.PromptTemplate
	p.OutputFormat = m.OutputFormat
	p.CreatedAt = m.CreatedAt
	p.UpdatedAt = m.UpdatedAt
	if err := unmarshalJSON(m.SourceIDs, &p.SourceIDs); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.Conditions, &p.Conditions); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.Schedule, &p.Schedule); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.Window, &p.Window); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.Dedupe, &p.Dedupe); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.TargetSinkIDs, &p.TargetSinkIDs); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.ModelConfig, &p.ModelConfig); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.Limits, &p.Limits); err != nil {
		return nil, err
	}
	return &p, nil
}

func toAIDigestRunModel(r *domainaidigest.Run) (*model.AIDigestRun, error) {
	taskIDs, err := marshalJSON(r.DeliveryTaskIDs)
	if err != nil {
		return nil, err
	}
	usage, err := marshalJSON(r.TokenUsage)
	if err != nil {
		return nil, err
	}
	return &model.AIDigestRun{
		ID:                r.ID,
		ProfileID:         nullablePositive(r.ProfileID),
		Status:            string(r.Status),
		TriggerType:       string(r.TriggerType),
		WindowStart:       r.WindowStart,
		WindowEnd:         r.WindowEnd,
		InputMessageCount: r.InputMessageCount,
		IncludedCount:     r.IncludedCount,
		ExcludedCount:     r.ExcludedCount,
		DeliveryTaskIDs:   taskIDs,
		ModelName:         r.ModelName,
		TokenUsage:        usage,
		Error:             r.Error,
		StartedAt:         r.StartedAt,
		FinishedAt:        r.FinishedAt,
		CreatedAt:         r.CreatedAt,
	}, nil
}

func toAIDigestRunDomain(m *model.AIDigestRun) (*domainaidigest.Run, error) {
	var r domainaidigest.Run
	r.ID = m.ID
	if m.ProfileID != nil {
		r.ProfileID = *m.ProfileID
	}
	r.Status = domainaidigest.RunStatus(m.Status)
	r.TriggerType = domainaidigest.TriggerType(m.TriggerType)
	r.WindowStart = m.WindowStart
	r.WindowEnd = m.WindowEnd
	r.InputMessageCount = m.InputMessageCount
	r.IncludedCount = m.IncludedCount
	r.ExcludedCount = m.ExcludedCount
	r.ModelName = m.ModelName
	r.Error = m.Error
	r.StartedAt = m.StartedAt
	r.FinishedAt = m.FinishedAt
	r.CreatedAt = m.CreatedAt
	if err := unmarshalJSON(m.DeliveryTaskIDs, &r.DeliveryTaskIDs); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.TokenUsage, &r.TokenUsage); err != nil {
		return nil, err
	}
	return &r, nil
}

func toAIDigestOutputDomain(m *model.AIDigestOutput) *domainaidigest.Output {
	return &domainaidigest.Output{
		ID:          m.ID,
		RunID:       m.RunID,
		Format:      m.Format,
		Title:       m.Title,
		Content:     m.Content,
		RawResponse: []byte(m.RawResponse),
		CreatedAt:   m.CreatedAt,
	}
}

func marshalJSON(v any) (datatypes.JSON, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	return datatypes.JSON(b), nil
}

func unmarshalJSON(raw datatypes.JSON, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}
