package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// evidenceTextLoader 把 map 包装成 LoadChapterText 形参。
func evidenceTextLoader(texts map[int]string) func(int) (string, error) {
	return func(ch int) (string, error) { return texts[ch], nil }
}

func TestCollectEvidencePassages(t *testing.T) {
	firstParaText := "Dẫn nhập cảnh đêm khuya.\n\n" +
		"Lâm Uyển đẩy cửa bước vào sảnh chính.\n\n" +
		"Tiếng chuông vọng lại ba hồi."

	overlappingText := "Cảnh mở đầu.\n\n" +
		"Lâm Uyển nhìn thấy bức thư cũ trên bàn.\n\n" +
		"Chữ ký cuối thư khiến tay anh run.\n\n" +
		"Cảnh kết thúc chương."

	cases := []struct {
		name          string
		texts         map[int]string
		candidates    []evidenceCandidate
		wantItems     int
		wantPassage   []string // 每个 item 的 passage 必须包含的片段
		wantMerged    bool     // quote 字段含 \n（多来源合并）
		wantWarning   bool
		wantRuneLimit int // passage rune 上限校验，0 = 不检查
		wantSentence  string
	}{
		{
			name:  "quote ở đoạn đầu chương lấy đoạn đầu + đoạn kế",
			texts: map[int]string{2: firstParaText},
			candidates: []evidenceCandidate{
				{kind: "state_change", ref: "Lâm Uyển.location", chapter: 2, quote: "Dẫn nhập cảnh đêm khuya"},
			},
			wantItems:   1,
			wantPassage: []string{"Dẫn nhập cảnh đêm khuya", "Lâm Uyển đẩy cửa bước vào sảnh chính"},
		},
		{
			name:  "hai quote đoạn kề nhau gộp thành một cửa sổ",
			texts: map[int]string{3: overlappingText},
			candidates: []evidenceCandidate{
				{kind: "timeline", ref: "thấy thư", chapter: 3, quote: "Lâm Uyển nhìn thấy bức thư cũ trên bàn"},
				{kind: "foreshadow", ref: "fs-thu", chapter: 3, quote: "Chữ ký cuối thư khiến tay anh run"},
			},
			wantItems:   1,
			wantPassage: []string{"bức thư cũ trên bàn", "Chữ ký cuối thư", "Cảnh mở đầu", "Cảnh kết thúc chương"},
			wantMerged:  true,
		},
		{
			name:  "quote không còn trong chính văn bị bỏ kèm cảnh báo",
			texts: map[int]string{4: "Văn bản đã viết lại hoàn toàn."},
			candidates: []evidenceCandidate{
				{kind: "timeline", ref: "sự kiện cũ", chapter: 4, quote: "câu đã bị xoá khỏi bản mới"},
			},
			wantItems:   0,
			wantWarning: true,
		},
		{
			name: "đoạn dài cắt về giới hạn nhưng giữ nguyên câu chứa quote",
			texts: map[int]string{5: func() string {
				long := strings.Repeat("Một câu dài không liên quan. ", 30)
				return long + "\n\n" + strings.Repeat("Câu trước cũng dài. ", 15) +
					"Lâm Uyển tìm thấy chìa khóa dưới gối cũ." +
					strings.Repeat(" Câu sau cũng dài.", 15)
			}()},
			candidates: []evidenceCandidate{
				{kind: "foreshadow", ref: "fs-key", chapter: 5, quote: "chìa khóa dưới gối cũ"},
			},
			wantItems:     1,
			wantRuneLimit: evidenceMaxPassageRunes,
			wantSentence:  "Lâm Uyển tìm thấy chìa khóa dưới gối cũ.",
		},
		{
			name:  "chương thiếu bản cuối bị bỏ không cảnh báo",
			texts: map[int]string{},
			candidates: []evidenceCandidate{
				{kind: "timeline", ref: "x", chapter: 9, quote: "bất kỳ câu nào"},
			},
			wantItems:   0,
			wantWarning: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, warnings := collectEvidencePassages(tc.candidates, evidenceTextLoader(tc.texts), evidenceMaxItems)
			if len(items) != tc.wantItems {
				t.Fatalf("items = %d want %d (%+v)", len(items), tc.wantItems, items)
			}
			if tc.wantWarning && len(warnings) == 0 {
				t.Fatalf("expected warning, got none")
			}
			if !tc.wantWarning && len(warnings) != 0 {
				t.Fatalf("unexpected warnings: %v", warnings)
			}
			for i, fragment := range tc.wantPassage {
				if !strings.Contains(items[0].Passage, fragment) {
					t.Fatalf("item passage missing %q: %q", fragment, items[0].Passage)
				}
				_ = i
			}
			if tc.wantMerged && !strings.Contains(items[0].Quote, "\n") {
				t.Fatalf("merged window should carry both quotes: %q", items[0].Quote)
			}
			if tc.wantRuneLimit > 0 && utf8.RuneCountInString(items[0].Passage) > tc.wantRuneLimit {
				t.Fatalf("passage over limit: %d runes", utf8.RuneCountInString(items[0].Passage))
			}
			if tc.wantSentence != "" && !strings.Contains(items[0].Passage, tc.wantSentence) {
				t.Fatalf("passage lost quote sentence: %q", items[0].Passage)
			}
		})
	}
}

