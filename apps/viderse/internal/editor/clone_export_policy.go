package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func exportFinite(v any) bool   { n := exportNumber(v); return !math.IsNaN(n) && !math.IsInf(n, 0) }
func exportPositive(v any) bool { return exportFinite(v) && exportNumber(v) > 0 }
func exportStrict(o Object, required []string, optional ...string) bool {
	if o == nil {
		return false
	}
	allowed := map[string]bool{}
	for _, k := range required {
		allowed[k] = true
		if _, ok := o[k]; !ok {
			return false
		}
	}
	for _, k := range optional {
		allowed[k] = true
	}
	for k := range o {
		if !allowed[k] {
			return false
		}
	}
	return true
}
func exportShortString(v any) bool { s, ok := v.(string); return ok && len(s) > 0 && len(s) <= 256 }
func exportSize(v any) bool {
	o := exportObject(v)
	return exportStrict(o, []string{"width", "height"}) && exportPositive(o["width"]) && exportPositive(o["height"])
}
func invalidExportCoverage(reason string) error {
	return apiError("invalid_request", "Invalid caption coverage: "+reason)
}

func validateExportCommitMetadata(metadata Object) error {
	if metadata == nil {
		return nil
	}
	if !exportStrict(metadata, nil, "captionCoverage") {
		return invalidExportCoverage("unknown commit metadata")
	}
	raw, exists := metadata["captionCoverage"]
	if !exists {
		return nil
	}
	supplied := exportObject(raw)
	if !exportStrict(supplied, []string{"projectRevision", "coverage"}) || !exportShortString(supplied["projectRevision"]) {
		return invalidExportCoverage("invalid revision binding")
	}
	coverage := exportObject(supplied["coverage"])
	if !exportStrict(coverage, []string{"version", "output", "renderedFrames", "entries"}) || exportNumber(coverage["version"]) != 1 {
		return invalidExportCoverage("invalid coverage version or fields")
	}
	output := exportObject(coverage["output"])
	if !exportStrict(output, []string{"width", "height", "fps"}) || !exportPositive(output["width"]) || !exportPositive(output["height"]) || !exportPositive(output["fps"]) {
		return invalidExportCoverage("invalid output surface")
	}
	frames := exportNumber(coverage["renderedFrames"])
	if !exportPositive(frames) || math.Trunc(frames) != frames || frames > 9007199254740991 {
		return invalidExportCoverage("invalid completed frame count")
	}
	entries := exportArray(coverage["entries"])
	if entries == nil || len(entries) > 10000 {
		return invalidExportCoverage("invalid entries")
	}
	for _, raw := range entries {
		entry := exportObject(raw)
		if !exportStrict(entry, []string{"instancePath", "itemId", "space", "bounds"}) || !exportShortString(entry["itemId"]) || !exportSize(entry["space"]) {
			return invalidExportCoverage("invalid entry")
		}
		path := exportArray(entry["instancePath"])
		if path == nil || len(path) > 32 {
			return invalidExportCoverage("invalid instance path")
		}
		for _, part := range path {
			if !exportShortString(part) {
				return invalidExportCoverage("invalid wrapper ID")
			}
		}
		if entry["bounds"] != nil {
			bounds := exportObject(entry["bounds"])
			if !exportStrict(bounds, []string{"x", "y", "width", "height"}) || !exportFinite(bounds["x"]) || !exportFinite(bounds["y"]) || exportNumber(bounds["x"]) < 0 || exportNumber(bounds["y"]) < 0 || !exportPositive(bounds["width"]) || !exportPositive(bounds["height"]) {
				return invalidExportCoverage("invalid painted bounds")
			}
		}
	}
	return nil
}

