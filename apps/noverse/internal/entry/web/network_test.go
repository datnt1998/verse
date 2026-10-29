package web

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const loopbackRemote = "127.0.0.1:5555"

// enableTestNetwork gắn Network bind vào 127.0.0.1 cổng ngẫu nhiên (không phụ thuộc card mạng của máy chạy test).
func enableTestNetwork(t *testing.T, g *Gateway) *Network {
	t.Helper()
	n := g.EnableNetwork(NetworkOptions{Port: "0", Addresses: func() []string { return []string{"127.0.0.1"} }})
	t.Cleanup(func() { _ = n.Set(false) })
	return n
}

func putNetwork(t *testing.T, tg *testGateway, cookie, body, remote string, headers map[string]string) map[string]any {
	t.Helper()
	w := doFrom(t, tg.g, "PUT", "/api/system/network", cookie, body, remote, headers)
	if w.Code != 200 {
		t.Fatalf("PUT network %s -> %d %s", body, w.Code, w.Body.String())
	}
	return decodeJSONBody(t, w)
}

func reachable(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", u.Host, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func readServeJSON(t *testing.T) string {
	t.Helper()
	home, _ := os.UserHomeDir()
	raw, err := os.ReadFile(filepath.Join(home, ".ainovel", "serve.json"))
	if err != nil {
		t.Fatalf("serve.json: %v", err)
	}
	return string(raw)
}

func TestNetworkToggleKeepsSessionsAndWorkers(t *testing.T) {
	tg := newTestGateway(t)
	n := enableTestNetwork(t, tg.g)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")

	// Một worker đang viết: bật/tắt LAN không được động tới nó.
	done := make(chan struct{})
	var once sync.Once
	slot := tg.g.slot("proj/01-story")
	slot.worker = fakeWorker(done, &once)
	slot.running = true
	assertWorker := func(when string) {
		t.Helper()
		select {
		case <-done:
			t.Fatalf("%s: worker was stopped", when)
		default:
		}
		if open, running := tg.g.workerState("proj/01-story"); !open || !running {
			t.Fatalf("%s: worker state open=%v running=%v", when, open, running)
		}
	}

	if m := decodeJSONBody(t, doFrom(t, tg.g, "GET", "/api/system", admin, "", loopbackRemote, nil)); m["lan"] != false {
		t.Fatalf("lan before: %v", m["lan"])
	}
	if n.Open() {
		t.Fatal("listener open before the toggle")
	}

	on := putNetwork(t, tg, admin, `{"lan":true}`, loopbackRemote, nil)
	if on["lan"] != true {
		t.Fatalf("PUT on: %v", on)
	}
	links, _ := on["links"].([]any)
	if len(links) != 1 {
		t.Fatalf("links: %v", on["links"])
	}
	link := links[0].(map[string]any)
	if !strings.HasPrefix(link["url"].(string), "http://127.0.0.1:") || !strings.Contains(link["url"].(string), "/p/") || link["svg"] == "" {
		t.Fatalf("link: %v", link)
	}
	if !n.Open() || len(n.URLs()) != 1 {
		t.Fatalf("listener not open: %v", n.URLs())
	}
	base := n.URLs()[0]
	// Listener thật sự nhận kết nối, dùng chung handler: session cũ vẫn hợp lệ qua nó.
	req, _ := http.NewRequest("GET", base+"/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: writer})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"username":"writer"`) {
		t.Fatalf("session over LAN listener -> %d %s", resp.StatusCode, body)
	}
	if m := decodeJSONBody(t, doFrom(t, tg.g, "GET", "/api/system", admin, "", loopbackRemote, nil)); m["lan"] != true {
		t.Fatalf("lan after on: %v", m["lan"])
	}
	if raw := readServeJSON(t); !strings.Contains(raw, `"lan": true`) {
		t.Fatalf("serve.json after on: %s", raw)
	}
	assertWorker("after on")

	// Bật lại khi đang bật: idempotent, không bind lại.
	if again := putNetwork(t, tg, admin, `{"lan":true}`, loopbackRemote, nil); again["lan"] != true || n.URLs()[0] != base {
		t.Fatalf("second on rebound the listener: %v %v", again, n.URLs())
	}

	off := putNetwork(t, tg, admin, `{"lan":false}`, loopbackRemote, nil)
	if off["lan"] != false || len(off["links"].([]any)) != 0 {
		t.Fatalf("PUT off: %v", off)
	}
	if n.Open() || reachable(base) {
		t.Fatal("listener still reachable after off")
	}
	if raw := readServeJSON(t); !strings.Contains(raw, `"lan": false`) {
		t.Fatalf("serve.json after off: %s", raw)
	}
	if code := tg.do("GET", "/api/me", writer, "").Code; code != 200 {
		t.Fatalf("session lost after toggle: %d", code)
	}
	if code := tg.do("GET", "/api/me", admin, "").Code; code != 200 {
		t.Fatalf("admin session lost after toggle: %d", code)
	}
	assertWorker("after off")
	if m := decodeJSONBody(t, doFrom(t, tg.g, "GET", "/api/system", admin, "", loopbackRemote, nil)); m["lan"] != false {
		t.Fatalf("lan after off: %v", m["lan"])
	}
}

