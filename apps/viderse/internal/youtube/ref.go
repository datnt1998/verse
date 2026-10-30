package youtube

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxRefSegment chặn chuỗi dài vô lý trước khi gửi resolver (handle YouTube
// ngắn hơn nhiều, tên /c/ cũ dài hơn một chút).
const maxRefSegment = 100

// ParseChannelRef chuẩn hoá đầu vào "thêm đối thủ" thành URL kênh để
// ResolveChannelRef phân giải, hoặc trả thẳng UC id khi không cần request.
// ok=false khi đầu vào không thể trỏ tới một kênh: URL video (/watch, /shorts,
// youtu.be…), host khác YouTube, hay chuỗi không phải @handle/UC…/URL kênh.
//
// Đường dẫn kênh chấp nhận: /@handle, /channel/UC…, /c/…, /user/… (đồng các
// dạng yt-factory nhận qua resolveChannelId + resolve_url).
func ParseChannelRef(input string) (ref string, ok bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", false
	}
	if IsChannelID(raw) {
		return raw, true
	}
	if strings.HasPrefix(raw, "@") {
		// Dạng trần chỉ nhận đúng một đoạn handle; "@name/path" là link dán
		// thiếu host, không phải kênh.
		if !validRefSegment(strings.TrimSuffix(raw[1:], "/")) {
			return "", false
		}
		return "https://www.youtube.com/" + strings.TrimSuffix(raw, "/"), true
	}
	var u *url.URL
	if httpURLRe.MatchString(raw) {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		u = parsed
	} else {
		parsed, err := url.Parse("https://" + raw)
		if err != nil {
			return "", false
		}
		u = parsed
	}
	// Userinfo, cổng và query/fragment không thuộc URL kênh chuẩn tắc: từ chối
	// trước khi gửi resolver thay vì đoán ý người dùng.
	if u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host != "youtube.com" && !strings.HasSuffix(host, ".youtube.com") {
		return "", false
	}
	// Một đường dẫn kênh là đúng các đoạn sau (không lồng nhau, không thừa
	// đoạn); tab con hợp lệ của /channel/UC… được chấp nhận và chuẩn hoá về id.
	segments := splitPath(u.Path)
	if len(segments) == 0 {
		return "", false
	}
	switch {
	case strings.HasPrefix(segments[0], "@"):
		if !validRefSegment(segments[0][1:]) {
			return "", false
		}
		if len(segments) == 2 && !validChannelTab(segments[1]) {
			return "", false
		}
		if len(segments) > 2 {
			return "", false
		}
		return "https://www.youtube.com" + u.Path, true
	case segments[0] == "channel" && len(segments) >= 2 && len(segments) <= 3:
		if !IsChannelID(segments[1]) {
			return "", false
		}
		if len(segments) == 3 && !validChannelTab(segments[2]) {
			return "", false
		}
		return segments[1], true
	case (segments[0] == "c" || segments[0] == "user") && len(segments) == 2:
		if !validRefSegment(segments[1]) {
			return "", false
		}
		return "https://www.youtube.com" + u.Path, true
	}
	return "", false
}

// splitPath tách path đã giải mã thành đoạn, bỏ dấu "/" đầu/cuối; đoạn rỗng
// (do "//" hoặc mã hoá %2F) làm cả đường dẫn không hợp lệ.
func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return nil
		}
	}
	return parts
}

// validRefSegment nhận một đoạn handle/tên kênh: một đoạn đơn, không khoảng
// trắng/điều khiển, không ký tự phân tách URL hay mã hoá, cho phép chữ quốc tế.
func validRefSegment(seg string) bool {
	if seg == "" || utf8.RuneCountInString(seg) > maxRefSegment {
		return false
	}
	for _, r := range seg {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
		switch r {
		case '/', '\\', '?', '#', '&', '=', '%', ':', ';', '@':
			return false
		}
	}
	return true
}

// validChannelTab là các tab con của kênh — URL dán vào vẫn được hiểu là kênh.
func validChannelTab(seg string) bool {
	switch seg {
	case "videos", "shorts", "streams", "live", "playlists", "about",
		"community", "featured", "membership", "store", "podcasts", "releases":
		return true
	}
	return false
}
