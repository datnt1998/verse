package scan_test

// Test tích hợp Postgres của luồng quét: worker thật trên ScanStore thật (thứ
// tự discovery trước lịch sử, mốc head, tiến độ) và mặt service (Add, kiểm tra,
// cap, gỡ theo dõi) trên kho thật + activity.Service thật. Bỏ qua khi thiếu
// TEST_DATABASE_URL như các store test khác.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/scan"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// channelID hợp lệ 24 ký tự.
const channelID = "UCHnyfMqiRRG1u-2MsSQLbXA"

func scanPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := pgtest.Provision(t)
	ctx := context.Background()
	db, err := pgmigrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("mở DB migrate: %v", err)
	}
	defer db.Close()
	if _, err := pgmigrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("mở pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func v(id string, views int64) youtube.ScannedVideo {
	s := id
	for len(s) < 11 {
		s += "0"
	}
	return youtube.ScannedVideo{ID: s[:11], Title: "video " + id, ViewCount: new(views)}
}

func profile() *youtube.ChannelProfile {
	handle := "@fixture"
	desc := "kênh fixture"
	return &youtube.ChannelProfile{
		ID: channelID, Handle: &handle, Title: "Kênh Fixture", Description: &desc,
		URL: "https://www.youtube.com/channel/" + channelID, SubscriberCount: new(int64(1000)),
	}
}

func baselineCursor() *youtube.ChannelCursor {
	return &youtube.ChannelCursor{
		Version:      1,
		ChannelID:    channelID,
		Tab:          "videos",
		Order:        "latest",
		Continuation: youtube.Continuation{Token: "tok", Request: "CONTINUATION_REQUEST_TYPE_BROWSE"},
		Session:      youtube.SessionContext{ClientName: "WEB", ClientVersion: "2.20260101.00.00"},
	}
}

// scripted cung cấp trang YouTube theo kịch bản và ghi thứ tự lời gọi.
type scripted struct {
	scan.Provider
	mu      sync.Mutex
	calls   []string
	first   []*youtube.ChannelVideoPage
	next    []*youtube.ChannelVideoPage
	errs    []error
	lastCur youtube.ChannelCursor
}

func (p *scripted) ResolveChannelRef(_ context.Context, ref string) (string, error) {
	return channelID, nil
}

func (p *scripted) pop(kind string, pages *[]*youtube.ChannelVideoPage) (*youtube.ChannelVideoPage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, kind)
	if len(p.errs) > 0 {
		err := p.errs[0]
		p.errs = p.errs[1:]
		return nil, err
	}
	if len(*pages) == 0 {
		return nil, errors.New("kịch bản hết trang cho " + kind)
	}
	page := (*pages)[0]
	*pages = (*pages)[1:]
	return page, nil
}

func (p *scripted) FirstChannelPage(context.Context, string) (*youtube.ChannelVideoPage, error) {
	return p.pop("first", &p.first)
}

func (p *scripted) NextChannelPage(_ context.Context, cursor youtube.ChannelCursor) (*youtube.ChannelVideoPage, error) {
	p.mu.Lock()
	p.lastCur = cursor
	p.mu.Unlock()
	return p.pop("next", &p.next)
}

func (p *scripted) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.calls...)
}

func (p *scripted) cursor() youtube.ChannelCursor {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastCur
}

// seedBaseline ghi kênh + 3 video mới nhất và cursor sang trang kế.
func seedBaseline(t *testing.T, store *postgres.ScanStore) {
	t.Helper()
	cur := baselineCursor()
	lease := scan.Lease{ChannelID: channelID, Token: "00000000-0000-4000-8000-000000000001", Owner: "add", Until: time.Now().Add(time.Minute)}
	if _, err := store.CommitPage(context.Background(), scan.CommitPageInput{
		ChannelID:     channelID,
		Videos:        []youtube.ScannedVideo{v("vid3", 300), v("vid2", 200), v("vid1", 100)},
		NextCursor:    cur,
		Origin:        domain.OriginBaseline,
		Lease:         lease,
		NewestVideoID: new(v("vid3", 300).ID),
		Profile:       profile(),
		Now:           time.Now(),
	}); err != nil {
		t.Fatalf("commit baseline: %v", err)
	}
	// Baseline gieo fence trong giao dịch; nhả để worker nhận kênh.
	if err := store.ReleaseChannelLease(context.Background(), lease); err != nil {
		t.Fatalf("release baseline lease: %v", err)
	}
}

