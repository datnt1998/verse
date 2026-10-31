package editor

import (
	"encoding/base64"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// Command uses the engine's serialized {type, params} wire contract. This
// applicator supports the commands emitted by BuildTemplateFill; an unknown
// command fails atomically rather than being ignored.
type Command struct {
	Type   string `json:"type"`
	Params Object `json:"params"`
}

// ApplyCommands applies a fill command batch to a detached project. It retains
// unknown authored fields, grouped Animation Core v2 lanes, and media references.
func ApplyCommands(project Project, commands []Command) (Project, error) {
	return applyFillCommands(project, commands, nil)
}

func applyFillCommands(project Project, commands []Command, media []Object) (Project, error) {
	if fillNum(Object(project), "schemaVersion", 0) != 15 {
		return nil, fmt.Errorf("unsupported project schema")
	}
	p := Clone(project)
	if p == nil {
		return nil, fmt.Errorf("project is not JSON serializable")
	}
	timeline := p.Timeline()
	if timeline == nil {
		timeline = Object{}
		p["timeline"] = timeline
	}
	fps := fillNum(p.Metadata(), "fps", 30)
	if !fillFinite(fps) || fps <= 0 {
		return nil, fmt.Errorf("invalid project fps")
	}
	items := fillObjects(timeline["items"])
	tracks := fillObjects(timeline["tracks"])
	transitions := fillObjects(timeline["transitions"])
	keyframes := fillObjects(timeline["keyframes"])
	records := fillObjects(p["mediaItems"])
	knownMedia := map[string]Object{}
	for _, m := range records {
		knownMedia[fillString(m, "id")] = m
	}
	for _, m := range media {
		knownMedia[fillString(m, "id")] = m
	}
	used := map[string]bool{}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if id := fillString(x, "id"); id != "" {
				used[id] = true
			}
			for _, child := range x {
				collect(child)
			}
		case []any:
			for _, child := range x {
				collect(child)
			}
		case []Object:
			for _, child := range x {
				collect(child)
			}
		}
	}
	collect(Object(p))
	serial := 0
	mint := func() string {
		for {
			serial++
			id := fmt.Sprintf("editor-generated-%d", serial)
			if !used[id] {
				used[id] = true
				return id
			}
		}
	}
	find := func(objects []Object, id string) Object {
		for _, o := range objects {
			if fillString(o, "id") == id {
				return o
			}
		}
		return nil
	}
	for index, command := range commands {
		params := command.Params
		fail := func(message string) (Project, error) {
			return nil, fmt.Errorf("command %d (%s): %s", index, command.Type, message)
		}
		switch command.Type {
		case "track.update":
			track := find(tracks, fillString(params, "id"))
			if track == nil {
				return fail("track not found")
			}
			updates := fillObject(params["updates"])
			for k, v := range updates {
				track[k] = fillCopyValue(v)
			}
		case "item.removeExact":
			ids := map[string]bool{}
			switch a := params["itemIds"].(type) {
			case []string:
				for _, id := range a {
					ids[id] = true
				}
			case []any:
				for _, v := range a {
					if s, ok := v.(string); ok {
						ids[s] = true
					}
				}
			}
			found := false
			kept := []Object{}
			for _, item := range items {
				if ids[fillString(item, "id")] {
					found = true
				} else {
					kept = append(kept, item)
				}
			}
			if !found {
				return fail("no items to remove")
			}
			items = kept
			kept = []Object{}
			for _, t := range transitions {
				if !ids[fillString(t, "leftClipId")] && !ids[fillString(t, "rightClipId")] {
					kept = append(kept, t)
				}
			}
			transitions = kept
			kept = []Object{}
			for _, k := range keyframes {
				if !ids[fillString(k, "itemId")] {
					kept = append(kept, k)
				}
			}
			keyframes = kept
			tracks = fillPruneTracks(tracks, items)
		case "media.add":
			m := fillCopy(fillObject(params["media"]))
			id := fillString(m, "id")
			if err := fillValidateMedia(m); err != nil {
				return fail(err.Error())
			}
			knownMedia[id] = m
			exists := false
			for i, r := range records {
				if fillString(r, "id") == id {
					records[i] = m
					exists = true
					break
				}
			}
			if !exists {
				records = append([]Object{m}, records...)
			}
		case "item.add":
			item := fillCopy(fillObject(params["item"]))
			if item["id"] == nil {
				item["id"] = mint()
			}
			id := fillString(item, "id")
			if id == "" || find(items, id) != nil {
				return fail("duplicate or empty item id")
			}
			track := find(tracks, fillString(item, "trackId"))
			if track == nil {
				return fail("item track not found")
			}
			if fillBool(track, "locked") {
				return fail("item track is locked")
			}
			if err := fillValidateItem(item); err != nil {
				return fail(err.Error())
			}
			// The engine's item placement advances an overlapping incoming clip to
			// the first free span. Fill slots normally already tile without overlap.
			from := fillNum(item, "from", 0)
			duration := fillNum(item, "durationInFrames", 0)
			occupied := []Object{}
			for _, other := range items {
				if other["trackId"] == item["trackId"] {
					occupied = append(occupied, other)
				}
			}
			sort.SliceStable(occupied, func(i, j int) bool { return fillNum(occupied[i], "from", 0) < fillNum(occupied[j], "from", 0) })
			for _, other := range occupied {
				start := fillNum(other, "from", 0)
				end := start + fillNum(other, "durationInFrames", 0)
				if from < end && from+duration > start {
					from = end
				}
			}
			item["from"] = from
			items = append(items, item)
			fillNormalizeItem(item)
		case "item.update":
			id := fillString(params, "id")
			item := find(items, id)
			if item == nil {
				return fail("item not found")
			}
			updates := fillObject(params["updates"])
			next := fillMerge(item, updates)
			if clear, ok := params["clearFields"].([]any); ok {
				for _, field := range clear {
					if key, ok := field.(string); ok {
						delete(next, key)
					}
				}
			}
			if err := fillValidateItem(next); err != nil {
				return fail(err.Error())
			}
			for k := range item {
				delete(item, k)
			}
			for k, v := range next {
				item[k] = v
			}
			if updates["from"] != nil || updates["durationInFrames"] != nil || updates["trackId"] != nil {
				transitions = fillRepairTransitions(transitions, items, id, fps)
			}
			fillNormalizeItem(item)
		case "keyframe.add":
			itemID := fillString(params, "itemId")
			item := find(items, itemID)
			if item == nil {
				return fail("keyframe item not found")
			}
			property := fillString(params, "property")
			frame := fillNum(params, "frame", math.NaN())
			value := fillNum(params, "value", math.NaN())
			if property == "" || !fillFinite(frame) || frame < 0 || frame != math.Floor(frame) || !fillFinite(value) {
				return fail("invalid keyframe")
			}
			blocked := false
			absolute := fillNum(item, "from", 0) + frame
			for _, t := range transitions {
				left := find(items, fillString(t, "leftClipId"))
				right := find(items, fillString(t, "rightClipId"))
				if left == nil || right == nil {
					continue
				}
				if fillString(t, "leftClipId") != itemID && fillString(t, "rightClipId") != itemID {
					continue
				}
				head, tail := fillPortions(fillNum(t, "durationInFrames", 1), fillNum(t, "alignment", .5))
				cut := fillNum(right, "from", 0)
				if absolute >= cut-head && absolute < cut+tail {
					blocked = true
				}
			}
			if blocked {
				continue
			}
			id := mint()
			var group Object
			for _, k := range keyframes {
				if fillString(k, "itemId") == itemID {
					group = k
					break
				}
			}
			if group == nil {
				group = Object{"itemId": itemID, "animationVersion": 2, "properties": []Object{}}
				keyframes = append(keyframes, group)
			}
			group["animationVersion"] = 2
			properties := fillObjects(group["properties"])
			var lane Object
			for _, l := range properties {
				if fillString(l, "property") == property {
					lane = l
					break
				}
			}
			if lane == nil {
				lane = Object{"property": property, "keyframes": []Object{}}
				properties = append(properties, lane)
			}
			keys := fillObjects(lane["keyframes"])
			key := fillMerge(params, Object{"id": id, "easing": "linear"})
			delete(key, "itemId")
			delete(key, "property")
			if e, ok := params["easing"]; ok {
				key["easing"] = e
			}
			replaced := false
			for _, old := range keys {
				if fillNum(old, "frame", -1) == frame {
					for k, v := range key {
						if k != "id" {
							old[k] = v
						}
					}
					replaced = true
					break
				}
			}
			if !replaced {
				keys = append(keys, key)
			}
			sort.SliceStable(keys, func(i, j int) bool { return fillNum(keys[i], "frame", 0) < fillNum(keys[j], "frame", 0) })
			lane["keyframes"] = keys
			group["properties"] = properties
		case "caption.importTimed":
			item := find(items, fillString(params, "itemId"))
			if item == nil || (fillString(item, "type") != "subtitle" && fillString(item, "type") != "text") {
				return fail("caption target not found")
			}
			data := fillObject(params["data"])
			words := fillObjects(data["words"])
			if !fillValidCaptionWords(words, fillNum(item, "durationInFrames", 0)) {
				return fail("caption words outside item or out of order")
			}
			item["words"] = fillCopyValue(words)
			item["wordsEstimated"] = fillBool(data, "estimated")
			cues := fillObjects(data["cues"])
			if fillString(item, "type") == "subtitle" {
				item["cues"] = fillCopyValue(cues)
			} else if len(cues) == 0 {
				texts := []string{}
				for _, w := range words {
					texts = append(texts, fillString(w, "text"))
				}
				item["text"] = strings.Join(texts, " ")
			} else {
				source := fillObject(item["captionSource"])
				for _, key := range []string{"text", "textSpans", "spanLayout", "textLayoutDrafts", "textStylePresetId", "textStyleScale", "textMotion", "textRole", "captionSource"} {
					delete(item, key)
				}
				item["type"] = "subtitle"
				item["cues"] = fillCopyValue(cues)
				name := fillString(source, "fileName")
				if name == "" {
					name = fillString(item, "label")
				}
				format := fillString(source, "format")
				if format == "" {
					format = "vtt"
					if strings.HasSuffix(strings.ToLower(fillString(item, "label")), ".srt") {
						format = "srt"
					}
				}
				item["source"] = Object{"type": "subtitle-import", "fileName": name, "format": format, "importedAt": fillNum(source, "importedAt", 0)}
			}
		case "waveform.setEnvelope":
			item := find(items, fillString(params, "itemId"))
			if item == nil || fillString(item, "type") != "shape" || item["waveformVisualizer"] == nil {
				return fail("waveform target not found")
			}
			envelope := fillObject(params["envelope"])
			if err := fillValidateEnvelope(envelope); err != nil {
				return fail(err.Error())
			}
			mediaID := fillString(params, "mediaId")
			if mediaID == "" {
				mediaID = fillString(item, "mediaId")
			}
			record := knownMedia[mediaID]
			if record == nil || !strings.HasPrefix(fillString(record, "mimeType"), "audio/") || fillString(record, "contentHash") != fillString(envelope, "sourceHash") {
				return fail("waveform audio source hash mismatch")
			}
			item["mediaId"] = mediaID
			item["sourceDuration"] = fillNum(envelope, "durationSeconds", 0) * fillNum(item, "sourceFps", fps)
			item["waveformVisualizer"] = fillMerge(fillObject(item["waveformVisualizer"]), envelope)
		case "transition.add":
			left := find(items, fillString(params, "leftClipId"))
			right := find(items, fillString(params, "rightClipId"))
			if left == nil || right == nil {
				return fail("transition clips not found")
			}
			alignment := fillNum(params, "alignment", .5)
			duration := math.Max(1, math.Min(fillRound(fillNum(params, "durationInFrames", 15)), math.Floor(math.Min(fillNum(left, "durationInFrames", 0), fillNum(right, "durationInFrames", 0))-1)))
			adjacent := math.Abs(fillNum(left, "from", 0)+fillNum(left, "durationInFrames", 0)-fillNum(right, "from", 0)) <= 1
			if adjacent {
				duration = math.Min(duration, fillMaxTransition(left, right, alignment, fps))
			}
			if duration < 1 || !fillCanTransition(left, right, duration, alignment, fps) {
				continue
			}
			duplicate := false
			for _, t := range transitions {
				if t["leftClipId"] == left["id"] && t["rightClipId"] == right["id"] {
					duplicate = true
				}
			}
			if duplicate {
				continue
			}
			t := Object{"id": mint(), "leftClipId": left["id"], "rightClipId": right["id"], "trackId": left["trackId"], "type": "crossfade", "durationInFrames": duration, "presentation": "fade", "timing": "linear", "alignment": alignment}
			for _, key := range []string{"presentation", "direction", "timing"} {
				if params[key] != nil {
					t[key] = params[key]
				}
			}
			transitions = append(transitions, t)
		default:
			return fail("unsupported command")
		}
	}
	timeline["items"] = items
	timeline["tracks"] = tracks
	timeline["transitions"] = transitions
	timeline["keyframes"] = keyframes
	for _, key := range []string{"markers", "compositions", "topLevelSequenceIds"} {
		if timeline[key] == nil {
			timeline[key] = []any{}
		}
	}
	for _, key := range []string{"inPoint", "outPoint"} {
		if timeline[key] == nil {
			delete(timeline, key)
		}
	}
	for _, key := range []string{"currentFrame", "scrollPosition", "masterBusDb"} {
		if timeline[key] == nil {
			timeline[key] = 0
		}
	}
	refs := []Object{}
	for _, record := range records {
		refs = append(refs, Object{"id": record["id"]})
	}
	p["mediaItems"] = refs
	end := 0.
	for _, item := range items {
		end = math.Max(end, fillNum(item, "from", 0)+fillNum(item, "durationInFrames", 0))
	}
	p["duration"] = end / fps
	return p, nil
}

