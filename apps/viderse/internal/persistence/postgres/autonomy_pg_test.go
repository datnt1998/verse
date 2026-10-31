package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	cloneimages "github.com/datnt1998/verse/apps/viderse/internal/clone/images"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

const autonomyChannel = "11111111-1111-4111-8111-111111111111"

type autonomyFixture struct {
	pool  *pgxpool.Pool
	clone *postgres.CloneStore
	svc   *autonomy.Service
}

func newAutonomyFixture(t *testing.T, preset string) autonomyFixture {
	t.Helper()
	pool := provision(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO owner_channels(id,name,language,auto_pick) VALUES($1,'Clone fixture','en',false)`, autonomyChannel); err != nil {
		t.Fatal(err)
	}
	clone := postgres.NewCloneStore(pool)
	a, _ := domain.PresetAutonomy(preset)
	raw, _ := json.Marshal(a)
	if _, err := pool.Exec(context.Background(), `UPDATE owner_channels SET autonomy=$1::jsonb WHERE id=$2`, raw, autonomyChannel); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewAutonomyStore(pool)
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(context.Background(), media.Descriptor{ID: "autonomy-test", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	edits := &jobedit.Service{Store: clone, Media: files}
	imageSvc := &cloneimages.Service{Edits: edits, Autonomy: store}
	svc := &autonomy.Service{Diag: func(m string) { t.Log(m) }, Store: store, Reviewer: cloneimages.AutoReviewer{Service: imageSvc}, RetryDelay: -1}
	return autonomyFixture{pool: pool, clone: clone, svc: svc}
}

// reviewJob đưa một job tới Chờ duyệt thật: các step trước images xong, scenes
// có manifest, images đỗ awaiting_review với manifest có điểm.
func (f autonomyFixture) reviewJob(t *testing.T, key string, scores []int) *domain.CloneJob {
	t.Helper()
	ctx := context.Background()
	job := cloneTestJob(t, f.clone, key)
	if _, err := f.clone.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	for {
		claim, err := f.clone.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil {
			t.Fatalf("claim: %v %v", claim, err)
		}
		if claim.Step.Step == domain.CloneImages {
			n := 1
			m := images.Manifest{Version: 1, Revision: 1, SceneRevision: 3}
			for i, s := range scores {
				kind, id := "scene", fmt.Sprintf("s%d", i+1)
				if i == len(scores)-1 {
					kind, id = "thumbnail", "thumbnail"
				}
				m.Items = append(m.Items, images.Item{ID: id, Kind: kind, Status: "review", ReviewVersion: &n})
				item := &m.Items[len(m.Items)-1]
				item.Versions = []images.Version{{Version: 1, SourceFingerprint: images.SourceFingerprint(*item), Score: &domain.ImageScore{Value: s, Reason: domain.ScoreReason{Agent: "qa", Text: "test"}}}}
			}
			raw, _ := json.Marshal(m)
			if err := f.clone.WriteStepCheckpoint(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, raw); err != nil {
				t.Fatal(err)
			}
			waiting := &domain.CloneError{Code: domain.CloneErrAwaitingReview, Message: "Chờ duyệt ảnh"}
			if err := f.clone.ApplyStepWaiting(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, &domain.CloneStepOutput{Data: map[string]any{"revision": 1}}, waiting); err != nil {
				t.Fatal(err)
			}
			break
		}
		out := domain.CloneStepOutput{}
		if claim.Step.Step == domain.CloneScenes {
			out.Data = map[string]any{"manifest": map[string]any{"version": 1, "revision": 3}}
		}
		if err := f.clone.ApplyStepSuccess(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, out); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.clone.ReleaseJobLease(ctx, job.ID, "worker"); err != nil {
		t.Fatal(err)
	}
	return job
}

func (f autonomyFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAutonomyAutoApprovesWhenEveryImageMeetsThreshold(t *testing.T) {
	f := newAutonomyFixture(t, domain.PresetAuto)
	job := f.reviewJob(t, "all-pass", []int{91, 88, 90, 86, 92, 85, 89, 95}) // 7 cảnh + bìa
	if got := f.count(t, `SELECT count(*) FROM notifications WHERE kind='review_ready'`); got != 0 {
		t.Fatalf("review_ready notices = %d, want 0 (máy duyệt ngay)", got)
	}
	if err := f.svc.DispatchReviews(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM clone_bulk_review_intents WHERE job_id=$1`, job.ID) != 1 {
		t.Fatal("ảnh chưa được duyệt qua đường Duyệt hết")
	}
	if f.count(t, `SELECT count(*) FROM clone_jobs WHERE id=$1 AND image_approved_at IS NOT NULL`, job.ID) != 1 {
		t.Fatal("image_approved_at chưa ghi")
	}
	var text string
	var detail []byte
	if err := f.pool.QueryRow(context.Background(), `SELECT text, detail FROM notifications WHERE kind='action_result'`).Scan(&text, &detail); err != nil {
		t.Fatalf("thiếu thông báo action_result: %v", err)
	}
	if !strings.HasPrefix(text, "Tự duyệt theo điểm") || !strings.Contains(string(detail), `"reason"`) {
		t.Fatalf("thông báo = %q %s", text, detail)
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='auto_approve'`, job.ID) != 1 {
		t.Fatal("thiếu Hành động tự làm auto_approve")
	}
	// Chạy lại không duyệt/thông báo lần hai.
	if err := f.svc.DispatchReviews(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM notifications WHERE kind='action_result'`) != 1 {
		t.Fatal("thông báo bị lặp")
	}
}

