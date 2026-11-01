package build

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

func marshalFixture(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func signedSnapshot(t *testing.T, project editor.Project) library.Snapshot {
	t.Helper()
	data, err := editor.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	snap := library.Snapshot{
		TemplateID: "template", Project: data, Style: "full", Thumbnail: "clean",
		Images:     domain.ImagesMode{Mode: "fixed", Count: 2},
		Binding:    library.TemplateBinding{ProjectID: "template", Revision: editor.Revision(data), SHA256: strings.TrimPrefix(editor.Revision(data), "sha256:"), Media: []library.TemplateMediaBinding{}},
		MediaLinks: library.TemplateMediaLinks{Version: "1.0", MediaIDs: []library.TemplateMediaLink{}},
		MediaFiles: map[string]string{},
	}
	resignLinks(t, &snap)
	return snap
}

func resignLinks(t *testing.T, snap *library.Snapshot) {
	t.Helper()
	data, err := json.MarshalIndent(snap.MediaLinks, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	snap.Binding.LinksSHA256 = strings.TrimPrefix(editor.Revision(append(data, '\n')), "sha256:")
}

func fixtureDocument[T any](t *testing.T, m *stepstest.FakeMedia, path string) T {
	t.Helper()
	doc, err := steps.DecodeJSONFile[T](filepath.Join(m.Root, filepath.FromSlash(path)), "fixture document")
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func fixtureTemplate(t *testing.T, s *pipeline.StepContext) library.Snapshot {
	t.Helper()
	var snap library.Snapshot
	if err := json.Unmarshal(s.Job.TemplateSnapshot, &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

func requireBuildGate(t *testing.T, h Handler, s *pipeline.StepContext, code string) {
	t.Helper()
	extracted := false
	extract := h.Deps.ExtractAudio
	h.Deps.ExtractAudio = func(ctx context.Context, step *pipeline.StepContext, input, output string) error {
		extracted = true
		return extract(ctx, step, input, output)
	}
	_, err := h.Run(context.Background(), s)
	var cloneErr *domain.CloneError
	if !errors.As(err, &cloneErr) || cloneErr.Code != code {
		t.Fatalf("expected %s gate before import, got %v", code, err)
	}
	if extracted {
		t.Fatal("invalid build inputs reached audio extraction/editor import")
	}
}

func TestBuildRejectsTamperedSnapshotBindingBeforeImport(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*library.Snapshot)
		code   string
	}{
		{"project digest", func(s *library.Snapshot) { s.Binding.SHA256 = strings.Repeat("a", 64) }, domain.CloneErrStale},
		{"project revision", func(s *library.Snapshot) { s.Binding.Revision = "sha256:" + strings.Repeat("b", 64) }, domain.CloneErrStale},
		{"project bytes", func(s *library.Snapshot) {
			var p editor.Project
			if err := json.Unmarshal(s.Project, &p); err != nil {
				t.Fatal(err)
			}
			p["name"] = "Changed after capture"
			s.Project = marshalFixture(t, p)
		}, domain.CloneErrStale},
		{"project identity", func(s *library.Snapshot) { s.Binding.ProjectID = "another-template" }, domain.CloneErrTemplateChanged},
		{"links digest", func(s *library.Snapshot) { s.Binding.LinksSHA256 = strings.Repeat("c", 64) }, domain.CloneErrStale},
		{"links content", func(s *library.Snapshot) {
			s.MediaLinks.MediaIDs = append(s.MediaLinks.MediaIDs, library.TemplateMediaLink{ID: "unbound", AddedAt: 1})
		}, domain.CloneErrStale},
		{"unbound file", func(s *library.Snapshot) { s.MediaFiles["unbound"] = "images/scene-0.webp" }, domain.CloneErrStale},
		{"duplicate pin", func(s *library.Snapshot) {
			pin := library.TemplateMediaBinding{MediaID: "media", ContentHash: "sha256:" + strings.Repeat("d", 64)}
			s.Binding.Media = []library.TemplateMediaBinding{pin, pin}
			s.MediaFiles["media"] = "editor/media/media/source.png"
		}, domain.CloneErrStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, s, _ := fixture(t)
			snap := fixtureTemplate(t, s)
			tc.mutate(&snap)
			s.Job.TemplateSnapshot = marshalFixture(t, snap)
			requireBuildGate(t, h, s, tc.code)
		})
	}
}

func TestBuildVerifiesPinnedTemplateMediaBeforeAudio(t *testing.T) {
	for _, mode := range []string{"deleted index", "deleted bytes", "replaced bytes", "replaced index"} {
		t.Run(mode, func(t *testing.T) {
			h, s, m := fixture(t)
			snap := fixtureTemplate(t, s)
			snap.MediaFiles["music"] = "editor/media/music/source.m4a"
			m.Put(snap.MediaFiles["music"], []byte("original template bytes"))
			hash, err := steps.SHA256File(filepath.Join(m.Root, snap.MediaFiles["music"]))
			if err != nil {
				t.Fatal(err)
			}
			snap.Binding.Media = []library.TemplateMediaBinding{{MediaID: "music", ContentHash: "sha256:" + hash}}
			m.Put("editor/media/music/metadata.json", marshalFixture(t, editor.Object{"contentHash": "sha256:" + hash, "sourceFile": "source.m4a"}))
			snap.MediaLinks.MediaIDs = []library.TemplateMediaLink{{ID: "music", AddedAt: 1}}
			resignLinks(t, &snap)
			s.Job.TemplateSnapshot = marshalFixture(t, snap)
			switch mode {
			case "deleted index":
				if err := os.Remove(filepath.Join(m.Root, "editor/media/music/metadata.json")); err != nil {
					t.Fatal(err)
				}
			case "deleted bytes":
				if err := os.Remove(filepath.Join(m.Root, snap.MediaFiles["music"])); err != nil {
					t.Fatal(err)
				}
			case "replaced bytes":
				m.Put(snap.MediaFiles["music"], []byte("new bytes under the same media id"))
			case "replaced index":
				m.Put("editor/media/music/metadata.json", marshalFixture(t, editor.Object{"contentHash": "sha256:" + strings.Repeat("e", 64), "sourceFile": "replacement.m4a"}))
			}
			requireBuildGate(t, h, s, domain.CloneErrStale)
		})
	}
}

func TestBuildRejectsIncompleteOrMismatchedImagesBeforeImport(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*images.Manifest)
		code   string
	}{
		{"manifest version", func(m *images.Manifest) { m.Version = 2 }, domain.CloneErrStale},
		{"style drift", func(m *images.Manifest) { m.Style = "cutout" }, domain.CloneErrStale},
		{"missing scene", func(m *images.Manifest) { m.Items = m.Items[1:] }, domain.CloneErrStale},
		{"duplicate scene", func(m *images.Manifest) { m.Items = append(m.Items, m.Items[0]) }, domain.CloneErrStale},
		{"foreign scene", func(m *images.Manifest) { m.Items[0].SceneID = "foreign" }, domain.CloneErrStale},
		{"changed span", func(m *images.Manifest) {
			m.Items[0].EndFrame--
			m.Items[0].Versions[0].SourceFingerprint = images.SourceFingerprint(m.Items[0])
		}, domain.CloneErrStale},
		{"changed reference", func(m *images.Manifest) {
			m.Items[0].Reference = scenes.FileRef{Path: "scenes/other.jpg", SHA256: strings.Repeat("e", 64)}
			m.Items[0].Versions[0].RefHash = m.Items[0].Reference.SHA256
			m.Items[0].Versions[0].SourceFingerprint = images.SourceFingerprint(m.Items[0])
		}, domain.CloneErrStale},
		{"changed narration", func(m *images.Manifest) {
			m.Items[0].Narration = "changed scene"
			m.Items[0].Versions[0].SourceFingerprint = images.SourceFingerprint(m.Items[0])
		}, domain.CloneErrStale},
		{"changed title", func(m *images.Manifest) {
			m.Items[2].Narration = "another video"
			m.Items[2].Versions[0].SourceFingerprint = images.SourceFingerprint(m.Items[2])
		}, domain.CloneErrStale},
		{"changed context", func(m *images.Manifest) {
			m.Items[0].Context = &scenes.TextContext{CueID: "c2", Position: "next", Text: "Câu hai"}
			m.Items[0].Versions[0].SourceFingerprint = images.SourceFingerprint(m.Items[0])
		}, domain.CloneErrStale},
		{"pending with old candidate", func(m *images.Manifest) { m.Items[0].Status = "pending" }, domain.CloneErrAwaitingReview},
		{"failed with old candidate", func(m *images.Manifest) { m.Items[0].Status = "failed" }, domain.CloneErrAwaitingReview},
		{"in flight candidate", func(m *images.Manifest) { m.Items[0].InFlight = &images.Flight{Version: 2} }, domain.CloneErrAwaitingReview},
		{"missing candidate", func(m *images.Manifest) { m.Items[0].ReviewVersion = nil }, domain.CloneErrAwaitingReview},
		{"stale candidate", func(m *images.Manifest) { m.Items[0].Versions[0].SourceFingerprint = strings.Repeat("f", 64) }, domain.CloneErrStale},
		{"wrong dimensions", func(m *images.Manifest) { m.Items[0].Versions[0].Width = 1280 }, domain.CloneErrStale},
		{"wrong mime", func(m *images.Manifest) { m.Items[0].Versions[0].Mime = "image/png" }, domain.CloneErrStale},
		{"incomplete bytes", func(m *images.Manifest) { m.Items[0].Versions[0].Bytes = 0 }, domain.CloneErrStale},
		{"incorrect bytes", func(m *images.Manifest) { m.Items[0].Versions[0].Bytes++ }, domain.CloneErrStale},
		{"missing generation", func(m *images.Manifest) { m.Items[0].Versions[0].GenerationID = "" }, domain.CloneErrStale},
		{"missing timestamp", func(m *images.Manifest) { m.Items[0].Versions[0].CreatedAt = "" }, domain.CloneErrStale},
		{"duplicate version", func(m *images.Manifest) { m.Items[0].Versions = append(m.Items[0].Versions, m.Items[0].Versions[0]) }, domain.CloneErrStale},
		{"empty hash", func(m *images.Manifest) { m.Items[0].Versions[0].SHA256 = "" }, domain.CloneErrStale},
		{"changed thumbnail bytes", func(m *images.Manifest) { m.Items[2].Versions[0].SHA256 = strings.Repeat("a", 64) }, domain.CloneErrStale},
		{"missing thumbnail", func(m *images.Manifest) { m.Items = m.Items[:2] }, domain.CloneErrStale},
		{"skipped thumbnail", func(m *images.Manifest) { m.Items[2].Status = "skipped" }, domain.CloneErrStale},
		{"all scenes skipped", func(m *images.Manifest) { m.Items[0].Status = "skipped"; m.Items[1].Status = "skipped" }, domain.CloneErrStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, s, m := fixture(t)
			doc := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
			tc.mutate(&doc)
			m.Put("images/manifest.json", marshalFixture(t, doc))
			requireBuildGate(t, h, s, tc.code)
		})
	}
}

