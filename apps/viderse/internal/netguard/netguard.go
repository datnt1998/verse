// Gói netguard là một quy tắc duy nhất cho "đích nào được phép gọi hộ thành
// viên": máy chủ Viderse chạy công khai (PUBLIC_URL) nên không được để link
// của thành viên chạm tới loopback, mạng riêng, metadata cloud hay hạ tầng nội
// bộ của máy chủ chia sẻ.
//
// Cùng một quy tắc được áp ở ba chỗ, không có bản sao thứ hai:
//   - client HTTP phía Go (aiconn, tải danh sách phát HLS của editor) — kiểm
//     địa chỉ lúc nối, nối thẳng bằng IP đã kiểm;
//   - proxy chuyển tiếp trong process (chỉ ở chế độ hosted) mà mọi tiến trình
//     con do thành viên gián tiếp điều khiển (yt-dlp, ffmpeg) phải đi qua;
//   - env của tiến trình con, để biến môi trường thừa kế (NO_PROXY, ALL_PROXY)
//     không mở đường vòng.
package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"
)

// ErrBlocked: đích là địa chỉ nội bộ — máy chủ chia sẻ không gọi hộ.
// ErrUnresolvable: không xác minh được đích nên cũng không gọi.
var (
	ErrBlocked      = errors.New("địa chỉ nội bộ")
	ErrUnresolvable = errors.New("không phân giải được địa chỉ")
)

// BlockedMessage là câu trả cho thành viên khi đích bị chặn. Không bao giờ
// kèm địa chỉ thật: chính địa chỉ đó là thứ không được lộ ra ngoài.
const BlockedMessage = "Link trỏ tới địa chỉ nội bộ nên không tải"

// blockedMarker đi trong reason phrase và thân phản hồi của proxy. Nhờ nó,
// lỗi của tiến trình con (yt-dlp in "HTTP Error 403: …") được nhận ra là do
// cổng chặn chứ không phải đích trả 403 thật.
const blockedMarker = "viderse-egress-blocked"

// IsBlockedOutput báo chuỗi lỗi của tiến trình con có phải do cổng chặn đích.
func IsBlockedOutput(s string) bool { return strings.Contains(s, blockedMarker) }

// LookupIPFunc phân giải tên host. nil → resolver của hệ điều hành.
type LookupIPFunc func(ctx context.Context, host string) ([]net.IP, error)

// blockedPrefixes là các dải không nằm trong IsGlobalUnicast/IsPrivate của
// thư viện chuẩn nhưng vẫn là máy nội bộ, hạ tầng nhà mạng hoặc địa chỉ không
// dùng để nối ra Internet.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),      // "this network"
	netip.MustParsePrefix("10.0.0.0/8"),     // RFC1918
	netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT, Tailscale
	netip.MustParsePrefix("127.0.0.0/8"),    // loopback
	netip.MustParsePrefix("169.254.0.0/16"), // link-local, metadata cloud
	netip.MustParsePrefix("172.16.0.0/12"),  // RFC1918
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
	netip.MustParsePrefix("192.168.0.0/16"), // RFC1918
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmark
	netip.MustParsePrefix("224.0.0.0/4"),    // multicast
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved + broadcast
	netip.MustParsePrefix("255.255.255.255/32"),
	netip.MustParsePrefix("::/96"),          // IPv4-compatible (gồm :: và ::1)
	netip.MustParsePrefix("fc00::/7"),       // ULA
	netip.MustParsePrefix("fe80::/10"),      // link-local
	netip.MustParsePrefix("ff00::/8"),       // multicast
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64 well-known: 32 bit cuối là IPv4
	netip.MustParsePrefix("64:ff9b:1::/48"), // NAT64 local-use
	netip.MustParsePrefix("2002::/16"),      // 6to4: IPv4 nằm trong bit 16..48
}

