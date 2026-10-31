package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/platform/blobstore"
)

func TestCloneCutoutRetiredAfterLastJob(t *testing.T) {
	ctx := context.Background()
	pool := provision(t)
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels(id,name,language,auto_pick) VALUES('11111111-1111-4111-8111-111111111111','Clone fixture','en',false)`); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewCloneStore(pool)
	a := cloneTestJob(t, store, "cutout-a")
	b := cloneTestJob(t, store, "cutout-b")
	root := t.TempDir()
	repo := postgres.NewBlobRepository(pool)
	if _, err := repo.Initialize(ctx, media.Descriptor{ID: "cutout-test", Config: blobstore.Config{Driver: "local", LocalRoot: root}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(repo, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	src := filepath.Join(t.TempDir(), "cutout.png")
	if err := os.WriteFile(src, []byte("cutout bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	sum := sha256.Sum256([]byte("cutout bytes"))
	path := "clone/cutouts/" + hash + "/" + hex.EncodeToString(sum[:]) + ".png"
	if err := files.Publish(ctx, []media.Artifact{{Path: path, File: src, ContentType: "image/png"}}, func(tx context.Context) error { return store.OwnCutout(tx, a.ID, hash, path) }); err != nil {
		t.Fatal(err)
	}
	if err := files.Publish(ctx, nil, func(tx context.Context) error { return store.OwnCutout(tx, b.ID, hash, path) }); err != nil {
		t.Fatal(err)
	}
	service := pipeline.Service{Store: store, Media: files}
	for i, id := range []string{a.ID, b.ID} {
		if err := store.SetJobStatus(ctx, id, domain.CloneCancelled); err != nil {
			t.Fatal(err)
		}
		if err := service.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		var owners, bindings int
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM clone_cutouts WHERE logical_path=$1), (SELECT count(*) FROM blob_refs WHERE logical_path=$1)`, path).Scan(&owners, &bindings); err != nil {
			t.Fatal(err)
		}
		want := 1 - i
		if owners != want || bindings != want {
			t.Fatalf("after deletion %d: owners=%d bindings=%d, want %d", i, owners, bindings, want)
		}
		_, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if (want == 0 && !os.IsNotExist(statErr)) || (want == 1 && statErr != nil) {
			t.Fatalf("after deletion %d: local file: %v", i, statErr)
		}
	}
}
