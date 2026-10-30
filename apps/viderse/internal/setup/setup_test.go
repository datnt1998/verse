// Điều phối Chuẩn bị máy chép hành vi startSetup/retry của
// src/server/tools/setup.ts (yt-factory): cài theo thứ tự, lỗi một bước
// không chặn bước khác.
package setup

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

type fakeTools struct {
	mu        sync.Mutex
	checks    map[domain.SetupStepID]domain.SetupCheck
	ensureErr map[domain.SetupStepID]error
	ensured   []domain.SetupStepID
	gate      chan struct{} // nil = không chặn
}

func newFakeTools() *fakeTools {
	f := &fakeTools{checks: map[domain.SetupStepID]domain.SetupCheck{}, ensureErr: map[domain.SetupStepID]error{}}
	for _, id := range []domain.SetupStepID{
		domain.SetupStepYtDlp, domain.SetupStepFfmpeg, domain.SetupStepTranscription,
		domain.SetupStepChrome, domain.SetupStepCodex,
	} {
		f.checks[id] = domain.SetupCheck{Need: domain.SetupNeedInstall}
	}
	f.checks[domain.SetupStepCodexLogin] = domain.SetupCheck{Need: domain.SetupNeedLogin}
	return f
}

func (f *fakeTools) StepChecks(_ context.Context, _ bool) (map[domain.SetupStepID]domain.SetupCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[domain.SetupStepID]domain.SetupCheck, len(f.checks))
	for k, v := range f.checks {
		out[k] = v
	}
	return out, nil
}

func (f *fakeTools) Ensure(ctx context.Context, id domain.SetupStepID) error {
	f.mu.Lock()
	f.ensured = append(f.ensured, id)
	err := f.ensureErr[id]
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.checks[id] = domain.SetupCheck{Need: domain.SetupNeedReady}
	f.mu.Unlock()
	return nil
}

func (f *fakeTools) LiveProgress(domain.SetupStepID) *domain.InstallProgress { return nil }
func (f *fakeTools) RequiredBytes(domain.SetupStepID) int64                  { return 10 }
func (f *fakeTools) ApproxBytes(domain.SetupStepID) int64                    { return 20 }

func (f *fakeTools) order() []domain.SetupStepID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.SetupStepID(nil), f.ensured...)
}

func waitIdle(t *testing.T, svc *Service) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		done := svc.run == nil
		svc.mu.Unlock()
		if done {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("lượt cài không kết thúc")
}

func TestRunInstallsInOrderAndStopsAtLogin(t *testing.T) {
	f := newFakeTools()
	svc := New(context.Background(), f, t.TempDir(), nil)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)

	got := f.order()
	want := []domain.SetupStepID{
		domain.SetupStepYtDlp, domain.SetupStepFfmpeg, domain.SetupStepTranscription,
		domain.SetupStepChrome, domain.SetupStepCodex,
	}
	if len(got) != len(want) {
		t.Fatalf("thứ tự = %v, muốn %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("thứ tự = %v, muốn %v", got, want)
		}
	}
	st, err := svc.Status(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != OverallLogin {
		t.Fatalf("tổng = %q, muốn login", st.State)
	}
	last := st.Steps[len(st.Steps)-1]
	if last.ID != domain.SetupStepCodexLogin || last.State != StepNeedsUser {
		t.Fatalf("hàng đăng nhập = %+v", last)
	}
}

func TestRunFailureDoesNotStopOtherSteps(t *testing.T) {
	f := newFakeTools()
	f.ensureErr[domain.SetupStepFfmpeg] = errors.New("mạng bị ngắt")
	svc := New(context.Background(), f, t.TempDir(), nil)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)

	got := f.order()
	if len(got) != 5 {
		t.Fatalf("không chạy tiếp sau lỗi: %v", got)
	}
	st, _ := svc.Status(context.Background(), false)
	if st.State != OverallFailed {
		t.Fatalf("tổng = %q, muốn failed", st.State)
	}
	var ff Step
	for _, s := range st.Steps {
		if s.ID == domain.SetupStepFfmpeg {
			ff = s
		}
	}
	if ff.State != StepFailed || ff.Error != "mạng bị ngắt" {
		t.Fatalf("hàng ffmpeg = %+v", ff)
	}
}

