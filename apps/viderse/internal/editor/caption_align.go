package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
)

// CueIDs retains the JSON union: a string array or the literal all-drifting.
type AlignCaptionsInput struct {
	CueIDs   any    `json:"cueIds"`
	Revision string `json:"revision,omitempty"`
}

type alignmentCue struct {
	item, stored          Object
	cue                   transcript.Cue
	firstFrame, lastFrame float64
}

func (s Service) AlignCaptions(ctx context.Context, pid string, input AlignCaptionsInput) (Object, error) {
	if err := ValidateID(pid); err != nil {
		return nil, err
	}
	selected, all, err := captionSelection(input.CueIDs)
	if err != nil {
		return nil, err
	}
	project, revision, err := s.Store.Read(ctx, pid)
	if err != nil {
		return nil, err
	}
	if input.Revision != "" {
		if err := CheckRevision(revision, input.Revision); err != nil {
			return nil, err
		}
	}
	if fillNum(Object(project), "schemaVersion", 0) != SchemaVersion {
		return nil, apiError("unsupported_format", "Phiên bản project không được hỗ trợ")
	}
	fps := fillNum(project.Metadata(), "fps", 0)
	if !fillFinite(fps) || fps <= 0 {
		return nil, apiError("unsupported_format", "FPS project không hợp lệ")
	}
	loader := s.Anchors
	if loader == nil {
		loader = s.LoadCaptionAnchors
	}
	evidence, err := loader(ctx, pid)
	if err != nil {
		return nil, err
	}
	anchors, err := parseCaptionAnchors(evidence)
	if err != nil {
		return nil, err
	}
	next := Clone(project)
	cues, err := projectAlignmentCues(next, fps)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, entry := range cues {
		known[entry.cue.ID] = true
	}
	for id := range selected {
		if !known[id] {
			return nil, apiError("invalid_request", "Không tìm thấy cue: "+id)
		}
	}
	results := []Object{}
	changedItems := map[string]Object{}
	for i, entry := range cues {
		if !all && !selected[entry.cue.ID] {
			continue
		}
		estimated := len(entry.cue.Words) == 0
		for _, word := range entry.cue.Words {
			estimated = estimated || word.Estimated()
		}
		// Sampled nonestimated timing remains independent evidence, even when
		// explicitly selected. It must never be silently replaced with ASR.
		if !estimated {
			continue
		}
		nextStart := math.Inf(1)
		if i+1 < len(cues) {
			nextStart = cues[i+1].cue.StartMs
		}
		aligned := transcript.AlignCaptionWords(entry.cue, anchors.Words, nextStart)
		if aligned.EndMs == nil {
			continue
		}
		hasMeasured := false
		for _, word := range aligned.Words {
			hasMeasured = hasMeasured || !word.Estimated()
		}
		if !hasMeasured {
			continue
		}
		words, responseWords := []Object{}, []Object{}
		origin := fillNum(entry.item, "from", 0)
		for _, word := range aligned.Words {
			// Keep exact source onsets until the timeline's single frame
			// conversion. Negative local frames are rebased as one item below.
			start := fillRound(word.StartMs*fps/1000) - origin
			end := math.Max(start+1, fillRound(word.EndMs*fps/1000)-origin)
			words = append(words, Object{
				"text": word.Text, "startFrame": start, "endFrame": end,
				"estimated": word.Estimated(), "timingSource": word.TimingSource,
			})
			responseWords = append(responseWords, Object{
				"w": word.Text, "startSec": word.StartMs / 1000, "endSec": word.EndMs / 1000,
			})
		}
		retained := []Object{}
		for _, word := range fillObjects(entry.item["words"]) {
			start := fillNum(word, "startFrame", 0)
			if start < entry.firstFrame || start >= entry.lastFrame {
				retained = append(retained, word)
			}
		}
		entry.item["words"] = append(retained, words...)
		if entry.stored != nil {
			entry.stored["startSeconds"] = aligned.StartMs/1000 - origin/fps
			entry.stored["endSeconds"] = *aligned.EndMs/1000 - origin/fps
		}
		changedItems[fillString(entry.item, "id")] = entry.item
		results = append(results, Object{
			"cueId": entry.cue.ID, "offsetSec": (aligned.StartMs - entry.cue.StartMs) / 1000,
			"words": responseWords,
		})
	}
	for _, item := range changedItems {
		rebaseAlignedCaption(item, fps)
	}
	if len(changedItems) > 0 {
		end := 0.0
		for _, item := range fillObjects(next.Timeline()["items"]) {
			end = math.Max(end, fillNum(item, "from", 0)+fillNum(item, "durationInFrames", 0))
		}
		next["duration"] = end / fps
		revision, err = s.Store.Write(ctx, pid, next, revision)
		if err != nil {
			return nil, err
		}
	}
	return Object{"cues": results, "revision": revision}, nil
}

