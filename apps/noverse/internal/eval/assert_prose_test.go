package eval

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
)

func proseCase(prose *ProseExpect, facts *FactsExpect) Case {
	return Case{
		ID:          "wr-prose",
		Fixture:     "wr-prose",
		Instruction: &Instruction{Agent: "writer", Task: "Viết chương 3"},
		Expect:      Expect{Prose: prose, Facts: facts},
	}
}

func TestGradeProseRegex(t *testing.T) {
	c := proseCase(&ProseExpect{
		Chapter:      3,
		ForbidRegex:  []string{`(?i)sáng hôm sau`},
		RequireRegex: []string{`đồng hồ`},
	}, nil)
	col := Collected{ChapterTexts: map[int]string{3: "Sáng hôm sau, cô mở hộp."}}
	res := Result{}
	gradeProseExpect(c, col, &res)
	if !hasIssue(res.HardFails, "contract:prose:forbid", "(?i)sáng hôm sau") {
		t.Fatalf("命中禁用正则应 hard fail: %+v", res.HardFails)
	}
	if !hasIssue(res.HardFails, "contract:prose:require", "đồng hồ") {
		t.Fatalf("缺少必需正则应 hard fail: %+v", res.HardFails)
	}
	if res.WriterViolations != 2 {
		t.Fatalf("writer_violations = %d want 2", res.WriterViolations)
	}
}

func TestGradeProsePasses(t *testing.T) {
	c := proseCase(&ProseExpect{
		Chapter:      3,
		ForbidRegex:  []string{`(?i)sáng hôm sau`},
		RequireRegex: []string{`đồng hồ`},
	}, nil)
	col := Collected{ChapterTexts: map[int]string{3: "Chiều muộn, chiếc đồng hồ trên quầy điểm ba tiếng."}}
	res := Result{}
	gradeProseExpect(c, col, &res)
	if len(res.HardFails) != 0 || res.WriterViolations != 0 {
		t.Fatalf("不应有违例: %+v", res.HardFails)
	}
	if len(res.Passed) != 2 {
		t.Fatalf("两条断言都应记录通过，得到 %+v", res.Passed)
	}
}

func TestGradeProseMissingChapterFailsLoud(t *testing.T) {
	c := proseCase(&ProseExpect{Chapter: 3}, nil)
	res := Result{}
	gradeProseExpect(c, Collected{}, &res)
	if !hasIssue(res.HardFails, "contract:prose", "không đọc được văn bản") {
		t.Fatalf("读不到正文应 hard fail: %+v", res.HardFails)
	}
	if res.WriterViolations != 1 {
		t.Fatalf("writer_violations = %d want 1", res.WriterViolations)
	}
}

func TestGradeProseOpeningOverlap(t *testing.T) {
	prev := "Mưa rơi trên mái ngói cũ, Lâm Vãn khép cửa tiệm rồi thắp đèn."
	replayed := "Mưa rơi trên mái ngói cũ, Lâm Vãn khép cửa tiệm rồi đi vào trong."
	c := proseCase(&ProseExpect{Chapter: 3, MaxOpeningOverlapRunes: 20}, nil)

	res := Result{}
	gradeProseExpect(c, Collected{ChapterTexts: map[int]string{2: prev, 3: replayed}}, &res)
	if !hasIssue(res.HardFails, "contract:prose:opening_overlap", "lặp") {
		t.Fatalf("开篇复读应 hard fail: %+v", res.HardFails)
	}
	if res.WriterViolations != 1 {
		t.Fatalf("writer_violations = %d want 1", res.WriterViolations)
	}

	distinct := Result{}
	gradeProseExpect(c, Collected{ChapterTexts: map[int]string{
		2: prev,
		3: "Tô Khiết đẩy cửa bước vào, tay ôm chiếc hộp gỗ nặng trịch.",
	}}, &distinct)
	if len(distinct.HardFails) != 0 {
		t.Fatalf("开篇不同不应报错: %+v", distinct.HardFails)
	}
}

func TestGradeProseOpeningOverlapNeedsPreviousChapter(t *testing.T) {
	c := proseCase(&ProseExpect{Chapter: 3, MaxOpeningOverlapRunes: 20}, nil)
	res := Result{}
	gradeProseExpect(c, Collected{ChapterTexts: map[int]string{3: "Một đoạn mới."}}, &res)
	if !hasIssue(res.HardFails, "contract:prose:opening_overlap", "thiếu văn bản chương 2") {
		t.Fatalf("缺上一章应 hard fail: %+v", res.HardFails)
	}

	first := Result{}
	gradeProseExpect(proseCase(&ProseExpect{Chapter: 1, MaxOpeningOverlapRunes: 20}, nil),
		Collected{ChapterTexts: map[int]string{1: "Chương đầu tiên."}}, &first)
	if len(first.HardFails) != 0 {
		t.Fatalf("第 1 章没有可比的上一章，不应报错: %+v", first.HardFails)
	}
}

func TestGradeFactsForbidContains(t *testing.T) {
	c := proseCase(nil, &FactsExpect{Chapter: 4, ForbidContains: []string{"đã đến", "tới nơi"}})
	facts := domain.ChapterFacts{
		Title:     "Khách lạ",
		Summary:   "Lâm Vãn đã đến cửa tiệm phía đông.",
		KeyEvents: []string{"Lâm Vãn quay về tiệm."},
	}
	res := Result{}
	gradeFactsExpect(c, Collected{ChapterFacts: map[int]domain.ChapterFacts{4: facts}}, &res)
	if !hasIssue(res.HardFails, "contract:facts:forbid", "đã đến") {
		t.Fatalf("禁用事实应 hard fail: %+v", res.HardFails)
	}
	if res.WriterViolations != 1 {
		t.Fatalf("writer_violations = %d want 1", res.WriterViolations)
	}

	clean := Result{}
	gradeFactsExpect(c, Collected{ChapterFacts: map[int]domain.ChapterFacts{4: {Title: "Khách lạ", Summary: "Lâm Vãn định đi."}}}, &clean)
	if len(clean.HardFails) != 0 {
		t.Fatalf("事实干净不应报错: %+v", clean.HardFails)
	}
}

func TestGradeFactsMissingRecordFailsLoud(t *testing.T) {
	c := proseCase(nil, &FactsExpect{Chapter: 4, ForbidContains: []string{"đã đến"}})
	res := Result{}
	gradeFactsExpect(c, Collected{}, &res)
	if !hasIssue(res.HardFails, "contract:facts", "không có bản ghi") {
		t.Fatalf("缺事实记录应 hard fail: %+v", res.HardFails)
	}
}
