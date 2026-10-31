package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
	"github.com/datnt1998/verse/platform/blobstore"
)

// ScoreInput contains only saved, generation-bound inputs. The caller supplies
// the immutable template snapshot, never the current library template.
type ScoreInput struct {
	Project       editor.Project
	Revision      string
	Job           domain.CloneJob
	Upstream      map[domain.CloneStep]domain.CloneStepOutput
	LUFS          *float64
	MeasurementID *string
	Template      editor.Project
	Media         pipeline.Media
	Output        *OutputEvidence
}
type OutputEvidence struct {
	Facts MediaFacts
	LUFS  *float64
}

type scoreProject struct {
	metadata                   editor.Object
	tracks, items, transitions []editor.Object
}

// Project JSON deliberately preserves unknown engine fields. Inspect individual
// entries leniently, as the upstream Zod schemas do, rather than failing a gate
// request because an unrelated stored item is malformed.
func scoreReadProject(project editor.Project) scoreProject {
	timeline := project.Timeline()
	p := scoreProject{metadata: project.Metadata()}
	for _, track := range scoreObjects(timeline["tracks"]) {
		if _, ok := track["id"].(string); ok {
			p.tracks = append(p.tracks, track)
		}
	}
	for _, item := range scoreObjects(timeline["items"]) {
		_, id := item["id"].(string)
		_, kind := item["type"].(string)
		_, track := item["trackId"].(string)
		from := scoreNumber(item, "from", math.NaN())
		duration := scoreNumber(item, "durationInFrames", math.NaN())
		if id && kind && track && scoreFinite(from) && scoreFinite(duration) {
			p.items = append(p.items, item)
		}
	}
	p.transitions = scoreObjects(timeline["transitions"])
	return p
}
func scoreObject(value any) editor.Object {
	object, _ := value.(map[string]any)
	return object
}

func scoreObjects(value any) []editor.Object {
	switch list := value.(type) {
	case []editor.Object:
		return list
	case []any:
		out := make([]editor.Object, 0, len(list))
		for _, entry := range list {
			if object := scoreObject(entry); object != nil {
				out = append(out, object)
			}
		}
		return out
	}
	return nil
}

func scoreString(object editor.Object, key string) string {
	value, _ := object[key].(string)
	return value
}

