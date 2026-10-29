package domain

import (
	"strings"
	"testing"
)

// TestLocateQuote pin 规范化匹配与偏移口径：弯/直引号、折叠空白、三种连字符、
// "…" 与 "..." 互相可命中，返回的 [start, end) 始终落在原文上。
func TestLocateQuote(t *testing.T) {
	text := `Nàng nói: “Trời   đã tối— rồi… chưa về.” Gió thổi qua khe cửa.`

	for _, tc := range []struct {
		name  string
		quote string
		want  string // đoạn text[start:end] phải đúng bằng
	}{
		{"exact", "Trời   đã tối", "Trời   đã tối"},
		{"curly to straight", `"Trời   đã tối`, `“Trời   đã tối`},
		{"straight to curly", `“Trời   đã tối`, `“Trời   đã tối`},
		{"folded whitespace", "Trời đã tối", "Trời   đã tối"},
		{"em dash to hyphen", "đã tối— rồi", "đã tối— rồi"},
		{"dots to ellipsis", "rồi… chưa", "rồi… chưa"},
		{"ellipsis to dots", "rồi... chưa", "rồi… chưa"},
		{"plain words", "chưa về", "chưa về"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := LocateQuote(text, tc.quote)
			if !ok {
				t.Fatalf("LocateQuote(%q) = not found", tc.quote)
			}
			if got := text[start:end]; got != tc.want {
				t.Fatalf("text[%d:%d] = %q, want %q", start, end, got, tc.want)
			}
		})
	}

	// 单引号弯/直合一单独验证：双引号与单引号是两种标记，不互替。
	single := `Hắn đáp: ‘Được thôi.’`
	start, end, ok := LocateQuote(single, "‘Được thôi")
	if !ok || single[start:end] != "‘Được thôi" {
		t.Fatalf("single quote unify = %q %v", single[start:end], ok)
	}
	if _, _, ok := LocateQuote(single, `"Được thôi`); ok {
		t.Fatal("double-quote mark must not match a single-quote mark")
	}
}

// TestLocateQuoteNormalizesNormalizedOffset 确定偏移回指原文而非规范化串：
// 命中点之前有大量被折叠的空白时，返回区间仍切出原文片段。
func TestLocateQuoteOffsetsReferToOriginal(t *testing.T) {
	text := "Đầu câu.   " + strings.Repeat(" \t\n", 20) + "Cuối câu dài hơn."
	start, end, ok := LocateQuote(text, "Cuối câu dài hơn")
	if !ok {
		t.Fatal("LocateQuote = not found")
	}
	if got := text[start:end]; got != "Cuối câu dài hơn" {
		t.Fatalf("text[%d:%d] = %q", start, end, got)
	}
}

// TestLocateQuoteRejects 锁死两类失败：引文太短不具辨识度、引文与原文不同。
// 越南语附加符号不同就是不同一句话，不因规范化而误命中。
func TestLocateQuoteRejects(t *testing.T) {
	text := "Nàng nói: “Trời đã tối rồi.”"

	if _, _, ok := LocateQuote(text, "tối."); ok {
		t.Fatal("quote ngắn hơn 6 rune phải bị từ chối")
	}
	if _, _, ok := LocateQuote(text, "Trời đã sáng rồi"); ok {
		t.Fatal("câu khác nội dung không được khớp")
	}
	// 去附加符号后的"khác"是不同文本。
	if _, _, ok := LocateQuote("Tiền đã mất.", "Tien da mat khac roi"); ok {
		t.Fatal("bỏ dấu tiếng Việt không được khớp")
	}
	if _, _, ok := LocateQuote(text, "một câu chưa từng viết ra"); ok {
		t.Fatal("câu không có trong chương không được khớp")
	}
	if _, _, ok := LocateQuote("", "câu đủ dài để tìm"); ok {
		t.Fatal("văn bản rỗng không được khớp")
	}
	if _, _, ok := LocateQuote(text, "   "); ok {
		t.Fatal("quote toàn khoảng trắng không hợp lệ")
	}
}

// TestNearestQuotePrefix pin 错误提示用的最近片段：前缀 ≥ 12 rune 时回映射到
// 原文开头那段；太短或完全不沾边时放弃提示。
func TestNearestQuotePrefix(t *testing.T) {
	text := "Nàng nói: “Trời đã tối rồi, em về trước đi.” Gió thổi."

	snippet, ok := NearestQuotePrefix(text, "Trời đã tối rồi, nhưng không ai về")
	if !ok {
		t.Fatal("NearestQuotePrefix = not found")
	}
	if snippet != "Trời đã tối rồi, " && !strings.HasPrefix(snippet, "Trời đã tối rồi") {
		t.Fatalf("snippet = %q", snippet)
	}

	// 规范化后命中的前缀映射回原文带弯引号的片段。
	if snippet, ok := NearestQuotePrefix(text, `"Trời đã tối rồi" không đúng lắm`); !ok || snippet != `“Trời đã tối rồi` {
		t.Fatalf("normalized snippet = %q %v", snippet, ok)
	}

	if _, ok := NearestQuotePrefix(text, "Câu hoàn toàn khác biệt không ăn nhập gì"); ok {
		t.Fatal("không có tiền tố chung đủ dài thì không gợi ý")
	}
	if _, ok := NearestQuotePrefix(text, "ngắn"); ok {
		t.Fatal("quote quá ngắn không gợi ý")
	}
}
