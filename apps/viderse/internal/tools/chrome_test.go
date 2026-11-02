// Test chuyển từ src/server/tools/chrome.test.ts của yt-factory.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/toolchain"
)

func TestParseChromeVersion(t *testing.T) {
	if v := ParseChromeVersion("Google Chrome 131.0.6778.205\n"); v != "131.0.6778.205" {
		t.Fatalf("version = %q", v)
	}
	if v := ParseChromeVersion("Chromium 130.0.6723.116\n"); v != "130.0.6723.116" {
		t.Fatalf("version = %q", v)
	}
	if v := ParseChromeVersion("not a version"); v != "" {
		t.Fatalf("version = %q", v)
	}
}

func TestParseProbeDomExtractsCaps(t *testing.T) {
	dom := "<html><body>PROBE:ctx:true,aac:true,opus:false</body></html>"
	caps := ParseProbeDom(dom)
	if caps == nil || !caps.OfflineAudioContext || !caps.AAC || caps.Opus {
		t.Fatalf("caps = %+v", caps)
	}
	if ParseProbeDom("<html>no marker</html>") != nil {
		t.Fatal("không marker mà vẫn đọc được caps")
	}
}

// installManagedChrome dựng sẵn bản Chrome for Testing active dưới base (đúng
// bố cục thư mục mà bộ cài tạo ra) và trả đường dẫn executable; nền tảng chưa
// có bản ghim thì bỏ qua test.
func installManagedChrome(t *testing.T, base string) string {
	t.Helper()
	key := toolchain.CurrentPlatformKey()
	assets := ChromePin().Assets[key]
	if len(assets) == 0 {
		t.Skip("nền tảng chưa có bản Chrome for Testing ghim")
	}
	verDir := toolchain.ToolVersionDir(base, chromeToolID, ChromePin().Version)
	exe := filepath.Join(verDir, assets[0].Name, filepath.FromSlash(managedChromeExecutable))
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolchain.ToolRoot(base, chromeToolID), "current"), []byte("v-"+ChromePin().Version), 0o644); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestChromeResolutionPrefersConfiguredThenDetectedThenManaged(t *testing.T) {
	t.Setenv("PUBLIC_URL", "") // bản desktop: có Chrome hệ thống thì dùng
	base := t.TempDir()
	store := &FilePathStore{Dir: t.TempDir()}
	empty := Deps{Runner: &fakeRunner{}, Paths: store, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}
	c := &Chrome{d: empty}
	if got := c.Resolve(); got != nil {
		t.Fatalf("resolve = %+v", got)
	}
	if err := store.Set(ChromeSettingKey, "/opt/custom/chrome"); err != nil {
		t.Fatal(err)
	}
	if got := c.Resolve(); got == nil || got.Path != "/opt/custom/chrome" || got.Source != ChromeConfigured {
		t.Fatalf("resolve = %+v", got)
	}
	// Xoá cấu hình → hệ thống.
	store2 := &FilePathStore{Dir: t.TempDir()}
	sysDir := t.TempDir()
	sysExe := writeExecutable(t, sysDir, "chromium")
	c.d = Deps{Runner: &fakeRunner{}, Paths: store2, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}
	if got := c.Resolve(); got == nil || got.Path != sysExe || got.Source != ChromeDetected {
		t.Fatalf("resolve = %+v", got)
	}
	// Xoá hệ thống → managed (dựng sẵn bản active).
	exe := installManagedChrome(t, base)
	c.d.PathEnv = t.TempDir()
	c.d.Home = t.TempDir()
	if got := c.Resolve(); got == nil || got.Path != exe || got.Source != ChromeManaged {
		t.Fatalf("resolve = %+v", got)
	}
}

