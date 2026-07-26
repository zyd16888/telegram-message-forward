package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

const aiDigestRunItemBatchSize = 500

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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "uq_ai_digest_runs_active_profile" {
			return domainaidigest.ErrActiveRunExists
		}
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
			"name":               m.Name,
			"enabled":            m.Enabled,
			"source_ids":         m.SourceIDs,
			"conditions":         m.Conditions,
			"schedule":           m.Schedule,
			"window":             m.Window,
			"dedupe":             m.Dedupe,
			"prompt_template":    m.PromptTemplate,
			"output_format":      m.OutputFormat,
			"output_template_id": m.OutputTemplateID,
			"output_template":    m.OutputTemplate,
			"filter_id":          m.FilterID,
			"filter_ids":         m.FilterIDs,
			"target_sink_ids":    m.TargetSinkIDs,
			"model_config":       m.ModelConfig,
			"limits":             m.Limits,
			"multimodal":         m.Multimodal,
			"updated_at":         m.UpdatedAt,
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

func (r *AIDigestRepository) ListOutputTemplates(ctx context.Context) ([]*domainaidigest.OutputTemplate, error) {
	var ms []model.AIDigestOutputTemplate
	if err := r.db.WithContext(ctx).Order("built_in DESC, id ASC").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainaidigest.OutputTemplate, 0, len(ms))
	for i := range ms {
		out = append(out, toAIDigestOutputTemplateDomain(&ms[i]))
	}
	return out, nil
}

func (r *AIDigestRepository) GetOutputTemplate(ctx context.Context, id int64) (*domainaidigest.OutputTemplate, error) {
	var m model.AIDigestOutputTemplate
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toAIDigestOutputTemplateDomain(&m), nil
}

func (r *AIDigestRepository) CreateOutputTemplate(ctx context.Context, t *domainaidigest.OutputTemplate) error {
	m := &model.AIDigestOutputTemplate{
		Name:        t.Name,
		Description: t.Description,
		Format:      t.Format,
		Content:     t.Content,
		BuiltIn:     t.BuiltIn,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	t.ID = m.ID
	t.CreatedAt = m.CreatedAt
	t.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *AIDigestRepository) UpdateOutputTemplate(ctx context.Context, t *domainaidigest.OutputTemplate) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.AIDigestOutputTemplate{}).
		Where("id = ?", t.ID).
		Updates(map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"format":      t.Format,
			"content":     t.Content,
			"updated_at":  now,
		}).Error; err != nil {
		return err
	}
	t.UpdatedAt = now
	return nil
}

func (r *AIDigestRepository) DeleteOutputTemplate(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.AIDigestOutputTemplate{}, id).Error
}

func (r *AIDigestRepository) CountProfilesUsingTemplate(ctx context.Context, templateID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AIDigestProfile{}).
		Where("output_template_id = ?", templateID).
		Count(&count).Error
	return count, err
}

func toAIDigestOutputTemplateDomain(m *model.AIDigestOutputTemplate) *domainaidigest.OutputTemplate {
	return &domainaidigest.OutputTemplate{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Format:      m.Format,
		Content:     m.Content,
		BuiltIn:     m.BuiltIn,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
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
			"status":               m.Status,
			"input_message_count":  m.InputMessageCount,
			"included_count":       m.IncludedCount,
			"excluded_count":       m.ExcludedCount,
			"prompt_message_count": m.PromptMessageCount,
			"prompt_omitted_count": m.PromptOmittedCount,
			"prompt_chars":         m.PromptChars,
			"delivery_task_ids":    m.DeliveryTaskIDs,
			"provider_id":          m.ProviderID,
			"provider_name":        m.ProviderName,
			"model_name":           m.ModelName,
			"system_prompt":        m.SystemPrompt,
			"user_prompt":          m.UserPrompt,
			"request_config":       m.RequestConfig,
			"media_audit":          m.MediaAudit,
			"token_usage":          m.TokenUsage,
			"error":                m.Error,
			"started_at":           m.StartedAt,
			"finished_at":          m.FinishedAt,
			"profile_snapshot":     m.ProfileSnapshot,
		}).Error
}

func (r *AIDigestRepository) HasRunningRun(ctx context.Context, profileID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AIDigestRun{}).
		Where("profile_id = ? AND status IN ?", profileID, []string{"pending", "running"}).
		Count(&count).Error
	return count > 0, err
}

func (r *AIDigestRepository) LastExecutionRun(ctx context.Context, profileID int64) (*domainaidigest.Run, error) {
	var m model.AIDigestRun
	if err := r.db.WithContext(ctx).
		Omit("system_prompt", "user_prompt", "request_config", "media_audit").
		Where("profile_id = ? AND trigger_type <> ?", profileID, string(domainaidigest.TriggerPreview)).
		Order("id DESC").
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toAIDigestRunDomain(&m)
}