// 候选超上限时按来源顺序截断，后面的仍允许并入既有窗口。
func TestCollectEvidencePassagesCapsItems(t *testing.T) {
	texts := map[int]string{}
	var candidates []evidenceCandidate
	for ch := 1; ch <= 6; ch++ {
		texts[ch] = "Đoạn một của chương.\n\nCâu chứa bằng chứng của chương này.\n\nĐoạn cuối."
		candidates = append(candidates, evidenceCandidate{
			kind: "timeline", ref: "ev", chapter: ch, quote: "Câu chứa bằng chứng của chương này"})
	}
	items, warnings := collectEvidencePassages(candidates, evidenceTextLoader(texts), 4)
	if len(items) != 4 {
		t.Fatalf("items = %d want 4", len(items))
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	chapters := map[int]bool{}
	for _, it := range items {
		chapters[it.Chapter] = true
	}
	if !chapters[1] || chapters[5] || chapters[6] {
		t.Fatalf("expected earliest candidates kept: %+v", items)
	}
}

func TestEvidencePassageNeverSplitsQuoteOrExceedsBudget(t *testing.T) {
	t.Run("overlong evidence sentence is omitted", func(t *testing.T) {
		text := "Lan giữ chìa khóa " + strings.Repeat("rất lâu ", 100) + "."
		items, warnings := collectEvidencePassages(
			[]evidenceCandidate{{kind: "timeline", ref: "key", chapter: 2, quote: "Lan giữ chìa khóa"}},
			evidenceTextLoader(map[int]string{2: text}), 4)
		if len(items) != 0 || len(warnings) != 1 {
			t.Fatalf("oversized sentence must be omitted with warning: items=%v warnings=%v", items, warnings)
		}
	})
	t.Run("multi sentence quote stays whole", func(t *testing.T) {
		quote := "Lan giữ chìa khóa. Mẹ đã trao nó tận tay."
		text := strings.Repeat("Mưa rơi. ", 100) + quote + strings.Repeat(" Nắng lên.", 100)
		got := trimPassageKeepingQuote(text, quote, evidenceMaxPassageRunes)
		if !strings.Contains(got, quote) || utf8.RuneCountInString(got) > evidenceMaxPassageRunes {
			t.Fatalf("quote lost or budget exceeded: %q", got)
		}
	})
	t.Run("merged metadata only names retained evidence", func(t *testing.T) {
		first, second := "Lan nhận chìa khóa.", "Mẹ đã mở chiếc hộp."
		text := first + strings.Repeat(" Mưa vẫn rơi.", 100) + "\n\n" + second
		items, warnings := collectEvidencePassages([]evidenceCandidate{
			{kind: "timeline", ref: "first", chapter: 2, quote: first},
			{kind: "timeline", ref: "second", chapter: 2, quote: second},
		}, evidenceTextLoader(map[int]string{2: text}), 4)
		if len(items) != 1 || len(warnings) != 0 {
			t.Fatalf("merged result: items=%v warnings=%v", items, warnings)
		}
		if items[0].Quote != first || !strings.Contains(items[0].Passage, first) ||
			strings.Contains(items[0].Passage, second) || utf8.RuneCountInString(items[0].Passage) > evidenceMaxPassageRunes {
			t.Fatalf("quote metadata does not match retained passage: %+v", items[0])
		}
	})
}

func TestUnusableEvidenceDoesNotDisplaceValidSources(t *testing.T) {
	long := "Lan giữ chìa khóa " + strings.Repeat("rất lâu ", 100) + "."
	short := "Mẹ nhận lại chiếc hộp."
	t.Run("adjacent valid source survives", func(t *testing.T) {
		items, warnings := collectEvidencePassages([]evidenceCandidate{
			{chapter: 1, ref: "long", quote: "Lan giữ chìa khóa"},
			{chapter: 1, ref: "short", quote: short},
		}, evidenceTextLoader(map[int]string{1: long + "\n\n" + short}), 4)
		if len(items) != 1 || items[0].Quote != short || len(warnings) != 1 {
			t.Fatalf("valid adjacent evidence lost: items=%v warnings=%v", items, warnings)
		}
	})
	t.Run("oversized sources do not reserve slots", func(t *testing.T) {
		texts := map[int]string{5: short}
		var candidates []evidenceCandidate
		for chapter := 1; chapter <= 4; chapter++ {
			texts[chapter] = long
			candidates = append(candidates, evidenceCandidate{chapter: chapter, quote: "Lan giữ chìa khóa"})
		}
		candidates = append(candidates, evidenceCandidate{chapter: 5, quote: short})
		items, warnings := collectEvidencePassages(candidates, evidenceTextLoader(texts), 4)
		if len(items) != 1 || items[0].Chapter != 5 || len(warnings) != 4 {
			t.Fatalf("valid evidence displaced by oversized sources: items=%v warnings=%v", items, warnings)
		}
	})
}

func TestBridgingEvidenceMergesAllIntersectedWindows(t *testing.T) {
	paragraphs := []string{"Mở đầu.", "Lan nhận thư.", "Trời đã sáng.", "Mẹ đọc thư.", "Bếp đã nguội.", "Lan cất thư.", "Khép lại."}
	items, warnings := collectEvidencePassages([]evidenceCandidate{
		{chapter: 1, quote: paragraphs[1]},
		{chapter: 1, quote: paragraphs[5]},
		{chapter: 1, quote: paragraphs[3]},
	}, evidenceTextLoader(map[int]string{1: strings.Join(paragraphs, "\n\n")}), 4)
	if len(items) != 1 || len(warnings) != 0 {
		t.Fatalf("bridging evidence must coalesce windows: items=%v warnings=%v", items, warnings)
	}
	if items[0].Passage != strings.Join(paragraphs, "\n\n") {
		t.Fatalf("merged passage duplicated or omitted paragraphs: %q", items[0].Passage)
	}
}