func TestRetryRunsOnlyRequestedStep(t *testing.T) {
	f := newFakeTools()
	for _, id := range []domain.SetupStepID{
		domain.SetupStepYtDlp, domain.SetupStepFfmpeg, domain.SetupStepTranscription,
		domain.SetupStepCodex,
	} {
		f.checks[id] = domain.SetupCheck{Need: domain.SetupNeedReady}
	}
	f.ensureErr[domain.SetupStepChrome] = errors.New("hỏng")
	svc := New(context.Background(), f, t.TempDir(), nil)
	if _, err := svc.Retry(context.Background(), domain.SetupStepChrome); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	if got := f.order(); len(got) != 1 || got[0] != domain.SetupStepChrome {
		t.Fatalf("retry chạy %v, muốn chỉ chrome", got)
	}
	if _, err := svc.Retry(context.Background(), domain.SetupStepCodexLogin); err == nil {
		t.Fatal("retry hàng đăng nhập phải lỗi")
	}
}

func TestStartJoinsRunningRun(t *testing.T) {
	f := newFakeTools()
	f.gate = make(chan struct{})
	svc := New(context.Background(), f, t.TempDir(), nil)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Cho goroutine kịp vào Ensure đầu tiên.
	deadline := time.Now().Add(time.Second)
	for len(f.order()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st, _ := svc.Status(context.Background(), false); !st.Running {
		t.Fatal("Start phải đánh dấu đang chạy")
	}
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(f.gate)
	waitIdle(t, svc)
	if got := f.order(); len(got) != 5 {
		t.Fatalf("Start lặp chạy kép: %v", got)
	}
}

// Máy sạch: GET trả idle (không tác dụng phụ); POST start mới chạy, và gọi
// lại không tạo lượt mới.
func TestIdleThenStartIsIdempotent(t *testing.T) {
	tools := newFakeTools()
	svc := New(context.Background(), tools, "", nil)
	st, err := svc.Status(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != OverallIdle || st.Running {
		t.Fatalf("status = %+v, muốn idle", st)
	}
	if len(tools.order()) != 0 {
		t.Fatalf("GET đã chạy: %v", tools.order())
	}
	st, err = svc.Start(context.Background())
	if err != nil || st.State != OverallRunning {
		t.Fatalf("start = %+v, %v", st, err)
	}
	waitIdle(t, svc)
	if got := len(tools.order()); got != 5 {
		t.Fatalf("ensured = %v, muốn 5 bước", tools.order())
	}
	st, _ = svc.Status(context.Background(), false)
	if st.State != OverallLogin || st.Complete {
		t.Fatalf("sau cài = %+v, muốn login", st)
	}
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(tools.order()); got != 5 {
		t.Fatalf("start lặp chạy lại: %v", tools.order())
	}
}

// Sau lượt lỗi, Start không chạy lại — chỉ POST retry mới thử lại.
func TestStartDoesNotRestartAfterFailure(t *testing.T) {
	tools := newFakeTools()
	tools.ensureErr[domain.SetupStepYtDlp] = errors.New("mạng hỏng")
	svc := New(context.Background(), tools, "", nil)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	before := len(tools.order())
	st, _ := svc.Status(context.Background(), false)
	if st.State != OverallFailed {
		t.Fatalf("state = %q, muốn failed", st.State)
	}
	svc.Start(context.Background())
	waitIdle(t, svc)
	if got := len(tools.order()); got != before {
		t.Fatalf("tự chạy lại sau lỗi: %v", tools.order())
	}
}

// Shutdown chờ bước đang cài thoát (kể cả tiến trình con) trước khi trả.
func TestShutdownWaitsForRunningStep(t *testing.T) {
	tools := newFakeTools()
	tools.gate = make(chan struct{})
	svc := New(context.Background(), tools, "", nil)
	svc.Start(context.Background())

	short, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := svc.Shutdown(short); err == nil {
		t.Fatal("shutdown phải chờ bước đang chạy")
	}
	close(tools.gate)
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown sau khi bước xong: %v", err)
	}
}
