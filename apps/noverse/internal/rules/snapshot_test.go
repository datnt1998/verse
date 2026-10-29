package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBuildSnapshot_FieldOverridePrecedence(t *testing.T) {
	// Thấp→cao: defaults đặt tiên hiệp, project ghi đè thành đô thị; bên ưu tiên cao thắng.
	snap := BuildSnapshot([]Candidate{
		{Source: "system_defaults", Structured: Structured{Genre: "tiên hiệp"}},
		{Source: "project:a.md", Structured: Structured{Genre: "đô thị"}},
	})
	if snap.Structured.Genre != "đô thị" {
		t.Fatalf("mong đợi project ghi đè defaults, nhận %q", snap.Structured.Genre)
	}
	if snap.Status != StatusReady {
		t.Fatalf("期望 ready，得到 %s", snap.Status)
	}
	if snap.Version != SnapshotVersion {
		t.Fatalf("version 应为 %d，得到 %d", SnapshotVersion, snap.Version)
	}
}

func TestBuildSnapshot_EmptyAndZeroAreAbsent(t *testing.T) {
	// Bộ chuẩn hoá nhả ra giá trị giữ chỗ: genre:"", phần tử chuỗi rỗng — đều phải coi là thiếu, không ghi đè giá trị thật của bên ưu tiên thấp.
	snap := BuildSnapshot([]Candidate{
		{Source: "system_defaults", Structured: Structured{
			Genre: "tiên hiệp",
		}},
		{Source: "startup_prompt", Structured: Structured{
			Genre:            "",                 // 占位空串 → 不覆盖
			ForbiddenPhrases: []string{"", "  "}, // 全空 → 丢弃
		}},
	})
	if snap.Structured.Genre != "tiên hiệp" {
		t.Fatalf("genre rỗng không được ghi đè, mong đợi tiên hiệp, nhận %q", snap.Structured.Genre)
	}
	if len(snap.Structured.ForbiddenPhrases) != 0 {
		t.Fatalf("全空 forbidden_phrases 应被丢弃，得到 %v", snap.Structured.ForbiddenPhrases)
	}
}

func TestBuildSnapshot_PreferencesPrecedenceOrder(t *testing.T) {
	snap := BuildSnapshot([]Candidate{
		{Source: "global:g.md", Preferences: "全局偏好"},
		{Source: "project:p.md", Preferences: "项目偏好"},
	})
	gi := strings.Index(snap.Preferences, "全局偏好")
	pi := strings.Index(snap.Preferences, "项目偏好")
	if gi < 0 || pi < 0 || gi > pi {
		t.Fatalf("preferences 应按优先级低→高拼接（项目在后），得到:\n%s", snap.Preferences)
	}
	if !strings.Contains(snap.Preferences, "## [global:g.md]") {
		t.Fatalf("preferences 应带来源标题，得到:\n%s", snap.Preferences)
	}
}

func TestBuildSnapshot_FatigueWordsMergeByWord(t *testing.T) {
	snap := BuildSnapshot([]Candidate{
		{Source: "system_defaults", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"bỗng nhiên": 0.67, "dường như": 1.33}}},
		{Source: "project:p.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"dường như": 3.33}}},
	})
	if snap.Structured.FatigueWords["bỗng nhiên"] != 0.67 {
		t.Fatalf("bỗng nhiên phải giữ ngưỡng 0.67 của defaults, nhận %v", snap.Structured.FatigueWords["bỗng nhiên"])
	}
	if snap.Structured.FatigueWords["dường như"] != 3.33 {
		t.Fatalf("dường như phải bị project ghi đè thành 3.33, nhận %v", snap.Structured.FatigueWords["dường như"])
	}
}

func TestBuildSnapshot_DegradedPropagates(t *testing.T) {
	snap := BuildSnapshot([]Candidate{
		{Source: "system_defaults", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"bỗng nhiên": 0.67}}},
		{Source: "project:bad.md", Preferences: "原文降级", Degraded: true},
	})
	if snap.Status != StatusDegraded {
		t.Fatalf("任一来源降级则 status=degraded，得到 %s", snap.Status)
	}
	// 降级来源仍以 raw preferences 进入，不阻断；其它来源 structured 照常。
	if len(snap.Structured.FatigueWords) == 0 {
		t.Fatalf("降级不应影响其它来源的 structured")
	}
	if !strings.Contains(snap.Preferences, "原文降级") {
		t.Fatalf("降级来源应作为 raw preferences 保留")
	}
}