// Render identity resolution matches render-caption-tracks.ts, including its
// intentional retention of hidden saved caption items as explicit null entries.
func validateExportCaptionCoverage(metadata, session Object, p Project, revision string) (Object, error) {
	if err := validateExportCommitMetadata(metadata); err != nil {
		return nil, err
	}
	supplied := exportObject(metadata["captionCoverage"])
	if supplied == nil {
		return nil, nil
	}
	if supplied["projectRevision"] != session["projectRevision"] || supplied["projectRevision"] != revision {
		return nil, apiError("conflict", "Caption coverage belongs to a different project revision")
	}
	timeline := p.Timeline()
	items := exportArray(timeline["items"])
	tracks := exportArray(timeline["tracks"])
	if items == nil || tracks == nil {
		return nil, invalidExportCoverage("saved timeline cannot resolve caption instances")
	}
	coverage := exportObject(supplied["coverage"])
	output := exportObject(coverage["output"])
	duration := float64(0)
	for _, raw := range items {
		item := exportObject(raw)
		if !exportValidTimelineItem(item) {
			return nil, invalidExportCoverage("invalid saved timeline item")
		}
		duration = math.Max(duration, exportNumber(item["from"])+exportNumber(item["durationInFrames"]))
	}
	fps := exportNumber(p.Metadata()["fps"])
	if output["width"] == nil || exportNumber(output["width"]) != 1920 || exportNumber(output["height"]) != 1080 || exportNumber(output["fps"]) != 30 || !exportPositive(fps) || exportNumber(coverage["renderedFrames"]) != math.Floor(duration/fps*30+0.5) {
		return nil, invalidExportCoverage("output dimensions, fps or completed frame count do not match the whole clone export")
	}
	compositions := map[string]Object{}
	if raw, ok := timeline["compositions"]; ok {
		a := exportArray(raw)
		if a == nil {
			return nil, invalidExportCoverage("invalid saved compositions")
		}
		for _, raw := range a {
			c := exportObject(raw)
			id, ok := c["id"].(string)
			if !ok || !exportPositive(c["width"]) || !exportPositive(c["height"]) || !exportPositive(c["fps"]) || exportArray(c["tracks"]) == nil || exportArray(c["items"]) == nil {
				return nil, invalidExportCoverage("invalid saved composition")
			}
			if _, exists := compositions[id]; exists {
				return nil, invalidExportCoverage("ambiguous duplicate saved composition IDs")
			}
			compositions[id] = c
		}
	}
	expected := map[string]Object{}
	visited := 0
	var visit func(Object, Object, []any, map[string]bool) error
	visit = func(timeline, space Object, path []any, ancestors map[string]bool) error {
		renderItems, err := exportRenderCaptionItems(timeline, space, len(path) == 0)
		if err != nil {
			return invalidExportCoverage("saved caption sources cannot resolve rendered caption instances: " + err.Error())
		}
		ids := map[string]bool{}
		for _, item := range renderItems {
			visited++
			if visited > 100000 {
				return invalidExportCoverage("too many expanded composition items")
			}
			id := exportString(item["id"])
			if ids[id] {
				return invalidExportCoverage("ambiguous duplicate saved item IDs")
			}
			ids[id] = true
			kind := exportString(item["type"])
			if kind == "text" || kind == "subtitle" {
				if len(expected) >= 10000 {
					return invalidExportCoverage("too many caption instances")
				}
				expected[exportCaptionKey(path, id)] = space
			}
			if kind != "composition" {
				continue
			}
			compositionID := exportString(item["compositionId"])
			c := compositions[compositionID]
			if c == nil || ancestors[compositionID] || len(path) >= 32 {
				return invalidExportCoverage("missing, cyclic or too deeply nested saved composition")
			}
			nextAncestors := make(map[string]bool, len(ancestors)+1)
			for k, v := range ancestors {
				nextAncestors[k] = v
			}
			nextAncestors[compositionID] = true
			nextPath := append(append([]any{}, path...), id)
			if err = visit(c, Object{"width": item["compositionWidth"], "height": item["compositionHeight"], "fps": c["fps"]}, nextPath, nextAncestors); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(timeline, p.Metadata(), []any{}, map[string]bool{}); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, raw := range exportArray(coverage["entries"]) {
		entry := exportObject(raw)
		key := exportCaptionKey(exportArray(entry["instancePath"]), exportString(entry["itemId"]))
		if seen[key] {
			return nil, invalidExportCoverage("duplicate caption instance")
		}
		seen[key] = true
		space := expected[key]
		if space == nil {
			return nil, invalidExportCoverage("unknown caption item or wrapper path")
		}
		actual := exportObject(entry["space"])
		if exportNumber(actual["width"]) != exportNumber(space["width"]) || exportNumber(actual["height"]) != exportNumber(space["height"]) {
			return nil, invalidExportCoverage("composition space mismatch")
		}
		if bounds := exportObject(entry["bounds"]); bounds != nil {
			if exportNumber(bounds["x"])+exportNumber(bounds["width"]) > exportNumber(space["width"])+1e-6 || exportNumber(bounds["y"])+exportNumber(bounds["height"]) > exportNumber(space["height"])+1e-6 {
				return nil, invalidExportCoverage("painted bounds exceed their composition surface")
			}
		}
	}
	if len(seen) != len(expected) {
		return nil, invalidExportCoverage("missing caption instances; use explicit null bounds for observed empty captions")
	}
	return exportCopy(coverage)
}
func exportCaptionKey(path []any, id string) string {
	b, _ := json.Marshal([]any{path, id})
	return string(b)
}
func exportValidTimelineItem(item Object) bool {
	if item == nil {
		return false
	}
	for _, key := range []string{"id", "type", "trackId"} {
		if _, ok := item[key].(string); !ok {
			return false
		}
	}
	return exportFinite(item["from"]) && exportFinite(item["durationInFrames"]) && exportNumber(item["durationInFrames"]) >= 0
}
func exportOptionalNumber(o Object, key string, fallback float64) float64 {
	v, ok := o[key]
	if !ok {
		return fallback
	}
	return exportNumber(v)
}

func exportRenderCaptionItems(timeline, canvas Object, root bool) ([]Object, error) {
	if !exportPositive(canvas["width"]) || !exportPositive(canvas["height"]) || !exportPositive(canvas["fps"]) {
		return nil, fmt.Errorf("invalid canvas")
	}
	rawTracks := exportArray(timeline["tracks"])
	rawItems := exportArray(timeline["items"])
	if rawTracks == nil || rawItems == nil {
		return nil, fmt.Errorf("invalid timeline")
	}
	tracks := make([]Object, 0, len(rawTracks))
	groups := map[string]Object{}
	for _, raw := range rawTracks {
		track := exportObject(raw)
		if _, ok := track["id"].(string); !ok {
			return nil, fmt.Errorf("invalid track")
		}
		for _, key := range []string{"visible", "solo"} {
			if v, ok := track[key]; ok {
				if _, ok := v.(bool); !ok {
					return nil, fmt.Errorf("invalid track %s", key)
				}
			}
		}
		if v, ok := track["order"]; ok && !exportFinite(v) {
			return nil, fmt.Errorf("invalid track order")
		}
		if track["isGroup"] == true {
			groups[exportString(track["id"])] = track
		}
	}
	for _, raw := range rawTracks {
		track := exportObject(raw)
		if root && track["isGroup"] == true {
			continue
		}
		if track["id"] == "__virtual-transcript-captions__" {
			continue
		}
		copy := Object{}
		for k, v := range track {
			copy[k] = v
		}
		if root {
			if group := groups[exportString(track["parentTrackId"])]; group != nil {
				copy["visible"] = track["visible"] != false && group["visible"] != false
				copy["solo"] = track["solo"] == true || group["solo"] == true
			}
		}
		tracks = append(tracks, copy)
	}
	items := make([]Object, 0, len(rawItems))
	for _, raw := range rawItems {
		item := exportObject(raw)
		if err := exportValidateRenderItem(item); err != nil {
			return nil, err
		}
		if root && item["type"] == "video" && item["isReversed"] == true && item["reverseConformStatus"] == "ready" && exportString(item["reverseConformSrc"]) != "" {
			copy := Object{}
			for k, v := range item {
				copy[k] = v
			}
			delete(copy, "isReversed")
			local := exportOptionalNumber(item, "reverseConformLocalStart", 0)
			end := local + exportNumber(item["durationInFrames"])
			copy["speed"] = 1
			copy["sourceStart"] = local
			copy["sourceEnd"] = end
			copy["sourceDuration"] = end
			copy["sourceFps"] = canvas["fps"]
			item = copy
		}
		items = append(items, item)
	}
	laneItems := make(map[string][]Object, len(tracks))
	for _, track := range tracks {
		id := exportString(track["id"])
		for _, item := range items {
			if item["trackId"] == id {
				laneItems[id] = append(laneItems[id], item)
			}
		}
	}
	reversed := map[string]bool{}
	for _, lane := range laneItems {
		for _, item := range lane {
			if (item["type"] == "audio" || item["type"] == "video") && item["isReversed"] == true {
				reversed[exportString(item["id"])] = true
			}
		}
	}
	solo := false
	for _, track := range tracks {
		solo = solo || track["solo"] == true
	}
	out := []Object{}
	virtual := []Object{}
	for _, track := range tracks {
		for _, item := range laneItems[exportString(track["id"])] {
			source := exportObject(item["source"])
			captionSource := exportObject(item["captionSource"])
			if item["type"] == "text" && reversed[exportString(captionSource["clipId"])] {
				continue
			}
			if item["type"] == "subtitle" && (source["type"] == "transcript" || source["type"] == "embedded-subtitles") && reversed[exportString(source["clipId"])] {
				continue
			}
			out = append(out, item)
			if solo && track["solo"] != true || !solo && track["visible"] == false {
				continue
			}
			if (item["type"] != "audio" && item["type"] != "video") || exportString(item["mediaId"]) == "" || item["isReversed"] == true {
				continue
			}
			transcript := exportObject(item["transcriptCaptions"])
			if transcript["enabled"] != true || transcript["type"] != "transcript" {
				continue
			}
			fps := exportNumber(canvas["fps"])
			sourceFPS := exportOptionalNumber(item, "sourceFps", fps)
			speed := exportOptionalNumber(item, "speed", 1)
			start := exportOptionalNumber(item, "sourceStart", 0)
			end := exportOptionalNumber(item, "sourceEnd", start+math.Floor(exportNumber(item["durationInFrames"])*speed*sourceFPS/fps+0.5))
			for _, rawCue := range exportArray(transcript["cues"]) {
				cue := exportObject(rawCue)
				overlapStart := math.Max(exportNumber(cue["startSeconds"]), start/sourceFPS)
				overlapEnd := math.Min(exportNumber(cue["endSeconds"]), end/sourceFPS)
				if overlapEnd > overlapStart && math.Ceil((overlapEnd-start/sourceFPS)/speed*fps) > math.Floor((overlapStart-start/sourceFPS)/speed*fps) {
					virtual = append(virtual, Object{"id": "virtual-transcript-" + exportString(item["id"]), "type": "subtitle"})
					break
				}
			}
		}
	}
	return append(out, virtual...), nil
}

func exportValidateRenderItem(item Object) error {
	if !exportValidTimelineItem(item) {
		return fmt.Errorf("invalid item")
	}
	for _, key := range []string{"sourceStart", "sourceEnd", "sourceDuration", "trimStart", "trimEnd"} {
		if v, ok := item[key]; ok && !exportFinite(v) {
			return fmt.Errorf("invalid %s", key)
		}
	}
	for _, key := range []string{"sourceFps", "speed"} {
		if v, ok := item[key]; ok && !exportPositive(v) {
			return fmt.Errorf("invalid %s", key)
		}
	}
	if v, ok := item["isReversed"]; ok {
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("invalid reverse state")
		}
	}
	if v, ok := item["mediaId"]; ok {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("invalid media ID")
		}
	}
	if v, ok := item["compositionId"]; ok {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("invalid composition ID")
		}
	}
	if c := exportObject(item["transcriptCaptions"]); c != nil {
		if raw, ok := c["style"]; ok && exportObject(raw) == nil {
			return fmt.Errorf("invalid transcript caption style")
		}
	}
	source := exportObject(item["source"])
	if item["type"] == "subtitle" && source == nil {
		return fmt.Errorf("Subtitle requires a source")
	}
	if _, ok := item["source"]; ok {
		if source == nil {
			return fmt.Errorf("invalid subtitle source")
		}
		kind := exportString(source["type"])
		if kind != "transcript" && kind != "embedded-subtitles" && kind != "subtitle-import" {
			return fmt.Errorf("invalid subtitle source type")
		}
		if v, ok := source["clipId"]; ok {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("invalid subtitle clipId")
			}
		}
		if (kind == "transcript" || kind == "embedded-subtitles") && exportString(source["clipId"]) == "" {
			return fmt.Errorf("clip-bound subtitle requires clipId")
		}
	}
	if raw, ok := item["captionSource"]; ok {
		c := exportObject(raw)
		if c == nil {
			return fmt.Errorf("invalid generated caption source")
		}
		if v, ok := c["clipId"]; ok {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("invalid caption clipId")
			}
		}
	}
	if raw, ok := item["transcriptCaptions"]; ok {
		c := exportObject(raw)
		if c == nil || c["type"] != "transcript" {
			return fmt.Errorf("invalid transcript captions")
		}
		if _, ok := c["mediaId"].(string); !ok {
			return fmt.Errorf("invalid transcript media ID")
		}
		if _, ok := c["enabled"].(bool); !ok {
			return fmt.Errorf("invalid transcript enabled state")
		}
		cues := exportArray(c["cues"])
		if cues == nil {
			return fmt.Errorf("invalid transcript cues")
		}
		for _, raw := range cues {
			cue := exportObject(raw)
			if _, ok := cue["id"].(string); !ok {
				return fmt.Errorf("invalid cue ID")
			}
			if _, ok := cue["text"].(string); !ok {
				return fmt.Errorf("invalid cue text")
			}
			if !exportFinite(cue["startSeconds"]) || !exportFinite(cue["endSeconds"]) {
				return fmt.Errorf("invalid cue timing")
			}
		}
	}
	return nil
}