func TestBuildRejectsSceneSourceAndRecipeDriftBeforeImport(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*scenes.Manifest)
		code   string
	}{
		{"source digest", func(m *scenes.Manifest) { m.Source.SHA256 = strings.Repeat("a", 64) }, domain.CloneErrStale},
		{"scene narration drift", func(m *scenes.Manifest) { m.Scenes[0].Text = "changed after drawing" }, domain.CloneErrStale},
		{"scene context drift", func(m *scenes.Manifest) {
			m.Scenes[0].Context = &scenes.TextContext{CueID: "c2", Position: "next", Text: "Câu hai"}
		}, domain.CloneErrStale},
		{"scene analysis pending", func(m *scenes.Manifest) { m.Pending = &scenes.Analysis{Kind: "timing"} }, domain.CloneErrAwaitingReview},
		{"recipe count", func(m *scenes.Manifest) { m.Recipe.ImageCount++ }, domain.CloneErrTemplateChanged},
		{"recipe style", func(m *scenes.Manifest) { m.Recipe.Style = "cutout" }, domain.CloneErrTemplateChanged},
		{"recipe mode", func(m *scenes.Manifest) { m.Recipe.ImagesMode = "none" }, domain.CloneErrTemplateChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, s, m := fixture(t)
			doc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
			tc.mutate(&doc)
			m.Put("scenes/manifest.json", marshalFixture(t, doc))
			requireBuildGate(t, h, s, tc.code)
		})
	}
}

