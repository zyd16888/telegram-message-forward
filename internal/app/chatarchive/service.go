// Package chatarchive 提供聊天归档的任务编排与检索用例。
//
// 归档是与 Flow 实时转发、AI 整理并列的第三条旁路：拉取结果只写归档表，
// 不走 ingest、不进 Flow、不产生投递任务。
package chatarchive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainarchive "telegram-message-forward/internal/domain/chatarchive"
	domainpeer "telegram-message-forward/internal/domain/peer"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/mediastore"
	tgsource "telegram-message-forward/internal/plugin/source/telegram"
)

var (
	// ErrInvalidInput 表示请求参数不合法。
	ErrInvalidInput = errors.New("聊天归档参数无效")
	// ErrAccountInactive 表示账号未登录。
	ErrAccountInactive = errors.New("账号未登录，无法拉取会话历史")
	// ErrJobNotCancellable 表示任务已到终态。
	ErrJobNotCancellable = errors.New("任务已结束，无法取消")
)

// Exporter 是拉取层能力（由 Telegram Source 插件实现）。
type Exporter interface {
	ExportHistory(
		ctx context.Context,
		acc *domainaccount.Account,
		opts tgsource.ExportOptions,
		visit func(context.Context, tgsource.ExportPage) error,
	) error
}

// Service 是聊天归档应用服务。
type Service struct {
	archives domainarchive.ArchiveRepository
	messages domainarchive.MessageRepository
	jobs     domainarchive.JobRepository
	accounts domainaccount.Repository
	peers    domainpeer.Repository
	exporter Exporter
	media    mediastore.Store
	clk      clock.Clock
	log      *slog.Logger

	mu      sync.Mutex
	running map[int64]context.CancelFunc
}

// NewService 创建聊天归档服务。
func NewService(
	archives domainarchive.ArchiveRepository,
	messages domainarchive.MessageRepository,
	jobs domainarchive.JobRepository,
	accounts domainaccount.Repository,
	peers domainpeer.Repository,
	exporter Exporter,
	clk clock.Clock,
	log *slog.Logger,
) *Service {
	return &Service{
		archives: archives, messages: messages, jobs: jobs,
		accounts: accounts, peers: peers, exporter: exporter,
		clk: clk, log: log, running: map[int64]context.CancelFunc{},
	}
}

// UseMediaStore 注入媒体存储；未注入时即使开启 include_media 也只保留元信息。
func (s *Service) UseMediaStore(store mediastore.Store) *Service {
	s.media = store
	return s
}

// CreateJobInput 是创建归档任务的入参。
type CreateJobInput struct {
	AccountID     int64
	PeerType      string
	PeerID        int64
	FromDate      *time.Time
	ToDate        *time.Time
	MaxMessages   int
	IncludeMedia  bool
	MediaMaxBytes int64
}

// defaultMaxMessages 是未指定条数上限时的默认硬顶，避免误点导致拉取几十万条。
const defaultMaxMessages = 50000

// CreateJob 校验入参、幂等建立归档，并在后台启动拉取任务。
func (s *Service) CreateJob(ctx context.Context, in CreateJobInput) (*domainarchive.Job, *domainarchive.Archive, error) {
	peerType := domainarchive.PeerType(strings.TrimSpace(in.PeerType))
	if !peerType.Valid() {
		return nil, nil, fmt.Errorf("%w: 不支持的会话类型 %q", ErrInvalidInput, in.PeerType)
	}
	if in.AccountID <= 0 || in.PeerID == 0 {
		return nil, nil, fmt.Errorf("%w: 缺少账号或会话标识", ErrInvalidInput)
	}
	if in.FromDate != nil && in.ToDate != nil && !in.FromDate.Before(*in.ToDate) {
		return nil, nil, fmt.Errorf("%w: 起始时间必须早于结束时间", ErrInvalidInput)
	}
	if in.MaxMessages < 0 {
		return nil, nil, fmt.Errorf("%w: 条数上限不能为负", ErrInvalidInput)
	}
	if in.MaxMessages == 0 {
		in.MaxMessages = defaultMaxMessages
	}

	acc, err := s.accounts.GetByID(ctx, in.AccountID)
	if err != nil {
		return nil, nil, err
	}
	if acc.Status != domainaccount.StatusActive {
		return nil, nil, ErrAccountInactive
	}

	archive, err := s.archives.Ensure(ctx, &domainarchive.Archive{
		AccountID: in.AccountID,
		PeerType:  peerType,
		PeerID:    in.PeerID,
		PeerName:  s.peerDisplayName(ctx, in.AccountID, peerType, in.PeerID),
	})
	if err != nil {
		return nil, nil, err
	}

	job := &domainarchive.Job{
		ArchiveID:     archive.ID,
		Status:        domainarchive.JobPending,
		FromDate:      in.FromDate,
		ToDate:        in.ToDate,
		IncludeMedia:  in.IncludeMedia,
		MediaMaxBytes: in.MediaMaxBytes,
		MaxMessages:   in.MaxMessages,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, nil, err
	}

	s.start(job, archive, acc)
	return job, archive, nil
}

