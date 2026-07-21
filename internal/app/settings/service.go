// Package settings 提供页面可管理的系统设置应用服务。
//
// 设置按组存储（settings 表一组一行），页面保存的值优先于配置文件，
// 数据库无记录时回退到配置文件默认值。
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"telegram-message-forward/internal/config"
	domainsettings "telegram-message-forward/internal/domain/settings"
)

// MediaSettings 是媒体存储设置（不含敏感字段，敏感字段单独加密存储）。
type MediaSettings struct {
	Dir            string           `json:"dir"`
	PublicBaseURL  string           `json:"public_base_url"`
	URLTTLHours    float64          `json:"url_ttl_hours"`
	RetentionHours float64          `json:"retention_hours"`
	Download       DownloadSettings `json:"download"`
	S3             S3Settings       `json:"s3"`
}

// DownloadSettings 是 Source 侧媒体下载策略。
type DownloadSettings struct {
	// ImageMaxMB 是图片下载大小上限（MB），<=0 使用默认 20。
	ImageMaxMB float64 `json:"image_max_mb"`
	// FileMaxMB 是文件下载大小上限（MB），<=0 使用默认 50。
	FileMaxMB float64 `json:"file_max_mb"`
	// FileTypes 是文件扩展名白名单（不带点）；空列表表示不限类型，
	// nil 表示未配置（回退到配置文件默认值）。
	FileTypes []string `json:"file_types"`
}

// S3Settings 是 S3 兼容对象存储设置（secret_key 除外）。
type S3Settings struct {
	Enabled       bool   `json:"enabled"`
	Endpoint      string `json:"endpoint"`
	Region        string `json:"region"`
	Bucket        string `json:"bucket"`
	AccessKey     string `json:"access_key"`
	UseSSL        bool   `json:"use_ssl"`
	KeyPrefix     string `json:"key_prefix"`
	PublicBaseURL string `json:"public_base_url"`
	AutoCleanup   bool   `json:"auto_cleanup"`
}

// URLTTL 返回签名 URL 有效期。
func (m MediaSettings) URLTTL() time.Duration {
	if m.URLTTLHours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(m.URLTTLHours * float64(time.Hour))
}

// Retention 返回媒体保留时长；0 表示不清理。
func (m MediaSettings) Retention() time.Duration {
	if m.RetentionHours <= 0 {
		return 0
	}
	return time.Duration(m.RetentionHours * float64(time.Hour))
}

// mediaSecret 是媒体设置的敏感字段，整体加密存储。
type mediaSecret struct {
	S3SecretKey string `json:"s3_secret_key"`
}

// SourceFile / SourceDatabase 标识当前生效设置的来源。
const (
	SourceFile     = "file"
	SourceDatabase = "database"
)

// MediaReloader 在媒体设置保存后重建媒体存储实现（bootstrap 注入）。
type MediaReloader func(ms MediaSettings, s3SecretKey string) error

// MediaTester 校验一组 S3 设置的连通性（bootstrap 注入，底层走 mediastore）。
type MediaTester func(ctx context.Context, ms MediaSettings, s3SecretKey string) error

// ErrCleanupInProgress 表示同类清理任务已在执行。
var ErrCleanupInProgress = errors.New("已有清理任务正在执行")

// AIRunsCleaner 清理超过保留期的终态 AI run。
type AIRunsCleaner func(ctx context.Context, retentionDays int) (int64, error)

// MediaCleanupInput 指定手动媒体清理范围。
type MediaCleanupInput struct {
	DeleteLocal  bool
	DeleteRemote bool
}

// MediaCleanupResult 是媒体清理结果。
type MediaCleanupResult struct {
	DeletedLocalFiles    int     `json:"deleted_local_files"`
	DeletedRemoteObjects int     `json:"deleted_remote_objects"`
	RetentionHours       float64 `json:"retention_hours"`
}

// MediaCleaner 对当前生效媒体存储执行清理。
type MediaCleaner func(ctx context.Context, retention time.Duration, in MediaCleanupInput) (MediaCleanupResult, error)

// DefaultMessagesRetentionDays 是消息保留天数默认值（0=不清理）。
const DefaultMessagesRetentionDays = 90

