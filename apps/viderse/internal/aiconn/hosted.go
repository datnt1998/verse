package aiconn

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/netguard"
)

// LookupIPFunc phân giải tên host. nil → resolver hệ thống. Cùng kiểu với
// netguard để test và wiring truyền thẳng, không qua lớp chuyển.
type LookupIPFunc = netguard.LookupIPFunc

// errInternalAddress: đích là loopback/link-local/mạng riêng — máy chủ chia sẻ
// không được gọi hộ người dùng. errUnresolvableHost: không xác minh được đích.
//
// Cả hai là lỗi của gói netguard: quy tắc "địa chỉ nào là nội bộ" chỉ có một
// bản trong toàn app, dùng chung với proxy của tiến trình con và bộ tải media
// từ link.
var (
	errInternalAddress  = netguard.ErrBlocked
	errUnresolvableHost = netguard.ErrUnresolvable
)

// hostedAddressError là lý do từ chối địa chỉ nội bộ khi máy chủ phục vụ người khác.
func hostedAddressError(message string) *Error {
	return apiError(http.StatusBadRequest, domain.ReasonInvalidBaseURL, message)
}

// publicAddrs phân giải host thành địa chỉ và từ chối nếu BẤT KỲ địa chỉ nào
// là nội bộ. Một hàm duy nhất cho cả lúc lưu cấu hình (checkHostedAddress) và
// lúc nối (publicDialContext): hai chỗ phải áp đúng một quy tắc.
func publicAddrs(ctx context.Context, lookup LookupIPFunc, host string) ([]netip.Addr, error) {
	return netguard.PublicAddrs(ctx, lookup, host)
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
	dialer := &netguard.Dialer{Lookup: lookup}
	return dialer.DialContext
}

// publicOnlyTransport là transport của client gọi địa chỉ do người dùng nhập
// khi máy chủ đang chia sẻ: mỗi lần nối đều kiểm địa chỉ thật. Proxy từ biến
// môi trường bị tắt — proxy tự nối tới đích nên sẽ vòng qua kiểm tra này.
func publicOnlyTransport(lookup LookupIPFunc) *http.Transport {
	dialer := &netguard.Dialer{Lookup: lookup}
	return dialer.Transport()
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
