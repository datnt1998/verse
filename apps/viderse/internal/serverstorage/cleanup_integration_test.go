package serverstorage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/retention"
	"github.com/datnt1998/verse/apps/viderse/internal/serverstorage"
	"github.com/datnt1998/verse/apps/viderse/internal/storagegc"
	"github.com/datnt1998/verse/platform/blobstore"
)

func activateS3(t *testing.T, f fixture) {
	t.Helper()
	saveGarage(t, f)
	if _, err := f.manager.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitMigration(t, f, "done")
}
func TestGarageCleanupRoleReplayAndPinnedCache(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	ref := f.publish(t, "cache input")
	activateS3(t, f)
	ctx := context.Background()
	handle, err := f.media.Acquire(ctx, ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	cachePath := handle.Path
	handle.Release()
	clean := serverstorage.NewCleanup(nil)
	clean.BindManager(f.manager)
	admin := retention.WithCleanupViewer(ctx, "admin", true)
	member := retention.WithCleanupViewer(ctx, "member", false)
	preview, err := clean.PreviewCleanup(admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range preview.Items {
		if item.Kind == "orphan_objects" && (item.Count != 0 || item.Bytes != 0) {
			t.Fatalf("fresh fixture selected remote orphans: %+v", item)
		}
	}
	found := false
	for _, item := range preview.Items {
		if item.Kind == "work_cache" {
			found = true
			if item.Count != 1 || item.Bytes != ref.Bytes {
				t.Fatalf("cache preview: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("admin cache group missing")
	}
	memberPreview, err := clean.PreviewCleanup(member)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range memberPreview.Items {
		if item.Kind == "work_cache" {
			t.Fatal("member saw admin cache")
		}
		if item.Kind != "orphan_objects" {
			t.Fatalf("server scanned pipeline %q", item.Kind)
		}
	}
	if _, err := clean.RunCleanup(member, preview.ID); err == nil {
		t.Fatal("member replayed admin preview")
	} else {
		var forbidden *serverstorage.Error
		if !errors.As(err, &forbidden) || forbidden.Status != 403 {
			t.Fatalf("replay status: %v", err)
		}
	}
	pin, err := f.media.Acquire(ctx, ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Release()
	result, err := clean.RunCleanup(admin, preview.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Freed != 0 {
		t.Fatalf("cleanup freed bytes while the only cache input was pinned: %+v", result)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatal("active input evicted")
	}
	input, err := os.Open(pin.Path)
	if err != nil {
		t.Fatal(err)
	}
	sum, bytes, readErr := blobstore.DigestAll(input)
	closeErr := input.Close()
	if readErr != nil || closeErr != nil || sum != ref.SHA256 || bytes != ref.Bytes {
		t.Fatalf("pinned input unusable after cleanup: bytes=%d digest=%s read=%v close=%v", bytes, sum, readErr, closeErr)
	}
	t.Logf("held pin cleanup: freed=%d removed=%d usableBytes=%d", result.Freed, result.Removed, bytes)
	pin.Release()
	preview, err = clean.PreviewCleanup(admin)
	if err != nil {
		t.Fatal(err)
	}
	result, err = clean.RunCleanup(admin, preview.ID)
	if err != nil || result.Freed != ref.Bytes {
		t.Fatalf("cold trim: %+v %v", result, err)
	}
	if _, err := os.Stat(cachePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cold cache not removed: %v", err)
	}
	t.Logf("released pin cleanup: freed=%d removed=%d cacheDeleted=true", result.Freed, result.Removed)
}
func TestGarageGCGraceReaderPinAndRootRecheck(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	activateS3(t, f)
	ctx := context.Background()
	// A verified object whose publication never acquired a domain pointer.
	source := filepath.Join(t.TempDir(), "orphan")
	payload := []byte("gc-owned-" + time.Now().Format(time.RFC3339Nano))
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	path := "library/overlays/gc-orphan.mp4"
	if err := f.media.Publish(ctx, []media.Artifact{{Path: path, File: source, ContentType: "video/mp4", Filename: "orphan.mp4"}}, nil); err != nil {
		t.Fatal(err)
	}
	state, err := f.repo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.repo.Resolve(ctx, state.Active, path)
	if err != nil {
		t.Fatal(err)
	}
	gc := storagegc.Service{Repository: f.repo, Media: f.media, Grace: 24 * time.Hour, Now: func() time.Time { return time.Now().Add(48 * time.Hour) }}
	pin, err := f.media.Acquire(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := gc.Run(ctx, state.Active, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range preview.Objects {
		if info.Key == ref.Key {
			t.Fatal("GC selected active hydrated input")
		}
	}
	pin.Release()
	first, err := gc.Run(ctx, state.Active, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range first.Objects {
		if info.Key == ref.Key {
			t.Fatal("GC skipped fresh detach grace")
		}
	}
	second, err := gc.Run(ctx, state.Active, false)
	if err != nil {
		t.Fatal(err)
	}
	var candidate *blobstore.BlobInfo
	for i := range second.Objects {
		if second.Objects[i].Key == ref.Key {
			candidate = &second.Objects[i]
		}
	}
	if candidate == nil {
		t.Fatal("aged orphan was not collectible")
	}
	// A valid owner attached after preview invalidates deletion under the lock.
	if err := f.library.InsertOverlay(ctx, domain.Overlay{ID: "b1111111-1111-4111-a111-111111111111", SHA256: ref.SHA256, Name: "Revived", FilePath: path, Mime: "video/mp4", Bytes: ref.Bytes, Background: domain.BgBlack, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	applied, err := gc.Apply(ctx, state.Active, []blobstore.BlobInfo{*candidate})
	if err != nil || applied.Deleted != 0 {
		t.Fatalf("GC ignored new root: %+v %v", applied, err)
	}
	store, _, err := f.media.Store(ctx, state.Active)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, ref.Key); err != nil {
		t.Fatal("GC deleted newly referenced object")
	}
}

func TestGarageCleanupIsolatesOtherFixtureLiveInput(t *testing.T) {
	garageConfig(t)
	owner := newFixture(t)
	other := newFixture(t)
	ref := other.publish(t, "another fixture's live input")
	activateS3(t, owner)
	activateS3(t, other)
	ctx := context.Background()
	ownerStore, ownerDescriptor, _, err := owner.media.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	otherStore, otherDescriptor, _, err := other.media.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ownerDescriptor.Config.S3.Bucket == otherDescriptor.Config.S3.Bucket ||
		ownerDescriptor.Config.S3.Bucket == os.Getenv("S3_TEST_BUCKET") ||
		otherDescriptor.Config.S3.Bucket == os.Getenv("S3_TEST_BUCKET") {
		t.Fatal("independent repositories must not share each other's or the environment's bucket")
	}
	// Identical content exists in both buckets, but only the other repository
	// has a live domain root. Its lease cannot protect a different repository.
	if err := owner.media.Publish(ctx, []media.Artifact{{Path: ref.Path, File: filepath.Join(other.root, ref.Path), ContentType: ref.ContentType, Filename: ref.Filename}}, nil); err != nil {
		t.Fatal(err)
	}
	pin, err := other.media.Acquire(ctx, ref.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Release()
	gc := storagegc.Service{Repository: owner.repo, Media: owner.media, Grace: 24 * time.Hour, Now: func() time.Time { return time.Now().Add(48 * time.Hour) }}
	// The first visit starts this repository's detach grace. The future clock
	// then makes its own orphan eligible without aging or touching shared data.
	first, err := gc.Run(ctx, ownerDescriptor.ID, false)
	if err != nil || first.Candidates != 0 {
		t.Fatalf("fresh detach grace: %+v %v", first, err)
	}
	preview, err := gc.Run(ctx, ownerDescriptor.ID, false)
	if err != nil || preview.Candidates != 1 || preview.Bytes != ref.Bytes || preview.Objects[0].Key != ref.Key {
		t.Fatalf("isolated orphan preview: %+v %v", preview, err)
	}
	applied, err := gc.Apply(ctx, ownerDescriptor.ID, preview.Objects)
	if err != nil || applied.Deleted != 1 || applied.Bytes != ref.Bytes {
		t.Fatalf("isolated orphan cleanup: %+v %v", applied, err)
	}
	if _, err := ownerStore.Stat(ctx, ref.Key); !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatalf("owned orphan was not removed: %v", err)
	}
	if _, err := otherStore.Stat(ctx, ref.Key); err != nil {
		t.Fatalf("cleanup touched another fixture's live object: %v", err)
	}
	input, err := os.Open(pin.Path)
	if err != nil {
		t.Fatal(err)
	}
	sum, bytes, readErr := blobstore.DigestAll(input)
	closeErr := input.Close()
	if readErr != nil || closeErr != nil || sum != ref.SHA256 || bytes != ref.Bytes {
		t.Fatalf("other fixture's pinned input unusable: bytes=%d digest=%s read=%v close=%v", bytes, sum, readErr, closeErr)
	}
	t.Logf("isolated cleanup: deletedOwnBytes=%d otherRemoteIntact=true otherPinnedBytes=%d", applied.Bytes, bytes)
}
