package host

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/store"
)

func TestSimulationProfileViewHidesAbsoluteSourceDir(t *testing.T) {
	storyDir := t.TempDir()
	t.Chdir(storyDir)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewStore(filepath.Join(storyDir, "output", "novel"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ sourceDir, want string }{
		{filepath.ToSlash(filepath.Join(wd, "van-mau", "tap-1")), "van-mau/tap-1"},
		{filepath.ToSlash(filepath.Join(t.TempDir(), "ngoai-thu-vien")), "ngoai-thu-vien"},
		{"van-mau", "van-mau"},
	} {
		if err := st.Simulation.Save(domain.SimulationProfile{Corpus: domain.SimulationCorpusManifest{SourceDir: c.sourceDir}}); err != nil {
			t.Fatal(err)
		}
		view, err := (&Host{store: st}).SimulationProfileView()
		if err != nil {
			t.Fatal(err)
		}
		if view.SourceDir != c.want {
			t.Errorf("sourceDir %q displayed as %q, want %q", c.sourceDir, view.SourceDir, c.want)
		}
	}
}
