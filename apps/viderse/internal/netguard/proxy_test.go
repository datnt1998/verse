package netguard_test

// Proxy chuyển tiếp của cổng ra: chỉ nhận đúng credential của process, chỉ
// nối tới địa chỉ công khai, và chặn cả đường vòng (redirect, DNS rebinding).

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/netguard"
)

// publicFixture là đích "công khai" của test: một máy chủ thử trên loopback,
// còn resolver giả trả địa chỉ công khai cho tên trong publicHosts và loopback
// cho mọi tên khác (như /etc/hosts của một máy thật với localhost). Seam Dial
// nối địa chỉ ĐÃ KIỂM vào máy chủ thử, nên test chạy được HTTP/TLS thật mà
// không cần Internet.
type publicFixture struct {
	server *httptest.Server
	guard  *netguard.Guard
	hits   atomic.Int64
	mu     sync.Mutex
	dialed []string
	calls  int
}

var publicHosts = map[string]bool{"public.test": true, "rebind.test": true}

func newPublicFixture(t *testing.T, handler http.HandlerFunc) *publicFixture {
	t.Helper()
	f := &publicFixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// newTLSFixture như newPublicFixture nhưng đích nói TLS: dùng cho đường hầm CONNECT.
func newTLSFixture(t *testing.T, handler http.HandlerFunc) *publicFixture {
	t.Helper()
	f := &publicFixture{}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// install dựng cổng ra hosted; answers quyết định địa chỉ cho các tên công khai
// theo số thứ tự lần phân giải (để mô phỏng DNS đổi giữa hai lần nối).
func (f *publicFixture) install(t *testing.T, answers func(call int) []string) *netguard.Guard {
	t.Helper()
	t.Cleanup(netguard.Close)
	lookup := func(_ context.Context, host string) ([]net.IP, error) {
		if !publicHosts[strings.ToLower(host)] {
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}
		f.mu.Lock()
		f.calls++
		call := f.calls
		f.mu.Unlock()
		out := []net.IP{}
		for _, raw := range answers(call) {
			out = append(out, net.ParseIP(raw))
		}
		return out, nil
	}
	guard, err := netguard.Install(netguard.Options{
		Hosted: true,
		Lookup: lookup,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			f.mu.Lock()
			f.dialed = append(f.dialed, address)
			f.mu.Unlock()
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, f.server.Listener.Addr().String())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.guard = guard
	return guard
}

func (f *publicFixture) count() int64 { return f.hits.Load() }

func (f *publicFixture) dials() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.dialed...)
}

// proxyParts tách proxy của cổng ra thành địa chỉ và header credential.
func proxyParts(t *testing.T, guard *netguard.Guard) (addr, credential string) {
	t.Helper()
	u, err := url.Parse(guard.ProxyURL())
	if err != nil {
		t.Fatal(err)
	}
	password, ok := u.User.Password()
	if !ok {
		t.Fatalf("URL proxy thiếu credential: %s", guard.ProxyURL())
	}
	return u.Host, base64.StdEncoding.EncodeToString([]byte("viderse:" + password))
}

// proxyClient là client HTTP đi qua proxy của cổng ra (kèm credential).
func proxyClient(t *testing.T, raw string, tlsSkipVerify bool) *http.Client {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	if tlsSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func TestProxyRequiresProcessCredential(t *testing.T) {
	f := newPublicFixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	addr, _ := proxyParts(t, guard)

	// Không credential: proxy không được là relay mở cho tiến trình khác trên máy.
	resp, err := proxyClient(t, "http://"+addr, false).Get("http://public.test/")
	if err != nil {
		t.Fatalf("request không credential: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("không credential: status = %d, muốn 407", resp.StatusCode)
	}

	wrong := proxyClient(t, "http://viderse:khong-phai-token@"+addr, false)
	resp, err = wrong.Get("http://public.test/")
	if err != nil {
		t.Fatalf("request sai credential: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("sai credential: status = %d, muốn 407", resp.StatusCode)
	}

	// CONNECT không credential cũng bị 407.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "CONNECT public.test:443 HTTP/1.1\r\nHost: public.test:443\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || !strings.Contains(status, "407") {
		t.Fatalf("CONNECT không credential: %q %v", status, err)
	}

	// Đúng credential thì đi qua.
	resp, err = proxyClient(t, guard.ProxyURL(), false).Get("http://public.test/")
	if err != nil {
		t.Fatalf("request có credential: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "ok" || f.count() != 1 {
		t.Fatalf("status = %d body = %q hits = %d", resp.StatusCode, body, f.count())
	}
}

func TestProxyRefusesInternalTargets(t *testing.T) {
	// Máy chủ "nội bộ" đếm request: không request nào được tới nó.
	var internalHits atomic.Int64
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits.Add(1)
		_, _ = io.WriteString(w, "secret")
	}))
	defer internal.Close()
	_, internalPort, _ := net.SplitHostPort(strings.TrimPrefix(internal.URL, "http://"))

	f := newPublicFixture(t, func(w http.ResponseWriter, _ *http.Request) {})
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	client := proxyClient(t, guard.ProxyURL(), false)

	targets := []string{
		"http://127.0.0.1:" + internalPort + "/",
		"http://localhost:" + internalPort + "/",
		"http://[::1]:" + internalPort + "/",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.7:8080/",
		"http://100.64.1.1/",
		"http://2130706433:" + internalPort + "/", // 127.0.0.1 dạng số nguyên
	}
	for _, target := range targets {
		resp, err := client.Get(target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden || !netguard.IsBlockedOutput(string(body)) {
			t.Fatalf("%s: status = %d body = %q, muốn 403 có dấu cổng chặn", target, resp.StatusCode, body)
		}
		if strings.Contains(string(body), "127.0.0.1") || strings.Contains(string(body), "169.254") {
			t.Fatalf("%s: phản hồi lộ địa chỉ nội bộ: %q", target, body)
		}
	}

	// CONNECT tới loopback cũng bị chặn trước khi nối.
	addr, credential := proxyParts(t, guard)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "CONNECT 127.0.0.1:%s HTTP/1.1\r\nHost: 127.0.0.1:%s\r\nProxy-Authorization: Basic %s\r\n\r\n", internalPort, internalPort, credential)
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || !strings.Contains(status, "403") || !netguard.IsBlockedOutput(status) {
		t.Fatalf("CONNECT loopback: %q %v", status, err)
	}

	if got := internalHits.Load(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request — proxy để lọt", got)
	}
	if got := len(f.dials()); got != 0 {
		t.Fatalf("đã nối %v — đích nội bộ không được chạm tới dial", f.dials())
	}
}

func TestProxyForwardsPublicDestination(t *testing.T) {
	f := newPublicFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello "+r.URL.Path)
	})
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	resp, err := proxyClient(t, guard.ProxyURL(), false).Get("http://public.test/clip.mp4")
	if err != nil {
		t.Fatalf("đích công khai phải tải được qua proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "hello /clip.mp4" {
		t.Fatalf("status = %d body = %q", resp.StatusCode, body)
	}
	if dials := f.dials(); len(dials) != 1 || dials[0] != "93.184.216.34:80" {
		t.Fatalf("dial = %v, muốn địa chỉ ĐÃ KIỂM 93.184.216.34:80", dials)
	}
}

