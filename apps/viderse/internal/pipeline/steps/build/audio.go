package build

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"time"
)

func (h Handler) extractAudio(ctx context.Context, s *pipeline.StepContext, input, output string) error {
	if s.ProcessRunner == nil || h.Deps.Ffmpeg == "" {
		return steps.CloneError(domain.CloneErrToolMissing, "Thiếu FFmpeg", "Kiểm tra FFmpeg trong Cài đặt")
	}
	// AAC is copied, all other codecs are normalized to AAC without borrowing
	// the original video as an editor playback dependency.
	codec := "aac"
	if out, ok := s.Upstream[domain.CloneDownload]; ok {
		if c, ok := out.Data["audioCodec"].(string); ok && c == "aac" {
			codec = "copy"
		}
	}
	_, err := s.RunProcess(ctx, h.Deps.Ffmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", input, "-vn", "-c:a", codec, output}, tools.Spec{Timeout: 30 * time.Minute})
	return err
}

var integratedLUFS = regexp.MustCompile(`(?m)^\s*I:\s*(-?\d+(?:\.\d+)?)\s*LUFS\s*$`)

func ParseIntegratedLUFS(output string) (float64, error) {
	m := integratedLUFS.FindAllStringSubmatch(output, -1)
	if len(m) == 0 {
		return 0, fmt.Errorf("Không đọc được integrated loudness của file")
	}
	v, err := strconv.ParseFloat(m[len(m)-1][1], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("invalid integrated loudness")
	}
	return v, nil
}
func (h Handler) measureLUFS(ctx context.Context, s *pipeline.StepContext, file string) (float64, error) {
	if s.ProcessRunner == nil || h.Deps.Ffmpeg == "" {
		return 0, steps.CloneError(domain.CloneErrToolMissing, "Thiếu FFmpeg", "Kiểm tra FFmpeg trong Cài đặt")
	}
	facts, err := h.probe(ctx, s, file)
	if err != nil {
		return 0, err
	}
	filter, err := SourceAudioReferenceFilter(facts.Channels)
	if err != nil {
		return 0, err
	}
	r, err := s.RunProcess(ctx, h.Deps.Ffmpeg, []string{"-hide_banner", "-nostdin", "-i", file, "-af", filter, "-f", "null", "-"}, tools.Spec{Timeout: 30 * time.Minute, LogLimit: 16384})
	if err != nil {
		return 0, err
	}
	return ParseIntegratedLUFS(r.Stderr)
}
func (h Handler) envelope(ctx context.Context, s *pipeline.StepContext, file, hash string) (editor.Object, error) {
	if s.ProcessRunner == nil || h.Deps.Ffmpeg == "" {
		return nil, steps.CloneError(domain.CloneErrToolMissing, "Thiếu FFmpeg", "Kiểm tra FFmpeg trong Cài đặt")
	}
	// Binary PCM goes to a private staged file, never the text log callback.
	// Decode incrementally; retain only 50 energy bins/second, then remove PCM.
	pcm := file + ".envelope.f32le"
	defer os.Remove(pcm)
	_, err := s.RunProcess(ctx, h.Deps.Ffmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", file, "-vn", "-ac", "2", "-ar", "48000", "-f", "f32le", pcm}, tools.Spec{Timeout: 30 * time.Minute, LogLimit: 1024})
	if err != nil {
		return nil, err
	}
	input, err := os.Open(pcm)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	return DecodeEnvelopePCM(input, hash)
}

type envelopeAccumulator struct {
	energies            []float64
	sum, peak           float64
	framesInBin, frames int
}

func (a *envelopeAccumulator) flush() {
	if a.framesInBin == 0 {
		return
	}
	a.energies = append(a.energies, .7*math.Sqrt(a.sum/float64(a.framesInBin))+.3*a.peak)
	a.sum = 0
	a.peak = 0
	a.framesInBin = 0
}
func (a *envelopeAccumulator) push(data []byte) error {
	for n := 0; n < len(data); n += 8 {
		if a.frames >= 24*3600*48000 {
			return fmt.Errorf("voice source exceeds 24 hours")
		}
		left := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[n:])))
		right := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[n+4:])))
		if math.IsNaN(left) || math.IsInf(left, 0) || math.IsNaN(right) || math.IsInf(right, 0) {
			return fmt.Errorf("nonfinite PCM")
		}
		a.sum += (left*left + right*right) / 2
		a.peak = math.Max(a.peak, math.Max(math.Abs(left), math.Abs(right)))
		a.frames++
		a.framesInBin++
		if a.frames >= int(math.Ceil(float64(len(a.energies)+1)*48000/50)) {
			a.flush()
		}
	}
	return nil
}
func (a *envelopeAccumulator) finish(hash string) (editor.Object, error) {
	a.flush()
	if a.frames == 0 {
		return nil, fmt.Errorf("voice source contains no PCM samples")
	}
	normalization := 0.
	for _, e := range a.energies {
		normalization = math.Max(normalization, e)
	}
	bins := make([]byte, len(a.energies))
	if normalization > 0 {
		for i, e := range a.energies {
			bins[i] = byte(math.Round(e / normalization * 255))
		}
	}
	return editor.Object{"sourceHash": hash, "samplesPerSecond": 50, "durationSeconds": float64(a.frames) / 48000, "normalization": normalization, "envelopeBase64": base64.StdEncoding.EncodeToString(bins)}, nil
}

// DecodeEnvelopePCM exposes the pure streaming accumulator for parity tests.
func DecodeEnvelopePCM(reader io.Reader, hash string) (editor.Object, error) {
	r := bufio.NewReader(reader)
	a := envelopeAccumulator{}
	frame := make([]byte, 8)
	for {
		_, err := io.ReadFull(r, frame)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if err = a.push(frame); err != nil {
			return nil, err
		}
	}
	return a.finish(hash)
}

func SourceAudioReferenceFilter(channels int) (string, error) {
	if channels < 1 {
		return "", fmt.Errorf("invalid source audio channel count")
	}
	if channels == 1 {
		return "pan=stereo|c0=c0|c1=c0,ebur128", nil
	}
	if channels == 2 {
		return "ebur128", nil
	}
	side := func(indices []int) string {
		out := ""
		for _, n := range indices {
			if n < channels {
				out += fmt.Sprintf("+%s*c%d", strconv.FormatFloat(math.Sqrt(.5), 'g', -1, 64), n)
			}
		}
		return out
	}
	return "pan=stereo|c0=c0" + side([]int{2, 4, 6}) + "|c1=c1" + side([]int{2, 5, 7}) + ",ebur128", nil
}