// DefaultDeliveryTasksRetentionDays 是投递任务保留天数默认值（0=不清理）。
const DefaultDeliveryTasksRetentionDays = 90

// DefaultAIRunsRetentionDays 是 AI 运行记录保留天数默认值。
const DefaultAIRunsRetentionDays = 30

// archiveBatchSize 是单次分批删除上限，避免长锁。
const archiveBatchSize = 500

// maxArchiveRounds 是单次归档任务最多循环批次数。
const maxArchiveRounds = 100

// DataRetentionSettings 是消息与投递记录保留策略。
// 0 表示不清理；单位为天，保存后热生效。
type DataRetentionSettings struct {
	MessagesRetentionDays      int `json:"messages_retention_days"`
	DeliveryTasksRetentionDays int `json:"delivery_tasks_retention_days"`
	// AIRunsRetentionDays 是 AI 整理 run 保留天数；0=不清理。
	AIRunsRetentionDays int `json:"ai_runs_retention_days"`
}

// ArchiveResult 汇总一次归档清理的删除计数。
type ArchiveResult struct {
	DeletedMessages      int64 `json:"deleted_messages"`
	DeletedDeliveryTasks int64 `json:"deleted_delivery_tasks"`
}

// DataCleanupTargets 指定一次手动数据清理的目标。
type DataCleanupTargets struct {
	Messages      bool
	DeliveryTasks bool
	AIRuns        bool
}

// Empty 返回是否未选择任何目标。
func (t DataCleanupTargets) Empty() bool {
	return !t.Messages && !t.DeliveryTasks && !t.AIRuns
}

// DataCleanupResult 汇总手动数据清理结果和实际使用的保留策略。
type DataCleanupResult struct {
	DeletedMessages      int64                 `json:"deleted_messages"`
	DeletedDeliveryTasks int64                 `json:"deleted_delivery_tasks"`
	DeletedAIRuns        int64                 `json:"deleted_ai_runs"`
	LimitReached         []string              `json:"limit_reached"`
	Retention            DataRetentionSettings `json:"retention"`
	Source               string                `json:"-"`
}

// ArchiveStore 负责按时间窗口分批删除历史消息与终态投递任务。
type ArchiveStore interface {
	// DeleteTerminalDeliveryTasksBefore 删除 created_at < before 的终态投递任务（含 attempts 级联）。
	DeleteTerminalDeliveryTasksBefore(ctx context.Context, before time.Time, limit int) (int64, error)
	// DeleteMessagesBefore 删除 received_at < before 的消息（delivery_tasks 经 FK CASCADE）。
	DeleteMessagesBefore(ctx context.Context, before time.Time, limit int) (int64, error)
}

// Service 是系统设置应用服务。
type Service struct {
	repo         domainsettings.Repository
	fileDefaults MediaSettings
	fileSecret   string
	reload       MediaReloader
	testS3       MediaTester
	cleanMedia   MediaCleaner
	archive      ArchiveStore
	cleanAIRuns  AIRunsCleaner
	archiveMu    sync.Mutex
	now          func() time.Time
}

// NewService 创建设置服务，配置文件的 media 段作为默认值。
func NewService(repo domainsettings.Repository, fileCfg config.MediaConfig) *Service {
	return &Service{
		repo:         repo,
		fileDefaults: fromConfig(fileCfg),
		fileSecret:   fileCfg.S3.SecretKey,
		now:          time.Now,
	}
}

// SetMediaReloader 注入媒体存储热重载回调。
func (s *Service) SetMediaReloader(fn MediaReloader) { s.reload = fn }

// SetMediaTester 注入 S3 连通性测试实现。
func (s *Service) SetMediaTester(fn MediaTester) { s.testS3 = fn }

// SetMediaCleaner 注入当前媒体存储的清理实现。
func (s *Service) SetMediaCleaner(fn MediaCleaner) { s.cleanMedia = fn }

// SetArchiveStore 注入消息/投递归档存储。
func (s *Service) SetArchiveStore(store ArchiveStore) { s.archive = store }

// SetAIRunsCleaner 注入 AI run 清理实现。
func (s *Service) SetAIRunsCleaner(fn AIRunsCleaner) { s.cleanAIRuns = fn }

