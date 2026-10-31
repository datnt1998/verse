package host

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/core/agentcore"
)

func TestToolErrorIsSafeInLiveAndDurableEvents(t *testing.T) {
	var events []Event
	o := testObserver(&events)
	o.store = store.NewStore(t.TempDir())
	o.dispatchStart("writer", "Viết", "")
	o.handleToolUpdate(agentcore.Event{Progress: &agentcore.ProgressPayload{Kind: agentcore.ProgressToolStart, Agent: "writer", Tool: "commit_chapter", Args: json.RawMessage(`{"chapter":4}`)}})
	o.handleToolUpdate(agentcore.Event{Progress: &agentcore.ProgressPayload{Kind: agentcore.ProgressToolError, Agent: "writer", Tool: "commit_chapter", ErrorKind: agentcore.ToolErrorValidation, Message: "token=sk-secret /private/path"}})
	last := events[len(events)-1]
	want := "Thông tin thao tác ở chương 4 chưa hợp lệ. Trợ lý sẽ kiểm tra và thử lại."
	if last.Error != want || strings.Contains(last.Error, "secret") || strings.Contains(last.Error, "/private") {
		t.Fatalf("live error: %+v", last)
	}
	queue, err := o.store.Runtime.LoadQueue()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(queue[len(queue)-1].Payload)
	if err != nil {
		t.Fatal(err)
	}
	var saved Event
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Error != want {
		t.Fatalf("stored error: %+v", saved)
	}
}
