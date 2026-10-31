package web

import (
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

type historyFakeHost struct {
	*fakeHost
	runtime store.RuntimeStore
}

func (h *historyFakeHost) RuntimeHistory() store.RuntimeStore { return h.runtime }

func TestStreamRoundBoundaryKeepsPreviousContent(t *testing.T) {
	b := newBroadcaster()
	text := strings.Repeat("Nội dung trước. ", 6000)
	b.emit(message{kind: "delta", data: text})
	b.emit(message{kind: "clear"})
	b.emit(message{kind: "delta", data: "✻ Đọc chương\nchapter: 2"})
	_, replay, ch := b.subscribe()
	defer b.unsubscribe(ch)
	if !strings.HasPrefix(replay, text) || !strings.Contains(replay, host.StreamRoundBoundary) {
		t.Fatalf("round boundary discarded full prior content: got %d bytes, want prefix %d", len(replay), len(text))
	}
}

func TestHostedStreamHistorySurvivesWorkerRestart(t *testing.T) {
	f := provisionHostedGateway(t, "live-history", "story")
	st, ok := f.repo.ResolveStory(t.Context(), nil, f.project, f.story)
	if !ok {
		t.Fatal("story missing")
	}
	facts, err := f.g.openFactStoreInternal(st)
	if err != nil {
		t.Fatal(err)
	}
	first := &historyFakeHost{fakeHost: newFakeHost(), runtime: facts.Runtime}
	b := newBroadcaster()
	s := newWorkerServer(first, "secret", nil, b)
	text := "✻ save_book\ntitle: Truyện còn nguyên" + host.StreamRoundBoundary + "Văn xuôi đầy đủ."
	if !s.persistStream(text) {
		t.Fatal("persist failed")
	}
	ev := host.Event{ID: "persisted-tool", Time: time.Now(), FinishedAt: time.Now(), Category: "TOOL", Tool: "save_book", Agent: "architect", Args: `{"title":"Truyện còn nguyên"}`, Status: "ok"}
	if _, err := facts.Runtime.AppendQueue(domain.RuntimeQueueItem{Payload: ev}); err != nil {
		t.Fatal(err)
	}
	// New scoped facade and new broadcaster: neither process memory nor cwd survive.
	reopened, err := f.g.openFactStoreInternal(st)
	if err != nil {
		t.Fatal(err)
	}
	next := &historyFakeHost{fakeHost: newFakeHost(), runtime: reopened.Runtime}
	restored := newBroadcaster()
	if err := restoreStreamHistory(next, restored); err != nil {
		t.Fatal(err)
	}
	events, transcript, ch := restored.subscribe()
	defer restored.unsubscribe(ch)
	if transcript != text {
		t.Fatalf("lost transcript: %q", transcript)
	}
	if len(events) != 1 || events[0].data.(host.Event).ID != ev.ID {
		t.Fatalf("lost action details: %+v", events)
	}
}
