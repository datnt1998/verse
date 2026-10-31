package web

// Điểm nối duy nhất giữa Thư viện và Video nền trên tiền tố /api/templates/:
// route GET /api/templates/{id}/backdrop/mix do mảng Video nền sở hữu (nó đọc
// recipe bằng SQL của mình và dùng hàm chia clip của pickBackdropSources),
// nhưng router của tiền tố nằm ở mảng Thư viện. Tách ở đây để chỉ một chỗ
// khai báo tên, hai mảng không định nghĩa trùng.

import (
	"net/http"
	"strings"
)

// TemplateBackdropMix là mặt phục vụ route trộn nền của một template — hiện
// thực là service của mảng Video nền trong cmd/viderse.
type TemplateBackdropMix interface {
	// ServeMix trả GET /api/templates/{id}/backdrop/mix?lengthSec=N với
	// templateID đã tách; tự ghi status và body.
	ServeMix(w http.ResponseWriter, r *http.Request, templateID string)
}

// IsTemplateBackdropMix báo path là route trộn nền của template.
func IsTemplateBackdropMix(path string) bool {
	_, ok := parseTemplateMixPath(path)
	return ok
}

// ServeTemplateBackdropMix chuyển tiếp cho mảng Video nền; mix nil thì 404
// (route chưa được ráp, không phải lỗi 500).
func ServeTemplateBackdropMix(w http.ResponseWriter, r *http.Request, mix TemplateBackdropMix) {
	id, ok := parseTemplateMixPath(r.URL.Path)
	if !ok || mix == nil {
		http.Error(w, `{"error":"không tìm thấy"}`, http.StatusNotFound)
		return
	}
	mix.ServeMix(w, r, id)
}

// parseTemplateMixPath tách id từ /api/templates/{id}/backdrop/mix.
func parseTemplateMixPath(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/templates/")
	if !ok {
		return "", false
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] != "backdrop" || parts[2] != "mix" {
		return "", false
	}
	return parts[0], true
}
