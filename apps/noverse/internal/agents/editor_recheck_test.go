package agents

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	llmtypes "github.com/datnt1998/verse/apps/noverse/internal/llm"
	"github.com/datnt1998/verse/apps/noverse/internal/llmcontract"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
)

func TestEditorRecheckStopsThroughBuiltWorkers(t *testing.T) {
	s := store.NewStore(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Init())
	must(s.Progress.Init(3))
	must(s.Progress.MarkChapterComplete(1, 100, "", ""))
	source := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Verdict: "rewrite", Summary: "Cần sửa", Issues: []domain.ConsistencyIssue{{Severity: "error", Description: "Cửa khóa", Quote: "Cánh cửa bị khóa từ phía bên ngoài.", RequiresChange: true}}}
	must(s.World.SaveReview(source))
	must(s.Drafts.SaveFinalChapter(1, "Lan mở chốt cửa rồi đi xuống sân."))
	must(s.Progress.CompleteRewrite(1, true))
	entry := domain.ReviewEntry{Chapter: 1, Scope: "chapter", Recheck: true, Verdict: "accept", Summary: "Đã sửa", Issues: []domain.ConsistencyIssue{}, Dimensions: []domain.DimensionScore{{Dimension: "continuity", Score: 90, Comment: "Cửa mở được"}}, RecheckResults: []domain.RecheckResult{{FindingID: domain.ReviewEntryItemIDs(source)[0], Status: "fixed", Note: "Chốt đã mở từ bên trong"}}}
	args, err := llmcontract.EncodeArgs(tools.NewSaveReviewTool(s, nil).Schema(), entry)
	must(err)
	model := &contractModel{fn: func(i int, _ []llmtypes.Message) (*llmtypes.LLMResponse, error) {
		if i > 0 {
			t.Fatal("real editor guard requested an extra turn after recheck checkpoint")
		}
		return &llmtypes.LLMResponse{Message: assistantToolCall("save_review", string(args))}, nil
	}}
	models := &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("test", "test", model, nil)}
	bundle := assets.Load("default", assets.LoadOptions{})
	must(bundle.Err)
	runner, _, _ := BuildWorkers(bootstrap.Config{}, s, tools.NewStyleStatsIndex(s), models, bundle, nil, nil)
	_, err = runner.Run(context.Background(), "editor", "Kiểm lại chương 1: save_review(scope=chapter, recheck=true)")
	must(err)
	if model.calls() != 1 {
		t.Fatalf("model calls=%d", model.calls())
	}
	p, err := s.Progress.Load()
	must(err)
	if !p.ReviewWorkDrained() {
		t.Fatalf("real review did not drain queue: %+v", p)
	}
	saved, err := s.World.LoadReview(1)
	must(err)
	if saved == nil || !saved.Recheck {
		t.Fatalf("missing real recheck: %+v", saved)
	}
}
