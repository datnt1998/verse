// Chạy sidecar Python clone-transcribe.py trong nhóm process riêng — Go chỉ
// giám sát: stream event JSONL stderr → progress, đọc report JSON ở --report.
// Chuyển từ defaultRunTranscribe trong transcript.ts (2c201e6).
package transcript

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// cloneTranscribeScript là bản embed của apps/viderse/scripts/clone-transcribe.py
// — giữ hai bản đồng byte (kiểm ở TestSidecarEmbeddedMatchesScript) để bước
// transcript không phụ thuộc layout thư mục lúc chạy.
//
//go:embed clone-transcribe.py
var cloneTranscribeScript []byte

// TranscribeRun là tham số một lượt clone-transcribe.py.
type TranscribeRun struct {
	Mode       string // "vad" | "asr"
	InputWav   string
	ReportPath string
	Regions    []Interval
	WindowSec  float64
	ContextSec float64
	Language   string
	ModelSize  string
	BeamSize   int64
	OnProgress func(doneSec, totalSec float64)
	OnLog      func(line string)
}

// StepRunner là seam process của step (ctx.ProcessRunner theo contract).
type StepRunner interface {
	Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error)
}

type runnerFunc func(context.Context, string, []string, tools.Spec) (tools.Result, error)

func (f runnerFunc) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return f(ctx, file, args, spec)
}

// scriptPath ghi sidecar ra staging (lần đầu) và trả đường dẫn — file nằm
// trong fence của step, bị xoá cùng generation.
func scriptPath(stagingDir string) (string, error) {
	dir := filepath.Join(stagingDir, "transcript")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "clone-transcribe.py")
	cur, err := os.ReadFile(p)
	if err == nil && bytes.Equal(cur, cloneTranscribeScript) {
		return p, nil
	}
	if err := os.WriteFile(p, cloneTranscribeScript, 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// SidecarRunner chạy sidecar qua runner của step (process group riêng; huỷ
// step giết cả nhóm). Python là binary của venv transcription; Env từ
// tools.Transcription.RunEnv. Adapter chạy tới khi thoát — tiến độ đi qua
// event JSONL trên stderr, không bộ đếm thời gian bên ngoài.
type SidecarRunner struct {
	Runner  StepRunner
	Python  string
	Uv      string
	Env     map[string]string
	Staging string
}

func (r SidecarRunner) Run(ctx context.Context, run TranscribeRun) (any, error) {
	script, err := scriptPath(r.Staging)
	if err != nil {
		return nil, err
	}
	args := []string{
		script,
		"--mode", run.Mode,
		"--input", run.InputWav,
		"--report", run.ReportPath,
		"--threads", "4",
	}
	if run.Mode == "vad" {
		window := run.WindowSec
		if window <= 0 {
			window = VADWindowSec
		}
		contextSec := run.ContextSec
		if contextSec <= 0 {
			contextSec = VADContextSec
		}
		args = append(args, "--window", formatSec(window), "--context", formatSec(contextSec))
	} else {
		size := run.ModelSize
		if size == "" {
			size = "small"
		}
		beam := run.BeamSize
		if beam <= 0 {
			beam = 5
		}
		args = append(args, "--model-size", size, "--beam-size", fmt.Sprint(beam))
		if run.Language != "" {
			args = append(args, "--language", run.Language)
		}
	}
	if len(run.Regions) > 0 {
		parts := make([]string, 0, len(run.Regions))
		for _, rg := range run.Regions {
			parts = append(parts, fmt.Sprintf("%g,%g", rg.StartMs/1000, rg.EndMs/1000))
		}
		args = append(args, "--ranges", strings.Join(parts, ";"))
	}

	onLine := func(line string, stream tools.Stream) {
		if stream != tools.StreamStderr {
			return
		}
		ev := ParseAdapterEvent(line)
		if ev == nil {
			return
		}
		if ev["event"] == "progress" {
			done, _ := ev["done"].(float64)
			total, _ := ev["total"].(float64)
			if run.OnProgress != nil {
				run.OnProgress(done, total)
			}
		} else if ev["event"] != "done" && run.OnLog != nil {
			run.OnLog(strings.TrimSpace(line))
		}
	}
	args = append([]string{"run", "--no-project", "--no-sync", "--python", r.Python}, args...)
	timeout := 3 * time.Hour
	if run.Mode == "vad" {
		timeout = time.Hour
	}
	_, err = r.Runner.Run(ctx, r.Uv, args, tools.Spec{
		Label:      "clone-transcribe",
		Env:        r.Env,
		OnLine:     onLine,
		Background: true,
		Timeout:    timeout,
	})
	if err != nil {
		var pfe *tools.ProcessFailedError
		if errors.As(err, &pfe) && !pfe.TimedOut {
			return nil, fmt.Errorf("clone-transcribe thất bại (%d): %s", pfe.Code, pfe.Stderr)
		}
		return nil, err
	}
	raw, err := os.ReadFile(run.ReportPath)
	if err != nil {
		return nil, fmt.Errorf("clone-transcribe không ghi report: %w", err)
	}
	var report any
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("clone-transcribe report không phải JSON: %w", err)
	}
	return report, nil
}

func formatSec(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}
