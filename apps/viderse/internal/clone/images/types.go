// Package images exposes persisted owner edits to the clone image pipeline.
package images

import (
	"context"
	"encoding/json"
	"fmt"

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

type Service struct {
	Edits     *jobedit.Service
	Prompts   PromptCatalog
	QuotaLeft func(context.Context) (int, error)
	// Quotas trả hạn mức ảnh còn lại của từng nguồn vẽ của video này.
	Quotas func(context.Context) ([]ProviderQuota, error)
	// RefLimits trả trần ảnh tham chiếu của một nguồn vẽ ("" = nguồn đang dùng).
	RefLimits func(context.Context, string) (int, bool)
	// Binder thay media của clip trong project đã dựng (07 §26).
	Binder Binder
	// Cancels là sổ huỷ lần vẽ dùng chung với worker.
	Cancels *model.CancelRegistry
	Ffmpeg  FfmpegResolver
	Runner  tools.Runner
	// AutoApprove đọc cờ Tự duyệt của kênh để tóm tắt điểm theo ngưỡng cố định
	// 85; nil → không tóm tắt. Không còn cấu hình Mức tự chủ.
	AutoApprove interface {
		JobAutoApprove(ctx context.Context, jobID string) (bool, bool, error)
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
	Prompt            string          `json:"prompt,omitempty"`
	SourceFingerprint string          `json:"sourceFingerprint"`
	Source            string          `json:"source"`
	Cutout            *scenes.FileRef `json:"cutout"`
	OriginalURL       string          `json:"originalUrl,omitempty"`
	CutoutURL         string          `json:"cutoutUrl,omitempty"`
	CutoutState       string          `json:"cutoutState"`
	// Score là điểm Kiểm định của version này (nil khi chưa chấm được).
	Score      *domain.ImageScore `json:"score"`
	ScoreIssue string             `json:"scoreIssue,omitempty"`
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
	Score      *domain.ImageScore `json:"score"`
	ScoreIssue string             `json:"scoreIssue,omitempty"`
}

// AutonomyView tóm tắt điểm so với ngưỡng tự duyệt — dòng "cảnh 4 được 72"
// của Chờ duyệt. Giữ nguyên khoá wire `autonomy` để client cũ đọc được: Mode
// suy ra từ cờ Tự duyệt ("score" khi bật, "ask" khi tắt) và Min cố định 85;
// không còn cấu hình người dùng đứng sau hai trường này.
type AutonomyView struct {
	Mode    string               `json:"mode"`
	Min     int                  `json:"min"`
	Total   int                  `json:"total"`
	Passed  int                  `json:"passed"`
	Pending int                  `json:"pending"`
	Below   []autonomy.BelowItem `json:"below"`
	Summary string               `json:"summary"`
}

// ImageView là một ảnh của video theo 07 §26: mọi lần vẽ là một phiên bản,
// kèm đúng prompt và ảnh tham chiếu đã gửi.
type ImageView struct {
	ID          string             `json:"id"`
	Kind        string             `json:"kind"`
	Index       int                `json:"index"`
	Label       string             `json:"label"`
	UsedVersion int                `json:"usedVersion"`
	NeedsReview bool               `json:"needsReview"`
	Status      string             `json:"status"`
	ScoreIssue  string             `json:"scoreIssue,omitempty"`
	RefLimit    int                `json:"refLimit"`
	Versions    []ImageVersionView `json:"versions"`
}

// ImageVersionView là một lần vẽ: nguyên văn đã gửi, ảnh tham chiếu, thời gian,
// chi phí và lỗi thật.
type ImageVersionView struct {
	N          int                `json:"n"`
	State      string             `json:"state"`
	URL        *string            `json:"url"`
	PreviewURL *string            `json:"previewUrl"`
	Width      int                `json:"width"`
	Height     int                `json:"height"`
	Parts      []model.PromptPart `json:"parts"`
	// Wrapper là phần không sửa được của prompt đã gửi (hàng rào an toàn và
	// chỉ dẫn công cụ của đúng nguồn vẽ). §26 chưa có chỗ cho nó — trường thêm;
	// `sentText` vẫn là toàn bộ nguyên văn gửi nhà cung cấp.
	Wrapper      model.PromptWrapper `json:"wrapper"`
	SentText     string              `json:"sentText"`
	Refs         []model.ImageRef    `json:"refs"`
	Provider     string              `json:"provider"`
	Model        string              `json:"model"`
	Params       model.PromptParams  `json:"params"`
	Feedback     *string             `json:"feedback"`
	DiffFromPrev [][2]string         `json:"diffFromPrev"`
	StartedAt    string              `json:"startedAt"`
	FinishedAt   *string             `json:"finishedAt"`
	DurationSec  *float64            `json:"durationSec"`
	CostImages   int                 `json:"costImages"`
	ScoreIssue   string              `json:"scoreIssue,omitempty"`
	Score        *domain.ImageScore  `json:"score"`
	Error        *model.VersionError `json:"error"`
	// PartsLegacy: bản cũ chưa tách phần — giao diện hiện sentText như một phần.
	PartsLegacy bool `json:"partsLegacy,omitempty"`
}

// ProviderQuota là số ảnh còn lại hôm nay của một nguồn vẽ.
type ProviderQuota struct {
	Provider string  `json:"provider"`
	Label    string  `json:"label"`
	Left     int     `json:"left"`
	Total    int     `json:"total"`
	ResetAt  *string `json:"resetAt"`
	Paid     bool    `json:"paid"`
}

// QuotaView là hạn mức ảnh của video này, đủ cho câu "ChatGPT còn 7/50 hôm nay".
type QuotaView struct {
	Providers []ProviderQuota `json:"providers"`
	Left      int             `json:"left"`
	Fallback  *ProviderQuota  `json:"fallback"`
}

// RedrawVersionsInput là yêu cầu vẽ lại một hoặc nhiều ảnh (07 §26).
type RedrawVersionsInput struct {
	// False leaves the undoable project edit to the requesting Editor.
	Project  *bool             `json:"project,omitempty"`
	ImageIDs []string          `json:"imageIds"`
	Feedback string            `json:"feedback,omitempty"`
	Parts    map[string]string `json:"parts,omitempty"`
	Refs     []RedrawRef       `json:"refs,omitempty"`
	Provider string            `json:"provider,omitempty"`
	// BaseVersion giữ đúng bản ảnh chủ kênh đang xem làm nguồn prompt/ảnh
	// tham chiếu cho lần vẽ lại (kể cả khi bản đang dùng khác bản đó). Chỉ dùng
	// cho một ảnh; vắng mặt thì tiếp nối bản đang dùng như trước.
	BaseVersion *int `json:"baseVersion,omitempty"`
}

// RedrawRef là một ảnh tham chiếu chủ kênh chọn cho lần vẽ lại.
type RedrawRef struct {
	ID           string         `json:"id,omitempty"`
	Upload       string         `json:"upload,omitempty"`
	VideoImageID string         `json:"videoImageId,omitempty"`
	RivalFrame   *RivalFrameRef `json:"rivalFrame,omitempty"`
	Role         string         `json:"role"`
}

// RivalFrameRef là một khung của video gốc.
type RivalFrameRef struct {
	VideoID string  `json:"videoId"`
	AtSec   float64 `json:"atSec"`
}

// RedrawVersionsResult trả số bản sẽ vẽ của từng ảnh.
type RedrawVersionsResult struct {
	Versions []PlannedVersion `json:"versions"`
}

// PlannedVersion là số bản kế tiếp của một ảnh.
type PlannedVersion struct {
	ImageID string `json:"imageId"`
	N       int    `json:"n"`
}

// UseVersionInput chọn bản đang dùng cho video.
type UseVersionInput struct {
	N       int   `json:"n"`
	Project *bool `json:"project,omitempty"`
}

// UseVersionResult trả bản đang dùng và thay đổi clip editor cần áp (nếu có).
type UseVersionResult struct {
	ImageID         string  `json:"imageId"`
	N               int     `json:"n"`
	UsedVersion     int     `json:"usedVersion"`
	NeedsReview     bool    `json:"needsReview"`
	ProjectID       *string `json:"projectId"`
	ClipID          *string `json:"clipId"`
	MediaID         *string `json:"mediaId"`
	Src             *string `json:"src"`
	PreviousMediaID *string `json:"previousMediaId"`
	PreviousVersion *int    `json:"previousVersion"`
	Quality         string  `json:"quality"`
	ProjectError    string  `json:"projectError,omitempty"`
}

// CancelVersionResult trả lần huỷ và số ảnh đã tốn.
type CancelVersionResult struct {
	ImageID     string `json:"imageId"`
	N           int    `json:"n"`
	Cancelled   bool   `json:"cancelled"`
	CostImages  int    `json:"costImages"`
	UsedVersion int    `json:"usedVersion"`
}

// TooManyRefsError là 422 {code:"too_many_refs", limit} của 07 §26.
type TooManyRefsError struct {
	Limit int
}

func (e *TooManyRefsError) Error() string {
	return fmt.Sprintf("Nguồn vẽ nhận tối đa %d ảnh tham chiếu", e.Limit)
}

type View struct {
	Autonomy      *AutonomyView `json:"autonomy"`
	Manifest      *ManifestView `json:"manifest"`
	Items         []ItemView    `json:"items"`
	Images        []ImageView   `json:"images"`
	Quota         *QuotaView    `json:"quota"`
	Approved      bool          `json:"approved"`
	ApprovedToday *int          `json:"approvedToday"`
	Reason        string        `json:"reason,omitempty"`
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
	// Enables: video bật lại ảnh cảnh; Redraw.Scenes là số ảnh cảnh sẽ vẽ mới.
	Enables bool `json:"enablesImages"`
	// rescan: cảnh đã cắt khi tắt ảnh (một đoạn), phải cắt lại cho bộ ảnh mới.
	rescan bool
}

type loaded struct {
	snapshot jobedit.Snapshot
	manifest model.Manifest
}
