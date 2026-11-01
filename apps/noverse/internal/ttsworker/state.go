package ttsworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// Provisioner is the engine-side install entry point: it installs everything
// the reader needs (pinned toolchain, source, dependencies, model) and reports
// progress through the shared voice.ProvisionReporter. It must be idempotent —
// when the pinned build is already installed it returns in milliseconds
// without touching the network.
type Provisioner interface {
	Provision(ctx context.Context, report voice.ProvisionReporter) error
}

// defaultProvisionBackoff is the wait before each retry of a failed provision:
// 30 s → 1 m → 2 m → 5 m → 10 m, which bounds one start to six attempts. After
// that the worker keeps the error and waits for an operator command (or the
// next launchd restart).
var defaultProvisionBackoff = []time.Duration{
	30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute,
}

// stateRequestTimeout bounds a state/command exchange: a slow server must never
// stall the install for the length of the media client timeout.
const stateRequestTimeout = 15 * time.Second

// channel is the address this worker dials (the "Kênh nối" line in Nâng cao).
// It is the configured API URL with credentials stripped: it identifies the
// server, not the machine.
func (w *Worker) channel() string {
	if w.base == nil {
		return ""
	}
	return w.base.Scheme + "://" + w.base.Host
}

// statePayload is the POST /internal/worker/state body: the shared provision
// state plus what only this machine can answer.
type statePayload struct {
	voice.ProvisionState
	FFmpeg bool `json:"ffmpeg"`
}

// stateBody builds the one complete payload this machine publishes. The state
// post and the job heartbeat both send exactly this: the server replaces the
// stored JSON on either path, so a heartbeat carrying less would erase the
// channel, the ffmpeg fact and the preview capability on its first beat.
func (w *Worker) stateBody() statePayload {
	next := w.State()
	next.UpdatedAt = w.now().UTC()
	// Hai sự thật chỉ máy này biết, luôn đi kèm mỗi lần công bố: kênh nối (chỗ
	// Nâng cao hiện "Kênh nối") và việc máy nhận làm thêm ngoài tạo hàng loạt.
	next.Channel = w.channel()
	next.Capabilities = &voice.ProvisionCapabilities{Preview: w.cfg.Preview}
	return statePayload{ProvisionState: next, FFmpeg: w.ffmpeg}
}

// now reads the worker clock (tests replace Config.Now).
func (w *Worker) now() time.Time { return w.cfg.Now() }

// State returns the last provision state (a copy: the caller never holds the
// worker lock).
func (w *Worker) State() voice.ProvisionState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// Ready reports whether the reader is serving, i.e. whether the worker may
// claim jobs. A worker that is not ready leaves work in the queue instead of
// failing it.
func (w *Worker) Ready() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ready
}

// pinned reports whether this start has reconciled the configured pin. It is
// false until Provision succeeds once, so the first readiness pass always
// reconciles even when the files are already on disk.
func (w *Worker) pinned() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pinDone
}

// refreshReady re-derives readiness from the engine before a claim. The cached
// flag alone latches: a reader that stops after its first success (idle
// shutdown, crash) would keep receiving claims the engine cannot serve, and
// those chapters would be failed for a machine-side reason. Losing readiness
// returns the loop to the preparation/backoff path instead.
func (w *Worker) refreshReady(ctx context.Context) bool {
	st := w.cfg.Engine.Status(ctx)
	if st.Ready {
		if !w.Ready() {
			w.markReady(st)
		}
		return true
	}
	if w.Ready() {
		// The reader stopped while we were advertising it: report the truth and
		// let ensureReady restart it under the normal backoff.
		w.setState(w.statusState(st))
	}
	return false
}

// prepare advances the reader towards ready and keeps the server informed.
// It is one not-ready tick: start/install when due, publish the state, then
// pick up an operator command. Errors are transient by design — the run loop
// retries on its poll interval.
func (w *Worker) prepare(ctx context.Context) {
	w.ensureReady(ctx)
	if ctx.Err() != nil {
		return
	}
	_ = w.publishState(ctx)
	_ = w.pollCommand(ctx)
}

