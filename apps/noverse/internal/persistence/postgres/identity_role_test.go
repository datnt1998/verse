package postgres_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

const identityRolePassword = "identity-role-test-only"

func roleDSN(t *testing.T, adminDSN, user string) string {
	t.Helper()
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	u.User = url.UserPassword(user, identityRolePassword)
	return u.String()
}

// applyIdentityRoleScript provisions the real viderse_identity role by running
// the deployment script through psql, exactly as the operator does. The schema
// is passed as a variable so the grants land in the throwaway test schema.
func applyIdentityRoleScript(t *testing.T, adminDSN, schema string) {
	t.Helper()
	bin, err := exec.LookPath("psql")
	if err != nil {
		t.Fatalf("psql is required to apply the identity role script: %v", err)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..",
		"apps", "noverse", "deploy", "postgres-init", "02-identity-role.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// libpq rejects pgx's extra search_path runtime parameter; the script
	// schema-qualifies every table, so the parameter is not needed.
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN for psql: %v", err)
	}
	query := u.Query()
	query.Del("search_path")
	u.RawQuery = query.Encode()
	cmd := exec.Command(bin, "-X", u.String(),
		"-v", "ON_ERROR_STOP=1",
		"-v", "identity_schema="+schema,
		"-f", script)
	cmd.Env = append(minimalPsqlEnv(), "IDENTITY_ROLE_PASSWORD="+identityRolePassword)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply identity role script: %v\n%s", err, out)
	}
	if strings.Contains(string(out), identityRolePassword) {
		t.Fatalf("psql echoed the role password:\n%s", out)
	}
}

