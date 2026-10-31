package render

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// SourceAudioReferenceVersion versions evidence measured in the engine's stereo layout.
const SourceAudioReferenceVersion = 1

// MediaFacts keeps unknown probe evidence distinct from zero and false.
type MediaFacts struct {
	DurationSec   *float64 `json:"durationSec"`
	Width         *int     `json:"width"`
	Height        *int     `json:"height"`
	Progressive   *bool    `json:"progressive"`
	VideoCodec    *string  `json:"videoCodec"`
	AudioCodec    *string  `json:"audioCodec"`
	AudioChannels *int     `json:"audioChannels"`
	Container     *string  `json:"container"`
}

var integratedLUFSPattern = regexp.MustCompile(`(?m)^\s*I:\s*(-?\d+(?:\.\d+)?)\s*LUFS\s*$`)

func ParseIntegratedLUFS(output string) *float64 {
	matches := integratedLUFSPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	value, err := strconv.ParseFloat(matches[len(matches)-1][1], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}

// SourceAudioReferenceFilter mirrors downmixToOutputChannels(..., 2), without normalization.
func SourceAudioReferenceFilter(channels int) (string, error) {
	if channels < 1 {
		return "", fmt.Errorf("Không có số kênh audio nguồn hợp lệ — dựng lại project")
	}
	if channels == 1 {
		return "pan=stereo|c0=c0|c1=c0,ebur128", nil
	}
	if channels == 2 {
		return "ebur128", nil
	}
	// Match JavaScript Math.SQRT1_2's serialized coefficient exactly.
	const coefficient = "0.7071067811865476"
	side := func(indices []int) string {
		var out strings.Builder
		for _, index := range indices {
			if index < channels {
				fmt.Fprintf(&out, "+%s*c%d", coefficient, index)
			}
		}
		return out.String()
	}
	return "pan=stereo|c0=c0" + side([]int{2, 4, 6}) + "|c1=c1" + side([]int{2, 5, 7}) + ",ebur128", nil
}

func MeasureLayoutAwareLUFS(ctx context.Context, runner tools.Runner, ffmpeg, file string, channels int) (float64, error) {
	filter, err := SourceAudioReferenceFilter(channels)
	if err != nil {
		return 0, err
	}
	return measureFilteredFileLUFS(ctx, runner, ffmpeg, file, filter)
}

func MeasureFileLUFS(ctx context.Context, runner tools.Runner, ffmpeg, file string) (float64, error) {
	return measureFilteredFileLUFS(ctx, runner, ffmpeg, file, "ebur128")
}

func measureFilteredFileLUFS(ctx context.Context, runner tools.Runner, ffmpeg, file, filter string) (float64, error) {
	if runner == nil || ffmpeg == "" || file == "" {
		return 0, fmt.Errorf("missing loudness runner, ffmpeg or file")
	}
	out, err := runner.Run(ctx, ffmpeg, []string{"-hide_banner", "-nostdin", "-i", file, "-af", filter, "-f", "null", "-"}, tools.Spec{Timeout: 30 * time.Minute})
	if err != nil {
		return 0, err
	}
	parsed := ParseIntegratedLUFS(out.Stderr)
	if parsed == nil {
		return 0, fmt.Errorf("Không đọc được integrated loudness của file")
	}
	return *parsed, nil
}

func MeasurePCMLUFS(ctx context.Context, runner tools.Runner, ffmpeg, file string, sampleRate, channels int) (float64, error) {
	if runner == nil || ffmpeg == "" || file == "" || sampleRate <= 0 || channels <= 0 {
		return 0, fmt.Errorf("invalid PCM loudness input")
	}
	out, err := runner.Run(ctx, ffmpeg, []string{"-hide_banner", "-nostdin", "-f", "f32le", "-ar", strconv.Itoa(sampleRate), "-ac", strconv.Itoa(channels), "-i", file, "-af", "ebur128", "-f", "null", "-"}, tools.Spec{Timeout: 30 * time.Minute})
	if err != nil {
		return 0, err
	}
	parsed := ParseIntegratedLUFS(out.Stderr)
	if parsed == nil {
		return 0, fmt.Errorf("Không đọc được integrated loudness của mix đã đo")
	}
	return *parsed, nil
}

// ParseFFprobeMediaFacts accepts only the ffprobe JSON document shape; malformed
// evidence remains unknown rather than being filled from assumed defaults.
func ParseFFprobeMediaFacts(raw string) MediaFacts {
	var doc map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &doc) != nil || doc == nil {
		return MediaFacts{}
	}
	var format map[string]any
	if value, ok := doc["format"]; ok {
		if json.Unmarshal(value, &format) != nil || format == nil {
			return MediaFacts{}
		}
	}
	var streams []map[string]any
	if value, ok := doc["streams"]; ok {
		if json.Unmarshal(value, &streams) != nil || streams == nil {
			return MediaFacts{}
		}
		for _, stream := range streams {
			if stream == nil {
				return MediaFacts{}
			}
		}
	}
	facts := MediaFacts{}
	if value, ok := format["duration"]; ok {
		var duration float64
		switch v := value.(type) {
		case string:
			duration, _ = strconv.ParseFloat(v, 64)
		case float64:
			duration = v
		}
		if duration > 0 && !math.IsNaN(duration) && !math.IsInf(duration, 0) {
			facts.DurationSec = &duration
		}
	}
	if container, ok := format["format_name"].(string); ok {
		facts.Container = &container
	}
	var video, audio map[string]any
	for _, stream := range streams {
		if stream["codec_type"] == "video" && video == nil {
			video = stream
		}
		if stream["codec_type"] == "audio" && audio == nil {
			audio = stream
		}
	}
	facts.Width = probeInteger(video["width"])
	facts.Height = probeInteger(video["height"])
	if order, ok := video["field_order"].(string); ok {
		progressive := order == "progressive" || order == "unknown"
		facts.Progressive = &progressive
	}
	if codec, ok := video["codec_name"].(string); ok {
		facts.VideoCodec = &codec
	}
	if codec, ok := audio["codec_name"].(string); ok {
		facts.AudioCodec = &codec
	}
	facts.AudioChannels = probeInteger(audio["channels"])
	return facts
}

func probeInteger(value any) *int {
	number, ok := value.(float64)
	if !ok || number <= 0 || math.IsInf(number, 0) || math.IsNaN(number) || math.Trunc(number) != number || number >= float64(int(^uint(0)>>1)) {
		return nil
	}
	integer := int(number)
	return &integer
}

func ProbeMedia(ctx context.Context, runner tools.Runner, ffprobe, file string) (MediaFacts, error) {
	if runner == nil || ffprobe == "" || file == "" {
		return MediaFacts{}, fmt.Errorf("missing probe runner, ffprobe or file")
	}
	out, err := runner.Run(ctx, ffprobe, []string{"-v", "error", "-show_format", "-show_streams", "-print_format", "json", file}, tools.Spec{Timeout: time.Minute})
	if err != nil {
		return MediaFacts{}, err
	}
	facts := ParseFFprobeMediaFacts(out.Stdout)
	if facts.DurationSec == nil {
		return MediaFacts{}, fmt.Errorf("ffprobe không đọc được file: %s", file)
	}
	return facts, nil
}
