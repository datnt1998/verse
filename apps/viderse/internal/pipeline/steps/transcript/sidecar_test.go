package transcript

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

func TestSidecarUsesUvManagedPythonAndSeconds(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.json")
	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (tools.Result, error) {
		raw, _ := json.Marshal(vadReport())
		err := os.WriteFile(reportPath, raw, 0600)
		return tools.Result{}, err
	}}
	_, err := (SidecarRunner{Runner: runner, Uv: "/managed/uv", Python: "/env/bin/python", Staging: dir}).Run(context.Background(), TranscribeRun{Mode: "vad", InputWav: "/x.wav", ReportPath: reportPath, Regions: []Interval{{1500, 2750}}, WindowSec: 30, ContextSec: 2})
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(runner.lastArgs, " ")
	for _, want := range []string{"run --no-project --no-sync --python /env/bin/python", "--ranges 1.5,2.75", "--window 30 --context 2", "--threads 4"} {
		if !strings.Contains(argv, want) {
			t.Fatalf("missing %q: %s", want, argv)
		}
	}
	if runner.lastFile != "/managed/uv" {
		t.Fatalf("binary %s", runner.lastFile)
	}
}

func TestSidecarCancelKillsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "uv-fixture")
	// Real descendant, not a mocked process result. The wrapper accepts the uv
	// argv but intentionally never imports a Python package or uses the network.
	script := "#!/bin/sh\nsleep 60 &\nchild=$!\ntrap 'wait \"$child\"; exit 0' TERM\nprintf '%s\\n' \"$child\"\nwait \"$child\"\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	ready := make(chan int, 1)
	runner := runnerFunc(func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
		original := spec.OnLine
		spec.OnLine = func(line string, stream tools.Stream) {
			if stream == tools.StreamStdout {
				if pid, e := strconv.Atoi(line); e == nil {
					select {
					case ready <- pid:
					default:
					}
				}
			}
			if original != nil {
				original(line, stream)
			}
		}
		return (&tools.ExecRunner{}).Run(ctx, file, args, spec)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := (SidecarRunner{Runner: runner, Uv: executable, Python: "ignored", Staging: dir}).Run(ctx, TranscribeRun{Mode: "vad", ReportPath: filepath.Join(dir, "report.json")})
		done <- err
	}()
	var child int
	select {
	case child = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("child did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, toolchain.ErrCancelled) && !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not stop")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err := syscall.Kill(child, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(fmt.Sprintf("orphaned child %d: %v", child, err))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
