package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/identity/identitytest"
	identitypg "github.com/datnt1998/verse/platform/identity/postgres"
	"github.com/datnt1998/verse/platform/pgtest"
)

// identityRepo builds the shared account repository with the Noverse project
// surface enabled. Tests that exercise project APIs use this; the least-
// privilege Viderse role tests build their own project-less instance.
func identityRepo(pool *postgres.Pool) *identitypg.Accounts {
	return identitypg.NewAccounts(pool.Pool(), identitypg.Options{
		ProjectAccess: true, ValidProjectID: library.ValidProjectID,
	})
}

// provisionAccounts migrates an isolated schema and returns a Store facade
// over the postgres repository + the raw pool for storage-level assertions.
func provisionAccounts(t *testing.T) (*identity.Store, *postgres.Pool) {
	t.Helper()
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	return identity.NewFromRepository(identityRepo(pool), library.IdentityOptions()), pool
}

// TestAccountContractPostgresBackend chạy đúng contract suite của file
// backend trên Postgres — parity là điều kiện qua cửa của phase 04.
func TestAccountContractPostgresBackend(t *testing.T) {
	identitytest.Run(t, func(t *testing.T) *identity.Store {
		t.Helper()
		s, _ := provisionAccounts(t)
		return s
	})
}

// TestAccountsSessionTokenHash pins session-hijack hardening: sessions lưu
// chỉ SHA-256(secret), không bao giờ lưu cookie/secret plaintext.
func TestAccountsSessionTokenHash(t *testing.T) {
	s, pool := provisionAccounts(t)
	ctx := context.Background()
	if _, err := s.CreateUser(identity.NewUser{Username: "chu", Role: identity.RoleAdmin, Password: "mat-khau-12345"}); err != nil {
		t.Fatal(err)
	}
	ns, err := s.Login("chu", "mat-khau-12345", "Mac", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	id, secret, ok := strings.Cut(ns.Cookie, ".")
	if !ok {
		t.Fatalf("cookie shape: %q", ns.Cookie)
	}
	var tokenHash []byte
	err = pool.Pool().QueryRow(ctx,
		`SELECT token_hash FROM sessions WHERE id = $1`, id).Scan(&tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokenHash) != 32 {
		t.Fatalf("token_hash must be a SHA-256 digest, got %d bytes", len(tokenHash))
	}
	sum := sha256.Sum256([]byte(secret))
	if hex.EncodeToString(tokenHash) != hex.EncodeToString(sum[:]) {
		t.Fatal("token_hash is not SHA-256(secret) — verifier drifted")
	}
	// Plaintext secret/cookie phải vắng mặt trong mọi cột liên quan.
	var hits int
	if err := pool.Pool().QueryRow(ctx,
		`SELECT count(*) FROM sessions WHERE token_hash = decode($1,'hex') OR user_agent LIKE '%'||$2||'%'`,
		secret, secret).Scan(&hits); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("plaintext secret leaked into sessions: %d rows", hits)
	}
	// Cookie sai secret (id đúng) phải bị từ chối — kẻ cướp DB row không login được.
	other, _, _ := strings.Cut(ns.Cookie, ".")
	fake := other + "." + strings.Repeat("ab", 32)
	if _, _, err := s.Authenticate(fake); err == nil {
		t.Fatal("forged cookie with valid id but wrong secret authenticated")
	}
}

// TestAccountsNoFileAccess pins "PG facade does not read the accounts file":
// Path() mô tả backend chứ không trả đường file; mọi dữ liệu đi qua repository.
func TestAccountsNoFileAccess(t *testing.T) {
	s, _ := provisionAccounts(t)
	if p := s.Path(); strings.Contains(p, ".json") || strings.Contains(p, "/") {
		t.Fatalf("PG accounts must not expose a file path: %q", p)
	}
}

