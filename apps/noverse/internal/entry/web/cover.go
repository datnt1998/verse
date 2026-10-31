package web

// Gói AF / 10-audio A7 + 06 §AF: GET S/cover?size= → bìa truyện (uploaded ở
// <project>/tai-san/bia, mới nhất) hoặc bìa tự tạo. PNG, có cache đĩa trong
// audio/bia/. size ∈ [96,512] hoặc 1400; volume= tuỳ chọn (mặc định 1).
//
// Content-Disposition 按 RFC 6266/5987：filename="…"（ASCII 无调号退路）+
// filename*=UTF-8''…。音频导出文件名含越南语 + " · "，旧客户端读 filename，
// 新客户端读 filename*。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// coverMinSize/coverMaxSize 与 §AF 的 "96–512 px" 一致；1400 是内嵌封面母版。
const (
	coverMinSize   = 96
	coverMaxSize   = 512
	coverEmbedSize = 1400
)

// serveStoryCover 处理 GET S/cover?size=[&volume=]。
func (g *Gateway) serveStoryCover(w http.ResponseWriter, r *http.Request, st library.Story, sr *host.StoryReader) {
	size := coverMaxSize
	if raw := strings.TrimSpace(r.URL.Query().Get("size")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || (v < coverMinSize || v > coverMaxSize) && v != coverEmbedSize {
			jsonError(w, http.StatusBadRequest, "size phải là số từ 96 đến 512 (hoặc 1400).")
			return
		}
		size = v
	}
	volume := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("volume")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			jsonError(w, http.StatusBadRequest, "volume phải là số nguyên dương.")
			return
		}
		volume = v
	}
	// Hosted trả cùng hình dạng: bìa upload (project_documents kind=asset) hoặc
	// bìa tự tạo, PNG kết xuất cache trong BlobStore + export_cache.
	if st.Dir == "" {
		g.hostedCover(w, r, st, sr, size, volume)
		return
	}
	layout, err := audio.ResolveLayout(st)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	title, short := storyFileNames(st)
	if title == "" && sr != nil {
		title = sr.BookTitle() // bookTitle 优先于 slug（A7 命名规则）
	}
	path, err := layout.CoverPNG(coverShortName(short, title, st), volume, size)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không dựng được bìa.")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	// key 已含内容指纹 → 强缓存安全；If-Modified-Since 由 ServeContent 处理。
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// coverShortName 是封面/文件名用的 {Tên ngắn}：shortTitle 有就用，
// 否则按 A7 规则截 title（词边界 ≤32），再退回 slug。
func coverShortName(shortTitle, title string, st library.Story) string {
	name := audio.ShortName(shortTitle, title)
	if name == "" {
		name = st.Story
	}
	return name
}

// contentDisposition 生成 `attachment; filename="<ascii>"; filename*=UTF-8”<quoted>`。
// ASCII 退路：NFD 去声调、đ→d、去非 ASCII、压空白；quoted 走 RFC 5987
// percent-encoding（url.PathEscape 的字符集正好覆盖 attr-char 的补集）。
func contentDisposition(name string) string {
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiFilename(name), url.PathEscape(name))
}

// asciiFilename 把任意显示名转 ASCII 安全文件名（越南语 → 无调号）。
// " · " 在退路里变成 " - "；结果为空时兜底 "noverse"。
func asciiFilename(name string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range norm.NFD.String(name) {
		switch {
		case r == 'đ':
			b.WriteByte('d')
			lastSpace = false
		case r == 'Đ':
			b.WriteByte('D')
			lastSpace = false
		case r == '·' || r == '—':
			b.WriteString(" - ")
			lastSpace = false
		case r == '–' || r == '-':
			b.WriteByte('-')
			lastSpace = false
		case r >= '!' && r <= '~' && r != '"' && r != '\\':
			if r == ' ' {
				if lastSpace || b.Len() == 0 {
					continue
				}
				lastSpace = true
			} else {
				lastSpace = false
			}
			b.WriteByte(byte(r))
		case r == ' ':
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
		default:
			// 去声调后仍非 ASCII → 丢。
		}
	}
	out := strings.TrimSpace(b.String())
	for strings.Contains(out, "- -") {
		out = strings.ReplaceAll(out, "- -", "-")
	}
	if out == "" || !utf8.ValidString(out) {
		return "noverse"
	}
	return out
}

