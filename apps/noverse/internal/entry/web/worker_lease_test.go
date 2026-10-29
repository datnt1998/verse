package web

import (
	"testing"
	"time"
)

// TestWorkerLeaseIsServerBound pins the difference between a client request and
// the server's authoritative bound: a worker may ask for a lease, but the
// server clamps it so a dead Mac cannot pin a job for a year and a very short
// request cannot lose a live job mid-synthesis.
func TestWorkerLeaseIsServerBound(t *testing.T) {
	a := &WorkerAPI{Lease: 120 * time.Second}
	cases := []struct {
		name string
		sec  int
		want time.Duration
	}{
		{"client asks too long", 24 * 60 * 60, maxWorkerLease},
		{"client asks too short", 1, minWorkerLease},
		{"client asks inside bounds", 120, 120 * time.Second},
		{"no request uses configured default", 0, 120 * time.Second},
		{"negative is treated as no request", -5, 120 * time.Second},
	}
	for _, tc := range cases {
		if got := a.leaseDuration(tc.sec); got != tc.want {
			t.Fatalf("%s: lease = %v, want %v", tc.name, got, tc.want)
		}
	}

	// A configured default is clamped too: the bound belongs to the server, not
	// to whoever wrote the config.
	b := &WorkerAPI{Lease: time.Hour}
	if got := b.leaseDuration(0); got != maxWorkerLease {
		t.Fatalf("configured default above bound = %v, want %v", got, maxWorkerLease)
	}
}
