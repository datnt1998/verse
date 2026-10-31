package transcript

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

// Checkpoint binds every cached operation to the actual source bytes and
// immutable upstream evidence. Reports live in PostgreSQL as well as staging:
// a crash after report creation cannot force the expensive Python work again.
type checkpoint struct {
	Version     int                        `json:"version"`
	Input       string                     `json:"input"`
	AudioSHA256 string                     `json:"audioSha256,omitempty"`
	Reports     map[string]json.RawMessage `json:"reports"`
}

func loadCheckpoint(s *pipeline.StepContext, sourceSHA string, stored []StoredSegment) (*checkpoint, error) {
	evidence, err := json.Marshal(struct {
		Source   string
		Upstream any
		Stored   []StoredSegment
	}{sourceSHA, s.Upstream, stored})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(evidence)
	input := hex.EncodeToString(sum[:])
	cp := &checkpoint{}
	if len(s.Checkpoint) > 0 {
		if err := json.Unmarshal(s.Checkpoint, cp); err != nil {
			return nil, fmt.Errorf("transcript checkpoint: %w", err)
		}
	}
	if cp.Version != 1 || cp.Input != input {
		cp = &checkpoint{Version: 1, Input: input, Reports: map[string]json.RawMessage{}}
	}
	if cp.Reports == nil {
		cp.Reports = map[string]json.RawMessage{}
	}
	return cp, nil
}

func (cp *checkpoint) transcribe(s *pipeline.StepContext, run func(context.Context, TranscribeRun) (any, error)) func(context.Context, TranscribeRun) (any, error) {
	return func(ctx context.Context, o TranscribeRun) (any, error) {
		keyBytes, err := json.Marshal(struct {
			Mode            string
			Regions         []Interval
			Window, Context float64
			Language, Model string
			Beam            int64
		}{o.Mode, o.Regions, o.WindowSec, o.ContextSec, o.Language, o.ModelSize, o.BeamSize})
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(keyBytes)
		key := hex.EncodeToString(sum[:])
		if raw, ok := cp.Reports[key]; ok {
			var result any
			if err := json.Unmarshal(raw, &result); err != nil {
				return nil, err
			}
			if err := os.WriteFile(o.ReportPath, raw, 0600); err != nil {
				return nil, err
			}
			return result, nil
		}
		result, err := run(ctx, o)
		if err != nil {
			return nil, err
		}
		if o.Mode == "vad" {
			_, err = parseVadReport(result)
		} else {
			_, err = parseAsrReport(result)
		}
		if err != nil {
			return nil, fmt.Errorf("%s adapter report failed schema: %w", o.Mode, err)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(o.ReportPath, raw, 0600); err != nil {
			return nil, err
		}
		cp.Reports[key] = raw
		if err := s.SaveCheckpoint(ctx, cp); err != nil {
			return nil, err
		}
		if s.PauseRequested() {
			return nil, pipeline.ErrPaused
		}
		return result, nil
	}
}
