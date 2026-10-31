// Ca test chuyển từ src/server/backdrop/pacing.test.ts của yt-factory: khoảng
// cách 10–20 giây cho lời gọi API Bilibili, 5–10 giây cho lượt trích yt-dlp,
// chờ tăng dần, từ chối khi đang chờ, Retry-After kéo dài, hạn mức cuộn, các
// lỗi khác không tạo chờ, lease quét và lease yt-dlp.
package platforms_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// memStore là StateStore trong bộ nhớ mô phỏng ngữ nghĩa của
// queries/platforms.sql để test pacing không cần Postgres.
type memStore struct {
	mu     sync.Mutex
	states map[domain.PlatformID]domain.PlatformState
}

func newMemStore() *memStore {
	return &memStore{states: map[domain.PlatformID]domain.PlatformState{}}
}

func (m *memStore) get(id domain.PlatformID) domain.PlatformState {
	if st, ok := m.states[id]; ok {
		return st
	}
	return domain.PlatformState{Platform: id}
}

func (m *memStore) All(context.Context) ([]domain.PlatformState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.PlatformState, 0, len(m.states))
	for _, st := range m.states {
		out = append(out, st)
	}
	return out, nil
}

func (m *memStore) One(_ context.Context, id domain.PlatformID) (domain.PlatformState, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.states[id]
	return st, ok, nil
}

func (m *memStore) Pause(_ context.Context, id domain.PlatformID, now time.Time) (*time.Time, *string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.PausedUntil == nil {
		return nil, nil, nil
	}
	if !st.PausedUntil.After(now) {
		st.PausedUntil, st.PauseReason = nil, nil
		m.states[id] = st
		return nil, nil, nil
	}
	return st.PausedUntil, st.PauseReason, nil
}

func (m *memStore) RecordBlocked(_ context.Context, id domain.PlatformID, now time.Time, reason string, retryAfter time.Duration) (domain.PlatformState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	st.FailureCount++
	wait := time.Duration(platforms.PauseMinutes(st.FailureCount)) * time.Minute
	if retryAfter > wait {
		wait = retryAfter
	}
	until := now.Add(wait)
	st.PausedUntil, st.PauseReason = &until, &reason
	m.states[id] = st
	return st, nil
}

func (m *memStore) ClearPause(_ context.Context, id domain.PlatformID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	st.PausedUntil, st.PauseReason, st.FailureCount = nil, nil, 0
	m.states[id] = st
	return nil
}

func (m *memStore) RecordError(_ context.Context, id domain.PlatformID, code string, at time.Time, jobID *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	st.LastErrorCode, st.LastErrorAt, st.LastErrorJobID = code, &at, jobID
	m.states[id] = st
	return nil
}

func (m *memStore) ClearError(_ context.Context, id domain.PlatformID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	st.LastErrorCode, st.LastErrorAt, st.LastErrorJobID = "", nil, nil
	m.states[id] = st
	return nil
}

func (m *memStore) ReserveSlot(_ context.Context, id domain.PlatformID, now time.Time, spacing time.Duration) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	start := now
	if st.NextRequestAt.After(start) {
		start = st.NextRequestAt
	}
	st.NextRequestAt = start.Add(spacing)
	m.states[id] = st
	return start, nil
}

func (m *memStore) ReserveWeighted(_ context.Context, id domain.PlatformID, now time.Time, req platforms.SlotRequest) (platforms.SlotDecision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.PausedUntil != nil && st.PausedUntil.After(now) {
		return platforms.SlotDecision{PausedUntil: st.PausedUntil, PauseReason: st.PauseReason}, nil
	}
	budget := platforms.BudgetOf(st.RequestHistory, now, req.Windows, req.Cost)
	if budget.AvailableAt.After(now) {
		at := budget.AvailableAt
		return platforms.SlotDecision{BudgetAt: &at}, nil
	}
	start := now
	if st.NextRequestAt.After(start) {
		start = st.NextRequestAt
	}
	cost := req.Cost
	if cost < 1 {
		cost = 1
	}
	st.RequestHistory = append(st.RequestHistory, domain.PlatformRequest{At: start, Cost: cost})
	st.NextRequestAt = start.Add(req.Spacing)
	m.states[id] = st
	return platforms.SlotDecision{StartAt: start}, nil
}

func (m *memStore) AcquireYtDlp(_ context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.YtdlpLeaseUntil != nil && st.YtdlpLeaseUntil.After(now) {
		return false, nil
	}
	st.YtdlpOwner, st.YtdlpLeaseUntil = &owner, &until
	m.states[id] = st
	return true, nil
}

