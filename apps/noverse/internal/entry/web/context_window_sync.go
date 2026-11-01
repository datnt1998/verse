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
	return g.postWorkerModelPath(ctx, scope, project, story, "models/context-reload", "cập nhật ngữ cảnh")
}

// syncWorkerModelSelection pushes the app-wide provider/model/thinking choice
// into running workers. Without it a story that booted with a provider whose
// subscription has expired keeps using that provider after the user switches
// the shared one in settings. Workers apply it to the next turn; an in-flight
// request finishes on the old client.
func (g *Gateway) syncWorkerModelSelection(ctx context.Context, scope, project, story string) error {
	return g.postWorkerModelPath(ctx, scope, project, story, "models/selection-reload", "cập nhật nhà cung cấp và model")
}

func (g *Gateway) postWorkerModelPath(ctx context.Context, scope, project, story, path, failureLabel string) error {
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+worker.addr+"/api/"+path, strings.NewReader("{}"))
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			failures = append(failures, fmt.Errorf("%s: %d %s", failureLabel, resp.StatusCode, strings.TrimSpace(string(body))))
		}
	}
	return errors.Join(failures...)
}
