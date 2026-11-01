// Package secheaders applies the response headers that make a publicly served
// app a poor target: no MIME sniffing, no framing by another site, no search
// indexing, no referrer leak to third parties — plus HSTS, which is only
// correct when the deployment really is https.
//
// The header set is deliberately narrow. A full Content-Security-Policy would
// need auditing against every SPA bundle and every inline style the editors
// inject, so only frame-ancestors is sent: it is the one directive that cannot
// break a page's own scripts, and it is what stops clickjacking. Responses that
// already carry stricter headers (blob delivery sets a sandbox CSP and nosniff,
// render routes set no-referrer) keep them: Apply never overwrites a header
// that is already present, and a handler that Sets later wins anyway.
package secheaders

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/platform/webguard"
)

// HSTSMaxAge is six months, in seconds. includeSubDomains and preload are not
// set: the zone still has hostnames that are not audited for TLS.
const HSTSMaxAge = "15552000"

// RobotsBody disallows every crawler. Both apps are invitation-only and their
// login pages must not be indexed.
const RobotsBody = "User-agent: *\nDisallow: /\n"

// Policy is the deployment-wide header set. It carries one decision only —
// whether the deployment is served over https — because everything else is
// constant for both apps.
type Policy struct {
	// HSTS adds Strict-Transport-Security. True only for an https PUBLIC_URL:
	// on a plain-http local or desktop run the header would pin a loopback
	// host to https and lock the user out of their own app.
	HSTS bool
}

// FromProxy derives the policy from the trusted-proxy boundary parsed at
// startup. A nil config means "not configured yet": no HSTS.
func FromProxy(cfg *webguard.Config) Policy {
	if cfg == nil || cfg.PublicURL == nil {
		return Policy{}
	}
	return Policy{HSTS: strings.EqualFold(cfg.PublicURL.Scheme, "https")}
}

// Apply writes the header set, leaving any header a handler already set alone.
// Callers run it before dispatch so health probes, 401s, static files and API
// errors all carry the headers, not just successful handler output.
func (p Policy) Apply(w http.ResponseWriter) {
	header := w.Header()
	setIfAbsent(header, "X-Content-Type-Options", "nosniff")
	setIfAbsent(header, "Referrer-Policy", "same-origin")
	// Neither app frames its own pages (no <iframe> in either web source), so
	// the strictest pair is correct: DENY plus frame-ancestors 'none'.
	setIfAbsent(header, "X-Frame-Options", "DENY")
	setIfAbsent(header, "Content-Security-Policy", "frame-ancestors 'none'")
	setIfAbsent(header, "X-Robots-Tag", "noindex, nofollow")
	if p.HSTS {
		setIfAbsent(header, "Strict-Transport-Security", "max-age="+HSTSMaxAge)
	}
}

// Robots serves /robots.txt when the request asks for it and reports whether it
// answered. Neither app embeds a robots.txt, and the SPA fallback would
// otherwise return index.html for that path.
func (p Policy) Robots(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/robots.txt" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(RobotsBody)))
	if r.Method == http.MethodHead {
		return true
	}
	_, _ = w.Write([]byte(RobotsBody))
	return true
}

func setIfAbsent(header http.Header, name, value string) {
	if header.Get(name) == "" {
		header.Set(name, value)
	}
}