func fillValidateItem(item Object) error {
	if fillString(item, "type") == "" || fillString(item, "trackId") == "" || item["label"] == nil {
		return fmt.Errorf("invalid timeline item identity")
	}
	from := fillNum(item, "from", math.NaN())
	duration := fillNum(item, "durationInFrames", math.NaN())
	if !fillFinite(from) || !fillFinite(duration) || duration <= 0 {
		return fmt.Errorf("invalid timeline item span")
	}
	if fillBool(item, "loop") {
		if fillString(item, "type") != "video" || !fillBool(item, "embeddedAudioMuted") || fillNum(item, "sourceEnd", fillNum(item, "sourceDuration", 0)) <= fillNum(item, "sourceStart", fillNum(item, "trimStart", fillNum(item, "offset", 0))) {
			return fmt.Errorf("loop requires muted video and measured source window")
		}
	}
	return nil
}
func fillValidateMedia(m Object) error {
	if fillString(m, "id") == "" || fillString(m, "storageType") != "workspace" || fillString(m, "mimeType") == "" {
		return fmt.Errorf("invalid canonical media metadata")
	}
	for _, key := range []string{"fileSize", "duration", "width", "height", "fps", "bitrate", "createdAt", "updatedAt"} {
		if !fillFinite(fillNum(m, key, math.NaN())) {
			return fmt.Errorf("invalid media %s", key)
		}
	}
	if fillNum(m, "fileSize", -1) < 0 {
		return fmt.Errorf("negative media size")
	}
	for _, key := range []string{"fileName", "codec"} {
		if _, ok := m[key].(string); !ok {
			return fmt.Errorf("invalid media %s", key)
		}
	}
	return nil
}

var fillHashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func fillValidateEnvelope(envelope Object) error {
	duration := fillNum(envelope, "durationSeconds", 0)
	norm := fillNum(envelope, "normalization", math.NaN())
	if len(envelope) != 5 || !fillHashPattern.MatchString(fillString(envelope, "sourceHash")) || fillNum(envelope, "samplesPerSecond", 0) != 50 || !fillFinite(duration) || duration <= 0 || duration > 86400 || !fillFinite(norm) || norm < 0 {
		return fmt.Errorf("invalid waveform envelope")
	}
	encoded := fillString(envelope, "envelopeBase64")
	if len(encoded) > 5760000 {
		return fmt.Errorf("waveform envelope too large")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) != int(math.Ceil(duration*50-1e-8)) {
		return fmt.Errorf("waveform sample count does not match duration")
	}
	return nil
}
func fillCanTransition(left, right Object, duration, alignment, fps float64) bool {
	if left["trackId"] != right["trackId"] || !fillVisualTransition(left) || !fillVisualTransition(right) || duration >= fillNum(left, "durationInFrames", 0) || duration >= fillNum(right, "durationInFrames", 0) {
		return false
	}
	end := fillNum(left, "from", 0) + fillNum(left, "durationInFrames", 0)
	start := fillNum(right, "from", 0)
	if math.Abs(end-start) <= 1 {
		head, tail := fillPortions(duration, alignment)
		return tail <= fillAvailableHandle(left, false, fps) && head <= fillAvailableHandle(right, true, fps)
	}
	return start < end-1
}
func fillVisualTransition(item Object) bool {
	switch fillString(item, "type") {
	case "video", "image", "composition":
		return true
	}
	return false
}
func fillRepairTransitions(transitions, items []Object, changed string, fps float64) []Object {
	valid, repaired := []Object{}, []Object{}
	lookup := map[string]Object{}
	for _, item := range items {
		lookup[fillString(item, "id")] = item
	}
	for _, t := range transitions {
		if fillString(t, "leftClipId") != changed && fillString(t, "rightClipId") != changed {
			valid = append(valid, t)
			continue
		}
		left, right := lookup[fillString(t, "leftClipId")], lookup[fillString(t, "rightClipId")]
		if left == nil || right == nil || left["trackId"] != right["trackId"] || !fillVisualTransition(left) || !fillVisualTransition(right) {
			continue
		}
		end := fillNum(left, "from", 0) + fillNum(left, "durationInFrames", 0)
		start := fillNum(right, "from", 0)
		duration := fillNum(t, "durationInFrames", 1)
		maximum := duration
		if math.Abs(end-start) <= 1 {
			maximum = fillMaxTransition(left, right, fillNum(t, "alignment", .5), fps)
			if maximum <= 0 {
				continue
			}
		} else if start < end-1 {
			if end-start < duration {
				maximum = math.Max(2, math.Floor(end-start))
				if maximum >= fillNum(left, "durationInFrames", 0) || maximum >= fillNum(right, "durationInFrames", 0) {
					continue
				}
			} else if duration >= fillNum(left, "durationInFrames", 0) || duration >= fillNum(right, "durationInFrames", 0) {
				maximum = math.Max(2, math.Min(fillNum(left, "durationInFrames", 0), fillNum(right, "durationInFrames", 0))-1)
			}
		} else {
			continue
		}
		if duration > maximum {
			repaired = append(repaired, fillMerge(t, Object{"durationInFrames": maximum}))
		} else {
			valid = append(valid, t)
		}
	}
	return append(valid, repaired...)
}
