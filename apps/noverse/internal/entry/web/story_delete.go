package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/webauth"
	"github.com/gofrs/flock"
)

var errStoryDeleteRunning = errors.New("Truyện đang chạy — dừng trước khi xoá")

// deleteStory 处理 DELETE S；授权已在 serveStories 的 storyAccessAllowed 完成（非 GET 需可写）。
func (g *Gateway) deleteStory(w http.ResponseWriter, r *http.Request, lib library.Repository, user webauth.User, st library.Story) {
	if !jsonContent(r) {
		jsonError(w, 400, "Content-Type phải là application/json")
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Confirm != st.Story {
		jsonError(w, 400, "Tên truyện xác nhận không khớp")
		return
	}
	// Reserve removal using bookkeeping only, never a mutex across worker I/O.
	// Active metadata/proxy operations must finish before removal can be retried.
	slot := g.slot(st.ID)
	slot.mu.Lock()
	if slot.deleting || slot.active > 0 || slot.starting != nil {
		slot.mu.Unlock()
		jsonError(w, 409, errStoryDeleteRunning.Error())
		return
	}
	slot.deleting = true
	slot.mu.Unlock()
	defer func() {
		slot.mu.Lock()
		slot.deleting = false
		slot.mu.Unlock()
	}()
	err := g.removeIdleStory(r.Context(), lib, st, actorOf(user))
	if err != nil {
		switch {
		case errors.Is(err, errStoryDeleteRunning):
			jsonError(w, 409, errStoryDeleteRunning.Error())
		case errors.Is(err, os.ErrNotExist):
			jsonError(w, 404, "Không tìm thấy truyện")
		default:
			jsonError(w, 500, "Không xoá được truyện: "+err.Error())
		}
		return
	}
	g.mu.Lock()
	if slot := g.workers[st.ID]; slot != nil {
		slot.mu.Lock()
		if slot.deleted {
			delete(g.workers, st.ID)
		}
		slot.mu.Unlock()
	}
	g.mu.Unlock()
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (g *Gateway) removeIdleStory(ctx context.Context, lib library.Repository, st library.Story, actor *library.Actor) error {
	slot := g.slot(st.ID)
	slot.mu.Lock()
	deleted, worker := slot.deleted, slot.worker
	slot.mu.Unlock()
	if deleted {
		return os.ErrNotExist
	}
	if worker != nil && !workerDead(worker) {
		// Do not trust the cached running bit: an engine may start without a
		// browser reading a snapshot. An unreachable worker is not proven idle.
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		status, body, err := g.workerCall(checkCtx, worker, http.MethodGet, "snapshot", nil)
		cancel()
		var snapshot struct {
			IsRunning     *bool `json:"isRunning"`
			ActionPending bool  `json:"actionPending"`
		}
		if err != nil || status != 200 || json.Unmarshal(body, &snapshot) != nil || snapshot.IsRunning == nil || *snapshot.IsRunning || snapshot.ActionPending {
			return errStoryDeleteRunning
		}
		stopWorker(worker)
	}
	slot.mu.Lock()
	slot.worker, slot.running = nil, false
	slot.mu.Unlock()
	if st.Dir == "" {
		// Hosted backend: không có thư mục — repository soft-delete hàng stories
		// và history (chapters/versions...) giữ nguyên; không có tệp lịch sử nào
		// bị xoá, khớp chính sách "không xoá dữ liệu lịch sử".
		if err := lib.DeleteStory(ctx, actor, st.Project, st.Story); err != nil {
			return err
		}
		slot.mu.Lock()
		slot.deleted = true
		slot.mu.Unlock()
		g.removeStoryWatch(st.ID)
		g.jobsHub().removeStory(st.Project, st.Story)
		return nil
	}
	if _, err := os.Stat(st.Dir); err != nil {
		return err
	}
	output := g.storyOutputDir(st)
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	lease := flock.New(filepath.Join(output, ".ainovel.lock"))
	locked, err := lease.TryLock()
	if err != nil {
		_ = lease.Close()
		return err
	}
	if !locked {
		_ = lease.Close()
		return errStoryDeleteRunning
	}
	defer lease.Close()
	// The idle worker has released its lease; keep ours until removal finishes,
	// excluding a TUI process that tries to start during the delete operation.
	// This per-story lock prevents audio service recreation during removal; it
	// does not block library reads or other stories.
	audioLock := audioCreateLock(st.Dir)
	audioLock.Lock()
	defer audioLock.Unlock()
	entry, hasAudio := audioServices.LoadAndDelete(st.Dir)
	if hasAudio {
		entry.(audioEntry).svc.Close()
		audioServicesMu.Lock()
		close(audioServicesChanged)
		audioServicesChanged = make(chan struct{})
		audioServicesMu.Unlock()
	}
	if err := os.RemoveAll(st.Dir); err != nil {
		return err
	}
	slot.mu.Lock()
	slot.deleted = true
	slot.mu.Unlock()
	g.removeStoryWatch(st.ID)
	g.jobsHub().removeStory(st.Project, st.Story)
	return nil
}

func (g *Gateway) removeStoryWatch(id string) {
	hub := g.events()
	hub.mu.Lock()
	for _, sub := range hub.subs {
		delete(sub.watch, id)
	}
	hooks := append([]func(){}, hub.onWatch...)
	hub.mu.Unlock()
	runHooks(hooks)
}

func (h *jobsHub) removeStory(project, story string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.deletedStories == nil {
		h.deletedStories = make(map[string]bool)
	}
	key := project + "/" + story
	h.deletedStories[key] = true
	for _, job := range h.jobs {
		if job.Project == project && job.Story == story {
			h.applyLocked(JobChange{Job: job, Removed: true})
		}
	}
	for id := range h.controls {
		if strings.HasPrefix(id, key+"/") {
			delete(h.controls, id)
		}
	}
	for id := range h.dismissed {
		if strings.HasPrefix(id, key+"/") {
			delete(h.dismissed, id)
		}
	}
	delete(h.watching, key)
}

func (h *jobsHub) allowStory(project, story string) {
	h.mu.Lock()
	delete(h.deletedStories, project+"/"+story)
	h.mu.Unlock()
}
