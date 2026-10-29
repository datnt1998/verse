package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// manifestSections dùng đúng nội dung goi.json thật, không giả lập kết quả đo.
type manifestSections []byte

func (m manifestSections) Section(key string, dest any) (bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(m, &raw); err != nil {
		return false, err
	}
	section, ok := raw[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(section, dest)
}

func measurementPack(t *testing.T) Candidate {
	t.Helper()
	data, err := os.ReadFile("../../assets/packs/nu-tan-bao-thu/goi.json")
	if err != nil {
		t.Fatal(err)
	}
	pack, err := PackDefaults(manifestSections(data))
	if err != nil {
		t.Fatal(err)
	}
	if pack.Structured.Measurements == nil {
		t.Fatal("gói A thiếu do_do")
	}
	return pack
}

func sampleEditor(t *testing.T) string {
	t.Helper()
	var chapters []string
	for chapter := 1; chapter <= 8; chapter++ {
		data, err := os.ReadFile(fmt.Sprintf("testdata/tra-xanh/%02d.txt", chapter))
		if err != nil {
			t.Fatal(err)
		}
		chapters = append(chapters, fmt.Sprintf("第 %d 章  %s", chapter, strings.TrimPrefix(strings.TrimSpace(string(data)), "# ")))
	}
	return "《Trà Xanh Cướp Kịch Bản Của Tôi, Tôi Lên Show Thực Tế Đọc Bản Gốc Cho Cả Nước Nghe》\n\n" + strings.Join(chapters, "\n\n\n")
}

// Golden sinh từ locals của main trong kiem_tra_truyen.py chạy nguyên bản:
// python3 docs/prd/rules/kiem_tra_truyen.py editor.txt --so-voi editor.txt
// Editor được ráp giống hệt tệp xuất thật, không sửa byte nội dung chương.
func TestMeasurementsOracle(t *testing.T) {
	pack := measurementPack(t)
	text := sampleEditor(t)
	got := Measure(text, *pack.Structured.Measurements, []Opening{{Name: "same-story", Text: text}})
	data, err := os.ReadFile("testdata/tra-xanh/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var want MeasurementResult
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	for i, chapter := range want.Chapters {
		if i >= len(got.Chapters) {
			t.Fatalf("missing chapter %d", i+1)
		}
		actual := got.Chapters[i]
		t.Logf("chapter %d: script/Go tiếng %d/%d; thoại %d/%d = %.0f%%/%.0f%%; thủ tục %d/%d", i+1, chapter.Words, actual.Words, chapter.DialogueWords, actual.DialogueWords, chapter.DialogueRatio*100, actual.DialogueRatio*100, chapter.Procedural, actual.Procedural)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("oracle mismatch\nGo: %#v\nPython: %#v", got, want)
	}
	if got.Chapters[6].DialogueWords != 475 || got.Chapters[6].Words != 1673 {
		t.Fatal("chương 7 phải có 28% thoại (475/1673)")
	}
	if !reflect.DeepEqual(got.Quiet, []QuietStretch{{Start: 725, End: 742, Words: 332}}) {
		t.Fatalf("quãng một phút: %v", got.Quiet)
	}
	t.Logf("global script/Go equal: English %v; ngày %v; câu %v; đoạn %v; cụm lặp %v; quiet %v; opening %v", got.English, got.DayLines, got.SentenceLines, got.ParagraphLines, got.Repeated, got.Quiet, got.Openings)
	byChapterData, err := os.ReadFile("testdata/tra-xanh/by-chapter.json")
	if err != nil {
		t.Fatal(err)
	}
	var byChapter []MeasurementResult
	if err := json.Unmarshal(byChapterData, &byChapter); err != nil {
		t.Fatal(err)
	}
	for i, expected := range byChapter {
		data, err := os.ReadFile(fmt.Sprintf("testdata/tra-xanh/%02d.txt", i+1))
		if err != nil {
			t.Fatal(err)
		}
		chapterText := fmt.Sprintf("第 %d 章  %s", i+1, strings.TrimPrefix(strings.TrimSpace(string(data)), "# "))
		actual := Measure(chapterText, *pack.Structured.Measurements, []Opening{{Name: "same-story", Text: chapterText}})
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("chapter %d oracle mismatch: Go %#v; Python %#v", i+1, actual, expected)
		}
		t.Logf("chapter %d script/Go equal: English %v; ngày %v; câu %v; đoạn %v; cụm lặp %v; quiet %v; opening %v", i+1, actual.English, actual.DayLines, actual.SentenceLines, actual.ParagraphLines, actual.Repeated, actual.Quiet, actual.Openings)
	}
}

func TestMeasurementDefinitions(t *testing.T) {
	config := *measurementPack(t).Structured.Measurements
	tests := []struct {
		name     string
		text     string
		config   Measurements
		openings []Opening
		value    func(MeasurementResult) any
		want     any
	}{
		{"dialogue", "“xin chào” đời thường", config, nil, func(m MeasurementResult) any { return m.Chapters[0].DialogueRatio }, 0.5},
		{"python whitespace tiếng", "“xin\u00a0chào” \u001cđời —", config, nil, func(m MeasurementResult) any { return m.Chapters[0].Words }, 4},
		{"procedural Unicode boundaries", "xác nhận, xác nhậnx _xác nhận xác nhận_ (xác nhận)", config, nil, func(m MeasurementResult) any { return m.Chapters[0].Procedural }, 2},
		{"quiet crosses chapter heading", "Chương 1: a\nmột hai\n\nChương 2: b\nba bốn năm\n“ngắt”\ncuối", Measurements{QuietMax: 3}, nil, func(m MeasurementResult) any { return m.Quiet }, []QuietStretch{{Start: 2, End: 5, Words: 5}}},
		{"day ordinal words not digits", "ngày thứ 2 ngày thứ haiX ngày thứ hai_ ngày thứ hai.\nNGÀY THỨ BẢY\nxngày thứ tư!", config, nil, func(m MeasurementResult) any { return m.DayLines }, []int{1, 2, 3}},
		{"English Unicode boundaries no mark floor", "credit creditx credit_ _credit àcredit credità (CREDIT)", config, nil, func(m MeasurementResult) any { return m.English }, map[string]int{"credit": 2}},
		{"sentence punctuation before whitespace", "một hai ba.\u00a0bốn năm\nmột hai ba?\" bốn năm", Measurements{SentenceMax: 3}, nil, func(m MeasurementResult) any { return m.SentenceLines }, []int{2}},
		{"paragraph means line", "một hai\nba bốn năm sáu", Measurements{ParagraphMax: 3}, nil, func(m MeasurementResult) any { return m.ParagraphLines }, []int{2}},
		{"repeated Unicode 5grams", "sắt thép đá gạch vôi.\nsắt-thép đá gạch vôi.\ntôi thép đá gạch vôi tôi thép đá gạch vôi", config, nil, func(m MeasurementResult) any { return m.Repeated }, []string{"sắt thép đá gạch vôi"}},
		{"opening strips rather than splits punctuation", "sắt-thép đá gạch vôi cát", config, []Opening{{Name: "cũ", Text: "sắtthép đá gạch vôi cát"}}, func(m MeasurementResult) any { return m.Openings }, []OpeningOverlap{{Name: "cũ", Ratio: 1}}},
		{"narration three newlines", "một hai\n\n\nba bốn", config, nil, func(m MeasurementResult) any { return len(m.Chapters) }, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.value(Measure(test.text, test.config, test.openings))
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v; script semantics want %#v", got, test.want)
			}
		})
	}
}