func TestSystemDefaults_MatchesLegacyDefaultMD(t *testing.T) {
	d := SystemDefaults().Structured
	if len(d.ForbiddenPhrases) != 4 {
		t.Fatalf("默认禁语应为 4 条，得到 %d", len(d.ForbiddenPhrases))
	}
	if len(d.FatigueWords) != 16 {
		t.Fatalf("默认疲劳词应为 16 条，得到 %d", len(d.FatigueWords))
	}
}

// 内置基线必须是越南语套句：写作语种只剩越南语，中文词表永远匹配不上正文，
// 模型读到的 user_rules 也会被带回中文语境。
func TestSystemDefaultsAreVietnamese(t *testing.T) {
	d := SystemDefaults().Structured
	han := regexp.MustCompile(`\p{Han}`)
	for _, p := range d.ForbiddenPhrases {
		if han.MatchString(p) {
			t.Errorf("forbidden_phrase còn chữ Hán: %q", p)
		}
	}
	for w := range d.FatigueWords {
		if han.MatchString(w) {
			t.Errorf("fatigue_word còn chữ Hán: %q", w)
		}
	}
	if _, ok := d.FatigueWords["dường như"]; !ok {
		t.Errorf("thiếu mục 'dường như' trong baseline, got %v", d.FatigueWords)
	}
}

func TestBuildSnapshot_ForbiddenListsAreUnion(t *testing.T) {
	// Truyện thêm 1 cụm cấm không được xoá 4 cụm mặc định (trước đây thay nguyên cả danh sách).
	snap := BuildSnapshot([]Candidate{
		SystemDefaults(),
		{Source: "story:quy-tac.md", Structured: Structured{
			ForbiddenPhrases: []string{"đột nhiên", "đáng chú ý là"}, // trùng 1 cụm mặc định → dedupe
			ForbiddenChars:   []string{"~"},
		}},
	})
	want := []string{"theo một nghĩa nào đó", "đáng chú ý là", "không rõ vì sao", "không nói thành lời", "đột nhiên"}
	if strings.Join(snap.Structured.ForbiddenPhrases, "|") != strings.Join(want, "|") {
		t.Fatalf("cụm cấm phải là hợp (giữ thứ tự, bỏ trùng): want %v, got %v", want, snap.Structured.ForbiddenPhrases)
	}
	if len(snap.Structured.ForbiddenChars) != 1 || snap.Structured.ForbiddenChars[0] != "~" {
		t.Fatalf("ký tự cấm: %v", snap.Structured.ForbiddenChars)
	}
	// Không dùng chung mảng nền với ứng viên: sửa snapshot không làm bẩn SystemDefaults.
	snap.Structured.ForbiddenPhrases[0] = "x"
	if SystemDefaults().Structured.ForbiddenPhrases[0] == "x" {
		t.Fatal("snapshot không được chia sẻ mảng với ứng viên")
	}
}

func TestOverlaySnapshot_ForbiddenListsAreUnion(t *testing.T) {
	base := BuildSnapshot([]Candidate{SystemDefaults()})
	out := OverlaySnapshot(base, Candidate{Source: "runtime_update", Structured: Structured{ForbiddenPhrases: []string{"ngay lập tức"}}})
	if len(out.Structured.ForbiddenPhrases) != 5 {
		t.Fatalf("overlay phải giữ 4 cụm mặc định + 1 mới, got %v", out.Structured.ForbiddenPhrases)
	}
	if len(base.Structured.ForbiddenPhrases) != 4 {
		t.Fatalf("overlay không được sửa snapshot gốc: %v", base.Structured.ForbiddenPhrases)
	}
}

func TestParseRuleLines(t *testing.T) {
	got := ParseRuleLines("# Quy tắc\n\n- Không dùng \"đột nhiên\"\n* Tối đa 2 lần \"dường như\"\n  dòng thường  \n-\n")
	want := []string{"Không dùng \"đột nhiên\"", "Tối đa 2 lần \"dường như\"", "dòng thường"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", got)
	}
}

