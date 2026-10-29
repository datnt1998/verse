package stylestat

import (
	"strings"
	"testing"
)

func chapterWith(body string) string {
	return "# Tiêu đề\n" + body
}

func TestComputeBelowMinChapters(t *testing.T) {
	in := Input{Chapters: []string{"a", "b", "c", "d"}}
	if Compute(in) != nil {
		t.Fatal("below minChapters should return nil")
	}
}

func TestComputePatterns(t *testing.T) {
	// Mỗi câu trúng đúng một lớp mẫu, để per_chapter phải bằng 1.0.
	body := "Không phải giận dữ mà là sợ hãi. Chốc lát sau, trời tối. Nàng nhìn nó như thể nhìn con vật lạ. " +
		"Cả căn phòng im lặng. Khoé môi hắn nhếch lên. Tim nàng thắt lại. Nàng nghĩ thầm trong lòng. " +
		"Đó là một cảm giác khó tả.\nChính văn.\n"
	chapters := make([]string, 6)
	for i := range chapters {
		chapters[i] = chapterWith(body)
	}
	s := Compute(Input{Chapters: chapters})
	if s == nil {
		t.Fatal("expected stats")
	}
	want := map[string]int{
		"Câu đối lập“không phải… mà là”":                6,
		"Lượng từ thời gian“một thoáng/chốc lát”":       6,
		"So sánh sáo“như thể/tựa như/giống như”":        6,
		"Nhịp im lặng“im lặng/không nói gì”":            6,
		"Mẫu thần thái“ánh mắt lóe lên/khoé môi nhếch”": 6,
		"Phản ứng cơ thể“tim thắt lại/người run lên”":   6,
		"Dấu hiệu suy nghĩ“nghĩ thầm/nhận ra”":          6,
		"Sáo trừu tượng“một cảm giác khó tả”":           6,
	}
	for _, p := range s.Patterns {
		if w, ok := want[p.Name]; ok && p.Total != w {
			t.Errorf("%s total: got %d want %d", p.Name, p.Total, w)
		}
		if p.PerChapter != 1.0 {
			t.Errorf("%s per_chapter: got %v want 1.0", p.Name, p.PerChapter)
		}
	}
	if len(s.Patterns) != len(want) {
		t.Errorf("want %d pattern classes, got %d: %+v", len(want), len(s.Patterns), s.Patterns)
	}
}

func TestComputeTopPhrasesWithStopwords(t *testing.T) {
	// "đỉnh Thanh Vân" xuất hiện dày; "Lục Cửu Uyên" là tên nhân vật phải bị lọc.
	line := "Mọi người nhìn về đỉnh Thanh Vân, Lục Cửu Uyên đứng chắp tay.\n"
	chapters := make([]string, 10)
	for i := range chapters {
		chapters[i] = chapterWith(strings.Repeat(line, 3))
	}
	s := Compute(Input{Chapters: chapters, Stopwords: []string{"Lục Cửu Uyên"}})
	if s == nil {
		t.Fatal("expected stats")
	}
	var hasMountain, hasName bool
	for _, p := range s.TopPhrases {
		if strings.Contains(p.Text, "thanh vân") {
			hasMountain = true
		}
		for _, namePart := range []string{"lục", "cửu", "uyên"} {
			if strings.Contains(" "+p.Text+" ", " "+namePart+" ") {
				hasName = true
			}
		}
	}
	if !hasMountain {
		t.Errorf("expected 'thanh vân' phrase mined, got %+v", s.TopPhrases)
	}
	if hasName {
		t.Errorf("character name should be filtered, got %+v", s.TopPhrases)
	}
}

