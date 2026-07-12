package dto

import (
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
)

type MessageDeliveryCountsDTO struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Success    int64 `json:"success"`
	Failed     int64 `json:"failed"`
	Retrying   int64 `json:"retrying"`
	Dead       int64 `json:"dead"`
	Cancelled  int64 `json:"cancelled"`
}

type MessageDTO struct {
	ID                int64                    `json:"id"`
	SourceID          int64                    `json:"source_id"`
	SourceName        string                   `json:"source_name"`
	SourceType        string                   `json:"source_type"`
	SourceUsername    string                   `json:"source_username,omitempty"`
	ExternalMessageID int64                    `json:"external_message_id"`
	GroupedID         *int64                   `json:"grouped_id,omitempty"`
	MessageType       string                   `json:"message_type"`
	SenderPeerType    string                   `json:"sender_peer_type,omitempty"`
	SenderID          int64                    `json:"sender_id,omitempty"`
	SenderName        string                   `json:"sender_name,omitempty"`
	Text              string                   `json:"text,omitempty"`
	Media             []domainmessage.Media    `json:"media"`
	Links             []domainmessage.Link     `json:"links"`
	OriginalURL       string                   `json:"original_url,omitempty"`
	SentAt            *time.Time               `json:"sent_at,omitempty"`
	ReceivedAt        time.Time                `json:"received_at"`
	CreatedAt         time.Time                `json:"created_at"`
	Deliveries        MessageDeliveryCountsDTO `json:"deliveries"`
}

type MessageDeliveryRefDTO struct {
	ID           int64     `json:"id"`
	SinkID       int64     `json:"sink_id"`
	SinkName     string    `json:"sink_name"`
	SinkType     string    `json:"sink_type"`
	OriginType   string    `json:"origin_type"`
	OriginID     int64     `json:"origin_id,omitempty"`
	OriginNodeID int64     `json:"origin_node_id,omitempty"`
	FlowName     string    `json:"flow_name,omitempty"`
	Status       string    `json:"status"`
	AttemptCount int       `json:"attempt_count"`
	MaxAttempts  int       `json:"max_attempts"`
	LastError    string    `json:"last_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MessageDetailDTO struct {
	Message    MessageDTO              `json:"message"`
	Deliveries []MessageDeliveryRefDTO `json:"deliveries"`
}

func NewMessageDTO(item *domainmessage.ListItem) MessageDTO {
	m := item.Message
	media := m.Media
	if media == nil {
		media = []domainmessage.Media{}
	}
	links := m.Links
	if links == nil {
		links = []domainmessage.Link{}
	}
	return MessageDTO{
		ID: m.ID, SourceID: m.SourceID, SourceName: item.SourceName, SourceType: item.SourceType,
		SourceUsername: item.SourceUsername, ExternalMessageID: m.ExternalMessageID, GroupedID: m.GroupedID,
		MessageType: m.MessageType, SenderPeerType: m.SenderPeerType, SenderID: m.SenderID,
		SenderName: m.SenderName, Text: m.Text, Media: media, Links: links, OriginalURL: m.OriginalURL,
		SentAt: m.SentAt, ReceivedAt: m.ReceivedAt, CreatedAt: m.CreatedAt,
		Deliveries: MessageDeliveryCountsDTO{
			Total: item.Deliveries.Total, Pending: item.Deliveries.Pending, Processing: item.Deliveries.Processing,
			Success: item.Deliveries.Success, Failed: item.Deliveries.Failed, Retrying: item.Deliveries.Retrying,
			Dead: item.Deliveries.Dead, Cancelled: item.Deliveries.Cancelled,
		},
	}
}

func NewMessageDetailDTO(detail *domainmessage.Detail) MessageDetailDTO {
	deliveries := make([]MessageDeliveryRefDTO, 0, len(detail.Deliveries))
	for _, item := range detail.Deliveries {
		deliveries = append(deliveries, MessageDeliveryRefDTO{
			ID: item.ID, SinkID: item.SinkID, SinkName: item.SinkName, SinkType: item.SinkType,
			OriginType: item.OriginType, OriginID: item.OriginID, OriginNodeID: item.OriginNodeID,
			FlowName: item.FlowName, Status: item.Status, AttemptCount: item.AttemptCount,
			MaxAttempts: item.MaxAttempts, LastError: item.LastError,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	}
	return MessageDetailDTO{Message: NewMessageDTO(detail.Item), Deliveries: deliveries}
}
