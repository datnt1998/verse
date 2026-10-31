package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

func cloneStore(t *testing.T) *postgres.CloneStore {
	t.Helper()
	pool := provision(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO owner_channels(id,name,language,auto_pick) VALUES('11111111-1111-4111-8111-111111111111','Clone fixture','en',false)`); err != nil {
		t.Fatal(err)
	}
	return postgres.NewCloneStore(pool)
}
func cloneTestJob(t testing.TB, store *postgres.CloneStore, key string) *domain.CloneJob {
	t.Helper()
	result, err := store.StartJob(context.Background(), postgres.StartJobInput{ChannelID: "11111111-1111-4111-8111-111111111111", SourceKey: key, Source: domain.CloneSourceSnapshot{ID: key, Title: key, URL: "https://www.youtube.com/watch?v=" + key}, TemplateSnapshot: json.RawMessage(`{}`), ModelRefs: json.RawMessage(`{}`), Priority: domain.ClonePick, Origin: domain.OriginManual})
	if err != nil {
		t.Fatal(err)
	}
	return result.Job
}

func TestCloneStepCheckpointAndFencedPublication(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "checkpoint")
	if _, err := store.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || claim == nil {
		t.Fatalf("claim: %v %v", claim, err)
	}
	cp := json.RawMessage(`{"nextScene":2}`)
	if err := store.WriteStepCheckpoint(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, cp); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence+1, claim.Step.Attempt, domain.CloneStepOutput{}); !errors.Is(err, pipeline.ErrFence) {
		t.Fatalf("stale commit: %v", err)
	}
	rec, err := store.GetStep(ctx, job.ID, claim.Step.Step)
	if err != nil {
		t.Fatal(err)
	}
	if string(rec.Checkpoint) != string(cp) {
		var value map[string]int
		if json.Unmarshal(rec.Checkpoint, &value) != nil || value["nextScene"] != 2 {
			t.Fatalf("checkpoint lost: %s", rec.Checkpoint)
		}
	}
	if rec.Status != domain.CloneRunning {
		t.Fatalf("stale commit changed status: %s", rec.Status)
	}
	if err := store.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil {
		t.Fatal(err)
	}
	rec, _ = store.GetStep(ctx, job.ID, claim.Step.Step)
	if rec.Status != domain.CloneSucceeded || len(rec.Checkpoint) > 0 {
		t.Fatalf("commit: %+v", rec)
	}
}

func TestCloneWorkerAdvisoryLockExcludesSecondServe(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	first, err := store.AcquireWorkerLock(ctx)
	if err != nil || first == nil {
		t.Fatalf("first: %v", err)
	}
	defer first.Close(ctx)
	second, err := store.AcquireWorkerLock(ctx)
	if err != nil || second != nil {
		t.Fatalf("second obtained lock: %v %v", second, err)
	}
}

func TestClonePauseResumeKeepsCheckpoint(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "pause")
	_, _ = store.ClaimJobLease(ctx, job.ID, "worker", time.Minute)
	claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || claim == nil {
		t.Fatal(err)
	}
	cp := json.RawMessage(`{"cursor":3}`)
	if err := store.WriteStepCheckpoint(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, cp); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPaused(ctx, job.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := store.ParkStepPaused(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt); err != nil {
		t.Fatal(err)
	}
	if err := store.SetJobStatus(ctx, job.ID, domain.ClonePending); err != nil {
		t.Fatal(err)
	}
	_ = store.ReleaseJobLease(ctx, job.ID, "worker")
	jobs, err := store.ListClaimableJobs(ctx, 10)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("paused claimable: %+v %v", jobs, err)
	}
	if _, err := store.ResumeJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	steps, err := store.ListSteps(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint map[string]int
	for _, step := range steps {
		if step.Step == claim.Step.Step {
			_ = json.Unmarshal(step.Checkpoint, &checkpoint)
		}
	}
	if checkpoint["cursor"] != 3 {
		t.Fatalf("checkpoint lost: %+v", checkpoint)
	}
}

func TestCloneProviderRetryAtSurvivesReload(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "provider")
	_, _ = store.ClaimJobLease(ctx, job.ID, "worker", time.Minute)
	claim, err := store.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || claim == nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	if err := store.ReportStepFailure(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, &domain.CloneError{Code: domain.CloneErrQuota, Message: "quota", RetryAt: &at, RetryAtSource: domain.RetryAtProvider}); err != nil {
		t.Fatal(err)
	}
	step, err := store.GetStep(ctx, job.ID, claim.Step.Step)
	if err != nil {
		t.Fatal(err)
	}
	if step.Error == nil || step.Error.RetryAt == nil || !step.Error.RetryAt.Equal(at) || step.Error.RetryAtSource != domain.RetryAtProvider {
		t.Fatalf("retry metadata: %+v", step)
	}
}

func TestCloneBoardThousandJobsP95(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	for i := range 1000 {
		cloneTestJob(t, store, fmt.Sprintf("video-%04d", i))
	}
	service := pipeline.Service{Store: store}
	samples := make([]time.Duration, 20)
	for i := range samples {
		start := time.Now()
		list, err := service.List(ctx, nil, 1000)
		samples[i] = time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(list.Jobs) != 1000 {
			t.Fatalf("jobs=%d", len(list.Jobs))
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[18]
	t.Logf("1,000 jobs, board p95=%s", p95)
	if p95 > 100*time.Millisecond {
		t.Fatalf("board p95 %s exceeds 100ms", p95)
	}
}

func TestCloneInterruptedRunningJobResumesPersistedStepCheckpoint(t *testing.T) {
	store := cloneStore(t)
	ctx := context.Background()
	job := cloneTestJob(t, store, "interrupted")
	if _, err := store.ClaimJobLease(ctx, job.ID, "dead-worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	step, err := store.ClaimNextStep(ctx, job.ID, "dead-worker")
	if err != nil || step == nil {
		t.Fatalf("claim: %v %v", step, err)
	}
	cp := json.RawMessage(`{"nextScene":3}`)
	if err = store.WriteStepCheckpoint(ctx, job.ID, step.Step.Step, step.Fence, step.Step.Attempt, cp); err != nil {
		t.Fatal(err)
	}
	if err = store.ApplyStepWaiting(ctx, job.ID, step.Step.Step, step.Fence, step.Step.Attempt, nil, &domain.CloneError{Code: domain.CloneErrInterrupted, Message: "worker died"}); err != nil {
		t.Fatal(err)
	}
	if err = store.ReleaseJobLease(ctx, job.ID, "dead-worker"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ResumeJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetStep(ctx, job.ID, step.Step.Step)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.ClonePending || string(got.Checkpoint) == "" {
		t.Fatalf("interrupted step not resumed from checkpoint: %+v", got)
	}
	var saved map[string]int
	if err = json.Unmarshal(got.Checkpoint, &saved); err != nil || saved["nextScene"] != 3 {
		t.Fatalf("checkpoint lost: %s %v", got.Checkpoint, err)
	}
}
