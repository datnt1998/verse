package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// Hosted voice enqueues must serialize per chapter before they read/update any
// job row. Two first Generate clicks observe zero rows, so without the
// transaction-scoped advisory lock the loser's InsertJob resolves to the
// winner's frozen input through the idempotency conflict (same key), or inserts
// a second active row (different keys): the later request's source/quality is
// silently dropped and the chapter keeps rendering the old intent.

type chapterEnqueueResult struct {
	job   postgres.Job
	err   error
	input map[string]any
}

func enqueueChapterPayload(chapterID, quality string, sourceRevision int, marker string) map[string]any {
	return map[string]any{
		"kind": "create", "chapter": 1, "volume": 1,
		"chapterId": chapterID, "sourceRevision": sourceRevision,
		"quality":  quality,
		"segments": []map[string]any{{"id": "s0001", "text": marker}},
	}
}

func jsonPayloadEqual(t *testing.T, raw []byte, want map[string]any) bool {
	t.Helper()
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want payload: %v", err)
	}
	var got, expected any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("job input: %v", err)
	}
	if err := json.Unmarshal(wantRaw, &expected); err != nil {
		t.Fatalf("unmarshal want payload: %v", err)
	}
	return reflect.DeepEqual(got, expected)
}

// burst runs n concurrent EnqueueChapter calls released by one barrier and
// returns their results in launch order.
func burstEnqueueChapter(t *testing.T, jobs *postgres.Jobs, scope postgres.ScopeRef, chID pgtype.UUID,
	chapterID string, base int, sameKey bool, n int) []chapterEnqueueResult {
	t.Helper()
	ctx := context.Background()
	total := int32(1)
	results := make([]chapterEnqueueResult, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		key := "storm-a1"
		if !sameKey {
			key = fmt.Sprintf("storm-%d-a%d", base, i+1)
		}
		quality, rev, marker := "high", base+2, fmt.Sprintf("beta-%d", base)
		if i%2 == 1 {
			quality, rev, marker = "fast", base+3, fmt.Sprintf("gamma-%d", base)
		}
		payload := enqueueChapterPayload(chapterID, quality, rev, marker)
		wg.Add(1)
		go func(i int, key string, payload map[string]any) {
			defer wg.Done()
			<-start
			job, err := jobs.EnqueueChapter(ctx, scope, key, payload, &total, chID, pgtype.UUID{})
			results[i] = chapterEnqueueResult{job: job, err: err, input: payload}
		}(i, key, payload)
	}
	close(start)
	wg.Wait()
	return results
}

// assertOneChapterRow asserts the burst left exactly one queued row and that
// the final frozen input is the last serialized request's complete intent.
func assertOneChapterRow(t *testing.T, jobs *postgres.Jobs, scope postgres.ScopeRef,
	chapterID string, results []chapterEnqueueResult) postgres.Job {
	t.Helper()
	ctx := context.Background()
	for i, res := range results {
		if res.err != nil {
			t.Fatalf("request %d: %v", i, res.err)
		}
	}
	list, err := jobs.List(ctx, scope)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var rows []postgres.Job
	for _, row := range list {
		if row.Kind != postgres.JobTTS {
			continue
		}
		var in struct {
			ChapterID string `json:"chapterId"`
		}
		if json.Unmarshal(row.Input, &in) == nil && in.ChapterID == chapterID {
			rows = append(rows, row)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("active tts rows = %d (%+v), want exactly one", len(rows), rows)
	}
	final := rows[0]
	if final.State != postgres.JobQueued || final.WorkerID.Valid || final.LeaseUntil != nil {
		t.Fatalf("final row = %+v, want queued with no native worker/lease", final)
	}
	applied := false
	for i, res := range results {
		if res.job.ID != final.ID {
			t.Fatalf("request %d returned row %s, want the one chapter row %s",
				i, postgres.UUIDString(res.job.ID), postgres.UUIDString(final.ID))
		}
		if res.job.Version > final.Version {
			t.Fatalf("request %d returned version %d, final row is %d", i, res.job.Version, final.Version)
		}
		if res.job.Version != final.Version {
			continue
		}
		applied = true
		if !jsonPayloadEqual(t, final.Input, res.input) {
			t.Fatalf("final row lost the last request's intent:\nfinal=%s\nlast=%v", final.Input, res.input)
		}
	}
	if !applied {
		t.Fatalf("no request produced the final row version %d", final.Version)
	}
	return final
}

func TestJobsEnqueueChapterSerializesPerChapter(t *testing.T) {
	for _, tc := range []struct {
		name string
		slug string
		same bool
	}{
		{name: "same key", slug: "enqueue-race-same", same: true},
		{name: "different keys", slug: "enqueue-race-diff", same: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := provisionNarrative(t, tc.slug+"-proj", tc.slug+"-story")
			jobs := postgres.NewJobs(f.pool)
			ctx := context.Background()
			chID := deterministicUUID("chapter", tc.slug)
			chapterID := postgres.UUIDString(chID)

			// No row exists yet: every request starts from zero, which is the
			// window the advisory lock closes.
			first := burstEnqueueChapter(t, jobs, f.scope, chID, chapterID, 0, tc.same, 8)
			final := assertOneChapterRow(t, jobs, f.scope, chapterID, first)

			// A native worker claims the row; a later burst must supersede the
			// same row and fence that claim out.
			worker := registerTestWorker(t, jobs, "race-"+tc.slug)
			claimed, err := jobs.Claim(ctx, postgres.JobTTS, worker.ID, postgres.DefaultLease, pgtype.UUID{})
			if err != nil {
				t.Fatalf("claim: %v", err)
			}
			if claimed.ID != final.ID {
				t.Fatalf("claimed %s, want %s", postgres.UUIDString(claimed.ID), postgres.UUIDString(final.ID))
			}
			lease := postgres.WorkerLease{
				JobKey:   postgres.JobKey{ProjectID: final.ProjectID, ID: final.ID},
				WorkerID: worker.ID,
				Fence:    claimed.FencingToken,
			}
			oldFence := claimed.FencingToken

			second := burstEnqueueChapter(t, jobs, f.scope, chID, chapterID, 10, tc.same, 8)
			final = assertOneChapterRow(t, jobs, f.scope, chapterID, second)
			if final.FencingToken <= oldFence {
				t.Fatalf("fence = %d, want it past the old claim %d", final.FencingToken, oldFence)
			}
			if _, err := jobs.Complete(ctx, lease, map[string]any{"segments": 1}, 1, nil); !errors.Is(err, postgres.ErrJobLeaseLost) {
				t.Fatalf("old claimant completed after concurrent supersede: %v", err)
			}
		})
	}
}
