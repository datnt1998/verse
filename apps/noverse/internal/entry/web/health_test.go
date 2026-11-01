package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestHealthBeforeSPA pins the routing fix: /healthz and /readyz answer JSON
// before the static SPA handler, which otherwise serves index.html with 200.
func TestHealthBeforeSPA(t *testing.T) {
	g := &Gateway{}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" && ct != "application/json" {
		t.Fatalf("/healthz content-type = %q (SPA fallback?)", ct)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Status != "ok" {
		t.Fatalf("/healthz body = %s err=%v", rec.Body.String(), err)
	}

	rec = httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz status = %d", rec.Code)
	}
}

func TestHealthzIgnoresDependencyState(t *testing.T) {
	g := &Gateway{}
	g.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { return 0, errors.New("db down") },
		BlobReachable: func(context.Context) error { return errors.New("bucket down") },
	})
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness must not depend on storage: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness with both deps down = %d", rec.Code)
	}
}

func TestReadyzReportsSchemaAndBlob(t *testing.T) {
	g := &Gateway{}
	g.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { return 7, nil },
		BlobReachable: func(context.Context) error { return nil },
	})
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, loopbackRequest("/readyz"))
	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status        string `json:"status"`
		SchemaVersion *int64 `json:"schemaVersion"`
		Checks        []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.SchemaVersion == nil || *body.SchemaVersion != 7 {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if len(body.Checks) != 2 || !body.Checks[0].OK || !body.Checks[1].OK {
		t.Fatalf("checks = %+v", body.Checks)
	}
	if body := rec.Body.String(); body == "" || strings.Contains(body, "postgres://") ||
		strings.Contains(body, "password") || strings.Contains(body, "DSN") {
		t.Fatalf("readiness leaks connection details: %s", body)
	}

	// No dependencies configured (file mode): ready, empty check list.
	file := &Gateway{}
	rec = httptest.NewRecorder()
	file.ServeHTTP(rec, loopbackRequest("/readyz"))
	if rec.Code != http.StatusOK {
		t.Fatalf("file-mode /readyz = %d", rec.Code)
	}
}

func TestReadyzBlobFailureAndCache(t *testing.T) {
	var calls atomic.Int32
	g := &Gateway{}
	g.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { return 7, nil },
		BlobReachable: func(context.Context) error {
			calls.Add(1)
			return errors.New("s3 unreachable")
		},
		CacheTTL: time.Hour,
	})
	for range 3 {
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, loopbackRequest("/readyz"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("blob down must be 503, got %d", rec.Code)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("blob probe hit %d times, want 1 within CacheTTL", got)
	}
}

func TestReadyzTimeout(t *testing.T) {
	g := &Gateway{}
	g.SetHealth(HealthDeps{
		Timeout: 50 * time.Millisecond,
		SchemaVersion: func(ctx context.Context) (int64, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	})
	start := time.Now()
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, loopbackRequest("/readyz"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("hung database must be 503, got %d", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("readiness hung for %s", elapsed)
	}
}

// loopbackRequest builds the probe request the operator's own machine sends
// (docker healthcheck, deploy script): peer 127.0.0.1 with no proxy in front.
func loopbackRequest(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.RemoteAddr = "127.0.0.1:51234"
	return r
}

// TestReadyzPublicBodyHidesDeploymentDetail pins the public shape of /readyz:
// a client from the internet learns whether the app is ready, never the schema
// version or which dependency failed.
func TestReadyzPublicBodyHidesDeploymentDetail(t *testing.T) {
	g := &Gateway{}
	g.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { return 7, nil },
		BlobReachable: func(context.Context) error { return nil },
	})
	for _, tc := range []struct {
		name string
		want string
	}{
		{"ready", `{"ok":true}`},
	} {
		r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		r.RemoteAddr = "203.0.113.9:443"
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", tc.name, rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != tc.want {
			t.Fatalf("%s: body = %s, want %s", tc.name, got, tc.want)
		}
	}

	// A failing dependency keeps the 503 but not the diagnosis.
	down := &Gateway{}
	down.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { return 0, errors.New("db down") },
		BlobReachable: func(context.Context) error { return errors.New("bucket down") },
	})
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.RemoteAddr = "203.0.113.9:443"
	rec := httptest.NewRecorder()
	down.ServeHTTP(rec, r)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("down: status = %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"ok":false}` {
		t.Fatalf("down: body = %s", got)
	}
}

// TestReadyzDetailForAdminSession pins the second privileged path: an admin
// signed in from anywhere still gets the full probe body.
func TestReadyzDetailForAdminSession(t *testing.T) {
	g, accounts := newAuthGateway(t)
	session, err := accounts.Login("admin", "admin-pass", "test", "203.0.113.9")
	if err != nil {
		t.Fatal(err)
	}
	g.SetHealth(HealthDeps{SchemaVersion: func(context.Context) (int64, error) { return 7, nil }})
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.RemoteAddr = "203.0.113.9:443"
	r.AddCookie(&http.Cookie{Name: g.cookiePolicy().Name, Value: session.Cookie})
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /readyz status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"schemaVersion":7`) {
		t.Fatalf("admin body = %s, want deployment detail", rec.Body.String())
	}

	// A member is not an operator: minimal body.
	member, err := accounts.Login("writer", "writer-pass", "test", "203.0.113.9")
	if err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.RemoteAddr = "203.0.113.9:443"
	r.AddCookie(&http.Cookie{Name: g.cookiePolicy().Name, Value: member.Cookie})
	rec = httptest.NewRecorder()
	g.ServeHTTP(rec, r)
	if got := strings.TrimSpace(rec.Body.String()); got != `{"ok":true}` {
		t.Fatalf("member body = %s, want minimal", got)
	}
}

// TestReadyzCoalescesProbes pins the cooldown: /readyz is reachable from the
// internet, so repeated calls must not each query the database.
func TestReadyzCoalescesProbes(t *testing.T) {
	var calls atomic.Int32
	g := &Gateway{}
	g.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { calls.Add(1); return 7, nil },
	})
	for range 5 {
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, loopbackRequest("/readyz"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("schema probe hit %d times, want 1 within the cooldown", got)
	}
	// After the cooldown the next probe runs again.
	time.Sleep(10 * time.Millisecond)
	g.SetHealth(HealthDeps{
		SchemaVersion: func(context.Context) (int64, error) { calls.Add(1); return 7, nil },
		ReadyCacheTTL: 5 * time.Millisecond,
	})
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, loopbackRequest("/readyz"))
	if got := calls.Load(); got != 2 {
		t.Fatalf("probe count = %d after cooldown, want 2", got)
	}
}
