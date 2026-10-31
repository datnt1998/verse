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
	result, err := clean.RunCleanup(admin, preview.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Freed != 0 {
		t.Fatal("clean reported pinned cache removed")
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatal("active input evicted")
	}
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