func TestComputeRepeatedSentences(t *testing.T) {
	motto := "Cả đời này ta chưa từng đi xa, mong ngươi thay ta nhìn ngắm núi sông phương ấy."
	chapters := make([]string, 6)
	for i := range chapters {
		body := "Chính văn bình thường, không có gì lặp.\n"
		if i%2 == 0 {
			body += motto + "\n"
		}
		chapters[i] = chapterWith(body)
	}
	s := Compute(Input{Chapters: chapters})
	if s == nil {
		t.Fatal("expected stats")
	}
	if len(s.RepeatedSentences) == 0 {
		t.Fatalf("expected repeated sentence, got none")
	}
	got := s.RepeatedSentences[0]
	if got.Chapters != 3 || got.Count != 3 {
		t.Errorf("repeated sentence: %+v", got)
	}
	if !strings.HasPrefix(got.Text, "Cả đời này") {
		t.Errorf("text: %q", got.Text)
	}
}

func TestComputeEndingAndOpening(t *testing.T) {
	short := chapterWith("Cả đêm không ngủ.\nChính văn rất dài.\nHắn bỏ đi.")
	longSentence := strings.Repeat("đây là một câu kết rất dài ", 35) + "hết."
	long := chapterWith("Chuyện ban ngày.\nChính văn.\n" + longSentence)
	chapters := []string{short, short, short, long, long}
	s := Compute(Input{Chapters: chapters})
	if s == nil {
		t.Fatal("expected stats")
	}
	if s.Ending.ShortRatio != 0.6 {
		t.Errorf("short_ratio: got %v want 0.6", s.Ending.ShortRatio)
	}
	if s.Ending.MedianWords <= 0 {
		t.Errorf("median words should be counted in words: %+v", s.Ending)
	}
	if s.OpeningTimeRate != 0.6 {
		t.Errorf("opening_time_rate: got %v want 0.6", s.OpeningTimeRate)
	}
}

func TestComputeTitleFormats(t *testing.T) {
	chapters := make([]string, 5)
	for i := range chapters {
		chapters[i] = chapterWith("Chính văn.")
	}
	// 混用 → 上报
	s := Compute(Input{Chapters: chapters, Titles: []string{"Chương 1: Gió nổi", "Mây dâng", "Chương 3: Sấm động"}})
	if s.TitleFormats == nil || s.TitleFormats.WithPrefix != 2 || s.TitleFormats.WithoutPrefix != 1 {
		t.Errorf("title formats: %+v", s.TitleFormats)
	}
	// 统一 → 不上报
	s = Compute(Input{Chapters: chapters, Titles: []string{"Gió nổi", "Mây dâng"}})
	if s.TitleFormats != nil {
		t.Errorf("uniform titles should not report: %+v", s.TitleFormats)
	}
	// 旧数据的中文标题仍要认得前缀（含中文数字），否则历史书会误报混用
	s = Compute(Input{Chapters: chapters, Titles: []string{"第3章 雷动", "Mây dâng"}})
	if s.TitleFormats == nil || s.TitleFormats.WithPrefix != 1 {
		t.Errorf("legacy chinese prefix should still count: %+v", s.TitleFormats)
	}
	s = Compute(Input{Chapters: chapters, Titles: []string{"第十二章 雷动", "Mây dâng"}})
	if s.TitleFormats == nil || s.TitleFormats.WithPrefix != 1 {
		t.Errorf("legacy chinese numerals should still count: %+v", s.TitleFormats)
	}
}

// 章末「短结尾」按词判定：30 词（约 127 字符）的收尾句仍算短句。
// 旧的 30 rune 判定会把它算成长句，short_ratio 从 1.0 掉到 0。
func TestComputeEndingShortRatioCountsWords(t *testing.T) {
	ending := strings.Repeat("gió lạnh thổi qua ", 7) + "rồi tắt."
	chapters := make([]string, 5)
	for i := range chapters {
		chapters[i] = chapterWith("Chính văn.\n" + ending)
	}
	s := Compute(Input{Chapters: chapters})
	if s == nil {
		t.Fatal("expected stats")
	}
	if s.Ending.ShortRatio != 1 {
		t.Fatalf("short_ratio: got %v want 1 (30 词 = 短结尾)", s.Ending.ShortRatio)
	}
	if s.Ending.MedianWords != 30 {
		t.Fatalf("median words: got %d want 30", s.Ending.MedianWords)
	}
}