func builtProject(t *testing.T, h Handler, s *pipeline.StepContext, m *stepstest.FakeMedia) editor.Project {
	t.Helper()
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Publish(context.Background(), result.Artifacts, nil); err != nil {
		t.Fatal(err)
	}
	project, _, err := (editor.Store{Media: m}).Read(context.Background(), result.Data["projectId"].(string))
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func sceneItems(t *testing.T, project editor.Project) []editor.Object {
	t.Helper()
	var out []editor.Object
	items, ok := project.Timeline()["items"].([]any)
	if !ok {
		t.Fatal("timeline items missing")
	}
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			t.Fatal("invalid timeline item")
		}
		if item["type"] == "image" && item["trackId"] == "scenes" {
			out = append(out, item)
		}
	}
	return out
}

func TestBuildImportsCompleteCanonicalEngineMetadata(t *testing.T) {
	h, s, m := fixture(t)
	project := builtProject(t, h, s, m)
	refs := project["mediaItems"].([]any)
	if len(refs) != 4 {
		t.Fatalf("wanted audio, two scenes and thumbnail; got %d media", len(refs))
	}
	roles := map[string]int{}
	for _, value := range refs {
		id := value.(map[string]any)["id"].(string)
		record := fixtureDocument[editor.Object](t, m, "editor/media/"+id+"/metadata.json")
		tags, ok := record["tags"].([]any)
		if !ok || len(tags) != 1 {
			t.Fatalf("missing canonical role tags: %+v", record)
		}
		roles[tags[0].(string)]++
		if _, ok := record["createdAt"]; ok {
			t.Fatalf("content-addressed record carries job bookkeeping: %+v", record)
		}
		if record["contentHash"] == "" || record["sourceFile"] == "" {
			t.Fatalf("incomplete persisted record: %+v", record)
		}
		probe := record["probe"].(map[string]any)
		if probe["kind"] != "video" && probe["fps"] != float64(0) {
			t.Fatalf("nonvideo media has a fabricated frame rate: %+v", probe)
		}
	}
	if !reflect.DeepEqual(roles, map[string]int{"clone-audio": 1, "clone-scene": 2, "clone-thumbnail": 1}) {
		t.Fatal(roles)
	}
	if project["createdAt"] != float64(1) || project["updatedAt"] != float64(1) {
		t.Fatal("project timestamps changed")
	}
}

