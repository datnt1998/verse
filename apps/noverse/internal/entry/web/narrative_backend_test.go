package web

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/pgtest"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/webauth"
)

// provisionWebStory migrates an isolated schema and anchors one project+story
// so the server-side factory has a real scope to resolve.
func provisionWebStory(t *testing.T, projectSlug, storySlug string) (*postgres.Pool, *postgres.Narrative, postgres.ScopeRef) {
	t.Helper()
	dsn := pgtest.Provision(t)
	return provisionWebStoryDSNs(t, projectSlug, storySlug, dsn, dsn)
}

func provisionWebStoryRoles(t *testing.T, projectSlug, storySlug string) (*postgres.Pool, *postgres.Narrative, postgres.ScopeRef) {
	t.Helper()
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t)
	return provisionWebStoryDSNs(t, projectSlug, storySlug, appDSN, maintenanceDSN)
}

func provisionWebStoryDSNs(t *testing.T, projectSlug, storySlug, appDSN, maintenanceDSN string) (*postgres.Pool, *postgres.Narrative, postgres.ScopeRef) {
	t.Helper()
	db, err := migrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: appDSN})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	ctx := context.Background()
	nar := postgres.NewNarrative(pool)
	if _, err := gen.New(pool.Pool()).EnsureProject(ctx, gen.EnsureProjectParams{
		ID: pgUUID(t, projectSlug), Slug: projectSlug, Name: projectSlug,
	}); err != nil {
		t.Fatalf("anchor project: %v", err)
	}
	if _, err := nar.CreateStory(ctx, projectSlug, nil, postgres.StoryInfo{
		Slug: storySlug, Title: "Truyện thử", Status: "writing",
	}); err != nil {
		t.Fatalf("create story: %v", err)
	}
	scope, err := nar.Resolve(ctx, projectSlug, storySlug, nil, false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return pool, nar, scope
}

func pgUUID(t *testing.T, seed string) pgtype.UUID {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var u pgtype.UUID
	copy(u.Bytes[:], b)
	u.Valid = true
	return u
}

// TestNarrativeBackendBothModes proves the narrative readers behind
// engineBookTitle use whichever backend the composition root installed:
// file by default, postgres when a scoped factory is set — and that the PG
// path refuses to fall back to the directory when the factory fails.
func TestNarrativeBackendBothModes(t *testing.T) {
	tg := newTestGateway(t)
	st := library.Story{Project: "proj", Story: "truyen", ID: "proj/truyen"}

	t.Run("file", func(t *testing.T) {
		out := tg.g.storyOutputDir(st)
		engine := store.NewStore(out)
		if err := engine.Book.Save(domain.BookMetadata{Title: "Sách file", Synopsis: "mở"}); err != nil {
			t.Fatal(err)
		}
		if got := tg.g.engineBookTitle(st, nil); got != "Sách file" {
			t.Fatalf("file mode must read the engine book: %q", got)
		}
	})

	t.Run("postgres", func(t *testing.T) {
		pool, nar, scope := provisionWebStory(t, "proj", "truyen")
		backend := postgres.NewBackend(context.Background(), pool, nar, scope)
		scoped, err := store.NewFromBackend(backend)
		if err != nil {
			t.Fatalf("postgres facade: %v", err)
		}
		if err := scoped.Book.Save(domain.BookMetadata{Title: "Sách PG", Synopsis: "mở"}); err != nil {
			t.Fatal(err)
		}
		calls := 0
		tg.g.SetFactStore(func(library.Story, *webauth.User) (*store.Store, error) {
			calls++
			return scoped, nil
		})
		if got := tg.g.engineBookTitle(st, nil); got != "Sách PG" {
			t.Fatalf("postgres mode must read the row book: %q", got)
		}
		if calls != 1 {
			t.Fatalf("factory must be the only source: %d", calls)
		}

		tg.g.SetFactStore(func(library.Story, *webauth.User) (*store.Store, error) {
			return nil, fmt.Errorf("pool down")
		})
		if got := tg.g.engineBookTitle(st, nil); got != "" {
			t.Fatalf("failed factory must not fall back to the directory: %q", got)
		}
	})

	t.Run("no_engine_directory_on_pg", func(t *testing.T) {
		// The PG backend must never claim a directory: engine code paths that
		// still assume one have to fail loudly instead of reading a tree.
		tg.g.SetFactStore(func(library.Story, *webauth.User) (*store.Store, error) {
			return nil, fmt.Errorf("not configured")
		})
		if _, err := tg.g.openFactStore(st, nil); err == nil {
			t.Fatal("openFactStore must surface factory errors")
		}
		if _, err := os.Stat(tg.g.storyOutputDir(st)); err == nil {
			t.Log("file fixture exists; PG factory still takes precedence")
		}
	})
}
