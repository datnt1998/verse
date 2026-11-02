//go:build !integration

package web

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
)

// staticComposer returns one fixed segment immediately (unit boundary only).
type staticComposer struct{ result audio.ComposeResult }

func (c staticComposer) Compose(context.Context, audio.ComposeInput) (audio.ComposeResult, error) {
	return c.result, nil
}

// gateComposer blocks inside the model call so a test can make a user edit
// while the provider is "thinking".
type gateComposer struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
	result  audio.ComposeResult
}

func newGateComposer(text string) *gateComposer {
	return &gateComposer{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		result: audio.ComposeResult{Model: "test", Segments: []audio.Segment{
			{ID: "s0001", Speaker: audio.NarratorID, Text: text},
		}},
	}
}

func (c *gateComposer) Compose(ctx context.Context, _ audio.ComposeInput) (audio.ComposeResult, error) {
	c.calls.Add(1)
	select {
	case c.started <- struct{}{}:
	default:
	}
	select {
	case <-c.release:
		return c.result, nil
	case <-ctx.Done():
		return audio.ComposeResult{}, ctx.Err()
	}
}

func newPGComposeFixture(t *testing.T) (*audio.PGService, *postgres.Narrative, postgres.ScopeRef) {
	t.Helper()
	ctx := context.Background()
	pool, nar, scope := provisionWebStoryRoles(t, "story-proj", "novel-b")
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		pgComposeChapterText, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept chapter: %v", err)
	}
	if err := nar.ProgressSave(ctx, scope, &domain.Progress{
		Phase: domain.PhaseWriting, CurrentChapter: 2, CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	jobs := postgres.NewJobs(pool)
	audioRepo := postgres.NewAudio(pool)
	cache := postgres.NewExportCache(pool)
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("local blob store: %v", err)
	}
	tg := newTestGateway(t)
	st := library.Story{Project: "story-proj", Story: "novel-b", ID: "story-proj/novel-b"}
	tg.g.SetFactStore(func(_ library.Story, _ *identity.User) (*store.Store, error) {
		return bootstrap.OpenStoryStoreInternal(ctx, pool, st.Project, st.Story)
	})
	tg.g.SetAudioStore(&PGAudioDeps{Jobs: jobs, Audio: audioRepo, Cache: cache, Nar: nar, Blobs: blobs})
	svc, err := tg.g.pgAudioService(st)
	if err != nil {
		t.Fatalf("pg audio service: %v", err)
	}
	t.Cleanup(tg.g.closePGAudioServices)
	return svc, nar, scope
}

func subscribeCompose(t *testing.T, svc *audio.PGService) <-chan audio.Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return svc.Hub().Subscribe(ctx)
}

func waitCompose(t *testing.T, events <-chan audio.Event, chapter int) audio.Event {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type != audio.EventCompose || ev.Data["chapter"] != chapter {
				continue
			}
			if ev.Data["state"] == "done" || ev.Data["state"] == "error" {
				return ev
			}
		case <-timeout:
			t.Fatal("compose event timeout")
		}
	}
}

func TestPGComposeInvalidBatchDoesNotBlockValidChapter(t *testing.T) {
	svc, _, _ := newPGComposeFixture(t)
	events := subscribeCompose(t, svc)
	source := []audio.SourceChapter{{Chapter: 1, Text: pgComposeChapterText, Completed: true}}
	composer := staticComposer{result: audio.ComposeResult{Model: "test", Segments: []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: "Bản đọc sau yêu cầu không hợp lệ."},
	}}}
	if err := svc.Compose(source, 1, []int{1, 999}, composer, audio.CharacterRef{}); err == nil {
		t.Fatal("batch with a chapter outside the volume must be rejected")
	}
	if err := svc.Compose(source, 1, []int{1}, composer, audio.CharacterRef{}); err != nil {
		t.Fatalf("valid chapter remained blocked after rejected batch: %v", err)
	}
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "done" {
		t.Fatalf("valid compose state = %v", ev.Data)
	}
	script, err := svc.Script(context.Background(), 1, 1)
	if err != nil || script == nil || len(script.Segments) != 1 || script.Segments[0].Text != composer.result.Segments[0].Text {
		t.Fatalf("valid chapter did not receive its script: %+v %v", script, err)
	}
}

func TestPGComposeClaimsChapterExactlyOnce(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newPGComposeFixture(t)
	events := subscribeCompose(t, svc)
	source := []audio.SourceChapter{{Chapter: 1, Text: pgComposeChapterText, Completed: true}}
	gate := newGateComposer("Trời đã tối.")

	if err := svc.Compose(source, 1, []int{1}, gate, audio.CharacterRef{}); err != nil {
		t.Fatalf("first compose: %v", err)
	}
	// The slot is claimed synchronously, before any goroutine: the duplicate
	// must be refused instead of launching a second provider call.
	if err := svc.Compose(source, 1, []int{1}, gate, audio.CharacterRef{}); err == nil {
		t.Fatal("duplicate compose must conflict, not start a second model call")
	}
	close(gate.release)
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "done" {
		t.Fatalf("compose state = %v", ev.Data)
	}
	if got := gate.calls.Load(); got != 1 {
		t.Fatalf("model calls = %d, want 1", got)
	}
	if _, err := svc.Script(ctx, 1, 1); err != nil {
		t.Fatalf("script after compose: %v", err)
	}
}

