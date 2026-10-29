// Test chuyển từ src/server/tools/codex.test.ts của yt-factory.
package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// codexRunner mô phỏng các lời gọi probe: `--version`, `login status`,
// `exec --help`…
type codexRunner struct {
	version string
	login   string // "chatgpt" | "api_key" | "not_logged_in" | "unknown"
	isolate CodexIsolation
	err     error
	ran     [][]string
}

func (c *codexRunner) Run(ctx context.Context, file string, args []string, spec Spec) (Result, error) {
	c.ran = append(c.ran, append([]string{file}, args...))
	if c.err != nil {
		return Result{}, c.err
	}
	if len(args) > 0 {
		switch args[0] {
		case "--version":
			return Result{Stdout: "codex-cli " + c.version + "\n"}, nil
		case "login":
			return Result{Stdout: c.login + "\n"}, nil
		case "exec":
			return Result{Stdout: "--json --ephemeral --ignore-user-config --cd --skip-git-repo-check --output-last-message\n"}, nil
		}
	}
	return Result{Stdout: ""}, nil
}

func okCodexRunner() *codexRunner {
	return &codexRunner{
		version: CodexPin().Version,
		login:   "Logged in using ChatGPT",
		isolate: CodexIsolation{State: IsolationVerified},
	}
}

func newTestCodex(t *testing.T, runner Runner, base string) *Codex {
	t.Helper()
	return &Codex{d: Deps{Runner: runner, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base},
		verifiedIsolation: NewFileVerificationCache[CodexIsolation](
			func(is CodexIsolation) bool { return is.State == IsolationVerified }, 0)}
}

func TestParseCodexVersion(t *testing.T) {
	if v := ParseCodexVersion("codex-cli 0.97.0\n"); v != "0.97.0" {
		t.Fatalf("version = %q", v)
	}
	if v := ParseCodexVersion("codex-cli 0.162.1"); v != "0.162.1" {
		t.Fatalf("version = %q", v)
	}
}

func TestCodexResolvePrefersConfigured(t *testing.T) {
	base := t.TempDir()
	configured := writeExecutable(t, t.TempDir(), "codex")
	store := &FilePathStore{Dir: t.TempDir()}
	if err := store.Set(CodexSettingKey, configured); err != nil {
		t.Fatal(err)
	}
	c := newTestCodex(t, okCodexRunner(), base)
	c.d.Paths = store
	if got := c.Resolve(context.Background()); got != configured {
		t.Fatalf("resolve = %q", got)
	}
}

func TestCodexResolvePicksManaged(t *testing.T) {
	base := t.TempDir()
	verDir := ToolVersionDir(base, codexToolID, CodexPin().Version)
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(verDir, "codex")
	if err := os.WriteFile(managed, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ToolRoot(base, codexToolID), "current"), []byte("v-"+CodexPin().Version), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newTestCodex(t, okCodexRunner(), base)
	if got := c.Resolve(context.Background()); got != managed {
		t.Fatalf("resolve = %q", got)
	}
	if got := c.ManagedPath(); got != managed {
		t.Fatalf("managed = %q", got)
	}
}

func TestCodexStatusMissingIsDistinctFromBroken(t *testing.T) {
	base := t.TempDir()
	// Không có gì: missing.
	c := newTestCodex(t, okCodexRunner(), base)
	st := c.Status(context.Background())
	if st.State != CodexMissing {
		t.Fatalf("state = %s", st.State)
	}
	// Ứng viên tồn tại nhưng không chạy được: broken.
	sysDir := t.TempDir()
	badExe := filepath.Join(sysDir, "codex")
	if err := os.WriteFile(badExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &codexRunner{err: &ProcessFailedError{Message: "spawn ENOENT", Code: -1}}
	c2 := newTestCodex(t, runner, base)
	c2.d.PathEnv = sysDir
	st = c2.Status(context.Background())
	if st.State != CodexBroken {
		t.Fatalf("state = %s", st.State)
	}
	if len(st.Broken) == 0 || st.Broken[0].Path == "" || st.Broken[0].Error == "" {
		t.Fatalf("broken = %+v", st.Broken)
	}
}

func TestCodexStatusOutdated(t *testing.T) {
	base := t.TempDir()
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "codex")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := newTestCodex(t, &codexRunner{version: "0.50.0", login: "x"}, base)
	c.d.PathEnv = sysDir
	st := c.Status(context.Background())
	if st.State != CodexOutdated || st.Binary == nil || st.Binary.Version != "0.50.0" {
		t.Fatalf("status = %+v", st)
	}
}

func TestCodexStatusLoggedOut(t *testing.T) {
	base := t.TempDir()
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "codex")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := newTestCodex(t, &codexRunner{version: CodexPin().Version, login: "Not logged in"}, base)
	c.d.PathEnv = sysDir
	st := c.Status(context.Background())
	if st.State != CodexLoggedOut {
		t.Fatalf("status = %+v", st)
	}
}

func TestCodexStatusApiKeyLoginCountsAsLoggedOut(t *testing.T) {
	base := t.TempDir()
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "codex")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := newTestCodex(t, &codexRunner{version: CodexPin().Version, login: "Logged in using API key"}, base)
	c.d.PathEnv = sysDir
	if st := c.Status(context.Background()); st.State != CodexLoggedOut {
		t.Fatalf("status = %+v", st)
	}
}
