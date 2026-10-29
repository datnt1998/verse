// Ca test chuyển từ src/server/db/settings-store.test.ts của yt-factory: khoá
// thiếu đọc như chưa ghi, giá trị sai kiểu đọc như chưa ghi, ghi đè lần hai,
// xoá khoá không có là no-op, và cờ tạm dừng đọc đúng qua đúng-khoá.
//
// Chạy trên Postgres thật: provision schema riêng bằng pgtest với cặp role
// của Viderse rồi áp migration nhúng. Bỏ qua khi thiếu TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cặp role và khoá advisory của Viderse (Noverse dùng 2710,11 — không được trùng
// khi các suite chạy song song trên cùng cụm).
var roleSpec = pgtest.RoleSpec{
	MigrateRole:   "viderse_migrate",
	AppRole:       "viderse_app",
	LockKey1:      2790,
	LockKey2:      1,
	InitScriptRef: "deploy/postgres-init/01-roles.sh",
}

func provision(t *testing.T) *pgxpool.Pool {
	t.Helper()
	appDSN, maintenanceDSN := pgtest.ProvisionRoles(t, roleSpec)
	ctx := context.Background()
	db, err := pgmigrate.OpenDB(maintenanceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := pgmigrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	pool, err := postgres.OpenPool(ctx, appDSN)
	if err != nil {
		t.Fatalf("mở pool app: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSettingsAbsentKeyReadsAsAbsent(t *testing.T) {
	store := postgres.NewSettingsStore(provision(t))
	var v map[string]any
	found, err := store.Get(context.Background(), "missing", &v)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("khoá thiếu phải trả found=false")
	}
}

func TestSettingsWrongTypeReadsAsAbsent(t *testing.T) {
	store := postgres.NewSettingsStore(provision(t))
	ctx := context.Background()
	if err := store.Set(ctx, "k", "không phải object"); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	found, err := store.Get(ctx, "k", &v)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("giá trị sai kiểu phải đọc như chưa ghi")
	}
}

func TestSettingsWriteOverwrites(t *testing.T) {
	store := postgres.NewSettingsStore(provision(t))
	ctx := context.Background()
	if err := store.Set(ctx, "k", map[string]any{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "k", map[string]any{"n": 2}); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if found, err := store.Get(ctx, "k", &v); err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if v["n"] != float64(2) {
		t.Fatalf("v = %v", v)
	}
}

func TestSettingsClearAbsentIsNoop(t *testing.T) {
	store := postgres.NewSettingsStore(provision(t))
	if err := store.Clear(context.Background(), "missing"); err != nil {
		t.Fatalf("xoá khoá không có phải no-op: %v", err)
	}
}

// Cờ tạm dừng đọc false khi chưa ghi, đúng giá trị sau khi ghi, và dùng đúng
// khoá settings (clone.queuePaused) của bản TS.
func TestQueuePausedDefaultsFalseAndRoundTrips(t *testing.T) {
	store := postgres.NewSettingsStore(provision(t))
	ctx := context.Background()

	if paused, err := store.QueuePaused(ctx); err != nil || paused {
		t.Fatalf("mặc định paused=%v err=%v", paused, err)
	}
	if err := store.SetQueuePaused(ctx, true); err != nil {
		t.Fatal(err)
	}
	if paused, err := store.QueuePaused(ctx); err != nil || !paused {
		t.Fatalf("sau khi ghi paused=%v err=%v", paused, err)
	}

	var raw bool
	found, err := store.Get(ctx, domain.KeyQueuePaused, &raw)
	if err != nil || !found || !raw {
		t.Fatalf("khoá %q: found=%v raw=%v err=%v", domain.KeyQueuePaused, found, raw, err)
	}
}
