package chatarchive

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainarchive "telegram-message-forward/internal/domain/chatarchive"
	"telegram-message-forward/internal/infra/clock"
	tgsource "telegram-message-forward/internal/plugin/source/telegram"
)

// ---- 共用 stub ----

// stubMessageRepo 按批返回预置消息，并记录写入。
type stubMessageRepo struct {
	mu       sync.Mutex
	all      []*domainarchive.Message
	upserted []*domainarchive.Message
	calls    int
}

func (r *stubMessageRepo) BulkUpsert(_ context.Context, _ int64, msgs []*domainarchive.Message) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upserted = append(r.upserted, msgs...)
	return int64(len(msgs)), nil
}

func (r *stubMessageRepo) Search(context.Context, domainarchive.MessageQuery) ([]*domainarchive.Message, bool, error) {
	return nil, false, nil
}

func (r *stubMessageRepo) CountByArchive(context.Context, int64) (int64, error) {
	return int64(len(r.all)), nil
}

func (r *stubMessageRepo) StreamPage(_ context.Context, q domainarchive.ExportQuery) ([]*domainarchive.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	out := make([]*domainarchive.Message, 0, q.Limit)
	for _, m := range r.all {
		if m.ID <= q.AfterID {
			continue
		}
		if !q.IncludeService && m.ServiceAction != "" {
			continue
		}
		out = append(out, m)
		if len(out) >= q.Limit {
			break
		}
	}
	return out, nil
}

func (r *stubMessageRepo) upsertedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.upserted)
}

type stubArchiveRepo struct {
	archive   *domainarchive.Archive
	refreshed int
}

func (r *stubArchiveRepo) Ensure(context.Context, *domainarchive.Archive) (*domainarchive.Archive, error) {
	return r.archive, nil
}
func (r *stubArchiveRepo) GetByID(context.Context, int64) (*domainarchive.Archive, error) {
	return r.archive, nil
}
func (r *stubArchiveRepo) List(context.Context) ([]*domainarchive.Archive, error) { return nil, nil }
func (r *stubArchiveRepo) Delete(context.Context, int64) error                    { return nil }
func (r *stubArchiveRepo) RefreshStats(context.Context, int64, time.Time) error {
	r.refreshed++
	return nil
}

// stubJobRepo 用内存保存单个任务。
type stubJobRepo struct {
	mu      sync.Mutex
	job     domainarchive.Job
	updates int
}

func (r *stubJobRepo) Create(_ context.Context, j *domainarchive.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	j.ID = 1
	r.job = *j
	return nil
}

func (r *stubJobRepo) Update(_ context.Context, j *domainarchive.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates++
	cancel := r.job.CancelRequested
	r.job = *j
	r.job.CancelRequested = cancel
	return nil
}

func (r *stubJobRepo) GetByID(context.Context, int64) (*domainarchive.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.job
	return &out, nil
}

func (r *stubJobRepo) ListByArchive(context.Context, int64, int) ([]*domainarchive.Job, error) {
	return nil, nil
}

func (r *stubJobRepo) ListActive(context.Context) ([]*domainarchive.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job.Status.Terminal() {
		return nil, nil
	}
	out := r.job
	return []*domainarchive.Job{&out}, nil
}

func (r *stubJobRepo) RequestCancel(context.Context, int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.job.CancelRequested = true
	return nil
}

func (r *stubJobRepo) snapshot() domainarchive.Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.job
}

// stubExporter 按预置页序列回调。
type stubExporter struct {
	pages []tgsource.ExportPage
	err   error
	opts  tgsource.ExportOptions
}