// Máy chủ không mượn trình duyệt cá nhân của chủ máy: Chrome hệ thống có trên
// máy bị bỏ hẳn — không chọn, không probe, không spawn — nên hàng Chrome báo
// thiếu để bước cài tự cài Chrome for Testing.
func TestChromeHostedIgnoresSystemChromeAndLaunchesNothing(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example.com")
	base := t.TempDir()
	sysDir := t.TempDir()
	writeExecutable(t, sysDir, "chrome")
	runner := &fakeRunner{}
	c := &Chrome{d: Deps{Runner: runner, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}}
	if got := c.Resolve(); got != nil {
		t.Fatalf("resolve = %+v (muốn nil)", got)
	}
	if st := c.Status(context.Background()); st.State != ChromeMissing {
		t.Fatalf("status = %+v (muốn missing)", st)
	}
	if caps := c.ExportCaps(context.Background()); caps.State != ExportMissing {
		t.Fatalf("export caps = %+v (muốn missing)", caps)
	}
	if len(runner.ran) != 0 {
		t.Fatalf("đã chạy Chrome của máy ở chế độ máy chủ: %v", runner.ran)
	}
}

// Máy chủ chỉ dùng bản Chrome for Testing app tự cài, kể cả khi máy có Chrome
// hệ thống.
func TestChromeHostedPrefersManagedOverSystem(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example.com")
	base := t.TempDir()
	sysDir := t.TempDir()
	writeExecutable(t, sysDir, "chrome")
	managed := installManagedChrome(t, base)
	c := &Chrome{d: Deps{Runner: &fakeRunner{}, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}}
	if got := c.Resolve(); got == nil || got.Path != managed || got.Source != ChromeManaged {
		t.Fatalf("resolve = %+v (muốn managed %s)", got, managed)
	}
}

// Ở chế độ máy chủ, đường dẫn chủ máy tự chọn vẫn thắng: đó là quyết định
// tường minh, không phải tự bám lấy Chrome hệ thống.
func TestChromeHostedKeepsConfiguredPath(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://vid.example.com")
	base := t.TempDir()
	store := &FilePathStore{Dir: t.TempDir()}
	if err := store.Set(ChromeSettingKey, "/opt/custom/chrome"); err != nil {
		t.Fatal(err)
	}
	sysDir := t.TempDir()
	writeExecutable(t, sysDir, "chrome")
	installManagedChrome(t, base)
	c := &Chrome{d: Deps{Runner: &fakeRunner{}, Paths: store, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}}
	if got := c.Resolve(); got == nil || got.Path != "/opt/custom/chrome" || got.Source != ChromeConfigured {
		t.Fatalf("resolve = %+v", got)
	}
}

// Hàng Chrome dùng cho export/measure phải nói về bản Chrome for Testing app
// quản, không phải Chrome hệ thống: máy có Chrome chạy được vẫn không được
// tính là sẵn sàng khi bản quản chưa cài (export sẽ chết tool_missing).
func TestChromeManagedStatusNeverClaimsSystemChrome(t *testing.T) {
	t.Setenv("PUBLIC_URL", "") // bản desktop: Resolve có thể thấy Chrome hệ thống
	base := t.TempDir()
	sysDir := t.TempDir()
	sysExe := writeExecutable(t, sysDir, "chrome")
	runner := &fakeRunner{}
	c := &Chrome{d: Deps{Runner: runner, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}}
	if got := c.Resolve(); got == nil || got.Path != sysExe {
		t.Fatalf("resolve = %+v (muốn Chrome hệ thống có mặt)", got)
	}
	if st := c.ManagedStatus(context.Background()); st.State != ChromeMissing || st.Path != "" {
		t.Fatalf("managed status = %+v (muốn missing, không mượn Chrome hệ thống)", st)
	}
	if caps := c.ManagedExportCaps(context.Background()); caps.State != ExportMissing {
		t.Fatalf("managed export caps = %+v (muốn missing)", caps)
	}
	if len(runner.ran) != 0 {
		t.Fatalf("đã probe Chrome hệ thống cho hàng export/measure: %v", runner.ran)
	}
}

