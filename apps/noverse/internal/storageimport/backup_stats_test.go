package storageimport_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/storageimport"
)

func TestSSOBackupCOPYHandlesQuotedIdentifiersNullsAndInfinity(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	if _, err := pool.Pool().Exec(ctx, `
CREATE TABLE "copy table" ("time""quoted" timestamptz, "all null" timestamptz, payload text);
CREATE TABLE "empty table" (at timestamptz);
INSERT INTO "copy table" ("time""quoted",payload) VALUES
('2026-01-01 01:00:00+07', E'escaped\\t tab\\n newline\\\\ slash'),
('2026-01-01 01:00:00+05:30', 'offset'),
('2026-01-01 01:00:00-02:30', 'offset'),
('2026-01-01 01:00:00+00', 'offset'),
('-infinity', 'negative'), ('infinity', 'positive');`); err != nil {
		t.Fatal(err)
	}
	backup := dumpForTest(t, pool)
	if backup.PGRestorePath == "" || !filepath.IsAbs(backup.PGRestorePath) || !strings.Contains(backup.PGRestore, "pg_restore") {
		t.Fatalf("receipt phải giữ đường executable và phiên bản: %+v", backup)
	}
	resolved, err := filepath.EvalSymlinks(backup.PGRestorePath)
	if err != nil || resolved != backup.PGRestorePath {
		t.Fatalf("đường pg_restore phải resolve symlink: %v", err)
	}
	if _, err := pool.Pool().Exec(ctx, `UPDATE "copy table" SET "all null"='infinity' WHERE payload='positive'`); err != nil {
		t.Fatal(err)
	}
	if _, err := storageimport.CheckBackupDir(ctx, pool, filepath.Dir(backup.Path)); err == nil ||
		!strings.Contains(err.Error(), "copy table") || !strings.Contains(err.Error(), "all null") ||
		strings.Contains(err.Error(), "positive") {
		t.Fatalf("timestamp NULL thành infinity phải từ chối mà không in dòng: %v", err)
	}
	if info, err := os.Stat(backup.Path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("dump phải private: %v", err)
	}
}
