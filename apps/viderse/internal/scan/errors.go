package scan

import (
	"errors"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Error là lỗi nghiệp vụ của luồng quét ở mặt API: Code là mã ổn định cho
// wire/CLI, Recovery là câu hướng dẫn tiếng Việt, RetryAt kèm khi YouTube bảo
// chờ, Existing là kênh đã theo dõi khi Add gặp trùng.
type Error struct {
	Code     string
	Message  string
	Recovery string
	RetryAt  *time.Time
	Existing *domain.RivalChannel
}

func (e *Error) Error() string { return e.Message }

// CodeOf trả mã lỗi của một *Error (rỗng với loại lỗi khác).
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// Mã lỗi mặt API của Trinh sát (contract §HTTP).
const (
	CodeInvalidInput   = "invalid_input"
	CodeNotFound       = "not_found"
	CodeExists         = "exists"
	CodeRateLimited    = "rate_limited"
	CodeBlocked        = "blocked"
	CodeUnexpectedPage = "unexpected_page"
	CodeTransient      = "transient"
	CodeInvalidCap     = "invalid_cap"
	CodeLeaseBusy      = "lease_busy"
)

// ── lỗi tầng lưu trữ (fence, việc, miền khoá) ────────────────────────────────

// StoreError là lỗi nguyên tắc của kho quét: fence hết chủ, việc đã dừng,
// kênh không còn, khoá sắp xếp vượt miền hay trang vượt trần.
type StoreError struct {
	Code    string
	Message string
}

func (e *StoreError) Error() string { return e.Message }

// Mã lỗi lưu trữ (giữ tập của bản TS).
const (
	StoreLeaseLost      = "lease_lost"
	StoreJobStopped     = "job_stopped"
	StoreChannelMissing = "channel_not_found"
	StoreKeyRange       = "key_range"
	StoreLimit          = "limit"
	// StoreCheckpointOwned: run job-less đã được một việc discovery khác giữ.
	StoreCheckpointOwned = "checkpoint_owned"
	// StoreAlreadyCapped: chỉ số lịch sử đã đạt cap (khi muốn quét tiếp).
	StoreAlreadyCapped = "already_capped"
	// StoreCursorInvalid: cursor/remainder đã lưu không đọc lại được.
	StoreCursorInvalid = "cursor_invalid"
	// StoreChannelExists: baseline kèm hồ sơ gặp kênh đã có chủ (Add đua nhau).
	StoreChannelExists = "channel_exists"
)

// StoreCodeOf trả mã StoreError (rỗng với loại lỗi khác).
func StoreCodeOf(err error) string {
	var e *StoreError
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// IsStopped báo lỗi có nghĩa "bỏ lượt, không ghi gì": fence đổi chủ, việc đã
// dừng hay kênh đã bị xoá giữa chừng.
func IsStopped(err error) bool {
	switch StoreCodeOf(err) {
	case StoreLeaseLost, StoreJobStopped, StoreChannelMissing:
		return true
	}
	return false
}
