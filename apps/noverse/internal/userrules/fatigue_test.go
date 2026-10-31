package userrules

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func findFatigue(t *testing.T, words []rules.FatigueWord, word string) rules.FatigueWord {
	t.Helper()
	for _, entry := range words {
		if entry.Word == word {
			return entry
		}
	}
	t.Fatalf("missing word %q in %+v", word, words)
	return rules.FatigueWord{}
}

func writeFatigueRules(t *testing.T, dir string, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rules.ManagedRulesFile), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEffectiveFatigueFollowsExistingRuleApplyAndDelete(t *testing.T) {
	st := store.NewStore(t.TempDir())
	project, story := t.TempDir(), t.TempDir()
	const projectRule = "“như một” tối đa 4 lần mỗi chương."
	const storyRule = "“như một” tối đa 5 lần mỗi chương."
	writeFatigueRules(t, project, "- "+projectRule+"\n")
	writeFatigueRules(t, story, "- "+storyRule+"\n")
	cache := map[string]store.NormCacheEntry{
		cacheKey(projectRule): {Structured: rules.Structured{FatigueWords: map[string]rules.FatiguePer1000{"như một": 2.67}}},
		cacheKey(storyRule):   {Structured: rules.Structured{FatigueWords: map[string]rules.FatiguePer1000{"như một": 3.33}}},
	}
	if err := st.UserRules.PutNormCache(cache); err != nil {
		t.Fatal(err)
	}
	// Freeze an empty pack lesson layer: this test needs no global catalog.
	if _, err := st.UserRules.CaptureLessonRules(nil); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, rules.LoadOptions{ProjectRulesDir: project, StoryRulesDir: story})
	if _, err := svc.Apply(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	assertWord := func(source string, chapter int) {
		t.Helper()
		effective, err := svc.Effective(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(effective.FatigueWords) != 16 || len(effective.ForbiddenDefaults) != 4 {
			t.Fatalf("missing immutable defaults: %+v", effective)
		}
		if got := findFatigue(t, effective.FatigueWords, "như một"); got.Source != source || got.PerChapter != chapter {
			t.Fatalf("got %+v; want source %q, chapter %d", got, source, chapter)
		}
	}
	assertWord("story", 5)
	// Editing files without Apply must not invent a new effective threshold.
	writeFatigueRules(t, story, "")
	assertWord("story", 5)
	if _, err := svc.Apply(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	assertWord("project", 4)
	if err := st.UserRules.AddRuntime(store.RuntimeRule{ID: "fatigue-learned", Text: "“như một” tối đa 0 lần mỗi chương.", Candidate: rules.Candidate{
		Structured: rules.Structured{FatigueWords: map[string]rules.FatiguePer1000{"như một": 0}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	assertWord("learned", 0)
	if err := svc.DropRuntime("fatigue-learned"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	assertWord("project", 4)
	writeFatigueRules(t, project, "")
	if _, err := svc.Apply(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	assertWord("default", 3)
}

func TestFileEffectiveFatigueAndForbiddenDefaults(t *testing.T) {
	project := t.TempDir()
	const text = "“như một” tối đa 4 lần mỗi chương."
	writeFatigueRules(t, project, "- "+text+"\n")
	pack := rules.Candidate{Source: "genre_pack", Structured: rules.Structured{FatigueWords: map[string]rules.FatiguePer1000{"dường như": 0.67, "như một": 5}}}
	reader := NewFileRules(nil, rules.LoadOptions{ProjectRulesDir: project}, "", pack)
	if err := reader.sink.PutNormCache(map[string]store.NormCacheEntry{cacheKey(text): {
		Structured: rules.Structured{FatigueWords: map[string]rules.FatiguePer1000{"như một": 2.67}, ForbiddenPhrases: []string{"cụm riêng"}},
	}}); err != nil {
		t.Fatal(err)
	}
	effective, err := reader.Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := findFatigue(t, effective.FatigueWords, "như một"); got.Source != "project" || got.PerChapter != 4 || got.Per1000 != 2.67 {
		t.Fatalf("incorrect project threshold: %+v", got)
	}
	if got := findFatigue(t, effective.FatigueWords, "dường như"); got.Source != "pack" || got.PerChapter != 1 || effective.Snapshot.Structured.FatigueWords["dường như"] != 0.67 {
		t.Fatalf("project effective snapshot must include pack thresholds: %+v", got)
	}
	if len(effective.FatigueWords) != 16 || !reflect.DeepEqual(effective.ForbiddenDefaults, rules.SystemDefaults().Structured.ForbiddenPhrases) || len(effective.Snapshot.Structured.ForbiddenPhrases) != 5 {
		t.Fatalf("custom rules must not alter four forbidden defaults: %+v", effective)
	}
	data, err := json.Marshal(effective)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		FatigueWords      []rules.FatigueWord `json:"fatigueWords"`
		ForbiddenDefaults []string           `json:"forbiddenDefaults"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.FatigueWords) != 16 || len(wire.ForbiddenDefaults) != 4 {
		t.Fatalf("missing response contract: %s, %v", data, err)
	}
	writeFatigueRules(t, project, "")
	reset, err := reader.Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(reset.Items) != 0 || findFatigue(t, reset.FatigueWords, "như một").Source != "pack" || reset.Snapshot.Structured.FatigueWords["như một"] != 5 {
		t.Fatalf("deleting the existing rule must restore defaults: %+v", reset)
	}
}

func TestEffectiveRelabelsDefaultLessonsWithoutGlobalItems(t *testing.T) {
	st := store.NewStore(t.TempDir())
	lessons := []rules.Candidate{
		{Source: "global:lessons", Preferences: "bài học mặc định đã giữ"},
		{Source: "pack:bao-thu:lessons", Preferences: "bài học của gói"},
	}
	if _, err := st.UserRules.CaptureLessonRules(lessons); err != nil {
		t.Fatal(err)
	}
	snapshot := rules.BuildSnapshot(append([]rules.Candidate{rules.SystemDefaults()}, lessons...))
	if err := st.UserRules.Save(&snapshot); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, nil, rules.LoadOptions{})
	effective, err := svc.Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(effective.Items) != 1 || effective.Items[0].Scope != "pack" || strings.Contains(effective.Snapshot.Preferences, "global:lessons") || !strings.Contains(effective.Snapshot.Preferences, "bài học mặc định đã giữ") || !strings.Contains(effective.Snapshot.Preferences, "bài học của gói") {
		t.Fatalf("default lessons must remain effective without global rule items: %+v", effective)
	}
	captured, err := CaptureLessons(st, "bao-thu")
	if err != nil || len(captured) != 2 || captured[0].Source != "default:lessons" || captured[1].Source != "pack:bao-thu:lessons" {
		t.Fatalf("frozen default lessons must be relabeled without losing pack lessons: %+v, %v", captured, err)
	}
	context, err := LessonContext(st, "bao-thu")
	if err != nil || !strings.Contains(context, "bài học mặc định đã giữ") || !strings.Contains(context, "bài học của gói") {
		t.Fatalf("architect must retain default and pack lessons: %q, %v", context, err)
	}
	applied, err := svc.Apply(t.Context(), "")
	if err != nil || strings.Contains(applied.Snapshot.Preferences, "global:lessons") || !strings.Contains(applied.Snapshot.Preferences, "bài học mặc định đã giữ") || !strings.Contains(applied.Snapshot.Preferences, "bài học của gói") {
		t.Fatalf("Apply must retain defaults under the new label: %+v, %v", applied, err)
	}
}

func TestToCandidateFatigueAllowsZeroToNineAndRejectsInvalidDensity(t *testing.T) {
	for chapter := 0; chapter <= 9; chapter++ {
		out := normalizerOutput{Structured: normalizerStructured{FatigueWords: []fatigueWordEntry{{Word: "như một", MaxPerChapter: &chapter}}}}
		candidate, err := out.toCandidate("story")
		if err != nil || candidate.Structured.FatigueWords["như một"].MaxPerChapter() != chapter {
			t.Fatalf("chapter threshold %d lost: %+v, %v", chapter, candidate, err)
		}
	}
	zero, nine := 0.0, 9
	candidate, err := (normalizerOutput{Structured: normalizerStructured{FatigueWords: []fatigueWordEntry{{Word: "như một", Per1000: &zero, MaxPerChapter: &nine}}}}).toCandidate("story")
	if err != nil || candidate.Structured.FatigueWords["như một"] != 0 {
		t.Fatalf("explicit zero must win: %+v, %v", candidate, err)
	}
	for _, invalid := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, err := (normalizerOutput{Structured: normalizerStructured{FatigueWords: []fatigueWordEntry{{Word: "như một", Per1000: &invalid, MaxPerChapter: &nine}}}}).toCandidate("story"); err == nil {
			t.Fatalf("invalid density %v must fail rather than fall through to legacy threshold", invalid)
		}
	}
}

func TestFileEffectiveFatigueHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := NewFileRules(nil, rules.LoadOptions{}, "").Effective(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request must fail: %v", err)
	}
}

func TestHistoricalSnapshotSourcesMigrateWithoutChangingThresholds(t *testing.T) {
	st := store.NewStore(t.TempDir())
	snapshot := rules.BuildSnapshot([]rules.Candidate{
		rules.SystemDefaults(),
		{Source: "global:legacy.md", Preferences: "Giữ giọng văn mộc mạc", Structured: rules.Structured{FatigueWords: map[string]rules.FatiguePer1000{"như một": 0}}},
		{Source: "global:lessons", Preferences: "Bài học đã giữ"},
	})
	if err := st.UserRules.Save(&snapshot); err != nil {
		t.Fatal(err)
	}
	effective, err := NewService(st, nil, rules.LoadOptions{}).Effective(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effective.Snapshot.Structured, snapshot.Structured) || effective.Snapshot.FatigueSources["như một"] != "project:legacy.md" {
		t.Fatalf("source migration must not change thresholds: %+v", effective.Snapshot)
	}
	data, err := json.Marshal(effective)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "global:") || !strings.Contains(effective.Snapshot.Preferences, "## [project:legacy.md]") || !strings.Contains(effective.Snapshot.Preferences, "## [default:lessons]") {
		t.Fatalf("historical global labels must disappear without dropping content: %s", data)
	}
}
