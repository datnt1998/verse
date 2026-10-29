package webauth

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var recoveryFormat = regexp.MustCompile(`^[A-Z0-9]{4}(-[A-Z0-9]{4}){3}$`)

func TestIssueRecoveryFormatAndStorage(t *testing.T) {
	s, c := newTestStore(t)
	code, created, err := s.IssueRecovery("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	if !recoveryFormat.MatchString(code) || !created.Equal(c.t) {
		t.Fatalf("code %q created %v", code, created)
	}
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), code) || strings.Contains(string(raw), strings.ReplaceAll(code, "-", "")) {
		t.Fatal("recovery code stored in plaintext")
	}
	if !strings.Contains(string(raw), `"$argon2id$`) {
		t.Fatal("recovery code must be stored with the password hash scheme")
	}
	st, err := s.RecoveryStatus("admin")
	if err != nil || st.CreatedAt == nil || !st.CreatedAt.Equal(c.t) || st.Used {
		t.Fatalf("status: %+v %v", st, err)
	}
}

func TestRecoveryStatusBeforeIssue(t *testing.T) {
	s, _ := newTestStore(t)
	st, err := s.RecoveryStatus("admin")
	if err != nil || st.CreatedAt != nil || st.Used {
		t.Fatalf("status without code: %+v %v", st, err)
	}
}

