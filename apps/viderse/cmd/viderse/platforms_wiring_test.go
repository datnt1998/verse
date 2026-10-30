package main

// Regression cho review L2: probe hồ sơ YouTube chạy trên toolkit dùng chung
// được truyền vào (cùng runner/registry/quyền cài đặt của serve), không phải
// một tools.New riêng — và lỗi "video mẫu không xem được" không làm hỏng
// kiểm chứng cookie.

import (
	"context"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// fakeRunner ghi lại tham số Run và trả kết quả gài sẵn.
type fakeRunner struct {
	installed bool
	args      []string
	calls     int
	result    string
	err       error
}

func (f *fakeRunner) IsInstalled() bool { return f.installed }

func (f *fakeRunner) Run(_ context.Context, args []string, _ tools.RunYtDlpOptions) (string, error) {
	f.calls++
	f.args = append([]string(nil), args...)
	return f.result, f.err
}

func TestProbeUsesSuppliedRunner(t *testing.T) {
	runner := &fakeRunner{installed: true}
	probe := ytDlpProbe{tool: runner}
	if err := probe.Check(context.Background(), "chrome", "Default"); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("Run = %d lần", runner.calls)
	}
	want := []string{"--simulate", "--skip-download", "--no-playlist", "--no-warnings",
		"--cookies-from-browser", "chrome:Default", probeURL}
	if len(runner.args) != len(want) {
		t.Fatalf("args = %v", runner.args)
	}
	for i, a := range runner.args {
		if a != want[i] {
			t.Fatalf("args[%d] = %q, muốn %q", i, a, want[i])
		}
	}
}

func TestProbeMapsOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		runner   *fakeRunner
		wantCode string // "" = pass
	}{
		{"ok", &fakeRunner{installed: true}, ""},
		{"unavailable-passes", &fakeRunner{installed: true, err: &tools.YtDlpError{Code: tools.YtDlpUnavailable, StderrTail: "x"}}, ""},
		{"cookies_failed-rejects", &fakeRunner{installed: true, err: &tools.YtDlpError{Code: tools.YtDlpCookiesFailed, StderrTail: "x"}}, "cookies_failed"},
		{"generic-error", &fakeRunner{installed: true, err: errors.New("boom")}, "fail"},
		{"not-installed", &fakeRunner{installed: false}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := ytDlpProbe{tool: tc.runner}
			err := probe.Check(context.Background(), "brave", "Profile 2")
			switch tc.wantCode {
			case "":
				if tc.name == "not-installed" {
					if err == nil {
						t.Fatal("yt-dlp thiếu phải lỗi")
					}
				} else if err != nil {
					t.Fatalf("err = %v", err)
				}
			case "fail":
				if err == nil {
					t.Fatal("lỗi phải lọt ra ngoài")
				}
			case "cookies_failed":
				var coded interface{ ErrorCode() string }
				if !errors.As(err, &coded) || coded.ErrorCode() != "cookies_failed" {
					t.Fatalf("err = %v", err)
				}
			}
		})
	}
}
