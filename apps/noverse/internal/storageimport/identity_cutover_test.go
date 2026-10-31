package storageimport_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/migrate"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/storageimport"
	"github.com/datnt1998/verse/platform/pgtest"
)

const (
	fixtureDir        = "testdata/sso"
	fixtureNoverse    = fixtureDir + "/noverse-accounts.json"
	fixtureViderse    = fixtureDir + "/viderse-accounts.json"
	fixtureCollision  = fixtureDir + "/collision-viderse.json"
	fixtureTombstone  = fixtureDir + "/tombstone-viderse.json"
	fixtureVersion3   = fixtureDir + "/version3-accounts.json"
	noverseSourceUUID = "6f1d0f9e-3f2a-4c88-9d0e-1b1a2b3c4d5e"
	viderseSourceUUID = "7a2e1c4d-5b6f-4a7b-8c9d-0e1f2a3b4c5d"
)

func absFixture(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func copyFixture(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(rel))
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

func noverseSource(path string) storageimport.IdentitySource {
	return storageimport.IdentitySource{App: storageimport.SourceNoverse, SourceID: noverseSourceUUID, Path: path}
}

func viderseSource(path string) storageimport.IdentitySource {
	return storageimport.IdentitySource{App: storageimport.SourceViderse, SourceID: viderseSourceUUID, Path: path}
}

func emptyViderse() storageimport.IdentitySource {
	return storageimport.IdentitySource{App: storageimport.SourceViderse, SourceID: "00000000-0000-0000-0000-000000000000", Empty: true}
}

// openCutoverDB provisions a migrated schema and opens the domain pool.
func openCutoverDB(t *testing.T) *postgres.Pool {
	t.Helper()
	pool, _ := openCutoverDBDSN(t)
	return pool
}

// openCutoverDBDSN additionally returns the provisioned DSN (the
// search_path-scoped maintenance DSN the CLI receives via
// MIGRATION_DATABASE_URL).
func openCutoverDBDSN(t *testing.T) (*postgres.Pool, string) {
	t.Helper()
	dsn := pgtest.Provision(t)
	db, err := migrate.OpenDB(dsn)
	if err != nil {
		t.Fatalf("open migration db: %v", err)
	}
	if _, err := migrate.Up(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrate up: %v", err)
	}
	db.Close()
	pool, err := postgres.Open(context.Background(), postgres.PoolConfig{DSN: dsn})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, dsn
}

func writeAccounts(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "web-accounts.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// baseUser is a minimal valid v4 account entry the table tests mutate.
func baseUser(username string) map[string]any {
	return map[string]any{
		"username":           username,
		"name":               username,
		"role":               "member",
		"projects":           map[string]string{},
		"disabled":           false,
		"passwordHash":       "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		"onboardedAt":        "2026-01-01T00:00:00Z",
		"mustChangePassword": false,
		"createdAt":          "2026-01-01T00:00:00Z",
		"updatedAt":          "2026-01-01T00:00:00Z",
	}
}

func planSources(t *testing.T, pool *postgres.Pool, nov, vid storageimport.IdentitySource, renames []storageimport.RenameEntry) *storageimport.CutoverPlan {
	t.Helper()
	plan, err := storageimport.PlanCutover(context.Background(), pool, nov, vid, renames)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan
}

func collisionKinds(plan *storageimport.CutoverPlan) []string {
	var kinds []string
	for _, c := range plan.Collisions {
		kinds = append(kinds, c.Kind)
	}
	return kinds
}

// ── strict decode ────────────────────────────────────────────────────────────

func TestSSOCutoverDecodesExactV4(t *testing.T) {
	src, err := storageimport.DecodeIdentitySource(noverseSource(absFixture(t, fixtureNoverse)))
	if err != nil {
		t.Fatalf("decode v4: %v", err)
	}
	rep := src
	_ = rep
	plan := planSources(t, nil, noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	if plan.Failed() {
		t.Fatalf("clean v4 must not fail: %v %v", plan.Fatal, plan.Collisions)
	}
	srcs := plan.Sources()
	if len(srcs) != 2 || srcs[0].Users != 7 || srcs[0].SessionsDrop != 1 || srcs[0].PairingsDrop != 1 || srcs[0].Audit != 1 {
		t.Fatalf("source report sai: %+v", srcs)
	}
	if !srcs[1].Empty {
		t.Fatal("viderse-empty phải báo nguồn trống")
	}
}

func TestSSOCutoverRejectsOtherVersionsAndCorrupt(t *testing.T) {
	noverse := absFixture(t, fixtureNoverse)
	cases := []struct {
		name string
		file string
		want string
	}{
		{"older-v3", absFixture(t, fixtureVersion3), "BẢN SAO"},
		{"newer", writeAccounts(t, map[string]any{"version": 5, "users": []any{baseUser("a1")}}), "mới hơn"},
		{"missing-version", writeAccounts(t, map[string]any{"users": []any{baseUser("a1")}}), "phiên bản lạ"},
		{"zero-users", writeAccounts(t, map[string]any{"version": 4, "users": []any{}}), "không có tài khoản"},
	}
	// Truncated JSON must error.
	trunc := filepath.Join(t.TempDir(), "trunc.json")
	data, _ := os.ReadFile(fixtureNoverse)
	if err := os.WriteFile(trunc, data[:len(data)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	cases = append(cases, struct {
		name string
		file string
		want string
	}{"truncated", trunc, "hỏng"})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := storageimport.DecodeIdentitySource(noverseSource(tc.file))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("cần lỗi chứa %q, có %v", tc.want, err)
			}
		})
	}
	// ENOENT is never an empty store.
	if _, err := storageimport.DecodeIdentitySource(noverseSource(filepath.Join(t.TempDir(), "missing.json"))); err == nil {
		t.Fatal("file thiếu phải lỗi, không coi là kho trống")
	}
	// Relative paths are refused.
	if _, err := storageimport.DecodeIdentitySource(noverseSource("relative.json")); err == nil {
		t.Fatal("đường tương đối phải lỗi")
	}
	_ = noverse
}

func TestSSOCutoverRejectsMalformedAccounts(t *testing.T) {
	make := func(mut func(u map[string]any)) string {
		u := baseUser("ok-user")
		mut(u)
		return writeAccounts(t, map[string]any{"version": 4, "users": []any{u}})
	}
	cases := []struct {
		name string
		file string
		want string
	}{
		{"username-uppercase", make(func(u map[string]any) { u["username"] = "Alice" }), "username"},
		{"username-space", make(func(u map[string]any) { u["username"] = " alice" }), "username"},
		{"username-short", make(func(u map[string]any) { u["username"] = "a" }), "username"},
		{"role-unknown", make(func(u map[string]any) { u["role"] = "owner" }), "role lạ"},
		{"hash-not-argon2", make(func(u map[string]any) { u["passwordHash"] = "bcrypt$xyz" }), "Argon2id"},
		{"temp-ttl-inverted", make(func(u map[string]any) {
			u["tempPassword"] = map[string]any{
				"codeHash":  "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
				"expiresAt": "2026-01-01T00:00:00Z", "createdAt": "2026-02-01T00:00:00Z"}
		}), "TTL"},
		{"temp-hash-not-argon2", make(func(u map[string]any) {
			u["tempPassword"] = map[string]any{
				"codeHash": "plain", "expiresAt": "2026-03-01T00:00:00Z", "createdAt": "2026-02-01T00:00:00Z"}
		}), "Argon2id"},
		{"recovery-hash-not-argon2", make(func(u map[string]any) {
			u["recovery"] = map[string]any{"codeHash": "md5$xx", "createdAt": "2026-02-01T00:00:00Z"}
		}), "Argon2id"},
		{"disabledBy-unknown", make(func(u map[string]any) { u["disabledBy"] = "ghost" }), "disabledBy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := storageimport.DecodeIdentitySource(noverseSource(tc.file))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("cần lỗi chứa %q, có %v", tc.want, err)
			}
		})
	}
}

