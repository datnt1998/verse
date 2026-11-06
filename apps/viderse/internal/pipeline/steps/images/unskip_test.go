package images

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

func unskipFixture() Manifest {
	m := reviewFixture()
	other := m.Items[0]
	other.ID = "scene-2"
	m.Items = append(m.Items, other)
	return m
}

func TestUnskipRestoresPersistedExactReviewState(t *testing.T) {
	for _, status := range []string{"review", "approved", "failed", "blocked", "legacy-ready"} {
		t.Run(status, func(t *testing.T) {
			m := unskipFixture()
			one, two := 1, 2
			i := &m.Items[0]
			i.Status = status
			i.CurrentVersion = &one
			i.ReviewVersion = &two
			i.SkipReason = "previous reason"
			i.Versions = append(i.Versions, Version{Version: 2, SourceFingerprint: SourceFingerprint(*i)})
			before := *i
			skipped, err := Review(m, i.ID, "skip", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if skipped.Items[0].BeforeSkip == nil || skipped.Items[0].CurrentVersion != nil || skipped.Items[0].ReviewVersion != nil {
				t.Fatalf("skip did not preserve hidden state: %+v", skipped.Items[0])
			}
			raw, err := json.Marshal(skipped)
			if err != nil {
				t.Fatal(err)
			}
			var reloaded Manifest
			if err := json.Unmarshal(raw, &reloaded); err != nil {
				t.Fatal(err)
			}
			restored, err := Review(reloaded, i.ID, "unskip", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restored.Items[0], before) {
				t.Fatalf("state changed across persisted undo:\nbefore=%+v\nafter=%+v", before, restored.Items[0])
			}
			if restored.Revision != m.Revision+2 || generationPending(restored) {
				t.Fatalf("undo queued generation or lost revision: %+v", restored)
			}
			if !reflect.DeepEqual(m.Items[0], before) {
				t.Fatal("review mutated input manifest")
			}
		})
	}
}

func TestUnskipReviewCandidateDoesNotApproveOrInventSelection(t *testing.T) {
	m := unskipFixture()
	skipped, err := Review(m, "scene-1", "skip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Review(skipped, "scene-1", "unskip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	i := restored.Items[0]
	if i.Status != "review" || i.CurrentVersion != nil || i.ReviewVersion == nil || *i.ReviewVersion != 1 || ImagesApproved(restored) {
		t.Fatalf("undo implicitly chose/approved a candidate: %+v", i)
	}
}

func TestUnskipRejectsMissingOwnerSnapshotAndUnsafeState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Item)
	}{
		{"legacy owner", func(i *Item) { i.BeforeSkip = nil }},
		{"refused", func(i *Item) { i.SkipReason = "refused" }},
		{"already restored", func(i *Item) { i.Status = "review" }},
		{"pending snapshot", func(i *Item) { i.BeforeSkip.Status = "pending" }},
		{"queued snapshot", func(i *Item) { i.BeforeSkip.Status = "queued" }},
		{"running snapshot", func(i *Item) { i.BeforeSkip.Status = "running" }},
		{"in flight", func(i *Item) { i.InFlight = &Flight{GenerationID: "draw"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Review(unskipFixture(), "scene-1", "skip", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&m.Items[0])
			_, err = Review(m, "scene-1", "unskip", "", nil)
			var ce *domain.CloneError
			if !errors.As(err, &ce) || ce.Code != domain.CloneErrConflict {
				t.Fatalf("unsafe undo returned %v", err)
			}
		})
	}
}

func TestSkipPreservesRequiredSceneAndDrawGuards(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		mutate     func(*Manifest)
	}{
		{"last scene", domain.CloneErrInvalidRequest, func(m *Manifest) { m.Items = m.Items[:2] }},
		{"cover", domain.CloneErrInvalidRequest, func(m *Manifest) { m.Items[0].Kind = "thumbnail" }},
		{"pending draw", domain.CloneErrConflict, func(m *Manifest) { m.Items[0].Status = "pending"; m.QueueOnly = []string{"scene-1"} }},
		{"queued draw", domain.CloneErrConflict, func(m *Manifest) { m.Items[0].Status = "queued" }},
		{"running", domain.CloneErrConflict, func(m *Manifest) { m.Items[0].Status = "running" }},
		{"in flight", domain.CloneErrConflict, func(m *Manifest) { m.Items[0].InFlight = &Flight{GenerationID: "draw"} }},
		{"refused skip", domain.CloneErrConflict, func(m *Manifest) { m.Items[0].Status = "skipped"; m.Items[0].SkipReason = "refused" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := unskipFixture()
			tc.mutate(&m)
			_, err := Review(m, "scene-1", "skip", "", nil)
			var ce *domain.CloneError
			if !errors.As(err, &ce) || ce.Code != tc.code || m.Items[0].BeforeSkip != nil {
				t.Fatalf("unsafe skip returned %v or wrote snapshot", err)
			}
		})
	}
}

func TestUnskipSnapshotIsConsumedAndReplacedOnlyByNewSkip(t *testing.T) {
	m, err := Review(unskipFixture(), "scene-1", "skip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Review(m, "scene-1", "skip", "", nil); err == nil {
		t.Fatal("repeated skip replaced original snapshot")
	}
	m, err = Review(m, "scene-1", "unskip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Review(m, "scene-1", "unskip", "", nil); err == nil {
		t.Fatal("repeated undo succeeded")
	}
	m, err = Review(m, "scene-1", "approve", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err = Review(m, "scene-1", "skip", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err = Review(m, "scene-1", "unskip", "", nil)
	if err != nil || m.Items[0].Status != "approved" || m.Items[0].CurrentVersion == nil || m.Items[0].ReviewVersion != nil {
		t.Fatalf("new skip restored wrong earlier action: %+v %v", m.Items[0], err)
	}
}
