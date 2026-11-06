package build

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/backdrop"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"os"
	"strings"
	"testing"
	"time"
)

// Same two-scene/six-second fixture as yt-factory build.test.ts. Media I/O
// uses the shared Publish/Acquire fake; no ffmpeg or image provider is called.
func fixture(t *testing.T) (Handler, *pipeline.StepContext, *stepstest.FakeMedia) {
	t.Helper()
	job := stepstest.Job("job-1")
	job.Source.Title = "Fixture"
	job.CreatedAt = time.UnixMilli(1)
	project, _ := editor.New("template", "Template", 1920, 1080, 30, time.UnixMilli(1))
	project["timeline"] = editor.Object{"tracks": []any{editor.Object{"id": "scenes", "name": "Ảnh cảnh", "kind": "video", "height": 80, "locked": false, "visible": true, "muted": false, "solo": false, "order": 0, "templateSlot": "scenes"}, editor.Object{"id": "captions", "name": "Phụ đề", "kind": "video", "height": 80, "locked": false, "visible": true, "muted": false, "solo": false, "order": 1, "templateSlot": "captions"}, editor.Object{"id": "audio", "name": "Âm thanh gốc", "kind": "audio", "height": 80, "locked": false, "visible": true, "muted": false, "solo": false, "order": 2, "templateSlot": "source-audio"}}, "items": []any{}, "transitions": []any{}, "keyframes": []any{}}
	snapshot := signedSnapshot(t, project)
	job.TemplateID = snapshot.TemplateID
	job.TemplateSnapshot = marshalFixture(t, snapshot)
	s, m := stepstest.Ctx(t, domain.CloneBuild, job)
	m.Put("download/source.mp4", []byte("fake-source-bytes"))
	m.Put("images/scene-0.webp", []byte("webp-1"))
	m.Put("images/scene-1.webp", []byte("webp-2"))
	m.Put("images/thumbnail.webp", []byte("webp-thumb"))
	sourceHash, err := steps.SHA256File(m.Root + "/download/source.mp4")
	if err != nil {
		t.Fatal(err)
	}
	doc := scenes.Manifest{Version: 1, Revision: 3, FPS: 30, TotalFrames: 180, DurationSec: 6, Source: scenes.FileRef{Path: "download/source.mp4", SHA256: sourceHash}, Recipe: scenes.Recipe{Style: "full", ImagesMode: "fixed", ImageCount: 2, Thumbnail: "clean"}, Scenes: []scenes.Scene{{SceneID: "sc-f0", StartFrame: 0, EndFrame: 90, KeyframeFrame: 45, Text: "mở đầu"}, {SceneID: "sc-f90", StartFrame: 90, EndFrame: 180, KeyframeFrame: 135, Text: "kết"}}}
	m.Put("scenes/manifest.json", marshalFixture(t, doc))
	review := images.Manifest{Version: 1, Revision: 2, SceneRevision: 3, Style: "full", Thumbnail: "clean", Items: []images.Item{}}
	for n, id := range []string{"sc-f0", "sc-f90", "thumbnail"} {
		kind, path, w, height := "scene", "images/scene-0.webp", 1920, 1080
		if n == 1 {
			path = "images/scene-1.webp"
		}
		if n == 2 {
			kind, path, w, height = "thumbnail", "images/thumbnail.webp", 1280, 720
		}
		narration := job.Source.Title
		if n < 2 {
			narration = doc.Scenes[n].Text
		}
		item := images.Item{ID: id, SceneID: id, Kind: kind, StartFrame: n * 90, EndFrame: (n + 1) * 90, Width: w, Height: height, Narration: narration, Status: "review"}
		hash, err := steps.SHA256File(m.Root + "/" + path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(m.Root + "/" + path)
		if err != nil {
			t.Fatal(err)
		}
		one := 1
		item.ReviewVersion = &one
		item.Versions = []images.Version{{Version: 1, GenerationID: "generation-1", CreatedAt: time.UnixMilli(1).UTC().Format(time.RFC3339Nano), Path: path, SHA256: hash, Width: w, Height: height, Bytes: info.Size(), Mime: "image/webp", SourceFingerprint: images.SourceFingerprint(item)}}
		review.Items = append(review.Items, item)
	}
	m.Put("images/manifest.json", marshalFixture(t, review))
	m.Put("transcript/captions.json", []byte(`{"version":1,"revision":2,"cues":[{"id":"c1","startMs":0,"endMs":2500,"text":"Xin chào","words":[{"startMs":0,"endMs":1000,"text":"Xin"},{"startMs":1000,"endMs":2500,"text":"chào"}]},{"id":"c2","startMs":3000,"endMs":5000,"text":"Câu hai","words":null},{"id":"c3","startMs":5500,"endMs":null,"text":"không timing","words":null}]}`))
	h := Handler{Deps: Deps{ExtractAudio: func(_ context.Context, _ *pipeline.StepContext, _ string, out string) error {
		return os.WriteFile(out, []byte("audio"), 0644)
	}, MeasureLUFS: func(context.Context, *pipeline.StepContext, string) (float64, error) { return -20, nil }, Envelope: func(context.Context, *pipeline.StepContext, string, string) (editor.Object, error) {
		return editor.Object{"sourceHash": "sha256:" + strings.Repeat("b", 64), "samplesPerSecond": 50, "durationSeconds": 6, "envelopeBase64": base64.StdEncoding.EncodeToString(make([]byte, 300)), "normalization": .4}, nil
	}}}
	h.Deps.MediaMetadata = func(_ context.Context, id string) (editor.MediaRecord, error) {
		var record editor.MediaRecord
		data, err := os.ReadFile(m.Root + "/editor/media/" + id + "/metadata.json")
		if err != nil {
			return record, err
		}
		err = json.Unmarshal(data, &record)
		return record, err
	}
	return h, s, m
}
func TestBuildUsesPersistedFixedBackdropInput(t *testing.T) {
	h, s, _ := fixture(t)
	manifest := backdrop.Seal(backdrop.Manifest{Fallback: "none", FixedClips: []backdrop.FixedClip{{VideoID: "fixed", Path: "library/backdrops/missing.mp4", FromSec: 10, ToSec: 20}}})
	s.Upstream[domain.CloneBackdrop] = domain.CloneStepOutput{Data: map[string]any{"manifest": manifest}}
	if _, err := h.Run(context.Background(), s); err == nil {
		t.Fatal("build did not acquire fixed backdrop input")
	}
	manifest.FixedClips = nil
	manifest.ExcludedFixed = []string{"fixed"}
	manifest = backdrop.Seal(manifest)
	s.Upstream[domain.CloneBackdrop] = domain.CloneStepOutput{Data: map[string]any{"manifest": manifest}}
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatalf("excluded fixed backdrop still used: %v", err)
	}
}
func TestBuildProducesSchema15DraftThenWaitsWithoutApprovingImages(t *testing.T) {
	h, s, m := fixture(t)
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result.Waiting == nil || result.Waiting.Code != domain.CloneErrAwaitingReview || result.Waiting.RetryAt != nil {
		t.Fatalf("not awaiting review: %+v", result.Waiting)
	}
	if result.Data["projectId"] != "clone-job-1-g1" {
		t.Fatal(result.Data)
	}
	if err = m.Publish(context.Background(), result.Artifacts, nil); err != nil {
		t.Fatal(err)
	}
	project, _, err := (editor.Store{Media: m}).Read(context.Background(), "clone-job-1-g1")
	if err != nil {
		t.Fatal(err)
	}
	if project["schemaVersion"] != float64(15) || project["duration"] != 6.0 {
		t.Fatalf("wrong schema/duration: %+v", project)
	}
	original, err := m.Acquire(context.Background(), "images/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := steps.DecodeJSONFile[images.Manifest](original.Path, "")
	if images.ImagesApproved(doc) {
		t.Fatal("build silently approved images")
	}
}
func TestBuildRejectsChangedImageGeneration(t *testing.T) {
	h, s, m := fixture(t)
	handle, _ := m.Acquire(context.Background(), "images/manifest.json")
	doc, _ := steps.DecodeJSONFile[images.Manifest](handle.Path, "")
	doc.SceneRevision++
	b, _ := json.Marshal(doc)
	m.Put("images/manifest.json", b)
	_, err := h.Run(context.Background(), s)
	var e *domain.CloneError
	if !errors.As(err, &e) || e.Code != domain.CloneErrStale {
		t.Fatalf("got %v", err)
	}
}
func TestParseIntegratedLufsTakesLastFiniteSummary(t *testing.T) {
	got, err := ParseIntegratedLUFS(" I: -22.5 LUFS\n I: -18.2 LUFS\n")
	if err != nil || got != -18.2 {
		t.Fatal(got, err)
	}
	if _, err = ParseIntegratedLUFS(" I: -inf LUFS\n"); err == nil {
		t.Fatal("accepted silent loudness")
	}
}
