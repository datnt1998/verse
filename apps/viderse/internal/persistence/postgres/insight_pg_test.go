package postgres_test

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"testing"
	"time"
)

func TestActivityRecapAndQuietHoursPostgres(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	store := postgres.NewActivityStore(pool)
	svc := activity.New(store, nil)
	prefs := activity.DefaultNotificationSettings()
	prefs.DoNotDisturb = activity.QuietHours{Enabled: true, Start: "00:00", End: "00:00"}
	if err := svc.SetNotificationSettings(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := svc.Notify(ctx, "review_ready", "Video chờ duyệt", "/review", "Duyệt ảnh"); err != nil {
			t.Fatal(err)
		}
	}
	notes, err := svc.Notifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].Count != 3 {
		t.Fatalf("notifications: %+v", notes)
	}
	prefs, err = svc.NotificationSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if prefs.SystemAllowed(notes[0].Kind, notes[0].CreatedAt) {
		t.Fatal("quiet hours delivered")
	}
	var channel string
	if err := pool.QueryRow(ctx, `INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'Kênh') RETURNING id::text`).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"review", "review", "attention"} {
		_, err = pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,stage_group) VALUES(gen_random_uuid(),$1,gen_random_uuid()::text,'{"title":"Video"}'::jsonb,$2)`, channel, state)
		if err != nil {
			t.Fatal(err)
		}
	}
	recap, err := store.ReadRecap(ctx, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(recap.NeedsYou) != 3 || recap.Channels[0].Waiting != 3 || recap.Summary != "Trong lúc bạn vắng: 2 video chờ duyệt, 1 cần xử lý." {
		t.Fatalf("recap: %+v", recap)
	}
}
