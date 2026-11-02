package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

// runnerFunc adapts the fenced step's process-group runner without bypassing
// tools.YtDlp's managed resolution, cache flags or error classification.
type runnerFunc pipeline.ProcessRunner

func (f runnerFunc) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return f(ctx, file, args, spec)
}

type activityGate struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	stall    *time.Timer
	deadline *time.Timer
	reason   string
}

func newActivityGate(parent context.Context, stall, max time.Duration) *activityGate {
	ctx, cancel := context.WithCancel(parent)
	g := &activityGate{ctx: ctx, cancel: cancel}
	if stall > 0 {
		g.stall = time.AfterFunc(stall, func() { g.expire("stall") })
	}
	if max > 0 {
		g.deadline = time.AfterFunc(max, func() { g.expire("deadline") })
	}
	return g
}
func (g *activityGate) expire(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.reason == "" {
		g.reason = reason
		g.cancel()
	}
}
func (g *activityGate) touch(stall time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.reason == "" && g.stall != nil {
		g.stall.Reset(stall)
	}
}
func (g *activityGate) why() string { g.mu.Lock(); defer g.mu.Unlock(); return g.reason }
func (g *activityGate) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stall != nil {
		g.stall.Stop()
	}
	if g.deadline != nil {
		g.deadline.Stop()
	}
	g.cancel()
}

type activityWriter struct {
	gate  *activityGate
	stall time.Duration
}

func (w activityWriter) Write(p []byte) (int, error) { w.gate.touch(w.stall); return len(p), nil }

var _ io.Writer = activityWriter{}

// TranscodeJob is the optional service-boundary seam. Runner is already bound
// to StepContext.ProcessRunner; a supplied transcode must not publish files.
type TranscodeJob struct {
	Step              *pipeline.StepContext
	FF                *tools.FfmpegResolved
	Runner            tools.Runner
	Inputs            []string
	SourceDurationSec *float64
	HasAudio          bool
	Target            string
	Limits            Limits
}

func (h *Handler) transcode(ctx context.Context, job TranscodeJob) error {
	if h.deps.Transcode != nil {
		return h.deps.Transcode(ctx, job)
	}
	g := newActivityGate(ctx, 5*time.Minute, job.Limits.TranscodeMax)
	defer g.close()
	encoder := "libx264"
	if job.FF.Probe != nil && job.FF.Probe.Caps.H264 != "" {
		encoder = job.FF.Probe.Caps.H264
	}
	var callbackErr error
	_, err := job.Runner.Run(g.ctx, job.FF.Ffmpeg, BuildTranscodeArgs(TranscodeArgsOpts{Inputs: job.Inputs, Output: job.Target, H264Encoder: encoder, HasAudio: job.HasAudio}), tools.Spec{
		Background: true, LogLimit: 256 * 1024, Log: activityWriter{gate: g, stall: 5 * time.Minute},
		OnLine: func(line string, _ tools.Stream) {
			if callbackErr != nil {
				return
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok || (key != "out_time_us" && key != "out_time_ms") {
				return
			}
			us, e := strconv.ParseInt(value, 10, 64)
			if e != nil || us < 0 {
				return
			}
			var total *int64
			if job.SourceDurationSec != nil {
				ms := int64(*job.SourceDurationSec * 1000)
				total = &ms
			}
			callbackErr = reportProgress(g.ctx, job.Step, us/1000, total, "transcode")
			if callbackErr != nil {
				g.cancel()
			}
		},
	})
	if callbackErr != nil {
		return callbackErr
	}
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %w", toolchain.ErrCancelled, ctx.Err())
	}
	if g.why() != "" {
		return retryError("Transcode nguồn bị timeout", "Chạy lại bước tải; video dài có thể cần tăng YTF_CLONE_TRANSCODE_MAX_MS", h.now(), 10*time.Minute)
	}
	if err != nil {
		var failed *tools.ProcessFailedError
		if errors.As(err, &failed) {
			if isDiskFailure(failed) {
				return h.diskError()
			}
			tail := strings.TrimSpace(failed.Stderr)
			if tail == "" {
				tail = failed.Error()
			}
			lines := strings.Split(tail, "\n")
			return stepError(domain.CloneErrInternal, "ffmpeg chuyển mã thất bại: "+lines[len(lines)-1], "")
		}
		return err
	}
	return nil
}
func reportProgress(ctx context.Context, step *pipeline.StepContext, done int64, total *int64, phase string) error {
	if done < 0 {
		done = 0
	}
	if total != nil {
		if *total < 0 {
			total = nil
		} else if done > *total {
			total = &done
		}
	}
	if step.ReportProgress == nil {
		return fmt.Errorf("thiếu bộ ghi tiến độ bước tải")
	}
	return step.Progress(ctx, domain.CloneProgress{UnitsDone: done, UnitsTotal: total, Phase: phase})
}
