package telegram

import (
	"errors"
	"testing"

	"github.com/gotd/td/tgerr"
)

func TestClassifyLoginErrSessionDuplicated(t *testing.T) {
	err := classifyLoginErr(tgerr.New(406, "AUTH_KEY_DUPLICATED"))
	if !errors.Is(err, ErrSessionDuplicated) {
		t.Fatalf("应映射为 ErrSessionDuplicated，实际 %v", err)
	}
}