func scoreBool(object editor.Object, key string, fallback bool) bool {
	value, ok := object[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func scoreNumber(object editor.Object, key string, fallback float64) float64 {
	switch value := object[key].(type) {
	case float64:
		if scoreFinite(value) {
			return value
		}
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		parsed, err := value.Float64()
		if err == nil && scoreFinite(parsed) {
			return parsed
		}
	}
	return fallback
}

func scoreFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func scoreDigest(value string) string {
	return strings.TrimPrefix(value, "sha256:")
}

func scoreSameDigest(a, b string) bool {
	return a != "" && b != "" && scoreDigest(a) == scoreDigest(b)
}

type scoreTrackContext struct{ visible, muted bool }

func scoreTrackContexts(tracks []editor.Object) map[string]scoreTrackContext {
	byID := make(map[string]editor.Object, len(tracks))
	for _, track := range tracks {
		byID[scoreString(track, "id")] = track
	}
	contexts := make(map[string]scoreTrackContext, len(tracks))
	visiting := map[string]bool{}
	var resolve func(string) scoreTrackContext
	resolve = func(id string) scoreTrackContext {
		if cached, ok := contexts[id]; ok {
			return cached
		}
		track, ok := byID[id]
		if !ok {
			return scoreTrackContext{true, false}
		}
		// A corrupt parent cycle must not recurse forever or make a hidden layer pass.
		if visiting[id] {
			return scoreTrackContext{false, true}
		}
		visiting[id] = true
		parent := scoreTrackContext{true, false}
		if parentID := scoreString(track, "parentTrackId"); parentID != "" {
			parent = resolve(parentID)
		}
		result := scoreTrackContext{parent.visible && scoreBool(track, "visible", true), parent.muted || scoreBool(track, "muted", false)}
		delete(visiting, id)
		contexts[id] = result
		return result
	}
	for id := range byID {
		resolve(id)
	}
	return contexts
}
func scoreDurationFrames(project scoreProject) float64 {
	end := 0.
	for _, item := range project.items {
		end = math.Max(end, scoreNumber(item, "from", 0)+scoreNumber(item, "durationInFrames", 0))
	}
	return end
}
func scoreSceneItems(project scoreProject) []editor.Object {
	contexts := scoreTrackContexts(project.tracks)
	slot := ""
	for _, track := range project.tracks {
		if scoreString(track, "templateSlot") == "scenes" {
			slot = scoreString(track, "id")
			break
		}
	}
	out := []editor.Object{}
	for _, item := range project.items {
		if scoreString(item, "type") == "image" && scoreBool(item, "visible", true) && contexts[scoreString(item, "trackId")].visible && scoreNumber(item, "from", 0) >= 0 && scoreNumber(item, "durationInFrames", 0) > 0 && (slot == "" || scoreString(item, "trackId") == slot) {
			out = append(out, item)
		}
	}
	return out
}
func scoreAudibleItems(project scoreProject, mediaID string) []editor.Object {
	contexts := scoreTrackContexts(project.tracks)
	out := []editor.Object{}
	for _, item := range project.items {
		if scoreString(item, "type") == "audio" && scoreString(item, "mediaId") == mediaID && scoreBool(item, "visible", true) && !scoreBool(item, "muted", false) && !contexts[scoreString(item, "trackId")].muted && scoreNumber(item, "volume", 0) > -59.9 {
			out = append(out, item)
		}
	}
	return out
}
func scoreCutRhythm(items []editor.Object, spans []scenes.Scene, fps float64) []string {
	tolerance := math.Max(quality.CutToleranceMaxFrames, math.Ceil(quality.CutToleranceMaxMS/1000.*fps))
	sorted := append([]editor.Object(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool { return scoreNumber(sorted[i], "from", 0) < scoreNumber(sorted[j], "from", 0) })
	problems := []string{}
	if len(sorted) != len(spans) {
		problems = append(problems, fmt.Sprintf("%d clip ảnh ≠ %d cảnh nguồn", len(sorted), len(spans)))
	}
	for index := range min(len(sorted), len(spans)) {
		start := scoreNumber(sorted[index], "from", 0)
		end := start + scoreNumber(sorted[index], "durationInFrames", 0)
		span := spans[index]
		left, right := math.Abs(start-float64(span.StartFrame)), math.Abs(end-float64(span.EndFrame))
		if left > tolerance || right > tolerance {
			problems = append(problems, fmt.Sprintf("clip %g–%g lệch cảnh %s %d–%d (đầu %gf, cuối %gf)", start, end, span.SceneID, span.StartFrame, span.EndFrame, left, right))
		}
	}
	for index := range max(0, len(sorted)-1) {
		end := scoreNumber(sorted[index], "from", 0) + scoreNumber(sorted[index], "durationInFrames", 0)
		next := scoreNumber(sorted[index+1], "from", 0)
		if next < end {
			problems = append(problems, fmt.Sprintf("clip %d chồng lấn clip %d", index, index+1))
		}
		if next > end && index+1 < len(spans) && math.Abs(next-float64(spans[index+1].StartFrame)) > tolerance {
			problems = append(problems, fmt.Sprintf("khoảng trống trước clip %d", index+1))
		}
	}
	return problems
}
func scoreSubtitleCueSpans(item editor.Object, fps float64) []transcript.Interval {
	spans := []transcript.Interval{}
	if fps <= 0 {
		return spans
	}
	start := scoreNumber(item, "from", 0) / fps
	end := start + scoreNumber(item, "durationInFrames", 0)/fps
	for _, cue := range scoreObjects(item["cues"]) {
		if strings.TrimSpace(scoreString(cue, "text")) == "" {
			continue
		}
		left := math.Max(start, start+scoreNumber(cue, "startSeconds", 0))
		right := math.Min(end, start+scoreNumber(cue, "endSeconds", 0))
		if right > left {
			spans = append(spans, transcript.Interval{StartMs: left * 1000, EndMs: right * 1000})
		}
	}
	return spans
}
func scoreCoveredCaptions(project scoreProject, fps float64) []transcript.Interval {
	contexts := scoreTrackContexts(project.tracks)
	spans := []transcript.Interval{}
	for _, item := range project.items {
		if scoreString(item, "type") == "subtitle" && scoreBool(item, "visible", true) && contexts[scoreString(item, "trackId")].visible {
			spans = append(spans, scoreSubtitleCueSpans(item, fps)...)
		}
	}
	return spans
}
func scoreSpokenSpans(cues []transcript.Cue, fps float64, duration int) []transcript.Interval {
	input := make([]editor.FillCue, 0, len(cues))
	for _, cue := range cues {
		if cue.EndMs == nil || (*cue.EndMs <= cue.StartMs && len(cue.Words) == 0) {
			continue
		}
		entry := editor.FillCue{ID: cue.ID, Text: cue.Text, StartSec: cue.StartMs / 1000, EndSec: *cue.EndMs / 1000}
		for _, word := range cue.Words {
			entry.Words = append(entry.Words, editor.FillCueWord{Text: word.Text, StartMs: word.StartMs, EndMs: word.EndMs, Estimated: word.Estimated()})
		}
		input = append(input, entry)
	}
	prepared := editor.PrepareCaptionData(input, fps, duration)
	out := make([]transcript.Interval, 0, len(prepared.Words))
	for _, word := range prepared.Words {
		out = append(out, transcript.Interval{StartMs: scoreNumber(word, "startFrame", 0) * 1000 / fps, EndMs: scoreNumber(word, "endFrame", 0) * 1000 / fps})
	}
	return out
}

type scoreArtifacts struct {
	media    pipeline.Media
	upstream map[domain.CloneStep]domain.CloneStepOutput
}

func scoreMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, blobstore.ErrNotFound) || errors.Is(err, pipeline.ErrNotFound)
}
func (a scoreArtifacts) acquire(ctx context.Context, path string) (*media.Handle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	logical := ""
	for _, output := range a.upstream {
		for _, artifact := range output.Artifacts {
			if artifact.Path == path || strings.HasSuffix(artifact.Path, "/"+path) {
				if logical != "" && logical != artifact.Path {
					return nil, fmt.Errorf("ambiguous score artifact %s", path)
				}
				logical = artifact.Path
			}
		}
	}
	if logical == "" {
		logical = path
	}
	return a.media.Acquire(ctx, logical)
}
func (a scoreArtifacts) bytes(ctx context.Context, path string) ([]byte, error) {
	handle, err := a.acquire(ctx, path)
	if scoreMissing(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Release()
	data, err := os.ReadFile(handle.Path)
	if scoreMissing(err) {
		return nil, nil
	}
	return data, err
}
func scoreReadJSON[T any](ctx context.Context, a scoreArtifacts, path string) (*T, error) {
	data, err := a.bytes(ctx, path)
	if err != nil || data == nil {
		return nil, err
	}
	var value T
	if json.Unmarshal(data, &value) != nil {
		return nil, nil
	}
	return &value, nil
}
func (a scoreArtifacts) hash(ctx context.Context, path string) (string, error) {
	handle, err := a.acquire(ctx, path)
	if scoreMissing(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer handle.Release()
	hash, err := steps.SHA256File(handle.Path)
	if scoreMissing(err) {
		return "", nil
	}
	return hash, err
}
func scoreHashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// DeriveBinding is the sole freshness derivation. Hash manifest bytes (including
// whitespace), and declared large-source facts, not the source WAV/MP4 payload.
func DeriveBinding(ctx context.Context, media pipeline.Media, upstream map[domain.CloneStep]domain.CloneStepOutput, project editor.Project, revision string) (quality.Binding, error) {
	if media == nil {
		return quality.Binding{}, fmt.Errorf("score media is required")
	}
	p := scoreReadProject(project)
	settings, err := scoreHashJSON(editor.Object{"width": p.metadata["width"], "height": p.metadata["height"], "fps": p.metadata["fps"]})
	if err != nil {
		return quality.Binding{}, err
	}
	artifacts := scoreArtifacts{media, upstream}
	parts := editor.Object{"sourceAudioReferenceVersion": SourceAudioReferenceVersion}
	for _, path := range []string{"scenes/manifest.json", "images/manifest.json", "transcript/captions.json", "transcript/asr-anchors.json", "build/manifest.json", "build/checkpoint.json"} {
		hash, err := artifacts.hash(ctx, path)
		if err != nil {
			return quality.Binding{}, fmt.Errorf("bind %s: %w", path, err)
		}
		parts[path] = nil
		if hash != "" {
			parts[path] = hash
		}
	}
	sceneDoc, err := scoreReadJSON[scenes.Manifest](ctx, artifacts, "scenes/manifest.json")
	if err != nil {
		return quality.Binding{}, err
	}
	imageDoc, err := scoreReadJSON[images.Manifest](ctx, artifacts, "images/manifest.json")
	if err != nil {
		return quality.Binding{}, err
	}
	source, err := scoreReadJSON[editor.Object](ctx, artifacts, "download/source.info.json")
	if err != nil {
		return quality.Binding{}, err
	}
	declared := editor.Object{"sceneKeyframes": nil, "imageVersions": nil, "sourceSha256": nil, "sourceDurationSec": nil}
	if sceneDoc != nil {
		hashes := make([]any, len(sceneDoc.Scenes))
		for i, scene := range sceneDoc.Scenes {
			if scene.Keyframe.SHA256 != "" {
				hashes[i] = scene.Keyframe.SHA256
			}
		}
		declared["sceneKeyframes"] = hashes
	}
	if imageDoc != nil {
		hashes := make([]any, len(imageDoc.Items))
		for i, item := range imageDoc.Items {
			if version := images.Current(item); version != nil {
				hashes[i] = version.SHA256
			}
		}
		declared["imageVersions"] = hashes
	}
	if source != nil {
		declared["sourceSha256"] = (*source)["sha256"]
		declared["sourceDurationSec"] = (*source)["durationSec"]
	}
	parts["declared"] = declared
	hash, err := scoreHashJSON(parts)
	if err != nil {
		return quality.Binding{}, err
	}
	return quality.Binding{ProjectRevision: revision, SettingsHash: settings, ArtifactHash: hash}, nil
}

type scoreBuild struct {
	Audio *struct {
		MediaID          string   `json:"mediaId"`
		SourceLUFS       *float64 `json:"sourceLufs"`
		ReferenceVersion int      `json:"referenceVersion"`
		TargetLUFS       *float64 `json:"loudnessTargetLufs"`
	} `json:"audio"`
	Scenes []struct {
		SceneID string `json:"sceneId"`
		MediaID string `json:"mediaId"`
	} `json:"scenes"`
	ItemIDs struct {
		Captions string `json:"captions"`
	} `json:"itemIds"`
}

func scoreFix(item editor.Object) *quality.FixTarget {
	if item == nil {
		return nil
	}
	return &quality.FixTarget{ItemID: scoreString(item, "id")}
}
func scoreFirst(items []editor.Object) editor.Object {
	if len(items) > 0 {
		return items[0]
	}
	return nil
}
func scoreClipStrings(values []string, count int) []string { return values[:min(count, len(values))] }

// ComputeScore ports the complete checklist and hard prerequisites. Missing or
// malformed evidence is unknown/failing, while storage/cancellation errors are
// returned, never converted into a falsely passing score.
func ComputeScore(ctx context.Context, input ScoreInput) (*quality.Score, error) {
	if input.Media == nil {
		return nil, fmt.Errorf("score media is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	project := scoreReadProject(input.Project)
	template := scoreReadProject(input.Template)
	artifacts := scoreArtifacts{input.Media, input.Upstream}
	sceneDoc, err := scoreReadJSON[scenes.Manifest](ctx, artifacts, "scenes/manifest.json")
	if err != nil {
		return nil, err
	}
	imageDoc, err := scoreReadJSON[images.Manifest](ctx, artifacts, "images/manifest.json")
	if err != nil {
		return nil, err
	}
	captions, err := scoreReadJSON[transcript.CaptionManifest](ctx, artifacts, "transcript/captions.json")
	if err != nil {
		return nil, err
	}
	build, err := scoreReadJSON[scoreBuild](ctx, artifacts, "build/manifest.json")
	if err != nil {
		return nil, err
	}
	checkpoint, err := scoreReadJSON[scoreBuild](ctx, artifacts, "build/checkpoint.json")
	if err != nil {
		return nil, err
	}
	result := &quality.Score{Version: 1, Items: []quality.Item{}, Blockers: []quality.Blocker{}, MeasurementID: input.MeasurementID, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano), Generation: input.Job.Generation}
	add := func(id, status, evidence string, measured any, expected string, fix *quality.FixTarget) {
		points := 0.
		if status == "pass" {
			points = quality.Weights[id]
		}
		result.Items = append(result.Items, quality.Item{ID: id, Status: status, Points: points, Weight: quality.Weights[id], Evidence: evidence, Measured: measured, Expected: expected, Fix: fix})
	}
	block := func(id, message string, fix *quality.FixTarget) {
		result.Blockers = append(result.Blockers, quality.Blocker{ID: id, Message: message, Fix: fix})
	}
	status := func(pass bool) string {
		if pass {
			return "pass"
		}
		return "fail"
	}
	fps := scoreNumber(project.metadata, "fps", 0)
	frames := scoreDurationFrames(project)
	contexts := scoreTrackContexts(project.tracks)
	templateMissing := input.Template == nil
	if templateMissing {
		block("template", "Job thiếu bản lưu template — chạy lại bước dựng để ràng buộc lại", nil)
	}
	requiresCaptions := false
	var audioTrack, backgroundTrack editor.Object
	for _, track := range template.tracks {
		switch scoreString(track, "templateSlot") {
		case "captions":
			requiresCaptions = true
		case "source-audio":
			if audioTrack == nil {
				audioTrack = track
			}
		case "background-video":
			if backgroundTrack == nil {
				backgroundTrack = track
			}
		}
	}
	audioID := ""
	if build != nil && build.Audio != nil {
		audioID = build.Audio.MediaID
	}
	sourceDuration := 0.
	spans := []scenes.Scene{}
	noImages := false
	cutout := false
	if sceneDoc != nil {
		sourceDuration = sceneDoc.DurationSec
		spans = sceneDoc.Scenes
		noImages = sceneDoc.Recipe.ImagesMode == "none"
		cutout = strings.HasPrefix(sceneDoc.Recipe.Style, "cutout")
	}
	sceneItems := scoreSceneItems(project)
	outputDuration := 0.
	if fps > 0 {
		outputDuration = frames / fps
	}
	if input.Output != nil && input.Output.Facts.DurationSec != nil {
		outputDuration = *input.Output.Facts.DurationSec
	}
	if !scoreFinite(sourceDuration) || sourceDuration <= 0 || !scoreFinite(outputDuration) || outputDuration <= 0 {
		block("duration-invalid", "Không đo được thời lượng nguồn hoặc timeline — kiểm tra lại video nguồn và project", nil)
		add("duration", "unknown", "Thiếu thời lượng hợp lệ", nil, "", nil)
	} else {
		ratio := outputDuration / sourceDuration
		pass := math.Abs(ratio-1) <= quality.DurationToleranceRatio
		add("duration", status(pass), fmt.Sprintf("%.2fs / %.2fs", outputDuration, sourceDuration), editor.Object{"outputSec": outputDuration, "sourceSec": sourceDuration, "ratio": ratio}, "±10% so với nguồn", nil)
	}
	expected := len(spans)
	pass := expected > 0 && (noImages || len(sceneItems) == expected)
	sceneStatus := status(pass)
	if expected == 0 {
		sceneStatus = "unknown"
	}
	evidence := fmt.Sprintf("%d clip ảnh trên timeline", len(sceneItems))
	if noImages {
		evidence = "Template không tạo ảnh cảnh"
	}
	add("sceneCount", sceneStatus, evidence, editor.Object{"items": len(sceneItems), "scenes": expected}, fmt.Sprintf("%d cảnh", expected), nil)
	rhythm := []string{}
	if expected == 0 {
		rhythm = []string{"Không có manifest cảnh"}
	} else if !noImages {
		rhythm = scoreCutRhythm(sceneItems, spans, fps)
	}
	rhythmStatus := status(len(rhythm) == 0)
	if expected == 0 {
		rhythmStatus = "unknown"
	}
	evidence = fmt.Sprintf("%d boundary khớp", expected)
	if len(rhythm) > 0 {
		evidence = strings.Join(scoreClipStrings(rhythm, 3), "; ")
	}
	add("cutRhythm", rhythmStatus, evidence, editor.Object{"mismatches": scoreClipStrings(rhythm, 6)}, fmt.Sprintf("mép clip khớp boundary ≤ %gf", math.Max(quality.CutToleranceMaxFrames, math.Ceil(quality.CutToleranceMaxMS/1000.*fps))), scoreFix(scoreFirst(sceneItems)))

	if templateMissing {
		add("background", "unknown", "Job không có bản lưu template — không biết template có vai trò Nền video hay không", nil, "", nil)
	} else if backgroundTrack == nil {
		add("background", "pass", "Template không có vai trò Nền video — không yêu cầu nền", nil, "", nil)
	} else {
		clips := []editor.Object{}
		trackID := scoreString(backgroundTrack, "id")
		for _, entry := range project.items {
			if scoreString(entry, "trackId") == trackID && scoreString(entry, "type") == "video" && scoreBool(entry, "visible", true) && contexts[trackID].visible {
				clips = append(clips, entry)
			}
		}
		sort.SliceStable(clips, func(i, j int) bool { return scoreNumber(clips[i], "from", 0) < scoreNumber(clips[j], "from", 0) })
		joins := map[string]float64{}
		for _, transition := range project.transitions {
			if scoreString(transition, "trackId") == trackID {
				joins[scoreString(transition, "leftClipId")+"|"+scoreString(transition, "rightClipId")] = scoreNumber(transition, "durationInFrames", 0)
			}
		}
		problems := []string{}
		var fix *quality.FixTarget
		mark := func(message string, entry editor.Object, frame float64) {
			problems = append(problems, message)
			if fix == nil && entry != nil {
				at := int(frame)
				fix = &quality.FixTarget{ItemID: scoreString(entry, "id"), Frame: &at}
			}
		}
		if len(clips) == 0 {
			mark("track Nền video trong template chưa có clip nào — thêm clip vào template rồi chạy lại bước dựng", nil, 0)
		} else {
			if start := scoreNumber(clips[0], "from", 0); start > quality.CutToleranceMaxFrames {
				mark(fmt.Sprintf("thiếu nền 0–%gf", start), clips[0], 0)
			}
			covered := scoreNumber(clips[0], "from", 0) + scoreNumber(clips[0], "durationInFrames", 0)
			for i := 1; i < len(clips); i++ {
				clip := clips[i]
				start := scoreNumber(clip, "from", 0)
				allowed := joins[scoreString(clips[i-1], "id")+"|"+scoreString(clip, "id")]
				if start-covered > quality.CutToleranceMaxFrames {
					mark(fmt.Sprintf("hở nền %g–%gf", covered, start), clip, covered)
				} else if covered-start > allowed {
					mark(fmt.Sprintf("chồng nền %g–%gf", start, covered), clip, start)
				}
				covered = math.Max(covered, start+scoreNumber(clip, "durationInFrames", 0))
			}
			if frames-covered > quality.CutToleranceMaxFrames {
				mark(fmt.Sprintf("thiếu nền %g–%gf ở đuôi", covered, frames), clips[len(clips)-1], covered)
			}
		}
		evidence := fmt.Sprintf("%d clip nền phủ kín video", len(clips))
		if len(problems) > 0 {
			evidence = strings.Join(scoreClipStrings(problems, 3), "; ")
			block("background", "Nền video chưa phủ kín: "+problems[0], fix)
		}
		add("background", status(len(problems) == 0), evidence, editor.Object{"clips": len(clips), "problems": scoreClipStrings(problems, 6)}, "Clip nền phủ kín toàn video, hở ≤ 2f", fix)
	}

	sceneMedia := map[string]string{}
	if build != nil {
		for _, scene := range build.Scenes {
			sceneMedia[scene.SceneID] = scene.MediaID
		}
	}
	if noImages {
		add("images", "pass", "Template không tạo ảnh cảnh — video dùng nền và phụ đề", nil, "không có ảnh cảnh", nil)
	} else {
		missing := []string{}
		if imageDoc == nil || !images.ImagesApproved(*imageDoc) {
			missing = append(missing, "manifest ảnh chưa duyệt đủ")
		}
		checked := []images.Item{}
		if imageDoc != nil {
			for _, entry := range imageDoc.Items {
				if entry.Kind == "scene" {
					checked = append(checked, entry)
				}
			}
		}
		verified, skipped := 0, 0
		for _, entry := range checked {
			var sceneItem editor.Object
			start := -1
			for _, span := range spans {
				if span.SceneID == entry.ID {
					start = span.StartFrame
					break
				}
			}
			for _, entryItem := range sceneItems {
				if start >= 0 && math.Abs(scoreNumber(entryItem, "from", 0)-float64(start)) <= quality.CutToleranceMaxFrames && scoreString(entryItem, "mediaId") == sceneMedia[entry.ID] {
					sceneItem = entryItem
					break
				}
			}
			if entry.Status == "skipped" {
				if sceneItem == nil {
					missing = append(missing, fmt.Sprintf("cảnh %s chưa có clip ảnh hiển thị đủ span", entry.ID))
				} else {
					verified++
					skipped++
				}
				continue
			}
			chosen := images.Current(entry)
			hash := ""
			if chosen != nil {
				hash, err = artifacts.hash(ctx, chosen.Path)
				if err != nil {
					return nil, err
				}
			}
			switch {
			case chosen == nil || hash == "":
				missing = append(missing, fmt.Sprintf("ảnh %s thiếu file", entry.ID))
			case !scoreSameDigest(hash, chosen.SHA256):
				missing = append(missing, fmt.Sprintf("ảnh %s đổi hash", entry.ID))
			case chosen.Width != entry.Width || chosen.Height != entry.Height:
				missing = append(missing, fmt.Sprintf("ảnh %s sai kích thước", entry.ID))
			case cutout && chosen.Cutout == nil:
				missing = append(missing, fmt.Sprintf("ảnh %s thiếu tách nền", entry.ID))
			default:
				if cutout {
					hash, err := artifacts.hash(ctx, chosen.Cutout.Path)
					if err != nil {
						return nil, err
					}
					if !scoreSameDigest(hash, chosen.Cutout.SHA256) {
						missing = append(missing, fmt.Sprintf("tách nền %s đổi hash", entry.ID))
						continue
					}
					// Phase-05 FileRef has no dimensions: inspect the actual PNG rather than
					// trusting fields that that manifest model cannot carry.
					handle, err := artifacts.acquire(ctx, chosen.Cutout.Path)
					if err != nil {
						return nil, err
					}
					file, err := os.Open(handle.Path)
					if err != nil {
						handle.Release()
						return nil, err
					}
					config, format, decodeErr := image.DecodeConfig(file)
					closeErr := file.Close()
					handle.Release()
					if closeErr != nil {
						return nil, closeErr
					}
					if decodeErr != nil || format != "png" || config.Width != entry.Width || config.Height != entry.Height {
						missing = append(missing, fmt.Sprintf("tách nền %s sai kích thước", entry.ID))
						continue
					}
				}
				if sceneItem == nil {
					missing = append(missing, fmt.Sprintf("cảnh %s chưa có clip ảnh hiển thị đủ span", entry.ID))
				} else {
					verified++
				}
			}
		}
		pass := len(checked) > 0 && verified == len(checked) && len(missing) == 0
		itemStatus := status(pass)
		if len(checked) == 0 {
			itemStatus = "unknown"
		}
		evidence := fmt.Sprintf("%d/%d ảnh đúng", verified, len(checked))
		if skipped > 0 {
			evidence += fmt.Sprintf(" (%d cảnh bỏ qua dùng ảnh cảnh bên cạnh)", skipped)
		}
		if len(missing) > 0 {
			evidence = strings.Join(scoreClipStrings(missing, 3), "; ")
		}
		style := "WebP"
		if cutout {
			style = "PNG tách nền"
		}
		fix := scoreFix(scoreFirst(sceneItems))
		add("images", itemStatus, evidence, editor.Object{"verified": verified, "total": len(checked)}, fmt.Sprintf("%d ảnh %s đã duyệt, đúng kích thước, trên timeline", len(checked), style), fix)
		if !pass {
			why := "thiếu manifest"
			if len(missing) > 0 {
				why = missing[0]
			}
			block("images", "Ảnh cảnh chưa đủ: "+why, fix)
		}
	}

	var captionItem editor.Object
	captionID := ""
	if build != nil {
		captionID = build.ItemIDs.Captions
	}
	for _, entry := range project.items {
		if scoreString(entry, "type") == "subtitle" && (captionID == "" || scoreString(entry, "id") == captionID) {
			captionItem = entry
			break
		}
	}
	if templateMissing {
		add("captions", "unknown", "Job không có bản lưu template — không biết template có vai trò Phụ đề hay không", nil, "", nil)
	} else if !requiresCaptions {
		add("captions", "pass", "Template không có vai trò Phụ đề — không yêu cầu phụ đề", nil, "", nil)
	} else if captions == nil || captions.SpeechIntervals == nil || fps <= 0 {
		add("captions", "unknown", "Thiếu VAD speech intervals", nil, "", nil)
		block("captions", "Thiếu bằng chứng speech/caption — chạy lại bước phụ đề", nil)
	} else if len(captions.SpeechIntervals) == 0 {
		add("captions", "pass", "VAD không thấy speech — không cần phụ đề", nil, "", nil)
	} else {
		speech := captions.SpeechIntervals
		spoken := scoreSpokenSpans(captions.Cues, fps, int(frames))
		target := transcript.IntervalGaps(speech, transcript.IntervalGaps(speech, spoken))
		gaps := []transcript.Interval{}
		if len(spoken) == 0 {
			gaps = speech
		} else {
			for _, gap := range transcript.IntervalGaps(target, scoreCoveredCaptions(project, fps)) {
				if gap.EndMs-gap.StartMs >= 1000/fps {
					gaps = append(gaps, gap)
				}
			}
		}
		pass := len(gaps) == 0
		evidence := fmt.Sprintf("%d khoảng có lời đều có phụ đề", len(speech))
		var firstGap any
		if !pass {
			firstGap = []float64{gaps[0].StartMs / 1000, gaps[0].EndMs / 1000}
			evidence = fmt.Sprintf("thiếu phụ đề %d đoạn, đầu tiên %.1fs", len(gaps), gaps[0].StartMs/1000)
			if len(spoken) == 0 {
				evidence = "Bản chép lời không có câu nào dù video có tiếng nói"
			}
		}
		add("captions", status(pass), evidence, editor.Object{"speechIntervals": len(speech), "gapCount": len(gaps), "firstGapSec": firstGap, "gaps": gaps, "ranges": scoreCaptionGapRanges(gaps, captions.Cues, captionItem)}, "Mọi lúc đang nói có phụ đề hiển thị", scoreFix(captionItem))
		if !pass {
			message := fmt.Sprintf("Phụ đề thiếu %d đoạn đang nói — track bị ẩn hoặc câu bị xoá", len(gaps))
			if len(spoken) == 0 {
				message = "Video có tiếng nói nhưng chưa có phụ đề — chạy lại bước phụ đề"
			}
			fix := scoreFix(captionItem)
			if fix != nil {
				frame := int(math.Floor(gaps[0].StartMs/1000*fps + .5))
				fix.Frame = &frame
			}
			block("captions", message, fix)
		}
	}
	if requiresCaptions {
		anchors, err := scoreReadJSON[transcript.AsrAnchors](ctx, artifacts, "transcript/asr-anchors.json")
		if err != nil {
			return nil, err
		}
		pages := []transcript.DisplayedCaptionPage{}
		if fps > 0 {
			for _, entry := range project.items {
				if scoreString(entry, "type") != "subtitle" || !scoreBool(entry, "visible", true) || !contexts[scoreString(entry, "trackId")].visible {
					continue
				}
				for _, page := range ResolveCaptionPages(entry, fps) {
					displayed := transcript.DisplayedCaptionPage{ItemID: scoreString(entry, "id"), Estimated: page.Estimated, Words: []transcript.PageWord{}}
					for _, word := range page.Words {
						frame := scoreNumber(entry, "from", 0) + float64(word.StartFrame)
						displayed.Words = append(displayed.Words, transcript.PageWord{
							Text: word.Text, Frame: int64(frame), StartMs: frame * 1000 / fps,
							EndMs: (scoreNumber(entry, "from", 0) + float64(word.EndFrame)) * 1000 / fps,
						})
					}
					pages = append(pages, displayed)
				}
			}
		}
		var metric *scoreAlignmentEvidence
		pass := true
		karaoke := 0.
		for _, page := range pages {
			if !page.Estimated {
				karaoke++
			}
		}
		if len(pages) > 0 {
			karaoke /= float64(len(pages))
		}
		evidence := "Cảnh báo: thiếu mốc ASR độc lập — chưa đo được độ khớp tiếng"
		if anchors != nil && len(anchors.Words) > 0 {
			measured := transcript.MeasureCaptionAlignment(pages, *anchors)
			metric = &scoreAlignmentEvidence{AlignmentMetric: measured, Ranges: []quality.Range{}}
			pass = measured.MatchedRatio >= .6 && measured.MedianMs != nil && *measured.MedianMs <= 120 && measured.P95Ms != nil && *measured.P95Ms <= 250
			if !pass {
				metric.Ranges = scoreAlignmentRanges(measured, project, fps)
			}
			karaoke = measured.KaraokeRatio
			median, p95 := "—", "—"
			if measured.MedianMs != nil {
				median = fmt.Sprintf("%g", *measured.MedianMs)
			}
			if measured.P95Ms != nil {
				p95 = fmt.Sprintf("%g", *measured.P95Ms)
			}
			evidence = fmt.Sprintf("Phụ đề khớp tiếng: %.0f%% · median %s ms · p95 %s ms", measured.MatchedRatio*100, median, p95)
		}
		fix := &quality.FixTarget{Href: "/clone/" + url.PathEscape(input.Job.ID) + "#captions"}
		add("captionAlignment", status(pass), evidence, metric, "Khớp ≥60% · median ≤120 ms · p95 ≤250 ms", fix)
		add("captionKaraoke", "pass", fmt.Sprintf("%.0f%% trang karaoke · không chặn xuất", karaoke*100), editor.Object{"karaokeRatio": karaoke}, "Thông tin: ≥90% trang có mốc thật", nil)
		if !pass {
			block("captionAlignment", "Phụ đề khớp tiếng chưa đạt — chỉnh các cue lệch trong editor", fix)
		}
	}

	// Loudness is measured against the bound source or authored template target,
	// never a fixed target chosen by the scorer.
	lufs := input.LUFS
	if input.Output != nil && input.Output.LUFS != nil {
		lufs = input.Output.LUFS
	}
	var sourceLUFS, targetLUFS *float64
	currentReference := false
	audioEvidence := build
	if checkpoint != nil && checkpoint.Audio != nil {
		audioEvidence = checkpoint
	}
	if audioEvidence != nil && audioEvidence.Audio != nil {
		currentReference = audioEvidence.Audio.ReferenceVersion == SourceAudioReferenceVersion
		if currentReference {
			sourceLUFS = audioEvidence.Audio.SourceLUFS
		}
		targetLUFS = audioEvidence.Audio.TargetLUFS
	}
	reference := sourceLUFS
	if targetLUFS != nil {
		reference = targetLUFS
	}
	finite := lufs != nil && scoreFinite(*lufs) && reference != nil && scoreFinite(*reference)
	pass = finite && math.Abs(*lufs-*reference) <= quality.LoudnessToleranceLUFS
	loudnessStatus := "unknown"
	if finite {
		loudnessStatus = status(pass)
	}
	expectedLUFS := "Bằng âm lượng video gốc ±1 LUFS"
	if targetLUFS != nil {
		expectedLUFS = fmt.Sprintf("%.1f±1 LUFS (mục tiêu template)", *targetLUFS)
	} else if sourceLUFS != nil {
		expectedLUFS = fmt.Sprintf("%.1f±1 LUFS (bằng âm thanh gốc, mix cuối)", *sourceLUFS)
	}
	evidence = "Chưa có số đo âm lượng của mix cuối hoặc của âm thanh gốc"
	if !currentReference {
		evidence = "Tham chiếu âm thanh dùng layout cũ — chạy lại build và đo lại mix cuối"
	}
	if finite {
		kind := "gốc"
		if targetLUFS != nil {
			kind = "mục tiêu"
		}
		evidence = fmt.Sprintf("%.1f LUFS so với %.1f LUFS %s (layout engine)", *lufs, *reference, kind)
	}
	add("loudness", loudnessStatus, evidence, editor.Object{"lufs": lufs, "sourceLufs": sourceLUFS, "targetLufs": targetLUFS}, expectedLUFS, scoreFix(scoreFirst(scoreAudibleItems(project, audioID))))

	width, height := scoreNumber(project.metadata, "width", 0), scoreNumber(project.metadata, "height", 0)
	progressive := true
	if input.Output != nil {
		facts := input.Output.Facts
		if facts.Width != nil {
			width = float64(*facts.Width)
		}
		if facts.Height != nil {
			height = float64(*facts.Height)
		}
		if facts.Progressive != nil {
			progressive = *facts.Progressive
		}
	}
	pass = width == quality.RequiredWidth && height == quality.RequiredHeight && fps == quality.RequiredFPS && progressive
	add("resolution1080p", status(pass), fmt.Sprintf("%g×%g · %g fps", width, height, fps), editor.Object{"width": width, "height": height, "fps": fps, "progressive": progressive}, "1920×1080 · 30 fps · progressive", nil)
	if !pass {
		block("resolution1080p", "Project/MP4 phải là 1920×1080 · 30 fps progressive", nil)
	}

	var thumbnail *images.Version
	if imageDoc != nil {
		for _, entry := range imageDoc.Items {
			if entry.Kind == "thumbnail" {
				thumbnail = images.Current(entry)
				break
			}
		}
	}
	hash := ""
	if thumbnail != nil {
		hash, err = artifacts.hash(ctx, thumbnail.Path)
		if err != nil {
			return nil, err
		}
	}
	pass = thumbnail != nil && scoreSameDigest(hash, thumbnail.SHA256) && thumbnail.Width == quality.ThumbnailWidth && thumbnail.Height == quality.ThumbnailHeight && thumbnail.Mime == "image/webp"
	evidence = "Chưa có thumbnail đã duyệt"
	// Thumbnail facts have no common shape with timing/alignment measurements.
	var thumbMeasured any
	if thumbnail != nil {
		evidence = "Thumbnail sai kích thước/hash"
		thumbMeasured = editor.Object{"sha256": thumbnail.SHA256, "width": thumbnail.Width, "height": thumbnail.Height}
	}
	if pass {
		evidence = "Thumbnail đúng hash hiện hành"
	}
	thumbFix := &quality.FixTarget{Href: "/clone"}
	add("thumbnail", status(pass), evidence, thumbMeasured, "WebP đã duyệt 1280×720", thumbFix)
	if !pass {
		block("thumbnail", "Thumbnail chưa được duyệt đúng WebP 1280×720", thumbFix)
	}

	// Authored zero-point layers are hard prerequisites, independent of total.
	if audioID == "" {
		block("audio", "Thiếu định danh âm thanh gốc trong build manifest — dựng lại project", nil)
	} else if len(scoreAudibleItems(project, audioID)) == 0 {
		block("audio", "Âm thanh gốc phải hiện hữu và được nghe trên timeline (track không mute, clip không tắt)", nil)
	}
	var prototype editor.Object
	for _, entry := range template.items {
		if audioTrack != nil && scoreString(entry, "trackId") == scoreString(audioTrack, "id") && scoreString(entry, "type") == "audio" && (prototype == nil || scoreNumber(entry, "from", 0) < scoreNumber(prototype, "from", 0)) {
			prototype = entry
		}
	}
	for _, entry := range template.items {
		if scoreObject(entry["waveformVisualizer"]) == nil || prototype == nil || scoreString(prototype, "mediaId") == "" || scoreString(entry, "mediaId") != scoreString(prototype, "mediaId") {
			continue
		}
		found := false
		for _, candidate := range project.items {
			if scoreString(candidate, "id") == scoreString(entry, "id") && scoreString(candidate, "mediaId") == audioID && scoreString(scoreObject(candidate["waveformVisualizer"]), "envelopeBase64") != "" && scoreBool(candidate, "visible", true) && contexts[scoreString(candidate, "trackId")].visible {
				found = true
				break
			}
		}
		if !found {
			block("waveform", "Sóng giọng trong template phải hiển thị và dùng âm thanh gốc", scoreFix(entry))
		}
	}
	groups := map[string][]editor.Object{}
	groupOrder := []string{}
	for _, entry := range template.items {
		if group := scoreString(entry, "linkedGroupId"); group != "" {
			if _, ok := groups[group]; !ok {
				groupOrder = append(groupOrder, group)
			}
			groups[group] = append(groups[group], entry)
		}
	}
	visible := map[string]bool{}
	for _, entry := range project.items {
		if scoreBool(entry, "visible", true) && contexts[scoreString(entry, "trackId")].visible {
			visible[scoreString(entry, "id")] = true
		}
	}
	for _, id := range groupOrder {
		group := groups[id]
		text, shapes := false, 0
		for _, entry := range group {
			if scoreString(entry, "type") == "shape" {
				shapes++
			}
			label := scoreString(entry, "text")
			if _, ok := entry["text"].(string); !ok {
				label = scoreString(entry, "label")
			}
			label = strings.ToLower(label)
			if scoreString(entry, "type") == "text" && (strings.Contains(label, "subscribe") || strings.Contains(label, "đăng ký")) {
				text = true
			}
		}
		if !text || shapes < 2 {
			continue
		}
		for _, entry := range group {
			if !visible[scoreString(entry, "id")] {
				block("subscribe", "Các lớp Subscribe trong template phải hiện hữu trên timeline", scoreFix(group[0]))
				break
			}
		}
	}
	mediaIDs := []string{}
	seen := map[string]bool{}
	for _, entry := range sceneItems {
		id := scoreString(entry, "mediaId")
		if id != "" && !seen[id] {
			seen[id] = true
			mediaIDs = append(mediaIDs, id)
		}
	}
	if audioID != "" && !seen[audioID] {
		mediaIDs = append(mediaIDs, audioID)
	}
	for _, id := range mediaIDs {
		path := "editor/media/" + id + "/metadata.json"
		if err := media.ValidatePath(path); err != nil {
			block("media", fmt.Sprintf("Media %s không còn tồn tại — nhập lại", id), nil)
			break
		}
		record, err := scoreReadJSON[editor.Object](ctx, artifacts, path)
		if err != nil {
			return nil, err
		}
		if record == nil || scoreString(*record, "id") != id {
			block("media", fmt.Sprintf("Media %s không còn tồn tại — nhập lại", id), nil)
			break
		}
	}
	result.Binding, err = DeriveBinding(ctx, input.Media, input.Upstream, input.Project, input.Revision)
	if err != nil {
		return nil, err
	}
	result.Total = quality.Total(result.Items)
	result.Passed = quality.AllowsExport(result)
	return result, nil
}

type scoreAlignmentEvidence struct {
	transcript.AlignmentMetric
	Ranges []quality.Range `json:"ranges"`
}

func scoreCaptionGapRanges(gaps []transcript.Interval, cues []transcript.Cue, item editor.Object) []quality.Range {
	ranges := []quality.Range{}
	for _, gap := range gaps {
		matched := false
		for _, cue := range cues {
			if cue.EndMs == nil {
				continue
			}
			start, end := math.Max(gap.StartMs, cue.StartMs), math.Min(gap.EndMs, *cue.EndMs)
			if end <= start {
				continue
			}
			matched = true
			ranges = append(ranges, quality.Range{
				Start: start / 1000, End: end / 1000, CheckID: "captions", Status: "fail",
				CueID: cue.ID, ItemID: scoreString(item, "id"), Text: cue.Text,
			})
		}
		if !matched {
			ranges = append(ranges, quality.Range{Start: gap.StartMs / 1000, End: gap.EndMs / 1000, CheckID: "captions", Status: "fail", ItemID: scoreString(item, "id")})
		}
	}
	return ranges
}

func scoreAlignmentRanges(metric transcript.AlignmentMetric, project scoreProject, fps float64) []quality.Range {
	ranges := []quality.Range{}
	if fps <= 0 {
		return ranges
	}
	byID := make(map[string]editor.Object, len(project.items))
	for _, item := range project.items {
		byID[scoreString(item, "id")] = item
	}
	for _, worst := range metric.Worst {
		// Worst also contains exact/passing matches. Only offsets beyond the
		// source median limit contribute actionable alignment failure ranges.
		if math.Abs(worst.DeltaMs) <= 120 {
			continue
		}
		item := byID[worst.ItemID]
		if item == nil {
			continue
		}
		localFrame := float64(worst.Frame) - scoreNumber(item, "from", 0)
		span := quality.Range{
			Start: float64(worst.Frame) / fps, End: float64(worst.Frame+1) / fps,
			CheckID: "captionAlignment", Status: "fail", ItemID: worst.ItemID,
			Text: worst.Text, OffsetSec: new(worst.DeltaMs / 1000),
		}
		for _, cue := range scoreObjects(item["cues"]) {
			if localFrame/fps >= scoreNumber(cue, "startSeconds", 0) && localFrame/fps < scoreNumber(cue, "endSeconds", 0) {
				span.CueID = scoreString(cue, "id")
				break
			}
		}
		for _, page := range ResolveCaptionPages(item, fps) {
			for _, word := range page.Words {
				if float64(word.StartFrame) == localFrame {
					span.End = (scoreNumber(item, "from", 0) + float64(word.EndFrame)) / fps
					break
				}
			}
		}
		ranges = append(ranges, span)
	}
	return ranges
}

// ScoreRanges exposes actionable failing intervals in seconds, preferring real
// caption gaps/alignment frames and otherwise the bound item's visible range.
func ScoreRanges(score *quality.Score, project editor.Project) []quality.Range {
	ranges := []quality.Range{}
	if score == nil {
		return ranges
	}
	p := scoreReadProject(project)
	fps := scoreNumber(p.metadata, "fps", 0)
	if fps <= 0 {
		return ranges
	}
	duration := scoreDurationFrames(p) / fps
	byID := make(map[string]editor.Object, len(p.items))
	for _, entry := range p.items {
		byID[scoreString(entry, "id")] = entry
	}
	boundRange := func(id, status string, fix *quality.FixTarget) quality.Range {
		span := quality.Range{Start: 0, End: duration, CheckID: id, Status: status}
		if fix != nil {
			if item := byID[fix.ItemID]; item != nil {
				span.ItemID = fix.ItemID
				span.Start = scoreNumber(item, "from", 0) / fps
				span.End = span.Start + scoreNumber(item, "durationInFrames", 0)/fps
			}
			if fix.Frame != nil {
				span.Start = float64(*fix.Frame) / fps
				if span.End <= span.Start {
					span.End = math.Min(duration, span.Start+1/fps)
				}
			}
		}
		return span
	}
	represented := map[string]bool{}
	for _, check := range score.Items {
		if check.Status == "pass" {
			continue
		}
		represented[check.ID] = true
		// Measured is heterogeneous persisted JSON. Decode this small common view
		// so freshly computed and reloaded scores produce identical ranges.
		var measured struct {
			Ranges      []quality.Range             `json:"ranges"`
			Gaps        []transcript.Interval       `json:"gaps"`
			FirstGapSec []float64                   `json:"firstGapSec"`
			Worst       []transcript.AlignmentWorst `json:"worst"`
		}
		data, err := json.Marshal(check.Measured)
		if err == nil && json.Unmarshal(data, &measured) == nil {
			if len(measured.Ranges) > 0 {
				for _, span := range measured.Ranges {
					if check.ID == "captionAlignment" && span.OffsetSec != nil && math.Abs(*span.OffsetSec)*1000 <= 120 {
						continue
					}
					span.CheckID, span.Status = check.ID, check.Status
					ranges = append(ranges, span)
				}
				continue
			}
			if len(measured.Gaps) > 0 {
				for _, gap := range measured.Gaps {
					ranges = append(ranges, quality.Range{Start: gap.StartMs / 1000, End: gap.EndMs / 1000, CheckID: check.ID, Status: check.Status})
				}
				continue
			}
			if len(measured.FirstGapSec) == 2 {
				ranges = append(ranges, quality.Range{Start: measured.FirstGapSec[0], End: measured.FirstGapSec[1], CheckID: check.ID, Status: check.Status})
				continue
			}
			if len(measured.Worst) > 0 {
				for _, span := range scoreAlignmentRanges(transcript.AlignmentMetric{Worst: measured.Worst}, p, fps) {
					span.CheckID, span.Status = check.ID, check.Status
					ranges = append(ranges, span)
				}
				continue
			}
		}
		fix := check.Fix
		for _, blocker := range score.Blockers {
			if blocker.ID == check.ID && blocker.Fix != nil {
				fix = blocker.Fix
				break
			}
		}
		ranges = append(ranges, boundRange(check.ID, check.Status, fix))
	}
	// A 100-point score can still have zero-point hard prerequisites, including
	// waveform/Subscribe/audio. They must remain actionable in the editor too.
	for _, blocker := range score.Blockers {
		if !represented[blocker.ID] {
			ranges = append(ranges, boundRange(blocker.ID, "fail", blocker.Fix))
			represented[blocker.ID] = true
		}
	}
	return ranges
}
