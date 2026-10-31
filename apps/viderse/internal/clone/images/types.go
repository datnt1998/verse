// Package images exposes persisted owner edits to the clone image pipeline.
package images

import (
	"context"
	"encoding/json"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/scenes"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

type PromptCatalog interface {
	GetPrompt(context.Context, string) (*domain.ImagePrompt, error)
}
type FfmpegResolver interface {
	Resolve(context.Context) *tools.FfmpegResolved
}

// Learner nhận góp ý vẽ lại của chủ kênh để Hoạ sĩ học (07 §19). Cả hai hàm
// trả đề xuất learn_rule hoặc nil; lỗi của Learner không làm hỏng lần vẽ lại.
type Learner interface {
	ObserveRedraw(ctx context.Context, jobID, channelID, imagePromptID, promptName, feedback string) (any, error)
	OpenFor(ctx context.Context, jobID string) (any, error)
}
type Service struct {
	Learner   Learner
	Edits     *jobedit.Service
	Prompts   PromptCatalog
	QuotaLeft func(context.Context) (int, error)
	Ffmpeg    FfmpegResolver
	Runner    tools.Runner
	// Autonomy đọc Mức tự chủ của kênh để tóm tắt điểm; nil → không tóm tắt.
	Autonomy interface {
		JobAutonomy(ctx context.Context, jobID string) (domain.Autonomy, bool, error)
	}
}
type RevisionInput struct {
	ExpectedRevision *int `json:"expectedRevision"`
	ApproveExport    bool `json:"-"`
}
type Action struct {
	ID      string  `json:"id"`
	Op      string  `json:"op"`
	Version *int    `json:"version,omitempty"`
	Prompt  *string `json:"prompt,omitempty"`
}
type ReviewInput struct {
	ExpectedRevision *int     `json:"expectedRevision"`
	Actions          []Action `json:"actions"`
}
type RedrawInput struct {
	ExpectedRevision *int    `json:"expectedRevision"`
	Prompt           *string `json:"prompt,omitempty"`
}
type ResetInput struct {
	ExpectedRevision *int   `json:"expectedRevision"`
	ConfirmCount     int    `json:"confirmCount"`
	Kind             string `json:"kind,omitempty"`
	Feedback         string `json:"feedback,omitempty"`
}
type MutationResult struct {
	JobID    string  `json:"jobId"`
	Revision int     `json:"revision"`
	RunID    *string `json:"runId"`
}
type CoverResult struct {
	JobID    string  `json:"jobId"`
	Revision int     `json:"revision"`
	Version  int     `json:"version"`
	RunID    *string `json:"runId"`
}
type GenerateResult struct {
	Accepted bool     `json:"accepted"`
	JobID    string   `json:"jobId"`
	RunID    *string  `json:"runId"`
	Pending  []string `json:"pending"`
	Count    int      `json:"count,omitempty"`
	ItemID   string   `json:"itemId,omitempty"`
	// Learn là đề xuất “Thêm vào Ghi chú đã học” khi góp ý vừa lặp lần thứ 2.
	Learn any `json:"learn"`
}
type ReviewResult struct {
	Manifest struct {
		Revision int `json:"revision"`
	} `json:"manifest"`
	Approved      bool            `json:"approved"`
	ChosenChanged bool            `json:"chosenChanged"`
	Invalidated   bool            `json:"invalidated"`
	Kicked        bool            `json:"kicked"`
	Generation    *GenerateResult `json:"generation"`
}
type EffectivePrompt struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Overridden bool   `json:"overridden"`
}
type ManifestView struct {
	Revision        int                `json:"revision"`
	SceneGeneration int                `json:"sceneGeneration"`
	SceneRevision   int                `json:"sceneRevision"`
	Waiting         *domain.CloneError `json:"waiting"`
	ImageMode       *string            `json:"imageMode"`
	ScenePrompt     EffectivePrompt    `json:"scenePrompt"`
	CoverPrompt     EffectivePrompt    `json:"coverPrompt"`
	Overrides       *model.Overrides   `json:"overrides"`
}
type VersionView struct {
	Version           int             `json:"version"`
	Path              string          `json:"path"`
	SHA256            string          `json:"sha256"`
	Width             int             `json:"width"`
	Height            int             `json:"height"`
	Bytes             int64           `json:"bytes"`
	CreatedAt         string          `json:"createdAt"`
	PromptVersion     int             `json:"promptVersion"`
	SourceFingerprint string          `json:"sourceFingerprint"`
	Source            string          `json:"source"`
	Cutout            *scenes.FileRef `json:"cutout"`
	OriginalURL       string          `json:"originalUrl,omitempty"`
	CutoutURL         string          `json:"cutoutUrl,omitempty"`
	CutoutState       string          `json:"cutoutState"`
	// Score là điểm Kiểm định của version này (nil khi chưa chấm được).
	Score *domain.ImageScore `json:"score"`
}
type ItemView struct {
	ID             string        `json:"id"`
	Kind           string        `json:"kind"`
	Status         string        `json:"status"`
	SkipReason     string        `json:"skipReason,omitempty"`
	Prompt         string        `json:"prompt"`
	PromptVersion  int           `json:"promptVersion"`
	CurrentVersion *int          `json:"currentVersion"`
	ReviewVersion  *int          `json:"reviewVersion"`
	QueuePosition  *int          `json:"queuePosition"`
	Reference      string        `json:"reference"`
	ReferenceURL   string        `json:"referenceUrl,omitempty"`
	Width          int           `json:"width"`
	Height         int           `json:"height"`
	Versions       []VersionView `json:"versions"`
	SceneID        string        `json:"sceneId,omitempty"`
	StartFrame     int           `json:"startFrame,omitempty"`
	EndFrame       int           `json:"endFrame,omitempty"`
	StartSec       *float64      `json:"startSec,omitempty"`
	EndSec         *float64      `json:"endSec,omitempty"`
	DurationSec    *float64      `json:"durationSec,omitempty"`
	Narration      string        `json:"narration,omitempty"`
	Direction      string        `json:"direction,omitempty"`
	MediaID        string        `json:"mediaId,omitempty"`
	// Score là điểm của ảnh đang hiển thị (đề cử đang xét hoặc ảnh đã chọn).
	Score *domain.ImageScore `json:"score"`
}

