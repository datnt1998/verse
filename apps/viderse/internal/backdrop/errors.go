// Package backdrop giữ thư viện "Video nền": kênh nền Twitch/Bilibili kèm danh
// mục video đã đọc, video lẻ có đoạn dùng đã tải, bộ chọn nguồn nền cho một
// video, và bộ lập lịch đọc dần danh mục Bilibili.
//
// Mọi lời gọi ra ngoài đi qua internal/platforms (nhịp, hạn mức cuộn, giờ chờ
// bị chặn, lease) và internal/tools (yt-dlp); gói này không tự gọi mạng trong
// test — fetch và runner đều tiêm được.
package backdrop

import (
	"errors"
	"fmt"
	"time"
)

// Mã lỗi của mảng Video nền, dùng chung với các mảng khác (web map sang HTTP).
const (
	CodeInvalidInput        = "invalid_input"
	CodeNotFound            = "not_found"
	CodeConflict            = "conflict"
	CodeInUse               = "in_use"
	CodeRateLimited         = "rate_limited"
	CodeUnavailable         = "unavailable"
	CodeLoginRequired       = "login_required"
	CodeFailed              = "failed"
	CodeInvalidCursor       = "invalid_cursor"
	CodeUnsupportedPlatform = "unsupported_platform"
)

// Error là lỗi nghiệp vụ đã dịch: mã ổn định, câu cho người dùng, gợi ý phục
// hồi (tuỳ chọn). Không lộ chi tiết thô của nền tảng.
type Error struct {
	Code     string
	Message  string
	Recovery string
	RetryAt  *time.Time
}

func (e *Error) Error() string { return e.Message }

// ErrorCode cho pacing/nhãn lỗi phân loại mà không phải so khớp chuỗi riêng.
func (e *Error) ErrorCode() string { return e.Code }

// Errorf dựng lỗi nghiệp vụ.
func Errorf(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// ErrorfRecovery dựng lỗi nghiệp vụ kèm gợi ý phục hồi.
func ErrorfRecovery(code, message, recovery string) *Error {
	return &Error{Code: code, Message: message, Recovery: recovery}
}

// RateLimitError là lỗi "bị giới hạn yêu cầu" kèm Retry-After của server;
// platforms.Pacer nhận diện qua RateLimited để đặt giờ chờ tăng dần.
type RateLimitError struct {
	Message string
	Retry   time.Duration
}

func (e *RateLimitError) Error() string             { return e.Message }
func (e *RateLimitError) ErrorCode() string         { return CodeRateLimited }
func (e *RateLimitError) RetryAfter() time.Duration { return e.Retry }

// errorCode đọc mã lỗi của một lỗi bất kỳ (rỗng khi không mang mã).
func errorCode(err error) string {
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) {
		return coded.ErrorCode()
	}
	return ""
}

// asError đổi lỗi bất kỳ về *Error để ghi vào danh mục (mã + câu).
func asError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeFailed, Message: err.Error()}
}

func notFoundChannel() *Error {
	return ErrorfRecovery(CodeNotFound, "Kênh nền không tồn tại", "Chạy `viderse backdrop list` để xem mã kênh")
}

func notFoundVideo() *Error {
	return ErrorfRecovery(CodeNotFound, "Video lẻ không tồn tại", "Mở lại Thư viện › Video nền để xem danh sách")
}

// invalidCursor báo con trỏ phân trang không đọc được.
func invalidCursor() *Error {
	return ErrorfRecovery(CodeInvalidCursor, "Con trỏ trang không hợp lệ", "Tải lại danh sách từ đầu (bỏ tham số cursor)")
}

// wrapf thêm ngữ cảnh nội bộ vào lỗi mà vẫn giữ mã lỗi gốc để map HTTP.
func wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
