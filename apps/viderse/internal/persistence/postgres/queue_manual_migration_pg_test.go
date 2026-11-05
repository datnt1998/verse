package postgres_test

import (
	"context"
	"testing"
	"time"
)

func TestManualQueueUpgradePreservesOrderHistoryAndAutoCadence(t *testing.T) {
	pool, provider := provisionLegacyChannelSchema(t)
	ctx := context.Background()
	if _, err := provider.UpTo(ctx, 40); err != nil {
		t.Fatal(err)
	}
	seedCloneChannel(t, pool)
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, row := range []struct {
		key, priority, origin, state string
		position                     float64
		removed                      bool
	}{
		{"manual-pick", "pick", "manual", "queued", 20, false},
		{"manual-bulk", "batch", "bulk", "queued", 1, false},
		{"undo-bulk", "batch", "bulk", "queued", 2, true},
		{"auto", "batch", "auto", "queued", 5, false},
		{"started", "batch", "bulk", "started", 7, false},
	} {
		execChannelSQL(t, pool, `INSERT INTO clone_queue(id,channel_id,source_video_id,source_key,priority,origin,state,position,created_at,removed_at) VALUES(md5($1)::uuid,'11111111-1111-4111-8111-111111111111',$1,$1,$2,$3,$4,$5,$6,CASE WHEN $7 THEN $6::timestamptz ELSE NULL END)`, row.key, row.priority, row.origin, row.state, row.position, created, row.removed)
	}
	if _, err := provider.UpTo(ctx, 41); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		key, priority, state string
		position             float64
	}{
		{"manual-pick", "pick", "queued", 1},
		{"manual-bulk", "pick", "queued", 2},
		{"undo-bulk", "pick", "queued", 3},
		{"auto", "batch", "queued", 5},
		{"started", "batch", "started", 7},
	} {
		var priority, state string
		var position float64
		var actualCreated time.Time
		if err := pool.QueryRow(ctx, `SELECT priority,state,position,created_at FROM clone_queue WHERE source_key=$1`, want.key).Scan(&priority, &state, &position, &actualCreated); err != nil {
			t.Fatal(err)
		}
		if priority != want.priority || state != want.state || position != want.position || !actualCreated.Equal(created) {
			t.Fatalf("%s changed incorrectly: priority=%s state=%s position=%v created=%s", want.key, priority, state, position, actualCreated)
		}
	}
}
