package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
)

func queuePatchResponse(t *testing.T, svc *queue.Service, id, action string, status int) []byte {
	t.Helper()
	body, err := json.Marshal(domain.QueuePatch{ID: &id, Action: action})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	patchQueue(w, httptest.NewRequest("PATCH", "/api/queue", bytes.NewReader(body)), svc, func(err error) { t.Log(err) })
	if w.Code != status {
		t.Fatalf("PATCH %s status=%d want=%d body=%s", action, w.Code, status, w.Body.String())
	}
	return w.Body.Bytes()
}

func TestQueueDurableRemoveRestore(t *testing.T) {
	_, pool := planFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	events := []string{}
	repo := postgres.NewQueueStore(pool)
	svc := queue.New(queue.Options{Pool: pool, Store: repo, Settings: postgres.NewSettingsStore(pool), Now: func() time.Time { return now }, Notify: func(event string) { events = append(events, event) }})
	templateID := "aaaaaaaa-1111-4111-8111-111111111111"
	enqueued, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: []string{"https://youtu.be/aaaaaaaaaaa"}, Priority: domain.ClonePick, TemplateID: &templateID, NoImages: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(enqueued.Items) != 1 {
		t.Fatalf("enqueue=%+v", enqueued)
	}
	id := enqueued.Items[0].ID
	position := 12.5
	if _, err := svc.Patch(ctx, domain.QueuePatch{ID: &id, Position: &position}); err != nil {
		t.Fatal(err)
	}
	before, err := repo.GetQueueItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	recipeBefore, err := repo.TemplateRecipe(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	events = nil
	removedBody := queuePatchResponse(t, svc, id, "remove", 200)
	stored, err := repo.GetQueueItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil {
		t.Fatal("remove physically deleted the queue row; Undo must retain its identity")
	}
	var removed struct {
		domain.CloneQueueView
		Receipt *struct {
			ID       string               `json:"id"`
			Position float64              `json:"position"`
			Priority domain.ClonePriority `json:"priority"`
		} `json:"removed"`
	}
	if err := json.Unmarshal(removedBody, &removed); err != nil {
		t.Fatal(err)
	}
	if len(removed.Items) != 0 || removed.Receipt == nil || removed.Receipt.ID != id || removed.Receipt.Position != position || removed.Receipt.Priority != before.Priority {
		t.Fatalf("remove response=%s", removedBody)
	}
	if len(events) != 1 || events[0] != domain.CloneEventQueue {
		t.Fatalf("remove events=%v", events)
	}
	view, err := svc.List(ctx)
	if err != nil || len(view.Items) != 0 || view.Rotation.Next != nil {
		t.Fatalf("removed row leaked into view/ETA: view=%+v err=%v", view, err)
	}
	if n, err := repo.CountTotalQueued(ctx); err != nil || n != 0 {
		t.Fatalf("removed row counted: n=%d err=%v", n, err)
	}
	if counts, err := repo.ListQueueCountsByChannel(ctx); err != nil || len(counts) != 0 {
		t.Fatalf("removed row in lane badge: %v err=%v", counts, err)
	}
	if counts, err := gen.New(pool).CountQueuedByChannel(ctx); err != nil || len(counts) != 0 {
		t.Fatalf("removed row in lane count: %v err=%v", counts, err)
	}
	if lane, err := repo.ListLaneItems(ctx, before.ChannelID); err != nil || len(lane) != 0 {
		t.Fatalf("removed row in lane: %v err=%v", lane, err)
	}
	if keys, err := repo.QueuedSourceKeys(ctx); err != nil || len(keys) != 0 {
		t.Fatalf("removed row in duplicate keys: %v err=%v", keys, err)
	}
	if rows, err := repo.FindQueueRowsBySourceKeys(ctx, []string{before.SourceKey}); err != nil || len(rows) != 0 {
		t.Fatalf("removed row blocks planning: %v err=%v", rows, err)
	}
	// Stale reorder/skip commands must not mutate a removed row either.
	if err := repo.PromotePick(ctx, id, 999); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateQueuePosition(ctx, id, 888); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkQueueItemSkipped(ctx, id, "stale"); err != nil {
		t.Fatal(err)
	}
	events = nil
	queuePatchResponse(t, svc, id, "restore", 200)
	after, err := repo.GetQueueItem(ctx, id)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("restore changed queue identity or recipe selection: before=%+v after=%+v err=%v", before, after, err)
	}
	recipeAfter, err := repo.TemplateRecipe(ctx, templateID)
	if err != nil || !bytes.Equal(recipeBefore, recipeAfter) {
		t.Fatalf("restore changed recipe: before=%s after=%s err=%v", recipeBefore, recipeAfter, err)
	}
	if len(events) != 1 || events[0] != domain.CloneEventQueue {
		t.Fatalf("restore events=%v", events)
	}
}

