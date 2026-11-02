//go:build darwin || linux

package toolsvc

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/toolchain"
)

// chromeProbeRunner giả lập binary Chrome cho probe: `--version` trả banner,
// lần mở headless đổ DOM năng lực audio. Ghi lại mọi lời gọi để test chứng
// minh hàng công cụ không probe Chrome hệ thống.
type chromeProbeRunner struct {
	version string
	dom     string

	mu   sync.Mutex
	runs []chromeRun
}

type chromeRun struct {
	path string
	args []string
}

func (r *chromeProbeRunner) Run(_ context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	r.mu.Lock()
	r.runs = append(r.runs, chromeRun{path: file, args: append([]string(nil), args...)})
	r.mu.Unlock()
	if len(args) == 1 && args[0] == "--version" {
		return tools.Result{Stdout: "Google Chrome " + r.version + "\n"}, nil
	}
	if spec.Log != nil {
		_, _ = spec.Log.Write([]byte(r.dom))
	}
	return tools.Result{Stdout: r.dom}, nil
}

func (r *chromeProbeRunner) ranPath(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, run := range r.runs {
		if run.path == path {
			return true
		}
	}
	return false
}

// managedChromeFixture dựng bố cục bản Chrome for Testing active giống bộ cài
// (thư mục phiên bản + current), không tải/không chạy Chrome thật.
func managedChromeFixture(t *testing.T, base string) string {
	t.Helper()
	pin := tools.ChromePin()
	assets := pin.Assets[toolchain.CurrentPlatformKey()]
	if len(assets) == 0 {
		t.Skip("nền tảng chưa có bản Chrome for Testing ghim")
	}
	verDir := toolchain.ToolVersionDir(base, "chrome", pin.Version)
	exe := filepath.Join(verDir, assets[0].Name,
		filepath.FromSlash("Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"))
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolchain.ToolRoot(base, "chrome"), "current"), []byte("v-"+pin.Version), 0o644); err != nil {
		t.Fatal(err)
	}
	return exe
}

