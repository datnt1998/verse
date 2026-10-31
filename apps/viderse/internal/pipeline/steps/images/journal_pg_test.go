package images_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// These tests use the real PostgreSQL receipt journal. They live in the
// external test package because persistence/postgres depends (indirectly) on
// this step package.

// Port of images.test.ts "capture survives encode interruption": exercise
// the real imagegen.Service + receipt journal, not an idempotent fake Drawer.
func realJournal(t *testing.T) imagegen.Journal {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := pgmigrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = pgmigrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgres.NewImageReceiptStore(pool)
}

func TestCaptureSurvivesEncodeCrashWithoutDuplicateReceipt(t *testing.T) {
	s, _ := images.TestFixture(t)
	p := &images.TestProvider{}
	j := realJournal(t)
	service := imagegen.NewService(imagegen.Options{Providers: []imagegen.Provider{p}, Journal: j})
	h := images.Handler{Deps: images.Deps{Drawer: service, Analyse: images.TestAnalysis, Encode: func(context.Context, *pipeline.StepContext, string, string, int, int) (images.Encoded, error) {
		return images.Encoded{}, errors.New("encode interrupted")
	}}}
	if _, err := h.Run(context.Background(), s); err == nil {
		t.Fatal("expected encode interruption")
	}
	if p.Calls() != 1 {
		t.Fatal(p.Calls())
	}
	h.Deps.Encode = images.TestEncode
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result.Waiting != nil {
		t.Fatal(result.Waiting)
	}
	if p.Calls() != 1 {
		t.Fatalf("charged again after crash: %d", p.Calls())
	}
	if n, err := j.Charged(context.Background(), "codex", time.Time{}); err != nil || n != 1 {
		t.Fatalf("duplicate receipts: %d, %v", n, err)
	}
	var m images.Manifest
	if err = json.Unmarshal(s.Checkpoint, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Items[0].Versions) != 1 || m.Items[0].InFlight != nil {
		t.Fatalf("not recovered: %+v", m.Items[0])
	}
}
func TestOwnerRedrawUsesNewReceiptWhileCrashRetryReusesFlight(t *testing.T) {
	s, _ := images.TestFixture(t)
	p := &images.TestProvider{}
	j := realJournal(t)
	h := images.Handler{Deps: images.Deps{Drawer: imagegen.NewService(imagegen.Options{Providers: []imagegen.Provider{p}, Journal: j}), Analyse: images.TestAnalysis, Encode: images.TestEncode}}
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	var m images.Manifest
	if err := json.Unmarshal(s.Checkpoint, &m); err != nil {
		t.Fatal(err)
	}
	next, err := images.Review(m, "thumbnail", "regenerate", "new composition", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveCheckpoint(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if p.Calls() != 2 {
		t.Fatalf("redraw reused old paid receipt: calls=%d", p.Calls())
	}
	if n, err := j.Charged(context.Background(), "codex", time.Time{}); err != nil || n != 2 {
		t.Fatalf("redraw journal=%d %v", n, err)
	}
}
func TestD7ExhaustionParksWithRetryAtAndNoPaidDraw(t *testing.T) {
	s, _ := images.TestFixture(t)
	p := &images.TestProvider{}
	zero := 0
	service := imagegen.NewService(imagegen.Options{Providers: []imagegen.Provider{p}, Journal: realJournal(t), DailyPaidImageLimit: &zero})
	h := images.Handler{Deps: images.Deps{Drawer: service, Analyse: images.TestAnalysis, Encode: images.TestEncode}}
	result, err := h.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result.Waiting == nil || result.Waiting.Code != domain.CloneErrQuota || result.Waiting.RetryAt == nil {
		t.Fatalf("not waiting: %+v", result)
	}
	if p.Calls() != 0 {
		t.Fatalf("quota spent: %d", p.Calls())
	}
	if len(result.Artifacts) != 1 {
		t.Fatal("partial manifest not published")
	}
}
