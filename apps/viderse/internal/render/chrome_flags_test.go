package render

import (
	"context"
	"errors"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func TestSupervisorChromeNeverUsesUserKeychain(t *testing.T) {
	for _, kind := range []string{"render", "measure"} {
		t.Run(kind, func(t *testing.T) {
			s, runner, _ := fakeSupervisor(t, "exit")
			observed := make(chan []string, 1)
			runner.run = func(_ context.Context, _ int, args []string, _ tools.Spec) error {
				observed <- args
				return errors.New("fixture early exit")
			}
			s.defaults()
			if kind == "render" {
				r, err := s.Store.CreateExport("project", "job")
				if err != nil {
					t.Fatal(err)
				}
				_, _, _ = s.runExportAttempt(context.Background(), r, false, nil)
			} else {
				r, err := s.Store.CreateMeasurement("job", "project", quality.Binding{ProjectRevision: "revision", SettingsHash: "settings", ArtifactHash: "artifacts"})
				if err != nil {
					t.Fatal(err)
				}
				_, _, _ = s.runMeasurementAttempt(context.Background(), r)
			}
			args := <-observed
			for _, required := range []string{"--use-mock-keychain", "--password-store=basic"} {
				found := false
				for _, arg := range args {
					if arg == required {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s browser can prompt for user's Keychain: missing %s", kind, required)
				}
			}
		})
	}
}
