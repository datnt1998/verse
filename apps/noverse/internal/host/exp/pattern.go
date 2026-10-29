package exp

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// labels 是导出排版用的越南语标签集合（创作语种固定 tiếng Việt）。
type labels struct {
	chapter   func(n int) string               // 章节标题，如 "Chương 12"
	volume    func(n int, title string) string // 卷分隔标题
	volumeNav string                           // 目录页标题（EPUB nav）
	coverNav  string                           // 目录页封面链接文字
	xmlLang   string                           // xml:lang / dc:language
}

var exportLabels = labels{
	chapter: func(n int) string { return fmt.Sprintf("Chương %d", n) },
	volume: func(n int, title string) string {
		if title == "" {
			return fmt.Sprintf("Tập %d", n)
		}
		return fmt.Sprintf("Tập %d  %s", n, title)
	},
	volumeNav: "Mục lục",
	coverNav:  "Bìa",
	xmlLang:   "vi",
}

// ── 输出文件名模板 ──

// PatternVars 渲染 {truyen} {tap} {tu} {den} {ngay} {ext} 六个变量。
type PatternVars struct {
	Story  string    // {truyen} 故事目录名
	Volume int       // {tap}   卷号（扁平书恒为 1）
	From   int       // {tu}    起始章
	To     int       // {den}   结束章
	Date   time.Time // {ngay} 与 onExists=keep 的同日
	Format string    // {ext}   txt / epub
}

// RenderPattern 用变量渲染模板，返回相对路径（不解析到目录）。
// 未知变量、空段、. / .. 段都报错——模板来自书库配置，宁可显式拒绝也不静默
// 写错位置。路径分隔符仅 "/"，段内容经 sanitizeFileName 后再拼。
func RenderPattern(pattern string, vars PatternVars) (string, error) {
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("mẫu đường dẫn xuất rỗng")
	}
	repl := map[string]string{
		"truyen": vars.Story,
		"tap":    strconv.Itoa(vars.Volume),
		"tu":     strconv.Itoa(vars.From),
		"den":    strconv.Itoa(vars.To),
		"ngay":   vars.Date.Format("20060102"),
		"ext":    vars.Format,
	}
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '{' {
			end := strings.IndexByte(pattern[i:], '}')
			if end < 0 {
				return "", fmt.Errorf("mẫu đường dẫn xuất thiếu '}' ở vị trí %d", i)
			}
			key := pattern[i+1 : i+end]
			v, ok := repl[key]
			if !ok {
				return "", fmt.Errorf("mẫu đường dẫn xuất chứa biến không hỗ trợ {%s}", key)
			}
			b.WriteString(sanitizeFileName(v))
			i += end
			continue
		}
		b.WriteByte(c)
	}
	raw := strings.ReplaceAll(b.String(), "\\", "/")
	var segs []string
	for _, seg := range strings.Split(raw, "/") {
		seg = strings.TrimSpace(seg)
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("mẫu đường dẫn xuất sinh đoạn không hợp lệ: %q", raw)
		}
		segs = append(segs, seg)
	}
	return filepath.Join(segs...), nil
}

// ResolvePatternPath 渲染模板并拼到 exportsDir 下，返回绝对/相对目标路径。
func ResolvePatternPath(exportsDir, pattern string, vars PatternVars) (string, error) {
	rel, err := RenderPattern(pattern, vars)
	if err != nil {
		return "", err
	}
	return filepath.Join(exportsDir, rel), nil
}

// keepVariantPath 为 onExists="keep" 生成带时间戳的替代路径：
// name.ext → name-YYYYMMDD-HHmmss.ext；同秒仍重名则追加 -2、-3…（11-api-contract GAP-27）。
func keepVariantPath(path string, now time.Time) (string, error) {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(filepath.Base(path), ext)
	base := fmt.Sprintf("%s-%s", stem, now.Format("20060102-150405"))
	for n := 1; ; n++ {
		name := base + ext
		if n > 1 {
			name = fmt.Sprintf("%s-%d%s", base, n, ext)
		}
		candidate := filepath.Join(filepath.Dir(path), name)
		_, err := os.Stat(candidate)
		if os.IsNotExist(err) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("kiểm tra tệp đầu ra thất bại: %w", err)
		}
	}
}
