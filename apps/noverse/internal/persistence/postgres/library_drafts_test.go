package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/blobstore"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storagegc"
	"github.com/datnt1998/verse/apps/noverse/internal/storageimport"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestCoCreateDraftIsolationAndGC(t *testing.T) {
	f := provisionLibrary(t, "draft-project")
	ctx := context.Background()
	id := "20261031-120000-abcdef"
	if dir, err := f.repo.CreateDraft(ctx, f.owner, "draft-project", id); err != nil || dir != "" {
		t.Fatalf("create draft without directory: %q %v", dir, err)
	}
	nar := postgres.NewNarrative(f.pool)
	scope, err := nar.ResolveDraft(ctx, "draft-project", id, &postgres.Actor{Username: f.owner.Username}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nar.ResolveDraft(ctx, "draft-project", id, &postgres.Actor{Username: f.outsider.Username}, false); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("outsider draft scope: %v", err)
	}
	if _, err := nar.Resolve(ctx, "draft-project", "draft-"+id, nil, false); !errors.Is(err, errs.ErrStorageNotFound) {
		t.Fatalf("public/internal story resolver exposed draft: %v", err)
	}
	lease, err := postgres.NewStoryLease(ctx, nar, scope, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Close() })
	if other, err := postgres.NewStoryLease(ctx, nar, scope, time.Minute); !errors.Is(err, postgres.ErrLeaseLost) {
		if other != nil {
			_ = other.Close()
		}
		t.Fatalf("second draft worker: %v", err)
	}
	if _, _, err := f.repo.CommitDraft(ctx, f.owner, "draft-project", id, ""); !errors.Is(err, errs.ErrStorageConflict) {
		t.Fatalf("promote a leased session: %v", err)
	}
	if n, err := f.repo.CleanupDrafts(ctx, nil, -time.Hour); err != nil || n != 0 {
		t.Fatalf("cleanup removed live lease: %d %v", n, err)
	}
	st, err := store.NewFromBackend(postgres.NewBackend(ctx, f.pool, nar, scope, postgres.WithLeaseFence(lease)))
	if err != nil {
		t.Fatal(err)
	}
	if err = st.Init(); err != nil {
		t.Fatal(err)
	}
	if err = st.Docs().WriteJSON(".ainovel/cocreate/session.json", map[string]any{"schemaVersion": 1, "draft": "ý tưởng bí mật"}); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	// Reclaiming a session before promotion keeps the original facts and FK.
	lease, err = postgres.NewStoryLease(ctx, nar, scope, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	st, err = store.NewFromBackend(postgres.NewBackend(ctx, f.pool, nar, scope, postgres.WithLeaseFence(lease)))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err = st.Docs().ReadJSON(".ainovel/cocreate/session.json", &saved); err != nil || saved["draft"] != "ý tưởng bí mật" {
		t.Fatalf("restore: %v %v", saved, err)
	}
	blobs, err := blobstore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blobs.Close() })
	body := "draft attachment"
	sum := sha256.Sum256([]byte(body))
	sha := hex.EncodeToString(sum[:])
	keys := blobkey.Keys{}
	key, err := keys.Cover(postgres.UUIDString(scope.ProjectID), postgres.UUIDString(scope.StoryID), sha, "png")
	if err != nil {
		t.Fatal(err)
	}
	audio := postgres.NewAudio(f.pool)
	if _, err = blobs.Put(ctx, key, strings.NewReader(body), blobstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	blob, err := audio.PublishBlob(ctx, postgres.PublishInput{ProjectID: scope.ProjectID, Key: key, Driver: "local", ContentType: "image/png", SHA256: sha, Size: int64(len(body))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Pool().Exec(ctx, `INSERT INTO story_artifacts(project_id,story_id,kind,artifact_key,blob_id) VALUES($1,$2,'diagnostic','draft-attachment',$3)`, scope.ProjectID, scope.StoryID, blob.ID); err != nil {
		t.Fatal(err)
	}
	// Draft stories, documents and story-owned bytes must not leak into rollback exports.
	exported, err := storageimport.ExportFiles(ctx, storageimport.ExportOptions{SourceID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Destination: t.TempDir(), Pool: f.pool, Blobs: blobs})
	if err != nil {
		t.Fatal(err)
	}
	if exported.Stories != 0 || exported.BlobsWritten != 0 {
		t.Fatalf("exported hidden draft: %+v", exported)
	}
	future := time.Now().Add(48 * time.Hour)
	opts := storagegc.Options{Pool: f.pool, Blobs: blobs, Keys: keys, Now: func() time.Time { return future }}
	if report, err := storagegc.Run(ctx, opts); err != nil || report.BlobsReaped != 0 {
		t.Fatalf("GC removed referenced draft blob: %+v %v", report, err)
	}
	if err = f.repo.RemoveDraft(ctx, f.owner, "draft-project", id); err != nil {
		t.Fatal(err)
	}
	// A writer already in flight has lost its fence, not a valid new empty session.
	if err = st.Docs().WriteJSON(".ainovel/cocreate/session.json", saved); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("abandoned worker still writes: %v", err)
	}
	if _, err = postgres.NewStoryLease(ctx, nar, scope, time.Minute); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("removed scope reclaimed: %v", err)
	}
	var facts int
	if err = f.pool.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM stories)+(SELECT count(*) FROM story_state)+(SELECT count(*) FROM story_artifacts)+(SELECT count(*) FROM story_events)+(SELECT count(*) FROM story_leases)`).Scan(&facts); err != nil || facts != 0 {
		t.Fatalf("abandoned facts: %d %v", facts, err)
	}
	if report, err := storagegc.Run(ctx, opts); err != nil || report.BlobsReaped != 1 || report.ObjectsDeleted != 1 {
		t.Fatalf("GC did not reclaim draft blob: %+v %v", report, err)
	}
	if _, err = blobs.Stat(ctx, key); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("draft bytes survived GC: %v", err)
	}
	// Age-based abandonment follows the same path, but does not touch a promoted story.
	if _, err = f.repo.CreateDraft(ctx, f.owner, "draft-project", "old-draft"); err != nil {
		t.Fatal(err)
	}
	if n, err := f.repo.CleanupDrafts(ctx, nil, -time.Hour); err != nil || n != 1 {
		t.Fatalf("cleanup old draft: %d %v", n, err)
	}
	if _, err = f.repo.CreateDraft(ctx, f.owner, "draft-project", "commit-draft"); err != nil {
		t.Fatal(err)
	}
	commitScope, err := nar.ResolveDraft(ctx, "draft-project", "commit-draft", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := f.repo.CommitDraft(ctx, f.owner, "draft-project", "commit-draft", "Tên truyện")
	if err != nil || story.Story != "01-ten-truyen" {
		t.Fatalf("promotion: %+v %v", story, err)
	}
	if _, err = postgres.NewStoryLease(ctx, nar, commitScope, time.Minute); !errors.Is(err, postgres.ErrLeaseLost) {
		t.Fatalf("pre-promotion draft scope reclaimed: %v", err)
	}
	if n, err := f.repo.CleanupDrafts(ctx, nil, -time.Hour); err != nil || n != 0 {
		t.Fatalf("cleanup promoted story: %d %v", n, err)
	}
}
