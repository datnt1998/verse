package render

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

var regionEncoded = EncodedFacts{Width: 1920, Height: 1080, DurationSec: 10}

func regionItem(id string, extra editor.Object) editor.Object {
	item := editor.Object{"id": id, "type": "shape", "trackId": "foreground", "from": 30, "durationInFrames": 60, "transform": editor.Object{"x": 200, "y": -100, "width": 400, "height": 100}}
	for key, value := range extra {
		item[key] = value
	}
	return item
}
func regionProject(items []editor.Object, extra editor.Object) editor.Project {
	timeline := editor.Object{"tracks": []editor.Object{{"id": "foreground", "visible": true}}, "items": items}
	for key, value := range extra {
		timeline[key] = value
	}
	return editor.Project{"id": "fixture", "name": "ROI", "description": "", "createdAt": 0, "updatedAt": 0, "duration": 10, "metadata": editor.Object{"width": 1920, "height": 1080, "fps": 30}, "timeline": timeline}
}
func regionDerive(t *testing.T, p editor.Project, e EncodedFacts, padding float64, coverage editor.Object) []ExportRegion {
	t.Helper()
	regions, err := DeriveExportRegions(p, e, padding, coverage)
	if err != nil {
		t.Fatal(err)
	}
	return regions
}
func regionRequire(t *testing.T, got, want []ExportRegion) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("regions = %+v; want %+v", got, want)
	}
}

func TestRegionsSavedGeometryAndEncodedScaling(t *testing.T) {
	saved := regionProject([]editor.Object{regionItem("title", nil)}, nil)
	want := []ExportRegion{{948, 378, 424, 124, 1, 3}}
	regionRequire(t, regionDerive(t, saved, regionEncoded, 12, nil), want)
	saved["metadata"] = editor.Object{"width": 3840, "height": 2160, "fps": 30}
	saved["timeline"] = editor.Object{"tracks": []editor.Object{{"id": "foreground"}}, "items": []editor.Object{regionItem("title", editor.Object{"transform": editor.Object{"x": 400, "y": -200, "width": 800, "height": 200}})}}
	regionRequire(t, regionDerive(t, saved, regionEncoded, 12, nil), want)
}

func TestRegionsRotationAnchorFlipsAndClipping(t *testing.T) {
	saved := regionProject([]editor.Object{regionItem("title", editor.Object{"transform": editor.Object{"x": -900, "y": -490, "width": 100, "height": 40, "anchorX": 0, "anchorY": 0, "rotation": 90, "flipHorizontal": true}})}, nil)
	regions := regionDerive(t, saved, regionEncoded, 0, nil)
	if len(regions) != 1 {
		t.Fatalf("regions %+v", regions)
	}
	r := regions[0]
	if r.X != 0 || r.Y != 0 || r.X+r.Width < 10 || r.Y+r.Height < 30 || r.Width*r.Height >= 1000 {
		t.Fatalf("rotated fragment %+v", r)
	}
	off := regionProject([]editor.Object{regionItem("off", editor.Object{"transform": editor.Object{"x": 9999, "width": 10, "height": 10}})}, nil)
	if got := regionDerive(t, off, regionEncoded, 12, nil); len(got) != 0 {
		t.Fatalf("offscreen protected %+v", got)
	}
}

func TestRegionsProtectWaveformsAndCompactGraphicsNotFootage(t *testing.T) {
	items := []editor.Object{
		regionItem("caption", nil),
		regionItem("wave", editor.Object{"waveformVisualizer": editor.Object{}, "transform": editor.Object{"width": 900, "height": 60, "y": 300}}),
		regionItem("logo", editor.Object{"type": "image", "transform": editor.Object{"width": 100, "height": 100, "x": -800, "y": -400}}),
		regionItem("rain", editor.Object{"type": "video"}),
		regionItem("full-image", editor.Object{"type": "image", "transform": editor.Object{"width": 1920, "height": 1080}}),
		regionItem("backdrop", editor.Object{"type": "image", "trackId": "bg"}),
		regionItem("scene", editor.Object{"type": "image", "trackId": "scene"}),
	}
	regions := regionDerive(t, regionProject(items, editor.Object{"tracks": []editor.Object{{"id": "foreground"}, {"id": "bg", "templateSlot": "background-video"}, {"id": "scene", "templateSlot": "scenes"}}}), regionEncoded, 12, nil)
	if len(regions) != 3 {
		t.Fatalf("foreground count %+v", regions)
	}
	for _, r := range regions {
		if r.Width >= 1920 {
			t.Fatalf("footage protected %+v", r)
		}
	}
}