func (m *memStore) ExtendYtDlp(_ context.Context, id domain.PlatformID, owner string, until time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.YtdlpOwner == nil || *st.YtdlpOwner != owner {
		return false, nil
	}
	st.YtdlpLeaseUntil = &until
	m.states[id] = st
	return true, nil
}

func (m *memStore) ReleaseYtDlp(_ context.Context, id domain.PlatformID, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.YtdlpOwner != nil && *st.YtdlpOwner == owner {
		st.YtdlpOwner, st.YtdlpLeaseUntil = nil, nil
		m.states[id] = st
	}
	return nil
}

func (m *memStore) SetNextTrickle(_ context.Context, id domain.PlatformID, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	st.NextTrickleAt = at
	m.states[id] = st
	return nil
}

func (m *memStore) AcquireScanner(_ context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.ScannerLeaseUntil != nil && st.ScannerLeaseUntil.After(now) {
		return false, nil
	}
	st.ScannerOwner, st.ScannerLeaseUntil = &owner, &until
	m.states[id] = st
	return true, nil
}

func (m *memStore) ReleaseScanner(_ context.Context, id domain.PlatformID, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.get(id)
	if st.ScannerOwner != nil && *st.ScannerOwner == owner {
		st.ScannerOwner, st.ScannerLeaseUntil = nil, nil
		m.states[id] = st
	}
	return nil
}

// fakeLogins trả đăng nhập rỗng; pacing không đụng tới.
type fakeLogins struct{}

func (fakeLogins) Login(context.Context, domain.PlatformID) (domain.PlatformLogin, error) {
	return domain.PlatformLogin{}, nil
}
func (fakeLogins) Profiles(context.Context, bool) ([]domain.BrowserProfile, error) { return nil, nil }
func (fakeLogins) SetCookieSource(context.Context, string, string) (domain.PlatformLogin, error) {
	return domain.PlatformLogin{}, nil
}
func (fakeLogins) SaveManualCookie(context.Context, string) (domain.PlatformLogin, error) {
	return domain.PlatformLogin{}, nil
}
func (fakeLogins) ClearLogin(context.Context, domain.PlatformID) (domain.PlatformLogin, error) {
	return domain.PlatformLogin{}, nil
}
func (fakeLogins) StartQR(context.Context) (domain.QRSession, error)     { return domain.QRSession{}, nil }
func (fakeLogins) PollQR(context.Context, string) (domain.QRPoll, error) { return domain.QRPoll{}, nil }
func (fakeLogins) Refresh(context.Context) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	return "", domain.PlatformLogin{}, nil
}
func (fakeLogins) Check(context.Context, domain.PlatformID) (domain.PlatformLogin, error) {
	return domain.PlatformLogin{}, nil
}

// rateError là lỗi bị chặn kèm Retry-After (như BilibiliApiError của bản TS).
type rateError struct{ after time.Duration }

func (e rateError) Error() string             { return "blocked" }
func (e rateError) RetryAfter() time.Duration { return e.after }

// codedError là lỗi chỉ có mã (như tools.YtDlpError{Code:"rate_limited"}).
type codedError struct{ code string }

func (e codedError) Error() string     { return e.code }
func (e codedError) ErrorCode() string { return e.code }

// harness dựng service + pacer với đồng hồ giả và sleep ghi lại.
type harness struct {
	store  *memStore
	svc    *platforms.Service
	pacer  *platforms.Pacer
	now    time.Time
	sleeps []time.Duration
	runs   int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{store: newMemStore(), now: time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)}
	h.svc = platforms.New(platforms.Deps{
		Store:  h.store,
		Logins: fakeLogins{},
		Now:    func() time.Time { return h.now },
	})
	h.pacer = h.svc.NewPacer(func(_ context.Context, d time.Duration) error {
		h.sleeps = append(h.sleeps, d)
		h.now = h.now.Add(d)
		return nil
	}, func() float64 { return 0.5 })
	return h
}

func TestBilibiliSpacingAndTwitchUnchanged(t *testing.T) {
	h := newHarness(t)
	run := func(ctx context.Context, id domain.PlatformID) error {
		h.runs++
		return h.pacer.Request(ctx, id, true, func(context.Context) error { return nil })
	}
	for range 3 {
		if err := run(context.Background(), domain.PlatformBilibili); err != nil {
			t.Fatalf("bilibili: %v", err)
		}
	}
	// 10 giây + jitter 10 giây × random 0.5 = 15 giây mỗi lời gọi API.
	want := []time.Duration{15 * time.Second, 15 * time.Second}
	if len(h.sleeps) != len(want) {
		t.Fatalf("sleeps = %v", h.sleeps)
	}
	for i, d := range h.sleeps {
		if d != want[i] {
			t.Fatalf("sleep[%d] = %v, muốn %v", i, d, want[i])
		}
	}
	h.sleeps = nil
	for range 2 {
		if err := run(context.Background(), domain.PlatformTwitch); err != nil {
			t.Fatalf("twitch: %v", err)
		}
	}
	if len(h.sleeps) != 0 {
		t.Fatalf("twitch phải không chờ, sleeps = %v", h.sleeps)
	}
}

