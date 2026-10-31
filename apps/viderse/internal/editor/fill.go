package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
)

// FillInput is the engine's template-fill boundary. Media contains canonical
// engine metadata; Backdrops contains canonical video items appended to the pool.
// Neither the snapshot nor any supplied object is modified.
type FillInput struct {
	Project        Project     `json:"project"`
	DurationFrames int         `json:"durationInFrames"`
	IDPrefix       string      `json:"idPrefix"`
	Seed           uint32      `json:"seed"`
	Scenes         []FillScene `json:"scenes"`
	Cues           []FillCue   `json:"cues"`
	Audio          FillAudio   `json:"audio"`
	Backdrops      []Object    `json:"backdrops,omitempty"`
	Media          []Object    `json:"media"`
	Envelope       Object      `json:"envelope"`
}
type SubjectBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type FillScene struct {
	SceneID    string      `json:"sceneId"`
	Title      string      `json:"title"`
	StartFrame int         `json:"startFrame"`
	EndFrame   int         `json:"endFrame"`
	MediaID    string      `json:"mediaId"`
	Src        string      `json:"src"`
	SubjectBox *SubjectBox `json:"subjectBox,omitempty"`
}
type FillCueWord struct {
	StartMs   float64 `json:"startMs"`
	EndMs     float64 `json:"endMs"`
	Text      string  `json:"text"`
	Estimated bool    `json:"estimated,omitempty"`
}
type FillCue struct {
	ID       string        `json:"id"`
	StartSec float64       `json:"startSec"`
	EndSec   float64       `json:"endSec"`
	Text     string        `json:"text"`
	Words    []FillCueWord `json:"words"`
}
type FillAudio struct {
	MediaID        string   `json:"mediaId"`
	Src            string   `json:"src"`
	DurationFrames int      `json:"durationInFrames"`
	MeasuredLUFS   *float64 `json:"measuredLufs,omitempty"`
}
type FillItemIDs struct {
	Scenes    []string `json:"scenes"`
	Captions  string   `json:"captions"`
	Waveform  string   `json:"waveform"`
	Subscribe []string `json:"subscribe"`
}
type FillCaptionSummary struct {
	CueCount       int  `json:"cueCount"`
	WordsEstimated bool `json:"wordsEstimated"`
}
type FillLoudness struct {
	TargetLUFS    float64 `json:"targetLufs"`
	AppliedGainDB float64 `json:"appliedGainDb"`
}
type FillResult struct {
	Project  Project            `json:"project"`
	Commands []Command          `json:"commands"`
	ItemIDs  FillItemIDs        `json:"itemIds"`
	Captions FillCaptionSummary `json:"captions"`
	Loudness *FillLoudness      `json:"loudness"`
}