var nat64Prefixes = []netip.Prefix{
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

var sixToFour = netip.MustParsePrefix("2002::/16")

// Blocked báo địa chỉ thuộc loopback, link-local (gồm metadata cloud), mạng
// riêng, CGNAT, multicast, chưa xác định, dải dành riêng, hoặc là dạng
// IPv4-mapped/NAT64/6to4 của một trong các địa chỉ đó.
func Blocked(addr netip.Addr) bool {
	addr = addr.Unmap().WithZone("")
	if !addr.IsValid() {
		return true
	}
	if addr.Is4() {
		return blockedV4(addr)
	}
	b := addr.As16()
	for _, p := range nat64Prefixes {
		if p.Contains(addr) {
			return Blocked(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
		}
	}
	if sixToFour.Contains(addr) {
		return Blocked(netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}))
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// blockedV4 áp quy tắc cho địa chỉ IPv4 đã chuẩn hoá.
func blockedV4(addr netip.Addr) bool {
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// PublicAddrs phân giải host thành địa chỉ và từ chối nếu BẤT KỲ địa chỉ nào
// là nội bộ: một tên có cả bản ghi công khai lẫn nội bộ vẫn là đích nội bộ.
func PublicAddrs(ctx context.Context, lookup LookupIPFunc, host string) ([]netip.Addr, error) {
	if host == "" || strings.Contains(host, "%") {
		return nil, ErrBlocked
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if Blocked(ip) {
			return nil, ErrBlocked
		}
		return []netip.Addr{ip.Unmap().WithZone("")}, nil
	}
	if lookup == nil {
		lookup = SystemLookupIP
	}
	ips, err := lookup(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, ErrUnresolvable
	}
	addrs := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if a, ok := netip.AddrFromSlice(ip); ok {
			addrs = append(addrs, a.Unmap().WithZone(""))
		}
	}
	if len(addrs) == 0 {
		return nil, ErrUnresolvable
	}
	for _, a := range addrs {
		if Blocked(a) {
			return nil, ErrBlocked
		}
	}
	return addrs, nil
}

// SystemLookupIP phân giải bằng resolver của hệ điều hành.
func SystemLookupIP(ctx context.Context, host string) ([]net.IP, error) {
	found, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	out := make([]net.IP, 0, len(found))
	for _, f := range found {
		out = append(out, f.IP)
	}
	return out, err
}

// Dialer nối tới địa chỉ công khai. Tên được phân giải ĐÚNG MỘT LẦN rồi nối
// bằng IP literal: để net.Dialer tự phân giải lại chính là lỗ hổng DNS
// rebinding (kiểm thấy IP công khai, nối lại thấy 127.0.0.1).
type Dialer struct {
	// Lookup phân giải tên; nil → resolver hệ thống.
	Lookup LookupIPFunc
	// Timeout cho mỗi lần nối; 0 → 30 giây.
	Timeout time.Duration
	// Dial là seam test: nối tới địa chỉ ĐÃ kiểm. nil → net.Dialer thật.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
}

// DialContext nối tới address nếu mọi địa chỉ phân giải được đều công khai.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := PublicAddrs(ctx, d.Lookup, host)
	if err != nil {
		return nil, err
	}
	dial := d.Dial
	if dial == nil {
		timeout := d.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
		dial = dialer.DialContext
	}
	var firstErr error
	for _, a := range addrs {
		conn, derr := dial(ctx, network, net.JoinHostPort(a.String(), port))
		if derr == nil {
			return conn, nil
		}
		if firstErr == nil {
			firstErr = derr
		}
	}
	if firstErr == nil {
		firstErr = ErrUnresolvable
	}
	return nil, firstErr
}

// Transport trả transport chỉ nối tới địa chỉ công khai. Proxy từ biến môi
// trường bị tắt — proxy tự nối tới đích nên sẽ vòng qua kiểm tra này.
func (d *Dialer) Transport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = d.DialContext
	return transport
}

// Guard là cổng ra của cả process. Ở chế độ hosted nó giữ một proxy chuyển
// tiếp trên loopback (cổng ngẫu nhiên, credential ngẫu nhiên) mà mọi tiến
// trình con chạm tới link của thành viên phải đi qua; ở chế độ desktop (app
// một chủ trên máy cá nhân) nó không làm gì — hành vi cũ giữ nguyên.
type Guard struct {
	hosted   bool
	dialer   *Dialer
	proxy    *proxyServer
	proxyURL string
}

var (
	activeMu sync.Mutex
	active   *Guard
)

// Install dựng cổng ra cho process và (khi hosted) mở proxy loopback. Gọi
// nhiều lần thì cổng cũ được đóng trước — test dùng điều này để đổi chế độ.
func Install(o Options) (*Guard, error) {
	activeMu.Lock()
	defer activeMu.Unlock()
	if active != nil {
		active.close()
		active = nil
	}
	g := &Guard{hosted: o.Hosted, dialer: &Dialer{Lookup: o.Lookup, Dial: o.Dial}}
	if o.Hosted {
		p, err := startProxy(g.dialer)
		if err != nil {
			return nil, err
		}
		g.proxy = p
		g.proxyURL = p.url()
	}
	active = g
	return g, nil
}

// Options cấu hình cổng ra.
type Options struct {
	// Hosted báo máy chủ đang phục vụ người khác qua PUBLIC_URL.
	Hosted bool
	// Lookup phân giải tên; nil → resolver hệ thống.
	Lookup LookupIPFunc
	// Dial là seam test: nối tới địa chỉ ĐÃ kiểm.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
}

