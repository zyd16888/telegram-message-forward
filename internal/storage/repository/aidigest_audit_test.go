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
		},
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
	if restored.RequestConfig != run.RequestConfig {
		t.Fatalf("request config round trip mismatch: %+v", restored.RequestConfig)
	}
}
