package ttsworker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// stateEngine is a fake reader whose readiness the test drives: it starts
// "missing", becomes installed, then serves after a Voices call.
type stateEngine struct {
	mu        sync.Mutex
	installed bool
	ready     bool
	starting  bool
	startErr  string
	voices    int
	voicesErr error
}

func (e *stateEngine) Name() string                     { return "state-test" }
func (e *stateEngine) Capabilities() voice.Capabilities { return voice.Capabilities{} }
func (e *stateEngine) Close() error                     { return nil }
func (e *stateEngine) AddVoice(context.Context, string, string) (voice.Voice, error) {
	return voice.Voice{}, errors.New("không dùng trong test")
}
func (e *stateEngine) Synthesize(context.Context, voice.SynthRequest) (voice.Audio, error) {
	return voice.Audio{}, errors.New("không dùng trong test")
}

func (e *stateEngine) Status(context.Context) voice.Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := voice.Status{Engine: e.Name()}
	switch {
	case e.ready:
		st.Ready, st.State = true, voice.StateReady
	case e.startErr != "":
		st.State, st.Error = voice.StateError, e.startErr
	case e.starting:
		st.State = voice.StateStarting
	case e.installed:
		st.State = voice.StateStopped
	default:
		st.State = voice.StateMissing
	}
	return st
}

func (e *stateEngine) Voices(context.Context) ([]voice.Voice, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.voices++
	if e.voicesErr != nil {
		e.startErr = e.voicesErr.Error()
		return nil, e.voicesErr
	}
	if e.installed {
		e.ready, e.starting = true, false
	}
	return nil, nil
}

func (e *stateEngine) Installed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.installed
}

func (e *stateEngine) calls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.voices
}

func (e *stateEngine) set(fn func(*stateEngine)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e)
}

// fakeProvisioner reports scripted milestones and can fail or block.
type fakeProvisioner struct {
	mu    sync.Mutex
	calls int
	steps []voice.ProvisionState
	err   error
	gate  chan struct{}
	onRun func()
}

func (p *fakeProvisioner) Provision(ctx context.Context, report voice.ProvisionReporter) error {
	p.mu.Lock()
	p.calls++
	steps, err, gate, onRun := p.steps, p.err, p.gate, p.onRun
	p.mu.Unlock()
	if onRun != nil {
		onRun()
	}
	for _, st := range steps {
		if report != nil {
			report(st)
		}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (p *fakeProvisioner) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// stateServer is the worker-listener surface the state machine uses: state
// posts, the operator command and the claim counter (a claim means "the worker
// thinks it may work").
type stateServer struct {
	mu         sync.Mutex
	states     []voice.ProvisionState
	ffmpeg     []bool
	claims     int
	commandSeq int64
	commandOp  string
	heartbeats []map[string]any
	srv        *httptest.Server
	stateCh    chan struct{}
}

func newStateServer(t *testing.T) *stateServer {
	t.Helper()
	s := &stateServer{stateCh: make(chan struct{}, 64)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/worker/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		var body statePayload
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.states = append(s.states, body.ProvisionState)
		s.ffmpeg = append(s.ffmpeg, body.FFmpeg)
		s.mu.Unlock()
		select {
		case s.stateCh <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /internal/worker/command", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		seq, op := s.commandSeq, s.commandOp
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"seq": seq, "op": op})
	})
	mux.HandleFunc("POST /internal/worker/claim", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.claims++
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /internal/worker/jobs/{id}/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.heartbeats = append(s.heartbeats, body)
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"fence": 1, "leaseUntil": time.Now().Add(time.Minute)})
	})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stateServer) setCommand(seq int64, op string) {
	s.mu.Lock()
	s.commandSeq, s.commandOp = seq, op
	s.mu.Unlock()
}

func (s *stateServer) snapshot() (states []voice.ProvisionState, claims int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]voice.ProvisionState(nil), s.states...), s.claims
}

