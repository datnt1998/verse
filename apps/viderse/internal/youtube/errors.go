package youtube

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ErrorKind phân loại lỗi phía YouTube để caller quyết định có lên lịch lại,
// báo người dùng hay đánh dấu kênh hỏng. Giá trị khớp enum của bản TS.
type ErrorKind string

const (
	// KindRateLimited: HTTP 429; RetryDelay mang Retry-After nếu có.
	KindRateLimited ErrorKind = "rate_limited"
	// KindTransient: lỗi mạng, 5xx hoặc thân phản hồi bị cắt.
	KindTransient ErrorKind = "transient"
	// KindBlocked: trang thử thách bot/đăng nhập hoặc 401/403.
	KindBlocked ErrorKind = "blocked"
	// KindCursorInvalid: continuation bị từ chối, hết hạn hoặc phát cho phiên
	// bản client khác; cũng dùng cho cursor/remainder lưu trữ bị hỏng.
	KindCursorInvalid ErrorKind = "cursor_invalid"
	// KindNotFound: kênh không tồn tại.
	KindNotFound ErrorKind = "not_found"
	// KindUnexpectedPage: hình dạng renderer/thứ tự không đọc được an toàn.
	KindUnexpectedPage ErrorKind = "unexpected_page"
)

// FetchError là lỗi nhà cung cấp đã phân loại. Message không bao giờ chứa
// continuation token, visitor data hay thân phản hồi nên an toàn để lưu và hiển thị.
type FetchError struct {
	Kind    ErrorKind
	Message string
	// Status là mã HTTP khi lỗi đến từ một phản hồi; 0 nếu không có.
	Status int
	// RetryDelay là Retry-After đã parse (chỉ khi 429); nil khi thiếu/không đọc được.
	RetryDelay *time.Duration
}

func (e *FetchError) Error() string { return e.Message }

func newFetchError(kind ErrorKind, message string, status int, retryDelay *time.Duration) *FetchError {
	return &FetchError{Kind: kind, Message: message, Status: status, RetryDelay: retryDelay}
}

// KindOf trả ErrorKind của một FetchError, ok=false cho các loại lỗi khác.
func KindOf(err error) (ErrorKind, bool) {
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe.Kind, true
	}
	return "", false
}

// HTTPStatusError là phản hồi không-2xx từ YouTube trước khi phân loại;
// chỉ mang những gì việc phân loại cần.
type HTTPStatusError struct {
	Status     int
	RetryAfter string
	// BodySnippet là đầu thân phản hồi, chỉ để nhận diện trang thử thách bot/consent.
	BodySnippet string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("YouTube request failed with HTTP %d", e.Status)
}

// ChannelNotFoundError: đầu vào không trỏ tới kênh YouTube nào tồn tại.
type ChannelNotFoundError struct{ Message string }

func (e *ChannelNotFoundError) Error() string { return e.Message }

// UploadTimeBlockedError: YouTube trả trang thử thách bot/đăng nhập thay vì
// trang video — dừng thăm dò một lúc, không phải lỗi của video riêng lẻ.
type UploadTimeBlockedError struct{ Reason string }

func (e *UploadTimeBlockedError) Error() string {
	reason := e.Reason
	if len(reason) > 200 {
		reason = reason[:200]
	}
	return fmt.Sprintf("YouTube refused the video page: %s", reason)
}

// DeferredError do bộ điều tốc trả khi ô lịch toàn cục nằm xa hơn ngân sách chờ
// của caller (deadline của context, hoặc Pacer.MaxWait khi không có deadline).
// Không mang nghĩa lỗi YouTube; caller nên thử lại sau RetryAt.
type DeferredError struct {
	RetryAt time.Time
}

func (e *DeferredError) Error() string {
	return fmt.Sprintf("youtube request deferred until %s", e.RetryAt.UTC().Format(time.RFC3339))
}

const maxRetryAfterSeconds = 31_536_000

