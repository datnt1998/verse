package channels

import (
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// ResolveAutonomy chuẩn hoá một Mức tự chủ client gửi: chỉ gửi `preset` (không
// dòng nào) thì nở thành mức sẵn đó; còn lại kiểm từng dòng rồi để server suy
// ra preset — client không quyết định preset. Ngưỡng 0 nghĩa là chưa chọn → 85.
func ResolveAutonomy(in domain.Autonomy) (domain.Autonomy, error) {
	if in.Pick == "" && in.Approve.Mode == "" && in.OnError == "" && in.BelowQuality == "" {
		p, ok := domain.PresetAutonomy(in.Preset)
		if !ok {
			return domain.Autonomy{}, &Error{Code: CodeInvalidInput, Message: "Mức tự chủ không hợp lệ.",
				Recovery: "Chọn Cẩn thận, Cân bằng, Tự chạy hoặc Tuỳ chỉnh."}
		}
		return p, nil
	}
	if in.Approve.Min == 0 {
		in.Approve.Min = domain.ApproveMinDefault
	}
	if err := in.Validate(); err != nil {
		return domain.Autonomy{}, autonomyInvalid(err)
	}
	return in.Normalize(), nil
}

func autonomyInvalid(err error) error {
	var fe *domain.AutonomyFieldError
	if errors.As(err, &fe) {
		return &Error{Code: CodeInvalidInput, Message: fe.Message, Field: fe.Field}
	}
	return errInvalid("%s", err.Error())
}

func errConfirmRequired(next domain.Autonomy) *Error {
	msg := "Mức Tự chạy giao thêm việc cho máy — cần bạn xác nhận."
	if next.Approve.Mode == domain.ApproveAll {
		msg = "Tự duyệt hết không xem điểm ảnh — cần bạn xác nhận."
	}
	return &Error{Code: CodeConfirmRequired, Message: msg,
		Recovery: "Xác nhận rồi lưu lại; máy vẫn không tự đăng, tự xoá hay đổi template."}
}

func validPriority(p string) error {
	switch p {
	case domain.PrioritySteady, domain.PriorityViews:
		return nil
	}
	return &Error{Code: CodeInvalidInput, Message: "Ưu tiên kênh phải là đều tay hoặc ưu tiên lượt xem.", Field: "priority"}
}
