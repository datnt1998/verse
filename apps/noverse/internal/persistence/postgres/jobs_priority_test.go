package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func enqueuePriorityTTS(t *testing.T, jobs *postgres.Jobs, scope postgres.ScopeRef, key string,
	volume, chapter int, extra map[string]any) postgres.Job {
	t.Helper()
	input := map[string]any{"volume": volume, "chapter": chapter, "kind": "create", "segments": 2}
	for k, v := range extra {
		input[k] = v
	}
	job, err := jobs.Enqueue(context.Background(), scope, postgres.JobTTS, key, input, nil, pgtype.UUID{})
	if err != nil {
		t.Fatalf("enqueue %s: %v", key, err)
	}
	return job
}

// "Làm trước" must reorder the durable queue, not just the browser: the list
// reload, the native claim and the preparation claim all see the same order.
// It changes only priority — state, ownership, lease, fence and created_at stay.
func TestJobsPrioritizeChapterOrdersListAndClaim(t *testing.T) {
	const projectSlug = "jobs-priority-proj"
	f := provisionNarrative(t, projectSlug, "jobs-priority-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	if _, err := f.nar.CreateStory(ctx, projectSlug, nil, postgres.StoryInfo{
		Slug: "jobs-priority-other", Title: "Truyện khác", Status: "writing",
	}); err != nil {
		t.Fatalf("create other story: %v", err)
	}
	otherScope, err := f.nar.Resolve(ctx, projectSlug, "jobs-priority-other", nil, false)
	if err != nil {
		t.Fatalf("resolve other story: %v", err)
	}

	one := enqueuePriorityTTS(t, jobs, f.scope, "priority-one", 1, 1, nil)
	two := enqueuePriorityTTS(t, jobs, f.scope, "priority-two", 1, 2, nil)
	three := enqueuePriorityTTS(t, jobs, f.scope, "priority-three", 1, 3, nil)
	otherVolume := enqueuePriorityTTS(t, jobs, f.scope, "priority-other-volume", 2, 2, nil)
	otherStory := enqueuePriorityTTS(t, jobs, otherScope, "priority-other-story", 1, 2, nil)

	got, err := jobs.PrioritizeChapter(ctx, f.scope, 1, 2)
	if err != nil {
		t.Fatalf("prioritize chapter 2: %v", err)
	}
	if got.ID != two.ID || got.QueuePriority != 1 {
		t.Fatalf("prioritized row = %v priority %d, want %v priority 1", got.ID, got.QueuePriority, two.ID)
	}
	if got.State != postgres.JobQueued || !got.CreatedAt.Equal(two.CreatedAt) {
		t.Fatalf("prioritize changed the job: %+v", got)
	}

	// Same volume/chapter in another story and another volume are untouched.
	for _, tc := range []struct {
		name string
		id   pgtype.UUID
	}{
		{"other volume", otherVolume.ID},
		{"other story", otherStory.ID},
	} {
		row, err := jobs.Get(ctx, postgres.JobKey{ProjectID: f.scope.ProjectID, ID: tc.id})
		if err != nil {
			t.Fatalf("get %s: %v", tc.name, err)
		}
		if row.QueuePriority != 0 {
			t.Fatalf("%s was prioritized: %+v", tc.name, row)
		}
	}

	list, err := jobs.List(ctx, f.scope)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 4 || list[0].ID != two.ID || list[0].QueuePriority != 1 {
		t.Fatalf("list order = %+v, want prioritized chapter 2 first", list)
	}
	for _, row := range list[1:] {
		if row.QueuePriority != 0 {
			t.Fatalf("unexpected priority on %v: %+v", row.ID, row)
		}
	}
	if list[1].ID != one.ID || list[2].ID != three.ID || list[3].ID != otherVolume.ID {
		t.Fatalf("unaffected order changed: %+v", list)
	}

	worker := registerTestWorker(t, jobs, "priority-worker")
	claimed, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.ID != two.ID {
		t.Fatalf("claimed %v, want prioritized chapter %v", claimed.ID, two.ID)
	}
	if claimed.QueuePriority != 1 || claimed.LeaseUntil == nil || claimed.WorkerID != worker.ID {
		t.Fatalf("claim lost priority/ownership: %+v", claimed)
	}

	// A newer "Làm trước" sorts before the earlier one, including a running
	// prioritized row in the durable list.
	got, err = jobs.PrioritizeChapter(ctx, f.scope, 1, 3)
	if err != nil {
		t.Fatalf("prioritize chapter 3: %v", err)
	}
	if got.QueuePriority != 2 {
		t.Fatalf("second priority = %d, want 2", got.QueuePriority)
	}
	afterTwo, err := jobs.Get(ctx, postgres.JobKey{ProjectID: f.scope.ProjectID, ID: two.ID})
	if err != nil {
		t.Fatalf("get running job: %v", err)
	}
	if afterTwo.FencingToken != claimed.FencingToken || afterTwo.WorkerID != worker.ID ||
		afterTwo.LeaseUntil == nil || !afterTwo.LeaseUntil.Equal(*claimed.LeaseUntil) ||
		!afterTwo.CreatedAt.Equal(two.CreatedAt) {
		t.Fatalf("prioritize disturbed a running job: %+v", afterTwo)
	}
	list, err = jobs.List(ctx, f.scope)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list[0].ID != three.ID || list[1].ID != two.ID {
		t.Fatalf("priority order = %+v, want chapter 3 then running chapter 2", list)
	}

	// Only a queued chapter can move; the running one and a missing one fail
	// without writing.
	if _, err := jobs.PrioritizeChapter(ctx, f.scope, 1, 2); !errors.Is(err, postgres.ErrJobState) {
		t.Fatalf("prioritize running = %v, want ErrJobState", err)
	}
	if _, err := jobs.PrioritizeChapter(ctx, f.scope, 9, 9); !errors.Is(err, postgres.ErrJobNotFound) {
		t.Fatalf("prioritize missing = %v, want ErrJobNotFound", err)
	}
}

