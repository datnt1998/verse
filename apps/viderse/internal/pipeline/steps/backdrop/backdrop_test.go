package backdrop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"github.com/datnt1998/verse/platform/blobstore"
)

var testNow = time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)

type memoryStore struct {
	channels map[string]domain.BackdropChannel
	videos   map[string][]domain.BackdropChannelVideo
	fixed    map[string]domain.BackdropVideo
	marks    []string
}

func newStore() *memoryStore {
	return &memoryStore{channels: map[string]domain.BackdropChannel{}, videos: map[string][]domain.BackdropChannelVideo{}, fixed: map[string]domain.BackdropVideo{}}
}
func (s *memoryStore) seed(id string, count int, duration float64) {
	s.channels[id] = domain.BackdropChannel{ID: id, Platform: PlatformOf(id), Handle: strings.Split(id, ":")[1], Title: new("Kênh " + id)}
	for i := range count {
		s.videos[id] = append(s.videos[id], domain.BackdropChannelVideo{ChannelID: id, VideoID: fmt.Sprintf("%s-v%d", id, i), URL: fmt.Sprintf("https://example.test/%s/v%d", id, i), Kind: "clip", Title: fmt.Sprintf("V%d", i), DurationSec: new(duration)})
	}
}
func (s *memoryStore) Channel(_ context.Context, id string) (domain.BackdropChannel, bool, error) {
	v, ok := s.channels[id]
	return v, ok, nil
}
func (s *memoryStore) ChannelVideos(_ context.Context, id string) ([]domain.BackdropChannelVideo, error) {
	return s.videos[id], nil
}
func (s *memoryStore) ChannelVideo(_ context.Context, id, video string) (domain.BackdropChannelVideo, bool, error) {
	for _, v := range s.videos[id] {
		if v.VideoID == video {
			return v, true, nil
		}
	}
	return domain.BackdropChannelVideo{}, false, nil
}
func (s *memoryStore) Video(_ context.Context, id string) (domain.BackdropVideo, bool, error) {
	v, ok := s.fixed[id]
	return v, ok, nil
}
func (s *memoryStore) MarkChannelVideoUnavailable(_ context.Context, id, video, reason string, at time.Time) error {
	s.marks = append(s.marks, video+":"+reason)
	for i, v := range s.videos[id] {
		if v.VideoID == video {
			s.videos[id][i].UnavailableAt = &at
		}
	}
	return nil
}

type fakeMedia struct{ files map[string]string }

func (f *fakeMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	if file := f.files[path]; file != "" {
		return &media.Handle{Path: file}, nil
	}
	return nil, blobstore.ErrNotFound
}
func (f *fakeMedia) Publish(context.Context, []media.Artifact, func(context.Context) error) error {
	return errors.New("handler must not publish")
}

type harness struct {
	t         *testing.T
	store     *memoryStore
	d         Deps
	s         *pipeline.StepContext
	fetches   []Item
	refreshes map[string]time.Duration
	probes    []string
	paused    map[domain.PlatformID]time.Time
	fetch     func(context.Context, Item) error
	refresh   func(string) error
	probe     func(string, string) (domain.BackdropChannelVideo, error)
	saves     []Checkpoint
	saveErr   error
	pausedJob bool
}

