package postgres_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

// minimalPsqlEnv is the whole environment handed to psql/sh children: PATH
// (and a stable locale for message matching) plus whatever the caller adds.
// Ambient PG* and *_PASSWORD variables are never inherited, so a leaked
// WORKER_ROLE_PASSWORD/IDENTITY_ROLE_PASSWORD in the test runner's env can
// never change what a script under test sees.
func minimalPsqlEnv() []string {
	return []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
}

// roleTestCluster isolates cluster-global login names from the web suites and
// enables the logging collector so password-leak assertions always run.
func roleTestCluster(t *testing.T, settings ...string) string {
	t.Helper()
	pgtest.RequireURL(t)
	dir, err := os.MkdirTemp("/tmp", "verse-roles-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	dataDir := filepath.Join(dir, "data")
	for _, name := range []string{"initdb", "pg_ctl", "psql"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("%s is required for role-script tests: %v", name, err)
		}
	}
	init := exec.Command("initdb", "-D", dataDir, "-U", "dat", "-A", "trust", "--no-locale")
	init.Env = minimalPsqlEnv()
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("initdb: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "pg_hba.conf"),
		[]byte("local all dat trust\nlocal all all scram-sha-256\n"), 0600); err != nil {
		t.Fatal(err)
	}
	options := "-k " + dir + " -c listen_addresses='' -c logging_collector=on -c log_statement=all -c log_min_error_statement=error"
	if len(settings) != 0 {
		options += " " + strings.Join(settings, " ")
	}
	start := exec.Command("pg_ctl", "-D", dataDir, "-l", filepath.Join(dir, "startup.log"), "-o", options, "-w", "start")
	start.Env = minimalPsqlEnv()
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("pg_ctl start: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		stop := exec.Command("pg_ctl", "-D", dataDir, "-m", "immediate", "-w", "stop")
		stop.Env = minimalPsqlEnv()
		if out, err := stop.CombinedOutput(); err != nil {
			t.Errorf("pg_ctl stop: %v\n%s", err, out)
		}
	})
	u := &url.URL{Scheme: "postgres", User: url.User("dat"), Path: "/postgres"}
	q := url.Values{"host": {dir}, "search_path": {"public"}}
	u.RawQuery = q.Encode()
	return u.String()
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

// runRoleScript runs a postgres-init psql script and returns its combined
// output without asserting success, so refusal tests can inspect the failure.
// The deploy contract is psql-variables + meta commands, so the real psql
// binary runs it — re-implementing \if/:\var substitution would test a
// different script than the one operators run.
func runRoleScript(t *testing.T, dsn, script string, env, vars []string) ([]byte, error) {
	t.Helper()
	psql, err := exec.LookPath("psql")
	if err != nil {
		t.Fatalf("psql is required to apply the role script as written: %v", err)
	}
	clean, searchPath := libpqDSN(t, dsn)
	args := []string{"-X", clean, "-v", "ON_ERROR_STOP=1"}
	for _, v := range vars {
		args = append(args, "-v", v)
	}
	args = append(args, "-f", script)
	cmd := exec.Command(psql, args...)
	cmd.Env = append(minimalPsqlEnv(), env...)
	if searchPath != "" {
		cmd.Env = append(cmd.Env, "PGOPTIONS=-csearch_path="+searchPath)
	}
	return cmd.CombinedOutput()
}

// applyRoleScript runs a postgres-init psql script against the test schema and
// fails the test when the script fails.
func applyRoleScript(t *testing.T, adminDSN, script string, env []string, vars ...string) {
	t.Helper()
	vars = append([]string{"worker_schema=" + pgtest.SchemaOf(adminDSN)}, vars...)
	out, err := runRoleScript(t, adminDSN, script, env, vars)
	if err != nil {
		t.Fatalf("psql %s: %v\n%s", filepath.Base(script), err, out)
	}
}