// start 在后台执行任务。
//
// 刻意不继承请求 ctx：归档动辄跑几分钟到几十分钟，HTTP 请求返回后 ctx 即被取消，
// 继承会导致任务刚起步就被掐断。取消走 Cancel/cancel_requested。
func (s *Service) start(job *domainarchive.Job, archive *domainarchive.Archive, acc *domainaccount.Account) {
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[job.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			delete(s.running, job.ID)
			s.mu.Unlock()
		}()
		if err := s.execute(runCtx, job, archive, acc); err != nil {
			s.log.Error("聊天归档任务失败", "job", job.ID, "archive", archive.ID, "err", err)
		}
	}()
}

func (s *Service) execute(ctx context.Context, job *domainarchive.Job, archive *domainarchive.Archive, acc *domainaccount.Account) error {
	started := s.clk.Now()
	job.Status = domainarchive.JobRunning
	job.StartedAt = &started
	if err := s.jobs.Update(ctx, job); err != nil {
		return err
	}

	opts := tgsource.ExportOptions{
		ArchiveID:     archive.ID,
		PeerType:      archive.PeerType,
		PeerID:        archive.PeerID,
		FromDate:      job.FromDate,
		ToDate:        job.ToDate,
		MaxMessages:   job.MaxMessages,
		OffsetID:      job.CursorOffsetID,
		IncludeMedia:  job.IncludeMedia && s.media != nil,
		MediaMaxBytes: job.MediaMaxBytes,
		PersistMedia:  s.mediaPersister(archive.ID),
	}

	cancelled := false
	visit := func(ctx context.Context, page tgsource.ExportPage) error {
		if len(page.Messages) > 0 {
			if _, err := s.messages.BulkUpsert(ctx, archive.ID, page.Messages); err != nil {
				return err
			}
			job.FetchedCount += len(page.Messages)
		}
		job.MediaCount += page.MediaDownloaded
		job.CursorOffsetID = page.NextOffsetID
		if err := s.jobs.Update(ctx, job); err != nil {
			return err
		}
		// 每页边界检查一次取消：既响应本进程的 Cancel，也响应其它路径写入的
		// cancel_requested（例如服务重启后由新进程接手取消）。
		if s.cancelRequested(ctx, job.ID) {
			cancelled = true
			return context.Canceled
		}
		return nil
	}

	execErr := s.exporter.ExportHistory(ctx, acc, opts, visit)

	// 统计以实际落库数据重算，保证与幂等 upsert 一致。
	if err := s.archives.RefreshStats(ctx, archive.ID, s.clk.Now()); err != nil {
		s.log.Warn("刷新归档统计失败", "archive", archive.ID, "err", err)
	}

	finished := s.clk.Now()
	job.FinishedAt = &finished
	switch {
	case cancelled || errors.Is(execErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		job.Status = domainarchive.JobCancelled
		job.LastError = ""
	case execErr != nil:
		job.Status = domainarchive.JobFailed
		job.LastError = execErr.Error()
	default:
		job.Status = domainarchive.JobSucceeded
		job.LastError = ""
	}
	if err := s.jobs.Update(ctx, job); err != nil {
		return err
	}
	s.log.Info("聊天归档任务结束",
		"job", job.ID, "archive", archive.ID, "status", job.Status,
		"fetched", job.FetchedCount, "media", job.MediaCount,
		"resume_from", job.CursorOffsetID,
	)
	return execErr
}

// cancelRequested 读取数据库里的取消标记。读失败按未取消处理，不打断任务。
func (s *Service) cancelRequested(ctx context.Context, jobID int64) bool {
	current, err := s.jobs.GetByID(ctx, jobID)
	if err != nil || current == nil {
		return false
	}
	return current.CancelRequested
}

// mediaPersister 把下载到临时目录的媒体收编进媒体存储。
func (s *Service) mediaPersister(archiveID int64) tgsource.MediaPersister {
	if s.media == nil {
		return nil
	}
	return func(ctx context.Context, localPath string, m *domainarchive.Media) (string, error) {
		key := archiveMediaKey(archiveID, localPath)
		if _, err := s.media.PutFile(ctx, key, localPath); err != nil {
			return "", err
		}
		return key, nil
	}
}

// archiveMediaKey 用归档 id 做命名空间，避免与监听源媒体互相覆盖。
func archiveMediaKey(archiveID int64, localPath string) string {
	return fmt.Sprintf("chat-archive/%d/%s", archiveID, path.Base(strings.ReplaceAll(localPath, "\\", "/")))
}

// Cancel 请求取消任务：置位数据库标记并中断本进程正在跑的 goroutine。
func (s *Service) Cancel(ctx context.Context, jobID int64) error {
	job, err := s.jobs.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status.Terminal() {
		return ErrJobNotCancellable
	}
	if err := s.jobs.RequestCancel(ctx, jobID); err != nil {
		return err
	}
	s.mu.Lock()
	cancel := s.running[jobID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// RecoverActive 在服务启动时把上次残留的进行中任务标记为失败。
//
// 不自动续跑：续跑会在用户不知情时打 Telegram 接口。断点游标已落库，
// 用户可以再建一个任务从断点继续。
func (s *Service) RecoverActive(ctx context.Context) error {
	active, err := s.jobs.ListActive(ctx)
	if err != nil {
		return err
	}
	for _, job := range active {
		finished := s.clk.Now()
		job.Status = domainarchive.JobFailed
		job.FinishedAt = &finished
		job.LastError = "服务重启导致任务中断，可从断点重新发起"
		if err := s.jobs.Update(ctx, job); err != nil {
			s.log.Warn("回收残留归档任务失败", "job", job.ID, "err", err)
			continue
		}
		s.log.Info("已回收残留归档任务", "job", job.ID, "resume_from", job.CursorOffsetID)
	}
	return nil
}

// ListArchives 返回全部归档。
func (s *Service) ListArchives(ctx context.Context) ([]*domainarchive.Archive, error) {
	return s.archives.List(ctx)
}

// GetArchive 返回归档详情。
func (s *Service) GetArchive(ctx context.Context, id int64) (*domainarchive.Archive, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}
	return s.archives.GetByID(ctx, id)
}

// DeleteArchive 删除归档及其消息与任务。
func (s *Service) DeleteArchive(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}
	return s.archives.Delete(ctx, id)
}

// GetJob 返回任务详情。
func (s *Service) GetJob(ctx context.Context, id int64) (*domainarchive.Job, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}
	return s.jobs.GetByID(ctx, id)
}