func TestYtDlpRunUsesLeaseAndNarrowerSpacing(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for range 2 {
		if err := h.pacer.RunYtDlp(ctx, domain.PlatformBilibili, 4, 30*time.Second, func(context.Context) error {
			if st, _, _ := h.store.One(ctx, domain.PlatformBilibili); st.YtdlpOwner == nil {
				t.Fatal("trong lệnh yt-dlp phải đang giữ lease")
			}
			return nil
		}); err != nil {
			t.Fatalf("yt-dlp: %v", err)
		}
	}
	// 5 giây + jitter 5 giây × 0.5 = 7,5 giây giữa hai lượt trích.
	if len(h.sleeps) != 1 || h.sleeps[0] != 7500*time.Millisecond {
		t.Fatalf("sleeps = %v, muốn [7.5s]", h.sleeps)
	}
	st, _, _ := h.store.One(ctx, domain.PlatformBilibili)
	if st.YtdlpOwner != nil || st.YtdlpLeaseUntil != nil {
		t.Fatal("lease yt-dlp phải được nhả sau lệnh")
	}
	if len(st.RequestHistory) != 2 || st.RequestHistory[0].Cost != 4 {
		t.Fatalf("lịch sử đặt chỗ = %+v, muốn hai lần cost 4", st.RequestHistory)
	}
}

func TestBudgetRefusesWhenExhaustedAndNoHistoryOnRefusal(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for i := range 20 {
		if err := h.pacer.Request(ctx, domain.PlatformBilibili, true, func(context.Context) error { return nil }); err != nil {
			t.Fatalf("lần %d: %v", i+1, err)
		}
	}
	calls := 0
	err := h.pacer.Request(ctx, domain.PlatformBilibili, true, func(context.Context) error { calls++; return nil })
	var budget *platforms.BudgetError
	if !errors.As(err, &budget) {
		t.Fatalf("muốn BudgetError, được %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, muốn 0 (không chạm mạng khi hết hạn mức)", calls)
	}
	first := h.store.get(domain.PlatformBilibili).RequestHistory[0].At
	if !budget.AvailableAt.Equal(first.Add(10 * time.Minute)) {
		t.Fatalf("availableAt = %v, muốn lần đặt chỗ cũ nhất +10 phút (%v)", budget.AvailableAt, first.Add(10*time.Minute))
	}
	before := len(h.store.get(domain.PlatformBilibili).RequestHistory)
	if before != 20 {
		t.Fatalf("lịch sử = %d, muốn 20 (từ chối không ghi)", before)
	}
	// Cửa sổ 10 phút nhả chỗ thì gọi lại được.
	h.now = budget.AvailableAt
	if err := h.pacer.Request(ctx, domain.PlatformBilibili, true, func(context.Context) error { calls++; return nil }); err != nil {
		t.Fatalf("sau cửa sổ: %v", err)
	}
}

func TestBudgetDailyWindowCapsAtTwoHundred(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	history := make([]domain.PlatformRequest, 0, 201)
	for i := range 200 {
		history = append(history, domain.PlatformRequest{At: now.Add(-time.Duration(i%30) * time.Minute), Cost: 1})
	}
	report := platforms.BudgetOf(history, now, platforms.BilibiliWindows, 1)
	if !report.AvailableAt.After(now) {
		t.Fatalf("200 lần/24 giờ phải hết hạn mức, availableAt = %v", report.AvailableAt)
	}
	if len(report.Windows) != 2 || report.Windows[1].Used != 200 || report.Windows[1].Limit != 200 {
		t.Fatalf("windows = %+v", report.Windows)
	}
	freed := platforms.BudgetOf(history, now.Add(24*time.Hour), platforms.BilibiliWindows, 1)
	if freed.AvailableAt.After(now.Add(24 * time.Hour)) {
		t.Fatalf("sau 24 giờ hạn mức phải nhả, availableAt = %v", freed.AvailableAt)
	}
}

