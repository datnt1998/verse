package web

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/webauth"
)

// GAP-34 · 09 §10.3: bộ duyệt thư mục trong app, dùng khi không có hộp chọn của máy
// (truy cập từ xa / điện thoại / 501). Chỉ quản trị; mọi đường dẫn được giới hạn trong các Nơi
// (library.Places) sau khi theo symlink.
//
//	GET  /api/fs/places                       → [{id,label,path,icon}]
//	GET  /api/fs/list?path=                   → [{name,path,kind,items,writable}]
//	POST /api/fs/mkdir {parent,name}          → {name,path,display}
//	GET  /api/library/inspect?path=           → library_inspect.go
func (g *Gateway) serveFS(w http.ResponseWriter, r *http.Request, user webauth.User) bool {
	path := r.URL.Path
	if path == "/api/library/inspect" {
		g.serveLibraryInspect(w, r, user)
		return true
	}
	if !strings.HasPrefix(path, "/api/fs/") {
		return false
	}
	// The in-app folder browser only makes sense against a filesystem
	// library; on hosted there is no Places-backed tree to browse — 501.
	if g.libRepo != nil {
		jsonError(w, http.StatusNotImplemented, "duyệt thư mục không hỗ trợ trên kho dữ liệu Postgres")
		return true
	}
	if !isAdmin(user) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return true
	}
	switch {
	case path == "/api/fs/places" && r.Method == http.MethodGet:
		places := library.DiscoverPlaces()
		if places == nil {
			places = library.Places{}
		}
		writeJSON(w, http.StatusOK, places)
	case path == "/api/fs/list" && r.Method == http.MethodGet:
		g.fsList(w, r)
	case path == "/api/fs/mkdir" && r.Method == http.MethodPost:
		g.fsMkdir(w, r)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
	return true
}

// fsError ánh xạ lỗi giới hạn Nơi / không tồn tại.
func fsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, library.ErrOutsidePlaces):
		jsonError(w, http.StatusForbidden, "Không duyệt được thư mục này")
	case errors.Is(err, os.ErrNotExist):
		jsonError(w, http.StatusNotFound, "Không tìm thấy thư mục")
	case errors.Is(err, os.ErrPermission):
		jsonError(w, http.StatusForbidden, "Không có quyền đọc thư mục này")
	default:
		jsonError(w, http.StatusBadRequest, err.Error())
	}
}

func (g *Gateway) fsList(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("path")
	if raw == "" {
		jsonError(w, http.StatusBadRequest, "thiếu tham số path")
		return
	}
	places := library.DiscoverPlaces()
	real, err := places.Confine(raw)
	if err != nil {
		fsError(w, err)
		return
	}
	if st, err := os.Stat(real); err != nil || !st.IsDir() {
		jsonError(w, http.StatusBadRequest, "Đây không phải thư mục")
		return
	}
	folders, err := library.ListFolders(real, places)
	if err != nil {
		fsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

// validFolderName: tên một thư mục mới. Cho phép dấu tiếng Việt và khoảng trắng; chặn dấu phân cách,
// tên chấm (ẩn) và ký tự Windows không hợp lệ để thư mục dùng được trên mọi hệ điều hành.
func validFolderName(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || len(name) > 100 || !utf8.ValidString(name) {
		return false
	}
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.ContainsAny(name, `/\:*?"<>|`) {
		return false
	}
	for _, c := range name {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

func (g *Gateway) fsMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if !validFolderName(name) {
		jsonError(w, http.StatusBadRequest, "Tên thư mục không hợp lệ")
		return
	}
	places := library.DiscoverPlaces()
	parent, err := places.Confine(req.Parent)
	if err != nil {
		fsError(w, err)
		return
	}
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		jsonError(w, http.StatusBadRequest, "Đây không phải thư mục")
		return
	}
	target := filepath.Join(parent, name)
	// os.Mkdir (không phải MkdirAll) để tồn tại → ErrExist → 409, không âm thầm dùng lại thư mục khác.
	if err := os.Mkdir(target, 0o755); err != nil {
		switch {
		case errors.Is(err, os.ErrExist):
			jsonError(w, http.StatusConflict, "Đã có thư mục tên này")
		case errors.Is(err, os.ErrPermission):
			jsonError(w, http.StatusForbidden, "Không ghi được vào thư mục này")
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"name": name, "path": target, "display": places.Display(target)})
}
