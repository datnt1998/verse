// Package quality owns the clone checklist and the single export-gate decision.
// Ported from yt-factory 2c201e6 src/lib/clone-quality.ts and export-gate.ts.
package quality

import (
	"fmt"
	"math"
)

const (
	MinScore               = 90
	MaxTotal               = 100
	DurationToleranceRatio = 0.1
	LoudnessToleranceLUFS  = 1
	CutToleranceMaxMS      = 100
	CutToleranceMaxFrames  = 2
	RequiredWidth          = 1920
	RequiredHeight         = 1080
	RequiredFPS            = 30
	ThumbnailWidth         = 1280
	ThumbnailHeight        = 720
)

var IDs = []string{"duration", "sceneCount", "cutRhythm", "images", "captions", "loudness", "resolution1080p", "thumbnail", "captionAlignment", "captionKaraoke", "background"}
var Weights = map[string]float64{"duration": 15, "sceneCount": 10, "cutRhythm": 15, "images": 20, "captions": 20, "loudness": 10, "resolution1080p": 5, "thumbnail": 5, "captionAlignment": 0, "captionKaraoke": 0, "background": 0}
var Labels = map[string]string{"duration": "Thời lượng", "sceneCount": "Số cảnh", "cutRhythm": "Nhịp cắt", "images": "Ảnh tất cả cảnh", "captions": "Phụ đề phủ lời thoại", "loudness": "Âm lượng", "resolution1080p": "1080p", "thumbnail": "Ảnh bìa", "captionAlignment": "Phụ đề khớp tiếng", "captionKaraoke": "Phụ đề karaoke", "background": "Nền video"}
var Fixes = map[string]string{"duration": "Kéo/cắt clip để thời lượng trong ±10% video gốc", "sceneCount": "Dựng lại video hoặc sửa số ảnh cảnh cho khớp nguồn", "cutRhythm": "Đặt mép ảnh cảnh đúng mốc đã duyệt", "images": "Duyệt đủ ảnh và giữ clip ảnh hiển thị đủ cảnh", "captions": "Hiện lớp phụ đề và phủ mọi khoảng đang nói", "loudness": "Đo lại hoặc đưa âm lượng về bằng video gốc", "resolution1080p": "Dùng cài đặt 1920×1080 · 30 fps, xuất cả video", "thumbnail": "Duyệt ảnh bìa WebP 1280×720 hiện hành", "captionAlignment": "Chỉnh mốc từng từ trong Biên tập theo các câu lệch", "captionKaraoke": "Đo mốc từng từ để giảm các trang ước lượng", "background": "Dựng lại video — các clip nền phải phủ kín toàn bộ video"}

type FixTarget struct {
	ItemID  string `json:"itemId,omitempty"`
	Frame   *int   `json:"frame,omitempty"`
	Href    string `json:"href,omitempty"`
	MediaID string `json:"mediaId,omitempty"`
}
type Item struct {
	ID       string     `json:"id"`
	Status   string     `json:"status"`
	Points   float64    `json:"points"`
	Weight   float64    `json:"weight"`
	Measured any        `json:"measured,omitempty"`
	Expected string     `json:"expected,omitempty"`
	Evidence string     `json:"evidence,omitempty"`
	Fix      *FixTarget `json:"fix,omitempty"`
}
type Blocker struct {
	ID      string     `json:"id"`
	Message string     `json:"message"`
	Fix     *FixTarget `json:"fix,omitempty"`
}
type Binding struct {
	ProjectRevision string `json:"projectRevision"`
	SettingsHash    string `json:"settingsHash"`
	ArtifactHash    string `json:"artifactHash"`
}
type Score struct {
	Version       int       `json:"version"`
	Total         float64   `json:"total"`
	Passed        bool      `json:"passed"`
	Items         []Item    `json:"items"`
	Blockers      []Blocker `json:"blockers"`
	Binding       Binding   `json:"binding"`
	MeasurementID *string   `json:"measurementId"`
	MeasuredAt    string    `json:"measuredAt"`
	Generation    int       `json:"generation"`
}
type GateDecision struct {
	Score    *Score   `json:"score"`
	Allowed  bool     `json:"allowed"`
	Fresh    bool     `json:"fresh"`
	Accepted bool     `json:"accepted"`
	Reason   *string  `json:"reason"`
	Reasons  []string `json:"reasons"`
}
type Range struct {
	Start     float64  `json:"start"`
	End       float64  `json:"end"`
	CheckID   string   `json:"checkId"`
	Status    string   `json:"status"`
	CueID     string   `json:"cueId,omitempty"`
	ItemID    string   `json:"itemId,omitempty"`
	Text      string   `json:"text,omitempty"`
	OffsetSec *float64 `json:"offsetSec,omitempty"`
}

func Total(items []Item) float64 {
	var n float64
	for _, item := range items {
		if item.Status == "pass" {
			n += item.Weight
		}
	}
	return n
}
func AllowsExport(score *Score) bool {
	if score == nil || score.Total < MinScore || math.IsNaN(score.Total) || math.IsInf(score.Total, 0) || len(score.Blockers) > 0 {
		return false
	}
	for _, item := range score.Items {
		if item.Status == "unknown" {
			return false
		}
	}
	return true
}
func CutToleranceMS(fps float64) float64 {
	return math.Max(CutToleranceMaxMS, CutToleranceMaxFrames/fps*1000)
}
func GateReasons(score *Score) []string {
	out := []string{}
	if score == nil || AllowsExport(score) {
		return out
	}
	blocked := map[string]bool{}
	for _, b := range score.Blockers {
		blocked[b.ID] = true
		out = append(out, b.Message)
	}
	if score.Total < MinScore {
		out = append(out, fmt.Sprintf("Điểm %g/100 chưa đạt %d", score.Total, MinScore))
	}
	for _, item := range score.Items {
		if blocked[item.ID] || item.Status == "pass" {
			continue
		}
		why := item.Evidence
		if why == "" {
			why = "không đạt"
		}
		if item.Status == "unknown" {
			why = "chưa đo được"
		}
		out = append(out, Labels[item.ID]+": "+why)
	}
	return out
}
func SameBinding(a, b Binding) bool { return a == b }
func EvaluateGate(score *Score, binding *Binding, accepted bool) GateDecision {
	var reason string
	switch {
	case score == nil:
		reason = "Chưa có điểm Độ hoàn thiện — chạy đo trước"
	case binding == nil:
		reason = "Không đọc được project/binding"
	case score.Binding.ProjectRevision != binding.ProjectRevision:
		reason = "Project đã sửa sau lần đo — đo lại Độ hoàn thiện"
	case score.Binding.SettingsHash != binding.SettingsHash:
		reason = "Cài đặt canvas/fps đã đổi sau lần đo"
	case score.Binding.ArtifactHash != binding.ArtifactHash:
		reason = "Ảnh/phụ đề/âm thanh đã thay đổi sau lần đo"
	}
	d := GateDecision{Score: score, Fresh: reason == "", Accepted: accepted, Reasons: GateReasons(score)}
	if reason != "" {
		d.Reason = &reason
	}
	d.Allowed = accepted || (d.Fresh && AllowsExport(score))
	return d
}
func (s Score) Validate() error {
	if s.Version != 1 || s.Generation < 0 || math.IsNaN(s.Total) || math.IsInf(s.Total, 0) {
		return fmt.Errorf("invalid quality score")
	}
	for _, i := range s.Items {
		if _, ok := Weights[i.ID]; !ok || (i.Status != "pass" && i.Status != "fail" && i.Status != "unknown") {
			return fmt.Errorf("invalid quality item")
		}
	}
	return nil
}
