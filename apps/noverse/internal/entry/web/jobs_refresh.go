package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

// refreshJobs reads only already-running, readable workers. It never launches a
// worker simply because the dock is opened.
func (g *Gateway) refreshJobs(ctx context.Context, user identity.User) {
	g.mu.Lock()
	slots := make(map[string]*storyWorker, len(g.workers))
	for key, slot := range g.workers {
		if user.CanRead(key) {
			slots[key] = slot
		}
	}
	g.mu.Unlock()
	for key, slot := range slots {
		project, story, ok := splitStoryKey(key)
		if !ok {
			continue
		}
		slot.mu.Lock()
		worker := slot.worker
		slot.mu.Unlock()
		if worker == nil || workerDead(worker) {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		status, body, err := g.workerCall(callCtx, worker, http.MethodGet, "jobs", nil)
		cancel()
		if err != nil || status != http.StatusOK {
			continue
		}
		var response struct {
			Jobs []Job `json:"jobs"`
		}
		if json.Unmarshal(body, &response) != nil {
			continue
		}
		jobs := make([]Job, 0, len(response.Jobs))
		for _, job := range response.Jobs {
			jobs = append(jobs, qualifyJob(project, story, job))
		}
		g.jobsHub().resetStory(project, story, jobs)
	}
	hub := g.jobsHub()
	hub.mu.Lock()
	providers := make([]JobsProvider, 0, len(hub.providers))
	for _, provider := range hub.providers {
		providers = append(providers, provider)
	}
	hub.mu.Unlock()
	for _, provider := range providers {
		for _, job := range provider.Jobs() {
			hub.apply(JobChange{Job: job})
		}
	}
}
