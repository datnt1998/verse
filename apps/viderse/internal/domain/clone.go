package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type CloneStep string

const (
	CloneDownload   CloneStep = "download"
	CloneBackdrop   CloneStep = "backdrop"
	CloneTranscript CloneStep = "transcript"
	CloneMetadata   CloneStep = "metadata"
	CloneScenes     CloneStep = "scenes"
	CloneImages     CloneStep = "images"
	CloneBuild      CloneStep = "build"
	CloneScore      CloneStep = "score"
	CloneExport     CloneStep = "export"
)

var CloneSteps = [...]CloneStep{CloneDownload, CloneBackdrop, CloneTranscript, CloneMetadata, CloneScenes, CloneImages, CloneBuild, CloneScore, CloneExport}

// IndexStep trả thứ tự của step trong chuỗi; -1 khi step không thuộc chuỗi.
func IndexStep(s CloneStep) int {
	for i, step := range CloneSteps {
		if step == s {
			return i
		}
	}
	return -1
}

// IsCloneStep báo s có phải tên step hợp lệ của chuỗi hiện tại không.
func IsCloneStep(s string) bool { return IndexStep(CloneStep(s)) >= 0 }

// StepBefore báo a chạy trước b trong chuỗi; một step không bao giờ trước chính nó.
func StepBefore(a, b CloneStep) bool { return IndexStep(a) < IndexStep(b) }

// cloneDependents là đồ thị vô hiệu: khi đầu vào một step đổi, mọi step liệt kê
// phải đánh `stale` để bản chạy tiếp chạy lại chúng (bao hàm bắc cầu).
// Đầu ra của metadata (tiêu đề, mô tả, tag) chỉ mang tính thông tin nên không
// step nào phụ thuộc nó.
var cloneDependents = map[CloneStep][]CloneStep{
	CloneDownload:   {CloneBackdrop, CloneTranscript, CloneMetadata, CloneScenes, CloneImages, CloneBuild, CloneScore, CloneExport},
	CloneBackdrop:   {CloneBuild, CloneScore, CloneExport},
	CloneTranscript: {CloneMetadata, CloneScenes, CloneImages, CloneBuild, CloneScore, CloneExport},
	CloneMetadata:   {},
	CloneScenes:     {CloneImages, CloneBuild, CloneScore, CloneExport},
	CloneImages:     {CloneBuild, CloneScore, CloneExport},
	CloneBuild:      {CloneScore, CloneExport},
	CloneScore:      {CloneExport},
	CloneExport:     {},
}

// cloneStages nhóm bước của Bàn dựng cũ: step thuộc preparing, images hay rendering.
var cloneStages = map[CloneStep]string{
	CloneDownload:   "preparing",
	CloneBackdrop:   "preparing",
	CloneTranscript: "preparing",
	CloneMetadata:   "preparing",
	CloneScenes:     "preparing",
	CloneImages:     "images",
	CloneBuild:      "rendering",
	CloneScore:      "rendering",
	CloneExport:     "rendering",
}

// DependentsOf trả các step phải chạy lại khi đầu ra/đầu vào của step đổi.
func DependentsOf(step CloneStep) []CloneStep { return cloneDependents[step] }

// StageOf trả nhóm bước (preparing|images|rendering) của step.
func StageOf(step CloneStep) string { return cloneStages[step] }

// InChainOrder sắp các record theo thứ tự chuỗi; record của step đã rút khỏi
// chuỗi bị bỏ qua.
func InChainOrder(records []CloneStepRecord) []CloneStepRecord {
	out := make([]CloneStepRecord, 0, len(records))
	for _, step := range CloneSteps {
		for _, r := range records {
			if r.Step == step {
				out = append(out, r)
			}
		}
	}
	return out
}

// NextOpenStep trả record đầu tiên theo thứ tự chuỗi chưa `succeeded`, bất kể
// thứ tự mảng vào; nil khi mọi step đã xong.
func NextOpenStep(records []CloneStepRecord) *CloneStepRecord {
	ordered := InChainOrder(records)
	for i := range ordered {
		if ordered[i].Status != CloneSucceeded {
			return &ordered[i]
		}
	}
	return nil
}

