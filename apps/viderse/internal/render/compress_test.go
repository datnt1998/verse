package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func compressionRenderFacts() MediaFactsWithBytes {
	return MediaFactsWithBytes{MediaFacts: MediaFacts{DurationSec: new(float64(5400)), Width: new(1920), Height: new(1080), Progressive: new(true), VideoCodec: new("h264"), AudioCodec: new("aac"), AudioChannels: new(2), Container: new("mov,mp4,m4a,3gp,3g2,mj2")}, Bytes: 900 * 1024 * 1024}
}

func TestCompressedOutputOnlyReplacesWithSmallerEquivalentVideo(t *testing.T) {
	before := compressionRenderFacts()
	after := before
	after.Bytes = 450 * 1024 * 1024
	if problem := CompressedOutputProblem(before, after); problem != "" {
		t.Fatal(problem)
	}
	for _, test := range []struct {
		name, contains string
		change         func(*MediaFactsWithBytes)
	}{
		{"larger", "không nhỏ hơn", func(f *MediaFactsWithBytes) { f.Bytes = 1300 * 1024 * 1024 }},
		{"equal", "không nhỏ hơn", func(f *MediaFactsWithBytes) { f.Bytes = before.Bytes }},
		{"shorter", "độ dài", func(f *MediaFactsWithBytes) { f.DurationSec = new(float64(5390)) }},
		{"dimensions", "kích thước", func(f *MediaFactsWithBytes) { f.Width = new(1280) }},
		{"height", "kích thước", func(f *MediaFactsWithBytes) { f.Height = new(720) }},
		{"codec", "codec", func(f *MediaFactsWithBytes) { f.VideoCodec = new("hevc") }},
		{"unknown-codec", "codec", func(f *MediaFactsWithBytes) { f.VideoCodec = nil }},
		{"channels", "tiếng", func(f *MediaFactsWithBytes) { f.AudioChannels = new(1) }},
		{"audio-codec", "tiếng", func(f *MediaFactsWithBytes) { f.AudioCodec = new("mp3") }},
		{"unknown-duration", "độ dài", func(f *MediaFactsWithBytes) { f.DurationSec = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := after
			test.change(&candidate)
			if problem := CompressedOutputProblem(before, candidate); !strings.Contains(problem, test.contains) {
				t.Fatalf("problem %q, want %q", problem, test.contains)
			}
		})
	}
	after.DurationSec = new(5400.5)
	if problem := CompressedOutputProblem(before, after); problem != "" {
		t.Fatalf("duration at tolerance rejected: %s", problem)
	}
	after.DurationSec = new(5400.5001)
	if problem := CompressedOutputProblem(before, after); !strings.Contains(problem, "độ dài") {
		t.Fatalf("duration outside tolerance accepted: %s", problem)
	}
	before.DurationSec = nil
	if problem := CompressedOutputProblem(before, after); !strings.Contains(problem, "độ dài") {
		t.Fatal("unknown original duration accepted")
	}
}

func compressionFixture(t *testing.T) (string, string, editor.Project) {
	t.Helper()
	root := t.TempDir()
	input, output := filepath.Join(root, "input.mp4"), filepath.Join(root, "output.mp4")
	if err := os.WriteFile(input, bytes.Repeat([]byte("render"), 1000), 0600); err != nil {
		t.Fatal(err)
	}
	project := editor.Project{"id": "fixture", "metadata": editor.Object{"width": 640, "height": 360, "fps": 30}, "timeline": editor.Object{"tracks": []editor.Object{{"id": "wave"}}, "items": []editor.Object{{"id": "waveform", "type": "shape", "waveformVisualizer": editor.Object{"visible": true}, "trackId": "wave", "from": 0, "durationInFrames": 90, "transform": editor.Object{"width": 240, "height": 60, "y": -100}}}}}
	return input, output, project
}

const compressionProbeDoc = `{"streams":[{"codec_type":"video","codec_name":"h264","width":320,"height":180,"field_order":"progressive"},{"codec_type":"audio","codec_name":"aac","channels":2}],"format":{"duration":"3","format_name":"mov,mp4,m4a,3gp,3g2,mj2"}}`

