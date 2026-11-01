package ttsworker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	w, err := New(Config{
		APIURL: srv.srv.URL, Token: testToken, Engine: eng,
		Lease: 30 * time.Second, Heartbeat: 5 * time.Second, Poll: 5 * time.Millisecond,
		Provisioner: prov,
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
	if err := w.Run(ctx); err != nil {
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

	runFor(t, w, 120*time.Millisecond)

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

	runFor(t, w, 60*time.Millisecond)

	if _, claims := srv.snapshot(); claims == 0 {
		t.Fatal("ready worker never claimed")
	}
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
	srv.setCommand(7, "install")

	runFor(t, w, 150*time.Millisecond)

	if got := prov.count(); got != 1 {
		t.Fatalf("provision calls = %d, want exactly 1 for one command seq", got)
	}
}

// A long job must keep the reader state fresh: heartbeats carry it.
func TestWorkerHeartbeatCarriesState(t *testing.T) {
	srv := newStateServer(t)
	eng := &stateEngine{installed: true, ready: true}
	w, err := New(Config{
		APIURL: srv.srv.URL, Token: testToken, Engine: eng,
		Lease: 30 * time.Second, Heartbeat: 5 * time.Millisecond, Poll: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	w.markReady(voice.Status{Ready: true, State: voice.StateReady})

	ctx, cancel := context.WithCancel(context.Background())
	stop := w.heartbeat(ctx, job{ID: "j1", Fence: 3}, func() {})
	time.Sleep(30 * time.Millisecond)
	stop()
	cancel()

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
}
