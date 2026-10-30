// Ca test cho cookie YouTube từ hồ sơ trình duyệt (port ý của
// src/server/tools/cookies.ts của yt-factory): liệt kê hồ sơ chỉ-đọc theo thứ
// tự tự nhiên, cache 60 giây, và chỉ lưu sau khi yt-dlp đọc được cookie.
package accounts_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/accounts"
)

// memSettings là kho cài đặt trong bộ nhớ cho test.
type memSettings map[string]string

func (m memSettings) Get(_ context.Context, key string, out any) (bool, error) {
	raw, ok := m[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, err
	}
	return true, nil
}

func (m memSettings) Set(_ context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m[key] = string(raw)
	return nil
}

func (m memSettings) Clear(_ context.Context, key string) error {
	delete(m, key)
	return nil
}

// probeFn là YouTubeProbe giả.
type probeFn func(ctx context.Context, browser, profile string) error

func (f probeFn) Check(ctx context.Context, browser, profile string) error {
	return f(ctx, browser, profile)
}

type ytEnv struct {
	t     *testing.T
	home  string
	now   *time.Time
	set   memSettings
	probe probeFn
}

func newYTEnv(t *testing.T) *ytEnv {
	t.Helper()
	e := &ytEnv{t: t, home: t.TempDir(), now: new(time.Time), set: memSettings{}}
	*e.now = time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	e.probe = func(context.Context, string, string) error { return nil }
	writeLocalState(t, e.home, "Google/Chrome", map[string][2]string{
		"Default":    {"Cá nhân", "phu@example.com"},
		"Profile 2":  {"Công việc", "work@example.com"},
		"Profile 10": {"Dự án", ""},
	}, "Default", "Profile 2", "Profile 10")
	writeLocalState(t, e.home, "BraveSoftware/Brave-Browser", map[string][2]string{
		"Default": {"Brave", ""},
	}, "Default")
	return e
}

