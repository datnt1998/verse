package host

import (
	"encoding/json"
	"errors"
	"testing"

	agentcore "github.com/datnt1998/verse/apps/noverse/internal/agent"
)

func TestObserverPolishFailureIsFinalOnly(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		retry, differentChapter bool
		runErr                  error
		wantFailed              bool
		handoff                 bool
	}{
		{name: "retry succeeds", retry: true},
		{name: "step succeeds after recovery"},
		{name: "step ultimately fails", runErr: errors.New("writer failed"), wantFailed: true},
		{name: "other chapter cannot recover", retry: true, differentChapter: true, runErr: errors.New("writer failed"), wantFailed: true},
		{name: "successful retry survives later unrelated failure", retry: true, runErr: errors.New("later failure")},
		{name: "question handoff then retry succeeds", handoff: true, retry: true},
		{name: "question handoff then actual failure", handoff: true, runErr: errors.New("writer failed after handoff"), wantFailed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []Event
			o := testObserver(&events)
			o.dispatchStart("writer", "Gọt giũa", "")
			progress := func(kind agentcore.ProgressPayloadKind, chapter int) {
				args, _ := json.Marshal(map[string]any{"chapter": chapter})
				o.workerProgress(agentcore.ProgressPayload{Kind: kind, Agent: "writer", Tool: "edit_chapter", Args: args, Message: "old_string không khớp"})
			}
			progress(agentcore.ProgressToolStart, 15)
			id := events[len(events)-1].ID
			progress(agentcore.ProgressToolError, 15)
			pending := events[len(events)-1]
			if pending.Failed || pending.Level == "error" || pending.Status == "error" {
				t.Fatalf("recoverable attempt surfaced as failure: %+v", pending)
			}
			if pending.Result != "" {
				t.Fatalf("attempt error leaked into user-facing result: %+v", pending)
			}
			if tc.handoff {
				o.dispatchFinish("writer", &workerQuestion{text: "Cần xác nhận phạm vi sửa"})
				if len(o.polishAttempts["writer"]) != 1 {
					t.Fatal("nonterminal question handoff settled a pending edit")
				}
				o.dispatchStart("writer", "Tiếp tục gọt giũa", "")
			}
			if tc.retry {
				chapter := 15
				if tc.differentChapter {
					chapter = 16
				}
				progress(agentcore.ProgressToolStart, chapter)
				progress(agentcore.ProgressToolEnd, chapter)
			}
			o.dispatchFinish("writer", tc.runErr)
			var final Event
			for _, ev := range events {
				if ev.ID == id {
					final = ev
				}
			}
			if final.Failed != tc.wantFailed || (final.Status == "error") != tc.wantFailed {
				t.Fatalf("final attempt state: %+v, want failed=%v", final, tc.wantFailed)
			}
			if !tc.wantFailed && final.Result != "" {
				t.Fatalf("recovered attempt retained an error result: %+v", final)
			}
			if len(final.Retries) == 0 || final.Retries[0].OK == tc.wantFailed {
				t.Fatalf("attempt detail was not retained: %+v", final)
			}
		})
	}
}