func TestQueueRestoreConflicts(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	enqueued, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: []string{"https://youtu.be/aaaaaaaaaaa", "https://youtu.be/bbbbbbbbbbb"}})
	if err != nil || len(enqueued.Items) != 2 {
		t.Fatalf("enqueue=%+v err=%v", enqueued, err)
	}
	started, purged := enqueued.Items[0].ID, enqueued.Items[1].ID
	if _, err := pool.Exec(ctx, `UPDATE clone_queue SET state='started',started_at=now() WHERE id=$1`, started); err != nil {
		t.Fatal(err)
	}
	queuePatchResponse(t, svc, started, "restore", 409)
	queuePatchResponse(t, svc, purged, "remove", 200)
	if _, err := pool.Exec(ctx, `DELETE FROM clone_queue WHERE id=$1`, purged); err != nil {
		t.Fatal(err)
	}
	queuePatchResponse(t, svc, purged, "restore", 409)
	queuePatchResponse(t, svc, "not-a-uuid", "restore", 400)
	replacement, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: enqueued.Items[1].ChannelID, URLs: []string{"https://youtu.be/bbbbbbbbbbb"}})
	if err != nil || len(replacement.Items) != 1 {
		t.Fatalf("enqueue after purge=%+v err=%v", replacement, err)
	}
	original := replacement.Items[0]
	queuePatchResponse(t, svc, original.ID, "remove", 200)
	replacement, err = svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: original.ChannelID, URLs: []string{"https://youtu.be/bbbbbbbbbbb"}})
	if err != nil || len(replacement.Items) != 1 || replacement.Items[0].ID == original.ID {
		t.Fatalf("removed row still blocks duplicate detection: result=%+v err=%v", replacement, err)
	}
	queuePatchResponse(t, svc, original.ID, "restore", 409)
	queuePatchResponse(t, svc, replacement.Items[0].ID, "remove", 200)
	if _, err := pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status,stage_group) VALUES ('22222222-2222-4222-8222-222222222222',$1,$2,'{}','succeeded','ready')`, original.ChannelID, original.SourceKey); err != nil {
		t.Fatal(err)
	}
	queuePatchResponse(t, svc, original.ID, "restore", 409)
	if _, err := pool.Exec(ctx, `DELETE FROM clone_jobs WHERE id='22222222-2222-4222-8222-222222222222'`); err != nil {
		t.Fatal(err)
	}
	queuePatchResponse(t, svc, original.ID, "restore", 200)
	queuePatchResponse(t, svc, original.ID, "restore", 200)
}

func TestQueueRemovedSchedulerAndRetention(t *testing.T) {
	svc, pool := planFixture(t)
	ctx := context.Background()
	enqueued, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: []string{"https://youtu.be/aaaaaaaaaaa", "https://youtu.be/bbbbbbbbbbb"}})
	if err != nil || len(enqueued.Items) != 2 {
		t.Fatalf("enqueue=%+v err=%v", enqueued, err)
	}
	repo := postgres.NewQueueStore(pool)
	if _, err := pool.Exec(ctx, `UPDATE clone_queue SET origin='auto'`); err != nil {
		t.Fatal(err)
	}
	for _, item := range enqueued.Items {
		queuePatchResponse(t, svc, item.ID, "remove", 200)
		if row, err := repo.GetQueueItem(ctx, item.ID); err != nil || row == nil {
			t.Fatalf("remove must retain queue row for scheduler race: row=%+v err=%v", row, err)
		}
	}
	if rival, err := repo.LastAutoQueuedChannel(ctx, enqueued.Items[0].ChannelID); err != nil || rival != "" {
		t.Fatalf("removed auto row controls rotation: rival=%q err=%v", rival, err)
	}
	if ids, err := repo.ListEligibleVideos(ctx, "UCaaaaaaaaaaaaaaaaaaaaaa", time.Now().Add(-24*time.Hour), enqueued.Items[0].ChannelID, 0); err != nil || len(ids) != 2 {
		t.Fatalf("removed rows block auto candidates: ids=%v err=%v", ids, err)
	}
	stale := enqueued.Items[0]
	if claimed, err := repo.MarkQueueItemStarted(ctx, stale.ID, nil, time.Now()); err != nil || claimed {
		t.Fatalf("scheduler claimed removed row: claimed=%v err=%v", claimed, err)
	}
	// Exercise the actual admission transaction with a stale scheduler snapshot;
	// a rejected claim must roll back the job and all of its steps.
	_, err = postgres.NewCloneStore(pool).StartJob(ctx, pipeline.StartJobInput{ChannelID: stale.ChannelID, SourceKey: stale.SourceKey, Source: stale.Source, TemplateID: "aaaaaaaa-1111-4111-8111-111111111111", TemplateSnapshot: json.RawMessage(`{}`), ModelRefs: json.RawMessage(`{}`), Priority: stale.Priority, Origin: stale.Origin, QueueItemID: stale.ID})
	var conflict *domain.CloneError
	if !errors.As(err, &conflict) || conflict.Code != domain.CloneErrConflict {
		t.Fatalf("stale scheduler admission must conflict: %v", err)
	}
	var jobs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM clone_jobs`).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("removed claim left a job: jobs=%d err=%v", jobs, err)
	}
	now := time.Now().UTC()
	old, recent := enqueued.Items[0].ID, enqueued.Items[1].ID
	if _, err := pool.Exec(ctx, `UPDATE clone_queue SET removed_at=$1,created_at=$2 WHERE id=$3`, now.Add(-24*time.Hour-time.Second), now, old); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE clone_queue SET removed_at=$1,created_at=$2 WHERE id=$3`, now.Add(-23*time.Hour), now.Add(-40*24*time.Hour), recent); err != nil {
		t.Fatal(err)
	}
	// The existing drain sweep must prune even when paused or all workers busy.
	paused := true
	if _, err := svc.Patch(ctx, domain.QueuePatch{Paused: &paused}); err != nil {
		t.Fatal(err)
	}
	sweeper := queue.New(queue.Options{Pool: pool, Store: repo, Settings: postgres.NewSettingsStore(pool), Now: func() time.Time { return now }})
	if _, err := sweeper.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if row, err := repo.GetQueueItem(ctx, old); err != nil || row != nil {
		t.Fatalf("24h removed row not purged: row=%+v err=%v", row, err)
	}
	if row, err := repo.GetQueueItem(ctx, recent); err != nil || row == nil {
		t.Fatalf("recent removal incorrectly purged by creation time: row=%+v err=%v", row, err)
	}
	queuePatchResponse(t, sweeper, old, "restore", 409)
	queuePatchResponse(t, sweeper, recent, "restore", 200)
}
