package netguard_test

// Quy tắc địa chỉ và môi trường của cổng ra: một bản duy nhất dùng chung cho
// client Go, proxy của tiến trình con và biến môi trường của chúng.

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/netguard"
)

func TestBlockedRefusesInternalDestinations(t *testing.T) {
	// Mọi đích mà máy chủ chia sẻ không được gọi hộ thành viên: loopback,
	// mạng riêng, link-local (metadata cloud), CGNAT, dải dành riêng,
	// multicast/broadcast, chưa xác định, và dạng IPv4-mapped/NAT64/6to4 của
	// chúng (đích thật vẫn là địa chỉ nội bộ).
	internal := []string{
		"127.0.0.1", "127.255.255.254", "::1", "0.0.0.0", "0.1.2.3", "::",
		"10.0.0.7", "10.255.255.255", "172.16.0.1", "172.31.255.255", "192.168.1.20",
		"169.254.169.254", "169.254.1.1", "fe80::1", "fc00::1", "fd12:3456::1",
		"100.64.0.1", "100.127.255.254", "198.18.0.1", "198.19.255.255",
		"192.0.0.1",
		"224.0.0.1", "239.255.255.250", "255.255.255.255", "240.0.0.1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.7", "::ffff:169.254.169.254",
		"64:ff9b::7f00:1", "64:ff9b::a00:7", "64:ff9b:1::a9fe:a9fe",
		"2002:7f00:1::", "2002:a00:7::",
	}
	for _, raw := range internal {
		addr := netip.MustParseAddr(raw)
		if !netguard.Blocked(addr) {
			t.Fatalf("%s phải bị chặn", raw)
		}
	}
}

func TestBlockedAllowsPublicDestinations(t *testing.T) {
	// 192.0.2.0/24, 198.51.100.0/24 và 203.0.113.0/24 là TEST-NET (chỉ để
	// viết tài liệu): không phải đích nội bộ nên quy tắc không chặn.
	for _, raw := range []string{"93.184.216.34", "1.1.1.1", "8.8.8.8", "2606:2800:220:1::1", "2a00:1450:4001::1", "192.0.2.1", "198.51.100.7", "203.0.113.9"} {
		addr := netip.MustParseAddr(raw)
		if netguard.Blocked(addr) {
			t.Fatalf("%s là địa chỉ công khai, không được chặn", raw)
		}
	}
}

// staticLookup trả cùng một danh sách địa chỉ cho mọi tên, đếm số lần gọi.
type staticLookup struct {
	mu      sync.Mutex
	calls   int
	answers func(call int) []string
}

func (l *staticLookup) lookup(_ context.Context, _ string) ([]net.IP, error) {
	l.mu.Lock()
	l.calls++
	call := l.calls
	l.mu.Unlock()
	out := []net.IP{}
	for _, raw := range l.answers(call) {
		out = append(out, net.ParseIP(raw))
	}
	return out, nil
}

func (l *staticLookup) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func TestPublicAddrsRefusesAnyInternalAnswer(t *testing.T) {
	lookup := &staticLookup{answers: func(int) []string { return []string{"93.184.216.34", "127.0.0.1"} }}
	if _, err := netguard.PublicAddrs(context.Background(), lookup.lookup, "mixed.example"); !errors.Is(err, netguard.ErrBlocked) {
		t.Fatalf("tên có một bản ghi nội bộ phải bị chặn, err = %v", err)
	}
	if _, err := netguard.PublicAddrs(context.Background(), nil, "127.0.0.1"); !errors.Is(err, netguard.ErrBlocked) {
		t.Fatalf("IP literal nội bộ phải bị chặn, err = %v", err)
	}
	if _, err := netguard.PublicAddrs(context.Background(), nil, "fe80::1%en0"); !errors.Is(err, netguard.ErrBlocked) {
		t.Fatalf("địa chỉ có zone phải bị chặn, err = %v", err)
	}
	empty := &staticLookup{answers: func(int) []string { return nil }}
	if _, err := netguard.PublicAddrs(context.Background(), empty.lookup, "nothing.example"); !errors.Is(err, netguard.ErrUnresolvable) {
		t.Fatalf("không phân giải được phải là ErrUnresolvable, err = %v", err)
	}
	addrs, err := netguard.PublicAddrs(context.Background(), lookup.lookup, "93.184.216.34")
	if err != nil || len(addrs) != 1 || addrs[0].String() != "93.184.216.34" {
		t.Fatalf("IP công khai: %v %v", addrs, err)
	}
}

