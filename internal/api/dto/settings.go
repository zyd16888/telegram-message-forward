package dto

import appsettings "telegram-message-forward/internal/app/settings"

// MediaS3DTO 是媒体设置里的 S3 段（脱敏，不含 secret_key）。
type MediaS3DTO struct {
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

// MediaSettingsDTO 是脱敏后的媒体设置响应。
type MediaSettingsDTO struct {
	Dir            string     `json:"dir"`
	PublicBaseURL  string     `json:"public_base_url"`
	URLTTLHours    float64    `json:"url_ttl_hours"`
	RetentionHours float64    `json:"retention_hours"`
	S3             MediaS3DTO `json:"s3"`
	HasS3Secret    bool       `json:"has_s3_secret"`
	// Source 标识当前生效设置来源：database（页面已保存）或 file（配置文件默认）。
	Source string `json:"source"`
}

// NewMediaSettingsDTO 构造媒体设置响应。
func NewMediaSettingsDTO(ms appsettings.MediaSettings, hasSecret bool, source string) MediaSettingsDTO {
	return MediaSettingsDTO{
		Dir:            ms.Dir,
		PublicBaseURL:  ms.PublicBaseURL,
		URLTTLHours:    ms.URLTTLHours,
		RetentionHours: ms.RetentionHours,
		S3: MediaS3DTO{
			Enabled:       ms.S3.Enabled,
			Endpoint:      ms.S3.Endpoint,
			Region:        ms.S3.Region,
			Bucket:        ms.S3.Bucket,
			AccessKey:     ms.S3.AccessKey,
			UseSSL:        ms.S3.UseSSL,
			KeyPrefix:     ms.S3.KeyPrefix,
			PublicBaseURL: ms.S3.PublicBaseURL,
			AutoCleanup:   ms.S3.AutoCleanup,
		},
		HasS3Secret: hasSecret,
		Source:      source,
	}
}

// MediaSettingsRequest 是保存/测试媒体设置的请求体。
// S3SecretKey 为 null 表示保留已有 secret；空串表示清除。
type MediaSettingsRequest struct {
	Dir            string     `json:"dir"`
	PublicBaseURL  string     `json:"public_base_url"`
	URLTTLHours    float64    `json:"url_ttl_hours"`
	RetentionHours float64    `json:"retention_hours"`
	S3             MediaS3DTO `json:"s3"`
	S3SecretKey    *string    `json:"s3_secret_key"`
}

// ToSettings 转换为应用层设置结构。
func (r MediaSettingsRequest) ToSettings() appsettings.MediaSettings {
	return appsettings.MediaSettings{
		Dir:            r.Dir,
		PublicBaseURL:  r.PublicBaseURL,
		URLTTLHours:    r.URLTTLHours,
		RetentionHours: r.RetentionHours,
		S3: appsettings.S3Settings{
			Enabled:       r.S3.Enabled,
			Endpoint:      r.S3.Endpoint,
			Region:        r.S3.Region,
			Bucket:        r.S3.Bucket,
			AccessKey:     r.S3.AccessKey,
			UseSSL:        r.S3.UseSSL,
			KeyPrefix:     r.S3.KeyPrefix,
			PublicBaseURL: r.S3.PublicBaseURL,
			AutoCleanup:   r.S3.AutoCleanup,
		},
	}
}