func TestBuildReviewUsesCompletedCandidateInsteadOfOlderApprovedVersion(t *testing.T) {
	h, s, m := fixture(t)
	doc := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	item := &doc.Items[0]
	candidate := item.Versions[0]
	candidate.Version = 2
	candidate.GenerationID = "generation-2"
	candidate.Path = "images/new-candidate.webp"
	m.Put(candidate.Path, []byte("new candidate"))
	var err error
	candidate.SHA256, err = steps.SHA256File(filepath.Join(m.Root, candidate.Path))
	if err != nil {
		t.Fatal(err)
	}
	candidate.Bytes = int64(len("new candidate"))
	one, two := 1, 2
	item.CurrentVersion, item.ReviewVersion = &one, &two
	item.Versions = append(item.Versions, candidate)
	m.Put("images/manifest.json", marshalFixture(t, doc))
	project := builtProject(t, h, s, m)
	items := sceneItems(t, project)
	if len(items) != 2 || items[0]["mediaId"] != "media-"+candidate.SHA256[:32] {
		t.Fatalf("draft did not show the pending candidate: %+v", items)
	}
	after := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	if after.Items[0].Status != "review" || *after.Items[0].CurrentVersion != 1 || *after.Items[0].ReviewVersion != 2 {
		t.Fatal("build approved or rewrote review state")
	}
}

func TestBuildSkippedScenesBorrowEarlierImageWithoutDuplicateImports(t *testing.T) {
	h, s, m := fixture(t)
	sceneDoc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
	imageDoc := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	base := imageDoc.Items[0]
	thumb := imageDoc.Items[2]
	sceneDoc.Scenes = nil
	imageDoc.Items = nil
	for n := range 4 {
		id := "scene-" + string(rune('a'+n))
		sceneDoc.Scenes = append(sceneDoc.Scenes, scenes.Scene{SceneID: id, StartFrame: n * 45, EndFrame: (n + 1) * 45, KeyframeFrame: n*45 + 20})
		item := base
		item.ID, item.SceneID, item.StartFrame, item.EndFrame = id, id, n*45, (n+1)*45
		item.Narration = ""
		item.Versions = append([]images.Version(nil), base.Versions...)
		if n == 3 {
			item.Versions[0] = imageDocVersion(t, m, "images/scene-1.webp")
		}
		item.Versions[0].SourceFingerprint = images.SourceFingerprint(item)
		if n == 1 || n == 2 {
			item.Status = "skipped"
		}
		imageDoc.Items = append(imageDoc.Items, item)
	}
	imageDoc.Items = append(imageDoc.Items, thumb)
	m.Put("scenes/manifest.json", marshalFixture(t, sceneDoc))
	m.Put("images/manifest.json", marshalFixture(t, imageDoc))
	project := builtProject(t, h, s, m)
	items := sceneItems(t, project)
	if len(items) != 4 {
		t.Fatal(items)
	}
	for n := 1; n <= 2; n++ {
		if items[n]["mediaId"] != items[0]["mediaId"] {
			t.Fatalf("skipped scene %d borrowed a later image: %+v", n, items)
		}
	}
	if len(project["mediaItems"].([]any)) != 4 {
		t.Fatal("borrowed scenes duplicated editor media associations")
	}
}

