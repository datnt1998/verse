package build

import "github.com/datnt1998/verse/apps/viderse/internal/editor"

// Manifest records the actual fill inputs and identities, rather than letting a
// scorer guess which authored clip represents a source scene or voice track.
// The shape follows yt-factory build.ts at 2c201e6.
type Manifest struct {
	Version           int                       `json:"version"`
	GeneratedAt       string                    `json:"generatedAt"`
	ProjectID         string                    `json:"projectId"`
	Generation        int                       `json:"generation"`
	BaseRevision      string                    `json:"baseRevision"`
	CommittedRevision string                    `json:"committedRevision"`
	RecipeFingerprint string                    `json:"recipeFingerprint"`
	ItemIDs           editor.FillItemIDs        `json:"itemIds"`
	Audio             BuildAudio                `json:"audio"`
	Source            ManifestMedia             `json:"source"`
	Envelope          editor.Object             `json:"envelope"`
	Caption           editor.FillCaptionSummary `json:"caption"`
	Thumbnail         ManifestMedia             `json:"thumbnail"`
	Scenes            []editor.FillScene        `json:"scenes"`
	CommandCount      int                       `json:"commandCount"`
}
type ManifestMedia struct {
	MediaID string `json:"mediaId,omitempty"`
	SHA256  string `json:"sha256"`
}
type BuildAudio struct {
	MediaID            string   `json:"mediaId"`
	SourceLUFS         float64  `json:"sourceLufs"`
	ReferenceVersion   int      `json:"referenceVersion"`
	DurationInFrames   int      `json:"durationInFrames"`
	SHA256             string   `json:"sha256"`
	LoudnessTargetLUFS *float64 `json:"loudnessTargetLufs,omitempty"`
	AppliedGainDB      *float64 `json:"appliedGainDb,omitempty"`
}
