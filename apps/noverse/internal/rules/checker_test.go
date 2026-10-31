package rules

import (
	"strings"
	"testing"
)

// findViolation 在结果中按 rule + target 查找第一条违规。
func findViolation(vs []Violation, rule, target string) *Violation {
	for i := range vs {
		if vs[i].Rule == rule && vs[i].Target == target {
			return &vs[i]
		}
	}
	return nil
}

func TestCheck_EmptyStructured(t *testing.T) {
	vs := Check("Bất kỳ nội dung nào", Structured{})
	if vs != nil {
		t.Errorf("empty structured should return nil, got %+v", vs)
	}
}

func TestCheck_ForbiddenChars(t *testing.T) {
	text := "Anh cười——rồi thở dài——bỏ đi."
	vs := Check(text, Structured{
		ForbiddenChars: []string{"——"},
	})
	v := findViolation(vs, "forbidden_chars", "——")
	if v == nil {
		t.Fatal("expected forbidden_chars violation")
	}
	if v.Severity != SeverityError {
		t.Errorf("severity=%s, want error", v.Severity)
	}
	if v.Actual != 2 {
		t.Errorf("actual=%v, want 2", v.Actual)
	}
}

func TestCheck_ForbiddenCharsNotPresent(t *testing.T) {
	vs := Check("Văn bản thường, không vi phạm", Structured{
		ForbiddenChars: []string{"——"},
	})
	if len(vs) != 0 {
		t.Errorf("expected no violations, got %+v", vs)
	}
}

func TestCheck_ForbiddenPhrases(t *testing.T) {
	text := "Không phải…… mà là sự thật bị che lấp. Ở đây bàn tới động cơ cốt lõi."
	vs := Check(text, Structured{
		ForbiddenPhrases: []string{"Không phải…… mà là", "động cơ cốt lõi"},
	})
	if len(vs) != 2 {
		t.Errorf("expected 2 violations, got %d: %+v", len(vs), vs)
	}
	for _, v := range vs {
		if v.Severity != SeverityError {
			t.Errorf("severity=%s, want error", v.Severity)
		}
	}
}

func TestCheck_FatigueWordsUnderLimit(t *testing.T) {
	// per_1000=1, chương ngắn ~5 tiếng dùng 1 lần → sàn "ít nhất 1 lần" cho qua.
	text := "Anh không khỏi bật cười."
	vs := Check(text, Structured{
		FatigueWords: map[string]FatiguePer1000{"không khỏi": 1},
	})
	if len(vs) != 0 {
		t.Errorf("under limit should not violate, got %+v", vs)
	}
}

func TestCheck_FatigueWordsOverLimit(t *testing.T) {
	// per_1000=1, chương ngắn (~13 tiếng) dùng 3 lần: mật độ ~231/1.000 tiếng → warning.
	text := "Anh không khỏi bật cười, rồi không khỏi nhíu mày, cuối cùng không khỏi bỏ đi."
	vs := Check(text, Structured{
		FatigueWords: map[string]FatiguePer1000{"không khỏi": 1},
	})
	v := findViolation(vs, "fatigue_words", "không khỏi")
	if v == nil {
		t.Fatal("expected fatigue_words violation")
	}
	if v.Severity != SeverityWarning {
		t.Errorf("severity=%s, want warning", v.Severity)
	}
	if v.Limit != 1 {
		t.Errorf("limit=%v, want 1 (số lần được phép ở chương này)", v.Limit)
	}
	if v.Per1000 != 1.0 {
		t.Errorf("per_1000=%v, want 1", v.Per1000)
	}
	if v.Actual != 3 {
		t.Errorf("actual=%v, want 3", v.Actual)
	}
	if v.Message == "" {
		t.Error("vi phạm fatigue_words phải kèm message tiếng Việt nêu mật độ")
	}
}

// Limit của vi phạm fatigue_words là số lần ĐƯỢC PHÉP ở chương đó (int) — giao diện cũ
// hiển thị "tối đa N" đúng nghĩa; ngưỡng mật độ nằm ở trường per_1000.
func TestCheck_FatigueLimitIsAllowedCount(t *testing.T) {
	// per_1000=0.8 × 3.000 tiếng = 2,4 → được phép 2 lần; dùng 3 lần là vượt.
	s := Structured{FatigueWords: map[string]FatiguePer1000{"một chút": 0.8}}
	vs := Check(chapterText("một chút", 3, 3000), s)
	v := findViolation(vs, "fatigue_words", "một chút")
	if v == nil {
		t.Fatal("3 lần/3.000 tiếng tại per_1000=0.8 phải vi phạm")
	}
	if v.Limit != 2 {
		t.Errorf("limit=%v, want 2 (số lần được phép, int)", v.Limit)
	}
	if v.Per1000 != 0.8 {
		t.Errorf("per_1000=%v, want 0.8", v.Per1000)
	}
}