// roleExists reports whether the cluster role is currently present.
func roleExists(t *testing.T, dsn, name string) bool {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect for role check: %v", err)
	}
	defer conn.Close(context.Background())
	var exists bool
	if err := conn.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)`, name).Scan(&exists); err != nil {
		t.Fatalf("role check %s: %v", name, err)
	}
	return exists
}

// dropRole removes a role together with every privilege/object it holds, so a
// DROP ROLE never fails on residual grants.
func dropRole(t *testing.T, dsn, name string) {
	t.Helper()
	if !roleExists(t, dsn, name) {
		return
	}
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect for role drop: %v", err)
	}
	defer conn.Close(context.Background())
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := conn.Exec(context.Background(), "DROP OWNED BY "+ident); err != nil {
		t.Fatalf("drop owned by %s: %v", name, err)
	}
	if _, err := conn.Exec(context.Background(), "DROP ROLE IF EXISTS "+ident); err != nil {
		t.Fatalf("drop role %s: %v", name, err)
	}
}

// setupLoggingProbe creates the non-superuser CREATEROLE principal the
// fail-closed tests run as, with per-role statement logging forced to its
// noisiest setting — if the scripts ever sent a password-bearing statement
// from this session it would land in the server log. Returns the probe's DSN
// and the server log file path ("" when the logging collector is off).
func setupLoggingProbe(t *testing.T, adminDSN string) (probeDSN, logPath string) {
	t.Helper()
	const (
		probe     = "verse_role_probe"
		probePass = "verse-role-probe-pw"
	)
	conn, err := pgx.Connect(context.Background(), adminDSN)
	if err != nil {
		t.Fatalf("connect admin for probe setup: %v", err)
	}
	defer conn.Close(context.Background())
	ctx := context.Background()
	probeIdent := pgx.Identifier{probe}.Sanitize()
	dropRole(t, adminDSN, probe)
	if _, err := conn.Exec(ctx, fmt.Sprintf(
		`CREATE ROLE %s LOGIN CREATEROLE PASSWORD '%s'`, probeIdent, probePass)); err != nil {
		t.Fatalf("create probe role: %v", err)
	}
	if _, err := conn.Exec(ctx, fmt.Sprintf(
		`ALTER ROLE %s SET log_statement = 'all';
		 ALTER ROLE %s SET log_min_error_statement = 'error'`, probeIdent, probeIdent)); err != nil {
		t.Fatalf("probe logging GUCs (needs a superuser test admin): %v", err)
	}
	if _, err := conn.Exec(ctx, `ALTER ROLE dat SET log_statement = 'all';
		ALTER ROLE dat SET log_min_error_statement = 'error';
		ALTER ROLE dat SET log_transaction_sample_rate = 1;
		ALTER ROLE dat SET log_min_duration_sample = 0;
		ALTER ROLE dat SET debug_print_parse = on;
		ALTER ROLE dat SET debug_print_rewritten = on;
		ALTER ROLE dat SET debug_print_plan = on`); err != nil {
		t.Fatalf("bootstrap role logging GUCs: %v", err)
	}
	t.Cleanup(func() { dropRole(t, adminDSN, probe) })

	var log *string
	if err := conn.QueryRow(ctx, `SELECT pg_current_logfile()`).Scan(&log); err != nil {
		t.Fatalf("pg_current_logfile: %v", err)
	}
	if log != nil && !filepath.IsAbs(*log) {
		var dataDir string
		if err := conn.QueryRow(ctx, `SHOW data_directory`).Scan(&dataDir); err != nil {
			t.Fatalf("data_directory: %v", err)
		}
		logPath = filepath.Join(dataDir, *log)
	} else if log != nil {
		logPath = *log
	}

	clean, _ := libpqDSN(t, adminDSN)
	u, err := url.Parse(clean)
	if err != nil {
		t.Fatalf("parse probe DSN: %v", err)
	}
	u.User = url.UserPassword(probe, probePass)
	return u.String(), logPath
}

// assertServerLogLacks lets the collector flush, then checks the actual server
// log. A missing collector file is a failure, never a skipped assertion.
func assertServerLogLacks(t *testing.T, logPath, marker string) {
	t.Helper()
	if logPath == "" {
		t.Fatal("private test cluster did not expose its logging collector file")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read server log %s: %v", logPath, err)
		}
		if time.Now().After(deadline) {
			if strings.Contains(string(data), marker) {
				t.Fatalf("marker password reached the server log %s", logPath)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// assertRolePasswordActivityHidden pauses psql with client-side meta commands
// after each password-bearing statement, including the last one before COMMIT.
// No intervening SQL can overwrite the activity text under inspection.
func assertRolePasswordActivityHidden(t *testing.T, dsn, sql string, env []string, checkpoints int, markers ...string) {
	t.Helper()
	beforeCommit, afterCommit, ok := strings.Cut(sql, "COMMIT;")
	if !ok {
		t.Fatal("role creation transaction has no COMMIT")
	}
	const checkpoint = "role-password-activity-checkpoint"
	var paused strings.Builder
	passwordSQL := false
	for _, line := range strings.Split(beforeCommit, "\n") {
		if strings.HasPrefix(line, "SELECT :'") {
			passwordSQL = true
		}
		paused.WriteString(line + "\n")
		if passwordSQL && (line == `\gset` || line == `\gexec`) {
			paused.WriteString("\\echo " + checkpoint + "\n\\prompt role_activity_resume\n")
		}
	}
	paused.WriteString("COMMIT;" + afterCommit)
	script := filepath.Join(t.TempDir(), "role-activity.sql")
	if err := os.WriteFile(script, []byte(paused.String()), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	observer, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close(context.Background())
	clean, searchPath := libpqDSN(t, dsn)
	cmd := exec.CommandContext(ctx, "psql", "-X", clean, "-v", "ON_ERROR_STOP=1", "-v", "database_name=postgres", "-f", script)
	cmd.Env = append(minimalPsqlEnv(), env...)
	cmd.Env = append(cmd.Env, "PGAPPNAME=role-password-activity")
	if searchPath != "" {
		cmd.Env = append(cmd.Env, "PGOPTIONS=-csearch_path="+searchPath)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	seen := 0
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if scanner.Text() != checkpoint {
			continue
		}
		seen++
		var state, query string
		if err := observer.QueryRow(ctx, `SELECT state, query FROM pg_stat_activity
			WHERE application_name = 'role-password-activity'`).Scan(&state, &query); err != nil {
			t.Fatal(err)
		}
		for _, marker := range markers {
			if strings.Contains(query, marker) {
				t.Errorf("password marker visible in pg_stat_activity at checkpoint %d", seen)
			}
		}
		if state != "disabled" || query != "" {
			t.Errorf("password activity at checkpoint %d: state=%q, query cleared=%v", seen, state, query == "")
		}
		if _, err := io.WriteString(stdin, "\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("paused role script: %v\n%s", err, stderr.String())
	}
	if seen != checkpoints {
		t.Fatalf("inspected %d password statements, want %d", seen, checkpoints)
	}
}

func TestWorkerRoleScriptKeepsPasswordOutOfActivity(t *testing.T) {
	dsn := roleTestCluster(t)
	migrateSchema(t, dsn)
	sql, err := os.ReadFile(workerRoleScript(t))
	if err != nil {
		t.Fatal(err)
	}
	assertRolePasswordActivityHidden(t, dsn, string(sql),
		[]string{"WORKER_ROLE_PASSWORD=worker-activity-marker-'quoted space"},
		2, "worker-activity-marker-")
}

func TestInitRolesScriptKeepsPasswordsOutOfActivity(t *testing.T) {
	dsn := roleTestCluster(t)
	source, err := os.ReadFile(initRolesScript(t))
	if err != nil {
		t.Fatal(err)
	}
	_, sql, ok := strings.Cut(string(source), "<<'SQL'\n")
	if !ok {
		t.Fatal("init script SQL heredoc is missing")
	}
	sql, ok = strings.CutSuffix(sql, "SQL\n")
	if !ok {
		t.Fatal("init script SQL heredoc terminator is missing")
	}
	assertRolePasswordActivityHidden(t, dsn, sql, []string{
		"POSTGRES_MIGRATE_PASSWORD=migrate-activity-marker-'quoted space",
		"POSTGRES_APP_PASSWORD=app-activity-marker-'quoted space",
	}, 4, "migrate-activity-marker-", "app-activity-marker-")
}

func TestRoleScriptsDisableStatementStatistics(t *testing.T) {
	dsn := roleTestCluster(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var available bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_available_extensions
		WHERE name = 'pg_stat_statements')`).Scan(&available); err != nil {
		t.Fatal(err)
	}
	for _, preloaded := range []bool{false, true} {
		name := "placeholder"
		if preloaded {
			name = "preloaded"
		}
		t.Run(name, func(t *testing.T) {
			target := dsn
			if preloaded {
				if !available {
					t.Skip("pg_stat_statements is not available in this PostgreSQL installation")
				}
				target = roleTestCluster(t, "-c shared_preload_libraries=pg_stat_statements",
					"-c pg_stat_statements.track=all")
			}
			conn, err := pgx.Connect(ctx, target)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close(ctx)
			if preloaded {
				if _, err := conn.Exec(ctx, `CREATE EXTENSION pg_stat_statements;
					SELECT 709123 AS role_statistics_probe`); err != nil {
					t.Fatal(err)
				}
				var tracked bool
				if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_stat_statements
					WHERE query LIKE $1)`, "%AS role_statistics_probe%").Scan(&tracked); err != nil || !tracked {
					t.Fatalf("statement statistics positive control: tracked=%v err=%v", tracked, err)
				}
			}
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, `SELECT set_config('pg_stat_statements.track', 'none', true),
				set_config('pg_stat_statements.track_utility', 'off', true)`); err != nil {
				t.Fatal(err)
			}
			var track, utility string
			if err := tx.QueryRow(ctx, `SELECT current_setting('pg_stat_statements.track', true),
				current_setting('pg_stat_statements.track_utility', true)`).Scan(&track, &utility); err != nil {
				t.Fatal(err)
			}
			if track != "none" || utility != "off" {
				t.Fatalf("statement statistics settings: track=%q utility=%q", track, utility)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			const marker = "role-statement-statistics-marker-'quoted space"
			if out, err := runInitRolesScript(t, target, marker, marker); err != nil {
				t.Fatalf("init roles: %v\n%s", err, out)
			}
			migrateSchema(t, target)
			for _, role := range []struct {
				file string
				env  string
			}{
				{"02-identity-role.sql", "IDENTITY_ROLE_PASSWORD"},
				{"03-engine-worker-role.sql", "WORKER_ROLE_PASSWORD"},
			} {
				script := filepath.Join("..", "..", "..", "deploy", "postgres-init", role.file)
				if out, err := runRoleScript(t, target, script, []string{role.env + "=" + marker}, nil); err != nil {
					t.Fatalf("%s: %v\n%s", role.file, err, out)
				}
			}
			if preloaded {
				for _, pattern := range []string{"%role-statement-statistics-marker-%", "%password_nonempty%"} {
					var count int
					if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_stat_statements
						WHERE query LIKE $1`, pattern).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						t.Errorf("statement statistics retained %d password-bearing statements for pattern %q", count, pattern)
					}
				}
			}
		})
	}
}