func TestPackWarningsAndPackOff(t *testing.T) {
	pack := measurementPack(t)
	text := sampleEditor(t)
	snapshot := BuildSnapshot([]Candidate{SystemDefaults()})
	before, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	active := Effective(&snapshot, pack)
	violations := Check(text, active, Opening{Name: "cũ", Text: text})
	counts := make(map[string]int)
	for _, violation := range violations {
		if violation.Rule == "fatigue_words" || violation.Rule == "forbidden_phrases" {
			continue
		}
		counts[violation.Rule]++
		if violation.Severity != SeverityWarning {
			t.Fatalf("pack metric must warn: %+v", violation)
		}
	}
	want := map[string]int{"dialogue_ratio": 5, "procedural_density": 6, "english_residue": 1, "day_ordinal": 1, "paragraph_length": 1, "repeated_5gram": 1, "quiet_stretch": 1, "opening_overlap": 1}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("pack warnings got %v want %v", counts, want)
	}
	// Mẫu thật không có câu dài; dùng một câu tổng hợp để kiểm tra cảnh báo.
	if got := Check(strings.Repeat("tiếng ", 41), active); len(got) == 0 || !hasRule(got, "sentence_length") {
		t.Fatalf("missing sentence warning: %v", got)
	}
	offText := "theo một nghĩa nào đó. ngày thứ hai credit\n" + strings.Repeat("xác nhận ", 11) + strings.Repeat("tiếng ", 251) + "\nsắt thép đá gạch vôi sắt thép đá gạch vôi"
	if got := CheckChapter(offText, Effective(&snapshot, Candidate{}), nil); len(got) != 1 || got[0].Rule != "forbidden_phrases" || got[0].Severity != SeverityError {
		t.Fatalf("pack-off changed legacy checks: %v", got)
	}
	after, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || snapshot.Structured.Measurements != nil {
		t.Fatal("pack values leaked into snapshot")
	}
	var previous []string
	for chapter := 1; chapter <= 7; chapter++ {
		data, err := os.ReadFile(fmt.Sprintf("testdata/tra-xanh/%02d.txt", chapter))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		checked := CheckChapter(content, active, previous)
		if chapter == 6 {
			found := false
			for _, v := range checked {
				if v.Rule == "quiet_stretch" {
					for _, q := range v.Actual.([]QuietStretch) {
						if q.Words == 332 {
							found = true
						}
					}
				}
			}
			if !found {
				t.Fatalf("missing cross-chapter 332-tiếng quiet stretch: %v", checked)
			}
		}
		if chapter == 7 {
			found := false
			for _, v := range checked {
				if v.Rule == "dialogue_ratio" && v.Actual == float64(475)/1673 {
					found = true
				}
			}
			if !found {
				t.Fatalf("chapter commit dialogue must exclude Markdown heading: %v", checked)
			}
		}
		previous = append([]string{content}, previous...)
	}
}