// AutonomyView tóm tắt điểm so với ngưỡng Duyệt theo điểm của kênh — dòng
// "cảnh 4 được 72" của Chờ duyệt. Luôn có khi video có ảnh.
type AutonomyView struct {
	Mode    string               `json:"mode"`
	Min     int                  `json:"min"`
	Total   int                  `json:"total"`
	Passed  int                  `json:"passed"`
	Pending int                  `json:"pending"`
	Below   []autonomy.BelowItem `json:"below"`
	Summary string               `json:"summary"`
}
type View struct {
	Autonomy      *AutonomyView `json:"autonomy"`
	Manifest      *ManifestView `json:"manifest"`
	Items         []ItemView    `json:"items"`
	Approved      bool          `json:"approved"`
	ApprovedToday *int          `json:"approvedToday"`
	Reason        string        `json:"reason,omitempty"`
	// Learn: đề xuất “Thêm vào Ghi chú đã học” đang mở mà video này góp phần.
	Learn any `json:"learn"`
}

// OptionalString distinguishes omission (retain override) from null (template).
type OptionalString struct {
	Set   bool
	Value *string
}

func (o *OptionalString) UnmarshalJSON(raw []byte) error {
	o.Set = true
	return json.Unmarshal(raw, &o.Value)
}

type OverrideInput struct {
	ExpectedRevision *int           `json:"expectedRevision"`
	ScenePromptID    OptionalString `json:"scenePromptId"`
	CoverPromptID    OptionalString `json:"coverPromptId"`
	NoImages         *bool          `json:"noImages,omitempty"`
	ConfirmCount     *int           `json:"confirmCount,omitempty"`
}
type OverridePlan struct {
	Redraw struct {
		Scenes int  `json:"scenes"`
		Cover  bool `json:"cover"`
	} `json:"redraw"`
	Cost struct {
		Images int `json:"images"`
	} `json:"cost"`
	QuotaLeft int  `json:"quotaLeft"`
	Rebuild   bool `json:"rebuild"`
}

type loaded struct {
	snapshot jobedit.Snapshot
	manifest model.Manifest
}