// A caller's echo settings (-e, -v ECHO=queries, ECHO_HIDDEN) must not print
// the interpolated password SQL to the operator's terminal.
func TestRoleScriptsIgnoreCallerEcho(t *testing.T) {
	dsn := roleTestCluster(t)
	migrateSchema(t, dsn)
	const marker = "role-echo-marker-'quoted space"
	for _, role := range []struct {
		file string
		env  string
	}{
		{"02-identity-role.sql", "IDENTITY_ROLE_PASSWORD"},
		{"03-engine-worker-role.sql", "WORKER_ROLE_PASSWORD"},
	} {
		script := filepath.Join("..", "..", "..", "deploy", "postgres-init", role.file)
		out, err := runRoleScript(t, dsn, script, []string{role.env + "=" + marker}, []string{"ECHO=queries", "ECHO_HIDDEN=on"})
		if err != nil {
			t.Fatalf("%s: %v\n%s", role.file, err, out)
		}
		if strings.Contains(string(out), "role-echo-marker-") {
			t.Fatalf("%s echoed the password:\n%s", role.file, out)
		}
	}
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
	applyRoleScript(t, adminDSN, script, []string{"WORKER_ROLE_PASSWORD=" + workerPassword})

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

// workerRoleScript is the deploy script under test for this file.
func workerRoleScript(t *testing.T) string {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "deploy", "postgres-init", "03-engine-worker-role.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return script
}

// TestWorkerRoleScriptSkipsWithoutPassword pins the operator contract: the
// script must be re-runnable in the initdb hook (before an operator supplies
// the password) without creating a role or failing the directory. The env is
// minimal so an inherited WORKER_ROLE_PASSWORD cannot leak in.
func TestWorkerRoleScriptSkipsWithoutPassword(t *testing.T) {
	adminDSN := roleTestCluster(t)
	t.Setenv("WORKER_ROLE_PASSWORD", "inherited-password-must-not-reach-psql")
	t.Setenv("IDENTITY_ROLE_PASSWORD", "inherited-identity-must-not-reach-psql")
	out, err := runRoleScript(t, adminDSN, workerRoleScript(t), nil, nil)
	if err != nil {
		t.Fatalf("unprovisioned role script failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "skipping") {
		t.Fatalf("expected the skip notice, got: %s", out)
	}
}

// TestWorkerRoleScriptReadsPasswordFromEnv runs the script the way the
// operator docs describe — password in WORKER_ROLE_PASSWORD via \getenv, not
// on the psql command line — and proves the login it creates works.
func TestWorkerRoleScriptReadsPasswordFromEnv(t *testing.T) {
	adminDSN := roleTestCluster(t)
	db, err := migrate.OpenDB(adminDSN)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()

	_, logPath := setupLoggingProbe(t, adminDSN)
	const workerPassword = "worker-env-marker-'quoted space-4c2b1a"
	applyRoleScript(t, adminDSN, workerRoleScript(t),
		[]string{"WORKER_ROLE_PASSWORD=" + workerPassword})
	out, err := runRoleScript(t, adminDSN, workerRoleScript(t), nil, nil)
	if err != nil {
		t.Fatalf("existing worker grants without password: %v\n%s", err, out)
	}
	t.Cleanup(func() { dropRole(t, adminDSN, "noverse_engine_worker") })

	ctx := context.Background()
	worker, err := pgx.Connect(ctx, workerDSN(t, adminDSN, workerPassword))
	if err != nil {
		t.Fatalf("worker login with the env-passed password: %v", err)
	}
	defer worker.Close(ctx)
	var n int
	if err := worker.QueryRow(ctx, "SELECT 1").Scan(&n); err != nil || n != 1 {
		t.Fatalf("worker query after env provisioning: n=%d err=%v", n, err)
	}
	assertServerLogLacks(t, logPath, "worker-env-marker-")
}

// TestWorkerRoleScriptRefusesEmptyPassword: a set-but-empty WORKER_ROLE_
// PASSWORD must be refused, never create a passwordless login.
func TestWorkerRoleScriptRefusesEmptyPassword(t *testing.T) {
	adminDSN := roleTestCluster(t)
	out, err := runRoleScript(t, adminDSN, workerRoleScript(t), []string{"WORKER_ROLE_PASSWORD="}, nil)
	if err == nil || !strings.Contains(string(out), "rỗng") {
		t.Fatalf("expected empty-password refusal: %v\n%s", err, out)
	}
	if roleExists(t, adminDSN, "noverse_engine_worker") {
		t.Fatal("empty password created noverse_engine_worker")
	}
}

// TestWorkerRoleScriptNonSuperuserNeverSendsPassword runs the script as a
// non-superuser CREATEROLE role whose statements are all logged. The run must
// stop before any password-bearing statement is sent: the role is not
// created, psql names the refusal, and the marker password never reaches the
// server log.
func TestWorkerRoleScriptNonSuperuserNeverSendsPassword(t *testing.T) {
	adminDSN := roleTestCluster(t)
	probeDSN, logPath := setupLoggingProbe(t, adminDSN)
	dropRole(t, adminDSN, "noverse_engine_worker")
	t.Cleanup(func() { dropRole(t, adminDSN, "noverse_engine_worker") })

	const marker = "worker-marker-7f3e2c-nosuper"
	out, err := runRoleScript(t, probeDSN, workerRoleScript(t),
		[]string{"WORKER_ROLE_PASSWORD=" + marker}, nil)
	if err == nil {
		t.Fatalf("non-superuser run succeeded unexpectedly:\n%s", out)
	}
	if !strings.Contains(string(out), "superuser") {
		t.Fatalf("expected the superuser refusal, got:\n%s", out)
	}
	if roleExists(t, adminDSN, "noverse_engine_worker") {
		t.Fatal("non-superuser run created noverse_engine_worker")
	}
	assertServerLogLacks(t, logPath, marker)
}

// initRolesScript is the docker-entrypoint init script under test.
func initRolesScript(t *testing.T) string {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "deploy", "postgres-init", "01-roles.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return script
}

// runInitRolesScript runs 01-roles.sh the way docker-entrypoint-initdb.d does:
// POSTGRES_* in a minimal environment, psql reached via PGHOST/PGPORT derived
// from the admin DSN.
func runInitRolesScript(t *testing.T, dsn, migratePassword, appPassword string) ([]byte, error) {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN for init script: %v", err)
	}
	host := u.Hostname()
	if socket := u.Query().Get("host"); socket != "" {
		host = socket
	}
	password, _ := u.User.Password()
	env := append(minimalPsqlEnv(),
		"POSTGRES_USER="+u.User.Username(),
		"POSTGRES_DB="+strings.TrimPrefix(u.Path, "/"),
		"POSTGRES_MIGRATE_PASSWORD="+migratePassword,
		"POSTGRES_APP_PASSWORD="+appPassword,
		"PGHOST="+host,
		"PGPORT="+u.Port(),
	)
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	cmd := exec.Command("sh", initRolesScript(t))
	cmd.Env = env
	return cmd.CombinedOutput()
}

// TestInitRolesScriptRefusesEmptyPassword: the shell refuses unset and empty
// POSTGRES_*_PASSWORD before psql ever runs.
func TestInitRolesScriptRefusesEmptyPassword(t *testing.T) {
	adminDSN := roleTestCluster(t)
	for _, passwords := range [][2]string{{"", "any-app-pass"}, {"any-migrate-pass", ""}} {
		out, err := runInitRolesScript(t, adminDSN, passwords[0], passwords[1])
		if err == nil || !strings.Contains(string(out), "rỗng") {
			t.Fatalf("expected the shell refusal: %v\n%s", err, out)
		}
		for _, role := range []string{"noverse_migrate", "noverse_app"} {
			if roleExists(t, adminDSN, role) {
				t.Fatalf("empty password created %s", role)
			}
		}
	}
}

func TestInitRolesScriptKeepsPasswordsOutOfServerLog(t *testing.T) {
	dsn := roleTestCluster(t)
	_, logPath := setupLoggingProbe(t, dsn)
	const migratePassword = "init-migrate-marker-'quoted space-8d1f"
	const appPassword = "init-app-marker-'quoted space-3e7a"
	out, err := runInitRolesScript(t, dsn, migratePassword, appPassword)
	if err != nil {
		t.Fatalf("init roles script: %v\n%s", err, out)
	}
	for role, password := range map[string]string{
		"noverse_migrate": migratePassword, "noverse_app": appPassword,
	} {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		u.User = url.UserPassword(role, password)
		conn, err := pgx.Connect(context.Background(), u.String())
		if err != nil {
			t.Fatalf("%s login: %v", role, err)
		}
		if err := conn.Ping(context.Background()); err != nil {
			t.Fatal(err)
		}
		conn.Close(context.Background())
		assertServerLogLacks(t, logPath, strings.SplitN(password, "'", 2)[0])
	}
}

func TestInitRolesScriptNonSuperuserNeverSendsPassword(t *testing.T) {
	dsn := roleTestCluster(t)
	probeDSN, logPath := setupLoggingProbe(t, dsn)
	const marker = "init-marker-7f3e2c-nosuper"
	out, err := runInitRolesScript(t, probeDSN, marker, marker)
	if err == nil || !strings.Contains(string(out), "superuser") {
		t.Fatalf("expected superuser refusal before password SQL: %v\n%s", err, out)
	}
	for _, role := range []string{"noverse_migrate", "noverse_app"} {
		if roleExists(t, dsn, role) {
			t.Fatalf("non-superuser created %s", role)
		}
	}
	assertServerLogLacks(t, logPath, marker)
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

// setupGrantOwner models a migration principal that owns the database, schema,
// and tables and administers an already-created runtime login. It is not a
// superuser and cannot change the logging settings used for password creation.
func setupGrantOwner(t *testing.T, dsn, runtimeRole string) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	runtimeIdent := pgx.Identifier{runtimeRole}.Sanitize()
	if _, err := admin.Exec(ctx, `CREATE ROLE noverse_migrate LOGIN CREATEROLE PASSWORD 'grant-owner-test-only';
		ALTER DATABASE postgres OWNER TO noverse_migrate;
		ALTER SCHEMA public OWNER TO noverse_migrate;
		CREATE ROLE `+runtimeIdent+` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
		GRANT `+runtimeIdent+` TO noverse_migrate WITH ADMIN TRUE, INHERIT FALSE, SET FALSE`); err != nil {
		t.Fatalf("provision grant owner: %v", err)
	}
	rows, err := admin.Query(ctx, `SELECT format('ALTER TABLE %I.%I OWNER TO noverse_migrate',
		n.nspname, c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')`)
	if err != nil {
		t.Fatal(err)
	}
	var statements []string
	for rows.Next() {
		var statement string
		if err := rows.Scan(&statement); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		statements = append(statements, statement)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, statement := range statements {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatalf("transfer migration table ownership: %v", err)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("noverse_migrate", "grant-owner-test-only")
	return u.String()
}

func TestWorkerRoleScriptOwnerSyncsExistingRoleWithoutPassword(t *testing.T) {
	dsn := roleTestCluster(t)
	migrateSchema(t, dsn)
	ownerDSN := setupGrantOwner(t, dsn, "noverse_engine_worker")
	admin, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	if _, err := admin.Exec(context.Background(), `GRANT SELECT ON public.users TO noverse_engine_worker`); err != nil {
		t.Fatal(err)
	}
	out, err := runRoleScript(t, ownerDSN, workerRoleScript(t), nil, nil)
	if err != nil || strings.Contains(string(out), "skipping") {
		t.Fatalf("non-superuser owner grant sync: %v\n%s", err, out)
	}
	var projects, users bool
	if err := admin.QueryRow(context.Background(), `SELECT
		has_table_privilege('noverse_engine_worker', 'public.projects', 'SELECT'),
		has_table_privilege('noverse_engine_worker', 'public.users', 'SELECT')`).Scan(&projects, &users); err != nil {
		t.Fatal(err)
	}
	if !projects || users {
		t.Fatalf("worker grant sync: projects SELECT=%v users SELECT=%v", projects, users)
	}
}

func TestInitRolesSQLRefusesWithoutCallerErrorStop(t *testing.T) {
	dsn := roleTestCluster(t)
	probeDSN, logPath := setupLoggingProbe(t, dsn)
	source, err := os.ReadFile(initRolesScript(t))
	if err != nil {
		t.Fatal(err)
	}
	_, sql, ok := strings.Cut(string(source), "<<'SQL'\n")
	if !ok {
		t.Fatal("init script SQL heredoc is missing")
	}
	sql, ok = strings.CutSuffix(sql, "SQL\n")
	if !ok {
		t.Fatal("init script SQL heredoc terminator is missing")
	}
	script := filepath.Join(t.TempDir(), "roles.sql")
	if err := os.WriteFile(script, []byte(sql), 0600); err != nil {
		t.Fatal(err)
	}
	const marker = "init-error-stop-off-marker-7e1f"
	out, err := runRoleScript(t, probeDSN, script,
		[]string{"POSTGRES_MIGRATE_PASSWORD=" + marker, "POSTGRES_APP_PASSWORD=" + marker},
		[]string{"ON_ERROR_STOP=off", "database_name=postgres"})
	if err == nil || !strings.Contains(string(out), "superuser") {
		t.Fatalf("expected refusal despite caller ON_ERROR_STOP=off: %v\n%s", err, out)
	}
	for _, role := range []string{"noverse_migrate", "noverse_app"} {
		if roleExists(t, dsn, role) {
			t.Fatalf("caller ON_ERROR_STOP=off allowed creating %s", role)
		}
	}
	assertServerLogLacks(t, logPath, marker)
}
