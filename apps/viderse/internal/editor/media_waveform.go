package editor

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// Accumulation retains only 50 energy bins per second, not decoded PCM.
func waveformEnvelope(r io.Reader, hash string) (Object, error) {
	reader := bufio.NewReaderSize(r, 64<<10)
	var frame [8]byte
	energies := []float64{}
	var frames, inBin int64
	var squares, peak float64
	flush := func() {
		if inBin > 0 {
			energies = append(energies, .7*math.Sqrt(squares/float64(inBin))+.3*peak)
			squares = 0
			peak = 0
			inBin = 0
		}
	}
	for {
		_, e := io.ReadFull(reader, frame[:])
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("incomplete PCM frame: %w", e)
		}
		if frames >= 86400*48000 {
			return nil, apiError("invalid_request", "Voice source exceeds 24 hours")
		}
		left := float64(math.Float32frombits(binary.LittleEndian.Uint32(frame[:4])))
		right := float64(math.Float32frombits(binary.LittleEndian.Uint32(frame[4:])))
		if math.IsNaN(left) || math.IsInf(left, 0) || math.IsNaN(right) || math.IsInf(right, 0) {
			return nil, apiError("probe_failed", "Non-finite PCM sample")
		}
		squares += (left*left + right*right) / 2
		peak = math.Max(peak, math.Max(math.Abs(left), math.Abs(right)))
		frames++
		inBin++
		if frames >= int64(len(energies)+1)*960 {
			flush()
		}
	}
	flush()
	if frames == 0 {
		return nil, apiError("probe_failed", "Voice source contains no PCM samples")
	}
	normalization := 0.0
	for _, v := range energies {
		normalization = math.Max(normalization, v)
	}
	bins := make([]byte, len(energies))
	if normalization > 0 {
		for i, v := range energies {
			bins[i] = byte(math.Round(v / normalization * 255))
		}
	}
	return Object{"sourceHash": hash, "samplesPerSecond": 50, "durationSeconds": float64(frames) / 48000, "normalization": normalization, "envelopeBase64": base64.StdEncoding.EncodeToString(bins)}, nil
}

// WaveformSource validates the cheap part of a waveform request so the POST can
// reject an unusable source before a decode job is queued.
func (s *Service) WaveformSource(ctx context.Context, id string) (MediaRecord, error) {
	m, e := s.GetMediaMetadata(ctx, id)
	if e != nil {
		return MediaRecord{}, e
	}
	return m, waveformSourceError(m)
}

func waveformSourceError(m MediaRecord) error {
	if len(m.MIME) < 6 || m.MIME[:6] != "audio/" {
		return apiError("invalid_request", "Chọn nguồn giọng là tệp âm thanh")
	}
	if m.Probe.DurationSec != nil && *m.Probe.DurationSec > 86400 {
		return apiError("invalid_request", "Voice source exceeds 24 hours")
	}
	return nil
}

func (s *Service) MediaWaveform(ctx context.Context, id string) (Object, error) {
	m, e := s.GetMediaMetadata(ctx, id)
	if e != nil {
		return nil, e
	}
	if e = waveformSourceError(m); e != nil {
		return nil, e
	}
	h, e := s.AcquireMedia(ctx, id)
	if e != nil {
		return nil, e
	}
	defer h.Release()
	source, e := os.Open(h.Path)
	if e != nil {
		return nil, e
	}
	hash, e := mediaFileHash(source)
	closeErr := source.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if hash != m.ContentHash {
		return nil, apiError("conflict", "Nguồn giọng đã thay đổi; tạo sóng lại")
	}
	f, e := os.CreateTemp(s.StagingDir, "editor-waveform-*.f32le")
	if e != nil {
		return nil, e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Close(); e != nil {
		return nil, e
	}
	runner := s.Runner
	if runner == nil {
		runner = &tools.ExecRunner{}
	}
	ffmpeg := s.FFmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	_, e = runner.Run(ctx, ffmpeg, []string{"-hide_banner", "-nostdin", "-v", "error", "-i", h.Path, "-map", "0:a:0", "-vn", "-sn", "-dn", "-ar", "48000", "-ac", "2", "-f", "f32le", "-c:a", "pcm_f32le", "-y", tmp}, tools.Spec{Timeout: 30 * time.Minute})
	if e != nil {
		return nil, e
	}
	pcm, e := os.Open(tmp)
	if e != nil {
		return nil, e
	}
	envelope, e := waveformEnvelope(pcm, hash)
	closeErr = pcm.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		return nil, closeErr
	}
	current, e := s.GetMediaMetadata(ctx, id)
	if e != nil {
		return nil, e
	}
	if current.ContentHash != hash {
		return nil, apiError("conflict", "Nguồn giọng đã thay đổi; tạo sóng lại")
	}
	return Object{"envelope": envelope}, nil
}
