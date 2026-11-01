package webguard

import (
	"net/http"
	"strings"
	"time"
)

// DeviceCookieName is the cookie both apps use to remember a browser that
// already signed in successfully as one account. It is a limiter hint only:
// the server stores just its digest, it never authenticates a request, and it
// carries no account name the server would trust.
const DeviceCookieName = "verse_device"

// DeviceCookie owns the set/clear policy of that cookie. Unlike the session
// cookie it is always host-only: the token is bound to one origin, so a sibling
// app (or any other host on the parent domain) never receives it. Secure is
// decided per request by the trusted-proxy boundary — a forged
// X-Forwarded-Proto cannot downgrade it.
type DeviceCookie struct {
	Name string
}

// NewDeviceCookie returns the shared device-cookie policy.
func NewDeviceCookie() DeviceCookie { return DeviceCookie{Name: DeviceCookieName} }

// Read returns the presented token, or "" when the cookie is absent or empty.
// A malformed value is returned as-is: only the identity store decides whether
// a token is real, and a forged one must cost the attacker the shared bucket.
func (c DeviceCookie) Read(r *http.Request) string {
	cookie, err := r.Cookie(c.Name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

// Set writes the device cookie: HttpOnly, host-only (no Domain), Path=/,
// SameSite=Strict, Secure on an HTTPS browser view. MaxAge follows expires.
func (c DeviceCookie) Set(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.Name,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   SecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// Clear expires the device cookie at the exact scope Set uses.
func (c DeviceCookie) Clear(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.Name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   SecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}
