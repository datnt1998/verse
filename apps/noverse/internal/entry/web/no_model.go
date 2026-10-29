package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/datnt1998/noverse/internal/commands"
)

const needsModelMessage = "Chưa có cấu hình mô hình — thiết lập ở Cài đặt › Nhà cung cấp"

func actionNeedsModel(r *http.Request, path string) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch path {
	case "input", "resume", "next", "start-file", "reopen", "retry", "models", "thinking", "cocreate/message", "cocreate/commit", "import", "import/preview", "import/confirm", "simulate", "importsim", "sync/fix":
		return true
	case "sync", "command":
		data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
		r.Body = io.NopCloser(bytes.NewReader(data))
		if err != nil || len(data) > 1<<20 {
			return false
		}
		if path == "sync" {
			var request struct {
				Check bool `json:"check"`
			}
			if json.Unmarshal(data, &request) != nil {
				return false
			}
			return !request.Check
		}
		var request struct {
			Line string `json:"line"`
		}
		if json.Unmarshal(data, &request) != nil {
			return false
		}
		inv, err := commands.Parse(request.Line)
		if err != nil {
			return false
		}
		switch inv.Name {
		case "help", "diag", "model", "config", "export", "cocreate":
			return false
		case "sync":
			return !inv.Flags["check"]
		default:
			return true
		}
	}
	if strings.HasPrefix(path, "diag/actions/") {
		return true
	}
	if strings.HasPrefix(path, "jobs/") {
		return strings.HasSuffix(path, "/resume") || strings.HasSuffix(path, "/retry")
	}
	return false
}

// Configuration mutations retire model-free hosts only. Existing model-backed
// engines are not interrupted, and a fresh launch reloads the normal config chain.
func (g *Gateway) restartUnconfiguredWorkers(scope, project, story string) {
	if scope != "global" && scope != "project" && scope != "story" {
		return
	}
	g.mu.Lock()
	slots := make([]*storyWorker, 0, len(g.workers))
	for key, slot := range g.workers {
		p, s, ok := splitStoryKey(key)
		if scope == "global" || (ok && p == project && (scope == "project" || s == story)) {
			slots = append(slots, slot)
		}
	}
	g.mu.Unlock()
	for _, slot := range slots {
		slot.mu.Lock()
		worker := slot.worker
		if worker == nil || !worker.noModel || slot.starting != nil {
			slot.mu.Unlock()
			continue
		}
		slot.worker, slot.running = nil, false
		slot.mu.Unlock()
		stopWorker(worker)
	}
}
