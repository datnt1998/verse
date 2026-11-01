package scenes

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type Probe struct {
	DurationSec   float64
	StartTimeSec  float64
	Width, Height int
	Rotation      float64
	VariableFPS   bool
}
type Deps struct {
	Ffmpeg, FFprobe      string
	Now                  func() time.Time
	Probe                func(context.Context, *pipeline.StepContext, string) (Probe, error)
	Detect               func(context.Context, *pipeline.StepContext, string, float64) ([]DetectFrame, error)
	Extract              func(context.Context, *pipeline.StepContext, string, string, int, int, float64) (float64, error)
	ReadNoImagesOverride func(context.Context, domain.CloneJob) (*bool, error)
}
type Handler struct{ Deps Deps }

var _ pipeline.StepHandler = Handler{}

// ResolveRecipe consumes the immutable library snapshot, never the mutable
// template row. NoImages only changes the mode, not the retained cover asset.
func ResolveRecipe(raw json.RawMessage, noImages bool, duration float64) (Recipe, error) {
	return ResolveRecipeWithOverride(raw, noImages, nil, duration)
}

// ResolveRecipeWithOverride overlays a video-level image choice without changing its template.
func ResolveRecipeWithOverride(raw json.RawMessage, noImages bool, override *bool, duration float64) (Recipe, error) {
	var snap library.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return Recipe{}, err
	}
	mode := snap.Images.Mode
	if override != nil {
		noImages = *override
		if !noImages && mode == library.ImagesNone {
			mode = "auto"
		}
	}
	if noImages {
		mode = library.ImagesNone
	}
	imageMode := snap.Images
	imageMode.Mode = mode
	count := library.ImageCountFor(imageMode, duration)
	style := snap.Style
	if style == "cutout" && snap.Framing == "close" {
		style = "cutout-close"
	}
	thumb := snap.Thumbnail
	if thumb == "" {
		thumb = "clean"
	}
	return Recipe{Style: style, ImagesMode: mode, ImageCount: count, Thumbnail: thumb, ImagePrompt: snap.ImagePrompt, CoverPrompt: snap.CoverPrompt.Prompt, CoverMediaID: snap.CoverMediaID}, nil
}
func (h Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	source, err := s.Acquire(ctx, "download/source.mp4")
	if err != nil {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrInternal, "Thiếu video nguồn", "Chạy lại bước download")
	}
	defer source.Release()
	captions, err := s.Acquire(ctx, "transcript/captions.json")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	defer captions.Release()
	var doc struct {
		Cues []Cue `json:"cues"`
	}
	raw, err := os.ReadFile(captions.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return pipeline.StepResult{}, err
	}
	probe := h.Deps.Probe
	if probe == nil {
		probe = h.probe
	}
	p, err := probe(ctx, s, source.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if !finite(p.DurationSec) || p.DurationSec <= 0 {
		return pipeline.StepResult{}, steps.CloneError(domain.CloneErrInternal, "Video nguồn không có thời lượng hợp lệ", "Chạy lại bước download")
	}
	var override *bool
	if h.Deps.ReadNoImagesOverride != nil {
		override, err = h.Deps.ReadNoImagesOverride(ctx, s.Job)
		if err != nil {
			return pipeline.StepResult{}, err
		}
	}
	recipe, err := ResolveRecipeWithOverride(s.Job.TemplateSnapshot, s.Job.NoImages, override, p.DurationSec)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	detect := h.Deps.Detect
	if detect == nil {
		detect = h.detect
	}
	frames, err := detect(ctx, s, source.Path, DefaultThreshold)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	normalized := NormalizeBoundaries(SceneCandidates(frames, DefaultThreshold), p.DurationSec, DefaultMinSceneSec, p.StartTimeSec, DefaultFPS)
	count := recipe.ImageCount
	if recipe.ImagesMode == library.ImagesNone {
		count = 1
	}
	normalized = SelectTargetBoundaries(normalized, p.DurationSec, count, DefaultMinSceneSec, DefaultFPS)
	spans, total, err := SpansFromBoundaries(normalized.Boundaries, p.DurationSec, DefaultFPS)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	sum, err := steps.SHA256File(source.Path)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	now := time.Now
	if h.Deps.Now != nil {
		now = h.Deps.Now
	}
	m := Manifest{Version: 1, Revision: 1, FPS: 30, TotalFrames: total, DurationSec: p.DurationSec, Source: FileRef{"download/source.mp4", sum}, Recipe: recipe, Detection: Detection{Method: "ffmpeg.scdet", Threshold: DefaultThreshold, MinDurationSec: DefaultMinSceneSec, RawFrames: len(frames), Width: p.Width, Height: p.Height, Rotation: p.Rotation, VariableFPS: p.VariableFPS}, Scenes: []Scene{}}
	for _, c := range normalized.Kept {
		m.Detection.Boundaries = append(m.Detection.Boundaries, Boundary{int(math.Round(c.TimeSec * 30)), c.TimeSec, c.Score})
	}
	m.Detection.Dropped = normalized.Dropped
	artifacts := []media.Artifact{}
	extract := h.Deps.Extract
	if extract == nil {
		extract = h.extract
	}
	for _, span := range spans {
		if err := ctx.Err(); err != nil {
			return pipeline.StepResult{}, err
		}
		if s.PauseRequested() {
			if err := s.SaveCheckpoint(ctx, m); err != nil {
				return pipeline.StepResult{}, err
			}
			return pipeline.StepResult{}, pipeline.ErrPaused
		}
		id := fmt.Sprintf("sc-%03d", span.Index)
		frame := span.StartFrame + (span.EndFrame-span.StartFrame)/2
		rel := "scenes/keyframes/" + id + ".jpg"
		out := filepath.Join(s.StagingDir, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return pipeline.StepResult{}, err
		}
		pts, err := extract(ctx, s, source.Path, out, span.StartFrame, span.EndFrame, float64(frame)/30)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		hash, err := steps.SHA256File(out)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		text := AssignSceneText(float64(span.StartFrame)/30, float64(span.EndFrame)/30, doc.Cues)
		m.Scenes = append(m.Scenes, Scene{SceneID: id, StartFrame: span.StartFrame, EndFrame: span.EndFrame, Text: text.Text, Context: text.Context, CaptionCueIDs: text.CueIDs, TextSource: "captions", Keyframe: FileRef{rel, hash}, KeyframeFrame: frame, KeyframePTS: pts})
		artifacts = append(artifacts, media.Artifact{Path: rel, File: out, ContentType: "image/jpeg"})
		done := int64(len(m.Scenes))
		all := int64(len(spans))
		if err = s.Progress(ctx, domain.CloneProgress{UnitsDone: done, UnitsTotal: &all, Phase: "keyframes"}); err != nil {
			return pipeline.StepResult{}, err
		}
	}
	if err = m.Validate(); err != nil {
		return pipeline.StepResult{}, err
	}
	// Design V3 reviews images, not scenes: the analysis is accepted as soon as
	// it is produced. Leaving it pending made `build` refuse every job forever
	// (no screen or endpoint could approve it). Editing scenes still bumps the
	// revision and clears both fields.
	fingerprint := AnalysisFingerprint(m)
	m.Approval = &Approval{Fingerprint: fingerprint, ApprovedAt: now().UTC().Format(time.RFC3339Nano)}
	m.Pending = nil
	file := filepath.Join(s.StagingDir, "scenes", "manifest.json")
	if err = steps.WriteJSONAtomic(file, m); err != nil {
		return pipeline.StepResult{}, err
	}
	artifacts = append(artifacts, media.Artifact{Path: "scenes/manifest.json", File: file, ContentType: "application/json"})
	return pipeline.StepResult{Artifacts: artifacts, Data: map[string]any{"sceneCount": len(m.Scenes), "durationSec": p.DurationSec, "totalFrames": total, "fps": 30}}, nil
}
func (h Handler) probe(ctx context.Context, s *pipeline.StepContext, file string) (Probe, error) {
	if s.ProcessRunner == nil || h.Deps.FFprobe == "" {
		return Probe{}, steps.CloneError(domain.CloneErrToolMissing, "Thiếu ffprobe", "Kiểm tra FFmpeg trong Cài đặt")
	}
	r, err := s.RunProcess(ctx, h.Deps.FFprobe, []string{"-v", "error", "-show_format", "-show_streams", "-of", "json", file}, tools.Spec{Timeout: time.Minute, LogLimit: 1 << 20})
	if err != nil {
		return Probe{}, err
	}
	var doc struct {
		Format struct {
			Duration string `json:"duration"`
			Start    string `json:"start_time"`
		} `json:"format"`
		Streams []struct {
			Type          string `json:"codec_type"`
			Width, Height int
			Average       string `json:"avg_frame_rate"`
			Rate          string `json:"r_frame_rate"`
			Tags          struct {
				Rotate string `json:"rotate"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err = json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
		return Probe{}, err
	}
	p := Probe{}
	p.DurationSec, _ = strconv.ParseFloat(doc.Format.Duration, 64)
	p.StartTimeSec, _ = strconv.ParseFloat(doc.Format.Start, 64)
	for _, stream := range doc.Streams {
		if stream.Type == "video" {
			p.Width, p.Height = stream.Width, stream.Height
			p.Rotation, _ = strconv.ParseFloat(stream.Tags.Rotate, 64)
			p.VariableFPS = stream.Rate != stream.Average
			break
		}
	}
	return p, nil
}
func (h Handler) detect(ctx context.Context, s *pipeline.StepContext, file string, threshold float64) ([]DetectFrame, error) {
	if s.ProcessRunner == nil || h.Deps.Ffmpeg == "" {
		return nil, steps.CloneError(domain.CloneErrToolMissing, "Thiếu FFmpeg", "Kiểm tra FFmpeg trong Cài đặt")
	}
	// ffmpeg writes every scdet metadata record to the staged evidence file;
	// never truncate it through the bounded process-log tail.
	log := filepath.Join(s.StagingDir, "scdet.txt")
	if err := os.MkdirAll(s.StagingDir, 0755); err != nil {
		return nil, err
	}
	escaped := strings.NewReplacer("\\", "\\\\", ":", "\\:", "'", "\\'").Replace(log)
	_, err := s.RunProcess(ctx, h.Deps.Ffmpeg, []string{"-hide_banner", "-nostdin", "-i", file, "-an", "-vf", fmt.Sprintf("scdet=threshold=%g,metadata=print:file='%s'", threshold, escaped), "-f", "null", "-"}, tools.Spec{Timeout: 2 * time.Hour, LogLimit: 8192})
	if err != nil {
		return nil, err
	}
	f, err := os.Open(log)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadSceneMetadata(f, -1)
}

var ptsPattern = regexp.MustCompile(`pts_time:([+-]?[0-9.]+)`)

func (h Handler) extract(ctx context.Context, s *pipeline.StepContext, input, output string, start, end int, at float64) (float64, error) {
	if s.ProcessRunner == nil || h.Deps.Ffmpeg == "" {
		return 0, steps.CloneError(domain.CloneErrToolMissing, "Thiếu FFmpeg", "Kiểm tra FFmpeg trong Cài đặt")
	}
	for _, precise := range []bool{false, true} {
		args := []string{"-hide_banner", "-nostdin", "-y"}
		if !precise {
			args = append(args, "-ss", fmt.Sprintf("%.9f", at))
		}
		args = append(args, "-i", input)
		filter := "showinfo,scale=1280:-2"
		if precise {
			filter = fmt.Sprintf("select='gte(t,%.9f)',showinfo,scale=1280:-2", at)
		}
		args = append(args, "-an", "-vf", filter, "-frames:v", "1", "-q:v", "2", output)
		r, err := s.RunProcess(ctx, h.Deps.Ffmpeg, args, tools.Spec{Timeout: 5 * time.Minute, LogLimit: 65536})
		if err != nil {
			return 0, err
		}
		matches := ptsPattern.FindAllStringSubmatch(r.Stderr, -1)
		if len(matches) == 0 {
			return 0, fmt.Errorf("keyframe lacks showinfo timestamp")
		}
		pts, _ := strconv.ParseFloat(matches[len(matches)-1][1], 64)
		if !precise {
			pts += at
		}
		f := int(math.Round(pts * 30))
		if f >= start && f < end {
			return pts, nil
		}
	}
	return 0, steps.CloneError(domain.CloneErrStale, "Keyframe nằm ngoài cảnh", "Phân tích lại cảnh")
}
