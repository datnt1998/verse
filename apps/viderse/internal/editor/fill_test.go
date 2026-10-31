package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

type fillGoldenFixture struct {
	Name     string          `json:"name"`
	Input    FillInput       `json:"input"`
	Expected json.RawMessage `json:"expected"`
}

func fillGoldenFixtures(t *testing.T) []fillGoldenFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/fill-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Source   string              `json:"source"`
		Runtime  string              `json:"runtime"`
		Fixtures []fillGoldenFixture `json:"fixtures"`
	}
	if err = json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Source != "yt-factory@2c201e6" || golden.Runtime != "1.3.11" {
		t.Fatal("wrong canonical golden provenance")
	}
	return golden.Fixtures
}
func fillJSON(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}
func fillAssertJSON(t *testing.T, got, want any) {
	t.Helper()
	a, b := fillJSON(t, got), fillJSON(t, want)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("canonical output differs at %s", fillJSONDifference(a, b, "$"))
	}
}
func fillJSONDifference(a, b any, path string) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: %T != %T", path, a, b)
		}
		for k, v := range x {
			if other, ok := y[k]; !ok {
				return path + "." + k + ": unexpected field"
			} else if d := fillJSONDifference(v, other, path+"."+k); d != "" {
				return d
			}
		}
		for k := range y {
			if _, ok := x[k]; !ok {
				return path + "." + k + ": missing field"
			}
		}
	case []any:
		y, ok := b.([]any)
		if !ok {
			return fmt.Sprintf("%s: %T != %T", path, a, b)
		}
		if len(x) != len(y) {
			return fmt.Sprintf("%s: lengths %d != %d", path, len(x), len(y))
		}
		for i, v := range x {
			if d := fillJSONDifference(v, y[i], fmt.Sprintf("%s[%d]", path, i)); d != "" {
				return d
			}
		}
	}
	return fmt.Sprintf("%s: got %v; want %v", path, a, b)
}