func TestWorkerRunsDiscoveryThenHistoryAndClosesBoth(t *testing.T) {
	pool := scanPool(t)
	store := postgres.NewScanStore(pool)
	ctx := context.Background()
	seedBaseline(t, store)

	discovery, err := store.StartDiscovery(ctx, channelID, false)
	if err != nil || !discovery {
		t.Fatalf("StartDiscovery = %v, %v", discovery, err)
	}
	historyJob, err := store.StartBackfill(ctx, channelID)
	if err != nil || historyJob == nil {
		t.Fatalf("StartBackfill = %v, %v", historyJob, err)
	}
	before, err := store.NextRunnableJobForChannel(ctx, channelID)
	if err != nil || before == nil || before.Kind != domain.ScanDiscovery {
		t.Fatalf("việc chạy được đầu tiên phải là discovery: %+v, %v", before, err)
	}
	discoveryJobID := before.ID

	provider := &scripted{
		first: []*youtube.ChannelVideoPage{{
			Videos: []youtube.ScannedVideo{v("vid4", 400), v("vid3", 300)},
		}},
		next: []*youtube.ChannelVideoPage{{
			Videos: []youtube.ScannedVideo{v("vid2", 200), v("vid1", 100)},
		}},
	}
	w := scan.NewWorker(scan.WorkerDeps{
		Store:   store,
		YouTube: provider,
		Now:     time.Now,
		Random:  func() float64 { return 0.5 },
	})
	runCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ran, err := w.RunInline(runCtx, channelID)
	if err != nil {
		t.Fatalf("RunInline: %v", err)
	}
	if !ran {
		t.Fatal("RunInline không chạy được (lease worker bận?)")
	}
	if got := strings.Join(provider.recorded(), ","); got != "first,next" {
		t.Fatalf("thứ tự lời gọi = %s, muốn first,next (discovery trước lịch sử)", got)
	}
	if got := provider.cursor().ChannelID; got != channelID {
		t.Fatalf("cursor lịch sử thuộc kênh %q, muốn %q", got, channelID)
	}

	discoveryJob, err := store.Job(ctx, discoveryJobID)
	if err != nil || discoveryJob == nil {
		t.Fatalf("đọc việc discovery: %v", err)
	}
	if discoveryJob.State != domain.ScanComplete {
		t.Fatalf("discovery = %v, muốn complete", discoveryJob.State)
	}
	finishedHistory, err := store.Job(ctx, historyJob.ID)
	if err != nil || finishedHistory == nil {
		t.Fatalf("đọc việc lịch sử: %v", err)
	}
	if finishedHistory.State != domain.ScanComplete || !finishedHistory.ReachedEnd {
		t.Fatalf("lịch sử = %v (reachedEnd=%v), muốn complete đã hết tab",
			finishedHistory.State, finishedHistory.ReachedEnd)
	}
	state, err := store.EnsureState(ctx, channelID)
	if err != nil {
		t.Fatalf("EnsureState: %v", err)
	}
	if !state.DiscoveryCaughtUp || state.HeadCheckedAt == nil {
		t.Fatalf("head check chưa đóng: caughtUp=%v checked=%v", state.DiscoveryCaughtUp, state.HeadCheckedAt)
	}
	if !state.HistoryEnd {
		t.Fatal("state.HistoryEnd phải true sau khi đọc hết tab")
	}
}

// recordingGate ghi lần chặn nền tảng; không chờ.
type recordingGate struct {
	blocked     int
	pausedUntil *time.Time
}

func (g *recordingGate) Until(context.Context, domain.PlatformID) (*time.Time, error) {
	return g.pausedUntil, nil
}
func (g *recordingGate) RecordBlocked(context.Context, domain.PlatformID, time.Duration) error {
	g.blocked++
	return nil
}

