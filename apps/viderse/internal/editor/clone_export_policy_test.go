package editor

import (
	"math"
	"testing"
)

func captionPolicyFixture() (Project, Object, Object) {
	caption := Object{"id": "caption", "type": "text", "trackId": "text", "from": 0, "durationInFrames": 75}
	wrapper := func(id string) Object {
		return Object{"id": id, "type": "composition", "trackId": "text", "from": 0, "durationInFrames": 75, "compositionId": "nested", "compositionWidth": 640, "compositionHeight": 360}
	}
	p := Project{"metadata": Object{"width": 3840, "height": 2160, "fps": 25}, "timeline": Object{"tracks": []any{Object{"id": "text", "visible": true}}, "items": []any{caption, wrapper("left"), wrapper("right")}, "compositions": []any{Object{"id": "nested", "width": 640, "height": 360, "fps": 25, "tracks": []any{Object{"id": "text", "visible": false}}, "items": []any{Object{"id": "nested-caption", "type": "text", "trackId": "text", "from": 0, "durationInFrames": 75}}}}}}
	entry := func(path []any, id string, w, h int, bounds any) Object {
		return Object{"instancePath": path, "itemId": id, "space": Object{"width": w, "height": h}, "bounds": bounds}
	}
	coverage := Object{"version": 1, "output": Object{"width": 1920, "height": 1080, "fps": 30}, "renderedFrames": 90, "entries": []any{entry([]any{}, "caption", 3840, 2160, Object{"x": 800, "y": 1700, "width": 2200, "height": 180}), entry([]any{"left"}, "nested-caption", 640, 360, nil), entry([]any{"right"}, "nested-caption", 640, 360, nil)}}
	return p, Object{"projectRevision": "sha256:bound"}, Object{"captionCoverage": Object{"projectRevision": "sha256:bound", "coverage": coverage}}
}

