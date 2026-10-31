package editor_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
)

func TestEditorDuplicateDeleteRestartRetiresRevision(t *testing.T) {
	store, repo, files, pool, root := editorPersistenceStore(t)
	ctx := context.Background()
	s := &editor.Service{Store: store, Repository: repo, Media: files, StagingDir: store.StagingDir}
	original, _, err := s.CreateProject(ctx, editor.CreateProjectInput{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	copy, _, err := s.DuplicateProject(ctx, original["id"].(string), "Conflict copy")
	if err != nil {
		t.Fatal(err)
	}
	id := copy["id"].(string)
	head, err := repo.Get(ctx, "projects/"+id)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteProject(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err = s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	assertStrictEditorInventory(t, ctx, root, postgres.NewBlobRepository(pool))
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(head.Path))); !os.IsNotExist(err) {
		t.Fatalf("deleted revision retained: %v", err)
	}
	if _, _, err = store.Read(ctx, original["id"].(string)); err != nil {
		t.Fatal("original was damaged:", err)
	}
}

func TestEditorDeletedRevisionStartupRequiresTombstoneAndMatchingID(t *testing.T) {
	for _, mode := range []string{"deleted", "unknown", "wrong-id", "still-bound"} {
		t.Run(mode, func(t *testing.T) {
			store, repo, files, pool, root := editorPersistenceStore(t)
			ctx := context.Background()
			s := &editor.Service{Store: store, Repository: repo, Media: files, StagingDir: store.StagingDir}
			project, _, err := s.CreateProject(ctx, editor.CreateProjectInput{Name: "Historical copy"})
			if err != nil {
				t.Fatal(err)
			}
			id := project["id"].(string)
			head, err := repo.Get(ctx, "projects/"+id)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(head.Path))
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unknown" {
				// An orphan without a deletion tombstone is not eligible, even if its
				// namespace and project payload look exactly like generated revisions.
				id = editor.ID()
				project["id"] = id
				data, err = editor.Marshal(project)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(root, "editor", "projects", id, "revisions", filepath.Base(path))
				if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err = s.DeleteProject(ctx, id); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "wrong-id" {
				project["id"] = editor.ID()
				data, err = editor.Marshal(project)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "still-bound" {
				// A new publisher can still own the same logical path after deletion.
				if err = files.Publish(ctx, []media.Artifact{{Path: head.Path, File: path, ContentType: "application/json"}}, nil); err != nil {
					t.Fatal(err)
				}
			}
			if err = s.AdoptBinaryRoots(ctx); err != nil {
				t.Fatal(err)
			}
			if mode == "deleted" {
				if _, err = os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("proven released revision retained: %v", err)
				}
				assertStrictEditorInventory(t, ctx, root, postgres.NewBlobRepository(pool))
				if err = s.AdoptBinaryRoots(ctx); err != nil {
					t.Fatal("repeat cleanup:", err)
				}
				return
			}
			if _, err = os.Stat(path); err != nil {
				t.Fatal("unproven revision removed:", err)
			}
			entries, err := storageimport.Inventory(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			roots, err := postgres.NewBlobRepository(pool).Roots(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = storageimport.RequireKnown(entries, roots); err == nil {
				t.Fatal("unproven revision bypassed strict inventory")
			}
		})
	}
}
