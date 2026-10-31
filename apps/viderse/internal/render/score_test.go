package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/transcript"
)

// These cases port score.test.ts at 2c201e6. Persistence/measurement-step cases
// belong to the service/step owners; this fixture exercises the real scorer's
// published-media boundary without Chrome, ffmpeg, or a database.
type scorerTestMedia struct {
	files   map[string]string
	failure error
}

func (m *scorerTestMedia) Acquire(ctx context.Context, path string) (*media.Handle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.failure != nil {
		return nil, m.failure
	}
	file, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &media.Handle{Path: file}, nil
}
func (*scorerTestMedia) Publish(context.Context, []media.Artifact, func(context.Context) error) error {
	return errors.New("the scorer must not publish")
}

type scorerFixture struct {
	t        *testing.T
	root     string
	media    *scorerTestMedia
	input    ScoreInput
	scenes   scenes.Manifest
	images   images.Manifest
	captions transcript.CaptionManifest
	build    editor.Object
}

func scorerTestHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (f *scorerFixture) put(path string, data []byte) {
	f.t.Helper()
	file := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0600); err != nil {
		f.t.Fatal(err)
	}
	f.media.files[path] = file
}
func (f *scorerFixture) putJSON(path string, value any) {
	f.t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		f.t.Fatal(err)
	}
	f.put(path, data)
}
func (f *scorerFixture) sync() {
	f.putJSON("scenes/manifest.json", f.scenes)
	f.putJSON("images/manifest.json", f.images)
	f.putJSON("transcript/captions.json", f.captions)
	f.putJSON("build/manifest.json", f.build)
}
func (f *scorerFixture) score() *quality.Score {
	f.t.Helper()
	f.sync()
	result, err := ComputeScore(context.Background(), f.input)
	if err != nil {
		f.t.Fatal(err)
	}
	return result
}
func scorerItem(t *testing.T, score *quality.Score, id string) quality.Item {
	t.Helper()
	for _, item := range score.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("missing check %s", id)
	return quality.Item{}
}
func scorerBlocked(score *quality.Score, id string) bool {
	for _, blocker := range score.Blockers {
		if blocker.ID == id {
			return true
		}
	}
	return false
}
func scorerEntry(project editor.Project, id string) editor.Object {
	for _, entry := range scoreObjects(project.Timeline()["items"]) {
		if scoreString(entry, "id") == id {
			return entry
		}
	}
	return nil
}
func scorerTrack(project editor.Project, id string) editor.Object {
	for _, entry := range scoreObjects(project.Timeline()["tracks"]) {
		if scoreString(entry, "id") == id {
			return entry
		}
	}
	return nil
}
func scorerRemove(project editor.Project, ids ...string) {
	items := []editor.Object{}
	for _, entry := range scoreObjects(project.Timeline()["items"]) {
		remove := false
		for _, id := range ids {
			if scoreString(entry, "id") == id {
				remove = true
			}
		}
		if !remove {
			items = append(items, entry)
		}
	}
	project.Timeline()["items"] = items
}
func newScorerFixture(t *testing.T) *scorerFixture {
	t.Helper()
	f := &scorerFixture{t: t, root: t.TempDir(), media: &scorerTestMedia{files: map[string]string{}}}
	f.scenes = scenes.Manifest{
		Version: 1, Revision: 1, FPS: 30, TotalFrames: 180, DurationSec: 6,
		Source: scenes.FileRef{Path: "download/source.mp4", SHA256: strings.Repeat("a", 64)},
		Recipe: scenes.Recipe{Style: "illustration", ImagesMode: "all"},
		Scenes: []scenes.Scene{
			{SceneID: "a", StartFrame: 0, EndFrame: 90, Keyframe: scenes.FileRef{SHA256: "key-a"}},
			{SceneID: "b", StartFrame: 90, EndFrame: 180, Keyframe: scenes.FileRef{SHA256: "key-b"}},
		},
	}
	f.images = images.Manifest{Version: 1, Revision: 1, SceneRevision: 1, Style: "illustration"}
	for _, spec := range []struct {
		id, kind                  string
		start, end, width, height int
	}{
		{"a", "scene", 0, 90, 1920, 1080}, {"b", "scene", 90, 180, 1920, 1080}, {"thumbnail", "thumbnail", 0, 0, 1280, 720},
	} {
		payload := []byte("approved-image-" + spec.id)
		path := "images/" + spec.id + ".webp"
		f.put(path, payload)
		entry := images.Item{ID: spec.id, Kind: spec.kind, StartFrame: spec.start, EndFrame: spec.end, Width: spec.width, Height: spec.height, Status: "approved", CurrentVersion: new(1), Versions: []images.Version{}}
		if spec.kind == "scene" {
			entry.SceneID = spec.id
		}
		entry.Versions = append(entry.Versions, images.Version{Version: 1, SourceFingerprint: images.SourceFingerprint(entry), Path: path, SHA256: scorerTestHash(payload), Width: spec.width, Height: spec.height, Mime: "image/webp"})
		f.images.Items = append(f.images.Items, entry)
	}
	f.captions = transcript.CaptionManifest{
		Version: 1, Revision: 1, SpeechIntervals: []transcript.Interval{{StartMs: 0, EndMs: 6000}},
		Cues: []transcript.Cue{{ID: "cue", StartMs: 0, EndMs: new(6000.), Text: "one two three four five six"}},
	}
	tracks := []editor.Object{
		{"id": "scene", "visible": true, "templateSlot": "scenes"},
		{"id": "captions", "visible": true, "templateSlot": "captions"},
		{"id": "audio", "muted": false, "templateSlot": "source-audio"},
		{"id": "overlay", "visible": true},
	}
	items := []editor.Object{
		{"id": "image-a", "type": "image", "trackId": "scene", "from": 0, "durationInFrames": 90, "mediaId": "image-a"},
		{"id": "image-b", "type": "image", "trackId": "scene", "from": 90, "durationInFrames": 90, "mediaId": "image-b"},
		{"id": "caption", "type": "subtitle", "trackId": "captions", "from": 0, "durationInFrames": 180, "cues": []editor.Object{{"id": "cue", "startSeconds": 0, "endSeconds": 6, "text": "one two three four five six"}}},
		{"id": "source-audio", "type": "audio", "trackId": "audio", "from": 0, "durationInFrames": 180, "mediaId": "source-audio"},
		{"id": "waveform", "type": "video", "trackId": "overlay", "from": 0, "durationInFrames": 180, "mediaId": "source-audio", "waveformVisualizer": editor.Object{"envelopeBase64": "AAAA"}},
		{"id": "subscribe-text", "type": "text", "trackId": "overlay", "from": 0, "durationInFrames": 180, "linkedGroupId": "owner-group", "text": "Subscribe"},
		{"id": "subscribe-shape-1", "type": "shape", "trackId": "overlay", "from": 0, "durationInFrames": 180, "linkedGroupId": "owner-group"},
		{"id": "subscribe-shape-2", "type": "shape", "trackId": "overlay", "from": 0, "durationInFrames": 180, "linkedGroupId": "owner-group"},
	}
	project := editor.Project{"id": "project", "metadata": editor.Object{"width": 1920, "height": 1080, "fps": 30}, "timeline": editor.Object{"tracks": tracks, "items": items}}
	f.build = editor.Object{
		"projectId": "project", "audio": editor.Object{"mediaId": "source-audio", "sourceLufs": -14., "referenceVersion": SourceAudioReferenceVersion},
		"itemIds": editor.Object{"captions": "caption", "scenes": []string{"image-a", "image-b"}, "waveform": "waveform", "subscribe": []string{"subscribe-text", "subscribe-shape-1", "subscribe-shape-2"}},
		"scenes":  []editor.Object{{"sceneId": "a", "startFrame": 0, "endFrame": 90, "mediaId": "image-a"}, {"sceneId": "b", "startFrame": 90, "endFrame": 180, "mediaId": "image-b"}},
	}
	f.input = ScoreInput{Project: project, Revision: "sha256:r1", Job: domain.CloneJob{ID: "caption-job", Generation: 1}, LUFS: new(-14.), MeasurementID: new("m1"), Template: editor.Clone(project), Media: f.media}
	for _, id := range []string{"image-a", "image-b", "source-audio"} {
		f.putJSON("editor/media/"+id+"/metadata.json", editor.Object{"id": id, "contentHash": "sha256:test"})
	}
	f.sync()
	return f
}

