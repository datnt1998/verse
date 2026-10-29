package host

import (
	"encoding/json"
	"testing"
)

func TestActivityEventOwnChapterAndArcSpan(t *testing.T) {
	h := &Host{events: make(chan Event, 1)}
	for _, tc := range []struct {
		summary, args string
		chapter       int
		arc           bool
	}{
		{summary: "writer (Viết lại chương 40)", chapter: 40},
		{summary: "editor (Thẩm duyệt cung 2 của tập 1 (chương 33-40): gọi novel_context(chapter=40), save_review dùng scope=arc, chapter=40)", chapter: 40, arc: true},
		{summary: "editor (Thẩm duyệt cấp cung cho tập 1 cung 2 (chương 33-40): gọi novel_context(chapter=40), save_review dùng scope=arc, chapter=40)", chapter: 40, arc: true},
		{summary: "read_chapter(chương 39)", args: `{"chapter":39}`, chapter: 39},
	} {
		h.emitEvent(Event{Category: "DISPATCH", Summary: tc.summary, Args: tc.args, Chapter: 41})
		ev := <-h.events
		if ev.Chapter != tc.chapter {
			t.Errorf("own chapter: got %d, want %d", ev.Chapter, tc.chapter)
		}
		if tc.arc {
			raw, _ := json.Marshal(ev)
			var data map[string]any
			_ = json.Unmarshal(raw, &data)
			for key, want := range map[string]any{"scope": "arc", "arc": float64(2), "volume": float64(1), "chapterFrom": float64(33), "chapterTo": float64(40)} {
				if data[key] != want {
					t.Errorf("arc %s: got %v, want %v", key, data[key], want)
				}
			}
		}
	}
}

func TestActivityOverviewHasNoChapter(t *testing.T) {
	h := &Host{events: make(chan Event, 1)}
	for _, args := range []string{``, `{}`, `{"chapter":0}`, `{"chapter":-1}`} {
		h.emitEvent(Event{Category: "TOOL", Tool: "novel_context", Args: args, Chapter: 41})
		if got := (<-h.events).Chapter; got != 0 {
			t.Errorf("overview chapter with %s: got %d, want 0", args, got)
		}
	}
}
