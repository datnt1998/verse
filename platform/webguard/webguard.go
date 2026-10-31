// Package webguard holds the trusted-proxy boundary shared by every app:
// client IP for limiters, "is this HTTPS" for Secure cookies, and the allowed
// origin for state-changing requests must come from the boundary — never from
// self-declared headers on an untrusted peer.
//
// The hosted deployment terminates TLS at a reverse proxy and forwards plain
// HTTP to the app. TRUSTED_PROXIES is the explicit boundary: only peers inside
// those CIDRs may supply X-Forwarded-*. Unset trusts no peer, loopback
// included, so a loopback proxy such as `tailscale serve` cannot mark a
// request HTTPS through X-Forwarded-Proto until TRUSTED_PROXIES=127.0.0.1/32
// is set. There is exactly one name: a second alias would let an operator
// change a setting that is silently ignored.
package webguard

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ErrConfig marks malformed proxy-boundary configuration; startup must fail
// on it rather than silently downgrading trust.
var ErrConfig = errors.New("lỗi cấu hình")

// Config is the parsed trusted-proxy contract.
type Config struct {
	// Trusted is the peer CIDR list. Empty trusts no peer: X-Forwarded-* is
	// ignored, loopback included.
	Trusted []*net.IPNet
	// PublicURL is the canonical public base URL. When set, browser
	// state-changing requests are accepted only from this origin plus the
	// explicit AppOrigins allowlist; SECURE_COOKIES requires it to be https.
	PublicURL *url.URL
	// SecureCookies forces every session cookie to carry Secure. Hosted
	// deployments must set it; startup fails when PUBLIC_URL is not HTTPS.
	SecureCookies bool
	// CookieDomain is the shared-session cookie Domain attribute, already
	// canonicalised (lower-case, no leading dot). Empty means host-only
	// cookies; a non-empty value is only legal on an HTTPS deployment with
	// SecureCookies, and every accepted origin must live under it.
	CookieDomain string
	// AppOrigins is the explicit allowlist of sibling-app origins (canonical
	// scheme://host[:port], deduplicated). An origin is accepted when it is
	// the deployment's own origin or is listed here exactly — never by
	// suffix, wildcard or subdomain inference.
	AppOrigins []string
}

type contextKey struct{}

// Facts are the security-relevant request facts computed at the boundary.
type Facts struct {
	ClientIP string
	Secure   bool
}

// ConfigFromEnv parses TRUSTED_PROXIES, PUBLIC_URL and SECURE_COOKIES
// strictly. A malformed CIDR, a wildcard trust entry or Secure cookies without
// an HTTPS public URL is a startup error — never a silent downgrade.
func ConfigFromEnv() (*Config, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	raw := get("TRUSTED_PROXIES")
	cfg := &Config{}
	trusted, err := ParseTrustedProxies(raw)
	if err != nil {
		return nil, err
	}
	cfg.Trusted = trusted

	if raw := get("PUBLIC_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("PUBLIC_URL=%q không phải URL tuyệt đối: %w", raw, ErrConfig)
		}
		switch u.Scheme {
		case "https", "http":
		default:
			return nil, fmt.Errorf("PUBLIC_URL=%q phải dùng http hoặc https: %w", raw, ErrConfig)
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("PUBLIC_URL không được chứa query/fragment: %w", ErrConfig)
		}
		cfg.PublicURL = u
	}

	if v := get("SECURE_COOKIES"); v != "" {
		switch v {
		case "true", "1":
			cfg.SecureCookies = true
		case "false", "0":
		default:
			return nil, fmt.Errorf("SECURE_COOKIES=%q không hợp lệ (true|false): %w", v, ErrConfig)
		}
	}
	if cfg.SecureCookies {
		if cfg.PublicURL == nil || cfg.PublicURL.Scheme != "https" {
			return nil, fmt.Errorf("SECURE_COOKIES=true yêu cầu PUBLIC_URL là https://…: %w", ErrConfig)
		}
	}

	if raw := get("VERSE_COOKIE_DOMAIN"); raw != "" {
		domain, err := ParseCookieDomain(raw)
		if err != nil {
			return nil, err
		}
		// A shared cookie domain only exists on an HTTPS deployment: the
		// cookie must be Secure, and its scope must cover the public URL.
		if !cfg.SecureCookies {
			return nil, fmt.Errorf("VERSE_COOKIE_DOMAIN yêu cầu SECURE_COOKIES=true và PUBLIC_URL https://…: %w", ErrConfig)
		}
		if !hostWithinDomain(cfg.PublicURL.Hostname(), domain) {
			return nil, fmt.Errorf("VERSE_COOKIE_DOMAIN=%q không chứa host của PUBLIC_URL: %w", raw, ErrConfig)
		}
		cfg.CookieDomain = domain
	}

	if raw := get("VERSE_APP_ORIGINS"); raw != "" {
		origins, err := parseAppOrigins(raw)
		if err != nil {
			return nil, err
		}
		for _, origin := range origins {
			u, _ := url.Parse(origin)
			host := u.Hostname()
			if cfg.PublicURL != nil {
				// Hosted: the allowlist never downgrades to plain HTTP, and a
				// configured cookie domain must cover every accepted origin.
				if u.Scheme != "https" {
					return nil, fmt.Errorf("VERSE_APP_ORIGINS mục %q phải là https khi PUBLIC_URL được đặt: %w", origin, ErrConfig)
				}
				if cfg.CookieDomain != "" && !hostWithinDomain(host, cfg.CookieDomain) {
					return nil, fmt.Errorf("VERSE_APP_ORIGINS mục %q nằm ngoài VERSE_COOKIE_DOMAIN: %w", origin, ErrConfig)
				}
			} else {
				// Local SSO is loopback-only: a named host or a LAN address
				// here would hand the shared cookie to whatever listens there.
				if host != "localhost" && !isLoopbackLiteral(host) {
					return nil, fmt.Errorf("VERSE_APP_ORIGINS mục %q chỉ được là localhost/loopback khi không có PUBLIC_URL: %w", origin, ErrConfig)
				}
			}
		}
		cfg.AppOrigins = origins
	}
	return cfg, nil
}

