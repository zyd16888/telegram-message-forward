// Package settings 提供页面可管理的系统设置应用服务。
//
// 设置按组存储（settings 表一组一行），页面保存的值优先于配置文件，
// 数据库无记录时回退到配置文件默认值。
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"telegram-message-forward/internal/config"
	domainsettings "telegram-message-forward/internal/domain/settings"
)

// MediaSettings 是媒体存储设置（不含敏感字段，敏感字段单独加密存储）。
type MediaSettings struct {
	Dir            string     `json:"dir"`
	PublicBaseURL  string     `json:"public_base_url"`
	URLTTLHours    float64    `json:"url_ttl_hours"`
	RetentionHours float64    `json:"retention_hours"`
	S3             S3Settings `json:"s3"`
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

// Service 是系统设置应用服务。
type Service struct {
	repo         domainsettings.Repository
	fileDefaults MediaSettings
	fileSecret   string
	reload       MediaReloader
	testS3       MediaTester
}

// NewService 创建设置服务，配置文件的 media 段作为默认值。
func NewService(repo domainsettings.Repository, fileCfg config.MediaConfig) *Service {
	return &Service{
		repo:         repo,
		fileDefaults: fromConfig(fileCfg),
		fileSecret:   fileCfg.S3.SecretKey,
	}
}

// SetMediaReloader 注入媒体存储热重载回调。
func (s *Service) SetMediaReloader(fn MediaReloader) { s.reload = fn }

// SetMediaTester 注入 S3 连通性测试实现。
func (s *Service) SetMediaTester(fn MediaTester) { s.testS3 = fn }

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
	return ms, secret.S3SecretKey, SourceDatabase, nil
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
	ms.PublicBaseURL = strings.TrimSpace(strings.TrimRight(ms.PublicBaseURL, "/"))
	ms.S3.Endpoint = strings.TrimSpace(ms.S3.Endpoint)
	ms.S3.Bucket = strings.TrimSpace(ms.S3.Bucket)
	ms.S3.AccessKey = strings.TrimSpace(ms.S3.AccessKey)
	ms.S3.KeyPrefix = strings.Trim(strings.TrimSpace(ms.S3.KeyPrefix), "/")
	ms.S3.PublicBaseURL = strings.TrimSpace(strings.TrimRight(ms.S3.PublicBaseURL, "/"))
}

func validateMedia(ms MediaSettings, s3Secret string) error {
	if ms.URLTTLHours < 0 || ms.RetentionHours < 0 {
		return fmt.Errorf("URL 有效期与保留时长不能为负数")
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
