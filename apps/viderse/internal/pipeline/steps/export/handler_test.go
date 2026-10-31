package export

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"testing"
)

func TestExportWorkerNeverRendersMissingScoreWithoutAcceptance(t *testing.T) {
	s, _ := stepstest.Ctx(t, domain.CloneExport, stepstest.Job("job"))
	called := false
	h := Handler{Deps: Deps{Gate: func(context.Context, string) (quality.GateDecision, error) {
		return quality.EvaluateGate(nil, nil, false), nil
	}, Export: func(context.Context, string, func(render.ExportProgress, bool)) (editor.ExportRecord, render.ExportLaunchResult, error) {
		called = true
		return editor.ExportRecord{}, render.ExportLaunchResult{}, nil
	}}}
	if _, err := h.Run(context.Background(), s); err == nil || called {
		t.Fatalf("missing score rendered: err=%v called=%v", err, called)
	}
}
func TestExportWorkerRecordsAcceptedOverride(t *testing.T) {
	s, _ := stepstest.Ctx(t, domain.CloneExport, stepstest.Job("job"))
	called := false
	h := Handler{Deps: Deps{Gate: func(context.Context, string) (quality.GateDecision, error) {
		return quality.EvaluateGate(nil, nil, true), nil
	}, Export: func(context.Context, string, func(render.ExportProgress, bool)) (editor.ExportRecord, render.ExportLaunchResult, error) {
		called = true
		return editor.ExportRecord{ID: "export", FileName: "fixture.mp4", Size: 10}, render.ExportLaunchResult{ProjectID: "project", ExportID: "export"}, nil
	}}}
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if !called || result.Data["accepted"] != true || result.Data["projectId"] != "project" {
		t.Fatalf("acceptance not recorded: %+v", result)
	}
}