func compressionScript(args []string) string {
	for i, arg := range args {
		if arg == "-filter_script:v" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
func compressionRequireScriptRemoved(t *testing.T, script string) {
	t.Helper()
	if script == "" {
		t.Fatal("encoder never received an ROI filter script")
	}
	if _, err := os.Stat(filepath.Dir(script)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ROI scratch directory remains: %s (%v)", script, err)
	}
}

func TestCompressExportExactArgumentsProgressAndCleanup(t *testing.T) {
	input, output, project := compressionFixture(t)
	var script string
	var progress []float64
	var totals []*float64
	probes := 0
	runner := evidenceRunnerFunc(func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
		if file == "ffprobe" {
			probes++
			return tools.Result{Stdout: compressionProbeDoc}, nil
		}
		if file != "ffmpeg" {
			t.Fatalf("unexpected binary %q", file)
		}
		script = compressionScript(args)
		want := []string{"-hide_banner", "-nostdin", "-y", "-progress", "pipe:1", "-nostats", "-i", input, "-map", "0:v:0", "-map", "0:a?", "-filter_script:v", script, "-c:v", "libx264", "-preset", "veryfast", "-crf", "28", "-maxrate", "900k", "-bufsize", "1800k", "-g", "300", "-pix_fmt", "yuv420p", "-c:a", "copy", "-movflags", "+faststart", output}
		if script == "" || !reflect.DeepEqual(args, want) || !spec.Background || spec.OnLine == nil {
			t.Fatalf("encoder args %q, background=%v", args, spec.Background)
		}
		filter, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		if string(filter) != "addroi=x=88:y=13:w=144:h=54:qoffset=-0.25:clear=1" {
			t.Fatalf("ROI filter %q", filter)
		}
		info, err := os.Stat(script)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("ROI script permissions %o", info.Mode().Perm())
		}
		spec.OnLine("out_time_us=500000", tools.StreamStderr)
		for _, line := range []string{"out_time_us=N/A", "out_time_us=-1", "out_time_us=12x", "out_time_ms=500000", "out_time_us=", "out_time_us=500000", "out_time_us=3000000"} {
			spec.OnLine(line, tools.StreamStdout)
		}
		return tools.Result{}, os.WriteFile(output, []byte("small mp4"), 0600)
	})
	err := CompressExportFile(context.Background(), CompressOptions{Runner: runner, Ffmpeg: "ffmpeg", FFprobe: "ffprobe", Input: input, Output: output, Project: project, OnProgress: func(done float64, total *float64) { progress = append(progress, done); totals = append(totals, total) }})
	if err != nil {
		t.Fatal(err)
	}
	if probes != 2 || !reflect.DeepEqual(progress, []float64{0.5, 3}) || len(totals) != 2 || totals[0] == nil || *totals[0] != 3 {
		t.Fatalf("probes=%d progress=%v totals=%v", probes, progress, totals)
	}
	compressionRequireScriptRemoved(t, script)
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, bytes.Repeat([]byte("render"), 1000)) {
		t.Fatal("compression mutated original")
	}
}

func TestCompressExportRemovesROIScriptOnFailureAbortAndRejectedOutput(t *testing.T) {
	for _, mode := range []string{"process-failure", "abort", "invalid-output", "probe-failure"} {
		t.Run(mode, func(t *testing.T) {
			input, output, project := compressionFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("encoder failed")
			var script string
			runner := evidenceRunnerFunc(func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
				if file == "ffprobe" {
					if args[len(args)-1] == output {
						if mode == "probe-failure" {
							return tools.Result{}, failure
						}
						return tools.Result{Stdout: strings.Replace(compressionProbeDoc, `"width":320`, `"width":640`, 1)}, nil
					}
					return tools.Result{Stdout: compressionProbeDoc}, nil
				}
				script = compressionScript(args)
				if _, err := os.Stat(script); err != nil {
					t.Fatal(err)
				}
				if mode == "process-failure" {
					return tools.Result{}, failure
				}
				if mode == "abort" {
					spec.OnLine("out_time_us=1000", tools.StreamStdout)
					return tools.Result{}, nil
				}
				return tools.Result{}, os.WriteFile(output, []byte("candidate"), 0600)
			})
			err := CompressExportFile(ctx, CompressOptions{Runner: runner, Ffmpeg: "ffmpeg", FFprobe: "ffprobe", Input: input, Output: output, Project: project, OnProgress: func(float64, *float64) {
				if mode == "abort" {
					cancel()
				}
			}})
			if err == nil {
				t.Fatal("failed pass accepted")
			}
			if mode == "abort" && !errors.Is(err, context.Canceled) {
				t.Fatalf("abort lost: %v", err)
			}
			if (mode == "process-failure" || mode == "probe-failure") && !errors.Is(err, failure) {
				t.Fatalf("process error lost: %v", err)
			}
			if mode == "invalid-output" && !strings.Contains(err.Error(), "kích thước") {
				t.Fatalf("wrong rejection: %v", err)
			}
			compressionRequireScriptRemoved(t, script)
			info, statErr := os.Stat(input)
			if statErr != nil || info.Size() != 6000 {
				t.Fatalf("original damaged: %+v %v", info, statErr)
			}
		})
	}
}

