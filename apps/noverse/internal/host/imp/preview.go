package imp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PreviewChapter 是导入预览中的一章（G9）：只读扫描，不落任何工件。
type PreviewChapter struct {
	N          int    `json:"n"`
	Title      string `json:"title"`
	Words      int    `json:"words"`
	FirstLine  string `json:"firstLine"`
	NewlySplit bool   `json:"newlySplit"`
	Warning    string `json:"warning,omitempty"`
}

// Preview 是 POST …/import/preview 的响应体。
type Preview struct {
	SchemaVersion int              `json:"schemaVersion"`
	Filename      string           `json:"filename"`
	Size          int64            `json:"size"`
	Words         int              `json:"words"`
	SplitHow      string           `json:"splitHow"`
	Chapters      []PreviewChapter `json:"chapters"`
	Warnings      []string         `json:"warnings"`
	Encoding      string           `json:"encoding"`
}

// previewHeadingRe 与 internal/domain/chapter_heading.go 的 chapterNumberRe 同源并
// 扩展了中文数字（旧稿常见「第一章」）：越南语「Chương 12」、英语「Chapter 12」、
// 中文「第 12 章 / 第一章」都认。预览必须是确定性的（无模型、无写盘），因此这里
// 只做标题行扫描；真正的 /import 切分仍由模型按语义裁定边界，二者共享的是
// 解码/归一化与标题写法。
var previewHeadingRe = regexp.MustCompile(`^\s*(?i:(chương|chapter)\s*(\d+)|第\s*[零〇一二三四五六七八九十百千万两\d]+\s*章)`)

// previewDecorRe 去掉 Markdown 标题/加粗装饰（与 domain.headingDecorRe 一致）。
var previewDecorRe = regexp.MustCompile(`^\s*(?:#{1,6}\s*|\*\*)\s*|\s*\*\*\s*$`)

// PreviewSource 读取并扫描一份手稿，返回章节清单、告警与检测到的编码。
// 不创建导入工作区、不调用模型、不写任何文件（G9）。
func PreviewSource(path string) (Preview, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Preview{}, fmt.Errorf("Đọc tệp nguồn: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return Preview{}, fmt.Errorf("Tệp nguồn trống: %s", path)
	}
	dec, err := decodeSource(raw)
	if err != nil {
		return Preview{}, err
	}
	text := normalize(dec.text)

	type rawChapter struct {
		title string
		body  []string
	}
	var chapters []rawChapter
	frontLines := 0
	current := -1
	for _, line := range strings.Split(text, "\n") {
		if title, ok := previewHeadingTitle(line); ok {
			chapters = append(chapters, rawChapter{title: title})
			current = len(chapters) - 1
			continue
		}
		if current < 0 {
			if strings.TrimSpace(line) != "" {
				frontLines++
			}
			continue
		}
		chapters[current].body = append(chapters[current].body, line)
	}

	out := Preview{SchemaVersion: 1, Filename: filepath.Base(path), Size: int64(len(raw)), Words: len(strings.Fields(text)), SplitHow: "Theo dòng «Chương N»", Chapters: []PreviewChapter{}, Warnings: []string{}, Encoding: dec.encoding}
	if frontLines > 0 {
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("Có %d dòng mở đầu trước chương đầu tiên; khi nhập sẽ được xử lý như front matter", frontLines))
	}
	if len(chapters) == 0 {
		out.Warnings = append(out.Warnings,
			"Không tìm thấy tiêu đề chương (Chương N / Chapter N / 第N章); khi nhập, mô hình sẽ phân tách theo ngữ nghĩa")
		return out, nil
	}

	seen := make(map[string]int, len(chapters))
	for i, ch := range chapters {
		body := strings.TrimSpace(strings.Join(ch.body, "\n"))
		words := len(strings.Fields(body))
		if body == "" {
			out.Warnings = append(out.Warnings,
				fmt.Sprintf("Chương %d (%q) không có nội dung", i+1, ch.title))
		}
		first := ""
		for _, line := range ch.body {
			if line = strings.TrimSpace(line); line != "" {
				first = line
				break
			}
		}
		out.Chapters = append(out.Chapters, PreviewChapter{
			N: i + 1, Title: ch.title, Words: words, FirstLine: first,
		})
		key := strings.Join(strings.Fields(strings.ToLower(ch.title)), " ")
		if prev, ok := seen[key]; ok {
			out.Warnings = append(out.Warnings,
				fmt.Sprintf("Chương %d và chương %d trùng tiêu đề %q, có thể bị tách nhầm", prev, i+1, ch.title))
		} else {
			seen[key] = i + 1
		}
	}
	warnLongChapters(&out)
	return out, nil
}

// previewHeadingTitle 判断一行是否是章节标题行，返回去掉装饰后的标题。
func previewHeadingTitle(line string) (string, bool) {
	bare := strings.TrimSpace(previewDecorRe.ReplaceAllString(strings.TrimSpace(line), ""))
	if bare == "" || !previewHeadingRe.MatchString(bare) {
		return "", false
	}
	return bare, true
}

// warnLongChapters uses the whole-book chapter average, including short chapters.
func warnLongChapters(out *Preview) {
	total := 0
	for _, ch := range out.Chapters {
		total += ch.Words
	}
	if total == 0 {
		return
	}
	for i := range out.Chapters {
		ch := &out.Chapters[i]
		if ch.Words*len(out.Chapters) >= 2*total {
			ch.Warning = fmt.Sprintf("Chương %d dài bất thường: %d chữ, gấp đôi trung bình hoặc hơn", ch.N, ch.Words)
			out.Warnings = append(out.Warnings, ch.Warning)
		}
	}
}
