package diag

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

// Exercise the registered rules, not a hand-written list of translated examples.
// Technical input is deliberate: machine identifiers must not reach normal copy.
func TestAllDiagnosticRulesVietnamese(t *testing.T) {
	completed := []int{64, 8, 40, 56}
	for ch := 1; ch <= 63; ch++ {
		if ch != 2 && ch != 8 && ch != 40 && ch != 56 {
			completed = append(completed, ch)
		}
	}
	base := Snapshot{
		Progress: &domain.Progress{
			Phase: domain.PhaseWriting, Flow: domain.FlowRewriting, Layered: true,
			CompletedChapters: completed, TotalChapters: len(completed),
			PendingRewrites:   []int{64, 8, 99, 40},
			ChapterWordCounts: map[int]int{8: 100, 40: 1000, 56: 1000, 64: 8000},
		},
		RunMeta:       &domain.RunMeta{PendingSteer: "required_beats=change"},
		Compass:       &domain.StoryCompass{LastUpdated: 1},
		Foreshadow:    []domain.ForeshadowEntry{{ID: "fs_secret", Status: "planted", PlantedAt: 1}},
		Characters:    []domain.Character{{Name: "Lan", Tier: "core"}, {Name: "Mai", Tier: "important"}},
		Summaries:     map[int]*domain.ChapterSummary{1: {Characters: []string{"Mai"}}},
		Timeline:      []domain.TimelineEvent{{Chapter: 1}},
		Relationships: []domain.RelationshipEntry{{Chapter: 1}},
		Reviews:       map[int]*domain.ReviewEntry{}, Plans: map[int]*domain.ChapterPlan{},
	}
	for _, ch := range []int{64, 8, 40, 56} {
		base.Reviews[ch] = &domain.ReviewEntry{Chapter: ch, Scope: "chapter", ContractStatus: "missed", Verdict: "rewrite", Dimensions: []domain.DimensionScore{{Dimension: "hook", Score: 50}, {Dimension: "custom_dimension", Score: 40}}}
		base.Plans[ch] = &domain.ChapterPlan{Chapter: ch, Contract: domain.ChapterContract{PayoffPoints: []string{"Một lời hứa"}}}
	}
	for ch := 1; ch <= 20; ch++ {
		base.Outline = append(base.Outline, domain.OutlineEntry{HookType: "lat_thong_tin", Promise: "Lời hứa"})
	}
	missing := base
	missing.Compass, missing.Timeline = nil, nil
	mismatch := base
	progress := *base.Progress
	progress.Phase = domain.PhaseComplete
	mismatch.Progress = &progress
	fixtures := []*Snapshot{&base, &missing, &mismatch}
	for _, rule := range allRules {
		name := runtime.FuncForPC(reflect.ValueOf(rule).Pointer()).Name()
		t.Run(name, func(t *testing.T) {
			count := 0
			for _, snap := range fixtures {
				for _, finding := range rule(snap) {
					count++
					assertVietnameseFinding(t, finding)
				}
			}
			if count == 0 {
				t.Fatal("registered rule has no triggering fixture")
			}
		})
	}
	rc := RuntimeCapture{
		Repeats:   []RepeatStat{{Sig: "writer · plan_chapter · err: invalid contract", Count: 9}, {Sig: "writer · save_plan (args invalid)", Count: 4}},
		StuckStep: "writing.commit_ch64", StuckCount: 9, LogKinds: map[string]int{"stream_idle": 4},
	}
	for _, rule := range runtimeRules {
		findings := rule(&rc)
		if len(findings) == 0 {
			t.Fatal("runtime rule has no triggering fixture")
		}
		for _, finding := range findings {
			assertVietnameseFinding(t, finding)
		}
	}
}

var diagnosticLeak = regexp.MustCompile(`(?i)\b(writer|architect|editor|contract|payoff|host|phase|flow|prompt|model|token|stream|checkpoint|step|engine|agentcore|subagent|tier|steer|rewrite|nil)\b|\b[a-z]+_[a-z_]+\b|\b[a-z][a-z0-9_]*\s*=|\bch\d+\b|[\[\]` + "`" + `]|\.(json|md)\b`)

func assertVietnameseFinding(t *testing.T, finding Finding) {
	t.Helper()
	for field, text := range map[string]string{"title": finding.Title, "evidence": finding.Evidence, "suggestion": finding.Suggestion} {
		if strings.TrimSpace(text) == "" {
			t.Errorf("%s %s is empty", finding.Rule, field)
		}
		if leak := diagnosticLeak.FindString(text); leak != "" {
			t.Errorf("%s %s exposes %q: %s", finding.Rule, field, leak, text)
		}
	}
	for _, action := range PlanActions([]Finding{finding}) {
		if leak := diagnosticLeak.FindString(action.Summary + " " + action.Message); leak != "" {
			t.Errorf("%s action exposes %q", finding.Rule, leak)
		}
	}
}

func TestDiagnosticLoadErrorVietnamese(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir(), "meta", "progress.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, finding := range Analyze(st).Findings {
		if finding.Rule == "LoadError" {
			count++
			assertVietnameseFinding(t, finding)
		}
	}
	if count == 0 {
		t.Fatal("corrupt data did not trigger LoadError")
	}
}

func TestDiagnosticChapterEvidenceSorted(t *testing.T) {
	snap := &Snapshot{Reviews: map[int]*domain.ReviewEntry{}, Plans: map[int]*domain.ChapterPlan{}}
	for _, ch := range []int{64, 8, 56, 40} {
		snap.Reviews[ch] = &domain.ReviewEntry{ContractStatus: "missed"}
		snap.Plans[ch] = &domain.ChapterPlan{Contract: domain.ChapterContract{PayoffPoints: []string{"Lời hứa"}}}
	}
	if got := ContractMissPattern(snap)[0].Evidence; got != "Chương 8, 40, 56, 64 chưa đạt hợp đồng chương (4/4)." {
		t.Errorf("contract evidence = %q", got)
	}
	if got := PayoffMissPattern(snap)[0].Evidence; !strings.HasPrefix(got, "Chương 8 có 1 điểm hồi đáp; chương 40 có 1 điểm hồi đáp; chương 56 có 1 điểm hồi đáp; chương 64 có 1 điểm hồi đáp.") {
		t.Errorf("payoff evidence = %q", got)
	}
}
