package editor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func mediaErrorCode(t *testing.T, e error, want string) {
	t.Helper()
	var api *Error
	if !errors.As(e, &api) || api.Code != want {
		t.Fatalf("want %s, got %v", want, e)
	}
}
func probeFixture(t *testing.T, name string) ProbeResult {
	t.Helper()
	p, e := ProbeMediaFile(context.Background(), filepath.Join("testdata", "__fixtures__", name), nil, "")
	if e != nil {
		t.Fatal(e)
	}
	return p
}

// All media-probe.test.ts container and still-image cases use pinned real bytes.
func TestMediaProbeVideoContainers(t *testing.T) {
	for _, c := range []struct{ file, video, audio string }{{"mp4-h264.mp4", "h264", "aac"}, {"webm-vp9.webm", "vp9", "opus"}, {"video-renamed.png", "h264", "aac"}} {
		t.Run(c.file, func(t *testing.T) {
			p := probeFixture(t, c.file)
			if p.Kind != "video" || p.VideoCodec == nil || *p.VideoCodec != c.video || p.AudioCodec == nil || *p.AudioCodec != c.audio || p.Width == nil || *p.Width != 160 || p.Height == nil || *p.Height != 90 || p.FPS == nil || *p.FPS < 10 || p.DurationSec == nil || *p.DurationSec <= .5 {
				t.Fatalf("probe: %+v", p)
			}
			if c.file == "mp4-h264.mp4" && (*p.FPS != 24 || p.Channels == nil || *p.Channels != 1) {
				t.Fatalf("MP4: %+v", p)
			}
		})
	}
}
func TestMediaProbeAudioContainers(t *testing.T) {
	for _, c := range []struct {
		file, codec string
		channels    float64
	}{{"audio.mp3", "mp3", 0}, {"audio.wav", "pcm", 2}, {"audio.m4a", "aac", 1}, {"audio.flac", "flac", 2}, {"audio.ogg", "vorbis", 2}} {
		t.Run(c.file, func(t *testing.T) {
			p := probeFixture(t, c.file)
			if p.Kind != "audio" || p.AudioCodec == nil || *p.AudioCodec != c.codec || p.SampleRate == nil || *p.SampleRate <= 0 || p.VideoCodec != nil || p.DurationSec == nil || *p.DurationSec <= 0 || (c.channels > 0 && (p.Channels == nil || *p.Channels != c.channels)) {
				t.Fatalf("probe: %+v", p)
			}
		})
	}
}
func TestMediaProbeImagesAndFailures(t *testing.T) {
	for _, c := range []struct {
		file string
		w, h float64
	}{{"image.png", 64, 48}, {"image.jpg", 80, 60}, {"image.gif", 64, 48}, {"image-renamed.mp4", 64, 48}} {
		p := probeFixture(t, c.file)
		if p.Kind != "image" || p.Width == nil || *p.Width != c.w || p.Height == nil || *p.Height != c.h || p.DurationSec != nil || !strings.HasPrefix(p.MIME, "image/") {
			t.Fatalf("%s: %+v", c.file, p)
		}
	}
	for _, c := range []struct{ file, code string }{{"truncated.mp4", "probe_failed"}, {"fake.mp4", "unsupported_format"}, {"missing.mp4", "probe_failed"}} {
		_, e := ProbeMediaFile(context.Background(), filepath.Join("testdata", "__fixtures__", c.file), nil, "")
		mediaErrorCode(t, e, c.code)
	}
}
func mediaTestZip(t *testing.T, docs map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, body := range docs {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(body); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestMediaProbeLottieContentAndManifestOrder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "animation.mp4")
	for _, prefix := range []string{"", "\ufeff  ", strings.Repeat(" ", 256<<10+1)} {
		b := []byte(prefix + `{"fr":30,"ip":15,"op":105,"w":640,"h":360,"layers":[]}`)
		if e := os.WriteFile(file, b, 0600); e != nil {
			t.Fatal(e)
		}
		p, e := ProbeMediaFile(context.Background(), file, nil, "")
		if e != nil || p.Kind != "image" || p.MIME != "application/lottie+json" || p.DurationSec == nil || *p.DurationSec != 3 || p.FPS == nil || *p.FPS != 30 {
			t.Fatalf("lottie: %+v %v", p, e)
		}
	}
	b := mediaTestZip(t, map[string][]byte{"animations/second.json": []byte(`{"fr":60,"ip":0,"op":120,"w":20,"h":30,"layers":[]}`), "manifest.json": []byte(`{"animations":[{"id":"first"},{"id":"second"}]}`), "animations/first.json": []byte(`{"fr":25,"ip":10,"op":85,"w":320,"h":180,"layers":[]}`)})
	if e := os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	p, e := ProbeMediaFile(context.Background(), file, nil, "")
	if e != nil || p.MIME != "application/zip+dotlottie" || p.DurationSec == nil || *p.DurationSec != 3 || p.Width == nil || *p.Width != 320 {
		t.Fatalf("dotlottie: %+v %v", p, e)
	}
}
func TestMediaProbeLottieRejectsMalformedAndMissingReferences(t *testing.T) {
	file := filepath.Join(t.TempDir(), "animation.lottie")
	cases := []struct {
		body []byte
		code string
	}{{[]byte(`{ malformed`), "probe_failed"}, {[]byte(`{"ordinary":"JSON"}`), "unsupported_format"}, {[]byte(`{"fr":0,"ip":0,"op":10,"w":10,"h":10,"layers":[]}`), "unsupported_format"}, {[]byte(`{"fr":30,"ip":0,"op":10,"w":10,"h":10,"layers":{}}`), "unsupported_format"}, {mediaTestZip(t, map[string][]byte{"unrelated.txt": []byte("Not dotLottie")}), "unsupported_format"}, {mediaTestZip(t, map[string][]byte{"manifest.json": []byte(`{"animations":[{"id":"missing"}]}`)}), "probe_failed"}, {[]byte{'P', 'K', 3, 4, 0, 0, 0}, "probe_failed"}}
	for _, c := range cases {
		if e := os.WriteFile(file, c.body, 0600); e != nil {
			t.Fatal(e)
		}
		_, e := ProbeMediaFile(context.Background(), file, nil, "")
		mediaErrorCode(t, e, c.code)
	}
}
func TestMediaProbeLottieBoundsDeclaredAndActualExpansion(t *testing.T) {
	file := filepath.Join(t.TempDir(), "oversized.bin")
	if e := os.WriteFile(file, []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Truncate(file, 20<<20+1); e != nil {
		t.Fatal(e)
	}
	_, e := ProbeMediaFile(context.Background(), file, nil, "")
	mediaErrorCode(t, e, "unsupported_format")
	b := mediaTestZip(t, map[string][]byte{"manifest.json": []byte("{}")})
	binary.LittleEndian.PutUint32(b[22:26], 50<<20+1)
	if e = os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	_, e = ProbeMediaFile(context.Background(), file, nil, "")
	mediaErrorCode(t, e, "unsupported_format")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, e := writer.Create("assets/highly-compressed.bin")
	if e != nil {
		t.Fatal(e)
	}
	zeros := make([]byte, 1<<20)
	for range 51 {
		if _, e = entry.Write(zeros); e != nil {
			t.Fatal(e)
		}
	}
	if e = writer.Close(); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, archive.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	_, e = ProbeMediaFile(context.Background(), file, nil, "")
	mediaErrorCode(t, e, "unsupported_format")
}

type mediaRunnerFunc func(context.Context, string, []string, tools.Spec) (tools.Result, error)

func (f mediaRunnerFunc) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return f(ctx, file, args, spec)
}
func TestMediaProbeRejectsNoTracksAndInvalidToolJSON(t *testing.T) {
	for _, raw := range []string{`{"streams":[]}`, `not JSON`} {
		r := mediaRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
			return tools.Result{Stdout: raw}, nil
		})
		_, e := ProbeMediaFile(context.Background(), filepath.Join("testdata", "__fixtures__", "mp4-h264.mp4"), r, "ffprobe")
		mediaErrorCode(t, e, "probe_failed")
	}
	p := probeFixture(t, "image.png")
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(b, []byte(`"audioCodec":null`)) {
		t.Fatalf("nullable probe shape: %s", b)
	}
}