// Active trả cổng ra của process. Chưa cài (desktop, hoặc lệnh không phục vụ)
// thì trả Guard rỗng: mọi phương thức là no-op, không phải kiểm nil.
func Active() *Guard {
	activeMu.Lock()
	defer activeMu.Unlock()
	if active != nil {
		return active
	}
	return &Guard{}
}

// Close đóng proxy của process (dừng phục vụ). Idempotent.
func Close() {
	activeMu.Lock()
	defer activeMu.Unlock()
	if active != nil {
		active.close()
		active = nil
	}
}

func (g *Guard) close() {
	if g != nil && g.proxy != nil {
		g.proxy.close()
		g.proxy = nil
	}
}

// Hosted báo cổng ra đang bật (máy chủ chia sẻ).
func (g *Guard) Hosted() bool { return g != nil && g.hosted }

// ProxyURL trả URL proxy kèm credential của process; rỗng ở chế độ desktop.
func (g *Guard) ProxyURL() string {
	if g == nil {
		return ""
	}
	return g.proxyURL
}

// YtDlpArgs là cờ phải đứng trước mọi lời gọi yt-dlp do thành viên gián tiếp
// điều khiển: đi qua proxy của cổng ra.
func (g *Guard) YtDlpArgs() []string {
	if !g.Hosted() {
		return nil
	}
	return []string{"--proxy", g.proxyURL}
}

// YtDlpFormatFilter là bộ lọc format của yt-dlp chỉ giữ lại những protocol đi
// qua được proxy HTTP: http, https, m3u8, m3u8_native, http_dash_segments và
// http_dash_segments_generator. rtmp/rtmpe/rtsp/mms/f4m/ism/websocket_frag…
// nối trực tiếp, không qua proxy, nên không được chọn khi máy chủ chia sẻ.
//
// Đây là bộ lọc của chính yt-dlp (mỗi format luôn có trường "protocol" sau
// bước sắp xếp), nên nó áp cho cả lựa chọn format lẫn các format được ghép.
const YtDlpFormatFilter = `[protocol~="^(https?|m3u8(_native)?|http_dash_segments(_generator)?)$"]`

// AllowedYtDlpProtocol báo một protocol của yt-dlp có đi qua proxy HTTP của
// cổng ra hay không. Danh sách này phải khớp đúng YtDlpFormatFilter.
func AllowedYtDlpProtocol(protocol string) bool {
	switch protocol {
	case "http", "https", "m3u8", "m3u8_native", "http_dash_segments", "http_dash_segments_generator":
		return true
	}
	return false
}

// UnsafeFormatMessage là câu trả cho thành viên khi video chỉ có định dạng
// không đi qua được cổng ra (rtmp hoặc tương tự).
const UnsafeFormatMessage = "Định dạng của video này không tải an toàn được (rtmp hoặc tương tự) — chọn video khác"

// ytDlpDefaultFormat là format mặc định của yt-dlp khi có ffmpeg để ghép; dùng
// khi lời gọi tải không tự đặt -f thì vẫn phải có bộ lọc protocol.
const ytDlpDefaultFormat = "bv*+ba/b"

// ytDlpProbeArgs là các cờ chỉ lấy thông tin, không tải: lời gọi như vậy không
// cần (và không nên) bị gắn thêm -f.
var ytDlpProbeArgs = []string{
	"--skip-download", "-s", "--simulate", "-J", "--dump-json",
	"--dump-single-json", "--list-formats", "-F", "--print-json",
	"--version", "--update", "--update-to",
}

// HardenYtDlpArgs gắn bộ lọc protocol vào mọi lựa chọn format của lời gọi
// yt-dlp do thành viên gián tiếp điều khiển, để yt-dlp không bao giờ chọn một
// format nối trực tiếp (rtmp…) thay vì đi qua proxy của cổng ra. Ở chế độ
// desktop trả nguyên args.
//
// Lời gọi tải mà không tự đặt -f sẽ được thêm format mặc định đã lọc; lời gọi
// chỉ lấy thông tin (-J, --skip-download…) thì để nguyên, vì chúng không tải
// và bộ lọc rỗng sẽ làm hỏng cả những video vốn tải được.
func (g *Guard) HardenYtDlpArgs(args []string) []string {
	if !g.Hosted() {
		return args
	}
	out := make([]string, 0, len(args)+2)
	hardened := false
	probe := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--format":
			if i+1 < len(args) {
				out = append(out, arg, hardenFormatSpec(args[i+1]))
				i++
				hardened = true
				continue
			}
		case strings.HasPrefix(arg, "--format="):
			out = append(out, "--format="+hardenFormatSpec(strings.TrimPrefix(arg, "--format=")))
			hardened = true
			continue
		case isYtDlpProbeArg(arg):
			probe = true
		}
		out = append(out, arg)
	}
	if hardened || probe {
		return out
	}
	return append(out, "-f", hardenFormatSpec(ytDlpDefaultFormat))
}