func TestAutonomyKeepsWholeVideoInReviewWhenOneImageIsBelow(t *testing.T) {
	f := newAutonomyFixture(t, domain.PresetAuto)
	job := f.reviewJob(t, "one-below", []int{91, 88, 90, 72, 92, 85, 89, 95})
	if got := f.count(t, `SELECT count(*) FROM notifications WHERE kind='review_ready'`); got != 1 {
		t.Fatalf("review_ready notices = %d, want 1", got)
	}
	if err := f.svc.DispatchReviews(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM clone_bulk_review_intents WHERE job_id=$1`, job.ID) != 0 || f.count(t, `SELECT count(*) FROM notifications WHERE kind='action_result'`) != 0 {
		t.Fatal("video có ảnh dưới ngưỡng không được tự duyệt")
	}
	var summary string
	if err := f.pool.QueryRow(context.Background(), `SELECT auto_review->>'summary' FROM clone_jobs WHERE id=$1`, job.ID).Scan(&summary); err != nil || summary != "cảnh 4 được 72" {
		t.Fatalf("summary = %q, %v", summary, err)
	}
	var group string
	if err := f.pool.QueryRow(context.Background(), `SELECT stage_group FROM clone_jobs WHERE id=$1`, job.ID).Scan(&group); err != nil || group != domain.StageReview {
		t.Fatalf("stage = %q, %v", group, err)
	}
}

func TestAutonomyBalancedNeverAutoApproves(t *testing.T) {
	f := newAutonomyFixture(t, domain.PresetBalanced)
	job := f.reviewJob(t, "balanced", []int{99, 99, 99})
	if err := f.svc.DispatchReviews(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM clone_bulk_review_intents WHERE job_id=$1`, job.ID) != 0 {
		t.Fatal("Cân bằng phải luôn hỏi")
	}
}

func TestAutonomyRetriesSourceDownloadOnceThenNeedsAttention(t *testing.T) {
	f := newAutonomyFixture(t, domain.PresetBalanced)
	ctx := context.Background()
	job := cloneTestJob(t, f.clone, "download-error")
	if _, err := f.clone.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	fail := func() (retried bool) {
		claim, err := f.clone.ClaimNextStep(ctx, job.ID, "worker")
		if err != nil || claim == nil {
			t.Fatalf("claim: %v %v", claim, err)
		}
		cerr := &domain.CloneError{Code: domain.CloneErrSourceUnavailable, Message: "Không tải được video nguồn"}
		if f.svc.AutoFix(ctx, job.ID, claim.Step.Step, cerr) {
			return true
		}
		if err := f.clone.ReportStepFailure(ctx, job.ID, claim.Step.Step, claim.Fence, claim.Step.Attempt, cerr); err != nil {
			t.Fatal(err)
		}
		if err := f.clone.SetJobStatus(ctx, job.ID, domain.CloneFailed); err != nil { // như settleNoClaim của runner
			t.Fatal(err)
		}
		return false
	}
	if !fail() {
		t.Fatal("lỗi tải nguồn đầu tiên phải được tự thử lại")
	}
	if got := f.count(t, `SELECT count(*) FROM notifications WHERE kind='needs_attention'`); got != 0 {
		t.Fatalf("needs_attention sau lần tự thử = %d, want 0", got)
	}
	if f.count(t, `SELECT count(*) FROM clone_auto_actions WHERE job_id=$1 AND kind='auto_retry'`, job.ID) != 1 {
		t.Fatal("thiếu Hành động tự làm auto_retry")
	}
	if fail() {
		t.Fatal("chỉ tự thử lại một lần")
	}
	got, err := f.clone.GetJob(ctx, job.ID)
	if err != nil || got.AutoFix == nil || got.AutoFix.Result != domain.AutoFixFailed {
		t.Fatalf("autoFix = %+v, %v", got.AutoFix, err)
	}
	var group string
	if err := f.pool.QueryRow(ctx, `SELECT stage_group FROM clone_jobs WHERE id=$1`, job.ID).Scan(&group); err != nil || group != domain.StageAttention {
		t.Fatalf("stage = %q, %v", group, err)
	}
	at, _ := time.Parse(time.RFC3339, got.AutoFix.At)
	want := fmt.Sprintf("Đã tự thử lại lúc %02d:%02d", at.Hour(), at.Minute())
	steps, _ := f.clone.ListSteps(ctx, job.ID)
	if sub := domain.Subline(*got, steps, time.Now()); !strings.Contains(sub, want) {
		t.Fatalf("subline = %q, want chứa %q", sub, want)
	}
}