// ── collisions ───────────────────────────────────────────────────────────────

func TestSSOUsernameCollisionFailsBeforeWrites(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()

	// two_sources: same canonical username across the files.
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureCollision)), nil)
	kinds := collisionKinds(plan)
	if !contains(kinds, "two_sources") {
		t.Fatalf("cần collision two_sources, có %v", kinds)
	}
	// Same-hash is not merge evidence: collision-viderse's tac-gia uses a
	// DIFFERENT hash than noverse's tac-gia, and even identical hashes would
	// still collide — nothing short of a ledger skip may merge.
	var collision storageimport.CutoverCollision
	for _, c := range plan.Collisions {
		if c.Kind == "two_sources" {
			collision = c
		}
	}
	if collision.SourceUsername != "tac-gia" || collision.TargetUsername != "tac-gia" {
		t.Fatalf("collision thiếu đầu mối nguồn/đích: %+v", collision)
	}
	assertNoUsers(t, pool)

	// tombstone: a deleted source account still reserves its username.
	plan = planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureTombstone)), nil)
	if !contains(collisionKinds(plan), "two_sources") {
		t.Fatalf("tombstone cùng tên phải collision, có %v", collisionKinds(plan))
	}

	// target_occupied: a user already in the store (even soft-deleted).
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status, deleted_at)
VALUES ('11111111-1111-4111-8111-111111111111', 'vid-member', 'Đích', 'member', 'active', now())`); err != nil {
		t.Fatalf("seed target tombstone: %v", err)
	}
	plan = planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	if !contains(collisionKinds(plan), "target_occupied") {
		t.Fatalf("đích có tombstone cùng tên phải collision, có %v", collisionKinds(plan))
	}
}

func assertNoUsers(t *testing.T, pool *postgres.Pool) {
	t.Helper()
	var n int
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("collision phải dừng trước mọi ghi, users=%d", n)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestSSOCutoverViderseProjectMapStops(t *testing.T) {
	u := baseUser("vid-admin")
	u["role"] = "admin"
	u["projects"] = map[string]string{"kenh-a": "owner"}
	file := writeAccounts(t, map[string]any{"version": 4, "users": []any{u}, "audit": []any{}})
	plan := planSources(t, nil,
		noverseSource(absFixture(t, fixtureNoverse)), viderseSource(file), nil)
	if !contains(collisionKinds(plan), "viderse_projects") {
		t.Fatalf("projects không rỗng phải dừng, có %v", collisionKinds(plan))
	}
}

// ── apply path ───────────────────────────────────────────────────────────────

func applyForTest(t *testing.T, pool *postgres.Pool, plan *storageimport.CutoverPlan) *storageimport.ApplyResult {
	t.Helper()
	backup := dumpForTest(t, pool)
	res, err := storageimport.ApplyCutover(context.Background(), pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC())
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return res
}

func testManifestSHA(plan *storageimport.CutoverPlan) string {
	m := storageimport.ManifestForPlan(plan, "postgres", "", "", nil)
	sum, _, err := storageimport.ManifestSHA256(m)
	if err != nil {
		panic(err)
	}
	return sum
}

func TestSSOApplyGrantsPerAppAndPreservesCredentials(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	if plan.Failed() {
		t.Fatalf("plan failed: %v %v", plan.Fatal, plan.Collisions)
	}
	res := applyForTest(t, pool, plan)
	if res.Inserted != 9 || res.Skipped != 0 {
		t.Fatalf("inserted=%d skipped=%d", res.Inserted, res.Skipped)
	}
	// Grants: 6 live noverse (tombstone excluded, locked included) + 2 viderse.
	if res.Grants != 8 {
		t.Fatalf("grants=%d, muốn 8", res.Grants)
	}
	type grant struct{ app, role string }
	grants := map[string]grant{}
	rows, err := pool.Pool().Query(ctx, `
SELECT u.username::text, g.app, g.role FROM app_grants g JOIN users u ON u.id = g.user_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var u, a, r string
		if err := rows.Scan(&u, &a, &r); err != nil {
			t.Fatal(err)
		}
		grants[u] = grant{a, r}
	}
	wantGrants := map[string]string{
		"quan-tri": "noverse", "tac-gia": "noverse", "khoa-tam": "noverse",
		"het-temp": "noverse", "dung-temp": "noverse", "moi-mat": "noverse",
		"vid-admin": "viderse", "vid-member": "viderse",
	}
	for u, app := range wantGrants {
		if grants[u].app != app || grants[u].role != "member" {
			t.Fatalf("grant của %s sai: %+v", u, grants[u])
		}
	}
	if _, ok := grants["da-xoa"]; ok {
		t.Fatal("tombstone không được cấp grant")
	}

	// Credentials preserved verbatim, never re-hashed and never extended.
	var hash string
	var mustChange bool
	var status string
	if err := pool.Pool().QueryRow(ctx, `
SELECT c.password_hash, u.must_change_password, u.status
FROM user_credentials c JOIN users u ON u.id = c.user_id WHERE u.username='tac-gia'`).
		Scan(&hash, &mustChange, &status); err != nil {
		t.Fatal(err)
	}
	if hash != "$argon2id$v=19$m=65536,t=3,p=4$c2ltdWxhdGVkc2FsdDI$c2ltdWxhdGVkaGFzaDI" || mustChange || status != "active" {
		t.Fatalf("credential sai: %s %v %s", hash, mustChange, status)
	}
	// Tombstone + locked keep their states.
	var deletedAt *time.Time
	if err := pool.Pool().QueryRow(ctx, `SELECT deleted_at, status FROM users WHERE username='da-xoa'`).Scan(&deletedAt, &status); err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil || status != "locked" {
		t.Fatalf("tombstone sai: %v %s", deletedAt, status)
	}
	// Temp passwords: expired stays expired, used stays used, created_by binds.
	var exp, used *time.Time
	var createdBy *string
	if err := pool.Pool().QueryRow(ctx, `
SELECT t.expires_at, t.used_at, ub.username::text
FROM temporary_passwords t JOIN users u ON u.id = t.user_id
LEFT JOIN users ub ON ub.id = t.created_by
WHERE u.username='het-temp'`).Scan(&exp, &used, &createdBy); err != nil {
		t.Fatal(err)
	}
	if exp == nil || !exp.Equal(time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)) || used != nil {
		t.Fatalf("temp hết hạn bị thay đổi: exp=%v used=%v", exp, used)
	}
	if createdBy == nil || *createdBy != "quan-tri" {
		t.Fatalf("temp created_by mất: %v", createdBy)
	}
	if err := pool.Pool().QueryRow(ctx, `
SELECT t.used_at FROM temporary_passwords t JOIN users u ON u.id=t.user_id WHERE u.username='dung-temp'`).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if used == nil {
		t.Fatal("temp đã dùng thành chưa dùng")
	}
	// Recovery state preserved.
	var recUsed *time.Time
	var mask string
	if err := pool.Pool().QueryRow(ctx, `
SELECT r.used_at, r.mask FROM recovery_codes r JOIN users u ON u.id=r.user_id WHERE u.username='moi-mat'`).Scan(&recUsed, &mask); err != nil {
		t.Fatal(err)
	}
	if recUsed != nil || mask != "ABCD-****-WXYZ" {
		t.Fatalf("recovery sai: %v %s", recUsed, mask)
	}
	// disabled_by resolves to the same source's disabler.
	var disabler string
	if err := pool.Pool().QueryRow(ctx, `
SELECT ub.username::text FROM users u JOIN users ub ON ub.id=u.disabled_by WHERE u.username='khoa-tam'`).Scan(&disabler); err != nil {
		t.Fatal(err)
	}
	if disabler != "quan-tri" {
		t.Fatalf("disabled_by sai: %s", disabler)
	}
	// Audit copied with source provenance, actor bound to imported user.
	var actorKind, action string
	if err := pool.Pool().QueryRow(ctx, `
SELECT actor_kind, action FROM audit_logs a
WHERE a.details->>'sourceApp'='noverse' ORDER BY occurred_at LIMIT 1`).Scan(&actorKind, &action); err != nil {
		t.Fatal(err)
	}
	if actorKind != "user" || action != "disable" {
		t.Fatalf("audit sai: %s %s", actorKind, action)
	}
}