// ChainDone báo mọi step của chuỗi đều có record và đều `succeeded`; thiếu
// record (job cũ) không tính là xong.
func ChainDone(records []CloneStepRecord) bool {
	for _, step := range CloneSteps {
		ok := false
		for _, r := range records {
			if r.Step == step && r.Status == CloneSucceeded {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// Nguồn tạo job: chọn tay, hàng loạt qua hàng chờ, hoặc Tự lấy video.
type CloneOrigin string

const (
	OriginManual CloneOrigin = "manual"
	OriginBulk   CloneOrigin = "bulk"
	OriginAuto   CloneOrigin = "auto"
)

// Trạng thái một mục hàng chờ: đang chờ, đã thành job, hoặc bỏ qua.
type CloneQueueState string

const (
	QueueQueued  CloneQueueState = "queued"
	QueueStarted CloneQueueState = "started"
	QueueSkipped CloneQueueState = "skipped"
)

// Mã lỗi ổn định của pipeline, lưu trên clone_steps.error.code. Mọi mã trong
// retryableCloneCodes được runner coi là việc đang đỗ (step → `waiting`);
// `awaiting_review` là cổng chờ người duyệt: không retryAt và không tự thử lại.
const (
	CloneErrNotFound          = "not_found"
	CloneErrConflict          = "conflict"
	CloneErrInvalidRequest    = "invalid_request"
	CloneErrSourceUnavailable = "source_unavailable"
	CloneErrToolMissing       = "tool_missing"
	CloneErrAwaitingReview    = "awaiting_review"
	CloneErrQuota             = "quota"
	CloneErrCancelled         = "cancelled"
	CloneErrInterrupted       = "interrupted"
	CloneErrStale             = "stale"
	CloneErrTemplateChanged   = "template_changed"
	CloneErrQualityGate       = "quality_gate"
	CloneErrInternal          = "internal_error"
)

// RetryAtSource* đánh dấu ai đặt retryAt: `provider` chỉ khi nhà cung cấp tự
// báo giờ reset; còn lại là backoff do worker tự chọn.
const (
	RetryAtProvider = "provider"
	RetryAtWorker   = "worker"
)

// retryableCloneCodes: mã được runner đổ thành `waiting` thay vì `failed`.
var retryableCloneCodes = map[string]bool{
	CloneErrQuota:          true,
	CloneErrInterrupted:    true,
	CloneErrToolMissing:    true,
	CloneErrAwaitingReview: true,
}

// CloneErrRetryable báo mã lỗi có được runner giữ job ở `waiting` (đỗ, có thể
// chạy tiếp) thay vì `failed` không.
func CloneErrRetryable(code string) bool { return retryableCloneCodes[code] }

// Giới hạn thử lại tự động do worker tự xếp (không phải retryAt của provider):
// sau AutoRetryAttempts lần, hoặc khi backoff vượt AutoRetryMaxWait, step đỗ
// `waiting` cho tới khi người dùng chạy tiếp.
const (
	AutoRetryAttempts = 3
	AutoRetryMaxWait  = 30 * time.Minute
)

// AutoRetryAt trả thời điểm worker tự thử lại step, hoặc nil khi việc thử lại
// thuộc về người dùng: retryAt của provider, lỗi quota ở bước ảnh, hết số lần
// tự thử, hoặc backoff quá dài.
func AutoRetryAt(step CloneStepRecord, now time.Time) *time.Time {
	if step.Error == nil || step.Error.RetryAt == nil || step.Error.RetryAtSource == RetryAtProvider {
		return nil
	}
	if step.Step == CloneImages && step.Error.Code == CloneErrQuota {
		return nil
	}
	at := *step.Error.RetryAt
	if step.Attempt >= AutoRetryAttempts || at.Sub(now) > AutoRetryMaxWait {
		return nil
	}
	return &at
}

// CloneSourceKey là khoá khử trùng của một job: một job đang hoạt động cho mỗi
// (kênh sở hữu, video nguồn).
func CloneSourceKey(channelID, videoID string) string {
	return "channel:" + channelID + ":video:" + videoID
}

// CloneSourceVideoID trả id video nguồn trong source key dạng
// `channel:<owner>:video:<id>` (hoặc khoá cũ `video:<id>`); "" khi không khớp.
func CloneSourceVideoID(sourceKey string) string {
	const marker = ":video:"
	if i := strings.LastIndex(sourceKey, marker); i >= 0 {
		return sourceKey[i+len(marker):]
	}
	if strings.HasPrefix(sourceKey, "video:") {
		return sourceKey[len("video:"):]
	}
	return ""
}

type CloneStatus string

const (
	ClonePending   CloneStatus = "pending"
	CloneRunning   CloneStatus = "running"
	CloneWaiting   CloneStatus = "waiting"
	CloneSucceeded CloneStatus = "succeeded"
	CloneFailed    CloneStatus = "failed"
	CloneCancelled CloneStatus = "cancelled"
	CloneStale     CloneStatus = "stale"
)

type ClonePriority string

const (
	ClonePick  ClonePriority = "pick"
	CloneBatch ClonePriority = "batch"
)

type CloneError struct {
	Code          string     `json:"code"`
	Message       string     `json:"message"`
	Recovery      *string    `json:"recovery"`
	RetryAt       *time.Time `json:"retryAt"`
	RetryAtSource string     `json:"retryAtSource,omitempty"`
}

func (e *CloneError) Error() string { return e.Message }

type CloneSourceSnapshot struct {
	ID                string     `json:"id"`
	URL               string     `json:"url"`
	Title             string     `json:"title"`
	ChannelID         *string    `json:"channelId"`
	ChannelTitle      *string    `json:"channelTitle"`
	ThumbnailURL      *string    `json:"thumbnailUrl"`
	DurationSec       *float64   `json:"durationSec"`
	ViewCount         *int64     `json:"viewCount,omitempty"`
	HasCaptions       *bool      `json:"hasCaptions,omitempty"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty"`
	Outlier           *float64   `json:"outlier,omitempty"`
	CaptionLanguage   *string    `json:"captionLanguage,omitempty"`
	ChapterCount      *int       `json:"chapterCount,omitempty"`
	TranscriptExcerpt *string    `json:"transcriptExcerpt,omitempty"`
	ScannedAt         time.Time  `json:"scannedAt"`
}
type CloneProgress struct {
	UnitsDone  int64  `json:"unitsDone"`
	UnitsTotal *int64 `json:"unitsTotal"`
	Phase      string `json:"phase"`
}
type CloneArtifact struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	SHA256      string    `json:"sha256"`
	Bytes       int64     `json:"bytes"`
	Step        CloneStep `json:"step"`
	ContentType string    `json:"contentType"`
	Filename    string    `json:"filename"`
}
type CloneStepOutput struct {
	Artifacts []CloneArtifact `json:"artifacts"`
	// Step-specific structured manifests have intentionally heterogeneous values.
	Data map[string]any `json:"data,omitempty"`
}
type CloneJob struct {
	ID                string              `json:"id"`
	ChannelID         string              `json:"channelId"`
	SourceKey         string              `json:"sourceKey"`
	Source            CloneSourceSnapshot `json:"source"`
	TemplateID        string              `json:"templateId"`
	TemplateSnapshot  json.RawMessage     `json:"templateSnapshot"`
	ModelRefs         json.RawMessage     `json:"modelRefs"`
	NoImages          bool                `json:"noImages"`
	Generation        int                 `json:"generation"`
	SupersededAt      *time.Time          `json:"supersededAt"`
	PausedAt          *time.Time          `json:"pausedAt"`
	CancelRequestedAt *time.Time          `json:"cancelRequestedAt"`
	Priority          ClonePriority       `json:"priority"`
	Origin            CloneOrigin         `json:"origin"`
	Status            CloneStatus         `json:"status"`
	StageGroup        string              `json:"stageGroup"`
	ActiveStep        *CloneStep          `json:"activeStep"`
	Fence             int64               `json:"-"`
	Version           int64               `json:"version"`
	PostedAt          *time.Time          `json:"postedAt"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
	// AutoFix là lần máy tự thử lại lỗi theo Mức tự chủ của kênh (07 §20).
	AutoFix *AutoFix `json:"autoFix"`
}
type CloneStepRecord struct {
	JobID        string           `json:"jobId"`
	Step         CloneStep        `json:"step"`
	Status       CloneStatus      `json:"status"`
	Attempt      int              `json:"attempt"`
	Progress     CloneProgress    `json:"progress"`
	Checkpoint   json.RawMessage  `json:"checkpoint"`
	Output       *CloneStepOutput `json:"output"`
	Error        *CloneError      `json:"error"`
	Fence        int64            `json:"-"`
	ProcessGroup *int             `json:"-"`
	StartedAt    *time.Time       `json:"startedAt"`
	FinishedAt   *time.Time       `json:"finishedAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}
type CloneQueueItem struct {
	ID               string              `json:"id"`
	ChannelID        string              `json:"channelId"`
	SourceVideoID    string              `json:"sourceVideoId"`
	SourceKey        string              `json:"sourceKey"`
	Source           CloneSourceSnapshot `json:"source"`
	Priority         ClonePriority       `json:"priority"`
	Origin           CloneOrigin         `json:"origin"`
	Position         float64             `json:"position"`
	State            CloneQueueState     `json:"state"`
	NoImages         bool                `json:"noImages"`
	ScenePromptID    *string             `json:"scenePromptId"`
	CoverPromptID    *string             `json:"coverPromptId"`
	NoImagesOverride *bool               `json:"noImagesOverride,omitempty"`
	OverrideRevision int64               `json:"overrideRevision"`
	TemplateID       *string             `json:"templateId,omitempty"`
	JobID            *string             `json:"jobId"`
	RestartOf        *string             `json:"previousJobId,omitempty"`
	Reason           *string             `json:"reason"`
	CreatedAt        time.Time           `json:"createdAt"`
	StartedAt        *time.Time          `json:"startedAt"`
	RemovedAt        *time.Time          `json:"-"`
}
type CloneAgentView struct {
	ID     string            `json:"id"`
	State  string            `json:"state"`
	Now    *string           `json:"now"`
	Metric *CloneAgentMetric `json:"metric"`
}
type CloneAgentMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type CloneJobView struct {
	CloneJob
	Subline       string            `json:"subline"`
	AgentView     []CloneAgentView  `json:"agentView"`
	Steps         []CloneStepRecord `json:"steps"`
	PreviousJobID *string           `json:"previousJobId"`
	ExportBytes   *int64            `json:"exportBytes"`
	// QueueItem mang mục hàng chờ gốc khi hàng này là mục chưa có job
	// (StageGroup "queued"); nil cho job thật.
	QueueItem *CloneQueueItem `json:"queueItem,omitempty"`
}

const (
	CloneEventCreated  = "clone.job.created"
	CloneEventUpdated  = "clone.job.updated"
	CloneEventDeleted  = "clone.job.deleted"
	CloneEventProgress = "clone.step.progress"
	CloneEventQueue    = "queue.updated"
)
