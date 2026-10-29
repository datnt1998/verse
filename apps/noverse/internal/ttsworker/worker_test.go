package ttsworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// fakeEngine returns deterministic WAV bytes per request and can block to
// exercise cancellation.
type fakeEngine struct {
	mu      sync.Mutex
	calls   int
	block   chan struct{} // when non-nil, Synthesize waits for ctx or this channel
	failOn  string
	release chan struct{}
}

func (f *fakeEngine) Name() string                                  { return "fake" }
func (f *fakeEngine) Capabilities() voice.Capabilities              { return voice.Capabilities{} }
func (f *fakeEngine) Status(context.Context) voice.Status           { return voice.Status{Ready: true} }
func (f *fakeEngine) Voices(context.Context) ([]voice.Voice, error) { return nil, nil }
func (f *fakeEngine) AddVoice(context.Context, string, string) (voice.Voice, error) {
	return voice.Voice{}, errors.New("unsupported")
}
func (f *fakeEngine) Close() error { return nil }

func (f *fakeEngine) Synthesize(ctx context.Context, req voice.SynthRequest) (voice.Audio, error) {
	f.mu.Lock()
	f.calls++
	block := f.block
	failOn := f.failOn
	f.mu.Unlock()
	if block != nil {
		select {
		case <-ctx.Done():
			return voice.Audio{}, ctx.Err()
		case <-block:
		}
	}
	if failOn != "" && strings.Contains(req.Text, failOn) {
		return voice.Audio{}, errors.New("engine boom")
	}
	// "WAV" payload derived from the text so each segment has a distinct hash.
	return voice.Audio{WAV: []byte("WAV:" + req.Text), SampleRate: 24000, DurationSec: 0.5}, nil
}

func (f *fakeEngine) synthesizeCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeServer implements the worker protocol surface the client uses, with
// counters so tests can assert exactly which publishes happened.
type fakeServer struct {
	mu           sync.Mutex
	jobs         []job
	claims       int
	uploads      map[string][]byte
	uploadsSHA   map[string]string
	completed    []map[string]any
	released     []int64
	failed       int
	forceFence   bool // complete/fail/upload answer 409
	heartbeat    int
	unauthorized int
	srv          *httptest.Server
}

func newFakeServer(t *testing.T, jobs ...job) *fakeServer {
	t.Helper()
	fs := &fakeServer{jobs: jobs, uploads: map[string][]byte{}, uploadsSHA: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/worker/claim", fs.handleClaim)
	mux.HandleFunc("POST /internal/worker/jobs/{id}/blob", fs.handleUpload)
	mux.HandleFunc("POST /internal/worker/jobs/{id}/complete", fs.handleComplete)
	mux.HandleFunc("POST /internal/worker/jobs/{id}/heartbeat", fs.handleHeartbeat)
	mux.HandleFunc("POST /internal/worker/jobs/{id}/release", fs.handleRelease)
	mux.HandleFunc("POST /internal/worker/jobs/{id}/fail", fs.handleFail)
	fs.srv = httptest.NewServer(mux)
	t.Cleanup(fs.srv.Close)
	return fs
}

func (f *fakeServer) authOK(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer "+testToken {
		f.mu.Lock()
		f.unauthorized++
		f.mu.Unlock()
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return false
	}
	return true
}

const testToken = "test-token-0123456789abcdefghijklmnop"

// Fixed tenant ids for the fake upload endpoint (keys are tenant-scoped UUIDs).
const (
	testProjectID = "11111111-1111-4111-8111-111111111111"
	testStoryID   = "22222222-2222-4222-8222-222222222222"
)

func (f *fakeServer) handleClaim(w http.ResponseWriter, r *http.Request) {
	if !f.authOK(w, r) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++
	if len(f.jobs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	j := f.jobs[0]
	f.jobs = f.jobs[1:]
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"job": j})
}

func (f *fakeServer) jobByID(id string) (job, bool) {
	for _, j := range f.jobs {
		if j.ID == id {
			return j, true
		}
	}
	return job{ID: id}, true // the fake treats unknown ids as the in-flight job
}

