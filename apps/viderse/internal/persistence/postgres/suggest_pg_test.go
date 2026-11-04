package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
)

type alwaysOn struct{}

func (alwaysOn) SuggestionsEnabled(context.Context) (bool, error) { return true, nil }

func suggestionTitles(list []suggest.Suggestion) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.Title
	}
	return out
}

func TestSuggestionsOpenLimitAndDismissCooldownPostgres(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	var channel string
	if err := pool.QueryRow(ctx, `INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'Midnight Tales') RETURNING id::text`).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	store := postgres.NewSuggestStore(pool)
	svc := &suggest.Service{Store: store, Prefs: alwaysOn{}, Now: func() time.Time { return now }}
	draft := func(obj string, value int) suggest.Draft {
		return suggest.Draft{Kind: suggest.KindHotVideo, Agent: "scout", ChannelID: channel, Object: obj, Title: "t " + obj, Evidence: "e",
			Reason: insight.Reason{Agent: "scout", Text: "r"}, Primary: suggest.Primary{Label: "x", Op: "prioritize", Params: json.RawMessage(`{}`)}, Value: value, TTL: 7 * 24 * time.Hour}
	}
	for i := range 8 {
		if _, _, err := store.Upsert(ctx, draft(fmt.Sprint("o", i), i), now); err != nil {
			t.Fatal(err)
		}
	}
	// Upsert lại cùng khoá không nhân đôi hàng.
	if _, _, err := store.Upsert(ctx, draft("o3", 3), now); err != nil {
		t.Fatal(err)
	}
	list, total, err := svc.List(ctx, "", suggest.MaxOpen)
	if err != nil || len(list) != 5 || total != 5 || list[0].Title != "t o7" {
		t.Fatalf("open ≤5 giá trị cao trước: %d/%d %v %v", len(list), total, suggestionTitles(list), err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM suggestions`).Scan(&count); err != nil || count != 8 {
		t.Fatalf("rows %d %v", count, err)
	}
	if err := store.TrimOpen(ctx, suggest.MaxOpen, now); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM suggestions WHERE state='open'`).Scan(&count); err != nil || count != 5 {
		t.Fatalf("trim open %d %v", count, err)
	}

	// Bỏ qua → ẩn; upsert trong 7 ngày vẫn dismissed, quá 7 ngày mở lại.
	top := list[0]
	if err := svc.Dismiss(ctx, top.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Upsert(ctx, draft("o7", 7), now.Add(6*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _, _ := store.Get(ctx, top.ID)
	if got.State != suggest.StateDismissed {
		t.Fatalf("mở lại trong cooldown: %s", got.State)
	}
	if _, _, err := store.Upsert(ctx, draft("o7", 7), now.Add(7*24*time.Hour+time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _, _ = store.Get(ctx, top.ID)
	if got.State != suggest.StateOpen {
		t.Fatalf("không mở lại sau 7 ngày: %s", got.State)
	}
}