// EffectiveMedia 返回当前生效的媒体设置、S3 secret 明文与来源。
// 仅供 bootstrap 装配与内部重载使用，不得直接返回给 API。
func (s *Service) EffectiveMedia(ctx context.Context) (MediaSettings, string, string, error) {
	row, err := s.repo.Get(ctx, domainsettings.KeyMedia)
	if err != nil {
		return MediaSettings{}, "", "", err
	}
	if row == nil {
		return s.fileDefaults, s.fileSecret, SourceFile, nil
	}
	ms, err := decodeMedia(row.Value)
	if err != nil {
		return MediaSettings{}, "", "", err
	}
	secret, err := decodeMediaSecret(row.Secret)
	if err != nil {
		return MediaSettings{}, "", "", err
	}
	s.fillDownloadDefaults(&ms)
	return ms, secret.S3SecretKey, SourceDatabase, nil
}

// fillDownloadDefaults 为历史记录补齐下载策略：字段缺失时回退到配置文件默认值。
// FileTypes 用 nil 区分「未配置」与「用户清空表示不限类型」（空数组）。
func (s *Service) fillDownloadDefaults(ms *MediaSettings) {
	if ms.Download.ImageMaxMB <= 0 {
		ms.Download.ImageMaxMB = s.fileDefaults.Download.ImageMaxMB
	}
	if ms.Download.FileMaxMB <= 0 {
		ms.Download.FileMaxMB = s.fileDefaults.Download.FileMaxMB
	}
	if ms.Download.FileTypes == nil {
		ms.Download.FileTypes = s.fileDefaults.Download.FileTypes
	}
}

// GetMedia 返回当前生效的媒体设置（脱敏：只返回是否已配置 secret）。
func (s *Service) GetMedia(ctx context.Context) (MediaSettings, bool, string, error) {
	ms, secret, source, err := s.EffectiveMedia(ctx)
	return ms, secret != "", source, err
}

// UpdateMedia 校验并保存媒体设置，然后热重载媒体存储。
// s3SecretKey 为 nil 表示保留已有 secret。
func (s *Service) UpdateMedia(ctx context.Context, in MediaSettings, s3SecretKey *string) (MediaSettings, error) {
	normalizeMedia(&in)
	secret, err := s.resolveSecret(ctx, s3SecretKey)
	if err != nil {
		return MediaSettings{}, err
	}
	if err := validateMedia(in, secret); err != nil {
		return MediaSettings{}, err
	}

	value, err := encodeMedia(in)
	if err != nil {
		return MediaSettings{}, err
	}
	secretJSON, err := json.Marshal(mediaSecret{S3SecretKey: secret})
	if err != nil {
		return MediaSettings{}, fmt.Errorf("序列化设置 secret 失败: %w", err)
	}
	if err := s.repo.Upsert(ctx, &domainsettings.Setting{
		Key:    domainsettings.KeyMedia,
		Value:  value,
		Secret: secretJSON,
	}); err != nil {
		return MediaSettings{}, err
	}

	if s.reload != nil {
		if err := s.reload(in, secret); err != nil {
			return MediaSettings{}, fmt.Errorf("设置已保存，但媒体存储重载失败: %w", err)
		}
	}
	return in, nil
}

// TestS3 用给定设置测试 S3 连通性；s3SecretKey 为 nil 时使用已保存的 secret。
func (s *Service) TestS3(ctx context.Context, in MediaSettings, s3SecretKey *string) error {
	if s.testS3 == nil {
		return fmt.Errorf("S3 测试能力未装配")
	}
	normalizeMedia(&in)
	secret, err := s.resolveSecret(ctx, s3SecretKey)
	if err != nil {
		return err
	}
	in.S3.Enabled = true
	if err := validateMedia(in, secret); err != nil {
		return err
	}
	return s.testS3(ctx, in, secret)
}

