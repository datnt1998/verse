package web

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

func TestHostedActivityUsesDurableRuntimeQueue(t *testing.T) {
	f := provisionHostedGateway(t, "activity-project", "story")
	tg := &testGateway{t: t, g: f.g}
	owner := tg.login("hosted-owner", "mat-khau-12345")
	st, ok := f.repo.ResolveStory(context.Background(), nil, f.project, f.story)
	if !ok {
		t.Fatal("resolve story")
	}
	facts, err := f.g.factStore(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "/api/projects/" + f.project + "/stories/" + f.story + "/events?limit=1"
	if empty := tg.do("GET", endpoint, owner, ""); empty.Code != 200 || !strings.Contains(empty.Body.String(), `"events":[]`) {
		t.Fatalf("fresh story history: %d %s", empty.Code, empty.Body.String())
	}
	now := time.Now().UTC()
	for i := range 3 {
		ev := host.Event{ID: fmt.Sprintf("event-%d", i), Time: now.Add(time.Duration(i) * time.Second), Agent: "writer", Category: "SYSTEM", Summary: fmt.Sprintf("Sự kiện %d", i), Chapter: 1}
		if i == 2 {
			ev.Level = "error"
			ev.Failed = true
		}
		if _, err := facts.Runtime.AppendQueue(domain.RuntimeQueueItem{Time: ev.Time, Agent: ev.Agent, Category: ev.Category, Summary: ev.Summary, Payload: ev}); err != nil {
			t.Fatal(err)
		}
	}
	first := tg.do("GET", endpoint, owner, "")
	if first.Code != 200 {
		t.Fatalf("history: %d %s", first.Code, first.Body.String())
	}
	body := decodeResp(t, first)
	events := body["events"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["summary"] != "Sự kiện 2" {
		t.Fatalf("durable order/payload: %v", body)
	}
	if body["counts"].(map[string]any)["total"] != float64(3) {
		t.Fatalf("counts: %v", body)
	}
	cursor, ok := body["nextCursor"].(string)
	if !ok {
		t.Fatalf("missing cursor: %v", body)
	}
	// Appends must not change the cursor's snapshot.
	ev := host.Event{ID: "new", Time: now.Add(time.Hour), Summary: "new append", Category: "SYSTEM"}
	if _, err := facts.Runtime.AppendQueue(domain.RuntimeQueueItem{Time: ev.Time, Payload: ev}); err != nil {
		t.Fatal(err)
	}
	next := tg.do("GET", endpoint+"&before="+cursor, owner, "")
	if next.Code != 200 || !strings.Contains(next.Body.String(), "Sự kiện 1") || strings.Contains(next.Body.String(), "new append") {
		t.Fatalf("snapshot page: %d %s", next.Code, next.Body.String())
	}
	failed := tg.do("GET", endpoint+"&level=error", owner, "")
	if failed.Code != 200 || !strings.Contains(failed.Body.String(), "Sự kiện 2") {
		t.Fatalf("error filter: %d %s", failed.Code, failed.Body.String())
	}
	if got := tg.do("GET", endpoint, tg.login("hosted-outsider", "mat-khau-12345"), ""); got.Code != 403 {
		t.Fatalf("nonmember history = %d", got.Code)
	}
}
