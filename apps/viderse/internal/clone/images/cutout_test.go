package images

import (
 "context"
 "image"
 "image/png"
 "os"
 "runtime"
 "testing"

 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
 "github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type cutoutRunner struct { calls int }
func (r *cutoutRunner) Run(_ context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
 r.calls++
 file, err := os.Create(args[len(args)-1]); if err != nil { return tools.Result{}, err }
 err = png.Encode(file, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
 if closeErr := file.Close(); err == nil { err = closeErr }
 return tools.Result{}, err
}

func TestCutoutPublishesAndReusesVersion(t *testing.T) {
 if runtime.GOOS != "darwin" { t.Skip("macOS Vision") }
 s, store := testService()
 media := &stepstest.FakeMedia{Root:t.TempDir()}
 s.Edits.Media = media
 runner := &cutoutRunner{}
 s.Runner = runner
 store.m.Items[0].Versions[0].Path = "images/a.webp"
 store.m.Items[0].Versions[0].Width = 2
 store.m.Items[0].Versions[0].Height = 2
 media.Put("clone/job/hash/images/a.webp", []byte("source"))
 store.artifacts = []domain.CloneArtifact{{ID:"image",Path:"clone/job/hash/images/a.webp"}}
 path, err := s.Cutout(context.Background(), "job", "scene-1", 1)
 if err != nil { t.Fatal(err) }
 if store.m.Items[0].Versions[0].Cutout == nil || store.m.Items[0].Versions[0].Cutout.Path != path || store.commits != 1 { t.Fatalf("cutout not committed: %+v", store.m.Items[0]) }
 if store.m.Revision != 3 { t.Fatalf("preview changed review revision: %d", store.m.Revision) }
 again, err := s.Cutout(context.Background(), "job", "scene-1", 1)
 if err != nil || again != path || runner.calls != 1 { t.Fatalf("cache miss: %q %v calls=%d", again, err, runner.calls) }
}