func TestSSOCutoverInvalidatesOldSessions(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	// A pre-existing live account with a session + pairing: cutover must kill
	// both inside the same transaction, and a rollback must not.
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('22222222-2222-4222-8222-222222222222', 'cu-song', 'Cũ', 'member', 'active');
INSERT INTO devices (id, user_id, name)
VALUES ('33333333-3333-4333-8333-333333333333', '22222222-2222-4222-8222-222222222222', 'laptop');
INSERT INTO sessions (id, user_id, device_id, token_hash, credential_kind, expires_at)
VALUES ('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222',
        '33333333-3333-4333-8333-333333333333', sha256('session-secret'::bytea), 'password', now() + interval '1 day');
INSERT INTO device_pairings (id, user_id, token_hash, expires_at)
VALUES ('55555555-5555-4555-8555-555555555555', '22222222-2222-4222-8222-222222222222',
        sha256('pairing-secret'::bytea), now() + interval '5 minutes');`); err != nil {
		t.Fatalf("seed session/pairing: %v", err)
	}
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	res := applyForTest(t, pool, plan)
	if res.SessionsRevoked != 1 || res.PairingsRevoked != 1 {
		t.Fatalf("revoked=%d/%d, muốn 1/1", res.SessionsRevoked, res.PairingsRevoked)
	}
	var live int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("còn session sống: %d", live)
	}
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM device_pairings WHERE revoked_at IS NULL`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("còn pairing sống: %d", live)
	}
	// File bearer entries were counted as dropped, never imported.
	srcs := plan.Sources()
	if srcs[0].SessionsDrop != 1 || srcs[0].PairingsDrop != 1 {
		t.Fatalf("file bearer phải bị đếm huỷ, không nhập: %+v", srcs[0])
	}
	var importedSessions int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&importedSessions); err != nil {
		t.Fatal(err)
	}
	if importedSessions != 1 {
		t.Fatalf("session file bị nhập nhầm: %d", importedSessions)
	}
}

