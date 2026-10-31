package transcript

// Chuyển ca test từ yt-factory src/server/clone/transcript.test.ts (2c201e6) —
// phần handler qua deps giả (không process thật). Context giả dựng trên hợp đồng
// pipeline.StepContext: upstream artifacts → s.Acquire, publish → StepResult.Artifacts.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

var readyStatus = tools.TranscriptionStatus{
	State: tools.TranscriptionReady, EnvDir: "/env", Python: "/env/bin/python", FasterWhisper: "1.2.1",
}

const json3Fixture = `{"events":[
	{"tStartMs":0,"dDurationMs":200},
	{"tStartMs":600,"dDurationMs":3000,"segs":[{"utf8":"Câu đầu"},{"utf8":" tiên","tOffsetMs":400}]}
]}`

func vadReport() map[string]any {
	return map[string]any{
		"event": "done",
		"mode":  "vad",
		"model": map[string]any{
			"name": "silero-vad", "version": "6.2.0",
			"checkpoint": map[string]any{"file": "silero_vad.jit", "sha256": strings.Repeat("a", 64)},
		},
		"params":      map[string]any{"threshold": 0.5, "merge_gap_ms": 32},
		"sampleRate":  16000,
		"windowSec":   30,
		"contextSec":  2,
		"durationSec": 20,
		"intervals": []any{
			map[string]any{"startMs": 600, "endMs": 3600},
			map[string]any{"startMs": 12100, "endMs": 14500},
		},
		"peakRssBytes": 1e6,
	}
}

func asrReport() map[string]any {
	return map[string]any{
		"event": "done",
		"mode":  "asr",
		"model": map[string]any{
			"name": "faster-whisper", "version": "1.2.1", "size": "small",
			"computeType": "int8", "device": "cpu",
		},
		"languageHint":    "vi",
		"detected":        map[string]any{"language": "vi", "probability": 0.97},
		"sliceSec":        600,
		"sliceOverlapSec": 5,
		"segments": []any{
			map[string]any{
				"startMs": 12100, "endMs": 14500, "text": "Đoạn nói không có phụ đề",
				"avgLogprob": -0.2, "words": []any{},
			},
		},
		"peakRssBytes": 2e6,
	}
}

// fakeMedia returns artifact contents from a map of local files.
type fakeMedia map[string]string

func (m fakeMedia) Acquire(_ context.Context, path string) (*media.Handle, error) {
	if p, ok := m[path]; ok {
		return &media.Handle{Path: p, Ref: media.Ref{Path: path}}, nil
	}
	return nil, errors.New("media: not found " + path)
}
func (m fakeMedia) Publish(context.Context, []media.Artifact, func(context.Context) error) error {
	return errors.New("media: publish not expected")
}

type fakeRunner struct {
	lastFile string
	lastArgs []string
	env      map[string]string
	run      func(ctx context.Context, file string, args []string) (tools.Result, error)
}

func (r *fakeRunner) Run(ctx context.Context, file string, args []string, spec tools.Spec) (tools.Result, error) {
	r.lastFile, r.lastArgs, r.env = file, append([]string(nil), args...), spec.Env
	if r.run != nil {
		return r.run(ctx, file, args)
	}
	return tools.Result{}, nil
}

func fakeStepCtx(t *testing.T, upstream map[domain.CloneStep]domain.CloneStepOutput) (*pipeline.StepContext, string) {
	t.Helper()
	staging := t.TempDir()
	ctx := &pipeline.StepContext{
		Job:             domain.CloneJob{ID: "job-transcript", Source: domain.CloneSourceSnapshot{ID: "abcdefghijk", URL: "https://youtu.be/abcdefghijk", Title: "Fixture"}},
		Step:            domain.CloneStepRecord{Step: domain.CloneTranscript, Attempt: 1},
		StagingDir:      staging,
		Upstream:        upstream,
		ReportProgress:  func(context.Context, domain.CloneProgress) error { return nil },
		WriteCheckpoint: func(context.Context, json.RawMessage) error { return nil },
		IsPaused:        func() bool { return false },
	}
	return ctx, staging
}