// Hosted reports whether the deployment has a canonical PUBLIC_URL: the
// shared-cookie contract (Domain attribute, exact app-origin allowlist) only
// exists there. Local runs bind loopback and use host-only cookies.
func (c *Config) Hosted() bool { return c != nil && c.PublicURL != nil }

// LocalSSO reports whether this run shares its session cookie with sibling
// loopback apps without a hosted domain: a non-empty AppOrigins allowlist on
// a deployment without PUBLIC_URL. Such a run must stay on loopback — the
// cookie travels to every port on the host, so a LAN/wildcard bind would hand
// it to untrusted neighbours.
func (c *Config) LocalSSO() bool { return c != nil && !c.Hosted() && len(c.AppOrigins) > 0 }

// isLoopbackLiteral reports whether host is an IP literal inside loopback.
func isLoopbackLiteral(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// hostWithinDomain reports whether host is domain itself or a subdomain of it
// — a DNS-label boundary check, never a raw string suffix (evil-example.com
// must not match example.com).
func hostWithinDomain(host, domain string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// ParseCookieDomain canonicalises VERSE_COOKIE_DOMAIN. It accepts one optional
// leading dot and returns the bare lower-case registrable domain. It refuses
// everything that is not a parent domain an app may scope a cookie to: URLs,
// ports, IPs, wildcards, single labels (localhost, internal) and public
// suffixes themselves — a cookie on "com" or "co.uk" would be shared with
// unrelated sites, and browsers silently drop it anyway.
func ParseCookieDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	domain = strings.TrimPrefix(domain, ".")
	if strings.ContainsAny(domain, "/:@?#&= \t") || strings.Contains(domain, "*") {
		return "", fmt.Errorf("VERSE_COOKIE_DOMAIN=%q không phải tên miền (cấm scheme, port, userinfo, wildcard): %w", raw, ErrConfig)
	}
	if net.ParseIP(domain) != nil {
		return "", fmt.Errorf("VERSE_COOKIE_DOMAIN=%q không được là địa chỉ IP: %w", raw, ErrConfig)
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("VERSE_COOKIE_DOMAIN=%q cần ít nhất hai nhãn DNS: %w", raw, ErrConfig)
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("VERSE_COOKIE_DOMAIN=%q có nhãn DNS không hợp lệ: %w", raw, ErrConfig)
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", fmt.Errorf("VERSE_COOKIE_DOMAIN=%q có ký tự không hợp lệ: %w", raw, ErrConfig)
			}
		}
	}
	// The Public Suffix List decides what a browser treats as a shared
	// suffix; checking the string's tail by hand would accept "co.uk".
	if suffix, _ := publicsuffix.PublicSuffix(domain); suffix == domain {
		return "", fmt.Errorf("VERSE_COOKIE_DOMAIN=%q là public suffix, không được đặt cookie ở đây: %w", raw, ErrConfig)
	}
	return domain, nil
}

