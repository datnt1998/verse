package scan

// Test đơn vị worker với kho giả: phân loại lỗi ghi bền (5 lần lỗi liên tiếp,
// giới hạn khởi động lại cursor, bị chặn), thăm dò giờ đăng bị chặn, và mất
// lease worker giữa phiên. Không DB, không mạng, không phụ thuộc thời gian thật
// (trừ heartbeat 5 ms).

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// stateStore giả cho một kênh/việc duy nhất.
type stateStore struct {
	Store
	state      State
	job        *Job
	backoffs   []Backoff
	errs       []JobError
	pauses     []time.Time
	uploads    []UploadCandidate
	probeCalls int
	released   int
}

func (f *stateStore) EnsureState(context.Context, string) (State, error) { return f.state, nil }
func (f *stateStore) FindKnownVideoIDs(context.Context, string, []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (f *stateStore) SetJobBackoff(_ context.Context, _ string, _ Lease, b Backoff) error {
	f.job.Attempts = b.Attempts
	f.job.State = domain.ScanBackoff
	f.backoffs = append(f.backoffs, b)
	return nil
}
func (f *stateStore) SetJobError(_ context.Context, _ string, _ Lease, e JobError) error {
	f.job.State = domain.ScanError
	f.job.ErrorCode = e.ErrorCode
	f.job.Recovery = e.Recovery
	f.job.LastError = SanitizeError(e.Err)
	if e.Attempts != nil {
		f.job.Attempts = *e.Attempts
	}
	f.errs = append(f.errs, e)
	return nil
}
func (f *stateStore) SetJobPaused(_ context.Context, _ string, _ Lease, e JobError) error {
	f.job.State = domain.ScanPaused
	f.job.ErrorCode = e.ErrorCode
	f.job.Recovery = e.Recovery
	f.errs = append(f.errs, e)
	return nil
}
func (f *stateStore) RestartJobCursor(context.Context, string, Lease, error) error {
	f.job.Restarts++
	f.job.State = domain.ScanQueued
	f.job.Cursor = ""
	return nil
}
func (f *stateStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	f.probeCalls++
	if len(f.uploads) == 0 {
		return nil, nil
	}
	c := f.uploads[0]
	f.uploads = f.uploads[1:]
	return &c, nil
}
func (f *stateStore) AcquireChannelLease(context.Context, string, string, time.Duration) (*Lease, error) {
	return &Lease{ChannelID: "c", Token: "00000000-0000-4000-8000-000000000001", Owner: "test", Until: time.Now().Add(time.Minute)}, nil
}
func (f *stateStore) ReleaseChannelLease(context.Context, Lease) error { f.released++; return nil }
func (f *stateStore) PauseUploads(_ context.Context, until time.Time) error {
	f.pauses = append(f.pauses, until)
	return nil
}
func (f *stateStore) RecordUploadTime(context.Context, Lease, string, *time.Time, bool, time.Time) error {
	return nil
}
func (f *stateStore) ResetUploadFailures(context.Context) error { return nil }
func (f *stateStore) BumpUploadFailures(context.Context) (int, error) {
	return 1, nil
}

// gate giả đếm lần báo chặn nền tảng.
type gate struct {
	blocked []time.Duration
	paused  *time.Time
}

func (g *gate) Until(context.Context, domain.PlatformID) (*time.Time, error) { return g.paused, nil }
func (g *gate) RecordBlocked(_ context.Context, id domain.PlatformID, retryAfter time.Duration) error {
	if id != domain.PlatformYouTube {
		panic("chặn nhầm nền tảng: " + string(id))
	}
	g.blocked = append(g.blocked, retryAfter)
	return nil
}

// errProvider trả lỗi cố định cho mọi lời gọi.
type errProvider struct {
	Provider
	err error
}

func (p errProvider) FirstChannelPage(context.Context, string) (*youtube.ChannelVideoPage, error) {
	return nil, p.err
}
func (p errProvider) NextChannelPage(context.Context, youtube.ChannelCursor) (*youtube.ChannelVideoPage, error) {
	return nil, p.err
}
func (p errProvider) FetchUploadTime(context.Context, string) (*time.Time, bool, error) {
	return nil, false, p.err
}

func baseJob(kind domain.ScanJobKind) *Job {
	return &Job{ID: "job", ChannelID: "c", Kind: kind, State: domain.ScanRunning}
}

func TestWorkerTransientFailureSeriesEndsInError(t *testing.T) {
	store := &stateStore{state: State{ChannelID: "c", HistoryCap: 150}, job: baseJob(domain.ScanDiscovery)}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{err: &youtube.FetchError{Kind: youtube.KindTransient, Message: "mạng"}}})
	for i := 1; i <= MaxConsecutiveFailure; i++ {
		if err := w.runOnePage(context.Background(), store.job, Lease{ChannelID: "c", Token: "t"}); err != nil {
			t.Fatalf("lượt %d: runOnePage = %v", i, err)
		}
		if i < MaxConsecutiveFailure && store.job.State != domain.ScanBackoff {
			t.Fatalf("lượt %d: state = %v, muốn backoff", i, store.job.State)
		}
	}
	if store.job.State != domain.ScanError {
		t.Fatalf("state = %v, muốn error", store.job.State)
	}
	if store.job.Recovery != RecoveryFailures || store.job.Attempts != MaxConsecutiveFailure {
		t.Fatalf("recovery/attempts = %q/%d, muốn %q/%d",
			store.job.Recovery, store.job.Attempts, RecoveryFailures, MaxConsecutiveFailure)
	}
	if len(store.backoffs) != MaxConsecutiveFailure-1 || len(store.errs) != 1 {
		t.Fatalf("backoff/error = %d/%d, muốn %d/1", len(store.backoffs), len(store.errs), MaxConsecutiveFailure-1)
	}
}

