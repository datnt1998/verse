package postgres_test

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// applyRoleScript runs a postgres-init psql script against the test schema.
// The deploy contract is psql-variables + meta commands, so the real psql
// binary runs it — re-implementing \if/:\var substitution would test a
// different script than the one operators run.
func applyRoleScript(t *testing.T, adminDSN, script string, vars ...string) {
	t.Helper()
	psql, err := exec.LookPath("psql")
	if err != nil {
		t.Skip("psql not on PATH; cannot apply the role script as written")
	}
	// libpq does not accept search_path as a URI query parameter; it travels
	// via PGOPTIONS instead.
	dsn, searchPath := libpqDSN(t, adminDSN)
	args := []string{dsn, "-v", "ON_ERROR_STOP=1", "-v", "worker_schema=" + pgtest.SchemaOf(adminDSN)}
	for _, v := range vars {
		args = append(args, "-v", v)
	}
	args = append(args, "-f", script)
	cmd := exec.Command(psql, args...)
	if searchPath != "" {
		cmd.Env = append(os.Environ(), "PGOPTIONS=-csearch_path="+searchPath)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("psql %s: %v\n%s", filepath.Base(script), err, out)
	}
}

// libpqDSN moves pgtest's search_path out of the URI into the PGOPTIONS form
// libpq understands; the returned string is safe to hand to psql.
func libpqDSN(t *testing.T, dsn string) (clean, searchPath string) {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	q := u.Query()
	searchPath = q.Get("search_path")
	q.Del("search_path")
	u.RawQuery = q.Encode()
	return u.String(), searchPath
}

// TestWorkerRoleCannotReadIdentity applies deploy/postgres-init/
// 03-engine-worker-role.sql to a migrated throwaway schema and asserts the
// exact boundary the hosted worker child gets: facts + story lease work,
// every account/grant/audit read and every DDL attempt is refused by the
// database itself — the role, not the app, enforces it.
func TestWorkerRoleCannotReadIdentity(t *testing.T) {
	adminDSN := pgtest.Provision(t)
	db, err := migrate.OpenDB(adminDSN)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()

	const workerPassword = "worker-test-pass-9f8e7d"
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "03-engine-worker-role.sql")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("role script: %v", err)
	}
	applyRoleScript(t, adminDSN, script, "worker_password="+workerPassword)

	ctx := context.Background()
	worker, err := pgx.Connect(ctx, workerDSN(t, adminDSN, workerPassword))
	if err != nil {
		t.Fatalf("worker connect: %v", err)
	}
	defer worker.Close(ctx)
	t.Cleanup(func() {
		admin, err := pgx.Connect(context.Background(), adminDSN)
		if err != nil {
			return
		}
		defer admin.Close(context.Background())
		_, _ = admin.Exec(context.Background(), "DROP ROLE IF EXISTS noverse_engine_worker")
	})

	// Facts scope: every granted table reads; the lease row is fully mutable.
	for _, table := range []string{
		"projects", "noverse_meta", "goose_db_version", "stories",
		"story_leases", "project_documents", "volumes", "chapters",
		"outlines", "characters", "comments", "comment_replies",
		"reviews", "review_decisions", "story_state", "story_artifacts",
		"story_events", "checkpoints", "agent_sessions", "chapter_versions",
	} {
		if _, err := worker.Exec(ctx, "SELECT 1 FROM "+table+" LIMIT 0"); err != nil {
			t.Fatalf("worker cannot SELECT facts table %s: %v", table, err)
		}
	}

	// Account/grant/audit/app tables are refused — SELECT and writes alike.
	for _, table := range []string{
		"users", "user_identities", "user_credentials", "sessions",
		"devices", "device_pairings", "app_grants", "audit_logs",
		"auth_invariant_lock", "login_rate_limits", "recovery_codes",
		"password_reset_tokens", "temporary_passwords", "project_members",
		"jobs", "blobs", "workers", "library_settings",
	} {
		if _, err := worker.Exec(ctx, "SELECT 1 FROM "+table+" LIMIT 0"); err == nil {
			t.Fatalf("worker read app table %s", table)
		}
	}

	// History stays append-only even where a table is granted: UPDATE and
	// DELETE on chapter_versions must fail the permission check, not the
	// row count.
	for _, verb := range []string{
		"UPDATE chapter_versions SET id = id WHERE false",
		"DELETE FROM chapter_versions WHERE false",
	} {
		if _, err := worker.Exec(ctx, verb); err == nil {
			t.Fatalf("worker mutated history: %s", verb)
		}
	}

	// No DDL, ever.
	if _, err := worker.Exec(ctx, "CREATE TABLE worker_ddl_probe (id int)"); err == nil {
		t.Fatal("worker created a table")
	}
	if _, err := worker.Exec(ctx, "CREATE SCHEMA worker_schema_probe"); err == nil {
		t.Fatal("worker created a schema")
	}
}

// TestWorkerRoleScriptSkipsWithoutPassword pins the operator contract: the
// script must be re-runnable in the initdb hook (before an operator supplies
// the password) without creating a role or failing the directory.
func TestWorkerRoleScriptSkipsWithoutPassword(t *testing.T) {
	psql, err := exec.LookPath("psql")
	if err != nil {
		t.Skip("psql not on PATH")
	}
	adminDSN := pgtest.RequireURL(t)
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "03-engine-worker-role.sql")
	cmd := exec.Command(psql, adminDSN, "-v", "ON_ERROR_STOP=1", "-f", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unprovisioned role script failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "skipping") {
		t.Fatalf("expected the skip notice, got: %s", out)
	}
}

// workerDSN rewrites a provisioned DSN to authenticate as the engine-worker
// role while keeping the schema's search_path runtime parameter (pgx reads
// it, unlike libpq).
func workerDSN(t *testing.T, dsn, password string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	u.User = url.UserPassword("noverse_engine_worker", password)
	return u.String()
}
