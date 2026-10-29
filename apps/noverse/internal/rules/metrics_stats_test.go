package rules

import (
	"strings"
	"testing"
)

// TestMeasureChapterStatsMaxima: số đo thô lấy giá trị lớn nhất (câu/đoạn/quãng
// không thoại) thay vì dòng vi phạm, cùng định nghĩa đo với script.
func TestMeasureChapterStatsMaxima(t *testing.T) {
	longSentence := strings.TrimSpace(strings.Repeat("một ", 45)) + "."
	longParagraph := strings.Join([]string{
		strings.TrimSpace(strings.Repeat("một ", 32)) + ".",
		strings.TrimSpace(strings.Repeat("một ", 32)) + ".",
		strings.TrimSpace(strings.Repeat("một ", 32)) + ".",
	}, " ")
	text := strings.Join([]string{
		longSentence,
		longParagraph,
		"“Cô đã nghe hết đoạn ghi âm đó chưa?” “Tôi nghe rồi.”",
		"Cô ấy nói live, rồi live lần nữa.",
		"trà xanh cướp kịch bản của tôi. trà xanh cướp kịch bản của cô.",
	}, "\n\n")
	config := Measurements{
		DialogueMin: 0.40, DialogueMax: 0.55,
		ProceduralMax: 10, QuietMax: 250, SentenceMax: 40, ParagraphMax: 90,
		RepeatedMin: 2, OpeningWords: 220, OpeningOverlapMax: 0.20,
		EnglishWords: []string{"live"},
	}
	stats := MeasureChapterStats(text, config, nil)
	if stats.MaxSentence != 45 {
		t.Errorf("MaxSentence = %d, muốn 45", stats.MaxSentence)
	}
	if stats.MaxParagraph != 96 {
		t.Errorf("MaxParagraph = %d, muốn 96", stats.MaxParagraph)
	}
	if stats.QuietWords != 141 {
		t.Errorf("QuietWords = %d, muốn 141 (quãng 45+96 tiếng trước câu thoại)", stats.QuietWords)
	}
	if stats.EnglishCount != 2 {
		t.Errorf("EnglishCount = %d, muốn 2", stats.EnglishCount)
	}
	if stats.RepeatedGrams != 1 {
		t.Errorf("RepeatedGrams = %d, muốn 1", stats.RepeatedGrams)
	}
	if stats.Procedural != 0 {
		t.Errorf("Procedural = %d, muốn 0", stats.Procedural)
	}
	// 12 tiếng thoại / 175 tiếng cả chương.
	if got, want := stats.DialogueRatio, 12.0/175; got < want-0.0001 || got > want+0.0001 {
		t.Errorf("DialogueRatio = %v, muốn %v", got, want)
	}
}

// TestMeasureChapterStatsStripsMarkdownHeading: tiêu đề Markdown "# " bị bỏ như
// CheckChapter, để số đo của một chương khớp lúc kiểm chương.
func TestMeasureChapterStatsStripsMarkdownHeading(t *testing.T) {
	stats := MeasureChapterStats("# Chương 1: Mở đầu\n\nMột câu ngắn.", Measurements{SentenceMax: 40, ParagraphMax: 90}, nil)
	if stats.MaxParagraph != 3 || stats.Words != 3 {
		t.Fatalf("stats = %+v, muốn đo phần thân (3 tiếng)", stats)
	}
}

// TestFatigueDensities: mật độ lần/1.000 tiếng, sắp giảm dần, kèm ngưỡng của luật.
func TestFatigueDensities(t *testing.T) {
	text := "dường như mưa. dường như cô quên. dường như anh tới muộn. im lặng thôi."
	fatigue := map[string]FatiguePer1000{"dường như": 1.33, "im lặng": 1.33}
	out := FatigueDensities(text, fatigue)
	if len(out) != 2 {
		t.Fatalf("densities = %+v", out)
	}
	// 3 lần "dường như" / 15 tiếng → 200 lần/1.000 tiếng; "im lặng" 1/15 → 66,67.
	if out[0].Word != "dường như" || out[0].Count != 3 {
		t.Fatalf("đầu bảng = %+v, muốn “dường như” ×3", out[0])
	}
	if got := out[0].Per1000; got < 199.9 || got > 200.1 {
		t.Errorf("Per1000 = %v, muốn 200", got)
	}
	if out[0].Limit != 1.33 {
		t.Errorf("Limit = %v, muốn 1.33", out[0].Limit)
	}
	if out[1].Word != "im lặng" || out[1].Count != 1 {
		t.Errorf("dòng hai = %+v", out[1])
	}
	if got := FatigueDensities("không có gì", fatigue); len(got) != 0 {
		t.Errorf("không dùng từ nào thì không có dòng: %+v", got)
	}
}
