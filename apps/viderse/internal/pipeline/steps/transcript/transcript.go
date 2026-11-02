// Bước `transcript` của pipeline clone: dựng manifest phụ đề chuẩn — cue json3
// thô với timing start/end/word thật (không bao giờ đường start-only mất
// mát), khoảng speech Silero VAD độc lập trên mix nguồn, và faster-whisper cục
// bộ lấp phần cue timing để trống (cue không timing, thiếu caption, khe
// detector). Chuyển 1:1 từ yt-factory src/server/clone/transcript.ts (2c201e6).
//
// Coverage là chỉ số detector trên khung speech VAD — không bao giờ độ chính
// xác ngữ nghĩa. Cue do ASR sinh hoặc còn thiếu bằng chứng được gắn cờ
// needsReview; không có gì tự duyệt. ApplyCaptionEdits là đường sửa lúc
// review: tăng revision manifest và đánh dấu cue đã đụng.
package transcript

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// StoredSegment là một dòng transcript đã dán của chủ nhân (fallback khi
// không có json3): start nil = không có timestamp.
type StoredSegment struct {
	Start *float64 `json:"start"`
	Text  string   `json:"text"`
}

// Deps gói phụ thuộc có thể thay của bước — test chèn fixture, production wire
// tools/media thật.
type Deps struct {
	// Media acquire artifact upstream (mặc định ctx.Media).
	Media pipeline.Media
	// Runner chạy process con (mặc định ctx.ProcessRunner — process group riêng).
	Runner StepRunner
	// ResolveFfmpeg trả ffmpeg/ffprobe đã quản; nil → tool_missing.
	ResolveFfmpeg func(ctx context.Context) *tools.FfmpegResolved
	// TranscriptionStatus trạng thái env phiên âm (mặc định qua tools.Transcription).
	ResolveUv           func(context.Context) string
	TranscriptionStatus func(ctx context.Context) tools.TranscriptionStatus
	// RunEnv là môi trường chạy venv phiên âm (tools.Transcription.RunEnv) —
	// cần khi RunTranscribe mặc định spawn sidecar.
	RunEnv map[string]string
	// RunTranscribe chạy clone-transcribe.py; mặc định SidecarRunner qua Runner.
	RunTranscribe func(ctx context.Context, run TranscribeRun) (any, error)
	// StoredSegments trả transcript đã dán của nguồn (video_sources legacy); nil
	// khi nguồn không có.
	StoredSegments func(videoID string) []StoredSegment
	// Decode giải mã media nguồn thành WAV float32 stereo 44.1kHz của adapter.
	Decode func(ctx context.Context, ffmpeg, input, output string) error
	// Now trả epoch ms (mặc định thời gian thật).
	Now func() int64
}

// Handler là StepHandler của bước transcript.
type Handler struct{ d *Deps }

// New dựng handler với deps; trường nil dùng mặc định của ctx/tooling.
func New(d *Deps) *Handler {
	if d == nil {
		d = &Deps{}
	}
	return &Handler{d: d}
}

var (
	reSourceArtifact  = regexp.MustCompile(`(^|/)download/source\.mp4$`)
	reCaptionArtifact = regexp.MustCompile(`(^|/)download/source\..+\.json3$`)
)

// upstreamArtifact acquires a read lease; the caller holds it until all
// subprocesses consuming the local file have exited.
func upstreamArtifact(ctx context.Context, s *pipeline.StepContext, step domain.CloneStep, match *regexp.Regexp) (*media.Handle, error) {
	for _, a := range s.Upstream[step].Artifacts {
		if match.MatchString(a.Path) {
			return s.Acquire(ctx, a.Path)
		}
	}
	return nil, nil
}

// decodeToWav giải mã artifact thành đầu vào WAV float32 stereo 44.1 kHz.
func decodeToWav(ctx context.Context, runner StepRunner, ffmpeg, input, output string) error {
	_, err := runner.Run(ctx, ffmpeg, []string{
		"-hide_banner", "-nostdin", "-y",
		"-i", input, "-vn", "-sn", "-dn", "-map", "0:a:0",
		"-af", "aresample=44100:first_pts=0",
		"-ac", "2", "-c:a", "pcm_f32le", output,
	}, tools.Spec{Label: "clone-decode-wav", Timeout: 30 * time.Minute})
	return err
}

