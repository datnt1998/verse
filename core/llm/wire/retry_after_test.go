package litellm

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPErrorPreservesRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		min, max    int
	}{
		{"seconds", "240", 240, 240},
		{"date", time.Now().Add(4 * time.Minute).UTC().Format(http.TimeFormat), 239, 240},
		{"invalid", "not-a-duration", 0, 0},
		{"negative", "-1", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewHTTPError("test", http.StatusTooManyRequests, "rate limit", http.Header{"Retry-After": {tc.value}})
			if err.RetryAfter < tc.min || err.RetryAfter > tc.max {
				t.Fatalf("retryAfter=%d want %d..%d", err.RetryAfter, tc.min, tc.max)
			}
		})
	}
	if err := NewHTTPError("test", http.StatusTooManyRequests, "rate limit"); err.RetryAfter != 0 {
		t.Fatalf("invented retry hint: %d", err.RetryAfter)
	}
}