func newHarness(t *testing.T, b *domain.BackdropRecipe, video *float64) *harness {
	t.Helper()
	h := &harness{t: t, store: newStore(), refreshes: map[string]time.Duration{}, paused: map[domain.PlatformID]time.Time{}}
	raw, err := json.Marshal(library.Snapshot{Backdrop: b, Appearance: domain.BackgroundAppearance{Speed: 1}})
	if err != nil {
		t.Fatal(err)
	}
	h.s = &pipeline.StepContext{Job: domain.CloneJob{ID: "test-job", TemplateSnapshot: raw}, StagingDir: t.TempDir(), Media: &fakeMedia{files: map[string]string{}}, Upstream: map[domain.CloneStep]domain.CloneStepOutput{}, IsPaused: func() bool { return h.pausedJob }}
	if video != nil {
		h.s.Upstream[domain.CloneDownload] = domain.CloneStepOutput{Data: map[string]any{"durationSec": *video}}
	}
	h.s.WriteCheckpoint = func(_ context.Context, b json.RawMessage) error {
		if h.saveErr != nil {
			return h.saveErr
		}
		c, err := ParseCheckpoint(b)
		if err == nil {
			h.saves = append(h.saves, c)
		}
		return err
	}
	h.d = Deps{Store: h.store, Now: func() time.Time { return testNow }, PlatformPause: func(_ context.Context, p domain.PlatformID) (*time.Time, *string, error) {
		if at, ok := h.paused[p]; ok {
			return &at, nil, nil
		}
		return nil, nil, nil
	}, RefreshChannel: func(_ context.Context, id string, age time.Duration) error {
		h.refreshes[id] = age
		if h.refresh != nil {
			return h.refresh(id)
		}
		return nil
	}, ProbeVideo: func(_ context.Context, id, video string) (domain.BackdropChannelVideo, error) {
		h.probes = append(h.probes, video)
		if h.probe != nil {
			return h.probe(id, video)
		}
		return domain.BackdropChannelVideo{}, errors.New("unexpected probe")
	}, FetchClip: func(ctx context.Context, s *pipeline.StepContext, item Item) (media.Artifact, error) {
		h.fetches = append(h.fetches, item)
		if h.fetch != nil {
			if err := h.fetch(ctx, item); err != nil {
				return media.Artifact{}, err
			}
		}
		name := fmt.Sprintf("%s-%d", item.VideoID, item.StartSec)
		file := filepath.Join(s.StagingDir, name+".mp4")
		if err := os.WriteFile(file, []byte(name), 0o600); err != nil {
			return media.Artifact{}, err
		}
		return media.Artifact{Path: "backdrop/" + name + ".mp4", File: file}, nil
	}}
	return h
}
func recipe(count *int, sec int, ids ...string) *domain.BackdropRecipe {
	b := &domain.BackdropRecipe{ClipCount: count, ClipSec: sec}
	for _, id := range ids {
		b.Channels = append(b.Channels, domain.BackdropChannelLink{ChannelID: id, Kinds: []string{"clip"}})
	}
	return b
}
func (h *harness) run() (pipeline.StepResult, error) { return New(h.d).Run(context.Background(), h.s) }
func (h *harness) manifest() Manifest {
	h.t.Helper()
	c, err := ParseCheckpoint(h.s.Checkpoint)
	if err != nil {
		h.t.Fatal(err)
	}
	return c.Manifest
}
func (h *harness) seed(m Manifest) {
	h.t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		h.t.Fatal(err)
	}
	h.s.Checkpoint = b
}
func (h *harness) speed(speed float64, fixed bool) {
	h.t.Helper()
	var r library.Snapshot
	if err := json.Unmarshal(h.s.Job.TemplateSnapshot, &r); err != nil {
		h.t.Fatal(err)
	}
	r.Appearance.Speed = speed
	if fixed {
		r.Backdrop.FixedVideoIDs = []string{"fixed"}
		h.store.fixed["fixed"] = domain.BackdropVideo{Segment: domain.BackdropSegment{ToSec: 10}}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		h.t.Fatal(err)
	}
	h.s.Job.TemplateSnapshot = raw
}
func requireReady(t *testing.T, m Manifest, n int) {
	t.Helper()
	if got := len(ReadyItems(m)); got != n {
		t.Fatalf("ready=%d want=%d manifest=%+v", got, n, m)
	}
}

