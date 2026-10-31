package serverstorage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/storageimport"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestGarageMaterializeIncludesPostCutoverWritesAndRefusesOverwrite(t *testing.T) {
	garageConfig(t)
	f := newFixture(t)
	before := f.publish(t, "before cutover")
	activateS3(t, f)
	after := f.publish(t, "after cutover")
	ctx := context.Background()
	target := filepath.Join(t.TempDir(), "restored")
	preview, err := storageimport.Materialize(ctx, f.media, target, false)
	if err != nil || preview.Files != 2 || preview.Applied {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run wrote files: %v", err)
	}
	applied, err := storageimport.Materialize(ctx, f.media, target, true)
	if err != nil || applied.Files != 2 || !applied.Applied {
		t.Fatalf("materialize: %+v %v", applied, err)
	}
	for _, ref := range []struct {
		path, sum string
		size      int64
	}{{before.Path, before.SHA256, before.Bytes}, {after.Path, after.SHA256, after.Bytes}} {
		file, err := os.Open(filepath.Join(target, ref.path))
		if err != nil {
			t.Fatal(err)
		}
		sum, size, err := blobstore.DigestAll(file)
		file.Close()
		if err != nil || sum != ref.sum || size != ref.size {
			t.Fatalf("materialized bytes changed: %q %v", ref.path, err)
		}
	}
	damaged := filepath.Join(target, after.Path)
	if err := os.WriteFile(damaged, []byte("operator-owned conflicting bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storageimport.Materialize(ctx, f.media, target, true); err == nil {
		t.Fatal("conflicting destination overwritten")
	}
	got, err := os.ReadFile(damaged)
	if err != nil || string(got) != "operator-owned conflicting bytes" {
		t.Fatal("materialize overwrote an existing destination")
	}
}
