package images

import (
	"context"
	"image"
	"image/png"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type cutoutRunner struct {
	mu    sync.Mutex
	calls int
}

func (r *cutoutRunner) Run(_ context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	file, err := os.Create(args[len(args)-1])
	if err != nil {
		return tools.Result{}, err
	}
	err = png.Encode(file, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return tools.Result{}, err
}

type cutoutTestStore struct {
	*editStore
	mu    sync.Mutex
	paths map[string]string
}

func (s *cutoutTestStore) FindCutout(_ context.Context, hash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paths[hash], nil
}
func (s *cutoutTestStore) OwnCutout(_ context.Context, _, hash, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paths[hash] = path
	return nil
}
func setupCutout(t *testing.T) (*Service, *editStore, *cutoutRunner) {
	t.Helper()
	s, store := testService()
	s.Edits.Store = &cutoutTestStore{editStore: store, paths: map[string]string{}}
	files := &stepstest.FakeMedia{Root: t.TempDir()}
	s.Edits.Media = files
	runner := &cutoutRunner{}
	s.Runner = runner
	v := &store.m.Items[0].Versions[0]
	v.Path = "images/a.webp"
	v.Width = 2
	v.Height = 2
	files.Put("clone/job/hash/images/a.webp", []byte("source"))
	store.artifacts = []domain.CloneArtifact{{ID: "image", Path: "clone/job/hash/images/a.webp"}}
	return s, store, runner
}
func TestCutoutDuringImageWorker(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Vision")
	}
	s, store, _ := setupCutout(t)
	store.m.Items[1].Status = "running"
	_, err := s.Cutout(context.Background(), "job", "scene-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if store.commits != 0 {
		t.Fatal("cutout changed checkpoint")
	}
}

func TestCutoutConcurrentRequestsGenerateOnce(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Vision")
	}
	s, store, runner := setupCutout(t)
	const count = 12
	var wg sync.WaitGroup
	errs := make(chan error, count)
	paths := make(chan string, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := s.Cutout(context.Background(), "job", "scene-1", 1)
			errs <- err
			paths <- path
		}()
	}
	wg.Wait()
	close(errs)
	close(paths)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first string
	for path := range paths {
		if first == "" {
			first = path
		}
		if path != first {
			t.Fatalf("different cutout paths %q and %q", first, path)
		}
	}
	if runner.calls != 1 || store.commits != 0 {
		t.Fatalf("calls=%d checkpoint commits=%d", runner.calls, store.commits)
	}
}