// ListJobs 返回某归档的任务列表。
func (s *Service) ListJobs(ctx context.Context, archiveID int64, limit int) ([]*domainarchive.Job, error) {
	if archiveID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.jobs.ListByArchive(ctx, archiveID, limit)
}

// maxSearchLimit 是单次检索返回条数上限。
const maxSearchLimit = 200

// SearchMessages 检索归档消息。
func (s *Service) SearchMessages(ctx context.Context, q domainarchive.MessageQuery) ([]*domainarchive.Message, bool, error) {
	if q.ArchiveID <= 0 {
		return nil, false, ErrInvalidInput
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	if q.From != nil && q.To != nil && !q.From.Before(*q.To) {
		return nil, false, fmt.Errorf("%w: 起始时间必须早于结束时间", ErrInvalidInput)
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > maxSearchLimit {
		q.Limit = maxSearchLimit
	}
	return s.messages.Search(ctx, q)
}

// peerDisplayName 从 peer 缓存取会话展示名；取不到时返回空串，不阻断归档。
func (s *Service) peerDisplayName(ctx context.Context, accountID int64, peerType domainarchive.PeerType, peerID int64) string {
	if s.peers == nil {
		return ""
	}
	peer, err := s.peers.Get(ctx, accountID, domainpeer.Type(peerType), peerID)
	if err != nil || peer == nil {
		return ""
	}
	return peer.Title
}
