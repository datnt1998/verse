package editor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/platform/blobstore"
)

// A workspace save, not startup adoption or fixture-seeded editor heads, must
// retain the resources it consumes from the current clone build.
func TestSaveProjectPinsCurrentCloneMedia(t *testing.T) {
	for _, test := range []struct {
		name     string
		adopted  bool
		commands bool
	}{
		{name: "without-source-head"},
		{name: "with-adopted-source-head", adopted: true},
		{name: "command-without-autosave", commands: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, repo, files, pool, _ := editorPersistenceStore(t)
			ctx := context.Background()
			s := &editor.Service{Store: store, Repository: repo, Media: files, StagingDir: store.StagingDir, MediaReferences: repo.MediaReferences, Now: func() time.Time { return time.Unix(1800000000, 0) }}
			channel := "00000000-0000-4000-8000-0000000000d1"
			job := "00000000-0000-4000-8000-0000000000d2"
			pid := "saved-clone"
			seedEditorCloneChannel(t, pool, channel)
			if _, err := pool.Exec(ctx, `INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,generation) VALUES($1,$2,'saved-clone','{}',1)`, job, channel); err != nil {
				t.Fatal(err)
			}
			source := []byte("generated image bytes")
			digest := editor.Revision(source)
			mid := "media-" + strings.TrimPrefix(digest, "sha256:")[:32]
			manifest := []editor.Object{}
			publish := func(logical string, body []byte, mime string) string {
				t.Helper()
				sum := strings.TrimPrefix(editor.Revision(body), "sha256:")
				path := "clone/" + job + "/" + sum + "/" + logical
				file := filepath.Join(t.TempDir(), "artifact")
				if err := os.WriteFile(file, body, 0600); err != nil {
					t.Fatal(err)
				}
				if err := files.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: mime}}, nil); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, `INSERT INTO clone_artifacts(job_id,id,step,path,sha256,bytes) VALUES($1,$2,'build',$3,$4,$5)`, job, logical, path, sum, len(body)); err != nil {
					t.Fatal(err)
				}
				manifest = append(manifest, editor.Object{"path": path, "sha256": sum, "step": "build"})
				return path
			}
			marshal := func(value any) []byte {
				t.Helper()
				body, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				return body
			}
			metadata := marshal(editor.Object{
				"id": mid, "fileName": "scene.webp", "sourceFile": "scene.webp", "mime": "image/webp", "size": len(source), "contentHash": digest,
				"createdAt": 1000, "updatedAt": 1100, "lastUsedAt": 1200, "storageType": "workspace", "tags": []string{"clone-scene"}, "overlay": false,
				"attribution":      editor.Object{"provider": "generator", "author": "artist"},
				"background":       editor.Object{"kind": "transparent", "version": 1, "contentHash": digest},
				"probe":            editor.Object{"kind": "image", "mime": "image/webp", "width": 1280, "height": 720},
				"generatorDetails": editor.Object{"seed": 42},
			})
			metadataPath := publish("editor/media/"+mid+"/metadata.json", metadata, "application/json")
			sourcePath := publish("editor/media/"+mid+"/scene.webp", source, "image/webp")
			publish("editor/projects/"+pid+"/media-links.json", marshal(editor.Object{"version": "1.0", "mediaIds": []editor.Object{{"id": mid, "addedAt": 1000}}}), "application/json")
			project := persistenceProject(t, pid)
			keyframes := []any{editor.Object{"itemId": "scene", "animationVersion": float64(2), "properties": []any{editor.Object{"property": "opacity", "keyframes": []any{
				editor.Object{"id": "fade-in", "frame": float64(0), "value": 0.25, "easing": "linear"},
				editor.Object{"id": "fade-out", "frame": float64(89), "value": float64(1), "easing": "linear"},
			}}}}}
			project["timeline"] = editor.Object{
				"tracks": []any{editor.Object{"id": "scenes", "kind": "video"}},
				"items": []any{editor.Object{
					"id": "scene", "type": "image", "trackId": "scenes", "label": "Generated scene",
					"mediaId": mid, "from": 0, "durationInFrames": 90,
				}},
			}
			project.Timeline()["keyframes"] = keyframes
			publish("editor/projects/"+pid+"/project.json", marshal(project), "application/json")
			build := marshal(editor.Object{"data": editor.Object{"projectId": pid}, "artifacts": manifest})
			if _, err := pool.Exec(ctx, `INSERT INTO clone_steps(job_id,step,status,output_manifest) VALUES($1,'build','succeeded',$2)`, job, build); err != nil {
				t.Fatal(err)
			}
			if test.adopted {
				problems, err := s.AdoptBinaryRoots(ctx)
				if err != nil || len(problems) != 0 {
					t.Fatalf("adopt sources: %v %v", problems, err)
				}
			}
			head, err := repo.Get(ctx, "media/"+mid)
			if err != nil || head.Version != 0 || head.Path != metadataPath {
				t.Fatalf("fixture must expose synthetic metadata: %+v %v", head, err)
			}
			before, err := s.GetMediaMetadata(ctx, mid)
			if err != nil {
				t.Fatal(err)
			}
			loaded, revision, err := s.LoadProject(ctx, pid)
			if err != nil {
				t.Fatal(err)
			}
			item := loaded.Timeline()["items"].([]any)[0].(map[string]any)
			transform := editor.Object{"x": float64(123), "y": float64(45), "scale": 1.25, "rotation": float64(12)}
			item["transform"] = transform
			var savedRevision string
			if test.commands {
				result, commandErr := s.RunCommands(ctx, pid, []editor.Command{{Type: "item.update", Params: editor.Object{"id": "scene", "updates": editor.Object{"transform": transform}}}}, editor.ApplyCommandsOptions{BaseRevision: revision})
				err = commandErr
				savedRevision, _ = result["revision"].(string)
			} else {
				savedRevision, err = s.SaveProject(ctx, pid, loaded, revision)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"media/" + mid, "media/" + mid + "/source", "projects/" + pid + "/links"} {
				head, err := repo.Get(ctx, key)
				if err != nil || head.Version == 0 {
					t.Fatalf("save did not promote %s: %+v %v", key, head, err)
				}
				var roots int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM storage_live_refs WHERE logical_path=$1`, head.Path).Scan(&roots); err != nil || roots == 0 {
					t.Fatalf("%s is not a durable root: %d %v", key, roots, err)
				}
			}
			// Redraw invalidates the build before the replacement manifest exists.
			// The subsequent build no longer advertises any old editor alias.
			for _, status := range []string{"stale", "succeeded"} {
				if _, err := pool.Exec(ctx, `UPDATE clone_steps SET status=$2,output_manifest=$3,updated_at=now() WHERE job_id=$1 AND step='build'`, job, status, marshal(editor.Object{"data": editor.Object{"projectId": pid}, "artifacts": []any{}})); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.ResolvePath(ctx, "editor/media/"+mid+"/metadata.json"); !errors.Is(err, blobstore.ErrNotFound) {
					t.Fatalf("old metadata alias survived %s: %v", status, err)
				}
				reclaimed, err := s.ReclaimUnreferencedMedia(ctx, false)
				if err != nil {
					t.Fatalf("collect after %s: %v", status, err)
				}
				for _, id := range reclaimed["mediaIds"].([]string) {
					if id == mid {
						t.Fatalf("saved generated media reclaimed after %s", status)
					}
				}
				after, err := s.GetMediaMetadata(ctx, mid)
				if err != nil || !reflect.DeepEqual(after, before) {
					t.Fatalf("metadata changed after %s: %+v %v", status, after, err)
				}
				handle, err := s.AcquireMedia(ctx, mid)
				if err != nil {
					t.Fatalf("source unavailable after %s: %v", status, err)
				}
				actual, err := os.ReadFile(handle.Path)
				handle.Release()
				if err != nil || !bytes.Equal(actual, source) {
					t.Fatalf("source changed after %s: %q %v", status, actual, err)
				}
				current, currentRevision, err := s.LoadProject(ctx, pid)
				if err != nil || currentRevision != savedRevision {
					t.Fatalf("saved project unavailable after %s: %s %v", status, currentRevision, err)
				}
				got := current.Timeline()["items"].([]any)[0].(map[string]any)
				if !reflect.DeepEqual(current.Timeline()["keyframes"], keyframes) || !reflect.DeepEqual(got["transform"], transform) || got["durationInFrames"] != float64(90) || got["mediaId"] != mid {
					t.Fatalf("editorial state changed: %+v", got)
				}
				linked, err := s.ListMedia(ctx, pid)
				if err != nil || len(linked) != 1 || linked[0].ID != mid {
					t.Fatalf("links unavailable after %s: %+v %v", status, linked, err)
				}
			}
			// Pinning preserves even storage-only and unknown metadata fields.
			head, err = repo.Get(ctx, "media/"+mid)
			if err != nil || head.Path != metadataPath || head.Revision != editor.Revision(metadata) {
				t.Fatalf("metadata was reserialized: %+v %v", head, err)
			}
			sourceHead, err := repo.Get(ctx, "media/"+mid+"/source")
			if err != nil || sourceHead.Path != sourcePath || sourceHead.Revision != digest {
				t.Fatalf("wrong source binding: %+v %v", sourceHead, err)
			}
			// Ownership belongs to saved documents, not promoted media heads.
			// A second project keeps the media alive after the first is removed.
			shared := persistenceProject(t, "shared-owner")
			shared["timeline"] = editor.Object{"items": []any{editor.Object{"type": "image", "mediaId": mid}}}
			sharedRevision, err := store.Create(ctx, shared)
			if err != nil {
				t.Fatal(err)
			}
			remove := func(keys ...string) {
				t.Helper()
				if err := files.Publish(ctx, nil, func(tx context.Context) error {
					for _, key := range keys {
						resource, err := repo.Get(tx, key)
						if err != nil {
							return err
						}
						if err := repo.Delete(tx, key, resource.Revision); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			remove("projects/"+pid+"/links", "projects/"+pid)
			if _, err := s.ReclaimUnreferencedMedia(ctx, false); err != nil {
				t.Fatal(err)
			}
			if _, err := s.GetMediaMetadata(ctx, mid); err != nil {
				t.Fatalf("shared project lost media: %v", err)
			}
			if _, revision, err := store.Read(ctx, "shared-owner"); err != nil || revision != sharedRevision {
				t.Fatalf("shared owner unavailable: %s %v", revision, err)
			}
			remove("projects/shared-owner")
			reclaimed, err := s.ReclaimUnreferencedMedia(ctx, false)
			if err != nil || !reflect.DeepEqual(reclaimed["mediaIds"], []string{mid}) {
				t.Fatalf("unowned promoted head became immortal: %+v %v", reclaimed, err)
			}
			for _, key := range []string{"media/" + mid, "media/" + mid + "/source"} {
				if _, err := repo.Get(ctx, key); !errors.Is(err, blobstore.ErrNotFound) {
					t.Fatalf("unowned resource retained: %s %v", key, err)
				}
			}
		})
	}
}

type savePinPublication struct {
	*media.Service
	before func() error
}

func (p *savePinPublication) Publish(ctx context.Context, artifacts []media.Artifact, commit func(context.Context) error) error {
	if p.before != nil {
		before := p.before
		p.before = nil
		if err := before(); err != nil {
			return err
		}
	}
	return p.Service.Publish(ctx, artifacts, commit)
}

func TestSaveProjectPinRespectsConcurrentTombstone(t *testing.T) {
	for _, deleted := range []string{"metadata", "source"} {
		t.Run(deleted, func(t *testing.T) {
			store, repo, files, _, _ := editorPersistenceStore(t)
			ctx := context.Background()
			pid, mid := "pin-conflict", "generated-media"
			project := persistenceProject(t, pid)
			project["timeline"] = editor.Object{"items": []any{editor.Object{"type": "image", "mediaId": mid}}}
			revision, err := store.Create(ctx, project)
			if err != nil {
				t.Fatal(err)
			}
			source := []byte("generated")
			metadata, err := json.Marshal(editor.Object{"id": mid, "mime": "image/webp", "fileName": "scene.webp", "sourceFile": "scene.webp", "size": len(source), "contentHash": editor.Revision(source)})
			if err != nil {
				t.Fatal(err)
			}
			artifacts := []media.Artifact{}
			for name, body := range map[string][]byte{"metadata.json": metadata, "scene.webp": source} {
				file := filepath.Join(t.TempDir(), name)
				if err := os.WriteFile(file, body, 0600); err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, media.Artifact{Path: "editor/media/" + mid + "/" + name, File: file})
			}
			if err := files.Publish(ctx, artifacts, nil); err != nil {
				t.Fatal(err)
			}
			publisher := &savePinPublication{Service: files, before: func() error {
				return files.Publish(ctx, nil, func(tx context.Context) error {
					if deleted == "source" {
						return repo.Delete(tx, "media/"+mid+"/source", "")
					}
					return repo.Delete(tx, "media/"+mid, editor.Revision(metadata))
				})
			}}
			store.Media = publisher
			s := &editor.Service{Store: store, Repository: repo, Media: publisher}
			if _, err := s.SaveProject(ctx, pid, project, revision); !errors.Is(err, editor.ErrConflict) {
				t.Fatalf("save revived %s tombstone: %v", deleted, err)
			}
			_, currentRevision, err := store.Read(ctx, pid)
			if err != nil || currentRevision != revision {
				t.Fatalf("failed save changed project: %s %v", currentRevision, err)
			}
			key := "media/" + mid
			if deleted == "source" {
				key += "/source"
			}
			if _, err := repo.Get(ctx, key); !errors.Is(err, blobstore.ErrNotFound) {
				t.Fatalf("tombstone lost: %s %v", key, err)
			}
			if deleted == "source" {
				head, err := repo.Get(ctx, "media/"+mid)
				if err != nil || head.Version != 0 {
					t.Fatalf("failed save leaked metadata promotion: %+v %v", head, err)
				}
			}
		})
	}
}

func TestSaveProjectPinMetadataOnlyAndMissingDeclaredSource(t *testing.T) {
	for _, sourceFile := range []string{"", "missing.webp"} {
		name := "metadata-only"
		if sourceFile != "" {
			name = "missing-declared-source"
		}
		t.Run(name, func(t *testing.T) {
			store, repo, files, _, _ := editorPersistenceStore(t)
			ctx := context.Background()
			pid, mid := "source-contract", "library-media"
			project := persistenceProject(t, pid)
			project["timeline"] = editor.Object{"items": []any{editor.Object{"type": "image", "mediaId": mid}}}
			revision, err := store.Create(ctx, project)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := json.Marshal(editor.Object{
				"id": mid, "mime": "image/webp", "fileName": "scene.webp",
				"sourceFile": sourceFile, "size": 9, "contentHash": editor.Revision([]byte("generated")),
			})
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "metadata.json")
			if err := os.WriteFile(file, metadata, 0600); err != nil {
				t.Fatal(err)
			}
			if err := files.Publish(ctx, []media.Artifact{{Path: "editor/media/" + mid + "/metadata.json", File: file, ContentType: "application/json"}}, nil); err != nil {
				t.Fatal(err)
			}
			s := &editor.Service{Store: store, Repository: repo, Media: files}
			_, saveErr := s.SaveProject(ctx, pid, project, revision)
			head, err := repo.Get(ctx, "media/"+mid)
			if err != nil {
				t.Fatal(err)
			}
			if sourceFile == "" {
				if saveErr != nil || head.Version == 0 {
					t.Fatalf("metadata-only save must retain metadata: %+v %v", head, saveErr)
				}
			} else {
				if !errors.Is(saveErr, blobstore.ErrNotFound) || head.Version != 0 {
					t.Fatalf("missing declared source must abort retention: %+v %v", head, saveErr)
				}
				_, current, err := store.Read(ctx, pid)
				if err != nil || current != revision {
					t.Fatalf("failed save changed project: %s %v", current, err)
				}
			}
			if _, err := repo.Get(ctx, "media/"+mid+"/source"); !errors.Is(err, blobstore.ErrNotFound) {
				t.Fatalf("save invented a source head: %v", err)
			}
		})
	}
}
