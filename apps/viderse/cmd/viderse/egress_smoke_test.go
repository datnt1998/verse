package main

// Smoke của cổng ra trên process `viderse serve` thật: máy chủ hosted dựng
// proxy loopback, proxy đòi credential của process, đích nội bộ bị chặn kèm
// câu tiếng Việt cho thành viên, và một lượt tải công khai thật đi qua proxy.

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/viderse/internal/netguard"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/download"
	"github.com/datnt1998/verse/apps/viderse/internal/testidentity"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// smokeClipURL là clip công khai ngắn (~1 MB, 10 giây) cho lượt tải thật.
// Đổi bằng VIDERSE_EGRESS_SMOKE_URL khi cần.
const smokeClipURL = "https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/360/Big_Buck_Bunny_360_10s_1MB.mp4"

// startHostedServe chạy `serve` thật trên hai DB thật với PUBLIC_URL (chế độ
// hosted) và trả địa chỉ HTTP khi /healthz đã 200.
func startHostedServe(t *testing.T) string {
	t.Helper()
	clearServeEnv(t)
	bootstrap.ConfigureIdentity()
	fx := testidentity.Provision(t)
	t.Setenv("VIDERSE_DATABASE_URL", provisionSchemaOnly(t))
	t.Setenv("VERSE_IDENTITY_DATABASE_URL", fx.IdentityDSN)
	t.Setenv("PUBLIC_URL", "https://viderse.smoke.example")

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	done := make(chan error, 1)
	go func() { done <- runServe([]string{"-addr", addr}) }()
	deadline := time.Now().Add(60 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case err := <-done:
			t.Fatalf("serve thoát trước khi phục vụ: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve không lên sau 60s")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Cleanup(func() {
		// Tắt qua SIGTERM như vận hành thật; runServe chờ shutdown xong mới trả.
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(60 * time.Second):
			t.Error("serve không tắt sau 60s")
		}
	})
	return addr
}

// smokeTools dựng bộ công cụ như composition của serve (runner thật).
func smokeTools(t *testing.T) *tools.Tools {
	t.Helper()
	base := t.TempDir()
	return tools.New(tools.Deps{
		Runner:  &tools.ExecRunner{},
		BaseDir: base,
		Home:    base,
	})
}

func requireYtDlp(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("bỏ qua: không có binary yt-dlp trong môi trường test")
	}
}

// TestServeHostedEgressProxyRefusesInternalTargets: process serve thật ở chế
// độ hosted dựng proxy cổng ra; proxy chỉ nhận credential của process và
// không để yt-dlp chạm máy chủ nội bộ.
func TestServeHostedEgressProxyRefusesInternalTargets(t *testing.T) {
	requireYtDlp(t)
	startHostedServe(t)

	guard := netguard.Active()
	if !guard.Hosted() {
		t.Fatal("serve hosted phải bật cổng ra")
	}
	proxyURL, err := url.Parse(guard.ProxyURL())
	if err != nil {
		t.Fatalf("URL proxy: %v", err)
	}
	host, _, _ := net.SplitHostPort(proxyURL.Host)
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("proxy phải chỉ nghe trên loopback, được %q", host)
	}

	// Không credential → 407: tiến trình khác trên máy không dùng được proxy
	// này làm relay mở.
	anonymous := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: proxyURL.Host})}, Timeout: 10 * time.Second}
	resp, err := anonymous.Get("http://example.com/")
	if err != nil {
		t.Fatalf("request không credential: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("không credential: status = %d, muốn 407", resp.StatusCode)
	}

	// Máy chủ nội bộ của chính máy chủ này: yt-dlp thật của process serve
	// không được chạm tới.
	var hits atomic.Int64
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "internal-secret")
	}))
	defer internal.Close()

	proxiedBefore, blockedBefore := guard.Stats()
	kit := smokeTools(t)
	_, err = kit.YtDlp.Run(context.Background(), []string{"-J", "--skip-download", internal.URL + "/video.mp4"}, tools.RunYtDlpOptions{Timeout: 2 * time.Minute})
	if err == nil {
		t.Fatal("yt-dlp phải thất bại khi đích là địa chỉ nội bộ")
	}
	var yde *tools.YtDlpError
	if !errors.As(err, &yde) {
		t.Fatalf("lỗi = %v, muốn YtDlpError", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request — yt-dlp nối thẳng", got)
	}
	if _, blocked := guard.Stats(); blocked <= blockedBefore {
		t.Fatalf("cổng ra không ghi nhận lượt chặn nào (proxied trước = %d)", proxiedBefore)
	}
	// Câu thành viên thấy trong hàng chờ tải.
	mapped := download.MapDownloadError(yde, time.Now().UnixMilli())
	if mapped.Message != netguard.BlockedMessage {
		t.Fatalf("thông báo cho thành viên = %q, muốn %q", mapped.Message, netguard.BlockedMessage)
	}
	if strings.Contains(mapped.Message, "127.0.0.1") || strings.Contains(mapped.Message, "internal-secret") {
		t.Fatalf("thông báo lộ địa chỉ/nội dung nội bộ: %q", mapped.Message)
	}
}

