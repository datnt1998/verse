package score

import (
	"context"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func TestScoreRemeasuresGateOnceThenRetainsFailureRegions(t *testing.T) {
	s, m := stepstest.Ctx(t, domain.CloneScore, stepstest.Job("job"))
	attempts := []bool{}
	failed := &quality.Score{Version: 1, Total: 80, Items: []quality.Item{{ID: "captions", Status: "fail", Measured: map[string]any{"ranges": []quality.Range{{Start: 1, End: 2, CheckID: "captions", Status: "fail", CueID: "real-cue"}}}}}, Blockers: []quality.Blocker{{ID: "captions", Message: "Thiếu phụ đề"}}}
	handler := Handler{Deps: Deps{Measure: func(_ context.Context, _ string, force bool) (*quality.Score, error) {
		attempts = append(attempts, force)
		return failed, nil
	}}}
	_, err := handler.Run(context.Background(), s)
	var clone *domain.CloneError
	if !errors.As(err, &clone) || clone.Code != domain.CloneErrQualityGate {
		t.Fatalf("gate did not stop: %v", err)
	}
	if len(attempts) != 2 || attempts[0] || !attempts[1] {
		t.Fatalf("remeasure attempts=%v", attempts)
	}
	file, err := m.Acquire(context.Background(), "score/score.json")
	if err != nil {
		t.Fatal(err)
	}
	score, err := steps.DecodeJSONFile[quality.Score](file.Path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(score.Blockers) != 1 || score.Items[0].Measured == nil {
		t.Fatal("failed regions were discarded")
	}
}
func TestScoreRemeasurePassAdvancesToExport(t *testing.T) {
	s, _ := stepstest.Ctx(t, domain.CloneScore, stepstest.Job("job"))
	calls := 0
	handler := Handler{Deps: Deps{Measure: func(_ context.Context, _ string, force bool) (*quality.Score, error) {
		calls++
		if calls == 1 {
			return &quality.Score{Version: 1, Total: 80}, nil
		}
		if !force {
			t.Fatal("second round did not remeasure")
		}
		return &quality.Score{Version: 1, Total: 90, Passed: true, Items: []quality.Item{}, Blockers: []quality.Blocker{}}, nil
	}}}
	result, err := handler.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result.Waiting != nil || result.Data["passed"] != true || calls != 2 || len(result.Artifacts) != 1 {
		t.Fatalf("score did not advance: %+v", result)
	}
}
func TestScoreAcceptedChecklistDoesNotLoop(t *testing.T) {
	s, _ := stepstest.Ctx(t, domain.CloneScore, stepstest.Job("job"))
	calls := 0
	handler := Handler{Deps: Deps{Accepted: func(context.Context, string) (bool, error) { return true, nil }, Measure: func(context.Context, string, bool) (*quality.Score, error) {
		calls++
		return &quality.Score{Version: 1, Total: 80}, nil
	}}}
	result, err := handler.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data["accepted"] != true || calls != 1 {
		t.Fatalf("acceptance lost: %+v calls%d", result, calls)
	}
}