// CloneBeginPolicy and CloneCommitPolicy are pure decisions. Slow scoring,
// probing and template verification belong to the injected render adapter.
type CloneBeginPolicy struct {
	Session       Object
	Score         Object
	Fresh         bool
	Accepted      bool
	StaleReason   string
	Reasons       []string
	TemplateDrift []string
	Recovery      string
}

func EvaluateCloneBeginPolicy(input CloneBeginPolicy) (Object, error) {
	session, err := exportCopy(input.Session)
	if err != nil {
		return nil, err
	}
	if len(input.TemplateDrift) > 0 {
		return session, apiError("invalid_request", strings.Join(input.TemplateDrift, "; ")+". "+input.Recovery, Object{"cloneGate": "template_media_drift", "recovery": input.Recovery, "blockers": input.TemplateDrift, "session": session})
	}
	if input.Accepted {
		return session, nil
	}
	if input.Score == nil {
		return session, apiError("invalid_request", "Clone project cần đo Độ hoàn thiện trước khi xuất — bấm Đo độ hoàn thiện", Object{"cloneGate": "missing_score", "blockers": []string{"measure"}, "session": session})
	}
	if !input.Fresh {
		return session, apiError("invalid_request", "Điểm Độ hoàn thiện đã cũ: "+input.StaleReason, Object{"cloneGate": "stale_score", "blockers": []string{input.StaleReason}, "session": session})
	}
	reasons := clonePolicyScoreReasons(input.Score, input.Reasons)
	if len(reasons) > 0 {
		return session, apiError("invalid_request", fmt.Sprintf("Clone project chưa đủ Độ hoàn thiện để xuất (%v/100): %s", input.Score["total"], reasons[0]), Object{"cloneGate": "below_threshold", "total": input.Score["total"], "blockers": reasons, "session": session})
	}
	return session, nil
}

