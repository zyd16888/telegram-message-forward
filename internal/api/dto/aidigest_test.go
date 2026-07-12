package dto

import (
	"testing"
	"time"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

func TestAIDigestRunDetailIncludesRequestAndMessageSnapshots(t *testing.T) {
	detail := &domainaidigest.RunDetail{
		Run: &domainaidigest.Run{
			ID: 7, SystemPrompt: "system", UserPrompt: "user",
			RequestConfig: domainaidigest.RequestConfig{Model: "model-1", MaxTokens: 1024},
		},
		Items: []*domainaidigest.RunItem{{
			MessageID: 9, SourceID: 3, Included: true,
			MessageSnapshot: &domainaidigest.MessageSnapshot{
				ID: 9, SourceID: 3, ExternalMessageID: 42, MessageType: "text",
				SenderName: "Alice", Text: "original", ReceivedAt: time.Now(),
			},
		}},
	}

	dto := NewAIDigestRunDetailDTO(detail)
	if dto.Request == nil || dto.Request.UserPrompt != "user" || dto.Request.RequestConfig.MaxTokens != 1024 {
		t.Fatalf("request snapshot missing: %+v", dto.Request)
	}
	if len(dto.Items) != 1 || dto.Items[0].Message == nil || dto.Items[0].Message.Text != "original" {
		t.Fatalf("message snapshot missing: %+v", dto.Items)
	}
}
