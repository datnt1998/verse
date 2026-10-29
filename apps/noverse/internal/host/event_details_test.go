package host

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/datnt1998/verse/apps/noverse/internal/agent"
	"github.com/datnt1998/verse/apps/noverse/internal/agent/subagent"
)

func TestActivityToolTruncationAndResult(t *testing.T) {
	var events []Event
	o := newObserver(nil, func(ev Event) { events = append(events, ev) }, func(string) {}, func() {}, nil)
	args := json.RawMessage(`{"chapter":3,"text":"` + strings.Repeat("ế", 500) + `"}`)
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressToolStart, Agent: "writer", Tool: "draft_chapter", Args: args})
	result := json.RawMessage(`{"ok":true,"text":"` + strings.Repeat("đ", 900) + `"}`)
	o.captureToolResult(subagent.RunMeta{Agent: "writer"}, agentcore.Event{Type: agentcore.EventToolExecEnd, Result: result})
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressToolEnd, Agent: "writer", Tool: "draft_chapter"})
	got := events[len(events)-1]
	if got.Tool != "draft_chapter" || got.Chapter != 3 || got.Status != "ok" || !utf8.ValidString(got.Args) || len([]rune(got.Args)) != 401 || len([]rune(got.Result)) != 801 || !strings.HasSuffix(got.Result, "…") {
		t.Fatalf("truncated tool detail: %+v", got)
	}
	if got.FinishedAt.IsZero() || got.Duration <= 0 || got.Failed {
		t.Fatalf("lifecycle: %+v", got)
	}
}
func TestActivityToolRetryCapture(t *testing.T) {
	var events []Event
	o := newObserver(nil, func(ev Event) { events = append(events, ev) }, func(string) {}, func() {}, nil)
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressToolStart, Agent: "writer", Tool: "draft_chapter", Args: json.RawMessage(`{"chapter":3}`)})
	o.toolStarts["writer"].start = time.Now().Add(-2 * time.Second)
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressRetry, Agent: "writer", Message: "Mất kết nối", Attempt: 1, Meta: json.RawMessage(`{"retry_delay_ms":8000}`)})
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressToolEnd, Agent: "writer", Tool: "draft_chapter"})
	got := events[len(events)-1]
	if len(got.Retries) != 1 || got.Retries[0].WaitSec != 8 || got.Retries[0].Error != "Mất kết nối" || got.Retries[0].AtSec < 2 || !got.Retries[0].OK {
		t.Fatalf("retry capture: %+v", got)
	}
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressToolStart, Agent: "writer", Tool: "read_chapter"})
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressRetry, Agent: "writer", Message: "Hết hạn mức", Attempt: 1})
	o.workerProgress(agentcore.ProgressPayload{Kind: agentcore.ProgressToolError, Agent: "writer", Tool: "read_chapter", Message: "Hết hạn mức"})
	got = events[len(events)-1]
	if got.Status != "error" || got.Result != "Hết hạn mức" || len(got.Retries) != 1 || got.Retries[0].OK {
		t.Fatalf("failed retry capture: %+v", got)
	}
}

func TestActivityUserOriginAndPersistence(t *testing.T) {
	h := &Host{events: make(chan Event, 1)}
	h.RecordUserEvent("reader", "next", "/next")
	got := <-h.events
	if got.Username != "reader" || got.Category != "USER" || got.Agent != "user" || got.Summary != "/next" || got.ID == "" || got.Command != "next" || got.Input != "/next" {
		t.Fatalf("user attribution: %+v", got)
	}
	var buf bytes.Buffer
	logEvent(slog.New(slog.NewTextHandler(&buf, nil)), got)
	if text := buf.String(); !strings.Contains(text, "event_data=") || !strings.Contains(text, `\"username\":\"reader\"`) {
		t.Fatalf("persisted attribution: %s", text)
	}
}

// tui.log là nguồn của Nhật ký web: sự kiện ghi xuống phải dùng đường dẫn tương đối thư mục truyện.
func TestLogEventWritesRootRelativePaths(t *testing.T) {
	wd := logRoot()
	ev := Event{Time: time.Now(), Category: "ERROR", Level: "error",
		Summary: "Xuất file thất bại: tệp đã tồn tại: " + filepath.Join(wd, "exports", "Trà Xanh.txt") + " (dùng --overwrite để ghi đè)",
		Retries: []EventRetry{{Error: "open " + filepath.Join(wd, "a.txt") + ": denied"}}}
	var buf bytes.Buffer
	logEvent(slog.New(slog.NewTextHandler(&buf, nil)), ev)
	text := buf.String()
	if strings.Contains(text, wd) {
		t.Fatalf("tui.log lộ đường dẫn tuyệt đối %s: %s", wd, text)
	}
	if !strings.Contains(text, "tệp đã tồn tại: exports/Trà Xanh.txt (dùng") || !strings.Contains(text, "open a.txt: denied") {
		t.Fatalf("thiếu đường dẫn tương đối: %s", text)
	}
}
