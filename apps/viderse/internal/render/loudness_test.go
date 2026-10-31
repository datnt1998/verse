package render

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type evidenceRunnerFunc func(context.Context, string, []string, tools.Spec) (tools.Result, error)

func (f evidenceRunnerFunc) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	return f(ctx, file, args, spec)
}

const loudnessSummary = `[Parsed_ebur128_0 @ 0x123] Summary:

  Integrated loudness:
    I:         -14.3 LUFS
    Threshold: -24.4 LUFS

  Loudness range:
    LRA:         6.1 LU
`

func TestParseIntegratedLUFSLastSummary(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want *float64
	}{
		{loudnessSummary, new(-14.3)},
		{loudnessSummary + "\n    I:         -16.0 LUFS\n", new(-16.0)},
		{"I:         -inf LUFS", nil}, {"", nil}, {"no loudness here", nil},
		{"I: -17.2 LUFS\r\n", new(-17.2)},
	} {
		if got := ParseIntegratedLUFS(test.raw); !reflect.DeepEqual(got, test.want) {
			t.Errorf("parse %q = %v, want %v", test.raw, got, test.want)
		}
	}
}

func TestParseFFprobeMediaFactsTracks(t *testing.T) {
	facts := ParseFFprobeMediaFacts(`{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"field_order":"progressive"},{"codec_type":"audio","codec_name":"aac","channels":2}],"format":{"duration":"12.500000","format_name":"mov,mp4,m4a,3gp,3g2,mj2"}}`)
	want := MediaFacts{DurationSec: new(12.5), Width: new(1920), Height: new(1080), Progressive: new(true), VideoCodec: new("h264"), AudioCodec: new("aac"), AudioChannels: new(2), Container: new("mov,mp4,m4a,3gp,3g2,mj2")}
	if !reflect.DeepEqual(facts, want) {
		t.Fatalf("facts = %+v, want %+v", facts, want)
	}
	for _, order := range []string{"tt", "unknown"} {
		got := ParseFFprobeMediaFacts(`{"streams":[{"codec_type":"video","field_order":"` + order + `"}],"format":{"duration":"3"}}`)
		if got.Progressive == nil || *got.Progressive != (order == "unknown") {
			t.Fatalf("field_order %q = %+v", order, got)
		}
	}
	for _, raw := range []string{"not json", `null`, `[]`, `{"streams":[],"format":{}}`, `{"streams":null}`, `{"streams":[null]}`, `{"streams":{}}`, `{"format":null}`, `{"format":{"duration":"NaN"}}`, `{"format":{"duration":0}}`} {
		if got := ParseFFprobeMediaFacts(raw); got.DurationSec != nil {
			t.Errorf("garbage %s has duration %v", raw, got.DurationSec)
		}
	}
}

func TestSourceAudioReferenceFilterMatchesEngine(t *testing.T) {
	if SourceAudioReferenceVersion != 1 {
		t.Fatal("source reference version changed")
	}
	for _, test := range []struct {
		channels int
		want     string
	}{
		{1, "pan=stereo|c0=c0|c1=c0,ebur128"}, {2, "ebur128"},
		{3, "pan=stereo|c0=c0+0.7071067811865476*c2|c1=c1+0.7071067811865476*c2,ebur128"},
		{6, "pan=stereo|c0=c0+0.7071067811865476*c2+0.7071067811865476*c4|c1=c1+0.7071067811865476*c2+0.7071067811865476*c5,ebur128"},
		{8, "pan=stereo|c0=c0+0.7071067811865476*c2+0.7071067811865476*c4+0.7071067811865476*c6|c1=c1+0.7071067811865476*c2+0.7071067811865476*c5+0.7071067811865476*c7,ebur128"},
	} {
		got, err := SourceAudioReferenceFilter(test.channels)
		if err != nil || got != test.want {
			t.Errorf("channels %d = %q, %v", test.channels, got, err)
		}
	}
	for _, channels := range []int{0, -1} {
		if _, err := SourceAudioReferenceFilter(channels); err == nil || !strings.Contains(err.Error(), "số kênh audio nguồn") {
			t.Errorf("channels %d accepted: %v", channels, err)
		}
	}
	// Non-integral and NaN channel counts cannot enter the integer API; probe
	// evidence still must reject those external JSON values.
}