func imageDocVersion(t *testing.T, m *stepstest.FakeMedia, path string) images.Version {
	t.Helper()
	hash, err := steps.SHA256File(filepath.Join(m.Root, path))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(m.Root, path))
	if err != nil {
		t.Fatal(err)
	}
	return images.Version{Version: 1, GenerationID: "generation-1", CreatedAt: time.UnixMilli(1).UTC().Format(time.RFC3339Nano), Path: path, SHA256: hash, Bytes: info.Size(), Width: 1920, Height: 1080, Mime: "image/webp"}
}

func TestBuildNoImagesUsesCoverOnlyManifestWithoutSceneImports(t *testing.T) {
	h, s, m := fixture(t)
	s.Job.NoImages = true
	sceneDoc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
	sceneDoc.Recipe.ImagesMode, sceneDoc.Recipe.ImageCount = "none", 0
	imageDoc := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	imageDoc.Items = imageDoc.Items[2:]
	m.Put("scenes/manifest.json", marshalFixture(t, sceneDoc))
	m.Put("images/manifest.json", marshalFixture(t, imageDoc))
	project := builtProject(t, h, s, m)
	if len(sceneItems(t, project)) != 0 {
		t.Fatal("no-images job created scene image items")
	}
	if len(project["mediaItems"].([]any)) != 2 {
		t.Fatal("no-images job imported scene images")
	}
}

func TestBuildValidatesCutoutHeadersBeforeImport(t *testing.T) {
	for _, width := range []int{1280, 1920} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			h, s, m := fixture(t)
			snap := fixtureTemplate(t, s)
			snap.Style = "cutout"
			s.Job.TemplateSnapshot = marshalFixture(t, snap)
			sceneDoc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
			sceneDoc.Recipe.Style = "cutout"
			imageDoc := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
			imageDoc.Style = "cutout"
			file := filepath.Join(m.Root, "images", "subject.png")
			output, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(output, image.NewNRGBA(image.Rect(0, 0, width, 1080))); err != nil {
				output.Close()
				t.Fatal(err)
			}
			if err := output.Close(); err != nil {
				t.Fatal(err)
			}
			hash, err := steps.SHA256File(file)
			if err != nil {
				t.Fatal(err)
			}
			for index := range imageDoc.Items {
				if imageDoc.Items[index].Kind == "scene" {
					imageDoc.Items[index].Versions[0].Cutout = &scenes.FileRef{Path: "images/subject.png", SHA256: hash}
				}
			}
			m.Put("scenes/manifest.json", marshalFixture(t, sceneDoc))
			m.Put("images/manifest.json", marshalFixture(t, imageDoc))
			if width != 1920 {
				requireBuildGate(t, h, s, domain.CloneErrStale)
				return
			}
			project := builtProject(t, h, s, m)
			for _, item := range sceneItems(t, project) {
				if item["mediaId"] != "media-"+hash[:32] {
					t.Fatal("cutout recipe imported the rendered WebP")
				}
			}
		})
	}
}