// TestServeHostedPublicDownloadSmoke: một lượt tải công khai thật (yt-dlp của
// process serve) đi qua proxy của cổng ra. Chỉ chạy khi bật
// VIDERSE_EGRESS_SMOKE=1 — suite thường không gọi mạng ra ngoài.
func TestServeHostedPublicDownloadSmoke(t *testing.T) {
	if os.Getenv("VIDERSE_EGRESS_SMOKE") != "1" {
		t.Skip("bỏ qua lượt tải công khai thật: đặt VIDERSE_EGRESS_SMOKE=1 để chạy")
	}
	requireYtDlp(t)
	startHostedServe(t)
	guard := netguard.Active()
	clip := smokeClipURL
	if custom := strings.TrimSpace(os.Getenv("VIDERSE_EGRESS_SMOKE_URL")); custom != "" {
		clip = custom
	}
	dir := t.TempDir()
	proxiedBefore, _ := guard.Stats()
	kit := smokeTools(t)
	out, err := kit.YtDlp.Run(context.Background(), []string{
		"-f", "b", "-o", dir + "/%(title).40s.%(ext)s", "--print", "after_move:filepath", clip,
	}, tools.RunYtDlpOptions{Timeout: 5 * time.Minute})
	if err != nil {
		t.Fatalf("tải clip công khai qua cổng ra: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("không có đường dẫn tệp sau khi tải: %q", out)
	}
	info, err := os.Stat(strings.TrimSpace(out))
	if err != nil || info.Size() == 0 {
		t.Fatalf("tệp tải về: %v %v", info, err)
	}
	if proxied, _ := guard.Stats(); proxied <= proxiedBefore {
		t.Fatalf("proxy không chuyển tiếp lượt nào (trước = %d) — tải đi thẳng", proxiedBefore)
	}
	t.Logf("tải công khai qua cổng ra: %s (%d bytes)", strings.TrimSpace(out), info.Size())

	// Cùng đường, đích loopback: bị chặn kèm câu tiếng Việt.
	var hits atomic.Int64
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer internal.Close()
	_, err = kit.YtDlp.Run(context.Background(), []string{"-J", "--skip-download", internal.URL + "/video.mp4"}, tools.RunYtDlpOptions{Timeout: 2 * time.Minute})
	var yde *tools.YtDlpError
	if !errors.As(err, &yde) {
		t.Fatalf("đích loopback phải thất bại: %v", err)
	}
	if mapped := download.MapDownloadError(yde, time.Now().UnixMilli()); mapped.Message != netguard.BlockedMessage {
		t.Fatalf("thông báo = %q, muốn %q", mapped.Message, netguard.BlockedMessage)
	}
	if hits.Load() != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request", hits.Load())
	}
}
