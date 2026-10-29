package exp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

// newTestStore 构造一个 t.TempDir() 之上的最小 store，已写入 1..n 章终稿与 progress。
func newTestStore(t *testing.T, novelName string, completed []int) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := s.Progress.Init(len(completed)); err != nil {
		t.Fatalf("init progress: %v", err)
	}
	if novelName != "" {
		if err := s.Book.Save(domain.BookMetadata{Title: novelName, Synopsis: "Một đoạn giới thiệu thử nghiệm dành cho độc giả."}); err != nil {
			t.Fatalf("save book: %v", err)
		}
	}
	if err := s.Progress.UpdatePhase(domain.PhaseWriting); err != nil {
		t.Fatalf("phase writing: %v", err)
	}
	for _, ch := range completed {
		if err := s.Drafts.SaveFinalChapter(ch, fmt.Sprintf("Nội dung ch %d.", ch)); err != nil {
			t.Fatalf("save chapter %d: %v", ch, err)
		}
		if err := s.Progress.StartChapter(ch); err != nil {
			t.Fatalf("start chapter %d: %v", ch, err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 5, "cliff", "main"); err != nil {
			t.Fatalf("mark complete %d: %v", ch, err)
		}
	}
	return s, dir
}

func TestRun_HappyPath_DefaultsToNovelDir(t *testing.T) {
	s, dir := newTestStore(t, "Quang Ban", []int{1, 2, 3})
	if err := s.Outline.SavePremise("Câu chuyện về ánh sáng và bóng tối."); err != nil {
		t.Fatalf("save premise: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "Người về đêm mưa"},
		{Chapter: 2, Title: "Rạng đông"},
		{Chapter: 3, Title: "Tro tàn"},
	}); err != nil {
		t.Fatalf("save outline: %v", err)
	}

	res, err := Run(context.Background(), Deps{Store: s}, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Chapters != 3 {
		t.Errorf("Chapters = %d, want 3", res.Chapters)
	}
	if res.Path != filepath.Join(dir, "Quang Ban.txt") {
		t.Errorf("Path = %q, muốn mặc định {dir}/Quang Ban.txt", res.Path)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(data)
	// 越南语导出书名不带《》书名号，直接顶行输出。
	if !strings.HasPrefix(text, "Quang Ban\n\n") {
		t.Errorf("output missing book title line\nfull:\n%s", text)
	}
	for _, want := range []string{"Chương 1  Người về đêm mưa", "Chương 3  Tro tàn"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\nfull:\n%s", want, text)
		}
	}
	// premise 不进导出（创作蓝图，非读者内容）
	if strings.Contains(text, "Câu chuyện về ánh sáng và bóng tối.") {
		t.Errorf("premise must not appear in export:\n%s", text)
	}
}

func TestRun_UsesCommittedTitleForCompletedChapter(t *testing.T) {
	s, _ := newTestStore(t, "Quang Ban", []int{1})
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Tiêu đề kế hoạch"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{
		Chapter: 1, Title: "Tiêu đề bản hoàn chỉnh", Summary: "Tóm tắt",
	}); err != nil {
		t.Fatal(err)
	}

	res, err := Run(context.Background(), Deps{Store: s}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Chương 1  Tiêu đề bản hoàn chỉnh") || strings.Contains(text, "Tiêu đề kế hoạch") {
		t.Fatalf("export title projection is wrong:\n%s", text)
	}
}

// TestRun_PremiseNotExported 端到端钉死：premise.md 存在也不进导出，书名保留（issue #27）。
func TestRun_PremiseNotExported(t *testing.T) {
	s, _ := newTestStore(t, "Quang Ban", []int{1})
	if err := s.Outline.SavePremise("# Quang Ban\n## Độc giả mục tiêu\nBản thiết kế sáng tác không được xuất hiện."); err != nil {
		t.Fatalf("save premise: %v", err)
	}
	res, err := Run(context.Background(), Deps{Store: s}, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "Bản thiết kế sáng tác không được xuất hiện.") || strings.Contains(text, "Độc giả mục tiêu") {
		t.Errorf("premise must not be exported, got:\n%s", text)
	}
	if !strings.HasPrefix(text, "Quang Ban\n\n") {
		t.Errorf("book title should remain: %s", text)
	}
}

func TestRun_NoCompletedChapters(t *testing.T) {
	s, _ := newTestStore(t, "X", nil)
	_, err := Run(context.Background(), Deps{Store: s}, Options{})
	if err == nil {
		t.Fatal("expect error when no completed chapters")
	}
}

func TestRun_ExistingFile_NoOverwrite(t *testing.T) {
	s, dir := newTestStore(t, "X", []int{1})
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("preexisting"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target})
	if err == nil {
		t.Fatal("expect error when target exists and !Overwrite")
	}
	// 已存在文件必须原样保留（未被部分写入污染）。
	if data, _ := os.ReadFile(target); string(data) != "preexisting" {
		t.Errorf("preexisting file was clobbered: %q", data)
	}

	// 加 Overwrite 应成功
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target, Overwrite: true})
	if err != nil {
		t.Fatalf("Overwrite Run: %v", err)
	}
	if res.Path != target {
		t.Errorf("Path = %q want %q", res.Path, target)
	}
	data, _ := os.ReadFile(target)
	if string(data) == "preexisting" {
		t.Error("file not overwritten")
	}
}