func TestServiceAddCheckCapAndRemove(t *testing.T) {
	pool := scanPool(t)
	store := postgres.NewScanStore(pool)
	ctx := context.Background()
	provider := &scripted{first: []*youtube.ChannelVideoPage{
		{Videos: []youtube.ScannedVideo{v("vid3", 300), v("vid2", 200)}, Profile: profile(), NextCursor: baselineCursor()},
		// Add lần hai vẫn đọc trang đầu (cần hồ sơ) rồi mới báo exists.
		{Videos: []youtube.ScannedVideo{v("vid3", 300), v("vid2", 200)}, Profile: profile(), NextCursor: baselineCursor()},
	}}
	activities := activity.New(postgres.NewActivityStore(pool), nil)
	svc := scan.New(scan.Deps{
		Store:    store,
		YouTube:  provider,
		Settings: postgres.NewSettingsStore(pool),
		Tasks:    activities,
		Platform: &recordingGate{},
		Now:      time.Now,
	})

	channel, err := svc.Add(ctx, "@fixture")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if channel.ID != channelID || channel.Title != "Kênh Fixture" {
		t.Fatalf("kênh = %+v", channel)
	}

	if _, err := svc.Add(ctx, "@fixture"); err == nil {
		t.Fatal("Add lần hai phải báo exists")
	} else {
		var se *scan.Error
		if !errors.As(err, &se) || se.Code != scan.CodeExists || se.Existing == nil {
			t.Fatalf("lỗi lần hai = %v, muốn exists kèm bản ghi", err)
		}
	}

	status, err := svc.Status(ctx, channelID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.IndexedVideos != 2 || status.RescanHours != domain.RescanHoursDefault {
		t.Fatalf("status = %+v", status)
	}
	if status.Discovery.CheckedAt == nil || !status.Discovery.CaughtUp {
		t.Fatalf("head check sau Add chưa đóng: %+v", status.Discovery)
	}

	afterCheck, err := svc.Control(ctx, channelID, domain.ScanActionCheck)
	if err != nil {
		t.Fatalf("Control(check): %v", err)
	}
	if !afterCheck.Discovery.Running {
		t.Fatalf("sau check phải có discovery đang chạy: %+v", afterCheck.Discovery)
	}

	started, err := svc.Control(ctx, channelID, domain.ScanActionStart)
	if err != nil {
		t.Fatalf("Control(start): %v", err)
	}
	if started.History.State != domain.ScanQueued && started.History.State != domain.ScanRunning {
		t.Fatalf("lịch sử sau start = %q", started.History.State)
	}
	if _, err := svc.Control(ctx, channelID, "bogus"); err == nil {
		t.Fatal("thao tác lạ phải lỗi")
	} else {
		var se *scan.Error
		if !errors.As(err, &se) || se.Code != scan.CodeInvalidInput {
			t.Fatalf("lỗi thao tác lạ = %v", err)
		}
	}
	cancelled, err := svc.Control(ctx, channelID, domain.ScanActionCancel)
	if err != nil {
		t.Fatalf("Control(cancel): %v", err)
	}
	if cancelled.History.State != domain.ScanCancelled {
		t.Fatalf("lịch sử sau huỷ = %q, muốn cancelled", cancelled.History.State)
	}

	if _, err := svc.SetCap(ctx, channelID, domain.HistoryCapMin-1); err == nil {
		t.Fatal("cap dưới sàn phải lỗi")
	} else {
		var se *scan.Error
		if !errors.As(err, &se) || se.Code != scan.CodeInvalidCap {
			t.Fatalf("lỗi cap = %v, muốn invalid_cap", err)
		}
	}
	capped, err := svc.SetCap(ctx, channelID, 300)
	if err != nil || capped.HistoryCap != 300 {
		t.Fatalf("SetCap(300) = %+v, %v", capped, err)
	}

	if err := svc.Remove(ctx, channelID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := svc.Status(ctx, channelID); err == nil {
		t.Fatal("Status sau Remove phải not_found")
	} else {
		var se *scan.Error
		if !errors.As(err, &se) || se.Code != scan.CodeNotFound {
			t.Fatalf("lỗi Status sau Remove = %v", err)
		}
	}
}

// TestCheckDoesNotResumePausedHistory: "check" chỉ xếp discovery; lịch
// sử người dùng đã tạm dừng/huỷ phải giữ nguyên trạng thái.
func TestCheckDoesNotResumePausedHistory(t *testing.T) {
	pool := scanPool(t)
	store := postgres.NewScanStore(pool)
	ctx := context.Background()
	provider := &scripted{first: []*youtube.ChannelVideoPage{{
		Videos: []youtube.ScannedVideo{v("vid3", 300), v("vid2", 200)}, Profile: profile(), NextCursor: baselineCursor(),
	}}}
	svc := scan.New(scan.Deps{
		Store: store, YouTube: provider, Settings: postgres.NewSettingsStore(pool),
		Tasks: activity.New(postgres.NewActivityStore(pool), nil), Platform: &recordingGate{}, Now: time.Now,
	})
	if _, err := svc.Add(ctx, "@fixture"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	started, err := svc.Control(ctx, channelID, domain.ScanActionStart)
	if err != nil || started.History.JobID == "" {
		t.Fatalf("Control(start) = %+v, %v", started, err)
	}
	historyID := started.History.JobID
	if _, err := svc.Control(ctx, channelID, domain.ScanActionPause); err != nil {
		t.Fatalf("Control(pause): %v", err)
	}
	afterCheck, err := svc.Control(ctx, channelID, domain.ScanActionCheck)
	if err != nil {
		t.Fatalf("Control(check): %v", err)
	}
	history, err := store.Job(ctx, historyID)
	if err != nil || history == nil {
		t.Fatalf("đọc lịch sử: %v", err)
	}
	if history.State != domain.ScanPaused {
		t.Fatalf("check đã đổi lịch sử paused → %v", history.State)
	}
	if !afterCheck.Discovery.Running {
		t.Fatalf("check phải xếp discovery: %+v", afterCheck.Discovery)
	}

	// Huỷ rồi check: lịch sử đã huỷ cũng không được tự chạy lại.
	if _, err := svc.Control(ctx, channelID, domain.ScanActionCancel); err != nil {
		t.Fatalf("Control(cancel): %v", err)
	}
	if _, err := svc.Control(ctx, channelID, domain.ScanActionCheck); err != nil {
		t.Fatalf("Control(check) sau huỷ: %v", err)
	}
	cancelled, err := store.Job(ctx, historyID)
	if err != nil || cancelled == nil {
		t.Fatalf("đọc lịch sử sau huỷ: %v", err)
	}
	if cancelled.State != domain.ScanCancelled {
		t.Fatalf("check đã đổi lịch sử cancelled → %v", cancelled.State)
	}
}

// TestTaskAdoptionKeepsStoredProgress: Start/ResetTask khi nhận chủ mới
// không được xoá tiến độ đang hiện của dòng Việc nền.
func TestTaskAdoptionKeepsStoredProgress(t *testing.T) {
	pool := scanPool(t)
	store := postgres.NewScanStore(pool)
	ctx := context.Background()
	provider := &scripted{first: []*youtube.ChannelVideoPage{{
		Videos: []youtube.ScannedVideo{v("vid3", 300), v("vid2", 200)}, Profile: profile(), NextCursor: baselineCursor(),
	}}}
	activities := activity.New(postgres.NewActivityStore(pool), nil)
	svc := scan.New(scan.Deps{
		Store: store, YouTube: provider, Settings: postgres.NewSettingsStore(pool),
		Tasks: activities, Platform: &recordingGate{}, Now: time.Now,
	})
	if _, err := svc.Add(ctx, "@fixture"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := svc.Control(ctx, channelID, domain.ScanActionStart); err != nil {
		t.Fatalf("Control(start): %v", err)
	}
	progressOfTask := func() (int64, int64, bool) {
		snap, err := activities.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		for _, task := range snap.Tasks {
			if task.Kind == domain.TaskChannelScan && task.Title == "Quét lịch sử" {
				if task.Progress == nil {
					return 0, 0, false
				}
				return task.Progress.Done, task.Progress.Total, true
			}
		}
		t.Fatal("không thấy dòng Việc nền của lịch sử")
		return 0, 0, false
	}
	done, total, ok := progressOfTask()
	if !ok || done != 2 || total != domain.HistoryCapDefault {
		t.Fatalf("progress sau start = %d/%d (ok=%v), muốn 2/%d", done, total, ok, domain.HistoryCapDefault)
	}
	// Adoption lặp lại (Start trên việc đang sống) ghi lại toàn bộ dòng; tiến
	// độ phải còn đó, không bị NULL hoá.
	if _, err := svc.Control(ctx, channelID, domain.ScanActionStart); err != nil {
		t.Fatalf("Control(start) lần hai: %v", err)
	}
	done, total, ok = progressOfTask()
	if !ok || done != 2 || total != domain.HistoryCapDefault {
		t.Fatalf("adoption làm mất tiến độ: %d/%d (ok=%v)", done, total, ok)
	}
}

// TestDiscoveryTaskActionsDispatchWithoutHistory: hành động mà dòng Việc nền
// của discovery phơi ra, khi bấm qua đúng Service.Control, không được đổi
// trạng thái lịch sử — với mọi trạng thái discovery còn hiện dòng.
func TestDiscoveryTaskActionsDispatchWithoutHistory(t *testing.T) {
	states := []domain.ScanJobState{domain.ScanQueued, domain.ScanRunning, domain.ScanBackoff, domain.ScanError, domain.ScanPaused}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			pool := scanPool(t)
			store := postgres.NewScanStore(pool)
			ctx := context.Background()
			provider := &scripted{first: []*youtube.ChannelVideoPage{
				{Videos: []youtube.ScannedVideo{v("d1", 100)}, Profile: profile(), NextCursor: baselineCursor()},
			}}
			activities := activity.New(postgres.NewActivityStore(pool), nil)
			svc := scan.New(scan.Deps{
				Store: store, YouTube: provider, Settings: postgres.NewSettingsStore(pool),
				Tasks: activities, Platform: &recordingGate{}, Now: time.Now,
			})
			if _, err := svc.Add(ctx, "@fixture"); err != nil {
				t.Fatalf("Add: %v", err)
			}
			if _, err := store.StartBackfill(ctx, channelID); err != nil {
				t.Fatalf("StartBackfill: %v", err)
			}
			disc, err := store.StartDiscovery(ctx, channelID, true)
			if err != nil || !disc {
				t.Fatalf("StartDiscovery = %v, %v", disc, err)
			}
			snap, err := store.Status(ctx, channelID)
			if err != nil || snap.DiscoveryJob == nil {
				t.Fatalf("Status discovery = %+v, %v", snap.DiscoveryJob, err)
			}
			job := snap.DiscoveryJob
			if state != domain.ScanQueued {
				lease, err := store.AcquireChannelLease(ctx, channelID, "test", time.Minute)
				if err != nil || lease == nil {
					t.Fatalf("AcquireChannelLease = %v, %v", lease, err)
				}
				claimed, err := store.ClaimJob(ctx, job.ID, *lease)
				if err != nil || claimed == nil {
					t.Fatalf("ClaimJob = %v, %v", claimed, err)
				}
				switch state {
				case domain.ScanBackoff:
					err = store.SetJobBackoff(ctx, job.ID, *lease, scan.Backoff{
						NextAttemptAt: time.Now().Add(time.Hour), Attempts: 1,
						ErrorCode: "transient", Err: errors.New("tạm thời"), Recovery: scan.RecoveryFailures,
					})
				case domain.ScanError:
					err = store.SetJobError(ctx, job.ID, *lease, scan.JobError{
						ErrorCode: "blocked", Err: errors.New("YouTube chặn"), Recovery: scan.RecoveryBlocked,
					})
				case domain.ScanPaused:
					err = store.SetJobPaused(ctx, job.ID, *lease, scan.JobError{
						ErrorCode: "blocked", Err: errors.New("YouTube chặn"), Recovery: scan.RecoveryBlocked,
					})
				}
				if err != nil {
					t.Fatalf("đặt trạng thái %q: %v", state, err)
				}
			}
			snap, err = store.Status(ctx, channelID)
			if err != nil || snap.DiscoveryJob == nil {
				t.Fatalf("Status sau setup = %+v, %v", snap.DiscoveryJob, err)
			}
			view, live := scan.TaskPresentation(*snap.DiscoveryJob)
			if !live {
				t.Fatalf("discovery %q phải còn dòng Việc nền", state)
			}
			if view.Action.Op == domain.TaskOpCancel {
				t.Fatalf("discovery %q phơi nút Huỷ — op huỷ chỉ tác động lịch sử", state)
			}
			if view.Action.Label == "Mở" {
				if view.Action.Href != "#/explore?rival="+channelID || view.Action.Op != "" {
					t.Fatalf("Mở phải trỏ kênh và không op: %+v", view.Action)
				}
			}
			var action domain.ScanAction
			switch view.Action.Op {
			case domain.TaskOpResume, domain.TaskOpRetry:
				action = domain.ScanActionResume
			}
			if action != "" {
				if _, err := svc.Control(ctx, channelID, action); err != nil {
					t.Fatalf("Control(%s): %v", action, err)
				}
			}
			after, err := store.Status(ctx, channelID)
			if err != nil || after.HistoryJob == nil {
				t.Fatalf("Status sau bấm = %+v, %v", after.HistoryJob, err)
			}
			if after.HistoryJob.State != domain.ScanQueued {
				t.Fatalf("bấm nút discovery %q đổi lịch sử sang %q", state, after.HistoryJob.State)
			}
		})
	}
}

// TestHealthySyncPublishesActivityChange: mọi lần ghi dòng Việc nền thành công
// (tạo, tạm dừng, tiếp tục) phát đúng id dòng qua activity.Service.Changed.
func TestHealthySyncPublishesActivityChange(t *testing.T) {
	pool := scanPool(t)
	store := postgres.NewScanStore(pool)
	ctx := context.Background()
	provider := &scripted{first: []*youtube.ChannelVideoPage{
		{Videos: []youtube.ScannedVideo{v("p1", 100)}, Profile: profile(), NextCursor: baselineCursor()},
	}}
	var mu sync.Mutex
	var events []string
	activities := activity.New(postgres.NewActivityStore(pool), func(ref string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ref)
	})
	svc := scan.New(scan.Deps{
		Store: store, YouTube: provider, Settings: postgres.NewSettingsStore(pool),
		Tasks: activities, Platform: &recordingGate{}, Now: time.Now,
	})
	if _, err := svc.Add(ctx, "@fixture"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	mu.Lock()
	events = nil
	mu.Unlock()

	published := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), events...)
	}
	if _, err := svc.Control(ctx, channelID, domain.ScanActionStart); err != nil {
		t.Fatalf("Control(start): %v", err)
	}
	snap, err := store.Status(ctx, channelID)
	if err != nil || snap.HistoryJob == nil || snap.HistoryJob.TaskID == "" {
		t.Fatalf("việc lịch sử chưa có task: %+v, %v", snap.HistoryJob, err)
	}
	taskID := snap.HistoryJob.TaskID
	if !slices.Contains(published(), taskID) {
		t.Fatalf("tạo dòng việc không phát %q; events=%v", taskID, published())
	}

	mu.Lock()
	events = nil
	mu.Unlock()
	if _, err := svc.Control(ctx, channelID, domain.ScanActionPause); err != nil {
		t.Fatalf("Control(pause): %v", err)
	}
	if !slices.Contains(published(), taskID) {
		t.Fatalf("tạm dừng không phát %q; events=%v", taskID, published())
	}

	mu.Lock()
	events = nil
	mu.Unlock()
	if _, err := svc.Control(ctx, channelID, domain.ScanActionResume); err != nil {
		t.Fatalf("Control(resume): %v", err)
	}
	if !slices.Contains(published(), taskID) {
		t.Fatalf("tiếp tục không phát %q; events=%v", taskID, published())
	}
}