func TestRawFileSources_ManagedFileAccumulatesAcrossLevels(t *testing.T) {
	home, proj, story := t.TempDir(), t.TempDir(), t.TempDir()
	for dir, text := range map[string]string{home: "- g\n", proj: "- p\n", story: "- s\n"} {
		if err := os.WriteFile(filepath.Join(dir, ManagedRulesFile), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "khac.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srcs := RawFileSources(LoadOptions{HomeRulesDir: home, ProjectRulesDir: proj, StoryRulesDir: story})
	managed, other := 0, 0
	for _, s := range srcs {
		switch s.Name {
		case ManagedRulesFile:
			managed++
		case "khac.md":
			other++
		}
	}
	if managed != 3 || other != 1 {
		t.Fatalf("quy-tac.md phải cộng dồn 3 cấp, tệp khác vẫn gần-thắng: managed=%d other=%d", managed, other)
	}
}

// Snapshot/candidate cũ lưu fatigue_words là số trần = max_per_chapter (lần mỗi chương
// chuẩn 1.500 tiếng); đọc lại phải quy đổi sang per_1000: 2 → 1,33.
func TestFatiguePer1000_LegacyBareNumberConverts(t *testing.T) {
	var s Structured
	if err := json.Unmarshal([]byte(`{"fatigue_words":{"một chút":2,"bỗng nhiên":1}}`), &s); err != nil {
		t.Fatalf("unmarshal legacy map: %v", err)
	}
	if got := s.FatigueWords["một chút"].Per1000(); got != 1.33 {
		t.Errorf("max_per_chapter=2 → per_1000 %v, want 1.33", got)
	}
	if got := s.FatigueWords["bỗng nhiên"].Per1000(); got != 0.67 {
		t.Errorf("max_per_chapter=1 → per_1000 %v, want 0.67", got)
	}
	// Object chỉ có max_per_chapter cũng quy đổi như vậy.
	if err := json.Unmarshal([]byte(`{"fatigue_words":{"một chút":{"max_per_chapter":3}}}`), &s); err != nil {
		t.Fatalf("unmarshal legacy object: %v", err)
	}
	if got := s.FatigueWords["một chút"].Per1000(); got != 2.0 {
		t.Errorf("max_per_chapter=3 → per_1000 %v, want 2.0", got)
	}
}

// Hai khóa cùng có thì per_1000 thắng; ghi ra JSON luôn mang cả per_1000 và
// max_per_chapter (giá trị tương đương ở chương chuẩn) để khách cũ đọc được.
func TestFatiguePer1000_Per1000WinsAndRoundTrip(t *testing.T) {
	var s Structured
	if err := json.Unmarshal([]byte(`{"fatigue_words":{"một chút":{"per_1000":2.5,"max_per_chapter":9}}}`), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := s.FatigueWords["một chút"].Per1000(); got != 2.5 {
		t.Fatalf("per_1000 phải thắng max_per_chapter: got %v, want 2.5", got)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]map[string]map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("marshal output không phải JSON: %v", err)
	}
	entry := out["fatigue_words"]["một chút"]
	if entry["per_1000"] != 2.5 {
		t.Errorf("wire per_1000=%v, want 2.5", entry["per_1000"])
	}
	if entry["max_per_chapter"] != 3.0 {
		t.Errorf("wire max_per_chapter=%v, want 3 (tương đương ở chương 1.500 tiếng)", entry["max_per_chapter"])
	}
	// Đọc lại output chính nó phải giữ nguyên per_1000 (round-trip ổn định).
	var s2 Structured
	if err := json.Unmarshal(data, &s2); err != nil || s2.FatigueWords["một chút"].Per1000() != 2.5 {
		t.Fatalf("round-trip mất per_1000: %v err=%v", s2.FatigueWords["một chút"], err)
	}
}

// Điểm ghi đè cho gói thể loại: SystemDefaults(overrides) thay ngưỡng từng từ.
func TestSystemDefaults_FatigueOverrides(t *testing.T) {
	d := SystemDefaults(map[string]float64{"một chút": 5, "từ gói mới": 0.5}).Structured
	if got := d.FatigueWords["một chút"].Per1000(); got != 5 {
		t.Errorf("gói phải ghi đè một chút → 5, got %v", got)
	}
	if got := d.FatigueWords["từ gói mới"].Per1000(); got != 0.5 {
		t.Errorf("gói phải thêm từ mới → 0.5, got %v", got)
	}
	if got := d.FatigueWords["im lặng"].Per1000(); got != 1.33 {
		t.Errorf("từ không nằm trong gói giữ nguyên 1.33, got %v", got)
	}
}