func (r *AIDigestRepository) LastSuccessfulRun(ctx context.Context, profileID int64) (*domainaidigest.Run, error) {
	var m model.AIDigestRun
	if err := r.db.WithContext(ctx).
		Omit("system_prompt", "user_prompt", "request_config", "media_audit").
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
	db := r.db.WithContext(ctx).
		Omit("system_prompt", "user_prompt", "request_config", "media_audit").
		Order("id DESC").Limit(limit).Offset(offset)
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

func (r *AIDigestRepository) CountRuns(ctx context.Context, profileID int64) (int64, error) {
	db := r.db.WithContext(ctx).Model(&model.AIDigestRun{})
	if profileID > 0 {
		db = db.Where("profile_id = ?", profileID)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
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
		snapshot := item.MessageSnapshot
		if snapshot == nil {
			snapshot = domainaidigest.NewMessageSnapshot(item.Message)
		}
		snapshotJSON, err := marshalJSON(snapshot)
		if err != nil {
			return err
		}
		ms = append(ms, model.AIDigestRunItem{
			RunID:           item.RunID,
			MessageID:       item.MessageID,
			SourceID:        item.SourceID,
			Included:        item.Included,
			Reason:          item.Reason,
			Score:           item.Score,
			SortOrder:       item.SortOrder,
			MessageSnapshot: snapshotJSON,
		})
	}
	return r.db.WithContext(ctx).CreateInBatches(&ms, aiDigestRunItemBatchSize).Error
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
		var snapshot domainaidigest.MessageSnapshot
		if err := unmarshalJSON(rows[i].MessageSnapshot, &snapshot); err != nil {
			return nil, err
		}
		item := &domainaidigest.RunItem{
			RunID:     rows[i].RunID,
			MessageID: rows[i].MessageID,
			SourceID:  rows[i].SourceID,
			Included:  rows[i].Included,
			Reason:    rows[i].Reason,
			Score:     rows[i].Score,
			SortOrder: rows[i].SortOrder,
		}
		if snapshot.ID > 0 {
			item.MessageSnapshot = &snapshot
		}
		out = append(out, item)
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

func (r *AIDigestRepository) CleanupRuns(ctx context.Context, before time.Time, statuses []domainaidigest.RunStatus) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("created_at < ? AND status IN ?", before, statuses).
		Delete(&model.AIDigestRun{})
	return res.RowsAffected, res.Error
}

func (r *AIDigestRepository) RecoverStaleRuns(ctx context.Context, before, finishedAt time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.AIDigestRun{}).
		Where("status IN ? AND COALESCE(started_at, created_at) < ?", []string{"pending", "running"}, before).
		Updates(map[string]any{
			"status":      string(domainaidigest.RunFailed),
			"finished_at": finishedAt,
			"error":       "运行进程异常中断或超过 2 小时未完成，已自动恢复",
		})
	return res.RowsAffected, res.Error
}

func (r *AIDigestRepository) AggregateStatsSince(ctx context.Context, since time.Time) (map[int64]domainaidigest.ProfileStats, error) {
	type row struct {
		ProfileID int64
		Runs      int64
		Success   int64
		Failed    int64
		Tokens    int64
	}
	var rows []row
	// token_usage 为 jsonb，total_tokens 可能缺失。
	err := r.db.WithContext(ctx).Raw(`
SELECT
  profile_id,
  COUNT(*) AS runs,
  COUNT(*) FILTER (WHERE status = 'success') AS success,
  COUNT(*) FILTER (WHERE status = 'failed') AS failed,
  COALESCE(SUM(COALESCE((token_usage->>'total_tokens')::bigint, 0)), 0) AS tokens
FROM ai_digest_runs
WHERE created_at >= ? AND profile_id IS NOT NULL AND trigger_type <> 'preview'
GROUP BY profile_id
`, since).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]domainaidigest.ProfileStats, len(rows))
	for _, row := range rows {
		out[row.ProfileID] = domainaidigest.ProfileStats{
			Runs:    row.Runs,
			Success: row.Success,
			Failed:  row.Failed,
			Tokens:  row.Tokens,
		}
	}
	return out, nil
}

func (r *AIDigestRepository) AggregateGlobalStatsSince(ctx context.Context, since time.Time) (domainaidigest.ProfileStats, error) {
	type row struct {
		Runs    int64
		Success int64
		Failed  int64
		Tokens  int64
	}
	var one row
	err := r.db.WithContext(ctx).Raw(`
SELECT
  COUNT(*) AS runs,
  COUNT(*) FILTER (WHERE status = 'success') AS success,
  COUNT(*) FILTER (WHERE status = 'failed') AS failed,
  COALESCE(SUM(COALESCE((token_usage->>'total_tokens')::bigint, 0)), 0) AS tokens
FROM ai_digest_runs
WHERE created_at >= ? AND trigger_type <> 'preview'
`, since).Scan(&one).Error
	if err != nil {
		return domainaidigest.ProfileStats{}, err
	}
	return domainaidigest.ProfileStats{
		Runs:    one.Runs,
		Success: one.Success,
		Failed:  one.Failed,
		Tokens:  one.Tokens,
	}, nil
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
	multimodal, err := marshalJSON(p.Multimodal)
	if err != nil {
		return nil, err
	}
	filterIDs := normalizeFilterIDs(p.FilterIDs, p.FilterID)
	filterIDsJSON, err := marshalJSON(filterIDs)
	if err != nil {
		return nil, err
	}
	primaryFilter := int64(0)
	if len(filterIDs) > 0 {
		primaryFilter = filterIDs[0]
	}
	return &model.AIDigestProfile{
		ID:               p.ID,
		Name:             p.Name,
		Enabled:          p.Enabled,
		SourceIDs:        sourceIDs,
		Conditions:       conditions,
		Schedule:         schedule,
		Window:           window,
		Dedupe:           dedupe,
		PromptTemplate:   p.PromptTemplate,
		OutputFormat:     p.OutputFormat,
		OutputTemplateID: nullablePositive(p.OutputTemplateID),
		OutputTemplate:   p.OutputTemplate,
		FilterID:         nullablePositive(primaryFilter),
		FilterIDs:        filterIDsJSON,
		TargetSinkIDs:    targets,
		ModelConfig:      modelCfg,
		Limits:           limits,
		Multimodal:       multimodal,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}, nil
}