func captionSelection(value any) (map[string]bool, bool, error) {
	selected := map[string]bool{}
	invalid := func() (map[string]bool, bool, error) {
		return nil, false, apiError("invalid_request", "cueIds phải là danh sách hoặc all-drifting")
	}
	switch ids := value.(type) {
	case string:
		if ids != "all-drifting" {
			return invalid()
		}
		return selected, true, nil
	case []string:
		for _, id := range ids {
			if id == "" {
				return invalid()
			}
			selected[id] = true
		}
	case []any:
		for _, value := range ids {
			id, ok := value.(string)
			if !ok || id == "" {
				return invalid()
			}
			selected[id] = true
		}
	default:
		return invalid()
	}
	return selected, false, nil
}

// Rebase only when a real anchor precedes the item's old origin. Shifting all
// local coordinates preserves every unselected cue's absolute measured timing.
func rebaseAlignedCaption(item Object, fps float64) {
	words, cues := fillObjects(item["words"]), fillObjects(item["cues"])
	shift := 0.0
	for _, word := range words {
		shift = math.Min(shift, fillNum(word, "startFrame", 0))
	}
	for _, cue := range cues {
		shift = math.Min(shift, math.Floor(fillNum(cue, "startSeconds", 0)*fps))
	}
	duration := fillNum(item, "durationInFrames", 0) - shift
	if shift < 0 {
		item["from"] = fillNum(item, "from", 0) + shift
		for _, word := range words {
			word["startFrame"] = fillNum(word, "startFrame", 0) - shift
			word["endFrame"] = fillNum(word, "endFrame", 0) - shift
		}
		for _, cue := range cues {
			cue["startSeconds"] = fillNum(cue, "startSeconds", 0) - shift/fps
			cue["endSeconds"] = fillNum(cue, "endSeconds", 0) - shift/fps
		}
	}
	sort.SliceStable(words, func(i, j int) bool {
		return fillNum(words[i], "startFrame", 0) < fillNum(words[j], "startFrame", 0)
	})
	estimated := false
	for _, word := range words {
		estimated = estimated || fillBool(word, "estimated")
		duration = math.Max(duration, fillNum(word, "endFrame", 0))
	}
	item["words"], item["wordsEstimated"], item["durationInFrames"] = words, estimated, duration
}

func projectAlignmentCues(project Project, fps float64) ([]alignmentCue, error) {
	out := []alignmentCue{}
	seen := map[string]bool{}
	for _, item := range fillObjects(project.Timeline()["items"]) {
		kind := fillString(item, "type")
		if kind != "subtitle" && kind != "text" {
			continue
		}
		origin := fillNum(item, "from", 0)
		itemWords := fillObjects(item["words"])
		stored := fillObjects(item["cues"])
		if len(stored) == 0 {
			stored = []Object{{
				"id": fillString(item, "id"), "text": fillString(item, "text"),
				"startSeconds": 0, "endSeconds": fillNum(item, "durationInFrames", 0) / fps,
			}}
		}
		for _, storedCue := range stored {
			id := fillString(storedCue, "id")
			start, end := fillNum(storedCue, "startSeconds", math.NaN()), fillNum(storedCue, "endSeconds", math.NaN())
			if id == "" || seen[id] || !fillFinite(start) || !fillFinite(end) || start < 0 || end <= start {
				return nil, apiError("unsupported_format", "Cue project không hợp lệ")
			}
			seen[id] = true
			startMs, endMs := (origin/fps+start)*1000, (origin/fps+end)*1000
			cue := transcript.Cue{ID: id, Text: fillString(storedCue, "text"), StartMs: startMs, EndMs: &endMs, Source: "captions"}
			first, last := start*fps, end*fps
			for _, word := range itemWords {
				a, b := fillNum(word, "startFrame", math.NaN()), fillNum(word, "endFrame", math.NaN())
				if a < first || a >= last {
					continue
				}
				if !fillFinite(a) || !fillFinite(b) || b <= a {
					return nil, apiError("unsupported_format", "Mốc từ project không hợp lệ")
				}
				// Item-level wordsEstimated also records dropped pieces. It
				// cannot turn a measured word into permission to overwrite it.
				estimated := fillBool(word, "estimated")
				cue.Words = append(cue.Words, transcript.Word{
					Text: fillString(word, "text"), StartMs: (origin + a) * 1000 / fps,
					EndMs: (origin + b) * 1000 / fps, EstimatedFlag: &estimated,
				})
			}
			var target Object
			if kind == "subtitle" {
				target = storedCue
			}
			out = append(out, alignmentCue{item: item, stored: target, cue: cue, firstFrame: first, lastFrame: last})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].cue.StartMs < out[j].cue.StartMs })
	return out, nil
}

var anchorHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func parseCaptionAnchors(value Object) (transcript.AsrAnchors, error) {
	invalid := func() (transcript.AsrAnchors, error) {
		return transcript.AsrAnchors{}, apiError("unsupported_format", "Bằng chứng mốc giọng gốc không hợp lệ")
	}
	_, generatedAtIsString := value["generatedAt"].(string)
	if fillNum(value, "version", 0) != 1 || !anchorHash.MatchString(fillString(value, "sourceAudioSha256")) || !generatedAtIsString {
		return invalid()
	}
	model := fillObject(value["model"])
	for _, key := range []string{"name", "version", "size", "computeType", "device"} {
		if _, ok := model[key].(string); !ok {
			return invalid()
		}
	}
	if value["words"] == nil || value["spans"] == nil {
		return invalid()
	}
	for _, word := range fillObjects(value["words"]) {
		a, b := fillNum(word, "startMs", math.NaN()), fillNum(word, "endMs", math.NaN())
		if _, ok := word["text"].(string); !ok || !fillFinite(a) || !fillFinite(b) || a < 0 || b < 0 {
			return invalid()
		}
		if probability, exists := word["probability"]; exists && probability != nil {
			p := fillNum(word, "probability", math.NaN())
			if !fillFinite(p) || p < 0 || p > 1 {
				return invalid()
			}
		}
	}
	for _, span := range fillObjects(value["spans"]) {
		if !fillFinite(fillNum(span, "startMs", math.NaN())) || !fillFinite(fillNum(span, "endMs", math.NaN())) {
			return invalid()
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return invalid()
	}
	var anchors transcript.AsrAnchors
	if err := json.Unmarshal(raw, &anchors); err != nil {
		return invalid()
	}
	return anchors, nil
}

// LoadCaptionAnchors resolves phase05 immutable build/transcript artifacts. When
// a job has multiple transcript versions, inject the job's current output lookup;
// a content hash's lexical order cannot establish which speech is authoritative.
func (s Service) LoadCaptionAnchors(ctx context.Context, pid string) (Object, error) {
	if !strings.HasPrefix(pid, "clone-") {
		return nil, apiError("not_found", "Project không có mốc giọng gốc")
	}
	rest := strings.TrimPrefix(pid, "clone-")
	split := strings.LastIndex(rest, "-g")
	if split <= 0 {
		return nil, apiError("not_found", "Không tìm thấy job của project")
	}
	jobID := rest[:split]
	if err := ValidateID(jobID); err != nil {
		return nil, err
	}
	paths, err := s.listPaths(ctx, "clone/"+jobID+"/")
	if err != nil {
		return nil, err
	}
	bound := false
	anchors := []string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "/build/checkpoint.json") {
			value, err := s.readCaptionArtifact(ctx, path)
			if err != nil {
				return nil, err
			}
			if value["projectId"] == pid {
				bound = true
			}
		}
		if strings.HasSuffix(path, "/transcript/asr-anchors.json") {
			anchors = append(anchors, path)
		}
	}
	if !bound || len(anchors) == 0 {
		return nil, apiError("not_found", "Không tìm thấy bằng chứng giọng gốc của bản dựng")
	}
	if len(anchors) != 1 {
		return nil, apiError("conflict", "Có nhiều phiên bản mốc giọng; cần chọn đầu ra transcript của job")
	}
	return s.readCaptionArtifact(ctx, anchors[0])
}

func (s Service) readCaptionArtifact(ctx context.Context, path string) (Object, error) {
	handle, err := s.acquire(ctx, path)
	if err != nil {
		return nil, err
	}
	defer handle.Release()
	raw, err := os.ReadFile(handle.Path)
	if err != nil {
		return nil, err
	}
	var value Object
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode caption artifact %s: %w", path, err)
	}
	if value == nil {
		return nil, apiError("unsupported_format", "Artifact phụ đề phải là đối tượng JSON")
	}
	return value, nil
}
