package identity

import (
	"fmt"
	"strings"

	"rsc.io/qr"
)

// QRSVG 把文本编码为自包含的 SVG 二维码（含 4 模块静区），供网页直接内嵌。
func QRSVG(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	const quiet = 4
	size := code.Size + 2*quiet
	var path strings.Builder
	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="100%%" height="100%%" fill="#fff"/><path fill="#000" d="%s"/></svg>`, size, size, path.String()), nil
}

// DeviceLabel 从 User-Agent 粗略识别设备，用于会话列表展示。
func DeviceLabel(userAgent string) string {
	ua := strings.ToLower(userAgent)
	device := "Thiết bị khác"
	switch {
	case strings.Contains(ua, "iphone"):
		device = "iPhone"
	case strings.Contains(ua, "ipad"):
		device = "iPad"
	case strings.Contains(ua, "android"):
		device = "Android"
	case strings.Contains(ua, "mac os x") || strings.Contains(ua, "macintosh"):
		device = "Mac"
	case strings.Contains(ua, "windows"):
		device = "Windows"
	case strings.Contains(ua, "linux"):
		device = "Linux"
	case strings.Contains(ua, "curl") || strings.Contains(ua, "go-http-client"):
		device = "Script"
	}
	browser := ""
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "crios") || strings.Contains(ua, "chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "fxios") || strings.Contains(ua, "firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "safari/"):
		browser = "Safari"
	}
	if browser == "" {
		return device
	}
	return device + " · " + browser
}
