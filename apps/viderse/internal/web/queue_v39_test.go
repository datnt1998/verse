package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
)

const v39Owner = "11111111-1111-4111-8111-111111111111"
const v39OtherOwner = "99999999-1111-4111-8111-111111111111"

func TestJobsCanonicalRouteRejectsRetiredCreationContracts(t *testing.T) {
	enqueuer := &fakeCloneEnqueuer{}
	tg, cookie := cloneFixture(t, &fakeCloneService{}, enqueuer, nil)
	for _, path := range []string{"/api/clone", "/api/clone/enqueue"} {
		w := tg.do(http.MethodPost, path, cookie, `{"channelId":"channel-1","urls":["https://youtu.be/aaaaaaaaaaa"]}`)
		if w.Code == http.StatusCreated || enqueuer.calls != 0 {
			t.Fatalf("retired creation route %s accepted: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := tg.do(http.MethodPost, "/api/jobs", "", `{"channelId":"channel-1","urls":["https://youtu.be/aaaaaaaaaaa"]}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create: %d %s", w.Code, w.Body.String())
	}
}

func TestJobsMixedResultAndPersistedQueueTarget(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	paused := true
	if _, err := svc.Patch(ctx, domain.QueuePatch{Paused: &paused}); err != nil {
		t.Fatal(err)
	}
	tg := newTestGateway(t)
	MountClone(tg.g, &fakeCloneService{}, svc, nil)
	MountQueue(tg.g, svc)
	cookie := tg.sessionCookie()
	w := tg.do(http.MethodPost, "/api/jobs", cookie, `{"channelId":"`+v39Owner+`","urls":["https://youtu.be/aaaaaaaaaaa","https://example.com/not-youtube","https://youtube.com/watch?v=aaaaaaaaaaa","https://youtu.be/bbbbbbbbbbb"]}`)
	var created domain.CloneEnqueueResult
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || w.Code != http.StatusCreated || len(created.Items) != 2 || len(created.Duplicates) != 1 || len(created.Invalid) != 1 {
		t.Fatalf("mixed result=%+v status=%d err=%v body=%s", created, w.Code, err, w.Body.String())
	}
	id := created.Items[0].ID
	if created.Duplicates[0].QueueItemID == nil || *created.Duplicates[0].QueueItemID != id || created.Duplicates[0].JobID != nil || created.Invalid[0].Reason != "not_youtube" {
		t.Fatalf("untruthful duplicate/invalid: %+v", created)
	}
	w = tg.do(http.MethodPost, "/api/jobs", cookie, `{"channelId":"`+v39Owner+`","urls":["https://youtu.be/aaaaaaaaaaa","https://example.com/not-youtube"]}`)
	var retry domain.CloneEnqueueResult
	if err := json.Unmarshal(w.Body.Bytes(), &retry); err != nil || w.Code != http.StatusCreated || len(retry.Items) != 0 || len(retry.Duplicates) != 1 || retry.Duplicates[0].QueueItemID == nil || *retry.Duplicates[0].QueueItemID != id || len(retry.Invalid) != 1 {
		t.Fatalf("retry duplicated accepted work: %+v %v status=%d", retry, err, w.Code)
	}
	readTarget := func(jobID *string, state domain.CloneQueueState) {
		t.Helper()
		response := tg.do(http.MethodGet, "/api/queue/items/"+id, cookie, "")
		var target domain.QueueItemTarget
		if err := json.Unmarshal(response.Body.Bytes(), &target); err != nil || response.Code != http.StatusOK || target.ID != id || target.ChannelID != v39Owner || !reflect.DeepEqual(target.JobID, jobID) || target.State != state {
			t.Fatalf("target=%+v status=%d err=%v body=%s", target, response.Code, err, response.Body.String())
		}
	}
	readTarget(nil, domain.QueueQueued)
	if response := tg.do(http.MethodGet, "/api/queue/items/"+id, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("queue target leaked without session: %d", response.Code)
	}
	item := created.Items[0]
	admitted, err := postgres.NewCloneStore(pool).StartJob(ctx, pipeline.StartJobInput{ChannelID: item.ChannelID, SourceKey: item.SourceKey, Source: item.Source, TemplateSnapshot: json.RawMessage(`{}`), ModelRefs: json.RawMessage(`{}`), Priority: item.Priority, Origin: item.Origin, QueueItemID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	readTarget(&admitted.Job.ID, domain.QueueStarted)
	otherID := created.Items[1].ID
	if _, err = svc.Patch(ctx, domain.QueuePatch{ID: &otherID, Action: "remove"}); err != nil {
		t.Fatal(err)
	}
	for _, missing := range []string{otherID, "not-a-uuid", "00000000-0000-4000-8000-000000000000"} {
		if response := tg.do(http.MethodGet, "/api/queue/items/"+missing, cookie, ""); response.Code != http.StatusNotFound {
			t.Fatalf("missing/removed target %s: %d %s", missing, response.Code, response.Body.String())
		}
	}
}

func TestQueueManualBlockPrecedesPendingAndAutoWithoutChangingInflight(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO rival_videos(id,channel_id,title,position,origin) VALUES ('ccccccccccc','UCaaaaaaaaaaaaaaaaaaaaaa','Video C',3,'discovery')`,
		`INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status) VALUES ('eeeeeeee-1111-4111-8111-111111111111','` + v39Owner + `','inflight','{"id":"eeeeeeeeeee","title":"E"}','running')`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: v39Owner, URLs: []string{"https://youtu.be/aaaaaaaaaaa"}}); err != nil {
		t.Fatal(err)
	}
	block, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: v39Owner, URLs: []string{"https://youtu.be/bbbbbbbbbbb", "https://youtu.be/ccccccccccc"}})
	if err != nil || len(block.Items) != 2 {
		t.Fatalf("block=%+v err=%v", block, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,source_snapshot,priority,origin,position) VALUES ('dddddddd-1111-4111-8111-111111111111',$1,'ddddddddddd','auto-d','{"id":"ddddddddddd","title":"D"}','batch','auto',1)`, v39Owner); err != nil {
		t.Fatal(err)
	}
	rows, err := postgres.NewQueueStore(pool).ListQueuedItems(ctx)
	if err != nil || len(rows) != 4 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	ids := []string{rows[0].SourceVideoID, rows[1].SourceVideoID, rows[2].SourceVideoID, rows[3].SourceVideoID}
	if !reflect.DeepEqual(ids, []string{"bbbbbbbbbbb", "ccccccccccc", "aaaaaaaaaaa", "ddddddddddd"}) {
		t.Fatalf("pending order=%v", ids)
	}
	var state string
	var generation int
	if err := pool.QueryRow(ctx, `SELECT status,generation FROM clone_jobs WHERE source_key='inflight'`).Scan(&state, &generation); err != nil || state != "running" || generation != 1 {
		t.Fatalf("inflight changed: %s/%d %v", state, generation, err)
	}
}

func TestQueueConcurrentDedupAndCrossChannelAllocation(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,default_template_id) VALUES ($1,'Khác','aaaaaaaa-1111-4111-8111-111111111111')`, v39OtherOwner); err != nil {
		t.Fatal(err)
	}
	const requests = 12
	results := make(chan domain.CloneEnqueueResult, requests)
	errors := make(chan error, requests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			owner := v39Owner
			if i%2 != 0 {
				owner = v39OtherOwner
			}
			out, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: owner, URLs: []string{"https://youtu.be/aaaaaaaaaaa"}})
			results <- out
			errors <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := postgres.NewQueueStore(pool).ListQueuedItems(ctx)
	if err != nil || len(rows) != 2 || rows[0].Position == rows[1].Position {
		t.Fatalf("dedup/allocation rows=%+v err=%v", rows, err)
	}
	realIDs := map[string]bool{rows[0].ID: true, rows[1].ID: true}
	inserted, duplicates := 0, 0
	for out := range results {
		inserted += len(out.Items)
		duplicates += len(out.Duplicates)
		for _, duplicate := range out.Duplicates {
			if duplicate.QueueItemID == nil || !realIDs[*duplicate.QueueItemID] || duplicate.JobID != nil {
				t.Fatalf("concurrent dedup returned nonpersisted entity: %+v", duplicate)
			}
		}
	}
	if inserted != 2 || duplicates != requests-2 {
		t.Fatalf("inserted=%d duplicates=%d", inserted, duplicates)
	}
}

func TestQueueGlobalSelectionAndBoundsBeyondFetchCap(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	// Three channels, each below its lane cap, collectively beyond fetch cap.
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,default_template_id) VALUES ('55555555-1111-4111-8111-111111111111','Giữa','aaaaaaaa-1111-4111-8111-111111111111'),($1,'Cuối','aaaaaaaa-1111-4111-8111-111111111111')`, v39OtherOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,source_snapshot,priority,origin,position) SELECT md5(c.id::text || n::text)::uuid,c.id,lpad(n::text,11,'0'),'bound:'||c.id::text||':'||n,'{"title":"Auto"}','batch','auto',n FROM owner_channels c CROSS JOIN generate_series(1,800) n`); err != nil {
		t.Fatal(err)
	}
	// An auto tail outside the capped read must still participate in allocation.
	if _, err := pool.Exec(ctx, `UPDATE clone_queue SET position=100000 WHERE channel_id=$1 AND source_video_id='00000000800'`, v39OtherOwner); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewQueueStore(pool)
	minimum, maximum, err := repo.QueuePositionBounds(ctx)
	if err != nil || minimum != 0 || maximum != 100000 {
		t.Fatalf("complete bounds=%v/%v err=%v", minimum, maximum, err)
	}
	out, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: v39OtherOwner, URLs: []string{"https://youtu.be/aaaaaaaaaaa", "https://youtu.be/bbbbbbbbbbb"}})
	if err != nil || len(out.Items) != 2 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	rows, err := repo.ListQueuedItems(ctx)
	if err != nil || len(rows) != queueFetchCapForTest || rows[0].ID != out.Items[0].ID || rows[1].ID != out.Items[1].ID {
		t.Fatalf("manual omitted before cap: rows=%d err=%v", len(rows), err)
	}
	id := rows[len(rows)-1].ID
	if _, err = svc.Patch(ctx, domain.QueuePatch{ID: &id, Action: "front"}); err != nil {
		t.Fatal(err)
	}
	rows, err = repo.ListQueuedItems(ctx)
	if err != nil || len(rows) == 0 || rows[0].ID != id || rows[0].Position >= out.Items[0].Position {
		t.Fatalf("front ignored complete manual bounds: rows=%d err=%v", len(rows), err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status) VALUES ('ffffffff-1111-4111-8111-111111111111',$1,'restart-bound','{"id":"ccccccccccc","title":"Restart"}','failed')`, v39Owner); err != nil {
		t.Fatal(err)
	}
	job, err := postgres.NewCloneStore(pool).GetJob(ctx, "ffffffff-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := svc.EnqueueRestart(ctx, job)
	if err != nil || restarted.Position >= rows[0].Position {
		t.Fatalf("restart ignored complete bounds: %+v err=%v", restarted, err)
	}
}

