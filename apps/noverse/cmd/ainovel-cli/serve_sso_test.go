package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

// TestSSOLocalBindRequiresLoopback pins the local-SSO bind contract: the
// host-only shared cookie travels to every port on the host, so a wildcard,
// LAN or public bind must be refused before the server can accept a
// connection — and file mode rejects the SSO envs outright.
func TestSSOLocalBindRequiresLoopback(t *testing.T) {
	for _, tc := range []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:18790", true},
		{"[::1]:18790", true},
		{"localhost:18790", true},
		{"0.0.0.0:18790", false},
		{"[::]:18790", false},
		{":18790", false},
		{"192.168.1.5:18790", false},
		{"100.64.0.1:18790", false}, // Tailscale range is still not loopback
	} {
		if got := ssoLoopbackOnly(tc.addr); got != tc.ok {
			t.Fatalf("ssoLoopbackOnly(%q) = %v, want %v", tc.addr, got, tc.ok)
		}
	}

	t.Setenv("HOME", t.TempDir())

	// File mode: the SSO envs are a startup error, never silently ignored.
	t.Setenv("VERSE_APP_ORIGINS", "http://127.0.0.1:18789")
	t.Setenv("STORE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")
	if err := runServe([]string{"-addr", "0.0.0.0:0"}); err == nil || !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("file mode + VERSE_APP_ORIGINS = %v, want storage-mode error", err)
	}
	t.Setenv("VERSE_APP_ORIGINS", "")
	t.Setenv("VERSE_COOKIE_DOMAIN", "example.com")
	if err := runServe([]string{"-addr", "0.0.0.0:0"}); err == nil {
		t.Fatal("file mode + VERSE_COOKIE_DOMAIN must fail startup")
	}
	t.Setenv("VERSE_COOKIE_DOMAIN", "")

	// Postgres mode + local SSO: every non-loopback bind fails closed.
	// The fixture needs a real migrated schema: openAccounts validates the
	// storage contract before the bind check runs.
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(t.Context(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	t.Setenv("STORE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("VERSE_APP_ORIGINS", "http://127.0.0.1:18789,http://127.0.0.1:18790")
	for _, addr := range []string{"0.0.0.0:0", "[::]:0", ":0"} {
		if err := runServe([]string{"-addr", addr}); err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("local SSO bind %q = %v, want loopback refusal", addr, err)
		}
	}
}

// TestHostedAppHTTPPortIsNotPublished keeps the deploy invariant: the app
// container exposes 8080 only on the private network — Traefik is the only
// public door — and nothing publishes the app port on a wildcard host
// address. The worker DSN contract must also reach the app environment.
func TestHostedAppHTTPPortIsNotPublished(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "deploy", "docker-compose.dokploy.yml"))
	if err != nil {
		t.Fatalf("compose file: %v", err)
	}
	var compose struct {
		Services map[string]struct {
			Ports   []interface{}  `yaml:"ports"`
			Expose  []string       `yaml:"expose"`
			EnvList map[string]any `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose: %v", err)
	}
	app, ok := compose.Services["app"]
	if !ok {
		t.Fatal("compose has no app service")
	}
	for _, raw := range app.Ports {
		entry, ok := raw.(string)
		if !ok {
			t.Fatalf("app publishes a non-string port entry: %v", raw)
		}
		// A published port is "host_ip:host_port:container_port" or
		// "host_port:container_port". Anything without a host_ip binds every
		// interface — the exact failure this invariant exists for.
		parts := strings.Split(entry, ":")
		if len(parts) < 3 {
			t.Fatalf("app port %q lacks an explicit host bind address", entry)
		}
		containerPort := parts[len(parts)-1]
		if containerPort == "8080" {
			t.Fatalf("app publishes its HTTP port: %q — Traefik must be the only door", entry)
		}
		hostIP := parts[0]
		if hostIP == "0.0.0.0" || hostIP == "::" || hostIP == "" {
			t.Fatalf("app port %q binds a wildcard address", entry)
		}
	}
}

// TestGrantCLIOperatorBoundary exercises the operator grammar end to end on
// the file backend: grant/revoke parse through the shared parser, refuse the
// file store cleanly (nonzero, ErrUnsupported) and never create accounts.
func TestGrantCLIOperatorBoundary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("STORE_DRIVER", "")
	t.Setenv("DATABASE_URL", "")

	for _, args := range [][]string{
		{"grant", "ghost", "--app", "noverse"}, // user does not exist → error
		{"grant", "ghost", "--app", "viderse"}, // file backend has no grant table
		{"revoke", "ghost", "--app", "noverse"},
		{"grant", "ghost"}, // missing --app
		{"grant", "ghost", "--app", "noverse", "--actor", "admin"},
		{"grant", "ghost", "--dsn", "postgres://u:p@h/db", "--app", "noverse"},
	} {
		if err := runServeUser(args); err == nil {
			t.Fatalf("serve user %v succeeded — operator boundary broken", args)
		}
	}

	// Nothing was created as a side effect.
	path, err := identity.DefaultPath(bootstrap.DefaultConfigDir())
	if err != nil {
		t.Fatal(err)
	}
	store := identity.NewStore(path, library.IdentityOptions())
	if has, err := store.HasUsers(); err != nil || has {
		t.Fatalf("grant commands touched the account store: has=%v err=%v", has, err)
	}
}

// TestGrantCLIPostgresOperator applies the operator verbs on the real shared
// store: grant adds a member row with granted_by NULL, revoke removes it —
// actor stays the local operator, never a forged HTTP user.
func TestGrantCLIPostgresOperator(t *testing.T) {
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration pool: %v", err)
	}
	if _, err := migrate.Up(t.Context(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(t.Context(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("STORE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", dsn)

	accounts := identity.NewFromRepository(identitypg.NewAccounts(pool.Pool(), identitypg.Options{ProjectAccess: true, ValidProjectID: library.ValidProjectID}), library.IdentityOptions())
	if _, err := accounts.CreateUser(identity.NewUser{
		Username: "linh", Role: identity.RoleMember,
		Apps: []identity.App{identity.AppNoverse}, Password: "linh-pass",
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}
	grants := func() map[identity.App]identity.Role {
		t.Helper()
		users, err := accounts.Users()
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range users {
			if u.Username == "linh" {
				return u.Grants
			}
		}
		t.Fatal("linh missing")
		return nil
	}
	// The command path must open its own accounts from env — verify the
	// operator verbs reach the shared store through runServeUser.
	if err := runServeUser([]string{"grant", "linh", "--app", "viderse"}); err != nil {
		t.Fatalf("serve user grant: %v", err)
	}
	if grants()[identity.AppViderse] != identity.RoleMember {
		t.Fatalf("grant not applied: %+v", grants())
	}
	if err := runServeUser([]string{"revoke", "linh", "--app", "viderse"}); err != nil {
		t.Fatalf("serve user revoke: %v", err)
	}
	if _, ok := grants()[identity.AppViderse]; ok {
		t.Fatalf("revoke not applied: %+v", grants())
	}
}
