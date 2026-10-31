package transcript

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

func TestTranscriptResumesCompletedPhasesAfterCrash(t *testing.T) {
	files := setupArtifacts(t)
	s, _ := fakeStepCtx(t, upstreamWith(map[string]any{"kind": "auto", "language": "vi", "path": "download/source.vi.json3"}))
	var saved json.RawMessage
	s.WriteCheckpoint = func(_ context.Context, b json.RawMessage) error { saved = bytes.Clone(b); return nil }
	decodes, vads, asrs := 0, 0, 0
	crash := errors.New("simulated process crash")
	d := fakeDeps(t, files, func(d *Deps) {
		oldDecode := d.Decode
		d.Decode = func(ctx context.Context, ff, in, out string) error { decodes++; return oldDecode(ctx, ff, in, out) }
		d.RunTranscribe = func(_ context.Context, o TranscribeRun) (any, error) {
			if o.Mode == "vad" {
				vads++
				return vadReport(), nil
			}
			asrs++
			if asrs == 1 {
				return nil, crash
			}
			return asrReport(), nil
		}
	})
	if _, err := New(d).Run(context.Background(), s); !errors.Is(err, crash) {
		t.Fatalf("first run: %v", err)
	}
	if len(saved) == 0 {
		t.Fatal("missing durable checkpoint")
	}
	s.Checkpoint = saved
	if _, err := New(d).Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if decodes != 1 || vads != 1 {
		t.Fatalf("repeated completed work decode=%d vad=%d", decodes, vads)
	}
}

func TestTranscriptPauseSavesCheckpointBeforeParking(t *testing.T) {
	files := setupArtifacts(t)
	s, _ := fakeStepCtx(t, upstreamWith(map[string]any{"kind": "auto", "language": "vi"}))
	saved := false
	s.WriteCheckpoint = func(context.Context, json.RawMessage) error { saved = true; return nil }
	s.IsPaused = func() bool { return true }
	_, err := New(fakeDeps(t, files, nil)).Run(context.Background(), s)
	if !errors.Is(err, pipeline.ErrPaused) || !saved {
		t.Fatalf("err=%v saved=%v", err, saved)
	}
}

func TestSidecarEmbeddedMatchesScript(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "scripts", "clone-transcribe.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, cloneTranscribeScript) {
		t.Fatal("embedded Python sidecar differs from scripts/clone-transcribe.py")
	}
}
