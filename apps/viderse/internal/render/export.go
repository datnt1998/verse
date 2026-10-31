package render

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
)

func (s *Service) RunExport(ctx context.Context, jobID string, onProgress func(ExportProgress, bool)) (editor.ExportRecord, ExportLaunchResult, error) {
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
	result, err := s.Supervisor.LaunchExport(ctx, pid, jobID, onProgress)
	if err != nil {
		return editor.ExportRecord{}, result, err
	}
	h, record, err := s.Editor.AcquireExport(ctx, pid, result.ExportID)
	if err != nil {
		return record, result, err
	}
	h.Release()
	var snap library.Snapshot
	if err = json.Unmarshal(job.TemplateSnapshot, &snap); err != nil {
		return record, result, err
	}
	template, err := editor.Parse(snap.Project)
	if err != nil {
		return record, result, err
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
		return record, result, nil
	}
	ffmpeg, ffprobe, err := s.ResolveFFmpeg(ctx)
	if err != nil {
		return record, result, err
	}
	compressed, err := s.Editor.ReplaceCloneExportFile(ctx, pid, result.ExportID, func(ctx context.Context, input, output string, project editor.Project, coverage editor.Object) error {
		return CompressExportFile(ctx, CompressOptions{Runner: s.Runner, Ffmpeg: ffmpeg, FFprobe: ffprobe, Input: input, Output: output, Project: project, CaptionCoverage: coverage, OnProgress: func(done float64, total *float64) {
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
	if err != nil {
		if ctx.Err() != nil {
			return record, result, ctx.Err()
		}
		return record, result, nil
	}
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
