package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Refresh already-running workers only; a new worker reads the config chain at
// startup. Never restart a model-backed engine or change its active selection.
func (g *Gateway) refreshWorkerModelContexts(ctx context.Context, scope, project, story string) error {
	g.mu.Lock()
	slots := make([]*storyWorker, 0, len(g.workers))
	for key, slot := range g.workers {
		p, s, ok := splitStoryKey(key)
		if scope == "global" || (ok && p == project && (scope == "project" || s == story)) {
			slots = append(slots, slot)
		}
	}
	g.mu.Unlock()
	client := &http.Client{Transport: g.workerTransport(), Timeout: 5 * time.Second}
	var failures []error
	for _, slot := range slots {
		slot.mu.Lock()
		worker := slot.worker
		slot.mu.Unlock()
		if worker == nil || worker.noModel {
			continue
		}
		// Worker POSTs are JSON-only (webguard.JSONContent); a bodyless POST is rejected with 400.
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+worker.addr+"/api/models/context-reload", strings.NewReader("{}"))
		if err != nil {
			failures = append(failures, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Ainovel-Worker-Token", g.workerToken)
		resp, err := client.Do(req)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			failures = append(failures, fmt.Errorf("cập nhật ngữ cảnh: %d", resp.StatusCode))
		}
	}
	return errors.Join(failures...)
}
