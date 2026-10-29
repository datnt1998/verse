package web

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/host"
)

func TestWorkerDoesNotBroadcastInternalJudgments(t *testing.T) {
	b := newBroadcaster()
	_, _, ch := b.subscribe()
	defer b.unsubscribe(ch)
	for _, ev := range []host.Event{
		{ID: "internal", Kind: "internal", Category: "SYSTEM", Agent: "arbiter", Summary: "private reasoning"},
		{ID: "legacy", Category: "SYSTEM", Summary: "Phán định: Người dùng hỏi khả năng xử lý lỗi"},
		{ID: "old", Category: "SYSTEM", Summary: "裁定: private reasoning"},
	} {
		b.emit(message{kind: "event", data: ev})
	}
	b.emit(message{kind: "event", data: host.Event{ID: "answer", Category: "SYSTEM", Summary: "Em sẽ kiểm tra."}})
	replay, _, _ := b.subscribe()
	if len(replay) != 1 || replay[0].data.(host.Event).ID != "answer" {
		t.Errorf("internal judgment reached SSE replay: %+v", replay)
	}
	if len(ch) != 1 {
		t.Errorf("internal judgment reached live SSE: %d frames", len(ch))
	}
}

func TestActivityHistoryHidesLoggedInternalJudgments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tui.log")
	for _, ev := range []host.Event{
		{ID: "internal", Time: time.Unix(1, 0), Kind: "internal", Category: "SYSTEM", Summary: "private reasoning"},
		{ID: "legacy", Time: time.Unix(2, 0), Category: "SYSTEM", Summary: "Phán định: Người dùng hỏi khả năng xử lý lỗi"},
		{ID: "old", Time: time.Unix(3, 0), Category: "SYSTEM", Summary: "裁定: private reasoning"},
		{ID: "answer", Time: time.Unix(4, 0), Category: "SYSTEM", Summary: "Em sẽ kiểm tra."},
		{ID: "user", Time: time.Unix(5, 0), Category: "USER", Summary: "Phán định: đây là lời tôi trích"},
	} {
		appendActivityFixture(t, path, ev)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	events, err := readActivity(path, info.Size(), filepath.Dir(path), filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ID != "user" || events[1].ID != "answer" {
		t.Fatalf("logged judgments reached chat history: %+v", events)
	}
}