func TestSSOApplyFailureRollsBackBothSources(t *testing.T) {
	ctx := context.Background()
	assertRolledBack := func(t *testing.T, pool *postgres.Pool, extraUsernames ...string) {
		t.Helper()
		var n int
		names := append([]string{"quan-tri", "tac-gia", "khoa-tam", "da-xoa", "het-temp", "dung-temp", "moi-mat", "vid-member"}, extraUsernames...)
		if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE username = ANY($1)`, names).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("rollback không đủ hai nguồn: còn %d user mới", n)
		}
		var ledger int
		if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM import_items`).Scan(&ledger); err != nil {
			t.Fatal(err)
		}
		if ledger != 0 {
			t.Fatalf("ledger còn sót sau rollback: %d", ledger)
		}
	}

	t.Run("username appears after preflight", func(t *testing.T) {
		pool := openCutoverDB(t)
		plan := planSources(t, pool,
			noverseSource(absFixture(t, fixtureNoverse)),
			viderseSource(fileFor(t, "vid-admin")), nil)
		if plan.Failed() {
			t.Fatalf("plan failed: %v %v", plan.Fatal, plan.Collisions)
		}
		if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('66666666-6666-4666-8666-666666666666', 'vid-admin', 'X', 'member', 'active')`); err != nil {
			t.Fatalf("seed raced user: %v", err)
		}
		backup := dumpForTest(t, pool)
		_, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC())
		if err == nil {
			t.Fatal("apply phải rollback khi username xuất hiện sau preflight")
		}
		assertRolledBack(t, pool)
	})

	t.Run("audit id collides mid transaction", func(t *testing.T) {
		pool := openCutoverDB(t)
		plan := planSources(t, pool,
			noverseSource(absFixture(t, fixtureNoverse)),
			viderseSource(absFixture(t, fixtureViderse)), nil)
		if plan.Failed() {
			t.Fatalf("plan failed: %v %v", plan.Fatal, plan.Collisions)
		}
		// Seed the deterministic audit id the source will write — users and
		// grants of BOTH sources are already inserted when this insert fails,
		// proving the single transaction rolls everything back.
		auditID := storageimport.DeterministicUUID(viderseSourceUUID, "audit", "audit-viderse-1")
		if _, err := pool.Pool().Exec(ctx, `
INSERT INTO audit_logs (id, actor_kind, action, target_kind, target_id)
VALUES ($1, 'system', 'preexisting', 'account', 'x')`, auditID); err != nil {
			t.Fatalf("seed audit: %v", err)
		}
		backup := dumpForTest(t, pool)
		_, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC())
		if err == nil {
			t.Fatal("apply phải rollback khi insert audit đụng id")
		}
		assertRolledBack(t, pool, "vid-admin")
	})
}

func fileFor(t *testing.T, usernames ...string) string {
	t.Helper()
	var users []any
	for _, name := range usernames {
		users = append(users, baseUser(name))
	}
	return writeAccounts(t, map[string]any{"version": 4, "users": users, "audit": []any{}})
}

func TestSSOResumePreservesPostImportPassword(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	applyForTest(t, pool, plan)

	// Post-import changes the source must never resurrect: the user changes
	// their password, an admin revokes the viderse-member-equivalent grant,
	// and the open temp password gets consumed.
	newHash := "$argon2id$v=19$m=65536,t=3,p=4$bmlld2hhc2hzYWx0$bmlld2hhc2g"
	if _, err := pool.Pool().Exec(ctx, `
UPDATE user_credentials SET password_hash=$1, password_changed_at=now(), updated_at=now(), version=version+1
WHERE user_id = (SELECT id FROM users WHERE username='tac-gia')`, newHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Pool().Exec(ctx, `
DELETE FROM app_grants WHERE user_id = (SELECT id FROM users WHERE username='moi-mat')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Pool().Exec(ctx, `
UPDATE temporary_passwords SET used_at=now(), updated_at=now(), version=version+1
WHERE user_id = (SELECT id FROM users WHERE username='het-temp')`); err != nil {
		t.Fatal(err)
	}

	// Rerun with the same source + manifest: ledger-proven skip everywhere.
	plan = planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	if plan.Failed() {
		t.Fatalf("rerun plan failed: %v %v", plan.Fatal, plan.Collisions)
	}
	res := applyForTest(t, pool, plan)
	if res.Inserted != 0 || res.Skipped != 7 {
		t.Fatalf("resume phải skip hết: inserted=%d skipped=%d", res.Inserted, res.Skipped)
	}
	var hash string
	if err := pool.Pool().QueryRow(ctx, `
SELECT password_hash FROM user_credentials WHERE user_id=(SELECT id FROM users WHERE username='tac-gia')`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != newHash {
		t.Fatalf("password đổi sau import bị ghi đè về: %s", hash)
	}
	var grants int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM app_grants WHERE user_id=(SELECT id FROM users WHERE username='moi-mat')`).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if grants != 0 {
		t.Fatal("grant đã thu hồi bị hồi sinh")
	}
	var used *time.Time
	if err := pool.Pool().QueryRow(ctx, `
SELECT used_at FROM temporary_passwords WHERE user_id=(SELECT id FROM users WHERE username='het-temp')`).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if used == nil {
		t.Fatal("temp đã dùng bị hồi sinh")
	}
}

func TestSSOConcurrentApplyInsertsOnce(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	// Hold the global cutover lock on another connection; a second apply must
	// fail fast instead of queueing a second import.
	conn, err := pool.Pool().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	key := fnvLockKey("verse/sso-cutover/identity")
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if !locked {
		t.Fatal("không giữ được khoá cutover")
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key)

	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	backup := dumpForTest(t, pool)
	_, err = storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "cutover") {
		t.Fatalf("apply thứ hai phải dừng vì lock, có: %v", err)
	}
	assertNoUsers(t, pool)
}

// fnvLockKey mirrors storageimport's advisory lock recipe.
func fnvLockKey(sourceID string) int64 {
	h := uint64(14695981039346656037)
	for i := range len(sourceID) {
		h ^= uint64(sourceID[i])
		h *= 1099511628211
	}
	var out [8]byte
	binary.BigEndian.PutUint64(out[:], h)
	return int64(binary.BigEndian.Uint64(out[:]) & 0x0fff_ffff_ffff_ffff)
}

// ── backups ──────────────────────────────────────────────────────────────────

func TestSSOBackupIsExactAndNeverOverwritten(t *testing.T) {
	src := copyFixture(t, fixtureNoverse)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	bs := storageimport.BackupSource{Path: src, SHA256: hex.EncodeToString(sum[:])}

	receipts, err := storageimport.EnsureSourceBackups([]storageimport.BackupSource{bs})
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if len(receipts) != 1 || !receipts[0].Created {
		t.Fatalf("receipt sai: %+v", receipts)
	}
	bak := src + ".v4.bak"
	got, err := os.ReadFile(bak)
	if err != nil {
		t.Fatal(err)
	}
	if !equalByteSlices(got, data) {
		t.Fatal("backup không byte-equal")
	}
	st, err := os.Stat(bak)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode %o, muốn 0600", st.Mode().Perm())
	}

	// Second run verifies and does NOT overwrite (mtime + bytes stable).
	receipts, err = storageimport.EnsureSourceBackups([]storageimport.BackupSource{bs})
	if err != nil {
		t.Fatalf("verify backup: %v", err)
	}
	if receipts[0].Created {
		t.Fatal("backup đã có phải verify, không tạo lại")
	}

	// A different-digest backup stops the run without being replaced.
	if err := os.WriteFile(bak, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := storageimport.EnsureSourceBackups([]storageimport.BackupSource{bs}); err == nil {
		t.Fatal("backup khác digest phải dừng")
	}
	got, _ = os.ReadFile(bak)
	if string(got) != "tampered" {
		t.Fatal("backup lạ bị ghi đè — cấm tuyệt đối")
	}

	// O_EXCL on an unwritable directory stops before any DB work.
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("chmod không được hỗ trợ")
	}
	blocked := filepath.Join(dir, "web-accounts.json")
	if err := os.WriteFile(blocked, data, 0o600); err != nil {
		t.Skip("không mô phỏng được thư mục read-only")
	}
	if _, err := storageimport.EnsureSourceBackups([]storageimport.BackupSource{
		{Path: blocked, SHA256: hex.EncodeToString(sum[:])},
	}); err == nil {
		t.Fatal("thư mục không ghi được phải lỗi")
	}
}

func equalByteSlices(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ── rename map / admin approval / manifest ──────────────────────────────────

func TestSSORenameMapResolvesCollisionAndRemapsRefs(t *testing.T) {
	pool := openCutoverDB(t)
	renames, err := storageimport.LoadRenameMap(absFixture(t, fixtureDir+"/rename-map.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureCollision)), renames)
	if plan.Failed() {
		t.Fatalf("rename phải hết collision: %v %v", plan.Fatal, plan.Collisions)
	}
	res := applyForTest(t, pool, plan)
	if res.Inserted != 9 {
		t.Fatalf("inserted=%d", res.Inserted)
	}
	var count int
	if err := pool.Pool().QueryRow(context.Background(), `SELECT count(*) FROM users WHERE username IN ('tac-gia','tac-gia-viderse')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("phải có 2 account riêng sau rename: %d", count)
	}
	// Ledger keeps the SOURCE username as record key.
	var key string
	if err := pool.Pool().QueryRow(context.Background(), `
SELECT record_key FROM import_items WHERE source_id=$1 AND relative_path='@accounts' AND record_key='tac-gia'`,
		viderseSourceUUID).Scan(&key); err != nil {
		t.Fatalf("ledger phải giữ source username: %v", err)
	}
}

func TestSSODemoteAdminViaMapNeedsManifestDigest(t *testing.T) {
	pool := openCutoverDB(t)
	renames, err := storageimport.LoadRenameMap(absFixture(t, fixtureDir+"/demote-admin-map.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), renames)
	if plan.Failed() {
		t.Fatalf("plan failed: %v", plan.Collisions)
	}
	// The demoted admin drops off the approval list; the surviving admin
	// from the Noverse file stays on it.
	origins := map[string]string{}
	for _, a := range plan.GlobalAdmins {
		origins[a.Username] = a.Origin
	}
	if _, ok := origins["vid-admin"]; ok {
		t.Fatalf("admin đã hạ không được còn trong globalAdmins: %+v", plan.GlobalAdmins)
	}
	if origins["quan-tri"] != storageimport.SourceNoverse {
		t.Fatalf("admin Noverse phải nằm trong danh sách duyệt: %+v", plan.GlobalAdmins)
	}
	if plan.GlobalAdminsDigest == "" {
		t.Fatal("globalAdminsDigest phải luôn được tính")
	}
	res := applyForTest(t, pool, plan)
	_ = res
	var role string
	if err := pool.Pool().QueryRow(context.Background(), `SELECT global_role FROM users WHERE username='vid-admin'`).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "member" {
		t.Fatalf("role sau map phải member, có %s", role)
	}
}

func TestSSOAdminPromotionReportListsDigest(t *testing.T) {
	pool := openCutoverDB(t)
	if _, err := pool.Pool().Exec(context.Background(), `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('00000000-0000-0000-0000-0000000000aa', 'admin-cu', 'Admin cũ', 'admin', 'active')`); err != nil {
		t.Fatal(err)
	}
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	if plan.GlobalAdminsDigest == "" {
		t.Fatal("phải luôn có globalAdminsDigest")
	}
	// Every origin is on the list: the committed DB admin, the Noverse file
	// admin and the Viderse file admin.
	origins := map[string]string{}
	for _, a := range plan.GlobalAdmins {
		origins[a.Username] = a.Origin
	}
	for username, origin := range map[string]string{
		"admin-cu": "existing", "quan-tri": "noverse", "vid-admin": "viderse",
	} {
		if origins[username] != origin {
			t.Fatalf("globalAdmins thiếu hoặc gán sai origin cho %s: %+v", username, plan.GlobalAdmins)
		}
	}
	// The digest is deterministic for the same approved list.
	plan2 := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	if plan.GlobalAdminsDigest != plan2.GlobalAdminsDigest {
		t.Fatal("admin digest không ổn định")
	}
}

func TestSSOManifestDigestStableAndCoversDecisions(t *testing.T) {
	plan := planSources(t, nil,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	m := storageimport.ManifestForPlan(plan, "postgres", "", "", nil)
	sha1, bytes1, err := storageimport.ManifestSHA256(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(bytes1, &decoded); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes1)
	if hex.EncodeToString(sum[:]) != sha1 {
		t.Fatal("manifest digest không khớp bytes — owner không verify được")
	}
	renames, _ := storageimport.LoadRenameMap(absFixture(t, fixtureDir+"/rename-map.json"))
	plan2 := planSources(t, nil,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureCollision)), renames)
	m2 := storageimport.ManifestForPlan(plan2, "postgres", "", "", nil)
	sha2, _, _ := storageimport.ManifestSHA256(m2)
	if sha1 == sha2 {
		t.Fatal("manifest phải đổi khi rename map đổi")
	}
}

// ── report secret hygiene ────────────────────────────────────────────────────

func TestMigrationReportContainsNoSecrets(t *testing.T) {
	pool := openCutoverDB(t)
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	res := applyForTest(t, pool, plan)
	if res == nil {
		t.Fatal("no result")
	}
	// Marshal every owner-facing structure and scan for credential material.
	report := map[string]any{
		"sources":    plan.Sources(),
		"collisions": plan.Collisions,
		"fatal":      plan.Fatal,
		"renames":    plan.Renames,
		"admins":     plan.GlobalAdmins,
		"identity":   res,
		"manifest":   storageimport.ManifestForPlan(plan, "postgres", "", "", nil),
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{
		"$argon2id$", "passwordHash", "codeHash", "secretHash", "token",
		"postgres://", "password=", "sess-noverse-1", "ABCD-****-WXYZ",
		"c2ltdWxhdGVkc2FsdA", "cnVubmluZ3NhbHQ",
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("report rò rỉ %q", secret)
		}
	}
	// The manifest/report keep only paths, digests, counts — collisions and
	// renames carry usernames; the clean run reports by digest and counts.
	if !strings.Contains(text, "sha256") || !strings.Contains(text, "users") {
		t.Fatal("report thiếu thông tin cần đối soát")
	}
	// Manifest file itself must be clean too.
	_, mBytes, _ := storageimport.ManifestSHA256(storageimport.ManifestForPlan(plan, "postgres", "", "", nil))
	if strings.Contains(string(mBytes), "argon2") || strings.Contains(string(mBytes), "Hash") {
		t.Fatal("manifest chứa vật liệu bí mật")
	}
}

// ── pg_dump backup verification ─────────────────────────────────────────────

// pgDump runs a real pg_dump; the test refuses to pass on a fake archive.
func pgDump(t *testing.T, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("pg_restore"); err != nil {
		t.Fatalf("pg_restore không có trên PATH: %v", err)
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Fatalf("pg_dump không có trên PATH: %v", err)
	}
	if err := exec.Command("pg_dump", args...).Run(); err != nil {
		t.Fatalf("pg_dump thất bại: %v", err)
	}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--file=") {
			continue
		}
		path := strings.TrimPrefix(arg, "--file=")
		if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			mode := os.FileMode(0o600)
			if info.IsDir() {
				mode = 0o700
			}
			return os.Chmod(path, mode)
		}); err != nil {
			t.Fatalf("đặt quyền riêng tư cho dump: %v", err)
		}
	}
}

