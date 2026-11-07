package render

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
)

func (s *Service) RunExport(ctx context.Context, jobID string, onProgress func(ExportProgress, bool)) (record editor.ExportRecord, result ExportLaunchResult, err error) {
	job, _, pid, _, err := s.Inputs(ctx, jobID)
	if err != nil {
		return editor.ExportRecord{}, ExportLaunchResult{}, err
	}
	gate, err := s.Gate(ctx, jobID)
	if err != nil {
		return editor.ExportRecord{}, ExportLaunchResult{}, err
	}
	if !gate.Allowed {
		return editor.ExportRecord{}, ExportLaunchResult{}, renderError(domain.CloneErrQualityGate, strings.Join(gate.Reasons, "; "))
	}
	result, err = s.Supervisor.LaunchExport(ctx, pid, jobID, onProgress)
	if err != nil {
		return editor.ExportRecord{}, result, err
	}
	postStarted := time.Now()
	post := &ExportPostprocessTimings{}
	nextResourceSample := time.Time{}
	observe := func(phase string, force bool) {
		if s.Supervisor.ObserveResources == nil || (!force && time.Now().Before(nextResourceSample)) {
			return
		}
		if request, readErr := s.Store.GetExport(result.RequestID); readErr == nil {
			request.Progress = &ExportProgress{Phase: phase}
			s.Supervisor.ObserveResources(ctx, request)
		}
		nextResourceSample = time.Now().Add(30 * time.Second)
	}
	observe("postprocess", true)
	defer func() {
		post.TotalMS = time.Since(postStarted).Milliseconds()
		result.Postprocess = post
		slog.InfoContext(ctx, "export_postprocess", "request_id", result.RequestID, "success", err == nil,
			"total_ms", post.TotalMS, "source_prepare_ms", post.SourcePrepareMS,
			"compress_ms", post.CompressMS, "replacement_finalize_ms", post.ReplacementFinalizeMS,
			"original_acquire_ms", post.OriginalAcquireMS, "compression_attempted", post.CompressionAttempted, "compression_applied", post.CompressionApplied)
		observe("postprocess_done", true)
	}()
	// Successful replacement already acquires and validates the original. Large
	// files use disposable workspaces, so an eager acquire/release here would
	// download the entire original twice. Only unchanged/fallback output needs
	// this separate acquisition; preserve its integrity check and error precedence.
	original := func(cause error) (editor.ExportRecord, ExportLaunchResult, error) {
		started := time.Now()
		h, r, acquireErr := s.Editor.AcquireExport(ctx, pid, result.ExportID)
		post.OriginalAcquireMS += time.Since(started).Milliseconds()
		if acquireErr != nil {
			return r, result, acquireErr
		}
		h.Release()
		return r, result, cause
	}
	var snap library.Snapshot
	if err = json.Unmarshal(job.TemplateSnapshot, &snap); err != nil {
		return original(err)
	}
	template, err := editor.Parse(snap.Project)
	if err != nil {
		return original(err)
	}
	compress := true
	raw, _ := json.Marshal(template.Timeline()["tracks"])
	var tracks []editor.Object
	_ = json.Unmarshal(raw, &tracks)
	for _, track := range tracks {
		if settings, ok := track["templateScenes"].(map[string]any); ok && track["templateSlot"] == "scenes" && settings["compress"] == false {
			compress = false
		}
	}
	if !compress {
		return original(nil)
	}
	ffmpeg, ffprobe, err := s.ResolveFFmpeg(ctx)
	if err != nil {
		return original(err)
	}
	replaceStarted := time.Now()
	post.CompressionAttempted = true
	transformStarted := false
	compressed, replaceErr := s.Editor.ReplaceCloneExportFile(ctx, pid, result.ExportID, func(ctx context.Context, input, output string, project editor.Project, coverage editor.Object) error {
		transformStarted = true
		post.SourcePrepareMS = time.Since(replaceStarted).Milliseconds()
		compressStarted := time.Now()
		defer func() { post.CompressMS = time.Since(compressStarted).Milliseconds() }()
		return CompressExportFile(ctx, CompressOptions{Runner: s.Runner, Ffmpeg: ffmpeg, FFprobe: ffprobe, Input: input, Output: output, Project: project, CaptionCoverage: coverage, OnProgress: func(done float64, total *float64) {
			observe("compressing", false)
			if onProgress == nil {
				return
			}
			d := int64(done * 1000)
			var t *int64
			if total != nil {
				n := int64(*total * 1000)
				t = &n
			}
			onProgress(ExportProgress{Phase: "compressing", FramesDone: &d, FramesTotal: t}, false)
		}})
	})
	replaceMS := time.Since(replaceStarted).Milliseconds()
	if transformStarted {
		post.ReplacementFinalizeMS = max(0, replaceMS-post.SourcePrepareMS-post.CompressMS)
	} else {
		post.SourcePrepareMS = replaceMS
	}
	if replaceErr != nil {
		if ctx.Err() != nil {
			return original(ctx.Err())
		}
		return original(nil)
	}
	post.CompressionApplied = true
	return compressed, result, nil
}

// Template integrity is not part of checklist acceptance. Even "Vẫn xuất"
// cannot authorize changed/missing bytes in the pinned template snapshot.
func (s *Service) CheckTemplate(ctx context.Context, jobID string) error {
	job, upstream, _, files, err := s.Inputs(ctx, jobID)
	if err != nil {
		return err
	}
	_ = upstream
	var snap library.Snapshot
	if err = json.Unmarshal(job.TemplateSnapshot, &snap); err != nil {
		return err
	}
	if len(snap.Project) == 0 || snap.Binding.ProjectID == "" {
		return &editor.Error{Code: "gate", Message: "Job thiếu bản lưu template — chạy lại bước dựng để ràng buộc lại", Extra: editor.Object{"cloneGate": "template_media_drift"}}
	}
	for _, pin := range snap.Binding.Media {
		path := snap.MediaFiles[pin.MediaID]
		if path == "" {
			return &editor.Error{Code: "gate", Message: "Template thiếu media đã chốt", Extra: editor.Object{"cloneGate": "template_media_drift"}}
		}
		h, e := files.Acquire(ctx, path)
		if e != nil {
			return &editor.Error{Code: "gate", Message: "Media template không còn tồn tại", Extra: editor.Object{"cloneGate": "template_media_drift"}}
		}
		hash, e := steps.SHA256File(h.Path)
		h.Release()
		if e != nil {
			return e
		}
		if hash != strings.TrimPrefix(pin.ContentHash, "sha256:") {
			return &editor.Error{Code: "gate", Message: fmt.Sprintf("Media template %s đã thay đổi", pin.MediaID), Extra: editor.Object{"cloneGate": "template_media_drift"}}
		}
	}
	return nil
}