func TestProxyTunnelsPublicHTTPS(t *testing.T) {
	f := newTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "tls-ok") })
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	// Đường hầm CONNECT: client TLS đi xuyên proxy tới đích công khai.
	resp, err := proxyClient(t, guard.ProxyURL(), true).Get("https://public.test:8443/segment.ts")
	if err != nil {
		t.Fatalf("CONNECT tới đích công khai: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "tls-ok" {
		t.Fatalf("status = %d body = %q", resp.StatusCode, body)
	}
	if dials := f.dials(); len(dials) != 1 || dials[0] != "93.184.216.34:8443" {
		t.Fatalf("dial = %v, muốn 93.184.216.34:8443", dials)
	}
}

func TestProxyRefusesRedirectToInternal(t *testing.T) {
	var secretHits atomic.Int64
	secret := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secretHits.Add(1)
		_, _ = io.WriteString(w, "secret")
	}))
	defer secret.Close()

	// Đích công khai trả redirect về loopback: client đi tiếp qua proxy, nên
	// chặng thứ hai bị cổng chặn trước khi nối.
	f := newPublicFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, secret.URL+"/secret", http.StatusFound)
	})
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	resp, err := proxyClient(t, guard.ProxyURL(), false).Get("http://public.test/redirect")
	if err != nil {
		t.Fatalf("redirect: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || !netguard.IsBlockedOutput(string(body)) {
		t.Fatalf("chặng thứ hai phải bị cổng chặn: status = %d body = %q", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "127.0.0.1") {
		t.Fatalf("phản hồi lộ địa chỉ nội bộ: %q", body)
	}
	if got := secretHits.Load(); got != 0 {
		t.Fatalf("máy chủ nội bộ nhận %d request — redirect lọt", got)
	}
}

func TestProxyRefusesRebindingAtDial(t *testing.T) {
	// Tên phân giải công khai lần đầu, rồi loopback đúng lúc nối.
	f := newPublicFixture(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	guard := f.install(t, func(call int) []string {
		if call == 1 {
			return []string{"93.184.216.34"}
		}
		return []string{"127.0.0.1"}
	})
	client := proxyClient(t, guard.ProxyURL(), false)
	resp, err := client.Get("http://rebind.test/one")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("lượt đầu (IP công khai): %v %v", resp, err)
	}
	resp.Body.Close()
	// Cổng khác ⇒ khoá pool khác ⇒ nối mới ⇒ phân giải lại đúng lúc nối.
	// (Keep-alive tới địa chỉ ĐÃ KIỂM không phải đường vòng: nó không đổi đích.)
	resp, err = client.Get("http://rebind.test:8080/two")
	if err != nil {
		t.Fatalf("lượt sau: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || !netguard.IsBlockedOutput(string(body)) {
		t.Fatalf("lượt sau (loopback) phải bị chặn: status = %d body = %q", resp.StatusCode, body)
	}
	if got := f.count(); got != 1 {
		t.Fatalf("máy chủ nhận %d request, muốn 1", got)
	}
	if dials := f.dials(); len(dials) != 1 || dials[0] != "93.184.216.34:80" {
		t.Fatalf("dial = %v, muốn chỉ lần nối công khai", dials)
	}
}

func TestProxyClosesWithGuard(t *testing.T) {
	f := newPublicFixture(t, func(w http.ResponseWriter, _ *http.Request) {})
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	addr, _ := proxyParts(t, guard)
	netguard.Close()
	if netguard.Active().Hosted() {
		t.Fatal("sau Close, cổng ra phải về trạng thái desktop")
	}
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("proxy phải đóng cổng khi cổng ra đóng")
	}
}

func TestProxyRefusesNonHTTPAbsoluteRequest(t *testing.T) {
	f := newPublicFixture(t, func(w http.ResponseWriter, _ *http.Request) {})
	guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
	addr, credential := proxyParts(t, guard)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "GET file:///etc/passwd HTTP/1.1\r\nHost: file\r\nProxy-Authorization: Basic %s\r\n\r\n", credential)
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "400") {
		t.Fatalf("request không phải http(s) phải bị từ chối: %q", line)
	}
	if f.count() != 0 {
		t.Fatal("không request nào được tới đích")
	}
}

