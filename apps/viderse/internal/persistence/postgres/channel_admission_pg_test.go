package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/jackc/pgx/v5/pgconn"
)

func admissionInput(key, template string, explicit bool, revision int64, snapshot string) postgres.StartJobInput {
	return postgres.StartJobInput{ChannelID: chOne, SourceKey: key, Source: domain.CloneSourceSnapshot{ID: key, Title: key}, TemplateID: template, TemplateExplicit: explicit, SnapshotRevision: revision, TemplateSnapshot: json.RawMessage(snapshot), ModelRefs: json.RawMessage(`{}`)}
}

func TestChannelDefaultPatchSerializesWithAdmissionAndPreservesOverrides(t *testing.T) {
	pool := provision(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	seedChannelTemplate(t, pool, tplA, "A")
	seedChannelTemplate(t, pool, tplB, "B")
	execChannelSQL(t, pool, `INSERT INTO owner_channels(id,name,default_template_id,auto_fetch) VALUES($1,'Owner',$2,false)`, chOne, tplA)
	store := postgres.NewCloneStore(pool)
	old, err := store.StartJob(ctx, admissionInput("already-admitted", tplA, false, 1, `{"bound":"A1"}`))
	if err != nil {
		t.Fatal(err)
	}
	const queueID = "55555555-5555-4555-8555-555555555555"
	execChannelSQL(t, pool, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,position,template_id) VALUES($1,$2,'explicit-queued','explicit-queued',1,$3)`, queueID, chOne, tplA)
	var queuedBefore string
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(q)::text FROM clone_queue q WHERE id=$1`, queueID).Scan(&queuedBefore); err != nil {
		t.Fatal(err)
	}

	// Use the actual PATCH store inside an open transaction. Admission must
	// wait for this channel lock, then reject a default candidate built at A.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	var patchPID int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&patchPID); err != nil {
		t.Fatal(err)
	}
	ok, err := postgres.NewChannelStore(tx).PatchChannel(ctx, chOne, channels.Patch{DefaultTemplateID: channels.Field[string]{Set: true, Value: tplB}})
	if err != nil || !ok {
		t.Fatalf("patch=%v %v", ok, err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := store.StartJob(ctx, admissionInput("racing-default", tplA, false, 1, `{"bound":"A1"}`))
		result <- err
	}()
	// Observe an actual database wait, not a sleep/scheduler timing guess.
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, patchPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("admission passed locked PATCH: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		var ce *domain.CloneError
		if !errors.As(err, &ce) || ce.Code != domain.CloneErrStale {
			t.Fatalf("old default candidate must retry, got %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clone_jobs WHERE source_key='racing-default')`).Scan(&exists); err != nil || exists {
		t.Fatalf("stale admission persisted job: %v", err)
	}
	next, err := store.StartJob(ctx, admissionInput("racing-default", tplB, false, 1, `{"bound":"B1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Job.TemplateID != tplB || !snapshotBound(next.Job.TemplateSnapshot, "B1") {
		t.Fatalf("incoherent template snapshot %+v", next.Job)
	}
	var queuedAfter string
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(q)::text FROM clone_queue q WHERE id=$1`, queueID).Scan(&queuedAfter); err != nil {
		t.Fatal(err)
	}
	if queuedBefore != queuedAfter {
		t.Fatal("changing default rewrote old explicit queued item")
	}
	in := admissionInput("explicit-queued", tplA, true, 1, `{"bound":"A1"}`)
	in.QueueItemID, in.QueueOverrideRevision = queueID, 1
	override, err := store.StartJob(ctx, in)
	if err != nil || override.Job.TemplateID != tplA {
		t.Fatalf("explicit override lost: %+v %v", override, err)
	}
	persisted, err := store.GetJob(ctx, old.Job.ID)
	if err != nil || persisted.TemplateID != tplA || !reflect.DeepEqual(persisted.TemplateSnapshot, old.Job.TemplateSnapshot) {
		t.Fatalf("old job changed: %+v %v", persisted, err)
	}
	// Template edits invalidate a built snapshot, rather than binding a new
	// revision to old recipe data. No queued item is consumed on rejection.
	execChannelSQL(t, pool, `UPDATE templates SET revision=2,recipe='{"bound":"B2"}' WHERE id=$1`, tplB)
	_, err = store.StartJob(ctx, admissionInput("stale-revision", tplB, false, 1, `{"bound":"B1"}`))
	var ce *domain.CloneError
	if !errors.As(err, &ce) || ce.Code != domain.CloneErrStale {
		t.Fatalf("revision fence=%v", err)
	}
	fresh, err := store.StartJob(ctx, admissionInput("stale-revision", tplB, false, 2, `{"bound":"B2"}`))
	if err != nil || fresh.Job.TemplateID != tplB || !snapshotBound(fresh.Job.TemplateSnapshot, "B2") {
		t.Fatalf("fresh revision binding=%+v %v", fresh, err)
	}
}

func TestChannelInstructionsConcurrentMergeAndAdmissionSnapshot(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedCloneChannel(t, pool)
	channelStore := postgres.NewChannelStore(pool)
	patch := func(key, value string) error {
		ok, err := channelStore.PatchChannel(ctx, chOne, channels.Patch{AgentInstructions: channels.AgentInstructionPatch{Set: true, Key: key, Value: value}})
		if err == nil && !ok {
			return errors.New("missing channel")
		}
		return err
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for key, value := range map[string]string{"director": "slow narration", "artist": "warm palette"} {
		go func(key, value string) { <-start; errs <- patch(key, value) }(key, value)
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	want := domain.AgentInstructions{"director": "slow narration", "artist": "warm palette"}
	view, found, err := channelStore.GetChannel(ctx, chOne)
	if err != nil || !found || !reflect.DeepEqual(view.AgentInstructions, want) {
		t.Fatalf("merged readback=%+v found=%v err=%v", view.AgentInstructions, found, err)
	}
	jobs := postgres.NewCloneStore(pool)
	old, err := jobs.StartJob(ctx, admissionInput("before-instruction-change", fixtureTemplate, false, 1, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old.Job.Instructions, want) {
		t.Fatalf("snapshot=%+v", old.Job.Instructions)
	}
	execChannelSQL(t, pool, `UPDATE clone_jobs SET status='running' WHERE id=$1`, old.Job.ID)
	if err := patch("artist", "cool palette"); err != nil {
		t.Fatal(err)
	}
	if err := patch("qa", "check continuity"); err != nil {
		t.Fatal(err)
	}
	if err := patch("director", ""); err != nil {
		t.Fatal(err)
	}
	nextWant := domain.AgentInstructions{"artist": "cool palette", "qa": "check continuity"}
	view, found, err = channelStore.GetChannel(ctx, chOne)
	if err != nil || !found || !reflect.DeepEqual(view.AgentInstructions, nextWant) {
		t.Fatalf("delete/update merge=%+v %v", view.AgentInstructions, err)
	}
	next, err := jobs.StartJob(ctx, admissionInput("after-instruction-change", fixtureTemplate, false, 1, `{}`))
	if err != nil || !reflect.DeepEqual(next.Job.Instructions, nextWant) {
		t.Fatalf("new snapshot=%+v %v", next, err)
	}
	// Construct a fresh repository to exercise durable readback, not a cached map.
	persisted, err := postgres.NewCloneStore(pool).GetJob(ctx, old.Job.ID)
	if err != nil || !reflect.DeepEqual(persisted.Instructions, want) {
		t.Fatalf("running/queued job snapshot mutated: %+v %v", persisted, err)
	}
}

func TestChannelPerDayDatabaseBoundaryAndAtomicPatch(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedCloneChannel(t, pool)
	store := postgres.NewChannelStore(pool)
	for _, n := range []int{1, 20, 0, 21} {
		name := "valid patch"
		if n == 0 || n == 21 {
			name = "invalid atomic patch"
		}
		ok, err := store.PatchChannel(ctx, chOne, channels.Patch{Name: channels.Field[string]{Set: true, Value: name}, PerDay: channels.Field[int]{Set: true, Value: n}})
		if n == 1 || n == 20 {
			if err != nil || !ok {
				t.Fatalf("perDay %d rejected: %v", n, err)
			}
			continue
		}
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != "23514" {
			t.Fatalf("perDay %d: %v", n, err)
		}
		view, _, err := store.GetChannel(ctx, chOne)
		if err != nil || view.PerDay != 20 || view.Name != "valid patch" {
			t.Fatalf("invalid patch changed persisted channel: %+v %v", view, err)
		}
	}
}

func snapshotBound(raw json.RawMessage, want string) bool {
	var value struct {
		Bound string `json:"bound"`
	}
	return json.Unmarshal(raw, &value) == nil && value.Bound == want
}

func TestChannelCreateAutomationAndPerDayConstraintRoundTrip(t *testing.T) {
	pool := provision(t)
	seedFixtureTemplate(t, pool)
	ctx := context.Background()
	store := postgres.NewChannelStore(pool)
	for _, n := range []int{0, 1, 20, 21} {
		_, err := store.CreateChannel(ctx, channels.CreateInput{ID: chTwo, Name: "Created", Language: domain.LangVI, DefaultTemplateID: fixtureTemplate, AutoFetch: false, AutoApprove: true, PerDay: n})
		if n == 0 || n == 21 {
			var pe *pgconn.PgError
			if !errors.As(err, &pe) || pe.Code != "23514" {
				t.Fatalf("create perDay=%d: %v", n, err)
			}
			if _, found, err := store.GetChannel(ctx, chTwo); err != nil || found {
				t.Fatalf("invalid creation persisted channel: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		view, found, err := store.GetChannel(ctx, chTwo)
		if err != nil || !found || view.AutoFetch || !view.AutoApprove || view.PerDay != n || view.DefaultTemplateID != fixtureTemplate {
			t.Fatalf("create roundtrip=%+v %v", view, err)
		}
		if ok, err := store.DeleteChannel(ctx, chTwo); err != nil || !ok {
			t.Fatalf("delete=%v %v", ok, err)
		}
	}
}

func TestChannelServiceResolvesAgainAfterDefaultCommitsBeforeAdmission(t *testing.T) {
	pool := provision(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	seedChannelTemplate(t, pool, tplA, "A")
	seedChannelTemplate(t, pool, tplB, "B")
	execChannelSQL(t, pool, `INSERT INTO owner_channels(id,name,default_template_id) VALUES($1,'Owner',$2)`, chOne, tplA)
	channelsStore := postgres.NewChannelStore(pool)
	prepared := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	service := &pipeline.Service{Store: postgres.NewCloneStore(pool)}
	service.ResolveTemplate = func(ctx context.Context, item domain.CloneQueueItem) (string, json.RawMessage, json.RawMessage, int64, error) {
		view, _, err := channelsStore.GetChannel(ctx, item.ChannelID)
		if err != nil {
			return "", nil, nil, 0, err
		}
		id := view.DefaultTemplateID
		if item.TemplateID != nil {
			id = *item.TemplateID
		}
		calls++
		if calls == 1 {
			close(prepared)
			select {
			case <-release:
			case <-ctx.Done():
				return "", nil, nil, 0, ctx.Err()
			}
		}
		bound := "A1"
		if id == tplB {
			bound = "B1"
		}
		raw, err := json.Marshal(map[string]string{"bound": bound})
		return id, raw, json.RawMessage(`{}`), 1, err
	}
	type outcome struct {
		job *domain.CloneJob
		err error
	}
	result := make(chan outcome, 1)
	go func() {
		job, err := service.Start(ctx, domain.CloneQueueItem{ChannelID: chOne, SourceKey: "service-race", Source: domain.CloneSourceSnapshot{ID: "service-race", Title: "Race"}})
		result <- outcome{job, err}
	}()
	select {
	case <-prepared:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	ok, err := channelsStore.PatchChannel(ctx, chOne, channels.Patch{DefaultTemplateID: channels.Field[string]{Set: true, Value: tplB}})
	if err != nil || !ok {
		close(release)
		t.Fatalf("patch=%v %v", ok, err)
	}
	close(release)
	select {
	case got := <-result:
		if got.err != nil || got.job == nil || got.job.TemplateID != tplB || !snapshotBound(got.job.TemplateSnapshot, "B1") || calls != 2 {
			t.Fatalf("service admitted incoherent/default-stale job: %+v err=%v resolves=%d", got.job, got.err, calls)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// The same admission path must not reinterpret an explicit A as a stale
	// default candidate now that B is the channel default.
	override := tplA
	got, err := service.Start(ctx, domain.CloneQueueItem{ChannelID: chOne, SourceKey: "service-override", Source: domain.CloneSourceSnapshot{ID: "service-override", Title: "Override"}, TemplateID: &override})
	if err != nil || got == nil || got.TemplateID != tplA || !snapshotBound(got.TemplateSnapshot, "A1") {
		t.Fatalf("explicit override=%+v err=%v", got, err)
	}
}
