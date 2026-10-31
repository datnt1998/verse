package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
)

func TestGlobalRulesMigrationCopiesAllHostedProjectsOnce(t *testing.T) {
	f := provisionLibrary(t, "one")
	ctx := context.Background()
	q := gen.New(f.pool.Pool())
	if _, err := q.InsertProject(ctx, gen.InsertProjectParams{ID: deterministicUUID("project", "two"), Slug: "two", Name: "Hai"}); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "tone.md"), []byte("Giọng toàn cục cũ"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, rules.ManagedRulesFile), []byte("- Quy tắc chung\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	docs := postgres.NewTextDocuments(f.pool)
	if _, err := docs.Mutate(ctx, nil, "one", "", "rules", rules.ManagedRulesFile, "- Quy tắc dự án\n", "", false, 0); err != nil {
		t.Fatal(err)
	}
	if err := docs.MigrateGlobalRules(ctx, home); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"one", "two"} {
		rows, err := docs.List(ctx, nil, slug, "", "rules")
		if err != nil || len(rows) != 2 {
			t.Fatalf("%s rows: %+v %v", slug, rows, err)
		}
		for _, row := range rows {
			if row.Name == rules.ManagedRulesFile && strings.Count(row.Text, "Quy tắc chung") != 1 {
				t.Fatalf("missing/duplicate migrated rule: %+v", row)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(home, "later.md"), []byte("Không chép lại"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := docs.MigrateGlobalRules(ctx, home); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE target_id=$1`, rules.GlobalMigrationKey).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
	if err := f.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM noverse_meta WHERE key=$1 AND value='done'`, rules.GlobalMigrationKey).Scan(&count); err != nil || count != 1 {
		t.Fatalf("marker=%d err=%v", count, err)
	}
	rows, err := docs.List(ctx, nil, "two", "", "rules")
	if err != nil || len(rows) != 2 || rows[0].Version != 1 || rows[1].Version != 1 {
		t.Fatalf("rerun changed project documents: %+v %v", rows, err)
	}
}

func TestGlobalRulesMigrationRollsBackAuditAndCopiesTogether(t *testing.T) {
	f := provisionLibrary(t, "one")
	ctx := context.Background()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "tone.md"), []byte("Luật cũ"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Pool().Exec(ctx, `CREATE FUNCTION reject_rules_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected audit failure'; END $$; CREATE TRIGGER reject_rules_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION reject_rules_audit()`); err != nil {
		t.Fatal(err)
	}
	docs := postgres.NewTextDocuments(f.pool)
	if err := docs.MigrateGlobalRules(ctx, home); err == nil {
		t.Fatal("ignored audit failure")
	}
	rows, err := docs.List(ctx, nil, "one", "", "rules")
	if err != nil || len(rows) != 0 {
		t.Fatalf("partial copies committed: %+v %v", rows, err)
	}
	var count int
	if err := f.pool.Pool().QueryRow(ctx, `SELECT count(*) FROM noverse_meta WHERE key=$1`, rules.GlobalMigrationKey).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial marker committed: %d %v", count, err)
	}
}

func TestSystemAuditCapabilityPersistsOnceInHostedStore(t *testing.T) {
	f := provisionLibrary(t, "one")
	repo := identityRepo(f.pool)
	for range 2 {
		if err := repo.RecordSystemAudit("Chuyển quy tắc", "migration-test", "Đã chuyển"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := f.pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE actor_kind='system' AND action='Chuyển quy tắc' AND target_id='migration-test' AND details->>'reason'='Đã chuyển'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("system audit count=%d err=%v", count, err)
	}
}
