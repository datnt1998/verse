package serverstorage_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestLegacyImportRejectsPipelineBeforeWritingAnyManifest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	rel := "library/overlays/legacy.mp4"
	payload := []byte("legacy media")
	if err := os.MkdirAll(filepath.Join(f.root, "library/overlays"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, rel), payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.library.InsertOverlay(ctx, domain.Overlay{ID: "a1111111-1111-4111-a111-111111111111", SHA256: fmt.Sprintf("%x", sha256.Sum256(payload)), Name: "Legacy", FilePath: rel, Mime: "video/mp4", Bytes: int64(len(payload)), Background: domain.BgBlack, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"editor/projects/one/project.json", "sources/video/source.mp4"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(f.root, path)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.root, path), []byte("durable"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := f.manager.ImportExisting(ctx)
	if err == nil || !strings.Contains(err.Error(), "editor/projects/one/project.json") || !strings.Contains(err.Error(), "sources/video/source.mp4") {
		t.Fatalf("unknown pipeline did not fail closed with paths: %v", err)
	}
	if _, err := f.repo.Resolve(ctx, "local", rel); !blobstore.IsNotFound(err) {
		t.Fatalf("import wrote before validating complete inventory: %v", err)
	}
}
