package web

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

const liveTranscriptLog = "web-live-stream"

type runtimeHistoryHost interface{ RuntimeHistory() store.RuntimeStore }

func restoreStreamHistory(h workerHost, b *broadcaster) error {
	source, ok := h.(runtimeHistoryHost)
	if !ok {
		return nil
	}
	history := source.RuntimeHistory()
	if history == nil {
		return nil
	}
	entries, err := history.LoadTaskLog(liveTranscriptLog)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		b.stream.WriteString(entry.Summary)
	}
	queue, err := history.LoadQueue()
	if err != nil {
		return err
	}
	// Preserve full payloads here: the activity endpoint intentionally truncates
	// diagnostics, but the writing stream needs the original content.
	for _, item := range queue {
		raw, err := json.Marshal(item.Payload)
		if err != nil {
			return err
		}
		var ev host.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			return err
		}
		if ev.ID != "" {
			b.emit(message{kind: "event", data: ev})
		}
	}
	return nil
}

func (s *workerServer) persistStream(text string) bool {
	source, ok := s.h.(runtimeHistoryHost)
	if !ok {
		return true
	}
	if source.RuntimeHistory() == nil {
		return true
	}
	if err := source.RuntimeHistory().AppendTaskLog(liveTranscriptLog, domain.RuntimeTaskLogEntry{Time: time.Now(), Event: "stream", Summary: text}); err != nil {
		slog.Error("Không lưu được luồng viết", "err", err)
		s.b.emit(message{kind: "event", data: host.Event{Time: time.Now(), Category: "ERROR", Level: "error", Summary: "Không lưu được luồng viết", Detail: err.Error()}})
		return false
	}
	return true
}