// plainDumpDSN strips runtime params (search_path) libpq tools reject.
func plainDumpDSN(t *testing.T) string {
	t.Helper()
	dsn := pgtest.RequireURL(t)
	if i := strings.Index(dsn, "?"); i >= 0 {
		dsn = dsn[:i]
	}
	return dsn
}

func dumpForTest(t *testing.T, pool *postgres.Pool) *storageimport.VerifiedBackup {
	t.Helper()
	var schema string
	if err := pool.Pool().QueryRow(context.Background(), `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("đọc schema để dump: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	pgDump(t, "--format=custom", "--schema="+schema, "--file="+filepath.Join(dir, "backup.dump"), plainDumpDSN(t))
	backup, err := storageimport.CheckBackupDir(context.Background(), pool, dir)
	if err != nil {
		t.Fatalf("kiểm tra dump thật: %v", err)
	}
	if backup == nil {
		t.Fatal("kiểm tra dump thật không trả receipt")
	}
	return backup
}

func TestSSOBackupDirVerifiesAgainstTargetDB(t *testing.T) {
	pool, dsn := openCutoverDBDSN(t)
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	schema := pgtest.SchemaOf(dsn)
	dumpDSN := plainDumpDSN(t)

	// A magic-only stub is not a readable archive.
	stub := filepath.Join(dir, "stub.dump")
	if err := os.WriteFile(stub, []byte("PGDMP\x01\x10\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if vb, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || vb != nil {
		t.Fatalf("file chỉ có magic phải bị từ chối: %+v", vb)
	}

	// A genuine dump of the WRONG database is refused: its dbname header
	// must equal current_database().
	wrong := filepath.Join(dir, "wrong-db.dump")
	pgDump(t, "--format=custom", "--file="+wrong, "postgres")
	if vb, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || vb != nil {
		t.Fatalf("dump database khác phải bị từ chối: %+v", vb)
	}

	// A genuine dump of THIS database but truncated cannot be read end to
	// end — restore integrity fails, not just the header check.
	good := filepath.Join(dir, "good.dump")
	pgDump(t, "--format=custom", "--schema="+schema, "--file="+good, dumpDSN)
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	trunc := filepath.Join(dir, "truncated.dump")
	half := append([]byte(nil), data[:len(data)/2]...)
	if err := os.WriteFile(trunc, half, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(good); err != nil {
		t.Fatal(err)
	}
	if vb, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || vb != nil {
		t.Fatalf("dump bị cắt phải bị từ chối: %+v", vb)
	}

	// The intact schema dump of the target verifies and is fingerprinted.
	if err := os.WriteFile(good, data, 0o600); err != nil {
		t.Fatal(err)
	}
	vb, err := storageimport.CheckBackupDir(ctx, pool, dir)
	if err != nil {
		t.Fatalf("dump hợp lệ phải được chấp nhận: %v", err)
	}
	if vb.Path != good || vb.Format != "custom" {
		t.Fatalf("receipt sai: %+v", vb)
	}
	sum := sha256.Sum256(data)
	if vb.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 receipt sai: %s", vb.SHA256)
	}
	if !strings.Contains(vb.PGRestore, "pg_restore") || !filepath.IsAbs(vb.PGRestorePath) {
		t.Fatalf("receipt thiếu phiên bản/đường dẫn pg_restore: %+v", vb)
	}

	// Directory-format dumps cannot be verified as a single immutable handle.
	dirDump := filepath.Join(dir, "dirdump")
	pgDump(t, "--format=directory", "--schema="+schema, "--file="+dirDump, dumpDSN)
	if vb, err := storageimport.CheckBackupDir(ctx, pool, t.TempDir()); err == nil || vb != nil {
		t.Fatalf("thư mục không có dump phải từ chối: %+v", vb)
	}
	for _, stale := range []string{good, stub, wrong, trunc} {
		os.Remove(stale)
	}
	vb, err = storageimport.CheckBackupDir(ctx, pool, dir)
	if err == nil || vb != nil || !strings.Contains(err.Error(), dirDump) || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("directory dump phải bị từ chối kèm lý do: %+v, %v", vb, err)
	}
}

func TestSSOExistingBackupMustBePrivate(t *testing.T) {
	src := copyFixture(t, fixtureNoverse)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	bs := storageimport.BackupSource{Path: src, SHA256: hex.EncodeToString(sum[:])}
	bak := src + ".v4.bak"

	// A symlink — even one resolving to identical bytes — is never backup
	// evidence.
	if err := os.Symlink(src, bak); err != nil {
		t.Skip("symlink không được hỗ trợ")
	}
	if _, err := storageimport.EnsureSourceBackups([]storageimport.BackupSource{bs}); err == nil {
		t.Fatal(".v4.bak là symlink phải bị từ chối")
	}
	os.Remove(bak)

	// A group/other-readable copy with identical bytes is refused, not
	// chmod-ed or trusted.
	if err := os.WriteFile(bak, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := storageimport.EnsureSourceBackups([]storageimport.BackupSource{bs}); err == nil {
		t.Fatal(".v4.bak mode 0644 phải bị từ chối")
	}
	got, _ := os.ReadFile(bak)
	if !equalByteSlices(got, data) {
		t.Fatal("backup bị từ chối không được ghi đè")
	}

	// Same bytes, private mode: verified, never rewritten.
	if err := os.Chmod(bak, 0o600); err != nil {
		t.Fatal(err)
	}
	receipts, err := storageimport.EnsureSourceBackups([]storageimport.BackupSource{bs})
	if err != nil {
		t.Fatalf("backup 0600 hợp lệ phải verify được: %v", err)
	}
	if receipts[0].Created {
		t.Fatal("backup đã có phải verify, không tạo lại")
	}
}

// ── source id rules ─────────────────────────────────────────────────────────

func TestSSOEqualSourceIDsRefused(t *testing.T) {
	same := noverseSource(absFixture(t, fixtureNoverse))
	vid := viderseSource(absFixture(t, fixtureViderse))
	vid.SourceID = strings.ToUpper(same.SourceID) // same UUID, other case
	plan := planSources(t, nil, same, vid, nil)
	if plan.Fatal == "" {
		t.Fatalf("hai nguồn cùng source id phải bị từ chối: %+v", plan)
	}
}

func TestSSOReusedSourceIDRefused(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	insertRun := func(sourceID, label string) {
		t.Helper()
		manifestSHA := sha256.Sum256([]byte(label))
		if _, err := pool.Pool().Exec(ctx, `
INSERT INTO import_runs (id, source_id, manifest_sha256, status, importer_version, source_label)
VALUES ($1, $2, $3, 'verified', 'test', $4)`,
			storageimport.DeterministicUUID(sourceID, "probe", label), sourceID,
			hex.EncodeToString(manifestSHA[:]), label); err != nil {
			t.Fatal(err)
		}
	}
	insertRun(viderseSourceUUID, "khac-app-library")
	plan := planSources(t, pool,
		noverseSource(absFixture(t, fixtureNoverse)),
		viderseSource(absFixture(t, fixtureViderse)), nil)
	if !contains(collisionKinds(plan), "source_id_reuse") {
		t.Fatalf("id tái dùng từ nguồn khác phải là collision: %+v %q", plan.Collisions, plan.Fatal)
	}
	// The labels this exact source id legitimately writes are allowed —
	// identity run plus the library-pass label.
	ownID := "11111111-2222-3333-4444-555555555555"
	insertRun(ownID, "noverse-identity")
	insertRun(ownID, "library")
	nov := noverseSource(absFixture(t, fixtureNoverse))
	nov.SourceID, nov.Label = ownID, "library"
	plan = planSources(t, pool, nov, emptyViderse(), nil)
	if contains(collisionKinds(plan), "source_id_reuse") {
		t.Fatalf("nhãn của chính nguồn này không được coi là reuse: %+v", plan.Collisions)
	}
}

func TestSSOBackupCandidatesMustBeSafeAndComplete(t *testing.T) {
	pool, dsn := openCutoverDBDSN(t)
	ctx := context.Background()
	schema := pgtest.SchemaOf(dsn)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "complete.dump")
	pgDump(t, "--format=custom", "--schema="+schema, "--file="+good, plainDumpDSN(t))
	t.Run("symlink", func(t *testing.T) {
		links := t.TempDir()
		if err := os.Chmod(links, 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(links, "linked.dump")
		if err := os.Symlink(good, link); err != nil {
			t.Fatal(err)
		}
		if _, err := storageimport.CheckBackupDir(ctx, pool, links); err == nil ||
			!strings.Contains(err.Error(), link) || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink phải bị từ chối kèm lý do: %v", err)
		}
	})
	t.Run("group writable directory", func(t *testing.T) {
		if err := os.Chmod(dir, 0o770); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(dir, 0o700)
		if _, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || !strings.Contains(err.Error(), "group/other") {
			t.Fatalf("thư mục group ghi được phải bị từ chối: %v", err)
		}
	})
	t.Run("group writable archive", func(t *testing.T) {
		if err := os.Chmod(good, 0o660); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(good, 0o600)
		if _, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || !strings.Contains(err.Error(), good) {
			t.Fatalf("archive group ghi được phải bị từ chối: %v", err)
		}
	})
	t.Run("readable directory", func(t *testing.T) {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if backup, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || backup != nil {
			t.Fatal("thư mục 0755 phải bị từ chối")
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := storageimport.CheckBackupDir(ctx, pool, dir); err != nil {
			t.Fatalf("thư mục sau chmod 0700 phải hợp lệ: %v", err)
		}
	})
	t.Run("readable archive", func(t *testing.T) {
		if err := os.Chmod(good, 0o644); err != nil {
			t.Fatal(err)
		}
		if backup, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || backup != nil ||
			!strings.Contains(err.Error(), good) {
			t.Fatalf("dump 0644 phải bị từ chối: %v", err)
		}
		if err := os.Chmod(good, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := storageimport.CheckBackupDir(ctx, pool, dir); err != nil {
			t.Fatalf("dump sau chmod 0600 phải hợp lệ: %v", err)
		}
	})
	t.Run("users only", func(t *testing.T) {
		partialDir := t.TempDir()
		if err := os.Chmod(partialDir, 0o700); err != nil {
			t.Fatal(err)
		}
		partial := filepath.Join(partialDir, "users-only.dump")
		pgDump(t, "--format=custom", "--table="+schema+".users", "--file="+partial, plainDumpDSN(t))
		if _, err := storageimport.CheckBackupDir(ctx, pool, partialDir); err == nil ||
			!strings.Contains(err.Error(), partial) || !strings.Contains(err.Error(), "TABLE DATA") {
			t.Fatalf("dump chỉ users phải bị từ chối: %v", err)
		}
	})
}

func TestSSOBackupChoosesNewestPassingCandidate(t *testing.T) {
	pool, dsn := openCutoverDBDSN(t)
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "a-old.dump")
	newest := filepath.Join(dir, "z-new.dump")
	pgDump(t, "--format=custom", "--schema="+pgtest.SchemaOf(dsn), "--file="+old, plainDumpDSN(t))
	pgDump(t, "--format=custom", "--schema="+pgtest.SchemaOf(dsn), "--file="+newest, plainDumpDSN(t))
	setMtime := func(path string, mtime time.Time) {
		t.Helper()
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	oldMtime := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	newMtime := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	setMtime(old, oldMtime)
	setMtime(newest, newMtime)
	last, err := storageimport.CheckBackupDir(ctx, pool, dir)
	if err != nil || last == nil || last.Path != newest {
		t.Fatalf("phải chọn dump có mtime mới nhất: %+v, %v", last, err)
	}
	setMtime(old, newMtime)
	last, err = storageimport.CheckBackupDir(ctx, pool, dir)
	if err != nil || last == nil || last.Path != old {
		t.Fatalf("mtime bằng nhau phải chọn đường dẫn tăng dần: %+v, %v", last, err)
	}
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('66666666-6666-4666-8666-666666666666','ghi-sau','Ghi sau','member','active')`); err != nil {
		t.Fatal(err)
	}
	if backup, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || backup != nil ||
		!strings.Contains(err.Error(), old) || !strings.Contains(err.Error(), newest) || !strings.Contains(err.Error(), "users") {
		t.Fatalf("cả hai dump không khớp dữ liệu phải bị từ chối: %v", err)
	}
	matching := filepath.Join(dir, "matching.dump")
	pgDump(t, "--format=custom", "--schema="+pgtest.SchemaOf(dsn), "--file="+matching, plainDumpDSN(t))
	setMtime(matching, oldMtime)
	last, err = storageimport.CheckBackupDir(ctx, pool, dir)
	if err != nil || last == nil || last.Path != matching {
		t.Fatalf("phải bỏ qua dump có mtime mới hơn nhưng không khớp: %+v, %v", last, err)
	}
}