// RunMediaCleanup 按当前已保存并生效的保留期执行一次媒体清理。
func (s *Service) RunMediaCleanup(ctx context.Context, in MediaCleanupInput) (MediaCleanupResult, error) {
	if !in.DeleteLocal && !in.DeleteRemote {
		return MediaCleanupResult{}, fmt.Errorf("至少选择一个媒体清理目标")
	}
	if s.cleanMedia == nil {
		return MediaCleanupResult{}, fmt.Errorf("媒体清理能力未装配")
	}
	ms, _, _, err := s.EffectiveMedia(ctx)
	if err != nil {
		return MediaCleanupResult{}, err
	}
	retention := ms.Retention()
	if retention <= 0 {
		return MediaCleanupResult{}, fmt.Errorf("媒体保留时长为 0，当前没有可清理的过期媒体")
	}
	if in.DeleteRemote && !ms.S3.Enabled {
		return MediaCleanupResult{}, fmt.Errorf("当前未启用 S3，不能清理远端对象")
	}
	result, err := s.cleanMedia(ctx, retention, in)
	result.RetentionHours = ms.RetentionHours
	return result, err
}

// GetDataRetention 返回当前生效的归档保留策略。
func (s *Service) GetDataRetention(ctx context.Context) (DataRetentionSettings, string, error) {
	row, err := s.repo.Get(ctx, domainsettings.KeyDataRetention)
	if err != nil {
		return DataRetentionSettings{}, "", err
	}
	if row == nil {
		return defaultDataRetention(), SourceFile, nil
	}
	dr, err := decodeDataRetention(row.Value)
	if err != nil {
		return DataRetentionSettings{}, "", err
	}
	return dr, SourceDatabase, nil
}

// UpdateDataRetention 校验并保存归档保留策略（热生效，下次定时清理读取新值）。
func (s *Service) UpdateDataRetention(ctx context.Context, in DataRetentionSettings) (DataRetentionSettings, error) {
	if err := validateDataRetention(in); err != nil {
		return DataRetentionSettings{}, err
	}
	value, err := encodeDataRetention(in)
	if err != nil {
		return DataRetentionSettings{}, err
	}
	if err := s.repo.Upsert(ctx, &domainsettings.Setting{
		Key:   domainsettings.KeyDataRetention,
		Value: value,
	}); err != nil {
		return DataRetentionSettings{}, err
	}
	return in, nil
}

// RunArchiveCleanup 按当前保留策略分批清理过期消息与终态投递任务。
// 保留天数为 0 时对应类型不清理。先清投递再清消息，避免无用的长事务。
func (s *Service) RunArchiveCleanup(ctx context.Context) (ArchiveResult, error) {
	result, err := s.runDataCleanup(ctx, DataCleanupTargets{Messages: true, DeliveryTasks: true})
	return ArchiveResult{
		DeletedMessages:      result.DeletedMessages,
		DeletedDeliveryTasks: result.DeletedDeliveryTasks,
	}, err
}

// RunDataCleanup 按当前已保存并生效的保留策略清理选中的数据目标。
func (s *Service) RunDataCleanup(ctx context.Context, targets DataCleanupTargets) (DataCleanupResult, error) {
	if targets.Empty() {
		return DataCleanupResult{}, fmt.Errorf("至少选择一个数据清理目标")
	}
	return s.runDataCleanup(ctx, targets)
}