func TestRegionsOmitHiddenTransparentAndOutsideExport(t *testing.T) {
	saved := regionProject([]editor.Object{regionItem("hidden", editor.Object{"trackId": "child"}), regionItem("transparent", editor.Object{"transform": editor.Object{"opacity": 0}}), regionItem("late", editor.Object{"from": 400}), regionItem("visible", nil)}, editor.Object{"tracks": []editor.Object{{"id": "foreground"}, {"id": "group", "visible": false}, {"id": "child", "parentTrackId": "group", "visible": true}}})
	if got := regionDerive(t, saved, regionEncoded, 12, nil); len(got) != 1 {
		t.Fatalf("regions %+v", got)
	}
}

func TestRegionsUnsafeGeometryProtectsFullFrame(t *testing.T) {
	for _, extra := range []editor.Object{
		{"motionModifiers": []editor.Object{{"type": "wander"}}}, {"motionLayers": []editor.Object{{}}}, {"cornerPin": editor.Object{"topLeft": []any{-2000, -2000}}}, {"transform": editor.Object{"width": -1}}, {"transform": editor.Object{"x": math.NaN()}},
	} {
		regionRequire(t, regionDerive(t, regionProject([]editor.Object{regionItem("moving", extra)}, nil), regionEncoded, 12, nil), []ExportRegion{{0, 0, 1920, 1080, 1, 3}})
	}
	saved := regionProject([]editor.Object{regionItem("moving", nil)}, editor.Object{"keyframes": []editor.Object{{"itemId": "moving", "properties": []any{}}}})
	if got := regionDerive(t, saved, regionEncoded, 12, nil); got[0].Width != 1920 {
		t.Fatalf("animated geometry %+v", got)
	}
}

func TestRegionsTransformParentBindingAndUnsafeParents(t *testing.T) {
	ref := editor.Object{"x": 0, "y": 0, "width": 100, "height": 100, "rotation": 0}
	parent := regionItem("parent", editor.Object{"type": "controller", "transform": editor.Object{"x": 300, "y": 0, "width": 100, "height": 100}})
	child := regionItem("child", editor.Object{"transform": editor.Object{"width": 100, "height": 100}, "transformParent": editor.Object{"parentItemId": "parent", "parentReference": ref, "childLocalReference": ref, "childWorldReference": ref}})
	if got := regionDerive(t, regionProject([]editor.Object{parent, child}, nil), regionEncoded, 0, nil); len(got) != 1 || got[0].X != 1210 {
		t.Fatalf("parented %+v", got)
	}
	if got := regionDerive(t, regionProject([]editor.Object{parent, child}, editor.Object{"keyframes": []editor.Object{{"itemId": "parent"}}}), regionEncoded, 0, nil); got[0].Width != 1920 {
		t.Fatalf("animated parent %+v", got)
	}
	parent["transformParent"] = editor.Object{"parentItemId": "child", "parentReference": ref, "childLocalReference": ref, "childWorldReference": ref}
	if got := regionDerive(t, regionProject([]editor.Object{child, parent}, nil), regionEncoded, 12, nil); got[0].Width != 1920 {
		t.Fatalf("cyclic parent %+v", got)
	}
}