// ParseRetryAfter đọc Retry-After thành độ trễ: delta-seconds hoặc HTTP date
// so với now. Trả nil khi thiếu hoặc không parse được.
func ParseRetryAfter(value string, now time.Time) *time.Duration {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	if isAllDigits(text) {
		var sec int64
		for _, r := range text {
			sec = sec*10 + int64(r-'0')
			if sec > maxRetryAfterSeconds {
				sec = maxRetryAfterSeconds
				break
			}
		}
		d := time.Duration(sec) * time.Second
		return &d
	}
	at, err := time.Parse(time.RFC1123, text)
	if err != nil {
		// HTTP date có thể dùng các layout khác nhau; thử layout đầy đủ RFC 850/ANSI C.
		for _, layout := range []string{time.RFC850, time.ANSIC, "Mon, 02-Jan-2006 15:04:05 GMT"} {
			if at, err = time.Parse(layout, text); err == nil {
				break
			}
		}
		if err != nil {
			return nil
		}
	}
	d := at.Sub(now)
	if d < 0 {
		d = 0
	}
	return &d
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

var challengeRe = regexp.MustCompile(`(?i)confirm you.?re not a bot|unusual traffic|captcha|sign in to confirm`)

var tokenLikeRe = regexp.MustCompile(`[\w%+/=.~-]{32,}`)
var whitespaceRe = regexp.MustCompile(`\s+`)

// sanitizeDiagnostic dồn khoảng trắng, che các chuỗi dài dạng token và giới hạn
// độ dài chẩn đoán — message lỗi không bao giờ lộ token/visitor/body.
func sanitizeDiagnostic(text string) string {
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = tokenLikeRe.ReplaceAllString(text, "[redacted]")
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

// errSession là lỗi bootstrap session không đến từ HTTP status (thân
// sw.js_data sai định dạng); xếp transient như SyntaxError bên TS.
type errSession struct{ msg string }

func (e *errSession) Error() string { return e.msg }

// errInnertube tương đương InnertubeError của youtubei.js: dữ liệu trang không
// dùng được; ngữ cảnh continuation → cursor_invalid, còn lại → unexpected_page.
type errInnertube struct{ msg string }

func (e *errInnertube) Error() string { return e.msg }

// httpFailure xếp loại một phản hồi không-2xx theo ngữ cảnh trang đầu hay trang tiếp.
func httpFailure(e *HTTPStatusError, continuation bool, now time.Time) *FetchError {
	status := e.Status
	if challengeRe.MatchString(e.BodySnippet) || status == 401 || status == 403 {
		return newFetchError(KindBlocked,
			fmt.Sprintf("YouTube refused the request or asked for verification (HTTP %d)", status), status, nil)
	}
	if status == 429 {
		return newFetchError(KindRateLimited,
			"YouTube rate-limited the request (HTTP 429)", status, ParseRetryAfter(e.RetryAfter, now))
	}
	if status == 408 || status >= 500 {
		return newFetchError(KindTransient,
			fmt.Sprintf("Temporary YouTube failure (HTTP %d)", status), status, nil)
	}
	if continuation && (status == 400 || status == 404) {
		return newFetchError(KindCursorInvalid,
			fmt.Sprintf("YouTube rejected the continuation (HTTP %d)", status), status, nil)
	}
	if status == 404 {
		return newFetchError(KindNotFound, "Channel not found (HTTP 404)", status, nil)
	}
	return newFetchError(KindUnexpectedPage,
		fmt.Sprintf("Unexpected YouTube answer (HTTP %d)", status), status, nil)
}

// normalizeError: lỗi ctx (huỷ/hết hạn) và DeferredError của bộ điều tốc đi
// nguyên vẹn; mọi thứ còn lại thành FetchError đã phân loại.
func normalizeError(err error, continuation bool, now time.Time) error {
	var fe *FetchError
	if errors.As(err, &fe) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var de *DeferredError
	if errors.As(err, &de) {
		return err
	}
	var hs *HTTPStatusError
	if errors.As(err, &hs) {
		return httpFailure(hs, continuation, now)
	}
	var nf *ChannelNotFoundError
	if errors.As(err, &nf) {
		return newFetchError(KindNotFound, nf.Message, 0, nil)
	}
	if isTransportError(err) {
		return newFetchError(KindTransient, "Network failure or truncated YouTube response", 0, nil)
	}
	var ie *errInnertube
	if errors.As(err, &ie) {
		if continuation {
			return newFetchError(KindCursorInvalid,
				fmt.Sprintf("Continuation response could not be used: %s", sanitizeDiagnostic(ie.msg)), 0, nil)
		}
		return newFetchError(KindUnexpectedPage,
			fmt.Sprintf("Unexpected YouTube response: %s", sanitizeDiagnostic(ie.msg)), 0, nil)
	}
	// Lỗi không rõ loại luôn là trang bất thường: chỉ InnertubeError (dữ liệu
	// continuation không dùng được) mới thành cursor_invalid.
	return newFetchError(KindUnexpectedPage,
		fmt.Sprintf("Unexpected YouTube response: %s", sanitizeDiagnostic(err.Error())), 0, nil)
}
