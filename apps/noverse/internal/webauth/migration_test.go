package webauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAccountsV3ToV4Migration là bằng chứng an toàn cho bước tự di trú của file
// store khi serve khởi động lại: mọi user/role/grant/hash giữ nguyên, backup
// .v3.bak ghi đúng byte gốc, phiên cũ còn sống, login vẫn hoạt động và chạy
// lại trên file đã migrate là no-op. Fixture phản ánh shape file production
// (admin có recovery mask, member có grants + mustChangePassword, sessions/
// pairing/audit); hash argon2 là hash giả được sinh trong test, không phải
// hash thật — cùng shape, cùng đường đi verify.
//
// Live proof: chạy lại test này trên bản sao của file thật bằng
//
//	ACCOUNTS_V3_SRC=/tmp/acc-proof/web-accounts.json go test ./internal/webauth -run TestAccountsV3ToV4Migration
func TestAccountsV3ToV4Migration(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "web-accounts-src.json")
	if live := os.Getenv("ACCOUNTS_V3_SRC"); live != "" {
		raw, err := os.ReadFile(live)
		if err != nil {
			t.Fatalf("read live copy: %v", err)
		}
		if err := os.WriteFile(src, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		var head struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(raw, &head); err != nil || head.Version != 3 {
			t.Fatalf("live copy must be a v3 accounts file, got version %d (%v)", head.Version, err)
		}
	} else {
		buildV3Fixture(t, src)
	}
	proveMigration(t, src, filepath.Join(dir, "a"), filepath.Join(dir, "b"))
}