func fakeDeps(t *testing.T, files fakeMedia, overrides func(*Deps)) *Deps {
	t.Helper()
	d := &Deps{
		Media: files,
		ResolveFfmpeg: func(context.Context) *tools.FfmpegResolved {
			return &tools.FfmpegResolved{Ffmpeg: "/managed/ffmpeg", Ffprobe: "/managed/ffprobe"}
		},
		TranscriptionStatus: func(context.Context) tools.TranscriptionStatus { return readyStatus },
		RunTranscribe: func(_ context.Context, o TranscribeRun) (any, error) {
			report := asrReport()
			if o.Mode == "vad" {
				report = vadReport()
			}
			raw, _ := json.Marshal(report)
			_ = os.WriteFile(o.ReportPath, raw, 0o600)
			return report, nil
		},
		StoredSegments: func(string) []StoredSegment { return nil },
		Decode: func(_ context.Context, _ string, _ string, output string) error {
			return os.WriteFile(output, []byte("analysis PCM fixture"), 0o600)
		},
		Now: func() int64 { return 1700000000000 },
	}
	if overrides != nil {
		overrides(d)
	}
	return d
}

func upstreamWith(captionData any) map[domain.CloneStep]domain.CloneStepOutput {
	return map[domain.CloneStep]domain.CloneStepOutput{
		domain.CloneDownload: {
			Artifacts: []domain.CloneArtifact{
				{Path: "download/source.mp4", SHA256: "a", Bytes: 3},
				{Path: "download/source.vi.json3", SHA256: "b", Bytes: 10},
			},
			Data: map[string]any{"durationSec": 20.0, "caption": captionData},
		},
	}
}