func extLower(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return strings.ToLower(name[i:])
	}
	return ""
}

// ── Tên tệp tải về (A7) ──

// downloadFileName 决定 exports/file?path= 下载名：
//   - 新导出（文件名已含 " · "）→ 直接用磁盘名；
//   - 旧导出（tap-… 命名）→ 按 A7 规则重建 "{Tên ngắn} · Tập V · …"；
//   - 章标题从 committed 正文/plan 取；取不到就省略该段。
func downloadFileName(st library.Story, sr *host.StoryReader, target, base string) string {
	if strings.Contains(base, " · ") {
		return base
	}
	title, shortTitle := storyFileNames(st)
	short := coverShortName(shortTitle, title, st)
	ext := extLower(base)
	chapterTitle := func(n int) string {
		if sr != nil {
			if ct, err := sr.ChapterText(n, "committed"); err == nil && strings.TrimSpace(ct.Title) != "" {
				return strings.TrimSpace(ct.Title)
			}
			if plan, err := sr.ChapterPlan(n); err == nil && plan != nil && strings.TrimSpace(plan.Title) != "" {
				return strings.TrimSpace(plan.Title)
			}
		}
		return ""
	}
	// tap-V-chuong-NN.ext / tap-V-ban-thu-chuong-N.ext（旧约定，含 chuong*/ 内的文件）。
	if m := legacyChapterName.FindStringSubmatch(base); m != nil {
		volume, _ := strconv.Atoi(m[1])
		chapter, _ := strconv.Atoi(m[3])
		trial := strings.HasPrefix(m[2], "ban-thu")
		name := fmt.Sprintf("%s · Tập %d · Chương %02d", short, volume, chapter)
		if t := chapterTitle(chapter); t != "" {
			name += " · " + t
		}
		if trial {
			name += " (bản thử)"
		}
		return name + ext
	}
	// tap-V(-ch…)(-stamp).zip → "{short} · Tập V · N chương.zip"。
	if m := legacyZipName.FindStringSubmatch(base); m != nil {
		volume, _ := strconv.Atoi(m[1])
		count := legacyChapterCount(filepath.Dir(target))
		if count <= 0 {
			count = metaChapterCount(filepath.Dir(target))
		}
		if count <= 0 {
			return base
		}
		return fmt.Sprintf("%s · Tập %d · %d chương.zip", short, volume, count)
	}
	// "{title} — Tập V(-ch…)(-stamp).ext"（旧整卷名）→ 保留 -ch 后缀（新规则同款）。
	if m := legacyVolumeName.FindStringSubmatch(base); m != nil {
		volume, _ := strconv.Atoi(m[2])
		name := fmt.Sprintf("%s · Tập %d%s%s", short, volume, m[3], m[4])
		return name + ext
	}
	return base
}

var (
	legacyChapterName = regexp.MustCompile(`^tap-(\d+)-(chuong|ban-thu-chuong)-(\d+)\.[a-z0-9]+$`)
	legacyZipName     = regexp.MustCompile(`^tap-(\d+)(?:-ch[\d_-]+)?(?:-\d{8}-\d{6})?\.zip$`)
	legacyVolumeName  = regexp.MustCompile(`^(.*) — Tập (\d+)(-ch[\d_-]+)?(-\d{8}-\d{6})?\.(mp3|m4b|wav)$`)
)

// legacyChapterCount 数同目录下 chuong*/ 里的音频文件（旧 zip 的 N）。
func legacyChapterCount(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "chuong") {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() {
				count++
			}
		}
	}
	return count
}

// metaChapterCount 读 .audio-export.json 的 chapterCount（目录内没有章文件时兜底）。
func metaChapterCount(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, ".audio-export.json"))
	if err != nil {
		return 0
	}
	var meta struct {
		ChapterCount int `json:"chapterCount"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return 0
	}
	return meta.ChapterCount
}