// ensureReady drives the engine from "not installed" to "ready": it reconciles
// the pinned build when automatic provisioning is on, starts the reader when it
// is installed but idle, and records the reason when either fails.
func (w *Worker) ensureReady(ctx context.Context) {
	st := w.cfg.Engine.Status(ctx)
	// Automatic provisioning reconciles the configured pin before anything is
	// served — even when the files already exist. Installed() only proves
	// presence, not the tag: without this, a worker upgraded against an older
	// installation would run the old source and never reach Provision's
	// idempotent upgrade path (which keeps the downloaded model and voices).
	// With --no-provision there is no provisioner: nothing is ever downloaded.
	if w.cfg.Provisioner != nil && !w.pinned() {
		if !w.due() {
			return
		}
		w.setState(w.statusState(st))
		w.provision(ctx)
		return
	}
	if st.Ready {
		w.markReady(st)
		return
	}
	if !w.due() {
		return
	}
	if !w.installed() {
		w.setState(w.statusState(st))
		if w.cfg.Provisioner == nil {
			// --no-provision: report the missing reader and wait for a human.
			return
		}
		w.provision(ctx)
		return
	}
	// Installed but not serving: start it. On the first run this is also where
	// the model is downloaded, so it can take minutes.
	attempt := w.beginAttempt()
	w.setState(voice.ProvisionState{
		State: voice.ProvisionStateStarting, Step: voice.ProvisionStepStart,
		Percent: voice.ProvisionPercentUnknown,
		Engine:  w.cfg.Engine.Name(), Model: st.Model, Attempts: attempt,
		Message: "Đang khởi động bộ đọc…",
	})
	_ = w.publishState(ctx)
	if _, err := w.cfg.Engine.Voices(ctx); err != nil {
		if ctx.Err() != nil {
			return
		}
		w.fail(&voice.ProvisionError{Code: voice.ProvisionErrStartFailed, Message: err.Error()})
		return
	}
	w.resetAttempts()
	if st := w.cfg.Engine.Status(ctx); st.Ready {
		w.markReady(st)
	}
}

// provision runs one install attempt. The caller checked that an attempt is due.
func (w *Worker) provision(ctx context.Context) {
	attempt := w.beginAttempt()
	w.setState(voice.ProvisionState{
		State: voice.ProvisionStateProvisioning, Step: voice.ProvisionStepTools,
		Percent: voice.ProvisionPercentUnknown, Attempts: attempt,
		Engine: w.cfg.Engine.Name(), Message: "Đang chuẩn bị bộ đọc…",
	})
	_ = w.publishState(ctx)
	err := w.cfg.Provisioner.Provision(ctx, func(st voice.ProvisionState) { w.report(ctx, st) })
	if err == nil {
		// The pinned build is now on disk: the readiness pass may start the
		// reader instead of reconciling again.
		w.mu.Lock()
		w.pinDone = true
		w.mu.Unlock()
		w.resetAttempts()
		st := w.cfg.Engine.Status(ctx)
		if st.Ready {
			// The reader is already serving: claim without waiting for the next
			// poll tick.
			w.markReady(st)
			return
		}
		// Provision reports the install as ready, not the engine. Until it
		// actually serves, publish the start path's state rather than leaving a
		// premature "ready" that no claim may act on.
		w.setState(voice.ProvisionState{
			State: voice.ProvisionStateStarting, Step: voice.ProvisionStepStart,
			Percent: voice.ProvisionPercentUnknown,
			Engine:  w.cfg.Engine.Name(), Model: st.Model,
			Message: "Đang khởi động bộ đọc…",
		})
		return
	}
	if ctx.Err() != nil {
		return
	}
	var p *voice.ProvisionError
	if !errors.As(err, &p) || p == nil {
		// Provision always reports a contract error; anything else means the
		// reader could not be brought up at all.
		p = &voice.ProvisionError{Code: voice.ProvisionErrStartFailed, Message: err.Error()}
	}
	w.fail(p)
}

