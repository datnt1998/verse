package editor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
)

func seedLegacyReadyBinary(t *testing.T) (*editor.Service, *postgres.BlobRepository, string, string, string, editor.Resource) {
	t.Helper()
	store, repo, files, pool, root := editorPersistenceStore(t)
	blobs := postgres.NewBlobRepository(pool)
	ctx := context.Background()
	service := &editor.Service{Store: store, Repository: repo, Media: files, StagingDir: store.StagingDir}
	project, revision, err := service.CreateProject(ctx, editor.CreateProjectInput{Name: "Ready export"})
	if err != nil {
		t.Fatal(err)
	}
	pid := project["id"].(string)
	eid := editor.ID()
	path := "editor/exports/" + pid + "/" + eid + "/files/" + editor.ID() + "/ready.mp4"
	bytes := []byte("published original export bytes")
	sum := sha256.Sum256(bytes)
	digest := hex.EncodeToString(sum[:])
	file := filepath.Join(t.TempDir(), "ready.mp4")
	if err = os.WriteFile(file, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	value := editor.Object{"id": eid, "fileName": "ready.mp4", "mime": "video/mp4", "status": "ready", "size": len(bytes), "createdAt": 1000, "committedAt": 1001, "filePath": path, "stagedSha256": digest, "clone": editor.Object{"projectRevision": revision}}
	meta, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	metaFile := filepath.Join(t.TempDir(), "meta.json")
	if err = os.WriteFile(metaFile, meta, 0600); err != nil {
		t.Fatal(err)
	}
	key := "exports/" + pid + "/" + eid
	head := editor.Resource{Key: key, Path: "editor/revisions/" + editor.ID() + ".json", Revision: editor.Revision(meta)}
	if err = files.Publish(ctx, []media.Artifact{{Path: path, File: file, ContentType: "video/mp4"}, {Path: head.Path, File: metaFile, ContentType: "application/json"}}, func(tx context.Context) error { return repo.CAS(tx, key, "", head) }); err != nil {
		t.Fatal(err)
	}
	return service, blobs, root, pid, eid, head
}

func assertStrictEditorInventory(t *testing.T, ctx context.Context, root string, repo *postgres.BlobRepository) {
	t.Helper()
	entries, err := storageimport.Inventory(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := repo.Roots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = storageimport.RequireKnown(entries, roots); err != nil {
		t.Fatal(err)
	}
}

func TestEditorReadyExportBinaryRootsSurviveRestart(t *testing.T) {
	s, blobs, root, pid, eid, head := seedLegacyReadyBinary(t)
	ctx := context.Background()
	entries, err := storageimport.Inventory(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := blobs.Roots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = storageimport.RequireKnown(entries, roots); err == nil {
		t.Fatal("pre-adapter legacy export unexpectedly had durable binary ownership")
	}
	if err = s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	key := "exports/" + pid + "/" + eid + "/file"
	binary, err := s.Repository.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(binary.Path))); err != nil {
		t.Fatal("valid ready file deleted:", err)
	}
	if err = s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := s.Repository.Get(ctx, key)
	if err != nil || again.Version != binary.Version || again.Path != binary.Path {
		t.Fatalf("adoption not idempotent: %v %v", again, err)
	}
	current, err := s.Repository.Get(ctx, head.Key)
	if err != nil || current.Revision != head.Revision {
		t.Fatalf("adoption changed authoritative metadata: %v %v", current, err)
	}
	oldPath := filepath.Join(root, filepath.FromSlash(binary.Path))
	if _, err = s.ReplaceCloneExportFile(ctx, pid, eid, func(ctx context.Context, input, output string, p editor.Project, coverage editor.Object) error {
		if _, err = os.Stat(oldPath); err != nil {
			return err
		}
		active, err := s.Repository.Get(ctx, key)
		if err != nil {
			return err
		}
		if active.Path != binary.Path {
			return errors.New("old binary ownership released before replacement commit")
		}
		return os.WriteFile(output, []byte("published replacement bytes"), 0600)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("superseded local bytes not reclaimed after commit: %v", err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	h, record, err := s.AcquireExport(ctx, pid, eid)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(h.Path)
	h.Release()
	if err != nil || string(actual) != "published replacement bytes" || record.Status != "ready" {
		t.Fatalf("replacement=%q %v %v", actual, record, err)
	}
	finalHead, err := s.Repository.Get(ctx, head.Key)
	if err != nil {
		t.Fatal(err)
	}
	metadataBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(finalHead.Path)))
	if err != nil {
		t.Fatal(err)
	}
	var metadata editor.Object
	if err = json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if "sha256:"+metadata["originalSha256"].(string) != binary.Revision || metadata["originalSize"] != float64(len("published original export bytes")) {
		t.Fatalf("explicit original lineage missing: %v", metadata)
	}
}

func TestEditorBinaryAdoptionRejectsCorruptReadyBytes(t *testing.T) {
	s, _, root, pid, eid, head := seedLegacyReadyBinary(t)
	ctx := context.Background()
	h, _, err := s.AcquireExport(ctx, pid, eid)
	if err != nil {
		t.Fatal(err)
	}
	path := h.Path
	h.Release()
	if err = os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.AdoptBinaryRoots(ctx); err == nil {
		t.Fatal("corrupt export was adopted")
	}
	if _, err = s.Repository.Get(ctx, head.Key+"/file"); err == nil {
		t.Fatal("corrupt bytes received durable owner")
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatalf("verification failure deleted ready file: %v", err)
	}
	if _, err = os.Stat(root); err != nil {
		t.Fatal(err)
	}
}

func TestEditorMediaBinaryRootsSurviveRestartAndReplacement(t *testing.T) {
	store, repo, files, pool, root := editorPersistenceStore(t)
	s := &editor.Service{Store: store, Media: files, Repository: repo, StagingDir: store.StagingDir}
	blobs := postgres.NewBlobRepository(pool)
	ctx := context.Background()
	input, err := os.ReadFile("testdata/__fixtures__/image.png")
	if err != nil {
		t.Fatal(err)
	}
	record, err := s.ImportMedia(ctx, editor.ImportMediaOptions{FileName: "image.png", DeclaredMIME: "image/png", Body: bytes.NewReader(input)})
	if err != nil {
		t.Fatal(err)
	}
	binary, err := repo.Get(ctx, "media/"+record.ID+"/source")
	if err != nil || binary.Revision != record.ContentHash {
		t.Fatalf("source owner=%v %v", binary, err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	input = append(input, 0)
	replaced, err := s.ReplaceMedia(ctx, record.ID, editor.ImportMediaOptions{FileName: "image.png", DeclaredMIME: "image/png", Body: bytes.NewReader(input)})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ContentHash == record.ContentHash {
		t.Fatal("replacement hash did not change")
	}
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(binary.Path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old source not released after commit: %v", err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	if err = s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	if err = s.DeleteMedia(ctx, record.ID); err != nil {
		t.Fatal(err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
}

func TestEditorBinaryAdoptionRetiresOnlyReleasedSupersededExports(t *testing.T) {
	s, blobs, root, pid, eid, _ := seedLegacyReadyBinary(t)
	ctx := context.Background()
	stage := filepath.Join(t.TempDir(), "old.mp4")
	if err := os.WriteFile(stage, []byte("previous committed bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	old := "editor/exports/" + pid + "/" + eid + "/files/" + editor.ID() + "/ready.mp4"
	if err := s.Media.Publish(ctx, []media.Artifact{{Path: old, File: stage, ContentType: "video/mp4"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Media.Unbind(ctx, []string{old}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(old))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("released prior replacement remains: %v", err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	unknown := "editor/exports/" + pid + "/" + eid + "/files/" + editor.ID() + "/ready.mp4"
	if err := s.Media.Publish(ctx, []media.Artifact{{Path: unknown, File: stage, ContentType: "video/mp4"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(unknown))); err != nil {
		t.Fatalf("still-bound unknown publisher file was deleted: %v", err)
	}
	entries, err := storageimport.Inventory(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := blobs.Roots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = storageimport.RequireKnown(entries, roots); err == nil {
		t.Fatal("unknown bound file bypassed strict inventory")
	}
}

func seedHeld9BoundOriginal(t *testing.T, mismatch bool) (*editor.Service, *postgres.BlobRepository, string, string, string, string) {
	t.Helper()
	s, blobs, root, pid, eid, head := seedLegacyReadyBinary(t)
	ctx := context.Background()
	original := bytes.Repeat([]byte("original uploaded frame bytes"), 10)
	sum := sha256.New()
	sum.Write(original)
	state, err := sum.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(head.Path)))
	if err != nil {
		t.Fatal(err)
	}
	var meta editor.Object
	if err = json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	meta["uploadedBytes"] = len(original)
	meta["stagedHashState"] = base64.StdEncoding.EncodeToString(state)
	if mismatch {
		meta["originalSha256"] = hex.EncodeToString(sha256.New().Sum(nil))
		meta["originalSize"] = len(original)
	}
	raw, err = json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	metaFile := filepath.Join(t.TempDir(), "meta.json")
	if err = os.WriteFile(metaFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	old := "editor/exports/" + pid + "/" + eid + "/files/" + editor.ID() + "/ready.mp4"
	originalFile := filepath.Join(t.TempDir(), "original.mp4")
	if err = os.WriteFile(originalFile, original, 0600); err != nil {
		t.Fatal(err)
	}
	next := editor.Resource{Key: head.Key, Path: "editor/revisions/" + editor.ID() + ".json", Revision: editor.Revision(raw)}
	if err = s.Media.Publish(ctx, []media.Artifact{{Path: old, File: originalFile, ContentType: "video/mp4"}, {Path: next.Path, File: metaFile, ContentType: "application/json"}}, func(tx context.Context) error { return s.Repository.CAS(tx, head.Key, head.Revision, next) }); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, filepath.FromSlash(head.Path))); err != nil {
		t.Fatal(err)
	}
	return s, blobs, root, pid, eid, old
}

func TestEditorHeld9BoundOriginalRetiresOnlyWithVerifiedLineage(t *testing.T) {
	s, blobs, root, pid, eid, old := seedHeld9BoundOriginal(t, false)
	ctx := context.Background()
	if err := s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	assertStrictEditorInventory(t, ctx, root, blobs)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(old))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("proven superseded original remains: %v", err)
	}
	h, record, err := s.AcquireExport(ctx, pid, eid)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Release()
	if record.Status != "ready" {
		t.Fatal("current ready export changed")
	}
	current, err := os.ReadFile(h.Path)
	if err != nil || string(current) != "published original export bytes" {
		t.Fatalf("current replacement lost: %q %v", current, err)
	}
}

func TestEditorHeld9MismatchedOriginalLineageRemainsFailClosed(t *testing.T) {
	s, blobs, root, _, _, old := seedHeld9BoundOriginal(t, true)
	ctx := context.Background()
	if err := s.AdoptBinaryRoots(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(old))); err != nil {
		t.Fatalf("unproven file deleted: %v", err)
	}
	entries, err := storageimport.Inventory(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := blobs.Roots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = storageimport.RequireKnown(entries, roots); err == nil {
		t.Fatal("mismatched original proof bypassed inventory")
	}
}

func TestEditorHeld9CorruptCurrentNeverRetiresValidOriginal(t *testing.T) {
	s, _, root, pid, eid, old := seedHeld9BoundOriginal(t, false)
	ctx := context.Background()
	handle, _, err := s.AcquireExport(ctx, pid, eid)
	if err != nil {
		t.Fatal(err)
	}
	filePath, logical, digest := handle.Path, handle.Ref.Path, "sha256:"+handle.Ref.SHA256
	handle.Release()
	key := "exports/" + pid + "/" + eid + "/file"
	if err = s.Media.Publish(ctx, nil, func(tx context.Context) error {
		return s.Repository.CAS(tx, key, "", editor.Resource{Key: key, Path: logical, Revision: digest})
	}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filePath, []byte("corrupted replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.AdoptBinaryRoots(ctx); err == nil {
		t.Fatal("corrupt replacement let original binding retire")
	}
	if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(old))); err != nil {
		t.Fatalf("valid original lost: %v", err)
	}
	oldHandle, err := s.Media.Acquire(ctx, old)
	if err != nil {
		t.Fatalf("valid original binding retired: %v", err)
	}
	oldHandle.Release()
}