func (e *stubExporter) ExportHistory(
	ctx context.Context,
	_ *domainaccount.Account,
	opts tgsource.ExportOptions,
	visit func(context.Context, tgsource.ExportPage) error,
) error {
	e.opts = opts
	for _, page := range e.pages {
		if err := visit(ctx, page); err != nil {
			return err
		}
	}
	return e.err
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testService(msgs *stubMessageRepo, archive *domainarchive.Archive) *Service {
	return NewService(
		&stubArchiveRepo{archive: archive}, msgs, nil, nil, nil, nil,
		clock.System{}, testLogger(),
	)
}

func newMsg(id int64) *domainarchive.Message {
	t := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	return &domainarchive.Message{ID: id, MessageID: 1000 + id, Date: &t, MessageType: "text", Text: "x"}
}

// ---- 编排 ----

func runExecute(t *testing.T, exp *stubExporter) (*stubJobRepo, *stubArchiveRepo, *stubMessageRepo) {
	t.Helper()
	archive := &domainarchive.Archive{ID: 3, PeerType: domainarchive.PeerUser, PeerID: 7}
	archives := &stubArchiveRepo{archive: archive}
	msgs := &stubMessageRepo{}
	jobs := &stubJobRepo{}
	svc := NewService(archives, msgs, jobs, nil, nil, exp, clock.System{}, testLogger())

	job := &domainarchive.Job{ArchiveID: archive.ID, MaxMessages: 100}
	if err := jobs.Create(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	_ = svc.execute(context.Background(), job, archive, &domainaccount.Account{ID: 1})
	return jobs, archives, msgs
}

func TestExecutePersistsPagesAndCursor(t *testing.T) {
	exp := &stubExporter{pages: []tgsource.ExportPage{
		{Messages: []*domainarchive.Message{newMsg(1), newMsg(2)}, NextOffsetID: 900},
		{Messages: []*domainarchive.Message{newMsg(3)}, NextOffsetID: 0, Done: true},
	}}
	jobs, archives, msgs := runExecute(t, exp)

	if got := msgs.upsertedCount(); got != 3 {
		t.Fatalf("落库条数 = %d, want 3", got)
	}
	job := jobs.snapshot()
	if job.Status != domainarchive.JobSucceeded {
		t.Fatalf("状态 = %s, want succeeded", job.Status)
	}
	if job.FetchedCount != 3 {
		t.Fatalf("fetched = %d, want 3", job.FetchedCount)
	}
	// 拉到底时断点必须清零，否则会被误认为还有剩余。
	if job.CursorOffsetID != 0 {
		t.Fatalf("到底后断点应清零，实际 %d", job.CursorOffsetID)
	}
	if archives.refreshed == 0 {
		t.Fatal("结束时应重算归档统计")
	}
}

// 中途停（命中上限/翻页顶）必须保留断点，否则剩余历史会被静默丢掉。
func TestExecuteKeepsCursorWhenTruncated(t *testing.T) {
	exp := &stubExporter{pages: []tgsource.ExportPage{
		{Messages: []*domainarchive.Message{newMsg(1)}, NextOffsetID: 555, Done: true},
	}}
	jobs, _, _ := runExecute(t, exp)

	job := jobs.snapshot()
	if job.Status != domainarchive.JobSucceeded {
		t.Fatalf("状态 = %s", job.Status)
	}
	if job.CursorOffsetID != 555 {
		t.Fatalf("中途停应保留断点 555，实际 %d", job.CursorOffsetID)
	}
}

func TestExecuteMarksFailedOnError(t *testing.T) {
	exp := &stubExporter{
		pages: []tgsource.ExportPage{{Messages: []*domainarchive.Message{newMsg(1)}, NextOffsetID: 10}},
		err:   errors.New("FLOOD_WAIT"),
	}
	jobs, archives, _ := runExecute(t, exp)

	job := jobs.snapshot()
	if job.Status != domainarchive.JobFailed {
		t.Fatalf("状态 = %s, want failed", job.Status)
	}
	if job.LastError == "" {
		t.Fatal("失败应记录原因")
	}
	// 失败也要保留已拉到的进度与断点，便于续传。
	if job.CursorOffsetID != 10 || job.FetchedCount != 1 {
		t.Fatalf("失败时应保留进度: cursor=%d fetched=%d", job.CursorOffsetID, job.FetchedCount)
	}
	if archives.refreshed == 0 {
		t.Fatal("失败也应重算统计（已落库的部分要计入）")
	}
}

// 取消在页边界生效，且已拉到的部分要保留。
func TestExecuteCancelsAtPageBoundary(t *testing.T) {
	archive := &domainarchive.Archive{ID: 3, PeerType: domainarchive.PeerUser, PeerID: 7}
	archives := &stubArchiveRepo{archive: archive}
	msgs := &stubMessageRepo{}
	jobs := &stubJobRepo{}
	exp := &stubExporter{pages: []tgsource.ExportPage{
		{Messages: []*domainarchive.Message{newMsg(1)}, NextOffsetID: 700},
		{Messages: []*domainarchive.Message{newMsg(2)}, NextOffsetID: 600},
	}}
	svc := NewService(archives, msgs, jobs, nil, nil, exp, clock.System{}, testLogger())

	job := &domainarchive.Job{ArchiveID: archive.ID, MaxMessages: 100}
	_ = jobs.Create(context.Background(), job)
	// 第一页处理完就请求取消。
	_ = jobs.RequestCancel(context.Background(), job.ID)

	_ = svc.execute(context.Background(), job, archive, &domainaccount.Account{ID: 1})

	got := jobs.snapshot()
	if got.Status != domainarchive.JobCancelled {
		t.Fatalf("状态 = %s, want cancelled", got.Status)
	}
	if msgs.upsertedCount() != 1 {
		t.Fatalf("取消前已拉到的应保留，实际落库 %d 条", msgs.upsertedCount())
	}
	if got.CursorOffsetID != 700 {
		t.Fatalf("取消时应保留断点 700，实际 %d", got.CursorOffsetID)
	}
}

// 未注入 mediastore 时即使开了 include_media 也不能尝试下载。
func TestExecuteDisablesMediaWithoutStore(t *testing.T) {
	archive := &domainarchive.Archive{ID: 3, PeerType: domainarchive.PeerUser, PeerID: 7}
	exp := &stubExporter{pages: []tgsource.ExportPage{{Done: true}}}
	svc := NewService(&stubArchiveRepo{archive: archive}, &stubMessageRepo{}, &stubJobRepo{},
		nil, nil, exp, clock.System{}, testLogger())

	job := &domainarchive.Job{ID: 1, ArchiveID: archive.ID, IncludeMedia: true}
	_ = svc.execute(context.Background(), job, archive, &domainaccount.Account{ID: 1})

	if exp.opts.IncludeMedia {
		t.Fatal("未注入媒体存储时不应开启媒体下载")
	}
	if exp.opts.PersistMedia != nil {
		t.Fatal("未注入媒体存储时不应提供 PersistMedia")
	}
	if exp.opts.ArchiveID != archive.ID {
		t.Fatalf("应传入归档 id 作媒体命名空间，实际 %d", exp.opts.ArchiveID)
	}
}

func TestCreateJobValidatesInput(t *testing.T) {
	svc := testService(&stubMessageRepo{}, &domainarchive.Archive{ID: 1})
	from := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	cases := map[string]CreateJobInput{
		"非法会话类型": {AccountID: 1, PeerType: "bot", PeerID: 2},
		"缺少账号":   {PeerType: "user", PeerID: 2},
		"缺少会话":   {AccountID: 1, PeerType: "user"},
		"时间窗颠倒":  {AccountID: 1, PeerType: "user", PeerID: 2, FromDate: &from, ToDate: &to},
		"负数上限":   {AccountID: 1, PeerType: "user", PeerID: 2, MaxMessages: -1},
	}
	for name, in := range cases {
		if _, _, err := svc.CreateJob(context.Background(), in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s 应返回 ErrInvalidInput，实际 %v", name, err)
		}
	}
}

// 服务重启后残留的 running 任务要标成失败并保留断点，而不是一直挂着。
func TestRecoverActiveMarksFailed(t *testing.T) {
	jobs := &stubJobRepo{job: domainarchive.Job{
		ID: 1, ArchiveID: 2, Status: domainarchive.JobRunning, CursorOffsetID: 321,
	}}
	svc := NewService(&stubArchiveRepo{}, &stubMessageRepo{}, jobs, nil, nil, nil, clock.System{}, testLogger())

	if err := svc.RecoverActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := jobs.snapshot()
	if got.Status != domainarchive.JobFailed {
		t.Fatalf("状态 = %s, want failed", got.Status)
	}
	if got.CursorOffsetID != 321 {
		t.Fatalf("断点应保留，实际 %d", got.CursorOffsetID)
	}
	if got.LastError == "" {
		t.Fatal("应说明中断原因")
	}
}

func TestCancelRejectsTerminalJob(t *testing.T) {
	jobs := &stubJobRepo{job: domainarchive.Job{ID: 1, Status: domainarchive.JobSucceeded}}
	svc := NewService(&stubArchiveRepo{}, &stubMessageRepo{}, jobs, nil, nil, nil, clock.System{}, testLogger())

	if err := svc.Cancel(context.Background(), 1); !errors.Is(err, ErrJobNotCancellable) {
		t.Fatalf("终态任务取消应报错，实际 %v", err)
	}
}

func TestSearchMessagesClampsLimit(t *testing.T) {
	svc := testService(&stubMessageRepo{}, &domainarchive.Archive{ID: 1})
	if _, _, err := svc.SearchMessages(context.Background(), domainarchive.MessageQuery{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("缺少 archive_id 应报错")
	}
}
