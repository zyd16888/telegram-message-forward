package ingest

import (
	"context"
	"testing"

	"telegram-message-forward/internal/dispatch"
	domainmessage "telegram-message-forward/internal/domain/message"
	"telegram-message-forward/internal/infra/clock"
)

// cursorMessageRepo 是只关心落库成功的最小消息仓储。
type cursorMessageRepo struct{}

func (cursorMessageRepo) Create(_ context.Context, msg *domainmessage.NormalizedMessage) error {
	msg.ID = msg.ExternalMessageID
	return nil
}

func (cursorMessageRepo) ApplyEdit(context.Context, *domainmessage.NormalizedMessage) (domainmessage.EditResult, error) {
	return domainmessage.EditResult{}, nil
}

func (cursorMessageRepo) GetByID(context.Context, int64) (*domainmessage.NormalizedMessage, error) {
	return nil, nil
}

func (cursorMessageRepo) ExistsByExternalID(context.Context, int64, int64) (bool, error) {
	return false, nil
}

// recordingCursor 记录游标推进调用。
type recordingCursor struct {
	advanced []int64
}

func (c *recordingCursor) AdvanceLastMessageID(_ context.Context, _, messageID int64) error {
	c.advanced = append(c.advanced, messageID)
	return nil
}

func newCursorService(cursor *recordingCursor) *Service {
	svc := NewService(cursorMessageRepo{}, nil, nil, dispatch.NewQueue(&editTaskRepo{}, 3), clock.System{}, testLogger())
	return svc.UseSourceCursor(cursor)
}

// 实时路径必须推进游标，否则重启后会重复补拉。
func TestIngestAdvancesCursorOnRealtimePath(t *testing.T) {
	cursor := &recordingCursor{}
	svc := newCursorService(cursor)

	if err := svc.Ingest(context.Background(), &domainmessage.NormalizedMessage{
		SourceID: 1, ExternalMessageID: 42, Text: "hi",
	}); err != nil {
		t.Fatal(err)
	}

	if len(cursor.advanced) != 1 || cursor.advanced[0] != 42 {
		t.Fatalf("实时消息应推进游标到 42，实际 %v", cursor.advanced)
	}
}

// 手动回捞路径拉的是「最近 N 条」而非连续区间，推进游标会让中间未覆盖的部分
// 被自动追平永久跳过。
func TestIngestSkipsCursorForManualBackfill(t *testing.T) {
	cursor := &recordingCursor{}
	svc := newCursorService(cursor)

	if err := svc.Ingest(context.Background(), &domainmessage.NormalizedMessage{
		SourceID: 1, ExternalMessageID: 9999, Text: "old", SkipCursorAdvance: true,
	}); err != nil {
		t.Fatal(err)
	}

	if len(cursor.advanced) != 0 {
		t.Fatalf("手动回捞不应推进游标，实际 %v", cursor.advanced)
	}
}

// 断线追平走正向分页、逐条 ingest，游标必须逐条连续前进。
func TestIngestAdvancesCursorContiguouslyOnCatchUp(t *testing.T) {
	cursor := &recordingCursor{}
	svc := newCursorService(cursor)

	for _, id := range []int64{11, 12, 13} {
		if err := svc.Ingest(context.Background(), &domainmessage.NormalizedMessage{
			SourceID: 1, ExternalMessageID: id, Text: "gap",
		}); err != nil {
			t.Fatal(err)
		}
	}

	want := []int64{11, 12, 13}
	if len(cursor.advanced) != len(want) {
		t.Fatalf("游标推进次数 = %d，want %d（%v）", len(cursor.advanced), len(want), cursor.advanced)
	}
	for i, id := range want {
		if cursor.advanced[i] != id {
			t.Fatalf("游标推进序列 = %v，want %v", cursor.advanced, want)
		}
	}
}
