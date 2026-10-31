package render

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/editor"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

const compressedDurationToleranceSec = 0.5

type MediaFactsWithBytes struct {
	MediaFacts
	Bytes int64
}

type CompressOptions struct {
	Runner  tools.Runner
	Ffmpeg  string
	FFprobe string
	Input   string
	Output  string
	// Project is the committed export revision, never the latest editable draft.
	Project         editor.Project
	CaptionCoverage editor.Object
	OnProgress      func(float64, *float64)
}

func compressedSameValue[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func compressedValue[T any](value *T, missing string) string {
	if value == nil {
		return missing
	}
	return fmt.Sprint(*value)
}

// CompressedOutputProblem is empty only when a smaller result preserves the
// original frame dimensions, duration and copied audio, and remains H.264.
func CompressedOutputProblem(before, after MediaFactsWithBytes) string {
	if !compressedSameValue(after.Width, before.Width) || !compressedSameValue(after.Height, before.Height) {
		return fmt.Sprintf("kích thước khung hình đổi từ %s×%s thành %s×%s", compressedValue(before.Width, "null"), compressedValue(before.Height, "null"), compressedValue(after.Width, "null"), compressedValue(after.Height, "null"))
	}
	if after.VideoCodec == nil || *after.VideoCodec != "h264" {
		return fmt.Sprintf("codec hình là %s, không phải h264", compressedValue(after.VideoCodec, "không rõ"))
	}
	if !compressedSameValue(after.AudioCodec, before.AudioCodec) || !compressedSameValue(after.AudioChannels, before.AudioChannels) {
		return "track tiếng không còn như bản gốc"
	}
	if before.DurationSec == nil || after.DurationSec == nil || !roiPositive(*before.DurationSec) || !roiPositive(*after.DurationSec) || math.Abs(*after.DurationSec-*before.DurationSec) > compressedDurationToleranceSec {
		return fmt.Sprintf("độ dài đổi từ %ss thành %ss", compressedValue(before.DurationSec, "?"), compressedValue(after.DurationSec, "?"))
	}
	if after.Bytes >= before.Bytes {
		return fmt.Sprintf("bản nén %.0f MB không nhỏ hơn bản gốc %.0f MB", math.Floor(float64(after.Bytes)/(1024*1024)+0.5), math.Floor(float64(before.Bytes)/(1024*1024)+0.5))
	}
	return ""
}

// CompressExportFile writes a candidate only: callers publish/replace the
// committed render after this function's output validation has succeeded.
func CompressExportFile(ctx context.Context, opts CompressOptions) (err error) {
	if opts.Runner == nil || opts.Ffmpeg == "" || opts.FFprobe == "" || opts.Input == "" || opts.Output == "" {
		return fmt.Errorf("missing export compression runner, tools or paths")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	inputPath, err := filepath.Abs(opts.Input)
	if err != nil {
		return err
	}
	outputPath, err := filepath.Abs(opts.Output)
	if err != nil {
		return err
	}
	if inputPath == outputPath {
		return fmt.Errorf("compression output must not replace input before validation")
	}
	inputInfo, err := os.Stat(opts.Input)
	if err != nil {
		return err
	}
	if !inputInfo.Mode().IsRegular() {
		return fmt.Errorf("compression input is not a regular file")
	}
	if outputInfo, statErr := os.Stat(opts.Output); statErr == nil {
		if os.SameFile(inputInfo, outputInfo) {
			return fmt.Errorf("compression output aliases input")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	beforeFacts, err := ProbeMedia(ctx, opts.Runner, opts.FFprobe, opts.Input)
	if err != nil {
		return err
	}
	before := MediaFactsWithBytes{MediaFacts: beforeFacts, Bytes: inputInfo.Size()}
	filter := ""
	if opts.Project != nil {
		encoded := EncodedFacts{DurationSec: *before.DurationSec}
		if before.Width != nil {
			encoded.Width = *before.Width
		}
		if before.Height != nil {
			encoded.Height = *before.Height
		}
		regions, deriveErr := DeriveExportRegions(opts.Project, encoded, 12, opts.CaptionCoverage)
		if deriveErr != nil {
			return deriveErr
		}
		filter, err = ExportRegionFilter(regions)
		if err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	var filterArgs []string
	if filter != "" {
		scratch, createErr := os.MkdirTemp("", "ytf-export-roi-")
		if createErr != nil {
			return createErr
		}
		defer func() {
			if cleanupErr := os.RemoveAll(scratch); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("remove ROI filter: %w", cleanupErr))
			}
		}()
		script := filepath.Join(scratch, "regions.filter")
		if err = os.WriteFile(script, []byte(filter), 0600); err != nil {
			return err
		}
		filterArgs = []string{"-filter_script:v", script}
	}
	args := []string{"-hide_banner", "-nostdin", "-y", "-progress", "pipe:1", "-nostats", "-i", opts.Input, "-map", "0:v:0", "-map", "0:a?"}
	args = append(args, filterArgs...)
	args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "28", "-maxrate", "900k", "-bufsize", "1800k", "-g", "300", "-pix_fmt", "yuv420p", "-c:a", "copy", "-movflags", "+faststart", opts.Output)
	spec := tools.Spec{Background: true}
	if opts.OnProgress != nil {
		spec.OnLine = func(line string, stream tools.Stream) {
			if stream != tools.StreamStdout || !strings.HasPrefix(line, "out_time_us=") {
				return
			}
			value := strings.TrimPrefix(line, "out_time_us=")
			if value == "" {
				return
			}
			for _, char := range value {
				if char < '0' || char > '9' {
					return
				}
			}
			microseconds, parseErr := strconv.ParseFloat(value, 64)
			if parseErr == nil && !math.IsNaN(microseconds) && !math.IsInf(microseconds, 0) {
				opts.OnProgress(microseconds/1e6, before.DurationSec)
			}
		}
	}
	if _, err = opts.Runner.Run(ctx, opts.Ffmpeg, args, spec); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	afterFacts, err := ProbeMedia(ctx, opts.Runner, opts.FFprobe, opts.Output)
	if err != nil {
		return err
	}
	afterInfo, err := os.Stat(opts.Output)
	if err != nil {
		return err
	}
	if !afterInfo.Mode().IsRegular() {
		return fmt.Errorf("compression output is not a regular file")
	}
	problem := CompressedOutputProblem(before, MediaFactsWithBytes{MediaFacts: afterFacts, Bytes: afterInfo.Size()})
	if problem != "" {
		return fmt.Errorf("Bản nén lại không dùng được: %s", problem)
	}
	return ctx.Err()
}
