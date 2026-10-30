// Package pgtest provisions isolated schemas for Postgres integration tests.
//
// Every test that needs a real database gets its own schema on the database
// named by TEST_DATABASE_URL, so suites never touch shared data and can run
// in parallel. No default DSN exists: without TEST_DATABASE_URL the test is
// skipped, unless REQUIRE_POSTGRES_TEST=1, in which case the test fails so
// acceptance jobs cannot silently pass on a skipped suite.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// RequireURL returns TEST_DATABASE_URL or stops the test. It never fails when
// a DSN exists but the server is down — that surfaces as a normal error so the
// acceptance gate sees a red, not a skip.
func RequireURL(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		if os.Getenv("REQUIRE_POSTGRES_TEST") == "1" {
			t.Fatal("TEST_DATABASE_URL is required for this acceptance run")
		}
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	return dsn
}

var schemaNameRe = regexp.MustCompile(`[^a-z0-9_]`)

// SchemaName dựng tên schema từ tên test kèm hậu tố ngẫu nhiên: lần chạy lặp,
// lần chạy cũ còn sót và các lần chạy song song không thể trùng tên, và cả tên
// vẫn nằm trong giới hạn 63 byte của Postgres.
func SchemaName(t *testing.T) string {
	t.Helper()
	name := schemaNameRe.ReplaceAllString(strings.ToLower(t.Name()), "_")
	name = strings.Trim(name, "_")
	if name == "" {
		name = "pgtest"
	}
	// 12 ký tự hex (48 bit) + tên cắt còn 36: schema ≤ 56 byte, và role mà
	// migrate_test dựng từ tên schema (thêm tiền tố "app_") vẫn ≤ 63 nên phần
	// ngẫu nhiên không bị cắt mất.
	const prefix, randHex = "pgtest_", 12
	if len(name) > 36 {
		name = name[:36]
		name = strings.TrimRight(name, "_")
	}
	return prefix + name + "_" + randomHex(t)
}

// randomHex trả 6 byte ngẫu nhiên dạng hex; máy không có nguồn ngẫu nhiên thì
// vẫn phải ra một tên khác lần trước nên rơi về đồng hồ nano.
func randomHex(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%012x", time.Now().UnixNano()%0xffffffffffff)
	}
	return hex.EncodeToString(b[:])
}

// Provision creates an isolated schema on the test database and returns a DSN
// that routes every connection into it via the search_path runtime parameter.
// The schema is dropped (CASCADE) on test cleanup.
func Provision(t *testing.T) string {
	t.Helper()
	adminDSN := RequireURL(t)
	schema := SchemaName(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	defer func() { _ = admin.Close(context.Background()) }()

	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier{schema}.Sanitize())); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c, err := pgx.Connect(ctx, adminDSN)
		if err != nil {
			t.Logf("cleanup connect failed: %v", err)
			return
		}
		defer func() { _ = c.Close(context.Background()) }()
		if _, err := c.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", pgx.Identifier{schema}.Sanitize())); err != nil {
			t.Logf("drop test schema %s failed: %v", schema, err)
		}
	})

	// Embed search_path in the DSN text: pgx.ConnString() drops RuntimeParams,
	// but a DSN carrying `search_path=` is parsed back into RuntimeParams for
	// every connection (pgxpool and stdlib alike).
	sep := "?"
	if strings.Contains(adminDSN, "?") {
		sep = "&"
	}
	// search_path also includes `public` so database-global extensions pinned
	// there (citext, pg_trgm) resolve while test tables stay isolated.
	return adminDSN + sep + "search_path=" + schema + ",public"
}

// SchemaOf extracts the provisioned schema (first search_path entry) from a
// DSN produced by Provision.
func SchemaOf(dsn string) string {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return ""
	}
	sp, _, _ := strings.Cut(cfg.RuntimeParams["search_path"], ",")
	return sp
}