func newStateWorker(t *testing.T, srv *stateServer, eng voice.Engine, prov Provisioner) *Worker {
	t.Helper()
	return newStateWorkerWith(t, srv, eng, prov, testToken, t.TempDir())
}

// newStateWorkerWith pins the credential and the durable state directory, so a
// test can build a replacement worker that shares a credential (a restart).
func newStateWorkerWith(t *testing.T, srv *stateServer, eng voice.Engine, prov Provisioner, token, dir string) *Worker {
	t.Helper()
	w, err := New(Config{
		APIURL: srv.srv.URL, Token: token, Engine: eng,
		Lease: 30 * time.Second, Heartbeat: 5 * time.Second, Poll: 5 * time.Millisecond,
		Provisioner: prov, StateDir: dir,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	return w
}

// runFor runs the worker loop for d (or until it stops on its own).
func runFor(t *testing.T, w *Worker, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	if err := w.Run(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run: %v", err)
	}
}

// runUntil runs the worker until cond holds (or the deadline passes), then stops
// it. Fixed sleeps make these tests depend on machine load; waiting for the
// observable effect does not.
func runUntil(t *testing.T, w *Worker, deadline time.Duration, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	limit := time.After(deadline)
	for !cond() {
		select {
		case <-limit:
			cancel()
			<-done
			t.Fatal("condition not reached in time")
		case <-time.After(2 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("run: %v", err)
	}
}

// A worker whose reader is not ready must not claim: work stays queued on the
// server instead of being failed. It must still tell the server why.
func TestWorkerDoesNotClaimBeforeReady(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{}
	prov := &fakeProvisioner{err: &voice.ProvisionError{Code: voice.ProvisionErrUVFailed, Message: "không tải được uv"}}
	w := newStateWorker(t, srv, eng, prov)

	// Stop as soon as the failure is published: by then the retry is scheduled,
	// so a second attempt would mean the backoff was ignored.
	runUntil(t, w, 5*time.Second, func() bool {
		states, _ := srv.snapshot()
		return len(states) > 0 && states[len(states)-1].State == voice.ProvisionStateError
	})

	states, claims := srv.snapshot()
	if claims != 0 {
		t.Fatalf("worker claimed %d job(s) while the reader was not ready", claims)
	}
	if len(states) == 0 {
		t.Fatal("worker never published its state")
	}
	last := states[len(states)-1]
	if last.State != voice.ProvisionStateError {
		t.Fatalf("last state = %q (%+v), want error", last.State, last)
	}
	if last.Error == nil || last.Error.Code != voice.ProvisionErrUVFailed {
		t.Fatalf("error = %+v, want code %q", last.Error, voice.ProvisionErrUVFailed)
	}
	if prov.count() != 1 {
		t.Fatalf("provision attempts = %d, want 1 (backoff must hold the retry)", prov.count())
	}
}

// A ready reader lets the worker claim.
func TestWorkerClaimsWhenReady(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{installed: true, ready: true}
	w := newStateWorker(t, srv, eng, &fakeProvisioner{})

	runUntil(t, w, 5*time.Second, func() bool {
		_, claims := srv.snapshot()
		return claims > 0
	})
	states, _ := srv.snapshot()
	if len(states) == 0 || states[len(states)-1].State != voice.ProvisionStateReady {
		t.Fatalf("states = %+v, want a ready state", states)
	}
}

// Every milestone the engine reports reaches the server, in order, with its
// step and percent intact.
func TestWorkerReportsProvisioningProgress(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{}
	gate := make(chan struct{})
	prov := &fakeProvisioner{
		gate: gate,
		steps: []voice.ProvisionState{
			{State: voice.ProvisionStateProvisioning, Step: voice.ProvisionStepTools, Percent: 0, Message: "Đang tải uv…"},
			{State: voice.ProvisionStateProvisioning, Step: voice.ProvisionStepDeps, Percent: 40, Message: "Đang cài thư viện…"},
			{State: voice.ProvisionStateProvisioning, Step: voice.ProvisionStepModel, Percent: 70, Message: "Đang tải mô hình…"},
		},
	}
	w := newStateWorker(t, srv, eng, prov)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		states, _ := srv.snapshot()
		if len(states) >= 3 {
			break
		}
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatalf("only %d state post(s) arrived: %+v", len(states), states)
		case <-time.After(2 * time.Millisecond):
		}
	}
	cancel()
	close(gate)
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("run: %v", err)
	}

	states, _ := srv.snapshot()
	var steps []string
	for _, st := range states {
		if st.State == voice.ProvisionStateProvisioning {
			steps = append(steps, st.Step)
		}
	}
	want := []string{voice.ProvisionStepTools, voice.ProvisionStepDeps, voice.ProvisionStepModel}
	if len(steps) < len(want) {
		t.Fatalf("steps = %v, want at least %v", steps, want)
	}
	for i, step := range want {
		if steps[i] != step {
			t.Fatalf("steps = %v, want prefix %v", steps, want)
		}
	}
	if states[1].Percent != 40 || states[2].Percent != 70 {
		t.Fatalf("percent lost in transit: %+v", states[:3])
	}
}

// One operator command provisions once: the same seq polled again does nothing.
func TestWorkerCommandProvisionsOnce(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{installed: true, ready: true}
	prov := &fakeProvisioner{}
	w := newStateWorker(t, srv, eng, prov)

	// Let the startup pin reconciliation run first; the command below is then
	// unambiguously the one extra provision.
	runUntil(t, w, 5*time.Second, func() bool { return prov.count() > 0 })
	base := prov.count()
	srv.setCommand(7, "install")
	runUntil(t, w, 5*time.Second, func() bool { return prov.count() > base })
	// Keep polling the same seq: it must not run a second time.
	runFor(t, w, 150*time.Millisecond)
	if got := prov.count(); got != base+1 {
		t.Fatalf("provision calls = %d, want %d for one command seq", got, base+1)
	}
}

// A long job must keep the reader state fresh: heartbeats carry it, and they
// carry the same complete payload as the state post — the server replaces the
// stored JSON on either path, so a slimmer beat would erase channel, ffmpeg and
// the preview capability on the first tick.
func TestWorkerHeartbeatCarriesState(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{installed: true, ready: true}
	w, err := New(Config{
		APIURL: srv.srv.URL, Token: testToken, Engine: eng,
		Lease: 30 * time.Second, Heartbeat: 5 * time.Millisecond, Poll: 5 * time.Millisecond,
		Preview: true, StateDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	w.markReady(voice.Status{Ready: true, State: voice.StateReady})

	ctx, cancel := context.WithCancel(context.Background())
	stop := w.heartbeat(ctx, job{ID: "j1", Fence: 3}, func() {})
	time.Sleep(30 * time.Millisecond)
	stop()

	srv.mu.Lock()
	beats := append([]map[string]any(nil), srv.heartbeats...)
	srv.mu.Unlock()
	if len(beats) == 0 {
		t.Fatal("no heartbeat was sent")
	}
	st, ok := beats[0]["state"].(map[string]any)
	if !ok {
		t.Fatalf("heartbeat body %v carries no state", beats[0])
	}
	if st["state"] != voice.ProvisionStateReady {
		t.Fatalf("heartbeat state = %v, want ready", st["state"])
	}
	if st["channel"] != srv.srv.URL {
		t.Fatalf("heartbeat state channel = %v, want %q", st["channel"], srv.srv.URL)
	}
	if _, ok := st["ffmpeg"].(bool); !ok {
		t.Fatalf("heartbeat state carries no ffmpeg fact: %v", st)
	}
	caps, _ := st["capabilities"].(map[string]any)
	if caps == nil || caps["preview"] != true {
		t.Fatalf("heartbeat state capabilities = %v, want preview", st["capabilities"])
	}
	if st["updatedAt"] == nil {
		t.Fatalf("heartbeat state carries no timestamp: %v", st)
	}

	// An error state must survive the same path: the heartbeat is the only
	// traffic while a job runs, so it is what the status card reads.
	w.setState(voice.ProvisionState{
		State: voice.ProvisionStateError, Percent: voice.ProvisionPercentUnknown,
		Engine: eng.Name(), Message: "tải mẫu giọng hỏng",
		Error: &voice.ProvisionError{Code: voice.ProvisionErrModelFailed, Message: "tải mẫu giọng hỏng"},
	})
	stop2 := w.heartbeat(ctx, job{ID: "j1", Fence: 3}, func() {})
	time.Sleep(20 * time.Millisecond)
	stop2()
	cancel()

	srv.mu.Lock()
	beats = append([]map[string]any(nil), srv.heartbeats...)
	srv.mu.Unlock()
	last, ok := beats[len(beats)-1]["state"].(map[string]any)
	if !ok || last["state"] != voice.ProvisionStateError {
		t.Fatalf("last heartbeat state = %v, want error", beats[len(beats)-1])
	}
	perr, _ := last["error"].(map[string]any)
	if perr == nil || perr["code"] != voice.ProvisionErrModelFailed {
		t.Fatalf("heartbeat lost the error code: %v", last["error"])
	}
	if last["channel"] != srv.srv.URL {
		t.Fatalf("error heartbeat lost the channel: %v", last["channel"])
	}
}

// A transient failure retries on the backoff; hash_mismatch is a safety signal
// (the bytes did not match the pin) and waits for an operator instead.
func TestWorkerHashMismatchNeverAutoRetries(t *testing.T) {
	for _, tc := range []struct {
		code  string
		retry bool
	}{
		{voice.ProvisionErrDownloadFailed, true},
		{voice.ProvisionErrHashMismatch, false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			srv := newStateServer(t)
			prov := &fakeProvisioner{err: &voice.ProvisionError{Code: tc.code, Message: "tải hỏng"}}
			w, err := New(Config{
				APIURL: srv.srv.URL, Token: testToken, Engine: &stateEngine{},
				Lease: 30 * time.Second, Heartbeat: 5 * time.Second, Poll: 5 * time.Millisecond,
				Provisioner:      prov,
				ProvisionBackoff: []time.Duration{time.Millisecond, time.Millisecond},
				StateDir:         t.TempDir(),
			})
			if err != nil {
				t.Fatalf("new worker: %v", err)
			}
			if tc.retry {
				runUntil(t, w, 5*time.Second, func() bool { return prov.count() >= 2 })
				return
			}
			runUntil(t, w, 5*time.Second, func() bool { return prov.count() >= 1 })
			// Many poll ticks past the 1 ms backoff: a retry would have run by now.
			runFor(t, w, 200*time.Millisecond)
			if got := prov.count(); got != 1 {
				t.Fatalf("provision attempts = %d, want 1: hash_mismatch must wait for an operator", got)
			}
			if st := w.State(); st.State != voice.ProvisionStateError || st.Error == nil || st.Error.Code != voice.ProvisionErrHashMismatch {
				t.Fatalf("state = %+v, want error/hash_mismatch kept", st)
			}
		})
	}
}

// Every published state carries the channel this machine dials and whether it
// takes on-demand previews: the server's status card reads both from here.
func TestWorkerPublishesChannelAndPreview(t *testing.T) {
	srv := newStateServer(t)
	w, err := New(Config{
		APIURL: srv.srv.URL, Token: testToken, Engine: &stateEngine{installed: true, ready: true},
		Lease: 30 * time.Second, Heartbeat: 5 * time.Second, Poll: 5 * time.Millisecond,
		Provisioner: &fakeProvisioner{}, Preview: true, StateDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	runUntil(t, w, 5*time.Second, func() bool {
		states, _ := srv.snapshot()
		return len(states) > 0
	})
	states, _ := srv.snapshot()
	last := states[len(states)-1]
	if last.Channel != srv.srv.URL {
		t.Fatalf("channel = %q, want %q", last.Channel, srv.srv.URL)
	}
	if last.Capabilities == nil || !last.Capabilities.Preview {
		t.Fatalf("capabilities = %+v, want preview", last.Capabilities)
	}
}

// A reader that stops after serving (idle shutdown, crash) must send the worker
// back to preparation/backoff instead of claiming a chapter the engine cannot
// read. Readiness is re-derived before every claim, not latched at the first
// success.
func TestWorkerStopsClaimingWhenReaderDies(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{installed: true, ready: true}
	w := newStateWorker(t, srv, eng, nil) // --no-provision: the machine restarts it itself

	runUntil(t, w, 5*time.Second, func() bool {
		_, claims := srv.snapshot()
		return claims > 0
	})
	// The engine dies and cannot be restarted.
	eng.set(func(e *stateEngine) {
		e.ready = false
		e.voicesErr = errors.New("máy đọc không khởi động lại được")
	})
	_, before := srv.snapshot()
	runFor(t, w, 200*time.Millisecond)
	states, after := srv.snapshot()
	if after != before {
		t.Fatalf("claims went %d → %d after the reader died: the worker claimed without a ready engine", before, after)
	}
	// Simulate the latched cache: the worker still believes it is ready while
	// the engine is dead. A claim must re-check the engine, not the cache, or
	// the chapter would be failed for a machine-side reason.
	w.markReady(voice.Status{Ready: true, State: voice.StateReady})
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if _, afterDirect := srv.snapshot(); afterDirect != after {
		t.Fatalf("RunOnce claimed a job with a dead reader (%d → %d)", after, afterDirect)
	}
	last := states[len(states)-1]
	if last.State != voice.ProvisionStateError || last.Error == nil || last.Error.Code != voice.ProvisionErrStartFailed {
		t.Fatalf("last state = %+v, want error/start_failed", last)
	}
}

// Files on disk are not proof of the pinned tag. With automatic provisioning
// on, the worker must run the idempotent provision pass before its first claim
// even though the reader is installed — otherwise a newer worker binary keeps
// serving the old source.
func TestWorkerReconcilesPinBeforeClaim(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{installed: true} // files exist, tag may be stale
	var once sync.Once
	provisioned := make(chan struct{})
	prov := &fakeProvisioner{onRun: func() {
		once.Do(func() { close(provisioned) })
		eng.set(func(e *stateEngine) { e.ready = true })
	}}
	w := newStateWorker(t, srv, eng, prov)

	runUntil(t, w, 5*time.Second, func() bool {
		_, claims := srv.snapshot()
		return claims > 0
	})
	select {
	case <-provisioned:
	default:
		t.Fatal("worker claimed before reconciling the pinned build")
	}
	if prov.count() == 0 {
		t.Fatal("installed files skipped the provision pass")
	}
}

// --no-provision must never download or start anything: the worker only reports
// the missing reader and waits for a human.
func TestWorkerNoProvisionNeverDownloads(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{} // nothing installed
	w := newStateWorker(t, srv, eng, nil)

	runFor(t, w, 150*time.Millisecond)
	states, claims := srv.snapshot()
	if claims != 0 {
		t.Fatalf("worker claimed %d job(s) without a reader", claims)
	}
	if eng.calls() != 0 {
		t.Fatalf("--no-provision started the reader %d time(s)", eng.calls())
	}
	if len(states) == 0 || states[len(states)-1].State != voice.ProvisionStateMissing {
		t.Fatalf("states = %+v, want a missing state", states)
	}
}

// A restart with the same credential must not replay the command the previous
// process already ran — replaying would also clear a hash_mismatch stop nobody
// overrode. A genuinely new seq still runs, and the marker never carries the
// token.
func TestWorkerCommandSeqSurvivesRestart(t *testing.T) {
	srv := newStateServer(t)
	dir := t.TempDir()
	eng := &stateEngine{installed: true, ready: true}
	p1 := &fakeProvisioner{}
	w1 := newStateWorkerWith(t, srv, eng, p1, testToken, dir)
	srv.setCommand(7, "install")
	if err := w1.pollCommand(context.Background()); err != nil {
		t.Fatalf("poll command: %v", err)
	}
	if p1.count() != 1 {
		t.Fatalf("first process provisioned %d time(s), want 1", p1.count())
	}

	p2 := &fakeProvisioner{}
	w2 := newStateWorkerWith(t, srv, eng, p2, testToken, dir)
	if got := w2.lastSeq; got != 7 {
		t.Fatalf("restored seq = %d, want 7", got)
	}
	if err := w2.pollCommand(context.Background()); err != nil {
		t.Fatalf("poll command after restart: %v", err)
	}
	if p2.count() != 0 {
		t.Fatalf("old command replayed after restart: %d provision(s)", p2.count())
	}

	// A new operator click must still run.
	srv.setCommand(8, "install")
	if err := w2.pollCommand(context.Background()); err != nil {
		t.Fatalf("poll new command: %v", err)
	}
	if p2.count() != 1 {
		t.Fatalf("new command provisioned %d time(s), want 1", p2.count())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read state dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("state dir holds %d entries, want one marker", len(entries))
	}
	if strings.Contains(entries[0].Name(), testToken) {
		t.Fatalf("state file name leaks the token: %q", entries[0].Name())
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatalf("stat marker: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("marker mode = %o, want 600", perm)
	}
}

// The durable acknowledgement is per credential: a second machine sharing the
// state directory has its own command history.
func TestWorkerCommandSeqIsPerCredential(t *testing.T) {
	srv := newStateServer(t)
	dir := t.TempDir()
	eng := &stateEngine{installed: true, ready: true}
	p1 := &fakeProvisioner{}
	w1 := newStateWorkerWith(t, srv, eng, p1, testToken, dir)
	srv.setCommand(7, "install")
	if err := w1.pollCommand(context.Background()); err != nil {
		t.Fatalf("poll command: %v", err)
	}
	if p1.count() != 1 {
		t.Fatalf("first credential provisioned %d time(s), want 1", p1.count())
	}

	p2 := &fakeProvisioner{}
	w2 := newStateWorkerWith(t, srv, eng, p2, "other-machine-token", dir)
	if err := w2.pollCommand(context.Background()); err != nil {
		t.Fatalf("poll command: %v", err)
	}
	if p2.count() != 1 {
		t.Fatalf("second credential provisioned %d time(s), want 1 (its own history)", p2.count())
	}
}

// The sequence is recorded before the install runs. If the marker cannot be
// written, the command waits for a later poll instead of running unrecorded and
// being replayed after a restart.
func TestWorkerCommandWaitsWhenAckCannotBePersisted(t *testing.T) {
	srv := newStateServer(t)
	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("prepare blocker: %v", err)
	}
	prov := &fakeProvisioner{}
	w := newStateWorkerWith(t, srv, &stateEngine{installed: true, ready: true}, prov, testToken, blocked)
	srv.setCommand(9, "install")

	if err := w.pollCommand(context.Background()); err == nil {
		t.Fatal("pollCommand ran a command whose sequence it could not record")
	}
	if prov.count() != 0 {
		t.Fatalf("command ran %d time(s) without a durable ack", prov.count())
	}
	if w.lastSeq != 0 {
		t.Fatalf("lastSeq = %d, want 0 after a failed ack", w.lastSeq)
	}
}
