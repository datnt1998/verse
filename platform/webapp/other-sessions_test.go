package webapp_test

import (
	"net/http"
	"testing"
)

func TestRevokeOtherDevicesKeepsCurrent(t *testing.T) {
	ts := newTestServer(t, true)
	current := ts.sessionCookie()
	other := ts.sessionCookie()
	w := ts.do(http.MethodPost, "/api/me/sessions/revoke-all", current, `{}`)
	if w.Code != 200 {
		t.Fatalf("revoke status %d", w.Code)
	}
	if w := ts.do(http.MethodGet, "/api/me", current, ""); w.Code != 200 {
		t.Fatalf("current device revoked: %d", w.Code)
	}
	if w := ts.do(http.MethodGet, "/api/me", other, ""); w.Code != 401 {
		t.Fatalf("other device survived: %d", w.Code)
	}
}
