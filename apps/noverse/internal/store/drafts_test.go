package store

import (
	"strings"
	"testing"
)

// LoadChapterContent 返回的字数按词计（旧实现按 rune），否则每次草稿回读都会
// 把「chữ」报成 4-5 倍的字符数，模型据此判断章节长度会严重失真。
func TestLoadChapterContentCountsWords(t *testing.T) {
	st := NewStore(t.TempDir())
	content := "# Chương 1\n\nHắn bỏ đi. Nàng ở lại."
	if err := st.Drafts.SaveDraft(1, content); err != nil {
		t.Fatal(err)
	}
	got, words, err := st.Drafts.LoadChapterContent(1)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Fatalf("nội dung nháp sai: %q", got)
	}
	// "#" 不计；Chương / 1 / Hắn / bỏ / đi. / Nàng / ở / lại. = 8
	if words != 8 {
		t.Fatalf("words = %d, want 8 (đếm theo từ, không theo ký tự)", words)
	}
}

// ExtractStyleAnchors 的段落窗口按词换算：50-300 词。
func TestExtractStyleAnchorsWordWindow(t *testing.T) {
	st := NewStore(t.TempDir())
	short := strings.Repeat("ngắn ", 20) // 20 词：低于下限
	medium := strings.Repeat("vừa ", 80) // 80 词：命中
	long := strings.Repeat("dài ", 320)  // 320 词：高于上限
	content := strings.Join([]string{short, medium, long}, "\n\n")
	if err := st.Drafts.SaveFinalChapter(1, content); err != nil {
		t.Fatal(err)
	}
	anchors, err := st.Drafts.ExtractStyleAnchors(5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 1 {
		t.Fatalf("chỉ đoạn 80 词 được chọn, got %d: %+v", len(anchors), anchors)
	}
	if !strings.HasPrefix(anchors[0], "vừa") {
		t.Fatalf("chọn nhầm đoạn: %q", anchors[0])
	}
}

// 对话样本下限按词：只有 1-3 个词的应答不具代表性。
// 用例特意选「字符多但词少」的一句（9 个字符、2 个词）：旧的 >5 字符判定会留下它，
// 按词判定必须丢掉。
func TestExtractDialogueSkipsTinySamples(t *testing.T) {
	st := NewStore(t.TempDir())
	content := "Lâm đứng dậy.\nLâm đáp: \"Ừ, được.\"\nLâm nói: \"Ta đi trước đây, ngươi ở lại giữ cửa.\"\n"
	if err := st.Drafts.SaveFinalChapter(1, content); err != nil {
		t.Fatal(err)
	}
	samples, err := st.Drafts.ExtractDialogue("Lâm", nil, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range samples {
		if strings.Contains(s, "\"Ừ, được.\"") {
			t.Fatalf("mẫu thoại quá ngắn (2 từ) không được lấy: %+v", samples)
		}
	}
	if len(samples) != 1 || !strings.Contains(samples[0], "Ta đi trước đây") {
		t.Fatalf("phải lấy đúng câu thoại dài: %+v", samples)
	}
}
