package editor

import (
	"fmt"
	"math"
)

type fillBackgroundAppearance struct {
	Opacity, Blur, Brightness, Contrast, Saturation, Hue, Speed, Zoom float64
	FlipX, FlipY                                                      bool
}
type fillBackgroundJoin struct {
	Left, Right                  string
	LeftFrames, RightFrames, Max float64
}
type fillWindow struct{ Start, Frames, Speed, FPS float64 }

func fillAppearance(track Object) (fillBackgroundAppearance, error) {
	a := fillBackgroundAppearance{Opacity: 1, Contrast: 1, Saturation: 1, Speed: 1, Zoom: 1}
	if track["templateBackgroundAppearance"] == nil {
		return a, nil
	}
	m := fillObject(track["templateBackgroundAppearance"])
	fields := []struct {
		Key      string
		Value    *float64
		Min, Max float64
	}{{"opacity", &a.Opacity, 0, 1}, {"blurRadius", &a.Blur, 0, 50}, {"brightness", &a.Brightness, -1, 1}, {"contrast", &a.Contrast, 0, 3}, {"saturation", &a.Saturation, 0, 3}, {"hueShift", &a.Hue, 0, 1}, {"speed", &a.Speed, .25, 3}, {"zoom", &a.Zoom, 1, 3}}
	for _, f := range fields {
		v := fillNum(m, f.Key, math.NaN())
		if !fillFinite(v) || v < f.Min || v > f.Max {
			return a, fmt.Errorf("invalid background appearance: %s", f.Key)
		}
		*f.Value = v
	}
	var ok bool
	a.FlipX, ok = m["flipX"].(bool)
	if !ok {
		return a, fmt.Errorf("invalid background flipX")
	}
	a.FlipY, ok = m["flipY"].(bool)
	if !ok {
		return a, fmt.Errorf("invalid background flipY")
	}
	if len(m) != 10 {
		return a, fmt.Errorf("invalid background appearance fields")
	}
	return a, nil
}
func fillRetimeBackground(clip Object, global, fps float64) (Object, error) {
	if global == 1 {
		return clip, nil
	}
	local := fillNum(clip, "speed", 1)
	speed := local * global
	if !fillFinite(speed) || speed < .1 || speed > 10 {
		return nil, fmt.Errorf("Tốc độ nền hiệu dụng của clip %s phải trong khoảng 0.1–10.", fillString(clip, "id"))
	}
	sourceFPS := fillNum(clip, "sourceFps", fps)
	start := fillNum(clip, "sourceStart", fillNum(clip, "trimStart", fillNum(clip, "offset", 0)))
	authoredEnd := fillNum(clip, "sourceEnd", fillNum(clip, "sourceDuration", start+math.Max(1, math.Ceil(fillNum(clip, "durationInFrames", 0)*local*sourceFPS/fps))))
	end := math.Min(authoredEnd, fillNum(clip, "sourceDuration", authoredEnd))
	if !fillFinite(sourceFPS) || sourceFPS <= 0 || !fillFinite(start) || start < 0 || !fillFinite(end) || end <= start {
		return nil, fmt.Errorf("Khoảng nguồn của clip nền %s không hợp lệ.", fillString(clip, "id"))
	}
	duration := math.Floor(fillNum(clip, "durationInFrames", 0) / global)
	if !fillBool(clip, "loop") {
		duration = math.Min(duration, fillTimeline(end-start, speed, sourceFPS, fps))
	}
	return fillMerge(clip, Object{"speed": speed, "sourceFps": sourceFPS, "sourceStart": start, "sourceEnd": end, "sourceDuration": fillNum(clip, "sourceDuration", end), "durationInFrames": math.Max(1, duration)}), nil
}
func fillJoinWindow(clip Object, head, tail, fps float64) *fillWindow {
	if fillBool(clip, "loop") || clip["sourceEnd"] == nil || clip["sourceDuration"] == nil {
		return nil
	}
	speed := fillNum(clip, "speed", 1)
	sourceFPS := fillNum(clip, "sourceFps", fps)
	start := math.Max(fillNum(clip, "sourceStart", 0), fillSource(head, speed, fps, sourceFPS))
	end := math.Min(fillNum(clip, "sourceEnd", 0), fillNum(clip, "sourceDuration", 0)-fillSource(tail, speed, fps, sourceFPS))
	frames := fillTimeline(end-start, speed, sourceFPS, fps)
	if frames*2 >= fillNum(clip, "durationInFrames", 0) {
		return &fillWindow{Start: start, Frames: math.Min(frames, fillNum(clip, "durationInFrames", 0)), Speed: speed, FPS: sourceFPS}
	}
	return nil
}
func fillApplyAppearance(clip Object, a fillBackgroundAppearance, width, height float64) Object {
	if a.Opacity != 1 || a.Zoom != 1 || a.FlipX || a.FlipY {
		transform := fillCopy(fillObject(clip["transform"]))
		if a.Opacity != 1 {
			transform["opacity"] = fillNum(transform, "opacity", 1) * a.Opacity
		}
		if a.Zoom != 1 {
			sw := fillNum(clip, "sourceWidth", width)
			sh := fillNum(clip, "sourceHeight", height)
			fit := math.Min(width/sw, height/sh)
			transform["width"] = fillNum(transform, "width", sw*fit) * a.Zoom
			transform["height"] = fillNum(transform, "height", sh*fit) * a.Zoom
			for _, key := range []string{"anchorX", "anchorY"} {
				if transform[key] != nil {
					transform[key] = fillNum(transform, key, 0) * a.Zoom
				}
			}
		}
		if a.FlipX {
			transform["flipHorizontal"] = !fillBool(transform, "flipHorizontal")
		}
		if a.FlipY {
			transform["flipVertical"] = !fillBool(transform, "flipVertical")
		}
		clip["transform"] = transform
	}
	effects := append([]Object{}, fillObjects(clip["effects"])...)
	existing := map[string]bool{}
	for _, e := range effects {
		existing[fillString(e, "id")] = true
	}
	for _, field := range []struct {
		Name, Type, Param string
		Value, Identity   float64
	}{{"blur", "gpu-gaussian-blur", "radius", a.Blur, 0}, {"brightness", "gpu-brightness", "amount", a.Brightness, 0}, {"contrast", "gpu-contrast", "amount", a.Contrast, 1}, {"saturation", "gpu-saturation", "amount", a.Saturation, 1}, {"hue", "gpu-hue-shift", "shift", a.Hue, 0}} {
		if field.Value == field.Identity {
			continue
		}
		base := fillString(clip, "id") + "-background-" + field.Name
		id := base
		for n := 1; existing[id]; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		effects = append(effects, Object{"id": id, "enabled": true, "effect": Object{"type": "gpu-effect", "gpuEffectType": field.Type, "params": Object{field.Param: field.Value}}})
	}
	if len(effects) > len(fillObjects(clip["effects"])) {
		clip["effects"] = effects
	}
	return clip
}
func fillPortions(duration, alignment float64) (float64, float64) {
	duration = math.Max(1, math.Floor(duration))
	left := math.Floor(duration * math.Max(0, math.Min(1, alignment)))
	return left, duration - left
}
func fillAvailableHandle(clip Object, head bool, fps float64) float64 {
	switch fillString(clip, "type") {
	case "image", "text", "shape", "adjustment":
		return math.Inf(1)
	case "audio":
		return 0
	}
	frames := math.Max(0, fillNum(clip, "sourceDuration", 0)-fillNum(clip, "sourceEnd", fillNum(clip, "sourceDuration", 0)))
	if head {
		frames = fillNum(clip, "sourceStart", fillNum(clip, "trimStart", 0))
	}
	return fillTimeline(frames, fillNum(clip, "speed", 1), fillNum(clip, "sourceFps", fps), fps)
}
func fillMaxTransition(left, right Object, alignment, fps float64) float64 {
	low := 1.
	high := math.Floor(math.Min(fillNum(left, "durationInFrames", 0), fillNum(right, "durationInFrames", 0)) - 1)
	best := 0.
	tail := fillAvailableHandle(left, false, fps)
	head := fillAvailableHandle(right, true, fps)
	for low <= high {
		mid := math.Floor((low + high) / 2)
		l, r := fillPortions(mid, alignment)
		if r <= tail && l <= head {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return best
}