// sha256File băm một file — bằng chứng nguồn audio của anchor.
func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

const (
	// ASRModelSize: mô hình lớn hơn + beam search cho speech không caption.
	ASRModelSize = "small"
	asrBeamSize  = 5
	// AnchorModelSize: anchor chỉ gán giờ cho từ caption đã có — base greedy
	// nhanh hơn small beam nhiều mà p95 lệch start < 3 khung ở 30fps.
	AnchorModelSize = "base"
	anchorBeamSize  = 1
)

func (h *Handler) progress(ctx context.Context, s *pipeline.StepContext, done int64, total *int64, phase string) {
	_ = s.Progress(ctx, domain.CloneProgress{UnitsDone: done, UnitsTotal: total, Phase: phase})
}

func i64(v float64) *int64 { x := int64(math.Round(v)); return &x }

// Run chạy bước transcript theo hợp đồng StepHandler.
func (h *Handler) Run(ctx context.Context, s *pipeline.StepContext) (pipeline.StepResult, error) {
	if err := os.MkdirAll(s.StagingDir, 0o700); err != nil {
		return pipeline.StepResult{}, err
	}
	out := func(name string) string { return filepath.Join(s.StagingDir, name) }
	warnings := []CaptionWarning{}
	if h.d.Media != nil {
		copy := *s
		copy.Media = h.d.Media
		s = &copy
	}
	runner := h.d.Runner
	if runner == nil {
		runner = runnerFunc(s.ProcessRunner)
	}

	// Thời lượng nguồn: probe đo được mà bước download đã lưu.
	downloadData := s.Upstream[domain.CloneDownload].Data
	var sourceDurationSec float64
	if v, ok := downloadData["durationSec"].(float64); ok {
		sourceDurationSec = v
	} else if s.Job.Source.DurationSec != nil {
		sourceDurationSec = *s.Job.Source.DurationSec
	} else {
		return pipeline.StepResult{}, &domain.CloneError{
			Code:    domain.CloneErrInternal,
			Message: "Thiếu thời lượng nguồn đã đo — chạy lại bước tải",
		}
	}
	sourceDurationMs := math.Round(sourceDurationSec * 1000)
	var captionLanguage *string
	var captionKind *string
	if captionMeta, ok := downloadData["caption"].(map[string]any); ok {
		if l, ok := captionMeta["language"].(string); ok {
			captionLanguage = &l
		}
		if k, ok := captionMeta["kind"].(string); ok {
			captionKind = &k
		}
	}

	// Audio cho VAD/ASR: decode mới của mix nguồn (video cuối giữ audio nguồn,
	// không có track vocal tách).
	var ff *tools.FfmpegResolved
	if h.d.ResolveFfmpeg != nil {
		ff = h.d.ResolveFfmpeg(ctx)
	}
	if ff == nil || ff.Ffprobe == "" {
		return pipeline.StepResult{}, &domain.CloneError{
			Code:     domain.CloneErrToolMissing,
			Message:  "ffmpeg/ffprobe chưa sẵn sàng",
			Recovery: new("Mở Cài đặt → Công cụ và cài ffmpeg"),
		}
	}
	sourceHandle, err := upstreamArtifact(ctx, s, domain.CloneDownload, reSourceArtifact)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	analysisWav := out("analysis.pcm.wav")
	h.progress(ctx, s, 0, nil, "decode")
	if sourceHandle == nil {
		return pipeline.StepResult{}, &domain.CloneError{
			Code:    domain.CloneErrInternal,
			Message: "Thiếu âm thanh nguồn cho lời kể — chạy lại bước tải",
		}
	}
	defer sourceHandle.Release()
	sourceFile := sourceHandle.Path
	sourceSHA, err := sha256File(sourceFile)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	var stored []StoredSegment
	if h.d.StoredSegments != nil {
		stored = h.d.StoredSegments(s.Job.Source.ID)
	}
	cp, err := loadCheckpoint(s, sourceSHA, stored)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	decode := h.d.Decode
	if decode == nil {
		decode = func(ctx context.Context, _, input, output string) error {
			return decodeToWav(ctx, runner, ff.Ffmpeg, input, output)
		}
	}
	audioSHA, audioErr := sha256File(analysisWav)
	if cp.AudioSHA256 == "" || audioErr != nil || audioSHA != cp.AudioSHA256 {
		if err := decode(ctx, ff.Ffmpeg, sourceFile, analysisWav); err != nil {
			return pipeline.StepResult{}, err
		}
		cp.AudioSHA256, err = sha256File(analysisWav)
		if err != nil {
			return pipeline.StepResult{}, err
		}
		if err := s.SaveCheckpoint(ctx, cp); err != nil {
			return pipeline.StepResult{}, err
		}
	}
	sourceHandle.Release()
	if s.PauseRequested() {
		return pipeline.StepResult{}, pipeline.ErrPaused
	}

	/* -------- cue có timing: json3 thô trước, rồi transcript đã lưu -------- */
	var cues []Cue
	var sourceTrack *CaptionTrack
	var rawSHA256 string
	captionHandle, err := upstreamArtifact(ctx, s, domain.CloneDownload, reCaptionArtifact)
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if captionHandle != nil {
		defer captionHandle.Release()
		captionArtifact := captionHandle.Path
		raw, err := os.ReadFile(captionArtifact)
		captionHandle.Release()
		if err != nil {
			return pipeline.StepResult{}, err
		}
		parsed, err := ParseTimedJson3(raw, sourceDurationMs)
		if err != nil {
			return pipeline.StepResult{}, &domain.CloneError{
				Code: domain.CloneErrInternal, Message: fmt.Sprintf("Tệp phụ đề json3 không đọc được: %v", err),
			}
		}
		cues = parsed.Cues
		rawSHA256 = parsed.RawSHA256
		warnings = append(warnings, parsed.Warnings...)
		trackKey := "unknown"
		if captionLanguage != nil {
			trackKey = *captionLanguage
		}
		trackKind := "unknown"
		if captionKind != nil {
			trackKind = *captionKind
		}
		trackPath := captionHandle.Ref.Path
		if trackPath == "" {
			for _, a := range s.Upstream[domain.CloneDownload].Artifacts {
				if reCaptionArtifact.MatchString(a.Path) {
					trackPath = a.Path
					break
				}
			}
		}
		sourceTrack = &CaptionTrack{Key: trackKey, Kind: trackKind, Path: trackPath}
	}
	if len(cues) == 0 && h.d.StoredSegments != nil {
		stored := h.d.StoredSegments(s.Job.Source.ID)
		if len(stored) > 0 {
			untimed := 0
			for i, st := range stored {
				if st.Start == nil {
					untimed++
				}
				startMs := 0.0
				if st.Start != nil {
					startMs = math.Round(*st.Start * 1000)
				}
				if strings.TrimSpace(st.Text) == "" {
					continue
				}
				cues = append(cues, Cue{
					ID: fmt.Sprintf("stored-%d", i), StartMs: startMs, EndMs: nil,
					Text: st.Text, Words: nil, Source: "captions",
				})
			}
			if untimed > 0 {
				warnings = append(warnings, CaptionWarning{
					Code: "untimed_source", Detail: fmt.Sprintf("%d stored cues carry no timestamps", untimed)})
			}
		}
	}

	/* -------- VAD độc lập trên audio phân tích -------- */
	var trStatus tools.TranscriptionStatus
	if h.d.TranscriptionStatus != nil {
		trStatus = h.d.TranscriptionStatus(ctx)
	}
	if trStatus.State != tools.TranscriptionReady {
		return pipeline.StepResult{}, &domain.CloneError{
			Code:     domain.CloneErrToolMissing,
			Message:  "Môi trường phiên âm chưa sẵn sàng",
			Recovery: new("Mở Cài đặt → Công cụ và cài môi trường phiên âm (faster-whisper)"),
			RetryAt:  new(time.UnixMilli(h.now() + 60_000)),
		}
	}
	runTranscribe := h.d.RunTranscribe
	if runTranscribe == nil {
		if h.d.ResolveUv == nil {
			return pipeline.StepResult{}, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "uv chưa sẵn sàng"}
		}
		uv := h.d.ResolveUv(ctx)
		if uv == "" {
			return pipeline.StepResult{}, &domain.CloneError{Code: domain.CloneErrToolMissing, Message: "uv chưa sẵn sàng"}
		}
		sidecar := SidecarRunner{
			Runner: runner, Python: trStatus.Python, Uv: uv, Env: h.d.RunEnv, Staging: s.StagingDir,
		}
		runTranscribe = sidecar.Run
	}
	runTranscribe = cp.transcribe(s, runTranscribe)
	h.progress(ctx, s, 0, i64(sourceDurationSec), "vad")
	vad, err := DetectSpeech(ctx, SpeechDeps{
		RunVad: func(ctx context.Context, o VadRun) (any, error) {
			return runTranscribe(ctx, TranscribeRun{
				Mode: "vad", InputWav: o.InputWav, Regions: o.Regions,
				WindowSec: o.WindowSec, ContextSec: o.ContextSec, ReportPath: o.ReportPath,
				OnProgress: func(done, total float64) {
					h.progress(ctx, s, int64(math.Round(done)), i64(total), "vad")
				},
			})
		},
	}, VadRun{InputWav: analysisWav, ReportPath: out("vad-report.json")})
	if err != nil {
		return pipeline.StepResult{}, err
	}
	speech := vad.Intervals
	if s.PauseRequested() {
		return pipeline.StepResult{}, pipeline.ErrPaused
	}

	/* -------- ASR trên mọi vùng speech cue để trống -------- */
	cueSpans := []Interval{}
	for _, c := range cues {
		if iv := CueInterval(c); iv != nil {
			cueSpans = append(cueSpans, *iv)
		}
	}
	gaps := UncoveredSpeech(speech, cueSpans, 350)
	// Cue không timing được probe ASR trên vùng hợp lý của nó (start → +20s).
	cueRegions := []Interval{}
	for _, c := range cues {
		if c.EndMs == nil && c.StartMs > 0 {
			cueRegions = append(cueRegions, Interval{
				StartMs: math.Max(0, c.StartMs-UntimedCueBackoffMs),
				EndMs:   math.Min(sourceDurationMs, c.StartMs+UntimedCueAsrMs),
			})
		}
	}
	asrRegions := MergeIntervals(append(gaps, cueRegions...), 200)

	var asrInfo *AsrInfo
	needsReview := []ReviewItem{}
	languageHint := ""
	if captionLanguage != nil {
		languageHint = strings.Split(*captionLanguage, "-")[0]
	}
	if len(asrRegions) > 0 && len(speech) > 0 {
		totalAsrSec := 0.0
		for _, r := range asrRegions {
			totalAsrSec += (r.EndMs - r.StartMs) / 1000
		}
		h.progress(ctx, s, 0, i64(totalAsrSec), "asr")
		reportPath := out("asr-report.json")
		rawReport, err := runTranscribe(ctx, TranscribeRun{
			Mode: "asr", InputWav: analysisWav, Regions: asrRegions,
			Language: languageHint, ModelSize: ASRModelSize, BeamSize: asrBeamSize,
			ReportPath: reportPath,
			OnProgress: func(done, total float64) {
				h.progress(ctx, s, int64(math.Round(done)), i64(total), "asr")
			},
		})
		if err != nil {
			return pipeline.StepResult{}, err
		}
		parsedAsr, err := parseAsrReport(rawReport)
		if err != nil {
			return pipeline.StepResult{}, &domain.CloneError{
				Code: domain.CloneErrInternal, Message: fmt.Sprintf("Bộ nhận giọng nói trả dữ liệu sai dạng: %v", err)}
		}
		merged := MergeAsrSegments(cues, parsedAsr.Segments, sourceDurationMs)
		cues = merged.Cues
		needsReview = append(needsReview, merged.NeedsReview...)
		var detected *AsrDetected
		if parsedAsr.Detected != nil {
			detected = &AsrDetected{Language: parsedAsr.Detected.Language, Probability: parsedAsr.Detected.Probability}
		}
		model := parsedAsr.Model
		asrInfo = &AsrInfo{
			Model: &model, Detected: detected,
			SegmentsAdded: merged.Added, SegmentsRejected: merged.Rejected,
			RegionsTranscribed: asrRegions, PeakRSSBytes: parsedAsr.PeakRSS,
		}
	}
	if s.PauseRequested() {
		return pipeline.StepResult{}, pipeline.ErrPaused
	}

	/* -------- anchor từng từ độc lập với cue nguồn -------- */
	anchorSpans := CaptionAlignmentSpans(cues, sourceDurationMs)
	var anchors *AsrAnchors
	if len(anchorSpans) > 0 && len(speech) > 0 {
		started := h.now()
		report, err := runTranscribe(ctx, TranscribeRun{
			Mode: "asr", InputWav: analysisWav, Regions: anchorSpans,
			ModelSize: AnchorModelSize, BeamSize: anchorBeamSize, Language: languageHint,
			ReportPath: out("anchor-report.json"),
			OnProgress: func(done, total float64) {
				h.progress(ctx, s, int64(math.Round(done)), i64(total), "align")
			},
		})
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, pipeline.ErrPaused) || errors.Is(err, pipeline.ErrFence) {
				return pipeline.StepResult{}, err
			}
			warnings = append(warnings, CaptionWarning{Code: "asr_anchors_missing", Detail: err.Error()})
		} else {
			parsed, perr := parseAsrReport(report)
			if perr != nil {
				warnings = append(warnings, CaptionWarning{Code: "asr_anchors_missing", Detail: perr.Error()})
			} else {
				audioSHA, herr := sha256File(analysisWav)
				if herr != nil {
					return pipeline.StepResult{}, herr
				}
				words := []AnchorWord{}
				for _, seg := range parsed.Segments {
					for _, w := range seg.Words {
						words = append(words, AnchorWord{
							Text: w.Text, StartMs: w.StartMs, EndMs: w.EndMs, Probability: w.Probability})
					}
				}
				sort.Slice(words, func(i, j int) bool { return words[i].StartMs < words[j].StartMs })
				anchors = &AsrAnchors{
					Version: 1, SourceAudioSHA256: audioSHA,
					Spans: anchorSpans, Words: words,
					Model: AnchorModel{
						Name: parsed.Model.Name, Version: parsed.Model.Version, Size: parsed.Model.Size,
						ComputeType: parsed.Model.ComputeType, Device: parsed.Model.Device,
					},
					GeneratedAt: epochISO(h.now()),
				}
				sorted := append([]Cue(nil), cues...)
				sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartMs < sorted[j].StartMs })
				for i, cue := range sorted {
					nextStart := math.MaxFloat64
					if i+1 < len(sorted) {
						nextStart = sorted[i+1].StartMs
					}
					sorted[i] = AlignCaptionWords(cue, anchors.Words, nextStart)
				}
				sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartMs < sorted[j].StartMs })
				for i := range len(sorted) - 1 {
					if sorted[i].EndMs != nil && *sorted[i].EndMs > sorted[i+1].StartMs {
						sorted[i].EndMs = new(sorted[i+1].StartMs)
					}
				}
				cues = sorted
				raw, werr := json.MarshalIndent(anchors, "", "  ")
				if werr != nil {
					return pipeline.StepResult{}, werr
				}
				if err := os.WriteFile(out("asr-anchors.json"), raw, 0o600); err != nil {
					return pipeline.StepResult{}, err
				}
				_ = started
			}
		}
	}

	/* -------- khe còn thiếu text có timing → review, khe thành thật -------- */
	finalSpans := []Interval{}
	for _, c := range cues {
		if iv := CueInterval(c); iv != nil {
			finalSpans = append(finalSpans, *iv)
		}
	}
	unresolved := UncoveredSpeech(speech, finalSpans, 350)
	for _, g := range unresolved {
		needsReview = append(needsReview, ReviewItem{
			CueID: "", Reason: fmt.Sprintf("speech %.1f–%.1fs has no timed caption", g.StartMs/1000, g.EndMs/1000)})
	}
	for _, c := range cues {
		if c.EndMs == nil {
			needsReview = append(needsReview, ReviewItem{CueID: c.ID, Reason: "cue has no end timing"})
		}
	}

	coverage := FrameCoverage(speech, finalSpans, CloneDefaultFPS)
	var language *string
	if asrInfo != nil && asrInfo.Detected != nil {
		language = new(asrInfo.Detected.Language)
	} else {
		language = captionLanguage
	}
	var rawSHA *string
	if rawSHA256 != "" {
		rawSHA = &rawSHA256
	}
	manifest := CaptionManifest{
		Version: 1, Revision: 1, Language: language, Cues: cues,
		SpeechIntervals: speech, VadEvidence: &vad.Evidence, Coverage: coverage,
		Source: CaptionSource{Track: sourceTrack, RawSHA256: rawSHA},
		Asr:    asrInfo, UnresolvedGaps: unresolved, NeedsReview: needsReview,
		Warnings: warnings, GeneratedAt: epochISO(h.now()),
	}

	h.progress(ctx, s, coverage.CoveredFrames, &coverage.SpeechFrames, "publish")
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return pipeline.StepResult{}, err
	}
	if err := os.WriteFile(out("captions.json"), manifestRaw, 0o600); err != nil {
		return pipeline.StepResult{}, err
	}
	if err := os.WriteFile(out("captions.srt"), []byte(CuesToSrt(cues)), 0o600); err != nil {
		return pipeline.StepResult{}, err
	}
	if err := os.WriteFile(out("captions.vtt"), []byte(CuesToVtt(cues)), 0o600); err != nil {
		return pipeline.StepResult{}, err
	}

	artifacts := []media.Artifact{
		{Path: "transcript/captions.json", File: out("captions.json"), ContentType: "application/json", Filename: "captions.json"},
		{Path: "transcript/captions.srt", File: out("captions.srt"), ContentType: "application/x-subrip", Filename: "captions.srt"},
		{Path: "transcript/captions.vtt", File: out("captions.vtt"), ContentType: "text/vtt", Filename: "captions.vtt"},
	}
	if anchors != nil {
		artifacts = append(artifacts, media.Artifact{
			Path: "transcript/asr-anchors.json", File: out("asr-anchors.json"),
			ContentType: "application/json", Filename: "asr-anchors.json"})
	}
	if _, err := os.Stat(out("vad-report.json")); err == nil {
		if err := copyFile(out("vad-report.json"), out("speech-report.json")); err != nil {
			return pipeline.StepResult{}, err
		}
		artifacts = append(artifacts, media.Artifact{
			Path: "transcript/speech-report.json", File: out("speech-report.json"),
			ContentType: "application/json", Filename: "speech-report.json"})
	}
	if asrInfo != nil {
		artifacts = append(artifacts, media.Artifact{
			Path: "transcript/asr-report.json", File: out("asr-report.json"),
			ContentType: "application/json", Filename: "asr-report.json"})
	}

	captionsSHA, err := sha256File(out("captions.json"))
	if err != nil {
		return pipeline.StepResult{}, err
	}
	var timedCount int64
	for _, c := range cues {
		if c.EndMs != nil {
			timedCount++
		}
	}
	var anchorWords, anchorSpansCount int64
	if anchors != nil {
		anchorWords = int64(len(anchors.Words))
		anchorSpansCount = int64(len(anchors.Spans))
	}
	return pipeline.StepResult{
		Artifacts: artifacts,
		Data: map[string]any{
			"revision":            manifest.Revision,
			"language":            language,
			"cueCount":            int64(len(cues)),
			"timedCueCount":       timedCount,
			"speechIntervalCount": int64(len(speech)),
			"speechCoverage":      coverage,
			"analysisAudio":       "mix",
			"captionsPath":        "transcript/captions.json",
			"captionsSha256":      captionsSHA,
			"needsReviewCount":    int64(len(needsReview)),
			"unresolvedGapCount":  int64(len(unresolved)),
			"anchorWordCount":     anchorWords,
			"anchorSpanCount":     anchorSpansCount,
		},
	}, nil
}

func (h *Handler) now() int64 {
	if h.d.Now != nil {
		return h.d.Now()
	}
	return time.Now().UnixMilli()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