func TestSSOSourceIDCaseUsesSameIdentityAndLock(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	nov := noverseSource(absFixture(t, fixtureNoverse))
	lower := planSources(t, pool, nov, emptyViderse(), nil)
	nov.SourceID = strings.ToUpper(nov.SourceID)
	upper := planSources(t, pool, nov, emptyViderse(), nil)
	if upper.Failed() || upper.Sources()[0].SourceID != noverseSourceUUID ||
		testManifestSHA(lower) != testManifestSHA(upper) {
		t.Fatalf("UUID phải canonical ngay khi decode: %+v", upper)
	}
	conn, err := pool.Pool().Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key := fnvLockKey(noverseSourceUUID)
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		conn.Release()
		t.Fatal(err)
	}
	backup := dumpForTest(t, pool)
	_, applyErr := storageimport.ApplyCutover(ctx, pool, upper, backup, testManifestSHA(upper), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now())
	_, unlockErr := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, key)
	conn.Release()
	if unlockErr != nil {
		t.Fatal(unlockErr)
	}
	if applyErr == nil || !strings.Contains(applyErr.Error(), "importer khác") {
		t.Fatalf("UUID hoa phải dùng cùng lock nguồn: %v", applyErr)
	}
	applyForTest(t, pool, upper)
	var id string
	if err := pool.Pool().QueryRow(ctx, `SELECT id::text FROM users WHERE username='quan-tri'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != storageimport.DeterministicUUID(noverseSourceUUID, "user", "quan-tri") {
		t.Fatalf("id hoa khác id thường: %s", id)
	}
	nov.SourceID = noverseSourceUUID
	resume := planSources(t, pool, nov, emptyViderse(), nil)
	if res := applyForTest(t, pool, resume); res.Inserted != 0 || res.Skipped != 7 {
		t.Fatalf("SQL/ledger phải cùng canonical UUID: %+v", res)
	}
}

func TestSSOApplyRechecksApprovedAdminsInsideTransaction(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	plan := planSources(t, pool, noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	backup := dumpForTest(t, pool)
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('66666666-6666-4666-8666-666666666666','admin-moi','Admin mới','admin','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now()); err == nil ||
		!strings.Contains(err.Error(), "admin nền tảng") {
		t.Fatalf("admin đổi sau duyệt phải rollback: %v", err)
	}
	var users, runs int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE username <> 'admin-moi'`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM import_runs`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if users != 0 || runs != 0 {
		t.Fatalf("rollback phải không để lại user/run: %d/%d", users, runs)
	}
}

func TestSSOBackupRejectsChangedTableContent(t *testing.T) {
	cases := []struct {
		name   string
		table  string
		seed   string
		mutate string
	}{
		{
			name:  "timestamped users update",
			table: "users",
			seed: `
INSERT INTO users (id, username, display_name, global_role, status, created_at, updated_at)
VALUES ('22222222-2222-4222-8222-222222222222', 'cu-song', 'Cũ', 'member', 'active',
        '2000-01-01T00:00:00Z', '2000-01-01T00:00:00Z')`,
			mutate: `
UPDATE users SET display_name='Đã đổi', updated_at='2001-01-01T00:00:00Z', version=version+1
WHERE username='cu-song'`,
		},
		{
			name:  "sessions delete changes rowcount only",
			table: "sessions",
			seed: `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('22222222-2222-4222-8222-222222222222', 'cu-song', 'Cũ', 'member', 'active');
INSERT INTO devices (id, user_id, name)
VALUES ('33333333-3333-4333-8333-333333333333', '22222222-2222-4222-8222-222222222222', 'laptop');
INSERT INTO sessions (id, user_id, device_id, token_hash, credential_kind, created_at, updated_at, last_seen_at, expires_at)
VALUES ('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222',
        '33333333-3333-4333-8333-333333333333', sha256('session-one'::bytea), 'password',
        '2000-01-01T00:00:00Z', '2000-01-01T00:00:00Z', '2000-01-01T00:00:00Z', '2100-01-01T00:00:00Z'),
       ('55555555-5555-4555-8555-555555555555', '22222222-2222-4222-8222-222222222222',
        '33333333-3333-4333-8333-333333333333', sha256('session-two'::bytea), 'password',
        '2000-01-01T00:00:00Z', '2000-01-01T00:00:00Z', '2000-01-01T00:00:00Z', '2100-01-01T00:00:00Z')`,
			mutate: `DELETE FROM sessions WHERE id='44444444-4444-4444-8444-444444444444'`,
		},
		{
			name:  "nonaccount insert",
			table: "projects",
			mutate: `
INSERT INTO projects (id, slug, name)
VALUES ('77777777-7777-4777-8777-777777777777', 'ghi-sau', 'Dự án mới')`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := openCutoverDB(t)
			ctx := context.Background()
			if tc.seed != "" {
				if _, err := pool.Pool().Exec(ctx, tc.seed); err != nil {
					t.Fatalf("chuẩn bị dữ liệu: %v", err)
				}
			}
			backup := dumpForTest(t, pool)
			if _, err := pool.Pool().Exec(ctx, tc.mutate); err != nil {
				t.Fatalf("thay đổi dữ liệu sau dump: %v", err)
			}
			if got, err := storageimport.CheckBackupDir(ctx, pool, filepath.Dir(backup.Path)); err == nil || got != nil ||
				!strings.Contains(err.Error(), tc.table) {
				t.Fatalf("dump trước thay đổi bảng %s phải bị từ chối: %v", tc.table, err)
			}
		})
	}
}

func TestSSOBackupMatchingContentAcceptsDumpTimezones(t *testing.T) {
	for _, zone := range []string{"America/St_Johns", "UTC", "Asia/Tokyo"} {
		t.Run(zone, func(t *testing.T) {
			t.Setenv("PGTZ", zone)
			pool := openCutoverDB(t)
			ctx := context.Background()
			if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status, onboarded_at, created_at, updated_at)
VALUES ('22222222-2222-4222-8222-222222222222', 'cu-song', 'Cũ', 'member', 'active',
        '2000-06-01T12:34:56.123456-02:30', '2000-01-01T12:34:56.654321+09:00', '2001-01-01T12:34:56.123456Z')`); err != nil {
				t.Fatalf("chuẩn bị timestamp: %v", err)
			}
			plan := planSources(t, pool, noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
			backup := dumpForTest(t, pool)
			if _, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC()); err != nil {
				t.Fatalf("dump khớp dữ liệu với PGTZ=%s phải được chấp nhận: %v", zone, err)
			}
		})
	}
}

