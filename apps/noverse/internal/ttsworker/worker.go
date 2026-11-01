// Package ttsworker is the Mac-side pull worker: it claims TTS jobs from the
// VPS private listener, synthesizes each segment with the local engine,
// uploads the immutable audio objects and ACKs them with the hash and fencing
// token. It never listens for inbound connections, never talks to Postgres and
// never sends the service token anywhere except the configured API URL.
//
// Failure model (phase 07):
//   - a killed process simply stops heartbeating; the lease lapses and another
//     worker (or the restarted one) reclaims the job;
//   - a worker whose fence moved (cancel, retry, reclaim) gets 409/404 on any
//     publish and stops instead of retrying;
//   - Ctrl-C/EOT during a job releases the lease so the job is claimable
//     immediately, while progress already reported is preserved.
package ttsworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// Errors surfaced to the CLI.
var (
	// ErrFenced means the job no longer belongs to this worker: it was
	// cancelled, retried, or reclaimed after the lease lapsed. Never retry.
	ErrFenced = errors.New("quyền xử lý việc đã mất")
	// ErrTransport is a bounded-retry network failure (the job is left to the
	// lease, not failed, because the server may still consider it ours).
	ErrTransport = errors.New("mất kết nối tới máy chủ việc")
)

// Config is the worker's runtime configuration. Token comes from the
// environment (TTS_WORKER_TOKEN); it is never taken from argv.
type Config struct {
	APIURL  string
	Token   string
	Engine  voice.Engine
	Project string // optional: only claim jobs of this project
	Lease   time.Duration
	// Heartbeat defaults to a third of the lease (plan: 30 s for a 120 s lease).
	Heartbeat time.Duration
	// Poll is the idle backoff between empty claims.
	Poll time.Duration
	// HTTP is optional; a bounded default client is built when nil.
	HTTP *http.Client
	// Provisioner installs the reader when it is missing. Nil (--no-provision)
	// keeps the worker from installing anything: it still reports the state and
	// claims only once the engine is ready.
	Provisioner Provisioner
	// ProvisionBackoff is the wait before each retry of a failed provision; the
	// number of delays + 1 bounds the attempts of one start.
	ProvisionBackoff []time.Duration
	// Preview advertises whether the server may ask this machine for an
	// on-demand voice preview (`capabilities.preview`). The Mac build leaves it
	// false until the server has a reverse preview path: the UI then hides
	// Nghe thử instead of offering a button that cannot work (10-audio.md §8.4).
	Preview bool
	// StateEvery throttles percent-only state posts (default 2 s).
	StateEvery time.Duration
	// Now is the clock; tests replace it.
	Now func() time.Time
}

// Worker is the claim → synthesize → upload → ACK loop.
type Worker struct {
	cfg    Config
	base   *url.URL
	client *http.Client
	ffmpeg bool

	// mu guards the provision state the run loop publishes and the operator
	// command sequence it has already executed.
	mu          sync.Mutex
	state       voice.ProvisionState
	ready       bool
	attempts    int
	exhausted   bool
	nextAttempt time.Time
	sent        voice.ProvisionState
	sentAt      time.Time
	sentOnce    bool
	lastSeq     int64
}

// New validates the configuration and returns a worker.
func New(cfg Config) (*Worker, error) {
	raw := strings.TrimSpace(cfg.APIURL)
	if raw == "" {
		return nil, errors.New("thiếu địa chỉ API worker (--api)")
	}
	base, err := url.Parse(raw)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("địa chỉ API worker không hợp lệ: %q", raw)
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("thiếu TTS_WORKER_TOKEN")
	}
	if cfg.Engine == nil {
		return nil, errors.New("chưa cấu hình máy đọc")
	}
	if cfg.Lease <= 0 {
		cfg.Lease = 120 * time.Second
	}
	if cfg.Heartbeat <= 0 {
		cfg.Heartbeat = cfg.Lease / 4
	}
	if cfg.Heartbeat >= cfg.Lease {
		cfg.Heartbeat = cfg.Lease / 2
	}
	if cfg.Poll <= 0 {
		cfg.Poll = 2 * time.Second
	}
	if cfg.StateEvery <= 0 {
		cfg.StateEvery = 2 * time.Second
	}
	if cfg.ProvisionBackoff == nil {
		cfg.ProvisionBackoff = defaultProvisionBackoff
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	_, ffmpegErr := exec.LookPath("ffmpeg")
	w := &Worker{cfg: cfg, base: base, client: client, ffmpeg: ffmpegErr == nil}
	w.state = voice.ProvisionState{
		State: voice.ProvisionStateMissing, Percent: voice.ProvisionPercentUnknown,
		Engine: cfg.Engine.Name(),
	}
	return w, nil
}

