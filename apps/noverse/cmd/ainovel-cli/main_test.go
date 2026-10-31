package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run the real entry point in a subprocess so exit codes, streams, startup
// side effects and terminal pauses are covered without starting a server.
func TestCLIProcessHelper(t *testing.T) {
	if os.Getenv("AINOVEL_TEST_CLI_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"ainovel-cli"}, os.Args[i+1:]...)
			break
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "__test_die" {
		die("Lỗi khởi động thử nghiệm")
	}
	main()
	os.Exit(0)
}

func cliTestProcess(t *testing.T, home string, stdin *os.File, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	commandArgs := append([]string{"-test.run=^TestCLIProcessHelper$", "--"}, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "AINOVEL_TEST_CLI_PROCESS=1", "HOME="+home, "USERPROFILE="+home)
	cmd.Dir = home
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("CLI blocked with non-terminal stdin: %v; stderr=%q", ctx.Err(), stderr.String())
	}
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("start CLI: %v", err)
	}
	return exitErr.ExitCode(), stdout.String(), stderr.String()
}

func assertNoCLIErrorLog(t *testing.T, home string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(home, ".ainovel", "last-error.log")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("usage must not write a startup error log: %v", err)
	}
}

func TestCLIHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		for _, config := range []string{"unconfigured", "invalid_config"} {
			t.Run(arg+"/"+config, func(t *testing.T) {
				home := t.TempDir()
				if config == "invalid_config" {
					dir := filepath.Join(home, ".ainovel")
					if err := os.MkdirAll(dir, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("not JSON"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				code, stdout, stderr := cliTestProcess(t, home, nil, arg)
				if code != 0 || stderr != "" {
					t.Errorf("help must exit 0 without errors: code=%d stderr=%q", code, stderr)
				}
				for _, want := range []string{"Cách dùng:", "ainovel-cli serve", "ainovel-cli login", "ainovel-cli storage migrate", "ainovel-cli storage copy", "ainovel-cli storage import", "ainovel-cli storage reconcile", "ainovel-cli storage export-files", "ainovel-cli storage gc", "ainovel-cli storage purge-audit", "ainovel-cli worker tts", "ainovel-cli worker token", "ainovel-cli version"} {
					if !strings.Contains(stdout, want) {
						t.Errorf("help missing %q: %q", want, stdout)
					}
				}
				assertNoCLIErrorLog(t, home)
			})
		}
	}
}

func TestCLIUnknownFlags(t *testing.T) {
	for _, arg := range []string{"--not-a-real-flag", "-x", "--help=wrong"} {
		t.Run(arg, func(t *testing.T) {
			home := t.TempDir()
			code, stdout, stderr := cliTestProcess(t, home, nil, arg)
			if code == 0 || stdout != "" || !strings.Contains(stderr, arg) {
				t.Errorf("unknown flag needs a concise error: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if strings.Count(strings.TrimSpace(stderr), "\n") > 1 || strings.Contains(stderr, "Nhấn Enter") {
				t.Errorf("unknown flag must not start setup, log or pause: %q", stderr)
			}
			assertNoCLIErrorLog(t, home)
		})
	}
}

func TestParseCLIOptionsHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		_, args, err := parseCLIOptions([]string{arg})
		if err != nil || len(args) != 0 {
			t.Errorf("%s must be parsed as help, not a story request: args=%q err=%v", arg, args, err)
		}
	}
}

func TestParseCLIOptionsUnknownFlags(t *testing.T) {
	for _, args := range [][]string{{"--unknown"}, {"-x"}, {"--headless", "--unknown"}, {"--help", "--unknown"}, {"--unknown", "--help"}} {
		if _, _, err := parseCLIOptions(args); err == nil {
			t.Errorf("unknown flag must be rejected by shared parsing: %q", args)
		}
	}
}

func TestStdinIsTerminalRejectsNonTTY(t *testing.T) {
	original := os.Stdin
	t.Cleanup(func() { os.Stdin = original })
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	os.Stdin = null
	if stdinIsTerminal() {
		t.Error("a character device such as /dev/null is not a terminal")
	}
}

func TestCLIDieDoesNotPauseWithoutTTY(t *testing.T) {
	for _, input := range []string{"null", "open_pipe"} {
		t.Run(input, func(t *testing.T) {
			var stdin *os.File
			if input == "open_pipe" {
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				defer writer.Close() // Keep stdin open: a read would block.
				stdin = reader
			}
			code, _, stderr := cliTestProcess(t, t.TempDir(), stdin, "__test_die")
			if code != 1 || strings.Contains(stderr, "Nhấn Enter") {
				t.Errorf("fatal errors must exit without a non-TTY pause: code=%d stderr=%q", code, stderr)
			}
		})
	}
}
