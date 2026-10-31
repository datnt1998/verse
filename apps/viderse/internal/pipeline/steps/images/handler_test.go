package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
)

type provider struct{ calls int }

func (p *provider) Name() string                { return "codex" }
func (p *provider) Model() string               { return imagegen.CodexImageModel }
func (p *provider) Paid() bool                  { return true }
func (p *provider) Ready(context.Context) error { return nil }
func (p *provider) Draw(_ context.Context, r imagegen.Request, _ imagegen.JournalHandle) (imagegen.Receipt, error) {
	p.calls++
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a5XcAAAAASUVORK5CYII=")
	if err := os.WriteFile(r.Output, data, 0600); err != nil {
		return imagegen.Receipt{}, err
	}
	hash, _ := steps.SHA256File(r.Output)
	return imagegen.Receipt{Provider: "codex", Model: imagegen.CodexImageModel, PngSha256: hash, CallID: fmt.Sprint(p.calls)}, nil
}
func (p *provider) Reconcile(context.Context, json.RawMessage, imagegen.Request) (imagegen.Receipt, bool, error) {
	return imagegen.Receipt{}, false, nil
}
func fixture(t *testing.T) (*pipeline.StepContext, *stepstest.FakeMedia) {
	t.Helper()
	s, m := stepstest.Ctx(t, domain.CloneImages, stepstest.Job("job-1"))
	m.Put("reference.jpg", []byte("reference"))
	hash, _ := steps.SHA256File(m.Root + "/reference.jpg")
	doc := scenes.Manifest{Version: 1, Revision: 1, FPS: 30, TotalFrames: 30, DurationSec: 1, Recipe: scenes.Recipe{ImagesMode: "none", Thumbnail: "clean"}, Scenes: []scenes.Scene{{SceneID: "sc-0", StartFrame: 0, EndFrame: 30, KeyframeFrame: 15}}}
	b, _ := json.Marshal(doc)
	m.Put("scenes/manifest.json", b)
	s.Upstream[domain.CloneDownload] = domain.CloneStepOutput{Data: map[string]any{"thumbnail": scenes.FileRef{Path: "reference.jpg", SHA256: hash}}}
	// Receipt recovery tests exercise the legacy cover-only generation queue,
	// not a no-image recipe (whose cover is supplied by template/upload).
	manifest := CreateManifest(doc, scenes.FileRef{Path: "reference.jpg", SHA256: hash}, "")
	manifest.ImageMode = ""
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	return s, m
}
func analysis(_ context.Context, _ string, _ []string, ids []string, fingerprint, look, thumbnail string) (Story, error) {
	out := Story{Fingerprint: fingerprint, Shots: map[string]string{}}
	for _, id := range ids {
		out.Shots[id] = "Ada in shot " + id
	}
	return out, nil
}
func encode(_ context.Context, _ *pipeline.StepContext, _ string, out string, w, h int) (Encoded, error) {
	if err := os.WriteFile(out, []byte("webp bytes"), 0644); err != nil {
		return Encoded{}, err
	}
	hash, err := steps.SHA256File(out)
	return Encoded{Width: w, Height: h, SHA256: hash, Bytes: 10}, err
}
func TestCancellationNeverInvokesGeneration(t *testing.T) {
	s, _ := fixture(t)
	p := &provider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := Handler{Deps: Deps{Drawer: imagegen.NewService(imagegen.Options{Providers: []imagegen.Provider{p}}), Analyse: analysis, Encode: encode}}
	_, err := h.Run(ctx, s)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if p.calls != 0 {
		t.Fatal(p.calls)
	}
}

type failingDrawer struct{ calls atomic.Int32 }

func (d *failingDrawer) Draw(context.Context, imagegen.Request) (imagegen.Receipt, error) {
	d.calls.Add(1)
	return imagegen.Receipt{}, errors.New("provider unavailable")
}

func TestImageFailureStopsWaiterBeforeSlotRelease(t *testing.T) {
	s, _ := fixture(t)
	m := Manifest{Version: 1, Revision: 1, Story: &Story{Shots: map[string]string{"one": "first shot", "two": "second shot"}}, Items: []Item{{ID: "one", Kind: "scene", Status: "pending", Width: 1920, Height: 1080}, {ID: "two", Kind: "scene", Status: "pending", Width: 1920, Height: 1080}}}
	var slot sync.Mutex
	var entered atomic.Int32
	secondFinished := make(chan struct{})
	s.ImageSlot = func(ctx context.Context, run func(context.Context) error) error {
		slot.Lock()
		n := entered.Add(1)
		err := run(ctx)
		slot.Unlock()
		// Force the waiting callback to run before the first slot wrapper returns.
		// An error recorded outside the callback is necessarily too late.
		if n == 1 {
			<-secondFinished
		} else {
			close(secondFinished)
		}
		return err
	}
	d := &failingDrawer{}
	err := (Handler{Deps: Deps{Drawer: d}}).runQueue(context.Background(), s, &m)
	if err == nil || err.Error() != "provider unavailable" {
		t.Fatalf("lost first failure: %v", err)
	}
	if got := d.calls.Load(); got != 1 {
		t.Fatalf("waiting image started after failure: paid calls = %d", got)
	}
}
