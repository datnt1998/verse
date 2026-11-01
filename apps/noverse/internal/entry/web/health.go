package web

import (
	"context"
	"net/http"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

// Liveness and readiness are deliberately separate:
//
//	/healthz  process is up and serving HTTP (no dependency, no auth, no
//	          secret). A restart of the container is justified only when this
//	          fails.
//	/readyz   the request path can actually do work: the database answers,
//	          its schema is the version this build supports, and the BlobStore
//	          is reachable. Traefik/load balancers route on this; a failing
//	          dependency returns 503 instead of hanging.
//
// The home Mac TTS worker is NOT part of readiness: an unreachable worker is
// queue degradation, not an unhealthy app (a restart would not fix it).

// HealthDeps is the readiness probe set. Nil function means "not applicable"
// (file mode has no database; the file blob driver has no remote endpoint).
type HealthDeps struct {
	// SchemaVersion must prove connectivity AND compatibility, returning the
	// applied migration version for the readiness body (never a DSN).
	SchemaVersion func(ctx context.Context) (int64, error)
	// BlobReachable checks the configured blob backend. It is called at most
	// once per CacheTTL so readiness never uploads a probe per request.
	BlobReachable func(ctx context.Context) error
	// Timeout bounds a single readiness request. Default 3s.
	Timeout time.Duration
	// CacheTTL bounds how often the blob probe hits the network. Default 30s.
	CacheTTL time.Duration
	// ReadyCacheTTL bounds how often the whole readiness probe runs. Default 2s.
	ReadyCacheTTL time.Duration
}

type readinessCheck struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

type readinessBody struct {
	Status        string           `json:"status"`
	SchemaVersion *int64           `json:"schemaVersion,omitempty"`
	Checks        []readinessCheck `json:"checks"`
}

// SetHealth installs the readiness dependencies. Called once by serve before
// the listener starts; the gateway keeps serving liveness without it.
func (g *Gateway) SetHealth(deps HealthDeps) {
	if deps.Timeout <= 0 {
		deps.Timeout = 3 * time.Second
	}
	if deps.CacheTTL <= 0 {
		deps.CacheTTL = 30 * time.Second
	}
	g.healthMu.Lock()
	g.health = &deps
	g.healthMu.Unlock()
}

// healthz is liveness: no dependency is touched, so a database blip does not
// restart a healthy process.
func (g *Gateway) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		jsonError(w, http.StatusMethodNotAllowed, "chỉ hỗ trợ GET/HEAD")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// defaultReadyCacheTTL is the readiness cooldown: /readyz is reachable from the
// internet and every probe queries the database, so the whole check runs at
// most once per window and concurrent probes share one result.
const defaultReadyCacheTTL = 2 * time.Second

// readyz reports dependency state. Each check has its own deadline; a slow
// database returns 503 quickly instead of hanging the probe. The HTTP status is
// the same for everyone — monitoring reads it — but the body is not: schema
// version and per-check failures go only to the operator (a direct loopback
// caller) or an authenticated admin.
func (g *Gateway) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		jsonError(w, http.StatusMethodNotAllowed, "chỉ hỗ trợ GET/HEAD")
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	body, status := g.readiness(r)
	if !g.readyzDetailAllowed(r) {
		// Public probe: ready or not, nothing else.
		writeJSON(w, status, map[string]any{"ok": body.Status == "ok"})
		return
	}
	writeJSON(w, status, body)
}

// readiness returns the cached probe result, running the checks at most once
// per ReadyCacheTTL. The probe context outlives the caller: a client hanging up
// mid-probe must not cache a false "unavailable".
func (g *Gateway) readiness(r *http.Request) (readinessBody, int) {
	g.healthMu.Lock()
	deps := g.health
	g.healthMu.Unlock()

	ttl := defaultReadyCacheTTL
	if deps != nil && deps.ReadyCacheTTL > 0 {
		ttl = deps.ReadyCacheTTL
	}

	g.readyMu.Lock()
	defer g.readyMu.Unlock()
	if g.readyAt != (time.Time{}) && time.Since(g.readyAt) < ttl {
		return g.readyBody, g.readyStatus
	}
	body, status := g.probeReadiness(context.WithoutCancel(r.Context()), deps)
	g.readyAt, g.readyBody, g.readyStatus = time.Now(), body, status
	return body, status
}

// probeReadiness runs every configured check once.
func (g *Gateway) probeReadiness(ctx context.Context, deps *HealthDeps) (readinessBody, int) {
	body := readinessBody{Status: "ok", Checks: []readinessCheck{}}
	if deps == nil {
		// File mode / no hosted dependencies: serving is ready by definition.
		return body, http.StatusOK
	}

	ctx, cancel := context.WithTimeout(ctx, deps.Timeout)
	defer cancel()

	if deps.SchemaVersion != nil {
		version, err := deps.SchemaVersion(ctx)
		body.Checks = append(body.Checks, readinessCheck{Name: "database", OK: err == nil})
		if err == nil {
			body.SchemaVersion = &version
		} else {
			body.Status = "unavailable"
		}
	}
	if deps.BlobReachable != nil {
		ok := g.blobReady(ctx, deps)
		body.Checks = append(body.Checks, readinessCheck{Name: "blob", OK: ok})
		if !ok {
			body.Status = "unavailable"
		}
	}

	status := http.StatusOK
	if body.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	return body, status
}

// readyzDetailAllowed reports whether the caller may see deployment detail: the
// operator's own machine (loopback peer with no proxy in front) or an
// authenticated admin. A session lookup only happens when a cookie is present,
// so an anonymous probe never touches the account store.
func (g *Gateway) readyzDetailAllowed(r *http.Request) bool {
	if g.proxy.DirectLoopback(r) {
		return true
	}
	if g.accounts == nil {
		return false
	}
	cookie, err := r.Cookie(g.cookiePolicy().Name)
	if err != nil {
		return false
	}
	user, _, err := g.accounts.Authenticate(cookie.Value)
	if err != nil {
		return false
	}
	return user.Role == identity.RoleAdmin
}

// blobReady caches the blob probe for CacheTTL, including failures: a down
// bucket must not turn every readiness poll into a network call, and must not
// be masked for longer than the TTL.
func (g *Gateway) blobReady(ctx context.Context, deps *HealthDeps) bool {
	g.healthMu.Lock()
	defer g.healthMu.Unlock()
	if g.blobProbedAt != (time.Time{}) && time.Since(g.blobProbedAt) < deps.CacheTTL {
		return g.blobOK
	}
	probeCtx, cancel := context.WithTimeout(ctx, deps.Timeout)
	defer cancel()
	err := deps.BlobReachable(probeCtx)
	g.blobProbedAt, g.blobOK = time.Now(), err == nil
	return g.blobOK
}

// healthMu guards the dependency set and the cached blob probe.
