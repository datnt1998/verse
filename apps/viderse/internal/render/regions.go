package render

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

const MaxExportRegions = 128

type EncodedFacts struct {
	Width, Height int
	DurationSec   float64
}
type ExportRegion struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	StartSec float64 `json:"startSec"`
	EndSec   float64 `json:"endSec"`
}

type roiMatrix [6]float64
type roiSpace struct{ width, height, fps float64 }
type roiClock struct{ start, secondsPerFrame, min, max, slack float64 }
type roiBox struct{ left, top, right, bottom float64 }
type roiTransform struct{ x, y, width, height, anchorX, anchorY, rotation, opacity, cornerRadius float64 }

// Saved editor JSON is intentionally open-ended; these helpers inspect only the
// fields that affect protection and leave all other engine fields untouched.
func roiObject(value any) editor.Object { object, _ := value.(map[string]any); return object }
func roiObjects(value any) []editor.Object {
	var result []editor.Object
	switch values := value.(type) {
	case []any:
		for _, value := range values {
			if object := roiObject(value); object != nil {
				result = append(result, object)
			}
		}
	case []editor.Object:
		result = values
	}
	return result
}
func roiNumber(value any) (float64, bool) {
	var number float64
	switch v := value.(type) {
	case float64:
		number = v
	case float32:
		number = float64(v)
	case int:
		number = float64(v)
	case int64:
		number = float64(v)
	case json.Number:
		var err error
		number, err = v.Float64()
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}
func roiNum(value any) float64   { number, _ := roiNumber(value); return number }
func roiPositive(value any) bool { number, ok := roiNumber(value); return ok && number > 0 }
func roiString(value any) string { text, _ := value.(string); return text }
func roiPopulated(value any) bool {
	switch v := value.(type) {
	case []any:
		return len(v) > 0
	case []editor.Object:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	return false
}
func roiDefault(value any, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	number, ok := roiNumber(value)
	if !ok {
		return math.NaN()
	}
	return number
}
func roiMultiply(a, b roiMatrix) roiMatrix {
	return roiMatrix{a[0]*b[0] + a[2]*b[1], a[1]*b[0] + a[3]*b[1], a[0]*b[2] + a[2]*b[3], a[1]*b[2] + a[3]*b[3], a[0]*b[4] + a[2]*b[5] + a[4], a[1]*b[4] + a[3]*b[5] + a[5]}
}
func roiInverse(m roiMatrix) (roiMatrix, bool) {
	d := m[0]*m[3] - m[1]*m[2]
	if math.Abs(d) < 0.000001 {
		return roiMatrix{}, false
	}
	return roiMatrix{m[3] / d, -m[1] / d, -m[2] / d, m[0] / d, (m[2]*m[5] - m[3]*m[4]) / d, (m[1]*m[4] - m[0]*m[5]) / d}, true
}
func roiReference(value any) *roiTransform {
	v := roiObject(value)
	if v == nil || !roiPositive(v["width"]) || !roiPositive(v["height"]) {
		return nil
	}
	for _, key := range []string{"x", "y", "rotation"} {
		if _, ok := roiNumber(v[key]); !ok {
			return nil
		}
	}
	return &roiTransform{x: roiNum(v["x"]), y: roiNum(v["y"]), width: roiNum(v["width"]), height: roiNum(v["height"]), rotation: roiNum(v["rotation"])}
}
func roiReferenceMatrix(t roiTransform) roiMatrix {
	angle := t.rotation * math.Pi / 180
	c, s := math.Cos(angle), math.Sin(angle)
	return roiMatrix{c * t.width, s * t.width, -s * t.height, c * t.height, t.x, t.y}
}
func roiApplyParent(local roiTransform, binding editor.Object, parent *roiTransform) *roiTransform {
	childLocal, childWorld := roiReference(binding["childLocalReference"]), roiReference(binding["childWorldReference"])
	parentReference := roiReference(binding["parentReference"])
	if childLocal == nil || childWorld == nil || (binding["parentReference"] != nil && parentReference == nil) {
		return nil
	}
	matrix := roiReferenceMatrix(local)
	if inverse, ok := roiInverse(roiReferenceMatrix(*childLocal)); ok {
		matrix = roiMultiply(roiReferenceMatrix(*childWorld), roiMultiply(inverse, matrix))
	}
	if parent != nil && parentReference != nil {
		if inverse, ok := roiInverse(roiReferenceMatrix(*parentReference)); ok {
			matrix = roiMultiply(roiMultiply(roiReferenceMatrix(*parent), inverse), matrix)
		}
	}
	width := math.Max(0.000001, math.Hypot(matrix[0], matrix[1]))
	height := math.Max(0.000001, math.Abs(matrix[0]*matrix[3]-matrix[1]*matrix[2])/width)
	sx, sy := width/math.Max(0.000001, local.width), height/math.Max(0.000001, local.height)
	local.x, local.y, local.width, local.height = matrix[4], matrix[5], width, height
	local.anchorX *= sx
	local.anchorY *= sy
	local.rotation = math.Atan2(matrix[1], matrix[0]) * 180 / math.Pi
	local.cornerRadius *= math.Sqrt(sx * sy)
	return &local
}
func roiLocalTransform(item editor.Object, space roiSpace) *roiTransform {
	t := roiObject(item["transform"])
	for _, value := range t {
		switch v := value.(type) {
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil
			}
		case float32:
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil
			}
		}
	}
	sw, sh := item["sourceWidth"], item["sourceHeight"]
	if item["type"] == "composition" {
		sw, sh = item["compositionWidth"], item["compositionHeight"]
	}
	fit := 1.0
	if roiPositive(sw) && roiPositive(sh) {
		fit = math.Min(space.width/roiNum(sw), space.height/roiNum(sh))
	}
	width, height := space.width, space.height
	if roiPositive(sw) {
		width = roiNum(sw) * fit
	}
	if roiPositive(sh) {
		height = roiNum(sh) * fit
	}
	width, height = roiDefault(t["width"], width), roiDefault(t["height"], height)
	if !roiPositive(width) || !roiPositive(height) {
		return nil
	}
	result := &roiTransform{roiDefault(t["x"], 0), roiDefault(t["y"], 0), width, height, roiDefault(t["anchorX"], width/2), roiDefault(t["anchorY"], height/2), roiDefault(t["rotation"], 0), roiDefault(t["opacity"], 1), roiDefault(t["cornerRadius"], 0)}
	for _, value := range []float64{result.x, result.y, result.anchorX, result.anchorY, result.rotation, result.opacity, result.cornerRadius} {
		if _, ok := roiNumber(value); !ok {
			return nil
		}
	}
	return result
}
func roiItemMatrix(item editor.Object, t roiTransform, space roiSpace) roiMatrix {
	style := roiObject(item["transform"])
	sx, sy := 1.0, 1.0
	if style["flipHorizontal"] == true {
		sx = -1
	}
	if style["flipVertical"] == true {
		sy = -1
	}
	angle := t.rotation * math.Pi / 180
	a, b, c, d := math.Cos(angle)*sx, math.Sin(angle)*sx, -math.Sin(angle)*sy, math.Cos(angle)*sy
	left, top := space.width/2+t.x-t.width/2, space.height/2+t.y-t.height/2
	return roiMatrix{a, b, c, d, left + t.anchorX - a*t.anchorX - c*t.anchorY, top + t.anchorY - b*t.anchorX - d*t.anchorY}
}
func roiBounds(m roiMatrix, width, height, bleed float64) roiBox {
	box := roiBox{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, point := range [4][2]float64{{-bleed, -bleed}, {width + bleed, -bleed}, {-bleed, height + bleed}, {width + bleed, height + bleed}} {
		x, y := m[0]*point[0]+m[2]*point[1]+m[4], m[1]*point[0]+m[3]*point[1]+m[5]
		box.left = math.Min(box.left, x)
		box.top = math.Min(box.top, y)
		box.right = math.Max(box.right, x)
		box.bottom = math.Max(box.bottom, y)
	}
	return box
}
func roiUnion(a, b ExportRegion) ExportRegion {
	x, y := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	return ExportRegion{x, y, math.Max(a.X+a.Width, b.X+b.Width) - x, math.Max(a.Y+a.Height, b.Y+b.Height) - y, math.Min(a.StartSec, b.StartSec), math.Max(a.EndSec, b.EndSec)}
}

// DeriveExportRegions protects rendered foreground detail conservatively. Unsafe
// transforms protect the full frame; missing painted text evidence rejects the pass.
func DeriveExportRegions(project editor.Project, encoded EncodedFacts, padding float64, coverage editor.Object) ([]ExportRegion, error) {
	metadata := project.Metadata()
	root := roiSpace{roiNum(metadata["width"]), roiNum(metadata["height"]), roiNum(metadata["fps"])}
	for _, n := range []float64{float64(encoded.Width), float64(encoded.Height), encoded.DurationSec, root.width, root.height, root.fps} {
		if !roiPositive(n) {
			return nil, fmt.Errorf("Invalid export region dimensions or duration")
		}
	}
	if _, ok := roiNumber(padding); !ok || padding < 0 {
		return nil, fmt.Errorf("Invalid export region dimensions or duration")
	}
	timeline := project.Timeline()
	regions := make([]ExportRegion, 0)
	if timeline == nil {
		return regions, nil
	}
	compositions := map[string]editor.Object{}
	for _, child := range roiObjects(timeline["compositions"]) {
		compositions[roiString(child["id"])] = child
	}
	coverageKey := func(path []string, id string) string {
		parts := make([]string, len(path)+1)
		copy(parts, path)
		parts[len(path)] = id
		b, _ := json.Marshal(parts)
		return string(b)
	}
	measured := map[string]editor.Object{}
	for _, entry := range roiObjects(coverage["entries"]) {
		var path []string
		switch values := entry["instancePath"].(type) {
		case []string:
			path = values
		case []any:
			for _, value := range values {
				part, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("Invalid painted caption instance path")
				}
				path = append(path, part)
			}
		default:
			return nil, fmt.Errorf("Invalid painted caption instance path")
		}
		id, ok := entry["itemId"].(string)
		if !ok {
			return nil, fmt.Errorf("Invalid painted caption item identity")
		}
		bounds, observed := entry["bounds"]
		if !observed {
			return nil, fmt.Errorf("Thiếu vùng chữ đã đo từ bản render — giữ bản xuất gốc")
		}
		if bounds != nil {
			painted := roiObject(bounds)
			if painted == nil {
				return nil, fmt.Errorf("Invalid painted caption bounds")
			}
			for _, field := range []string{"x", "y", "width", "height"} {
				if _, ok := roiNumber(painted[field]); !ok {
					return nil, fmt.Errorf("Non-finite export geometry")
				}
			}
			if !roiPositive(painted["width"]) || !roiPositive(painted["height"]) {
				return nil, fmt.Errorf("Invalid painted caption bounds")
			}
		}
		measured[coverageKey(path, id)] = entry
	}
	sx, sy := float64(encoded.Width)/root.width, float64(encoded.Height)/root.height
	full := roiBox{0, 0, root.width, root.height}
	compact := func() {
		sort.SliceStable(regions, func(i, j int) bool {
			a, b := regions[i], regions[j]
			if a.StartSec != b.StartSec {
				return a.StartSec < b.StartSec
			}
			if a.Y != b.Y {
				return a.Y < b.Y
			}
			return a.X < b.X
		})
		merged := regions[:0]
		for _, region := range regions {
			if len(merged) > 0 {
				previous := &merged[len(merged)-1]
				if previous.X == region.X && previous.Y == region.Y && previous.Width == region.Width && previous.Height == region.Height && region.StartSec <= previous.EndSec+1/root.fps {
					previous.EndSec = math.Max(previous.EndSec, region.EndSec)
					continue
				}
			}
			merged = append(merged, region)
		}
		for len(merged) > MaxExportRegions {
			next := merged[:0]
			for i := 0; i < len(merged); i += 2 {
				region := merged[i]
				if i+1 < len(merged) {
					region = roiUnion(region, merged[i+1])
				}
				next = append(next, region)
			}
			merged = next
		}
		regions = merged
	}
	add := func(box roiBox, start, end float64) error {
		x, y := math.Max(0, math.Floor(box.left*sx-padding)), math.Max(0, math.Floor(box.top*sy-padding))
		right, bottom := math.Min(float64(encoded.Width), math.Ceil(box.right*sx+padding)), math.Min(float64(encoded.Height), math.Ceil(box.bottom*sy+padding))
		start, end = math.Max(0, start), math.Min(encoded.DurationSec, end)
		for _, value := range []float64{x, y, right, bottom, start, end} {
			if _, ok := roiNumber(value); !ok {
				return fmt.Errorf("Non-finite export geometry")
			}
		}
		if right <= x || bottom <= y || end <= start {
			return nil
		}
		regions = append(regions, ExportRegion{x, y, right - x, bottom - y, start, end})
		if len(regions) > MaxExportRegions*2 {
			compact()
		}
		return nil
	}
	var visit func(editor.Object, roiSpace, roiMatrix, roiClock, bool, map[string]bool, []string) error
	visit = func(data editor.Object, space roiSpace, matrix roiMatrix, clock roiClock, unsafeAncestor bool, stack map[string]bool, path []string) error {
		tracks, items, err := roiRenderTracks(data, space, len(path) == 0)
		if err != nil {
			return err
		}
		byID := map[string]editor.Object{}
		for _, item := range items {
			byID[roiString(item["id"])] = item
		}
		animated := map[string]bool{}
		for _, keyframe := range roiObjects(data["keyframes"]) {
			animated[roiString(keyframe["itemId"])] = true
		}
		var visible func(string, map[string]bool) bool
		visible = func(id string, seen map[string]bool) bool {
			track := tracks[id]
			if track == nil || seen[id] {
				return true
			}
			if track["visible"] == false {
				return false
			}
			seen[id] = true
			if parent := roiString(track["parentTrackId"]); parent != "" {
				return visible(parent, seen)
			}
			return true
		}
		adjusted := false
		for _, item := range items {
			if item["type"] == "adjustment" && visible(roiString(item["trackId"]), map[string]bool{}) && roiPopulated(item["effects"]) {
				adjusted = true
			}
		}
		var resolve func(editor.Object, map[string]bool) *roiTransform
		resolve = func(item editor.Object, seen map[string]bool) *roiTransform {
			id := roiString(item["id"])
			if item["type"] == "text" || item["type"] == "subtitle" || seen[id] || animated[id] || roiPopulated(item["motionModifiers"]) || roiPopulated(item["motionLayers"]) || roiPopulated(item["cornerPin"]) || roiPopulated(item["effects"]) || item["shapeType"] == "path" {
				return nil
			}
			seen[id] = true
			local := roiLocalTransform(item, space)
			if local == nil || item["transformParent"] == nil {
				return local
			}
			binding := roiObject(item["transformParent"])
			if binding == nil {
				return nil
			}
			var parentWorld *roiTransform
			if parentID := roiString(binding["parentItemId"]); parentID != "" {
				parent := byID[parentID]
				if parent == nil {
					return nil
				}
				parentWorld = resolve(parent, seen)
				if parentWorld == nil {
					return nil
				}
			}
			return roiApplyParent(*local, binding, parentWorld)
		}
		transitions := roiObjects(data["transitions"])
		for _, item := range items {
			if !visible(roiString(item["trackId"]), map[string]bool{}) {
				continue
			}
			track := tracks[roiString(item["trackId"])]
			waveform := roiObject(item["waveformVisualizer"])
			if !unsafeAncestor && item["type"] == "shape" && waveform["visible"] == false {
				continue
			}
			transformed := resolve(item, map[string]bool{})
			if !unsafeAncestor && transformed != nil && transformed.opacity == 0 {
				continue
			}
			if duration, ok := roiNumber(item["durationInFrames"]); ok && duration <= 0 {
				continue
			}
			from, fromOK := roiNumber(item["from"])
			duration, durationOK := roiNumber(item["durationInFrames"])
			timed := fromOK && durationOK && duration > 0
			handles := 0.0
			for _, transition := range transitions {
				if transition["leftClipId"] == item["id"] || transition["rightClipId"] == item["id"] {
					if roiPositive(transition["durationInFrames"]) {
						handles = math.Max(handles, roiNum(transition["durationInFrames"]))
					}
				}
			}
			start, end := clock.min, clock.max
			if timed {
				start = math.Max(clock.min, clock.start+(from-handles)*clock.secondsPerFrame-clock.slack)
				end = math.Min(clock.max, clock.start+(from+duration+handles)*clock.secondsPerFrame+clock.slack)
			}
			if end <= start {
				continue
			}
			if item["type"] == "text" || item["type"] == "subtitle" {
				entry := measured[coverageKey(path, roiString(item["id"]))]
				entrySpace := roiObject(entry["space"])
				if entry == nil || roiNum(entrySpace["width"]) != space.width || roiNum(entrySpace["height"]) != space.height {
					return fmt.Errorf("Thiếu vùng chữ đã đo từ bản render — giữ bản xuất gốc")
				}
				if painted := roiObject(entry["bounds"]); painted != nil {
					for _, field := range []string{"x", "y", "width", "height"} {
						if _, ok := roiNumber(painted[field]); !ok {
							return fmt.Errorf("Non-finite export geometry")
						}
					}
					box := full
					if !unsafeAncestor {
						box = roiBounds(roiMultiply(matrix, roiMatrix{1, 0, 0, 1, roiNum(painted["x"]), roiNum(painted["y"])}), roiNum(painted["width"]), roiNum(painted["height"]), 0)
					}
					if err := add(box, start, end); err != nil {
						return err
					}
				}
				continue
			}
			unsafe := unsafeAncestor || adjusted || transformed == nil || !timed || handles > 0
			kind := roiString(item["type"])
			slot := roiString(track["templateSlot"])
			if (kind == "video" || kind == "image" || kind == "lottie") && (slot == "background-video" || slot == "scenes" || slot == "source-audio") {
				continue
			}
			if kind == "composition" {
				id := roiString(item["compositionId"])
				child := compositions[id]
				if !timed || child == nil || stack[id] || len(stack) >= 32 {
					if err := add(full, start, end); err != nil {
						return err
					}
					continue
				}
				fps, width, height := roiNum(child["fps"]), roiNum(item["compositionWidth"]), roiNum(item["compositionHeight"])
				speed, sourceFPS := roiDefault(item["speed"], 1), roiDefault(item["sourceFps"], fps)
				offset := roiDefault(item["sourceStart"], roiDefault(item["trimStart"], roiDefault(item["offset"], 0)))
				valid := true
				for _, n := range []float64{fps, width, height, speed, sourceFPS, roiNum(child["durationInFrames"])} {
					valid = valid && roiPositive(n)
				}
				if _, ok := roiNumber(offset); !ok {
					valid = false
				}
				if !valid {
					if err := add(full, start, end); err != nil {
						return err
					}
					continue
				}
				secondsPerFrame := clock.secondsPerFrame * space.fps / (sourceFPS * speed)
				childStart := clock.start + from*clock.secondsPerFrame - offset*secondsPerFrame
				slack := clock.slack + secondsPerFrame/2
				childMin, childMax := math.Max(start, childStart-slack), math.Min(end, childStart+roiNum(child["durationInFrames"])*secondsPerFrame+slack)
				if childMax <= childMin {
					continue
				}
				nextMatrix := matrix
				if transformed != nil {
					nextMatrix = roiMultiply(matrix, roiMultiply(roiItemMatrix(item, *transformed, space), roiMatrix{transformed.width / width, 0, 0, transformed.height / height, 0, 0}))
				}
				stack[id] = true
				nextPath := append(append([]string(nil), path...), roiString(item["id"]))
				err := visit(child, roiSpace{width, height, fps}, nextMatrix, roiClock{childStart, secondsPerFrame, childMin, childMax, slack}, unsafe || item["crop"] != nil || roiPopulated(item["compositionControlOverrides"]), stack, nextPath)
				delete(stack, id)
				if err != nil {
					return err
				}
				continue
			}
			semantic := kind == "shape" && item["waveformVisualizer"] != nil
			graphic := kind == "image" || kind == "lottie" || (kind == "shape" && item["isMask"] != true)
			if !semantic && !graphic {
				continue
			}
			authored := roiLocalTransform(item, space)
			size := roiObject(item["transform"])
			knownSize := (roiPositive(size["width"]) && roiPositive(size["height"])) || (roiPositive(item["sourceWidth"]) && roiPositive(item["sourceHeight"]))
			if !semantic && knownSize && authored != nil && authored.width*authored.height > space.width*space.height*0.25 {
				continue
			}
			box := full
			if !unsafe && transformed != nil {
				shadow, stroke := roiObject(item["textShadow"]), roiObject(item["stroke"])
				bleed, valid := 0.0, true
				for _, value := range []any{shadow["offsetX"], shadow["offsetY"], shadow["blur"], stroke["width"], item["strokeWidth"], waveform["strokeWidth"]} {
					n := roiDefault(value, 0)
					if _, ok := roiNumber(n); !ok {
						valid = false
					}
					bleed += math.Abs(n)
				}
				if valid {
					box = roiBounds(roiMultiply(matrix, roiItemMatrix(item, *transformed, space)), transformed.width, transformed.height, bleed)
				}
			}
			if err := add(box, start, end); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(timeline, root, roiMatrix{1, 0, 0, 1, 0, 0}, roiClock{0, 1 / root.fps, 0, encoded.DurationSec, 0}, false, map[string]bool{}, nil); err != nil {
		return nil, err
	}
	compact()
	return regions, nil
}

// ExportRegionFilter emits only validated numbers. Managed FFmpeg 6 addroi has
// no timeline support, so the encode protects each region's spatial union.
func ExportRegionFilter(regions []ExportRegion) (string, error) {
	if len(regions) == 0 {
		return "", nil
	}
	if len(regions) > MaxExportRegions {
		return "", fmt.Errorf("Too many export regions")
	}
	var out strings.Builder
	for i, r := range regions {
		if r.Width <= 0 || r.Height <= 0 || r.EndSec <= r.StartSec {
			return "", fmt.Errorf("Invalid ROI filter region")
		}
		values := []float64{r.X, r.Y, r.Width, r.Height, r.StartSec, r.EndSec}
		numbers := [4]string{}
		for j, value := range values {
			if _, ok := roiNumber(value); !ok || value < 0 {
				return "", fmt.Errorf("Invalid ROI filter number")
			}
			if j < 4 {
				s := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
				if s == "" || s == "-0" {
					s = "0"
				}
				numbers[j] = s
			}
		}
		if i > 0 {
			out.WriteString(",\n")
		}
		clear := 0
		if i == 0 {
			clear = 1
		}
		fmt.Fprintf(&out, "addroi=x=%s:y=%s:w=%s:h=%s:qoffset=-0.25:clear=%d", numbers[0], numbers[1], numbers[2], numbers[3], clear)
	}
	return out.String(), nil
}

// roiRenderTracks mirrors the engine's headless caption-track projection:
// group visibility, export reverse-conforms, reverse-source suppression and
// synthetic transcript identity/timing all precede geometry traversal.
func roiRenderTracks(data editor.Object, space roiSpace, root bool) (map[string]editor.Object, []editor.Object, error) {
	tracks := roiObjects(data["tracks"])
	items := roiObjects(data["items"])
	validArray := func(value any, count int) bool {
		switch values := value.(type) {
		case []any:
			return len(values) == count
		case []editor.Object:
			return len(values) == count
		default:
			return false
		}
	}
	if !validArray(data["tracks"], len(tracks)) || !validArray(data["items"], len(items)) {
		return nil, nil, fmt.Errorf("Invalid render timeline")
	}
	validField := func(object editor.Object, field, kind string) bool {
		value, exists := object[field]
		if !exists {
			return true
		}
		switch kind {
		case "string":
			_, ok := value.(string)
			return ok
		case "bool":
			_, ok := value.(bool)
			return ok
		case "number":
			_, ok := roiNumber(value)
			return ok
		case "positive":
			return roiPositive(value)
		}
		return false
	}
	groups := map[string]editor.Object{}
	for _, track := range tracks {
		if _, ok := track["id"].(string); !ok {
			return nil, nil, fmt.Errorf("Invalid render track")
		}
		for _, key := range []string{"solo", "visible"} {
			if !validField(track, key, "bool") {
				return nil, nil, fmt.Errorf("Invalid render track %s", key)
			}
		}
		if !validField(track, "order", "number") {
			return nil, nil, fmt.Errorf("Invalid render track order")
		}
		if track["isGroup"] == true {
			groups[roiString(track["id"])] = track
		}
	}
	for _, item := range items {
		for _, key := range []string{"id", "type", "trackId"} {
			if _, ok := item[key].(string); !ok {
				return nil, nil, fmt.Errorf("Invalid render item %s", key)
			}
		}
		if _, ok := roiNumber(item["from"]); !ok {
			return nil, nil, fmt.Errorf("Invalid render item from")
		}
		if duration, ok := roiNumber(item["durationInFrames"]); !ok || duration < 0 {
			return nil, nil, fmt.Errorf("Invalid render item duration")
		}
		for _, key := range []string{"mediaId"} {
			if !validField(item, key, "string") {
				return nil, nil, fmt.Errorf("Invalid render item %s", key)
			}
		}
		if !validField(item, "isReversed", "bool") {
			return nil, nil, fmt.Errorf("Invalid reversed item")
		}
		for _, key := range []string{"sourceStart", "sourceEnd", "sourceDuration", "trimStart", "trimEnd"} {
			if !validField(item, key, "number") {
				return nil, nil, fmt.Errorf("Invalid render item %s", key)
			}
		}
		for _, key := range []string{"sourceFps", "speed"} {
			if !validField(item, key, "positive") {
				return nil, nil, fmt.Errorf("Invalid render item %s", key)
			}
		}
		if sourceValue, exists := item["source"]; exists {
			source := roiObject(sourceValue)
			kind := roiString(source["type"])
			if source == nil || (kind != "transcript" && kind != "embedded-subtitles" && kind != "subtitle-import") || !validField(source, "clipId", "string") || ((kind == "transcript" || kind == "embedded-subtitles") && roiString(source["clipId"]) == "") {
				return nil, nil, fmt.Errorf("Invalid subtitle source")
			}
		} else if item["type"] == "subtitle" {
			return nil, nil, fmt.Errorf("Subtitle requires a source")
		}
		if sourceValue, exists := item["captionSource"]; exists {
			source := roiObject(sourceValue)
			if source == nil || !validField(source, "clipId", "string") {
				return nil, nil, fmt.Errorf("Invalid caption source")
			}
		}
		if value, exists := item["transcriptCaptions"]; exists {
			transcript := roiObject(value)
			_, mediaOK := transcript["mediaId"].(string)
			_, enabledOK := transcript["enabled"].(bool)
			cues := roiObjects(transcript["cues"])
			if transcript == nil || transcript["type"] != "transcript" || !mediaOK || !enabledOK || !validArray(transcript["cues"], len(cues)) {
				return nil, nil, fmt.Errorf("Invalid transcript captions")
			}
			for _, cue := range cues {
				_, idOK := cue["id"].(string)
				_, textOK := cue["text"].(string)
				_, startOK := roiNumber(cue["startSeconds"])
				_, endOK := roiNumber(cue["endSeconds"])
				if !idOK || !textOK || !startOK || !endOK {
					return nil, nil, fmt.Errorf("Invalid transcript cue")
				}
			}
			if style, exists := transcript["style"]; exists && roiObject(style) == nil {
				return nil, nil, fmt.Errorf("Invalid transcript style")
			}
		}
	}
	cloneObject := func(object editor.Object) editor.Object {
		next := editor.Object{}
		for key, value := range object {
			next[key] = value
		}
		return next
	}
	projectedTracks := make([]editor.Object, 0, len(tracks)+1)
	byTrack := map[string][]editor.Object{}
	for _, item := range items {
		byTrack[roiString(item["trackId"])] = append(byTrack[roiString(item["trackId"])], item)
	}
	for _, track := range tracks {
		if root && track["isGroup"] == true {
			continue
		}
		next := cloneObject(track)
		if root {
			if group := groups[roiString(track["parentTrackId"])]; group != nil {
				next["visible"] = track["visible"] != false && group["visible"] != false
				next["solo"] = track["solo"] == true || group["solo"] == true
			}
		}
		if next["id"] == "__virtual-transcript-captions__" {
			continue
		}
		projectedTracks = append(projectedTracks, next)
	}
	projectedItems := make([]editor.Object, 0, len(items))
	reversed := map[string]bool{}
	for _, track := range projectedTracks {
		for _, item := range byTrack[roiString(track["id"])] {
			next := item
			if root && item["type"] == "video" && item["isReversed"] == true && item["reverseConformStatus"] == "ready" && roiString(item["reverseConformSrc"]) != "" {
				next = cloneObject(item)
				delete(next, "isReversed")
				start := roiDefault(item["reverseConformLocalStart"], 0)
				next["speed"] = 1
				next["sourceStart"] = start
				next["trimStart"] = start
				next["offset"] = start
				next["sourceEnd"] = start + roiNum(item["durationInFrames"])
				next["sourceDuration"] = next["sourceEnd"]
				next["sourceFps"] = space.fps
			}
			if (next["type"] == "video" || next["type"] == "audio") && next["isReversed"] == true {
				reversed[roiString(next["id"])] = true
			}
			projectedItems = append(projectedItems, next)
		}
	}
	kept := projectedItems[:0]
	for _, item := range projectedItems {
		if item["type"] == "text" && reversed[roiString(roiObject(item["captionSource"])["clipId"])] {
			continue
		}
		if item["type"] == "subtitle" {
			source := roiObject(item["source"])
			if (source["type"] == "transcript" || source["type"] == "embedded-subtitles") && reversed[roiString(source["clipId"])] {
				continue
			}
		}
		kept = append(kept, item)
	}
	projectedItems = kept
	resultTracks := map[string]editor.Object{}
	hasSolo := false
	for _, track := range projectedTracks {
		resultTracks[roiString(track["id"])] = track
		hasSolo = hasSolo || track["solo"] == true
	}
	// Traverse in track order exactly as renderCaptionTracks does.
	baseCount := len(projectedItems)
	for _, item := range projectedItems[:baseCount] {
		track := resultTracks[roiString(item["trackId"])]
		if hasSolo {
			if track["solo"] != true {
				continue
			}
		} else if track["visible"] == false {
			continue
		}
		if (item["type"] != "video" && item["type"] != "audio") || roiString(item["mediaId"]) == "" || item["isReversed"] == true {
			continue
		}
		transcript := roiObject(item["transcriptCaptions"])
		if transcript["enabled"] != true || transcript["type"] != "transcript" {
			continue
		}
		sourceFPS, speed := roiDefault(item["sourceFps"], space.fps), roiDefault(item["speed"], 1)
		sourceStart := roiDefault(item["sourceStart"], 0)
		sourceEnd := roiDefault(item["sourceEnd"], sourceStart+math.Floor(roiNum(item["durationInFrames"])*speed*sourceFPS/space.fps+0.5))
		first, last := math.Inf(1), 0.0
		for _, cue := range roiObjects(transcript["cues"]) {
			start, end := math.Max(roiNum(cue["startSeconds"]), sourceStart/sourceFPS), math.Min(roiNum(cue["endSeconds"]), sourceEnd/sourceFPS)
			if end <= start {
				continue
			}
			startFrame, endFrame := math.Floor((start-sourceStart/sourceFPS)/speed*space.fps), math.Ceil((end-sourceStart/sourceFPS)/speed*space.fps)
			if endFrame <= startFrame {
				continue
			}
			first = math.Min(first, startFrame)
			last = math.Max(last, endFrame)
		}
		if math.IsInf(first, 1) {
			continue
		}
		offset := math.Max(0, math.Min(first, roiNum(item["durationInFrames"])-1))
		end := math.Min(roiNum(item["durationInFrames"]), math.Max(last, offset+1))
		virtual := editor.Object{"id": "virtual-transcript-" + roiString(item["id"]), "type": "subtitle", "trackId": "__virtual-transcript-captions__", "from": roiNum(item["from"]) + offset, "durationInFrames": math.Max(1, end-offset), "source": editor.Object{"type": "transcript", "clipId": item["id"]}}
		projectedItems = append(projectedItems, virtual)
		resultTracks["__virtual-transcript-captions__"] = editor.Object{"id": "__virtual-transcript-captions__", "visible": true, "solo": hasSolo}
	}
	return resultTracks, projectedItems, nil
}
