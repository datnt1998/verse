package script

import (
	"strings"
	"testing"
)

func TestWords(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"đơn giản", "Đế chế La Mã", []string{"đế", "chế", "la", "mã"}},
		{"nháy đầu từ bị tách", "don't stop 'em", []string{"don't", "stop", "em"}},
		{"nháy đầu từ bị tách", "rock 'n' roll", []string{"rock", "n", "roll"}},
		{"fact ref không tính", "xem [f:123e4567-e89b-42d3-a456-426614174000] đây", []string{"xem", "đây"}},
		{"số là từ", "năm 2025 có 365 ngày", []string{"năm", "2025", "có", "365", "ngày"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Words(tc.in)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("Words(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSpokenTextAndFullText(t *testing.T) {
	sections := []Section{
		{Kind: "hook", Narration: "lời hook"},
		{Kind: "part", Narration: "tóm tắt", Scenes: []Scene{{Narration: "cảnh một"}, {Narration: "cảnh hai"}}},
		{Kind: "ending_cta", Narration: "kêu gọi"},
	}
	if got := SpokenText(sections[1]); got != "cảnh một\ncảnh hai" {
		t.Fatalf("spoken của phần có cảnh = %q", got)
	}
	want := "lời hook\n\ncảnh một\ncảnh hai\n\nkêu gọi"
	if got := FullText(sections); got != want {
		t.Fatalf("FullText = %q, want %q", got, want)
	}
}

func TestMeasureScript(t *testing.T) {
	sections := []Section{
		{Kind: "hook", Narration: "một hai ba"},                                      // 3 từ → 1.2s
		{Kind: "part", Narration: "bỏ qua", Scenes: []Scene{{Narration: "bốn năm"}}}, // 2 từ → 0.8s
	}
	m := MeasureScript(sections, 150)
	if m.WordCount != 5 {
		t.Fatalf("word_count = %d, want 5", m.WordCount)
	}
	if m.EstDurationSec != 2 {
		t.Fatalf("est_duration = %v, want 2", m.EstDurationSec)
	}
	if len(m.Sections) != 2 || m.Sections[0].StartSec != 0 || m.Sections[1].StartSec != 1.2 {
		t.Fatalf("section measures = %+v", m.Sections)
	}
}

func TestValidate(t *testing.T) {
	ok := []Section{{Kind: "hook", Narration: "x", Scenes: []Scene{}}}
	if err := Validate(ok); err != nil {
		t.Fatalf("kịch bản hợp lệ bị từ chối: %v", err)
	}
	for _, tc := range []struct {
		name     string
		sections []Section
	}{
		{"rỗng", nil},
		{"kind hoa", []Section{{Kind: "Hook", Narration: "x"}}},
		{"kind parts", []Section{{Kind: "parts", Narration: "x"}}},
		{"kind dấu", []Section{{Kind: "mở đầu", Narration: "x"}}},
		{"quá nhiều cảnh", []Section{{Kind: "part", Narration: "x", Scenes: make([]Scene, MaxScenes+1)}}},
	} {
		if err := Validate(tc.sections); err == nil {
			t.Errorf("%s: phải bị từ chối", tc.name)
		}
	}
}

func TestBriefValidate(t *testing.T) {
	if err := (Brief{Topic: "x", TargetSeconds: 60}).Validate(); err != nil {
		t.Fatalf("brief seconds hợp lệ: %v", err)
	}
	if err := (Brief{Topic: "x", TargetWords: 300}).Validate(); err != nil {
		t.Fatalf("brief words hợp lệ: %v", err)
	}
	for _, b := range []Brief{
		{},
		{Topic: "x"},
		{Topic: "x", TargetSeconds: 60, TargetWords: 300},
		{Topic: "x", TargetWords: -5},
	} {
		if err := b.Validate(); err == nil {
			t.Errorf("brief %+v phải bị từ chối", b)
		}
	}
}

func TestTargetDescription(t *testing.T) {
	sec := Brief{Topic: "x", TargetSeconds: 90, WordsPerMinute: 120}
	if got := sec.TargetDescription(); got != "khoảng 90 giây (≈ 180 từ ở 120 từ/phút)" {
		t.Fatalf("mô tả seconds = %q", got)
	}
	words := Brief{Topic: "x", TargetWords: 700}
	if got := words.TargetDescription(); got != "khoảng 700 từ" {
		t.Fatalf("mô tả words = %q", got)
	}
}

func TestFactRefs(t *testing.T) {
	got := FactRefs("a [f:123e4567-e89b-42d3-a456-426614174000] b [f:123e4567-e89b-42d3-a456-426614174000] c")
	if len(got) != 1 {
		t.Fatalf("fact refs = %v, want một id không trùng", got)
	}
}
