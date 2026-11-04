package web

import (
	"net/http"
	"strings"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
	"github.com/datnt1998/verse/platform/webguard"
)

const adminOnlyMessage = "Chỉ quản trị viên được thay đổi cấu hình của máy chủ"

// Handle đăng ký route của app và đặt cổng quản trị CHUNG cho mọi route: một
// request tới bề mặt cấu hình toàn máy (machineAdminOnly) bởi người không phải
// quản trị viên bị từ chối 403 trước khi vào handler. Cổng nằm ở đây — không
// ở từng handler — nên route mới đăng ký sau này dưới các đường dẫn đó tự được
// bảo vệ, và route đăng ký trước tiền tố rộng hơn cũng không lách được.
func (g *Gateway) Handle(method, pattern string, h webapp.HandlerFunc) {
	g.Server.Handle(method, pattern, func(w http.ResponseWriter, r *http.Request, user identity.User) {
		if machineAdminOnly(r.Method, r.URL.Path) && !requireAdmin(w, user) {
			return
		}
		h(w, r, user)
	})
}

// SetProxyConfig cài ranh giới proxy cho server và ghi nhớ máy chủ có đang
// phục vụ người khác (PUBLIC_URL) hay không.
func (g *Gateway) SetProxyConfig(cfg *webguard.Config) {
	g.Server.SetProxyConfig(cfg)
	g.hosted = cfg.Hosted()
}

// Hosted báo máy chủ đang phục vụ người khác qua PUBLIC_URL (không phải app
// một chủ trên máy cá nhân).
func (g *Gateway) Hosted() bool { return g != nil && g.hosted }

// isAdmin báo user là quản trị viên.
func isAdmin(user identity.User) bool { return user.Role == identity.RoleAdmin }

// requireAdmin trả true cho quản trị viên; ngược lại ghi 403 và trả false.
func requireAdmin(w http.ResponseWriter, user identity.User) bool {
	if isAdmin(user) {
		return true
	}
	w.Header().Set("Cache-Control", "private, no-store")
	webapp.Error(w, http.StatusForbidden, adminOnlyMessage)
	return false
}

// machineAdminOnly liệt kê bề mặt cấu hình/credential toàn máy mà thành viên
// không được ghi (và vài đường đọc lộ bí mật của chủ). Đọc phục vụ công việc
// của thành viên (danh sách provider, model, công cụ, trạng thái) vẫn mở.
//
//   - AI: provider và gán agent — ghi; luồng đăng nhập (trạng thái chứa URL
//     uỷ quyền) — cả đọc lẫn ghi.
//   - Tài khoản nền tảng: mọi thứ dưới /api/platforms/ (cookie, hồ sơ Chrome
//     kèm email, QR) trừ POST /api/platforms/requests là yêu cầu của thành viên.
//   - Công cụ (cài/huỷ/kiểm), bước Chuẩn bị máy, dọn dữ liệu, thao tác thư mục
//     lưu trữ (kể cả duyệt thư mục của máy chủ), cài đặt thông báo — ghi.
//
// /api/setup, /api/setup/verify và /api/setup/admin (tạo quản trị viên đầu
// tiên) do platform/webapp phục vụ trước phiên đăng nhập nên không qua cổng này.
func machineAdminOnly(method, path string) bool {
	write := method != http.MethodGet && method != http.MethodHead
	switch {
	case path == "/api/ai/providers" || path == "/api/ai/agents":
		return write
	case strings.HasPrefix(path, "/api/ai/providers/"):
		if write {
			return true
		}
		rest := strings.Split(strings.Trim(strings.TrimPrefix(path, "/api/ai/providers/"), "/"), "/")
		return len(rest) >= 2 && rest[1] == "login"
	case path == "/api/platforms/requests":
		return false
	case strings.HasPrefix(path, "/api/platforms/"):
		return true
	case path == "/api/tools" || strings.HasPrefix(path, "/api/tools/"):
		return write
	case strings.HasPrefix(path, "/api/setup/"):
		return write
	case strings.HasPrefix(path, "/api/storage/cleanup/"):
		return write
	case path == "/api/storage/browse" || path == "/api/storage/reclaim":
		return true // liệt kê thư mục / thư mục cũ trên đĩa của máy chủ
	case strings.HasPrefix(path, "/api/storage/") && !strings.HasPrefix(path, "/api/storage/server"):
		return write
	case path == "/api/notifications/settings":
		return write
	}
	return false
}
