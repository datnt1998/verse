package diag

import (
	"os"
	"path/filepath"
	"testing"
)

// Session 日志里夹着 {"_event": …} 观察事件行；它们不是消息，不能在骨架里变成
// 空 role 的事件。
func TestScanSessionSkipsEventLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer-ch01.jsonl")
	lines := `{"role":"assistant","content":[{"type":"text","text":"x"}],"_meta":{"provider":"openai-codex","model":"gpt-6-luna"}}
{"_event":{"kind":"retry","agent":"writer","at":"2026-10-29T10:00:00Z","attempt":1,"max":3,"delay_ms":500,"error_kind":"stream_idle"}}
{"role":"tool","content":[{"type":"text","text":"ok"}],"metadata":{"tool_call_id":"c1"}}
{"_event":{"kind":"subagent_exit","agent":"writer","at":"2026-10-29T10:00:01Z","trigger":"tool"}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	var rc RuntimeCapture
	models := map[string]RoleModel{}
	evs := scanSession(path, "writer-ch01", &rc, models)
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2 (assistant + tool): %+v", len(evs), evs)
	}
	if evs[0].Role != "assistant" || evs[1].Role != "tool" {
		t.Errorf("roles = %q, %q; want assistant, tool", evs[0].Role, evs[1].Role)
	}
	if models["writer-ch01"].Model != "gpt-6-luna" {
		t.Errorf("model from _meta = %q, want gpt-6-luna", models["writer-ch01"].Model)
	}
}