// Khi bản Chrome for Testing app quản có mặt, ManagedStatus probe đúng binary
// đó (kể cả máy cũng có Chrome hệ thống) và trả bằng chứng năng lực.
func TestChromeManagedStatusProbesManagedFixture(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	writeExecutable(t, sysDir, "chrome")
	managed := installManagedChrome(t, base)
	runner := &scriptedRunner{version: "155.0.8059.79", dom: "PROBE:ctx:true,aac:true,opus:false"}
	c := &Chrome{d: Deps{Runner: runner, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base},
		verifiedProbe: NewFileVerificationCache[chromeProbe](func(p chromeProbe) bool { return p.Caps != nil && chromeCapsOk(*p.Caps) }, 0)}
	st := c.ManagedStatus(context.Background())
	if st.State != ChromeReady || st.Path != managed || st.Source != ChromeManaged || st.Version != "155.0.8059.79" {
		t.Fatalf("managed status = %+v (muốn ready trên %s)", st, managed)
	}
	if st.Path == filepath.Join(sysDir, "chrome") {
		t.Fatal("managed status lại probe Chrome hệ thống")
	}
}

func TestChromeStatusMissingWhenNoBinary(t *testing.T) {
	base := t.TempDir()
	c := &Chrome{d: Deps{Runner: &fakeRunner{}, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	if st := c.Status(context.Background()); st.State != ChromeMissing {
		t.Fatalf("state = %s", st.State)
	}
}

// scriptedRunner phát DOM probe khi được gọi với --dump-dom.
type scriptedRunner struct {
	version string
	dom     string
	err     error
	// launches giữ args của mọi lần mở Chrome thật (không tính --version).
	launches [][]string
}

func (s *scriptedRunner) Run(ctx context.Context, file string, args []string, spec Spec) (Result, error) {
	if s.err != nil {
		return Result{}, s.err
	}
	for _, a := range args {
		if a == "--version" {
			return Result{Stdout: "Google Chrome " + s.version + "\n"}, nil
		}
	}
	s.launches = append(s.launches, append([]string(nil), args...))
	if spec.Log != nil {
		_, _ = spec.Log.Write([]byte(s.dom))
	}
	return Result{Stdout: s.dom}, nil
}

// Probe năng lực và probe export đều mở Chrome với profile mới; thiếu cờ khoá
// giả, Chrome trên macOS hỏi Keychain của người dùng ở mỗi lần mở.
func TestChromeProbesNeverAskSystemKeychain(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "chrome")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &scriptedRunner{version: "131.0.6778.205", dom: "PROBE:ctx:true,aac:true,opus:true"}
	var cdpArgs []string
	c := &Chrome{d: Deps{Runner: runner, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base,
		CDPSpawn: func(cmd *exec.Cmd) (int, error) {
			cdpArgs = append([]string(nil), cmd.Args[1:]...)
			return 0, fmt.Errorf("không mở Chrome thật trong test")
		}}}
	if st := c.Status(context.Background()); st.State != ChromeReady {
		t.Fatalf("status = %+v", st)
	}
	c.ExportCaps(context.Background())
	if len(runner.launches) != 1 || cdpArgs == nil {
		t.Fatalf("launches = %v, cdp = %v", runner.launches, cdpArgs)
	}
	for name, args := range map[string][]string{"probe": runner.launches[0], "export": cdpArgs} {
		joined := " " + strings.Join(args, " ") + " "
		for _, flag := range []string{"--use-mock-keychain", "--password-store=basic", "--headless=new"} {
			if !strings.Contains(joined, " "+flag+" ") {
				t.Errorf("%s args thiếu %s: %v", name, flag, args)
			}
		}
	}
}

func TestChromeStatusBrokenWhenBinaryFails(t *testing.T) {
	base := t.TempDir()
	store := &FilePathStore{Dir: t.TempDir()}
	if err := store.Set(ChromeSettingKey, "/missing/chrome"); err != nil {
		t.Fatal(err)
	}
	c := &Chrome{d: Deps{Runner: &fakeRunner{}, Paths: store, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base},
		verifiedProbe: NewFileVerificationCache[chromeProbe](func(p chromeProbe) bool { return p.Caps != nil && chromeCapsOk(*p.Caps) }, 0)}
	st := c.Status(context.Background())
	if st.State != ChromeBroken {
		t.Fatalf("state = %s %+v", st.State, st)
	}
}

func TestChromeStatusReadyThroughScriptedProbe(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	store := &FilePathStore{Dir: t.TempDir()}
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "chrome")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &scriptedRunner{version: "131.0.6778.205", dom: "PROBE:ctx:true,aac:true,opus:true"}
	c := &Chrome{d: Deps{Runner: runner, Paths: store, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base},
		verifiedProbe: NewFileVerificationCache[chromeProbe](func(p chromeProbe) bool { return p.Caps != nil && chromeCapsOk(*p.Caps) }, 0)}
	st := c.Status(context.Background())
	if st.State != ChromeReady || st.Version != "131.0.6778.205" {
		t.Fatalf("status = %+v", st)
	}
}

