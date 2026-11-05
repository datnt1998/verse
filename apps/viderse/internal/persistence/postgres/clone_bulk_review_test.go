package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/bulk"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	exporter "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/export"
	imagemodel "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/jackc/pgx/v5/pgxpool"
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

func draftApprovalFixture(t *testing.T) (*postgres.CloneStore, *pgxpool.Pool) {
	t.Helper()
	pool := provision(t)
	seedCloneChannel(t, pool)
	return postgres.NewCloneStore(pool), pool
}

func draftApprovalJob(t *testing.T, store *postgres.CloneStore, key string, revision int, tweak func(pipeline.Registry)) (*domain.CloneJob, *pipeline.Runner) {
	t.Helper()
	ctx := context.Background()
	job := cloneTestJob(t, store, key)
	registry := pipeline.Registry{}
	for _, step := range domain.CloneSteps {
		registry[step] = pipeline.HandlerFunc(func(context.Context, *pipeline.StepContext) (pipeline.StepResult, error) {
			return pipeline.StepResult{}, nil
		})
	}
	registry[domain.CloneBuild] = pipeline.HandlerFunc(func(context.Context, *pipeline.StepContext) (pipeline.StepResult, error) {
		return pipeline.StepResult{Data: map[string]any{"projectId": "project", "projectRevision": "revision"}, Waiting: &domain.CloneError{Code: domain.CloneErrAwaitingReview, Message: "Chờ chủ duyệt"}}, nil
	})
	if tweak != nil {
		tweak(registry)
	}
	runner := &pipeline.Runner{Store: store, Handlers: registry, StagingRoot: t.TempDir()}
	if out, err := runner.RunJob(ctx, job.ID, "initial"); err != nil || out != domain.CloneWaiting {
		t.Fatalf("initial build: %+v %v", out, err)
	}
	before, err := store.Read(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(approvedBulkManifest(revision))
	if err = store.Commit(ctx, before, jobedit.Change{Checkpoint: raw}); err != nil {
		t.Fatal(err)
	}
	return job, runner
}

func TestApproveDraftOpensExistingBuildDraftWithoutRebuild(t *testing.T) {
	store, _ := draftApprovalFixture(t)
	ctx := context.Background()
	binding := quality.Binding{ProjectRevision: "revision", SettingsHash: "settings", ArtifactHash: "artifacts"}
	score := quality.Score{Version: 1, Total: 89, Passed: false, Binding: binding}
	exportCalls := 0
	job, runner := draftApprovalJob(t, store, "draft-owner-review", 2, func(registry pipeline.Registry) {
		registry[domain.CloneExport] = exporter.Handler{Deps: exporter.Deps{
			Gate: func(context.Context, string) (quality.GateDecision, error) {
				return quality.EvaluateGate(&score, &binding, false), nil
			},
			Export: func(context.Context, string, func(render.ExportProgress, bool)) (editor.ExportRecord, render.ExportLaunchResult, error) {
				exportCalls++
				return editor.ExportRecord{}, render.ExportLaunchResult{}, nil
			},
		}}
	})
	buildBefore, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	outputBefore, _ := json.Marshal(buildBefore.Output)
	if err = store.ApproveDraft(ctx, job.ID, 2); err != nil {
		t.Fatal(err)
	}
	buildAfter, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	outputAfter, _ := json.Marshal(buildAfter.Output)
	if buildAfter.Status != domain.CloneSucceeded || buildAfter.Attempt != buildBefore.Attempt || string(outputAfter) != string(outputBefore) {
		t.Fatalf("draft not reused: before=%+v after=%+v outputSame=%v", buildBefore, buildAfter, string(outputAfter) == string(outputBefore))
	}
	for _, step := range []domain.CloneStep{domain.CloneScore, domain.CloneExport} {
		record, err := store.GetStep(ctx, job.ID, step)
		if err != nil || record.Status != domain.ClonePending {
			t.Fatalf("%s not queued: %+v %v", step, record, err)
		}
	}
	images, err := store.GetStep(ctx, job.ID, domain.CloneImages)
	if err != nil {
		t.Fatal(err)
	}
	var manifest imagemodel.Manifest
	if err = json.Unmarshal(images.Checkpoint, &manifest); err != nil || manifest.Revision != 2 || len(manifest.Items) != 1 || manifest.Items[0].Status != "approved" {
		t.Fatalf("images checkpoint changed: %s %v", images.Checkpoint, err)
	}
	intents, err := store.ListBulkReviewIntents(ctx, 100)
	if err != nil || len(intents) != 0 {
		t.Fatalf("staged intent not consumed with the guarded transition: %+v %v", intents, err)
	}
	dispatch := &bulk.Service{Approvals: store, ApproveProject: store.PrepareCloneExport}
	if err = dispatch.DispatchReviews(ctx); err != nil {
		t.Fatal(err)
	}
	buildFinal, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil || buildFinal.Attempt != buildBefore.Attempt || buildFinal.Status != domain.CloneSucceeded {
		t.Fatalf("dispatcher rebuilt the draft: %+v %v", buildFinal, err)
	}
	imagesFinal, err := store.GetStep(ctx, job.ID, domain.CloneImages)
	if err != nil || string(imagesFinal.Checkpoint) != string(images.Checkpoint) {
		t.Fatalf("dispatcher touched images: %+v %v", imagesFinal, err)
	}
	if out, err := runner.RunJob(ctx, job.ID, "scorer"); err != nil || out != domain.CloneFailed {
		t.Fatalf("low score was not rejected: %+v %v", out, err)
	}
	failed, err := store.GetStep(ctx, job.ID, domain.CloneExport)
	if err != nil || failed.Error == nil || failed.Error.Code != domain.CloneErrQualityGate {
		t.Fatalf("export gate error: %+v %v", failed, err)
	}
	if exportCalls != 0 {
		t.Fatal("draft approval bypassed the score gate and rendered")
	}
}

func TestApproveDraftRejectsStaleImageRevision(t *testing.T) {
	store, pool := draftApprovalFixture(t)
	ctx := context.Background()
	job, _ := draftApprovalJob(t, store, "stale-draft", 2, nil)
	err := store.ApproveDraft(ctx, job.ID, 1)
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != domain.CloneErrStale {
		t.Fatalf("stale revision: %v", err)
	}
	var intents int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM clone_bulk_review_intents WHERE job_id=$1`, job.ID).Scan(&intents); err != nil || intents != 0 {
		t.Fatalf("stale approval recorded an intent: %d %v", intents, err)
	}
	build, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil || build.Status != domain.CloneWaiting {
		t.Fatalf("stale approval moved the draft: %+v %v", build, err)
	}
}

func TestApproveDraftKeepsCancelledOrSupersededJobsClosed(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ name, statement string }{
		{"cancelled", `UPDATE clone_jobs SET status='cancelled' WHERE id=$1`},
		{"superseded", `UPDATE clone_jobs SET superseded_at=now() WHERE id=$1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, pool := draftApprovalFixture(t)
			job, _ := draftApprovalJob(t, store, "closed-"+tc.name, 2, nil)
			if _, err := pool.Exec(ctx, tc.statement, job.ID); err != nil {
				t.Fatal(err)
			}
			if err := store.ApproveDraft(ctx, job.ID, 2); err == nil {
				t.Fatal("closed job accepted a draft approval")
			}
			var intents int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM clone_bulk_review_intents WHERE job_id=$1`, job.ID).Scan(&intents); err != nil || intents != 0 {
				t.Fatalf("closed job recorded intent: %d %v", intents, err)
			}
			build, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
			if err != nil || build.Status != domain.CloneWaiting {
				t.Fatalf("closed job moved the draft: %+v %v", build, err)
			}
		})
	}
}

func TestApproveDraftRepeatedWhileBuildRunningKeepsOneIntentAndAttempt(t *testing.T) {
	store, pool := draftApprovalFixture(t)
	ctx := context.Background()
	job, _ := draftApprovalJob(t, store, "running-draft", 2, nil)
	if _, err := pool.Exec(ctx, `UPDATE clone_steps SET status='running',error=NULL WHERE job_id=$1 AND step='build'`, job.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := store.ApproveDraft(ctx, job.ID, 2); err != nil {
			t.Fatal(err)
		}
	}
	after, err := store.GetStep(ctx, job.ID, domain.CloneBuild)
	if err != nil || after.Attempt != before.Attempt || after.Status != domain.CloneRunning {
		t.Fatalf("running draft changed: before=%+v after=%+v %v", before, after, err)
	}
	var intents, revision int
	if err = pool.QueryRow(ctx, `SELECT count(*), COALESCE(max(image_revision),0) FROM clone_bulk_review_intents WHERE job_id=$1`, job.ID).Scan(&intents, &revision); err != nil || intents != 1 || revision != 2 {
		t.Fatalf("intent not staged once: count=%d revision=%d %v", intents, revision, err)
	}
}
