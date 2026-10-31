package web

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// Hosts launched in-process share models.DefaultRegistry with HTTP contract
// tests. Their asynchronous pricing refresh outlives Host.Close, so per-test
// transport replacement or registry restoration cannot isolate a late response.
// Keep the external metadata source empty for this test process (including
// subprocess fixtures): the registry retains its compiled baseline. Gateway
// contracts do not depend on live pricing or upstream metadata changes.
func TestMain(m *testing.M) {
	upstream := http.DefaultTransport
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && r.URL.Scheme == "https" && r.URL.Host == "openrouter.ai" && r.URL.Path == "/api/v1/models" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
				Request:    r,
			}, nil
		}
		return upstream.RoundTrip(r)
	})
	os.Exit(m.Run())
}