// report is the ProvisionReporter handed to the engine: it stores the milestone
// and pushes it to the server (throttled by publishState).
func (w *Worker) report(ctx context.Context, st voice.ProvisionState) {
	if st.State == "" {
		st.State = voice.ProvisionStateProvisioning
	}
	w.mu.Lock()
	st.Attempts = w.attempts
	w.mu.Unlock()
	w.setState(st)
	_ = w.publishState(ctx)
}

// pollCommand asks the server whether an operator queued an install. A seq the
// worker has already seen never runs twice, so one click provisions once. The
// seen seq is durable (see command_state.go): the server keeps the command row
// indefinitely, so an in-memory counter alone would replay it after a restart
// and clear a hash_mismatch stop the operator never overrode.
func (w *Worker) pollCommand(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, stateRequestTimeout)
	defer cancel()
	status, raw, err := w.do(callCtx, http.MethodGet, "/internal/worker/command", nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		return nil
	}
	if status != http.StatusOK {
		return fmt.Errorf("command: %s", apiError(status, raw))
	}
	var cmd struct {
		Seq int64  `json:"seq"`
		Op  string `json:"op"`
	}
	if err := json.Unmarshal(raw, &cmd); err != nil {
		return nil
	}
	w.mu.Lock()
	fresh := cmd.Seq > w.lastSeq
	w.mu.Unlock()
	if !fresh {
		return nil
	}
	op := strings.TrimSpace(cmd.Op)
	if op == "" {
		return nil
	}
	if w.cfg.Provisioner == nil {
		return nil
	}
	// Record the sequence before the install runs: a crash between the write
	// and the work loses this one click (the operator can click again) but can
	// never replay it. If the marker cannot be written the command waits for
	// the next poll instead of running unrecorded — replaying it later is the
	// failure this file exists to prevent.
	if err := w.persistCommandSeq(cmd.Seq); err != nil {
		return fmt.Errorf("ghi nhận lệnh đã chạy: %w", err)
	}
	w.mu.Lock()
	w.lastSeq = cmd.Seq
	w.mu.Unlock()
	// An operator's click always runs once: clear an exhausted retry budget so
	// the command is not swallowed by the backoff.
	w.resetAttempts()
	w.provision(ctx)
	return nil
}

// publishState sends the current state when the server does not already have
// it. State/step transitions always go out; percent-only ticks are throttled to
// StateEvery so a fast download does not flood the listener.
func (w *Worker) publishState(ctx context.Context) error {
	if !w.shouldSend(w.State()) {
		return nil
	}
	payload := w.stateBody()
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, stateRequestTimeout)
	defer cancel()
	status, raw, err := w.do(callCtx, http.MethodPost, "/internal/worker/state", body, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("state: %s", apiError(status, raw))
	}
	w.mu.Lock()
	w.sent, w.sentAt, w.sentOnce = payload.ProvisionState, w.now(), true
	w.mu.Unlock()
	return nil
}

func (w *Worker) shouldSend(next voice.ProvisionState) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.sentOnce {
		return true
	}
	if next.State != w.sent.State || next.Step != w.sent.Step {
		return true
	}
	if next.Percent == w.sent.Percent && next.Message == w.sent.Message && sameProvisionError(next.Error, w.sent.Error) {
		return false
	}
	return w.now().Sub(w.sentAt) >= w.cfg.StateEvery
}

func sameProvisionError(a, b *voice.ProvisionError) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Code == b.Code && a.Message == b.Message && a.LogTail == b.LogTail
}

// setState stores a state snapshot without publishing it.
func (w *Worker) setState(next voice.ProvisionState) {
	if next.Engine == "" {
		next.Engine = w.cfg.Engine.Name()
	}
	w.mu.Lock()
	w.state = next
	w.ready = next.State == voice.ProvisionStateReady
	w.mu.Unlock()
}

