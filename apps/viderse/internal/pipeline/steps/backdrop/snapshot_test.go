package backdrop

import (
	"context"
	"encoding/json"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"testing"
)

func TestBackdropResolvesPinnedLibrarySnapshot(t *testing.T) {
	raw, err := json.Marshal(library.Snapshot{Appearance: domain.BackgroundAppearance{Speed: 2}, Overlays: []library.OverlaySnapshot{{ID: "overlay", FilePath: "library/overlay.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTemplate(context.Background(), raw, newStore())
	if err != nil {
		t.Fatal(err)
	}
	if got.PlaybackSpeed != 2 || got.Backdrop != nil {
		t.Fatalf("pinned snapshot projected incorrectly: %+v", got)
	}
}