// Hosted preparation rows go through a different claim query; a prioritized
// composition must be picked first there too.
func TestJobsPrioritizeChapterOrdersPreparationClaim(t *testing.T) {
	f := provisionNarrative(t, "jobs-priority-prep-proj", "jobs-priority-prep-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()

	enqueuePriorityTTS(t, jobs, f.scope, "prep-one", 1, 1, map[string]any{"needsScript": true})
	two := enqueuePriorityTTS(t, jobs, f.scope, "prep-two", 1, 2, map[string]any{"needsScript": true})

	if _, err := jobs.PrioritizeChapter(ctx, f.scope, 1, 2); err != nil {
		t.Fatalf("prioritize preparation: %v", err)
	}
	claimed, err := jobs.ClaimPreparation(ctx, f.scope, time.Minute)
	if err != nil {
		t.Fatalf("claim preparation: %v", err)
	}
	if claimed.ID != two.ID || claimed.QueuePriority != 1 {
		t.Fatalf("preparation claim = %v priority %d, want %v priority 1", claimed.ID, claimed.QueuePriority, two.ID)
	}
}

func TestJobsPrioritizeRequeuedAttemptBeforeNewerTerminalAttempt(t *testing.T) {
	f := provisionNarrative(t, "jobs-priority-history", "jobs-priority-history-story")
	jobs := postgres.NewJobs(f.pool)
	ctx := context.Background()
	older := enqueuePriorityTTS(t, jobs, f.scope, "older-attempt", 1, 1, nil)
	key := postgres.JobKey{ProjectID: f.scope.ProjectID, ID: older.ID}
	if _, err := jobs.Pause(ctx, key); err != nil {
		t.Fatal(err)
	}
	newer := enqueuePriorityTTS(t, jobs, f.scope, "newer-attempt", 1, 1, nil)
	if _, err := jobs.Cancel(ctx, postgres.JobKey{ProjectID: f.scope.ProjectID, ID: newer.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.Resume(ctx, key); err != nil {
		t.Fatal(err)
	}
	got, err := jobs.PrioritizeChapter(ctx, f.scope, 1, 1)
	if err != nil {
		t.Fatalf("prioritize current queued attempt: %v", err)
	}
	if got.ID != older.ID || got.QueuePriority <= 0 {
		t.Fatalf("prioritized historical terminal instead of queued attempt: %+v", got)
	}
	terminal, err := jobs.Get(ctx, postgres.JobKey{ProjectID: f.scope.ProjectID, ID: newer.ID})
	if err != nil || terminal.State != postgres.JobCancelled || terminal.QueuePriority != 0 {
		t.Fatalf("historical attempt changed: %+v %v", terminal, err)
	}
}
