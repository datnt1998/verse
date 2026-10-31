// Phân tích lời thoại của bước phụ đề: Silero VAD trên audio nguồn trong cửa
// sổ có biên, trộn khoảng, độ phủ khung hình trên cue có timing, và quyết định
// đoạn speech nào cần ASR cục bộ. Chuyển 1:1 từ
// yt-factory src/server/clone/speech-analysis.ts (2c201e6).
//
// Process VAD/ASR chạy trong môi trường transcription do app quản qua
// scripts/clone-transcribe.py; file này sở hữu hợp đồng request/response (kiểm
// ở biên), toán trộn khoảng và chỉ số độ phủ. Khoảng speech không bao giờ đến
// từ cue phụ đề — VAD là bộ dò độc lập.
package transcript

import (
	"context"
	"encoding/json"
	"fmt"
)

// ParseAdapterEvent parse một dòng stderr JSONL của adapter; nil với dòng nhiễu.
func ParseAdapterEvent(line string) map[string]any {
	trimmed := ""
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' && line[i] != '\r' && line[i] != '\n' {
			trimmed = line[i:]
			break
		}
	}
	if trimmed == "" || trimmed[0] != '{' {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil
	}
	if _, ok := parsed["event"].(string); !ok {
		return nil
	}
	return parsed
}

/* --------------------- schema report của adapter --------------------- */

type modelCheckpoint struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// VadModel mô tả bộ dò trong report vad.
type VadModel struct {
	Name       string           `json:"name"`
	Version    string           `json:"version"`
	Checkpoint *modelCheckpoint `json:"checkpoint"`
}

// VadReportFile là JSON `--report` mà adapter ghi ở mode vad.
type VadReportFile struct {
	Event       string             `json:"event"`
	Mode        string             `json:"mode"`
	Model       VadModel           `json:"model"`
	Params      map[string]float64 `json:"params"`
	SampleRate  float64            `json:"sampleRate"`
	WindowSec   float64            `json:"windowSec"`
	ContextSec  float64            `json:"contextSec"`
	DurationSec float64            `json:"durationSec"`
	Intervals   []Interval         `json:"intervals"`
	PeakRSS     float64            `json:"peakRssBytes"`
}

// AsrModel mô tả bộ nhận dạng trong report asr.
type AsrModel struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Size        string `json:"size"`
	ComputeType string `json:"computeType"`
	Device      string `json:"device"`
}

// AsrWord là một từ do ASR đo.
type AsrWord struct {
	StartMs     float64  `json:"startMs"`
	EndMs       float64  `json:"endMs"`
	Text        string   `json:"text"`
	Probability *float64 `json:"probability,omitempty"`
}

// AsrSegment là một đoạn thoại do ASR trả.
type AsrSegment struct {
	StartMs    float64   `json:"startMs"`
	EndMs      float64   `json:"endMs"`
	Text       string    `json:"text"`
	AvgLogprob *float64  `json:"avgLogprob"`
	Words      []AsrWord `json:"words"`
}

type asrDetected struct {
	Language    string  `json:"language"`
	Probability float64 `json:"probability"`
}

// AsrReportFile là JSON `--report` mà adapter ghi ở mode asr.
type AsrReportFile struct {
	Event           string       `json:"event"`
	Mode            string       `json:"mode"`
	Model           AsrModel     `json:"model"`
	LanguageHint    *string      `json:"languageHint"`
	Detected        *asrDetected `json:"detected"`
	SliceSec        float64      `json:"sliceSec"`
	SliceOverlapSec float64      `json:"sliceOverlapSec"`
	Segments        []AsrSegment `json:"segments"`
	PeakRSS         float64      `json:"peakRssBytes"`
}

// num đọc một số thực từ map report; sai kiểu/trống → lỗi schema.
func num(m map[string]any, key string) (float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("%s required", key)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("%s not a number", key)
	}
}

