package youtube

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Các parser thuần cho chữ người-đọc-được mà InnerTube trả về ("1.2M",
// "21.3M subscribers", "10d ago", "1:02:03"). Session luôn xin lang "en" nên
// chỉ cần xử lý dạng tiếng Anh.
var compactNumRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([KMBkmb])?`)
var noViewsRe = regexp.MustCompile(`(?i)^\s*no\s`)

// ParseCompactNumber: "1.2M" → 1200000, "1,234 views" → 1234, "No views" → 0;
// không parse được → -1 (dùng hàm Ptr biến thể cho giá trị nullable).
func ParseCompactNumber(text string) (int64, bool) {
	if text == "" {
		return 0, false
	}
	if noViewsRe.MatchString(text) {
		return 0, true
	}
	m := compactNumRe.FindStringSubmatch(strings.ReplaceAll(text, ",", ""))
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	switch strings.ToUpper(m[2]) {
	case "K":
		n *= 1e3
	case "M":
		n *= 1e6
	case "B":
		n *= 1e9
	}
	return int64(math.Round(n)), true
}

var durationRe = regexp.MustCompile(`^\d+(:\d{2}){1,2}$`)

// ParseDuration: "5:08" → 308, "1:02:03" → 3723, khác → 0,false.
func ParseDuration(text string) (int64, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !durationRe.MatchString(trimmed) {
		return 0, false
	}
	var acc int64
	for _, part := range strings.Split(trimmed, ":") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, false
		}
		acc = acc*60 + int64(n)
	}
	return acc, true
}

var relTimeRe = regexp.MustCompile(`(\d+)\s*([a-z]+)\s+ago\b`)

// unitByMS chuẩn hoá đơn vị viết tắt/dài về mili-giây (tháng=30d, năm=365d như bản TS).
func unitByMS(unit string) (int64, bool) {
	switch unit {
	case "s", "sec", "secs", "second", "seconds":
		return 1_000, true
	case "m", "min", "mins", "minute", "minutes":
		return 60_000, true
	case "h", "hr", "hrs", "hour", "hours":
		return 3_600_000, true
	case "d", "day", "days":
		return 86_400_000, true
	case "w", "wk", "wks", "week", "weeks":
		return 7 * 86_400_000, true
	case "mo", "mos", "month", "months":
		return 30 * 86_400_000, true
	case "y", "yr", "yrs", "year", "years":
		return 365 * 86_400_000, true
	}
	return 0, false
}

// ParseRelativeTime trả thời điểm tuyệt đối xấp xỉ cho "10d ago", "3 weeks ago",
// "Streamed 2 years ago". false khi không phải dạng thời gian tương đối quá khứ.
func ParseRelativeTime(text string, now time.Time) (time.Time, bool) {
	if text == "" {
		return time.Time{}, false
	}
	m := relTimeRe.FindStringSubmatch(strings.ToLower(text))
	if m == nil {
		return time.Time{}, false
	}
	ms, ok := unitByMS(m[2])
	if !ok {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return now.Add(-time.Duration(n*ms) * time.Millisecond), true
}

// Median trả trung vị của các giá trị không-nil; nil khi không có giá trị nào.
func Median(values []*float64) *float64 {
	sorted := make([]float64, 0, len(values))
	for _, v := range values {
		if v != nil {
			sorted = append(sorted, *v)
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	var m float64
	if len(sorted)%2 == 1 {
		m = sorted[mid]
	} else {
		m = (sorted[mid-1] + sorted[mid]) / 2
	}
	return &m
}