func TestDialerDialsCheckedLiteralOnce(t *testing.T) {
	lookup := &staticLookup{answers: func(int) []string { return []string{"93.184.216.34", "2606:2800:220:1::1"} }}
	var dialed []string
	dialer := &netguard.Dialer{Lookup: lookup.lookup, Dial: func(_ context.Context, network, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		return nil, errors.New("không nối được")
	}}
	_, err := dialer.DialContext(context.Background(), "tcp", "cdn.example:8443")
	if err == nil {
		t.Fatal("dial phải trả lỗi")
	}
	if lookup.count() != 1 {
		t.Fatalf("phân giải %d lần, muốn đúng 1 (nối bằng IP literal)", lookup.count())
	}
	want := []string{"93.184.216.34:8443", "[2606:2800:220:1::1]:8443"}
	if len(dialed) != 2 || dialed[0] != want[0] || dialed[1] != want[1] {
		t.Fatalf("dial = %v, muốn %v", dialed, want)
	}
}

func TestDialerRefusesRebindingAtDial(t *testing.T) {
	// DNS trả IP công khai lúc đầu rồi 127.0.0.1 đúng lúc nối.
	lookup := &staticLookup{answers: func(call int) []string {
		if call == 1 {
			return []string{"93.184.216.34"}
		}
		return []string{"127.0.0.1"}
	}}
	connections := 0
	dialer := &netguard.Dialer{Lookup: lookup.lookup, Dial: func(_ context.Context, _, _ string) (net.Conn, error) {
		connections++
		client, server := net.Pipe()
		server.Close()
		return client, nil
	}}
	if _, err := dialer.DialContext(context.Background(), "tcp", "rebind.example:443"); err != nil {
		t.Fatalf("lần đầu là IP công khai: %v", err)
	}
	if _, err := dialer.DialContext(context.Background(), "tcp", "rebind.example:443"); !errors.Is(err, netguard.ErrBlocked) {
		t.Fatalf("lần hai trỏ loopback phải bị chặn, err = %v", err)
	}
	if connections != 1 {
		t.Fatalf("đã nối %d lần, muốn 1 (lần nội bộ không được nối)", connections)
	}
}

func TestDesktopGuardIsInert(t *testing.T) {
	t.Cleanup(netguard.Close)
	if _, err := netguard.Install(netguard.Options{}); err != nil {
		t.Fatal(err)
	}
	guard := netguard.Active()
	if guard.Hosted() || guard.ProxyURL() != "" || guard.YtDlpArgs() != nil || guard.FFmpegProxyArgs() != nil || guard.Env() != nil {
		t.Fatalf("chế độ desktop phải không đổi gì: %+v", guard)
	}
	// Đường của app ở desktop không đi qua cổng ra: client Go vẫn nối loopback
	// như cũ (Ollama, máy chủ nội bộ của chủ máy) — test riêng ở aiconn.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(srv.URL)
	if err != nil {
		t.Fatalf("desktop phải nối được loopback: %v", err)
	}
	resp.Body.Close()
}

func TestGuardEnvReplacesInheritedProxyVariables(t *testing.T) {
	// Môi trường service có NO_PROXY/ALL_PROXY riêng: nếu tiến trình con thừa
	// kế, nó nối thẳng tới đích và cổng ra bị vòng qua.
	t.Setenv("NO_PROXY", "127.0.0.1,localhost,.internal")
	t.Setenv("No_Proxy", "*")
	t.Setenv("ALL_PROXY", "socks5://evil.invalid:1080")
	t.Setenv("http_proxy", "http://inherited.invalid:3128")
	t.Setenv("HTTPS_PROXY", "http://inherited.invalid:3128")
	t.Cleanup(netguard.Close)
	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	if !guard.Hosted() || guard.ProxyURL() == "" {
		t.Fatal("hosted phải có proxy")
	}
	env := guard.Env()
	if env["http_proxy"] != guard.ProxyURL() || env["HTTPS_PROXY"] != guard.ProxyURL() || env["ALL_PROXY"] != guard.ProxyURL() {
		t.Fatalf("proxy trong env con = %q/%q/%q, muốn %q", env["http_proxy"], env["HTTPS_PROXY"], env["ALL_PROXY"], guard.ProxyURL())
	}
	if env["no_proxy"] != "" || env["NO_PROXY"] != "" {
		t.Fatalf("no_proxy phải rỗng, được %q/%q", env["no_proxy"], env["NO_PROXY"])
	}
	for key := range env {
		if strings.EqualFold(key, "no_proxy") || strings.EqualFold(key, "all_proxy") {
			if strings.Contains(env[key], "evil.invalid") || env[key] == "*" {
				t.Fatalf("biến %q thừa kế chưa bị thay: %q", key, env[key])
			}
		}
	}
	if env["PATH"] == "" {
		t.Fatal("env con phải giữ PATH của app")
	}
	if !strings.HasPrefix(guard.ProxyURL(), "http://viderse:") || !strings.Contains(guard.ProxyURL(), "@127.0.0.1:") {
		t.Fatalf("URL proxy phải có credential và chỉ trên loopback: %s", guard.ProxyURL())
	}
	if args := guard.YtDlpArgs(); len(args) != 2 || args[0] != "--proxy" || args[1] != guard.ProxyURL() {
		t.Fatalf("yt-dlp args = %v", args)
	}
	if args := guard.FFmpegProxyArgs(); len(args) != 2 || args[0] != "-http_proxy" || args[1] != guard.ProxyURL() {
		t.Fatalf("ffmpeg args = %v", args)
	}
}