func (f *fakeServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if !f.authOK(w, r) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceFence {
		http.Error(w, `{"error":"fenced"}`, http.StatusConflict)
		return
	}
	body, _ := io.ReadAll(r.Body)
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	if sha != r.Header.Get("X-Sha256") {
		http.Error(w, `{"error":"sha mismatch"}`, http.StatusBadRequest)
		return
	}
	// The real server derives the key; the fake only has to hand back an
	// opaque key the client echoes in its ACK, so it uses the real builder
	// with fixed test UUIDs (no key literal in the test).
	key, err := blobkey.Keys{}.AudioSegment(testProjectID, testStoryID, sha, "wav")
	if err != nil {
		http.Error(w, `{"error":"key"}`, http.StatusInternalServerError)
		return
	}
	f.uploads[key] = body
	f.uploadsSHA[key] = sha
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"key": key, "sha256": sha, "size": len(body)})
}

func (f *fakeServer) handleComplete(w http.ResponseWriter, r *http.Request) {
	if !f.authOK(w, r) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceFence {
		http.Error(w, `{"error":"fenced"}`, http.StatusConflict)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad"}`, http.StatusBadRequest)
		return
	}
	f.completed = append(f.completed, body)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"job": map[string]any{"id": r.PathValue("id"), "state": "completed"}})
}

func (f *fakeServer) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !f.authOK(w, r) {
		return
	}
	f.mu.Lock()
	f.heartbeat++
	forced := f.forceFence
	f.mu.Unlock()
	if forced {
		http.Error(w, `{"error":"fenced"}`, http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"fence": 1, "leaseUntil": time.Now().Add(time.Minute)})
}