func TestBackoffEscalatesAndResets(t *testing.T) {
	blocked := true
	h := newHarness(t)
	run := func() error {
		return h.pacer.Request(context.Background(), domain.PlatformBilibili, true, func(context.Context) error {
			h.runs++
			if blocked {
				return codedError{code: "rate_limited"}
			}
			return nil
		})
	}
	for _, minutes := range []int{15, 30, 60, 120, 120} {
		err := run()
		var paused *platforms.PausedError
		if !errors.As(err, &paused) {
			t.Fatalf("muốn PausedError, được %v", err)
		}
		if strings.Contains(err.Error(), "rate_limited") {
			t.Fatalf("lỗi phải đã dịch, được %q", err.Error())
		}
		until, _, _ := h.store.Pause(context.Background(), domain.PlatformBilibili, h.now)
		if until == nil || !until.Equal(h.now.Add(time.Duration(minutes)*time.Minute)) {
			t.Fatalf("lần chặn kế: until = %v, muốn +%d phút", until, minutes)
		}
		before := h.runs
		if err := run(); err == nil {
			t.Fatal("lời gọi khi đang chờ phải bị từ chối")
		}
		if h.runs != before {
			t.Fatal("lời gọi khi đang chờ không được chạm mạng")
		}
		h.now = *until
		until, reason, _ := h.store.Pause(context.Background(), domain.PlatformBilibili, h.now)
		if until != nil || reason != nil {
			t.Fatal("hết giờ chờ thì trường chờ phải được dọn")
		}
		// Gọi thành công nhưng reset=false (nav/thiết bị) không xoá backoff.
		if err := h.pacer.Request(context.Background(), domain.PlatformBilibili, false, func(context.Context) error { return nil }); err != nil {
			t.Fatalf("reset=false: %v", err)
		}
		st, _, _ := h.store.One(context.Background(), domain.PlatformBilibili)
		if st.FailureCount == 0 {
			t.Fatal("reset=false không được đếm là thành công trang bị chặn")
		}
	}
	blocked = false
	if err := run(); err != nil {
		t.Fatalf("thành công: %v", err)
	}
	st, _, _ := h.store.One(context.Background(), domain.PlatformBilibili)
	if st.FailureCount != 0 {
		t.Fatalf("failureCount = %d, muốn 0", st.FailureCount)
	}
	blocked = true
	if err := run(); err == nil {
		t.Fatal("chặn lại phải tạo giờ chờ")
	}
	until, _, _ := h.store.Pause(context.Background(), domain.PlatformBilibili, h.now)
	if until == nil || !until.Equal(h.now.Add(15*time.Minute)) {
		t.Fatalf("lần chặn mới phải bắt đầu lại 15 phút, until = %v", until)
	}
}

func TestRetryAfterExtendsCooldown(t *testing.T) {
	h := newHarness(t)
	err := h.pacer.Request(context.Background(), domain.PlatformBilibili, true, func(context.Context) error {
		return rateError{after: 45 * time.Minute}
	})
	var paused *platforms.PausedError
	if !errors.As(err, &paused) {
		t.Fatalf("muốn PausedError, được %v", err)
	}
	until, _, _ := h.store.Pause(context.Background(), domain.PlatformBilibili, h.now)
	if until == nil || !until.Equal(h.now.Add(45*time.Minute)) {
		t.Fatalf("until = %v, muốn +45 phút", until)
	}
}

