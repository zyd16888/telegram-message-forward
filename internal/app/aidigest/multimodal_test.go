package aidigest

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domainmessage "telegram-message-forward/internal/domain/message"
)

func TestBuildMultimodalContentIncludesImageAndSafeAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	groupedID := int64(88)
	message := &domainmessage.NormalizedMessage{
		ID: 42, GroupedID: &groupedID,
		Media: []domainmessage.Media{{Type: "photo", FileName: "card.png", LocalPath: path}},
	}
	profile := &domainaidigest.Profile{Multimodal: domainaidigest.MultimodalConfig{
		Enabled: true, ImageDetail: "high", MaxImagesPerRun: 2,
		MaxImageBytes: 1 << 20, MaxTotalImageBytes: 2 << 20,
	}}
	parts, audit := (&Service{}).buildMultimodalContent(context.Background(), profile, []*domainmessage.NormalizedMessage{message})
	if len(parts) != 2 || parts[1].Type != "image" || !strings.HasPrefix(parts[1].ImageURL, "data:image/png;base64,") {
		t.Fatalf("unexpected content parts: %+v", parts)
	}
	if len(audit) != 1 || audit[0].Status != "included" || len(audit[0].SHA256) != 64 {
		t.Fatalf("unexpected audit: %+v", audit)
	}
	if strings.Contains(audit[0].Reason, path) || strings.Contains(audit[0].SHA256, "base64") {
		t.Fatalf("audit must not contain local path or image payload: %+v", audit[0])
	}
}

func TestBuildMultimodalContentSkipsOversizedImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.jpg")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := &domainaidigest.Profile{Multimodal: domainaidigest.MultimodalConfig{
		Enabled: true, MaxImagesPerRun: 1, MaxImageBytes: 8, MaxTotalImageBytes: 8,
	}}
	message := &domainmessage.NormalizedMessage{ID: 7, Media: []domainmessage.Media{{
		Type: "image", MimeType: "image/jpeg", Size: 32, LocalPath: path,
	}}}
	parts, audit := (&Service{}).buildMultimodalContent(context.Background(), profile, []*domainmessage.NormalizedMessage{message})
	if len(parts) != 0 || len(audit) != 1 || audit[0].Status != "skipped" || audit[0].Reason != "超过单图字节上限" {
		t.Fatalf("unexpected result: parts=%+v audit=%+v", parts, audit)
	}
}

func TestBuildGenerateRequestUsesVisionModel(t *testing.T) {
	profile := &domainaidigest.Profile{Multimodal: domainaidigest.MultimodalConfig{Enabled: true}}
	cfg := domainaidigest.ProviderConfig{Model: "text-model", VisionModel: "vision-model"}
	request, snapshot := buildGenerateRequest(profile, cfg, "prompt")
	if request.Model != "vision-model" || snapshot.Model != "vision-model" || snapshot.Multimodal == nil {
		t.Fatalf("unexpected multimodal request: request=%+v snapshot=%+v", request, snapshot)
	}
}