// markReady records a serving reader together with the numbers the UI shows.
func (w *Worker) markReady(st voice.Status) {
	next := voice.ProvisionState{
		State: voice.ProvisionStateReady, Percent: 100,
		Engine: w.cfg.Engine.Name(), Model: st.Model,
	}
	if rtf := w.rtf(st); rtf != nil {
		next.RTF = *rtf
	}
	if sizer, ok := w.cfg.Engine.(voice.Sizer); ok {
		next.DiskBytes = sizer.DiskUsage()
	}
	w.mu.Lock()
	w.state, w.ready = next, true
	w.attempts, w.exhausted, w.nextAttempt = 0, false, time.Time{}
	w.mu.Unlock()
}

func (w *Worker) rtf(st voice.Status) *float64 {
	if r, ok := w.cfg.Engine.(voice.RTFReporter); ok {
		if v := r.RTF(); v != nil {
			return v
		}
	}
	return st.RTF
}

// statusState turns an engine snapshot into the shared provision state.
func (w *Worker) statusState(st voice.Status) voice.ProvisionState {
	next := voice.ProvisionState{
		Engine: w.cfg.Engine.Name(), Model: st.Model, Percent: voice.ProvisionPercentUnknown,
	}
	switch {
	case st.Ready:
		next.State, next.Percent = voice.ProvisionStateReady, 100
	case st.State == voice.StateStarting:
		next.State, next.Message = voice.ProvisionStateStarting, "Đang khởi động bộ đọc…"
	case st.State == voice.StateError:
		next.State = voice.ProvisionStateError
		next.Message = st.Error
		next.Error = &voice.ProvisionError{Code: voice.ProvisionErrStartFailed, Message: st.Error}
	default:
		next.State, next.Message = voice.ProvisionStateMissing, "Chưa có bộ đọc trên máy"
	}
	return next
}

// installedReporter is the narrow view of voice.Installer the readiness gate
// needs: does the reader's pinned build exist on disk?
type installedReporter interface {
	Installed() bool
}

// installed reports whether the reader's files are on disk. An engine that
// cannot be installed (a remote one) counts as installed.
func (w *Worker) installed() bool {
	if ins, ok := w.cfg.Engine.(installedReporter); ok {
		return ins.Installed()
	}
	return true
}

// due reports whether another install/start attempt may run now.
func (w *Worker) due() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.exhausted {
		return false
	}
	return !w.now().Before(w.nextAttempt)
}

// beginAttempt counts a new attempt and returns its 1-based number.
func (w *Worker) beginAttempt() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.attempts++
	return w.attempts
}

// fail records a failed attempt and schedules the next one, or stops retrying
// once the budget for this start is spent.
//
// hash_mismatch never auto-retries: the file that arrived did not match the pin,
// which is a safety signal (a broken mirror, a proxy rewriting bytes), not a
// transient network glitch. The worker keeps the error and waits for an operator
// click, exactly as 10-audio.md §8.2 promises the user.
func (w *Worker) fail(p *voice.ProvisionError) {
	w.mu.Lock()
	attempt := w.attempts
	delay := time.Duration(0)
	switch {
	case p != nil && p.Code == voice.ProvisionErrHashMismatch:
		w.exhausted = true
	case attempt <= len(w.cfg.ProvisionBackoff):
		delay = w.cfg.ProvisionBackoff[attempt-1]
	default:
		w.exhausted = true
	}
	if delay > 0 {
		w.nextAttempt = w.now().Add(delay)
	}
	w.state = voice.ProvisionState{
		State: voice.ProvisionStateError, Percent: voice.ProvisionPercentUnknown,
		Engine: w.cfg.Engine.Name(), Message: p.Message, Error: p, Attempts: attempt,
	}
	w.ready = false
	w.mu.Unlock()
}

// resetAttempts clears the retry budget (success, or an operator command).
func (w *Worker) resetAttempts() {
	w.mu.Lock()
	w.attempts, w.exhausted, w.nextAttempt = 0, false, time.Time{}
	w.mu.Unlock()
}

// sleepCtx waits for d or ctx; it reports false when the context ended first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