func TestBuildRebindsAllTemplateMediaReferencesAndMeasuresImages(t *testing.T) {
	h, s, m := fixture(t)
	snap := fixtureTemplate(t, s)
	project, err := editor.Parse(snap.Project)
	if err != nil {
		t.Fatal(err)
	}
	project["mediaItems"] = []any{editor.Object{"id": "template-cover"}}
	tracks := project.Timeline()["tracks"].([]any)
	tracks[0].(map[string]any)["templateScenes"] = editor.Object{"style": "full", "coverMediaId": "template-cover"}
	snap = signedSnapshot(t, project)
	snap.CoverMediaID = "template-cover"
	file := "editor/media/template-cover/source.png"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(m.Root, file)), 0755); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(m.Root, file))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(output, image.NewNRGBA(image.Rect(0, 0, 320, 180))); err != nil {
		output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := steps.SHA256File(filepath.Join(m.Root, file))
	if err != nil {
		t.Fatal(err)
	}
	m.Put("editor/media/template-cover/metadata.json", marshalFixture(t, editor.Object{"contentHash": "sha256:" + hash, "sourceFile": "source.png"}))
	snap.MediaFiles["template-cover"] = file
	snap.Binding.Media = []library.TemplateMediaBinding{{MediaID: "template-cover", ContentHash: "sha256:" + hash}}
	snap.MediaLinks.MediaIDs = []library.TemplateMediaLink{{ID: "template-cover", AddedAt: 1}}
	resignLinks(t, &snap)
	s.Job.TemplateSnapshot = marshalFixture(t, snap)
	built := builtProject(t, h, s, m)
	if len(built["mediaItems"].([]any)) != 5 {
		t.Fatal("project retained obsolete template media associations")
	}
	for _, value := range built["mediaItems"].([]any) {
		if value.(map[string]any)["id"] == "template-cover" {
			t.Fatal("old template media id survived import")
		}
	}
	newID := "media-" + hash[:32]
	recipe := built.Timeline()["tracks"].([]any)[0].(map[string]any)["templateScenes"].(map[string]any)
	if recipe["coverMediaId"] != newID {
		t.Fatal("template recipe retained an obsolete cover id")
	}
	record := fixtureDocument[editor.Object](t, m, "editor/media/"+newID+"/metadata.json")
	probe := record["probe"].(map[string]any)
	if probe["width"] != float64(320) || probe["height"] != float64(180) {
		t.Fatalf("image geometry was inferred from a missing timeline clip: %+v", probe)
	}
	if tags, ok := record["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("template tags are not an empty canonical array: %+v", record)
	}
}

func TestBuildAcceptsMatchingSceneContextByValue(t *testing.T) {
	h, s, m := fixture(t)
	sceneDoc := fixtureDocument[scenes.Manifest](t, m, "scenes/manifest.json")
	imageDoc := fixtureDocument[images.Manifest](t, m, "images/manifest.json")
	sceneDoc.Scenes[0].Context = &scenes.TextContext{CueID: "c2", Position: "next", Text: "Câu hai"}
	imageDoc.Items[0].Context = &scenes.TextContext{CueID: "c2", Position: "next", Text: "Câu hai"}
	imageDoc.Items[0].Versions[0].SourceFingerprint = images.SourceFingerprint(imageDoc.Items[0])
	m.Put("scenes/manifest.json", marshalFixture(t, sceneDoc))
	m.Put("images/manifest.json", marshalFixture(t, imageDoc))
	project := builtProject(t, h, s, m)
	if len(sceneItems(t, project)) != 2 {
		t.Fatal("matching narration context prevented a valid build")
	}
}

// buildMediaRecords runs the same build under a given job clock and returns the
// bytes of every content-addressed media record it produced.
func buildMediaRecords(t *testing.T, created time.Time) map[string]string {
	t.Helper()
	h, s, _ := fixture(t)
	s.Job.CreatedAt = created
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, artifact := range result.Artifacts {
		if !strings.HasPrefix(artifact.Path, "editor/media/") || !strings.HasSuffix(artifact.Path, "/metadata.json") {
			continue
		}
		raw, err := os.ReadFile(artifact.File)
		if err != nil {
			t.Fatal(err)
		}
		out[artifact.Path] = string(raw)
	}
	return out
}

// Same digest must mean same bytes: two jobs of the same source share audio and
// template media, so the record cannot carry the build clock.
func TestBuildMediaMetadataIsContentAddressedAcrossJobs(t *testing.T) {
	first := buildMediaRecords(t, time.UnixMilli(1))
	second := buildMediaRecords(t, time.UnixMilli(1_700_000_000_000))
	if len(first) == 0 {
		t.Fatal("build produced no media records")
	}
	for path, raw := range first {
		other, ok := second[path]
		if !ok {
			t.Fatalf("second build lost %s", path)
		}
		if raw != other {
			t.Fatalf("%s differs between jobs:\n%s\n%s", path, raw, other)
		}
	}
}