func fillObject(v any) Object {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return Object{}
}
func (ids FillItemIDs) MarshalJSON() ([]byte, error) {
	var captions, waveform any
	if ids.Captions != "" {
		captions = ids.Captions
	}
	if ids.Waveform != "" {
		waveform = ids.Waveform
	}
	return json.Marshal(Object{"scenes": ids.Scenes, "captions": captions, "waveform": waveform, "subscribe": ids.Subscribe})
}
func fillObjects(v any) []Object {
	out := []Object{}
	switch a := v.(type) {
	case []Object:
		return a
	case []any:
		for _, x := range a {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}
func fillNum(m Object, k string, d float64) float64 {
	v, ok := m[k]
	if !ok || v == nil {
		return d
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint32:
		return float64(n)
	case json.Number:
		f, e := n.Float64()
		if e == nil {
			return f
		}
	}
	return math.NaN()
}
func fillString(m Object, k string) string { s, _ := m[k].(string); return s }
func fillBool(m Object, k string) bool     { b, _ := m[k].(bool); return b }
func fillCopy(m Object) Object {
	out := Object{}
	for k, v := range m {
		out[k] = fillCopyValue(v)
	}
	return out
}
func fillCopyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return fillCopy(x)
	case []Object:
		a := make([]Object, len(x))
		for i, m := range x {
			a[i] = fillCopy(m)
		}
		return a
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = fillCopyValue(e)
		}
		return a
	}
	return v
}
func fillMerge(a, b Object) Object {
	out := fillCopy(a)
	for k, v := range b {
		out[k] = fillCopyValue(v)
	}
	return out
}
func fillFinite(n float64) bool   { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func fillRound(n float64) float64 { return math.Floor(n + 0.5) }
func fillSource(frames, speed, fps, sourceFPS float64) float64 {
	return fillRound((frames / fps) * sourceFPS * speed)
}
func fillTimeline(frames, speed, sourceFPS, fps float64) float64 {
	return math.Floor((frames / sourceFPS * fps) / speed)
}

var fillIdentity = map[string]bool{"id": true, "trackId": true, "from": true, "durationInFrames": true, "label": true, "src": true, "audioSrc": true, "mediaId": true, "thumbnailUrl": true, "sourceStart": true, "sourceEnd": true, "sourceDuration": true, "sourceFps": true, "trimStart": true, "trimEnd": true, "offset": true, "speed": true, "isReversed": true, "linkedGroupId": true, "originId": true, "templateAnchor": true, "transformParent": true}

func fillPrototypeCopy(item Object, keep ...string) Object {
	out := Object{}
	for k, v := range item {
		skip := fillIdentity[k] || len(k) >= 14 && k[:14] == "reverseConform"
		for _, s := range keep {
			skip = skip || k == s
		}
		if !skip {
			out[k] = fillCopyValue(v)
		}
	}
	return out
}
func fillPrototype(items []Object, track, kind string) Object {
	var out Object
	for _, item := range items {
		if fillString(item, "trackId") == track && fillString(item, "type") == kind && (out == nil || fillNum(item, "from", 0) < fillNum(out, "from", 0)) {
			out = item
		}
	}
	return out
}
func fillSlot(tracks []Object, slot string) Object {
	for _, t := range tracks {
		if fillString(t, "templateSlot") == slot {
			return t
		}
	}
	return nil
}

// Fill ports buildTemplateFill and applies its real serialized engine commands.
func Fill(input FillInput) (FillResult, error) {
	result, err := BuildTemplateFill(input)
	if err != nil {
		return FillResult{}, err
	}
	project := Clone(input.Project)
	if len(input.Backdrops) > 0 {
		timeline := project.Timeline()
		items := fillObjects(timeline["items"])
		track := fillSlot(fillObjects(timeline["tracks"]), "background-video")
		if track == nil {
			return FillResult{}, fmt.Errorf("template has no background-video slot")
		}
		for _, b := range input.Backdrops {
			copy := fillCopy(b)
			copy["trackId"] = track["id"]
			items = append(items, copy)
		}
		timeline["items"] = items
	}
	result.Project, err = applyFillCommands(project, result.Commands, input.Media)
	if err != nil {
		return FillResult{}, err
	}
	return result, nil
}

// BuildTemplateFill returns the same command vocabulary and payloads as the
// canonical engine. It deliberately does not introduce a second timeline format.
func BuildTemplateFill(input FillInput) (FillResult, error) {
	D := float64(input.DurationFrames)
	meta := input.Project.Metadata()
	fps := fillNum(meta, "fps", 0)
	width := fillNum(meta, "width", 0)
	height := fillNum(meta, "height", 0)
	if input.IDPrefix == "" || D <= 0 || !fillFinite(fps) || fps <= 0 || width <= 0 || height <= 0 || fillNum(Object(input.Project), "schemaVersion", 0) != 15 {
		return FillResult{}, fmt.Errorf("Thông số đổ template không hợp lệ.")
	}
	if input.Audio.DurationFrames <= 0 || input.Audio.MediaID == "" || input.Audio.Src == "" {
		return FillResult{}, fmt.Errorf("Thông số âm thanh nguồn không hợp lệ.")
	}
	for _, scene := range input.Scenes {
		if scene.EndFrame <= scene.StartFrame || scene.StartFrame < 0 || scene.EndFrame > input.DurationFrames || scene.SceneID == "" || scene.MediaID == "" || scene.Src == "" {
			return FillResult{}, fmt.Errorf("Cảnh %s vượt ngoài thời lượng project.", scene.SceneID)
		}
	}
	for _, cue := range input.Cues {
		if !fillFinite(cue.StartSec) || !fillFinite(cue.EndSec) {
			return FillResult{}, fmt.Errorf("invalid caption timing")
		}
		for _, word := range cue.Words {
			if !fillFinite(word.StartMs) || !fillFinite(word.EndMs) {
				return FillResult{}, fmt.Errorf("invalid caption word timing")
			}
		}
	}
	timeline := input.Project.Timeline()
	tracks := fillObjects(timeline["tracks"])
	items := append([]Object{}, fillObjects(timeline["items"])...)
	transitions := fillObjects(timeline["transitions"])
	scenesTrack := fillSlot(tracks, "scenes")
	backgroundTrack := fillSlot(tracks, "background-video")
	captionsTrack := fillSlot(tracks, "captions")
	audioTrack := fillSlot(tracks, "source-audio")
	if len(input.Backdrops) > 0 && backgroundTrack == nil {
		return FillResult{}, fmt.Errorf("template has no background-video slot")
	}
	for _, b := range input.Backdrops {
		copy := fillCopy(b)
		copy["trackId"] = backgroundTrack["id"]
		items = append(items, copy)
	}
	recipe := fillObject(scenesTrack["templateScenes"])
	motion := fillObject(recipe["motion"])
	wander := fillString(motion, "kind") == "wander"
	opacity := fillNum(recipe, "opacity", 1)
	if !fillFinite(opacity) || opacity < 0 || opacity > 1 {
		return FillResult{}, fmt.Errorf("invalid scene opacity")
	}
	appearance, err := fillAppearance(backgroundTrack)
	if err != nil {
		return FillResult{}, err
	}
	result := FillResult{Commands: []Command{}, ItemIDs: FillItemIDs{Scenes: []string{}, Subscribe: []string{}}}
	commands := []Command{}
	add := func(kind string, params Object) { commands = append(commands, Command{Type: kind, Params: params}) }
	addItem := func(item Object) { add("item.add", Object{"item": item}) }
	for _, media := range input.Media {
		add("media.add", Object{"media": fillCopy(media)})
	}
	remove := []string{}
	slots := map[string]bool{}
	clearSlot := func(track Object) {
		if track == nil {
			return
		}
		id := fillString(track, "id")
		slots[id] = true
		for _, item := range items {
			if fillString(item, "trackId") == id {
				remove = append(remove, fillString(item, "id"))
			}
		}
	}
	if scenesTrack != nil {
		clearSlot(scenesTrack)
		prototype := fillPrototype(items, fillString(scenesTrack, "id"), "image")
		style := fillPrototypeCopy(prototype)
		transform := fillObject(style["transform"])
		if style["transform"] == nil {
			transform = Object{"x": 0, "y": 0, "width": width, "height": height}
		}
		for index, scene := range input.Scenes {
			id := input.IDPrefix + "-scene-" + scene.SceneID
			frames := scene.EndFrame - scene.StartFrame
			rect := fillCopy(transform)
			if fillString(recipe, "style") == "cutout-close" {
				if scene.SubjectBox == nil {
					return FillResult{}, fmt.Errorf("Cảnh %s thiếu khung nhân vật để phóng to.", scene.SceneID)
				}
				box := scene.SubjectBox
				if !fillFinite(box.X) || !fillFinite(box.Y) || !fillFinite(box.Width) || !fillFinite(box.Height) || box.X < 0 || box.Y < 0 || box.Width <= 0 || box.Height <= 0 || box.X+box.Width > 1.000001 || box.Y+box.Height > 1.000001 {
					return FillResult{}, fmt.Errorf("invalid subject box for scene %s", scene.SceneID)
				}
				bw, bh := fillNum(rect, "width", width), fillNum(rect, "height", height)
				scale := math.Min(2.2, math.Max(1.25, math.Min(.95*height/(.55*box.Height*bh), 1.15*width/(box.Width*bw))))
				scaledWidth, scaledHeight := bw*scale, bh*scale
				rect["width"], rect["height"] = scaledWidth, scaledHeight
				rect["x"] = fillNum(rect, "x", 0) + scaledWidth*(.5-(box.X+box.Width/2))
				rect["y"] = .04*height - box.Y*scaledHeight - height/2 + scaledHeight/2
			}
			// Merge copies nested objects: compose opacity before detaching the
			// scene transform, otherwise the emitted item retains prototype opacity.
			if opacity != 1 {
				rect["opacity"] = fillNum(rect, "opacity", 1) * opacity
			}
			item := fillMerge(style, Object{"id": id, "type": "image", "trackId": scenesTrack["id"], "from": scene.StartFrame, "durationInFrames": frames, "label": scene.Title, "mediaId": scene.MediaID, "src": scene.Src, "transform": rect})
			if scene.Title == "" {
				item["label"] = fmt.Sprintf("Cảnh %d", index+1)
			}
			if wander {
				intensity := fillNum(motion, "intensity", 1)
				speed := fillNum(motion, "speed", 1)
				if !fillFinite(intensity) || !fillFinite(speed) || speed <= 0 {
					return FillResult{}, fmt.Errorf("invalid wander motion")
				}
				item["motionModifiers"] = []Object{{"version": 2, "id": input.IDPrefix + "-mm-" + scene.SceneID, "type": "wander", "enabled": true, "amplitude": intensity, "frequency": .25 / speed, "phaseFrames": 0, "seed": fillSeedHash(input.Seed, uint32(index))}}
			}
			addItem(item)
			result.ItemIDs.Scenes = append(result.ItemIDs.Scenes, id)
			if !wander {
				fromScale, toScale := 1., 1.12
				if index%2 != 0 {
					fromScale, toScale = toScale, fromScale
				}
				for _, property := range []string{"width", "height"} {
					base := fillNum(rect, property, map[string]float64{"width": width, "height": height}[property])
					add("keyframe.add", Object{"itemId": id, "property": property, "frame": 0, "value": base * fromScale, "easing": "linear"})
					add("keyframe.add", Object{"itemId": id, "property": property, "frame": frames - 1, "value": base * toScale, "easing": "linear"})
				}
			}
		}
	}
	joins := []fillBackgroundJoin{}
	backgroundTransition := Object{"presentation": "fade", "timing": "ease-in-out", "alignment": .5, "durationInFrames": fps}
	if backgroundTrack != nil {
		clearSlot(backgroundTrack)
		pool := []Object{}
		poolIDs := map[string]bool{}
		groups := map[string]bool{}
		for _, item := range items {
			if fillString(item, "trackId") == fillString(backgroundTrack, "id") && fillString(item, "type") == "video" && fillNum(item, "durationInFrames", 0) > 0 {
				clip, e := fillRetimeBackground(item, appearance.Speed, fps)
				if e != nil {
					return FillResult{}, e
				}
				pool = append(pool, clip)
				poolIDs[fillString(item, "id")] = true
				if group := fillString(item, "linkedGroupId"); group != "" {
					groups[group] = true
				}
			}
		}
		sort.SliceStable(pool, func(i, j int) bool { return fillNum(pool[i], "from", 0) < fillNum(pool[j], "from", 0) })
		for _, t := range transitions {
			if fillString(t, "trackId") == fillString(backgroundTrack, "id") && poolIDs[fillString(t, "leftClipId")] && poolIDs[fillString(t, "rightClipId")] {
				backgroundTransition = t
				break
			}
		}
		for _, item := range items {
			if fillString(item, "trackId") != fillString(backgroundTrack, "id") && groups[fillString(item, "linkedGroupId")] {
				remove = append(remove, fillString(item, "id"))
			}
		}
		windows := make([]*fillWindow, len(pool))
		for i, clip := range pool {
			duration := fillNum(backgroundTransition, "durationInFrames", fps)
			if appearance.Speed != 1 {
				duration = math.Max(1, math.Min(duration, math.Floor(fillNum(clip, "durationInFrames", 0)*.4)))
			}
			head, tail := fillPortions(duration, fillNum(backgroundTransition, "alignment", .5))
			windows[i] = fillJoinWindow(clip, head, tail, fps)
		}
		random := fillRandom(input.Seed)
		previousIndex := -1
		var previous Object
		cursor := 0.
		index := 0
		for cursor < D && len(pool) > 0 {
			order := make([]int, len(pool))
			for i := range order {
				order[i] = i
			}
			for i := len(order) - 1; i > 0; i-- {
				j := int(math.Floor(random() * float64(i+1)))
				order[i], order[j] = order[j], order[i]
			}
			if len(pool) > 1 && order[0] == previousIndex {
				order[0], order[1] = order[1], order[0]
			}
			for _, poolIndex := range order {
				if cursor >= D {
					break
				}
				clip := pool[poolIndex]
				window := windows[poolIndex]
				frames := fillNum(clip, "durationInFrames", 0)
				if window != nil {
					frames = window.Frames
				}
				frames = math.Min(frames, D-cursor)
				output := fillCopy(clip)
				if window != nil {
					output["sourceStart"] = window.Start
					used := fillSource(frames, window.Speed, fps, window.FPS)
					if appearance.Speed != 1 {
						used = math.Max(1, math.Ceil(frames*window.Speed*window.FPS/fps))
					}
					output["sourceEnd"] = window.Start + used
				} else if appearance.Speed != 1 && !fillBool(clip, "loop") {
					output["sourceEnd"] = math.Min(fillNum(clip, "sourceEnd", 0), fillNum(clip, "sourceStart", 0)+math.Max(1, math.Ceil(frames*fillNum(clip, "speed", 1)*fillNum(clip, "sourceFps", fps)/fps)))
				}
				id := fmt.Sprintf("%s-bg-%d", input.IDPrefix, index)
				output["id"] = id
				output["trackId"] = backgroundTrack["id"]
				output["from"] = cursor
				output["durationInFrames"] = frames
				for _, key := range []string{"originId", "linkedGroupId", "templateAnchor", "transformParent"} {
					delete(output, key)
				}
				output["embeddedAudioMuted"] = true
				output = fillApplyAppearance(output, appearance, width, height)
				addItem(output)
				if previous != nil {
					maxDuration := math.Inf(1)
					if appearance.Speed != 1 {
						maxDuration = fillMaxTransition(previous, output, fillNum(backgroundTransition, "alignment", .5), fps)
					}
					joins = append(joins, fillBackgroundJoin{Left: fillString(previous, "id"), Right: id, LeftFrames: fillNum(previous, "durationInFrames", 0), RightFrames: frames, Max: maxDuration})
				}
				previous = output
				previousIndex = poolIndex
				cursor += frames
				index++
			}
		}
	}
	captions := PrepareCaptionData(input.Cues, fps, input.DurationFrames)
	if captionsTrack != nil {
		clearSlot(captionsTrack)
		if len(captions.Cues) > 0 {
			prototype := fillPrototype(items, fillString(captionsTrack, "id"), "subtitle")
			id := input.IDPrefix + "-captions"
			source := Object{"type": "subtitle-import", "fileName": "captions.json", "format": "vtt", "importedAt": 0}
			style := Object{}
			if prototype != nil {
				style = fillPrototypeCopy(prototype, "type", "cues", "words", "wordsEstimated", "source", "sourceLabel")
				if s := fillObject(prototype["source"]); fillString(s, "type") == "subtitle-import" {
					source = fillCopy(s)
				}
			} else {
				preset := "karaoke"
				if captions.Estimated {
					preset = "fade"
				}
				style = Object{"captionPreset": preset, "color": "#ffffff", "fontSize": height * .055, "textAlign": "center", "verticalAlign": "middle", "backgroundColor": "#111827", "transform": Object{"x": 0, "y": height * .3, "width": width * .85, "height": height * .2}}
			}
			style["source"] = source
			label := "Phụ đề"
			if prototype != nil {
				label = fillString(prototype, "label")
			}
			addItem(fillMerge(style, Object{"id": id, "type": "subtitle", "trackId": captionsTrack["id"], "from": 0, "durationInFrames": input.DurationFrames, "label": label, "cues": captions.Cues, "words": captions.Words, "wordsEstimated": captions.Estimated}))
			result.ItemIDs.Captions = id
			add("caption.importTimed", Object{"itemId": id, "data": Object{"words": captions.Words, "estimated": captions.Estimated, "cues": captions.Cues, "durationInFrames": input.DurationFrames}})
		}
	}
	audioProto := fillPrototype(items, fillString(audioTrack, "id"), "audio")
	audioSpan := math.Max(1, math.Min(float64(input.Audio.DurationFrames), D))
	if audioTrack != nil {
		clearSlot(audioTrack)
		style := Object{"volume": 0}
		if audioProto != nil {
			style = fillPrototypeCopy(audioProto, "type", "offset", "waveformData")
		}
		if audioTrack["templateLoudnessLufs"] != nil && input.Audio.MeasuredLUFS != nil && fillFinite(*input.Audio.MeasuredLUFS) {
			target := fillNum(audioTrack, "templateLoudnessLufs", 0)
			gain := math.Max(-12, math.Min(12, target-*input.Audio.MeasuredLUFS))
			style["volume"] = fillNum(audioProto, "volume", 0) + gain
			result.Loudness = &FillLoudness{TargetLUFS: target, AppliedGainDB: gain}
		}
		label := "Âm thanh gốc"
		if audioProto != nil {
			label = fillString(audioProto, "label")
		}
		addItem(fillMerge(style, Object{"id": input.IDPrefix + "-audio", "type": "audio", "trackId": audioTrack["id"], "from": 0, "durationInFrames": audioSpan, "label": label, "mediaId": input.Audio.MediaID, "src": input.Audio.Src, "sourceStart": 0, "sourceEnd": input.Audio.DurationFrames, "sourceDuration": input.Audio.DurationFrames, "sourceFps": fps}))
	}
	wavebound := map[string]bool{}
	if mediaID := fillString(audioProto, "mediaId"); mediaID != "" {
		for _, item := range items {
			if fillString(item, "type") == "shape" && item["waveformVisualizer"] != nil && fillString(item, "mediaId") == mediaID {
				id := fillString(item, "id")
				wavebound[id] = true
				add("waveform.setEnvelope", Object{"itemId": id, "mediaId": input.Audio.MediaID, "envelope": fillCopy(input.Envelope)})
				add("item.update", Object{"id": id, "updates": Object{"from": 0, "durationInFrames": audioSpan}})
				if result.ItemIDs.Waveform == "" {
					result.ItemIDs.Waveform = id
				}
			}
		}
	}
	T := 0.
	for _, item := range items {
		T = math.Max(T, fillNum(item, "from", 0)+fillNum(item, "durationInFrames", 0))
	}
	removed := map[string]bool{}
	for _, id := range remove {
		removed[id] = true
	}
	drop := []string{}
	dropped := map[string]bool{}
	for _, item := range items {
		id := fillString(item, "id")
		if slots[fillString(item, "trackId")] || removed[id] || wavebound[id] {
			continue
		}
		from := fillNum(item, "from", 0)
		frames := fillNum(item, "durationInFrames", 0)
		switch fillString(item, "templateAnchor") {
		case "full":
			updates := Object{"from": 0, "durationInFrames": input.DurationFrames}
			if fillString(item, "type") == "video" {
				updates["loop"] = true
			}
			add("item.update", Object{"id": id, "updates": updates})
		case "end":
			from = D - (T - from)
			if from < 0 {
				drop = append(drop, id)
				dropped[id] = true
			} else {
				add("item.update", Object{"id": id, "updates": Object{"from": from}})
			}
		default:
			if from >= D {
				drop = append(drop, id)
				dropped[id] = true
			} else if from+frames > D {
				add("item.update", Object{"id": id, "updates": Object{"durationInFrames": D - from}})
			}
		}
	}
	subscribe := regexp.MustCompile(`(?i)subscribe|đăng ký`)
	subscribeGroups := map[string]bool{}
	for _, item := range items {
		if fillString(item, "type") == "text" && fillString(item, "linkedGroupId") != "" && subscribe.MatchString(fillString(item, "text")) {
			subscribeGroups[fillString(item, "linkedGroupId")] = true
		}
	}
	for _, item := range items {
		if subscribeGroups[fillString(item, "linkedGroupId")] && !dropped[fillString(item, "id")] && !slots[fillString(item, "trackId")] {
			result.ItemIDs.Subscribe = append(result.ItemIDs.Subscribe, fillString(item, "id"))
		}
	}
	transitionCommand := func(left, right string, maximum float64, prototype Object) {
		if maximum < 1 {
			return
		}
		params := Object{"leftClipId": left, "rightClipId": right, "presentation": prototype["presentation"], "timing": prototype["timing"], "alignment": fillNum(prototype, "alignment", .5), "durationInFrames": math.Min(fillNum(prototype, "durationInFrames", fps), maximum)}
		if v, ok := prototype["direction"]; ok {
			params["direction"] = v
		}
		add("transition.add", params)
	}
	if scenesTrack != nil {
		ids := map[string]bool{}
		for _, item := range items {
			if fillString(item, "trackId") == fillString(scenesTrack, "id") && fillString(item, "type") == "image" {
				ids[fillString(item, "id")] = true
			}
		}
		for _, t := range transitions {
			if fillString(t, "trackId") == fillString(scenesTrack, "id") && ids[fillString(t, "leftClipId")] && ids[fillString(t, "rightClipId")] {
				for i := 0; i+1 < len(input.Scenes); i++ {
					a, b := input.Scenes[i], input.Scenes[i+1]
					if a.EndFrame == b.StartFrame {
						transitionCommand(result.ItemIDs.Scenes[i], result.ItemIDs.Scenes[i+1], math.Floor(float64(min(a.EndFrame-a.StartFrame, b.EndFrame-b.StartFrame))*.4), t)
					}
				}
				break
			}
		}
	}
	for _, join := range joins {
		transitionCommand(join.Left, join.Right, math.Min(math.Floor(math.Min(join.LeftFrames, join.RightFrames)*.4), join.Max), backgroundTransition)
	}
	leading := []Command{}
	for _, track := range tracks {
		if fillBool(track, "locked") {
			leading = append(leading, Command{Type: "track.update", Params: Object{"id": track["id"], "updates": Object{"locked": false}}})
		}
	}
	if len(remove)+len(drop) > 0 {
		leading = append(leading, Command{Type: "item.removeExact", Params: Object{"itemIds": append(remove, drop...)}})
	}
	commands = append(leading, commands...)
	for _, track := range tracks {
		if fillBool(track, "locked") {
			commands = append(commands, Command{Type: "track.update", Params: Object{"id": track["id"], "updates": Object{"locked": true}}})
		}
	}
	result.Commands = commands
	result.Captions = FillCaptionSummary{CueCount: len(captions.Cues), WordsEstimated: captions.Estimated}
	for _, page := range captions.Pages {
		result.Captions.WordsEstimated = result.Captions.WordsEstimated || page.Estimated
	}
	return result, nil
}

func fillSeedHash(seed, index uint32) uint32 {
	h := seed ^ 0x9e3779b9
	h = (h ^ (index + 1)) * 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}
func fillRandom(seed uint32) func() float64 {
	a := seed + 0x9e3779b9
	return func() float64 {
		a += 0x6d2b79f5
		t := (a ^ (a >> 15)) * (1 | a)
		t = (t + ((t ^ (t >> 7)) * (61 | t))) ^ t
		return float64(t^(t>>14)) / 4294967296
	}
}
