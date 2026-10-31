package editor

import (
	"testing"
)

func TestApplyCommandsRejectsUnknownAndRollsBackEntireBatch(t *testing.T) {
	input := fillGoldenFixtures(t)[0].Input
	before := fillJSON(t, input.Project)
	_, err := ApplyCommands(input.Project, []Command{{Type: "track.update", Params: Object{"id": "t-scenes", "updates": Object{"locked": true}}}, {Type: "template.subscribe", Params: Object{}}})
	if err == nil {
		t.Fatal("unknown commands must fail, not silently succeed")
	}
	fillAssertJSON(t, input.Project, before)
}
func TestApplyCommandsReplacesSameFrameWithoutFlatteningCanonicalLanes(t *testing.T) {
	input := fillGoldenFixtures(t)[0].Input
	input.Project.Timeline()["items"] = []Object{{"id": "image", "type": "image", "trackId": "t-scenes", "label": "Image", "from": 0, "durationInFrames": 1}}
	project, err := ApplyCommands(input.Project, []Command{{Type: "keyframe.add", Params: Object{"itemId": "image", "property": "width", "frame": 0, "value": 100, "easing": "linear"}}, {Type: "keyframe.add", Params: Object{"itemId": "image", "property": "width", "frame": 0, "value": 112, "easing": "linear"}}, {Type: "keyframe.add", Params: Object{"itemId": "image", "property": "height", "frame": 0, "value": 50, "easing": "linear"}}})
	if err != nil {
		t.Fatal(err)
	}
	groups := fillObjects(project.Timeline()["keyframes"])
	if len(groups) != 1 || groups[0]["itemId"] != "image" || fillNum(groups[0], "animationVersion", 0) != 2 {
		t.Fatal("keyframes are not canonical item groups")
	}
	lanes := fillObjects(groups[0]["properties"])
	if len(lanes) != 2 {
		t.Fatal("property lanes were flattened")
	}
	keys := fillObjects(lanes[0]["keyframes"])
	if len(keys) != 1 || fillNum(keys[0], "value", 0) != 112 || keys[0]["id"] != "editor-generated-1" {
		t.Fatalf("same-frame replacement lost its identity: %+v", keys)
	}
}
func TestApplyCommandsRemovesOnlyExactItemsAndCascadesTheirAnimation(t *testing.T) {
	input := fillGoldenFixtures(t)[1].Input
	before := fillJSON(t, input.Project)
	project, err := ApplyCommands(input.Project, []Command{{Type: "item.removeExact", Params: Object{"itemIds": []string{"tpl-sub-text", "scene-prototype"}}}})
	if err != nil {
		t.Fatal(err)
	}
	shapeFound := false
	for _, item := range fillObjects(project.Timeline()["items"]) {
		id := fillString(item, "id")
		if id == "tpl-sub-text" || id == "scene-prototype" {
			t.Fatal("exact removal retained requested item")
		}
		shapeFound = shapeFound || id == "tpl-sub-shape"
	}
	if !shapeFound {
		t.Fatal("exact removal expanded linked group")
	}
	if len(fillObjects(project.Timeline()["transitions"])) != 0 || len(fillObjects(project.Timeline()["keyframes"])) != 0 {
		t.Fatal("removed items retained transitions or keyframes")
	}
	fillAssertJSON(t, input.Project, before)
}
func TestFillWaveformRejectsMismatchedSourceAndMalformedEnvelope(t *testing.T) {
	for _, kind := range []string{"hash", "samples", "mime"} {
		t.Run(kind, func(t *testing.T) {
			input := fillGoldenFixtures(t)[1].Input
			switch kind {
			case "hash":
				input.Envelope["sourceHash"] = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "samples":
				input.Envelope["envelopeBase64"] = "AA=="
			case "mime":
				for _, m := range input.Media {
					if fillString(m, "id") == "new-audio" {
						m["mimeType"] = "video/mp4"
					}
				}
			}
			before := fillJSON(t, input.Project)
			if _, err := Fill(input); err == nil {
				t.Fatal("invalid waveform binding accepted")
			}
			fillAssertJSON(t, input.Project, before)
		})
	}
}
func TestApplyCommandsRetainsUnchangedProjectAndTimelineFields(t *testing.T) {
	input := fillGoldenFixtures(t)[1].Input
	input.Project.Timeline()["markers"] = []Object{{"id": "marker", "frame": 15, "label": "Keep"}}
	input.Project.Timeline()["compositions"] = []Object{{"id": "nested", "items": []any{}, "tracks": []any{}, "custom": "retain"}}
	result, err := Fill(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"authored", "metadata", "createdAt", "updatedAt", "schemaVersion"} {
		fillAssertJSON(t, result.Project[key], input.Project[key])
	}
	for _, key := range []string{"markers", "compositions", "customField", "busAudioEq"} {
		fillAssertJSON(t, result.Project.Timeline()[key], input.Project.Timeline()[key])
	}
}
