package web

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/host/exp"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

// runGatewayExport tracks direct disk exports without starting an engine worker.
func (g *Gateway) runGatewayExport(ctx context.Context, st library.Story, sr *host.StoryReader, opts exp.Options, exportsDir string) (*exp.Result, string, error) {
	preview := opts
	preview.DryRun, preview.Progress = true, nil
	incoming, err := sr.Export(ctx, preview)
	if err != nil {
		return nil, "", publicExportError(err, st.Dir)
	}
	mode := opts.OnExists
	if mode == "" || mode == "fail" {
		if _, err := os.Stat(incoming.Path); err == nil {
			return nil, "", &exportConflictError{payload: exportConflict(incoming.Path, incoming, st.Dir)}
		}
	}
	hub := g.jobsHub()
	hub.mu.Lock()
	hub.seq++
	// Use a disjoint numeric range from worker-local counters.
	id := fmt.Sprintf("%s/export/%d", st.ID, uint64(time.Now().UnixNano())+hub.seq)
	hub.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	job := Job{ID: id, Kind: "export", Title: "Xuất truyện", Status: JobRunning, Href: storyRouteBase(st.Project, st.Story) + "?tab=exports", Project: st.Project, Story: st.Story, StartedAt: time.Now()}
	hub.mu.Lock()
	if hub.controls == nil {
		hub.controls = map[string]func(context.Context, string) error{}
	}
	hub.controls[id] = func(ctx context.Context, op string) error {
		switch op {
		case "cancel":
			cancel()
			return nil
		case "retry":
			current, ok := hub.get(id)
			if !ok {
				return errJobNotFound
			}
			if current.Status != JobFailed {
				return errJobUnsupported
			}
			_, _, err := g.runGatewayExport(ctx, st, sr, opts, exportsDir)
			return err
		default:
			return errJobUnsupported
		}
	}
	hub.mu.Unlock()
	hub.apply(JobChange{Job: job})
	opts.Progress = func(done, total, chapter int, title string) {
		p := float64(done) / float64(total)
		job.Progress = &p
		job.Unit = &JobUnit{Done: done, Total: total, Label: "Chương"}
		job.Step = fmt.Sprintf("Chương %d · %s", chapter, title)
		hub.apply(JobChange{Job: job})
	}
	result, err := sr.Export(ctx, opts)
	now := time.Now()
	job.FinishedAt = &now
	if err != nil {
		var exists *exp.ExistsError
		if errors.As(err, &exists) {
			hub.apply(JobChange{Job: job, Removed: true})
			return nil, "", &exportConflictError{payload: exportConflict(exists.Path, incoming, st.Dir)}
		}
		job.Status = JobFailed
		job.Error = publicExportError(err, st.Dir).Error()
		hub.apply(JobChange{Job: job})
		return nil, id, publicExportError(err, st.Dir)
	}
	job.Status = JobDoneS
	p := 1.0
	job.Progress = &p
	rel, _ := filepath.Rel(exportsDir, result.Path)
	download := storyAPIBase(st.Project, st.Story) + "/exports/file?path=" + url.QueryEscape(filepath.ToSlash(rel))
	job.Result = &JobResult{Title: "Đã xuất truyện", Sub: fmt.Sprintf("%d chương · %s", result.Chapters, humanBytes(result.Bytes)), Actions: []JobAction{{Label: "Tải xuống", Kind: "download", Href: download, Primary: true}}, Data: map[string]any{"path": filepath.ToSlash(rel), "bytes": result.Bytes, "chapters": result.Chapters, "skipped": nonNilInts(result.Skipped)}}
	hub.apply(JobChange{Job: job})
	return result, id, nil
}