func TestCompressExportRejectsInputAliasesAndUnmeasuredCaptionsBeforeEncoding(t *testing.T) {
	input, output, project := compressionFixture(t)
	calls := 0
	runner := evidenceRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
		calls++
		return tools.Result{Stdout: compressionProbeDoc}, nil
	})
	opts := CompressOptions{Runner: runner, Ffmpeg: "ffmpeg", FFprobe: "ffprobe", Input: input, Output: input, Project: project}
	if err := CompressExportFile(context.Background(), opts); err == nil || calls != 0 {
		t.Fatalf("same path accepted (%d calls): %v", calls, err)
	}
	if err := os.Link(input, output); err != nil {
		t.Fatal(err)
	}
	opts.Output = output
	if err := CompressExportFile(context.Background(), opts); err == nil || calls != 0 {
		t.Fatalf("hardlink accepted (%d calls): %v", calls, err)
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	project.Timeline()["items"] = []editor.Object{{"id": "caption", "trackId": "wave", "type": "text", "from": 0, "durationInFrames": 90}}
	if err := CompressExportFile(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "Thiếu vùng chữ") || calls != 1 {
		t.Fatalf("unmeasured caption accepted (%d calls): %v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CompressExportFile(ctx, opts); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancelled pass ran (%d calls): %v", calls, err)
	}
}

// This is the source's real-binary regression, kept opt-in. All process launches
// still use the production runner; the wrapper only observes scratch paths.
func TestCompressExportRealROICleanup(t *testing.T) {
	if os.Getenv("YTF_CLONE_REAL_BINARIES") != "1" {
		t.Skip("set YTF_CLONE_REAL_BINARIES=1")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	input, output, project := compressionFixture(t)
	executor := &tools.ExecRunner{}
	_, err = executor.Run(ctx, ffmpeg, []string{"-hide_banner", "-nostdin", "-y", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30:duration=3", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-c:v", "libx264", "-preset", "ultrafast", "-crf", "0", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", input}, tools.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "process-failure", "abort"} {
		t.Run(mode, func(t *testing.T) {
			var script string
			runner := evidenceRunnerFunc(func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
				if candidate := compressionScript(args); candidate != "" {
					script = candidate
				}
				return executor.Run(ctx, file, args, spec)
			})
			childCtx, childCancel := context.WithCancel(ctx)
			defer childCancel()
			binary := ffmpeg
			if mode == "process-failure" {
				binary = filepath.Join(t.TempDir(), "missing-ffmpeg")
			}
			err := CompressExportFile(childCtx, CompressOptions{Runner: runner, Ffmpeg: binary, FFprobe: ffprobe, Input: input, Output: output, Project: project, OnProgress: func(float64, *float64) {
				if mode == "abort" {
					childCancel()
				}
			}})
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
				before, statErr := os.Stat(input)
				if statErr != nil {
					t.Fatal(statErr)
				}
				after, statErr := os.Stat(output)
				if statErr != nil {
					t.Fatal(statErr)
				}
				if after.Size() >= before.Size() {
					t.Fatal("real encode is not smaller")
				}
			} else if err == nil {
				t.Fatal(fmt.Sprintf("%s pass unexpectedly succeeded", mode))
			}
			if mode == "abort" && !errors.Is(childCtx.Err(), context.Canceled) {
				t.Fatal("progress did not cancel encode")
			}
			compressionRequireScriptRemoved(t, script)
		})
	}
}