// Lỗi ghi tệp đi thẳng vào Nhật ký/Job.error: phải là tiếng Việt và không mang đường dẫn tuyệt đối.
func TestRun_WriteFailureIsVietnameseWithoutAbsolutePath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bỏ qua quyền thư mục")
	}
	s, dir := newTestStore(t, "X", []int{1})
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	_, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: filepath.Join(locked, "out.txt")})
	if err == nil {
		t.Fatal("expect write failure in read-only directory")
	}
	want := `ghi tệp "out.txt" thất bại: không có quyền ghi vào thư mục đích`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestRun_RangeWithSkipped(t *testing.T) {
	s, _ := newTestStore(t, "X", []int{1, 2, 3})
	res, err := Run(context.Background(), Deps{Store: s}, Options{From: 2, To: 5, Overwrite: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Chapters != 2 {
		t.Errorf("Chapters = %d want 2 (only 2,3 completed in range 2..5)", res.Chapters)
	}
	if got := res.Skipped; len(got) != 2 || got[0] != 4 || got[1] != 5 {
		t.Errorf("Skipped = %v want [4 5]", got)
	}
}

func TestRun_FromGreaterThanTo(t *testing.T) {
	s, _ := newTestStore(t, "X", []int{1, 2})
	_, err := Run(context.Background(), Deps{Store: s}, Options{From: 5, To: 2})
	if err == nil {
		t.Fatal("expect error for invalid range")
	}
}

func TestRun_UnsupportedFormat(t *testing.T) {
	s, _ := newTestStore(t, "X", []int{1})
	_, err := Run(context.Background(), Deps{Store: s}, Options{Format: Format("pdf")})
	if err == nil {
		t.Fatal("expect error for unsupported format")
	}
}

func TestRunRejectsMissingBookMetadata(t *testing.T) {
	s, _ := newTestStore(t, "", []int{1})
	if _, err := Run(context.Background(), Deps{Store: s}, Options{}); err == nil {
		t.Fatal("Thiếu thông tin tác phẩm thì phải từ chối xuất")
	}
}

func TestInferFormat(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"", FormatTXT, false},
		{"book.txt", FormatTXT, false},
		{"book.TXT", FormatTXT, false},
		{"book.epub", FormatEPUB, false},
		{"book.EPUB", FormatEPUB, false},
		{"/abs/path/x.epub", FormatEPUB, false},
		{"book", FormatTXT, false}, // 无后缀按 TXT
		{"book.dat", "", true},
		{"book.pdf", "", true},
	}
	for _, c := range cases {
		got, err := inferFormat(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("inferFormat(%q) want error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("inferFormat(%q): unexpected err: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("inferFormat(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestRun_EPUB_FromExtension(t *testing.T) {
	s, dir := newTestStore(t, "Quang Ban", []int{1})
	if err := s.Outline.SavePremise("Ánh sáng và bóng tối."); err != nil {
		t.Fatalf("save premise: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Đêm mưa"}}); err != nil {
		t.Fatalf("save outline: %v", err)
	}

	target := filepath.Join(dir, "out.epub")
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: target})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Path != target {
		t.Errorf("Path = %q want %q", res.Path, target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// EPUB 是 zip，前 4 字节 PK 头
	if len(data) < 4 || string(data[:2]) != "PK" {
		t.Errorf("output does not look like a zip: %x", data[:min(8, len(data))])
	}
}

func TestRun_DefaultPathFollowsFormat(t *testing.T) {
	s, dir := newTestStore(t, "Quang Ban", []int{1})
	res, err := Run(context.Background(), Deps{Store: s}, Options{Format: FormatEPUB})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := filepath.Join(dir, "Quang Ban.epub")
	if res.Path != want {
		t.Errorf("Path = %q want %q", res.Path, want)
	}
}

// TestRun_DirectoryTargetGetsBookFileName 钉死两类目录参数：已存在目录（之前 rename
// 到目录报 "file exists"）与 "~/" 开头路径（之前被建成字面 "~" 目录）。
func TestRun_DirectoryTargetGetsBookFileName(t *testing.T) {
	s, _ := newTestStore(t, "Quang Ban", []int{1})

	existing := t.TempDir()
	res, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: existing, Overwrite: true})
	if err != nil {
		t.Fatalf("Run to existing dir: %v", err)
	}
	if want := filepath.Join(existing, "Quang Ban.txt"); res.Path != want {
		t.Errorf("existing dir: Path = %q want %q", res.Path, want)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	res, err = Run(context.Background(), Deps{Store: s}, Options{OutPath: "~/exports/", Format: FormatEPUB})
	if err != nil {
		t.Fatalf("Run to ~/exports/: %v", err)
	}
	if want := filepath.Join(home, "exports", "Quang Ban.epub"); res.Path != want {
		t.Errorf("~/ dir: Path = %q want %q", res.Path, want)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Errorf("output not written: %v", err)
	}

	res, err = Run(context.Background(), Deps{Store: s}, Options{OutPath: "~/book.txt"})
	if err != nil {
		t.Fatalf("Run to ~/book.txt: %v", err)
	}
	if want := filepath.Join(home, "book.txt"); res.Path != want {
		t.Errorf("~ file: Path = %q want %q", res.Path, want)
	}
}

func TestRun_UnknownExtension(t *testing.T) {
	s, _ := newTestStore(t, "X", []int{1})
	_, err := Run(context.Background(), Deps{Store: s}, Options{OutPath: "/tmp/foo.dat"})
	if err == nil {
		t.Fatal("expect error for unknown extension")
	}
	// 未知扩展名必须显式拒绝（不能静默写 .dat 当成别的格式）。
	if !strings.Contains(err.Error(), ".dat") {
		t.Errorf("error should mention the extension: %v", err)
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"":                     "novel",
		"   ":                  "novel",
		"normal":               "normal",
		"a/b":                  "a_b",
		"a\\b":                 "a_b",
		"a:b*c?\"d<e>f|g\x00h": "a_b_c__d_e_f_g_h",
	}
	for in, want := range cases {
		if got := sanitizeFileName(in); got != want {
			t.Errorf("sanitizeFileName(%q) = %q want %q", in, got, want)
		}
	}
}
