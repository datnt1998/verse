package pgtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

// ProvisionRoles provisions the production owner/app privilege model in an
// isolated schema. The test admin sets the connection's role at startup, so
// every statement (including migration DDL) runs with that principal's rights.
// Standard roles are created only if absent; existing credentials are untouched.
func ProvisionRoles(t *testing.T) (appDSN, maintenanceDSN string) {
	t.Helper()
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
		PERFORM pg_advisory_xact_lock(2710, 11);
		IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='noverse_migrate') THEN
			CREATE ROLE noverse_migrate LOGIN;
		END IF;
		IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='noverse_app') THEN
			CREATE ROLE noverse_app LOGIN;
		END IF;
	END $$;`)
	if err != nil {
		t.Fatalf("provision production roles: %v", err)
	}
	schema := pgx.Identifier{SchemaOf(dsn)}.Sanitize()
	// Match deploy/postgres-init/01-roles.sh; per-table exceptions are applied
	// by the actual embedded migrations, never re-created in this fixture.
	_, err = admin.Exec(ctx, fmt.Sprintf(`
		ALTER SCHEMA %s OWNER TO noverse_migrate;
		GRANT USAGE ON SCHEMA %s TO noverse_app;
		ALTER DEFAULT PRIVILEGES FOR ROLE noverse_migrate IN SCHEMA %s
			GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO noverse_app;
		ALTER DEFAULT PRIVILEGES FOR ROLE noverse_migrate IN SCHEMA %s
			GRANT USAGE, SELECT ON SEQUENCES TO noverse_app;`, schema, schema, schema, schema))
	if err != nil {
		t.Fatalf("provision production grants: %v", err)
	}
	return dsn + "&role=noverse_app", dsn + "&role=noverse_migrate"
}