// TestViderseIdentityRoleCannotReadNoverseOrPromoteUser runs the account, grant
// and password flows through the real least-privilege role and then proves the
// negatives directly in SQL: no project/story/audit reads, no role/status
// promotion, no temporary-password issuing, no DDL, no SET ROLE.
func TestViderseIdentityRoleCannotReadNoverseOrPromoteUser(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	schema := pgtest.SchemaOf(dsn)
	adminPool := openPool(t, dsn)
	adminStore := identity.NewFromRepository(identityRepo(adminPool), library.IdentityOptions())

	if _, err := adminStore.CreateUser(identity.NewUser{Username: "root", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := adminStore.CreateUser(identity.NewUser{Username: "mem", Role: identity.RoleMember,
		Projects: map[string]identity.ProjectRole{"proj": identity.ProjectOwner}, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	if _, err := adminStore.CreateUser(identity.NewUser{Username: "moi", Role: identity.RoleMember,
		Password: "temp-pass-111", MustChangePassword: true}); err != nil {
		t.Fatal(err)
	}

	applyIdentityRoleScript(t, dsn, schema)
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return
		}
		defer conn.Close(ctx)
		_, _ = conn.Exec(ctx, "DROP OWNED BY viderse_identity")
		_, _ = conn.Exec(ctx, "DROP ROLE IF EXISTS viderse_identity")
	})

	// The role must not inherit anyone, and must not be a member of any role.
	var super, createDB, createRole, inherit, canLogin bool
	if err := adminPool.Pool().QueryRow(context.Background(), `
		SELECT rolsuper, rolcreatedb, rolcreaterole, rolinherit, rolcanlogin
		FROM pg_roles WHERE rolname = 'viderse_identity'`).Scan(
		&super, &createDB, &createRole, &inherit, &canLogin); err != nil {
		t.Fatal(err)
	}
	if super || createDB || createRole || inherit || !canLogin {
		t.Fatalf("role attributes = super:%v createdb:%v createrole:%v inherit:%v login:%v",
			super, createDB, createRole, inherit, canLogin)
	}
	var memberships int
	if err := adminPool.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM pg_auth_members WHERE member = 'viderse_identity'::regrole`).Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if memberships != 0 {
		t.Fatalf("identity role inherits %d role memberships", memberships)
	}

	rolePool, err := identitypg.Open(context.Background(), identitypg.PoolConfig{
		DSN: roleDSN(t, dsn, "viderse_identity"),
	})
	if err != nil {
		t.Fatalf("open the real identity role: %v", err)
	}
	defer rolePool.Close()
	repo := identitypg.NewAccounts(rolePool.Pool(), identitypg.Options{ProjectAccess: false})
	s := identity.NewFromRepository(repo, library.IdentityOptions())

	// Account flow works, project projection is empty even though the account
	// owns a project, and grants load from the grant table.
	ns, err := s.Login("mem", "mat-khau-12345", "Mac", "")
	if err != nil {
		t.Fatalf("login under the identity role: %v", err)
	}
	u, sess, err := s.Authenticate(ns.Cookie)
	if err != nil {
		t.Fatalf("authenticate under the identity role: %v", err)
	}
	if len(u.Projects) != 0 {
		t.Fatalf("project-less projection leaked projects: %+v", u.Projects)
	}
	if !sess.Current {
		t.Fatal("session view lost its current flag")
	}
	if u.Grants[identity.AppNoverse] != identity.RoleMember {
		t.Fatalf("Noverse grant missing: %+v", u.Grants)
	}

	// Password change: the column-minimal UPDATE and audit INSERT work, and
	// the temporary password flag is cleared.
	temp := loginOrFatal(t, s, "moi", "temp-pass-111")
	if err := s.ChangePassword("moi", "temp-pass-111", "mat-khau-moi-999"); err != nil {
		t.Fatalf("password change under the identity role: %v", err)
	}
	if _, _, err := s.Authenticate(temp.Cookie); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("password change did not revoke the session: %v", err)
	}
	fresh, err := s.Login("moi", "mat-khau-moi-999", "", "")
	if err != nil {
		t.Fatalf("login after the change: %v", err)
	}
	if fresh.User.MustChangePassword {
		t.Fatal("must_change_password was not cleared with the column grant")
	}

	// Grant writes work through the trusted operator entry (granted_by NULL,
	// system audit) and through the HTTP-path entry with a global admin actor.
	if err := s.OperatorSetAppGrant("mem", identity.AppViderse, identity.RoleMember); err != nil {
		t.Fatalf("operator grant under the identity role: %v", err)
	}
	if grants, err := s.AppGrants("mem"); err != nil || grants[identity.AppViderse] != identity.RoleMember {
		t.Fatalf("grant read under the identity role: %+v %v", grants, err)
	}
	if err := s.RevokeAppGrant("mem", identity.AppViderse,
		&identity.AuditActor{Username: "root"}, "thu hồi"); err != nil {
		t.Fatalf("admin revoke under the identity role: %v", err)
	}
	if err := s.SetAppGrant("mem", identity.AppViderse, identity.RoleMember,
		&identity.AuditActor{Username: "moi"}, ""); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("non-admin actor grant = %v, want ErrForbidden", err)
	}
	if err := s.OperatorSetAppGrant("mem", identity.AppViderse, identity.RoleMember); err != nil {
		t.Fatalf("re-grant before the negatives: %v", err)
	}

	// Project, lifecycle and audit surfaces are rejected before SQL under a
	// project-less backend.
	if err := s.DeleteUser("moi"); !errors.Is(err, identity.ErrUnsupported) {
		t.Fatalf("DeleteUser under the identity role = %v", err)
	}
	role := identity.RoleAdmin
	if _, err := s.UpdateUser("mem", identity.UserPatch{Role: &role}); !errors.Is(err, identity.ErrUnsupported) {
		t.Fatalf("UpdateUser(role) under the identity role = %v", err)
	}
	if _, err := s.Audit(identity.AuditQuery{}); !errors.Is(err, identity.ErrUnsupported) {
		t.Fatalf("Audit under the identity role = %v", err)
	}

	// Direct SQL: Postgres itself must deny the Noverse data.
	ctx := context.Background()
	queries := map[string]string{
		"projects":        `SELECT count(*) FROM projects`,
		"project_members": `SELECT count(*) FROM project_members`,
		"stories":         `SELECT count(*) FROM stories`,
		"audit_logs":      `SELECT count(*) FROM audit_logs`,
		"promote role":    `UPDATE users SET global_role = 'admin' WHERE username = 'mem'`,
		"rename column":   `UPDATE users SET display_name = display_name WHERE username = 'mem'`,
		"bulk assignment": `UPDATE users SET (display_name, global_role) = (display_name, global_role) WHERE username = 'mem'`,
		"noverse meta":    `SELECT count(*) FROM noverse_meta`,
		"goose history":   `SELECT count(*) FROM goose_db_version`,
		"lock account":    `UPDATE users SET status = 'locked' WHERE username = 'mem'`,
		"soft delete":     `UPDATE users SET deleted_at = now() WHERE username = 'mem'`,
		"issue temp": `INSERT INTO temporary_passwords (id, user_id, password_hash, expires_at)
			VALUES ('00000000-0000-0000-0000-0000000009f9',
				(SELECT id FROM users WHERE username = 'mem'),
				'$argon2id$test', now() + interval '1 hour')`,
		"ddl":      `CREATE TABLE identity_role_ddl_probe (id int)`,
		"set role": `SET ROLE noverse_migrate`,
	}
	for what, q := range queries {
		if _, err := rolePool.Pool().Exec(ctx, q); err == nil {
			t.Fatalf("%s was allowed by the identity role", what)
		}
	}

	// INSERT on users exists only for first-run setup: once the store holds an
	// account the identity role cannot mint another one — not directly, and not
	// by shadowing "users" with a temporary table on its own connection.
	mint := `INSERT INTO ` + pgx.Identifier{schema, "users"}.Sanitize() + ` (id, username, display_name, global_role, status)
		VALUES ('00000000-0000-0000-0000-0000000009fa', 'minted', '', 'admin', 'active')`
	assertDenied := func(what string, err error) {
		t.Helper()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("%s under the identity role = %v, want insufficient_privilege", what, err)
		}
	}
	_, err = rolePool.Pool().Exec(ctx, mint)
	assertDenied("mint admin", err)
	conn, err := rolePool.Pool().Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire identity role connection: %v", err)
	}
	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE users (id uuid, deleted_at timestamptz)`); err != nil {
		t.Fatalf("temp shadow table: %v", err)
	}
	_, err = conn.Exec(ctx, mint)
	assertDenied("mint admin behind a temp shadow", err)
	if _, err := conn.Exec(ctx, `DROP TABLE pg_temp.users`); err != nil {
		t.Fatalf("drop temp shadow table: %v", err)
	}
	conn.Release()

	// Privilege catalog: the script's exact grants, with nothing external
	// reintroducing rights through PUBLIC/ownership.
	checks := []struct {
		what   string
		query  string
		args   []any
		expect bool
	}{
		{"users SELECT", `SELECT has_table_privilege('viderse_identity', $1, 'SELECT')`, []any{schema + ".users"}, true},
		{"users INSERT", `SELECT has_table_privilege('viderse_identity', $1, 'INSERT')`, []any{schema + ".users"}, true},
		{"users display_name UPDATE", `SELECT has_column_privilege('viderse_identity', $1, 'display_name', 'UPDATE')`, []any{schema + ".users"}, false},
		{"users global_role UPDATE", `SELECT has_column_privilege('viderse_identity', $1, 'global_role', 'UPDATE')`, []any{schema + ".users"}, false},
		{"users status UPDATE", `SELECT has_column_privilege('viderse_identity', $1, 'status', 'UPDATE')`, []any{schema + ".users"}, false},
		{"users deleted_at UPDATE", `SELECT has_column_privilege('viderse_identity', $1, 'deleted_at', 'UPDATE')`, []any{schema + ".users"}, false},
		{"users must_change UPDATE", `SELECT has_column_privilege('viderse_identity', $1, 'must_change_password', 'UPDATE')`, []any{schema + ".users"}, true},
		{"projects SELECT", `SELECT has_table_privilege('viderse_identity', $1, 'SELECT')`, []any{schema + ".projects"}, false},
		{"stories SELECT", `SELECT has_table_privilege('viderse_identity', $1, 'SELECT')`, []any{schema + ".stories"}, false},
		{"members SELECT", `SELECT has_table_privilege('viderse_identity', $1, 'SELECT')`, []any{schema + ".project_members"}, false},
		{"audit SELECT", `SELECT has_table_privilege('viderse_identity', $1, 'SELECT')`, []any{schema + ".audit_logs"}, false},
		{"audit action INSERT", `SELECT has_column_privilege('viderse_identity', $1, 'action', 'INSERT')`, []any{schema + ".audit_logs"}, true},
		{"audit sequence INSERT", `SELECT has_column_privilege('viderse_identity', $1, 'sequence', 'INSERT')`, []any{schema + ".audit_logs"}, false},
		{"audit sequence SELECT", `SELECT has_column_privilege('viderse_identity', $1, 'sequence', 'SELECT')`, []any{schema + ".audit_logs"}, false},
		{"temp INSERT", `SELECT has_table_privilege('viderse_identity', $1, 'INSERT')`, []any{schema + ".temporary_passwords"}, false},
		{"auth lock UPDATE", `SELECT has_table_privilege('viderse_identity', $1, 'UPDATE')`, []any{schema + ".auth_invariant_lock"}, true},
		{"schema CREATE", `SELECT has_schema_privilege('viderse_identity', $1, 'CREATE')`, []any{schema}, false},
		{"schema USAGE", `SELECT has_schema_privilege('viderse_identity', $1, 'USAGE')`, []any{schema}, true},
		{"database CONNECT", `SELECT has_database_privilege('viderse_identity', current_database(), 'CONNECT')`, nil, true},
		{"PUBLIC projects", `SELECT has_table_privilege('public', $1, 'SELECT')`, []any{schema + ".projects"}, false},
		{"PUBLIC audit", `SELECT has_table_privilege('public', $1, 'SELECT')`, []any{schema + ".audit_logs"}, false},
		{"noverse_meta", `SELECT has_table_privilege('viderse_identity', $1, 'SELECT')`, []any{schema + ".noverse_meta"}, false},
	}
	for _, c := range checks {
		var got bool
		if err := adminPool.Pool().QueryRow(ctx, c.query, c.args...).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got != c.expect {
			t.Fatalf("%s privilege = %v, want %v", c.what, got, c.expect)
		}
	}

	// The identity role scoped the grant table, not the whole database: the
	// migrator role (if present in this environment) is unreachable from it.
	var migrateMember bool
	_ = adminPool.Pool().QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'noverse_migrate')`).Scan(&migrateMember)
	if migrateMember {
		if _, err := rolePool.Pool().Exec(ctx, `SET ROLE noverse_migrate`); err == nil {
			t.Fatal("identity role can SET ROLE noverse_migrate")
		}
	}
}

func TestIdentityRoleScriptReadsPasswordFromEnv(t *testing.T) {
	dsn := roleTestCluster(t)
	migrateSchema(t, dsn)
	_, logPath := setupLoggingProbe(t, dsn)
	const password = "identity-env-marker-'quoted space-8f3a"
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "02-identity-role.sql")
	out, err := runRoleScript(t, dsn, script,
		[]string{"IDENTITY_ROLE_PASSWORD=" + password}, nil)
	if err != nil {
		t.Fatalf("identity env provisioning: %v\n%s", err, out)
	}
	out, err = runRoleScript(t, dsn, script, nil, nil)
	if err != nil {
		t.Fatalf("existing identity grants without password: %v\n%s", err, out)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("viderse_identity", password)
	conn, err := pgx.Connect(context.Background(), u.String())
	if err != nil {
		t.Fatalf("identity login with env password: %v", err)
	}
	defer conn.Close(context.Background())
	if err := conn.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertServerLogLacks(t, logPath, "identity-env-marker-")
}

func TestIdentityRoleScriptKeepsPasswordOutOfActivity(t *testing.T) {
	dsn := roleTestCluster(t)
	migrateSchema(t, dsn)
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "02-identity-role.sql")
	sql, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	assertRolePasswordActivityHidden(t, dsn, string(sql),
		[]string{"IDENTITY_ROLE_PASSWORD=identity-activity-marker-'quoted space"},
		2, "identity-activity-marker-")
}

func TestIdentityRoleScriptRefusesEmptyPassword(t *testing.T) {
	dsn := roleTestCluster(t)
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "02-identity-role.sql")
	out, err := runRoleScript(t, dsn, script, []string{"IDENTITY_ROLE_PASSWORD="}, nil)
	if err == nil || !strings.Contains(string(out), "rỗng") {
		t.Fatalf("expected empty-password refusal: %v\n%s", err, out)
	}
	if roleExists(t, dsn, "viderse_identity") {
		t.Fatal("empty password created viderse_identity")
	}
}

func TestIdentityRoleScriptNonSuperuserNeverSendsPassword(t *testing.T) {
	dsn := roleTestCluster(t)
	probeDSN, logPath := setupLoggingProbe(t, dsn)
	const marker = "identity-marker-7f3e2c-nosuper"
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "02-identity-role.sql")
	out, err := runRoleScript(t, probeDSN, script, []string{"IDENTITY_ROLE_PASSWORD=" + marker}, nil)
	if err == nil || !strings.Contains(string(out), "superuser") {
		t.Fatalf("expected superuser refusal before password SQL: %v\n%s", err, out)
	}
	if roleExists(t, dsn, "viderse_identity") {
		t.Fatal("non-superuser created viderse_identity")
	}
	assertServerLogLacks(t, logPath, marker)
}

func TestIdentityRoleScriptSkipsWithoutInheritedPassword(t *testing.T) {
	dsn := roleTestCluster(t)
	t.Setenv("IDENTITY_ROLE_PASSWORD", "inherited-password-must-not-reach-psql")
	t.Setenv("WORKER_ROLE_PASSWORD", "inherited-worker-must-not-reach-psql")
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "02-identity-role.sql")
	out, err := runRoleScript(t, dsn, script, nil, nil)
	if err != nil || !strings.Contains(string(out), "skipping") {
		t.Fatalf("expected skip with isolated environment: %v\n%s", err, out)
	}
}

func TestIdentityRoleScriptOwnerSyncsExistingRoleWithoutPassword(t *testing.T) {
	dsn := roleTestCluster(t)
	migrateSchema(t, dsn)
	ownerDSN := setupGrantOwner(t, dsn, "viderse_identity")
	admin, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	if _, err := admin.Exec(context.Background(), `GRANT SELECT ON public.projects TO viderse_identity`); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join("..", "..", "..", "deploy", "postgres-init", "02-identity-role.sql")
	out, err := runRoleScript(t, ownerDSN, script, nil, nil)
	if err != nil || strings.Contains(string(out), "skipping") {
		t.Fatalf("non-superuser owner grant sync: %v\n%s", err, out)
	}
	var users, projects bool
	if err := admin.QueryRow(context.Background(), `SELECT
		has_table_privilege('viderse_identity', 'public.users', 'SELECT'),
		has_table_privilege('viderse_identity', 'public.projects', 'SELECT')`).Scan(&users, &projects); err != nil {
		t.Fatal(err)
	}
	if !users || projects {
		t.Fatalf("identity grant sync: users SELECT=%v projects SELECT=%v", users, projects)
	}
}
