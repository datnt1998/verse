package serverstorage_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
)

// crashState is the tree a kill -9 leaves when a build publishes media: the
// logical path is durable, but the publication transaction never committed, so
// no clone artifact and no root adapter mentions it.
func crashState(t *testing.T, root string) string {
	t.Helper()
	rel := "clone/9c2b5d30-0000-4000-a000-000000000001/6d22876900000000000000000000000000000000000000000000000000000000/editor/media/media-6d228769/metadata.json"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"id":"media-6d228769"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return rel
}

// Start-up must keep serving: an unreferenced durable file is reported to the
// owner as Cần xử lý instead of aborting the process. Migration stays strict.
func TestBootImportReportsUnreferencedFileAndStillAdoptsRoots(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	legacy := "library/overlays/legacy-boot.mp4"
	if err := os.MkdirAll(filepath.Join(f.root, "library/overlays"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(legacy)), []byte("legacy media"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.library.InsertOverlay(ctx, domain.Overlay{ID: "a2222222-2222-4222-a222-222222222222", SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("legacy media"))), Name: "Legacy boot", FilePath: legacy, Mime: "video/mp4", Bytes: int64(len("legacy media")), Background: domain.BgBlack, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	stray := crashState(t, f.root)

	unreferenced, err := f.manager.BootImport(ctx)
	if err != nil {
		t.Fatalf("boot import refused to serve: %v", err)
	}
	if len(unreferenced) != 1 || unreferenced[0] != stray {
		t.Fatalf("unreferenced files not reported: %v", unreferenced)
	}
	if _, err := f.repo.Resolve(ctx, "local", legacy); err != nil {
		t.Fatalf("known root was not adopted next to the unreferenced file: %v", err)
	}

	err = f.manager.ImportExisting(ctx)
	var unknown *storageimport.UnknownPathsError
	if !errors.As(err, &unknown) || len(unknown.Paths) != 1 || unknown.Paths[0] != stray {
		t.Fatalf("migration no longer fails closed on unreferenced bytes: %v", err)
	}
	if !strings.Contains(err.Error(), stray) {
		t.Fatalf("owner cannot see which file is unreferenced: %v", err)
	}
}

// A crash between the reference commit and the logical-path link leaves staged
// bytes; start-up completes them before judging the tree.
func TestBootImportCompletesInterruptedLocalPublication(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ref := f.publish(t, "interrupted")
	path := filepath.Join(f.root, filepath.FromSlash(ref.Path))
	if err := os.Rename(path, path+".part"); err != nil {
		t.Fatal(err)
	}
	unreferenced, err := f.manager.BootImport(ctx)
	if err != nil {
		t.Fatalf("boot import failed on an interrupted publication: %v", err)
	}
	if len(unreferenced) != 0 {
		t.Fatalf("interrupted publication reported as unreferenced: %v", unreferenced)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("committed bytes were not restored: %v", err)
	}
	if _, err := os.Stat(path + ".part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged copy left behind: %v", err)
	}
}