func TestWorkerCursorRestartLimitPausesJob(t *testing.T) {
	store := &stateStore{state: State{ChannelID: "c", HistoryCap: 150}, job: baseJob(domain.ScanHistory)}
	store.job.Cursor = "opaque"
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{err: &youtube.FetchError{Kind: youtube.KindCursorInvalid, Message: "cursor hỏng"}}})
	for i := 0; i <= MaxCursorRestarts; i++ {
		if err := w.runOnePage(context.Background(), store.job, Lease{ChannelID: "c", Token: "t"}); err != nil {
			t.Fatalf("lượt %d: runOnePage = %v", i, err)
		}
	}
	if store.job.State != domain.ScanPaused || store.job.Restarts != MaxCursorRestarts {
		t.Fatalf("state/restarts = %v/%d, muốn paused/%d", store.job.State, store.job.Restarts, MaxCursorRestarts)
	}
	if store.job.Recovery != RecoveryRestarts || store.job.ErrorCode != "cursor_restart_limit" {
		t.Fatalf("recovery/code = %q/%q", store.job.Recovery, store.job.ErrorCode)
	}
}

func TestWorkerBlockedRecordsPlatformPause(t *testing.T) {
	store := &stateStore{state: State{ChannelID: "c", HistoryCap: 150}, job: baseJob(domain.ScanDiscovery)}
	g := &gate{}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{err: &youtube.FetchError{Kind: youtube.KindBlocked, Message: "bot"}}, Platform: g})
	if err := w.runOnePage(context.Background(), store.job, Lease{ChannelID: "c", Token: "t"}); err != nil {
		t.Fatalf("runOnePage = %v", err)
	}
	if len(g.blocked) != 1 || g.blocked[0] != 0 {
		t.Fatalf("RecordBlocked = %v, muốn một lần không Retry-After", g.blocked)
	}
	if store.job.State != domain.ScanError || store.job.Recovery != RecoveryBlocked || store.job.ErrorCode != "blocked" {
		t.Fatalf("job = %v/%q/%q, muốn error/%q/blocked", store.job.State, store.job.Recovery, store.job.ErrorCode, RecoveryBlocked)
	}
}

func TestWorkerProbeBlockedPausesUploadsTwoHours(t *testing.T) {
	store := &stateStore{uploads: []UploadCandidate{{VideoID: "dQw4w9WgXcQ", ChannelID: "c"}}}
	g := &gate{}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{err: &youtube.FetchError{Kind: youtube.KindBlocked, Message: "bot"}}, Platform: g})
	begin := time.Now()
	done, err := w.probeUploadTime(context.Background())
	if err != nil {
		t.Fatalf("probeUploadTime = %v", err)
	}
	if done {
		t.Fatal("thăm dò bị chặn không được coi là đã đọc")
	}
	if len(g.blocked) != 1 {
		t.Fatalf("RecordBlocked = %v", g.blocked)
	}
	if len(store.pauses) != 1 {
		t.Fatalf("PauseUploads = %v", store.pauses)
	}
	if got := store.pauses[0].Sub(begin); got < UploadBlockedPause || got > UploadBlockedPause+10*time.Second {
		t.Fatalf("giờ nghỉ thăm dò = %v, muốn khoảng %v", got, UploadBlockedPause)
	}
}

// leaseStore giả cho vòng acquire/heartbeat/release.
type leaseStore struct {
	Store
	mu        sync.Mutex
	renews    int
	releases  int
	acquired  int
	nextJob   *Job
	nextCh    *Job
	deadlines int
}

