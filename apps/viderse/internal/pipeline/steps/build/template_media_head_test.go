package build

import (
	"bytes"
	"context"
	"errors"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

func TestTemplateMediaUsesAuthoritativeEditorHead(t *testing.T) {
	ctx := context.Background()
	app, maintenance := pgtest.ProvisionRoles(t, pgtest.RoleSpec{MigrateRole: "viderse_migrate", AppRole: "viderse_app", LockKey1: 2790, LockKey2: 1, InitScriptRef: "deploy/postgres-init/01-roles.sh"})
	db, err := pgmigrate.OpenDB(maintenance)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = pgmigrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	blobs := postgres.NewBlobRepository(pool)
	if _, err = blobs.Initialize(ctx, media.Descriptor{ID: "template-head", Config: blobstore.Config{Driver: "local", LocalRoot: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	files := media.New(blobs, nil, func(d media.Descriptor) (blobstore.BlobStore, error) { return blobstore.NewLocal(d.Config.LocalRoot) })
	repo := postgres.NewEditorRepository(pool)
	service := &editor.Service{Store: editor.Store{Media: files, Repository: repo, StagingDir: t.TempDir()}, Media: files, Repository: repo}
	var buf bytes.Buffer
	pic := image.NewRGBA(image.Rect(0, 0, 2, 2))
	pic.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	if err = png.Encode(&buf, pic); err != nil {
		t.Fatal(err)
	}
	record, err := service.ImportMedia(ctx, editor.ImportMediaOptions{FileName: "reference.png", DeclaredMIME: "image/png", Body: bytes.NewReader(buf.Bytes())})
	if err != nil {
		t.Fatal(err)
	}
	source, err := repo.Get(ctx, "media/"+record.ID+"/source")
	if err != nil {
		t.Fatal(err)
	}
	if h, e := files.Acquire(ctx, "editor/media/"+record.ID+"/metadata.json"); e == nil {
		h.Release()
		t.Fatal("fixture unexpectedly published legacy metadata path")
	}
	pin := library.TemplateMediaBinding{MediaID: record.ID, ContentHash: record.ContentHash}
	step := &pipeline.StepContext{Media: files}
	handler := Handler{Deps: Deps{MediaMetadata: service.GetMediaMetadata}}
	if err = handler.verifyTemplateMedia(ctx, step, pin, source.Path); err != nil {
		t.Fatalf("valid pinned media cannot build after editor import: %v", err)
	}
	for _, mode := range []string{"deleted", "replaced"} {
		t.Run(mode, func(t *testing.T) {
			item, err := service.ImportMedia(ctx, editor.ImportMediaOptions{FileName: mode + ".png", DeclaredMIME: "image/png", Body: bytes.NewReader(buf.Bytes())})
			if err != nil {
				t.Fatal(err)
			}
			original, err := repo.Get(ctx, "media/"+item.ID+"/source")
			if err != nil {
				t.Fatal(err)
			}
			bound := library.TemplateMediaBinding{MediaID: item.ID, ContentHash: item.ContentHash}
			if mode == "deleted" {
				if err = service.DeleteMedia(ctx, item.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				var changed bytes.Buffer
				pic.SetRGBA(1, 1, color.RGBA{B: 255, A: 255})
				if err = png.Encode(&changed, pic); err != nil {
					t.Fatal(err)
				}
				if _, err = service.ReplaceMedia(ctx, item.ID, editor.ImportMediaOptions{FileName: "replacement.png", DeclaredMIME: "image/png", Body: bytes.NewReader(changed.Bytes())}); err != nil {
					t.Fatal(err)
				}
			}
			err = handler.verifyTemplateMedia(ctx, step, bound, original.Path)
			var rejected *domain.CloneError
			if !errors.As(err, &rejected) || rejected.Code != domain.CloneErrStale {
				t.Fatalf("changed template media was not rejected as stale: %v", err)
			}
		})
	}
}
