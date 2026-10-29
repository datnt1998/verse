package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
)

// The hosted deployment terminates TLS at Traefik and forwards plain HTTP to
// the app container. Everything the app derives from the request for security
// (client IP for limiters, "is this HTTPS" for Secure cookies, the allowed
// origin for state-changing requests) must therefore come from the trusted
// proxy boundary — never from self-declared headers on an untrusted peer.
//
// TRUSTED_PROXIES is the explicit boundary: only peers inside those CIDRs may
// supply X-Forwarded-*. Unset trusts no peer, loopback included, so a loopback
// proxy such as `tailscale serve` cannot mark a request HTTPS through
// X-Forwarded-Proto until TRUSTED_PROXIES=127.0.0.1/32 is set. `serve` always
// installs a ProxyConfig; the legacy loopback rule in secureRequest only runs
// for requests that never went through normalize (handlers called directly).
// There is exactly one name: a second alias would let an operator change a
// setting that is silently ignored.

// ProxyConfig is the parsed trusted-proxy contract.
type ProxyConfig struct {
	// Trusted is the peer CIDR list. Empty trusts no peer: X-Forwarded-* is
	// ignored, loopback included.
	Trusted []*net.IPNet
	// PublicURL is the canonical public base URL. Browser state-changing
	// requests are accepted from this origin or from the request's own Host;
	// SECURE_COOKIES requires it to be https.
	PublicURL *url.URL
	// SecureCookies forces every session cookie to carry Secure. Hosted
	// deployments must set it; startup fails when PUBLIC_URL is not HTTPS.
	SecureCookies bool
}

type proxyContextKey struct{}

type proxyContext struct {
	clientIP string
	secure   bool
}

// ProxyConfigFromEnv parses TRUSTED_PROXIES, PUBLIC_URL and SECURE_COOKIES
// strictly. A malformed CIDR, a wildcard trust entry or Secure cookies without
// an HTTPS public URL is a startup error — never a silent downgrade.
func ProxyConfigFromEnv() (*ProxyConfig, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	raw := get("TRUSTED_PROXIES")
	cfg := &ProxyConfig{}
	trusted, err := ParseTrustedProxies(raw)
	if err != nil {
		return nil, err
	}
	cfg.Trusted = trusted

	if raw := get("PUBLIC_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("PUBLIC_URL=%q không phải URL tuyệt đối: %w", raw, errs.ErrConfig)
		}
		switch u.Scheme {
		case "https", "http":
		default:
			return nil, fmt.Errorf("PUBLIC_URL=%q phải dùng http hoặc https: %w", raw, errs.ErrConfig)
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("PUBLIC_URL không được chứa query/fragment: %w", errs.ErrConfig)
		}
		cfg.PublicURL = u
	}

	if v := get("SECURE_COOKIES"); v != "" {
		switch v {
		case "true", "1":
			cfg.SecureCookies = true
		case "false", "0":
		default:
			return nil, fmt.Errorf("SECURE_COOKIES=%q không hợp lệ (true|false): %w", v, errs.ErrConfig)
		}
	}
	if cfg.SecureCookies {
		if cfg.PublicURL == nil || cfg.PublicURL.Scheme != "https" {
			return nil, fmt.Errorf("SECURE_COOKIES=true yêu cầu PUBLIC_URL là https://…: %w", errs.ErrConfig)
		}
	}
	return cfg, nil
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
				return nil, fmt.Errorf("TRUSTED_PROXIES: %q không phải IP/CIDR: %w", field, errs.ErrConfig)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			entry = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %q không phải CIDR: %w", field, errs.ErrConfig)
		}
		if ones, _ := network.Mask.Size(); ones == 0 {
			return nil, fmt.Errorf("TRUSTED_PROXIES: không được tin toàn bộ mạng (%q): %w", field, errs.ErrConfig)
		}
		out = append(out, network)
	}
	return out, nil
}

// trusted reports whether ip is inside the configured proxy boundary.
func (c *ProxyConfig) trusted(ip net.IP) bool {
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

// normalize returns a request whose context carries the security-relevant
// request facts. Headers are only honoured when the immediate peer is a
// trusted proxy; otherwise the raw peer address and TLS state are used.
func (c *ProxyConfig) normalize(r *http.Request) *http.Request {
	if c == nil {
		return r
	}
	peer := parseRemoteIP(r.RemoteAddr)
	facts := proxyContext{clientIP: peer.String(), secure: r.TLS != nil || c.SecureCookies}
	if peer == nil {
		facts.clientIP = r.RemoteAddr
	}

	if peer != nil && c.trusted(peer) {
		if proto := forwardedProto(r); proto != "" {
			facts.secure = proto == "https" || c.SecureCookies
		}
		if ip := c.forwardedClient(peer, r.Header.Values("X-Forwarded-For")); ip != nil {
			facts.clientIP = ip.String()
		}
	}
	if facts.clientIP == "" {
		facts.clientIP = r.RemoteAddr
	}
	return r.WithContext(context.WithValue(r.Context(), proxyContextKey{}, facts))
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
func (c *ProxyConfig) forwardedClient(peer net.IP, headers []string) net.IP {
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

// allowedOrigin reports whether an Origin value is one this deployment serves.
// PUBLIC_URL is authoritative; the request's own Host is accepted so same-origin
// requests keep working when the proxy rewrites Host.
func (c *ProxyConfig) allowedOrigin(r *http.Request, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "null" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if c != nil && c.PublicURL != nil && strings.EqualFold(u.Host, c.PublicURL.Host) {
		return true
	}
	return strings.EqualFold(u.Host, r.Host)
}

// crossSiteRequest rejects browser-issued state changes from another origin, in
// every mode. SameSite=Strict does not separate ports on one host:
// 127.0.0.1:8789 and 127.0.0.1:8790 are the same site, so only the Origin check
// keeps a sibling app's page from writing here. A request without Origin (CLI,
// worker, curl) passes unless the browser marks it cross-site.
func (c *ProxyConfig) crossSiteRequest(r *http.Request) bool {
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
	return !c.allowedOrigin(r, origin)
}

func parseRemoteIP(addr string) net.IP {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

// normalized reads the trusted-proxy facts from the request context. ok=false
// means ServeHTTP never normalized this request (tests calling handlers
// directly), so callers fall back to the legacy peer-based rules.
func normalized(r *http.Request) (proxyContext, bool) {
	facts, ok := r.Context().Value(proxyContextKey{}).(proxyContext)
	return facts, ok
}
