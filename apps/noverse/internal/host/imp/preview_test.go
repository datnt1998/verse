package imp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePreviewSource(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return path
}

func TestPreviewSourceSplitsVietnameseHeadings(t *testing.T) {
	path := writePreviewSource(t, "truyen.txt", strings.Join([]string{
		"Chương 1: Khởi đầu",
		"",
		"Dòng đầu tiên.",
		"Câu thứ hai.",
		"",
		"## Chương 2: Gặp gỡ",
		"",
		"Một hai ba.",
	}, "\n"))

	got, err := PreviewSource(path)
	if err != nil {
		t.Fatalf("PreviewSource: %v", err)
	}
	if got.Encoding != encodingUTF8 {
		t.Fatalf("encoding = %q, want %q", got.Encoding, encodingUTF8)
	}
	if len(got.Chapters) != 2 {
		t.Fatalf("chapters = %+v, want 2", got.Chapters)
	}
	first := got.Chapters[0]
	if first.N != 1 || first.Title != "Chương 1: Khởi đầu" {
		t.Fatalf("chapter 1 = %+v", first)
	}
	if first.Words != 6 || first.FirstLine != "Dòng đầu tiên." {
		t.Fatalf("chapter 1 words/firstLine = %d/%q", first.Words, first.FirstLine)
	}
	if got.Chapters[1].N != 2 || got.Chapters[1].Title != "Chương 2: Gặp gỡ" || got.Chapters[1].Words != 3 {
		t.Fatalf("chapter 2 = %+v", got.Chapters[1])
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", got.Warnings)
	}
}

func TestPreviewSourceAcceptsChineseAndEnglishHeadings(t *testing.T) {
	path := writePreviewSource(t, "old.txt", strings.Join([]string{
		"第一章 风起",
		"正文一",
		"Chapter 2: Tide",
		"body two",
		"第 3 章 云涌",
		"正文三",
	}, "\n"))

	got, err := PreviewSource(path)
	if err != nil {
		t.Fatalf("PreviewSource: %v", err)
	}
	if len(got.Chapters) != 3 {
		t.Fatalf("chapters = %+v, want 3", got.Chapters)
	}
	if got.Chapters[0].Title != "第一章 风起" || got.Chapters[1].Title != "Chapter 2: Tide" {
		t.Fatalf("titles = %q, %q", got.Chapters[0].Title, got.Chapters[1].Title)
	}
	if got.Chapters[2].N != 3 {
		t.Fatalf("third chapter n = %d", got.Chapters[2].N)
	}
}

func TestPreviewSourceWarnsWhenNoHeadings(t *testing.T) {
	path := writePreviewSource(t, "plain.txt", "Chỉ có vài dòng văn xuôi.\nKhông có tiêu đề chương.\n")

	got, err := PreviewSource(path)
	if err != nil {
		t.Fatalf("PreviewSource: %v", err)
	}
	if got.Chapters == nil || len(got.Chapters) != 0 {
		t.Fatalf("chapters = %#v, want empty non-nil slice", got.Chapters)
	}
	if !containsWarning(got.Warnings, "Không tìm thấy tiêu đề chương") {
		t.Fatalf("warnings = %v", got.Warnings)
	}
}

func TestPreviewSourceReportsFrontMatterAndDuplicateTitles(t *testing.T) {
	path := writePreviewSource(t, "front.txt", strings.Join([]string{
		"Lời giới thiệu",
		"",
		"Chương 1: Mở",
		"nội dung một",
		"Chương 2: Mở",
		"Chương 2: Mở",
		"",
	}, "\n"))

	got, err := PreviewSource(path)
	if err != nil {
		t.Fatalf("PreviewSource: %v", err)
	}
	if len(got.Chapters) != 3 {
		t.Fatalf("chapters = %+v", got.Chapters)
	}
	if !containsWarning(got.Warnings, "dòng mở đầu") {
		t.Fatalf("missing front matter warning: %v", got.Warnings)
	}
	if !containsWarning(got.Warnings, "trùng tiêu đề") {
		t.Fatalf("missing duplicate warning: %v", got.Warnings)
	}
	if !containsWarning(got.Warnings, "không có nội dung") {
		t.Fatalf("missing empty chapter warning: %v", got.Warnings)
	}
}

func containsWarning(warnings []string, phrase string) bool {
	for _, w := range warnings {
		if strings.Contains(w, phrase) {
			return true
		}
	}
	return false
}