const queueFetchCapForTest = 2000

func TestQueueRulesRejectRemovedFieldsAndOutOfBoundsWorkers(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	settings := postgres.NewSettingsStore(pool)
	for key, value := range map[string]int{"clone.imageConcurrency": 8, "clone.rescanHours": 24} {
		if err := settings.Set(ctx, key, value); err != nil {
			t.Fatal(err)
		}
	}
	tg := newTestGateway(t)
	MountQueue(tg.g, svc)
	cookie := tg.sessionCookie()
	for _, body := range []string{`{"workers":0}`, `{"workers":7}`, `{"imageWorkers":3}`, `{"rescanHours":1}`} {
		w := tg.do(http.MethodPatch, "/api/queue", cookie, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid rules %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	for _, workers := range []int{1, 6} {
		w := tg.do(http.MethodPatch, "/api/queue", cookie, fmt.Sprintf(`{"workers":%d}`, workers))
		var view domain.CloneQueueView
		if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil || w.Code != http.StatusOK || view.Workers.Total != workers || view.ImageWorkers != 3 || view.RescanHours != 1 {
			t.Fatalf("rules=%+v status=%d err=%v", view, w.Code, err)
		}
	}
}

func TestQueueConcurrentEnqueueFrontAndRestartAllocateOneGlobalOrder(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,default_template_id) VALUES ($1,'Khác','aaaaaaaa-1111-4111-8111-111111111111')`, v39OtherOwner); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: v39Owner, URLs: []string{"https://youtu.be/aaaaaaaaaaa"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status) VALUES ('ffffffff-1111-4111-8111-111111111111',$1,'restart-race','{"id":"ccccccccccc","title":"Restart"}','failed')`, v39OtherOwner); err != nil {
		t.Fatal(err)
	}
	old, err := postgres.NewCloneStore(pool).GetJob(ctx, "ffffffff-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errors := make(chan error, 3)
	actions := []func() error{
		func() error {
			_, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: v39OtherOwner, URLs: []string{"https://youtu.be/bbbbbbbbbbb"}})
			return err
		},
		func() error {
			id := first.Items[0].ID
			_, err := svc.Patch(ctx, domain.QueuePatch{ID: &id, Action: "front"})
			return err
		},
		func() error {
			_, err := svc.EnqueueRestart(ctx, old)
			return err
		},
	}
	for _, action := range actions {
		go func(action func() error) {
			<-start
			errors <- action()
		}(action)
	}
	close(start)
	for range actions {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := postgres.NewQueueStore(pool).ListQueuedItems(ctx)
	if err != nil || len(rows) != 3 {
		t.Fatalf("concurrent writers lost rows: count=%d err=%v", len(rows), err)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Position >= rows[i].Position {
			t.Fatalf("global allocation collision: %+v", rows)
		}
	}
	previous, err := postgres.NewCloneStore(pool).GetJob(ctx, old.ID)
	if err != nil || previous.SupersededAt != nil || previous.Status != domain.CloneFailed {
		t.Fatalf("restart changed old job before admission: %+v err=%v", previous, err)
	}
}