func TestIssueRecoveryRules(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.CreateUser(NewUser{Username: "writer", Role: RoleMember, Password: "writer-pass"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.IssueRecovery("writer", false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin issued a code: %v", err)
	}
	first, _, err := s.IssueRecovery("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.IssueRecovery("admin", false); !errors.Is(err, ErrRecoveryExists) {
		t.Fatalf("second issue while unused: %v", err)
	}
	second, _, err := s.IssueRecovery("admin", true)
	if err != nil || second == first {
		t.Fatalf("rotate: %q %v", second, err)
	}
	if _, _, err := s.Recover("admin", first, "new-password-1", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("rotated-away code still works: %v", err)
	}
	if _, _, err := s.Recover("admin", second, "new-password-1", "Mac", "1.1.1.1"); err != nil {
		t.Fatalf("rotated code: %v", err)
	}
	// 用掉之后可以直接再生成（不需要 rotate）。
	if _, _, err := s.IssueRecovery("admin", false); err != nil {
		t.Fatalf("issue after use: %v", err)
	}
}

func TestRecoverSuccessRevokesSessionsAndBurnsCode(t *testing.T) {
	s, _ := newTestStore(t)
	old, err := s.Login("admin", "admin-pass", "Mac", "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	pairCode, _, err := s.NewPairing("admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	code, _, err := s.IssueRecovery("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	// 容错：小写、无连字符、首尾空格。
	typed := " " + strings.ToLower(strings.ReplaceAll(code, "-", "")) + " "
	ns, revoked, err := s.Recover(" admin ", typed, "brand-new-pass", "Phone", "2.2.2.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(revoked) != 1 {
		t.Fatalf("revoked sessions: %v", revoked)
	}
	if _, _, err := s.Authenticate(old.Cookie); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("old session survived recovery: %v", err)
	}
	if u, _, err := s.Authenticate(ns.Cookie); err != nil || u.Username != "admin" {
		t.Fatalf("new session: %v %+v", err, u)
	}
	if _, err := s.Login("admin", "admin-pass", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password still works: %v", err)
	}
	if _, err := s.Login("admin", "brand-new-pass", "Mac", "1.1.1.1"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	if _, err := s.Pair(pairCode, "Phone", "2.2.2.2"); !errors.Is(err, ErrInvalidPairing) {
		t.Fatalf("pending pairing code survived recovery: %v", err)
	}
	st, _ := s.RecoveryStatus("admin")
	if !st.Used {
		t.Fatalf("code not marked used: %+v", st)
	}
	if _, _, err := s.Recover("admin", code, "another-pass-1", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("used code accepted: %v", err)
	}
}

func TestRecoverRejections(t *testing.T) {
	s, _ := newTestStore(t)
	code, _, err := s.IssueRecovery("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	other, err := newRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct{ user, code string }{
		"wrong code":   {"admin", formatRecoveryCode(other)},
		"unknown user": {"ghost", code},
		"empty code":   {"admin", ""},
		"bad charset":  {"admin", "!!!!-!!!!-!!!!-!!!!"},
		"truncated":    {"admin", code[:len(code)-1]},
	} {
		if _, _, err := s.Recover(tc.user, tc.code, "brand-new-pass", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidRecovery) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// 密码太短是 ErrInvalidInput，并且不会烧掉码。
	if _, _, err := s.Recover("admin", code, "short", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("short password: %v", err)
	}
	if _, _, err := s.Recover("admin", code, "brand-new-pass", "Mac", "1.1.1.1"); err != nil {
		t.Fatalf("code burned by rejected attempts: %v", err)
	}
}

func TestRecoverOnlyForEnabledAdmins(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.CreateUser(NewUser{Username: "second", Role: RoleAdmin, Password: "second-pass"}); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.IssueRecovery("second", false)
	if err != nil {
		t.Fatal(err)
	}
	role := RoleMember
	if _, err := s.UpdateUser("second", UserPatch{Role: &role}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Recover("second", code, "brand-new-pass", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("demoted account recovered: %v", err)
	}
	role = RoleAdmin
	disabled := true
	if _, err := s.UpdateUser("second", UserPatch{Role: &role, Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Recover("second", code, "brand-new-pass", "Mac", "1.1.1.1"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("disabled account recovered: %v", err)
	}
}

func TestRecoverIsSingleUseUnderRace(t *testing.T) {
	s, _ := newTestStore(t)
	code, _, err := s.IssueRecovery("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.Recover("admin", code, "brand-new-pass", "Mac", "1.1.1.1")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrInvalidRecovery) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("one-time code redeemed %d times", ok)
	}
}

func TestWindowLimiter(t *testing.T) {
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := NewWindowLimiter(5, time.Hour)
	l.now = c.now
	for i := range 5 {
		if wait := l.Wait("ip"); wait != 0 {
			t.Fatalf("attempt %d blocked early: %v", i+1, wait)
		}
		l.Fail("ip")
		c.t = c.t.Add(time.Minute)
	}
	// 第 1 次失败发生在 t0，已过 5 分钟，窗口内还差 55 分钟。
	if wait := l.Wait("ip"); wait != 55*time.Minute {
		t.Fatalf("6th attempt wait = %v, want 55m", wait)
	}
	if wait := l.Wait("other"); wait != 0 {
		t.Fatalf("limit leaked to another key: %v", wait)
	}
	c.t = c.t.Add(55 * time.Minute)
	if wait := l.Wait("ip"); wait != 0 {
		t.Fatalf("still blocked after the window slid: %v", wait)
	}
	l.Fail("ip")
	l.Succeed("ip")
	if got := len(l.hits["ip"]); got != 0 {
		t.Fatalf("Succeed left %d hits", got)
	}
}

// GAP-33/Dialogs › recovery: GET /api/me/recovery trả mask (TRAX-••••-••••-41ZN) khi còn mã chưa dùng.
func TestRecoveryStatusMask(t *testing.T) {
	s, _ := newTestStore(t)
	code, _, err := s.IssueRecovery("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	groups := strings.Split(code, "-")
	st, err := s.RecoveryStatus("admin")
	if err != nil {
		t.Fatal(err)
	}
	want := groups[0] + "-••••-••••-" + groups[3]
	if st.Mask != want {
		t.Fatalf("mask=%q, muốn %q", st.Mask, want)
	}
	if _, _, err := s.Recover("admin", code, "mat-khau-moi", "Mac", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	st, err = s.RecoveryStatus("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Used || st.Mask != "" {
		t.Fatalf("mã đã dùng mà status: used=%v mask=%q", st.Used, st.Mask)
	}
}
