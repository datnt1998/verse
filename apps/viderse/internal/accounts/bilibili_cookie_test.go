// Ca test chuyển từ src/server/tools/bilibili-cookies.test.ts của yt-factory:
// chuẩn hoá cookie dán tay, tệp bí mật 0600 + giá trị đã che, cookie bị từ
// chối không thay đăng nhập cũ, và tệp cookie Netscape chỉ áp cho bilibili.
package accounts_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/accounts"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// navServer trả /x/web-interface/nav theo thân cho trước và ghi cookie nhận.
func navServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/x/web-interface/nav" {
			seen = r.Header.Get("Cookie")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func newService(t *testing.T, dir string, apiURL string) *accounts.Service {
	t.Helper()
	return accounts.New(accounts.Deps{
		Dir:   dir,
		Hosts: accounts.BilibiliHosts{API: apiURL, Passport: apiURL, Site: apiURL},
		Now:   func() time.Time { return time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC) },
	})
}

func TestNormalizeBilibiliCookie(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"abc%2Cdef==", "SESSDATA=abc%2Cdef==", true},
		{"Cookie: SESSDATA=secret; bili_jct=csrf", "SESSDATA=secret; bili_jct=csrf", true},
		{"  SESSDATA=a;  bili_jct=b  ", "SESSDATA=a; bili_jct=b", true},
		{"", "", false},
		{"SESSDATA=", "", false},
		{"SESSDATA=secret\nInjected=x", "", false},
		{"bili_jct=x; foo=y", "", false}, // thiếu SESSDATA
		{"SESSDATA=a; SESSDATA=b", "", false},
	}
	for _, c := range cases {
		got, err := accounts.NormalizeBilibiliCookie(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Fatalf("%q → %q, %v; muốn %q", c.in, got, err, c.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%q phải lỗi", c.in)
		}
		if strings.Contains(err.Error(), "Injected") {
			t.Fatalf("lỗi không được echo nguyên liệu: %v", err)
		}
	}
}

func TestCookieFilesArePrivateAndMasked(t *testing.T) {
	dir := t.TempDir()
	srv, seen := navServer(t, 200, `{"code":0,"data":{"isLogin":true,"uname":"phu"}}`)
	svc := newService(t, dir, srv.URL)

	login, err := svc.Login(context.Background(), domain.PlatformBilibili)
	if err != nil {
		t.Fatal(err)
	}
	if login.Saved || login.Method != "" || login.Masked != nil {
		t.Fatalf("chưa đăng nhập = %+v", login)
	}

	saved, err := svc.SaveManualCookie(context.Background(), "long-secret-value")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !saved.Saved || saved.Masked == nil || *saved.Masked != "SESSDATA=••••alue" {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.Account == nil || *saved.Account != "phu" {
		t.Fatalf("account = %v", saved.Account)
	}
	if saved.Method != "manual" || saved.Renewable {
		t.Fatalf("method = %q renewable = %v", saved.Method, saved.Renewable)
	}
	cookieFile := filepath.Join(dir, "bilibili-cookie.txt")
	info, err := os.Stat(cookieFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("quyền tệp = %o", info.Mode().Perm())
	}
	if got, _ := svc.CookieHeader(context.Background()); got != "SESSDATA=long-secret-value" {
		t.Fatalf("cookie = %q", got)
	}
	if !strings.Contains(*seen, "SESSDATA=long-secret-value") {
		t.Fatalf("nav phải nhận cookie, thấy %q", *seen)
	}

	// Đổi cookie ngắn: quyền vẫn 0600, che đúng.
	if err := os.Chmod(cookieFile, 0o644); err != nil {
		t.Fatal(err)
	}
	saved, err = svc.SaveManualCookie(context.Background(), "short")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Masked == nil || *saved.Masked != "SESSDATA=••••" {
		t.Fatalf("masked ngắn = %v", saved.Masked)
	}
	if info, _ := os.Stat(cookieFile); info.Mode().Perm() != 0o600 {
		t.Fatalf("quyền sau ghi lại = %o", info.Mode().Perm())
	}

	// Xoá đăng nhập gỡ cả hai tệp.
	if _, err := svc.ClearLogin(context.Background(), domain.PlatformBilibili); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cookieFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cookie.txt còn sau khi xoá: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bilibili-login.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("login.json còn sau khi xoá: %v", err)
	}
}

func TestRejectedCookieKeepsPreviousLogin(t *testing.T) {
	dir := t.TempDir()
	// Server trả nav theo cookie: "working-secret" đăng nhập, "logged-out" không.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.Header.Get("Cookie"), "logged-out") {
			_, _ = w.Write([]byte(`{"code":-101,"message":"账号未登录","data":{"isLogin":false}}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"isLogin":true,"uname":"phu"}}`))
	}))
	defer srv.Close()
	svc := newService(t, dir, srv.URL)

	if _, err := svc.SaveManualCookie(context.Background(), "working-secret"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.SaveManualCookie(context.Background(), "logged-out")
	var le *accounts.LoginError
	if !errors.As(err, &le) || le.Reason != accounts.ReasonRejected {
		t.Fatalf("cookie chết phải bị từ chối, được %v", err)
	}
	if got, _ := svc.CookieHeader(context.Background()); got != "SESSDATA=working-secret" {
		t.Fatalf("đăng nhập cũ phải giữ, được %q", got)
	}
}

func TestCookieJarOnlyForBilibiliAndAlwaysRemoved(t *testing.T) {
	dir := t.TempDir()
	srv, _ := navServer(t, 200, `{"code":0,"data":{"isLogin":true,"uname":"phu"}}`)
	svc := newService(t, dir, srv.URL)
	if _, err := svc.SaveManualCookie(context.Background(), "SESSDATA=secret; bili_jct=csrf"); err != nil {
		t.Fatal(err)
	}

	var jar string
	err := svc.WithBilibiliCookieJar(context.Background(), "https://www.bilibili.com/video/BV1", func(path string) error {
		jar = path
		if path == "" {
			t.Fatal("URL bilibili phải có tệp cookie")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("quyền jar = %o", info.Mode().Perm())
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), ".bilibili.com\tTRUE\t/\tTRUE\t0\tSESSDATA\tsecret") {
			t.Fatalf("jar = %q", raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if jar == "" {
		t.Fatal("thiếu đường dẫn jar")
	}
	if _, err := os.Stat(jar); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jar phải bị xoá sau lượt chạy: %v", err)
	}

	runErr := svc.WithBilibiliCookieJar(context.Background(), "https://bilibili.com/video/x", func(path string) error {
		jar = path
		return errors.New("download failed")
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "download failed") {
		t.Fatalf("err = %v", runErr)
	}
	if _, err := os.Stat(jar); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jar phải bị xoá cả khi lỗi: %v", err)
	}

	for _, raw := range []string{"https://bilibili.com.evil.test/", "https://www.youtube.com/watch?v=x", "bad"} {
		if err := svc.WithBilibiliCookieJar(context.Background(), raw, func(path string) error {
			if path != "" {
				t.Fatalf("%s không được nhận cookie, được %q", raw, path)
			}
			return nil
		}); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}