func TestFillMatchesCanonicalEngineGoldens(t *testing.T) {
	for _, fixture := range fillGoldenFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			before := fillJSON(t, fixture.Input)
			result, err := Fill(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			fillAssertJSON(t, result, fixture.Expected)
			fillAssertJSON(t, fixture.Input, before)
		})
	}
}
func TestPrepareCaptionDataMatchesCanonicalGoldens(t *testing.T) {
	data, err := os.ReadFile("testdata/fill-captions-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name     string          `json:"name"`
		Cues     []FillCue       `json:"cues"`
		Expected json.RawMessage `json:"expected"`
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) { fillAssertJSON(t, PrepareCaptionData(fixture.Cues, 30, 180), fixture.Expected) })
	}
}
func TestFillRejectsInvalidBoundaryInputs(t *testing.T) {
	base := fillGoldenFixtures(t)[0].Input
	for _, test := range []struct {
		name   string
		change func(*FillInput)
	}{
		{"empty-prefix", func(i *FillInput) { i.IDPrefix = "" }},
		{"zero-duration", func(i *FillInput) { i.DurationFrames = 0 }},
		{"unsupported-schema", func(i *FillInput) { i.Project["schemaVersion"] = 14 }},
		{"invalid-fps", func(i *FillInput) { i.Project.Metadata()["fps"] = 0 }},
		{"negative-scene", func(i *FillInput) { i.Scenes[0].StartFrame = -1 }},
		{"scene-overrun", func(i *FillInput) { i.Scenes[0].EndFrame = 181 }},
		{"empty-scene", func(i *FillInput) { i.Scenes[0].EndFrame = 0 }},
		{"nan-caption", func(i *FillInput) { i.Cues[0].EndSec = math.NaN() }},
		{"invalid-audio", func(i *FillInput) { i.Audio.DurationFrames = 0 }},
		{"missing-subject-box", func(i *FillInput) {
			fillObjects(i.Project.Timeline()["tracks"])[0]["templateScenes"] = Object{"style": "cutout-close"}
		}},
		{"noncanonical-media", func(i *FillInput) { i.Media[0]["storageType"] = "server" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var input FillInput
			if err = json.Unmarshal(data, &input); err != nil {
				t.Fatal(err)
			}
			test.change(&input)
			if _, err = Fill(input); err == nil {
				t.Fatal("expected boundary rejection")
			}
		})
	}
}
func TestFillLoudnessGainClampAndPrototypeMix(t *testing.T) {
	base := fillGoldenFixtures(t)[1].Input
	for _, pair := range [][2]float64{{-20, 6}, {-5, -9}, {-40, 12}, {5, -12}} {
		input := base
		input.Audio.MeasuredLUFS = &pair[0]
		result, err := Fill(input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Loudness == nil || result.Loudness.AppliedGainDB != pair[1] {
			t.Fatalf("wrong loudness: %+v", result.Loudness)
		}
		for _, item := range fillObjects(result.Project.Timeline()["items"]) {
			if fillString(item, "type") == "audio" && fillNum(item, "volume", 0) != -3+pair[1] {
				t.Fatal("prototype mix not retained")
			}
		}
	}
	base.Audio.MeasuredLUFS = nil
	result, err := Fill(base)
	if err != nil {
		t.Fatal(err)
	}
	if result.Loudness != nil {
		t.Fatal("measurement absent must not normalize")
	}
}
func TestFillBackgroundTilingLongDurationsAndSeedIsolation(t *testing.T) {
	base := fillGoldenFixtures(t)[4].Input
	for _, duration := range []int{900, 30 * 60 * 30, 2 * 60 * 60 * 30} {
		t.Run(fmt.Sprint(duration), func(t *testing.T) {
			input := base
			input.DurationFrames = duration
			input.Scenes = nil
			result, err := BuildTemplateFill(input)
			if err != nil {
				t.Fatal(err)
			}
			cursor := 0.
			last := ""
			count := 0
			for _, command := range result.Commands {
				if command.Type != "item.add" {
					continue
				}
				item := fillObject(command.Params["item"])
				if fillString(item, "trackId") != "t-bg" {
					continue
				}
				if fillNum(item, "from", -1) != cursor || fillNum(item, "durationInFrames", 0) < 1 {
					t.Fatal("background gap or empty tile")
				}
				if fillString(item, "mediaId") == last {
					t.Fatal("cycle repeated previous clip")
				}
				last = fillString(item, "mediaId")
				cursor += fillNum(item, "durationInFrames", 0)
				count++
			}
			if cursor != float64(duration) || count == 0 {
				t.Fatal("background does not cover project")
			}
			again, err := BuildTemplateFill(input)
			if err != nil {
				t.Fatal(err)
			}
			fillAssertJSON(t, result.Commands, again.Commands)
		})
	}
}
func TestFillCaptionPresetPreservesAuthoredNonKaraoke(t *testing.T) {
	for _, preset := range []string{"none", "word-highlight", "word-by-word", "typewriter", "bounce", "pop", "fade"} {
		if EffectiveCaptionPreset(preset, true, true) != preset {
			t.Fatal("authored preset changed")
		}
	}
	if EffectiveCaptionPreset("karaoke", false, true) != "fade" || EffectiveCaptionPreset("karaoke", false, false) != "karaoke" {
		t.Fatal("incorrect karaoke estimate demotion")
	}
}
func TestFillDropsNegativeEndAnchorAndClampsStart(t *testing.T) {
	input := fillGoldenFixtures(t)[1].Input
	for _, item := range fillObjects(input.Project.Timeline()["items"]) {
		if fillString(item, "id") == "tpl-sub-text" {
			item["from"] = 30
		}
	}
	result, err := Fill(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range fillObjects(result.Project.Timeline()["items"]) {
		if fillString(item, "id") == "tpl-sub-text" {
			t.Fatal("negative end anchor retained")
		}
		if fillString(item, "id") == "start-clamp" && fillNum(item, "durationInFrames", 0) != 30 {
			t.Fatal("start anchor not clamped")
		}
	}
	for _, id := range result.ItemIDs.Subscribe {
		if id == "tpl-sub-text" {
			t.Fatal("dropped subscribe id reported")
		}
	}
}
func TestFillRejectsEffectiveBackgroundSpeedOutsideCanonicalRange(t *testing.T) {
	for _, pair := range [][2]float64{{.2, .25}, {4, 3}, {math.NaN(), 2}} {
		input := fillGoldenFixtures(t)[5].Input
		for _, track := range fillObjects(input.Project.Timeline()["tracks"]) {
			if fillString(track, "templateSlot") == "background-video" {
				fillObject(track["templateBackgroundAppearance"])["speed"] = pair[1]
			}
		}
		for _, item := range fillObjects(input.Project.Timeline()["items"]) {
			if fillString(item, "trackId") == "t-bg" {
				item["speed"] = pair[0]
			}
		}
		if _, err := BuildTemplateFill(input); err == nil || !strings.Contains(err.Error(), "0.1–10") {
			t.Fatalf("expected effective speed failure; got %v", err)
		}
	}
}
