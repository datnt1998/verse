// Luồng HTTP thật của bước Chuẩn bị máy: service thật + handler thật.
// Máy sạch → GET idle (không chạy gì) → POST start → running → POST lặp là
// no-op; sau lượt lỗi, POST start không tự chạy lại.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/setup"
)

type e2eTools struct {
	mu      sync.Mutex
	checks  map[domain.SetupStepID]domain.SetupCheck
	errs    map[domain.SetupStepID]error
	ensured []domain.SetupStepID
}

func newE2ETools() *e2eTools {
	f := &e2eTools{checks: map[domain.SetupStepID]domain.SetupCheck{}, errs: map[domain.SetupStepID]error{}}
	for _, id := range []domain.SetupStepID{
		domain.SetupStepYtDlp, domain.SetupStepFfmpeg, domain.SetupStepTranscription,
		domain.SetupStepChrome, domain.SetupStepCodex,
	} {
		f.checks[id] = domain.SetupCheck{Need: domain.SetupNeedInstall}
	}
	f.checks[domain.SetupStepCodexLogin] = domain.SetupCheck{Need: domain.SetupNeedLogin}
	return f
}

func (f *e2eTools) StepChecks(context.Context, bool) (map[domain.SetupStepID]domain.SetupCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[domain.SetupStepID]domain.SetupCheck, len(f.checks))
	for k, v := range f.checks {
		out[k] = v
	}
	return out, nil
}

func (f *e2eTools) Ensure(_ context.Context, id domain.SetupStepID) error {
	f.mu.Lock()
	f.ensured = append(f.ensured, id)
	err := f.errs[id]
	f.mu.Unlock()
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.checks[id] = domain.SetupCheck{Need: domain.SetupNeedReady}
	f.mu.Unlock()
	return nil
}

func (f *e2eTools) LiveProgress(domain.SetupStepID) *domain.InstallProgress { return nil }
func (f *e2eTools) RequiredBytes(domain.SetupStepID) int64                  { return 1 }
func (f *e2eTools) ApproxBytes(domain.SetupStepID) int64                    { return 2 }

func (f *e2eTools) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ensured)
}

func waitSetupIdle(t *testing.T, svc *setup.Service) setup.Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		st, err := svc.Status(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		if !st.Running && st.State != setup.OverallRunning {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("setup không kết thúc: %+v", st)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSetupHTTPFlowIdleStartAndNoRetry(t *testing.T) {
	tools := newE2ETools()
	svc := setup.New(context.Background(), tools, t.TempDir(), nil)
	tg := newTestGateway(t)
	MountSetup(tg.g, svc)
	cookie := tg.sessionCookie()

	get := func() setupWire {
		w := tg.do(http.MethodGet, "/api/setup/state", cookie, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET %d: %s", w.Code, w.Body.String())
		}
		var out setupWire
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	if got := get(); got.State != "idle" || got.Running || tools.count() != 0 {
		t.Fatalf("GET máy sạch = %+v (ensured %d), muốn idle", got, tools.count())
	}
	w := tg.do(http.MethodPost, "/api/setup/start", cookie, "{}")
	if w.Code != http.StatusAccepted {
		t.Fatalf("start %d: %s", w.Code, w.Body.String())
	}
	st := waitSetupIdle(t, svc)
	if st.State != setup.OverallLogin || tools.count() != 5 {
		t.Fatalf("sau start = %+v (ensured %d)", st, tools.count())
	}
	// POST lặp sau khi xong: không chạy lại.
	if w := tg.do(http.MethodPost, "/api/setup/start", cookie, "{}"); w.Code != http.StatusAccepted {
		t.Fatalf("start lặp %d", w.Code)
	}
	if tools.count() != 5 {
		t.Fatalf("start lặp chạy lại: %d", tools.count())
	}

	// Lượt lỗi: Start không tự chạy lại; GET trả failed.
	failTools := newE2ETools()
	failTools.errs[domain.SetupStepChrome] = errors.New("tải hỏng")
	failSvc := setup.New(context.Background(), failTools, t.TempDir(), nil)
	tg2 := newTestGateway(t)
	MountSetup(tg2.g, failSvc)
	cookie2 := tg2.sessionCookie()
	if w := tg2.do(http.MethodPost, "/api/setup/start", cookie2, "{}"); w.Code != http.StatusAccepted {
		t.Fatalf("start lỗi %d", w.Code)
	}
	if st := waitSetupIdle(t, failSvc); st.State != setup.OverallFailed {
		t.Fatalf("state = %+v, muốn failed", st)
	}
	before := failTools.count()
	if w := tg2.do(http.MethodPost, "/api/setup/start", cookie2, "{}"); w.Code != http.StatusAccepted {
		t.Fatalf("start sau lỗi %d", w.Code)
	}
	if got := failTools.count(); got != before {
		t.Fatalf("tự chạy lại sau lỗi: %d → %d", before, got)
	}
	// GET vẫn chỉ đọc.
	_ = tg2.do(http.MethodGet, "/api/setup/state", cookie2, "")
	if got := failTools.count(); got != before {
		t.Fatalf("GET đã chạy: %d → %d", before, got)
	}
}
