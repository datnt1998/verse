package vieneu

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/voice"
)

// fakeVoiceServer giả lập máy đọc VieNeu: sau 0,4 giây mới mở cổng, rồi trả
// /health status=ok. Nhờ vậy một lượt bật luôn dài hơn hạn chờ ngắn trong test.
const fakeVoiceServer = `#!/bin/sh
exec python3 -c '
import http.server, os, time
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"{\"status\":\"ok\"}"
        self.send_response(200)
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def do_POST(self):
        self.send_response(200)
        self.send_header("content-length", "2")
        self.end_headers()
        self.wfile.write(b"{}")
    def log_message(self, *a):
        pass
time.sleep(0.4)
http.server.HTTPServer(("127.0.0.1", int(os.environ["PORT"])), H).serve_forever()
'
`

// installFakeEngine tạo đủ dấu hiệu "đã cài" (mã nguồn + python của venv) để
// Engine.Installed() đúng, với python là máy chủ giả ở trên.
func installFakeEngine(t *testing.T, e *Engine) {
	t.Helper()
	apps := filepath.Join(e.srcDir(), "apps")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apps, "openai_speech.py"), []byte("# giả\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	py := e.venvPython()
	if err := os.MkdirAll(filepath.Dir(py), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, []byte(fakeVoiceServer), 0o755); err != nil {
		t.Fatal(err)
	}
	if !e.Installed() {
		t.Fatal("engine giả phải được coi là đã cài")
	}
}

// TestCanceledWaitKeepsEngineBoot: yêu cầu bị huỷ (đổi trang, tải lại, trình
// duyệt cắt kết nối) chỉ được thôi *chờ*, không được giết lượt bật máy đọc đang
// nạp mô hình. Trước khi sửa, ctx của người gọi đi thẳng vào waitReady nên bị huỷ
// là tiến trình bị tắt; mỗi lần tải lại trang lại nạp mô hình từ đầu và màn Audio
// không bao giờ qua được skeleton.
func TestCanceledWaitKeepsEngineBoot(t *testing.T) {
	for _, tool := range []string{"sh", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("thiếu %s để chạy máy đọc giả", tool)
		}
	}
	e := testEngine(t)
	installFakeEngine(t, e)
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	if _, err := e.ensureStarted(ctx, voice.QualityStandard, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ensureStarted với hạn 120ms = %v, muốn DeadlineExceeded", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for {
		e.mu.Lock()
		p, starting := e.proc, e.starting
		e.mu.Unlock()
		if p != nil && p.alive() {
			break
		}
		if !starting {
			t.Fatal("lượt bật bị dừng khi người gọi huỷ yêu cầu")
		}
		if time.Now().After(deadline) {
			t.Fatal("máy đọc không lên trong 8 giây")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Người gọi sau dùng lại đúng lượt bật đó, không bật thêm tiến trình.
	first := e.proc
	c, err := e.ensureStarted(context.Background(), voice.QualityStandard, nil)
	if err != nil || c == nil {
		t.Fatalf("ensureStarted sau khi bật xong = %v", err)
	}
	if e.proc != first {
		t.Fatal("lượt bật đã xong nhưng tiến trình bị thay mới")
	}
}