func TestChromeStatusIncompatibleWhenNoDecoder(t *testing.T) {
	t.Setenv("PUBLIC_URL", "")
	base := t.TempDir()
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "chrome")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &scriptedRunner{version: "131.0.6778.205", dom: "PROBE:ctx:true,aac:false,opus:false"}
	c := &Chrome{d: Deps{Runner: runner, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base},
		verifiedProbe: NewFileVerificationCache[chromeProbe](func(p chromeProbe) bool { return p.Caps != nil && chromeCapsOk(*p.Caps) }, 0)}
	st := c.Status(context.Background())
	if st.State != ChromeIncompatible {
		t.Fatalf("status = %+v", st)
	}
}

func TestStaleProbeDirsAreSweptOnlyWhenOld(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, ChromeProbePrefix+"dead")
	fresh := filepath.Join(root, ChromeProbePrefix+"live")
	other := filepath.Join(root, "unrelated-dir")
	for _, d := range []string{old, fresh, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-2 * staleProbeAge)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	stale := ListStaleTempDirs(root, []string{ChromeProbePrefix}, staleProbeAge, time.Now())
	if len(stale) != 1 || stale[0] != old {
		t.Fatalf("stale = %v", stale)
	}
	SweepStaleTempDirs(root, []string{ChromeProbePrefix}, staleProbeAge)
	if toolchain.FileExists(old) {
		t.Fatal("dir cũ còn sót")
	}
	if !toolchain.FileExists(fresh) || !toolchain.FileExists(other) {
		t.Fatal("dir tươi hoặc dir lạ bị xoá")
	}
}

// Chrome headless CDP spawn đi qua sổ Registry khi Deps cấp: nhóm được ghi
// trong lúc chạy và xoá sau khi probe kết thúc (hoặc bị dọn bởi reaper khi
// owner chết). Không ghi thì server sập giữa probe để lại Chrome mồ côi.
func TestCDPSpawnTracksThenForgetsGroupInRegistry(t *testing.T) {
	if isWindows() {
		t.Skip("POSIX")
	}
	base := t.TempDir()
	reg := NewRegistry(base, nil)
	c := &Chrome{d: Deps{
		Runner:   &ExecRunner{},
		BaseDir:  base,
		Registry: reg,
		CDPSpawn: func(cmd *exec.Cmd) (int, error) {
			// Con thật trong nhóm riêng: Track/TerminateGroup phải xử lý nó.
			real := exec.Command(testShell, "-c", "sleep 30")
			makeDetached(real)
			if err := real.Start(); err != nil {
				return 0, err
			}
			go real.Wait()
			return real.Process.Pid, nil
		},
	}}
	var pgid int
	origSpawn := c.d.CDPSpawn
	c.d.CDPSpawn = func(cmd *exec.Cmd) (int, error) {
		p, err := origSpawn(cmd)
		pgid = p
		return p, err
	}
	_, err := c.evaluateInHeadlessPage(context.Background(), "/nonexistent-chrome",
		nil, "about:blank", "1", 300*time.Millisecond)
	if err == nil {
		t.Fatal("probe không lỗi")
	}
	if pgid <= 1 {
		t.Fatalf("pgid = %d", pgid)
	}
	// Nhóm bị giết và bản ghi bị gỡ.
	if pidAlive(pgid) {
		t.Fatalf("nhóm %d còn sống sau probe", pgid)
	}
	if toolchain.FileExists(filepath.Join(base, "process-groups", fmt.Sprintf("%d.json", os.Getpid()))) {
		data, _ := os.ReadFile(filepath.Join(base, "process-groups", fmt.Sprintf("%d.json", os.Getpid())))
		if strings.Contains(string(data), fmt.Sprintf("%d", pgid)) {
			t.Fatalf("nhóm %d vẫn còn trong sổ: %s", pgid, data)
		}
	}
}