func TestNetworkPutPermissions(t *testing.T) {
	tg := newTestGateway(t)
	n := enableTestNetwork(t, tg.g)
	admin := tg.login("admin", "admin-pass")
	writer := tg.login("writer", "writer-pass")

	cases := []struct {
		name, cookie, remote string
		headers              map[string]string
		want                 int
	}{
		{"writer on loopback", writer, loopbackRemote, nil, 403},
		{"admin off loopback", admin, "192.0.2.1:1234", nil, 403},
		{"admin behind proxy", admin, loopbackRemote, map[string]string{"X-Forwarded-For": "10.0.0.1"}, 403},
	}
	for _, c := range cases {
		w := doFrom(t, tg.g, "PUT", "/api/system/network", c.cookie, `{"lan":true}`, c.remote, c.headers)
		if w.Code != c.want {
			t.Fatalf("%s -> %d %s", c.name, w.Code, w.Body.String())
		}
		if n.Open() {
			t.Fatalf("%s opened the listener", c.name)
		}
	}
	if w := doFrom(t, tg.g, "PUT", "/api/system/network", admin, `{}`, loopbackRemote, nil); w.Code != 400 {
		t.Fatalf("missing lan -> %d", w.Code)
	}
	if w := doFrom(t, tg.g, "GET", "/api/system/network", admin, "", loopbackRemote, nil); w.Code != 404 {
		t.Fatalf("GET network -> %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".ainovel", "serve.json")); err == nil {
		t.Fatal("rejected requests must not write serve.json")
	}
}

func TestNetworkSaveFailureRollsBack(t *testing.T) {
	tg := newTestGateway(t)
	n := tg.g.EnableNetwork(NetworkOptions{
		Port:      "0",
		Addresses: func() []string { return []string{"127.0.0.1"} },
		Save:      func(bool) error { return errors.New("đĩa đầy") },
	})
	t.Cleanup(func() { _ = n.Set(false) })
	admin := tg.login("admin", "admin-pass")
	w := doFrom(t, tg.g, "PUT", "/api/system/network", admin, `{"lan":true}`, loopbackRemote, nil)
	if w.Code != 500 || n.Open() {
		t.Fatalf("save failure -> %d open=%v", w.Code, n.Open())
	}
}

func TestNetworkWithoutAddressOrController(t *testing.T) {
	tg := newTestGateway(t)
	admin := tg.login("admin", "admin-pass")
	// Chưa gắn Network (serve chạy với --addr cố định) → 409.
	if w := doFrom(t, tg.g, "PUT", "/api/system/network", admin, `{"lan":true}`, loopbackRemote, nil); w.Code != 409 {
		t.Fatalf("no controller -> %d", w.Code)
	}
	n := tg.g.EnableNetwork(NetworkOptions{Port: "0", Addresses: func() []string { return nil }})
	t.Cleanup(func() { _ = n.Set(false) })
	w := doFrom(t, tg.g, "PUT", "/api/system/network", admin, `{"lan":true}`, loopbackRemote, nil)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "Wi-Fi") {
		t.Fatalf("no address -> %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".ainovel", "serve.json")); err == nil {
		t.Fatal("failed open must not be persisted")
	}
}

func TestNetworkCloseOnGatewayClose(t *testing.T) {
	tg := newTestGateway(t)
	n := enableTestNetwork(t, tg.g)
	if err := n.Set(true); err != nil {
		t.Fatal(err)
	}
	base := n.URLs()[0]
	if !reachable(base) {
		t.Fatal("listener not reachable")
	}
	tg.g.Close()
	if reachable(base) {
		t.Fatal("Gateway.Close left the LAN listener open")
	}
}