func str(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", fmt.Errorf("%s required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s not a string", key)
	}
	return s, nil
}

func params(m map[string]any, key string) (map[string]float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("%s required", key)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s not an object", key)
	}
	out := map[string]float64{}
	for k := range obj {
		f, err := num(obj, k)
		if err != nil {
			return nil, fmt.Errorf("%s.%s not a number", key, k)
		}
		out[k] = f
	}
	return out, nil
}

// parseVadReport kiểm report vad ở biên (tương đương vadReportSchema.safeParse).
func parseVadReport(raw any) (VadReportFile, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return VadReportFile{}, fmt.Errorf("report not an object")
	}
	var r VadReportFile
	var err error
	fail := func(e error) (VadReportFile, error) { return VadReportFile{}, e }
	if r.Event, err = str(m, "event"); err != nil || r.Event != "done" {
		return fail(fmt.Errorf("event must be done"))
	}
	if r.Mode, err = str(m, "mode"); err != nil || r.Mode != "vad" {
		return fail(fmt.Errorf("mode must be vad"))
	}
	model, ok := m["model"].(map[string]any)
	if !ok {
		return fail(fmt.Errorf("model required"))
	}
	if r.Model.Name, err = str(model, "name"); err != nil {
		return fail(err)
	}
	if r.Model.Version, err = str(model, "version"); err != nil {
		return fail(err)
	}
	if value, exists := model["checkpoint"]; !exists {
		return fail(fmt.Errorf("checkpoint required"))
	} else if value != nil {
		if _, ok := value.(map[string]any); !ok {
			return fail(fmt.Errorf("checkpoint not an object"))
		}
	}
	if cp, ok := model["checkpoint"].(map[string]any); ok {
		c := modelCheckpoint{}
		if c.File, err = str(cp, "file"); err != nil {
			return fail(err)
		}
		if c.SHA256, err = str(cp, "sha256"); err != nil {
			return fail(err)
		}
		r.Model.Checkpoint = &c
	}
	if r.Params, err = params(m, "params"); err != nil {
		return fail(err)
	}
	if r.SampleRate, err = num(m, "sampleRate"); err != nil {
		return fail(err)
	}
	if r.WindowSec, err = num(m, "windowSec"); err != nil {
		return fail(err)
	}
	if r.ContextSec, err = num(m, "contextSec"); err != nil {
		return fail(err)
	}
	if r.DurationSec, err = num(m, "durationSec"); err != nil {
		return fail(err)
	}
	rawIntervals, ok := m["intervals"].([]any)
	if !ok {
		return fail(fmt.Errorf("intervals required"))
	}
	for i, iv := range rawIntervals {
		obj, ok := iv.(map[string]any)
		if !ok {
			return fail(fmt.Errorf("intervals[%d] not an object", i))
		}
		s, e1 := num(obj, "startMs")
		e2, e2err := num(obj, "endMs")
		if e1 != nil || e2err != nil {
			return fail(fmt.Errorf("intervals[%d] malformed", i))
		}
		r.Intervals = append(r.Intervals, Interval{s, e2})
	}
	if r.PeakRSS, err = num(m, "peakRssBytes"); err != nil {
		return fail(err)
	}
	return r, nil
}