// TestAccountsInjectedClockExpiry dùng đồng hồ tiêm qua Options.Now để chứng
// minh TTL mật khẩu tạm: hết hạn → login hỏng; phiên restricted đã có vẫn đổi
// được mật khẩu (contract đã kiểm ở suite qua consume; TTL chỉ test được khi
// điều khiển được đồng hồ).
func TestAccountsInjectedClockExpiry(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	now := time.Now()
	repo := identitypg.NewAccounts(pool.Pool(), identitypg.Options{
		ProjectAccess: true, ValidProjectID: library.ValidProjectID,
		Now: func() time.Time { return now },
	})
	s := identity.NewFromRepository(repo, library.IdentityOptions())
	if _, err := s.CreateUser(identity.NewUser{Username: "moi", Role: identity.RoleMember,
		Password: "temp-pass-111", MustChangePassword: true}); err != nil {
		t.Fatal(err)
	}
	ns, err := s.Login("moi", "temp-pass-111", "Mac", "")
	if err != nil {
		t.Fatalf("temp login: %v", err)
	}
	// Temp chỉ một lần: nhảy đồng hồ qua TTL → lần login khác hỏng (đã đốt)…
	now = now.Add(25 * time.Hour)
	if _, err := s.Login("moi", "temp-pass-111", "", ""); err == nil {
		t.Fatal("consumed temp password must not login again, even before expiry checks")
	}
	// …nhưng phiên restricted vẫn sống và đổi được mật khẩu sau TTL.
	if _, _, err := s.Authenticate(ns.Cookie); err != nil {
		t.Fatalf("restricted session must survive temp expiry: %v", err)
	}
	if err := s.ChangePassword("moi", "temp-pass-111", "mat-khau-moi-999"); err != nil {
		t.Fatalf("change after temp expiry: %v", err)
	}
	if _, err := s.Login("moi", "mat-khau-moi-999", "", ""); err != nil {
		t.Fatalf("new password login: %v", err)
	}
}

// TestAuthLimiterShared proves DB-backed buckets: failures recorded through
// one Accounts instance are visible through another (restart/restart parity).
func TestAuthLimiterShared(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	pool := openPool(t, dsn)
	one := identityRepo(pool).LoginLimiter()
	two := identityRepo(pool).LoginLimiter()
	for range 4 {
		one.Fail("ip:shared")
	}
	if wait := two.Wait("ip:shared"); wait <= 0 {
		t.Fatal("DB limiter bucket must be shared across instances")
	}
	two.Succeed("ip:shared")
	if wait := one.Wait("ip:shared"); wait != 0 {
		t.Fatalf("clear on one instance must clear the shared bucket: %s", wait)
	}
	// Recovery window limiter: N tries per window shared too.
	r1 := identityRepo(pool).RecoveryLimiter(5, time.Hour)
	r2 := identityRepo(pool).RecoveryLimiter(5, time.Hour)
	for range 5 {
		r1.Fail("ip:recover")
	}
	if wait := r2.Wait("ip:recover"); wait <= 0 {
		t.Fatal("recovery bucket must be shared across instances")
	}
}

// TestAuthLimiterBacksOffAcrossBlocks proves the lock grows with repeated
// failures. The gateway never records a failure while a key is blocked, so
// the next failure always lands after the previous block expired; that one
// must lock longer, not restart at the base duration.
func TestAuthLimiterBacksOffAcrossBlocks(t *testing.T) {
	dsn := pgtest.Provision(t)
	migrateSchema(t, dsn)
	limiter := identityRepo(openPool(t, dsn)).LoginLimiter()
	for range 4 {
		limiter.Fail("user:backoff")
	}
	first := limiter.Wait("user:backoff")
	if first <= 0 {
		t.Fatal("the fourth failure must block")
	}
	deadline := time.Now().Add(first + 5*time.Second)
	for limiter.Wait("user:backoff") > 0 {
		if time.Now().After(deadline) {
			t.Fatal("first block never expired")
		}
		time.Sleep(50 * time.Millisecond)
	}
	limiter.Fail("user:backoff")
	if second := limiter.Wait("user:backoff"); second <= first {
		t.Fatalf("failure after an expired block locked for %s, want longer than the previous %s", second, first)
	}
}
