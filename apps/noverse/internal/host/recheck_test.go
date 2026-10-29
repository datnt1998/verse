package host

import (
	"context"
	"fmt"
	"testing"

	agentcore "github.com/datnt1998/noverse/internal/agent"
	"github.com/datnt1998/noverse/internal/agent/subagent"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/flow"
	llmtypes "github.com/datnt1998/noverse/internal/llm"
	"github.com/datnt1998/noverse/internal/llmcontract"
	"github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
)

func TestRecheckLifecycleHasTwoRewritesAndOneRecheck(t *testing.T) {
	s := store.NewStore(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Init())
	must(s.Progress.Init(3))
	must(s.Progress.UpdatePhase(domain.PhaseWriting))
	quote := "Cánh cửa đã bị khóa từ phía bên ngoài."
	content := quote + " Lan bước thẳng ra sân mà không mở cửa."
	facts := domain.ChapterFacts{Title: "Cánh cửa", Summary: "Lan ra sân", KeyEvents: []string{"Ra sân"}}
	must(s.Drafts.SaveFinalChapter(1, content))
	_, err := s.ChapterRecords.Accept(1, domain.ChapterOriginGenerated, content, facts, domain.StyleDelta{})
	must(err)
	must(s.Progress.MarkChapterComplete(1, 100, "", ""))
	must(s.Outline.SaveOutline([]domain.OutlineEntry{{Chapter: 1, Title: "Cửa"}, {Chapter: 2, Title: "Sân"}, {Chapter: 3, Title: "Đường"}}))
	review := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Summary: "Cần mở khóa", Dimensions: []domain.DimensionScore{{Dimension: "continuity", Score: 60, Comment: "Cửa chưa mở"}}, Issues: []domain.ConsistencyIssue{{Type: "continuity", Severity: "error", Description: "Cửa khóa ngoài nhưng Lan ra sân", Evidence: quote, Quote: quote, RequiresChange: true}}}
	saveReview := tools.NewSaveReviewTool(s, nil)
	_, err = saveReview.Execute(context.Background(), reviewArgs(t, review))
	must(err)
	source, err := s.World.LoadReview(1)
	must(err)
	recheck := review
	recheck.Recheck = true
	recheck.Summary = "Vẫn chưa sửa khóa"
	recheck.RecheckResults = []domain.RecheckResult{{FindingID: domain.ReviewEntryItemIDs(*source)[0], Status: "unfixed", Quote: quote, Note: "Đoạn lỗi còn nguyên"}}
	recheckArgs := reviewArgs(t, recheck)
	commitTool := tools.NewCommitChapterTool(s, tools.NewStyleStatsIndex(s))
	commitArgs, err := llmcontract.EncodeArgs(commitTool.Schema(), map[string]any{"chapter": 1, "title": "Cánh cửa", "summary": "Lan ra sân", "characters": []string{}, "key_events": []string{"Ra sân"}})
	must(err)
	writerCalls := 0
	rewrites := 0
	rechecks := 0
	writer := &scriptedChatModel{fn: func(_ []llmtypes.Message) llmtypes.Message {
		writerCalls++
		if writerCalls%2 == 1 {
			rewrites++
			return testToolCallMsg("draft_chapter", map[string]any{"chapter": 1, "mode": "write", "content": fmt.Sprintf("%s\nLan nhìn sân lần thứ %d.", content, rewrites)})
		}
		return testToolCallMsg("commit_chapter", commitArgs)
	}}
	editor := &scriptedChatModel{fn: func(_ []llmtypes.Message) llmtypes.Message {
		rechecks++
		return testToolCallMsg("save_review", recheckArgs)
	}}
	runner := subagent.NewRunner(
		subagent.Config{Name: "writer", Model: writer, SystemPrompt: "test", Tools: []agentcore.Tool{tools.NewDraftChapterTool(s), commitTool}, StopAfterTools: []string{"commit_chapter"}, MaxTurns: 4},
		subagent.Config{Name: "editor", Model: editor, SystemPrompt: "test", Tools: []agentcore.Tool{saveReview}, StopAfterTools: []string{"save_review"}, MaxTurns: 2},
	)
	e, _, _ := newTestEngine(t, s, runner, nil)
	instructions := 1 // initial chapter review above
	for ; instructions < 6; instructions++ {
		state, err := flow.LoadState(s)
		must(err)
		inst := flow.Route(state)
		if inst != nil && inst.Agent == "writer" && inst.Chapter == 2 {
			break
		}
		if inst == nil {
			t.Fatalf("stopped before next chapter: %+v", state.Progress)
		}
		must(e.runWorker(context.Background(), inst))
	}
	if instructions >= 6 || rewrites != 2 || rechecks != 1 {
		t.Fatalf("unbounded lifecycle: instructions=%d rewrites=%d rechecks=%d", instructions, rewrites, rechecks)
	}
	p, err := s.Progress.Load()
	must(err)
	if !p.ReviewWorkDrained() {
		t.Fatalf("queues not drained: %+v", p)
	}
	current, err := s.World.LoadReview(1)
	must(err)
	if !current.Recheck || len(current.Issues) != 1 || !current.Issues[0].RequiresChange {
		t.Fatalf("unresolved finding lost after last rewrite: %+v", current)
	}
}
