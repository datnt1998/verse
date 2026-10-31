package backdrop

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestManifestParsingLegacyAndCorruption(t *testing.T) {
	legacy := []byte(`{"version":1,"revision":"legacy","salt":0,"wantedCount":2,"fallback":"none","warnings":[],"items":[]}`)
	m, err := Parse(legacy)
	if err != nil || m.TargetSec != nil || m.WantedCount != 2 {
		t.Fatal(m, err)
	}
	for _, raw := range []string{`{`, `null`, `{}`, `{"version":2,"revision":"x"}`, `{"version":1,"revision":"x","wantedCount":41,"fallback":"none"}`, `{"version":1,"revision":"x","wantedCount":1,"targetSec":-1,"fallback":"none"}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatalf("accepted corrupt manifest: %s", raw)
		}
	}
	v := testItem("b1", "v1", "random", "pending")
	m = Seal(Manifest{WantedCount: 1, Fallback: "none", Items: []Item{v, v}})
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw); err == nil {
		t.Fatal("duplicate ids accepted")
	}
}
func TestManifestContentHashAndMonotonicIDs(t *testing.T) {
	m := Seal(Manifest{WantedCount: 1, Fallback: "none", Items: []Item{testItem("b7", "v1", "random", "ready")}})
	same := Seal(m)
	if same.Revision != m.Revision || len(m.Revision) != 16 {
		t.Fatal(m, same)
	}
	changed := m
	changed.Salt++
	changed = Seal(changed)
	if changed.Revision == m.Revision {
		t.Fatal("salt absent from revision")
	}
	if NextItemID([]Item{testItem("b2", "v", "owner", "pending"), testItem("other", "v", "owner", "pending"), testItem("b9", "v", "owner", "pending")}) != "b10" {
		t.Fatal("ids reused")
	}
	raw, err := json.Marshal(Checkpoint{Manifest: m})
	if err != nil {
		t.Fatal(err)
	}
	round, err := ParseCheckpoint(raw)
	if err != nil || !reflect.DeepEqual(round.Manifest, m) {
		t.Fatal(round, err)
	}
}
func TestPlannedSecondsReadyIDsAndShortfallWarnings(t *testing.T) {
	a := testItem("b1", "a", "random", "ready")
	b := testItem("b2", "b", "random", "pending")
	failed := testItem("b3", "c", "random", "failed")
	if PlannedSec([]Item{a, b, failed}) != 60 {
		t.Fatal("failed contributes coverage")
	}
	dup := a
	dup.ID = "b4"
	m := Seal(Manifest{WantedCount: 3, Fallback: "none", Items: []Item{a, dup, b, failed}})
	data := StepData(m)
	if data["readyCount"] != 1 || !reflect.DeepEqual(data["mediaIds"], []string{*a.MediaID}) {
		t.Fatal(data)
	}
	if got := ShortfallWarnings(nil, []Item{a}, 3, nil); !reflect.DeepEqual(got, []string{"Lấy được 1/3 clip nền"}) {
		t.Fatal(got)
	}
	if got := ShortfallWarnings([]string{"Lấy được 1/3 clip nền", "other"}, []Item{a}, 1, nil); !reflect.DeepEqual(got, []string{"other"}) {
		t.Fatal(got)
	}
	if got := ShortfallWarnings(nil, []Item{a, b}, 2, nil); len(got) != 0 {
		t.Fatal(got)
	}
	if got := ShortfallWarnings(nil, []Item{a}, 3, new(600)); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestInitialGoalsRoundAndPreserveModes(t *testing.T) {
	for _, speed := range []float64{.5, 2} {
		template := Template{Backdrop: recipe(nil, 60, "twitch:a"), FixedSec: 10 / speed, PlaybackSpeed: speed}
		m, note := InitialManifest(template, new(20.25))
		want := 1
		if speed == 2 {
			want = 31
		}
		if m.TargetSec == nil || *m.TargetSec != want || note != "" {
			t.Fatal(m, note)
		}
		m, _ = InitialManifest(template, new(1.0))
		if *m.TargetSec != 0 {
			t.Fatal(m)
		}
		m, note = InitialManifest(template, nil)
		if m.TargetSec != nil || m.WantedCount != 10 || note == "" {
			t.Fatal(m, note)
		}
		template.Backdrop.ClipCount = new(3)
		m, _ = InitialManifest(template, new(300.0))
		if m.TargetSec != nil || m.WantedCount != 3 {
			t.Fatal(m)
		}
	}
	for _, data := range []map[string]any{nil, {"durationSec": 0.0}, {"durationSec": -2.0}, {"durationSec": "30"}} {
		if DownloadedVideoSec(data) != nil {
			t.Fatal(data)
		}
	}
}
