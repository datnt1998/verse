package webguard

import (
	"net/http"
	"time"
)

// SessionCookie owns the set/clear policy for one login cookie on one
// deployment. It is constructed once per app from startup facts — name and
// domain never change per request — so there is no global cookie state and
// no way for a request to widen the scope.
//
// The name is the app's choice: Noverse keeps `ainovel_session` on the
// file backend and `verse_session` on the shared Postgres backend. The
// `__Host-` prefix is intentionally not used: it forbids a Domain attribute
// (needed by the hosted shared cookie) and requires Secure (impossible on
// loopback HTTP).
type SessionCookie struct {
	Name   string
	Domain string // empty = host-only (local); bare registrable domain (hosted)
}

// NewSessionCookie derives the policy from the parsed boundary config. A nil
// config means the unconfigured local run: host-only cookies, Secure decided
// per request.
func NewSessionCookie(name string, cfg *Config) SessionCookie {
	policy := SessionCookie{Name: name}
	if cfg != nil {
		policy.Domain = cfg.CookieDomain
	}
	return policy
}

// secure reports whether the cookie must carry Secure on this request:
// always when the deployment forced it (SECURE_COOKIES — mandatory whenever
// CookieDomain is configured), otherwise only when the boundary computed an
// HTTPS browser view. A forged X-Forwarded-Proto cannot downgrade it because
// SecureRequest only trusts the configured proxy.
func (p SessionCookie) secure(r *http.Request) bool {
	return SecureRequest(r)
}

// Set writes the session cookie: HttpOnly, Path=/, SameSite=Strict and the
// configured Domain. Strict suffices for SSO because the two apps live on the
// same schemeful site; Lax would only help links from unrelated sites, which
// is not a requirement.
func (p SessionCookie) Set(w http.ResponseWriter, r *http.Request, value string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     p.Name,
		Value:    value,
		Path:     "/",
		Domain:   p.Domain,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   p.secure(r),
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

// Clear expires the session cookie. The deletion must repeat the exact Name,
// Path and Domain scope of Set — a mismatched scope leaves the real cookie
// alive while the caller believes the user was logged out.
func (p SessionCookie) Clear(w http.ResponseWriter, r *http.Request) {
	p.Expire(w, r, p.Name)
}

// Expire clears any cookie name at this policy's scope plus the host-only
// scope. It exists for retiring legacy cookie names on shared login: the
// browser may hold the old name either host-only (set before Domain was
// configured) or on the parent domain, so both shapes are expired. Expiring
// an absent cookie is harmless.
func (p SessionCookie) Expire(w http.ResponseWriter, r *http.Request, name string) {
	secure := p.secure(r)
	past := time.Unix(0, 0)
	for _, domain := range []string{"", p.Domain} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   domain,
			Expires:  past,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteStrictMode,
		})
		if p.Domain == "" {
			break
		}
	}
}
