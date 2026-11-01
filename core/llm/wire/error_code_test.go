package litellm

import (
	"net/http"
	"testing"
)

func TestHTTPErrorPreservesStructuredTypeWhenCodeAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct{ name, body, code string }{
		{"billing type", `{"error":{"type":"insufficient_quota","message":"Account unavailable"}}`, "insufficient_quota"},
		{"rate limit type", `{"error":{"type":"rate_limit_exceeded","message":"Too many requests"}}`, "rate_limit_exceeded"},
		{"explicit code wins", `{"error":{"code":"api_key_expired","type":"insufficient_quota","message":"Update account"}}`, "api_key_expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewHTTPError("test", 429, tc.body, http.Header{"Retry-After": []string{"37"}})
			if err.Code != tc.code || err.StatusCode != 429 || err.RetryAfter != 37 {
				t.Fatalf("structured facts lost: %+v", err)
			}
		})
	}
}
