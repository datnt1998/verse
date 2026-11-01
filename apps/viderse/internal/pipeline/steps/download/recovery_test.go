package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

func infoJSON(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(fakeInfo)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func requireCode(t *testing.T, err error, code string) *domain.CloneError {
	t.Helper()
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != code {
		t.Fatalf("error = %v; want code %s", err, code)
	}
	return cloneErr
}
func mediaRunner(t *testing.T, dir string) *scriptedRunner {
	return &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		writeStaging(t, dir, "source.mp4", "mp4")
		return "", nil
	}}
}
func TestHandlerUsesTrackedRunnerWithManagedDefaults(t *testing.T) {
	step, dir := fakeCtx(t)
	runner := mediaRunner(t, dir)
	step.ProcessRunner = runner.Run
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = &scriptedRunner{yt: func([]string, func(string)) (string, error) { t.Fatal("untracked runner used"); return "", nil }}
		d.Tools.BaseDir = filepath.Join(dir, "app")
		d.CookieArgs = func(context.Context) ([]string, error) {
			return []string{"--cookies-from-browser", "chrome:Default"}, nil
		}
	})
	if _, err := New(deps).Run(context.Background(), step); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %v", runner.calls)
	}
	for _, args := range runner.calls {
		joined := strings.Join(args, " ")
		for _, want := range []string{"--ignore-config", "--cache-dir " + filepath.Join(dir, "app", "cache", "yt-dlp"), "--cookies-from-browser chrome:Default"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("missing %q in %v", want, args)
			}
		}
	}
}
func TestHandlerParksBeforeDownloadWhenDiskIsShort(t *testing.T) {
	step, _ := fakeCtx(t)
	runs := 0
	runner := &scriptedRunner{yt: func(args []string, _ func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		runs++
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) { d.Tools.Runner = runner; d.FreeDisk = func(string) int64 { return 1024 } })
	result, err := New(deps).Run(context.Background(), step)
	cloneErr := requireCode(t, err, domain.CloneErrQuota)
	if cloneErr.RetryAt == nil || cloneErr.RetryAt.UnixMilli() != 1700000000000+30*60_000 || runs != 0 || len(result.Artifacts) != 0 {
		t.Fatalf("error=%+v runs=%d result=%+v", cloneErr, runs, result)
	}
}
func TestHandlerRejectsOversizeBeforeDownload(t *testing.T) {
	step, _ := fakeCtx(t)
	runs := 0
	runner := &scriptedRunner{yt: func(args []string, _ func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		runs++
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = runner
		l := DownloadLimits(map[string]string{})
		l.MaxBytes = 100
		d.Limits = &l
	})
	_, err := New(deps).Run(context.Background(), step)
	requireCode(t, err, domain.CloneErrInvalidRequest)
	if runs != 0 {
		t.Fatalf("started %d media runs", runs)
	}
}
func TestHandlerKeepsPartAndIdenticalArgsAcrossInlineRetries(t *testing.T) {
	step, dir := fakeCtx(t)
	writeStaging(t, dir, "source.f137.mp4.part", "partial")
	var mediaArgs [][]string
	runner := &scriptedRunner{yt: func(args []string, _ func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		mediaArgs = append(mediaArgs, append([]string(nil), args...))
		part, err := os.ReadFile(filepath.Join(dir, "source.f137.mp4.part"))
		if err != nil || string(part) != "partial" {
			t.Fatalf("part=%q err=%v", part, err)
		}
		if len(mediaArgs) == 1 {
			return "", &tools.YtDlpError{Code: tools.YtDlpFailed, Message: "HTTP Error 500"}
		}
		writeStaging(t, dir, "source.mp4", "mp4")
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) { d.Tools.Runner = runner })
	if _, err := New(deps).Run(context.Background(), step); err != nil {
		t.Fatal(err)
	}
	if len(mediaArgs) != 2 || strings.Join(mediaArgs[0], "\x00") != strings.Join(mediaArgs[1], "\x00") {
		t.Fatalf("args changed: %v", mediaArgs)
	}
}
func TestHandlerResumesCheckpointWithoutRefetchingMetadata(t *testing.T) {
	step, dir := fakeCtx(t)
	var saved json.RawMessage
	step.WriteCheckpoint = func(_ context.Context, raw json.RawMessage) error { saved = append(saved[:0], raw...); return nil }
	mediaRuns := 0
	metadataRuns := 0
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		if containsArg(args, "-J") {
			metadataRuns++
			return infoJSON(t), nil
		}
		mediaRuns++
		if mediaRuns == 1 {
			writeStaging(t, dir, "source.mp4.part", "saved")
			return "", toolchain.ErrCancelled
		}
		part, err := os.ReadFile(filepath.Join(dir, "source.mp4.part"))
		if err != nil || string(part) != "saved" {
			t.Fatalf("lost part: %q %v", part, err)
		}
		emit(progressLine)
		writeStaging(t, dir, "source.mp4", "mp4")
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) { d.Tools.Runner = runner })
	_, err := New(deps).Run(context.Background(), step)
	if !errors.Is(err, toolchain.ErrCancelled) {
		t.Fatalf("first error = %v", err)
	}
	step.Checkpoint = append(json.RawMessage(nil), saved...)
	if _, err = New(deps).Run(context.Background(), step); err != nil {
		t.Fatal(err)
	}
	if metadataRuns != 1 || mediaRuns != 2 {
		t.Fatalf("metadata=%d media=%d", metadataRuns, mediaRuns)
	}
}
func TestHandlerResumesCompletedMediaWithoutDownloadingAgain(t *testing.T) {
	step, dir := fakeCtx(t)
	raw := infoJSON(t)
	writeStaging(t, dir, "source.info.json", raw)
	writeStaging(t, dir, "source.mp4", "mp4")
	sum := sha256.Sum256([]byte(raw))
	cp := checkpoint{Version: 1, URL: step.Job.Source.URL, InfoSHA256: hex.EncodeToString(sum[:]), MediaReady: true, Phase: "import"}
	var err error
	step.Checkpoint, err = json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	deps := fakeDeps(func(d *Deps) {
		d.Tools.Runner = &scriptedRunner{yt: func([]string, func(string)) (string, error) {
			t.Fatal("completed media downloaded again")
			return "", nil
		}}
	})
	step.Media = fakeStepMedia{}
	result, err := New(deps).Run(context.Background(), step)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("mp4"))
	if result.Data["sourceSha256"] != hex.EncodeToString(digest[:]) {
		t.Fatalf("hash=%v", result.Data["sourceSha256"])
	}
}
func TestHandlerPauseSavesCheckpointAndKeepsPart(t *testing.T) {
	step, dir := fakeCtx(t)
	var paused atomic.Bool
	step.IsPaused = paused.Load
	var saved checkpoint
	step.WriteCheckpoint = func(_ context.Context, raw json.RawMessage) error { return json.Unmarshal(raw, &saved) }
	runner := &scriptedRunner{yt: func(args []string, _ func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		writeStaging(t, dir, "source.mp4.part", "partial")
		paused.Store(true)
		return "", toolchain.ErrCancelled
	}}
	deps := fakeDeps(func(d *Deps) { d.Tools.Runner = runner })
	result, err := New(deps).Run(context.Background(), step)
	if !errors.Is(err, pipeline.ErrPaused) || len(result.Artifacts) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if saved.InfoSHA256 == "" || saved.URL != step.Job.Source.URL || saved.MediaReady {
		t.Fatalf("checkpoint=%+v", saved)
	}
	if _, err := os.Stat(filepath.Join(dir, "source.mp4.part")); err != nil {
		t.Fatal(err)
	}
}
func TestHandlerTranscodesNativePiecesUsingTrackedFfmpeg(t *testing.T) {
	step, dir := fakeCtx(t)
	runner := &scriptedRunner{}
	mediaRuns := 0
	runner.yt = func(args []string, _ func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		mediaRuns++
		if mediaRuns == 1 {
			return "", &tools.YtDlpError{Code: tools.YtDlpFailed, Message: "Postprocessing: Conversion failed!"}
		}
		writeStaging(t, dir, "source.f400.webm", "video")
		writeStaging(t, dir, "source.f251.webm", "audio")
		return "", nil
	}
	transcodes := 0
	runner.run = func(file string, args []string, emit func(string)) (tools.Result, error) {
		if file != "/managed/ffmpeg" {
			t.Fatalf("unexpected process %s", file)
		}
		transcodes++
		joined := strings.Join(args, " ")
		for _, want := range []string{"-c:v libx264", "-c:a aac", "-fps_mode:v vfr", "-map 0:v? -map 0:a? -map 1:v? -map 1:a?"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("missing %q in %v", want, args)
			}
		}
		emit("out_time_us=1000000")
		if err := os.WriteFile(args[len(args)-1], []byte("converted"), 0o600); err != nil {
			t.Fatal(err)
		}
		return tools.Result{}, nil
	}
	step.ProcessRunner = runner.Run
	deps := fakeDeps(func(d *Deps) {
		d.Transcode = nil
		d.ProbeFile = func(_ context.Context, _ string, file string) (ProbeSummary, error) {
			if strings.HasSuffix(file, ".webm") {
				p := mp4Probe
				p.FormatName = "matroska,webm"
				p.VideoCodec = "vp9"
				return p, nil
			}
			return mp4Probe, nil
		}
	})
	result, err := New(deps).Run(context.Background(), step)
	if err != nil {
		t.Fatal(err)
	}
	if transcodes != 1 || result.Data["transcoded"] != true {
		t.Fatalf("transcodes=%d data=%v", transcodes, result.Data)
	}
	content, err := os.ReadFile(filepath.Join(dir, "source.mp4"))
	if err != nil || string(content) != "converted" {
		t.Fatalf("source=%q err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "compat.part.mp4")); !os.IsNotExist(err) {
		t.Fatalf("transcode was copied rather than renamed: %v", err)
	}
}
func TestHandlerPropagatesProgressAndCheckpointFailures(t *testing.T) {
	for _, where := range []string{"progress", "checkpoint"} {
		t.Run(where, func(t *testing.T) {
			step, dir := fakeCtx(t)
			failure := errors.New("persistence failed")
			if where == "progress" {
				step.ReportProgress = func(context.Context, domain.CloneProgress) error { return failure }
			} else {
				step.WriteCheckpoint = func(context.Context, json.RawMessage) error { return failure }
			}
			deps := fakeDeps(func(d *Deps) { d.Tools.Runner = mediaRunner(t, dir) })
			_, err := New(deps).Run(context.Background(), step)
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
func TestHandlerRejectsMalformedInfoAndUnstableOutput(t *testing.T) {
	for _, test := range []struct {
		name, raw string
		probe     ProbeSummary
	}{
		{name: "invalid JSON", raw: `{"duration":"not a number"}`, probe: mp4Probe},
		{name: "negative size", raw: `{"filesize":-1}`, probe: mp4Probe},
		{name: "null document", raw: `null`, probe: mp4Probe},
		{name: "audio only", probe: ProbeSummary{DurationSec: new(60.0)}},
		{name: "unknown duration", probe: ProbeSummary{VideoCodec: "h264"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			step, dir := fakeCtx(t)
			runner := &scriptedRunner{yt: func(args []string, _ func(string)) (string, error) {
				if containsArg(args, "-J") {
					if test.raw != "" {
						return test.raw, nil
					}
					return infoJSON(t), nil
				}
				writeStaging(t, dir, "source.mp4", "mp4")
				return "", nil
			}}
			deps := fakeDeps(func(d *Deps) {
				d.Tools.Runner = runner
				d.ProbeFile = func(context.Context, string, string) (ProbeSummary, error) { return test.probe, nil }
			})
			_, err := New(deps).Run(context.Background(), step)
			requireCode(t, err, domain.CloneErrSourceUnavailable)
		})
	}
}
func TestHandlerMapsMidDownloadDiskExhaustionToQuota(t *testing.T) {
	step, _ := fakeCtx(t)
	runner := &scriptedRunner{yt: func(args []string, _ func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		return "", &tools.ProcessFailedError{Message: "write failed", Code: 1, Stderr: "ERROR: No space left on device"}
	}}
	deps := fakeDeps(func(d *Deps) { d.Tools.Runner = runner })
	_, err := New(deps).Run(context.Background(), step)
	failure := requireCode(t, err, domain.CloneErrQuota)
	if failure.RetryAt == nil || !strings.Contains(*failure.Recovery, "Giải phóng") {
		t.Fatalf("error=%+v", failure)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("disk exhaustion retried inline: %v", runner.calls)
	}
}
func TestParseFfprobeRejectsMalformedAndKeepsZeroRotation(t *testing.T) {
	if _, err := ParseFfprobeJSON([]byte(`{"streams":"invalid"}`)); err == nil {
		t.Fatal("accepted invalid streams")
	}
	probe, err := ParseFfprobeJSON([]byte(`{"format":{"duration":"N/A"},"streams":[{"codec_type":"video","codec_name":"h264","duration":"2.5","avg_frame_rate":"30000/1001","r_frame_rate":"0/0","tags":{"rotate":"0"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if probe.DurationSec == nil || *probe.DurationSec != 2.5 || probe.RotationDeg == nil || *probe.RotationDeg != 0 || probe.FPS == nil || *probe.FPS != 29.97 || probe.VariableFPS {
		t.Fatalf("probe=%+v", probe)
	}
}
func TestActivityGateSeparatesStallFromParentCancellation(t *testing.T) {
	gate := newActivityGate(context.Background(), time.Millisecond, time.Minute)
	defer gate.close()
	select {
	case <-gate.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("stall did not cancel")
	}
	if gate.why() != "stall" {
		t.Fatalf("reason=%q", gate.why())
	}
	ctx, cancel := context.WithCancel(context.Background())
	parentGate := newActivityGate(ctx, time.Minute, time.Minute)
	defer parentGate.close()
	cancel()
	<-parentGate.ctx.Done()
	if parentGate.why() != "" {
		t.Fatalf("parent cancellation became timeout: %q", parentGate.why())
	}
}

func TestHandlerStopsStalledDownloadWithRetryTime(t *testing.T) {
	step, _ := fakeCtx(t)
	step.ProcessRunner = func(ctx context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
		if containsArg(args, "-J") {
			return tools.Result{Stdout: infoJSON(t)}, nil
		}
		<-ctx.Done()
		return tools.Result{}, toolchain.ErrCancelled
	}
	deps := fakeDeps(func(d *Deps) {
		limits := DownloadLimits(map[string]string{})
		limits.Stall = 5 * time.Millisecond
		limits.Max = time.Second
		d.Limits = &limits
	})
	_, err := New(deps).Run(context.Background(), step)
	failure := requireCode(t, err, domain.CloneErrQuota)
	if failure.RetryAt == nil || failure.RetryAt.UnixMilli() != 1700000000000+5*60_000 || !strings.Contains(failure.Message, "ngừng tiến triển") {
		t.Fatalf("failure=%+v", failure)
	}
}
func TestHandlerPropagatesStreamingProgressFailure(t *testing.T) {
	step, dir := fakeCtx(t)
	failure := errors.New("progress transaction failed")
	step.ReportProgress = func(_ context.Context, p domain.CloneProgress) error {
		if p.Phase == "download" {
			return failure
		}
		return nil
	}
	runner := &scriptedRunner{yt: func(args []string, emit func(string)) (string, error) {
		if containsArg(args, "-J") {
			return infoJSON(t), nil
		}
		emit(progressLine)
		writeStaging(t, dir, "source.mp4", "mp4")
		return "", nil
	}}
	deps := fakeDeps(func(d *Deps) { d.Tools.Runner = runner })
	result, err := New(deps).Run(context.Background(), step)
	if !errors.Is(err, failure) || len(result.Artifacts) != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}
func TestHandlerRejectsChangedCheckpointMetadata(t *testing.T) {
	step, dir := fakeCtx(t)
	writeStaging(t, dir, "source.info.json", `{"id":"changed"}`)
	raw, err := json.Marshal(checkpoint{Version: 1, URL: step.Job.Source.URL, InfoSHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	step.Checkpoint = raw
	deps := fakeDeps(nil)
	if _, err := New(deps).Run(context.Background(), step); err == nil || !strings.Contains(err.Error(), "không khớp checkpoint") {
		t.Fatalf("checkpoint mismatch error=%v", err)
	}
}
func TestFindStagedMediaKeepsSplitAudioAndIgnoresEmptyMp4(t *testing.T) {
	dir := t.TempDir()
	writeStaging(t, dir, "source.mp4", "")
	writeStaging(t, dir, "source.f137.mp4", "video")
	writeStaging(t, dir, "source.f140.m4a", "audio")
	writeStaging(t, dir, "source.f137.mp4.part", "partial")
	staged, err := FindStagedMedia(dir)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Mp4 != "" || !reflect2(staged.Pieces, []string{"source.f137.mp4", "source.f140.m4a"}) {
		t.Fatalf("staged=%+v", staged)
	}
}
