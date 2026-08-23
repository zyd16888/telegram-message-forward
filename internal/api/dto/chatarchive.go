package dto

import (
	"time"

	domainarchive "telegram-message-forward/internal/domain/chatarchive"
)

// ChatArchiveDTO 是归档会话的对外表示。
type ChatArchiveDTO struct {
	ID           int64      `json:"id"`
	AccountID    int64      `json:"account_id"`
	PeerType     string     `json:"peer_type"`
	PeerID       int64      `json:"peer_id"`
	PeerName     string     `json:"peer_name,omitempty"`
	PeerUsername string     `json:"peer_username,omitempty"`
	MinMessageID int64      `json:"min_message_id"`
	MaxMessageID int64      `json:"max_message_id"`
	MessageCount int64      `json:"message_count"`
	MediaCount   int64      `json:"media_count"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// NewChatArchiveDTO 转换归档会话。
func NewChatArchiveDTO(a *domainarchive.Archive) ChatArchiveDTO {
	if a == nil {
		return ChatArchiveDTO{}
	}
	return ChatArchiveDTO{
		ID: a.ID, AccountID: a.AccountID, PeerType: string(a.PeerType), PeerID: a.PeerID,
		PeerName: a.PeerName, PeerUsername: a.PeerUsername,
		MinMessageID: a.MinMessageID, MaxMessageID: a.MaxMessageID,
		MessageCount: a.MessageCount, MediaCount: a.MediaCount,
		LastSyncedAt: a.LastSyncedAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// NewChatArchiveDTOs 批量转换。
func NewChatArchiveDTOs(items []*domainarchive.Archive) []ChatArchiveDTO {
	out := make([]ChatArchiveDTO, 0, len(items))
	for _, a := range items {
		out = append(out, NewChatArchiveDTO(a))
	}
	return out
}

// ChatExportJobDTO 是导出任务的对外表示。
type ChatExportJobDTO struct {
	ID        int64      `json:"id"`
	ArchiveID int64      `json:"archive_id"`
	Status    string     `json:"status"`
	FromDate  *time.Time `json:"from_date,omitempty"`
	ToDate    *time.Time `json:"to_date,omitempty"`

	IncludeMedia  bool  `json:"include_media"`
	MediaMaxBytes int64 `json:"media_max_bytes,omitempty"`
	MaxMessages   int   `json:"max_messages,omitempty"`

	FetchedCount int `json:"fetched_count"`
	MediaCount   int `json:"media_count"`
	// ResumeFrom 为非 0 表示本次未拉完，可从该消息 id 续传。
	ResumeFrom      int64      `json:"resume_from,omitempty"`
	CancelRequested bool       `json:"cancel_requested"`
	LastError       string     `json:"last_error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// NewChatExportJobDTO 转换导出任务。
func NewChatExportJobDTO(j *domainarchive.Job) ChatExportJobDTO {
	if j == nil {
		return ChatExportJobDTO{}
	}
	return ChatExportJobDTO{
		ID: j.ID, ArchiveID: j.ArchiveID, Status: string(j.Status),
		FromDate: j.FromDate, ToDate: j.ToDate,
		IncludeMedia: j.IncludeMedia, MediaMaxBytes: j.MediaMaxBytes, MaxMessages: j.MaxMessages,
		FetchedCount: j.FetchedCount, MediaCount: j.MediaCount,
		ResumeFrom: j.CursorOffsetID, CancelRequested: j.CancelRequested,
		LastError: j.LastError, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
		CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
	}
}

// NewChatExportJobDTOs 批量转换。
func NewChatExportJobDTOs(items []*domainarchive.Job) []ChatExportJobDTO {
	out := make([]ChatExportJobDTO, 0, len(items))
	for _, j := range items {
		out = append(out, NewChatExportJobDTO(j))
	}
	return out
}

// ChatArchiveMessageDTO 是归档消息的对外表示。
type ChatArchiveMessageDTO struct {
	ID        int64  `json:"id"`
	MessageID int64  `json:"message_id"`
	GroupedID *int64 `json:"grouped_id,omitempty"`
	ReplyTo   int64  `json:"reply_to_message_id,omitempty"`
	// Direction 用 in/out 表达方向，比裸布尔在前端与导出里都更可读。
	Direction      string                   `json:"direction"`
	SenderID       int64                    `json:"sender_id,omitempty"`
	SenderName     string                   `json:"sender_name,omitempty"`
	SenderUsername string                   `json:"sender_username,omitempty"`
	MessageType    string                   `json:"message_type"`
	Text           string                   `json:"text,omitempty"`
	Entities       []domainarchive.Entity   `json:"entities,omitempty"`
	Media          []domainarchive.Media    `json:"media"`
	Forward        *domainarchive.Forward   `json:"forward,omitempty"`
	Reactions      []domainarchive.Reaction `json:"reactions,omitempty"`
	ServiceAction  string                   `json:"service_action,omitempty"`
	Views          int                      `json:"views,omitempty"`
	Date           *time.Time               `json:"date,omitempty"`
	EditDate       *time.Time               `json:"edit_date,omitempty"`
}

// NewChatArchiveMessageDTO 转换归档消息。
func NewChatArchiveMessageDTO(m *domainarchive.Message) ChatArchiveMessageDTO {
	if m == nil {
		return ChatArchiveMessageDTO{}
	}
	direction := "in"
	if m.Out {
		direction = "out"
	}
	media := m.Media
	if media == nil {
		media = []domainarchive.Media{}
	}
	return ChatArchiveMessageDTO{
		ID: m.ID, MessageID: m.MessageID, GroupedID: m.GroupedID,
		ReplyTo: m.ReplyToMessageID, Direction: direction,
		SenderID: m.SenderID, SenderName: m.SenderName, SenderUsername: m.SenderUsername,
		MessageType: m.MessageType, Text: m.Text, Entities: m.Entities,
		Media: media, Forward: m.Forward, Reactions: m.Reactions,
		ServiceAction: m.ServiceAction, Views: m.Views,
		Date: m.Date, EditDate: m.EditDate,
	}
}

// NewChatArchiveMessageDTOs 批量转换。
func NewChatArchiveMessageDTOs(items []*domainarchive.Message) []ChatArchiveMessageDTO {
	out := make([]ChatArchiveMessageDTO, 0, len(items))
	for _, m := range items {
		out = append(out, NewChatArchiveMessageDTO(m))
	}
	return out
}
