package backup

import (
	"context"
	"testing"

	domainbackup "telegram-message-forward/internal/domain/backup"
)

type fakeRepo struct{ payload []byte }

func (f *fakeRepo) ExportPayload(context.Context, bool) ([]byte, domainbackup.Manifest, error) {
	return f.payload, domainbackup.Manifest{Version: 1}, nil
}
func (f *fakeRepo) InspectPayload(_ context.Context, payload []byte) (*domainbackup.Preview, error) {
	return &domainbackup.Preview{CanRestore: string(payload) == string(f.payload)}, nil
}
func (f *fakeRepo) RestorePayload(context.Context, []byte) (*domainbackup.RestoreResult, error) {
	return &domainbackup.RestoreResult{RestartRequired: true}, nil
}

func TestArchiveRoundTrip(t *testing.T) {
	svc := NewService(&fakeRepo{payload: []byte(`{"ok":true}`)})
	archive, _, err := svc.Export(context.Background(), "strong-password", true)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Inspect(context.Background(), archive, "strong-password")
	if err != nil || !preview.CanRestore {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	if _, err := svc.Inspect(context.Background(), archive, "wrong-password"); err == nil {
		t.Fatal("wrong password should fail")
	}
	archive[len(archive)-2] ^= 1
	if _, err := svc.Inspect(context.Background(), archive, "strong-password"); err == nil {
		t.Fatal("tampered archive should fail")
	}
}
