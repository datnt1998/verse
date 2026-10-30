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

func TestChromeResolutionPrefersConfiguredThenDetectedThenManaged(t *testing.T) {
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
	sysExe := filepath.Join(sysDir, "chromium")
	if err := os.WriteFile(sysExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c.d = Deps{Runner: &fakeRunner{}, Paths: store2, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}
	if got := c.Resolve(); got == nil || got.Path != sysExe || got.Source != ChromeDetected {
		t.Fatalf("resolve = %+v", got)
	}
	// Xoá hệ thống → managed (dựng sẵn bản active).
	key := CurrentPlatformKey()
	if len(ChromePin().Assets[key]) > 0 {
		verDir := ToolVersionDir(base, chromeToolID, ChromePin().Version)
		exe := filepath.Join(verDir, ChromePin().Assets[key][0].Name, filepath.FromSlash(managedChromeExecutable))
		if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ToolRoot(base, chromeToolID), "current"), []byte("v-"+ChromePin().Version), 0o644); err != nil {
			t.Fatal(err)
		}
		c.d.PathEnv = t.TempDir()
		c.d.Home = t.TempDir()
		if got := c.Resolve(); got == nil || got.Path != exe || got.Source != ChromeManaged {
			t.Fatalf("resolve = %+v", got)
		}
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
	if fileExists(old) {
		t.Fatal("dir cũ còn sót")
	}
	if !fileExists(fresh) || !fileExists(other) {
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
	if fileExists(filepath.Join(base, "process-groups", fmt.Sprintf("%d.json", os.Getpid()))) {
		data, _ := os.ReadFile(filepath.Join(base, "process-groups", fmt.Sprintf("%d.json", os.Getpid())))
		if strings.Contains(string(data), fmt.Sprintf("%d", pgid)) {
			t.Fatalf("nhóm %d vẫn còn trong sổ: %s", pgid, data)
		}
	}
}
