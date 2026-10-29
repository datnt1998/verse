package exp

import (
	"archive/zip"
	"bufio"
	"os"
	"regexp"
	"strings"
)

// epubChapterRe 匹配 renderEPUB 写出的每章文件名（OEBPS/chapterNNN.xhtml）。
var epubChapterRe = regexp.MustCompile(`^OEBPS/chapter\d+\.xhtml$`)

// txtChapterRe 匹配 renderTXT 写出的章标题行（"Chương N" 或 "Chương N  标题"）。
var txtChapterRe = regexp.MustCompile(`^Chương \d+(\s|$)`)

// CountChapters 估算一个已导出文件里的章数（同名文件比较卡片用）。
// EPUB 数章节文件；TXT 数章标题行。无法识别（非本工具导出、读失败）返回 ok=false。
func CountChapters(path string) (n int, ok bool) {
	switch {
	case strings.EqualFold(extOf(path), ".epub"):
		zr, err := zip.OpenReader(path)
		if err != nil {
			return 0, false
		}
		defer zr.Close()
		for _, f := range zr.File {
			if epubChapterRe.MatchString(f.Name) {
				n++
			}
		}
		return n, n > 0
	case strings.EqualFold(extOf(path), ".txt"):
		f, err := os.Open(path)
		if err != nil {
			return 0, false
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for sc.Scan() {
			if txtChapterRe.MatchString(sc.Text()) {
				n++
			}
		}
		if sc.Err() != nil {
			return 0, false
		}
		return n, n > 0
	default:
		return 0, false
	}
}

func extOf(path string) string {
	i := strings.LastIndexByte(path, '.')
	if i < 0 || strings.ContainsAny(path[i:], `/\`) {
		return ""
	}
	return path[i:]
}
