package render

import (
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

func TestQualityCaptionOnlyEditInvalidatesOnlyCaptionTiming(t *testing.T) {
	f := newScorerFixture(t)
	baseline := editor.Clone(f.input.Project)
	score := f.score()
	cue := scoreObjects(scorerEntry(f.input.Project, "caption")["cues"])[0]
	cue["startSeconds"] = 0.4
	stale := qualityStaleItems(score, baseline, f.input.Project, false)
	for _, id := range quality.IDs {
		want := id == "captionAlignment" || id == "captionKaraoke"
		if stale[id] != want {
			t.Errorf("check %s stale=%v want%v", id, stale[id], want)
		}
	}
	if quality.EvaluateGate(score, &quality.Binding{ProjectRevision: "changed", SettingsHash: score.Binding.SettingsHash, ArtifactHash: score.Binding.ArtifactHash}, false).Allowed {
		t.Fatal("item-level freshness reopened global revision gate")
	}
}

func TestQualityStalenessConservativelyHandlesUnknownAndNonCaptionChanges(t *testing.T) {
	f := newScorerFixture(t)
	baseline := editor.Clone(f.input.Project)
	score := f.score()
	for _, caseName := range []string{"missing-baseline", "audio-change"} {
		t.Run(caseName, func(t *testing.T) {
			before := baseline
			after := editor.Clone(baseline)
			if caseName == "missing-baseline" {
				before = nil
			} else {
				scorerEntry(after, "source-audio")["volume"] = 0.5
			}
			stale := qualityStaleItems(score, before, after, false)
			for _, id := range quality.IDs {
				if !stale[id] {
					t.Errorf("%s was assumed fresh", id)
				}
			}
		})
	}
	stale := qualityStaleItems(score, baseline, baseline, true)
	for _, id := range quality.IDs {
		if stale[id] {
			t.Errorf("fresh %s marked stale", id)
		}
	}
}
