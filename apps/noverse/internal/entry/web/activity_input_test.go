package web

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
)

type userAuditHost struct {
	*fakeHost
	audit []host.Event
}

func (h *userAuditHost) RecordUserEvent(username, command, text string) {
	h.audit = append(h.audit, host.Event{Username: username, Command: command, Input: text})
}

func TestActivityInputStructuredAudit(t *testing.T) {
	h := &userAuditHost{fakeHost: newFakeHost()}
	s := &workerServer{h: h}
	for _, tc := range []struct{ route, body, command, text string }{
		{"pause", `{}`, "pause", ""},
		{"abort", `{}`, "abort", ""},
		{"command", `{"line":"/export exports/truyen.epub"}`, "export", "/export exports/truyen.epub"},
		{"cocreate/message", `{"message":"Đổi hướng hồi ba"}`, "cocreate/message", "Đổi hướng hồi ba"},
	} {
		r := httptest.NewRequest("POST", "/api/"+tc.route, strings.NewReader(tc.body))
		r.Header.Set("X-Ainovel-Username", "reader")
		s.recordActivityInput(r, tc.route)
		got := h.audit[len(h.audit)-1]
		if got.Username != "reader" || got.Command != tc.command || got.Input != tc.text {
			t.Fatalf("%s: audit %+v", tc.route, got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != tc.body {
			t.Fatalf("%s consumed request: %q", tc.route, body)
		}
	}
	before := len(h.audit)
	for _, route := range []string{"input", "continue", "steer"} {
		r := httptest.NewRequest("POST", "/api/"+route, strings.NewReader(`{"text":"Tiếp tục"}`))
		r.Header.Set("X-Ainovel-Username", "reader")
		s.recordActivityInput(r, route)
	}
	if len(h.audit) != before {
		t.Fatalf("host-owned inputs were audited twice: %+v", h.audit[before:])
	}
}

func TestActivityHistoryLegacyUserPairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tui.log")
	at := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	for _, ev := range []host.Event{
		{ID: "audit-1", Time: at, Category: "USER", Agent: "user", Username: "reader", Summary: "/input Tiếp tục"},
		{ID: "host-1", Time: at.Add(time.Millisecond), Category: "USER", Username: "machine", Summary: "[Tiếp tục] Tiếp tục"},
		{ID: "audit-2", Time: at.Add(time.Second), Category: "USER", Agent: "user", Username: "reader", Summary: "/input Sửa tuyến tình cảm"},
		{ID: "host-2", Time: at.Add(time.Second + time.Millisecond), Category: "USER", Summary: "[Can thiệp] Sửa tuyến tình cảm"},
		{ID: "pause", Time: at.Add(2 * time.Second), Category: "USER", Summary: "/pause", Username: "reader"},
	} {
		appendActivityFixture(t, path, ev)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	events, err := readActivity(path, stat.Size(), filepath.Dir(path), filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected three user actions, got %d: %+v", len(events), events)
	}
	for i, command := range []string{"pause", "steer", "continue"} {
		if events[i].Command != command || events[i].Username != "reader" {
			t.Fatalf("action %d: %+v", i, events[i])
		}
	}
	if events[1].Input != "Sửa tuyến tình cảm" || events[2].Input != "Tiếp tục" {
		t.Fatalf("raw user prose lost: %+v", events)
	}
}
