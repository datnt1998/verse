package postgres_test

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"testing"
)

func TestNotificationCursorPagesWithoutGaps(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	_, err := pool.Exec(ctx, `INSERT INTO notifications(id,kind,text,created_at) SELECT gen_random_uuid(),'action_result',n::text,now() FROM generate_series(1,105) n`)
	if err != nil {
		t.Fatal(err)
	}
	svc := activity.New(postgres.NewActivityStore(pool), nil)
	first, cursor, err := svc.NotificationPage(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 100 || cursor == "" {
		t.Fatalf("first=%d cursor=%q", len(first), cursor)
	}
	second, next, err := svc.NotificationPage(ctx, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 5 || next != "" {
		t.Fatalf("second=%d next=%q", len(second), next)
	}
	seen := map[string]bool{}
	for _, n := range append(first, second...) {
		if seen[n.ID] {
			t.Fatalf("duplicate %s", n.ID)
		}
		seen[n.ID] = true
	}
	if _, _, err = svc.NotificationPage(ctx, "not-a-cursor"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}