func (s *Service) runDataCleanup(ctx context.Context, targets DataCleanupTargets) (DataCleanupResult, error) {
	var out DataCleanupResult
	if !s.archiveMu.TryLock() {
		return out, ErrCleanupInProgress
	}
	defer s.archiveMu.Unlock()

	dr, source, err := s.GetDataRetention(ctx)
	if err != nil {
		return out, err
	}
	out.Retention = dr
	out.Source = source
	now := s.now()
	if targets.DeliveryTasks && dr.DeliveryTasksRetentionDays > 0 && s.archive != nil {
		before := now.Add(-time.Duration(dr.DeliveryTasksRetentionDays) * 24 * time.Hour)
		n, limited, err := deleteInBatches(ctx, archiveBatchSize, maxArchiveRounds, func(limit int) (int64, error) {
			return s.archive.DeleteTerminalDeliveryTasksBefore(ctx, before, limit)
		})
		if err != nil {
			return out, fmt.Errorf("清理过期投递任务失败: %w", err)
		}
		out.DeletedDeliveryTasks = n
		if limited {
			out.LimitReached = append(out.LimitReached, "delivery_tasks")
		}
	}
	if targets.Messages && dr.MessagesRetentionDays > 0 && s.archive != nil {
		before := now.Add(-time.Duration(dr.MessagesRetentionDays) * 24 * time.Hour)
		n, limited, err := deleteInBatches(ctx, archiveBatchSize, maxArchiveRounds, func(limit int) (int64, error) {
			return s.archive.DeleteMessagesBefore(ctx, before, limit)
		})
		if err != nil {
			return out, fmt.Errorf("清理过期消息失败: %w", err)
		}
		out.DeletedMessages = n
		if limited {
			out.LimitReached = append(out.LimitReached, "messages")
		}
	}
	if targets.AIRuns && dr.AIRunsRetentionDays > 0 {
		if s.cleanAIRuns == nil {
			return out, fmt.Errorf("AI 运行清理能力未装配")
		}
		n, err := s.cleanAIRuns(ctx, dr.AIRunsRetentionDays)
		if err != nil {
			return out, fmt.Errorf("清理过期 AI 运行记录失败: %w", err)
		}
		out.DeletedAIRuns = n
	}
	return out, nil
}

func deleteInBatches(ctx context.Context, batch, maxRounds int, fn func(limit int) (int64, error)) (int64, bool, error) {
	var total int64
	limited := false
	for i := 0; i < maxRounds; i++ {
		if err := ctx.Err(); err != nil {
			return total, limited, err
		}
		n, err := fn(batch)
		if err != nil {
			return total, limited, err
		}
		total += n
		if n < int64(batch) {
			return total, false, nil
		}
		limited = i == maxRounds-1
	}
	return total, limited, nil
}

func defaultDataRetention() DataRetentionSettings {
	return DataRetentionSettings{
		MessagesRetentionDays:      DefaultMessagesRetentionDays,
		DeliveryTasksRetentionDays: DefaultDeliveryTasksRetentionDays,
		AIRunsRetentionDays:        DefaultAIRunsRetentionDays,
	}
}

func validateDataRetention(in DataRetentionSettings) error {
	if in.MessagesRetentionDays < 0 || in.DeliveryTasksRetentionDays < 0 || in.AIRunsRetentionDays < 0 {
		return fmt.Errorf("保留天数不能为负数（0 表示不清理）")
	}
	return nil
}

func decodeDataRetention(value map[string]any) (DataRetentionSettings, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return DataRetentionSettings{}, fmt.Errorf("解析归档设置失败: %w", err)
	}
	var dr DataRetentionSettings
	if err := json.Unmarshal(raw, &dr); err != nil {
		return DataRetentionSettings{}, fmt.Errorf("解析归档设置失败: %w", err)
	}
	// 历史记录无 ai 字段时回落默认 30 天，避免「字段缺失=0 永不清理」的意外。
	if _, ok := value["ai_runs_retention_days"]; !ok && dr.AIRunsRetentionDays == 0 {
		dr.AIRunsRetentionDays = DefaultAIRunsRetentionDays
	}
	return dr, nil
}

func encodeDataRetention(in DataRetentionSettings) (map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("序列化归档设置失败: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("序列化归档设置失败: %w", err)
	}
	return out, nil
}

// resolveSecret 处理「留空不修改」：入参非 nil 用入参，否则沿用当前生效 secret。
func (s *Service) resolveSecret(ctx context.Context, in *string) (string, error) {
	if in != nil {
		return strings.TrimSpace(*in), nil
	}
	_, secret, _, err := s.EffectiveMedia(ctx)
	return secret, err
}

func normalizeMedia(ms *MediaSettings) {
	ms.Dir = strings.TrimSpace(ms.Dir)
	if ms.Dir == "" {
		ms.Dir = "data/media"
	}
	ms.Download.FileTypes = normalizeFileTypes(ms.Download.FileTypes)
	ms.PublicBaseURL = strings.TrimSpace(strings.TrimRight(ms.PublicBaseURL, "/"))
	ms.S3.Endpoint = strings.TrimSpace(ms.S3.Endpoint)
	ms.S3.Bucket = strings.TrimSpace(ms.S3.Bucket)
	ms.S3.AccessKey = strings.TrimSpace(ms.S3.AccessKey)
	ms.S3.KeyPrefix = strings.Trim(strings.TrimSpace(ms.S3.KeyPrefix), "/")
	ms.S3.PublicBaseURL = strings.TrimSpace(strings.TrimRight(ms.S3.PublicBaseURL, "/"))
}

