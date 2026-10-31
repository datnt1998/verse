package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/bulk"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	exporter "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/export"
	imagemodel "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
)

func approvedBulkManifest(revision int) imagemodel.Manifest {
	current := 1
	item := imagemodel.Item{ID: "thumbnail", Kind: "thumbnail", Width: 1280, Height: 720, Status: "approved", CurrentVersion: &current}
	item.Versions = []imagemodel.Version{{Version: 1, Path: "images/cover.webp", SourceFingerprint: imagemodel.SourceFingerprint(item)}}
	return imagemodel.Manifest{Version: 1, Revision: revision, ImageMode: "none", Items: []imagemodel.Item{item}}
}
func TestBulkApprovalDurablyResumesThroughRenderGateButNeverBypassesLowScore(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "bulk-owner-review")
	registry := pipeline.Registry{}
	for _, step := range domain.CloneSteps {
		registry[step] = pipeline.HandlerFunc(func(context.Context, *pipeline.StepContext) (pipeline.StepResult, error) {
			return pipeline.StepResult{}, nil
		})
	}
	registry[domain.CloneBuild] = pipeline.HandlerFunc(func(context.Context, *pipeline.StepContext) (pipeline.StepResult, error) {
		return pipeline.StepResult{Data: map[string]any{"projectId": "project", "projectRevision": "revision"}, Waiting: &domain.CloneError{Code: domain.CloneErrAwaitingReview, Message: "Chờ chủ duyệt"}}, nil
	})
	binding := quality.Binding{ProjectRevision: "revision", SettingsHash: "settings", ArtifactHash: "artifacts"}
	score := quality.Score{Version: 1, Total: 89, Passed: false, Binding: binding}
	exportCalls := 0
	registry[domain.CloneExport] = exporter.Handler{Deps: exporter.Deps{Gate: func(context.Context, string) (quality.GateDecision, error) {
		return quality.EvaluateGate(&score, &binding, false), nil
	}, Export: func(context.Context, string, func(render.ExportProgress, bool)) (editor.ExportRecord, render.ExportLaunchResult, error) {
		exportCalls++
		return editor.ExportRecord{}, render.ExportLaunchResult{}, nil
	}}}
	runner := &pipeline.Runner{Store: store, Handlers: registry, StagingRoot: t.TempDir()}
	if out, err := runner.RunJob(ctx, job.ID, "initial"); err != nil || out != domain.CloneWaiting {
		t.Fatalf("initial build: %+v %v", out, err)
	}
	before, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	manifest := approvedBulkManifest(2)
	raw, _ := json.Marshal(manifest)
	if err = store.Commit(ctx, before, jobedit.Change{Checkpoint: raw, InvalidateFrom: domain.CloneBuild, ApproveExport: true}); err != nil {
		t.Fatal(err)
	}
	intents, err := store.ListBulkReviewIntents(ctx, 100)
	if err != nil || len(intents) != 0 {
		t.Fatalf("intent must wait for rebuild: %+v %v", intents, err)
	}
	if out, err := runner.RunJob(ctx, job.ID, "rebuilder"); err != nil || out != domain.CloneWaiting {
		t.Fatalf("rebuild: %+v %v", out, err)
	}
	// A fresh dispatcher represents a restarted process; the owner action lives
	// in PostgreSQL, not in an HTTP goroutine or a process-local callback.
	dispatch := &bulk.Service{Approvals: store, ApproveProject: store.PrepareCloneExport}
	if err = dispatch.DispatchReviews(ctx); err != nil {
		t.Fatal(err)
	}
	records, err := store.ListSteps(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Step == domain.CloneBuild && record.Status != domain.CloneSucceeded {
			t.Fatal("owner gate not applied")
		}
		if (record.Step == domain.CloneScore || record.Step == domain.CloneExport) && record.Status != domain.ClonePending {
			t.Fatalf("downstream not queued: %+v", record)
		}
	}
	intents, err = store.ListBulkReviewIntents(ctx, 100)
	if err != nil || len(intents) != 0 {
		t.Fatalf("consumed intent retained: %+v %v", intents, err)
	}
	if out, err := runner.RunJob(ctx, job.ID, "scorer"); err != nil || out != domain.CloneFailed {
		t.Fatalf("low score bypass: %+v %v", out, err)
	}
	failed, err := store.GetStep(ctx, job.ID, domain.CloneExport)
	if err != nil || failed.Error == nil || failed.Error.Code != domain.CloneErrQualityGate {
		t.Fatalf("export gate error: %+v %v", failed, err)
	}
	if exportCalls != 0 {
		t.Fatal("bulk owner approval bypassed score gate and rendered")
	}
}
