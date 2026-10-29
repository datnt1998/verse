package web

import (
	"testing"
	"time"

	"github.com/datnt1998/noverse/internal/host"
)

func TestActivityHistoryOwnChapterAndRecordedArc(t *testing.T) {
	tg, cookie, path := activityFixture(t)
	appendActivityFixture(t, path, host.Event{ID: "arc-task", Time: time.Unix(200, 0), FinishedAt: time.Unix(220, 0), Chapter: 41, Category: "DISPATCH", Agent: "editor", Summary: "editor (Thẩm duyệt cung 2 của tập 1 (chương 33-40): gọi novel_context(chapter=40), save_review dùng scope=arc, chapter=40)"})
	appendActivityFixture(t, path, host.Event{ID: "arc-tool", Time: time.Unix(210, 0), FinishedAt: time.Unix(211, 0), Chapter: 41, Category: "TOOL", Agent: "editor", Tool: "save_review", Args: `{"chapter":40,"contract_notes":"…`, Result: `{"chapter":40,"scope":"arc","saved":true}`})
	appendActivityFixture(t, path, host.Event{ID: "rewrite", Time: time.Unix(230, 0), Chapter: 41, Category: "DISPATCH", Agent: "writer", Summary: "writer (Viết lại chương 39)"})
	got := activityGet(t, tg, cookie, "?chapter=40")
	if len(got.Events) != 2 {
		t.Fatalf("own chapter filter: got %d, want arc task and tool", len(got.Events))
	}
	for _, event := range got.Events {
		if event.Chapter != 40 || event.Scope != "arc" || event.Volume != 1 || event.Arc != 2 || event.ChapterFrom != 33 || event.ChapterTo != 40 {
			t.Errorf("recorded arc context not restored: %+v", event.Event)
		}
	}
	rewrite := activityGet(t, tg, cookie, "?chapter=39")
	if len(rewrite.Events) != 1 || rewrite.Events[0].ID != "rewrite" {
		t.Fatalf("rewrite target: %+v", rewrite.Events)
	}
}
