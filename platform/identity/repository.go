package identity

import (
	"errors"
	"time"
)

// Repository là hợp đồng tài khoản mà mọi backend (file web-accounts.json,
// Postgres) phải cung cấp với cùng ngữ nghĩa quan sát được. Store là facade
// duy nhất của gateway: NewStore chọn file repository, NewFromRepository gắn
// một backend khác (postgres) mà không đổi chữ ký NewGateway hay caller.
//
// Hợp đồng chung hai backend phải giữ:
//   - Login/Pair/Recover trả ErrInvalidCredentials/ErrInvalidPairing/
//     ErrInvalidRecovery một cách đồng nhất; user không tồn tại vẫn trả giá
//     argon2 đầy đủ (không enumeration qua thời gian).
//   - Temporary password một lần, TTL TempPasswordTTL; phiên restricted chỉ
//     được /api/me*, /api/logout cho đến khi ChangePassword thành công.
//   - Đổi/reset mật khẩu, khoá tài khoản và RevokeAll thu hồi MỌI phiên của
//     tài khoản, kể cả phiên đang dùng; phiên hợp lệ kế tiếp chỉ có sau khi
//     đăng nhập lại.
//   - Device token (thiết bị đã đăng nhập thành công) chỉ đổi bucket limiter
//     của lần login sau: lưu SHA-256, không tự cấp phiên, và bị xoá ở đúng
//     những luồng thu hồi phiên ở trên.
//   - Tối thiểu một admin active và đúng một owner active mỗi project:
//     mutation để lại 0 admin → ErrLastAdmin; 0 owner → ErrLastOwner; mở
//     thêm owner thứ hai → ErrOwnerTaken. Chuyển owner chỉ qua
//     TransferProjectOwner (một giao dịch, owner cũ về collab).
//   - Audit ghi trong cùng giao dịch với mutation; no-op không thêm bản ghi.
type Repository interface {
	HasUsers() (bool, error)
	Users() ([]UserListView, error)
	CreateUser(in NewUser) (UserView, error)
	CreateFirstAdmin(in NewUser, device, ip string) (NewSession, error)
	FirstUserCreatedAt() (at time.Time, ok bool, err error)
	UpdateUser(username string, patch UserPatch) (UserView, error)
	DeleteUser(username string, context ...AuditContext) error
	MarkOnboarded(username string) (UserView, error)
	Login(username, password, device, ip string) (NewSession, error)
	Pair(code, device, ip string) (NewSession, error)
	NewPairing(username, createdBy string) (string, time.Time, error)
	Authenticate(cookie string) (User, Session, error)
	Sessions(username, currentID string) ([]Session, error)
	RevokeSession(id string, allowed func(owner string) bool, context ...AuditContext) error
	RevokeAll(username string, context AuditContext) error
	RevokeOtherSessions(username, currentID string, context AuditContext) error
	ChangePassword(username, current, next string, context ...AuditContext) error
	IssueRecovery(username string, rotate bool) (string, time.Time, error)
	RecoveryStatus(username string) (RecoveryStatus, error)
	Recover(username, code, newPassword, device, ip string) (NewSession, []string, error)
	Audit(query AuditQuery) (AuditPage, error)
	RecordSecurityFailure(username string) error

	// Known devices of an account: long-lived tokens that only relax the login
	// limiter bucket. Only the SHA-256 digest is stored (never the token) and
	// the token never authenticates anything by itself. Every revocation flow
	// that ends sessions (password change, recovery, disable, delete,
	// revoke-all) also forgets these rows.
	SaveDeviceToken(username, tokenHash string, expiresAt time.Time) error
	DeviceTokenValid(username, tokenHash string) (bool, error)
	RevokeDeviceTokens(username string) error

	// Quản lý membership phạm vi project (owner hoặc admin).
	// SetProjectMember chỉ nhận ProjectCollab/ProjectView; owner được gán qua
	// TransferProjectOwner để giữ invariant đúng một owner.
	ProjectMembers(project string) ([]ProjectMember, error)
	SetProjectMember(project, username string, role ProjectRole, actor *AuditActor) (UserView, error)
	RemoveProjectMember(project, username string, actor *AuditActor) error
	TransferProjectOwner(project, username string, actor *AuditActor) error
}

// ProjectMember là một dòng membership hiệu lực của project.
type ProjectMember struct {
	Username string      `json:"username"`
	Name     string      `json:"name"`
	Role     ProjectRole `json:"role"`
	Disabled bool        `json:"disabled"`
}

var (
	ErrLastOwner  = errors.New("Phải còn ít nhất một Chủ dự án đang hoạt động; hãy chuyển quyền trước.")
	ErrOwnerTaken = errors.New("Dự án đã có Chủ dự án; hãy chuyển quyền thay vì thêm chủ.")
)

// NewFromRepository tạo facade Store trên một backend ngoài (postgres), với
// định danh project của app ở opts (xem Options). Các phương thức Store chuyển
// tiếp 1:1; Path/SetProjectSource là tiện ích riêng của file backend và trả
// về giá trị mô tả cho postgres.
func NewFromRepository(repo Repository, opts Options) *Store {
	return &Store{repo: repo, opts: opts}
}