// TestFFmpegHonoursGuardProxy chạy ffmpeg thật qua proxy của cổng ra cho cả
// đầu vào http lẫn https: ffmpeg chỉ đọc biến http_proxy (chữ thường) cho cả
// hai, còn https cần -http_proxy kèm "httpproxy" trong -protocol_whitelist.
// Đây là đường mà ffmpeg của yt-dlp (HLS/DASH, --download-sections) đi qua.
func TestFFmpegHonoursGuardProxy(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("bỏ qua: không có ffmpeg trong môi trường test")
	}
	clip := filepath.Join(t.TempDir(), "clip.mp4")
	make := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", clip)
	if out, err := make.CombinedOutput(); err != nil {
		t.Skipf("bỏ qua: không tạo được clip thử (%v): %s", err, out)
	}
	serveClip := func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, clip) }

	t.Run("http", func(t *testing.T) {
		f := newPublicFixture(t, serveClip)
		guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
		proxiedBefore, _ := guard.Stats()
		cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "file,http,https,tcp,tls,crypto,httpproxy",
			"-http_proxy", guard.ProxyURL(), "-i", "http://public.test/clip.mp4", "-f", "null", "-")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg http qua proxy: %v: %s", err, out)
		}
		if f.count() == 0 {
			t.Fatal("đích công khai không nhận request nào")
		}
		if proxied, _ := guard.Stats(); proxied <= proxiedBefore {
			t.Fatal("proxy không chuyển tiếp lượt nào — ffmpeg nối thẳng")
		}
	})

	t.Run("https", func(t *testing.T) {
		f := newTLSFixture(t, serveClip)
		guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
		proxiedBefore, _ := guard.Stats()
		cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-tls_verify", "0", "-protocol_whitelist", "file,http,https,tcp,tls,crypto,httpproxy",
			"-http_proxy", guard.ProxyURL(), "-i", "https://public.test/clip.mp4", "-f", "null", "-")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg https qua proxy: %v: %s", err, out)
		}
		if f.count() == 0 {
			t.Fatal("đích công khai không nhận request nào")
		}
		if proxied, _ := guard.Stats(); proxied <= proxiedBefore {
			t.Fatal("proxy không chuyển tiếp lượt nào — ffmpeg nối thẳng")
		}
	})

	t.Run("env-only-https", func(t *testing.T) {
		// yt-dlp truyền proxy cho ffmpeg bằng env http_proxy (không phải cờ).
		f := newTLSFixture(t, serveClip)
		guard := f.install(t, func(int) []string { return []string{"93.184.216.34"} })
		proxiedBefore, _ := guard.Stats()
		cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-tls_verify", "0",
			"-protocol_whitelist", "file,http,https,tcp,tls,crypto,httpproxy", "-i", "https://public.test/clip.mp4", "-f", "null", "-")
		cmd.Env = append(os.Environ(), "http_proxy="+guard.ProxyURL(), "HTTP_PROXY="+guard.ProxyURL(), "no_proxy=")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg https qua proxy bằng env: %v: %s", err, out)
		}
		if proxied, _ := guard.Stats(); proxied <= proxiedBefore {
			t.Fatal("env http_proxy không được ffmpeg dùng cho https — ffmpeg của yt-dlp sẽ nối thẳng")
		}
	})
}
