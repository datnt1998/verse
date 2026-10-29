package vieneu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// modelBytes là dung lượng mô hình ước tính để quy ra phần trăm tải (đo thật: ~581 MB).
const modelBytes = 600 << 20

// Install cài bộ đọc vào <home>/vieneu: mã nguồn đúng tag đã ghim, môi trường uv,
// rồi nạp thử để tải sẵn mô hình và xác nhận chạy được.
func (e *Engine) Install(ctx context.Context, report func(voice.InstallProgress)) error {
	if report == nil {
		report = func(voice.InstallProgress) {}
	}
	if e.Installed() {
		report(voice.InstallProgress{Step: "done", Percent: 100, Message: "Bộ đọc trên máy đã có sẵn"})
		return nil
	}
	uv, err := findUV()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("cần git để cài bộ đọc (macOS: chạy `xcode-select --install`)")
	}
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		return err
	}

	report(voice.InstallProgress{Step: "source", Percent: 5, Message: "Đang tải mã nguồn VieNeu-TTS…"})
	if err := e.syncSource(ctx); err != nil {
		return err
	}

	report(voice.InstallProgress{Step: "deps", Percent: 25, Message: "Đang cài thư viện Python (lần đầu vài phút)…"})
	err = runStreaming(ctx, e.srcDir(), uv, []string{"sync", "--frozen", "--python", "3.12"}, e.logPath(),
		func(line string) {
			report(voice.InstallProgress{Step: "deps", Percent: 40, Message: "Đang cài thư viện: " + clip(line, 90)})
		})
	if err != nil {
		return fmt.Errorf("cài thư viện Python thất bại: %w · %s", err, e.logTail(4))
	}

	report(voice.InstallProgress{Step: "model", Percent: 65, Message: "Đang tải mô hình giọng đọc (~600 MB)…"})
	_, err = e.ensureStarted(ctx, e.cfg.Quality, func(bytes int64) {
		pct := 65 + int(min(bytes*30/modelBytes, 30))
		report(voice.InstallProgress{
			Step:    "model",
			Percent: pct,
			Message: "Đang tải mô hình giọng đọc…",
			Bytes:   bytes,
		})
	})
	if err != nil {
		return err
	}

	report(voice.InstallProgress{Step: "done", Percent: 100, Message: "Đã cài xong bộ đọc trên máy"})
	return nil
}

// syncSource bảo đảm <home>/vieneu/src đúng tag đã ghim.
func (e *Engine) syncSource(ctx context.Context) error {
	src := e.srcDir()
	if _, err := os.Stat(filepath.Join(src, ".git")); err == nil {
		if err := runQuiet(ctx, src, e.logPath(), "git", "fetch", "--depth", "1", "origin", "refs/tags/"+pinnedTag); err == nil {
			if err := runQuiet(ctx, src, e.logPath(), "git", "checkout", "-f", "FETCH_HEAD"); err == nil {
				return nil
			}
		}
		if err := os.RemoveAll(src); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		return err
	}
	err := runQuiet(ctx, filepath.Dir(src), e.logPath(), "git",
		"clone", "--depth", "1", "--branch", pinnedTag, repoURL, src)
	if err != nil {
		return fmt.Errorf("tải mã nguồn VieNeu-TTS thất bại: %w", err)
	}
	return nil
}

// findUV tìm uv theo PATH rồi các vị trí cài phổ biến.
func findUV() (string, error) {
	if p, err := exec.LookPath("uv"); err == nil {
		return p, nil
	}
	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", "uv"),
			filepath.Join(home, ".cargo", "bin", "uv"),
		)
	}
	candidates = append(candidates, "/opt/homebrew/bin/uv", "/usr/local/bin/uv", "/usr/bin/uv")
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("chưa có uv trên máy — cài bằng `brew install uv` (hoặc xem https://docs.astral.sh/uv/) rồi thử lại")
}

// runQuiet chạy lệnh, ghi toàn bộ output vào log của bộ đọc.
func runQuiet(ctx context.Context, dir, logPath, name string, args ...string) error {
	f, err := openAppend(logPath)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = f
	cmd.Stderr = f
	return cmd.Run()
}

// runStreaming chạy lệnh và đẩy từng dòng output ra onLine (để CLI/SSE thấy tiến độ thật).
func runStreaming(ctx context.Context, dir, name string, args []string, logPath string, onLine func(string)) error {
	f, err := openAppend(logPath)
	if err != nil {
		return err
	}
	defer f.Close()
	sink := &lineSink{file: f, onLine: onLine}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = sink
	cmd.Stderr = sink
	return cmd.Run()
}

func openAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// lineSink ghi log và cắt dòng cho callback (stdout/stderr của lệnh có thể gọi song song).
type lineSink struct {
	mu     sync.Mutex
	file   *os.File
	onLine func(string)
	buf    []byte
}

func (s *lineSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		_, _ = s.file.Write(p)
	}
	s.buf = append(s.buf, p...)
	for {
		i := strings.IndexByte(string(s.buf), '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(s.buf[:i]))
		s.buf = s.buf[i+1:]
		if line != "" && s.onLine != nil {
			s.onLine(line)
		}
	}
	if len(s.buf) > 4096 {
		s.buf = nil
	}
	return len(p), nil
}

var _ io.Writer = (*lineSink)(nil)

// clip rút gọn chuỗi theo rune để log/tiến độ không bị dài dòng.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
