package scenes

import (
	"context"
	"encoding/json"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"os"
	"testing"
)

func TestScenesTileWholeFramesAndPublishKeyframes(t *testing.T) {
	job := stepstest.Job("scenes-1")
	job.TemplateSnapshot = json.RawMessage(`{"style":"full","images":{"mode":"fixed","count":3},"thumbnail":"clean"}`)
	s, m := stepstest.Ctx(t, domain.CloneScenes, job)
	m.Put("download/source.mp4", []byte("source"))
	m.Put("transcript/captions.json", []byte(`{"cues":[{"id":"a","startMs":0,"endMs":2000,"text":"opening"},{"id":"b","startMs":2000,"endMs":4000,"text":"ending"}]}`))
	h := Handler{Deps: Deps{Probe: func(context.Context, *pipeline.StepContext, string) (Probe, error) {
		return Probe{DurationSec: 4.017, Width: 1280, Height: 720}, nil
	}, Detect: func(context.Context, *pipeline.StepContext, string, float64) ([]DetectFrame, error) {
		return ParseSceneMetadata("frame:1 pts:30 pts_time:1.0\nlavfi.scd.score=55\n"), nil
	}, Extract: func(_ context.Context, _ *pipeline.StepContext, _, out string, start, end int, at float64) (float64, error) {
		if int(at*30) < start || int(at*30) >= end {
			t.Fatalf("keyframe outside span")
		}
		return at, os.WriteFile(out, []byte("jpeg"), 0644)
	}}}
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data["sceneCount"] != 3 {
		t.Fatal(result.Data)
	}
	if len(result.Artifacts) != 4 {
		t.Fatal(result.Artifacts)
	}
	var doc Manifest
	for _, artifact := range result.Artifacts {
		if artifact.Path == "scenes/manifest.json" {
			doc, err = steps.DecodeJSONFile[Manifest](artifact.File, "")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if doc.TotalFrames != 121 || doc.Scenes[2].EndFrame != 121 {
		t.Fatal(doc)
	}
}
func TestSceneSplitKeepsStableIDAndMarksOutOfSpanKeyframe(t *testing.T) {
	m := Manifest{Revision: 1, Scenes: []Scene{{SceneID: "sc-original", StartFrame: 0, EndFrame: 90, KeyframeFrame: 45}}}
	got, err := ApplyEdits(m, []Edit{{Op: "split", SceneID: "sc-original", Frame: 30, RightID: "sc-right"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Scenes[0].SceneID != "sc-original" || !got.Scenes[0].KeyframeStale || got.Scenes[1].KeyframeStale {
		t.Fatal(got)
	}
	if len(m.Scenes) != 1 || m.Scenes[0].EndFrame != 90 {
		t.Fatal("mutated input")
	}
}