func (f *leaseStore) AcquireWorkerLease(context.Context, string, time.Duration) (*WorkerLease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acquired++
	return &WorkerLease{Owner: "owner", Until: time.Now().Add(time.Minute)}, nil
}
func (f *leaseStore) RenewWorkerLease(context.Context, WorkerLease, time.Duration) (*WorkerLease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.renews++
	return nil, nil // mất lease
}
func (f *leaseStore) ReleaseWorkerLease(context.Context, WorkerLease) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases++
	return nil
}
func (f *leaseStore) NextRunnableJob(context.Context, map[string]bool) (*Job, error) {
	return f.nextJob, nil
}
func (f *leaseStore) NextRunnableJobForChannel(context.Context, string) (*Job, error) {
	return f.nextCh, nil
}
func (f *leaseStore) NextWakeup(context.Context, int, time.Time) (Wakeup, error) {
	f.deadlines++
	return Wakeup{}, nil
}
func (f *leaseStore) DueChannels(context.Context, int, time.Time) ([]string, error) { return nil, nil }
func (f *leaseStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	return nil, nil
}

// TestWorkerOnLostLeaseReleasesAndReacquires: mất lease thì dừng phiên hiện tại,
// nhả lease và quay lại nhận lease — không kết thúc vòng đời worker.
func TestWorkerOnLostLeaseReleasesAndReacquires(t *testing.T) {
	store := &leaseStore{}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{}, Heartbeat: 5 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	waitFor(t, "nhả lease + nhận lại", func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return store.releases >= 1 && store.acquired >= 2
	})
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run sau khi ctx huỷ = %v, muốn nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker không dừng sau khi ctx huỷ")
	}
}

// ── heartbeat không được sửa lease mà trang/thăm dò đang đọc ──────────

// raceStore cài đủ mặt worker dùng trong một trang discovery + heartbeat.
type raceStore struct {
	Store
	job    *Job
	state  State
	lease  Lease
	mu     sync.Mutex
	renews int
}

func (r *raceStore) EnsureState(context.Context, string) (State, error) { return r.state, nil }
func (r *raceStore) FindKnownVideoIDs(context.Context, string, []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (r *raceStore) CommitPage(context.Context, CommitPageInput) (CommitPageResult, error) {
	return CommitPageResult{Scanned: 1, Added: 1, JobID: "job"}, nil
}
func (r *raceStore) CompleteHeadCheck(context.Context, string, HeadCheckInput) (HeadCheckResult, error) {
	return HeadCheckResult{}, nil
}
func (r *raceStore) Job(context.Context, string) (*Job, error) { return r.job, nil }
func (r *raceStore) RenewWorkerLease(context.Context, WorkerLease, time.Duration) (*WorkerLease, error) {
	return &WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil
}
func (r *raceStore) RenewChannelLease(_ context.Context, l Lease) (*Lease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renews++
	next := l
	next.Until = l.Until.Add(time.Minute)
	return &next, nil
}
func (r *raceStore) AcquireChannelLease(_ context.Context, channelID, owner string, ttl time.Duration) (*Lease, error) {
	return &Lease{ChannelID: channelID, Token: "00000000-0000-4000-8000-000000000001", Owner: owner, Until: time.Now().Add(ttl)}, nil
}
func (r *raceStore) ReleaseChannelLease(context.Context, Lease) error { return nil }
func (r *raceStore) RecordUploadTime(context.Context, Lease, string, *time.Time, bool, time.Time) error {
	return nil
}
func (r *raceStore) ResetUploadFailures(context.Context) error       { return nil }
func (r *raceStore) BumpUploadFailures(context.Context) (int, error) { return 1, nil }
func (r *raceStore) PauseUploads(context.Context, time.Time) error   { return nil }
func (r *raceStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	return &UploadCandidate{VideoID: "dQw4w9WgXcQ", ChannelID: "c"}, nil
}

// blockingList giữ lời gọi trang đầu lại cho tới khi test mở khoá.
type blockingList struct {
	Provider
	page    *youtube.ChannelVideoPage
	started chan struct{}
	release chan struct{}
}

func (p *blockingList) FirstChannelPage(context.Context, string) (*youtube.ChannelVideoPage, error) {
	close(p.started)
	<-p.release
	return p.page, nil
}

// blockingProbe giữ thăm dò giờ đăng lại cho tới khi test mở khoá.
type blockingProbe struct {
	Provider
	started chan struct{}
	release chan struct{}
}

func (p *blockingProbe) FetchUploadTime(context.Context, string) (*time.Time, bool, error) {
	close(p.started)
	<-p.release
	return nil, true, nil
}

func startHeartbeat(t *testing.T, w *Worker, ctx context.Context) (context.CancelFunc, <-chan struct{}) {
	t.Helper()
	hbCtx, cancelCause := context.WithCancelCause(ctx)
	watch := newLeaseWatch(cancelCause)
	watch.arm(time.Now().Add(time.Minute))
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.heartbeat(hbCtx, cancelCause, WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil, watch)
	}()
	return func() { cancelCause(nil) }, done
}

