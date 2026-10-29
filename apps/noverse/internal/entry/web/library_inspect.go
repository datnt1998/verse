package web

import (
	"errors"
	"net/http"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

// GAP-34 · 09 §10.4: GET /api/library/inspect?path= → library.Inspection.
// Chỉ quản trị. Không giới hạn trong các Nơi: hộp chọn của máy có thể trả thư mục bất kỳ,
// và kết quả chỉ gồm loại + số đếm + chuỗi "Nơi › Thư mục" (không lộ đường dẫn tuyệt đối).
func (g *Gateway) serveLibraryInspect(w http.ResponseWriter, r *http.Request, user identity.User) {
	// Folder inspection is a desktop picker helper; hosted has no filesystem
	// library to inspect — explicit 501.
	if g.libRepo != nil {
		jsonError(w, http.StatusNotImplemented, "duyệt thư mục không hỗ trợ trên kho dữ liệu Postgres")
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusNotFound, "không tìm thấy")
		return
	}
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		jsonError(w, http.StatusBadRequest, "thiếu tham số path")
		return
	}
	writeJSON(w, http.StatusOK, library.Inspect(path, library.DiscoverPlaces()))
}

// rootDisplay là "Nơi › Thư mục" của thư viện hiện tại; "" khi chưa chọn nơi lưu.
func (g *Gateway) rootDisplay() string {
	root := g.RootPath()
	if root == "" {
		return ""
	}
	return library.DiscoverPlaces().Display(root)
}

// writeUnconfiguredLibrary trả GET /api/library khi cài mới chưa chọn nơi lưu: cùng hình dạng
// với bản đã cấu hình (config mặc định, danh sách rỗng) kèm configured=false.
func (g *Gateway) writeUnconfiguredLibrary(w http.ResponseWriter, _ identity.User) {
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":  false,
		"rootDisplay": "",
		"rootPath":    "",
		"config":      library.DefaultConfig(),
		"projects":    []projectSummary{},
		"candidates":  []string{},
	})
}

// libraryError answers library-open failures: not having chosen a storage
// place is a recoverable 409; everything else follows the shared policy.
func libraryError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoLibrary) {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	jsonMappedError(w, err)
}