func isYtDlpProbeArg(arg string) bool {
	for _, probe := range ytDlpProbeArgs {
		if arg == probe || strings.HasPrefix(arg, probe+"=") {
			return true
		}
	}
	return false
}

// hardenFormatSpec gắn bộ lọc vào từng lựa chọn ở mức ngoài cùng của một
// format spec: "bv*+ba/b" → "bv*[lọc]+ba[lọc]/b[lọc]". Ba dấu phân cách ngoài
// cùng đều tách được: "/" (lựa chọn thay thế), "+" và "," (ghép nhiều format).
// Trong ngoặc và ngoặc vuông thì không tách — cả nhóm "(bv+ba)" nhận một bộ
// lọc ở cuối, đủ vì nhóm chỉ chọn ra một format.
func hardenFormatSpec(spec string) string {
	if spec == "" {
		return spec
	}
	var out strings.Builder
	depth, start := 0, 0
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case '/', '+', ',':
			if depth == 0 {
				out.WriteString(hardenFormatChoice(spec[start:i]))
				out.WriteByte(spec[i])
				start = i + 1
			}
		}
	}
	out.WriteString(hardenFormatChoice(spec[start:]))
	return out.String()
}

func hardenFormatChoice(choice string) string {
	if choice == "" || strings.Contains(choice, "protocol") {
		return choice
	}
	return choice + YtDlpFormatFilter
}

// FFmpegProxyArgs là cờ proxy cho ffmpeg khi đầu vào có URL của thành viên.
// ffmpeg chỉ đọc biến môi trường http_proxy (chữ thường) cho cả http lẫn
// https, nên cờ này là đường chắc chắn; Env() giữ phần còn lại.
func (g *Guard) FFmpegProxyArgs() []string {
	if !g.Hosted() {
		return nil
	}
	return []string{"-http_proxy", g.proxyURL}
}

// proxyVars là các biến môi trường mà tiến trình con có thể dùng để đi vòng
// qua cổng ra (proxy riêng hoặc danh sách miễn trừ).
var proxyVars = map[string]bool{
	"http_proxy":  true,
	"https_proxy": true,
	"all_proxy":   true,
	"no_proxy":    true,
	"ftp_proxy":   true,
}

// Env trả môi trường ĐẦY ĐỦ cho tiến trình con chạm link của thành viên, hoặc
// nil ở chế độ desktop (thừa kế môi trường của app như cũ).
//
// Mọi biến proxy thừa kế từ môi trường của service bị bỏ — kể cả cách viết
// khác hoa thường như No_Proxy — rồi đặt lại đúng proxy của cổng ra và để
// no_proxy rỗng: một NO_PROXY có sẵn trong môi trường service sẽ khiến
// yt-dlp/curl nối thẳng tới đích, đúng thứ cần chặn.
func (g *Guard) Env() map[string]string {
	if !g.Hosted() {
		return nil
	}
	env := make(map[string]string, 32)
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if proxyVars[strings.ToLower(key)] {
			continue
		}
		env[key] = value
	}
	for _, key := range []string{"http_proxy", "https_proxy", "all_proxy", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		env[key] = g.proxyURL
	}
	env["no_proxy"] = ""
	env["NO_PROXY"] = ""
	return env
}

// Stats trả số lượt proxy đã chuyển tiếp và số lượt bị chặn vì đích nội bộ.
// Dùng cho quan sát và cho smoke test của process thật; desktop luôn (0, 0).
func (g *Guard) Stats() (proxied, blocked uint64) {
	if g == nil || g.proxy == nil {
		return 0, 0
	}
	return g.proxy.proxied.Load(), g.proxy.blocked.Load()
}

// Redact xoá credential của proxy khỏi chuỗi sắp hiện cho thành viên: URL
// proxy đi vào argv của tiến trình con nên có thể xuất hiện trong stderr.
func (g *Guard) Redact(s string) string {
	if g == nil || g.proxyURL == "" {
		return s
	}
	if !strings.Contains(s, g.proxyURL) {
		return s
	}
	return strings.ReplaceAll(s, g.proxyURL, "http://127.0.0.1:0")
}
