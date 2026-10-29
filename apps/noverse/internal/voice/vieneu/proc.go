package vieneu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/voice"
	"github.com/datnt1998/verse/apps/noverse/internal/voice/openaispeech"
)

// proc là một tiến trình máy chủ VieNeu đang chạy.
type proc struct {
	cmd       *exec.Cmd
	client    *openaispeech.Client
	port      int
	token     string
	precision string
	logFile   *os.File
	done      chan struct{} // đóng khi tiến trình thoát

	exitMu  sync.Mutex
	exited  bool
	exitErr error

	stopOnce sync.Once
}

// alive cho biết tiến trình còn sống.
func (p *proc) alive() bool {
	p.exitMu.Lock()
	defer p.exitMu.Unlock()
	return !p.exited
}

// stop tắt tiến trình: tín hiệu ngắt trước, SIGKILL nếu quá 5 giây.
func (p *proc) stop() {
	p.stopOnce.Do(func() {
		if p.cmd != nil && p.cmd.Process != nil && p.alive() {
			if runtime.GOOS == "windows" {
				_ = p.cmd.Process.Kill()
				<-p.done
			} else {
				_ = p.cmd.Process.Signal(os.Interrupt)
				select {
				case <-p.done:
				case <-time.After(5 * time.Second):
					_ = p.cmd.Process.Kill()
					<-p.done
				}
			}
		}
		if p.logFile != nil {
			_ = p.logFile.Close()
		}
	})
}

// start dựng và bật máy chủ, chờ /health sẵn sàng. onDownload được gọi định kỳ với
// dung lượng thư mục mô hình (để hiện tiến độ tải ở lần chạy đầu).
func (e *Engine) start(ctx context.Context, precision string, onDownload func(int64)) (*proc, error) {
	if !e.Installed() {
		return nil, fmt.Errorf("%w: chưa có bộ đọc VieNeu trong %s", voice.ErrNotInstalled, e.srcDir())
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	logFile, err := e.openLog()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(e.venvPython(), "-m", "apps.openai_speech")
	cmd.Dir = e.srcDir()
	cmd.Env = append(os.Environ(), e.serverEnv(port, token, precision)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("không bật được máy đọc: %w", err)
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	p := &proc{
		cmd:       cmd,
		client:    openaispeech.NewClient(openaispeech.ClientOptions{Endpoint: base, APIKey: token, Model: e.cfg.Model, IgnoreSpeed: true}),
		port:      port,
		token:     token,
		precision: precision,
		logFile:   logFile,
		done:      make(chan struct{}),
	}
	go func() {
		waitErr := cmd.Wait()
		p.exitMu.Lock()
		p.exited, p.exitErr = true, waitErr
		p.exitMu.Unlock()
		close(p.done)
	}()

	e.cfg.Logf("vieneu: bật máy đọc ở %s (precision=%s, device=%s)", base, precision, e.cfg.Device)
	if err := e.waitReady(ctx, p, onDownload); err != nil {
		p.stop()
		return nil, err
	}
	return p, nil
}

// waitReady chờ /health trả "ok"; lần đầu có thể phải tải ~600 MB mô hình.
func (e *Engine) waitReady(ctx context.Context, p *proc, onDownload func(int64)) error {
	deadline := time.Now().Add(readyTimeout)
	lastReport := time.Time{}
	var lastErr error
	for {
		if !p.alive() {
			return fmt.Errorf("máy đọc dừng ngay khi khởi động: %s", e.logTail(6))
		}
		checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		h, err := p.client.Health(checkCtx)
		cancel()
		if err == nil && h.OK() {
			e.cfg.Logf("vieneu: máy đọc sẵn sàng sau %.1f giây", time.Since(deadline.Add(-readyTimeout)).Seconds())
			return nil
		}
		if err != nil {
			lastErr = err
		}
		if onDownload != nil && time.Since(lastReport) > 2*time.Second {
			onDownload(dirSize(e.hfHome()))
			lastReport = time.Now()
		}
		if time.Now().After(deadline) {
			msg := e.logTail(6)
			if lastErr != nil {
				return fmt.Errorf("máy đọc không sẵn sàng sau %s: %v · %s", readyTimeout, lastErr, msg)
			}
			return fmt.Errorf("máy đọc không sẵn sàng sau %s · %s", readyTimeout, msg)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return fmt.Errorf("máy đọc dừng khi đang khởi động: %s", e.logTail(6))
		case <-time.After(time.Second):
		}
	}
}

// serverEnv là biến môi trường cho apps/openai_speech.py.
func (e *Engine) serverEnv(port int, token, precision string) []string {
	backend, device := "onnx", "cpu"
	if e.cfg.Device == "cuda" {
		backend, device = "auto", "cuda"
	}
	return []string{
		"HOST=127.0.0.1",
		fmt.Sprintf("PORT=%d", port),
		"VIENEU_API_KEY=" + token,
		"VIENEU_BACKEND=" + backend,
		"VIENEU_DEVICE=" + device,
		"VIENEU_PRECISION=" + precision,
		"HF_HOME=" + e.hfHome(),
		"PYTHONUNBUFFERED=1",
		// Không dùng chung cache HF với ứng dụng khác: mô hình 600 MB thuộc về bộ đọc.
		"HUGGINGFACE_HUB_CACHE=" + filepath.Join(e.hfHome(), "hub"),
	}
}

func (e *Engine) openLog() (*os.File, error) {
	path := e.logPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// Giữ log gọn: mỗi lần bật chỉ cần chẩn đoán gần nhất.
	if st, err := os.Stat(path); err == nil && st.Size() > 2<<20 {
		_ = os.Remove(path)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// logTail trả vài dòng cuối log máy chủ để báo lỗi cho người dùng.
func (e *Engine) logTail(lines int) string {
	f, err := os.Open(e.logPath())
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	const window = 8 << 10
	start := st.Size() - window
	if start < 0 {
		start = 0
	}
	buf := make([]byte, st.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return ""
	}
	all := strings.Split(strings.TrimSpace(string(buf)), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	out := strings.Join(all, " · ")
	if len(out) > 600 {
		out = out[len(out)-600:]
	}
	return out
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("không tìm được cổng trống: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// healthOnce kiểm tra nhanh máy chủ còn phản hồi (dùng cho Status).
func healthOnce(ctx context.Context, c *openaispeech.Client) (openaispeech.Health, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.Health(ctx)
}
