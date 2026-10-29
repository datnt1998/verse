// Test chuyển từ src/server/tools/yt-dlp.test.ts của yt-factory.
package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner là Runner giả trả Result hoặc lỗi theo file được gọi.
type fakeRunner struct {
	execFn func(file string, args []string, spec Spec) (Result, error)
	ran    [][]string
}

func (f *fakeRunner) Run(ctx context.Context, file string, args []string, spec Spec) (Result, error) {
	f.ran = append(f.ran, append([]string{file}, args...))
	if f.execFn != nil {
		return f.execFn(file, args, spec)
	}
	return Result{}, &ProcessFailedError{Message: "spawn ENOENT", Code: -1}
}

func TestClassifyYtDlpErrorLoginRequired(t *testing.T) {
	stderr := "ERROR: [youtube] abc123: Sign in to confirm you're not a bot"
	e := ClassifyYtDlpError(stderr, "")
	if e.Code != YtDlpLoginRequired {
		t.Fatalf("code = %s", e.Code)
	}
	if e.StderrTail == "" {
		t.Fatal("stderr tail rỗng")
	}
}

func TestClassifyYtDlpErrorCookieKeychainFailureWinsOverSignInHint(t *testing.T) {
	stderr := "WARNING: [youtube] Sign in to confirm you're not a bot\nERROR: could not find cookie entry: cookies database locked"
	e := ClassifyYtDlpError(stderr, "")
	if e.Code != YtDlpCookiesFailed {
		t.Fatalf("code = %s", e.Code)
	}
	if e.StderrTail == "" {
		t.Fatal("stderr tail rỗng")
	}
}

func TestClassifyYtDlpErrorBilibiliAntiScrapeIsNotUnavailability(t *testing.T) {
	e := ClassifyYtDlpError("ERROR: [BiliBili] 1xxyyzz: Request is blocked by server (412)", "")
	if e.Code != YtDlpRateLimited {
		t.Fatalf("code = %s", e.Code)
	}
	if !strings.Contains(e.Message, "blocked by server") {
		t.Fatalf("message = %q", e.Message)
	}
	e = ClassifyYtDlpError("ERROR: Request is rejected by server (352)", "")
	if e.Code != YtDlpRateLimited {
		t.Fatalf("code = %s", e.Code)
	}
}

func TestClassifyYtDlpErrorUnavailableAndPlainFailure(t *testing.T) {
	e := ClassifyYtDlpError("ERROR: [youtube] xyz: Video unavailable. This video is private", "")
	if e.Code != YtDlpUnavailable {
		t.Fatalf("code = %s", e.Code)
	}
	e = ClassifyYtDlpError("ERROR: unable to download video data: HTTP Error 403: Forbidden", "")
	if e.Code != YtDlpFailed {
		t.Fatalf("code = %s", e.Code)
	}
	if e.Message != "unable to download video data: HTTP Error 403: Forbidden" {
		t.Fatalf("message = %q", e.Message)
	}
}

func TestClassifyYtDlpErrorUsesLastErrorLine(t *testing.T) {
	stderr := "ERROR: first error\nWARNING: something\nERROR: final error"
	e := ClassifyYtDlpError(stderr, "")
	if e.Message != "final error" {
		t.Fatalf("message = %q", e.Message)
	}
}

func TestYtDlpStatusWithoutBinaryIsMissing(t *testing.T) {
	base := t.TempDir()
	y := &YtDlp{d: Deps{Runner: &fakeRunner{}, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	v, p, ok := y.Status(context.Background())
	if ok || v != "" || p != "" {
		t.Fatalf("status = %q %q %v", v, p, ok)
	}
}

func TestYtDlpResolvePrefersConfigured(t *testing.T) {
	base := t.TempDir()
	store := &FilePathStore{Dir: base}
	if err := store.Set("tools:yt-dlp", "/opt/custom/yt-dlp"); err != nil {
		t.Fatal(err)
	}
	y := &YtDlp{d: Deps{Runner: &fakeRunner{}, Paths: store, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	if got := y.Resolve(); got != "/opt/custom/yt-dlp" {
		t.Fatalf("resolve = %q", got)
	}
}

func TestYtDlpResolveSystemBeforeManaged(t *testing.T) {
	base := t.TempDir()
	// Dựng một "system" binary ở chỗ PATH và một managed ở bin/yt-dlp.
	sysDir := t.TempDir()
	sysExe := filepath.Join(sysDir, "yt-dlp")
	if err := os.WriteFile(sysExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	asset, _ := ytDlpPlatformAsset()
	managedExe := filepath.Join(managed, asset.exe)
	if err := os.WriteFile(managedExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	y := &YtDlp{d: Deps{Runner: &fakeRunner{}, BaseDir: base, PathEnv: sysDir, SystemDirs: []string{}, Home: base}}
	if got := y.Resolve(); got != sysExe {
		t.Fatalf("resolve = %q, muốn hệ thống %q", got, sysExe)
	}
}

func TestYtDlpResolveFallsBackToManaged(t *testing.T) {
	base := t.TempDir()
	managed := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	asset, _ := ytDlpPlatformAsset()
	managedExe := filepath.Join(managed, asset.exe)
	if err := os.WriteFile(managedExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	y := &YtDlp{d: Deps{Runner: &fakeRunner{}, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	if got := y.Resolve(); got != managedExe {
		t.Fatalf("resolve = %q", got)
	}
}

func TestYtDlpRunAutoInstallsThenRuns(t *testing.T) {
	base := t.TempDir()
	managed := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	asset, _ := ytDlpPlatformAsset()
	exe := filepath.Join(managed, asset.exe)
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{execFn: func(file string, args []string, spec Spec) (Result, error) {
		return Result{Stdout: "https://youtu.be/abc\n"}, nil
	}}
	y := &YtDlp{d: Deps{Runner: runner, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	out, err := y.Run(context.Background(), []string{"--print", "%(original_url)s"}, RunYtDlpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://youtu.be/abc") {
		t.Fatalf("out = %q", out)
	}
	last := runner.ran[len(runner.ran)-1]
	joined := strings.Join(last, " ")
	if !strings.Contains(joined, "--ignore-config") || !strings.Contains(joined, "--cache-dir") {
		t.Fatalf("argv = %v", last)
	}
}

func TestYtDlpRunMapsProcessFailureToYtDlpError(t *testing.T) {
	base := t.TempDir()
	managed := filepath.Join(base, "bin", "yt-dlp")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	asset, _ := ytDlpPlatformAsset()
	exe := filepath.Join(managed, asset.exe)
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{execFn: func(file string, args []string, spec Spec) (Result, error) {
		return Result{}, &ProcessFailedError{
			Message: "yt-dlp exited 1", Code: 1,
			Stderr: "ERROR: [youtube] x: Video unavailable",
		}
	}}
	y := &YtDlp{d: Deps{Runner: runner, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	_, err := y.Run(context.Background(), []string{"x"}, RunYtDlpOptions{})
	var yde *YtDlpError
	if !errors.As(err, &yde) || yde.Code != YtDlpUnavailable {
		t.Fatalf("err = %v", err)
	}
}

func TestIsYtDlpErrorCode(t *testing.T) {
	if !IsYtDlpErrorCode("login_required") || IsYtDlpErrorCode("weird") {
		t.Fatal("IsYtDlpErrorCode sai")
	}
}
