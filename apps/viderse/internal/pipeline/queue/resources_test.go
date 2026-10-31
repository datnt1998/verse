package queue

import (
	"context"
	"errors"
	"testing"
)

func TestResourceSlotsCancellationAndRelease(t *testing.T) {
	slots := NewResourceSlots()
	release, err := slots.Acquire(context.Background(), "images", 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = slots.Acquire(ctx, "images", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquisition: %v", err)
	}
	release()
	release() // idempotent, must not free another holder's permit.
	next, err := slots.Acquire(context.Background(), "images", 1)
	if err != nil {
		t.Fatal(err)
	}
	next()
}
func TestAutoPickInterleavesAndRotatesCompetitors(t *testing.T) {
	got := InterleaveCandidates([]string{"a", "b"}, map[string][]string{"a": {"a0", "a1"}, "b": {"b0", "b1"}}, "a")
	want := []string{"b0", "a0", "b1", "a1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidates=%v want %v", got, want)
		}
	}
}
