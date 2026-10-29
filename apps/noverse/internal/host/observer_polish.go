package host

import (
	"fmt"
	"time"
)

// A rejected edit is feedback to the writer, not yet a failed polishing step.
// Keep its diagnostics under the same event ID and settle it only when a
// replacement succeeds or the dispatch has an authoritative final outcome.
func (o *observer) finishPolishAttempts(agent string, chapter int, runErr error) {
	attempts := o.polishAttempts[agent]
	if len(attempts) == 0 {
		return
	}
	remaining := attempts[:0]
	for _, call := range attempts {
		if chapter != 0 && chapter != call.chapter {
			remaining = append(remaining, call)
			continue
		}
		ev := Event{ID: call.id, Time: call.start, FinishedAt: time.Now(), Category: "TOOL", Agent: agent, Summary: call.summary, Detail: fmt.Sprintf("%s lỗi: %s", call.tool, call.result), Level: "info", Depth: call.depth, Duration: time.Since(call.start)}
		if runErr != nil && !o.aborting.Load() {
			ev.Failed = true
			ev.Level = "error"
			ev.Kind = errorKind(nil, call.result)
			ev.Summary = fmt.Sprintf("%s lỗi: %s", call.summary, truncate(call.result, 100))
		}
		call.fillToolEvent(&ev, !ev.Failed)
		if !ev.Failed {
			ev.Result = ""
		}
		o.emitEv(ev)
		o.persistEvent(ev)
	}
	if len(remaining) == 0 {
		delete(o.polishAttempts, agent)
	} else {
		o.polishAttempts[agent] = remaining
	}
}
