package archtest

import (
	"reflect"
	"testing"
)

func TestBlobTransportCutover(t *testing.T) {
	for _, issue := range scanModule(t, "..").blobTransportIssues() {
		t.Error(issue)
	}
}

func TestBlobTransportDetectsPrivateCopy(t *testing.T) {
	s := scan{packages: map[string]bool{"platform/blobstore": true, "platform/blobhttp": true, "apps/noverse/internal/blobstore": true}, edges: map[string]map[string]bool{"apps/noverse/internal/audio": {"apps/noverse/internal/blobstore": true}}}
	want := []string{"obsolete blob transport import: apps/noverse/internal/audio → apps/noverse/internal/blobstore", "obsolete blob transport package: apps/noverse/internal/blobstore"}
	if got := s.blobTransportIssues(); !reflect.DeepEqual(got, want) {
		t.Fatalf("cutover issues=%v want %v", got, want)
	}
}
