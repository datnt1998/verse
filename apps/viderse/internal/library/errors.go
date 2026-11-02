package library

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Lỗi nghiệp vụ của Thư viện ở mặt API/CLI. Code là mã ổn định cho wire; các
// trường phụ mang dữ liệu mà UI cần để hiện đúng chỗ: FieldErrors cho lỗi
// từng trường recipe, Channels cho tên kênh đang dùng template, UsedBy cho
// template đang dùng prompt.
type Error struct {
	Code     string
	Message  string
	Recovery string
	Fields   []FieldError
	Channels []string
	UsedBy   []string
	RetryAt  *time.Time
}

func (e *Error) Error() string { return e.Message }

// FieldError là lỗi của một trường recipe: Field theo đường dẫn JSON
// ("scenes.images.count"), Message câu tiếng Việt, Recovery khoảng giá trị.
type FieldError struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Recovery string `json:"recovery,omitempty"`
}

// Mã lỗi mặt API của Thư viện.
const (
	CodeInvalidInput = "invalid_input"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeBuiltin      = "builtin"
	CodeInUse        = "in_use"
	CodeBadMedia     = "bad_media"
	CodeToolMissing  = "tool_missing"
	// CodeConfirmRequired: POST vẽ thử thiếu xác nhận đúng một ảnh.
	CodeConfirmRequired = "confirm_required"
	// CodeQuota: hạn mức ảnh của provider đã hết.
	CodeQuota = "quota"
)

// Lỗi sentinel của store: "không có hàng", "tên đã có" và "sha256 đã có".
// Store dịch lỗi Postgres sang các lỗi này để tầng nghiệp vụ không cần biết SQLSTATE.
var (
	ErrNotFound      = errors.New("không tìm thấy")
	ErrNameTaken     = errors.New("tên đã có")
	ErrOverlayExists = errors.New("lớp phủ đã có")
	ErrInUse         = errors.New("đang được dùng")
	ErrPromptRef     = errors.New("tham chiếu chỉ dẫn không còn hợp lệ")
	ErrBackdropRef   = errors.New("tham chiếu video nền không còn hợp lệ")
)

// isUniqueViolation báo lỗi ghi đụng ràng buộc duy nhất (store đã dịch).
func isUniqueViolation(err error) bool {
	return errors.Is(err, ErrNameTaken) || errors.Is(err, ErrOverlayExists)
}

// CodeOf trả mã lỗi của một *Error (rỗng với loại lỗi khác).
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// invalidInput dựng lỗi 400 không kèm lỗi trường.
func invalidInput(msg, recovery string) *Error {
	return &Error{Code: CodeInvalidInput, Message: msg, Recovery: recovery}
}

// invalidFields dựng lỗi 400 kèm danh sách trường sai.
func invalidFields(msg string, fields []FieldError) *Error {
	return &Error{Code: CodeInvalidInput, Message: msg, Recovery: fieldsRecovery(fields), Fields: fields}
}

// fieldsRecovery gộp câu phục hồi của các trường (một dòng, không trùng).
func fieldsRecovery(fields []FieldError) string {
	seen := make(map[string]bool, len(fields))
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if f.Recovery == "" || seen[f.Recovery] {
			continue
		}
		seen[f.Recovery] = true
		parts = append(parts, f.Recovery)
	}
	return strings.Join(parts, " · ")
}

// notFound dựng lỗi 404 cho một đối tượng.
func notFound(object string) *Error {
	return &Error{Code: CodeNotFound, Message: object + " không tồn tại", Recovery: "Tải lại trang rồi thử lại"}
}

// conflictRevision là lỗi 409 khi revision đã cũ.
func conflictRevision() *Error {
	return &Error{
		Code:     CodeConflict,
		Message:  "Mẫu video vừa được sửa ở nơi khác",
		Recovery: "Tải lại trang rồi thử lại",
	}
}

// rangeText là câu khoảng giá trị dùng trong recovery.
func rangeText(min, max float64) string {
	return fmt.Sprintf("từ %s đến %s", trimFloat(min), trimFloat(max))
}

// trimFloat in số không có đuôi .0 thừa.
func trimFloat(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}
