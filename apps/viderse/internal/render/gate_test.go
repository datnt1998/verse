package render

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

// Ported from export-gate.test.ts at 2c201e6. The gate decision remains owned
// exclusively by quality.EvaluateGate; these exercise it with render bindings.
func TestDeriveBindingFollowsCanvasFPSRevisionAndArtifactInputs(t *testing.T) {
	f := newScorerFixture(t)
	binding := func(project editor.Project, revision string) quality.Binding {
		t.Helper()
		value, err := DeriveBinding(context.Background(), f.media, f.input.Upstream, project, revision)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	first := binding(f.input.Project, "sha256:r1")
	if got := binding(editor.Clone(f.input.Project), "sha256:r1"); got != first {
		t.Fatalf("non-deterministic binding: %+v / %+v", first, got)
	}
	resized := editor.Clone(f.input.Project)
	resized.Metadata()["width"] = 1280
	resized.Metadata()["height"] = 720
	if binding(resized, "sha256:r1").SettingsHash == first.SettingsHash {
		t.Fatal("canvas did not invalidate binding")
	}
	retimed := editor.Clone(f.input.Project)
	retimed.Metadata()["fps"] = 60
	if binding(retimed, "sha256:r1").SettingsHash == first.SettingsHash {
		t.Fatal("fps did not invalidate binding")
	}
	if binding(f.input.Project, "sha256:r2").ProjectRevision != "sha256:r2" {
		t.Fatal("revision was not bound")
	}
	if !strings.HasPrefix(first.ArtifactHash, "sha256:") || !strings.HasPrefix(first.SettingsHash, "sha256:") {
		t.Fatalf("digest format %+v", first)
	}
	for _, path := range []string{"scenes/manifest.json", "images/manifest.json", "transcript/captions.json", "transcript/asr-anchors.json", "build/manifest.json", "build/checkpoint.json"} {
		t.Run(path, func(t *testing.T) {
			fixture := newScorerFixture(t)
			before, err := DeriveBinding(context.Background(), fixture.media, nil, fixture.input.Project, "r1")
			if err != nil {
				t.Fatal(err)
			}
			original, err := fixture.media.Acquire(context.Background(), path)
			var data []byte
			if err == nil {
				// The fixture source is known JSON; changing formatting must still change
				// the binding, even when decoded manifest values are unchanged.
				data, err = fixture.mediaBytes(path)
				original.Release()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				data = []byte("{}")
			}
			fixture.put(path, append(data, '\n'))
			after, err := DeriveBinding(context.Background(), fixture.media, nil, fixture.input.Project, "r1")
			if err != nil {
				t.Fatal(err)
			}
			if after.ArtifactHash == before.ArtifactHash {
				t.Fatalf("%s not bound", path)
			}
			delete(fixture.media.files, path)
			removed, err := DeriveBinding(context.Background(), fixture.media, nil, fixture.input.Project, "r1")
			if err != nil {
				t.Fatal(err)
			}
			if removed.ArtifactHash == after.ArtifactHash {
				t.Fatalf("removed %s did not invalidate binding", path)
			}
		})
	}
}
func (f *scorerFixture) mediaBytes(path string) ([]byte, error) {
	return (scoreArtifacts{media: f.media}).bytes(context.Background(), path)
}
func TestDeriveBindingBindsDeclaredSourceNotFormatting(t *testing.T) {
	f := newScorerFixture(t)
	derive := func() quality.Binding {
		t.Helper()
		value, err := DeriveBinding(context.Background(), f.media, nil, f.input.Project, "r1")
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	base := derive()
	f.putJSON("download/source.info.json", editor.Object{"sha256": "source-a", "durationSec": 6})
	declared := derive()
	if declared.ArtifactHash == base.ArtifactHash {
		t.Fatal("source declarations not bound")
	}
	f.put("download/source.info.json", []byte("{\n \"durationSec\": 6, \"sha256\": \"source-a\"\n}\n"))
	if derive() != declared {
		t.Fatal("unconsulted source-info whitespace invalidated binding")
	}
	f.putJSON("download/source.info.json", editor.Object{"sha256": "source-b", "durationSec": 6})
	if derive().ArtifactHash == declared.ArtifactHash {
		t.Fatal("declared source digest not bound")
	}
	f.putJSON("download/source.info.json", editor.Object{"sha256": "source-a", "durationSec": 12})
	if derive().ArtifactHash == declared.ArtifactHash {
		t.Fatal("declared source duration not bound")
	}
}
func TestDeriveBindingResolvesPublishedUpstreamPaths(t *testing.T) {
	f := newScorerFixture(t)
	expected, err := DeriveBinding(context.Background(), f.media, nil, f.input.Project, "r1")
	if err != nil {
		t.Fatal(err)
	}
	upstream := map[domain.CloneStep]domain.CloneStepOutput{}
	for _, entry := range []struct {
		step domain.CloneStep
		path string
	}{
		{domain.CloneScenes, "scenes/manifest.json"}, {domain.CloneImages, "images/manifest.json"},
		{domain.CloneTranscript, "transcript/captions.json"}, {domain.CloneBuild, "build/manifest.json"},
	} {
		logical := "clone/job/digest/" + entry.path
		file := f.media.files[entry.path]
		f.media.files[logical] = file
		delete(f.media.files, entry.path)
		output := upstream[entry.step]
		output.Artifacts = append(output.Artifacts, domain.CloneArtifact{Path: logical, SHA256: "declared"})
		upstream[entry.step] = output
	}
	actual, err := DeriveBinding(context.Background(), f.media, upstream, f.input.Project, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("published paths changed semantic binding: %+v / %+v", actual, expected)
	}
	f.input.Upstream = upstream
	score := f.score()
	if !score.Passed {
		t.Fatalf("scorer did not resolve published artifact paths: %+v", score)
	}
}
func TestDeriveBindingRejectsAmbiguousArtifactsAndStorageFailures(t *testing.T) {
	f := newScorerFixture(t)
	upstream := map[domain.CloneStep]domain.CloneStepOutput{
		domain.CloneScenes: {Artifacts: []domain.CloneArtifact{{Path: "clone/a/one/scenes/manifest.json"}, {Path: "clone/a/two/scenes/manifest.json"}}},
	}
	if _, err := DeriveBinding(context.Background(), f.media, upstream, f.input.Project, "r1"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous paths: %v", err)
	}
	sentinel := errors.New("storage failed")
	f.media.failure = sentinel
	if _, err := DeriveBinding(context.Background(), f.media, nil, f.input.Project, "r1"); !errors.Is(err, sentinel) {
		t.Fatalf("storage failure: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DeriveBinding(ctx, f.media, nil, f.input.Project, "r1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
func TestExportGateBindingFreshnessAndAcceptance(t *testing.T) {
	f := newScorerFixture(t)
	score := f.score()
	binding := score.Binding
	missing := quality.EvaluateGate(nil, &binding, false)
	if missing.Allowed || missing.Fresh || missing.Reason == nil || !strings.Contains(*missing.Reason, "đo") {
		t.Fatalf("missing score %+v", missing)
	}
	open := quality.EvaluateGate(score, &binding, false)
	if !open.Fresh || !open.Allowed || open.Reason != nil || len(open.Reasons) != 0 {
		t.Fatalf("passing gate %+v", open)
	}
	for _, test := range []struct {
		name   string
		mutate func(*quality.Binding)
	}{
		{"project revision", func(b *quality.Binding) { b.ProjectRevision = "changed" }},
		{"settings", func(b *quality.Binding) { b.SettingsHash = "changed" }},
		{"artifacts", func(b *quality.Binding) { b.ArtifactHash = "changed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			stale := binding
			test.mutate(&stale)
			gate := quality.EvaluateGate(score, &stale, false)
			if gate.Fresh || gate.Allowed || gate.Reason == nil {
				t.Fatalf("stale gate %+v", gate)
			}
		})
	}
	unreadable := quality.EvaluateGate(score, nil, false)
	if unreadable.Fresh || unreadable.Allowed {
		t.Fatalf("unreadable gate %+v", unreadable)
	}
	failing := *score
	failing.Total = 70
	failing.Passed = false
	failing.Blockers = []quality.Blocker{{ID: "captions", Message: "Thiếu phụ đề"}}
	closed := quality.EvaluateGate(&failing, &binding, false)
	if !closed.Fresh || closed.Allowed || len(closed.Reasons) == 0 {
		t.Fatalf("closed gate %+v", closed)
	}
	stale := binding
	stale.ArtifactHash = "changed"
	for _, candidate := range []*quality.Score{&failing, score, nil} {
		for _, current := range []*quality.Binding{&binding, &stale, nil} {
			gate := quality.EvaluateGate(candidate, current, true)
			if !gate.Allowed || !gate.Accepted {
				t.Fatalf("acceptance did not open %+v", gate)
			}
		}
	}
	accepted := quality.EvaluateGate(&failing, &binding, true)
	if !reflect.DeepEqual(accepted.Reasons, closed.Reasons) {
		t.Fatal("acceptance erased the failing checklist")
	}
}
func TestExportGateRejectsFreshZeroWeightCaptionBlocker(t *testing.T) {
	f := newScorerFixture(t)
	f.align(12, false)
	score := f.score()
	if score.Total != 100 || !scorerBlocked(score, "captionAlignment") {
		t.Fatalf("fixture %+v", score)
	}
	gate := quality.EvaluateGate(score, &score.Binding, false)
	if !gate.Fresh || gate.Allowed || gate.Accepted || len(gate.Reasons) == 0 {
		t.Fatalf("alignment gate %+v", gate)
	}
	accepted := quality.EvaluateGate(score, &score.Binding, true)
	if !accepted.Allowed || !reflect.DeepEqual(gate.Reasons, accepted.Reasons) {
		t.Fatalf("accepted gate %+v", accepted)
	}
	if quality.EvaluateGate(score, &score.Binding, false).Allowed {
		t.Fatal("withdrawing acceptance did not close gate")
	}
}
func TestExportGateSavedScoreRoundTripAndProjectEdits(t *testing.T) {
	f := newScorerFixture(t)
	score := f.score()
	encoded, err := json.Marshal(score)
	if err != nil {
		t.Fatal(err)
	}
	var stored quality.Score
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	if err := stored.Validate(); err != nil {
		t.Fatal(err)
	}
	if !quality.EvaluateGate(&stored, &score.Binding, false).Allowed {
		t.Fatal("saved score unexpectedly blocked")
	}
	edited := editor.Clone(f.input.Project)
	edited["name"] = "Edited after measuring"
	current, err := DeriveBinding(context.Background(), f.media, nil, edited, "sha256:r2")
	if err != nil {
		t.Fatal(err)
	}
	gate := quality.EvaluateGate(&stored, &current, false)
	if gate.Fresh || gate.Allowed || gate.Reason == nil || !strings.Contains(*gate.Reason, "Project đã sửa") {
		t.Fatalf("edited saved gate %+v", gate)
	}
	if !quality.EvaluateGate(&stored, &current, true).Allowed {
		t.Fatal("acceptance should waive stale score")
	}
	// Ranges remain available after decoding the durable score's measured JSON.
	scorerEntry(f.input.Project, "caption")["cues"] = []editor.Object{{"startSeconds": 0, "endSeconds": 2, "text": "first"}}
	failed := f.score()
	encoded, err = json.Marshal(failed)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ScoreRanges(failed, f.input.Project), ScoreRanges(&stored, f.input.Project)) {
		t.Fatal("score persistence changed failing ranges")
	}
}