// parseAsrReport kiểm report asr ở biên (tương đương asrReportSchema.safeParse).
func parseAsrReport(raw any) (AsrReportFile, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return AsrReportFile{}, fmt.Errorf("report not an object")
	}
	var r AsrReportFile
	var err error
	fail := func(e error) (AsrReportFile, error) { return AsrReportFile{}, e }
	if r.Event, err = str(m, "event"); err != nil || r.Event != "done" {
		return fail(fmt.Errorf("event must be done"))
	}
	if r.Mode, err = str(m, "mode"); err != nil || r.Mode != "asr" {
		return fail(fmt.Errorf("mode must be asr"))
	}
	model, ok := m["model"].(map[string]any)
	if !ok {
		return fail(fmt.Errorf("model required"))
	}
	for _, k := range []string{"name", "version", "size", "computeType", "device"} {
		v, e := str(model, k)
		if e != nil {
			return fail(e)
		}
		switch k {
		case "name":
			r.Model.Name = v
		case "version":
			r.Model.Version = v
		case "size":
			r.Model.Size = v
		case "computeType":
			r.Model.ComputeType = v
		case "device":
			r.Model.Device = v
		}
	}
	if hint, ok := m["languageHint"].(string); ok {
		r.LanguageHint = &hint
	}
	if det, ok := m["detected"].(map[string]any); ok {
		d := asrDetected{}
		if d.Language, err = str(det, "language"); err != nil {
			return fail(err)
		}
		if d.Probability, err = num(det, "probability"); err != nil {
			return fail(err)
		}
		r.Detected = &d
	}
	if r.SliceSec, err = num(m, "sliceSec"); err != nil {
		return fail(err)
	}
	if r.SliceOverlapSec, err = num(m, "sliceOverlapSec"); err != nil {
		return fail(err)
	}
	rawSegs, ok := m["segments"].([]any)
	if !ok {
		return fail(fmt.Errorf("segments required"))
	}
	for i, s := range rawSegs {
		obj, ok := s.(map[string]any)
		if !ok {
			return fail(fmt.Errorf("segments[%d] not an object", i))
		}
		var seg AsrSegment
		if seg.StartMs, err = num(obj, "startMs"); err != nil {
			return fail(fmt.Errorf("segments[%d].startMs: %w", i, err))
		}
		if seg.EndMs, err = num(obj, "endMs"); err != nil {
			return fail(fmt.Errorf("segments[%d].endMs: %w", i, err))
		}
		if seg.Text, err = str(obj, "text"); err != nil {
			return fail(fmt.Errorf("segments[%d].text: %w", i, err))
		}
		if lp, ok := obj["avgLogprob"].(float64); ok {
			seg.AvgLogprob = &lp
		}
		words, ok := obj["words"].([]any)
		if !ok {
			return fail(fmt.Errorf("segments[%d].words required", i))
		}
		for j, w := range words {
			wobj, ok := w.(map[string]any)
			if !ok {
				return fail(fmt.Errorf("segments[%d].words[%d] malformed", i, j))
			}
			var aw AsrWord
			if aw.StartMs, err = num(wobj, "startMs"); err != nil {
				return fail(fmt.Errorf("words[%d].startMs: %w", j, err))
			}
			if aw.EndMs, err = num(wobj, "endMs"); err != nil {
				return fail(fmt.Errorf("words[%d].endMs: %w", j, err))
			}
			if aw.Text, err = str(wobj, "text"); err != nil {
				return fail(fmt.Errorf("words[%d].text: %w", j, err))
			}
			if p, ok := wobj["probability"].(float64); ok {
				aw.Probability = &p
			}
			seg.Words = append(seg.Words, aw)
		}
		r.Segments = append(r.Segments, seg)
	}
	if r.PeakRSS, err = num(m, "peakRssBytes"); err != nil {
		return fail(err)
	}
	return r, nil
}

/* ------------------------------ detectSpeech ------------------------------ */

// VadEvidence là bằng chứng detector ghi vào manifest — version+config model
// là một phần bằng chứng.
type VadEvidence struct {
	Model         VadModel           `json:"model"`
	Params        map[string]float64 `json:"params"`
	SampleRate    float64            `json:"sampleRate"`
	WindowSec     float64            `json:"windowSec"`
	ContextSec    float64            `json:"contextSec"`
	DurationSec   float64            `json:"durationSec"`
	IntervalCount int64              `json:"intervalCount"`
	PeakRSSBytes  float64            `json:"peakRssBytes"`
}

