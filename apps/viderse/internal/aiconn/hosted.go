package aiconn

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// LookupIPFunc phân giải tên host. nil → resolver hệ thống.
type LookupIPFunc func(ctx context.Context, host string) ([]net.IP, error)

// errInternalAddress: đích là loopback/link-local/mạng riêng — máy chủ chia sẻ
// không được gọi hộ người dùng. errUnresolvableHost: không xác minh được đích.
var (
	errInternalAddress  = errors.New("địa chỉ nội bộ")
	errUnresolvableHost = errors.New("không phân giải được địa chỉ")
)

// Dải địa chỉ không nằm trong IsPrivate/IsGlobalUnicast của thư viện chuẩn
// nhưng vẫn là mạng nội bộ hoặc hạ tầng của nhà mạng.
var internalPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), // CGNAT, Tailscale
	netip.MustParsePrefix("198.18.0.0/15"), // benchmark
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved
}

// nat64 là tiền tố 64:ff9b::/96: 32 bit cuối là một địa chỉ IPv4 thật.
var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// internalAddr báo địa chỉ thuộc loopback, link-local (gồm metadata cloud),
// mạng riêng, CGNAT, multicast hay chưa xác định — mọi đích mà một máy chủ
// được chia sẻ không được gọi hộ người dùng.
func internalAddr(addr netip.Addr) bool {
	addr = addr.Unmap().WithZone("")
	if addr.Is6() && nat64.Contains(addr) {
		b := addr.As16()
		return internalAddr(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
		return true
	}
	for _, p := range internalPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// hostedAddressError là lý do từ chối địa chỉ nội bộ khi máy chủ phục vụ người khác.
func hostedAddressError(message string) *Error {
	return apiError(http.StatusBadRequest, domain.ReasonInvalidBaseURL, message)
}

// publicAddrs phân giải host thành địa chỉ và từ chối nếu BẤT KỲ địa chỉ nào
// là nội bộ. Một hàm duy nhất cho cả lúc lưu cấu hình (checkHostedAddress) và
// lúc nối (publicDialContext): hai chỗ phải áp đúng một quy tắc.
func publicAddrs(ctx context.Context, lookup LookupIPFunc, host string) ([]netip.Addr, error) {
	if host == "" || strings.Contains(host, "%") {
		return nil, errInternalAddress
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if internalAddr(ip) {
			return nil, errInternalAddress
		}
		return []netip.Addr{ip}, nil
	}
	if lookup == nil {
		lookup = systemLookupIP
	}
	ips, err := lookup(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, errUnresolvableHost
	}
	addrs := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if a, ok := netip.AddrFromSlice(ip); ok {
			addrs = append(addrs, a)
		}
	}
	if len(addrs) == 0 {
		return nil, errUnresolvableHost
	}
	for _, a := range addrs {
		if internalAddr(a) {
			return nil, errInternalAddress
		}
	}
	return addrs, nil
}

// systemLookupIP phân giải bằng resolver của hệ điều hành.
func systemLookupIP(ctx context.Context, host string) ([]net.IP, error) {
	found, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	out := make([]net.IP, 0, len(found))
	for _, f := range found {
		out = append(out, f.IP)
	}
	return out, err
}

// checkHostedAddress, khi Options.Hosted bật, từ chối baseUrl phân giải ra
// địa chỉ nội bộ. Phải phân giải tên trước khi kiểm: "localhost" hay một tên
// miền trỏ về 10.x đều là đích nội bộ. Không phân giải được thì không xác
// minh được đích nên cũng từ chối.
func (s *Service) checkHostedAddress(ctx context.Context, base string) error {
	if !s.o.Hosted {
		return nil
	}
	const internal = "Máy chủ đang được chia sẻ nên không nhận địa chỉ nội bộ (localhost, mạng riêng). Dùng địa chỉ công khai."
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return hostedAddressError("Địa chỉ máy chủ không hợp lệ")
	}
	switch _, err := publicAddrs(ctx, s.o.LookupIP, u.Hostname()); {
	case err == nil:
		return nil
	case errors.Is(err, errInternalAddress):
		return hostedAddressError(internal)
	default:
		return hostedAddressError("Không phân giải được địa chỉ máy chủ")
	}
}

// publicDialContext trả DialContext chỉ nối tới địa chỉ công khai. Tên được
// phân giải ĐÚNG MỘT LẦN ở đây rồi nối thẳng bằng IP literal: để net.Dialer
// tự phân giải lại chính là lỗ hổng DNS rebinding (kiểm tra thấy IP công
// khai, nối lại thấy 127.0.0.1). Vì địa chỉ literal không đổi giữa hai lần,
// mọi kết nối — kể cả keep-alive nối lại sau khi đích đổi DNS — đều bị chặn.
func publicDialContext(lookup LookupIPFunc) func(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addrs, err := publicAddrs(ctx, lookup, host)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, a := range addrs {
			conn, derr := dialer.DialContext(ctx, network, net.JoinHostPort(a.String(), port))
			if derr == nil {
				return conn, nil
			}
			if firstErr == nil {
				firstErr = derr
			}
		}
		return nil, firstErr
	}
}

// publicOnlyTransport là transport của client gọi địa chỉ do người dùng nhập
// khi máy chủ đang chia sẻ: mỗi lần nối đều kiểm địa chỉ thật. Proxy từ biến
// môi trường bị tắt — proxy tự nối tới đích nên sẽ vòng qua kiểm tra này.
func publicOnlyTransport(lookup LookupIPFunc) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = publicDialContext(lookup)
	return transport
}

// sameHost báo hai baseUrl trỏ cùng scheme + host + cổng. Cùng máy chủ thì
// khoá đã lưu vẫn đi đúng nơi nó được cấp; khác máy chủ thì khoá không được đi theo.
func sameHost(a, b string) bool {
	return hostKey(a) == hostKey(b)
}

func hostKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return raw
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}