func TestGuardRedactsProxyCredential(t *testing.T) {
	t.Cleanup(netguard.Close)
	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	line := "ERROR: unable to download: proxy " + guard.ProxyURL() + " refused"
	if redacted := guard.Redact(line); strings.Contains(redacted, guard.ProxyURL()) || strings.Contains(redacted, "@127.0.0.1") {
		t.Fatalf("credential proxy còn trong lỗi: %s", redacted)
	}
	if netguard.Active().Redact("không có proxy") != "không có proxy" {
		t.Fatal("chuỗi không có proxy phải giữ nguyên")
	}
}

func TestAllowedYtDlpProtocolMatchesFilter(t *testing.T) {
	// Danh sách cho phép và bộ lọc format phải nói cùng một chuyện: protocol
	// nối trực tiếp (rtmp, rtsp, mms, f4m, ism, websocket_frag…) không được
	// lọt vào lựa chọn format của yt-dlp.
	allowed := []string{"http", "https", "m3u8", "m3u8_native", "http_dash_segments", "http_dash_segments_generator"}
	filter := netguard.YtDlpFormatFilter
	pattern := regexp.MustCompile(strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(filter, `"`, 3)[1], `"`), `"`))
	for _, protocol := range allowed {
		if !netguard.AllowedYtDlpProtocol(protocol) {
			t.Fatalf("%s phải được phép", protocol)
		}
		if !pattern.MatchString(protocol) {
			t.Fatalf("bộ lọc không khớp %s: %s", protocol, filter)
		}
	}
	for _, protocol := range []string{"rtmp", "rtmpe", "rtmp_ffmpeg", "rtsp", "mms", "f4m", "ism", "mhtml", "websocket_frag", "niconico_live", "fc2_live", "youtube_live_chat", "bunnycdn", "soopvod", ""} {
		if netguard.AllowedYtDlpProtocol(protocol) {
			t.Fatalf("%q không được phép", protocol)
		}
		if protocol != "" && pattern.MatchString(protocol) {
			t.Fatalf("bộ lọc khớp cả %q — sẽ lọt protocol nối trực tiếp: %s", protocol, filter)
		}
	}
}

func TestHardenYtDlpArgsAddsProtocolFilter(t *testing.T) {
	t.Cleanup(netguard.Close)
	desktop, err := netguard.Install(netguard.Options{})
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-f", "bv*+ba/b", "https://cdn.example/v"}
	if got := desktop.HardenYtDlpArgs(args); strings.Join(got, " ") != strings.Join(args, " ") {
		t.Fatalf("desktop phải giữ nguyên args: %v", got)
	}

	guard, err := netguard.Install(netguard.Options{Hosted: true})
	if err != nil {
		t.Fatal(err)
	}
	filter := netguard.YtDlpFormatFilter
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "selector nhiều lựa chọn",
			in:   []string{"-f", "bv*+ba/b", "-S", "res:720", "https://cdn.example/v"},
			want: []string{"-f", "bv*" + filter + "+ba" + filter + "/b" + filter, "-S", "res:720", "https://cdn.example/v"},
		},
		{
			name: "nhóm ngoặc",
			in:   []string{"--format", "(bv+ba)/b"},
			want: []string{"--format", "(bv+ba)" + filter + "/b" + filter},
		},
		{
			name: "dạng --format=",
			in:   []string{"--format=bv*"},
			want: []string{"--format=bv*" + filter},
		},
		{
			name: "đã có bộ lọc protocol thì giữ nguyên",
			in:   []string{"-f", "bv*[protocol=https]"},
			want: []string{"-f", "bv*[protocol=https]"},
		},
		{
			name: "lời gọi tải không có -f thì thêm format mặc định đã lọc",
			in:   []string{"--no-playlist", "https://cdn.example/v"},
			want: []string{"--no-playlist", "https://cdn.example/v", "-f", "bv*" + filter + "+ba" + filter + "/b" + filter},
		},
		{
			name: "lời gọi chỉ lấy thông tin thì không thêm -f",
			in:   []string{"-J", "--skip-download", "--no-playlist", "https://cdn.example/v"},
			want: []string{"-J", "--skip-download", "--no-playlist", "https://cdn.example/v"},
		},
		{
			name: "lời gọi phiên bản/cập nhật không thêm -f",
			in:   []string{"--version"},
			want: []string{"--version"},
		},
	}
	for _, tc := range cases {
		got := guard.HardenYtDlpArgs(append([]string(nil), tc.in...))
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Fatalf("%s: %v, muốn %v", tc.name, got, tc.want)
		}
	}
	// Lời gọi tải có --download-sections vẫn phải mang bộ lọc.
	withSections := guard.HardenYtDlpArgs([]string{"-f", "bv*+ba/b", "--download-sections", "*10-40"})
	if !strings.Contains(withSections[1], filter) {
		t.Fatalf("--download-sections: %v", withSections)
	}
}
