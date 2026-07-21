package dto

import (
	"fmt"

	appsettings "telegram-message-forward/internal/app/settings"
)

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

// MediaDownloadDTO 是媒体下载策略段。
type MediaDownloadDTO struct {
	ImageMaxMB float64  `json:"image_max_mb"`
	FileMaxMB  float64  `json:"file_max_mb"`
	FileTypes  []string `json:"file_types"`
}

// MediaSettingsDTO 是脱敏后的媒体设置响应。
type MediaSettingsDTO struct {
	Dir            string           `json:"dir"`
	PublicBaseURL  string           `json:"public_base_url"`
	URLTTLHours    float64          `json:"url_ttl_hours"`
	RetentionHours float64          `json:"retention_hours"`
	Download       MediaDownloadDTO `json:"download"`
	S3             MediaS3DTO       `json:"s3"`
	HasS3Secret    bool             `json:"has_s3_secret"`
	// Source 标识当前生效设置来源：database（页面已保存）或 file（配置文件默认）。
	Source string `json:"source"`
}

// NewMediaSettingsDTO 构造媒体设置响应。
func NewMediaSettingsDTO(ms appsettings.MediaSettings, hasSecret bool, source string) MediaSettingsDTO {
	fileTypes := ms.Download.FileTypes
	if fileTypes == nil {
		fileTypes = []string{}
	}
	return MediaSettingsDTO{
		Dir:            ms.Dir,
		PublicBaseURL:  ms.PublicBaseURL,
		URLTTLHours:    ms.URLTTLHours,
		RetentionHours: ms.RetentionHours,
		Download: MediaDownloadDTO{
			ImageMaxMB: ms.Download.ImageMaxMB,
			FileMaxMB:  ms.Download.FileMaxMB,
			FileTypes:  fileTypes,
		},
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
	Dir            string           `json:"dir"`
	PublicBaseURL  string           `json:"public_base_url"`
	URLTTLHours    float64          `json:"url_ttl_hours"`
	RetentionHours float64          `json:"retention_hours"`
	Download       MediaDownloadDTO `json:"download"`
	S3             MediaS3DTO       `json:"s3"`
	S3SecretKey    *string          `json:"s3_secret_key"`
}

// ToSettings 转换为应用层设置结构。
func (r MediaSettingsRequest) ToSettings() appsettings.MediaSettings {
	fileTypes := r.Download.FileTypes
	if fileTypes == nil {
		// 请求缺失该字段时按空列表处理（不限类型），避免误回退默认白名单。
		fileTypes = []string{}
	}
	return appsettings.MediaSettings{
		Dir:            r.Dir,
		PublicBaseURL:  r.PublicBaseURL,
		URLTTLHours:    r.URLTTLHours,
		RetentionHours: r.RetentionHours,
		Download: appsettings.DownloadSettings{
			ImageMaxMB: r.Download.ImageMaxMB,
			FileMaxMB:  r.Download.FileMaxMB,
			FileTypes:  fileTypes,
		},
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

// DataRetentionDTO 是消息/投递记录保留策略响应。
type DataRetentionDTO struct {
	MessagesRetentionDays      int    `json:"messages_retention_days"`
	DeliveryTasksRetentionDays int    `json:"delivery_tasks_retention_days"`
	AIRunsRetentionDays        int    `json:"ai_runs_retention_days"`
	Source                     string `json:"source"`
}

// NewDataRetentionDTO 构造归档设置响应。
func NewDataRetentionDTO(dr appsettings.DataRetentionSettings, source string) DataRetentionDTO {
	return DataRetentionDTO{
		MessagesRetentionDays:      dr.MessagesRetentionDays,
		DeliveryTasksRetentionDays: dr.DeliveryTasksRetentionDays,
		AIRunsRetentionDays:        dr.AIRunsRetentionDays,
		Source:                     source,
	}
}

// DataRetentionRequest 是保存归档保留策略的请求体。
type DataRetentionRequest struct {
	MessagesRetentionDays      int `json:"messages_retention_days"`
	DeliveryTasksRetentionDays int `json:"delivery_tasks_retention_days"`
	AIRunsRetentionDays        int `json:"ai_runs_retention_days"`
}

// ToSettings 转换为应用层结构。
func (r DataRetentionRequest) ToSettings() appsettings.DataRetentionSettings {
	return appsettings.DataRetentionSettings{
		MessagesRetentionDays:      r.MessagesRetentionDays,
		DeliveryTasksRetentionDays: r.DeliveryTasksRetentionDays,
		AIRunsRetentionDays:        r.AIRunsRetentionDays,
	}
}

// DataCleanupRequest 是手动数据清理请求。
type DataCleanupRequest struct {
	Targets []string `json:"targets"`
}

// ToTargets 校验并转换清理目标。
func (r DataCleanupRequest) ToTargets() (appsettings.DataCleanupTargets, error) {
	var out appsettings.DataCleanupTargets
	for _, target := range r.Targets {
		switch target {
		case "messages":
			out.Messages = true
		case "delivery_tasks":
			out.DeliveryTasks = true
		case "ai_runs":
			out.AIRuns = true
		default:
			return out, fmt.Errorf("未知的数据清理目标: %s", target)
		}
	}
	if out.Empty() {
		return out, fmt.Errorf("至少选择一个数据清理目标")
	}
	return out, nil
}

// DataCleanupDTO 是手动数据清理响应。
type DataCleanupDTO struct {
	DeletedMessages      int64            `json:"deleted_messages"`
	DeletedDeliveryTasks int64            `json:"deleted_delivery_tasks"`
	DeletedAIRuns        int64            `json:"deleted_ai_runs"`
	LimitReached         []string         `json:"limit_reached"`
	Retention            DataRetentionDTO `json:"retention"`
}

// NewDataCleanupDTO 构造手动数据清理响应。
func NewDataCleanupDTO(result appsettings.DataCleanupResult) DataCleanupDTO {
	limited := result.LimitReached
	if limited == nil {
		limited = []string{}
	}
	return DataCleanupDTO{
		DeletedMessages:      result.DeletedMessages,
		DeletedDeliveryTasks: result.DeletedDeliveryTasks,
		DeletedAIRuns:        result.DeletedAIRuns,
		LimitReached:         limited,
		Retention:            NewDataRetentionDTO(result.Retention, result.Source),
	}
}

// MediaCleanupRequest 是手动媒体清理请求。
type MediaCleanupRequest struct {
	DeleteLocal  bool `json:"delete_local"`
	DeleteRemote bool `json:"delete_remote"`
}

// ToInput 转换为应用层输入。
func (r MediaCleanupRequest) ToInput() appsettings.MediaCleanupInput {
	return appsettings.MediaCleanupInput{DeleteLocal: r.DeleteLocal, DeleteRemote: r.DeleteRemote}
}

// MediaCleanupDTO 是手动媒体清理响应。
type MediaCleanupDTO struct {
	DeletedLocalFiles    int     `json:"deleted_local_files"`
	DeletedRemoteObjects int     `json:"deleted_remote_objects"`
	RetentionHours       float64 `json:"retention_hours"`
}

// NewMediaCleanupDTO 构造手动媒体清理响应。
func NewMediaCleanupDTO(result appsettings.MediaCleanupResult) MediaCleanupDTO {
	return MediaCleanupDTO(result)
}