func (f *fakeServer) handleRelease(w http.ResponseWriter, r *http.Request) {
	if !f.authOK(w, r) {
		return
	}
	var body struct {
		Fence int64 `json:"fence"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	f.released = append(f.released, body.Fence)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"job": map[string]any{"state": "queued"}})
}

func (f *fakeServer) handleFail(w http.ResponseWriter, r *http.Request) {
	if !f.authOK(w, r) {
		return
	}
	f.mu.Lock()
	f.failed++
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"job": map[string]any{"state": "failed"}})
}

func snapshotInput(t *testing.T, kind string, segments ...Segment) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(Snapshot{
		Kind: kind, ChapterID: "55555555-5555-4555-8555-555555555555",
		ScriptRevision: 1, Segments: segments,
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	return body
}

func newTestWorker(t *testing.T, fs *fakeServer, engine voice.Engine) *Worker {
	t.Helper()
	w, err := New(Config{
		APIURL: fs.srv.URL, Token: testToken, Engine: engine,
		Lease: 120 * time.Second, Heartbeat: 50 * time.Millisecond, Poll: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	return w
}

// TestWorkerClaim: the worker claims, synthesizes every segment, uploads the
// bytes with their hash and ACKs the job with the fence and attachments.
func TestWorkerClaim(t *testing.T) {
	input := snapshotInput(t, "create",
		Segment{ID: "seg-1", Text: "Câu một.", Voice: "vieneu-nu-bac", Speed: 1},
		Segment{ID: "seg-2", Text: "Câu hai.", Voice: "vieneu-nu-bac", Speed: 1.1},
	)
	fs := newFakeServer(t, job{ID: "job-1", ProjectID: "p", Kind: "tts", State: "running", Fence: 3, Input: input})
	engine := &fakeEngine{}
	w := newTestWorker(t, fs, engine)

	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !processed {
		t.Fatal("expected a claimed job")
	}
	if engine.synthesizeCalls() != 2 {
		t.Fatalf("synthesize calls = %d, want 2", engine.synthesizeCalls())
	}
	if len(fs.uploads) != 2 {
		t.Fatalf("uploads = %d, want 2", len(fs.uploads))
	}
	for key, body := range fs.uploads {
		sum := sha256.Sum256(body)
		if fs.uploadsSHA[key] != hex.EncodeToString(sum[:]) {
			t.Fatalf("upload %s hash mismatch", key)
		}
	}
	if len(fs.completed) != 1 {
		t.Fatalf("completed = %d, want 1", len(fs.completed))
	}
	ack := fs.completed[0]
	if ack["fence"] != float64(3) {
		t.Fatalf("ack fence = %v, want 3", ack["fence"])
	}
	atts, _ := ack["attachments"].([]any)
	if len(atts) != 2 {
		t.Fatalf("ack attachments = %d, want 2", len(atts))
	}
	first, _ := atts[0].(map[string]any)
	if first["segmentId"] != "seg-1" || first["chapterId"] != "55555555-5555-4555-8555-555555555555" || first["select"] != true {
		t.Fatalf("first attachment = %v", first)
	}
	if first["synthKey"] == "" || len(first["synthKey"].(string)) != 64 {
		t.Fatalf("synth key = %v", first["synthKey"])
	}
	// An update job must not silently replace the selected render.
	if fs.failed != 0 || len(fs.released) != 0 {
		t.Fatalf("unexpected fail/release: failed=%d released=%v", fs.failed, fs.released)
	}
}

// TestWorkerFence: a 409 on any publish means the job was taken away; the
// worker stops immediately, reports ErrFenced and never marks the job failed.
func TestWorkerFence(t *testing.T) {
	input := snapshotInput(t, "create", Segment{ID: "seg-1", Text: "Câu một."})
	fs := newFakeServer(t, job{ID: "job-1", Kind: "tts", State: "running", Fence: 5, Input: input})
	fs.forceFence = true
	w := newTestWorker(t, fs, &fakeEngine{})

	processed, err := w.RunOnce(context.Background())
	if !processed {
		t.Fatal("expected a claim")
	}
	if !errors.Is(err, ErrFenced) {
		t.Fatalf("err = %v, want ErrFenced", err)
	}
	if len(fs.uploads) != 0 {
		t.Fatalf("stale worker uploaded %d objects", len(fs.uploads))
	}
	if len(fs.completed) != 0 || fs.failed != 0 {
		t.Fatalf("stale worker published: completed=%d failed=%d", len(fs.completed), fs.failed)
	}
}

// TestWorkerCancellation: interrupting a running job releases the lease (the
// work is claimable again) and never completes or fails it.
func TestWorkerCancellation(t *testing.T) {
	input := snapshotInput(t, "create", Segment{ID: "seg-1", Text: "Câu dài."})
	fs := newFakeServer(t, job{ID: "job-1", Kind: "tts", State: "running", Fence: 9, Input: input})
	engine := &fakeEngine{block: make(chan struct{})}
	w := newTestWorker(t, fs, engine)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.process(ctx, job{ID: "job-1", Fence: 9, Input: input}) }()

	// Wait until the engine is actually synthesizing, then interrupt.
	deadline := time.Now().Add(2 * time.Second)
	for engine.synthesizeCalls() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cancelled job returned %v, want nil after release", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	if len(fs.released) != 1 || fs.released[0] != 9 {
		t.Fatalf("released = %v, want [9]", fs.released)
	}
	if len(fs.completed) != 0 || fs.failed != 0 {
		t.Fatalf("cancelled job published: completed=%d failed=%d", len(fs.completed), fs.failed)
	}
}

// TestWorkerEngineFailure: a synthesis error is reported as a job failure with
// the fence, not as a silent success.
func TestWorkerEngineFailure(t *testing.T) {
	input := snapshotInput(t, "create", Segment{ID: "seg-1", Text: "Câu hỏng."})
	fs := newFakeServer(t, job{ID: "job-1", Kind: "tts", State: "running", Fence: 2, Input: input})
	w := newTestWorker(t, fs, &fakeEngine{failOn: "hỏng"})

	if _, err := w.RunOnce(context.Background()); err == nil {
		t.Fatal("expected an engine error")
	}
	if fs.failed != 1 {
		t.Fatalf("failed reports = %d, want 1", fs.failed)
	}
	if len(fs.completed) != 0 {
		t.Fatal("failed job was completed")
	}
}

// TestWorkerRequiresToken: a worker without a token refuses to start (the
// secret is env-only, never argv).
func TestWorkerRequiresToken(t *testing.T) {
	if _, err := New(Config{APIURL: "http://127.0.0.1:8081", Engine: &fakeEngine{}}); err == nil {
		t.Fatal("New accepted an empty token")
	}
	if _, err := New(Config{APIURL: "", Token: testToken, Engine: &fakeEngine{}}); err == nil {
		t.Fatal("New accepted an empty API URL")
	}
}