func regionNestedFixture(caption bool) (editor.Project, editor.Object) {
	wrapper := regionItem("wrapper", editor.Object{"type": "composition", "from": 60, "durationInFrames": 120, "compositionId": "nested", "compositionWidth": 1000, "compositionHeight": 500, "sourceStart": 48, "sourceFps": 24, "speed": 2, "transform": editor.Object{"x": -300, "y": -100, "width": 500, "height": 250}})
	inside := regionItem("inside", editor.Object{"from": 72, "durationInFrames": 48, "transform": editor.Object{"width": 200, "height": 40, "x": 100, "y": 0}})
	items := []editor.Object{inside}
	if caption {
		inside["type"] = "text"
	} else {
		items = append(items, regionItem("trimmed", editor.Object{"from": 0, "durationInFrames": 12}), regionItem("too-late", editor.Object{"from": 300}))
	}
	nested := editor.Object{"id": "nested", "width": 1000, "height": 500, "fps": 24, "durationInFrames": 240, "tracks": []editor.Object{{"id": "foreground"}}, "items": items}
	p := regionProject([]editor.Object{wrapper}, editor.Object{"compositions": []editor.Object{nested}})
	entry := editor.Object{"instancePath": []string{"wrapper"}, "itemId": "inside", "space": editor.Object{"width": 1000, "height": 500}, "bounds": editor.Object{"x": 500, "y": 230, "width": 200, "height": 40}}
	return p, editor.Object{"version": 1, "output": editor.Object{"width": 1920, "height": 1080, "fps": 30}, "renderedFrames": 300, "entries": []editor.Object{entry}}
}

func TestRegionsNestedCompositionClockTrimSpeedAndFPS(t *testing.T) {
	saved, _ := regionNestedFixture(false)
	regions := regionDerive(t, saved, regionEncoded, 0, nil)
	if len(regions) != 1 {
		t.Fatalf("regions %+v", regions)
	}
	r := regions[0]
	if r.X != 660 || r.Y != 430 || r.Width != 100 || r.Height != 20 || r.StartSec >= 2.5 || r.StartSec <= 2.48 || r.EndSec <= 3.5 || r.EndSec >= 3.52 {
		t.Fatalf("nested region %+v", r)
	}
}

func TestRegionsBoundLongForegroundWithoutLosingCoverage(t *testing.T) {
	items := make([]editor.Object, 2000)
	for i := range items {
		items[i] = regionItem(fmt.Sprintf("word-%d", i), editor.Object{"from": i, "durationInFrames": 1, "transform": editor.Object{"width": 20, "height": 20, "x": (i % 20) * 25, "y": 0}})
	}
	encoded := regionEncoded
	encoded.DurationSec = 100
	regions := regionDerive(t, regionProject(items, nil), encoded, 0, nil)
	if len(regions) > MaxExportRegions {
		t.Fatalf("unbounded count %d", len(regions))
	}
	for i := range items {
		found := false
		for _, r := range regions {
			if r.StartSec <= float64(i)/30+1e-9 && r.EndSec >= float64(i+1)/30-1e-9 && r.X <= float64(950+(i%20)*25) && r.X+r.Width >= float64(970+(i%20)*25) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("word %d lost protection", i)
		}
	}
	if filter, err := ExportRegionFilter(nil); filter != "" || err != nil {
		t.Fatalf("empty filter %q %v", filter, err)
	}
	if _, err := ExportRegionFilter([]ExportRegion{{math.NaN(), 0, 10, 10, 0, 1}}); err == nil {
		t.Fatal("NaN accepted")
	}
	encoded.Width = 0
	if _, err := DeriveExportRegions(regionProject(nil, nil), encoded, 12, nil); err == nil {
		t.Fatal("zero encoded width accepted")
	}
}

func TestRegionsEmptyIntervalsHiddenWaveformsAndUnknownGraphics(t *testing.T) {
	regions := regionDerive(t, regionProject([]editor.Object{regionItem("empty", editor.Object{"durationInFrames": 0}), regionItem("hidden-wave", editor.Object{"waveformVisualizer": editor.Object{"visible": false}}), regionItem("unknown-logo", editor.Object{"type": "image", "transform": editor.Object{"width": -1}})}, nil), regionEncoded, 12, nil)
	regionRequire(t, regions, []ExportRegion{{0, 0, 1920, 1080, 1, 3}})
}

