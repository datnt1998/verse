package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
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

type directDrawer struct{ p *provider }

func (d directDrawer) Draw(ctx context.Context, r imagegen.Request) (imagegen.Receipt, error) {
	return d.p.Draw(ctx, r, nil)
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

func TestImageWorkerDrainsRedrawAfterCurrentImage(t *testing.T) {
	s, _ := fixture(t)
	var manifest Manifest
	if err := json.Unmarshal(s.Checkpoint, &manifest); err != nil {
		t.Fatal(err)
	}
	second := manifest.Items[0]
	second.ID = "second"
	second.Status = "review"
	manifest.Items = append(manifest.Items, second)
	manifest.QueueOnly = []string{"thumbnail"}
	if err := s.SaveCheckpoint(context.Background(), manifest); err != nil {
		t.Fatal(err)
	}
	drains := 0
	s.DrainImageIntents = func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		drains++
		if drains != 2 {
			return raw, nil
		}
		var current Manifest
		if err := json.Unmarshal(raw, &current); err != nil {
			return nil, err
		}
		current.Items[1].Status = "pending"
		current.Items[1].Direction = "warmer"
		current.QueueOnly = append(current.QueueOnly, "second")
		current.Revision++
		return json.Marshal(current)
	}
	p := &provider{}
	h := Handler{Deps: Deps{Drawer: directDrawer{p}, Analyse: analysis, Encode: encode}}
	if _, err := h.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if p.calls != 2 {
		t.Fatalf("redraw was not executed: calls=%d", p.calls)
	}
	if err := json.Unmarshal(s.Checkpoint, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Items[0].Versions) != 1 || len(manifest.Items[1].Versions) != 1 || manifest.Items[1].Direction != "warmer" {
		t.Fatalf("lost image results: %+v", manifest.Items)
	}
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

func TestSceneCutoutMatchesNormalizedImageGeometry(t *testing.T) {
	s, media := fixture(t)
	manifest := Manifest{
		Version: 1, Revision: 1, Style: "cutout",
		Story: &Story{Shots: map[string]string{"scene": "Ada in the room"}},
		Items: []Item{{ID: "scene", SceneID: "sc-0", Kind: "scene", Status: "pending", Width: 1920, Height: 1080}},
	}
	writePNG := func(path string, width, height int) error {
		output, err := os.Create(path)
		if err != nil {
			return err
		}
		err = png.Encode(output, image.NewNRGBA(image.Rect(0, 0, width, height)))
		closeErr := output.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	p := &provider{}
	handler := Handler{Deps: Deps{
		Drawer: directDrawer{p},
		Encode: func(_ context.Context, _ *pipeline.StepContext, _, output string, width, height int) (Encoded, error) {
			if err := writePNG(output, width, height); err != nil {
				return Encoded{}, err
			}
			hash, err := steps.SHA256File(output)
			info, statErr := os.Stat(output)
			if statErr != nil {
				return Encoded{}, statErr
			}
			return Encoded{Width: width, Height: height, SHA256: hash, Bytes: info.Size()}, err
		},
		Cutout: func(_ context.Context, _ *pipeline.StepContext, input, output string) error {
			file, err := os.Open(input)
			if err != nil {
				return err
			}
			config, err := png.DecodeConfig(file)
			file.Close()
			if err != nil {
				return err
			}
			return writePNG(output, config.Width, config.Height)
		},
	}}
	if err := handler.runQueue(context.Background(), s, &manifest); err != nil {
		t.Fatal(err)
	}
	version := Displayed(manifest.Items[0])
	if version == nil || version.Cutout == nil {
		t.Fatal("completed scene has no reviewable cutout")
	}
	cutout, err := media.Acquire(context.Background(), version.Cutout.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer cutout.Release()
	file, err := os.Open(cutout.Path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != version.Width || config.Height != version.Height || config.Width != 1920 || config.Height != 1080 {
		t.Fatalf("cutout %dx%d does not match normalized scene %dx%d", config.Width, config.Height, version.Width, version.Height)
	}
	if p.calls != 1 {
		t.Fatalf("cutout processing redrew the image: %d calls", p.calls)
	}
}
