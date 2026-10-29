package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Only gateway-authenticated usernames reach the worker; never trust a browser header.
func (s *workerServer) recordActivityInput(r *http.Request, command string) {
	if r.Method != http.MethodPost {
		return
	}
	switch command {
	case "command", "start", "resume", "abort", "pause", "next", "retry", "reopen", "export", "import", "sync", "cocreate", "cocreate/message", "cocreate/commit":
	default:
		return
	}
	username := r.Header.Get("X-Ainovel-Username")
	if username == "" {
		return
	}
	recorder, ok := s.h.(interface{ RecordUserEvent(string, string, string) })
	if !ok {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return
	}
	text := ""
	for _, key := range []string{"line", "text", "direction", "message"} {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			text = value
			break
		}
	}
	if command == "command" {
		name, _, _ := strings.Cut(strings.TrimSpace(text), " ")
		command = strings.TrimPrefix(name, "/")
	}
	recorder.RecordUserEvent(username, command, text)
}