// normalizeFileTypes 清洗扩展名白名单：去点、小写、去重；nil 原样返回（表示未配置）。
func normalizeFileTypes(types []string) []string {
	if types == nil {
		return nil
	}
	out := make([]string, 0, len(types))
	seen := map[string]struct{}{}
	for _, t := range types {
		t = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "."))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func validateMedia(ms MediaSettings, s3Secret string) error {
	if ms.URLTTLHours < 0 || ms.RetentionHours < 0 {
		return fmt.Errorf("URL 有效期与保留时长不能为负数")
	}
	if ms.Download.ImageMaxMB < 0 || ms.Download.FileMaxMB < 0 {
		return fmt.Errorf("媒体下载大小上限不能为负数")
	}
	if ms.PublicBaseURL != "" && !strings.HasPrefix(ms.PublicBaseURL, "http://") && !strings.HasPrefix(ms.PublicBaseURL, "https://") {
		return fmt.Errorf("公网访问地址必须以 http:// 或 https:// 开头")
	}
	if !ms.S3.Enabled {
		return nil
	}
	if ms.S3.Endpoint == "" || ms.S3.Bucket == "" {
		return fmt.Errorf("启用 S3 时 endpoint 和 bucket 必填")
	}
	if ms.S3.AccessKey == "" || s3Secret == "" {
		return fmt.Errorf("启用 S3 时 access_key 和 secret_key 必填")
	}
	return nil
}

// fromConfig 把配置文件的 media 段转换为设置结构。
func fromConfig(cfg config.MediaConfig) MediaSettings {
	return MediaSettings{
		Dir:            cfg.Dir,
		PublicBaseURL:  strings.TrimRight(cfg.PublicBaseURL, "/"),
		URLTTLHours:    cfg.URLTTL.Hours(),
		RetentionHours: cfg.Retention.Hours(),
		Download: DownloadSettings{
			ImageMaxMB: cfg.Download.ImageMaxMB,
			FileMaxMB:  cfg.Download.FileMaxMB,
			FileTypes:  normalizeFileTypes(cfg.Download.FileTypes),
		},
		S3: S3Settings{
			Enabled:       cfg.S3.Enabled,
			Endpoint:      cfg.S3.Endpoint,
			Region:        cfg.S3.Region,
			Bucket:        cfg.S3.Bucket,
			AccessKey:     cfg.S3.AccessKey,
			UseSSL:        cfg.S3.UseSSL,
			KeyPrefix:     strings.Trim(cfg.S3.KeyPrefix, "/"),
			PublicBaseURL: strings.TrimRight(cfg.S3.PublicBaseURL, "/"),
			AutoCleanup:   cfg.S3.AutoCleanup,
		},
	}
}

func encodeMedia(ms MediaSettings) (map[string]any, error) {
	raw, err := json.Marshal(ms)
	if err != nil {
		return nil, fmt.Errorf("序列化媒体设置失败: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("序列化媒体设置失败: %w", err)
	}
	return out, nil
}

func decodeMedia(value map[string]any) (MediaSettings, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return MediaSettings{}, fmt.Errorf("解析媒体设置失败: %w", err)
	}
	var ms MediaSettings
	if err := json.Unmarshal(raw, &ms); err != nil {
		return MediaSettings{}, fmt.Errorf("解析媒体设置失败: %w", err)
	}
	return ms, nil
}

func decodeMediaSecret(raw []byte) (mediaSecret, error) {
	if len(raw) == 0 {
		return mediaSecret{}, nil
	}
	var sec mediaSecret
	if err := json.Unmarshal(raw, &sec); err != nil {
		return mediaSecret{}, fmt.Errorf("解析媒体设置 secret 失败: %w", err)
	}
	return sec, nil
}
