package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/utils"
	"github.com/datnt1998/verse/platform/identity"
)

// styleSamplesDir 是 /simulate 读取语料的目录（Host.Simulate 的默认源目录）。
// 目录名只存在于服务端：响应里只有 mẫu 自己的相对路径，不出现该名字。
const styleSamplesDir = "simulate"

// maxStyleSamples 是 GAP-35 的硬上限：20 份 văn mẫu。
const maxStyleSamples = 20

// maxStyleSampleBytes 是单份 văn mẫu的上传上限（JSON 路径由 decodeJSON 的 1 MiB 限制）。
const maxStyleSampleBytes = 4 << 20

// StyleSample 对应 GAP-35 的 `samples[]` 项。Chars 按“chữ”（domain.WordCount）计，
// 与 /simulate 完成卡和 “Văn phong của bạn” 卡片的数字同一口径。
type StyleSample struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Chars   int    `json:"chars"`
	AddedAt string `json:"addedAt"`
}

// StyleSampleTotals 对应 `total:{count, chars}`。
type StyleSampleTotals struct {
	Count int `json:"count"`
	Chars int `json:"chars"`
}

// serveStyleSamples 处理故事路径下的 /style-samples（GAP-35）：
//
//	GET    …/style-samples        → {samples, total}
//	POST   …/style-samples        → 201 sample（JSON {title, text} 或 multipart file）
//	DELETE …/style-samples/{id}   → 204
//
// 仅故事的写者可用（连 GET 也要求可写，语料喂给 /simulate）——这条特例写在
// serveStories 的 storyAccessAllowed 里，未授权一律 404。不再在此重复检查。
// 语料与 /simulate 共用同一目录，所以网页加进来的 văn mẫu立刻能被 /simulate 读到。
func (g *Gateway) serveStyleSamples(w http.ResponseWriter, r *http.Request, user identity.User, st library.Story, tail string) {
	if st.Dir == "" {
		g.serveHostedStyleSamples(w, r, user, st, tail)
		return
	}
	dir := filepath.Join(st.Dir, styleSamplesDir)
	switch {
	case tail == "" && r.Method == http.MethodGet:
		samples, err := listStyleSamples(dir)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "Không đọc được danh sách văn mẫu")
			return
		}
		total := StyleSampleTotals{Count: len(samples)}
		for _, s := range samples {
			total.Chars += s.Chars
		}
		writeJSON(w, http.StatusOK, map[string]any{"samples": samples, "total": total})
	case tail == "" && r.Method == http.MethodPost:
		g.createStyleSample(w, r, st, dir)
	case tail != "" && r.Method == http.MethodDelete:
		g.deleteStyleSample(w, r, st, dir, tail)
	default:
		jsonError(w, http.StatusNotFound, "không tìm thấy")
	}
}

// listStyleSamples 汇总语料目录下 /simulate 会读到的全部文件（.txt/.md/.markdown）。
// id 是相对该目录的路径；标题是文件名去掉扩展名——用户看到的只有标题。
func listStyleSamples(dir string) ([]StyleSample, error) {
	samples := []StyleSample{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			if path != dir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || !isStyleSourceName(path) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		samples = append(samples, StyleSample{
			ID:      filepath.ToSlash(rel),
			Title:   styleSampleTitle(entry.Name()),
			Chars:   domain.WordCount(utils.DecodeText(data)),
			AddedAt: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return samples, nil
		}
		return nil, err
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].ID < samples[j].ID })
	return samples, nil
}

// isStyleSourceName 与 sim 扫描器同一套扩展名（internal/host/sim/scanner.go）。
func isStyleSourceName(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".markdown":
		return true
	default:
		return false
	}
}

// styleSampleTitle 把文件名还原成标题：去掉扩展名。
func styleSampleTitle(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// createStyleSample 处理 POST：JSON {title, text} 或 multipart（file，可选 title）。
func (g *Gateway) createStyleSample(w http.ResponseWriter, r *http.Request, st library.Story, dir string) {
	samples, err := listStyleSamples(dir)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không đọc được danh sách văn mẫu")
		return
	}
	if len(samples) >= maxStyleSamples {
		jsonError(w, http.StatusConflict, fmt.Sprintf("Đã có %d văn mẫu — xoá bớt trước khi thêm.", maxStyleSamples))
		return
	}

	title, text, ok := readStyleSampleBody(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(text) == "" {
		jsonError(w, http.StatusBadRequest, "Cần dán nội dung văn mẫu")
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, "Không lưu được văn mẫu")
		return
	}
	file, name, err := createUnique(dir, styleSampleBase(title), ".txt")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Không lưu được văn mẫu")
		return
	}
	_, writeErr := io.WriteString(file, text)
	if cerr := file.Close(); writeErr == nil {
		writeErr = cerr
	}
	if writeErr != nil {
		_ = os.Remove(filepath.Join(dir, name))
		jsonError(w, http.StatusInternalServerError, "Không lưu được văn mẫu")
		return
	}
	// 并发添加可能同时越过上面的上限检查：写完后复核，超出就回滚。
	if after, err := listStyleSamples(dir); err == nil && len(after) > maxStyleSamples {
		_ = os.Remove(filepath.Join(dir, name))
		jsonError(w, http.StatusConflict, fmt.Sprintf("Đã có %d văn mẫu — xoá bớt trước khi thêm.", maxStyleSamples))
		return
	}
	info, statErr := os.Stat(filepath.Join(dir, name))
	addedAt := time.Now().UTC().Format(time.RFC3339)
	if statErr == nil {
		addedAt = info.ModTime().Format(time.RFC3339)
	}
	g.storyChanged(st.Project, st.Story, "style")
	writeJSON(w, http.StatusCreated, StyleSample{
		ID:      name,
		Title:   styleSampleTitle(name),
		Chars:   domain.WordCount(utils.DecodeText([]byte(text))),
		AddedAt: addedAt,
	})
}

