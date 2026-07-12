package repository

import (
	"testing"
	"time"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

func TestAIDigestRunAuditRoundTrip(t *testing.T) {
	run := &domainaidigest.Run{
		ID: 12, Status: domainaidigest.RunSuccess, TriggerType: domainaidigest.TriggerManual,
		WindowStart:  time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC),
		WindowEnd:    time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC),
		SystemPrompt: "system instructions",
		UserPrompt:   "rendered user prompt",
		RequestConfig: domainaidigest.RequestConfig{
			ProviderID: "provider-1", APIType: "responses", Model: "model-1", Temperature: 0.3, MaxTokens: 2048,
			Multimodal: &domainaidigest.MultimodalRequestConfig{Enabled: true, Included: 1},
		},
		MediaAudit:         []domainaidigest.MediaAudit{{MessageID: 9, MediaIndex: 0, Status: "included", SHA256: "abc"}},
		PromptMessageCount: 12,
		PromptOmittedCount: 3,
		PromptChars:        4096,
		ProfileSnapshot:    &domainaidigest.Profile{Name: "snapshot", PromptTemplate: "original"},
	}

	model, err := toAIDigestRunModel(run)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := toAIDigestRunDomain(model)
	if err != nil {
		t.Fatal(err)
	}
	if restored.SystemPrompt != run.SystemPrompt || restored.UserPrompt != run.UserPrompt {
		t.Fatalf("prompt round trip mismatch: %+v", restored)
	}
	if restored.RequestConfig.Multimodal == nil || restored.RequestConfig.Multimodal.Included != 1 {
		t.Fatalf("request config round trip mismatch: %+v", restored.RequestConfig)
	}
	if len(restored.MediaAudit) != 1 || restored.MediaAudit[0].SHA256 != "abc" {
		t.Fatalf("media audit round trip mismatch: %+v", restored.MediaAudit)
	}
	if restored.PromptMessageCount != 12 || restored.PromptOmittedCount != 3 || restored.PromptChars != 4096 {
		t.Fatalf("prompt audit round trip mismatch: %+v", restored)
	}
	if restored.ProfileSnapshot == nil || restored.ProfileSnapshot.PromptTemplate != "original" {
		t.Fatalf("profile snapshot round trip mismatch: %+v", restored.ProfileSnapshot)
	}
}
