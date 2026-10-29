package web

import (
	"context"
	"net/http"
	"time"
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

// readyz reports dependency state. Each check has its own deadline; a slow
// database returns 503 quickly instead of hanging the probe.
func (g *Gateway) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		jsonError(w, http.StatusMethodNotAllowed, "chỉ hỗ trợ GET/HEAD")
		return
	}
	w.Header().Set("Cache-Control", "no-store")

	g.healthMu.Lock()
	deps := g.health
	g.healthMu.Unlock()

	body := readinessBody{Status: "ok", Checks: []readinessCheck{}}
	if deps == nil {
		// File mode / no hosted dependencies: serving is ready by definition.
		writeJSON(w, http.StatusOK, body)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), deps.Timeout)
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
	writeJSON(w, status, body)
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