func TestNoChannelsIsStableWithoutNetwork(t *testing.T) {
	h := newHarness(t, nil, nil)
	out, err := h.run()
	if err != nil {
		t.Fatal(err)
	}
	m := h.manifest()
	if len(m.Items) != 0 || len(h.refreshes) != 0 || len(h.fetches) != 0 || out.Data["readyCount"] != 0 {
		t.Fatal(out, m)
	}
	if _, err = h.run(); err != nil {
		t.Fatal(err)
	}
	if h.manifest().Revision != m.Revision {
		t.Fatal("empty manifest changed")
	}
}
func TestRoundRobinDeterministicPicksAndWindows(t *testing.T) {
	h := newHarness(t, recipe(new(4), 30, "twitch:a", "bilibili:42"), nil)
	h.store.seed("twitch:a", 3, 200)
	h.store.seed("bilibili:42", 3, 400)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	m := h.manifest()
	requireReady(t, m, 4)
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, v := range m.Items {
		counts[v.ChannelID]++
		if seen[v.VideoID] || ItemSec(v) != 30 || v.StartSec < 0 || v.EndSec > 400 || v.Source != "random" {
			t.Fatal(v)
		}
		seen[v.VideoID] = true
	}
	if counts["twitch:a"] != 2 || counts["bilibili:42"] != 2 || len(h.probes) != 0 {
		t.Fatal(counts)
	}
	h2 := newHarness(t, recipe(new(4), 30, "twitch:a", "bilibili:42"), nil)
	h2.store = h.store
	h2.d.Store = h.store
	if _, err := h2.run(); err != nil {
		t.Fatal(err)
	}
	for i, v := range m.Items {
		other := h2.manifest().Items[i]
		if v.VideoID != other.VideoID || v.StartSec != other.StartSec {
			t.Fatal("nondeterministic picks")
		}
	}
}
func TestRefreshAgesAndCachedFallback(t *testing.T) {
	b := recipe(new(1), 60, "twitch:vod", "twitch:clip", "bilibili:7", "twitch:mixed")
	b.Channels[0].Kinds = []string{"vod"}
	b.Channels[3].Kinds = []string{"clip", "vod"}
	h := newHarness(t, b, nil)
	for _, link := range b.Channels {
		h.store.seed(link.ChannelID, 1, 300)
	}
	h.refresh = func(string) error { return errors.New("HTTP 500") }
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	want := map[string]time.Duration{"twitch:vod": time.Hour, "twitch:clip": 24 * time.Hour, "bilibili:7": 6 * time.Hour, "twitch:mixed": time.Hour}
	if !reflect.DeepEqual(h.refreshes, want) || len(h.manifest().Warnings) != 4 {
		t.Fatal(h.refreshes, h.manifest().Warnings)
	}
	requireReady(t, h.manifest(), 1)
}
func TestShortSourceExhaustionAndWarnings(t *testing.T) {
	h := newHarness(t, recipe(new(4), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 3, 20)
	out, err := h.run()
	if err != nil || out.Waiting != nil {
		t.Fatal(err, out.Waiting)
	}
	m := h.manifest()
	requireReady(t, m, 3)
	if !reflect.DeepEqual(m.Warnings, []string{"Lấy được 3/4 clip nền"}) {
		t.Fatal(m.Warnings)
	}
	for _, v := range h.fetches {
		if ItemSec(v) != 20 {
			t.Fatal(v)
		}
	}
}
func TestUnavailableRepicksAndMarksCatalogue(t *testing.T) {
	for _, code := range []tools.YtDlpErrorCode{tools.YtDlpUnavailable, tools.YtDlpLoginRequired} {
		t.Run(string(code), func(t *testing.T) {
			h := newHarness(t, recipe(new(3), 60, "twitch:a"), nil)
			h.store.seed("twitch:a", 5, 300)
			broken := ""
			h.fetch = func(_ context.Context, v Item) error {
				if broken == "" {
					broken = v.VideoID
				}
				if v.VideoID == broken {
					return &tools.YtDlpError{Code: code, Message: "gone"}
				}
				return nil
			}
			if _, err := h.run(); err != nil {
				t.Fatal(err)
			}
			requireReady(t, h.manifest(), 3)
			if len(h.store.marks) != 1 || h.store.marks[0] != broken+":"+string(code) {
				t.Fatal(h.store.marks)
			}
			for _, v := range h.manifest().Items {
				if v.VideoID == broken {
					t.Fatal(v)
				}
			}
		})
	}
}
func TestThreeTriesPerSlotAndPartialSuccess(t *testing.T) {
	h := newHarness(t, recipe(new(2), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 8, 300)
	h.fetch = func(context.Context, Item) error {
		if len(h.fetches) <= 3 {
			return errors.New("boom")
		}
		return nil
	}
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	m := h.manifest()
	if len(h.fetches) != 4 || m.Items[0].Status != "failed" || m.Items[1].Status != "ready" || m.Items[0].Error.Message != "boom" {
		t.Fatal(h.fetches, m)
	}
}
func TestPlatformThrottleSkipsOnlyBlockedPlatform(t *testing.T) {
	h := newHarness(t, recipe(new(4), 60, "bilibili:42", "twitch:a"), nil)
	h.store.seed("bilibili:42", 3, 300)
	h.store.seed("twitch:a", 3, 300)
	h.fetch = func(_ context.Context, v Item) error {
		if PlatformOf(v.ChannelID) == domain.PlatformBilibili {
			return &tools.YtDlpError{Code: tools.YtDlpRateLimited, Message: "412"}
		}
		return nil
	}
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	blocked := 0
	for _, v := range h.fetches {
		if PlatformOf(v.ChannelID) == domain.PlatformBilibili {
			blocked++
		}
	}
	if blocked != 1 || len(ReadyItems(h.manifest())) < 2 {
		t.Fatal(blocked, h.manifest())
	}
}
func TestExistingPauseAvoidsNetworkAndRetriesAtEarliestPause(t *testing.T) {
	h := newHarness(t, recipe(new(2), 60, "bilibili:42", "twitch:a"), nil)
	h.store.seed("bilibili:42", 2, 300)
	h.store.seed("twitch:a", 2, 300)
	h.paused[domain.PlatformBilibili] = testNow.Add(45 * time.Minute)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.refreshes["bilibili:42"]; ok {
		t.Fatal("refreshed paused platform")
	}
	requireReady(t, h.manifest(), 2)
	h2 := newHarness(t, recipe(new(2), 60, "bilibili:42"), nil)
	h2.store.seed("bilibili:42", 2, 300)
	h2.paused[domain.PlatformBilibili] = testNow.Add(45 * time.Minute)
	out, err := h2.run()
	if err != nil || out.Waiting == nil || !out.Waiting.RetryAt.Equal(h2.paused[domain.PlatformBilibili]) || out.Waiting.RetryAtSource != domain.RetryAtProvider || len(h2.fetches) != 0 {
		t.Fatal(out, err)
	}
}
func TestFixedFallbackAndEmptyCatalogueRetry(t *testing.T) {
	h := newHarness(t, recipe(new(2), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 0, 300)
	h.speed(1, true)
	out, err := h.run()
	if err != nil || out.Waiting != nil || h.manifest().Fallback != "template-clips" {
		t.Fatal(out, err)
	}
	h2 := newHarness(t, recipe(new(2), 60, "twitch:a"), nil)
	h2.store.seed("twitch:a", 0, 300)
	out, err = h2.run()
	if err != nil || out.Waiting == nil || !out.Waiting.RetryAt.Equal(testNow.Add(30*time.Minute)) {
		t.Fatal(out, err)
	}
	h2.store.seed("twitch:a", 2, 300)
	out, err = h2.run()
	if err != nil || out.Waiting != nil {
		t.Fatal(out, err)
	}
	requireReady(t, h2.manifest(), 2)
}
func TestCrashResumeKeepsSuccessfulAndPendingWindows(t *testing.T) {
	h := newHarness(t, recipe(new(4), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 6, 300)
	ctx, cancel := context.WithCancel(context.Background())
	h.fetch = func(context.Context, Item) error {
		if len(h.fetches) == 3 {
			cancel()
			return errors.New("reset")
		}
		return nil
	}
	_, err := New(h.d).Run(ctx, h.s)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	before := h.manifest()
	if before.Items[0].Status != "ready" || before.Items[1].Status != "ready" || before.Items[2].Status != "pending" {
		t.Fatal(before)
	}
	h.fetch = nil
	h.fetches = nil
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	after := h.manifest()
	requireReady(t, after, 4)
	if len(h.fetches) != 2 || !reflect.DeepEqual(before.Items[:2], after.Items[:2]) {
		t.Fatal(h.fetches)
	}
	for i := 2; i < 4; i++ {
		if before.Items[i].VideoID != after.Items[i].VideoID || before.Items[i].StartSec != after.Items[i].StartSec {
			t.Fatal("pending re-picked")
		}
	}
}
func TestDeletedReadyMediaFetchesSameWindow(t *testing.T) {
	h := newHarness(t, recipe(new(2), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 2, 300)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	c, err := ParseCheckpoint(h.s.Checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.Artifacts[0].File); err != nil {
		t.Fatal(err)
	}
	h.fetches = nil
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	if len(h.fetches) != 1 || h.fetches[0].URL != c.Manifest.Items[0].URL || h.fetches[0].StartSec != c.Manifest.Items[0].StartSec {
		t.Fatal(h.fetches)
	}
}
func TestOwnerFailureIsNotReplacedOrToppedUp(t *testing.T) {
	h := newHarness(t, recipe(new(4), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 4, 300)
	a := testItem("b1", "twitch:a-v0", "owner", "pending")
	b := testItem("b2", "twitch:a-v1", "owner", "pending")
	h.seed(Seal(Manifest{WantedCount: 2, Fallback: "none", Items: []Item{a, b}}))
	h.fetch = func(_ context.Context, v Item) error {
		if v.ID == "b1" {
			return errors.New("sub-only")
		}
		return nil
	}
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	m := h.manifest()
	if len(h.fetches) != 2 || len(m.Items) != 2 || m.WantedCount != 2 || m.Items[0].Status != "failed" {
		t.Fatal(m)
	}
}
func TestFillModesAndSourceSecondGoals(t *testing.T) {
	cases := []struct {
		name                string
		video               *float64
		fixed               bool
		speed               float64
		duration            float64
		pool, count, target int
	}{
		{"known", new(300.0), false, 1, 300, 10, 5, 300}, {"fixed", new(130.0), true, 1, 300, 10, 2, 120}, {"covered", new(8.0), true, 1, 300, 10, 0, 0}, {"short", new(120.0), false, 1, 20, 8, 6, 120}, {"exhausted", new(600.0), false, 1, 20, 3, 3, 600}, {"cap", new(7200.0), false, 1, 300, 8, 40, 7200}, {"slow", new(130.0), true, .5, 300, 12, 1, 55}, {"fast", new(130.0), true, 2, 300, 12, 5, 250}, {"slow-cap", new(7200.0), false, .5, 300, 8, 40, 3600}, {"fast-cap", new(7200.0), false, 2, 300, 8, 40, 14400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, recipe(nil, 60, "twitch:a"), tc.video)
			h.store.seed("twitch:a", tc.pool, tc.duration)
			h.speed(tc.speed, tc.fixed)
			if _, err := h.run(); err != nil {
				t.Fatal(err)
			}
			m := h.manifest()
			requireReady(t, m, tc.count)
			if m.TargetSec == nil || *m.TargetSec != tc.target || m.WantedCount != tc.count || len(m.Warnings) != 0 {
				t.Fatal(m)
			}
			h.fetches = nil
			if _, err := h.run(); err != nil {
				t.Fatal(err)
			}
			if len(h.fetches) != 0 || h.manifest().Revision != m.Revision {
				t.Fatal("rerun changed completed clips")
			}
		})
	}
}
func TestUnknownLengthFallbackFixedCountsAndLegacyMode(t *testing.T) {
	for _, speed := range []float64{.5, 1, 2} {
		t.Run(fmt.Sprint(speed), func(t *testing.T) {
			h := newHarness(t, recipe(nil, 60, "twitch:a"), nil)
			h.store.seed("twitch:a", 12, 300)
			h.speed(speed, false)
			if _, err := h.run(); err != nil {
				t.Fatal(err)
			}
			m := h.manifest()
			if m.TargetSec != nil || m.WantedCount != 10 || len(m.Warnings) != 1 {
				t.Fatal(m)
			}
			requireReady(t, m, 10)
			h2 := newHarness(t, recipe(new(3), 60, "twitch:a"), new(3000.0))
			h2.store.seed("twitch:a", 8, 300)
			if _, err := h2.run(); err != nil {
				t.Fatal(err)
			}
			requireReady(t, h2.manifest(), 3)
			h3 := newHarness(t, recipe(nil, 60, "twitch:a"), new(600.0))
			h3.store.seed("twitch:a", 6, 300)
			h3.speed(speed, false)
			h3.s.Checkpoint = json.RawMessage(`{"version":1,"revision":"legacy","salt":0,"wantedCount":2,"fallback":"none","warnings":[],"items":[]}`)
			if _, err := h3.run(); err != nil {
				t.Fatal(err)
			}
			requireReady(t, h3.manifest(), 2)
			if h3.manifest().TargetSec != nil {
				t.Fatal("legacy reinterpreted")
			}
		})
	}
}
func TestFillFailedSlotTopsUpOnRetry(t *testing.T) {
	h := newHarness(t, recipe(nil, 60, "twitch:a"), new(120.0))
	h.store.seed("twitch:a", 8, 300)
	h.fetch = func(context.Context, Item) error {
		if len(h.fetches) <= 3 {
			return errors.New("boom")
		}
		return nil
	}
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	requireReady(t, h.manifest(), 1)
	h.fetch = nil
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	requireReady(t, h.manifest(), 2)
}
func TestFillPauseRecoveryAndStoredSpeedGoals(t *testing.T) {
	h := newHarness(t, recipe(nil, 60, "bilibili:42"), new(180.0))
	h.store.seed("bilibili:42", 8, 300)
	h.paused[domain.PlatformBilibili] = testNow.Add(45 * time.Minute)
	out, err := h.run()
	if err != nil || out.Waiting == nil {
		t.Fatal(out, err)
	}
	delete(h.paused, domain.PlatformBilibili)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	requireReady(t, h.manifest(), 3)
	h2 := newHarness(t, recipe(nil, 60, "twitch:a"), new(130.0))
	h2.store.seed("twitch:a", 12, 300)
	h2.speed(1, true)
	if _, err := h2.run(); err != nil {
		t.Fatal(err)
	}
	m := h2.manifest()
	for _, speed := range []float64{.5, 2} {
		h2.speed(speed, true)
		h2.fetches = nil
		if _, err := h2.run(); err != nil {
			t.Fatal(err)
		}
		if len(h2.fetches) != 0 || h2.manifest().Revision != m.Revision || *h2.manifest().TargetSec != 120 {
			t.Fatal("stored goals changed")
		}
	}
}
func TestProbeFailuresAndCandidateFilters(t *testing.T) {
	h := newHarness(t, recipe(new(2), 30, "twitch:a"), nil)
	h.store.seed("twitch:a", 4, 300)
	for i := range h.store.videos["twitch:a"] {
		h.store.videos["twitch:a"][i].DurationSec = nil
	}
	h.probe = func(id, v string) (domain.BackdropChannelVideo, error) {
		row, _, _ := h.store.ChannelVideo(context.Background(), id, v)
		if len(h.probes) == 1 {
			return row, errors.New("unreadable")
		}
		row.DurationSec = new(120.0)
		return row, nil
	}
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	requireReady(t, h.manifest(), 2)
	b := recipe(new(4), 30, "twitch:a")
	b.Channels[0].Pinned = []string{"twitch:a-v1"}
	h2 := newHarness(t, b, nil)
	h2.store.seed("twitch:a", 4, 20)
	h2.store.videos["twitch:a"][0].Kind = "vod"
	h2.store.videos["twitch:a"][2].UnavailableAt = &testNow
	h2.store.videos["twitch:a"][3].DurationSec = new(5.0)
	if _, err := h2.run(); err != nil {
		t.Fatal(err)
	}
	m := h2.manifest()
	requireReady(t, m, 1)
	if m.Items[0].VideoID != "twitch:a-v1" {
		t.Fatal(m)
	}
}
func TestPauseAndCheckpointFailuresPreventFurtherFetches(t *testing.T) {
	h := newHarness(t, recipe(new(3), 60, "twitch:a"), nil)
	h.store.seed("twitch:a", 5, 300)
	h.fetch = func(context.Context, Item) error { h.pausedJob = true; return nil }
	_, err := h.run()
	if !errors.Is(err, pipeline.ErrPaused) || len(h.fetches) != 1 {
		t.Fatal(err, h.fetches)
	}
	requireReady(t, h.manifest(), 1)
	h.pausedJob = false
	h.fetch = nil
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	requireReady(t, h.manifest(), 3)
	h2 := newHarness(t, recipe(new(2), 60, "twitch:a"), nil)
	h2.store.seed("twitch:a", 3, 300)
	h2.saveErr = pipeline.ErrFence
	if _, err := h2.run(); !errors.Is(err, pipeline.ErrFence) || len(h2.fetches) != 0 {
		t.Fatal(err, h2.fetches)
	}
}
