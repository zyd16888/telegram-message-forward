package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

const postgresExtendedProtocolParameterLimit = 65535

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

func TestAddRunItemsBatchesBeyondPostgresParameterLimit(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=test dbname=test sslmode=disable",
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	var parameterCounts []int
	if err := db.Callback().Create().After("gorm:create").Register("test:capture_parameter_count", func(tx *gorm.DB) {
		parameterCounts = append(parameterCounts, len(tx.Statement.Vars))
	}); err != nil {
		t.Fatal(err)
	}

	const itemCount = postgresExtendedProtocolParameterLimit/8 + 1
	items := make([]*domainaidigest.RunItem, 0, itemCount)
	for i := 1; i <= itemCount; i++ {
		items = append(items, &domainaidigest.RunItem{
			RunID:           1,
			MessageID:       int64(i),
			SourceID:        1,
			SortOrder:       i - 1,
			MessageSnapshot: &domainaidigest.MessageSnapshot{ID: int64(i), SourceID: 1},
		})
	}

	repo := NewAIDigestRepository(db)
	if err := repo.AddRunItems(context.Background(), items); err != nil {
		t.Fatal(err)
	}
	if len(parameterCounts) <= 1 {
		t.Fatalf("expected multiple insert batches, got parameter counts %v", parameterCounts)
	}
	for i, count := range parameterCounts {
		if count > postgresExtendedProtocolParameterLimit {
			t.Fatalf("batch %d has %d parameters, exceeds PostgreSQL limit %d", i, count, postgresExtendedProtocolParameterLimit)
		}
	}
}
