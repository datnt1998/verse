package editor_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/platform/blobstore"
)

func workspaceService(t *testing.T) *editor.Service {
	t.Helper()
	store, repo, files, _, _ := editorPersistenceStore(t)
	return &editor.Service{Store: store, Media: files, Repository: repo, StagingDir: store.StagingDir, Now: func() time.Time { return time.UnixMilli(3000) }}
}

// Ported from yt-factory@2c201e6 projects.test.ts and projects.cli.test.ts:
// create/list/load/save/rename/duplicate/delete preserve wire shapes and identity.
func TestWorkspaceProjectLifecycle(t *testing.T) {
	s := workspaceService(t)
	ctx := context.Background()
	p, revision, err := s.CreateProject(ctx, editor.CreateProjectInput{Name: "Demo", Template: "youtube-1080p"})
	if err != nil {
		t.Fatal(err)
	}
	id := p["id"].(string)
	if p.Metadata()["width"] != 1920 || p.Metadata()["height"] != 1080 || p.Metadata()["fps"] != float64(30) {
		t.Fatalf("metadata=%v", p.Metadata())
	}
	list, err := s.ListProjects(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Demo" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	loaded, got, err := s.LoadProject(ctx, id)
	if err != nil || got != revision {
		t.Fatalf("load %s %v", got, err)
	}
	loaded["description"] = "edited"
	next, err := s.SaveProject(ctx, id, loaded, got)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProject(ctx, id, loaded, got); !errors.Is(err, editor.ErrConflict) {
		t.Fatalf("stale save=%v", err)
	}
	next, err = s.RenameProject(ctx, id, "Renamed", next)
	if err != nil {
		t.Fatal(err)
	}
	copy, _, err := s.DuplicateProject(ctx, id, "Copy")
	if err != nil {
		t.Fatal(err)
	}
	if copy["id"] == id || copy["name"] != "Copy" {
		t.Fatalf("copy=%v", copy)
	}
	list, err = s.ListProjects(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("duplicates=%v %v", list, err)
	}
	if err = s.DeleteProject(ctx, copy["id"].(string)); err != nil {
		t.Fatal(err)
	}
	list, err = s.ListProjects(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("after delete=%v %v", list, err)
	}
	if _, _, err = s.LoadProject(ctx, copy["id"].(string)); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("deleted read=%v", err)
	}
	if _, _, err = s.CreateProject(ctx, editor.CreateProjectInput{Name: "bad", Template: "nope"}); err == nil {
		t.Fatal("unknown preset accepted")
	}
}