// protocol shapes (wire contract with internal/entry/web/worker_api.go)

type job struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"projectId"`
	StoryID    string          `json:"storyId"`
	Kind       string          `json:"kind"`
	State      string          `json:"state"`
	Input      json.RawMessage `json:"input"`
	Fence      int64           `json:"fence"`
	LeaseUntil time.Time       `json:"leaseUntil"`
}

type jobEnvelope struct {
	Job job `json:"job"`
}

// Snapshot is the frozen job input written at enqueue time.
type Snapshot struct {
	// Kind is the Gói AK job kind: create | update | redo.
	Kind           string    `json:"kind"`
	ChapterID      string    `json:"chapterId"`
	ScriptRevision int32     `json:"scriptRevision"`
	Segments       []Segment `json:"segments"`
}

// Segment is one speakable unit with the exact synthesis parameters.
type Segment struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Speaker  string  `json:"speaker"`
	Voice    string  `json:"voice"`
	Tone     string  `json:"tone"`
	Quality  string  `json:"quality"`
	Speed    float64 `json:"speed"`
	Pitch    float64 `json:"pitch"`
	Engine   string  `json:"engine"`
	Model    string  `json:"model"`
	SynthKey string  `json:"synthKey"`
	Select   *bool   `json:"select"`
}

type attachment struct {
	SegmentID      string          `json:"segmentId"`
	SynthKey       string          `json:"synthKey"`
	Key            string          `json:"key"`
	SHA256         string          `json:"sha256"`
	DurationSec    float64         `json:"durationSec"`
	SampleRate     int32           `json:"sampleRate"`
	Parameters     json.RawMessage `json:"parameters,omitempty"`
	ChapterID      string          `json:"chapterId"`
	ScriptRevision int32           `json:"scriptRevision"`
	Select         bool            `json:"select"`
}

// Run claims and processes jobs until ctx is cancelled. It only claims while
// the reader is ready: a worker whose engine is still being prepared leaves the
// work queued (never failed) and keeps the server informed instead. An empty
// queue backs off by cfg.Poll.
func (w *Worker) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if !w.Ready() {
			w.prepare(ctx)
			if ctx.Err() != nil {
				return nil
			}
			if !w.Ready() {
				if !sleepCtx(ctx, w.cfg.Poll) {
					return nil
				}
				continue
			}
			continue
		}
		// An operator's install command must be seen even while we are busy
		// claiming; a command re-provisions before the next claim.
		_ = w.pollCommand(ctx)
		if !w.Ready() {
			continue
		}
		processed, err := w.RunOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			// A cancelled run surfaces as a transport error mid-request; that is
			// a shutdown (launchd/SIGTERM), not a failure to report.
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if !processed {
			if !sleepCtx(ctx, w.cfg.Poll) {
				return nil
			}
		}
	}
}

// RunOnce claims at most one job and processes it. It reports whether a job
// was claimed (false = queue empty).
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	claimed, err := w.claim(ctx)
	if err != nil {
		if errors.Is(err, errNoJob) {
			return false, nil
		}
		return false, err
	}
	return true, w.process(ctx, claimed)
}

var errNoJob = errors.New("không có việc")

func (w *Worker) claim(ctx context.Context) (job, error) {
	body := map[string]any{"leaseSec": int(w.cfg.Lease / time.Second)}
	if w.cfg.Project != "" {
		body["projectId"] = w.cfg.Project
	}
	payload, _ := json.Marshal(body)
	status, raw, err := w.do(ctx, http.MethodPost, "/internal/worker/claim", payload, nil)
	if err != nil {
		return job{}, err
	}
	if status == http.StatusNoContent {
		return job{}, errNoJob
	}
	if status != http.StatusOK {
		return job{}, fmt.Errorf("claim: %s", apiError(status, raw))
	}
	var env jobEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return job{}, fmt.Errorf("claim: phản hồi không đọc được: %w", err)
	}
	if env.Job.ID == "" {
		return job{}, errNoJob
	}
	return env.Job, nil
}

// process synthesizes every segment, uploads it and ACKs the job.
func (w *Worker) process(parent context.Context, j job) error {
	var snap Snapshot
	if err := json.Unmarshal(j.Input, &snap); err != nil {
		return w.failJob(parent, j, fmt.Errorf("input không đọc được: %w", err))
	}
	if len(snap.Segments) == 0 {
		return w.failJob(parent, j, errors.New("job không có đoạn nào để đọc"))
	}

	jobCtx, cancel := context.WithCancel(parent)
	defer cancel()
	fenced := make(chan struct{})
	var fenceOnce sync.Once
	stop := w.heartbeat(jobCtx, j, func() {
		fenceOnce.Do(func() { close(fenced) })
		cancel()
	})
	defer stop()

	selectValue := snap.Kind != "update"
	atts := make([]attachment, 0, len(snap.Segments))
	for i, seg := range snap.Segments {
		if err := jobCtx.Err(); err != nil {
			// Fenced or interrupted: never publish partial work.
			if isFenced(fenced) || errors.Is(parent.Err(), context.Canceled) {
				return w.abandon(parent, j, isFenced(fenced))
			}
			return w.abandon(parent, j, false)
		}
		att, err := w.synthesizeOne(jobCtx, j, snap, seg, selectValue)
		if err != nil {
			if isFenced(fenced) || errors.Is(err, ErrFenced) {
				return ErrFenced
			}
			if parent.Err() != nil {
				return w.abandon(parent, j, false)
			}
			return w.failJob(parent, j, fmt.Errorf("đoạn %d: %w", i+1, err))
		}
		atts = append(atts, att)
	}

	body, _ := json.Marshal(map[string]any{
		"fence":        j.Fence,
		"progressDone": len(atts),
		"result":       map[string]any{"segments": len(atts), "kind": snap.Kind},
		"attachments":  atts,
	})
	status, raw, err := w.do(parent, http.MethodPost, "/internal/worker/jobs/"+j.ID+"/complete", body, nil)
	if err != nil {
		// Transport failure: leave the job to the lease rather than reporting a
		// false failure — the server may already have committed the ACK.
		return err
	}
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusConflict, http.StatusNotFound:
		return ErrFenced
	default:
		return fmt.Errorf("complete: %s", apiError(status, raw))
	}
}

// synthesizeOne renders and uploads a single segment.
func (w *Worker) synthesizeOne(ctx context.Context, j job, snap Snapshot, seg Segment, selectDefault bool) (attachment, error) {
	req := voice.SynthRequest{
		Text: seg.Text, VoiceID: seg.Voice, Tone: seg.Tone,
		Quality: seg.Quality, Speed: seg.Speed,
	}.Normalized()
	rendered, err := w.cfg.Engine.Synthesize(ctx, req)
	if err != nil {
		return attachment{}, err
	}
	if len(rendered.WAV) == 0 {
		return attachment{}, errors.New("máy đọc trả về tệp rỗng")
	}
	sum := sha256.Sum256(rendered.WAV)
	sha := hex.EncodeToString(sum[:])
	key, err := w.upload(ctx, j, sha, rendered.WAV)
	if err != nil {
		return attachment{}, err
	}
	synthKey := seg.SynthKey
	if synthKey == "" {
		synthKey = audio.SynthKey{
			Text: seg.Text, VoiceID: seg.Voice, Tone: seg.Tone,
			Speed: req.Speed, Pitch: seg.Pitch, Quality: req.Quality,
			Engine: seg.Engine, Model: seg.Model,
		}.Hash()
	}
	selected := selectDefault
	if seg.Select != nil {
		selected = *seg.Select
	}
	sampleRate := int32(rendered.SampleRate)
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	params, _ := json.Marshal(map[string]any{
		"voice": seg.Voice, "tone": seg.Tone, "speed": req.Speed, "pitch": seg.Pitch,
		"quality": req.Quality, "engine": seg.Engine, "model": seg.Model,
	})
	return attachment{
		SegmentID: seg.ID, SynthKey: synthKey, Key: key, SHA256: sha,
		DurationSec: rendered.DurationSec, SampleRate: sampleRate, Parameters: params,
		ChapterID: snap.ChapterID, ScriptRevision: snap.ScriptRevision, Select: selected,
	}, nil
}

func (w *Worker) upload(ctx context.Context, j job, sha string, wav []byte) (string, error) {
	headers := map[string]string{
		"Content-Type":   "audio/wav",
		"X-Sha256":       sha,
		"X-Worker-Fence": fmt.Sprintf("%d", j.Fence),
	}
	status, raw, err := w.do(ctx, http.MethodPost, "/internal/worker/jobs/"+j.ID+"/blob", wav, headers)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		if status == http.StatusConflict || status == http.StatusNotFound {
			return "", ErrFenced
		}
		return "", fmt.Errorf("upload: %s", apiError(status, raw))
	}
	var stored struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Key == "" {
		return "", fmt.Errorf("upload: phản hồi không đọc được")
	}
	return stored.Key, nil
}

// heartbeat renews the lease; a fenced answer cancels the in-flight job.
func (w *Worker) heartbeat(ctx context.Context, j job, fenced func()) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(w.cfg.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				body, _ := json.Marshal(map[string]any{
					"fence": j.Fence, "leaseSec": int(w.cfg.Lease / time.Second),
					// A long job must not let the reader's state go stale: the
					// heartbeat is the only traffic while a job runs.
					"state": w.State(),
				})
				// A heartbeat failure that is not a fence answer is transient:
				// the lease still covers us until the next tick.
				status, _, err := w.do(ctx, http.MethodPost, "/internal/worker/jobs/"+j.ID+"/heartbeat", body, nil)
				if err != nil {
					continue
				}
				if status == http.StatusConflict || status == http.StatusNotFound {
					fenced()
					return
				}
			}
		}
	}()
	return func() { cancel(); <-done }
}

// abandon ends an interrupted job: a fenced job is left alone, otherwise the
// lease is released so the work is claimable immediately.
func (w *Worker) abandon(parent context.Context, j job, fenced bool) error {
	if fenced {
		return ErrFenced
	}
	body, _ := json.Marshal(map[string]any{"fence": j.Fence})
	// Use a short background context: the job context is already cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if status, _, err := w.do(ctx, http.MethodPost, "/internal/worker/jobs/"+j.ID+"/release", body, nil); err != nil {
		return err
	} else if status != http.StatusOK && status != http.StatusConflict && status != http.StatusNotFound {
		return fmt.Errorf("release: HTTP %d", status)
	}
	return nil
}

func (w *Worker) failJob(ctx context.Context, j job, cause error) error {
	body, _ := json.Marshal(map[string]any{
		"fence": j.Fence,
		"error": map[string]any{"message": cause.Error()},
	})
	callCtx := ctx
	if callCtx.Err() != nil {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	status, raw, err := w.do(callCtx, http.MethodPost, "/internal/worker/jobs/"+j.ID+"/fail", body, nil)
	if err != nil {
		return errors.Join(cause, err)
	}
	if status == http.StatusConflict || status == http.StatusNotFound {
		return errors.Join(cause, ErrFenced)
	}
	if status != http.StatusOK {
		return errors.Join(cause, fmt.Errorf("fail: %s", apiError(status, raw)))
	}
	return cause
}

// do performs one authenticated request. The token only ever travels in the
// Authorization header.
func (w *Worker) do(ctx context.Context, method, path string, body []byte, headers map[string]string) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, w.base.String()+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.cfg.Token)
	if body != nil && headers == nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, fmt.Errorf("%w: %v", ErrTransport, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, raw, nil
}

func apiError(status int, raw []byte) string {
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error != "" {
		return payload.Error
	}
	return fmt.Sprintf("HTTP %d", status)
}

func isFenced(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