// buildV3Fixture writes a sanitized v3 file mirroring the real shape: admin
// with a recovery mask, a member with project grants and mustChangePassword,
// live + expired sessions, a pairing and an audit row.
func buildV3Fixture(t *testing.T, src string) {
	t.Helper()
	base := time.Date(2026, 10, 26, 14, 26, 0, 0, time.UTC)
	hash := func(p string) string {
		h, err := hashPassword(p)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	at := func(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }
	d := data{
		Version: 3,
		Users: []User{
			{
				Username: "admin", Name: "Đạt Nguyễn", Role: RoleAdmin,
				Projects:     map[string]ProjectRole{},
				PasswordHash: hash("orig-admin-pass"),
				OnboardedAt:  new(at(6)),
				Recovery: &RecoveryCode{
					CodeHash:  hash("recovery-AAAA-BBBB-CCCC"),
					Mask:      "2N2B-••••-••••-OBBK",
					CreatedAt: at(5),
				},
				CreatedAt: base, UpdatedAt: at(19),
			},
			{
				Username: "duyet", Name: "Duyệt Trần", Role: RoleMember,
				Projects:           map[string]ProjectRole{"my-story": ProjectCollab, "x": ProjectView},
				PasswordHash:       hash("temp-pass-2026"),
				MustChangePassword: true,
				PasswordChangedAt:  new(at(20)),
				CreatedAt:          at(1), UpdatedAt: at(20),
			},
		},
		Sessions: []session{
			{ID: "fb04ab5fc29da450", SecretHash: digest("live-secret-1"), Username: "admin", Device: "Mac · Chrome", IP: "127.0.0.1", CreatedAt: at(3), LastSeen: at(40), ExpiresAt: base.Add(30 * 24 * time.Hour)},
			{ID: "df886e9a2e6a38de", SecretHash: digest("live-secret-2"), Username: "admin", Device: "iPhone · Safari", IP: "100.91.185.77", CreatedAt: at(10), LastSeen: at(11), ExpiresAt: base.Add(30*24*time.Hour + time.Hour)},
			{ID: "0000dead0000beef", SecretHash: digest("stale-secret"), Username: "admin", Device: "Mac · Chrome", IP: "127.0.0.1", CreatedAt: at(3), LastSeen: at(4), ExpiresAt: at(5)},
		},
		Pairings: []pairing{
			{CodeHash: digest("pairing-code"), Username: "admin", CreatedBy: "admin", ExpiresAt: at(6)},
		},
		Audit: []AuditEntry{
			{ID: "00000000000000000001", At: at(45), Type: "perm", Action: "Di trú quyền"},
		},
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// proveMigration runs the full safety proof against the v3 file at src on two
// independent copies: A for byte-parity/backup/session/idempotency, B for the
// known-password login proof.
func proveMigration(t *testing.T, src, dirA, dirB string) {
	t.Helper()
	origRaw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	var orig data
	if err := json.Unmarshal(origRaw, &orig); err != nil {
		t.Fatalf("fixture unmarshal: %v", err)
	}
	pathA := filepath.Join(dirA, "web-accounts.json")
	pathB := filepath.Join(dirB, "web-accounts.json")
	for _, p := range []string{pathA, pathB} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, origRaw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// --- Copy A: migrate + byte parity + backup + session preservation ---
	sa := NewStore(pathA)
	if _, err := sa.Users(); err != nil {
		t.Fatalf("read after migration: %v", err)
	}
	migrated := sread(t, sa)
	if migrated.Version != fileVersion {
		t.Fatalf("version = %d, want %d", migrated.Version, fileVersion)
	}
	bak, err := os.ReadFile(pathA + ".v3.bak")
	if err != nil {
		t.Fatalf(".v3.bak missing: %v", err)
	}
	if !bytes.Equal(bak, origRaw) {
		t.Fatal(".v3.bak must be byte-equal to the original v3 file")
	}
	compareMigrated(t, &orig, migrated)

	// Phiên cũ còn sống: mọi session chưa hết hạn giữ nguyên secretHash —
	// cookie đang lưu trong trình duyệt tiếp tục authenticate được.
	sessions, err := sa.Sessions("", "")
	if err != nil {
		t.Fatal(err)
	}
	live := unexpiredSessions(&orig)
	if len(sessions) != live {
		t.Fatalf("surviving sessions = %d, want %d", len(sessions), live)
	}
	for _, x := range sessions {
		found := false
		for _, rec := range migrated.Sessions {
			if rec.ID == x.ID {
				found = true
				if rec.Username != x.Username {
					t.Fatalf("session %s owner = %s", x.ID, rec.Username)
				}
			}
		}
		if !found {
			t.Fatalf("session %s vanished", x.ID)
		}
	}

	// Idempotent: mở lại store trên file v4 → snapshot không ghi thêm,
	// backup không bị đụng; mọi ghi sau vẫn giữ v4.
	sa2 := NewStore(pathA)
	if _, err := sa2.Users(); err != nil {
		t.Fatal(err)
	}
	if _, err := sa2.MarkOnboarded(firstUser(&orig).Username); err != nil {
		t.Fatal(err)
	}
	bak2, err := os.ReadFile(pathA + ".v3.bak")
	if err != nil || !bytes.Equal(bak2, origRaw) {
		t.Fatalf("re-run must not touch .v3.bak: %v", err)
	}
	if sread(t, sa2).Version != fileVersion {
		t.Fatal("file must stay v4 after a post-migration write")
	}

	// --- Copy B: login bằng mật khẩu đã biết trên file đã migrate ---
	sb := NewStore(pathB)
	victim := firstUser(&orig).Username
	pass := "prove-migration-pass"
	if _, err := sb.UpdateUser(victim, UserPatch{Password: &pass}); err != nil {
		t.Fatalf("set test password: %v", err)
	}
	ns, err := sb.Login(victim, pass, "proof", "127.0.0.1")
	if err != nil {
		t.Fatalf("login on migrated store: %v", err)
	}
	user, sess, err := sb.Authenticate(ns.Cookie)
	if err != nil {
		t.Fatalf("authenticate new session: %v", err)
	}
	if user.Username != victim || sess.Username != victim {
		t.Fatalf("session/user mismatch: %s %s", user.Username, sess.Username)
	}
	if _, err := sb.Login(victim, "wrong-wrong", "proof", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password = %v", err)
	}
}

// compareMigrated asserts every record survives v3→v4 byte-equal except the
// documented mustChangePassword transform (passwordHash → tempPassword.codeHash).
func compareMigrated(t *testing.T, orig, got *data) {
	t.Helper()
	if len(got.Users) != len(orig.Users) {
		t.Fatalf("users = %d, want %d", len(got.Users), len(orig.Users))
	}
	for i := range orig.Users {
		before, after := &orig.Users[i], &got.Users[i]
		b, g := jsonClone(before), jsonClone(after)
		// Trường hợp duy nhất được phép đổi: mustChangePassword → temp.
		if before.MustChangePassword && before.PasswordHash != "" && before.TempPassword == nil {
			if b["mustChangePassword"] != true {
				t.Fatalf("user %s lost mustChangePassword", before.Username)
			}
			temp, _ := g["tempPassword"].(map[string]any)
			if temp == nil || temp["codeHash"] != b["passwordHash"] {
				t.Fatalf("user %s temp password must reuse the old hash", before.Username)
			}
			if g["passwordHash"] != "" {
				t.Fatalf("user %s permanent hash must be cleared", before.Username)
			}
			base := before.UpdatedAt
			if before.PasswordChangedAt != nil {
				base = *before.PasswordChangedAt
			}
			exp, _ := temp["expiresAt"].(string)
			want := base.Add(TempPasswordTTL).UTC().Format(time.RFC3339Nano)
			expT, _ := time.Parse(time.RFC3339Nano, exp)
			if !expT.Equal(base.Add(TempPasswordTTL)) {
				t.Fatalf("user %s temp expiresAt = %s, want %s", before.Username, exp, want)
			}
			delete(b, "passwordHash")
			delete(g, "passwordHash")
			delete(g, "tempPassword")
		}
		if !jsonEqual(b, g) {
			t.Fatalf("user %s changed during migration:\n%v\n→\n%v", before.Username, b, g)
		}
	}
	wantSessions := make([]session, 0, len(orig.Sessions))
	for _, x := range orig.Sessions {
		if x.ExpiresAt.After(time.Now()) {
			wantSessions = append(wantSessions, x)
		}
	}
	if !jsonEq(wantSessions, got.Sessions) {
		t.Fatalf("live sessions changed: %d → %d", len(wantSessions), len(got.Sessions))
	}
	wantPairings := make([]pairing, 0, len(orig.Pairings))
	for _, p := range orig.Pairings {
		if p.ExpiresAt.After(time.Now()) {
			wantPairings = append(wantPairings, p)
		}
	}
	if !jsonEq(wantPairings, got.Pairings) {
		t.Fatalf("pairings changed: %d → %d", len(wantPairings), len(got.Pairings))
	}
	if !jsonEq(orig.Audit, got.Audit) {
		t.Fatalf("audit rows changed: %d → %d", len(orig.Audit), len(got.Audit))
	}
}

func sread(t *testing.T, s *Store) *data {
	t.Helper()
	d, err := s.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func unexpiredSessions(d *data) int {
	n := 0
	for _, x := range d.Sessions {
		if x.ExpiresAt.After(time.Now()) {
			n++
		}
	}
	return n
}

func firstUser(d *data) *User {
	for i := range d.Users {
		if d.Users[i].DeletedAt == nil && !d.Users[i].Disabled {
			return &d.Users[i]
		}
	}
	return nil
}

func jsonClone(v any) map[string]any {
	raw, _ := json.Marshal(v)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		panic(fmt.Sprintf("clone %v: %v", v, err))
	}
	return m
}

func jsonEqual(a, b map[string]any) bool { return jsonEq(a, b) }

func jsonEq(a, b any) bool {
	araw, _ := json.Marshal(a)
	braw, _ := json.Marshal(b)
	return bytes.Equal(araw, braw)
}
