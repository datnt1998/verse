package identity

import "time"

// TempPasswordTTL là thời hạn mật khẩu tạm do admin cấp/reset (quyết định 70
// mặc định đề xuất 24h). Mã là một lần: login đầu tiên đốt mã và tạo phiên
// restricted; phiên đó vẫn đổi được mật khẩu sau khi mã hết hạn.
const TempPasswordTTL = 24 * time.Hour

// TempPassword là bản ghi mật khẩu tạm gắn vào User (file v4, PG:
// temporary_passwords). Hash vẫn argon2id; plaintext chỉ tồn tại trong
// response một lần của admin. UsedAt nil + chưa hết hạn = mã còn dùng được
// để login; hash được giữ tới khi ChangePassword hoàn tất (cần để xác thực
// "mật khẩu hiện tại" của phiên restricted), sau đó bị xoá.
type TempPassword struct {
	CodeHash  string     `json:"codeHash"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedBy string     `json:"createdBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// usable báo mã còn đổi được phiên đăng nhập (chưa đốt, chưa hết hạn).
func (t *TempPassword) usable(now time.Time) bool {
	return t != nil && t.UsedAt == nil && t.ExpiresAt.After(now)
}
