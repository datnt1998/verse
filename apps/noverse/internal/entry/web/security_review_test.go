package web

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

func TestReviewInitialBacklogFiltersUnauthorizedWatch(t *testing.T) {
	tg := newTestGateway(t)
	hub := tg.g.events()
	hub.SetLiveStream("_/oldbook", "PUBLIC_BODY")
	hub.SetLiveStream("proj/hidden", "PRIVATE_BODY")
	hub.AppendLiveEvent("proj/hidden", json.RawMessage(`{"id":"PRIVATE_EVENT"}`))
	user := identity.User{Username: "reader", Role: identity.RoleMember, Projects: map[string]identity.ProjectRole{"_": identity.ProjectView}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("GET", "/api/events?watch=proj/hidden&watch=_/oldbook", nil).WithContext(ctx)
	w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
	done := make(chan struct{})
	go func() { tg.g.serveEvents(w, r, user); close(done) }()
	w.until(t, "PUBLIC_BODY")
	cancel()
	<-done
	w.mu.Lock()
	body := w.body.String()
	w.mu.Unlock()
	if strings.Contains(body, "PRIVATE_BODY") || strings.Contains(body, "PRIVATE_EVENT") || strings.Contains(body, "event: error") {
		t.Fatalf("unauthorized watch leaked initial backlog: %s", body)
	}
}
func TestReviewStoryStreamsCloseAfterStoredRevocation(t *testing.T) {
	for _, stream := range []string{"events/live", "audio/events"} {
		for _, change := range []string{"projects", "lock", "session"} {
			t.Run(stream+"/"+change, func(t *testing.T) {
				tg := newTestGateway(t)
				stubNextWorker(t, tg.g)
				reader := tg.login("reader", "reader-pass")
				tg.g.transport = transportFunc(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path != "/api/events" {
						return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
					}
					body, writer := io.Pipe()
					go func() {
						defer writer.Close()
						_, _ = io.WriteString(writer, "event: snapshot\ndata: {}\n\n")
						<-r.Context().Done()
					}()
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: body}, nil
				})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				r := httptest.NewRequest("GET", "/api/projects/_/stories/oldbook/"+stream, nil).WithContext(ctx)
				r.AddCookie(&http.Cookie{Name: sessionCookie, Value: reader})
				w := &streamWriter{signal: make(chan struct{}, 1), header: make(http.Header)}
				done := make(chan struct{})
				go func() { tg.g.ServeHTTP(w, r); close(done) }()
				if stream == "events/live" {
					w.until(t, "event: snapshot")
				} else {
					w.until(t, ": audio")
				}
				switch change {
				case "projects":
					projects := map[string]identity.ProjectRole{}
					if _, err := tg.g.accounts.UpdateUser("reader", identity.UserPatch{Projects: &projects}); err != nil {
						t.Fatal(err)
					}
				case "lock":
					disabled := true
					if _, err := tg.g.accounts.UpdateUser("reader", identity.UserPatch{Disabled: &disabled}); err != nil {
						t.Fatal(err)
					}
				case "session":
					if err := tg.g.accounts.RevokeSession(strings.Split(reader, ".")[0], func(string) bool { return true }); err != nil {
						t.Fatal(err)
					}
				}
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					cancel()
					select {
					case <-done:
					case <-time.After(time.Second):
					}
					t.Fatal("story stream survived stored authorization revocation for 2s")
				}
			})
		}
	}
}
func TestReviewSelfRenameAuditActor(t *testing.T) {
	g, _ := newAuthGateway(t)
	cookie := authLogin(t, g, "admin", "admin-pass")
	if w := authReq(g, "POST", "/api/me", `{"name":"Tên mới"}`, cookie); w.Code != 200 {
		t.Fatal(w.Body)
	}
	page, err := g.accounts.Audit(identity.AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) == 0 || page.Items[0].Actor == nil || page.Items[0].Actor.Username != "admin" {
		t.Fatalf("self rename attributed to system: %+v", page.Items)
	}
}
func TestReviewLoginAuditDoesNotBlameIPLockOnUsername(t *testing.T) {
	g, s := newAuthGateway(t)
	for _, name := range []string{"u1", "u2", "u3", "u4"} {
		if _, err := s.CreateUser(identity.NewUser{Username: name, Password: "valid-pass"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"u1", "u2", "u3", "u4", "admin"} {
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"`+name+`","password":"wrong"}`))
		r.RemoteAddr = "192.0.2.1:1234"
		r.Header.Set("Content-Type", "application/json")
		g.ServeHTTP(httptest.NewRecorder(), r)
	}
	page, err := s.Audit(identity.AuditQuery{Type: "sec"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("IP lock blamed last username: %+v", page.Items)
	}
}

// TestReviewSecureCookieAndTrustedProxy pin quyết định phase 04: cookie chỉ
// mang Secure khi request thật sự HTTPS — TLS trực tiếp, hoặc X-Forwarded-Proto
// do proxy tin cậy (loopback peer như tailscale serve) gắn. Header do máy khách
// từ xa giả mạo không được đổi cờ; X-Forwarded-For không đổi clientIP giới hạn.
func TestReviewSecureCookieAndTrustedProxy(t *testing.T) {
	newReq := func(remoteAddr, xfp string, directTLS bool) *http.Request {
		r := httptest.NewRequest("POST", "/api/login", nil)
		r.RemoteAddr = remoteAddr
		if xfp != "" {
			r.Header.Set("X-Forwarded-Proto", xfp)
		}
		if directTLS {
			r.TLS = &tls.ConnectionState{}
		}
		return r
	}
	for _, tc := range []struct {
		name   string
		r      *http.Request
		secure bool
	}{
		{"plain http", newReq("192.0.2.1:9", "", false), false},
		{"direct tls", newReq("192.0.2.1:9", "", true), true},
		{"loopback proxy xfp", newReq("127.0.0.1:9", "https", false), true},
		{"spoofed xfp remote", newReq("192.0.2.1:9", "https", false), false},
		{"spoofed xfp lan", newReq("192.168.1.20:9", "https", false), false},
		{"xfp http loopback", newReq("127.0.0.1:9", "http", false), false},
	} {
		if got := secureRequest(tc.r); got != tc.secure {
			t.Fatalf("%s: secureRequest = %v", tc.name, got)
		}
	}
	// X-Forwarded-For không đổi client IP (limiter/recovery keyed on peer).
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("X-Forwarded-For", "10.0.0.7")
	if ip := clientIP(r); ip != "192.0.2.1" {
		t.Fatalf("clientIP honoured spoofed XFF: %q", ip)
	}
}
