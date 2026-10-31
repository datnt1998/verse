package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func mediaSolidFrame(r, g, b byte) []byte {
	f := make([]byte, BackgroundSampleWidth*BackgroundSampleHeight*3)
	for i := range len(f) / 3 {
		f[i*3] = r
		f[i*3+1] = g
		f[i*3+2] = b
	}
	return f
}
func TestMediaBackgroundClassifyBlackGreenAndBusy(t *testing.T) {
	for _, c := range []struct {
		frame       []byte
		kind, color string
	}{{mediaSolidFrame(0, 0, 0), "black", ""}, {mediaSolidFrame(0, 255, 0), "green", "#00ff00"}, {mediaSolidFrame(255, 0, 255), "none", ""}, {nil, "none", ""}} {
		got := ClassifyBackground(c.frame, 64, 36)
		if got.Kind != c.kind || got.KeyColor != c.color {
			t.Fatalf("classification: %+v", got)
		}
	}
}
func TestMediaBackgroundSamplesCachesAndRejectsStaleHash(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	calls := 0
	s.Runner = mediaRunnerFunc(func(_ context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
		calls++
		if !strings.Contains(strings.Join(args, " "), "scale=64:36") {
			t.Fatalf("sampling args: %v", args)
		}
		return tools.Result{Stdout: string(mediaSolidFrame(0, 255, 0))}, nil
	})
	overlay := true
	got, e := s.DetectMediaBackground(ctx, m.ID, false, &overlay)
	if e != nil || got.Background == nil || got.Background.Kind != "green" || got.Background.ContentHash != m.ContentHash || !got.Overlay {
		t.Fatalf("detection: %+v %v", got, e)
	}
	got, e = s.DetectMediaBackground(ctx, m.ID, false, nil)
	if e != nil || calls != 1 || !got.Overlay {
		t.Fatalf("cache: %+v %v calls=%d", got, e, calls)
	}
	_, e = s.SaveMediaBackground(ctx, m.ID, MediaBackgroundDetection{Kind: "black"}, "sha256:stale", nil)
	mediaErrorCode(t, e, "conflict")
	s.Runner = mediaRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) { return tools.Result{}, nil })
	got, e = s.DetectMediaBackground(ctx, m.ID, true, nil)
	if e != nil || got.Background == nil || got.Background.Kind != "none" {
		t.Fatalf("empty samples %+v %v", got, e)
	}
	s.Runner = mediaRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
		return tools.Result{}, errors.New("ffmpeg failed")
	})
	if _, e = s.DetectMediaBackground(ctx, m.ID, true, nil); e == nil {
		t.Fatal("ffmpeg failure ignored")
	}
}
func TestMediaBackgroundRetriesOnceAfterSourceReplacement(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	m := mediaTestImport(t, s, "image.png")
	b, e := os.ReadFile(filepath.Join("testdata", "__fixtures__", "image.jpg"))
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	s.Runner = mediaRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
		calls++
		if calls == 1 {
			if _, e := s.ReplaceMedia(ctx, m.ID, ImportMediaOptions{DeclaredMIME: "image/jpeg", Body: bytes.NewReader(b)}); e != nil {
				return tools.Result{}, e
			}
		}
		return tools.Result{Stdout: string(mediaSolidFrame(0, 0, 0))}, nil
	})
	got, e := s.DetectMediaBackground(ctx, m.ID, true, nil)
	if e != nil || calls != 2 || got.Background == nil || got.Background.ContentHash == m.ContentHash {
		t.Fatalf("retry %+v %v calls=%d", got, e, calls)
	}
}
func TestMediaOverlayDedupAndRemovalPreservesProjectFiles(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	mediaTestProject(t, s, "holder")
	s.Runner = mediaRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
		return tools.Result{Stdout: string(mediaSolidFrame(0, 255, 0))}, nil
	})
	file := filepath.Join("testdata", "__fixtures__", "image.png")
	first, e := s.AddFileToOverlayLibrary(ctx, file)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.AddFileToOverlayLibrary(ctx, file)
	if e != nil || again.ID != first.ID {
		t.Fatalf("duplicate: %+v %v", again, e)
	}
	copy := mediaTestImport(t, s, "image.png")
	if _, e = s.UpdateMediaMetadata(ctx, copy.ID, Object{"overlay": true}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.LinkMedia(ctx, "holder", copy.ID); e != nil {
		t.Fatal(e)
	}
	records, e := s.ListOverlays(ctx)
	if e != nil || len(records) != 1 {
		t.Fatalf("library %v %v", records, e)
	}
	records, e = s.RemoveFromOverlayLibrary(ctx, first.ID)
	if e != nil || len(records) != 0 {
		t.Fatalf("remove %v %v", records, e)
	}
	for _, id := range []string{first.ID, copy.ID} {
		h, e := s.AcquireMedia(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		h.Release()
	}
	project, e := s.ListMedia(ctx, "holder")
	if e != nil || len(project) != 1 {
		t.Fatalf("project %v %v", project, e)
	}
	if _, e = s.AddFileToOverlayLibrary(ctx, "bad.exe"); e == nil {
		t.Fatal("unsupported extension accepted")
	}
}
func TestMediaOverlayBuiltinsSeedOnceAndStayRemoved(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	bytes, e := os.ReadFile(filepath.Join("testdata", "__fixtures__", "mp4-h264.mp4"))
	if e != nil {
		t.Fatal(e)
	}
	hash, e := mediaFileHash(strings.NewReader(string(bytes)))
	if e != nil {
		t.Fatal(e)
	}
	manifest, e := json.Marshal(Object{"effects": []any{Object{"id": "rain", "name": "Mưa.mp4", "file": "overlays/rain.mp4", "sha256": strings.TrimPrefix(hash, "sha256:")}}})
	if e != nil {
		t.Fatal(e)
	}
	assets := fstest.MapFS{"overlays/manifest.json": &fstest.MapFile{Data: manifest}, "overlays/rain.mp4": &fstest.MapFile{Data: bytes}}
	s.OverlayAssets = assets
	s.FFprobe = "ffprobe"
	records, e := s.ListOverlays(ctx)
	if e != nil || len(records) != 1 || records[0].Background == nil || records[0].Background.Kind != "black" {
		t.Fatalf("seed %v %v", records, e)
	}
	again, e := s.ListOverlays(ctx)
	if e != nil || !reflect.DeepEqual(records, again) {
		t.Fatalf("seed repeated %v %v", again, e)
	}
	if _, e = s.RemoveFromOverlayLibrary(ctx, records[0].ID); e != nil {
		t.Fatal(e)
	}
	records, e = s.ListOverlays(ctx)
	if e != nil || len(records) != 0 {
		t.Fatalf("removed builtin returned: %v %v", records, e)
	}
}
