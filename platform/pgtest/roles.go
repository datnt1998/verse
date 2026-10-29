package pgtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

// RoleSpec mô tả cặp principal migrate/app của một app: tên role, advisory
// lock dùng khi tạo role (tránh hai test song song race CREATE ROLE), và
// script init của deploy mà grant phía dưới phản chiếu — chỉ để diễn giải,
// script không được đọc lại ở đây.
type RoleSpec struct {
	MigrateRole   string // role chủ schema, chạy migration DDL
	AppRole       string // role app: chỉ DML
	LockKey1      int32  // hai thành phần của pg_advisory_xact_lock
	LockKey2      int32
	InitScriptRef string // vd. deploy/postgres-init/01-roles.sh — ghi vào comment/grant parity
}

// ProvisionRoles provisions the production owner/app privilege model in an
// isolated schema. The test admin sets the connection's role at startup, so
// every statement (including migration DDL) runs with that principal's rights.
// Standard roles are created only if absent; existing credentials are untouched.
func ProvisionRoles(t *testing.T, spec RoleSpec) (appDSN, maintenanceDSN string) {
	t.Helper()
	if spec.MigrateRole == "" || spec.AppRole == "" {
		t.Fatal("pgtest.ProvisionRoles: MigrateRole và AppRole không được rỗng")
	}
	dsn := Provision(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;
		CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public`); err != nil {
		t.Fatalf("provision production extensions: %v", err)
	}
	_, err = admin.Exec(ctx, `DO $$ BEGIN
		PERFORM pg_advisory_xact_lock($1, $2);
		IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname=$3) THEN
			EXECUTE format('CREATE ROLE %I LOGIN', $3);
		END IF;
		IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname=$4) THEN
			EXECUTE format('CREATE ROLE %I LOGIN', $4);
		END IF;
	END $$;`, spec.LockKey1, spec.LockKey2, spec.MigrateRole, spec.AppRole)
	if err != nil {
		t.Fatalf("provision production roles: %v", err)
	}
	schema := pgx.Identifier{SchemaOf(dsn)}.Sanitize()
	migrateRole := pgx.Identifier{spec.MigrateRole}.Sanitize()
	appRole := pgx.Identifier{spec.AppRole}.Sanitize()
	// Grants phản chiếu script init của app (spec.InitScriptRef); per-table
	// exceptions do migration nhúng của app áp, fixture không tái tạo.
	_, err = admin.Exec(ctx, fmt.Sprintf(`
		ALTER SCHEMA %s OWNER TO %s;
		GRANT USAGE ON SCHEMA %s TO %s;
		ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s
			GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s;
		ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA %s
			GRANT USAGE, SELECT ON SEQUENCES TO %s;`,
		schema, migrateRole, schema, appRole, migrateRole, schema, appRole, migrateRole, schema, appRole))
	if err != nil {
		t.Fatalf("provision production grants: %v", err)
	}
	return dsn + "&role=" + spec.AppRole, dsn + "&role=" + spec.MigrateRole
}
