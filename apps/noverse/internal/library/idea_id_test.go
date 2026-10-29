package library

import (
	"testing"
	"time"
)

// TestIdeaIDTimestampIsUTC pins that the timestamp embedded in an idea id (an
// operator-visible directory/doc name) is UTC. The id is parsed back into
// CreatedAt, so generation and parsing must agree on one zone; otherwise the
// same draft drifts by the host offset when the deployment moves timezone.
func TestIdeaIDTimestampIsUTC(t *testing.T) {
	lib, _ := openTemp(t)
	if _, err := lib.CreateProject("du-an", ProjectFile{Name: "Dự Án"}); err != nil {
		t.Fatal(err)
	}
	// Stamp granularity is one minute: allow the truncated minute.
	before := time.Now().UTC().Add(-90 * time.Second)
	idea, err := lib.CreateIdea("du-an", "# Ý tưởng\nNội dung", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(idea.ID) < 13 {
		t.Fatalf("idea id too short: %q", idea.ID)
	}
	stamp, err := time.ParseInLocation("20060102-1504", idea.ID[:13], time.UTC)
	if err != nil {
		t.Fatalf("parse id stamp %q: %v", idea.ID, err)
	}
	if stamp.Before(before) || stamp.After(time.Now().UTC().Add(2*time.Second)) {
		t.Fatalf("idea id stamp %s is not UTC (now %s, host %s)", stamp, time.Now().UTC(), time.Now())
	}
	if idea.CreatedAt == nil || idea.CreatedAt.Location() != time.UTC {
		t.Fatalf("createdAt = %v, want UTC", idea.CreatedAt)
	}
	list, err := lib.ListIdeas("du-an")
	if err != nil || len(list) != 1 || list[0].CreatedAt == nil || list[0].CreatedAt.Location() != time.UTC {
		t.Fatalf("list round-trip lost UTC: %+v err=%v", list, err)
	}
}