func setupArtifacts(t *testing.T) fakeMedia {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mp4")
	caps := filepath.Join(dir, "source.vi.json3")
	if err := os.WriteFile(src, []byte("mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caps, []byte(json3Fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return fakeMedia{"download/source.mp4": src, "download/source.vi.json3": caps}
}

func readManifest(t *testing.T, staging string) CaptionManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(staging, "captions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m CaptionManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestTranscriptStepParsesJson3RunsVadAsr(t *testing.T) {
	files := setupArtifacts(t)
	upstream := upstreamWith(map[string]any{"kind": "auto", "language": "vi", "path": "download/source.vi.json3"})
	ctx, staging := fakeStepCtx(t, upstream)
	modes := []string{}
	deps := fakeDeps(t, files, func(d *Deps) {
		d.RunTranscribe = func(_ context.Context, o TranscribeRun) (any, error) {
			modes = append(modes, o.Mode)
			report := asrReport()
			if o.Mode == "vad" {
				report = vadReport()
			}
			raw, _ := json.Marshal(report)
			_ = os.WriteFile(o.ReportPath, raw, 0o600)
			return report, nil
		}
	})
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrings(modes, []string{"vad", "asr", "asr"}) {
		t.Fatalf("modes: %v", modes)
	}
	paths := []string{}
	for _, a := range result.Artifacts {
		paths = append(paths, a.Path)
	}
	sort.Strings(paths)
	want := []string{
		"transcript/asr-anchors.json", "transcript/asr-report.json",
		"transcript/captions.json", "transcript/captions.srt", "transcript/captions.vtt",
		"transcript/speech-report.json",
	}
	if !equalStrings(paths, want) {
		t.Fatalf("artifacts: %v", paths)
	}
	manifest := readManifest(t, staging)
	var source, asr *Cue
	for i := range manifest.Cues {
		switch manifest.Cues[i].Source {
		case "captions":
			source = &manifest.Cues[i]
		case "asr":
			asr = &manifest.Cues[i]
		}
	}
	if source == nil || source.StartMs != 600 || source.EndMs == nil || *source.EndMs != 3600 {
		t.Fatalf("caption cue: %+v", manifest.Cues)
	}
	if asr == nil || asr.Text != "Đoạn nói không có phụ đề" {
		t.Fatalf("asr cue: %+v", manifest.Cues)
	}
	if manifest.Language == nil || *manifest.Language != "vi" || manifest.Revision != 1 {
		t.Fatalf("manifest: %+v", manifest)
	}
	if len(manifest.SpeechIntervals) != 2 || manifest.Coverage.Ratio != 1 {
		t.Fatalf("coverage: %+v", manifest.Coverage)
	}
	if len(manifest.UnresolvedGaps) != 0 {
		t.Fatalf("gaps: %+v", manifest.UnresolvedGaps)
	}
	if result.Data["analysisAudio"] != "mix" {
		t.Fatalf("data: %+v", result.Data)
	}
	srt, err := os.ReadFile(filepath.Join(staging, "captions.srt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srt), "Câu đầu tiên") || !strings.Contains(string(srt), "Đoạn nói không có phụ đề") {
		t.Fatalf("srt: %q", srt)
	}
}

func TestTranscriptDecodesFromSourceMix(t *testing.T) {
	files := setupArtifacts(t)
	upstream := upstreamWith(map[string]any{"kind": "auto", "language": "vi", "path": "download/source.vi.json3"})
	ctx, _ := fakeStepCtx(t, upstream)
	decoded := []string{}
	deps := fakeDeps(t, files, func(d *Deps) {
		d.Decode = func(_ context.Context, _ string, input, output string) error {
			decoded = append(decoded, input)
			return os.WriteFile(output, []byte("x"), 0o600)
		}
	})
	if _, err := New(deps).Run(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0] != files["download/source.mp4"] {
		t.Fatalf("decoded: %v", decoded)
	}
}

func TestTranscriptWithoutCaptionsLeavesReviewGaps(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mp4")
	_ = os.WriteFile(src, []byte("mp4"), 0o600)
	upstream := map[domain.CloneStep]domain.CloneStepOutput{
		domain.CloneDownload: {
			Artifacts: []domain.CloneArtifact{{Path: "download/source.mp4", SHA256: "a", Bytes: 3}},
			Data:      map[string]any{"durationSec": 20.0, "caption": nil},
		},
	}
	ctx, staging := fakeStepCtx(t, upstream)
	if _, err := New(fakeDeps(t, fakeMedia{"download/source.mp4": src}, nil)).Run(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, staging)
	if len(manifest.Cues) != 1 || manifest.Cues[0].Source != "asr" || manifest.Coverage.CoveredFrames == 0 {
		t.Fatalf("%+v", manifest)
	}
}

func TestStoredStartOnlyTranscriptIsReviewEvidence(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mp4")
	_ = os.WriteFile(src, []byte("mp4"), 0o600)
	upstream := map[domain.CloneStep]domain.CloneStepOutput{
		domain.CloneDownload: {
			Artifacts: []domain.CloneArtifact{{Path: "download/source.mp4", SHA256: "a", Bytes: 3}},
			Data:      map[string]any{"durationSec": 20.0, "caption": nil},
		},
	}
	ctx, staging := fakeStepCtx(t, upstream)
	deps := fakeDeps(t, fakeMedia{"download/source.mp4": src}, func(d *Deps) {
		d.StoredSegments = func(string) []StoredSegment {
			return []StoredSegment{
				{Start: new(0.6), Text: "Câu một"},
				{Start: nil, Text: "Câu không có thời gian"},
			}
		}
	})
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, staging)
	stored := 0
	for _, c := range manifest.Cues {
		if strings.HasPrefix(c.ID, "stored-") {
			stored++
			if c.EndMs != nil {
				t.Fatalf("stored cue got fabricated end: %+v", c)
			}
		}
	}
	if stored != 2 {
		t.Fatalf("stored cues: %+v", manifest.Cues)
	}
	found := false
	for _, w := range manifest.Warnings {
		if w.Code == "untimed_source" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings: %+v", manifest.Warnings)
	}
	found = false
	for _, r := range manifest.NeedsReview {
		if r.Reason == "cue has no end timing" {
			found = true
		}
	}
	if !found || result.Data["needsReviewCount"].(int64) == 0 {
		t.Fatalf("review: %+v", manifest.NeedsReview)
	}
}

func TestTranscriptFailsWithoutAudioEvidence(t *testing.T) {
	upstream := map[domain.CloneStep]domain.CloneStepOutput{
		domain.CloneDownload: {Artifacts: nil, Data: map[string]any{"durationSec": 20.0}},
	}
	ctx, _ := fakeStepCtx(t, upstream)
	_, err := New(fakeDeps(t, fakeMedia{}, nil)).Run(context.Background(), ctx)
	var cerr *domain.CloneError
	if !errors.As(err, &cerr) || cerr.Code != domain.CloneErrInternal {
		t.Fatalf("expected internal_error, got %v", err)
	}
}

func TestTranscriptFailsWhenEnvMissing(t *testing.T) {
	files := setupArtifacts(t)
	upstream := upstreamWith(map[string]any{"kind": "auto", "language": "vi", "path": "download/source.vi.json3"})
	ctx, _ := fakeStepCtx(t, upstream)
	deps := fakeDeps(t, files, func(d *Deps) {
		d.TranscriptionStatus = func(context.Context) tools.TranscriptionStatus {
			return tools.TranscriptionStatus{State: tools.TranscriptionMissing}
		}
	})
	_, err := New(deps).Run(context.Background(), ctx)
	var cerr *domain.CloneError
	if !errors.As(err, &cerr) || cerr.Code != domain.CloneErrToolMissing {
		t.Fatalf("expected tool_missing, got %v", err)
	}
}

func TestEstimatedSpansProduceHashedWordAnchors(t *testing.T) {
	files := setupArtifacts(t)
	upstream := upstreamWith(map[string]any{"kind": "auto", "language": "vi", "path": "download/source.vi.json3"})
	ctx, staging := fakeStepCtx(t, upstream)
	deps := fakeDeps(t, files, func(d *Deps) {
		d.RunTranscribe = func(_ context.Context, o TranscribeRun) (any, error) {
			report := asrReport()
			if o.Mode == "vad" {
				report = vadReport()
			} else {
				report["segments"] = []any{
					map[string]any{
						"startMs": 700, "endMs": 1900, "text": "Câu đầu tiên", "avgLogprob": -0.1,
						"words": []any{
							map[string]any{"text": "Câu", "startMs": 700, "endMs": 1000, "probability": 0.95},
							map[string]any{"text": "đầu", "startMs": 1100, "endMs": 1400, "probability": 0.95},
							map[string]any{"text": "tiên", "startMs": 1500, "endMs": 1800, "probability": 0.95},
						},
					},
				}
			}
			if strings.HasSuffix(o.ReportPath, "anchor-report.json") {
				found := false
				for _, r := range o.Regions {
					if r.StartMs == 300 && r.EndMs == 3900 {
						found = true
					}
				}
				if !found {
					t.Errorf("anchor regions: %+v", o.Regions)
				}
			}
			raw, _ := json.Marshal(report)
			_ = os.WriteFile(o.ReportPath, raw, 0o600)
			return report, nil
		}
	})
	result, err := New(deps).Run(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(staging, "asr-anchors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var anchors AsrAnchors
	if err := json.Unmarshal(raw, &anchors); err != nil {
		t.Fatal(err)
	}
	if len(anchors.SourceAudioSHA256) != 64 {
		t.Fatalf("sha: %q", anchors.SourceAudioSHA256)
	}
	if len(anchors.Words) == 0 || anchors.Words[0].Probability == nil || *anchors.Words[0].Probability != 0.95 {
		t.Fatalf("words: %+v", anchors.Words)
	}
	manifest := readManifest(t, staging)
	if manifest.Cues[0].Words[0].StartMs != 700 || manifest.Cues[0].Words[0].TimingSource != "aligned" {
		t.Fatalf("cue words: %+v", manifest.Cues[0].Words)
	}
	if result.Data["anchorWordCount"] != int64(3) {
		t.Fatalf("data: %+v", result.Data)
	}
}

func TestWordAnchorFailureIsHonestWarning(t *testing.T) {
	files := setupArtifacts(t)
	upstream := upstreamWith(map[string]any{"kind": "auto", "language": "vi", "path": "download/source.vi.json3"})
	ctx, staging := fakeStepCtx(t, upstream)
	deps := fakeDeps(t, files, func(d *Deps) {
		d.RunTranscribe = func(_ context.Context, o TranscribeRun) (any, error) {
			if strings.HasSuffix(o.ReportPath, "anchor-report.json") {
				return nil, errors.New("ASR unavailable")
			}
			report := asrReport()
			if o.Mode == "vad" {
				report = vadReport()
			}
			return report, nil
		}
	})
	if _, err := New(deps).Run(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	manifest := readManifest(t, staging)
	found := false
	for _, w := range manifest.Warnings {
		if w.Code == "asr_anchors_missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings: %+v", manifest.Warnings)
	}
	if _, err := os.Stat(filepath.Join(staging, "asr-anchors.json")); !os.IsNotExist(err) {
		t.Fatal("anchor file should not exist")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
