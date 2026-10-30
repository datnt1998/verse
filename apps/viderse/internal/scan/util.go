package scan

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// newToken sinh token fence (uuid v4 ngẫu nhiên, dạng chuỗi).
func newToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newOwner đặt tên chủ giữ lease: nhận diện process trong chẩn đoán mà không
// cần pid (in-process worker chỉ có một).
func newOwner() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("scan-%x", b[:])
}

var (
	errURLRe   = regexp.MustCompile(`https?://\S+`)
	errTokenRe = regexp.MustCompile(`[A-Za-z0-9_\-%=+/]{40,}`)
	errSpaceRe = regexp.MustCompile(`\s+`)
)

// errorTextMax là trần một dòng chẩn đoán lưu trong scan_jobs/notification.
const errorTextMax = 300

// SanitizeError làm sạch một dòng chẩn đoán: che URL, che chuỗi dài dạng
// token, gom khoảng trắng và cắt tới 300 ký tự (chuyển từ sanitizeScanError).
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	raw := err.Error()
	raw = errURLRe.ReplaceAllString(raw, "[url]")
	raw = errTokenRe.ReplaceAllString(raw, "[redacted]")
	raw = strings.TrimSpace(errSpaceRe.ReplaceAllString(raw, " "))
	if len(raw) > errorTextMax {
		raw = raw[:errorTextMax]
	}
	return raw
}

// scanBackoffDelay tính độ trễ backoff mũ có jitter ±20 %, trần 30 phút:
// min(round(min(base·2^attempt, 30min)·(1±0.2)), 30min) — attempt=0 cho lỗi đầu.
func scanBackoffDelay(base time.Duration, attempt int, random func() float64) time.Duration {
	const maxDelay = 30 * time.Minute
	if random == nil {
		random = func() float64 { return 0.5 }
	}
	raw := base
	for i := 0; i < attempt && raw < maxDelay; i++ {
		raw *= 2
	}
	if raw > maxDelay {
		raw = maxDelay
	}
	delay := time.Duration(float64(raw) * (1 + (random()*2-1)*0.2))
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}
