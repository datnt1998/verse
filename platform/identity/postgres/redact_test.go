package postgres

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgtest"
)

const leakySecret = "leaky-secret-9f8e7d"

// dsnWithPassword injects a fake password into a test DSN. Local auth is
// trust, so the value is never verified; it exists only to prove errors and
// logs never echo credentials.
func dsnWithPassword(t *testing.T, dsn, password string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse test DSN: %v", err)
	}
	u.User = url.UserPassword(u.User.Username(), password)
	return u.String()
}

func assertNoSecret(t *testing.T, what, text string) {
	t.Helper()
	if strings.Contains(text, leakySecret) {
		t.Fatalf("%s leaked the DSN password: %q", what, text)
	}
	if strings.Contains(text, "$argon2id$") {
		t.Fatalf("%s leaked a password hash: %q", what, text)
	}
}

// TestIdentityErrorsAndLogsRedactSecrets injects DSN parse, connect and query
// failures and checks every surfaced error stays credential-free. A database
// failure must never be reported as "unauthenticated": callers would log the
// user out instead of surfacing 503.
func TestIdentityErrorsAndLogsRedactSecrets(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)

	// Driver errors may wrap the normalized config: sanitizeErr strips both
	// URI userinfo and key=value credentials.
	raw := errors.New("failed to connect: postgres://alice:" + leakySecret + "@db.example/verse?sslmode=disable")
	assertNoSecret(t, "sanitizeErr", sanitizeErr(raw).Error())
	pair := errors.New("connect failed password=" + leakySecret)
	assertNoSecret(t, "sanitizeErr key/value", sanitizeErr(pair).Error())

	if _, err := Open(context.Background(), PoolConfig{
		DSN: "postgres://alice:" + leakySecret + "@%",
	}); err == nil {
		t.Fatal("malformed DSN accepted")
	} else {
		assertNoSecret(t, "parse error", err.Error())
	}

	unreachable := "postgres://alice:" + leakySecret + "@127.0.0.1:1/verse?sslmode=disable"
	if _, err := Open(context.Background(), PoolConfig{
		DSN: unreachable, ConnectTimeout: 2 * time.Second,
	}); err == nil {
		t.Fatal("unreachable database accepted")
	} else {
		assertNoSecret(t, "connect error", err.Error())
	}

	// The redaction checks below need a real server and schema. RequireURL
	// fails the acceptance gate (instead of skipping) when it is demanded.
	dsn := dsnWithPassword(t, pgtest.Provision(t), leakySecret)
	pool, err := Open(context.Background(), PoolConfig{DSN: dsn, SkipContractCheck: true})
	if err != nil {
		t.Fatalf("open identity pool: %v", err)
	}
	repo := NewAccounts(pool.Pool(), Options{ProjectAccess: false})

	// Empty schema → relation does not exist. It must surface as a storage
	// error, never as an authentication verdict.
	if _, _, err := repo.Authenticate("0123456789abcdef0123456789abcdef." + strings.Repeat("ab", 32)); err == nil {
		t.Fatal("query against an empty schema succeeded")
	} else {
		assertNoSecret(t, "query error", err.Error())
		if errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("database failure was disguised as unauthenticated: %v", err)
		}
	}

	// A closed pool is a transport failure, not an expired session.
	pool.Close()
	if _, _, err := repo.Authenticate("0123456789abcdef0123456789abcdef." + strings.Repeat("ab", 32)); err == nil {
		t.Fatal("authenticate on a closed pool succeeded")
	} else {
		assertNoSecret(t, "closed-pool error", err.Error())
		if errors.Is(err, identity.ErrUnauthenticated) {
			t.Fatalf("closed pool was disguised as unauthenticated: %v", err)
		}
	}

	logText := logs.String()
	assertNoSecret(t, "logs", logText)
	if strings.Contains(logText, strings.Repeat("ab", 32)) {
		t.Fatalf("logs leaked the session secret: %q", logText)
	}
}
