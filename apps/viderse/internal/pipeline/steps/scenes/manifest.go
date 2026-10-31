package scenes

import (
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps"
	"math"
)

type FileRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Scene struct {
	SceneID       string       `json:"sceneId"`
	StartFrame    int          `json:"startFrame"`
	EndFrame      int          `json:"endFrame"`
	Text          string       `json:"text"`
	Keyframe      FileRef      `json:"keyframe"`
	KeyframeFrame int          `json:"keyframeFrame"`
	KeyframePTS   float64      `json:"keyframePtsSec"`
	KeyframeStale bool         `json:"keyframeStale,omitempty"`
	CaptionCueIDs []string     `json:"captionCueIds"`
	TextSource    string       `json:"textSource,omitempty"`
	Context       *TextContext `json:"context"`
}
type Recipe struct {
	Style        string `json:"style"`
	ImagesMode   string `json:"imagesMode"`
	ImageCount   int    `json:"imageCount"`
	Thumbnail    string `json:"thumbnail"`
	ImagePrompt  string `json:"imagePrompt,omitempty"`
	CoverPrompt  string `json:"coverPrompt,omitempty"`
	CoverMediaID string `json:"coverMediaId,omitempty"`
}
type Boundary struct {
	Frame   int     `json:"frame"`
	TimeSec float64 `json:"timeSec"`
	Score   float64 `json:"score"`
}
type Detection struct {
	Method         string     `json:"method"`
	Threshold      float64    `json:"threshold"`
	MinDurationSec float64    `json:"minDurationSec"`
	Boundaries     []Boundary `json:"boundaries"`
	Dropped        []Dropped  `json:"dropped"`
	RawFrames      int        `json:"rawFrames"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	Rotation       float64    `json:"rotation"`
	VariableFPS    bool       `json:"variableFps"`
}
type Analysis struct {
	Kind        string `json:"kind"`
	Fingerprint string `json:"fingerprint"`
	GeneratedAt string `json:"generatedAt"`
}
type Approval struct {
	Fingerprint string `json:"fingerprint"`
	ApprovedAt  string `json:"approvedAt"`
}
type Manifest struct {
	Version     int       `json:"version"`
	Revision    int       `json:"revision"`
	FPS         int       `json:"fps"`
	TotalFrames int       `json:"totalFrames"`
	DurationSec float64   `json:"durationSec"`
	Source      FileRef   `json:"source"`
	Recipe      Recipe    `json:"recipe"`
	Detection   Detection `json:"detection"`
	Pending     *Analysis `json:"pendingAnalysis"`
	Approval    *Approval `json:"approval"`
	Scenes      []Scene   `json:"scenes"`
}

func (m Manifest) Validate() error {
	if m.Version != 1 || m.Revision < 1 || m.FPS != 30 || m.TotalFrames < 1 || math.IsNaN(m.DurationSec) || m.DurationSec <= 0 || len(m.Scenes) == 0 {
		return fmt.Errorf("invalid scene manifest")
	}
	next := 0
	ids := map[string]bool{}
	for _, s := range m.Scenes {
		if ids[s.SceneID] || s.SceneID == "" || s.StartFrame != next || s.EndFrame <= s.StartFrame || s.EndFrame > m.TotalFrames || s.KeyframeFrame < s.StartFrame || s.KeyframeFrame >= s.EndFrame {
			return fmt.Errorf("scene spans do not tile the source")
		}
		ids[s.SceneID] = true
		next = s.EndFrame
	}
	if next != m.TotalFrames {
		return fmt.Errorf("scene spans do not tile the source")
	}
	return nil
}
func AnalysisFingerprint(m Manifest) string {
	return steps.InputFingerprint(struct {
		Source FileRef
		Recipe Recipe
		Scenes []Scene
	}{m.Source, m.Recipe, m.Scenes})
}
func Approve(m Manifest, fingerprint, now string) (Manifest, error) {
	if m.Pending == nil || m.Pending.Fingerprint != fingerprint {
		return m, fmt.Errorf("stale scene analysis")
	}
	m.Approval = &Approval{Fingerprint: fingerprint, ApprovedAt: now}
	m.Pending = nil
	m.Revision++
	return m, nil
}