func TestRegionsTransitionHandlesProtectOutsideNominalSpan(t *testing.T) {
	p := regionProject([]editor.Object{regionItem("title", nil)}, editor.Object{"transitions": []editor.Object{{"leftClipId": "title", "rightClipId": "next", "durationInFrames": 15}}})
	regionRequire(t, regionDerive(t, p, regionEncoded, 12, nil), []ExportRegion{{0, 0, 1920, 1080, 0.5, 3.5}})
}

func TestRegionsPaintedKaraokeAndExpandedTitlesAreNotTransformedTwice(t *testing.T) {
	words := regionItem("words", editor.Object{"type": "subtitle", "source": editor.Object{"type": "subtitle-import", "fileName": "fixture.srt"}, "words": []editor.Object{{"text": "long", "startFrame": 0, "endFrame": 30}}, "captionPreset": "karaoke", "transform": editor.Object{"x": 200, "y": 300, "width": 400, "height": 40, "rotation": 90}})
	title := regionItem("title", editor.Object{"type": "text", "text": "A\nB\nC", "fontSize": 100, "transform": editor.Object{"width": 400, "height": 40}})
	entries := []editor.Object{
		{"instancePath": []string{}, "itemId": "words", "space": editor.Object{"width": 1920, "height": 1080}, "bounds": editor.Object{"x": 1000, "y": 700, "width": 180, "height": 120}},
		{"instancePath": []string{}, "itemId": "title", "space": editor.Object{"width": 1920, "height": 1080}, "bounds": editor.Object{"x": 760, "y": 390, "width": 400, "height": 300}},
	}
	coverage := editor.Object{"version": 1, "output": editor.Object{"width": 1920, "height": 1080, "fps": 30}, "renderedFrames": 300, "entries": entries}
	saved := regionProject([]editor.Object{words, title}, editor.Object{"keyframes": []editor.Object{{"itemId": "words"}}})
	regions := regionDerive(t, saved, regionEncoded, 0, coverage)
	foundWords, foundTitle := false, false
	for _, r := range regions {
		foundWords = foundWords || (r.X == 1000 && r.Y == 700 && r.Width == 180 && r.Height == 120)
		foundTitle = foundTitle || (r.Y <= 390 && r.Y+r.Height >= 690)
		if r.Width*r.Height >= 1920*1080/2 {
			t.Fatalf("painted detail expanded to full frame %+v", r)
		}
	}
	if !foundWords || !foundTitle {
		t.Fatalf("missing painted bounds %+v", regions)
	}
	if _, err := DeriveExportRegions(saved, regionEncoded, 0, nil); err == nil || !strings.Contains(err.Error(), "Thiếu vùng chữ") {
		t.Fatalf("missing coverage accepted: %v", err)
	}
	entries[0]["bounds"] = nil
	if got := regionDerive(t, saved, regionEncoded, 0, coverage); len(got) != 1 {
		t.Fatalf("observed empty caption protected %+v", got)
	}
}

func TestRegionsNestedPaintedCoordinatesAndInstanceIdentity(t *testing.T) {
	saved, coverage := regionNestedFixture(true)
	regions := regionDerive(t, saved, regionEncoded, 0, coverage)
	if len(regions) != 1 || regions[0].X != 660 || regions[0].Y != 430 || regions[0].Width != 100 || regions[0].Height != 20 {
		t.Fatalf("nested caption %+v", regions)
	}
	coverage["entries"].([]editor.Object)[0]["instancePath"] = []string{"another-wrapper"}
	if _, err := DeriveExportRegions(saved, regionEncoded, 0, coverage); err == nil || !strings.Contains(err.Error(), "Thiếu vùng chữ") {
		t.Fatalf("wrong instance accepted: %v", err)
	}
}

