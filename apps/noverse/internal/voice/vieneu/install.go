package vieneu

import (
	"context"
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

// Install là đường cài của desktop/CLI và nút "Cài bộ đọc trên máy": cùng một
// pipeline với Provision (uv ghim → Python → mã nguồn → thư viện → mô hình),
// chỉ dịch ProvisionState sang InstallProgress mà Service/CLI đang dùng.
func (e *Engine) Install(ctx context.Context, report func(voice.InstallProgress)) error {
	if report == nil {
		report = func(voice.InstallProgress) {}
	}
	err := e.Provision(ctx, func(st voice.ProvisionState) {
		report(voice.InstallProgress{
			State:      "running",
			Step:       st.Step,
			Percent:    installPercent(st.Percent),
			BytesDone:  st.BytesDone,
			BytesTotal: st.BytesTotal,
			EtaSec:     st.EtaSec,
			Version:    st.Version,
			Message:    st.Message,
			Updated:    st.UpdatedAt,
		})
	})
	if err != nil {
		return err
	}
	report(voice.InstallProgress{State: "done", Step: "done", Percent: new(100), Message: "Đã cài xong bộ đọc trên máy"})
	return nil
}

// installPercent đổi sentinel "không đo được" của ProvisionState thành nil: UI
// hiện vòng xoay thay vì một con số bịa.
func installPercent(v int) *int {
	if v < 0 {
		return nil
	}
	return &v
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
	return runStreamingEnv(ctx, dir, name, args, logPath, nil, onLine)
}

// runStreamingEnv như runStreaming nhưng đặt thêm biến môi trường (env nil =
// thừa hưởng môi trường process).
func runStreamingEnv(ctx context.Context, dir, name string, args []string, logPath string, env []string, onLine func(string)) error {
	f, err := openAppend(logPath)
	if err != nil {
		return err
	}
	defer f.Close()
	sink := &lineSink{file: f, onLine: onLine}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
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
