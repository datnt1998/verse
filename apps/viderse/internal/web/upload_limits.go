package web

import (
	"net/http"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/uploadlimit"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

// UploadLimitsWire là trần tải lên đang có hiệu lực, để SPA từ chối tệp quá
// lớn trước khi gửi. Không có nó, tệp vượt giới hạn body của Cloudflare bị
// chặn ở edge bằng trang 413 HTML — người dùng không đọc được lý do.
type UploadLimitsWire struct {
	// Hosted = máy chủ phục vụ người khác qua PUBLIC_URL (có edge ở trước).
	Hosted bool `json:"hosted"`
	// Limits theo loại tệp, tính bằng byte.
	Limits map[string]int64 `json:"limits"`
	// Message là câu từ chối dùng chung khi vượt trần công khai.
	Message string `json:"message"`
}

// uploadLimits trả trần đang áp dụng. Giá trị lấy từ chính các gói thực thi
// giới hạn (editor/library), nên UI và máy chủ không thể lệch nhau.
func uploadLimits() UploadLimitsWire {
	return UploadLimitsWire{
		Hosted: uploadlimit.Hosted(),
		Limits: map[string]int64{
			"video":    editor.MediaLimit("video"),
			"audio":    editor.MediaLimit("audio"),
			"image":    editor.MediaLimit("image"),
			"overlay":  library.OverlayLimit(),
			"template": library.TemplateBundleLimit(),
		},
		Message: uploadlimit.TooLargeMessage(uploadlimit.PublicMaxBytes),
	}
}

// MountUploadLimits đăng ký GET /api/upload-limits.
func MountUploadLimits(g *Gateway) {
	g.Handle(http.MethodGet, "/api/upload-limits", func(w http.ResponseWriter, r *http.Request, user identity.User) {
		w.Header().Set("Cache-Control", "private, max-age=300")
		webapp.WriteJSON(w, http.StatusOK, uploadLimits())
	})
}