func systemChromeFixture(t *testing.T, dir string) string {
	t.Helper()
	exe := filepath.Join(dir, "chrome")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

// chromeService dựng service thật trên tools.New với runner/CDP giả; DataDir
// là base để đường dẫn cài đặt trong hàng khớp fixture.
func chromeService(t *testing.T, base, sysDir string, runner tools.Runner, cdp func(*exec.Cmd) (int, error)) *Service {
	t.Helper()
	deps := tools.Deps{
		Runner:     runner,
		Paths:      &tools.FilePathStore{Dir: filepath.Join(base, "paths")},
		BaseDir:    base,
		Home:       base,
		PathEnv:    sysDir,
		SystemDirs: []string{},
		CDPSpawn:   cdp,
	}
	return New(Deps{Tools: tools.New(deps), DataDir: base})
}

// fakeChromeCDP trả lời giao thức --remote-debugging-pipe để probe export
// chứng minh WebGPU/encoder mà không mở Chrome thật. FD được dup vì evaluate
// đóng đầu pipe của parent ngay sau spawn.
type fakeChromeCDP struct {
	gpu     bool
	encoder bool

	mu         sync.Mutex
	path       string
	methods    []string
	expression string
}

func (f *fakeChromeCDP) spawn(cmd *exec.Cmd) (int, error) {
	f.mu.Lock()
	f.path = cmd.Path
	f.mu.Unlock()
	inFD, err := syscall.Dup(int(cmd.ExtraFiles[0].Fd()))
	if err != nil {
		return 0, err
	}
	outFD, err := syscall.Dup(int(cmd.ExtraFiles[1].Fd()))
	if err != nil {
		_ = syscall.Close(inFD)
		return 0, err
	}
	go f.serve(os.NewFile(uintptr(inFD), "fake-cdp-in"), os.NewFile(uintptr(outFD), "fake-cdp-out"))
	return 0, nil
}

func (f *fakeChromeCDP) serve(in, out *os.File) {
	defer in.Close()
	defer out.Close()
	reader := bufio.NewReader(in)
	for {
		frame, err := reader.ReadBytes(0)
		if err != nil {
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(frame[:len(frame)-1], &msg) != nil {
			continue
		}
		f.mu.Lock()
		f.methods = append(f.methods, msg.Method)
		if msg.Method == "Runtime.evaluate" {
			var p struct {
				Expression string `json:"expression"`
			}
			_ = json.Unmarshal(msg.Params, &p)
			f.expression = p.Expression
		}
		f.mu.Unlock()

		result := any(map[string]any{})
		switch msg.Method {
		case "Target.createTarget":
			result = map[string]any{"targetId": "target-1"}
		case "Target.attachToTarget":
			result = map[string]any{"sessionId": "session-1"}
		case "Runtime.evaluate":
			result = map[string]any{"result": map[string]any{"value": map[string]any{
				"gpu": f.gpu, "encoder": f.encoder,
			}}}
		}
		reply, _ := json.Marshal(map[string]any{"id": msg.ID, "result": result})
		_, _ = out.Write(append(reply, 0))
		if msg.Method == "Page.navigate" {
			event, _ := json.Marshal(map[string]any{"method": "Page.loadEventFired", "sessionId": "session-1"})
			_, _ = out.Write(append(event, 0))
		}
	}
}

// Máy có Chrome chạy được nhưng bản Chrome for Testing app quản chưa cài:
// hàng công cụ phải báo thiếu, không "sẵn sàng" rồi để measure/export chết
// tool_missing. Chrome hệ thống cũng không được probe hay hiện là Provided.
func TestChromeRowBlocksReadinessWhenOnlySystemChromeExists(t *testing.T) {
	t.Setenv("PUBLIC_URL", "") // bản desktop: Chrome hệ thống có thể bị Resolve chọn
	base := t.TempDir()
	sysDir := t.TempDir()
	sysExe := systemChromeFixture(t, sysDir)
	runner := &chromeProbeRunner{version: "156.0.0.0", dom: "PROBE:ctx:true,aac:true,opus:true"}
	svc := chromeService(t, base, sysDir, runner, nil)

	row := svc.probeChrome(context.Background())
	if row.State != StateMissing {
		t.Fatalf("hàng chrome = %+v (muốn missing vì bản quản chưa cài)", row)
	}
	if row.Provided {
		t.Fatalf("hàng chrome = %+v (Chrome hệ thống không được tính Provided)", row)
	}
	if row.Detail != "" {
		t.Fatalf("detail = %q (không được chỉ Chrome hệ thống)", row.Detail)
	}
	if runner.ranPath(sysExe) {
		t.Fatalf("đã probe Chrome hệ thống: %+v", runner.runs)
	}
	if strings.Contains(row.Hint, "Chrome có sẵn trên máy") {
		t.Fatalf("hint = %q (không được hứa Chrome cá nhân)", row.Hint)
	}
}

// Bản Chrome for Testing app quản có mặt: hàng báo ready trên đúng đường dẫn
// đó, Provided=false, và bằng chứng export (CDP WebGPU + H.264) cũng lấy từ
// chính binary ấy — không phải Chrome hệ thống.
func TestChromeRowReadyOnManagedFixtureWithExportCaps(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	sysExe := systemChromeFixture(t, sysDir)
	managed := managedChromeFixture(t, base)
	pinVersion := tools.ChromePin().Version
	runner := &chromeProbeRunner{version: pinVersion, dom: "PROBE:ctx:true,aac:true,opus:true"}
	cdp := &fakeChromeCDP{gpu: true, encoder: true}
	svc := chromeService(t, base, sysDir, runner, cdp.spawn)

	row := svc.probeChrome(context.Background())
	if row.State != StateReady {
		t.Fatalf("hàng chrome = %+v (muốn ready trên bản quản)", row)
	}
	if row.Provided {
		t.Fatalf("hàng chrome = %+v (bản app quản không phải Provided)", row)
	}
	if row.Detail != managed {
		t.Fatalf("detail = %q, muốn %q", row.Detail, managed)
	}
	if row.Version != pinVersion {
		t.Fatalf("version = %q, muốn %q", row.Version, pinVersion)
	}
	if runner.ranPath(sysExe) {
		t.Fatalf("đã probe Chrome hệ thống thay vì bản quản: %+v", runner.runs)
	}
	cdp.mu.Lock()
	cdpPath, expression := cdp.path, cdp.expression
	cdp.mu.Unlock()
	if cdpPath != managed {
		t.Fatalf("CDP export mở %q, muốn %q", cdpPath, managed)
	}
	if !strings.Contains(expression, "navigator.gpu") || !strings.Contains(expression, "avc1.640028") {
		t.Fatalf("bằng chứng export không hỏi WebGPU/H.264: %q", expression)
	}
}

// Lượt cài bản quản hỏng (ví dụ lệch checksum) không được biến thành "sẵn
// sàng" chỉ vì máy có Chrome hệ thống: hàng giữ lỗi cài thật và việc làm tiếp.
func TestChromeRowReportsFailedManagedInstallDespiteSystemChrome(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	systemChromeFixture(t, sysDir)
	msg := "checksum của chrome-mac-arm64 không khớp"
	state, err := json.Marshal(toolchain.InstallState{Phase: toolchain.PhaseFailed, Error: &msg})
	if err != nil {
		t.Fatal(err)
	}
	root := toolchain.ToolRoot(base, "chrome")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "install-state.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
	deps := tools.Deps{
		Runner:     &chromeProbeRunner{version: "156.0.0.0", dom: "PROBE:ctx:true,aac:true,opus:true"},
		Paths:      &tools.FilePathStore{Dir: filepath.Join(base, "paths")},
		BaseDir:    base,
		Home:       base,
		PathEnv:    sysDir,
		SystemDirs: []string{},
	}
	kit := tools.New(deps)
	svc := New(Deps{Tools: kit, Managed: tools.NewManaged(deps, nil, kit), DataDir: base})

	row := svc.probeChrome(context.Background())
	if row.State != StateFailed {
		t.Fatalf("hàng chrome = %+v (muốn failed vì lượt cài hỏng)", row)
	}
	if !strings.Contains(row.Hint, "chữ ký") || !strings.Contains(row.Hint, "Tải tiếp") {
		t.Fatalf("hint = %q (muốn lỗi cài thật + việc làm tiếp)", row.Hint)
	}
	if row.Provided {
		t.Fatalf("hàng chrome = %+v (không được Provided từ Chrome hệ thống)", row)
	}
}

func TestChromeRowFailsActionablyWhenAudioCapsUnsupported(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	systemChromeFixture(t, sysDir)
	managedChromeFixture(t, base)
	runner := &chromeProbeRunner{version: tools.ChromePin().Version, dom: "PROBE:ctx:true,aac:false,opus:false"}
	svc := chromeService(t, base, sysDir, runner, nil)

	row := svc.probeChrome(context.Background())
	if row.State != StateFailed {
		t.Fatalf("hàng chrome = %+v (muốn failed vì thiếu decoder)", row)
	}
	for _, want := range []string{"Chrome for Testing", "đo âm thanh", "cài lại"} {
		if !strings.Contains(row.Hint, want) {
			t.Fatalf("hint = %q (thiếu %q — không actionable)", row.Hint, want)
		}
	}
}

// Bản quản chạy được nhưng headless không có WebGPU/encoder H.264: hàng phải
// failed với hướng xử lý cụ thể (thiếu gì, cài lại bản ghim).
func TestChromeRowFailsActionablyWhenExportCapsUnsupported(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	systemChromeFixture(t, sysDir)
	managedChromeFixture(t, base)
	runner := &chromeProbeRunner{version: tools.ChromePin().Version, dom: "PROBE:ctx:true,aac:true,opus:true"}
	cdp := &fakeChromeCDP{gpu: false, encoder: false}
	svc := chromeService(t, base, sysDir, runner, cdp.spawn)

	row := svc.probeChrome(context.Background())
	if row.State != StateFailed {
		t.Fatalf("hàng chrome = %+v (muốn failed vì export thiếu năng lực)", row)
	}
	for _, want := range []string{"WebGPU", "H.264", "cài lại"} {
		if !strings.Contains(row.Hint, want) {
			t.Fatalf("hint = %q (thiếu %q — không actionable)", row.Hint, want)
		}
	}
}