// readStyleSampleBody 读取两种提交形式；失败时已写出 400。
func readStyleSampleBody(w http.ResponseWriter, r *http.Request) (title, text string, ok bool) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxStyleSampleBytes+(1<<20))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			jsonError(w, http.StatusBadRequest, "Không đọc được tệp gửi lên.")
			return "", "", false
		}
		title = strings.TrimSpace(r.FormValue("title"))
		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, http.StatusBadRequest, "Thiếu tệp.")
			return "", "", false
		}
		defer file.Close()
		if !isStyleSourceName(header.Filename) {
			jsonError(w, http.StatusBadRequest, "Chỉ nhận tệp .txt, .md hoặc .markdown")
			return "", "", false
		}
		data, err := io.ReadAll(io.LimitReader(file, maxStyleSampleBytes+1))
		if err != nil {
			jsonError(w, http.StatusBadRequest, "Không đọc được tệp gửi lên.")
			return "", "", false
		}
		if len(data) > maxStyleSampleBytes {
			jsonError(w, http.StatusRequestEntityTooLarge, "Tệp quá lớn (tối đa 4 MB).")
			return "", "", false
		}
		if title == "" {
			title = styleSampleTitle(filepath.Base(header.Filename))
		}
		return title, string(data), true
	}
	var body struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if !decodeJSON(w, r, &body) {
		return "", "", false
	}
	return body.Title, body.Text, true
}

// deleteStyleSample 处理 DELETE …/style-samples/{id}：id 必须是语料目录内的相对路径。
func (g *Gateway) deleteStyleSample(w http.ResponseWriter, r *http.Request, st library.Story, dir, id string) {
	target, err := styleSamplePath(dir, id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "không tìm thấy văn mẫu")
		return
	}
	if err := os.Remove(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			jsonError(w, http.StatusNotFound, "không tìm thấy văn mẫu")
			return
		}
		jsonError(w, http.StatusInternalServerError, "Không xoá được văn mẫu")
		return
	}
	g.storyChanged(st.Project, st.Story, "style")
	w.WriteHeader(http.StatusNoContent)
}

// styleSamplePath 把 id 解析到语料目录内：拒绝绝对路径、.. 段与不支持的扩展名。
func styleSamplePath(dir, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || filepath.IsAbs(id) {
		return "", errors.New("invalid path")
	}
	for _, seg := range strings.FieldsFunc(id, func(c rune) bool { return c == '/' || c == '\\' }) {
		if seg == ".." || seg == "." || seg == "" {
			return "", errors.New("invalid path")
		}
	}
	if !isStyleSourceName(id) {
		return "", errors.New("invalid path")
	}
	target := filepath.Join(dir, filepath.FromSlash(id))
	baseAbs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(targetAbs, baseAbs+string(filepath.Separator)) {
		return "", errors.New("invalid path")
	}
	return targetAbs, nil
}

// styleSampleBase 把标题变成安全的文件名主干（保留 tiếng Việt 的 dấu）：
// 去掉路径分隔符/控制字符，去掉重复的扩展名，最多 60 字。
func styleSampleBase(title string) string {
	title = strings.TrimSpace(title)
	for _, ext := range []string{".txt", ".md", ".markdown"} {
		if strings.HasSuffix(strings.ToLower(title), ext) {
			title = strings.TrimSpace(title[:len(title)-len(ext)])
			break
		}
	}
	var b strings.Builder
	for _, r := range title {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || unicode.IsControl(r):
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	name := strings.Trim(strings.TrimSpace(b.String()), ".")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "Văn mẫu"
	}
	if runes := []rune(name); len(runes) > 60 {
		name = strings.TrimSpace(string(runes[:60]))
	}
	return name
}
