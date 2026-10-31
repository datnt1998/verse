package rules

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func fatigueByWord(t *testing.T, words []FatigueWord, word string) FatigueWord {
	t.Helper()
	for _, entry := range words {
		if entry.Word == word {
			return entry
		}
	}
	t.Fatalf("missing fatigue word %q: %+v", word, words)
	return FatigueWord{}
}

func TestEffectiveFatigueTracksWinningWordSource(t *testing.T) {
	pack, err := PackDefaults(manifestSections(`{"fatigue_words":{"dường như":0.67,"im lặng":5,"bỗng nhiên":0.67,"như một":4}}`))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := BuildSnapshot([]Candidate{
		SystemDefaults(),
		{Source: "project:quy-tac.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"như một": 2.67, "im lặng": 3}}},
		{Source: "story:quy-tac.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"im lặng": 1.33, "không khỏi": 0}}},
	})
	before := snapshot.FatigueSources["như một"]
	overlaid := OverlaySnapshot(snapshot, Candidate{Source: "runtime_update", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"như một": 2}}})
	if snapshot.FatigueSources["như một"] != before {
		t.Fatal("overlay mutated the previous snapshot provenance")
	}
	data, err := json.Marshal(overlaid)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded Snapshot
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatal(err)
	}
	words := EffectiveFatigue(&reloaded, pack)
	if len(words) != 16 {
		t.Fatalf("must preserve all 16 defaults: %+v", words)
	}
	for _, want := range []FatigueWord{
		{Word: "dường như", Per1000: 0.67, PerChapter: 1, Source: "pack"},
		{Word: "bỗng nhiên", Per1000: 0.67, PerChapter: 1, Source: "pack"},
		{Word: "im lặng", Per1000: 1.33, PerChapter: 2, Source: "story"},
		{Word: "không khỏi", Per1000: 0, PerChapter: 0, Source: "story"},
		{Word: "như một", Per1000: 2, PerChapter: 3, Source: "learned"},
		{Word: "tuy nhiên", Per1000: 1.33, PerChapter: 2, Source: "default"},
	} {
		if got := fatigueByWord(t, words, want.Word); got != want {
			t.Errorf("word %q: got %+v, want %+v", want.Word, got, want)
		}
	}
	if got := fatigueByWord(t, EffectiveFatigue(&snapshot, pack), "như một"); got.Source != "project" || got.PerChapter != 4 {
		t.Fatalf("project winner lost: %+v", got)
	}
	// The pack remains live: editing it changes only words without explicit rules.
	pack.Structured.FatigueWords["dường như"] = 2
	if got := fatigueByWord(t, EffectiveFatigue(&reloaded, pack), "dường như"); got.PerChapter != 3 || got.Source != "pack" {
		t.Fatalf("live pack update not reflected: %+v", got)
	}
}

func TestEffectiveFatigueDefaultAndMeasurementOnlyPack(t *testing.T) {
	pack, err := PackDefaults(manifestSections(`{"do_do":{"sentence_max":40}}`))
	if err != nil {
		t.Fatal(err)
	}
	defaults := EffectiveFatigue(nil, Candidate{})
	if len(defaults) != 16 || !reflect.DeepEqual(defaults, EffectiveFatigue(nil, pack)) {
		t.Fatalf("measurement-only pack must not claim inherited fatigue words: %+v", EffectiveFatigue(nil, pack))
	}
	for _, entry := range defaults {
		if entry.Source != "default" {
			t.Fatalf("unexpected default source: %+v", entry)
		}
	}
}

func TestFatigueZeroAndChapterValuesRoundTrip(t *testing.T) {
	for chapter := 0; chapter <= 9; chapter++ {
		threshold := FatiguePer1000(MaxPerChapterToPer1000(float64(chapter)))
		data, err := json.Marshal(threshold)
		if err != nil {
			t.Fatal(err)
		}
		var decoded FatiguePer1000
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != threshold || decoded.MaxPerChapter() != chapter {
			t.Errorf("chapter %d lost during round trip: %s -> %v", chapter, data, decoded)
		}
	}
	var zero FatiguePer1000
	if err := json.Unmarshal([]byte(`{"per_1000":0,"max_per_chapter":9}`), &zero); err != nil || zero != 0 {
		t.Fatalf("explicit zero must win over legacy threshold: %v, %v", zero, err)
	}
}

func TestSnapshotFatigueRejectsInvalidButKeepsZero(t *testing.T) {
	snapshot := BuildSnapshot([]Candidate{SystemDefaults(), {Source: "project:quy-tac.md", Structured: Structured{
		FatigueWords: map[string]FatiguePer1000{"im lặng": 0, "dường như": -1, "nan": FatiguePer1000(math.NaN()), "inf": FatiguePer1000(math.Inf(1))},
	}}})
	if got := fatigueByWord(t, EffectiveFatigue(&snapshot, Candidate{}), "im lặng"); got.Per1000 != 0 || got.Source != "project" {
		t.Fatalf("explicit zero lost: %+v", got)
	}
	if len(snapshot.Structured.FatigueWords) != 16 || snapshot.Structured.FatigueWords["dường như"] != 1.33 {
		t.Fatalf("invalid thresholds contaminated snapshot: %+v", snapshot.Structured.FatigueWords)
	}
	if got := FatigueDensities("im lặng", snapshot.Structured.FatigueWords); len(got) != 1 || got[0].Limit != 0 || got[0].Count != 1 {
		t.Fatalf("zero threshold must remain visible in density results: %+v", got)
	}
}

func TestEffectiveFatigueDoesNotGuessAmbiguousHistoricalSource(t *testing.T) {
	snapshot := BuildSnapshot([]Candidate{
		SystemDefaults(),
		{Source: "project:quy-tac.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"như một": 4}}},
		{Source: "story:quy-tac.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"như một": 2}}},
	})
	snapshot.FatigueSources = nil
	word := fatigueByWord(t, EffectiveFatigue(&snapshot, Candidate{}), "như một")
	if word.Source != "" || word.Per1000 != 2 || word.PerChapter != 3 {
		t.Fatalf("legacy ambiguity must not change the threshold or invent a source: %+v", word)
	}
	data, err := json.Marshal(word)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if _, exists := wire["source"]; exists {
		t.Fatalf("unprovable historical source must be omitted: %s", data)
	}
	snapshot.Sources = []string{"system_defaults", "project:quy-tac.md"}
	if got := fatigueByWord(t, EffectiveFatigue(&snapshot, Candidate{}), "như một"); got.Source != "project" {
		t.Fatalf("unique historical scope is provable: %+v", got)
	}
}

func TestEffectiveFatigueResetBaselineKeepsZeroOnWire(t *testing.T) {
	baseline := BuildSnapshot([]Candidate{SystemDefaults(), {
		Source: "project:quy-tac.md",
		Structured: Structured{FatigueWords: map[string]FatiguePer1000{"im lặng": 0}},
	}})
	current := OverlaySnapshot(baseline, Candidate{Source: "story:quy-tac.md", Structured: Structured{FatigueWords: map[string]FatiguePer1000{"im lặng": 4.67}}})
	word := fatigueByWord(t, EffectiveFatigue(&current, Candidate{}, &baseline), "im lặng")
	if word.PerChapter != 7 || word.BasePerChapter == nil || *word.BasePerChapter != 0 || word.BaseSource != "project" {
		t.Fatalf("zero reset baseline must remain explicit: %+v", word)
	}
	data, err := json.Marshal(word)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire["basePerChapter"]) != "0" || string(wire["baseSource"]) != `"project"` {
		t.Fatalf("zero must not be omitted from reset contract: %s", data)
	}
}
