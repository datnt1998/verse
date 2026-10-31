package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
)

func templateProjectForTest(t *testing.T, raw json.RawMessage) editor.Project {
	t.Helper()
	project, err := editor.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func templateProjectBytes(t *testing.T, project editor.Project) json.RawMessage {
	t.Helper()
	data, err := editor.Marshal(project)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func templateTrackForTest(t *testing.T, project editor.Project, slot string) map[string]any {
	t.Helper()
	for _, value := range project.Timeline()["tracks"].([]any) {
		track := value.(map[string]any)
		if track["templateSlot"] == slot {
			return track
		}
	}
	t.Fatalf("missing %s track", slot)
	return nil
}

func TestTemplateProjectBackfillIsDurableAndDoesNotBumpRevision(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	created := svc.now().Add(-time.Hour)
	legacy := domain.Template{ID: "legacy", Name: "Đã có", Recipe: StandardRecipe(), Revision: 7, CreatedAt: created, UpdatedAt: created}
	legacy.Recipe.Scenes.Style, legacy.Recipe.Scenes.Framing = StyleCutout, FramingClose
	legacy.Recipe.Scenes.Images = domain.ImagesMode{Mode: ImagesNone}
	legacy.Recipe.Output.CompressExport = false
	if err := store.InsertTemplate(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	first, err := svc.SnapshotTemplate(ctx, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.GetTemplate(ctx, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Project) == 0 || stored.Revision != 7 || !stored.UpdatedAt.Equal(created) {
		t.Fatalf("backfill changed authored revision: %+v", stored)
	}
	second, err := svc.SnapshotTemplate(ctx, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored.Project, first.Project) || !bytes.Equal(first.Project, second.Project) || !reflect.DeepEqual(first.Binding, second.Binding) {
		t.Fatal("backfill was not persisted with stable bindings")
	}
	if first.Style != StyleCutout || first.Framing != FramingClose || first.Images.Mode != ImagesNone || first.CompressExport {
		t.Fatalf("legacy settings were lost: %+v", first)
	}
	project := templateProjectForTest(t, first.Project)
	if project["id"] != legacy.ID || project["createdAt"] != float64(created.UnixMilli()) {
		t.Fatal("backfill changed project identity")
	}
}

type competingTemplateStore struct {
	*memStore
	winner json.RawMessage
	calls  int
}

func (s *competingTemplateStore) InitializeTemplateProject(_ context.Context, id string, _ json.RawMessage, _ int64) (bool, error) {
	s.calls++
	s.mu.Lock()
	defer s.mu.Unlock()
	tpl := s.templates[id]
	tpl.Project = append(json.RawMessage(nil), s.winner...)
	tpl.Revision++
	s.templates[id] = tpl
	return false, nil
}

func TestTemplateBackfillLosesCASWithoutOverwritingAuthoredWinner(t *testing.T) {
	base := newMemStore()
	ctx := context.Background()
	svc := testService(t, base)
	tpl, err := svc.Create(ctx, "Nguyên bản")
	if err != nil {
		t.Fatal(err)
	}
	winner := templateProjectForTest(t, tpl.Project)
	winner["ownerField"] = map[string]any{"keep": true}
	winnerBytes := templateProjectBytes(t, winner)
	tpl.Project = nil
	base.templates[tpl.ID] = *tpl
	store := &competingTemplateStore{memStore: base, winner: winnerBytes}
	svc = testService(t, store)
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || snapshot.Revision != 2 || !bytes.Equal(snapshot.Project, winnerBytes) {
		t.Fatal("backfill did not use the concurrent authored project")
	}
}

func TestTemplateProjectSaveProjectsRecipeAndPreservesAuthoredData(t *testing.T) {
	store := newMemStore()
	store.backdrop["twitch:owner"] = true
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Biên tập")
	if err != nil {
		t.Fatal(err)
	}
	project := templateProjectForTest(t, tpl.Project)
	project["ownerField"] = map[string]any{"future": "retain"}
	project.Timeline()["compositions"] = []any{map[string]any{"id": "nested", "tracks": []any{}, "items": []any{}, "future": true}}
	scenes := templateTrackForTest(t, project, "scenes")
	scenes["templateScenes"] = map[string]any{
		"style": "cutout-close", "images": map[string]any{"mode": "per-minutes", "minutes": 3},
		"motion":    map[string]any{"kind": "wander", "intensity": 1.5, "speed": 2},
		"thumbnail": "hook-arrow", "opacity": .4, "compress": false,
	}
	appearance := DefaultAppearance()
	appearance.Speed, appearance.Zoom, appearance.FlipX = .5, 1.4, true
	background := map[string]any{"id": "owner-background", "kind": "video", "templateSlot": "background-video", "templateBackgroundAppearance": appearance,
		"templateBackdrop": domain.BackdropRecipe{Channels: []domain.BackdropChannelLink{{ChannelID: "twitch:owner", Kinds: []string{"vod"}, Pinned: []string{}}}, ClipSec: 45}}
	project.Timeline()["tracks"] = append(project.Timeline()["tracks"].([]any), background)
	beforeTimeline := templateProjectBytes(t, editor.Project{"timeline": project.Timeline()})
	updated, err := svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Project: templateProjectBytes(t, project)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Recipe.Scenes.Style != StyleCutout || updated.Recipe.Scenes.Framing != FramingClose || updated.Recipe.Scenes.Images.Minutes != 3 || updated.Recipe.Scenes.Motion.Kind != MotionWander || updated.Recipe.Scenes.SceneOpacity != .4 || updated.Recipe.Output.CompressExport || updated.Recipe.Scenes.CoverPromptID != BuiltinCoverHookArrow || updated.Recipe.BackgroundAppearance != appearance {
		t.Fatalf("project recipe not projected: %+v", updated.Recipe)
	}
	persisted, err := store.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted.Recipe, updated.Recipe) {
		t.Fatal("project recipe was not persisted atomically")
	}
	gotProject := templateProjectForTest(t, persisted.Project)
	if !reflect.DeepEqual(templateProjectForTest(t, beforeTimeline).Timeline(), gotProject.Timeline()) {
		t.Fatal("project save rebuilt authored timeline")
	}
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ImageCountKnown || list[0].BackdropChannels != 1 {
		t.Fatalf("summary is stale: %+v", list)
	}
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Style != StyleCutout || snapshot.Framing != FramingClose || snapshot.Thumbnail != "hook-arrow" || snapshot.Appearance != appearance || snapshot.Images.Minutes != 3 || snapshot.CompressExport {
		t.Fatal("snapshot recipe is stale")
	}
	// Settings writes replace only settings-owned fields and retain nested edits.
	patched, err := svc.Patch(ctx, tpl.ID, Patch{Revision: 2, Recipe: []byte(`{"scenes":{"sceneOpacity":0.7}}`)})
	if err != nil {
		t.Fatal(err)
	}
	after := templateProjectForTest(t, patched.Project)
	if !reflect.DeepEqual(after["ownerField"], gotProject["ownerField"]) || !reflect.DeepEqual(after.Timeline()["compositions"], gotProject.Timeline()["compositions"]) {
		t.Fatal("settings erased authored data")
	}
	if patched.Recipe.Scenes.Images.Minutes != 3 || patched.Recipe.Scenes.SceneOpacity != .7 {
		t.Fatal("partial settings lost projected fields")
	}
	snapshot, err = svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snapshot.Project, patched.Project) {
		t.Fatal("settings serialization changed snapshot byte identity")
	}
	stale := Patch{Revision: 2, Project: tpl.Project}
	if _, err := svc.Patch(ctx, tpl.ID, stale); CodeOf(err) != CodeConflict {
		t.Fatalf("stale project accepted: %v", err)
	}
	final, err := store.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(final.Project, patched.Project) || final.Revision != 3 {
		t.Fatal("stale save changed project")
	}
}

func TestTemplateDuplicatePreservesSnapshotAndDetachedAuthoredTimeline(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Gốc")
	if err != nil {
		t.Fatal(err)
	}
	project := templateProjectForTest(t, tpl.Project)
	project.Timeline()["ownerField"] = map[string]any{"keep": true}
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Project: templateProjectBytes(t, project)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshotBytes := append([]byte(nil), snapshot.Project...)
	copy, err := svc.Duplicate(ctx, tpl.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	copyProject := templateProjectForTest(t, copy.Project)
	if copy.ID == tpl.ID || copy.Name != "Gốc — Bản sao" || copy.Revision != 1 || copyProject["id"] != copy.ID || copyProject["name"] != copy.Name {
		t.Fatal("duplicate did not get fresh identity")
	}
	originalProject := templateProjectForTest(t, tpl.Project)
	if !reflect.DeepEqual(originalProject.Timeline(), copyProject.Timeline()) {
		t.Fatal("duplicate rebuilt the authored timeline")
	}
	copyProject.Timeline()["ownerField"] = "changed"
	if _, err := svc.Patch(ctx, copy.ID, Patch{Revision: 1, Project: templateProjectBytes(t, copyProject)}); err != nil {
		t.Fatal(err)
	}
	original, err := store.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original.Project, tpl.Project) || !bytes.Equal(snapshot.Project, snapshotBytes) {
		t.Fatal("duplicate edit changed source or snapshot")
	}
}

func TestTemplateSnapshotBindsLinkedAndCoverOnlyMediaDeterministically(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Bìa riêng")
	if err != nil {
		t.Fatal(err)
	}
	store.overlays["cover-only"] = domain.Overlay{ID: "cover-only", FilePath: "library/overlays/cover.webp", SHA256: strings.Repeat("c", 64), Mime: "image/webp"}
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: []byte(`{"scenes":{"images":{"mode":"none"},"coverMediaId":"cover-only"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	projectHash := sha256.Sum256(snapshot.Project)
	links, err := json.MarshalIndent(snapshot.MediaLinks, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	linksHash := sha256.Sum256(append(links, '\n'))
	if snapshot.Binding.ProjectID != tpl.ID || snapshot.Binding.SHA256 != hex.EncodeToString(projectHash[:]) || snapshot.Binding.Revision != editor.Revision(snapshot.Project) || snapshot.Binding.LinksSHA256 != hex.EncodeToString(linksHash[:]) {
		t.Fatal("snapshot identity does not bind the actual project and links bytes")
	}
	if len(snapshot.Binding.Media) != 5 || len(snapshot.MediaLinks.MediaIDs) != 5 || snapshot.MediaFiles["cover-only"] != "library/overlays/cover.webp" {
		t.Fatalf("cover-only media was not bound: %+v", snapshot.Binding)
	}
	previous := ""
	for i, binding := range snapshot.Binding.Media {
		if binding.MediaID <= previous || snapshot.MediaLinks.MediaIDs[i].ID != binding.MediaID || snapshot.MediaFiles[binding.MediaID] == "" || len(binding.ContentHash) != 71 {
			t.Fatal("binding is unsorted or missing media")
		}
		previous = binding.MediaID
	}
	delete(store.overlays, "cover-only")
	if _, err := svc.SnapshotTemplate(ctx, tpl.ID); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("missing cover was not rejected: %v", err)
	}
}

func TestTemplateProjectRejectsInvalidSettingsWithoutChangingRow(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Kiểm tra")
	if err != nil {
		t.Fatal(err)
	}
	for _, settings := range []string{
		`{"style":"cutout-close","images":{"mode":"fixed","count":0},"motion":{"kind":"wander","intensity":1,"speed":1}}`,
		`{"style":"full","images":{"mode":"none"},"motion":{"kind":"wander","intensity":1,"speed":1},"compress":true}`,
		`{"style":"full","images":{"mode":"none"},"motion":{"kind":"wander","intensity":1,"speed":1},"thumbnail":"unknown"}`,
		`{"style":"full","images":{"mode":"none"},"motion":{"kind":"wander","intensity":1,"speed":1},"imagePrompt":" untrimmed "}`,
		`{"style":"full","images":{"mode":"none"},"motion":{"kind":"wander","intensity":1,"speed":1},"unknown":true}`,
		`{"style":"full","images":{"mode":"none","count":2},"motion":{"kind":"wander","intensity":1,"speed":1}}`,
		`{"style":"full","images":{"mode":"fixed","count":5,"minutes":2},"motion":{"kind":"wander","intensity":1,"speed":1}}`,
		`{"style":"full","images":{"mode":"none"},"motion":{"kind":"wander","intensity":1,"speed":1},"opacity":null}`,
	} {
		project := templateProjectForTest(t, tpl.Project)
		var value map[string]any
		if err := json.Unmarshal([]byte(settings), &value); err != nil {
			t.Fatal(err)
		}
		templateTrackForTest(t, project, "scenes")["templateScenes"] = value
		if _, err := svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Project: templateProjectBytes(t, project)}); CodeOf(err) != CodeInvalidInput {
			t.Fatalf("invalid project accepted: %s: %v", settings, err)
		}
	}
	stored, err := store.GetTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 1 || !bytes.Equal(stored.Project, tpl.Project) {
		t.Fatal("rejected project changed stored revision")
	}
}

func TestTemplateBackfillPropagatesStoreFailure(t *testing.T) {
	store := &failedTemplateStore{memStore: newMemStore(), err: errors.New("write unavailable")}
	svc := testService(t, store)
	store.templates["legacy"] = domain.Template{ID: "legacy", Name: "Cũ", Recipe: StandardRecipe(), Revision: 1, CreatedAt: svc.now()}
	if _, err := svc.SnapshotTemplate(context.Background(), "legacy"); !errors.Is(err, store.err) {
		t.Fatalf("store failure hidden: %v", err)
	}
	if len(store.templates["legacy"].Project) != 0 {
		t.Fatal("failed backfill exposed a project")
	}
}

type failedTemplateStore struct {
	*memStore
	err error
}

func (s *failedTemplateStore) InitializeTemplateProject(context.Context, string, json.RawMessage, int64) (bool, error) {
	return false, s.err
}

func TestTemplateRecipeReadsFirstSlotsAndClearedSettingsUseCanonicalDefaults(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Mặc định")
	if err != nil {
		t.Fatal(err)
	}
	project := templateProjectForTest(t, tpl.Project)
	second := map[string]any{"id": "later-scenes", "templateSlot": "scenes", "templateScenes": map[string]any{"style": "cutout", "images": map[string]any{"mode": "none"}, "motion": map[string]any{"kind": "wander", "intensity": 1, "speed": 1}}}
	project.Timeline()["tracks"] = append(project.Timeline()["tracks"].([]any), second)
	templateTrackForTest(t, project, "scenes")["templateScenes"] = nil
	stale := tpl.Recipe
	stale.Scenes.Style, stale.Scenes.Framing = StyleCutout, FramingClose
	stale.Scenes.Images = domain.ImagesMode{Mode: ImagesNone}
	stale.Scenes.SceneOpacity, stale.Output.CompressExport = .2, false
	stale.BackgroundAppearance.Speed = 2
	got, err := svc.projectTemplateRecipe(ctx, templateProjectBytes(t, project), stale)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, StandardRecipe()) {
		t.Fatalf("cleared first slot inherited stale row or later slot: %+v", got)
	}
}

func TestTemplateCustomCoverAndEditorPromptSurviveSettingsWrites(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	cover, err := svc.CreatePrompt(ctx, PromptInput{Name: "Bìa riêng", Kind: domain.PromptCover, Composition: "split", Prompt: new("Bố cục riêng")})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := svc.Create(ctx, "Prompt riêng")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"scenes": map[string]any{"coverPromptId": cover.ID}})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Recipe: body})
	if err != nil {
		t.Fatal(err)
	}
	project := templateProjectForTest(t, tpl.Project)
	settings := templateTrackForTest(t, project, "scenes")["templateScenes"].(map[string]any)
	if settings["thumbnail"] != "split" {
		t.Fatal("custom cover composition not written to project")
	}
	settings["imagePrompt"] = "Ánh sáng dịu và màu lạnh"
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 2, Project: templateProjectBytes(t, project)})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 3, Recipe: []byte(`{"scenes":{"sceneOpacity":0.5}}`)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := svc.SnapshotTemplate(ctx, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Recipe.Scenes.CoverPromptID != cover.ID || snapshot.CoverPrompt.ID != cover.ID || snapshot.Thumbnail != "split" || snapshot.ImagePrompt != "Ánh sáng dịu và màu lạnh" {
		t.Fatalf("authored prompt or custom cover lost: %+v", snapshot)
	}
}

func TestTemplateSettingsInsertBackgroundBeforeSourceAudioWithoutIDCollision(t *testing.T) {
	store := newMemStore()
	svc := testService(t, store)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Nền")
	if err != nil {
		t.Fatal(err)
	}
	project := templateProjectForTest(t, tpl.Project)
	project.Timeline()["tracks"] = append(project.Timeline()["tracks"].([]any), map[string]any{"id": "standard-background", "kind": "video", "name": "Giữ lại"})
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 1, Project: templateProjectBytes(t, project)})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = svc.Patch(ctx, tpl.ID, Patch{Revision: 2, Recipe: []byte(`{"appearance":{"speed":0.5,"flipX":true}}`)})
	if err != nil {
		t.Fatal(err)
	}
	project = templateProjectForTest(t, tpl.Project)
	tracks := project.Timeline()["tracks"].([]any)
	ids := map[string]bool{}
	backgroundIndex, audioIndex := -1, -1
	for i, value := range tracks {
		track := value.(map[string]any)
		id := track["id"].(string)
		if ids[id] {
			t.Fatal("background creation duplicated a track id")
		}
		ids[id] = true
		if track["templateSlot"] == "background-video" {
			backgroundIndex = i
			if track["height"] != float64(100) || track["items"] == nil {
				t.Fatal("background track does not match engine defaults")
			}
		}
		if track["templateSlot"] == "source-audio" {
			audioIndex = i
		}
	}
	if backgroundIndex < 0 || audioIndex != backgroundIndex+1 || !ids["standard-background"] {
		t.Fatal("background is not directly above source audio or replaced an authored track")
	}
}