func TestPlanReadsUnknownSourcesConcurrentlyWithBoundedInputOrder(t *testing.T) {
	_, pool := planFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var mu sync.Mutex
	active, maximum, calls := 0, 0, 0
	svc := queue.New(queue.Options{
		Pool: pool, Store: postgres.NewQueueStore(pool), Settings: postgres.NewSettingsStore(pool),
		Probe: func(ctx context.Context, id string) (queue.VideoMetadata, error) {
			mu.Lock()
			active++
			calls++
			maximum = max(maximum, active)
			mu.Unlock()
			entered <- struct{}{}
			defer func() {
				mu.Lock()
				active--
				mu.Unlock()
			}()
			select {
			case <-release:
				return queue.VideoMetadata{Source: domain.CloneSourceSnapshot{ID: id, Title: id}}, nil
			case <-ctx.Done():
				return queue.VideoMetadata{}, ctx.Err()
			}
		},
	})
	type outcome struct {
		plan domain.ClonePlan
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		plan, err := svc.Plan(ctx, domain.CloneEnqueueRequest{ChannelID: v39Owner, URLs: []string{"https://youtu.be/ccccccccccc", "https://youtu.be/ddddddddddd", "https://youtu.be/eeeeeeeeeee", "https://youtube.com/watch?v=ccccccccccc"}})
		done <- outcome{plan, err}
	}()
	for range 2 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("metadata reads did not overlap")
		}
	}
	releaseOnce.Do(func() { close(release) })
	var result outcome
	select {
	case result = <-done:
	case <-ctx.Done():
		t.Fatal("plan did not finish")
	}
	mu.Lock()
	actualMaximum, actualCalls := maximum, calls
	mu.Unlock()
	if result.err != nil || actualMaximum != 2 || actualCalls != 3 || len(result.plan.Accepted) != 3 {
		t.Fatalf("metadata plan=%+v err=%v max=%d calls=%d", result.plan, result.err, actualMaximum, actualCalls)
	}
	for i, id := range []string{"ccccccccccc", "ddddddddddd", "eeeeeeeeeee"} {
		if result.plan.Accepted[i].VideoID != id {
			t.Fatalf("concurrent reads lost input order: %+v", result.plan.Accepted)
		}
	}
}

