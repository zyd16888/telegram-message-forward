package telegram

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/session"

	"telegram-message-forward/internal/infra/crypto"
)

func TestSessionStoreRoundTrip(t *testing.T) {
	cipher, err := crypto.NewCipher(bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatal(err)
	}

	var stored []byte
	store := NewSessionStore(cipher,
		func(context.Context) ([]byte, error) { return stored, nil },
		func(_ context.Context, enc []byte) error { stored = enc; return nil },
	)

	ctx := context.Background()

	// 初始无 session。
	if _, err := store.LoadSession(ctx); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("空 session 应返回 ErrNotFound, got %v", err)
	}

	plain := []byte("telegram-session-blob")
	if err := store.StoreSession(ctx, plain); err != nil {
		t.Fatal(err)
	}
	// 落库的是密文，不是明文。
	if bytes.Equal(stored, plain) {
		t.Fatal("session 必须加密落库，不能明文")
	}

	got, err := store.LoadSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("解密后 session 不一致: %q", got)
	}
}