func TestCheck_MultipleRulesAtOnce(t *testing.T) {
	text := "Anh không khỏi——rồi không khỏi——bỏ đi."
	s := Structured{
		ForbiddenChars: []string{"——"},
		FatigueWords:   map[string]FatiguePer1000{"không khỏi": 1},
	}
	vs := Check(text, s)

	// 应同时触发两类：forbidden_chars + fatigue_words
	rules := map[string]bool{}
	for _, v := range vs {
		rules[v.Rule] = true
	}
	if !rules["forbidden_chars"] || !rules["fatigue_words"] {
		t.Errorf("expected both rules triggered, got %+v", rules)
	}
}

func TestCheck_FatigueZeroLimitRejectsAnyUse(t *testing.T) {
	vs := Check("Anh không khỏi bật cười.", Structured{
		FatigueWords: map[string]FatiguePer1000{"không khỏi": 0},
	})
	v := findViolation(vs, "fatigue_words", "không khỏi")
	if v == nil || v.Limit != 0 || v.Actual != 1 || v.Severity != SeverityWarning {
		t.Fatalf("zero must reject even one use: %+v", vs)
	}
	if clean := Check("Anh bật cười.", Structured{FatigueWords: map[string]FatiguePer1000{"không khỏi": 0}}); len(clean) != 0 {
		t.Fatalf("unused word must pass: %+v", clean)
	}
}

func TestCheck_EmptyTargetsSkipped(t *testing.T) {
	// 空字符串目标不应导致 false positive
	vs := Check("Bất kỳ văn bản", Structured{
		ForbiddenChars:   []string{""},
		ForbiddenPhrases: []string{""},
		FatigueWords:     map[string]FatiguePer1000{"": 1},
	})
	if len(vs) != 0 {
		t.Errorf("empty targets should be skipped, got %+v", vs)
	}
}

// chapterText 拼一段恰好 tieng 个音节的正文，其中 word 出现 uses 次
// （word 自身的音节计入总数；填充物是不会撞上目标的 "x"）。
func chapterText(word string, uses, tieng int) string {
	parts := make([]string, 0, tieng)
	filler := tieng - uses*len(strings.Fields(word))
	for range filler {
		parts = append(parts, "x")
	}
	for range uses {
		parts = append(parts, word)
	}
	return strings.Join(parts, " ")
}

// Nghiệm thu P1-5: cùng một mật độ dùng từ phải cho cùng một kết quả bất kể chương
// dài ngắn — 2 lần/1.500 tiếng và 4 lần/3.000 tiếng (cùng 1,33 lần/1.000) phải giống nhau.
func TestCheck_FatigueSameDensitySameVerdict(t *testing.T) {
	s := Structured{FatigueWords: map[string]FatiguePer1000{"một chút": 1.33}}
	cases := []struct {
		tieng, uses int
		want        bool
	}{
		{1500, 1, false}, // dưới ngưỡng
		{3000, 2, false}, // cùng mật độ với 1/1.500 → cũng dưới ngưỡng
		{1500, 2, true},  // 2 > 1,33×1,5=1,995 → vượt
		{3000, 4, true},  // 4 > 1,33×3=3,99 → vượt, cùng mật độ cùng kết quả
		{3000, 3, false}, // 3 ≤ 3,99: vừa dưới biên → không vượt
	}
	for _, c := range cases {
		got := findViolation(Check(chapterText("một chút", c.uses, c.tieng), s), "fatigue_words", "một chút") != nil
		if got != c.want {
			t.Errorf("%d lần/%d tiếng: violation=%v, muốn %v", c.uses, c.tieng, got, c.want)
		}
	}
}

// Sàn "ít nhất 1 lần": chương rất ngắn vẫn được dùng đúng 1 lần dù mật độ vượt xa ngưỡng.
func TestCheck_FatigueAtLeastOneAllowed(t *testing.T) {
	s := Structured{FatigueWords: map[string]FatiguePer1000{"một chút": 0.1}}
	if vs := Check(chapterText("một chút", 1, 20), s); findViolation(vs, "fatigue_words", "một chút") != nil {
		t.Errorf("1 lần trong chương 20 tiếng không được báo vi phạm, got %+v", vs)
	}
	if vs := Check(chapterText("một chút", 2, 20), s); findViolation(vs, "fatigue_words", "một chút") == nil {
		t.Error("2 lần trong chương 20 tiếng phải vi phạm")
	}
}

// Message vi phạm phải nêu mật độ theo văn phong tiếng Việt:
// “một chút” 3 lần/1.400 tiếng, vượt 1,33 lần/1.000 tiếng.
func TestCheck_FatigueMessageStatesDensity(t *testing.T) {
	s := Structured{FatigueWords: map[string]FatiguePer1000{"một chút": 1.33}}
	vs := Check(chapterText("một chút", 3, 1400), s)
	v := findViolation(vs, "fatigue_words", "một chút")
	if v == nil {
		t.Fatal("expected fatigue_words violation")
	}
	want := "“một chút” 3 lần/1.400 tiếng, vượt 1,33 lần/1.000 tiếng"
	if v.Message != want {
		t.Errorf("message=%q, want %q", v.Message, want)
	}
}