type CloneCommitPolicy struct {
	JobPresent      bool
	Accepted        bool
	Probed          Object
	Score           Object
	ProbeError      string
	DriftReasons    []string
	TemplateReasons []string
	ScoreReasons    []string
	TemplateError   string
}

func EvaluateCloneCommitPolicy(input CloneCommitPolicy) (Object, error) {
	reasons := append([]string{}, input.DriftReasons...)
	reasons = append(reasons, input.TemplateReasons...)
	if input.ProbeError != "" {
		reasons = append(reasons, "Không probe được file đã dựng: "+input.ProbeError)
	}
	if !input.JobPresent && len(reasons) == 0 {
		reasons = append(reasons, "Project không còn thuộc job clone")
	}
	if input.TemplateError != "" {
		reasons = append(reasons, input.TemplateError)
	}
	score := input.Score
	if len(reasons) > 0 {
		score = nil
	} else if !input.Accepted {
		reasons = append(reasons, clonePolicyScoreReasons(input.Score, input.ScoreReasons)...)
	}
	var probeError any
	if input.ProbeError != "" {
		probeError = input.ProbeError
	}
	diagnostics := Object{"probed": input.Probed, "score": score, "reasons": reasons, "probeError": probeError}
	if len(reasons) > 0 {
		return diagnostics, apiError("invalid_request", "Bản xuất chưa đạt kiểm tra Độ hoàn thiện: "+reasons[0], Object{"cloneGate": "final_check_failed", "blockers": reasons})
	}
	return diagnostics, nil
}

func clonePolicyScoreReasons(score Object, provided []string) []string {
	if len(provided) > 0 {
		return append([]string{}, provided...)
	}
	reasons := []string{}
	total := exportNumber(score["total"])
	for _, raw := range exportArray(score["blockers"]) {
		blocker := exportObject(raw)
		message := exportString(blocker["message"])
		if message == "" {
			message = exportString(blocker["id"])
		}
		reasons = append(reasons, message)
	}
	if !exportFinite(total) || total < 90 {
		reasons = append(reasons, fmt.Sprintf("Điểm %v/100 chưa đạt 90", score["total"]))
	}
	for _, raw := range exportArray(score["items"]) {
		item := exportObject(raw)
		if item["status"] == "unknown" {
			reasons = append(reasons, exportString(item["id"])+": chưa đo được")
		}
	}
	return reasons
}
