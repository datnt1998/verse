package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// maxAssetUpload 是单个 Tư liệu 文件上限（与 giọng mẫu 上传一致）。
const maxAssetUpload = 64 << 20

// assetKinds：Tư liệu 的两种上传类型 → 允许的扩展名。giọng mẫu 走 /voices。
var assetKinds = map[string][]string{
	"cover": {".jpg", ".jpeg", ".png", ".webp"},
	"music": {".mp3", ".m4a", ".wav", ".ogg", ".flac"},
}

// uploadAsset 处理 POST /api/projects/:p/assets（multipart: kind=cover|music, file）。
// 文件落在 audio.Layout 的 CoverDir / MusicDir，重名自动加后缀，不覆盖已有文件。
func (g *Gateway) uploadAsset(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, p string) {
	if !g.requireProject(w, lib, r.Context(), actorOf(user), user, p) {
		return
	}
	if p == library.LegacyProjectID {
		jsonError(w, http.StatusConflict, "Dự án ẩn danh không có tư liệu")
		return
	}
	if !user.CanCollaborateProject(p) {
		jsonError(w, http.StatusForbidden, "Không có quyền")
		return
	}
	ref, ok := lib.ResolveProject(r.Context(), actorOf(user), p)
	if !ok || ref.Dir == "" {
		jsonError(w, http.StatusNotFound, "không tìm thấy dự án")
		return
	}
	layout, err := projectAudioLayout(g.RootPath(), ref.Dir)
	if err != nil {
		audioError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAssetUpload+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			jsonError(w, http.StatusRequestEntityTooLarge, "Tệp quá lớn (tối đa 64 MB).")
			return
		}
		jsonError(w, http.StatusBadRequest, "Không đọc được tệp gửi lên.")
		return
	}
	kind := r.FormValue("kind")
	exts, known := assetKinds[kind]
	if !known {
		jsonError(w, http.StatusBadRequest, "Loại tư liệu không hợp lệ.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Thiếu tệp.")
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := false
	for _, e := range exts {
		if e == ext {
			allowed = true
			break
		}
	}
	if !allowed {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("Định dạng chưa hỗ trợ — chọn tệp %s.", strings.TrimPrefix(strings.Join(exts, ", "), ".")))
		return
	}
	dir := layout.CoverDir()
	if kind == "music" {
		dir = layout.MusicDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		audioError(w, err)
		return
	}
	out, name, err := createUnique(dir, assetBaseName(header.Filename), ext)
	if err != nil {
		audioError(w, err)
		return
	}
	written, err := io.Copy(out, io.LimitReader(file, maxAssetUpload+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && written > maxAssetUpload {
		err = errAssetTooLarge
	}
	if err != nil {
		_ = os.Remove(filepath.Join(dir, name))
		if errors.Is(err, errAssetTooLarge) {
			jsonError(w, http.StatusRequestEntityTooLarge, "Tệp quá lớn (tối đa 64 MB).")
			return
		}
		audioError(w, err)
		return
	}
	rel, err := filepath.Rel(layout.AssetsDir, filepath.Join(dir, name))
	if err != nil {
		audioError(w, err)
		return
	}
	now := time.Now().UTC()
	g.projectChanged(p)
	writeJSON(w, http.StatusCreated, library.Asset{Path: filepath.ToSlash(rel), Kind: "file", Size: written, UpdatedAt: &now})
}

var errAssetTooLarge = errors.New("asset too large")

// assetBaseName 去掉路径与扩展名，并把不安全字符换成 "-"。
func assetBaseName(filename string) string {
	base := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(filename, "\\", "/")), filepath.Ext(filename))
	base = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(base))
	base = strings.Trim(base, ".- ")
	if base == "" {
		return "tep"
	}
	if r := []rune(base); len(r) > 80 {
		base = string(r[:80])
	}
	return base
}

// createUnique 以 O_EXCL 创建 <dir>/<base><ext>，重名则依次尝试 <base>-2 … 。
func createUnique(dir, base, ext string) (*os.File, string, error) {
	for i := 1; i < 1000; i++ {
		name := base + ext
		if i > 1 {
			name = fmt.Sprintf("%s-%d%s", base, i, ext)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("không tạo được tên tệp mới trong %s", dir)
}
