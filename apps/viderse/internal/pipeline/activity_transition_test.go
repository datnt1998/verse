package pipeline

import (
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"testing"
)

func TestActivityTransitionOnlyNotifiesHumanStates(t *testing.T) {
	review := domain.CloneStepRecord{Step: domain.CloneImages, Status: domain.CloneWaiting, Error: &domain.CloneError{Code: domain.CloneErrAwaitingReview}}
	if got := activityTransition(domain.CloneWaiting, &review); got != "review_ready" {
		t.Fatal(got)
	}
	progress := domain.CloneStepRecord{Step: domain.CloneMetadata, Status: domain.CloneSucceeded}
	if got := activityTransition(domain.CloneRunning, &progress); got != "" {
		t.Fatalf("progress notified %s", got)
	}
	if got := activityTransition(domain.CloneSucceeded, nil); got != "ready_to_post" {
		t.Fatal(got)
	}
}
