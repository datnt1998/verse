package images

import "testing"

// TestSplitWordsKeepsEveryWordOnce pins the word splitter behind DiffText: the
// diff of a redrawn scene must not repeat characters, so each word (with the
// whitespace in front of it) may appear exactly once.
func TestSplitWordsKeepsEveryWordOnce(t *testing.T) {
	text := "The shot and character data below were written"
	words := splitWords(text)
	if joined := joinWords(words); joined != text {
		t.Fatalf("splitWords must rebuild the text, got %q want %q", joined, text)
	}
	if len(words) != 8 {
		t.Fatalf("want 8 words, got %d: %q", len(words), words)
	}
	if words[0] != "The" || words[1] != " shot" || words[7] != " written" {
		t.Fatalf("unexpected split: %q", words)
	}
	// Khoảng trắng đứng trước từ được giữ nguyên khi ghép lại.
	if got := joinWords(splitWords("  a\tb\nc  ")); got != "  a\tb\nc  " {
		t.Fatalf("whitespace not preserved: %q", got)
	}
	if len(splitWords("")) != 0 {
		t.Fatal("empty text must split into no words")
	}
}

// TestDiffTextDoesNotDuplicateUnchangedWords: a redraw that only appends a
// clause must report the unchanged run once, then the insertion.
func TestDiffTextDoesNotDuplicateUnchangedWords(t *testing.T) {
	diff := DiffText("Khung cảnh phố cổ", "Khung cảnh phố cổ ban đêm")
	var kept, inserted string
	for _, segment := range diff {
		switch segment[1] {
		case "":
			kept += segment[0]
		case "ins":
			inserted += segment[0]
		case "del":
			t.Fatalf("no deletion expected: %q", segment[0])
		}
	}
	if kept != "Khung cảnh phố cổ" {
		t.Fatalf("unchanged run corrupted: %q", kept)
	}
	if inserted != " ban đêm" {
		t.Fatalf("inserted run: %q", inserted)
	}
}

func joinWords(words []string) string {
	out := ""
	for _, word := range words {
		out += word
	}
	return out
}