func normalizeFilterIDs(ids []int64, legacy int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 && legacy > 0 {
		out = append(out, legacy)
	}
	return out
}

func toAIDigestProfileDomain(m *model.AIDigestProfile) (*domainaidigest.Profile, error) {
	var p domainaidigest.Profile
	p.ID = m.ID
	p.Name = m.Name
	p.Enabled = m.Enabled
	p.PromptTemplate = m.PromptTemplate
	p.OutputFormat = m.OutputFormat
	if m.OutputTemplateID != nil {
		p.OutputTemplateID = *m.OutputTemplateID
	}
	p.OutputTemplate = m.OutputTemplate
	if m.FilterID != nil {
		p.FilterID = *m.FilterID
	}
	p.CreatedAt = m.CreatedAt
	p.UpdatedAt = m.UpdatedAt
	if len(m.FilterIDs) > 0 {
		if err := unmarshalJSON(m.FilterIDs, &p.FilterIDs); err != nil {
			return nil, err
		}
	}
	p.FilterIDs = normalizeFilterIDs(p.FilterIDs, p.FilterID)
	if p.FilterID == 0 && len(p.FilterIDs) > 0 {
		p.FilterID = p.FilterIDs[0]
	}
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
	if err := unmarshalJSON(m.Multimodal, &p.Multimodal); err != nil {
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
	requestConfig, err := marshalJSON(r.RequestConfig)
	if err != nil {
		return nil, err
	}
	mediaAudit, err := marshalJSON(r.MediaAudit)
	if err != nil {
		return nil, err
	}
	profileSnapshot, err := marshalJSON(r.ProfileSnapshot)
	if err != nil {
		return nil, err
	}
	return &model.AIDigestRun{
		ID:                 r.ID,
		ProfileID:          nullablePositive(r.ProfileID),
		Status:             string(r.Status),
		TriggerType:        string(r.TriggerType),
		WindowStart:        r.WindowStart,
		WindowEnd:          r.WindowEnd,
		InputMessageCount:  r.InputMessageCount,
		IncludedCount:      r.IncludedCount,
		ExcludedCount:      r.ExcludedCount,
		PromptMessageCount: r.PromptMessageCount,
		PromptOmittedCount: r.PromptOmittedCount,
		PromptChars:        r.PromptChars,
		DeliveryTaskIDs:    taskIDs,
		ProviderID:         r.ProviderID,
		ProviderName:       r.ProviderName,
		ModelName:          r.ModelName,
		SystemPrompt:       r.SystemPrompt,
		UserPrompt:         r.UserPrompt,
		RequestConfig:      requestConfig,
		MediaAudit:         mediaAudit,
		TokenUsage:         usage,
		Error:              r.Error,
		StartedAt:          r.StartedAt,
		FinishedAt:         r.FinishedAt,
		CreatedAt:          r.CreatedAt,
		ProfileSnapshot:    profileSnapshot,
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
	r.PromptMessageCount = m.PromptMessageCount
	r.PromptOmittedCount = m.PromptOmittedCount
	r.PromptChars = m.PromptChars
	r.ProviderID = m.ProviderID
	r.ProviderName = m.ProviderName
	r.ModelName = m.ModelName
	r.SystemPrompt = m.SystemPrompt
	r.UserPrompt = m.UserPrompt
	r.Error = m.Error
	r.StartedAt = m.StartedAt
	r.FinishedAt = m.FinishedAt
	r.CreatedAt = m.CreatedAt
	if err := unmarshalJSON(m.DeliveryTaskIDs, &r.DeliveryTaskIDs); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.RequestConfig, &r.RequestConfig); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.MediaAudit, &r.MediaAudit); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(m.TokenUsage, &r.TokenUsage); err != nil {
		return nil, err
	}
	if len(m.ProfileSnapshot) > 0 && string(m.ProfileSnapshot) != "{}" && string(m.ProfileSnapshot) != "null" {
		var snapshot domainaidigest.Profile
		if err := unmarshalJSON(m.ProfileSnapshot, &snapshot); err != nil {
			return nil, err
		}
		r.ProfileSnapshot = &snapshot
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
