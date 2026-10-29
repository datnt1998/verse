package notify

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/hkdf"
)

func openTest(t *testing.T, opts Options) *Store {
	t.Helper()
	if opts.Directory == "" {
		opts.Directory = t.TempDir()
	}
	s, e := Open(opts)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s
}
func mustRecord(t *testing.T, s *Store, user string, n PushNotification) {
	t.Helper()
	if e := s.Record(user, n); e != nil {
		t.Fatal(e)
	}
}
func TestPrefsFilteringQuietAndErrorException(t *testing.T) {
	p := DefaultPrefs()
	day := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	night := day.Add(11 * time.Hour)
	if !permits(p, "gate", day) || permits(p, "gate", night) {
		t.Fatal("quiet hours must suppress ordinary pushes only at night")
	}
	p.Types.Gate = false
	if permits(p, "gate", day) {
		t.Fatal("disabled type must not push")
	}
	p.Enabled = false
	if permits(p, "job", day) || !permits(p, "error", night) || !permits(p, "security", night) {
		t.Fatal("error and security must bypass disabled preferences and quiet hours")
	}
	p.Quiet = Quiet{"09:00", "17:00"}
	p.Enabled = true
	p.Types.Gate = true
	if permits(p, "gate", day) || !permits(p, "gate", night) {
		t.Fatal("same-day quiet range is incorrect")
	}
	p.Quiet = Quiet{}
	if !permits(p, "gate", night) {
		t.Fatal("empty quiet range must disable quiet hours")
	}
	s := openTest(t, Options{})
	p = s.Prefs("a")
	p.Types.Security = false
	p.Types.Error = false
	updated, e := s.PutPrefs("a", p, p.Version)
	if e != nil || !updated.Types.Security || !updated.Types.Error {
		t.Fatalf("mandatory types: %+v %v", updated, e)
	}
	if _, e = s.PutPrefs("a", p, p.Version); e != ErrConflict {
		t.Fatalf("stale version accepted: %v", e)
	}
}
func TestDigestGrouping(t *testing.T) {
	now := time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC)
	var clock atomic.Value
	clock.Store(now)
	s := openTest(t, Options{Clock: func() time.Time { return clock.Load().(time.Time) }})
	for i := 20; i < 24; i++ {
		mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/s", Chapter: i, URL: "/read/p/s/20"})
	}
	if rows, _, _ := s.Inbox("a", ""); len(rows) != 0 {
		t.Fatal("five-chapter digest sent prematurely")
	}
	mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/s", Chapter: 24})
	rows, count, _ := s.Inbox("a", "")
	if len(rows) != 1 || count != 1 || rows[0].Title != "Xong 5 chương (20–24)" || rows[0].URL != "/read/p/s/20" {
		t.Fatalf("bad digest: %+v count=%d", rows, count)
	}
	p := s.Prefs("a")
	p.Digest = "day"
	if _, e := s.PutPrefs("a", p, p.Version); e != nil {
		t.Fatal(e)
	}
	mustRecord(t, s, "a", PushNotification{Type: "done", Story: "p/other", Chapter: 7})
	if e := s.FlushDigests(); e != nil {
		t.Fatal(e)
	}
	rows, _, _ = s.Inbox("a", "")
	if len(rows) != 1 {
		t.Fatal("daily digest delivered before 21:00")
	}
	clock.Store(now.Add(time.Hour))
	if e := s.FlushDigests(); e != nil {
		t.Fatal(e)
	}
	rows, _, _ = s.Inbox("a", "")
	if len(rows) != 2 || rows[0].Story != "p/other" {
		t.Fatalf("daily digest missing: %+v", rows)
	}
	p = s.Prefs("b")
	p.Digest = "each"
	if _, e := s.PutPrefs("b", p, p.Version); e != nil {
		t.Fatal(e)
	}
	mustRecord(t, s, "b", PushNotification{Type: "done", Story: "p/s", Chapter: 1})
	rows, _, _ = s.Inbox("b", "")
	if len(rows) != 1 {
		t.Fatal("each-chapter notification missing")
	}
}
func TestPermissionFilter(t *testing.T) {
	var allowed atomic.Bool
	allowed.Store(true)
	s := openTest(t, Options{Allowed: func(user, story string) bool { return allowed.Load() && user == "reader" && story == "p/allowed" }})
	mustRecord(t, s, "reader", PushNotification{Type: "gate", Story: "p/secret"})
	mustRecord(t, s, "reader", PushNotification{Type: "gate", Story: "p/allowed"})
	mustRecord(t, s, "other", PushNotification{Type: "gate", Story: "p/allowed"})
	rows, count, _ := s.Inbox("reader", "")
	if len(rows) != 1 || rows[0].Story != "p/allowed" || count != 1 {
		t.Fatalf("unauthorized notification stored: %+v, %d", rows, count)
	}
	s.mu.Lock()
	stored := len(s.user("reader").Inbox)
	other := len(s.user("other").Inbox)
	s.mu.Unlock()
	if stored != 1 || other != 0 {
		t.Fatalf("permission denied records persisted: %d/%d", stored, other)
	}
	allowed.Store(false)
	rows, count, _ = s.Inbox("reader", "")
	if len(rows) != 0 || count != 0 {
		t.Fatal("revoked story remains visible")
	}
}
func TestReadAll(t *testing.T) {
	s := openTest(t, Options{})
	for i := range 3 {
		mustRecord(t, s, "a", PushNotification{Type: "gate", Story: "p/s", Chapter: i + 1})
	}
	mustRecord(t, s, "b", PushNotification{Type: "gate"})
	n, e := s.Read("a", nil, true)
	if e != nil || n != 0 {
		t.Fatalf("read all: %d %v", n, e)
	}
	rows, n, _ := s.Inbox("a", "")
	if n != 0 || len(rows) != 3 {
		t.Fatalf("bad inbox after read-all: %+v count=%d", rows, n)
	}
	for _, v := range rows {
		if !v.Read {
			t.Fatal("read-all left unread item")
		}
	}
	_, n, _ = s.Inbox("b", "")
	if n != 1 {
		t.Fatal("read-all changed another user")
	}
}
func TestEncryptedPushVAPIDAndPrune410(t *testing.T) {
	private, e := ecdh.P256().GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	auth := make([]byte, 16)
	if _, e = rand.Read(auth); e != nil {
		t.Fatal(e)
	}
	received := make(chan struct {
		body                   []byte
		auth, encoding, method string
	}, 1)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received <- struct {
			body                   []byte
			auth, encoding, method string
		}{b, r.Header.Get("Authorization"), r.Header.Get("Content-Encoding"), r.Method}
		w.WriteHeader(410)
	}))
	defer endpoint.Close()
	s := openTest(t, Options{})
	sub := Subscription{Endpoint: endpoint.URL, Keys: Keys{base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(auth)}, Device: "Điện thoại"}
	if e = s.Subscribe("a", "session-1", sub); e != nil {
		t.Fatal(e)
	}
	mustRecord(t, s, "a", PushNotification{Type: "security", Title: "Đăng nhập trên thiết bị mới", URL: "/more/devices"})
	select {
	case got := <-received:
		if got.method != "POST" || got.encoding != "aes128gcm" || !strings.HasPrefix(got.auth, "vapid t=") || !strings.Contains(got.auth, "k="+s.PublicKey()) {
			t.Fatalf("missing Web Push authentication: %+v", got)
		}
		b := got.body
		if len(b) < 86 || bytes.Contains(b, []byte("Đăng nhập")) {
			t.Fatal("payload is not encrypted")
		}
		pub, e := ecdh.P256().NewPublicKey(b[21 : 21+int(b[20])])
		if e != nil {
			t.Fatal(e)
		}
		shared, e := private.ECDH(pub)
		if e != nil {
			t.Fatal(e)
		}
		derive := func(secret, salt, info []byte, n int) []byte {
			out := make([]byte, n)
			if _, e := io.ReadFull(hkdf.New(sha256.New, secret, salt, info), out); e != nil {
				t.Fatal(e)
			}
			return out
		}
		info := append([]byte("WebPush: info\x00"), private.PublicKey().Bytes()...)
		info = append(info, pub.Bytes()...)
		ikm := derive(shared, auth, info, 32)
		key := derive(ikm, b[:16], []byte("Content-Encoding: aes128gcm\x00"), 16)
		nonce := derive(ikm, b[:16], []byte("Content-Encoding: nonce\x00"), 12)
		block, e := aes.NewCipher(key)
		if e != nil {
			t.Fatal(e)
		}
		gcm, e := cipher.NewGCM(block)
		if e != nil {
			t.Fatal(e)
		}
		plain, e := gcm.Open(nil, nonce, b[21+int(b[20]):], nil)
		if e != nil {
			t.Fatal(e)
		}
		plain = bytes.TrimRight(plain, "\x00")
		if plain[len(plain)-1] != 2 {
			t.Fatal("missing Web Push record delimiter")
		}
		var item PushNotification
		if e = json.Unmarshal(plain[:len(plain)-1], &item); e != nil {
			t.Fatal(e)
		}
		if item.Title != "Đăng nhập trên thiết bị mới" || item.ID == "" {
			t.Fatalf("bad decrypted payload: %+v", item)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("encrypted push not POSTed")
	}
	deadline := time.After(3 * time.Second)
	for len(s.Subscriptions("a")) != 0 {
		select {
		case <-deadline:
			t.Fatal("410 subscription was not pruned")
		case <-time.After(time.Millisecond):
		}
	}
}
func TestVAPIDAndInboxPersistUnderHOME(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, e := Open(Options{})
	if e != nil {
		t.Fatal(e)
	}
	key := s.PublicKey()
	mustRecord(t, s, "a", PushNotification{Type: "security", Session: "new-session"})
	s.Close()
	path := filepath.Join(os.Getenv("HOME"), ".ainovel", "push", "vapid.json")
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("VAPID permissions: %v %v", info, e)
	}
	s, e = Open(Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.PublicKey() != key {
		t.Fatal("VAPID key changed on restart")
	}
	rows, _, _ := s.Inbox("a", "")
	if len(rows) != 1 {
		t.Fatal("inbox not persisted")
	}
	item, ok := s.Find("a", rows[0].ID)
	if !ok || item.Session != "new-session" {
		t.Fatalf("security target lost on restart: %+v", item)
	}
}
