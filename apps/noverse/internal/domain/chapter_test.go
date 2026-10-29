package domain

import "testing"

// 「chữ」= 空白分隔、且至少含一个字母或数字的 token；纯标点片段不算。
func TestWordCountCountsWhitespaceTokens(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"spaces only", "   \n\t ", 0},
		{"vietnamese sentence", "Hắn bỏ đi.", 3},
		{"punctuation only", "— … *** «»", 0},
		{"digits count", "Chương 12", 2},
		{"dash between words is skipped", "Hắn đi — nàng ở lại.", 5},
		{"multi line", "# Chương 1\n\nHắn bỏ đi.", 5},
		// 旧实现按 rune 计：这句 6 个汉字会算成 6；按词只有一个 token。
		{"legacy chinese text counts tokens", "他不是愤怒", 1},
	}
	for _, c := range cases {
		if got := WordCount(c.text); got != c.want {
			t.Errorf("%s: WordCount(%q) = %d, want %d", c.name, c.text, got, c.want)
		}
	}
}

func TestShouldReviewMessagesAreVietnamese(t *testing.T) {
	ok, reason := ShouldReview(10)
	if !ok {
		t.Fatal("10 chapters should trigger review")
	}
	if reason != "Đã hoàn thành 10 chương, kích hoạt thẩm duyệt toàn cục" {
		t.Errorf("reason = %q", reason)
	}
	ok, reason = ShouldArcReview(false, true, 2, 3)
	if !ok || reason != "Tập 2 cung 3 kết thúc (hết tập), kích hoạt thẩm duyệt cấp cung + cấp tập" {
		t.Errorf("volume reason = %q", reason)
	}
	ok, reason = ShouldArcReview(true, false, 2, 3)
	if !ok || reason != "Tập 2 cung 3 kết thúc, kích hoạt thẩm duyệt cấp cung" {
		t.Errorf("arc reason = %q", reason)
	}
}
