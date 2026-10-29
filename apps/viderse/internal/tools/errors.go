// Mã lỗi ổn định mà các bước pipeline ghi lại để UI/CLI giải thích và hẹn
// chạy lại, theo CloneError của bản cũ. Gói pipeline sở hữu danh sách đầy đủ
// (quota, awaiting_review, …); gói này chỉ khai phần chính nó sinh ra.
package tools

import (
	"errors"
	"time"
)

// Mã lỗi do gói tools sinh.
const (
	// ErrCodeToolMissing: binary bắt buộc do app quản vắng mặt — recovery chỉ
	// đường sửa (mở Cài đặt › Công cụ).
	ErrCodeToolMissing = "tool_missing"
	// ErrCodeInternal: lỗi không phân loại được.
	ErrCodeInternal = "internal_error"
)

// ToolError là lỗi bước pipeline do công cụ gây ra, đủ dữ liệu để ghi xuống
// hàng chờ: code ổn định, gợi ý sửa ngắn cho người dùng, và thời điểm được
// thử lại (retryable). retryAtSource "provider" chỉ dành cho mốc reset do
// chính dịch vụ báo; mặc định "worker".
type ToolError struct {
	Code          string
	Message       string
	Recovery      string
	RetryAt       *time.Time
	RetryAtSource string // "provider" | "worker" | ""
}

func (e *ToolError) Error() string { return e.Message }

// MissingToolError dựng lỗi tool_missing cho một công cụ chưa sẵn sàng.
func MissingToolError(message, recovery string) *ToolError {
	return &ToolError{Code: ErrCodeToolMissing, Message: message, Recovery: recovery}
}

// AsToolError gói một lỗi bất kỳ thành ToolError: giữ nguyên ToolError, xếp
// huỷ về cancelled theo pipeline, còn lại internal_error.
func AsToolError(err error) *ToolError {
	var te *ToolError
	if errors.As(err, &te) {
		return te
	}
	if errors.Is(err, ErrCancelled) {
		return &ToolError{Code: "cancelled", Message: err.Error()}
	}
	return &ToolError{Code: ErrCodeInternal, Message: err.Error()}
}