// canonicalOrigin reduces an origin to its comparison form: lower-case
// scheme and host, explicit port only when it is not the scheme default, and
// no path, query, fragment or userinfo. It returns an error for anything the
// allowlist must not silently reinterpret.
func canonicalOrigin(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" || text == "*" || strings.EqualFold(text, "null") {
		return "", fmt.Errorf("origin %q không hợp lệ: %w", raw, ErrConfig)
	}
	u, err := url.Parse(text)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("origin %q phải là URL tuyệt đối có host: %w", raw, ErrConfig)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("origin %q phải dùng http hoặc https: %w", raw, ErrConfig)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("origin %q cấm userinfo/query/fragment: %w", raw, ErrConfig)
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origin %q cấm đường dẫn %q: %w", raw, u.Path, ErrConfig)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || strings.Contains(host, "*") {
		return "", fmt.Errorf("origin %q có host không hợp lệ: %w", raw, ErrConfig)
	}
	port := u.Port()
	if port != "" {
		if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
			port = ""
		}
	}
	out := u.Scheme + "://" + host
	if port != "" {
		out += ":" + port
	}
	return out, nil
}

// parseAppOrigins parses the comma-separated VERSE_APP_ORIGINS contract.
// Every entry must be a canonical absolute origin; malformed entries are a
// startup error, never silently skipped.
func parseAppOrigins(raw string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		origin, err := canonicalOrigin(field)
		if err != nil {
			return nil, fmt.Errorf("VERSE_APP_ORIGINS: %w", err)
		}
		if !seen[origin] {
			seen[origin] = true
			out = append(out, origin)
		}
	}
	return out, nil
}

// allowedListedOrigin reports whether canonical equals one of the explicit
// AppOrigins entries — byte-for-byte after canonicalisation, so sibling ports,
// lookalike hosts and scheme downgrades never match.
func (c *Config) allowedListedOrigin(canonical string) bool {
	if c == nil {
		return false
	}
	for _, origin := range c.AppOrigins {
		if canonical == origin {
			return true
		}
	}
	return false
}

// ParseTrustedProxies parses a comma/space separated CIDR list. A bare IP is
// accepted as a /32 or /128 host route. Widening trust to every peer
// (0.0.0.0/0, ::/0) is refused: that turns the X-Forwarded-* headers into
// client-controlled input and lets a forged header evade rate limits.
func ParseTrustedProxies(raw string) ([]*net.IPNet, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	out := make([]*net.IPNet, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		entry := field
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES: %q không phải IP/CIDR: %w", field, ErrConfig)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			entry = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %q không phải CIDR: %w", field, ErrConfig)
		}
		if ones, _ := network.Mask.Size(); ones == 0 {
			return nil, fmt.Errorf("TRUSTED_PROXIES: không được tin toàn bộ mạng (%q): %w", field, ErrConfig)
		}
		out = append(out, network)
	}
	return out, nil
}

