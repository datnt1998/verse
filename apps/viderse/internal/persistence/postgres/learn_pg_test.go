package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/apps/viderse/internal/learn"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	stepmetadata "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/metadata"
	"github.com/datnt1998/verse/apps/viderse/internal/suggest"
)

type learnPrefs struct{ off []suggest.Off }

func (p *learnPrefs) SuggestionOff(context.Context) ([]suggest.Off, error) { return p.off, nil }
func (p *learnPrefs) DisableSuggestion(_ context.Context, kind, channelID string) error {
	off := suggest.Off{Kind: kind}
	if channelID != "" {
		off.ChannelID = &channelID
	}
	p.off = append(p.off, off)
	return nil
}

// V19 Nghiệm thu: 3 lần sửa bỏ ngoặc → đề xuất; lưu → prompt metadata kế tiếp có quy tắc.
func TestLearnV19AcceptancePostgres(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	var channel string
	if err := pool.QueryRow(ctx, `INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'Midnight Tales') RETURNING id::text`).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	prefs := &learnPrefs{}
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	learnSvc := &learn.Service{Store: postgres.NewLearnStore(pool), Now: func() time.Time { return now }}
	sugg := &suggest.Service{Store: postgres.NewSuggestStore(pool), Prefs: prefs, Learn: learnSvc, Now: func() time.Time { return now }}

	var last *suggest.Suggestion
	for i := range 3 {
		var job string
		if err := pool.QueryRow(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot) VALUES(gen_random_uuid(),$1,gen_random_uuid()::text,'{}'::jsonb) RETURNING id::text`, channel).Scan(&job); err != nil {
			t.Fatal(err)
		}
		now = now.Add(24 * time.Hour)
		cands, err := learnSvc.ObserveMetadataEdit(ctx, learn.MetadataEdit{JobID: job, ChannelID: channel,
			Baseline: learn.Content{Title: fmt.Sprintf("The Lost Station %d (Full Documentary)", i)},
			Current:  learn.Content{Title: fmt.Sprintf("The Lost Station %d", i)}})
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 && len(cands) != 0 {
			t.Fatalf("learn trigger fired at edit %d: %+v", i+1, cands)
		}
		if i == 2 {
			if len(cands) != 1 || cands[0].Count != 3 || cands[0].Key != learn.KeyTrailingParens {
				t.Fatalf("3rd edit must trigger: %+v", cands)
			}
			if last, err = sugg.OfferLearn(ctx, cands); err != nil || last == nil {
				t.Fatalf("offer: %v %v", last, err)
			}
		}
	}
	if last.Kind != suggest.KindLearnRule || last.Primary.Op != "add_rule" || !strings.Contains(last.Banner, "lần thứ 3") || last.Reason.Agent != "director" {
		t.Fatalf("suggestion: %+v", last)
	}
	// Dedupe: offering again does not add a second open card.
	cands, _ := learnSvc.Candidates(ctx)
	if _, err := sugg.OfferLearn(ctx, cands); err != nil {
		t.Fatal(err)
	}
	list, total, err := sugg.List(ctx, "", 5)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("open after dedupe: %d %d %v", total, len(list), err)
	}

	// Save → rule with source label, next metadata prompt has it.
	res, err := sugg.Accept(ctx, last.ID, false)
	if err != nil || res.RuleID == "" || res.Undo == nil || res.Undo.Op != "remove_rule" {
		t.Fatalf("accept: %+v %v", res, err)
	}
	rules, err := learnSvc.List(ctx, learn.RuleFilter{ChannelID: channel})
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules: %+v %v", rules, err)
	}
	if got := rules[0].Label(); !strings.HasPrefix(got, "Học từ 3 lần bạn sửa · ") || rules[0].WireKind() != "edits" || !rules[0].Enabled {
		t.Fatalf("source label %q", got)
	}
	active, err := learnSvc.ChannelRules(ctx, channel)
	if err != nil {
		t.Fatal(err)
	}
	var in []stepmetadata.LearnedRule
	for _, r := range active {
		in = append(in, stepmetadata.LearnedRule{ID: r.ID, Text: r.Text})
	}
	prompt := stepmetadata.BuildMetadataPrompt(stepmetadata.PromptInput{Title: "Source", ChannelName: "Midnight Tales", LearnedRules: in})
	if !strings.Contains(prompt, "Không thêm phần trong ngoặc ở cuối tiêu đề") || !strings.Contains(prompt, rules[0].ID) {
		t.Fatalf("prompt lacks learned rule: %s", prompt)
	}
	// The saved pattern is no longer a candidate.
	if cands, _ := learnSvc.Candidates(ctx); len(cands) != 0 {
		t.Fatalf("candidate again: %+v", cands)
	}
	// Disabled rule is not injected; undo removes it.
	off := false
	if _, err := learnSvc.Patch(ctx, rules[0].ID, &off, nil); err != nil {
		t.Fatal(err)
	}
	if active, _ = learnSvc.ChannelRules(ctx, channel); len(active) != 0 {
		t.Fatalf("disabled rule injected: %+v", active)
	}
	raw, _ := json.Marshal(res.Undo.Params)
	if err := sugg.Undo(ctx, res.Undo.Op, raw); err != nil {
		t.Fatal(err)
	}
	if rules, _ = learnSvc.List(ctx, learn.RuleFilter{ChannelID: channel}); len(rules) != 0 {
		t.Fatalf("undo left rules: %+v", rules)
	}
}

func TestSuggestionsDismissCooldownLimitPostgres(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	var channel string
	if err := pool.QueryRow(ctx, `INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),'Midnight Tales') RETURNING id::text`).Scan(&channel); err != nil {
		t.Fatal(err)
	}
	prefs := &learnPrefs{}
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	store := postgres.NewSuggestStore(pool)
	sugg := &suggest.Service{Store: store, Prefs: prefs, Now: func() time.Time { return now }}
	draft := func(obj string, value int) suggest.Draft {
		return suggest.Draft{Kind: suggest.KindPace, Agent: "scout", ChannelID: channel, Object: obj, Title: "t " + obj, Evidence: "e",
			Reason: insight.Reason{Agent: "scout", Text: "r"}, Primary: suggest.Primary{Label: "x", Op: "set_pace", Params: json.RawMessage(`{}`)}, Value: value, TTL: 7 * 24 * time.Hour}
	}
	for i := range 8 {
		if _, _, err := store.Upsert(ctx, draft(fmt.Sprint("o", i), i), now); err != nil {
			t.Fatal(err)
		}
	}
	// Re-upsert of an existing key never duplicates.
	if _, _, err := store.Upsert(ctx, draft("o3", 3), now); err != nil {
		t.Fatal(err)
	}
	list, total, err := sugg.List(ctx, "", 5)
	if err != nil || len(list) != 5 || total != 5 || list[0].Title != "t o7" {
		t.Fatalf("open ≤5 ordered by value: %d/%d %v %v", len(list), total, titles(list), err)
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

	// Dismiss → hidden, upsert inside 7 days stays dismissed, after 7 days reopens.
	top := list[0]
	if err := sugg.Dismiss(ctx, top.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Upsert(ctx, draft("o7", 7), now.Add(6*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _, _ := store.Get(ctx, top.ID)
	if got.State != suggest.StateDismissed {
		t.Fatalf("reopened inside cooldown: %s", got.State)
	}
	if _, _, err := store.Upsert(ctx, draft("o7", 7), now.Add(7*24*time.Hour+time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _, _ = store.Get(ctx, top.ID)
	if got.State != suggest.StateOpen {
		t.Fatalf("not reopened after 7 days: %s", got.State)
	}
	// forever disables the kind for that channel.
	if err := sugg.Dismiss(ctx, top.ID, true); err != nil {
		t.Fatal(err)
	}
	if len(prefs.off) != 1 || prefs.off[0].Kind != suggest.KindPace || prefs.off[0].ChannelID == nil || *prefs.off[0].ChannelID != channel {
		t.Fatalf("forever: %+v", prefs.off)
	}
	if list, _, _ = sugg.List(ctx, "", 5); len(list) != 0 {
		t.Fatalf("disabled kind still listed: %v", titles(list))
	}
}

func titles(l []suggest.Suggestion) []string {
	out := make([]string, len(l))
	for i, s := range l {
		out[i] = s.Title
	}
	return out
}