// Ported from projects.test.ts: missing sidecars/presets use revision empty;
// subsequent writes enforce If-Match and duplication copies state, not exports.
func TestWorkspaceSidecarAndPresetRevisions(t *testing.T) {
	s := workspaceService(t)
	ctx := context.Background()
	p, _, err := s.CreateProject(ctx, editor.CreateProjectInput{Name: "Sidecars"})
	if err != nil {
		t.Fatal(err)
	}
	id := p["id"].(string)
	data, revision, err := s.LoadSidecar(ctx, id, "animation-presets")
	if err != nil || revision != "empty" || len(data) != 0 {
		t.Fatalf("missing=%v %s %v", data, revision, err)
	}
	revision, err = s.SaveSidecar(ctx, id, "animation-presets", editor.Object{"custom": true}, revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSidecar(ctx, id, "animation-presets", editor.Object{}, "empty"); !errors.Is(err, editor.ErrConflict) {
		t.Fatalf("stale sidecar=%v", err)
	}
	data, got, err := s.LoadSidecar(ctx, id, "animation-presets")
	if err != nil || got != revision || data["custom"] != true {
		t.Fatalf("sidecar=%v %s %v", data, got, err)
	}
	duplicate, _, err := s.DuplicateProject(ctx, id, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _, err = s.LoadSidecar(ctx, duplicate["id"].(string), "animation-presets")
	if err != nil || data["custom"] != true {
		t.Fatalf("copied sidecar=%v %v", data, err)
	}
	if _, _, err = s.LoadSidecar(ctx, id, "../../../escape"); err == nil {
		t.Fatal("traversal sidecar accepted")
	}
	_, revision, err = s.LoadEffectPresets(ctx)
	if err != nil || revision != "empty" {
		t.Fatalf("initial presets=%s %v", revision, err)
	}
	revision, err = s.SaveEffectPresets(ctx, editor.Object{"presets": []any{}}, revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveEffectPresets(ctx, editor.Object{}, "empty"); !errors.Is(err, editor.ErrConflict) {
		t.Fatalf("stale preset=%v", err)
	}
	_, got, err = s.LoadEffectPresets(ctx)
	if err != nil || got != revision {
		t.Fatalf("presets=%s %v", got, err)
	}
	first, err := s.WorkspaceID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.WorkspaceID(ctx)
	if err != nil || first != second || first == "" {
		t.Fatalf("workspace %q %q %v", first, second, err)
	}
}

// Portable single-component IDs and upload file names retain the upstream
// traversal guard; no filesystem-derived path escapes reach blob publication.
func TestWorkspacePortableInputs(t *testing.T) {
	for _, id := range []string{"../escape", "..\\escape", "%2e%2e", "/absolute", "a\x00b", "-leading", "_leading", ""} {
		if err := editor.ValidateID(id); err == nil {
			t.Errorf("accepted id %q", id)
		}
	}
	for _, name := range []string{"../name.mp4", "a\\name.mp4", "name?.mp4", "", ".."} {
		got := editor.SanitizeFileName(name)
		if got == "" || got == ".." {
			t.Errorf("unsafe filename %q -> %q", name, got)
		}
	}
}

// Ported from projects.test.ts: source replacement refreshes root/nested media
// URLs and invalidates derived caches without changing the saved revision.
func TestWorkspaceLoadRefreshesNestedSourcesAndWaveform(t *testing.T) {
	s := workspaceService(t)
	ctx := context.Background()
	p, revision, err := s.CreateProject(ctx, editor.CreateProjectInput{Name: "Source refresh"})
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile("testdata/__fixtures__/image.png")
	if err != nil {
		t.Fatal(err)
	}
	record, err := s.ImportMedia(ctx, editor.ImportMediaOptions{FileName: "image.png", DeclaredMIME: "image/png", Body: bytesReader(bytes)})
	if err != nil {
		t.Fatal(err)
	}
	item := editor.Object{"id": "root", "type": "image", "mediaId": record.ID, "src": "stale", "thumbnailUrl": "blob:stale", "reverseConformSrc": "stale"}
	nested := editor.Object{"id": "nested", "type": "image", "mediaId": record.ID, "src": "stale"}
	wave := editor.Object{"id": "wave", "type": "shape", "mediaId": record.ID, "waveformVisualizer": editor.Object{"sourceHash": "old", "envelopeBase64": "stale", "durationSeconds": 2, "normalization": editor.Object{"version": 1}}}
	p["timeline"] = editor.Object{"items": []any{item, wave}, "compositions": []any{editor.Object{"items": []any{nested}}}}
	revision, err = s.SaveProject(ctx, p["id"].(string), p, revision)
	if err != nil {
		t.Fatal(err)
	}
	loaded, got, err := s.LoadProject(ctx, p["id"].(string))
	if err != nil || got != revision {
		t.Fatalf("read rev=%s err=%v", got, err)
	}
	items := loaded.Timeline()["items"].([]any)
	root := items[0].(map[string]any)
	if !strings.Contains(root["src"].(string), record.ID+"?v=sha256%3A") || root["thumbnailUrl"] != nil || root["reverseConformSrc"] != nil || root["sourceWidth"] == nil {
		t.Fatalf("root refresh=%v", root)
	}
	payload := items[1].(map[string]any)["waveformVisualizer"].(map[string]any)
	if payload["envelopeBase64"] != nil || payload["durationSeconds"] != nil || payload["normalization"] != nil {
		t.Fatalf("stale waveform retained=%v", payload)
	}
	comp := loaded.Timeline()["compositions"].([]any)[0].(map[string]any)
	child := comp["items"].([]any)[0].(map[string]any)
	if child["src"] != root["src"] {
		t.Fatalf("nested refresh=%v", child)
	}
}
func bytesReader(data []byte) *bytes.Reader { return bytes.NewReader(data) }