func TestSSOBackupRejectsInsertFormats(t *testing.T) {
	pool, dsn := openCutoverDBDSN(t)
	ctx := context.Background()
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('22222222-2222-4222-8222-222222222222', 'cu-song', 'Cũ', 'member', 'active')`); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--inserts", "--column-inserts"} {
		t.Run(flag, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "insert.dump")
			pgDump(t, "--format=custom", flag, "--schema="+pgtest.SchemaOf(dsn), "--file="+path, plainDumpDSN(t))
			if backup, err := storageimport.CheckBackupDir(ctx, pool, dir); err == nil || backup != nil ||
				!strings.Contains(err.Error(), path) {
				t.Fatalf("dump %s phải bị từ chối: %v", flag, err)
			}
		})
	}
}

func TestSSOApplyRechecksBackupContentInsideTransaction(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	plan := planSources(t, pool, noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	backup := dumpForTest(t, pool)
	if _, err := pool.Pool().Exec(ctx, `
INSERT INTO projects (id, slug, name)
VALUES ('77777777-7777-4777-8777-777777777777', 'ghi-sau', 'Dự án mới')`); err != nil {
		t.Fatal(err)
	}
	if _, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC()); err == nil ||
		!strings.Contains(err.Error(), "projects") || !strings.Contains(err.Error(), "giao dịch") {
		t.Fatalf("ghi ngoài tài khoản sau kiểm tra backup phải bị từ chối trong giao dịch: %v", err)
	}
	assertNoUsers(t, pool)
	var runs int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM import_runs`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("rollback phải không để lại import run: %d", runs)
	}
	var projects int
	if err := pool.Pool().QueryRow(ctx, `SELECT count(*) FROM projects`).Scan(&projects); err != nil {
		t.Fatal(err)
	}
	if projects != 1 {
		t.Fatalf("rollback không được xóa lần ghi trước giao dịch: %d", projects)
	}
}