func hasRule(violations []Violation, name string) bool {
	for _, violation := range violations {
		if violation.Rule == name {
			return true
		}
	}
	return false
}

func TestLivePackOverridesWithoutSnapshotRebuild(t *testing.T) {
	snapshot := BuildSnapshot([]Candidate{SystemDefaults(), {Source: "story:quy-tac.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"im lặng": 1.33}}}})
	original, _ := json.Marshal(snapshot)
	load := func(data string) Candidate {
		t.Helper()
		pack, err := PackDefaults(manifestSections(data))
		if err != nil {
			t.Fatal(err)
		}
		return pack
	}
	first := load(`{"fatigue_words":{"một chút":4,"im lặng":5},"thoai":{"min":40,"max":55},"do_do":{"sentence_max":40}}`)
	second := load(`{"fatigue_words":{"một chút":6,"im lặng":7},"thoai":{"min":30,"max":45},"do_do":{"sentence_max":30}}`)
	for _, test := range []struct {
		pack     Candidate
		fatigue  float64
		sentence int
		dialogue float64
	}{{first, 4, 40, .4}, {second, 6, 30, .3}} {
		got := Effective(&snapshot, test.pack)
		if got.FatigueWords["một chút"].Per1000() != test.fatigue || got.FatigueWords["im lặng"].Per1000() != 1.33 || got.Measurements.SentenceMax != test.sentence || got.Measurements.DialogueMin != test.dialogue {
			t.Fatalf("live pack/user precedence wrong: %+v", got)
		}
	}
	serialized, _ := json.Marshal(snapshot)
	if string(original) != string(serialized) {
		t.Fatal("live pack changed persisted snapshot")
	}
	// Snapshot trước khi có provenance vẫn để gói thắng mặc định hệ thống.
	legacy := snapshot
	legacy.UserStructured = nil
	if got := Effective(&legacy, second).FatigueWords["một chút"].Per1000(); got != 6 {
		t.Fatalf("legacy baked defaults masked pack: %v", got)
	}
}

func TestSiblingOpeningsProjectScope(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	for name, text := range map[string]string{"new": "mới", "old": "cũ", "other": "khác"} {
		dir := filepath.Join(project, "stories", name)
		if err := os.MkdirAll(filepath.Join(dir, "output", "novel", "chapters"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "story.json"), []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "output", "novel", "chapters", "01.md"), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Dự án lồng nhau không phải truyện anh em thuộc dự án hiện tại.
	if err := os.WriteFile(filepath.Join(project, "stories", "other", "project.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(project, "stories", "new", "output", "novel")
	if got := SiblingOpenings(dir, 1); !reflect.DeepEqual(got, []Opening{{Name: "old", Text: "cũ"}}) {
		t.Fatalf("siblings got %v", got)
	}
	if got := SiblingOpenings(dir, 2); got != nil {
		t.Fatalf("not an opening: %v", got)
	}
}
