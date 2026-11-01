// Test chuyển từ src/server/tools/uv.test.ts của yt-factory.
package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/toolchain"
)

func TestParseUvVersion(t *testing.T) {
	if v := toolchain.ParseUvVersion("uv 0.13.0 (d5b9d49f0 2025-10-08)\n"); v != "0.13.0" {
		t.Fatalf("version = %q", v)
	}
	if v := toolchain.ParseUvVersion("garbage"); v != "" {
		t.Fatalf("version = %q", v)
	}
}

func TestUvCandidatesPreferAppOwnedDir(t *testing.T) {
	base := t.TempDir()
	u := &Uv{d: Deps{Runner: &fakeRunner{}, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	// Chưa cài → resolve vắng.
	if got := u.Resolve(context.Background()); got != "" {
		t.Fatalf("resolve = %q", got)
	}
}

func TestUvStatusMissingThenReady(t *testing.T) {
	base := t.TempDir()
	runner := &fakeRunner{}
	u := &Uv{d: Deps{Runner: runner, BaseDir: base, PathEnv: t.TempDir(), SystemDirs: []string{}, Home: base}}
	st := u.Status(context.Background())
	if st.State != UvMissing {
		t.Fatalf("state = %s", st.State)
	}
	// Giả một binary trong PATH chạy được.
	sysDir := t.TempDir()
	exe := filepath.Join(sysDir, "uv")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.execFn = func(file string, args []string, spec Spec) (Result, error) {
		return Result{Stdout: "uv 0.13.0\n"}, nil
	}
	u.d.PathEnv = sysDir
	st = u.Status(context.Background())
	if st.State != UvReady || st.Source != SourceSystem || st.Version != "0.13.0" {
		t.Fatalf("status = %+v", st)
	}
}

func TestUvEnsurePythonEnvBuildsAndCaches(t *testing.T) {
	base := t.TempDir()
	req := filepath.Join(t.TempDir(), "requirements.txt")
	if err := os.WriteFile(req, []byte("numpy==1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(base, "env", "whisper")

	var ran [][]string
	runner := &fakeRunner{}
	u := &Uv{d: Deps{Runner: runner, BaseDir: base}}
	runner.execFn = func(file string, args []string, spec Spec) (Result, error) {
		ran = append(ran, args)
		if len(args) > 0 && args[0] == "venv" {
			// Giả `uv venv` dựng staging/bin/python.
			stagingDir := args[len(args)-1]
			fakePython := filepath.Join(stagingDir, "bin", "python")
			if err := os.MkdirAll(filepath.Dir(fakePython), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakePython, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			return Result{}, nil
		}
		// Giả `uv python find` trả interpreter "đã cài".
		if len(args) > 0 && args[0] == "python" && args[1] == "find" {
			fakePython := filepath.Join(u.PythonDir(), "bin", "python")
			if err := os.MkdirAll(filepath.Dir(fakePython), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fakePython, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			return Result{Stdout: fakePython + "\n"}, nil
		}
		return Result{}, nil
	}

	info, err := u.EnsurePythonEnv(context.Background(), "/fake/uv", envDir, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	// python install → venv → pip sync → find: đúng thứ tự lời gọi.
	var sawFind, sawVenv, sawSync bool
	for _, args := range ran {
		j := strings.Join(args, " ")
		if strings.Contains(j, "python find") {
			sawFind = true
		}
		if args[0] == "venv" {
			sawVenv = true
		}
		if strings.Contains(j, "pip sync") {
			sawSync = true
		}
	}
	if !sawFind || !sawVenv || !sawSync {
		t.Fatalf("ran = %v", ran)
	}
	if !toolchain.FileExists(info.Python) {
		t.Fatal("python của env không có mặt")
	}
	meta := filepath.Join(envDir, "env-meta.json")
	if !toolchain.FileExists(meta) {
		t.Fatal("env-meta.json vắng")
	}
	// Lần gọi thứ hai cùng requirements → dùng lại env, không chạy thêm lệnh.
	ran = ran[:0]
	if _, err := u.EnsurePythonEnv(context.Background(), "/fake/uv", envDir, req, nil); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 0 {
		t.Fatalf("env đã khớp vân tay mà vẫn dựng lại: %v", ran)
	}
	// Đổi requirements → vân tay lệch → dựng lại.
	if err := os.WriteFile(req, []byte("numpy==2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := u.EnsurePythonEnv(context.Background(), "/fake/uv", envDir, req, nil); err != nil {
		t.Fatal(err)
	}
	if len(ran) == 0 {
		t.Fatal("đổi requirements không dựng lại env")
	}
}

func TestUvEnsurePythonEnvPropagatesFailure(t *testing.T) {
	base := t.TempDir()
	req := filepath.Join(t.TempDir(), "requirements.txt")
	if err := os.WriteFile(req, []byte("numpy==1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(base, "env", "whisper")
	runner := &fakeRunner{execFn: func(file string, args []string, spec Spec) (Result, error) {
		return Result{}, &ProcessFailedError{Message: "network down", Code: 1}
	}}
	u := &Uv{d: Deps{Runner: runner, BaseDir: base}}
	_, err := u.EnsurePythonEnv(context.Background(), "/fake/uv", envDir, req, nil)
	var pfe *ProcessFailedError
	if !errors.As(err, &pfe) {
		t.Fatalf("err = %v", err)
	}
	if toolchain.FileExists(envDir) {
		t.Fatal("staging hỏng còn sót thành env")
	}
}
