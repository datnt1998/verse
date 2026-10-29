package storagegc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storagegc"
)

func TestGCApplicationRoleCannotDelete(t *testing.T) {
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t)
	ctx := context.Background()
	db, err := migrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := migrate.Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	app, err := postgres.Open(ctx, postgres.PoolConfig{DSN: appDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	for _, table := range []string{"blobs", "export_cache", "audit_logs", "chapter_versions", "import_items"} {
		_, err := app.Pool().Exec(ctx, "DELETE FROM "+table)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "42501" {
			t.Fatalf("app DELETE FROM %s must fail with permission denied (42501): %v", table, err)
		}
	}
	maintenance, err := postgres.Open(ctx, postgres.PoolConfig{DSN: maintenanceDSN})
	if err != nil {
		t.Fatal(err)
	}
	defer maintenance.Close()
	keys, _ := blobkey.New(blobkey.EnvDev)
	f := &fixture{pool: maintenance, store: newMinioStore(t), keys: keys, now: time.Now()}
	if _, err := app.Pool().Exec(ctx, `INSERT INTO projects (id,slug,name) VALUES ($1,'roles','Roles');
		`, testProject); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Pool().Exec(ctx, `INSERT INTO stories (project_id,id,slug,title,status)
		VALUES ($1,$2,'roles','Roles','writing')`, testProject, testStory); err != nil {
		t.Fatal(err)
	}
	project, _ := postgres.ParseUUID(testProject)
	story, _ := postgres.ParseUUID(testStory)
	scope := postgres.ScopeRef{ProjectID: project, StoryID: story}
	key, _ := keys.Export(testProject, testStory, hexOf("app-export"), "roles.txt")
	f.put(t, key, "app-export")
	audio := postgres.NewAudio(app)
	staged, err := audio.StageBlob(ctx, project, key, "s3", "text/plain", pgtype.UUID{}, pgtype.UUID{})
	if err != nil {
		t.Fatalf("app stage upload: %v", err)
	}
	ready, err := audio.MarkBlobReady(ctx, project, staged.ID, hexOf("app-export"), 10)
	if err != nil {
		t.Fatalf("app publish ready: %v", err)
	}
	cache := postgres.NewExportCache(app)
	if _, err := cache.Put(ctx, scope, postgres.ExportCacheEntry{CacheKey: "export", Kind: "txt", BlobID: ready.ID}, nil); err != nil {
		t.Fatalf("app cache export: %v", err)
	}
	if _, err := cache.Get(ctx, scope, "export", "txt"); err != nil {
		t.Fatalf("app read export: %v", err)
	}
	if n, err := cache.Expire(ctx, scope); err != nil || n != 1 {
		t.Fatalf("app mark cache expiry: n=%d err=%v", n, err)
	}
	if len(f.blobRows(t)) != 1 || !f.exists(key) {
		t.Fatal("app expiry removed row or object")
	}
	// Supplying the app DSN under a maintenance label cannot authorize GC.
	if _, err := storagegc.Run(ctx, storagegc.Options{Pool: app, Blobs: f.store, Keys: keys}); err == nil {
		t.Fatal("GC accepted application principal")
	}
	rep := f.run(t, false)
	if rep.CacheDeleted != 1 || rep.CacheBlobsReaped != 1 || len(f.blobRows(t)) != 0 || f.exists(key) {
		t.Fatalf("maintenance GC failed to collect app's expired export: %+v", rep)
	}
}