// TestRaceHeartbeatDoesNotTouchPageLease: -race phải sạch khi nhịp gia hạn chạy
// chồng lên lúc trang đọc lease của mình. Trước bản sửa, heartbeat ghi qua đúng
// struct mà CommitPageInput đang đọc.
func TestRaceHeartbeatDoesNotTouchPageLease(t *testing.T) {
	store := &raceStore{job: &Job{ID: "job", ChannelID: "c", Kind: domain.ScanDiscovery, State: domain.ScanRunning}, state: State{ChannelID: "c", HistoryCap: 150}}
	provider := &blockingList{
		page:    &youtube.ChannelVideoPage{Videos: []youtube.ScannedVideo{{ID: "dQw4w9WgXcQ"}}},
		started: make(chan struct{}), release: make(chan struct{}),
	}
	w := NewWorker(WorkerDeps{Store: store, YouTube: provider, Heartbeat: time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	lease := Lease{ChannelID: "c", Token: "t", Owner: "test", Until: time.Now().Add(time.Minute)}
	errCh := make(chan error, 1)
	go func() { errCh <- w.runOnePage(ctx, store.job, lease) }()
	<-provider.started
	hbCancel, hbDone := startHeartbeat(t, w, ctx)
	time.Sleep(25 * time.Millisecond) // ≥ 20 nhịp heartbeat chồng lên fetch
	close(provider.release)
	if err := <-errCh; err != nil {
		t.Fatalf("runOnePage = %v", err)
	}
	hbCancel()
	<-hbDone
	store.mu.Lock()
	n := store.renews
	store.mu.Unlock()
	if n == 0 {
		t.Fatal("heartbeat không gia hạn kênh trong lúc fetch (test không tạo được chồng lấn)")
	}
}

// TestRaceHeartbeatDoesNotTouchProbeLease: như trên cho thăm dò giờ đăng.
func TestRaceHeartbeatDoesNotTouchProbeLease(t *testing.T) {
	store := &raceStore{}
	provider := &blockingProbe{started: make(chan struct{}), release: make(chan struct{})}
	w := NewWorker(WorkerDeps{Store: store, YouTube: provider, Heartbeat: time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		_, err := w.probeUploadTime(ctx)
		errCh <- err
	}()
	<-provider.started
	hbCancel, hbDone := startHeartbeat(t, w, ctx)
	time.Sleep(25 * time.Millisecond)
	close(provider.release)
	if err := <-errCh; err != nil {
		t.Fatalf("probeUploadTime = %v", err)
	}
	hbCancel()
	<-hbDone
}

// ── inline không spin khi kênh đang bị giữ fence ─────────────────────

// busyInline giữ fence worker nhưng kênh luôn bận; đếm số lần acquire.
type busyInline struct {
	Store
	mu       sync.Mutex
	acquires int
	claims   int
	job      *Job
}

func (b *busyInline) AcquireWorkerLease(context.Context, string, time.Duration) (*WorkerLease, error) {
	return &WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil
}
func (b *busyInline) RenewWorkerLease(context.Context, WorkerLease, time.Duration) (*WorkerLease, error) {
	return &WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil
}
func (b *busyInline) ReleaseWorkerLease(context.Context, WorkerLease) error { return nil }
func (b *busyInline) NextRunnableJobForChannel(context.Context, string) (*Job, error) {
	return b.job, nil
}
func (b *busyInline) AcquireChannelLease(context.Context, string, string, time.Duration) (*Lease, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.acquires++
	return nil, nil
}
func (b *busyInline) ClaimJob(context.Context, string, Lease) (*Job, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.claims++
	return nil, nil
}

// TestInlineBusyChannelWaitsWithoutSpin: fence kênh bận thì mỗi lần thử phải
// cách nhau một giấc ngủ có kiểm soát, không quay vòng claim liên tiếp.
func TestInlineBusyChannelWaitsWithoutSpin(t *testing.T) {
	store := &busyInline{job: &Job{ID: "job", ChannelID: "c", Kind: domain.ScanHistory, State: domain.ScanQueued}}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{}, BusyRetry: 40 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	ran, err := w.RunInline(ctx, "c")
	elapsed := time.Since(start)
	if err != nil || !ran {
		t.Fatalf("RunInline = %v, %v; muốn ran=true (đã giữ lease worker)", ran, err)
	}
	store.mu.Lock()
	acquires, claims := store.acquires, store.claims
	store.mu.Unlock()
	if claims != 0 {
		t.Fatalf("claim chạy dù fence kênh bận: %d lần", claims)
	}
	if acquires < 2 || acquires > 6 {
		t.Fatalf("số lần acquire = %d trong %v; muốn 2–6 lần cách nhau 40ms (không spin)", acquires, elapsed)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("RunInline trở về sau %v — không chờ fence", elapsed)
	}
}

// ── hạn lease worker (độc lập nhịp heartbeat) ────────────────────────────────

// runnerStore đủ mặt cho một vòng Run: một việc discovery, fetch bị chặn.
type runnerStore struct {
	Store
	mu         sync.Mutex
	until      time.Time
	blockRenew bool
	renewCalls int
	renewLimit time.Time // deadline mà lần gia hạn được cấp
	job        *Job
	jobGiven   bool
	nextUntil  time.Time
	commits    int
	released   int
	leases     int
	acquires   int
}

func (r *runnerStore) AcquireWorkerLease(context.Context, string, time.Duration) (*WorkerLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acquires++
	until := r.until
	if r.acquires > 1 {
		if r.nextUntil.IsZero() {
			return nil, nil // sau hết hạn: process khác đang giữ lease
		}
		until = r.nextUntil
	}
	return &WorkerLease{Owner: "o", Until: until}, nil
}
func (r *runnerStore) RenewWorkerLease(ctx context.Context, lease WorkerLease, _ time.Duration) (*WorkerLease, error) {
	r.mu.Lock()
	r.renewCalls++
	if dl, ok := ctx.Deadline(); ok {
		r.renewLimit = dl
	}
	block := r.blockRenew
	r.mu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, errors.New("renew tạm lỗi")
}
func (r *runnerStore) ReleaseWorkerLease(context.Context, WorkerLease) error {
	r.mu.Lock()
	r.released++
	r.mu.Unlock()
	return nil
}
func (r *runnerStore) DueChannels(context.Context, int, time.Time) ([]string, error) {
	return nil, nil
}
func (r *runnerStore) NextRunnableJob(context.Context, map[string]bool) (*Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.jobGiven {
		return nil, nil
	}
	r.jobGiven = true
	return r.job, nil
}
func (r *runnerStore) AcquireChannelLease(context.Context, string, string, time.Duration) (*Lease, error) {
	r.mu.Lock()
	r.leases++
	r.mu.Unlock()
	return &Lease{ChannelID: "c", Token: "00000000-0000-4000-8000-000000000001", Owner: "test", Until: time.Now().Add(time.Minute)}, nil
}
func (r *runnerStore) ClaimJob(context.Context, string, Lease) (*Job, error) {
	return r.job, nil
}
func (r *runnerStore) ReleaseChannelLease(context.Context, Lease) error { return nil }
func (r *runnerStore) EnsureState(context.Context, string) (State, error) {
	return State{ChannelID: "c", HistoryCap: 150}, nil
}
func (r *runnerStore) CommitPage(context.Context, CommitPageInput) (CommitPageResult, error) {
	r.mu.Lock()
	r.commits++
	r.mu.Unlock()
	return CommitPageResult{Scanned: 1, Added: 1}, nil
}
func (r *runnerStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	return nil, nil
}
func (r *runnerStore) NextWakeup(context.Context, int, time.Time) (Wakeup, error) {
	return Wakeup{}, nil
}

// ctxBlocking giữ trang đầu tới khi ctx huỷ (fetch đang bay).
type ctxBlocking struct {
	Provider
	started  chan struct{}
	canceled atomic.Bool
}

func (p *ctxBlocking) FirstChannelPage(ctx context.Context, _ string) (*youtube.ChannelVideoPage, error) {
	close(p.started)
	<-ctx.Done()
	p.canceled.Store(true)
	return nil, ctx.Err()
}

func newRunner(t *testing.T, store *runnerStore, provider Provider, heartbeat time.Duration) *Worker {
	t.Helper()
	return NewWorker(WorkerDeps{Store: store, YouTube: provider, Heartbeat: heartbeat})
}

func waitRun(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("worker không dừng đúng hạn")
		return nil
	}
}

// TestWorkerDeadlineExpiryCancelsFetchBetweenTicks: hạn đã biết huỷ fetch đang
// bay ngay tại hạn, không chờ nhịp heartbeat kế tiếp; phần ghi của lượt bị bỏ.
func TestWorkerDeadlineExpiryCancelsFetchBetweenTicks(t *testing.T) {
	store := &runnerStore{until: time.Now().Add(60 * time.Millisecond), job: &Job{ID: "job", ChannelID: "c", Kind: domain.ScanDiscovery, State: domain.ScanQueued}}
	provider := &ctxBlocking{started: make(chan struct{})}
	w := newRunner(t, store, provider, 300*time.Millisecond) // tick xa hơn hạn
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- w.Run(ctx) }()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("fetch không bắt đầu")
	}
	waitFor(t, "fetch bị huỷ tại hạn lease", provider.canceled.Load)
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		cancel()
		t.Fatalf("huỷ sau %v — đã chờ nhịp heartbeat thay vì hạn lease", elapsed)
	}
	store.mu.Lock()
	renews, commits := store.renewCalls, store.commits
	store.mu.Unlock()
	if renews != 0 {
		t.Fatalf("gia hạn chạy %d lần trước hạn (test cần hạn giữa hai nhịp)", renews)
	}
	if commits != 0 {
		t.Fatal("commit sau khi hạn lease đã qua")
	}
	cancel()
	if err := waitRun(t, done); err != nil {
		t.Fatalf("Run sau khi ctx huỷ = %v, muốn nil", err)
	}
}

