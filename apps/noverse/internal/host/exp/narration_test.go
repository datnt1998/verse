package exp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
)

// TestCleanNarrationLine 表驱动：Markdown/装饰符号剥离与整行丢弃。
func TestCleanNarrationLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // "" 表示整行丢弃
	}{
		{"plain", "Cô ấy mở cửa.", "Cô ấy mở cửa."},
		{"atx heading dropped", "# Vai của tôi là đổ canh", ""},
		{"deep heading dropped", "### Chương 3: Đoạn cuối", ""},
		{"thematic break", "---", ""},
		{"thematic stars", "* * *", ""},
		{"blockquote", "> Câu trích dẫn.", "Câu trích dẫn."},
		{"bullet", "- Việc thứ nhất.", "Việc thứ nhất."},
		{"ordered", "1. Việc thứ nhất.", "Việc thứ nhất."},
		{"emphasis", "cô *rất* _mệt_ hôm **nay**", "cô rất mệt hôm nay"},
		{"inline code", "mã `abc` hỏng", "mã abc hỏng"},
		{"link keeps text", "đọc [báo cáo](https://x.vn) xong", "đọc báo cáo xong"},
		{"image dropped", "xem ![ảnh](a.png) rồi đi", "xem rồi đi"},
		{"table sep dropped", "|---|---|", ""},
		{"table row", "| tên | giá |", "tên giá"},
		{"emoji stripped", "chào 😀 buổi sáng ⭐", "chào buổi sáng"},
		{"box drawing stripped", "════ mục ════", "mục"},
		{"dialogue dash kept", "— Cô không hiểu à?", "— Cô không hiểu à?"},
		{"empty", "   ", ""},
	}
	for _, c := range cases {
		if got := cleanNarrationLine(c.in); got != c.want {
			t.Errorf("%s: cleanNarrationLine(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestRenderNarration_Layout 钉死版式：无书名无章标题、章间两个空行、段间一个空行、末尾换行。
func TestRenderNarration_Layout(t *testing.T) {
	bodies := map[int]string{
		1: "# Tiêu đề chương một\n\nĐoạn đầu tiên.\n\nĐoạn thứ hai.",
		2: "# Tiêu đề chương hai\n\nĐoạn của chương hai.\n",
	}
	got := renderNarration([]int{1, 2}, bodies, false)
	want := "Đoạn đầu tiên.\n\nĐoạn thứ hai.\n\n\nĐoạn của chương hai.\n"
	if got != want {
		t.Errorf("renderNarration = %q, want %q", got, want)
	}
}

// TestRenderNarration_ReadNumbers 表驱动：readNumbers 关时数字保留，开时读成越南语字。
func TestRenderNarration_ReadNumbers(t *testing.T) {
	bodies := map[int]string{1: "Năm 1984 cô mất 3 con mèo.\n\nCó 90 giây để chạy."}

	off := renderNarration([]int{1}, bodies, false)
	if !strings.Contains(off, "1984") || !strings.Contains(off, "90 giây") {
		t.Errorf("readNumbers=false must keep digits, got: %q", off)
	}

	on := renderNarration([]int{1}, bodies, true)
	for _, want := range []string{
		"một nghìn chín trăm tám mươi tư",
		"chín mươi giây",
	} {
		if !strings.Contains(on, want) {
			t.Errorf("readNumbers=true missing %q in: %q", want, on)
		}
	}
	if strings.ContainsAny(on, "0123456789") {
		t.Errorf("readNumbers=true left digits: %q", on)
	}
}

// TestRun_Narration 端到端：Format=narration → 默认文件名 <title>-narration.txt，
// 无书名、无章标题，章间两个空行。
func TestRun_Narration(t *testing.T) {
	s, dir := newTestStore(t, "Quang Ban", []int{1, 2})
	if err := s.Drafts.SaveFinalChapter(1, "# Mở đầu\n\nCâu *đầu* tiên."); err != nil {
		t.Fatal(err)
	}
	if err := s.Drafts.SaveFinalChapter(2, "# Kết\n\nCâu cuối cùng."); err != nil {
		t.Fatal(err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "Mở đầu"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 2, Title: "Kết"}); err != nil {
		t.Fatal(err)
	}

	res, err := Run(context.Background(), Deps{Store: s}, Options{Format: FormatNarration})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Path != filepath.Join(dir, "Quang Ban-narration.txt") {
		t.Errorf("Path = %q, want %q", res.Path, filepath.Join(dir, "Quang Ban-narration.txt"))
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "Quang Ban") {
		t.Errorf("narration must not contain book title:\n%s", text)
	}
	if !strings.Contains(text, "Câu đầu tiên.\n\n\nCâu cuối cùng.\n") {
		t.Errorf("chapter separation wrong:\n%q", text)
	}
}