func TestRegionFilterExactNumericSyntaxAndValidation(t *testing.T) {
	got, err := ExportRegionFilter([]ExportRegion{{12, 2.125, 30, 40, 0, 2}, {0, 3, 50, 60, 1, 2}})
	want := "addroi=x=12:y=2.125:w=30:h=40:qoffset=-0.25:clear=1,\naddroi=x=0:y=3:w=50:h=60:qoffset=-0.25:clear=0"
	if err != nil || got != want {
		t.Fatalf("filter %q, %v", got, err)
	}
	for _, r := range []ExportRegion{{0, 0, 0, 1, 0, 1}, {0, 0, 1, 1, 1, 1}, {-1, 0, 1, 1, 0, 1}, {0, 0, 1, 1, -1, 1}, {0, 0, 1, math.Inf(1), 0, 1}} {
		if _, err := ExportRegionFilter([]ExportRegion{r}); err == nil {
			t.Errorf("invalid region accepted %+v", r)
		}
	}
	if _, err := ExportRegionFilter(make([]ExportRegion, MaxExportRegions+1)); err == nil {
		t.Fatal("unbounded filter accepted")
	}
}

func TestRegionsVirtualTranscriptAndReverseSuppression(t *testing.T) {
	clip := regionItem("clip", editor.Object{"type": "video", "mediaId": "media", "from": 0, "durationInFrames": 120, "sourceStart": 60, "sourceFps": 30, "speed": 2, "transcriptCaptions": editor.Object{"type": "transcript", "mediaId": "media", "enabled": true, "cues": []editor.Object{{"id": "cue", "startSeconds": 3, "endSeconds": 5, "text": "Hello"}}}})
	p := regionProject([]editor.Object{clip}, nil)
	coverage := editor.Object{"entries": []editor.Object{{"instancePath": []string{}, "itemId": "virtual-transcript-clip", "space": editor.Object{"width": 1920, "height": 1080}, "bounds": editor.Object{"x": 100, "y": 200, "width": 300, "height": 50}}}}
	regionRequire(t, regionDerive(t, p, regionEncoded, 0, coverage), []ExportRegion{{100, 200, 300, 50, 0.5, 1.5}})
	if _, err := DeriveExportRegions(p, regionEncoded, 0, nil); err == nil {
		t.Fatal("virtual transcript missing measured bounds accepted")
	}
	clip["isReversed"] = true
	caption := regionItem("source-caption", editor.Object{"type": "text", "captionSource": editor.Object{"clipId": "clip"}})
	p.Timeline()["items"] = []editor.Object{clip, caption}
	if got := regionDerive(t, p, regionEncoded, 0, nil); len(got) != 0 {
		t.Fatalf("reversed transcript protected %+v", got)
	}
	clip["reverseConformStatus"] = "ready"
	clip["reverseConformSrc"] = "render.mp4"
	if _, err := DeriveExportRegions(p, regionEncoded, 0, nil); err == nil {
		t.Fatal("export-conformed forward caption bypassed coverage")
	}
}

func TestRegionsRejectMalformedPaintedEvidence(t *testing.T) {
	saved := regionProject([]editor.Object{regionItem("title", editor.Object{"type": "text"})}, nil)
	for _, entry := range []editor.Object{
		{"instancePath": []any{1}, "itemId": "title", "bounds": nil},
		{"instancePath": []string{}, "itemId": "title"},
		{"instancePath": []string{}, "itemId": "title", "bounds": "not a rectangle"},
		{"instancePath": []string{}, "itemId": "title", "bounds": editor.Object{"x": 0, "y": 0, "width": -1, "height": 2}},
	} {
		entry["space"] = editor.Object{"width": 1920, "height": 1080}
		if _, err := DeriveExportRegions(saved, regionEncoded, 0, editor.Object{"entries": []editor.Object{entry}}); err == nil {
			t.Fatalf("malformed painted evidence accepted: %+v", entry)
		}
	}
}