func TestFFprobeChannelsRequirePositiveIntegerEvidence(t *testing.T) {
	for _, channels := range []any{nil, 0, -1, 1.5, "2"} {
		raw, err := json.Marshal(map[string]any{"streams": []any{map[string]any{"codec_type": "audio", "codec_name": "aac", "channels": channels}}})
		if err != nil {
			t.Fatal(err)
		}
		if got := ParseFFprobeMediaFacts(string(raw)); got.AudioChannels != nil {
			t.Errorf("channels %v accepted", channels)
		}
	}
	if got := ParseFFprobeMediaFacts(`{"streams":[{"codec_type":"audio"}]}`); got.AudioChannels != nil {
		t.Fatal("missing channel count defaulted")
	}
	for _, channels := range []int{1, 2, 6} {
		raw, err := json.Marshal(map[string]any{"streams": []any{map[string]any{"codec_type": "audio", "channels": channels}}})
		if err != nil {
			t.Fatal(err)
		}
		got := ParseFFprobeMediaFacts(string(raw))
		if got.AudioChannels == nil || *got.AudioChannels != channels {
			t.Errorf("channel %d = %+v", channels, got)
		}
	}
}

func TestLoudnessMeasurementUsesExactArguments(t *testing.T) {
	for _, test := range []struct {
		name   string
		args   []string
		invoke func(context.Context, tools.Runner) (float64, error)
	}{
		{"file", []string{"-hide_banner", "-nostdin", "-i", "input", "-af", "ebur128", "-f", "null", "-"}, func(ctx context.Context, r tools.Runner) (float64, error) {
			return MeasureFileLUFS(ctx, r, "ffmpeg", "input")
		}},
		{"pcm", []string{"-hide_banner", "-nostdin", "-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "input", "-af", "ebur128", "-f", "null", "-"}, func(ctx context.Context, r tools.Runner) (float64, error) {
			return MeasurePCMLUFS(ctx, r, "ffmpeg", "input", 48000, 2)
		}},
		{"layout", []string{"-hide_banner", "-nostdin", "-i", "input", "-af", "pan=stereo|c0=c0|c1=c0,ebur128", "-f", "null", "-"}, func(ctx context.Context, r tools.Runner) (float64, error) {
			return MeasureLayoutAwareLUFS(ctx, r, "ffmpeg", "input", 1)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := evidenceRunnerFunc(func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
				if file != "ffmpeg" || !reflect.DeepEqual(args, test.args) || spec.Timeout != 30*time.Minute {
					t.Fatalf("run %q %q timeout %v", file, args, spec.Timeout)
				}
				return tools.Result{Stderr: loudnessSummary}, nil
			})
			got, err := test.invoke(context.Background(), runner)
			if err != nil || math.Abs(got+14.3) > 1e-9 {
				t.Fatalf("LUFS %v, %v", got, err)
			}
			failure := errors.New("ffmpeg failed")
			_, err = test.invoke(context.Background(), evidenceRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
				return tools.Result{}, failure
			}))
			if !errors.Is(err, failure) {
				t.Fatalf("lost process error: %v", err)
			}
			_, err = test.invoke(context.Background(), evidenceRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
				return tools.Result{Stderr: "I: -inf LUFS"}, nil
			}))
			if err == nil {
				t.Fatal("silent mix accepted")
			}
		})
	}
}

func TestProbeMediaArgumentsAndMissingEvidence(t *testing.T) {
	runner := evidenceRunnerFunc(func(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
		if file != "ffprobe" || !reflect.DeepEqual(args, []string{"-v", "error", "-show_format", "-show_streams", "-print_format", "json", "input"}) || spec.Timeout != time.Minute {
			t.Fatalf("probe %s %q %+v", file, args, spec)
		}
		return tools.Result{Stdout: `{"format":{"duration":"3"}}`}, nil
	})
	got, err := ProbeMedia(context.Background(), runner, "ffprobe", "input")
	if err != nil || got.DurationSec == nil || *got.DurationSec != 3 {
		t.Fatalf("probe %+v %v", got, err)
	}
	_, err = ProbeMedia(context.Background(), evidenceRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
		return tools.Result{Stdout: "{}"}, nil
	}), "ffprobe", "input")
	if err == nil {
		t.Fatal("missing probe duration accepted")
	}
	failure := errors.New("probe failed")
	_, err = ProbeMedia(context.Background(), evidenceRunnerFunc(func(context.Context, string, []string, tools.Spec) (tools.Result, error) {
		return tools.Result{}, failure
	}), "ffprobe", "input")
	if !errors.Is(err, failure) {
		t.Fatalf("lost probe error: %v", err)
	}
}