func TestPGComposeDoesNotOverwriteUserEdit(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newPGComposeFixture(t)
	source := []audio.SourceChapter{{Chapter: 1, Text: pgComposeChapterText, Completed: true}}

	initial := staticComposer{result: audio.ComposeResult{Model: "test", Segments: []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: "Trời đã tối."},
	}}}
	events := subscribeCompose(t, svc)
	if err := svc.Compose(source, 1, []int{1}, initial, audio.CharacterRef{}); err != nil {
		t.Fatalf("initial compose: %v", err)
	}
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "done" {
		t.Fatalf("initial compose state = %v", ev.Data)
	}
	version := svc.ScriptVersion(1, 1)
	if version == "" {
		t.Fatal("script version token missing")
	}

	events = subscribeCompose(t, svc)
	gate := newGateComposer("Bản soạn lại.")
	if err := svc.Compose(source, 1, []int{1}, gate, audio.CharacterRef{}); err != nil {
		t.Fatalf("overwrite compose: %v", err)
	}
	<-gate.started // the row snapshot is already taken; edit while the model runs
	if _, err := svc.SaveScript(ctx, 1, 1, version, audio.ScriptEdit{Segments: []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: "Người dùng sửa."},
	}}); err != nil {
		t.Fatalf("user edit: %v", err)
	}
	close(gate.release)
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "error" {
		t.Fatalf("compose should fail CAS, got %v", ev.Data)
	}
	script, err := svc.Script(ctx, 1, 1)
	if err != nil || script == nil || len(script.Segments) != 1 {
		t.Fatalf("script after conflict: %+v %v", script, err)
	}
	if got := script.Segments[0].Text; got != "Người dùng sửa." {
		t.Fatalf("compose overwrote the user edit: %q", got)
	}
}

func TestPGComposeRejectsSourceChangedDuringModelCall(t *testing.T) {
	ctx := context.Background()
	svc, nar, scope := newPGComposeFixture(t)
	events := subscribeCompose(t, svc)
	gate := newGateComposer("Bản đọc của nội dung cũ.")
	if err := svc.Compose([]audio.SourceChapter{{Chapter: 1, Text: pgComposeChapterText, Completed: true}}, 1, []int{1}, gate, audio.CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	<-gate.started
	const revised = "Sáng hôm sau, nàng mở cửa bước ra khu vườn."
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		revised, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatal(err)
	}
	close(gate.release)
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "error" {
		t.Fatalf("stale compose was published: %v", ev.Data)
	}
	if script, err := svc.Script(ctx, 1, 1); err != nil || script != nil {
		t.Fatalf("old text was bound to the new revision: %+v %v", script, err)
	}
	events = subscribeCompose(t, svc)
	composer := staticComposer{result: audio.ComposeResult{Model: "test", Segments: []audio.Segment{{ID: "s0001", Speaker: audio.NarratorID, Text: revised}}}}
	if err := svc.Compose([]audio.SourceChapter{{Chapter: 1, Text: revised, Completed: true}}, 1, []int{1}, composer, audio.CharacterRef{}); err != nil {
		t.Fatal(err)
	}
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "done" {
		t.Fatalf("fresh retry did not complete: %v", ev.Data)
	}
	script, err := svc.Script(ctx, 1, 1)
	if err != nil || script == nil || script.SourceHash != audio.SourceHash(revised) || script.Segments[0].Text != revised {
		t.Fatalf("fresh source not composed: %+v %v", script, err)
	}
}

func TestPGRefreshRecomposeKeepsLockedScript(t *testing.T) {
	ctx := context.Background()
	svc, nar, scope := newPGComposeFixture(t)
	initial := staticComposer{result: audio.ComposeResult{Model: "test", Segments: []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: "Trời đã tối."},
	}}}
	events := subscribeCompose(t, svc)
	if err := svc.Compose([]audio.SourceChapter{{Chapter: 1, Text: pgComposeChapterText, Completed: true}},
		1, []int{1}, initial, audio.CharacterRef{}); err != nil {
		t.Fatalf("compose: %v", err)
	}
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "done" {
		t.Fatalf("compose state = %v", ev.Data)
	}
	if _, err := svc.Lock(ctx, 1, 1, audio.SourceHash(pgComposeChapterText)); err != nil {
		t.Fatalf("lock: %v", err)
	}

	const revised = "Trời đã tối. Nàng khép cửa sổ."
	if _, err := nar.AcceptRecord(ctx, scope, 1, domain.ChapterOriginGenerated,
		revised, domain.ChapterFacts{}, domain.StyleDelta{},
		postgres.ChapterAuthor{Kind: "agent", AgentName: "test"}); err != nil {
		t.Fatalf("accept revision: %v", err)
	}
	events = subscribeCompose(t, svc)
	refreshed := staticComposer{result: audio.ComposeResult{Model: "test", Segments: []audio.Segment{
		{ID: "s0001", Speaker: audio.NarratorID, Text: revised},
	}}}
	if err := svc.Refresh([]audio.SourceChapter{{Chapter: 1, Text: revised, Completed: true}},
		1, 1, refreshed, audio.CharacterRef{}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if ev := waitCompose(t, events, 1); ev.Data["state"] != "done" {
		t.Fatalf("refresh state = %v", ev.Data)
	}
	script, err := svc.Script(ctx, 1, 1)
	if err != nil || script == nil {
		t.Fatalf("script after refresh: %v", err)
	}
	if script.Status != audio.StatusLocked {
		t.Fatalf("refresh lost lock: %q", script.Status)
	}
	if script.SourceHash != audio.SourceHash(revised) {
		t.Fatalf("source hash = %q, want revised", script.SourceHash)
	}
	if len(script.Segments) == 0 || script.Segments[0].Text != revised {
		t.Fatalf("segments after refresh: %+v", script.Segments)
	}
}
