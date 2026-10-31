package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
)

type restartBoard struct{ store *postgres.QueueStore }

func (b restartBoard) BoardItems(ctx context.Context, _ *string) ([]domain.CloneQueueItem, error) {
	return b.store.ListQueuedItems(ctx)
}

func TestCloneRestartQueuesBeforeAdmission(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	_, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,auto_pick) VALUES('11111111-1111-4111-8111-111111111111','Test',false); INSERT INTO templates(id,name,recipe) VALUES('22222222-2222-4222-8222-222222222222','Next','{"scenes":{"images":{"mode":"fixed","count":5},"coverSource":"ai"}}'); INSERT INTO owner_channel_templates(owner_channel_id,template_id,position) VALUES('11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222',0)`)
	if err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	old := cloneTestJob(t, store, "restart-queued")
	if err = store.SetJobStatus(ctx, old.ID, domain.CloneFailed); err != nil {
		t.Fatal(err)
	}
	qs := queue.New(queue.Options{Pool: pool, Store: postgres.NewQueueStore(pool), Settings: postgres.NewSettingsStore(pool)})
	service := &pipeline.Service{Store: store, Queue: restartBoard{postgres.NewQueueStore(pool)}, PlanRestart: qs.RestartPlan, EnqueueRestart: qs.EnqueueRestart, ResolveTemplate: func(context.Context, domain.CloneQueueItem) (string, json.RawMessage, json.RawMessage, error) {
		return "22222222-2222-4222-8222-222222222222", json.RawMessage(`{}`), json.RawMessage(`{}`), nil
	}}
	plan, err := service.RestartPlan(ctx, old.ID)
	if err != nil || plan.Template.Name != "Next" || plan.ImageEstimate == nil || *plan.ImageEstimate != 6 || plan.Position != "front" || plan.ETAStart == nil {
		t.Fatalf("preview: %+v %v", plan, err)
	}
	result, err := service.Restart(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := postgres.NewQueueStore(pool).ListQueuedItems(ctx)
	if err != nil || len(rows) != 1 {
		t.Fatalf("restart must be queued, got %v: %+v result=%+v", err, rows, result)
	}
	if rows[0].ID != result.ID || rows[0].Priority != domain.ClonePick || rows[0].SourceKey != old.SourceKey {
		t.Fatalf("queued item %+v", rows[0])
	}
	board, err := service.List(ctx, &old.ChannelID, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range board.Jobs {
		if row.ID == result.ID {
			found = true
			if row.SourceKey != old.SourceKey || row.PreviousJobID == nil || *row.PreviousJobID != old.ID {
				t.Fatalf("queued board row %+v", row.CloneJob)
			}
		}
	}
	if !found {
		t.Fatalf("queued restart missing from board: %+v", board.Jobs)
	}
	still, err := store.GetJob(ctx, old.ID)
	if err != nil || still.SupersededAt != nil {
		t.Fatalf("old changed before admission: %+v %v", still, err)
	}
	if _, err = service.Restart(ctx, old.ID); err == nil {
		t.Fatal("duplicate restart queued")
	}
	next, err := service.Start(ctx, rows[0])
	if err != nil {
		t.Fatal(err)
	}
	if next.Generation != old.Generation+1 || next.SourceKey != old.SourceKey {
		t.Fatalf("generation %+v", next)
	}
	previous, err := store.GetJob(ctx, old.ID)
	if err != nil || previous.SupersededAt == nil || previous.Status != domain.CloneCancelled {
		t.Fatalf("old %+v %v", previous, err)
	}
	view, err := service.Get(ctx, next.ID)
	if err != nil || view.PreviousJobID == nil || *view.PreviousJobID != old.ID {
		t.Fatalf("history %+v %v", view, err)
	}
}
