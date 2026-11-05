package suggest

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	open      []Suggestion
	got       []Suggestion
	resolved  []string
	cooldown  *time.Time
	upserted  int
	hotCalled int
}

func (f *fakeStore) Upsert(context.Context, Draft, time.Time) (Suggestion, bool, error) {
	f.upserted++
	return Suggestion{}, false, nil
}
func (f *fakeStore) Get(_ context.Context, id string) (Suggestion, bool, error) {
	for _, x := range f.open {
		if x.ID == id {
			return x, true, nil
		}
	}
	return Suggestion{}, false, nil
}
func (f *fakeStore) Open(context.Context, time.Time) ([]Suggestion, error) {
	return f.open, nil
}
func (f *fakeStore) Resolve(_ context.Context, id, state string, cooldownUntil *time.Time, _ time.Time) (bool, error) {
	f.resolved = append(f.resolved, id+":"+state)
	f.cooldown = cooldownUntil
	return true, nil
}
func (f *fakeStore) ExpireStale(context.Context, time.Time) error { return nil }
func (f *fakeStore) CloseOpenExcept(context.Context, string, []string, time.Time) error {
	return nil
}
func (f *fakeStore) TrimOpen(context.Context, int, time.Time) error { return nil }
func (f *fakeStore) ChannelNames(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (f *fakeStore) HotVideos(context.Context, time.Time) ([]HotVideo, error) {
	f.hotCalled++
	return nil, nil
}
func (f *fakeStore) FailingTemplates(context.Context, time.Time) ([]FailingTemplate, error) {
	return nil, nil
}

type fixedPrefs bool

func (p fixedPrefs) SuggestionsEnabled(context.Context) (bool, error) { return bool(p), nil }

func openSuggestion(id, kind, op string) Suggestion {
	channel := "c1"
	return Suggestion{ID: id, Kind: kind, ChannelID: &channel, State: StateOpen, Primary: Primary{Op: op}}
}

func TestListIsEmptyWhenGlobalSwitchIsOff(t *testing.T) {
	store := &fakeStore{open: []Suggestion{openSuggestion("s1", KindHotVideo, "pick")}}
	svc := &Service{Store: store, Prefs: fixedPrefs(false)}
	list, total, err := svc.List(context.Background(), "", MaxOpen)
	if err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("list=%v total=%d err=%v", list, total, err)
	}
}

func TestListKeepsOnlyLiveKinds(t *testing.T) {
	store := &fakeStore{open: []Suggestion{
		openSuggestion("old", "learn_rule", "add_rule"),
		openSuggestion("hot", KindHotVideo, "pick"),
	}}
	svc := &Service{Store: store, Prefs: fixedPrefs(true)}
	list, total, err := svc.List(context.Background(), "", MaxOpen)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != "hot" {
		t.Fatalf("list=%v total=%d err=%v", list, total, err)
	}
}

func TestRefreshProducesNothingWhenSwitchIsOff(t *testing.T) {
	store := &fakeStore{}
	svc := &Service{Store: store, Prefs: fixedPrefs(false)}
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.upserted != 0 || store.hotCalled != 0 {
		t.Fatalf("producer ran while disabled: upserts=%d hot=%d", store.upserted, store.hotCalled)
	}
}

func TestAcceptRejectsRetiredOperations(t *testing.T) {
	for _, op := range []string{"add_rule", "set_pace", "set_autonomy"} {
		store := &fakeStore{open: []Suggestion{openSuggestion("s1", KindHotVideo, op)}}
		svc := &Service{Store: store, Prefs: fixedPrefs(true), Ops: noOps{}}
		if _, err := svc.Accept(context.Background(), "s1", true); !errors.Is(err, ErrBadOp) {
			t.Fatalf("%s: err=%v", op, err)
		}
		if len(store.resolved) != 0 {
			t.Fatalf("%s resolved retired suggestion", op)
		}
	}
}

func TestDismissCooldownIsSevenDays(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	store := &fakeStore{open: []Suggestion{openSuggestion("s1", KindCleanup, "cleanup")}}
	svc := &Service{Store: store, Prefs: fixedPrefs(true), Now: func() time.Time { return now }}
	if err := svc.Dismiss(context.Background(), "s1"); err != nil {
		t.Fatal(err)
	}
	if len(store.resolved) != 1 || store.resolved[0] != "s1:"+StateDismissed {
		t.Fatalf("resolved=%v", store.resolved)
	}
	if store.cooldown == nil || !store.cooldown.Equal(now.Add(Cooldown)) {
		t.Fatalf("cooldown=%v", store.cooldown)
	}
}

type noOps struct{}

func (noOps) Prioritize(context.Context, string, string) error { return nil }
func (noOps) PlanImages(context.Context, string, string) (*int, error) {
	return nil, nil
}
func (noOps) Pick(context.Context, string, string) error { return nil }
func (noOps) ReclaimMP4(context.Context) (int64, error)  { return 0, nil }
