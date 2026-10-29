package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/datnt1998/noverse/internal/audio"
)

// resumeAudioQueues opens only stories with unfinished persisted audio work.
func (g *Gateway) resumeAudioQueues() {
	ctx := context.Background()
	lib, err := g.lib(ctx, nil)
	if errors.Is(err, errNoLibrary) {
		return
	}
	if err != nil {
		slog.Warn("Không đọc được thư viện để tiếp tục audio", "module", "web", "err", err)
		return
	}
	scan, err := lib.Scan(ctx, nil)
	if err != nil {
		slog.Warn("Không quét được thư viện để tiếp tục audio", "module", "web", "err", err)
		return
	}
	for _, project := range scan.Projects {
		stories, err := lib.Stories(ctx, nil, project.ID)
		if err != nil {
			slog.Warn("Không đọc được truyện để tiếp tục audio", "module", "web", "project_slug", project.ID, "err", err)
			continue
		}
		for _, story := range stories {
			st, ok := lib.ResolveStory(ctx, nil, project.ID, story.Name)
			if !ok {
				continue
			}
			layout, err := audio.ResolveLayout(st)
			if err != nil {
				slog.Warn("Không đọc được thư mục audio", "module", "web", "story_slug", st.ID, "err", err)
				continue
			}
			pending, err := persistedAudioWork(layout)
			if err != nil {
				slog.Warn("Không đọc được hàng đợi audio", "module", "web", "story_slug", st.ID, "err", err)
				continue
			}
			if pending {
				if _, err := g.audioService(st); err != nil {
					slog.Warn("Không tiếp tục được audio", "module", "web", "story_slug", st.ID, "err", err)
				}
			}
		}
	}
}

func persistedAudioWork(layout audio.Layout) (bool, error) {
	data, err := os.ReadFile(filepath.Join(layout.AudioDir, "queue.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil {
		var queue struct {
			Jobs []audio.Job `json:"jobs"`
		}
		if err := json.Unmarshal(data, &queue); err != nil {
			return false, err
		}
		for _, job := range queue.Jobs {
			if job.State == audio.JobQueued || job.State == audio.JobRendering {
				return true, nil
			}
		}
	}
	data, err = os.ReadFile(filepath.Join(layout.AudioDir, "export-requests.json"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var requests map[int]audio.ExportRequest
	if err := json.Unmarshal(data, &requests); err != nil {
		return false, err
	}
	for _, request := range requests {
		if request.State == audio.JobQueued || request.State == audio.JobRendering {
			return true, nil
		}
	}
	return false, nil
}

func (g *Gateway) closeAudioServices() {
	root := g.RootPath()
	audioServices.Range(func(key, value any) bool {
		entry := value.(audioEntry)
		if entry.root != root {
			return true
		}
		lock := audioCreateLock(key.(string))
		lock.Lock()
		current, ok := audioServices.Load(key)
		if ok && current.(audioEntry).svc == entry.svc {
			audioServices.Delete(key)
		}
		lock.Unlock()
		if ok && current.(audioEntry).svc == entry.svc {
			entry.svc.Close()
		}
		return true
	})
}