// Hide duplicates only from preflight reads; With still binds the real store
// to the enqueue transaction. This exercises a stale plan followed by actual
// transactional duplicate resolution without replacing persistence behavior.
type staleRestartDuplicatePlan struct{ queue.Store }

func (s staleRestartDuplicatePlan) FindJobsBySourceKeys(context.Context, []string) ([]domain.CloneJob, error) {
	return nil, nil
}

func (s staleRestartDuplicatePlan) FindQueueRowsBySourceKeys(context.Context, []string) ([]domain.CloneQueueItem, error) {
	return nil, nil
}

func TestQueuedRestartDuplicateTargetsRestartUntilAdmission(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	paused := true
	if _, err := svc.Patch(ctx, domain.QueuePatch{Paused: &paused}); err != nil {
		t.Fatal(err)
	}
	request := domain.CloneEnqueueRequest{ChannelID: v39Owner, URLs: []string{"https://youtu.be/aaaaaaaaaaa"}}
	initial, err := svc.Enqueue(ctx, request)
	if err != nil || len(initial.Items) != 1 {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	store := postgres.NewCloneStore(pool)
	first := initial.Items[0]
	admitted, err := store.StartJob(ctx, pipeline.StartJobInput{ChannelID: first.ChannelID, SourceKey: first.SourceKey, Source: first.Source, TemplateSnapshot: json.RawMessage(`{}`), ModelRefs: json.RawMessage(`{}`), Priority: first.Priority, Origin: first.Origin, QueueItemID: first.ID})
	if err != nil {
		t.Fatal(err)
	}
	old := admitted.Job
	if err := store.SetJobStatus(ctx, old.ID, domain.CloneFailed); err != nil {
		t.Fatal(err)
	}
	restart, err := svc.EnqueueRestart(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	assertDuplicate := func(duplicates []domain.ClonePlanDuplicate) {
		t.Helper()
		if len(duplicates) != 1 || duplicates[0].QueueItemID == nil || *duplicates[0].QueueItemID != restart.ID || duplicates[0].JobID != nil || duplicates[0].State != string(domain.QueueQueued) {
			t.Fatalf("duplicate lost live restart target: %+v; restart=%s old=%s", duplicates, restart.ID, old.ID)
		}
	}
	plan, err := svc.Plan(ctx, request)
	if err != nil || len(plan.Accepted) != 0 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	assertDuplicate(plan.Duplicates)
	retry, err := svc.Enqueue(ctx, request)
	if err != nil || len(retry.Items) != 0 {
		t.Fatalf("retry created another item: %+v err=%v", retry, err)
	}
	assertDuplicate(retry.Duplicates)
	stale := queue.New(queue.Options{Pool: pool, Store: staleRestartDuplicatePlan{postgres.NewQueueStore(pool)}, Settings: postgres.NewSettingsStore(pool)})
	raced, err := stale.Enqueue(ctx, request)
	if err != nil || len(raced.Items) != 0 {
		t.Fatalf("stale plan created another item: %+v err=%v", raced, err)
	}
	assertDuplicate(raced.Duplicates)
	previous, err := store.GetJob(ctx, old.ID)
	if err != nil || previous.SupersededAt != nil {
		t.Fatalf("predecessor changed before admission: %+v err=%v", previous, err)
	}
	tg := newTestGateway(t)
	MountQueue(tg.g, svc)
	cookie := tg.sessionCookie()
	readTarget := func(jobID *string, state domain.CloneQueueState) {
		t.Helper()
		response := tg.do(http.MethodGet, "/api/queue/items/"+restart.ID, cookie, "")
		var target domain.QueueItemTarget
		if err := json.Unmarshal(response.Body.Bytes(), &target); err != nil || response.Code != http.StatusOK || target.ID != restart.ID || !reflect.DeepEqual(target.JobID, jobID) || target.State != state {
			t.Fatalf("restart target=%+v status=%d err=%v", target, response.Code, err)
		}
	}
	readTarget(nil, domain.QueueQueued)
	next, err := store.StartJob(ctx, pipeline.StartJobInput{ChannelID: restart.ChannelID, SourceKey: restart.SourceKey, Source: restart.Source, TemplateID: *restart.TemplateID, TemplateExplicit: true, TemplateSnapshot: json.RawMessage(`{}`), ModelRefs: json.RawMessage(`{}`), Priority: restart.Priority, Origin: restart.Origin, QueueItemID: restart.ID})
	if err != nil {
		t.Fatal(err)
	}
	if next.Job.ID == old.ID || next.Job.Generation != old.Generation+1 {
		t.Fatalf("restart did not admit a new generation: %+v", next.Job)
	}
	readTarget(&next.Job.ID, domain.QueueStarted)
}
