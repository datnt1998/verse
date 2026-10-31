package editor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func mediaPCM(left, right float32, frames int) []byte {
	b := make([]byte, frames*8)
	for i := range frames {
		binary.LittleEndian.PutUint32(b[i*8:i*8+4], math.Float32bits(left))
		binary.LittleEndian.PutUint32(b[i*8+4:i*8+8], math.Float32bits(right))
	}
	return b
}
func TestMediaWaveformEnergyRetainsInvertedStereoAndPartialBins(t *testing.T) {
	o, e := waveformEnvelope(bytes.NewReader(mediaPCM(.5, -.5, 961)), "sha256:voice")
	if e != nil {
		t.Fatal(e)
	}
	bins, e := base64.StdEncoding.DecodeString(o["envelopeBase64"].(string))
	if e != nil || len(bins) != 2 || bins[0] != 255 || bins[1] != 255 || math.Abs(o["normalization"].(float64)-.5) > 1e-12 || o["durationSeconds"] != float64(961)/48000 {
		t.Fatalf("envelope: %v %v", o, e)
	}
	for _, pcm := range [][]byte{nil, {1}, mediaPCM(float32(math.NaN()), 0, 1)} {
		if _, e = waveformEnvelope(bytes.NewReader(pcm), "sha256:voice"); e == nil {
			t.Fatal("invalid PCM accepted")
		}
	}
}
func TestMediaWaveformRouteRejectsImagesAndChecksCurrentSource(t *testing.T) {
	ctx := context.Background()
	s := newMediaTestService(t)
	image := mediaTestImport(t, s, "image.png")
	_, e := s.MediaWaveform(ctx, image.ID)
	mediaErrorCode(t, e, "invalid_request")
	file, e := os.Open(filepath.Join("testdata", "__fixtures__", "audio.wav"))
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.ImportMedia(ctx, ImportMediaOptions{FileName: "voice.wav", DeclaredMIME: "audio/wav", Body: file})
	closeErr := file.Close()
	if e != nil {
		t.Fatal(e)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	s.Prober = mediaProberFunc(func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Kind: "audio", MIME: "audio/mpeg", DurationSec: mediaNumber(1)}, nil
	})
	s.Runner = mediaRunnerFunc(func(_ context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
		return tools.Result{}, os.WriteFile(args[len(args)-1], mediaPCM(.5, -.5, 960), 0600)
	})
	o, e := s.MediaWaveform(ctx, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	envelope := o["envelope"].(Object)
	if envelope["sourceHash"] != m.ContentHash || envelope["samplesPerSecond"] != 50 {
		t.Fatalf("response: %v", o)
	}
	s.Runner = mediaRunnerFunc(func(_ context.Context, _ string, args []string, _ tools.Spec) (tools.Result, error) {
		replacement, e := os.Open(filepath.Join("testdata", "__fixtures__", "audio.mp3"))
		if e != nil {
			return tools.Result{}, e
		}
		defer replacement.Close()
		if _, e = s.ReplaceMedia(ctx, m.ID, ImportMediaOptions{FileName: "voice.mp3", DeclaredMIME: "audio/mpeg", Body: replacement}); e != nil {
			return tools.Result{}, e
		}
		return tools.Result{}, os.WriteFile(args[len(args)-1], mediaPCM(.5, .5, 960), 0600)
	})
	_, e = s.MediaWaveform(ctx, m.ID)
	mediaErrorCode(t, e, "conflict")
}
