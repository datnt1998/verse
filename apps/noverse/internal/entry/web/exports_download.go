package web

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/datnt1998/noverse/internal/host"
	"github.com/datnt1998/noverse/internal/library"
)

// serveExportDownloadFile 对应 GET /exports/download?file=<相对导出目录的路径>：
// 与 exports/file 相同的目录约束，但 txt/epub 以“友好名”下载
// （“<tên truyện> · Tập N.<đuôi>”），不把磁盘上的 slug 文件名暴露给用户。
// 授权已在 serveStories 完成（GET 需可读）；路径只能落在本故事的导出目录内。
func (g *Gateway) serveExportDownloadFile(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	if st.Dir == "" {
		jsonError(w, http.StatusNotImplemented, "xuất bản chưa hỗ trợ trên kho dữ liệu Postgres")
		return
	}
	_, exportsDir, err := libraryExports(st)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rel := r.URL.Query().Get("file")
	target, err := safeExportPath(exportsDir, rel)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "đường dẫn không hợp lệ")
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		// Kết quả lệnh /export trả đường dẫn tính từ thư mục truyện (“<mục xuất>/tap-1/x.epub”):
		// bỏ phần đầu rồi thử lại, vẫn trong thư mục xuất bản.
		if prefix, relErr := filepath.Rel(st.Dir, exportsDir); relErr == nil {
			if trimmed, ok := strings.CutPrefix(filepath.ToSlash(rel), filepath.ToSlash(prefix)+"/"); ok {
				if alt, altErr := safeExportPath(exportsDir, trimmed); altErr == nil {
					if altInfo, statErr := os.Stat(alt); statErr == nil {
						rel, target, info, err = trimmed, alt, altInfo, nil
					}
				}
			}
		}
	}
	if err != nil || info.IsDir() {
		jsonError(w, http.StatusNotFound, "không tìm thấy tệp")
		return
	}
	file, err := os.Open(target)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	name := info.Name()
	if format := exportFormatOf(name); format == "txt" || format == "epub" {
		volume := inferExportVolume(filepath.ToSlash(rel))
		if volume == nil {
			if ranges, err := sr.VolumeRanges(); err == nil && len(ranges) <= 1 {
				one := 1
				volume = &one
			}
		}
		name = friendlyExportName(exportStoryTitle(st, sr), volume, filepath.Ext(name))
	}
	w.Header().Set("Content-Disposition", contentDisposition(name))
	if ct := mime.TypeByExtension(filepath.Ext(info.Name())); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// exportStoryTitle: tên hiển thị = story.json.title → tên sách của engine → "Truyện".
func exportStoryTitle(st library.Story, sr *host.StoryReader) string {
	if f, err := library.LoadStoryFile(st.Dir); err == nil && f != nil && strings.TrimSpace(f.Title) != "" {
		return f.Title
	}
	if title := strings.TrimSpace(sr.BookTitle()); title != "" {
		return title
	}
	return "Truyện"
}

// friendlyExportName dựng “<tên truyện> · Tập N<ext>” (ext gồm dấu chấm). Phải khớp
// friendlyExportName ở web/src/screens/reader/logic.ts.
func friendlyExportName(title string, volume *int, ext string) string {
	title = sanitizeFileTitle(title)
	if title == "" {
		title = "Truyện"
	}
	if volume != nil && *volume > 0 {
		return fmt.Sprintf("%s · Tập %d%s", title, *volume, ext)
	}
	return title + ext
}

// sanitizeFileTitle bỏ ký tự không hợp lệ trong tên tệp và gộp khoảng trắng.
func sanitizeFileTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r) {
			return ' '
		}
		return r
	}, s)
	return strings.Trim(strings.Join(strings.Fields(s), " "), ". ")
}
