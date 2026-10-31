package images

import (
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/stepstest"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type coverResolver struct{}

func (coverResolver) Resolve(context.Context) *tools.FfmpegResolved {
	return &tools.FfmpegResolved{Ffmpeg: "ffmpeg", Ffprobe: "ffprobe", Probe: &tools.FfmpegProbe{Caps: tools.FfmpegCaps{Encoders: map[string]bool{"libwebp": true}}}}
}

type coverRunner struct {
	args []string
	fail bool
}

func (r *coverRunner) Run(_ context.Context, exe string, args []string, _ tools.Spec) (tools.Result, error) {
	if r.fail {
		return tools.Result{}, os.ErrInvalid
	}
	if exe == "ffprobe" {
		return tools.Result{Stdout: `{"streams":[{"codec_name":"webp","width":1280,"height":720}]}`}, nil
	}
	r.args = append([]string{}, args...)
	return tools.Result{}, os.WriteFile(args[len(args)-1], []byte("RIFF0000WEBPtest"), 0600)
}
func coverPNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.dat")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestCoverUsesRealEncoderContractAndPublishesApprovedUpload(t *testing.T) {
	s, store := testService()
	media := &stepstest.FakeMedia{Root: t.TempDir()}
	s.Edits.Media = media
	s.Ffmpeg = coverResolver{}
	runner := &coverRunner{}
	s.Runner = runner
	rev := 3
	out, err := s.UploadCover(context.Background(), "job", coverPNG(t), RevisionInput{ExpectedRevision: &rev})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(runner.args, " ")
	if !strings.Contains(args, "-c:v libwebp") || !strings.Contains(args, "scale=1280:720") || !strings.Contains(args, "-frames:v 1") {
		t.Fatalf("wrong encode args: %s", args)
	}
	if out.Version != 2 || store.m.Items[1].Status != "approved" || store.m.Items[0].Status != "review" || len(media.Published) != 2 {
		t.Fatalf("wrong upload transaction %+v %+v", out, store.m)
	}
	version := store.m.Items[1].Versions[1]
	if version.Source != "upload" || version.Receipt != nil {
		t.Fatalf("fabricated upload provenance: %+v", version)
	}
	raw, err := json.Marshal(version)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"receipt"`) {
		t.Fatalf("upload serialized an invented receipt: %s", raw)
	}
	if store.change.InvalidateFrom != "" {
		t.Fatal("cover-only upload invalidated rendered frames")
	}
}
func TestCoverEncoderFailureDoesNotCommit(t *testing.T) {
	s, store := testService()
	s.Ffmpeg = coverResolver{}
	s.Runner = &coverRunner{fail: true}
	rev := 3
	_, err := s.UploadCover(context.Background(), "job", coverPNG(t), RevisionInput{ExpectedRevision: &rev})
	requireCode(t, err, domain.CloneErrInvalidRequest)
	if store.commits != 0 {
		t.Fatal("failed encoding committed")
	}
}
