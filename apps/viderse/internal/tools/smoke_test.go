// Smoke test thật (miễn phí, không tải gì): phát hiện ffmpeg, ffprobe,
// yt-dlp, uv, codex cài trên máy và in phiên bản qua chính đường probe.
// TẮT mặc định — chỉ chạy khi VIDEROSE_TOOLS_SMOKE=1:
//
//	VIDEROSE_TOOLS_SMOKE=1 go test ./apps/viderse/internal/tools/ -run TestSmoke -v
package tools

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSmokeLocalTools(t *testing.T) {
	if os.Getenv("VIDEROSE_TOOLS_SMOKE") != "1" {
		t.Skip("đặt VIDEROSE_TOOLS_SMOKE=1 để probe binary thật")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := &ExecRunner{}
	d := Deps{Runner: r, BaseDir: t.TempDir()}

	ff := &Ffmpeg{d: d}
	if exe := FindOnPath("ffmpeg", d.pathEnv()); exe != "" {
		if p := ff.Probe(ctx, exe); p != nil {
			t.Logf("ffmpeg @ %s → %s (arch %s), missing=%v", exe, p.Version, p.Arch, p.Missing)
		} else {
			t.Logf("ffmpeg @ %s không probe được", exe)
		}
	} else {
		t.Log("ffmpeg không có trên PATH")
	}
	if exe := FindOnPath("ffprobe", d.pathEnv()); exe != "" {
		res, err := r.Run(ctx, exe, []string{"-version"}, Spec{Timeout: 15 * time.Second})
		if err == nil {
			t.Logf("ffprobe @ %s → %s", exe, firstLine(res.Stdout))
		}
	} else {
		t.Log("ffprobe không có trên PATH")
	}

	y := &YtDlp{d: d}
	if exe := FindOnPath("yt-dlp", d.pathEnv()); exe != "" {
		if v, err := y.readVersion(ctx, exe); err == nil {
			t.Logf("yt-dlp @ %s → %s", exe, v)
		} else {
			t.Logf("yt-dlp @ %s lỗi: %v", exe, err)
		}
	} else {
		t.Log("yt-dlp không có trên PATH")
	}

	u := &Uv{d: d}
	if exe := FindOnPath("uv", d.pathEnv()); exe != "" {
		if p := u.Probe(ctx, exe); p != nil {
			t.Logf("uv @ %s → %s", exe, p.Version)
		}
	} else {
		t.Log("uv không có trên PATH")
	}

	c := &Codex{d: d}
	if exe := FindOnPath("codex", d.pathEnv()); exe != "" {
		if info, bad := c.probe(ctx, exe, SourceSystem); info != nil {
			t.Logf("codex @ %s → %s", exe, info.Version)
		} else {
			t.Logf("codex @ %s hỏng: %s", exe, bad.Error)
		}
	} else {
		t.Log("codex không có trên PATH")
	}
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