func writeLocalState(t *testing.T, home, rel string, profiles map[string][2]string, dirs ...string) {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cache := map[string]map[string]string{}
	for name, p := range profiles {
		cache[name] = map[string]string{"name": p[0], "user_name": p[1]}
	}
	state, err := json.Marshal(map[string]any{"profile": map[string]any{"info_cache": cache}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Local State"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func (e *ytEnv) service() *accounts.Service {
	return accounts.New(accounts.Deps{
		Dir:      filepath.Join(e.home, ".viderse"),
		Settings: e.set,
		Probe:    probeFn(func(ctx context.Context, browser, profile string) error { return e.probe(ctx, browser, profile) }),
		Home:     e.home,
		GOOS:     "darwin",
		Now:      func() time.Time { return *e.now },
	})
}

func TestListBrowserProfilesOnlyInstalledWithLocalState(t *testing.T) {
	e := newYTEnv(t)
	profiles, err := e.service().Profiles(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range profiles {
		got = append(got, p.Browser+":"+p.Profile)
	}
	want := []string{
		"chrome:Default", "chrome:Profile 2", "chrome:Profile 10",
		"brave:Default",
	}
	if len(got) != len(want) {
		t.Fatalf("profiles = %v, muốn %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("profiles = %v, muốn %v", got, want)
		}
	}
	if profiles[0].Name != "Cá nhân" || profiles[0].Email != "phu@example.com" {
		t.Fatalf("hồ sơ đầu = %+v", profiles[0])
	}
	// Thư mục bị xoá thì hồ sơ đó không còn.
	if err := os.RemoveAll(filepath.Join(e.home, "Library", "Application Support", "Google/Chrome/Profile 2")); err != nil {
		t.Fatal(err)
	}
	profiles, err = e.service().Profiles(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range profiles {
		if p.Profile == "Profile 2" {
			t.Fatal("hồ sơ đã xoá vẫn còn")
		}
	}
}

// TestEmptyProfileListIsCachedToo kiểm danh sách rỗng cũng cache đúng — máy
// không cài Chromium thì không phải duyệt lại các user-data dir mỗi lượt hỏi
// (L2 của review).
func TestEmptyProfileListIsCachedToo(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	svc := accounts.New(accounts.Deps{
		Dir:  filepath.Join(home, ".viderse"),
		Home: home,
		GOOS: "darwin",
		Now:  func() time.Time { return now },
	})
	first, err := svc.Profiles(context.Background(), false)
	if err != nil || len(first) != 0 {
		t.Fatalf("lượt đầu = %v %v", first, err)
	}
	// Cài Chrome SAU lượt đầu: cache rỗng phải vẫn được trả trong TTL.
	writeLocalState(t, home, "Google/Chrome", map[string][2]string{"Default": {"Cá nhân", ""}}, "Default")
	second, err := svc.Profiles(context.Background(), false)
	if err != nil || len(second) != 0 {
		t.Fatalf("cache rỗng phải được dùng, được %v %v", second, err)
	}
	// refresh=1 ép quét lại thấy hồ sơ mới.
	fresh, err := svc.Profiles(context.Background(), true)
	if err != nil || len(fresh) != 1 {
		t.Fatalf("refresh = %v %v", fresh, err)
	}
}

func TestProfilesAreCachedForAMinute(t *testing.T) {
	e := newYTEnv(t)
	svc := e.service()
	if _, err := svc.Profiles(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(e.home, "Library", "Application Support", "BraveSoftware")); err != nil {
		t.Fatal(err)
	}
	cached, err := svc.Profiles(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != 4 {
		t.Fatalf("trong 60 giây phải dùng cache, được %d", len(cached))
	}
	*e.now = e.now.Add(61 * time.Second)
	fresh, err := svc.Profiles(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh) != 3 {
		t.Fatalf("sau TTL phải quét lại, được %d", len(fresh))
	}
}

func TestSetCookieSourceValidatesAndProbes(t *testing.T) {
	ctx := context.Background()
	e := newYTEnv(t)
	svc := e.service()

	var probed string
	e.probe = func(_ context.Context, browser, profile string) error {
		probed = browser + ":" + profile
		return nil
	}
	if _, err := svc.SetCookieSource(ctx, "firefox", "Default"); err == nil {
		t.Fatal("trình duyệt lạ phải bị từ chối")
	}
	if _, err := svc.SetCookieSource(ctx, "chrome", "Profile 9"); err == nil {
		t.Fatal("hồ sơ không có phải bị từ chối")
	}
	view, err := svc.SetCookieSource(ctx, "chrome", "Default")
	if err != nil {
		t.Fatal(err)
	}
	if probed != "chrome:Default" {
		t.Fatalf("probe = %q", probed)
	}
	if !view.Saved || view.Browser != "chrome" || view.Profile != "Default" {
		t.Fatalf("view = %+v", view)
	}
	args, err := svc.CookieArgs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "--cookies-from-browser" || args[1] != "chrome:Default" {
		t.Fatalf("args = %v", args)
	}

	// Probe từ chối → không lưu.
	e.probe = func(context.Context, string, string) error { return codedProbeErr{"cookies_failed"} }
	if _, err := svc.SetCookieSource(ctx, "chrome", "Profile 2"); err == nil {
		t.Fatal("cookies_failed phải bị từ chối")
	}
	view, err = svc.Login(ctx, "youtube")
	if err != nil {
		t.Fatal(err)
	}
	if view.Profile != "Default" {
		t.Fatalf("hồ sơ cũ phải giữ, được %+v", view)
	}

	// Xoá đăng nhập.
	if _, err := svc.ClearLogin(ctx, "youtube"); err != nil {
		t.Fatal(err)
	}
	view, _ = svc.Login(ctx, "youtube")
	if view.Saved {
		t.Fatal("sau xoá phải rỗng")
	}
	if args, err := svc.CookieArgs(ctx); err != nil || args != nil {
		t.Fatalf("args sau xoá = %v %v", args, err)
	}
}

type codedProbeErr struct{ code string }

func (e codedProbeErr) Error() string     { return e.code }
func (e codedProbeErr) ErrorCode() string { return e.code }

func TestCheckYouTubeMapsProbeOutcomes(t *testing.T) {
	ctx := context.Background()
	e := newYTEnv(t)
	svc := e.service()
	if _, err := svc.Check(ctx, "youtube"); err == nil {
		t.Fatal("chưa chọn hồ sơ phải lỗi")
	}
	if _, err := svc.SetCookieSource(ctx, "chrome", "Default"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		err  error
		code string
	}{
		{codedProbeErr{"cookies_failed"}, accounts.ReasonRejected},
		{codedProbeErr{"login_required"}, accounts.ReasonRejected},
		{codedProbeErr{"failed"}, accounts.ReasonUnavailable},
		{errors.New("binary lỗi"), accounts.ReasonUnavailable},
	} {
		e.probe = func(context.Context, string, string) error { return tc.err }
		_, err := svc.Check(ctx, "youtube")
		var le *accounts.LoginError
		if !errors.As(err, &le) || le.Reason != tc.code {
			t.Fatalf("%v → %v, muốn %s", tc.err, err, tc.code)
		}
	}
	e.probe = func(context.Context, string, string) error { return nil }
	if _, err := svc.Check(ctx, "youtube"); err != nil {
		t.Fatalf("kiểm tra thành công: %v", err)
	}
}

func TestYouTubeWithoutProbeIsUnavailable(t *testing.T) {
	e := newYTEnv(t)
	svc := accounts.New(accounts.Deps{
		Dir:      filepath.Join(e.home, ".viderse"),
		Settings: e.set,
		Home:     e.home,
		GOOS:     "darwin",
		Now:      func() time.Time { return *e.now },
	})
	_, err := svc.SetCookieSource(context.Background(), "chrome", "Default")
	var le *accounts.LoginError
	if !errors.As(err, &le) || le.Reason != accounts.ReasonUnavailable {
		t.Fatalf("thiếu probe phải unavailable, được %v", err)
	}
}
