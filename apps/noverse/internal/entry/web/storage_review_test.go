package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
)

func TestStorageProbeSharedQuotaAndStaleVersion(t *testing.T) {
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(403)
		_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
	}))
	defer provider.Close()
	f := storageFixture(t, true)
	access, secret := "test", "secret"
	candidate := bootstrap.StorageCandidate{Driver: "s3", Endpoint: provider.URL, Region: "garage", Bucket: "test", Delivery: "proxy", PathStyle: true, AllowHTTP: true, AccessKeyID: &access, Secret: &secret, Version: f.version(t) - 1}
	res := f.call(t, "PUT", "/api/system/storage", candidate)
	if res.Code != 409 || requests.Load() != 0 {
		t.Errorf("stale save probed provider: status=%d requests=%d", res.Code, requests.Load())
	}
	candidate.Version = f.version(t)
	for range 6 {
		f.fx.g.allowStorageTest("hosted-admin")
	}
	requests.Store(0)
	for _, mode := range []string{"json", "ndjson", "put"} {
		t.Run(mode, func(t *testing.T) {
			body, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			method, path := "POST", "/api/system/storage/test"
			if mode == "put" {
				method, path = "PUT", "/api/system/storage"
			}
			req := httptest.NewRequest(method, "http://127.0.0.1"+path, bytes.NewReader(body))
			req.AddCookie(&http.Cookie{Name: f.fx.g.cookiePolicy().Name, Value: f.cookie})
			req.Header.Set("Origin", "http://127.0.0.1")
			req.Header.Set("Content-Type", "application/json")
			if mode == "ndjson" {
				req.Header.Set("Accept", "application/x-ndjson")
			}
			rec := httptest.NewRecorder()
			f.fx.g.ServeHTTP(rec, req)
			if rec.Code != 429 || requests.Load() != 0 {
				t.Errorf("quota bypass: status=%d provider requests=%d", rec.Code, requests.Load())
			}
			if !strings.Contains(rec.Header().Get("Cache-Control"), "no-store") {
				t.Error("probe response may be cached")
			}
		})
	}
}

func TestStorageProbeJSONAndStreamNeverCached(t *testing.T) {
	f := storageFixture(t, true)
	for _, accept := range []string{"application/json", "application/x-ndjson"} {
		req := httptest.NewRequest("POST", "http://127.0.0.1/api/system/storage/test", strings.NewReader(`{"current":true}`))
		req.AddCookie(&http.Cookie{Name: f.fx.g.cookiePolicy().Name, Value: f.cookie})
		req.Header.Set("Origin", "http://127.0.0.1")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		f.fx.g.ServeHTTP(rec, req)
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: status=%d cache=%q", accept, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
}
