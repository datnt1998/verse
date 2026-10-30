package identity

import (
	"regexp"
	"strconv"
	"testing"
)

var (
	qrViewBox = regexp.MustCompile(`viewBox="0 0 (\d+) \d+"`)
	qrSeg     = regexp.MustCompile(`M(\d+) (\d+)h`)
)

// Mã QR trong hộp thoại đăng nhập nền tảng dùng vùng trống 2 module (nền đã có
// padding 14px làm viền trắng) để mã to hơn mà vẫn đủ mức tối thiểu 2 module.
// Lời gọi mặc định của Noverse giữ 4.
func TestQRSVGQuietZone(t *testing.T) {
	const text = "https://passport.bilibili.com/h5-app/passport/login/scan?k=abc123"
	for _, quiet := range []int{2, 4} {
		svg, err := QRSVGQuiet(text, quiet)
		if err != nil {
			t.Fatalf("quiet=%d: %v", quiet, err)
		}
		m := qrViewBox.FindStringSubmatch(svg)
		if m == nil {
			t.Fatalf("quiet=%d: no viewBox", quiet)
		}
		size, _ := strconv.Atoi(m[1])
		min := size
		for _, seg := range qrSeg.FindAllStringSubmatch(svg, -1) {
			x, _ := strconv.Atoi(seg[1])
			y, _ := strconv.Atoi(seg[2])
			if x < min {
				min = x
			}
			if y < min {
				min = y
			}
		}
		if min != quiet {
			t.Fatalf("quiet=%d: ink starts at module %d, want %d", quiet, min, quiet)
		}
	}
}