func TestCaptionCoverageCompleteRevisionBoundManifest(t *testing.T) {
	p, session, metadata := captionPolicyFixture()
	if _, err := validateExportCaptionCoverage(metadata, session, p, "sha256:bound"); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(Object){
		"revision":        func(m Object) { exportObject(m["captionCoverage"])["projectRevision"] = "sha256:other" },
		"dimensions":      func(m Object) { exportObject(exportCoverage(m)["output"])["width"] = 1280 },
		"fps":             func(m Object) { exportObject(exportCoverage(m)["output"])["fps"] = 25 },
		"partial render":  func(m Object) { exportCoverage(m)["renderedFrames"] = 89 },
		"partial entries": func(m Object) { exportCoverage(m)["entries"] = exportArray(exportCoverage(m)["entries"])[:2] },
		"duplicate":       func(m Object) { c := exportCoverage(m); a := exportArray(c["entries"]); c["entries"] = append(a, a[0]) },
		"unknown item":    func(m Object) { exportObject(exportArray(exportCoverage(m)["entries"])[0])["itemId"] = "unknown" },
		"unknown wrapper": func(m Object) {
			exportObject(exportArray(exportCoverage(m)["entries"])[1])["instancePath"] = []any{"unknown"}
		},
		"nested space": func(m Object) {
			exportObject(exportObject(exportArray(exportCoverage(m)["entries"])[1])["space"])["width"] = 1920
		},
		"outside": func(m Object) {
			exportObject(exportObject(exportArray(exportCoverage(m)["entries"])[0])["bounds"])["width"] = 5000
		},
		"nonfinite": func(m Object) {
			exportObject(exportObject(exportArray(exportCoverage(m)["entries"])[0])["bounds"])["x"] = math.Inf(1)
		},
		"unknown fields": func(m Object) { exportCoverage(m)["untrusted"] = true },
		"zero": func(m Object) {
			exportObject(exportObject(exportArray(exportCoverage(m)["entries"])[0])["bounds"])["height"] = 0
		},
		"depth": func(m Object) {
			a := make([]any, 33)
			for i := range a {
				a[i] = "left"
			}
			exportObject(exportArray(exportCoverage(m)["entries"])[1])["instancePath"] = a
		},
		"count": func(m Object) {
			a := make([]any, 10001)
			for i := range a {
				a[i] = exportArray(exportCoverage(m)["entries"])[0]
			}
			exportCoverage(m)["entries"] = a
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, m := captionPolicyFixture()
			mutate(m)
			if _, err := validateExportCaptionCoverage(m, session, p, "sha256:bound"); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
	if coverage, err := validateExportCaptionCoverage(nil, session, p, "sha256:bound"); err != nil || coverage != nil {
		t.Fatalf("legacy coverage = %v, %v", coverage, err)
	}
}

func TestCaptionRenderIdentitiesVirtualReverseSoloOverlap(t *testing.T) {
	source := Object{"id": "source", "type": "audio", "trackId": "audio", "from": 0, "durationInFrames": 90, "mediaId": "media", "sourceFps": 30, "transcriptCaptions": Object{"type": "transcript", "enabled": true, "mediaId": "media", "cues": []any{Object{"id": "cue", "startSeconds": 0, "endSeconds": 3, "text": "Caption"}}}}
	timeline := Object{"tracks": []any{Object{"id": "audio", "visible": true}, Object{"id": "captions", "visible": true}}, "items": []any{source}}
	identities := func() []Object {
		items, err := exportRenderCaptionItems(timeline, Object{"width": 1920, "height": 1080, "fps": 30}, true)
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	hasVirtual := func() bool {
		for _, i := range identities() {
			if i["id"] == "virtual-transcript-source" {
				return true
			}
		}
		return false
	}
	if !hasVirtual() {
		t.Fatal("virtual caption missing")
	}
	source["isReversed"] = true
	timeline["items"] = []any{source, Object{"id": "generated", "type": "text", "trackId": "captions", "from": 0, "durationInFrames": 90, "captionSource": Object{"clipId": "source"}}, Object{"id": "subtitle", "type": "subtitle", "trackId": "captions", "from": 0, "durationInFrames": 90, "source": Object{"type": "transcript", "clipId": "source"}}}
	if got := len(identities()); got != 1 {
		t.Fatalf("reverse retained %d items", got)
	}
	delete(source, "isReversed")
	timeline["items"] = []any{source}
	source["sourceStart"] = 90
	source["sourceEnd"] = 180
	if hasVirtual() {
		t.Fatal("nonoverlapping cue produced caption")
	}
	delete(source, "sourceStart")
	delete(source, "sourceEnd")
	timeline["tracks"] = append(exportArray(timeline["tracks"]), Object{"id": "solo", "solo": true})
	if hasVirtual() {
		t.Fatal("non-solo cue produced caption")
	}
}

func exportCoverage(m Object) Object {
	return exportObject(exportObject(m["captionCoverage"])["coverage"])
}

func TestCaptionCoverageVirtualRepeatedCompositionsAndRootConformance(t *testing.T) {
	for _, scenario := range []string{"virtual", "reverse", "hidden-group", "solo-group", "resized-definition"} {
		t.Run(scenario, func(t *testing.T) {
			p, session, metadata := captionPolicyFixture()
			timeline := p.Timeline()
			nested := exportObject(exportArray(timeline["compositions"])[0])
			exportObject(exportArray(nested["tracks"])[0])["visible"] = true
			source := func(id string) Object {
				return Object{"id": id, "type": "audio", "trackId": "text", "from": 0, "durationInFrames": 75, "mediaId": "media", "sourceFps": 25, "transcriptCaptions": Object{"type": "transcript", "enabled": true, "mediaId": "media", "cues": []any{Object{"id": "cue", "startSeconds": 0, "endSeconds": 3, "text": "Virtual"}}}}
			}
			rootSource := source("source")
			nestedSource := source("nested-source")
			timeline["items"] = append(exportArray(timeline["items"]), rootSource)
			nested["items"] = append(exportArray(nested["items"]), nestedSource)
			includeRoot := true
			if scenario == "reverse" {
				rootSource["type"] = "video"
				rootSource["isReversed"] = true
				rootSource["reverseConformStatus"] = "ready"
				rootSource["reverseConformSrc"] = "asset://reverse"
				rootSource["reverseConformLocalStart"] = 0
			}
			if scenario == "hidden-group" || scenario == "solo-group" {
				group := Object{"id": "group", "isGroup": true, "visible": scenario != "hidden-group", "solo": scenario == "solo-group"}
				track := exportObject(exportArray(timeline["tracks"])[0])
				track["parentTrackId"] = "group"
				timeline["tracks"] = []any{group, track}
				includeRoot = scenario != "hidden-group"
			}
			if scenario == "resized-definition" {
				nested["width"] = 1280
				nested["height"] = 720
			}
			entries := exportArray(exportCoverage(metadata)["entries"])
			entry := func(path []any, id string, w, h int) Object {
				return Object{"instancePath": path, "itemId": id, "space": Object{"width": w, "height": h}, "bounds": nil}
			}
			if includeRoot {
				entries = append(entries, entry([]any{}, "virtual-transcript-source", 3840, 2160))
			}
			entries = append(entries, entry([]any{"left"}, "virtual-transcript-nested-source", 640, 360), entry([]any{"right"}, "virtual-transcript-nested-source", 640, 360))
			exportCoverage(metadata)["entries"] = entries
			if _, e := validateExportCaptionCoverage(metadata, session, p, "sha256:bound"); e != nil {
				t.Fatal(e)
			}
			exportCoverage(metadata)["entries"] = entries[:len(entries)-1]
			if _, e := validateExportCaptionCoverage(metadata, session, p, "sha256:bound"); e == nil {
				t.Fatal("incomplete virtual coverage accepted")
			}
		})
	}
}

func TestCaptionCoverageRejectsAmbiguousAndCyclicSavedCompositions(t *testing.T) {
	for _, scenario := range []string{"duplicate-compositions", "duplicate-items", "cycle", "missing", "invalid-subtitle"} {
		t.Run(scenario, func(t *testing.T) {
			p, session, metadata := captionPolicyFixture()
			timeline := p.Timeline()
			nested := exportObject(exportArray(timeline["compositions"])[0])
			switch scenario {
			case "duplicate-compositions":
				timeline["compositions"] = append(exportArray(timeline["compositions"]), nested)
			case "duplicate-items":
				timeline["items"] = append(exportArray(timeline["items"]), exportArray(timeline["items"])[0])
			case "cycle":
				nested["items"] = append(exportArray(nested["items"]), Object{"id": "cycle", "type": "composition", "compositionId": "nested", "trackId": "text", "from": 0, "durationInFrames": 75, "compositionWidth": 640, "compositionHeight": 360})
			case "missing":
				exportObject(exportArray(timeline["items"])[1])["compositionId"] = "missing"
			case "invalid-subtitle":
				exportObject(exportArray(timeline["items"])[0])["type"] = "subtitle"
			}
			if _, e := validateExportCaptionCoverage(metadata, session, p, "sha256:bound"); e == nil {
				t.Fatal("unresolvable saved timeline accepted")
			}
		})
	}
}