func TestSSOApplyRefusesActiveLocksBeforeImportWrites(t *testing.T) {
	for _, tc := range []struct {
		name   string
		query  string
		table  string
		within time.Duration
	}{
		{"open writer", `INSERT INTO "cutover "" writer" VALUES ('private-data-must-not-appear')`, `"cutover "" writer"`, 2 * time.Second},
		{"invariant row", `SELECT id FROM auth_invariant_lock WHERE id=1 FOR UPDATE`, `"auth_invariant_lock"`, 2 * time.Second},
		{"maintenance table lock", `LOCK TABLE "cutover "" writer" IN SHARE UPDATE EXCLUSIVE MODE`, `"cutover "" writer"`, 2 * time.Second},
		// NOWAIT does not cover a table lock the statement queues behind;
		// the bounded lock wait must refuse it the same way.
		{"invariant table lock", `LOCK TABLE auth_invariant_lock IN ACCESS EXCLUSIVE MODE`, `"auth_invariant_lock"`, 4 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := openCutoverDB(t)
			ctx := context.Background()
			if _, err := pool.Pool().Exec(ctx, `CREATE TABLE "cutover "" writer" (value text)`); err != nil {
				t.Fatal(err)
			}
			plan := planSources(t, pool, noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
			backup := dumpForTest(t, pool)
			writer, err := pool.Pool().Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback(ctx)
			if _, err := writer.Exec(ctx, tc.query); err != nil {
				t.Fatal(err)
			}
			applyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			started := time.Now()
			_, err = storageimport.ApplyCutover(applyCtx, pool, plan, backup, testManifestSHA(plan),
				filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC())
			if err == nil || !strings.Contains(err.Error(), tc.table) ||
				!strings.Contains(err.Error(), "dừng mọi app/worker kết nối database rồi thử lại") ||
				!strings.Contains(err.Error(), "autovacuum/ANALYZE") ||
				!strings.Contains(err.Error(), "pg_locks/pg_stat_activity") ||
				strings.Contains(err.Error(), "private-data-must-not-appear") {
				t.Fatalf("khoá đang giữ phải bị từ chối với tên bảng và hướng dẫn xử lý: %v", err)
			}
			if elapsed := time.Since(started); elapsed >= tc.within {
				t.Fatalf("khoá đang giữ phải bị từ chối trong %v, mất %v: %v", tc.within, elapsed, err)
			}
			assertNoUsers(t, pool)
			var writes int
			if err := pool.Pool().QueryRow(ctx, `
SELECT (SELECT count(*) FROM import_runs) + (SELECT count(*) FROM import_items) +
       (SELECT count(*) FROM app_grants)`).Scan(&writes); err != nil {
				t.Fatal(err)
			}
			if writes != 0 {
				t.Fatalf("khoá đang giữ không được để lại lần ghi import nào: %d", writes)
			}
		})
	}
}

func TestSSOBackupCountsInheritanceTablesSeparately(t *testing.T) {
	pool := openCutoverDB(t)
	ctx := context.Background()
	if _, err := pool.Pool().Exec(ctx, `
CREATE TABLE backup_parent (id integer, changed_at timestamptz);
CREATE TABLE backup_child () INHERITS (backup_parent);
INSERT INTO backup_parent VALUES (1, '2000-01-01T00:00:00Z');
INSERT INTO backup_child VALUES (2, '2001-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	plan := planSources(t, pool, noverseSource(absFixture(t, fixtureNoverse)), emptyViderse(), nil)
	backup := dumpForTest(t, pool)
	if _, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan),
		filepath.Join(filepath.Dir(backup.Path), "inventory.json"), time.Now().UTC()); err != nil {
		t.Fatalf("COPY và aggregate phải chỉ đếm dòng riêng từng bảng kế thừa: %v", err)
	}
}

func TestSSOCommittedIdentityRecordsRollbackBackup(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "imported sources"
		if empty {
			name = "empty sources"
		}
		t.Run(name, func(t *testing.T) {
			pool := openCutoverDB(t)
			ctx := context.Background()
			source := noverseSource(absFixture(t, fixtureNoverse))
			if empty {
				source = storageimport.IdentitySource{App: storageimport.SourceNoverse, SourceID: noverseSourceUUID, Empty: true}
				if _, err := pool.Pool().Exec(ctx, `
INSERT INTO users (id, username, display_name, global_role, status)
VALUES ('22222222-2222-4222-8222-222222222222', 'admin-cu', 'Admin', 'admin', 'active')`); err != nil {
					t.Fatal(err)
				}
			}
			var firstRunID, firstInventory, firstDumpSHA string
			for attempt := range 2 {
				plan := planSources(t, pool, source, emptyViderse(), nil)
				backup := dumpForTest(t, pool)
				inventory := filepath.Join(filepath.Dir(backup.Path), "inventory.json")
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				relativeInventory, err := filepath.Rel(cwd, inventory)
				if err != nil {
					t.Fatal(err)
				}
				absoluteDump := backup.Path
				backup.Path, err = filepath.Rel(cwd, backup.Path)
				if err != nil {
					t.Fatal(err)
				}
				result, err := storageimport.ApplyCutover(ctx, pool, plan, backup, testManifestSHA(plan), relativeInventory, time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
				var recordedInventory, recordedDump, recordedSHA, status string
				if err := pool.Pool().QueryRow(ctx, `
SELECT summary->'cutoverBackup'->>'inventoryPath',
       summary->'cutoverBackup'->>'dumpPath',
       summary->'cutoverBackup'->>'dumpSha256', status
FROM import_runs WHERE id=$1 AND source_label='sso-cutover-identity'`,
					result.CutoverRunID).Scan(&recordedInventory, &recordedDump, &recordedSHA, &status); err != nil {
					t.Fatal(err)
				}
				if recordedInventory != inventory || recordedDump != absoluteDump || recordedSHA != backup.SHA256 || status != "verified" {
					t.Fatalf("bản ghi commit không khớp backup: %s %s %s %s", recordedInventory, recordedDump, recordedSHA, status)
				}
				sourceID := storageimport.DeterministicUUID("verse", "sso-cutover-identity", inventory)
				if result.CutoverRunID != storageimport.RunIDFor(sourceID, testManifestSHA(plan)) {
					t.Fatal("run ID phải dùng đường inventory tuyệt đối")
				}
				if attempt == 0 {
					firstRunID, firstInventory, firstDumpSHA = result.CutoverRunID, inventory, backup.SHA256
				} else if result.CutoverRunID == firstRunID {
					t.Fatal("resume phải có bản ghi commit riêng")
				}
			}
			var inventory, dumpSHA string
			if err := pool.Pool().QueryRow(ctx, `
SELECT summary->'cutoverBackup'->>'inventoryPath', summary->'cutoverBackup'->>'dumpSha256'
FROM import_runs WHERE id=$1`, firstRunID).Scan(&inventory, &dumpSHA); err != nil {
				t.Fatal(err)
			}
			if inventory != firstInventory || dumpSHA != firstDumpSHA {
				t.Fatal("resume không được ghi đè điểm rollback của transaction đã commit trước")
			}
		})
	}
}
