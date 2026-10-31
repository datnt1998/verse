package postgres_test

import (
	"context"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"strings"
	"testing"
)

func TestBundledTemplateMediaIsOwnedAcrossRestartAndGC(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	hash := strings.Repeat("0", 64)
	key := "v1/sha256/00/" + hash
	if _, err := pool.Exec(ctx, `INSERT INTO blobs(object_key,sha256,bytes) VALUES($1,$2,1)`, key, hash); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"library/standard-template/scene-cool.webp", "library/standard-template/not-bundled.webp"} {
		if _, err := pool.Exec(ctx, `INSERT INTO blob_refs(logical_path,object_key,content_type,filename) VALUES($1,$2,'image/webp','asset.webp')`, path, key); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := postgres.NewBlobRepository(pool).Roots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Path != "library/standard-template/scene-cool.webp" || roots[0].Group != "library" {
		t.Fatalf("immutable system asset has no exact durable owner: %+v", roots)
	}
}
