package eval

import (
	"strings"
	"testing"

	llmtypes "github.com/datnt1998/verse/core/llm"
)

func contextCase(needles ...string) Case {
	return Case{
		ID: "wr-thread-overdue", Fixture: "wr-thread-overdue",
		Instruction: &Instruction{Agent: "writer", Task: "Viết chương 3"},
		Expect:      Expect{ContextContains: needles},
	}
}

func TestGradeContextExpectPassesOnToolResult(t *testing.T) {
	col := Collected{ContextResults: []string{
		`{"selected_memory":{"story_threads":[{"key":"loi_hoi_1979","reason":"phục bút đã quá hạn thu"}]}}`,
	}}
	res := Result{}
	gradeContextExpect(contextCase("loi_hoi_1979", "quá hạn thu"), col, &res)
	if len(res.HardFails) != 0 {
		t.Fatalf("không được hard fail: %+v", res.HardFails)
	}
	if len(res.Passed) != 2 {
		t.Fatalf("muốn 2 pass, nhận %+v", res.Passed)
	}
}

func TestGradeContextExpectFailsWhenNeedleMissing(t *testing.T) {
	col := Collected{ContextResults: []string{
		`{"selected_memory":{"story_threads":[{"key":"loi_hoi_1979","reason":"phục bút ngủ quên"}]}}`,
	}}
	res := Result{}
	gradeContextExpect(contextCase("loi_hoi_1979", "quá hạn thu"), col, &res)
	if !hasIssue(res.HardFails, "contract:context:contains", "quá hạn thu") {
		t.Fatalf("thiếu chuỗi phải hard fail: %+v", res.HardFails)
	}
}

func TestGradeContextExpectFailsWithoutAnyContext(t *testing.T) {
	res := Result{}
	gradeContextExpect(contextCase("loi_hoi_1979"), Collected{}, &res)
	if !hasIssue(res.HardFails, "contract:context", "novel_context") {
		t.Fatalf("không có kết quả novel_context phải hard fail: %+v", res.HardFails)
	}
}

func TestGradeContextExpectSkipsWithoutExpectation(t *testing.T) {
	res := Result{}
	gradeContextExpect(Case{ID: "x"}, Collected{}, &res)
	if len(res.HardFails) != 0 || len(res.Passed) != 0 {
		t.Fatalf("không khai báo context_contains thì không chấm: %+v", res)
	}
}

// novel_context 结果原文要被采集进 Collected，其他工具的结果不算。
func TestCollectGathersNovelContextResults(t *testing.T) {
	dir := t.TempDir()
	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch03.jsonl", llmtypes.Message{
		Role: llmtypes.RoleTool,
		Content: []llmtypes.ContentBlock{
			llmtypes.TextBlock(`{"selected_memory":{"story_threads":[{"key":"loi_hoi_1979"}]}}`),
		},
		Metadata: map[string]any{"tool_name": "novel_context"},
	})
	writeSessionLine(t, dir, "meta/sessions/agents/writer-ch04.jsonl", llmtypes.Message{
		Role:     llmtypes.RoleTool,
		Content:  []llmtypes.ContentBlock{llmtypes.TextBlock(`{"selected_memory":{"story_threads":[{"key":"khong_lien_quan"}]}}`)},
		Metadata: map[string]any{"tool_name": "read_chapter"},
	})

	col := Collect(dir, nil)
	if len(col.ContextResults) != 1 {
		t.Fatalf("chỉ được gom kết quả novel_context: %+v", col.ContextResults)
	}
	if !strings.Contains(col.ContextResults[0], "loi_hoi_1979") {
		t.Fatalf("kết quả novel_context sai: %+v", col.ContextResults)
	}
}