// VadReport là kết quả detectSpeech: khoảng đã trộn + bằng chứng.
type VadReport struct {
	Intervals []Interval
	Evidence  VadEvidence
}

// VadRun là tham số một lượt `clone-transcribe.py --mode vad`.
type VadRun struct {
	InputWav   string
	Regions    []Interval
	WindowSec  float64
	ContextSec float64
	ReportPath string
}

// SpeechDeps gói phụ thuộc detectSpeech — RunVad chạy adapter và trả report
// đã parse thành map; test thay bằng fixture.
type SpeechDeps struct {
	RunVad func(ctx context.Context, run VadRun) (any, error)
}

const (
	// VADWindowSec là kích thước cửa sổ Silero.
	VADWindowSec = 30.0
	// VADContextSec là ngữ cảnh thêm mỗi bên cửa sổ.
	VADContextSec = 2.0
)

// DetectSpeech dò khoảng lời thoại trên trục nguồn. `Regions` giới hạn vùng dò
// (ví dụ các vùng caption-gap); khoảng trộn theo chính sách gap nhỏ của detector.
func DetectSpeech(ctx context.Context, deps SpeechDeps, run VadRun) (VadReport, error) {
	if run.WindowSec <= 0 {
		run.WindowSec = VADWindowSec
	}
	if run.ContextSec <= 0 {
		run.ContextSec = VADContextSec
	}
	rawReport, err := deps.RunVad(ctx, run)
	if err != nil {
		return VadReport{}, err
	}
	report, err := parseVadReport(rawReport)
	if err != nil {
		return VadReport{}, fmt.Errorf("VAD adapter report failed schema: %w", err)
	}
	filtered := []Interval{}
	for _, iv := range report.Intervals {
		if iv.EndMs > iv.StartMs {
			filtered = append(filtered, iv)
		}
	}
	mergeGap := report.Params["merge_gap_ms"]
	if mergeGap == 0 {
		mergeGap = 32
	}
	intervals := MergeIntervals(filtered, mergeGap)
	return VadReport{
		Intervals: intervals,
		Evidence: VadEvidence{
			Model: report.Model, Params: report.Params, SampleRate: report.SampleRate,
			WindowSec: run.WindowSec, ContextSec: run.ContextSec, DurationSec: report.DurationSec,
			IntervalCount: int64(len(intervals)), PeakRSSBytes: report.PeakRSS,
		},
	}, nil
}

// UncoveredSpeech trả vùng speech còn thiếu bằng chứng: các đoạn không có cue
// trong khoảng speech đã dò, mỗi đoạn đệm theo dung sai mép dùng khi khớp cue
// với speech (timing cue mơ hồ ở biên).
func UncoveredSpeech(speech, timedCueSpans []Interval, edgePadMs float64) []Interval {
	padded := []Interval{}
	for _, s := range timedCueSpans {
		padded = append(padded, Interval{s.StartMs - edgePadMs, s.EndMs + edgePadMs})
	}
	covered := MergeIntervals(padded, 0)
	gaps := []Interval{}
	for _, iv := range MergeIntervals(speech, 0) {
		cursor := iv.StartMs
		for _, c := range covered {
			if c.EndMs <= cursor {
				continue
			}
			if c.StartMs >= iv.EndMs {
				break
			}
			if c.StartMs > cursor {
				gaps = append(gaps, Interval{cursor, c.StartMs})
			}
			cursor = max(cursor, c.EndMs)
			if cursor >= iv.EndMs {
				break
			}
		}
		if cursor < iv.EndMs {
			gaps = append(gaps, Interval{cursor, iv.EndMs})
		}
	}
	// Dưới 40ms một "khe" chỉ là nhiễu lượng tử khung hình, không phải bằng
	// chứng thiếu.
	out := gaps[:0]
	for _, g := range gaps {
		if g.EndMs-g.StartMs >= 40 {
			out = append(out, g)
		}
	}
	return out
}