func TestPauseStartedWhileWaitingRefusesQueuedRequest(t *testing.T) {
	h := newHarness(t)
	calls := 0
	if err := h.pacer.Request(context.Background(), domain.PlatformBilibili, true, func(context.Context) error {
		calls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h.pacer = h.svc.NewPacer(func(ctx context.Context, d time.Duration) error {
		h.now = h.now.Add(d)
		until := h.now.Add(time.Minute)
		reason := "Bilibili đang giới hạn yêu cầu"
		h.store.mu.Lock()
		st := h.store.get(domain.PlatformBilibili)
		st.PausedUntil, st.PauseReason = &until, &reason
		h.store.states[domain.PlatformBilibili] = st
		h.store.mu.Unlock()
		return nil
	}, func() float64 { return 0.5 })
	err := h.pacer.Request(context.Background(), domain.PlatformBilibili, true, func(context.Context) error {
		calls++
		return nil
	})
	if err == nil {
		t.Fatal("phải từ chối lời gọi đã xếp hàng khi nền tảng bị chặn")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, muốn 1", calls)
	}
}

func TestOtherFailuresAndTwitchRateLimitsDoNotPause(t *testing.T) {
	h := newHarness(t)
	if err := h.pacer.Request(context.Background(), domain.PlatformBilibili, true, func(context.Context) error {
		return codedError{code: "failed"}
	}); err == nil {
		t.Fatal("lỗi khác phải trả ra ngoài")
	}
	if until, _, _ := h.store.Pause(context.Background(), domain.PlatformBilibili, h.now); until != nil {
		t.Fatal("lỗi khác không được tạo giờ chờ")
	}
	if err := h.pacer.Request(context.Background(), domain.PlatformTwitch, true, func(context.Context) error {
		return rateError{after: time.Minute}
	}); err == nil {
		t.Fatal("rate limit Twitch phải trả ra ngoài")
	}
	if until, _, _ := h.store.Pause(context.Background(), domain.PlatformTwitch, h.now); until != nil {
		t.Fatal("Twitch không dùng giờ chờ tăng dần")
	}
}

func TestOneScannerPerPlatformAndLeaseReleasedOnFailure(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	innerRefused := false
	err := h.pacer.WithScanner(ctx, domain.PlatformBilibili, func(ctx context.Context) error {
		inner := h.pacer.WithScanner(ctx, domain.PlatformBilibili, func(context.Context) error {
			t.Fatal("scanner thứ hai không được chạy")
			return nil
		})
		if inner == nil {
			t.Fatal("scanner thứ hai phải bị từ chối")
		}
		innerRefused = true
		return errors.New("crashed")
	})
	if err == nil || !strings.Contains(err.Error(), "crashed") {
		t.Fatalf("err = %v", err)
	}
	if !innerRefused {
		t.Fatal("scanner thứ hai phải bị từ chối")
	}
	if err := h.pacer.WithScanner(ctx, domain.PlatformBilibili, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("lượt quét sau: %v", err)
	}
	st, _, _ := h.store.One(ctx, domain.PlatformBilibili)
	if st.ScannerOwner != nil {
		t.Fatal("lease phải được nhả sau lượt quét")
	}
}

// codedRetryError mang Retry-After nhưng mã lỗi KHÔNG phải rate_limited.
type codedRetryError struct{}

func (codedRetryError) Error() string             { return "lỗi 500" }
func (codedRetryError) ErrorCode() string         { return "failed" }
func (codedRetryError) RetryAfter() time.Duration { return 30 * time.Minute }

// TestRetryAfterOnlyCountsForRateLimited: Retry-After trên lỗi khác không được
// biến thành giờ chờ.
func TestRetryAfterOnlyCountsForRateLimited(t *testing.T) {
	h := newHarness(t)
	err := h.pacer.Request(context.Background(), domain.PlatformBilibili, true, func(context.Context) error {
		return codedRetryError{}
	})
	var paused *platforms.PausedError
	if errors.As(err, &paused) {
		t.Fatalf("không được tạo PausedError từ lỗi không phải rate limit: %v", err)
	}
	if !errors.As(err, new(codedRetryError)) {
		t.Fatalf("phải trả nguyên lỗi gốc, được %v", err)
	}
	until, _, _ := h.store.Pause(context.Background(), domain.PlatformBilibili, h.now)
	if until != nil {
		t.Fatalf("không được ghi giờ chờ, được %v", until)
	}
}

// TestScannerLeaseBoundsRun: lượt quét chạy dưới deadline của lease.
func TestScannerLeaseBoundsRun(t *testing.T) {
	h := newHarness(t)
	var deadline time.Time
	err := h.pacer.WithScannerLease(context.Background(), domain.PlatformBilibili, 5*time.Minute, func(scanCtx context.Context) error {
		if d, ok := scanCtx.Deadline(); ok {
			deadline = d
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if deadline.IsZero() || deadline.Sub(h.now) > 5*time.Minute+time.Second {
		t.Fatalf("deadline = %v (now %v), muốn ~5 phút", deadline, h.now)
	}
}

// TestYtDlpRunsUnderLeaseDeadline: lệnh yt-dlp nhận ctx có deadline phủ
// timeout + đệm lease, kể cả khi phải chờ tới lượt.
func TestYtDlpRunsUnderLeaseDeadline(t *testing.T) {
	h := newHarness(t)
	var deadline time.Time
	err := h.pacer.RunYtDlp(context.Background(), domain.PlatformBilibili, 4, 30*time.Second, func(runCtx context.Context) error {
		if d, ok := runCtx.Deadline(); ok {
			deadline = d
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if deadline.IsZero() || deadline.Sub(h.now) < 30*time.Second {
		t.Fatalf("deadline = %v (now %v), phải phủ timeout", deadline, h.now)
	}
}
