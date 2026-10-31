package storageimport

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"os"
	"path/filepath"
	"testing"
)

func TestInventoryRecognizesPublishedCloneAndBundledTemplateRoots(t *testing.T) {
	dir := t.TempDir()
	paths := []string{"clone/job/g1/download/source.mp4", "library/standard-template/scene-cool.webp"}
	roots := make([]media.Root, 0, len(paths))
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte("durable publication"), 0600); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, media.Root{Path: p})
	}
	entries, err := Inventory(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = RequireKnown(entries, roots); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Group != "clone" || entries[1].Group != "library" {
		t.Fatalf("wrong owner groups: %+v", entries)
	}
}
