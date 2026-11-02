package audio

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// The HTTP "Làm trước" action must reorder the durable queue snapshot (which
// drives the displayed position), not answer 501 or reorder only the client.
func TestPGPrioritizeChapterOrdersDurableQueue(t *testing.T) {
	ctx := context.Background()
	_, _, scope, _, svc := pgCastFixture(t, 0)
	for _, chapter := range []int{1, 2, 3} {
		if _, err := svc.deps.Jobs.Enqueue(ctx, scope, postgres.JobTTS,
			fmt.Sprintf("priority-%d", chapter),
			map[string]any{"volume": 1, "chapter": chapter, "kind": "create"},
			nil, pgtype.UUID{}); err != nil {
			t.Fatalf("enqueue chapter %d: %v", chapter, err)
		}
	}
	if err := svc.Prioritize(ctx, 1, 2); err != nil {
		t.Fatalf("prioritize: %v", err)
	}
	positions := map[int]int{}
	for _, job := range svc.Queue() {
		if job.Volume == 1 && job.State == JobQueued {
			positions[job.Chapter] = job.Position
		}
	}
	if positions[2] != 1 || positions[1] != 2 || positions[3] != 3 {
		t.Fatalf("positions = %v, want 2:1 1:2 3:3", positions)
	}
	if err := svc.Prioritize(ctx, 1, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing chapter = %v, want ErrNotFound", err)
	}
}

func TestPGPrioritizedDuplicateKeepsCurrentAttempt(t *testing.T) {
	ctx := context.Background()
	_, nar, scope, repo, svc := pgCastFixture(t, 0)
	pgCastLockedScript(t, ctx, nar, scope, repo, "Nội dung kiểm tra hàng đợi.")
	ref, err := nar.ChapterRefByNumber(ctx, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	input := pgTTSInput{ChapterID: postgres.UUIDString(ref.ID), Volume: 1, Chapter: 1, Kind: JobKindCreate, Quality: "standard", SourceRevision: int32(ref.CurrentRevision), ScriptRevision: 1}
	older, err := svc.deps.Jobs.Enqueue(ctx, scope, postgres.JobTTS, "duplicate-old", input, nil, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	current, err := svc.deps.Jobs.Enqueue(ctx, scope, postgres.JobTTS, "duplicate-current", input, nil, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.deps.Jobs.PrioritizeChapter(ctx, scope, 1, 1); err != nil {
		t.Fatal(err)
	}
	slot, err := svc.chapterQueueSlot(ctx, scope, pgQueueIntent{ChapterID: input.ChapterID, Volume: 1, Chapter: 1, Kind: input.Kind, Quality: input.Quality, SourceRevision: input.SourceRevision, ScriptRevision: 1})
	if err != nil || slot.reuse != postgres.UUIDString(current.ID) {
		t.Fatalf("reuse current attempt: %+v %v", slot, err)
	}
	kept, err := svc.deps.Jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: current.ID})
	if err != nil || kept.State != postgres.JobQueued || kept.QueuePriority <= 0 {
		t.Fatalf("current prioritized attempt was cancelled: %+v %v", kept, err)
	}
	cancelled, err := svc.deps.Jobs.Get(ctx, postgres.JobKey{ProjectID: scope.ProjectID, ID: older.ID})
	if err != nil || cancelled.State != postgres.JobCancelled {
		t.Fatalf("duplicate attempt still active: %+v %v", cancelled, err)
	}
}

func TestPGResumeFailedChapterRequeuesAndFencesOldWorker(t *testing.T) {
	ctx := context.Background()
	_, _, scope, _, svc := pgCastFixture(t, 0)
	queued, err := svc.deps.Jobs.Enqueue(ctx, scope, postgres.JobTTS, "failed-resume", map[string]any{"volume": 1, "chapter": 1, "kind": "create"}, nil, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	token := sha256.Sum256([]byte("failed-resume-worker"))
	worker, err := svc.deps.Jobs.RegisterWorker(ctx, "failed-resume-worker", token[:], []string{"tts"})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := svc.deps.Jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, scope.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	lease := postgres.WorkerLease{JobKey: postgres.JobKey{ProjectID: scope.ProjectID, ID: queued.ID}, WorkerID: worker.ID, Fence: claimed.FencingToken}
	failed, err := svc.deps.Jobs.Fail(ctx, lease, map[string]string{"message": "máy đọc dừng"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Resume(postgres.UUIDString(queued.ID)); err != nil {
		t.Fatalf("retry from chapter action: %v", err)
	}
	retried, err := svc.deps.Jobs.Get(ctx, lease.JobKey)
	if err != nil || retried.State != postgres.JobQueued || retried.FencingToken <= failed.FencingToken {
		t.Fatalf("retry state/fence: %+v %v", retried, err)
	}
	if _, err := svc.deps.Jobs.Complete(ctx, lease, nil, 0, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
		t.Fatalf("old worker may still publish: %v", err)
	}
}
