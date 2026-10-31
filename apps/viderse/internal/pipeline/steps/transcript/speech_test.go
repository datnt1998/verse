package transcript

// Chuyển ca test từ yt-factory src/server/clone/speech-analysis.test.ts (2c201e6).

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestParseAdapterEvent(t *testing.T) {
	ev := ParseAdapterEvent(`{"event":"progress","done":1,"total":2}`)
	if ev == nil || ev["event"] != "progress" || ev["done"] != float64(1) || ev["total"] != float64(2) {
		t.Fatalf("%+v", ev)
	}
	if ParseAdapterEvent("loading model…") != nil {
		t.Fatal("noise accepted")
	}
	if ParseAdapterEvent("{not json") != nil {
		t.Fatal("bad JSON accepted")
	}
	if ParseAdapterEvent(`{"foo":1}`) != nil {
		t.Fatal("event-less object accepted")
	}
}

func vadReportFixture() map[string]any {
	return map[string]any{
		"event": "done",
		"mode":  "vad",
		"model": map[string]any{
			"name": "silero-vad", "version": "6.2.0",
			"checkpoint": map[string]any{"file": "silero_vad.jit", "sha256": "abc"},
		},
		"params":      map[string]any{"threshold": 0.5, "merge_gap_ms": 32},
		"sampleRate":  16000,
		"windowSec":   30,
		"contextSec":  2,
		"durationSec": 60,
		"intervals": []any{
			map[string]any{"startMs": 1000, "endMs": 2000},
			map[string]any{"startMs": 2010, "endMs": 3000},
			map[string]any{"startMs": 10000, "endMs": 12000},
		},
		"peakRssBytes": 123,
	}
}

func TestDetectSpeechValidatesAndMerges(t *testing.T) {
	report, err := DetectSpeech(context.Background(), SpeechDeps{
		RunVad: func(context.Context, VadRun) (any, error) { return vadReportFixture(), nil },
	}, VadRun{InputWav: "/x.wav", ReportPath: "/r.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Intervals, []Interval{{1000, 3000}, {10000, 12000}}) {
		t.Fatalf("%+v", report.Intervals)
	}
	if report.Evidence.Model.Version != "6.2.0" || report.Evidence.IntervalCount != 2 || report.Evidence.PeakRSSBytes != 123 {
		t.Fatalf("%+v", report.Evidence)
	}
}

func TestDetectSpeechRejectsMalformedReport(t *testing.T) {
	_, err := DetectSpeech(context.Background(), SpeechDeps{
		RunVad: func(context.Context, VadRun) (any, error) {
			return map[string]any{"event": "done", "mode": "vad"}, nil
		},
	}, VadRun{InputWav: "/x.wav", ReportPath: "/r.json"})
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected schema error: %v", err)
	}
}

func TestVadReportSchemaEnforcesIntervalShape(t *testing.T) {
	ok := map[string]any{
		"event": "done", "mode": "vad",
		"model":  map[string]any{"name": "silero-vad", "version": "6.2.0", "checkpoint": nil},
		"params": map[string]any{"merge_gap_ms": 32}, "sampleRate": 16000, "windowSec": 30,
		"contextSec": 2, "durationSec": 60,
		"intervals": []any{map[string]any{"startMs": 1, "endMs": 2}}, "peakRssBytes": 1,
	}
	if _, err := parseVadReport(ok); err != nil {
		t.Fatal(err)
	}
	bad := map[string]any{
		"event": "done", "mode": "vad",
		"model":  map[string]any{"name": "silero-vad", "version": "6.2.0", "checkpoint": nil},
		"params": map[string]any{"merge_gap_ms": 32}, "sampleRate": 16000, "windowSec": 30,
		"contextSec": 2, "durationSec": 60,
		"intervals": []any{map[string]any{"startMs": "x", "endMs": 2}}, "peakRssBytes": 1,
	}
	if _, err := parseVadReport(bad); err == nil {
		t.Fatal("malformed interval accepted")
	}
}

func TestUncoveredSpeechEdgePadding(t *testing.T) {
	gaps := UncoveredSpeech([]Interval{{0, 10000}}, []Interval{{1000, 5000}}, 350)
	if !reflect.DeepEqual(gaps, []Interval{{0, 650}, {5350, 10000}}) {
		t.Fatalf("%+v", gaps)
	}
}

func TestUncoveredSpeechDropsSlivers(t *testing.T) {
	if got := UncoveredSpeech([]Interval{{0, 1000}}, []Interval{{-400, 970}}, 50); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := UncoveredSpeech([]Interval{{0, 1000}}, []Interval{{0, 970}}, 0); len(got) != 0 {
		t.Fatalf("30ms sliver must drop: %+v", got)
	}
}

func TestUncoveredSpeechMultiSpan(t *testing.T) {
	gaps := UncoveredSpeech(
		[]Interval{{0, 2000}, {5000, 9000}},
		[]Interval{{5500, 6000}},
		0,
	)
	if !reflect.DeepEqual(gaps, []Interval{{0, 2000}, {5000, 5500}, {6000, 9000}}) {
		t.Fatalf("%+v", gaps)
	}
}