// TestWorkerRenewalBlockedPastExpiryStops: gia hạn bị treo bị chặn bởi hạn đang
// giữ; hết hạn là fetch bị huỷ, không chờ tick, không commit.
func TestWorkerRenewalBlockedPastExpiryStops(t *testing.T) {
	until := time.Now().Add(70 * time.Millisecond)
	store := &runnerStore{until: until, blockRenew: true, job: &Job{ID: "job", ChannelID: "c", Kind: domain.ScanDiscovery, State: domain.ScanQueued}}
	provider := &ctxBlocking{started: make(chan struct{})}
	w := newRunner(t, store, provider, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("fetch không bắt đầu")
	}
	waitFor(t, "fetch bị huỷ khi renew treo", provider.canceled.Load)
	cancel()
	if err := waitRun(t, done); err != nil {
		t.Fatalf("Run sau khi ctx huỷ = %v, muốn nil", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.renewCalls == 0 {
		t.Fatal("test không tạo được lần gia hạn bị treo")
	}
	if store.renewLimit.After(until.Add(5 * time.Millisecond)) {
		t.Fatalf("gia hạn được cấp hạn %v, vượt hạn lease đang giữ %v", store.renewLimit, until)
	}
	if store.commits != 0 {
		t.Fatal("commit sau khi hết hạn")
	}
	if store.released != 1 {
		t.Fatalf("ReleaseWorkerLease = %d, muốn 1 (dừng sạch, quay lại nhận lease)", store.released)
	}
}

// TestWorkerReacquiresLeaseAfterExpiry: hết hạn không kết thúc worker — nó
// quay lại nhận lease (lease đầu 50ms, lease sau dài), không làm việc cũ lần nữa.
func TestWorkerReacquiresLeaseAfterExpiry(t *testing.T) {
	store := &runnerStore{until: time.Now().Add(50 * time.Millisecond), job: &Job{ID: "job", ChannelID: "c", Kind: domain.ScanDiscovery, State: domain.ScanQueued}}
	// Lease thứ hai còn hạn dài; acquire đếm được.
	store.nextUntil = time.Now().Add(time.Minute)
	provider := &ctxBlocking{started: make(chan struct{})}
	w := newRunner(t, store, provider, 300*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("fetch không bắt đầu")
	}
	waitFor(t, "nhận lease lần hai", func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return store.acquires >= 2
	})
	cancel()
	if err := waitRun(t, errCh); err != nil {
		t.Fatalf("Run sau khi ctx huỷ = %v, muốn nil", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.acquires < 2 {
		t.Fatalf("AcquireWorkerLease = %d, muốn ≥ 2", store.acquires)
	}
	if !store.jobGiven {
		t.Fatal("test không tạo được việc cho lượt đầu")
	}
}

// ── thăm dò bị fence: chờ có kiểm soát, kênh khác vẫn chạy ──────────────────

// probeStore: không có listing job, chỉ có upload candidate; NextWakeup do test
// điều khiển; fence của kênh A luôn bận.
type probeStore struct {
	Store
	mu         sync.Mutex
	jobB       *Job
	jobCalls   int
	bReady     bool
	wakeup     Wakeup
	selections int
	aFence     bool
}

func (p *probeStore) DueChannels(context.Context, int, time.Time) ([]string, error) {
	return nil, nil
}
func (p *probeStore) NextRunnableJob(context.Context, map[string]bool) (*Job, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.jobCalls++
	if p.bReady {
		p.bReady = false
		return p.jobB, nil
	}
	return nil, nil
}
func (p *probeStore) NextWakeup(context.Context, int, time.Time) (Wakeup, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.wakeup, nil
}
func (p *probeStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.selections++
	if p.aFence {
		return &UploadCandidate{VideoID: "dQw4w9WgXcQ", ChannelID: "A"}, nil
	}
	return nil, nil
}
func (p *probeStore) AcquireChannelLease(_ context.Context, channelID, owner string, ttl time.Duration) (*Lease, error) {
	if channelID == "A" {
		return nil, nil // fence của process khác
	}
	return &Lease{ChannelID: channelID, Token: "00000000-0000-4000-8000-000000000002", Owner: owner, Until: time.Now().Add(ttl)}, nil
}
func (p *probeStore) ClaimJob(context.Context, string, Lease) (*Job, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.jobB, nil
}
func (p *probeStore) ReleaseChannelLease(context.Context, Lease) error { return nil }
func (p *probeStore) EnsureState(context.Context, string) (State, error) {
	return State{ChannelID: "B", HistoryCap: 150}, nil
}
func (p *probeStore) CommitPage(context.Context, CommitPageInput) (CommitPageResult, error) {
	p.mu.Lock()
	p.wakeup = Wakeup{Next: new(time.Now().Add(time.Hour))}
	p.mu.Unlock()
	return CommitPageResult{Scanned: 1, Added: 1}, nil
}
func (p *probeStore) CompleteHeadCheck(context.Context, string, HeadCheckInput) (HeadCheckResult, error) {
	return HeadCheckResult{}, nil
}
func (p *probeStore) Job(context.Context, string) (*Job, error) {
	return &Job{ID: "jobB", ChannelID: "B", Kind: domain.ScanDiscovery, State: domain.ScanRunning, NewestID: "vidB0000001"}, nil
}

// pageProvider trả một trang discovery cho kênh B.
type pageProvider struct {
	Provider
	bRan     chan struct{}
	bCalls   atomic.Int32
	probeHit atomic.Int32
}

func (p *pageProvider) FirstChannelPage(context.Context, string) (*youtube.ChannelVideoPage, error) {
	p.bCalls.Add(1)
	close(p.bRan)
	return &youtube.ChannelVideoPage{Videos: []youtube.ScannedVideo{{ID: "vidB0000001"}}}, nil
}
func (p *pageProvider) FetchUploadTime(context.Context, string) (*time.Time, bool, error) {
	p.probeHit.Add(1)
	return nil, false, nil
}

// TestProbeFenceMissWaitsWithoutSpin: fence bận, không có hạn fence trong lịch
// (Next rỗng) thì worker ngủ một nhịp có kiểm soát thay vì chọn lại liên tục.
func TestProbeFenceMissWaitsWithoutSpin(t *testing.T) {
	store := &probeStore{aFence: true}
	// Hình thái từng gây quay vòng: "có việc" (pause đã hết hạn + kênh khác
	// giữ fence) nhưng lựa chọn vẫn trả đúng candidate bị fence.
	store.wakeup = Wakeup{Actionable: true, Next: new(time.Now().Add(80 * time.Millisecond))}
	provider := &pageProvider{bRan: make(chan struct{})}
	w := NewWorker(WorkerDeps{Store: store, YouTube: provider, BusyRetry: 40 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); _ = w.loop(ctx, "") }()
	<-done
	store.mu.Lock()
	selections := store.selections
	store.mu.Unlock()
	if selections < 1 {
		t.Fatal("test không chạm nhánh thăm dò bị fence")
	}
	if selections > 6 {
		t.Fatalf("chọn candidate %d lần trong 130ms — quay vòng vì fence", selections)
	}
}

// TestFencedProbeLetsOtherChannelProceed: kênh A bị fence nhưng kênh B còn chạy
// được (Actionable) thì worker đi tiếp ngay, không ngủ tới hạn fence của A.
func TestFencedProbeLetsOtherChannelProceed(t *testing.T) {
	store := &probeStore{
		aFence: true,
		jobB:   &Job{ID: "jobB", ChannelID: "B", Kind: domain.ScanDiscovery, State: domain.ScanQueued, NewestID: "vidB0000001"},
		wakeup: Wakeup{Actionable: true, Next: new(time.Now().Add(time.Hour))},
	}
	store.bReady = true
	provider := &pageProvider{bRan: make(chan struct{})}
	w := NewWorker(WorkerDeps{Store: store, YouTube: provider, BusyRetry: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = w.loop(ctx, "") }()
	select {
	case <-provider.bRan:
	case <-time.After(time.Second):
		t.Fatal("kênh B không được xử lý trong khi A bị fence")
	}
	if got := provider.bCalls.Load(); got == 0 {
		t.Fatal("không có request cho kênh B")
	}
	if got := provider.probeHit.Load(); got != 0 {
		t.Fatalf("thăm dò A vẫn gọi mạng khi fence bận: %d", got)
	}
}

// ── chọn việc bị fence: chờ có hẹn giờ, không chọn lại liên tục ─────────────

// fencedListingStore giả lập lần đọc lịch còn báo Actionable cho đúng kênh
// đang bị fence (dạng SQL trước khi retained-running được fence-gate): kênh
// trả về luôn là c1, AcquireChannelLease luôn thất bại.
type fencedListingStore struct {
	Store
	mu         sync.Mutex
	selections int
	actionable bool
	next       time.Time
}

func (f *fencedListingStore) AcquireWorkerLease(context.Context, string, time.Duration) (*WorkerLease, error) {
	return &WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil
}
func (f *fencedListingStore) NextRunnableJob(_ context.Context, skip map[string]bool) (*Job, error) {
	if skip["c1"] {
		return nil, nil
	}
	f.mu.Lock()
	f.selections++
	f.mu.Unlock()
	return &Job{ID: "job", ChannelID: "c1", Kind: domain.ScanHistory, State: domain.ScanRunning}, nil
}
func (f *fencedListingStore) AcquireChannelLease(context.Context, string, string, time.Duration) (*Lease, error) {
	return nil, nil
}
func (f *fencedListingStore) NextWakeup(context.Context, int, time.Time) (Wakeup, error) {
	return Wakeup{Actionable: f.actionable, Next: &f.next}, nil
}
func (f *fencedListingStore) DueChannels(context.Context, int, time.Time) ([]string, error) {
	return nil, nil
}
func (f *fencedListingStore) NextUploadCandidate(context.Context, time.Time) (*UploadCandidate, error) {
	return nil, nil
}
func (f *fencedListingStore) RenewWorkerLease(_ context.Context, l WorkerLease, _ time.Duration) (*WorkerLease, error) {
	l.Until = time.Now().Add(time.Minute)
	return &l, nil
}
func (f *fencedListingStore) ReleaseWorkerLease(context.Context, WorkerLease) error { return nil }
func (f *fencedListingStore) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.selections
}

// TestWorkerFencedListingSelectionStaysBounded: kênh đang bị fence không được
// chọn lại trong vòng lặp không hẹn giờ, kể cả khi lần đọc lịch còn báo có
// việc; mỗi vòng lặp lại phải đi qua một lần chờ (busyRetry/fence/wake).
func TestWorkerFencedListingSelectionStaysBounded(t *testing.T) {
	store := &fencedListingStore{actionable: true, next: time.Now().Add(30 * time.Millisecond)}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, "lượt chọn đầu", func() bool { return store.count() >= 1 })
	time.Sleep(60 * time.Millisecond)
	got := store.count()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run sau khi ctx huỷ = %v, muốn nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker không dừng sau khi ctx huỷ")
	}
	if got > 4 {
		t.Fatalf("chọn lại cùng kênh bị fence %d lần trong 60ms — vòng lặp không hẹn giờ", got)
	}
}

// ── huỷ trong lúc lấy lease ────────────────────────────────────────────────

// cancelAcquireStore chỉ trả về khi ctx huỷ (giống câu truy vấn DB bị huỷ) và
// báo đã vào câu lấy lease để test huỷ đúng lúc đang chờ.
type cancelAcquireStore struct {
	Store
	once    sync.Once
	entered chan struct{}
}

func (s *cancelAcquireStore) AcquireWorkerLease(ctx context.Context, _ string, _ time.Duration) (*WorkerLease, error) {
	s.once.Do(func() { close(s.entered) })
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestRunCancelDuringAcquireReturnsNil: huỷ từ caller khi đang chờ lease là
// dừng sạch, không phải lỗi context.Canceled.
func TestRunCancelDuringAcquireReturnsNil(t *testing.T) {
	store := &cancelAcquireStore{entered: make(chan struct{})}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("worker không vào câu lấy lease")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, muốn nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run không thoát sau khi ctx huỷ trong lúc lấy lease")
	}
}

// leaseThenCancelStore trả lease đồng thời với việc caller huỷ ctx.
type leaseThenCancelStore struct {
	Store
	cancel   context.CancelFunc
	released int
}

func (s *leaseThenCancelStore) AcquireWorkerLease(context.Context, string, time.Duration) (*WorkerLease, error) {
	s.cancel()
	return &WorkerLease{Owner: "o", Until: time.Now().Add(time.Minute)}, nil
}
func (s *leaseThenCancelStore) ReleaseWorkerLease(context.Context, WorkerLease) error {
	s.released++
	return nil
}

// TestRunReleasesLeaseReturnedWithCancellation: lease về cùng lúc ctx huỷ vẫn
// được trả lại trong một ngân sách ngắn; Run trả nil.
func TestRunReleasesLeaseReturnedWithCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &leaseThenCancelStore{cancel: cancel}
	w := NewWorker(WorkerDeps{Store: store, YouTube: errProvider{}})
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, muốn nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run không thoát")
	}
	if store.released != 1 {
		t.Fatalf("ReleaseWorkerLease = %d, muốn 1 (lease về cùng lúc huỷ)", store.released)
	}
}

// acquireErrStore trả lỗi thật của kho.
type acquireErrStore struct{ Store }

func (acquireErrStore) AcquireWorkerLease(context.Context, string, time.Duration) (*WorkerLease, error) {
	return nil, errors.New("db down")
}

// TestRunPropagatesAcquireError: lỗi thật của kho không bị nuốt thành nil.
func TestRunPropagatesAcquireError(t *testing.T) {
	w := NewWorker(WorkerDeps{Store: acquireErrStore{}, YouTube: errProvider{}})
	err := w.Run(context.Background())
	if err == nil || err.Error() != "db down" {
		t.Fatalf("Run = %v, muốn lỗi kho", err)
	}
}