func TestScoreCutRhythm(t *testing.T) {
	spans := []scenes.Scene{{SceneID: "a", StartFrame: 0, EndFrame: 90}, {SceneID: "b", StartFrame: 90, EndFrame: 180}}
	for _, test := range []struct {
		name                             string
		secondStart, firstEnd, secondEnd int
		pass                             bool
	}{
		{"exact", 90, 90, 180, true}, {"within tolerance", 92, 90, 180, true}, {"at tolerance", 93, 90, 180, true}, {"drift", 100, 90, 180, false}, {"overlap", 89, 90, 180, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			items := []editor.Object{{"from": 0, "durationInFrames": test.firstEnd}, {"from": test.secondStart, "durationInFrames": test.secondEnd - test.secondStart}}
			problems := scoreCutRhythm(items, spans, 30)
			if (len(problems) == 0) != test.pass {
				t.Fatalf("problems %v", problems)
			}
		})
	}
	if len(scoreCutRhythm([]editor.Object{{"from": 0, "durationInFrames": 180}}, spans, 30)) == 0 {
		t.Fatal("missing scene passed")
	}
}
func TestScoreSubtitleCueSpans(t *testing.T) {
	item := editor.Object{"from": 60, "durationInFrames": 90, "cues": []editor.Object{
		{"startSeconds": -1, "endSeconds": 1, "text": "clipped"},
		{"startSeconds": 1.5, "endSeconds": 10, "text": "clipped"},
		{"startSeconds": 0, "endSeconds": 1, "text": "   "},
	}}
	want := []transcript.Interval{{StartMs: 2000, EndMs: 3000}, {StartMs: 3500, EndMs: 5000}}
	if got := scoreSubtitleCueSpans(item, 30); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
func TestScoreTrackVisibilityMuteAndMalformedEntries(t *testing.T) {
	f := newScorerFixture(t)
	tracks := scoreObjects(f.input.Project.Timeline()["tracks"])
	tracks = append(tracks, editor.Object{"id": "parent", "visible": false, "muted": true})
	f.input.Project.Timeline()["tracks"] = tracks
	scorerTrack(f.input.Project, "scene")["parentTrackId"] = "parent"
	scorerTrack(f.input.Project, "captions")["parentTrackId"] = "parent"
	scorerTrack(f.input.Project, "audio")["parentTrackId"] = "parent"
	items := []any{}
	for _, item := range scoreObjects(f.input.Project.Timeline()["items"]) {
		items = append(items, item)
	}
	items = append(items, nil, "bad", editor.Object{"id": "missing-duration", "type": "image", "trackId": "scene", "from": 0})
	f.input.Project.Timeline()["items"] = items
	p := scoreReadProject(f.input.Project)
	if len(p.items) != 8 || len(scoreSceneItems(p)) != 0 || len(scoreCoveredCaptions(p, 30)) != 0 || len(scoreAudibleItems(p, "source-audio")) != 0 {
		t.Fatalf("incorrect lenient/parent context: %+v", p)
	}
	cyc := scoreTrackContexts([]editor.Object{{"id": "a", "parentTrackId": "b"}, {"id": "b", "parentTrackId": "a"}})
	if cyc["a"].visible || !cyc["a"].muted {
		t.Fatal("parent cycle must fail closed")
	}
}
func TestComputeScorePassingFixture(t *testing.T) {
	f := newScorerFixture(t)
	score := f.score()
	if score.Total != 100 || !score.Passed || len(score.Blockers) != 0 {
		t.Fatalf("unexpected score %+v", score)
	}
	for _, item := range score.Items {
		if item.Status != "pass" {
			t.Fatalf("unexpected check %+v", item)
		}
	}
	if score.Binding.ProjectRevision != "sha256:r1" || !strings.HasPrefix(score.Binding.SettingsHash, "sha256:") || !strings.HasPrefix(score.Binding.ArtifactHash, "sha256:") || score.MeasurementID == nil || *score.MeasurementID != "m1" {
		t.Fatalf("binding/measurement %+v", score)
	}
	if err := score.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestComputeScoreLoudness(t *testing.T) {
	for _, test := range []struct {
		name        string
		source, mix *float64
		version     int
		status      string
		total       float64
		passed      bool
	}{
		{"matching", new(-14.), new(-14.), 1, "pass", 100, true},
		{"off source exactly ninety", new(-14.), new(-20.), 1, "fail", 90, true},
		{"quiet source", new(-23.), new(-23.), 1, "pass", 100, true},
		{"not fixed target", new(-23.), new(-14.), 1, "fail", 90, true},
		{"unmeasured", new(-14.), nil, 1, "unknown", 90, false},
		{"missing reference", nil, new(-14.), 1, "unknown", 90, false},
		{"old reference", new(-14.), new(-14.), 0, "unknown", 90, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScorerFixture(t)
			audio := scoreObject(f.build["audio"])
			audio["sourceLufs"] = test.source
			audio["referenceVersion"] = test.version
			f.input.LUFS = test.mix
			score := f.score()
			if got := scorerItem(t, score, "loudness").Status; got != test.status || score.Total != test.total || score.Passed != test.passed {
				t.Fatalf("got %s / %g / %v", got, score.Total, score.Passed)
			}
		})
	}
}
func TestComputeScoreCheckpointTarget(t *testing.T) {
	for _, test := range []struct {
		name   string
		mix    float64
		status string
	}{
		{"target matches", -14.4, "pass"},
		{"target drift", -17, "fail"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScorerFixture(t)
			scoreObject(f.build["audio"])["sourceLufs"] = -23.
			f.putJSON("build/checkpoint.json", editor.Object{"audio": editor.Object{"sourceLufs": -23, "referenceVersion": 1, "loudnessTargetLufs": -14}})
			f.input.LUFS = new(test.mix)
			check := scorerItem(t, f.score(), "loudness")
			if check.Status != test.status || !strings.Contains(check.Expected, "-14.0±1 LUFS (mục tiêu") {
				t.Fatalf("%+v", check)
			}
		})
	}
}
func TestComputeScoreOptionalTemplateLayers(t *testing.T) {
	f := newScorerFixture(t)
	for _, project := range []editor.Project{f.input.Template, f.input.Project} {
		tracks := []editor.Object{}
		for _, track := range scoreObjects(project.Timeline()["tracks"]) {
			if scoreString(track, "templateSlot") != "captions" {
				tracks = append(tracks, track)
			}
		}
		project.Timeline()["tracks"] = tracks
		scorerRemove(project, "caption", "waveform", "subscribe-text", "subscribe-shape-1", "subscribe-shape-2")
	}
	if score := f.score(); score.Total != 100 || !score.Passed || len(score.Blockers) != 0 {
		t.Fatalf("optional layers %+v", score)
	}
}
func TestComputeScoreNoTemplate(t *testing.T) {
	f := newScorerFixture(t)
	f.input.Template = nil
	score := f.score()
	if !scorerBlocked(score, "template") || score.Passed || scorerItem(t, score, "captions").Status != "unknown" || scorerBlocked(score, "waveform") || scorerBlocked(score, "subscribe") {
		t.Fatalf("%+v", score)
	}
}
func TestComputeScoreMissingManifests(t *testing.T) {
	f := newScorerFixture(t)
	f.media.files = map[string]string{}
	f.input.Template = nil
	score, err := ComputeScore(context.Background(), f.input)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"duration-invalid", "images", "thumbnail", "audio", "template"} {
		if !scorerBlocked(score, id) {
			t.Fatalf("missing blocker %s: %+v", id, score)
		}
	}
	if score.Passed || scorerBlocked(score, "captions") || scorerBlocked(score, "waveform") || scorerBlocked(score, "subscribe") {
		t.Fatalf("%+v", score)
	}
}
func TestComputeScoreMissingBuildIsNotInferred(t *testing.T) {
	f := newScorerFixture(t)
	delete(f.media.files, "build/manifest.json")
	f.putJSON("build/checkpoint.json", editor.Object{"version": 1, "audioSha256": strings.Repeat("a", 64), "sourceLoudnessLufs": -14})
	f.input.Project["media"] = []editor.Object{{"id": "source-audio", "tags": []string{"clone-audio"}, "contentHash": "sha256:" + strings.Repeat("a", 64)}}
	score, err := ComputeScore(context.Background(), f.input)
	if err != nil {
		t.Fatal(err)
	}
	if !scorerBlocked(score, "audio") || scorerItem(t, score, "loudness").Status != "unknown" || score.Passed {
		t.Fatalf("inferred missing evidence %+v", score)
	}
}
func TestComputeScoreSpeechCoverage(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*scorerFixture)
		pass   bool
	}{
		{"deleted tail", func(f *scorerFixture) {
			scorerEntry(f.input.Project, "caption")["cues"] = []editor.Object{{"startSeconds": 0, "endSeconds": 2, "text": "first"}}
		}, false},
		{"hidden track", func(f *scorerFixture) { scorerTrack(f.input.Project, "captions")["visible"] = false }, false},
		{"empty transcript", func(f *scorerFixture) { f.captions.Cues = nil }, false},
		{"no speech", func(f *scorerFixture) { f.captions.SpeechIntervals = []transcript.Interval{} }, true},
		{"missing vad", func(f *scorerFixture) { f.captions.SpeechIntervals = nil }, false},
		{"music gap", func(f *scorerFixture) {
			f.captions.Cues = []transcript.Cue{{ID: "a", StartMs: 0, EndMs: new(2000.), Text: "first"}, {ID: "b", StartMs: 4000, EndMs: new(6000.), Text: "last"}}
			scorerEntry(f.input.Project, "caption")["cues"] = []editor.Object{{"startSeconds": 0, "endSeconds": 2, "text": "first"}, {"startSeconds": 4, "endSeconds": 6, "text": "last"}}
		}, true},
		{"deleted sentence after music", func(f *scorerFixture) {
			f.captions.Cues = []transcript.Cue{{ID: "a", StartMs: 0, EndMs: new(2000.), Text: "first"}, {ID: "b", StartMs: 4000, EndMs: new(6000.), Text: "last"}}
			scorerEntry(f.input.Project, "caption")["cues"] = []editor.Object{{"startSeconds": 0, "endSeconds": 2, "text": "first"}}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScorerFixture(t)
			test.mutate(f)
			score := f.score()
			if (scorerItem(t, score, "captions").Status == "pass") != test.pass || scorerBlocked(score, "captions") == test.pass {
				t.Fatalf("%+v", score)
			}
			if test.name == "hidden track" {
				for _, blocker := range score.Blockers {
					if blocker.ID == "captions" && (blocker.Fix == nil || blocker.Fix.ItemID != "caption") {
						t.Fatalf("missing caption target %+v", blocker)
					}
				}
			}
		})
	}
}
func TestComputeScoreHardPrerequisites(t *testing.T) {
	for _, test := range []struct {
		name, id string
		mutate   func(*scorerFixture)
	}{
		{"muted source", "audio", func(f *scorerFixture) { scorerEntry(f.input.Project, "source-audio")["muted"] = true }},
		{"muted source track", "audio", func(f *scorerFixture) { scorerTrack(f.input.Project, "audio")["muted"] = true }},
		{"silent source", "audio", func(f *scorerFixture) { scorerEntry(f.input.Project, "source-audio")["volume"] = -100 }},
		{"waveform removed", "waveform", func(f *scorerFixture) { scorerRemove(f.input.Project, "waveform") }},
		{"waveform hidden", "waveform", func(f *scorerFixture) { scorerEntry(f.input.Project, "waveform")["visible"] = false }},
		{"subscribe custom group missing layer", "subscribe", func(f *scorerFixture) { scorerRemove(f.input.Project, "subscribe-shape-1") }},
		{"wrong canvas", "resolution1080p", func(f *scorerFixture) {
			f.input.Project["metadata"] = editor.Object{"width": 1280, "height": 720, "fps": 30}
		}},
		{"deleted media", "media", func(f *scorerFixture) { delete(f.media.files, "editor/media/image-a/metadata.json") }},
		{"thumbnail bytes changed", "thumbnail", func(f *scorerFixture) { f.put("images/thumbnail.webp", []byte("tampered")) }},
		{"image bytes changed", "images", func(f *scorerFixture) { f.put("images/a.webp", []byte("tampered")) }},
		{"unapproved image", "images", func(f *scorerFixture) { f.images.Items[0].CurrentVersion = nil }},
		{"image geometry", "images", func(f *scorerFixture) { f.images.Items[0].Versions[0].Width = 1280 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScorerFixture(t)
			test.mutate(f)
			score := f.score()
			if !scorerBlocked(score, test.id) || score.Passed {
				t.Fatalf("missing %s blocker: %+v", test.id, score)
			}
			if test.id == "audio" && score.Total != 100 {
				t.Fatalf("audio prerequisite should not lose points: %g", score.Total)
			}
		})
	}
}
func scorerAddBackground(project editor.Project, clips []editor.Object) {
	project.Timeline()["tracks"] = append(scoreObjects(project.Timeline()["tracks"]), editor.Object{"id": "bg", "templateSlot": "background-video", "visible": true})
	project.Timeline()["items"] = append(scoreObjects(project.Timeline()["items"]), clips...)
}
func TestComputeScoreBackground(t *testing.T) {
	for _, test := range []struct {
		name        string
		start, tail int
		transition  bool
		pass        bool
	}{
		{"covered", 100, 80, false, true}, {"gap", 110, 70, false, false}, {"short tail", 100, 30, false, false}, {"unapproved overlap", 90, 90, false, false}, {"transition overlap", 90, 90, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScorerFixture(t)
			scorerAddBackground(f.input.Template, nil)
			scorerAddBackground(f.input.Project, []editor.Object{{"id": "bg-a", "type": "video", "trackId": "bg", "from": 0, "durationInFrames": 100}, {"id": "bg-b", "type": "video", "trackId": "bg", "from": test.start, "durationInFrames": test.tail}})
			if test.transition {
				f.input.Project.Timeline()["transitions"] = []editor.Object{{"id": "join", "trackId": "bg", "leftClipId": "bg-a", "rightClipId": "bg-b", "durationInFrames": 10}}
			}
			score := f.score()
			if (scorerItem(t, score, "background").Status == "pass") != test.pass || scorerBlocked(score, "background") == test.pass {
				t.Fatalf("%+v", score)
			}
		})
	}
	t.Run("empty required track", func(t *testing.T) {
		f := newScorerFixture(t)
		scorerAddBackground(f.input.Template, nil)
		if !scorerBlocked(f.score(), "background") {
			t.Fatal("empty required background passed")
		}
	})
}
func TestComputeScoreActualOutputOverridesTimeline(t *testing.T) {
	f := newScorerFixture(t)
	f.input.Output = &OutputEvidence{Facts: MediaFacts{DurationSec: new(12.), Width: new(1280), Height: new(720), Progressive: new(true)}, LUFS: new(-22.)}
	score := f.score()
	for _, id := range []string{"duration", "resolution1080p", "loudness"} {
		if scorerItem(t, score, id).Status != "fail" {
			t.Fatalf("%s should use output facts", id)
		}
	}
	f.input.Output = &OutputEvidence{Facts: MediaFacts{Progressive: new(false)}}
	if !scorerBlocked(f.score(), "resolution1080p") {
		t.Fatal("interlaced output passed")
	}
}
func (f *scorerFixture) align(shift int, editManifest bool) {
	subtitle := scorerEntry(f.input.Project, "caption")
	subtitle["from"] = shift
	subtitle["cues"] = []editor.Object{{"id": "cue", "startSeconds": .8, "endSeconds": 4., "text": "one two three"}}
	words := []editor.Object{}
	anchorWords := []transcript.AnchorWord{}
	transcriptWords := []transcript.Word{}
	for i, text := range []string{"one", "two", "three"} {
		words = append(words, editor.Object{"text": text, "startFrame": 30 + i*15, "endFrame": 40 + i*15, "estimated": false})
		start := 1000. + float64(i)*500
		anchorWords = append(anchorWords, transcript.AnchorWord{Text: text, StartMs: start, EndMs: start + 1000./3})
		transcriptWords = append(transcriptWords, transcript.Word{Text: text, StartMs: start, EndMs: start + 1000./3, EstimatedFlag: new(false)})
	}
	subtitle["words"] = words
	f.captions.SpeechIntervals = []transcript.Interval{{StartMs: 1500, EndMs: 2333}}
	f.captions.Cues = []transcript.Cue{{ID: "cue", StartMs: 0, EndMs: new(6000.), Text: "one two three", Source: "captions", Words: transcriptWords}}
	if editManifest {
		start, end := 400., 6400.
		f.captions = transcript.ApplyCaptionEdits(f.captions, []transcript.CaptionEdit{{CueID: "cue", StartMs: &start, EndMs: &end}}, 0)
		cue := f.captions.Cues[0]
		fill := editor.FillCue{ID: cue.ID, Text: cue.Text, StartSec: cue.StartMs / 1000, EndSec: *cue.EndMs / 1000}
		for _, word := range cue.Words {
			fill.Words = append(fill.Words, editor.FillCueWord{Text: word.Text, StartMs: word.StartMs, EndMs: word.EndMs, Estimated: word.Estimated()})
		}
		prepared := editor.PrepareCaptionData([]editor.FillCue{fill}, 30, 180)
		subtitle["cues"], subtitle["words"] = prepared.Cues, prepared.Words
	}
	f.putJSON("transcript/asr-anchors.json", transcript.AsrAnchors{Version: 1, SourceAudioSHA256: strings.Repeat("a", 64), Spans: []transcript.Interval{{StartMs: 0, EndMs: 6000}}, Words: anchorWords})
}
func TestComputeScoreIndependentCaptionAlignment(t *testing.T) {
	for _, test := range []struct {
		name   string
		shift  int
		edit   bool
		median float64
		pass   bool
	}{
		{"aligned", 0, false, 0, true},
		{"shifted item", 12, false, 400, false},
		{"edited manifest", 0, true, 400, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScorerFixture(t)
			f.align(test.shift, test.edit)
			score := f.score()
			check := scorerItem(t, score, "captionAlignment")
			metric, ok := check.Measured.(*scoreAlignmentEvidence)
			if !ok || metric.MedianMs == nil || *metric.MedianMs != test.median || metric.KaraokeRatio != 1 || (check.Status == "pass") != test.pass {
				t.Fatalf("%+v / %+v", check, metric)
			}
			if score.Total != 100 || score.Passed != test.pass || scorerBlocked(score, "captionAlignment") == test.pass {
				t.Fatalf("alignment must block independently of points %+v", score)
			}
			if !test.pass && (check.Fix == nil || check.Fix.Href != "/clone/caption-job#captions") {
				t.Fatalf("fix %+v", check.Fix)
			}
			if test.pass && len(metric.Ranges) != 0 {
				t.Fatalf("passing alignment exposed failing ranges %+v", metric.Ranges)
			}
		})
	}
	t.Run("missing anchors only warn", func(t *testing.T) {
		f := newScorerFixture(t)
		score := f.score()
		if !strings.Contains(scorerItem(t, score, "captionAlignment").Evidence, "Cảnh báo") || scorerBlocked(score, "captionAlignment") || !score.Passed {
			t.Fatalf("%+v", score)
		}
	})
}
func TestComputeScoreCutoutHashesAndDimensions(t *testing.T) {
	f := newScorerFixture(t)
	f.scenes.Recipe.Style = "cutout"
	for i := range f.images.Items {
		entry := &f.images.Items[i]
		if entry.Kind != "scene" {
			continue
		}
		path := "images/" + entry.ID + ".cutout.png"
		file := filepath.Join(f.root, filepath.FromSlash(path))
		stream, err := os.Create(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(stream, image.NewNRGBA(image.Rect(0, 0, 1920, 1080))); err != nil {
			stream.Close()
			t.Fatal(err)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f.media.files[path] = file
		entry.Versions[0].Cutout = &scenes.FileRef{Path: path, SHA256: scorerTestHash(data)}
	}
	if check := scorerItem(t, f.score(), "images"); check.Status != "pass" {
		t.Fatalf("%+v", check)
	}
	path := f.images.Items[0].Versions[0].Cutout.Path
	f.put(path, []byte("tampered"))
	if check := scorerItem(t, f.score(), "images"); check.Status != "fail" || !strings.Contains(check.Evidence, "tách nền a đổi hash") {
		t.Fatalf("%+v", check)
	}
	// Valid hash is not sufficient: the cutout must actually be the expected PNG.
	f.images.Items[0].Versions[0].Cutout.SHA256 = scorerTestHash([]byte("tampered"))
	if check := scorerItem(t, f.score(), "images"); check.Status != "fail" || !strings.Contains(check.Evidence, "sai kích thước") {
		t.Fatalf("%+v", check)
	}
}
func TestComputeScoreNoImagesAndSkippedScenes(t *testing.T) {
	t.Run("none recipe", func(t *testing.T) {
		f := newScorerFixture(t)
		f.scenes.Recipe.ImagesMode = "none"
		scorerRemove(f.input.Project, "image-a", "image-b")
		f.images.Items = f.images.Items[2:]
		score := f.score()
		if score.Total != 100 || !score.Passed {
			t.Fatalf("%+v", score)
		}
	})
	t.Run("skipped uses bound neighbor", func(t *testing.T) {
		f := newScorerFixture(t)
		f.images.Items[0].Status = "skipped"
		f.images.Items[0].CurrentVersion = nil
		scoreObjects(f.build["scenes"])[0]["mediaId"] = "image-b"
		scorerEntry(f.input.Project, "image-a")["mediaId"] = "image-b"
		score := f.score()
		if !score.Passed || !strings.Contains(scorerItem(t, score, "images").Evidence, "1 cảnh bỏ qua") {
			t.Fatalf("%+v", score)
		}
	})
}
func TestComputeScoreStorageErrorsAndCancellation(t *testing.T) {
	f := newScorerFixture(t)
	sentinel := errors.New("object store unavailable")
	f.media.failure = sentinel
	if _, err := ComputeScore(context.Background(), f.input); !errors.Is(err, sentinel) {
		t.Fatalf("storage error swallowed: %v", err)
	}
	f.media.failure = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ComputeScore(ctx, f.input); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation swallowed: %v", err)
	}
}
func TestScoreRangesUseSpeechGapsAndAlignmentFrames(t *testing.T) {
	f := newScorerFixture(t)
	scorerEntry(f.input.Project, "caption")["cues"] = []editor.Object{{"startSeconds": 0, "endSeconds": 2, "text": "first"}}
	score := f.score()
	ranges := ScoreRanges(score, f.input.Project)
	found := false
	for _, span := range ranges {
		if span.CheckID == "captions" {
			found = true
			if span.Start != 2 || span.End != 6 || span.Status != "fail" || span.ItemID != "caption" || span.CueID != "cue" || span.Text != "one two three four five six" || span.OffsetSec != nil {
				t.Fatalf("gap range %+v", span)
			}
		}
	}
	if !found {
		t.Fatal("missing speech gap range")
	}
	f = newScorerFixture(t)
	f.align(12, false)
	ranges = ScoreRanges(f.score(), f.input.Project)
	found = false
	for _, span := range ranges {
		if span.CheckID == "captionAlignment" {
			found = true
			if span.Start < 1.4 || span.End <= span.Start || span.ItemID != "caption" || span.CueID != "cue" || span.Text == "" || span.OffsetSec == nil || *span.OffsetSec != .4 {
				t.Fatalf("alignment range %+v", span)
			}
		}
	}
	if !found {
		t.Fatal("missing alignment range")
	}
	if got := ScoreRanges(nil, f.input.Project); len(got) != 0 {
		t.Fatalf("nil score ranges %+v", got)
	}
}
func TestResolveCaptionPagesStoredCueSemantics(t *testing.T) {
	item := editor.Object{"id": "caption", "durationInFrames": 180, "cues": []editor.Object{{"id": "a", "startSeconds": 0, "endSeconds": 3, "text": "one two"}, {"id": "b", "startSeconds": 2, "endSeconds": 6, "text": "three"}}, "words": []editor.Object{{"text": "one two", "startFrame": 15, "endFrame": 45, "estimated": false}, {"text": "three", "startFrame": 60, "endFrame": 75, "estimated": false}}}
	pages := ResolveCaptionPages(item, 30)
	if len(pages) != 2 || pages[0].StartFrame != 15 || pages[0].EndFrame != 60 || len(pages[0].Words) != 2 || pages[0].Words[0].StartFrame != 15 || pages[0].Words[1].StartFrame != 30 || pages[0].Estimated {
		t.Fatalf("stored pages %+v", pages)
	}
	item["wordsEstimated"] = true
	for _, page := range ResolveCaptionPages(item, 30) {
		if !page.Estimated {
			t.Fatal("item estimated flag ignored")
		}
	}
	t.Run("measured pause stays blank", func(t *testing.T) {
		pages := ResolveCaptionPages(editor.Object{"durationInFrames": 90, "words": []editor.Object{{"text": "one", "startFrame": 0, "endFrame": 10}, {"text": "two", "startFrame": 30, "endFrame": 40}}}, 30)
		if len(pages) != 2 || pages[0].EndFrame != 10 || pages[1].StartFrame != 30 {
			t.Fatalf("pause lost %+v", pages)
		}
	})
	t.Run("wordless plain cue estimates but markup stays static", func(t *testing.T) {
		pages := ResolveCaptionPages(editor.Object{"durationInFrames": 60, "cues": []editor.Object{{"id": "plain", "startSeconds": 0, "endSeconds": 1, "text": "one two"}, {"id": "markup", "startSeconds": 1, "endSeconds": 2, "text": "<b>three</b>"}}}, 30)
		if len(pages) != 2 || !pages[0].Estimated || len(pages[0].Words) != 2 || len(pages[1].Words) != 0 {
			t.Fatalf("wordless cues %+v", pages)
		}
	})
	t.Run("page limits", func(t *testing.T) {
		pages := ResolveCaptionPages(editor.Object{"durationInFrames": 120, "cues": []editor.Object{{"id": "long", "startSeconds": 0, "endSeconds": 4, "text": "one two three four five six seven eight nine ten eleven twelve"}}}, 30)
		if len(pages) < 2 {
			t.Fatal("long cue did not page")
		}
		for _, page := range pages {
			if len(page.Words) > 7 || scoreUTF16Length(page.Text) > 42 {
				t.Fatalf("page limits %+v", page)
			}
		}
	})
}

func TestScoreRangesIncludeZeroPointHardPrerequisites(t *testing.T) {
	f := newScorerFixture(t)
	scorerRemove(f.input.Project, "waveform")
	score := f.score()
	if score.Total != 100 || score.Passed {
		t.Fatalf("expected independent prerequisite failure %+v", score)
	}
	found := false
	for _, span := range ScoreRanges(score, f.input.Project) {
		if span.CheckID == "waveform" {
			found = true
			if span.Status != "fail" || span.CueID != "" || span.OffsetSec != nil {
				t.Fatalf("invented prerequisite cue evidence %+v", span)
			}
		}
	}
	if !found {
		t.Fatal("zero-point hard prerequisite missing from ranges")
	}
}
