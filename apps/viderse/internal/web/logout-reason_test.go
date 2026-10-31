package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/webapp"
)

func TestSSOLogoutRetainsRevokedCookie(t *testing.T) {
	p := newPGGateway(t)
	p.fx.CreateMember(t, "member", "member-pass-99", false, identity.AppNoverse, identity.AppViderse)
	v := httptest.NewServer(p.g)
	defer v.Close()
	n := httptest.NewServer(webapp.New(p.fx.Admin, webapp.Options{App: identity.AppNoverse, AppName: "Noverse"}))
	defer n.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	post := func(base, path, body string) *http.Response {
		t.Helper()
		res, err := client.Post(base+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	login := func() {
		if r := post(v.URL, "/api/login", `{"username":"member","password":"member-pass-99"}`); r.StatusCode != 200 {
			t.Fatalf("login=%d", r.StatusCode)
		}
	}
	login()
	u, _ := url.Parse(v.URL)
	before := jar.Cookies(u)[0].Value
	if r := post(v.URL, "/api/logout", `{}`); r.StatusCode != 204 {
		t.Fatalf("logout=%d", r.StatusCode)
	}
	if len(jar.Cookies(u)) == 0 {
		t.Fatal("logout deleted shared cookie; sibling cannot read reason")
	}
	for _, path := range []string{"/api/me", "/api/status"} {
		res, err := client.Get(n.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		err = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 401 || body["reason"] != "logout" || body["app"] != "viderse" {
			t.Fatalf("sibling=%d %v", res.StatusCode, body)
		}
	}
	login()
	if jar.Cookies(u)[0].Value == before {
		t.Fatal("login reused revoked token")
	}
	res, err := client.Get(n.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("new session=%d", res.StatusCode)
	}
}
