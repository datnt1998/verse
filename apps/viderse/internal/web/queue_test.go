package web

import (
	"context"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/queue"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestQueueFrontPromotesBatchAheadOfExistingPick(t *testing.T) {
	svc, _ := planFixture(t)
	ctx := context.Background()
	first, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: []string{"https://youtu.be/aaaaaaaaaaa"}, Priority: domain.ClonePick})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: []string{"https://youtu.be/bbbbbbbbbbb"}, Priority: domain.CloneBatch})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || len(second.Items) != 1 {
		t.Fatal("enqueue missing items")
	}
	id := second.Items[0].ID
	view, err := svc.Patch(ctx, domain.QueuePatch{ID: &id, Action: "front"})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 2 || view.Items[0].ID != id || view.Items[0].Priority != domain.ClonePick {
		t.Fatalf("front did not become global priority head: %+v", view.Items)
	}
}
func TestQueuePauseExplainsAllRowsAndPersistsTimestamp(t *testing.T) {
	svc, _ := planFixture(t)
	ctx := context.Background()
	_, err := svc.Enqueue(ctx, domain.CloneEnqueueRequest{ChannelID: "11111111-1111-4111-8111-111111111111", URLs: []string{"https://youtu.be/aaaaaaaaaaa", "https://youtu.be/bbbbbbbbbbb"}})
	if err != nil {
		t.Fatal(err)
	}
	paused := true
	workers := 99
	images := 0
	rescan := 99
	view, err := svc.Patch(ctx, domain.QueuePatch{Paused: &paused, Workers: &workers, ImageWorkers: &images, RescanHours: &rescan})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Paused || view.PausedAt == nil || view.Workers.Total != 6 || view.ImageWorkers != 1 || view.RescanHours != 24 {
		t.Fatalf("rules=%+v", view)
	}
	expected := time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local)
	if !view.PausedAt.Equal(expected) {
		t.Fatalf("pausedAt=%v", view.PausedAt)
	}
	for _, row := range view.Items {
		if row.WaitReason != domain.WaitQueuePaused || row.ETAStart != nil {
			t.Fatalf("paused row=%+v", row)
		}
	}
}

func TestAutoPickRescanDedupAndBaselineReset(t *testing.T) {
	_, pool := planFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local)
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE owner_channels SET auto_pick=true`, nil},
		{`UPDATE owner_channel_sources SET auto_baseline_at=$1`, []any{now.Add(-24 * time.Hour)}},
		{`UPDATE rival_channels SET scanned_at=$1`, []any{now.Add(-time.Minute)}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	scans := 0
	svc := queue.New(queue.Options{Pool: pool, Now: func() time.Time { return now }, Scan: func(context.Context, string) error { scans++; return nil }})
	if err := svc.Discover(ctx); err != nil {
		t.Fatal(err)
	}
	view, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 2 || scans != 0 {
		t.Fatalf("auto items=%d scans=%d", len(view.Items), scans)
	}
	if err = svc.Discover(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != 2 {
		t.Fatalf("duplicate discoveries=%d", len(after.Items))
	}
	var owner pgtype.UUID
	_ = owner.Scan("11111111-1111-4111-8111-111111111111")
	if _, err = gen.New(pool).SetAutoPick(ctx, gen.SetAutoPickParams{ID: owner, AutoPick: false}); err != nil {
		t.Fatal(err)
	}
	links, err := postgres.NewQueueStore(pool).ListSourcesWithChannels(ctx, "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if links[0].AutoBaselineAt != nil {
		t.Fatal("disabling retained stale baseline")
	}
	paused, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range paused.Items {
		if row.WaitReason != domain.WaitPausedChannel {
			t.Fatalf("disabled auto row=%+v", row)
		}
	}
}

func TestQueuePaceCountsOnlyBatchAndResetsAtMachineMidnight(t *testing.T) {
	_, pool := planFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.Local)
	if _, err := pool.Exec(ctx, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,position,priority,state,started_at) VALUES
 ('44444444-4444-4444-8444-444444444444','11111111-1111-4111-8111-111111111111','aaaaaaaaaaa','oldpick',1,'pick','started',$1),
 ('55555555-5555-4555-8555-555555555555','11111111-1111-4111-8111-111111111111','bbbbbbbbbbb','oldbatch',2,'batch','started',$2)`, now, queue.LocalMidnight(now).Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewQueueStore(pool)
	used, err := repo.PaceUsedToday(ctx, queue.LocalMidnight(now))
	if err != nil {
		t.Fatal(err)
	}
	if used["11111111-1111-4111-8111-111111111111"] != 0 {
		t.Fatalf("pick or yesterday consumed today's pace: %v", used)
	}
	if _, err = pool.Exec(ctx, `UPDATE clone_queue SET started_at=$1 WHERE priority='batch'`, queue.LocalMidnight(now)); err != nil {
		t.Fatal(err)
	}
	used, err = repo.PaceUsedToday(ctx, queue.LocalMidnight(now))
	if err != nil {
		t.Fatal(err)
	}
	if used["11111111-1111-4111-8111-111111111111"] != 1 {
		t.Fatalf("batch at midnight missing: %v", used)
	}
}
