package aiconn

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
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
	host := u.Hostname()
	if strings.Contains(host, "%") {
		return hostedAddressError(internal)
	}
	var addrs []netip.Addr
	if ip, perr := netip.ParseAddr(host); perr == nil {
		addrs = []netip.Addr{ip}
	} else {
		lookup := s.o.LookupIP
		if lookup == nil {
			lookup = func(ctx context.Context, host string) ([]net.IP, error) {
				found, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				out := make([]net.IP, 0, len(found))
				for _, f := range found {
					out = append(out, f.IP)
				}
				return out, err
			}
		}
		ips, lerr := lookup(ctx, host)
		if lerr != nil || len(ips) == 0 {
			return hostedAddressError("Không phân giải được địa chỉ máy chủ")
		}
		for _, ip := range ips {
			if a, ok := netip.AddrFromSlice(ip); ok {
				addrs = append(addrs, a)
			}
		}
		if len(addrs) == 0 {
			return hostedAddressError("Không phân giải được địa chỉ máy chủ")
		}
	}
	for _, a := range addrs {
		if internalAddr(a) {
			return hostedAddressError(internal)
		}
	}
	return nil
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
