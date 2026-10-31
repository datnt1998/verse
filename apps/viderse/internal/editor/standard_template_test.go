package editor

import (
	"reflect"
	"testing"
	"time"
)

// Ported from channels/standard-template.test.ts at yt-factory 2c201e6:
// real slot prototypes, waveform binding, animated Subscribe and looping snow.
func TestStandardTemplateOwnsCanonicalEditableLayout(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	project, err := BuildStandardTemplateProject("owner-template", "Mẫu của tôi", now)
	if err != nil {
		t.Fatal(err)
	}
	if project["id"] != "owner-template" || project["name"] != "Mẫu của tôi" || project["createdAt"] != now.UnixMilli() || project["updatedAt"] != now.UnixMilli() || fillNum(Object(project), "schemaVersion", 0) != 15 {
		t.Fatal("standard layout did not get a fresh canonical identity")
	}
	if !reflect.DeepEqual(project.Metadata(), Object{"width": float64(1920), "height": float64(1080), "fps": float64(30)}) {
		t.Fatalf("metadata: %+v", project.Metadata())
	}
	timeline := project.Timeline()
	tracks, items := fillObjects(timeline["tracks"]), fillObjects(timeline["items"])
	if len(tracks) != 11 || len(items) != 12 || fillNum(Object(project), "duration", 0) != 12 {
		t.Fatal("standard template is missing its canonical authored layout")
	}
	for _, role := range []string{"scenes", "captions", "source-audio"} {
		if fillSlot(tracks, role) == nil {
			t.Fatalf("missing %s slot", role)
		}
	}
	var scenes, captions, audio, waveform, snow Object
	subscribe := []Object{}
	for _, item := range items {
		switch fillString(item, "id") {
		case "standard-scene-0":
			scenes = item
		case "standard-captions":
			captions = item
		case "standard-audio":
			audio = item
		case "standard-wave":
			waveform = item
		case "standard-snow":
			snow = item
		}
		if fillString(item, "linkedGroupId") != "" {
			subscribe = append(subscribe, item)
		}
	}
	if fillString(scenes, "type") != "image" || fillNum(scenes, "durationInFrames", 0) != 180 || fillString(captions, "type") != "subtitle" || fillString(captions, "captionPreset") != "karaoke" {
		t.Fatal("canonical scene/caption prototypes were replaced")
	}
	if fillString(waveform, "mediaId") != fillString(audio, "mediaId") || fillNum(fillObject(waveform["waveformVisualizer"]), "samplesPerSecond", 0) != 50 {
		t.Fatal("waveform is not bound to the measured placeholder voice")
	}
	if fillString(snow, "templateAnchor") != "full" || !fillBool(snow, "loop") || !fillBool(snow, "embeddedAudioMuted") {
		t.Fatal("snow must loop silently over the complete clone")
	}
	if len(subscribe) != 6 || len(fillObjects(timeline["keyframes"])) != 6 || len(fillObjects(timeline["transitions"])) != 1 {
		t.Fatal("Subscribe animation or scene transition is missing")
	}
	for _, item := range subscribe {
		anchor := fillString(item, "templateAnchor")
		if anchor != "start" && anchor != "end" {
			t.Fatal("Subscribe lost its authored anchor")
		}
	}
}

func TestStandardTemplateInstancesDoNotShareNestedState(t *testing.T) {
	now := time.Unix(1793448000, 0)
	first, err := BuildStandardTemplateProject("one", "Một", now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildStandardTemplateProject("two", "Hai", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	before := fillJSON(t, second)
	fillObjects(first.Timeline()["items"])[0]["label"] = "Changed"
	fillObjects(first.Timeline()["tracks"])[0]["locked"] = true
	first.Metadata()["width"] = 1280
	fillAssertJSON(t, second, before)
	if _, err := BuildStandardTemplateProject("../bad", "Bad", now); err == nil {
		t.Fatal("invalid project id accepted")
	}
	if _, err := BuildStandardTemplateProject("three", "  ", now); err == nil {
		t.Fatal("blank project name accepted")
	}
}