func TestAutonomyCarefulDoesNotRetry(t *testing.T) {
	f := newAutonomyFixture(t, domain.PresetCareful)
	ctx := context.Background()
	job := cloneTestJob(t, f.clone, "careful-error")
	if _, err := f.clone.ClaimJobLease(ctx, job.ID, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	claim, err := f.clone.ClaimNextStep(ctx, job.ID, "worker")
	if err != nil || claim == nil {
		t.Fatalf("claim: %v %v", claim, err)
	}
	if f.svc.AutoFix(ctx, job.ID, claim.Step.Step, &domain.CloneError{Code: domain.CloneErrSourceUnavailable}) {
		t.Fatal("Cẩn thận không tự thử lại")
	}
}

func TestAutonomyNeverRetriesHumanErrors(t *testing.T) {
	for _, code := range []string{domain.CloneErrToolMissing, domain.CloneErrTemplateChanged, domain.CloneErrInvalidRequest, domain.CloneErrStale} {
		if got := domain.AutoFixCodeOf(domain.CloneDownload, &domain.CloneError{Code: code}); got != "" {
			t.Fatalf("%s phân loại tự sửa được: %q", code, got)
		}
	}
}

func TestAutonomyServiceSetAndRestore(t *testing.T) {
	f := newAutonomyFixture(t, domain.PresetBalanced)
	ctx := context.Background()
	prev, err := f.svc.SetApprove(ctx, autonomyChannel, domain.ApproveScore, 90)
	if err != nil || prev.Approve.Mode != domain.ApproveAsk {
		t.Fatalf("prev=%+v err=%v", prev, err)
	}
	cur, _ := f.svc.GetAutonomy(ctx, autonomyChannel)
	if cur.Approve.Mode != domain.ApproveScore || cur.Approve.Min != 90 || cur.Preset != domain.PresetAuto {
		// Tự duyệt theo điểm + Cân bằng khác Tự chạy ở belowQuality → Tuỳ chỉnh.
		if cur.Preset != domain.PresetCustom {
			t.Fatalf("cur=%+v", cur)
		}
	}
	if _, err := f.svc.SetApprove(ctx, autonomyChannel, domain.ApproveAll, 0); err == nil {
		t.Fatal("Tự duyệt hết cần xác nhận rõ")
	}
	if err := f.svc.Restore(ctx, autonomyChannel, prev); err != nil {
		t.Fatal(err)
	}
	cur, _ = f.svc.GetAutonomy(ctx, autonomyChannel)
	if cur.Preset != domain.PresetBalanced {
		t.Fatalf("restore: %+v", cur)
	}
	if err := f.svc.ApplyPreset(ctx, autonomyChannel, "custom"); err == nil {
		t.Fatal("ApplyPreset không nhận custom")
	}
	if err := f.svc.ApplyPreset(ctx, autonomyChannel, domain.PresetAuto); err != nil {
		t.Fatal(err)
	}
	if p, _ := f.svc.Preset(ctx, autonomyChannel); p != domain.PresetAuto {
		t.Fatalf("preset=%s", p)
	}
	var _ pipeline.AutoFixer = f.svc
}