// trusted reports whether ip is inside the configured proxy boundary.
func (c *Config) trusted(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, network := range c.Trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Normalize returns a request whose context carries the security-relevant
// request facts. Headers are only honoured when the immediate peer is a
// trusted proxy; otherwise the raw peer address and TLS state are used.
func (c *Config) Normalize(r *http.Request) *http.Request {
	if c == nil {
		return r
	}
	peer := ParseRemoteIP(r.RemoteAddr)
	facts := Facts{ClientIP: peer.String(), Secure: r.TLS != nil || c.SecureCookies}
	if peer == nil {
		facts.ClientIP = r.RemoteAddr
	}

	if peer != nil && c.trusted(peer) {
		if proto := forwardedProto(r); proto != "" {
			facts.Secure = proto == "https" || c.SecureCookies
		}
		if ip := c.forwardedClient(peer, r.Header.Values("X-Forwarded-For")); ip != nil {
			facts.ClientIP = ip.String()
		}
	}
	if facts.ClientIP == "" {
		facts.ClientIP = r.RemoteAddr
	}
	return r.WithContext(context.WithValue(r.Context(), contextKey{}, facts))
}

// forwardedProto accepts exactly one well-formed scheme token. Multiple or
// unknown values are discarded, not interpreted charitably.
func forwardedProto(r *http.Request) string {
	values := r.Header.Values("X-Forwarded-Proto")
	if len(values) != 1 {
		return ""
	}
	proto := strings.ToLower(strings.TrimSpace(values[0]))
	if proto != "http" && proto != "https" {
		return ""
	}
	return proto
}

// forwardedClient walks X-Forwarded-For right-to-left through trusted hops and
// returns the first address the chain did not add itself. A malformed element
// stops the walk at the last trusted address rather than trusting garbage.
func (c *Config) forwardedClient(peer net.IP, headers []string) net.IP {
	hops := make([]net.IP, 0, len(headers))
	for _, header := range headers {
		for _, part := range strings.Split(header, ",") {
			ip := net.ParseIP(strings.TrimSpace(part))
			if ip == nil {
				return nil
			}
			hops = append(hops, ip)
		}
	}
	if len(hops) == 0 {
		return peer
	}
	current := peer
	for i := len(hops) - 1; i >= 0; i-- {
		if !c.trusted(current) {
			break
		}
		current = hops[i]
	}
	if current == nil {
		return peer
	}
	return current
}

// AllowedOrigin reports whether an Origin value is one this deployment serves.
// Hosted (PUBLIC_URL set): only PUBLIC_URL. Local: the origin must be the
// request's own host:port and that host must be an IP literal (127.0.0.1, ::1,
// the LAN/Tailscale address a phone connected to) or localhost. A different
// port is a sibling app on the same machine; a domain name equal to Host is a
// DNS-rebinding page, since nothing in local mode is served under a name.
func (c *Config) AllowedOrigin(r *http.Request, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "null" {
		return false
	}
	// An explicit allowlist match is decided on the canonical form, so
	// "http://127.0.0.1:18789/" and "HTTP://127.0.0.1:18789" count but
	// suffixes, lookalike hosts and credentials never do.
	if canonical, err := canonicalOrigin(origin); err == nil && c.allowedListedOrigin(canonical) {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if c != nil && c.PublicURL != nil {
		return strings.EqualFold(u.Scheme, c.PublicURL.Scheme) && strings.EqualFold(u.Host, c.PublicURL.Host)
	}
	if !strings.EqualFold(u.Host, r.Host) {
		return false
	}
	host := u.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil
}

// CrossSiteRequest rejects browser-issued state changes from another origin, in
// every mode. SameSite=Strict does not separate ports on one host:
// 127.0.0.1:8789 and 127.0.0.1:8790 are the same site, so only the Origin check
// keeps a sibling app's page from writing here. A request without Origin (CLI,
// worker, curl) passes unless the browser marks it cross-site.
func (c *Config) CrossSiteRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	if site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); site == "cross-site" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false // non-browser client (curl, worker) or same-origin GET-style POST
	}
	return !c.AllowedOrigin(r, origin)
}

// ParseRemoteIP extracts the IP literal from a host:port peer address.
func ParseRemoteIP(addr string) net.IP {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

// Normalized reads the trusted-proxy facts from the request context. ok=false
// means the request never went through Normalize (tests calling handlers
// directly), so callers fall back to peer-based rules.
func Normalized(r *http.Request) (Facts, bool) {
	facts, ok := r.Context().Value(contextKey{}).(Facts)
	return facts, ok
}

// ClientIP returns the trusted client address: the boundary-computed value
// for normalized requests, the raw peer otherwise.
func ClientIP(r *http.Request) string {
	if facts, ok := Normalized(r); ok && facts.ClientIP != "" {
		return facts.ClientIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// SecureRequest reports whether the browser sees HTTPS: direct TLS, or
// X-Forwarded-Proto from a TRUSTED_PROXIES peer (with no trust list no peer is
// trusted, loopback included). Self-declared headers from the remote are
// ignored — a forged header could otherwise downgrade Secure cookies. The
// loopback rule below only applies to requests that never went through
// Normalize.
func SecureRequest(r *http.Request) bool {
	if facts, ok := Normalized(r); ok {
		return facts.Secure
	}
	if r.TLS != nil {
		return true
	}
	if !strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return false
	}
	ip := net.ParseIP(ClientIP(r))
	return ip != nil && ip.IsLoopback()
}

// JSONBodyRequired reports whether the request carries a body that must be
// JSON. POST and PUT always carry one; PATCH and DELETE are checked only when
// a body is present (a bodiless DELETE is the normal form). ContentLength -1
// (chunked) counts as a body.
func JSONBodyRequired(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut:
		return true
	case http.MethodPatch, http.MethodDelete:
		return r.ContentLength != 0
	}
	return false
}

// JSONContent reports whether Content-Type parses as application/json.
func JSONContent(r *http.Request) bool {
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && typ == "application/json"
}
