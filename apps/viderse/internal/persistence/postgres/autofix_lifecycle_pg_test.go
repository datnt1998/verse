package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

func claimAutoFixStep(t *testing.T, f autonomyFixture, job *domain.CloneJob, target domain.CloneStep) *pipeline.ClaimedStep {
	t.Helper()
	ctx := context.Background()
	if _, err := f.clone.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	for {
		claim, err := f.clone.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil {
			t.Fatalf("claim=%+v err=%v", claim, err)
		}
		if claim.Step.Step == target {
			return claim
		}
		if err := f.clone.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, domain.CloneStepOutput{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAutoFixQualityGateAlwaysNeedsOwner(t *testing.T) {
	f := newAutonomyFixture(t, true)
	ctx := context.Background()
	job := cloneTestJob(t, f.clone, "quality-below-90")
	claim := claimAutoFixStep(t, f, job, domain.CloneScore)
	cerr := &domain.CloneError{Code: domain.CloneErrQualityGate, Message: "89/100"}
	if f.svc.AutoFix(ctx, job.ID, domain.CloneScore, cerr) {
		t.Fatal("quality below 90 must ask, not consume repair")
	}
	if err := f.clone.ReportStepFailure(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, cerr); err != nil {
		t.Fatal(err)
	}
	if err := f.clone.SetJobStatus(ctx, job.ID, domain.CloneFailed); err != nil {
		t.Fatal(err)
	}
	got, err := f.clone.GetJob(ctx, job.ID)
	if err != nil || got.AutoFix != nil || got.StageGroup != domain.StageAttention {
		t.Fatalf("quality boundary=%+v %v", got, err)
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind IN ('auto_retry','auto_fix')`, job.ID) != 0 {
		t.Fatal("quality error consumed repair budget")
	}
}

func TestAutoFixWorkerRestartKeepsBudgetAcrossStepsAndSettlement(t *testing.T) {
	f := newAutonomyFixture(t, false)
	ctx := context.Background()
	job := cloneTestJob(t, f.clone, "restart-budget")
	claimAutoFixStep(t, f, job, domain.CloneDownload)
	if !f.svc.AutoFix(ctx, job.ID, domain.CloneDownload, &domain.CloneError{Code: domain.CloneErrSourceUnavailable}) {
		t.Fatal("first error must retry")
	}
	// The committed marker survives constructing entirely new store/service.
	restarted := &autonomy.Service{Store: postgres.NewAutonomyStore(f.pool), RetryDelay: -1}
	retry, err := f.clone.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || retry == nil {
		t.Fatalf("retry=%+v %v", retry, err)
	}
	if err := f.clone.ApplyStepSuccess(ctx, job.ID, retry.Step.Step, retry.Fence, retry.Step.Attempt, domain.CloneStepOutput{}); err != nil {
		t.Fatal(err)
	}
	restarted.AttemptSettled(ctx, job.ID, domain.CloneImages, domain.CloneSucceeded)
	waiting, err := f.clone.GetJob(ctx, job.ID)
	if err != nil || waiting.AutoFix == nil || waiting.AutoFix.Result != domain.AutoFixRetrying {
		t.Fatalf("wrong step settled retry: %+v %v", waiting, err)
	}
	restarted.AttemptSettled(ctx, job.ID, domain.CloneDownload, domain.CloneSucceeded)
	restarted.AttemptSettled(ctx, job.ID, domain.CloneDownload, domain.CloneSucceeded)
	got, err := f.clone.GetJob(ctx, job.ID)
	if err != nil || got.AutoFix == nil || got.AutoFix.Result != domain.AutoFixRecovered {
		t.Fatalf("settlement=%+v %v", got, err)
	}
	before := *got.AutoFix
	// Wrong-step and failed settlements must leave the recovered record alone.
	restarted.AttemptSettled(ctx, job.ID, domain.CloneImages, domain.CloneSucceeded)
	restarted.AttemptSettled(ctx, job.ID, domain.CloneDownload, domain.CloneFailed)
	got, err = f.clone.GetJob(ctx, job.ID)
	if err != nil || !reflect.DeepEqual(got.AutoFix, &before) {
		t.Fatalf("nonmatching settlement changed marker: %+v %v", got, err)
	}
	claimAutoFixStep(t, f, job, domain.CloneImages)
	if restarted.AutoFix(ctx, job.ID, domain.CloneImages, &domain.CloneError{Code: domain.CloneErrInternal}) {
		t.Fatal("restart or new step reset job-generation repair budget")
	}
	got, err = f.clone.GetJob(ctx, job.ID)
	if err != nil || got.AutoFix.Result != domain.AutoFixFailed {
		t.Fatalf("second error=%+v %v", got, err)
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='auto_retry'`, job.ID) != 1 {
		t.Fatal("repair action duplicated across restart")
	}
}

func TestAutoFixAttemptZeroAndNonrunningStepDoNotConsumeBudget(t *testing.T) {
	f := newAutonomyFixture(t, false)
	ctx := context.Background()
	job := cloneTestJob(t, f.clone, "attempt-zero")
	cerr := &domain.CloneError{Code: domain.CloneErrSourceUnavailable}
	if f.svc.AutoFix(ctx, job.ID, domain.CloneDownload, cerr) {
		t.Fatal("pending attempt-zero step is not a failed running attempt")
	}
	got, err := f.clone.GetJob(ctx, job.ID)
	if err != nil || got.AutoFix != nil {
		t.Fatalf("non-running attempt consumed budget: %+v %v", got, err)
	}
	claimAutoFixStep(t, f, job, domain.CloneDownload)
	if !f.svc.AutoFix(ctx, job.ID, domain.CloneDownload, cerr) {
		t.Fatal("nonrunning call consumed the real first attempt")
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='auto_retry'`, job.ID) != 1 {
		t.Fatal("expected exactly one durable repair action")
	}
}

func TestAutoFixRestartGenerationGetsFreshBudgetWithoutMutatingOldMarker(t *testing.T) {
	f := newAutonomyFixture(t, false)
	ctx := context.Background()
	old := cloneTestJob(t, f.clone, "generation-budget")
	claimAutoFixStep(t, f, old, domain.CloneDownload)
	cerr := &domain.CloneError{Code: domain.CloneErrSourceUnavailable}
	if !f.svc.AutoFix(ctx, old.ID, domain.CloneDownload, cerr) {
		t.Fatal("first repair denied")
	}
	claim, err := f.clone.ClaimNextStep(ctx, old.ID, "worker")
	if err != nil || claim == nil {
		t.Fatalf("claim=%+v %v", claim, err)
	}
	if f.svc.AutoFix(ctx, old.ID, domain.CloneDownload, cerr) {
		t.Fatal("second repair allowed")
	}
	if err := f.clone.ReportStepFailure(ctx, old.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, cerr); err != nil {
		t.Fatal(err)
	}
	if err := f.clone.SetJobStatus(ctx, old.ID, domain.CloneFailed); err != nil {
		t.Fatal(err)
	}
	if err := f.clone.ReleaseJobLease(ctx, old.ID, "worker"); err != nil {
		t.Fatal(err)
	}
	previous, err := f.clone.GetJob(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := admissionInput(old.SourceKey, fixtureTemplate, true, 1, `{}`)
	in.Replaces = old.ID
	next, err := f.clone.StartJob(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if next.Job.Generation != old.Generation+1 || next.Job.AutoFix != nil {
		t.Fatalf("new generation=%+v", next.Job)
	}
	claimAutoFixStep(t, f, next.Job, domain.CloneDownload)
	if !f.svc.AutoFix(ctx, next.Job.ID, domain.CloneDownload, cerr) {
		t.Fatal("new generation lost its own repair")
	}
	persisted, err := f.clone.GetJob(ctx, old.ID)
	if err != nil || !reflect.DeepEqual(persisted.AutoFix, previous.AutoFix) {
		t.Fatalf("restart mutated old repair record=%+v %v", persisted, err)
	}
}

func TestAutonomyMissingScoreBlocksWholeVideo(t *testing.T) {
	f := newAutonomyFixture(t, true)
	job := f.reviewJob(t, "unscored", []int{85, -1, 99})
	if err := f.svc.DispatchReviews(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := f.clone.GetJob(context.Background(), job.ID)
	if err != nil || got.StageGroup != domain.StageReview {
		t.Fatalf("unscored video left review=%+v %v", got, err)
	}
	if f.count(t, `SELECT count(*) FROM clone_jobs WHERE id=$1 AND image_approved_at IS NOT NULL`, job.ID) != 0 {
		t.Fatal("unscored image automatically approved")
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='auto_approve'`, job.ID) != 0 {
		t.Fatal("unscored review recorded as approved")
	}
}

// Wrap only the review boundary; all pending selection/audit state is real PG.
type guardedReviewer struct {
	inner      autonomy.Reviewer
	state      func(*autonomy.ReviewState)
	approveErr error
	calls      int
}

func (r *guardedReviewer) ReviewState(ctx context.Context, id string) (autonomy.ReviewState, error) {
	state, err := r.inner.ReviewState(ctx, id)
	if err == nil && r.state != nil {
		r.state(&state)
	}
	return state, err
}
func (r *guardedReviewer) ApproveAll(ctx context.Context, id string, revision int) error {
	r.calls++
	if r.approveErr != nil {
		return r.approveErr
	}
	return r.inner.ApproveAll(ctx, id, revision)
}

func TestAutonomyRechecksToggleRevisionAndReadinessBeforeApproving(t *testing.T) {
	for _, guard := range []string{"toggle-off", "revision-moved", "drawing"} {
		t.Run(guard, func(t *testing.T) {
			f := newAutonomyFixture(t, true)
			job := f.reviewJob(t, "guarded", []int{85, 85})
			r := &guardedReviewer{inner: f.svc.Reviewer}
			r.state = func(state *autonomy.ReviewState) {
				switch guard {
				case "toggle-off":
					execChannelSQL(t, f.pool, `UPDATE owner_channels SET auto_approve=false WHERE id=$1`, autonomyChannel)
				case "revision-moved":
					state.Revision++
				case "drawing":
					state.Ready = false
				}
			}
			f.svc.Reviewer = r
			if err := f.svc.DispatchReviews(context.Background()); err != nil {
				t.Fatal(err)
			}
			if r.calls != 0 || f.count(t, `SELECT count(*) FROM clone_jobs WHERE id=$1 AND image_approved_at IS NOT NULL`, job.ID) != 0 {
				t.Fatal("guard allowed automatic approval")
			}
		})
	}
}

func TestAutonomyApprovalGateErrorsStopAfterBoundedTries(t *testing.T) {
	f := newAutonomyFixture(t, true)
	job := f.reviewJob(t, "gate-error", []int{85, 85})
	r := &guardedReviewer{inner: f.svc.Reviewer, approveErr: errors.New("revision gate rejected")}
	f.svc.Reviewer = r
	for range autonomy.MaxReviewTries + 1 {
		if err := f.svc.DispatchReviews(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r.calls != autonomy.MaxReviewTries {
		t.Fatalf("gate attempts=%d", r.calls)
	}
	var result string
	var tries int
	if err := f.pool.QueryRow(context.Background(), `SELECT auto_review->>'result',(auto_review->>'tries')::int FROM clone_jobs WHERE id=$1`, job.ID).Scan(&result, &tries); err != nil {
		t.Fatal(err)
	}
	if result != autonomy.ResultError || tries != autonomy.MaxReviewTries || f.count(t, `SELECT count(*) FROM clone_jobs WHERE id=$1 AND image_approved_at IS NOT NULL`, job.ID) != 0 {
		t.Fatalf("gate error state=%s tries=%d", result, tries)
	}
}

func TestAutoFixTransactionFailureRollsBackMarkerStepAndAction(t *testing.T) {
	pool, provider := provisionLegacyChannelSchema(t)
	ctx := context.Background()
	if _, err := provider.UpTo(ctx, 40); err != nil {
		t.Fatal(err)
	}
	seedCloneChannel(t, pool)
	f := autonomyFixture{pool: pool, clone: postgres.NewCloneStore(pool)}
	store := postgres.NewAutonomyStore(pool)
	job := cloneTestJob(t, f.clone, "repair-rollback")
	claimAutoFixStep(t, f, job, domain.CloneDownload)
	// A real write failure after the step reset must roll back the entire
	// repair transaction; the retry budget is only spent by a commit.
	execChannelSQL(t, pool, `CREATE FUNCTION reject_repair_action() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture action write failure'; END $$`)
	execChannelSQL(t, pool, `CREATE TRIGGER reject_repair_action BEFORE INSERT ON clone_auto_actions FOR EACH ROW EXECUTE FUNCTION reject_repair_action()`)
	retried, err := store.BeginAutoFix(ctx, job.ID, domain.CloneDownload, domain.AutoFixSourceDownload, time.Now())
	if err == nil || retried {
		t.Fatalf("failed repair write=%v %v", retried, err)
	}
	got, err := f.clone.GetJob(ctx, job.ID)
	if err != nil || got.AutoFix != nil {
		t.Fatalf("rolled-back repair marker=%+v %v", got, err)
	}
	step, err := f.clone.GetStep(ctx, job.ID, domain.CloneDownload)
	if err != nil || step.Status != domain.CloneRunning {
		t.Fatalf("step reset escaped transaction: %+v %v", step, err)
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1`, job.ID) != 0 {
		t.Fatal("rolled-back repair action survived")
	}
	execChannelSQL(t, pool, `DROP TRIGGER reject_repair_action ON clone_auto_actions; DROP FUNCTION reject_repair_action()`)
	retried, err = postgres.NewAutonomyStore(pool).BeginAutoFix(ctx, job.ID, domain.CloneDownload, domain.AutoFixSourceDownload, time.Now())
	if err != nil || !retried {
		t.Fatalf("rollback consumed repair budget: %v %v", retried, err)
	}
}
