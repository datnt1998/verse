package domain

import "time"

// DTO của GET/PATCH /api/queue (hợp đồng dữ liệu §3). `CloneQueueItem` ở
// clone.go là bản ghi DB; các kiểu ở đây chỉ tồn tại trên wire.

// Giới hạn luồng chạy (Quy tắc chạy): jobConcurrency 1–6 mặc định 2,
// imageConcurrency 1–8 mặc định 3; rescanHours ở rivals.go.
const (
	JobConcurrencyMin       = 1
	JobConcurrencyMax       = 6
	JobConcurrencyDefault   = 2
	ImageConcurrencyMin     = 1
	ImageConcurrencyMax     = 8
	ImageConcurrencyDefault = 3
	KeyJobConcurrency       = "clone.jobConcurrency"
	KeyImageConcurrency     = "clone.imageConcurrency"
	// KeyQueuePaused (domain.go) là cờ tạm dừng hàng chờ.
)

// QueueRunningJob là một job đang giữ luồng, hiển thị trên thẻ Đang chạy.
type QueueRunningJob struct {
	JobID        string     `json:"jobId"`
	ChannelID    string     `json:"channelId"`
	Title        string     `json:"title"`
	Agent        string     `json:"agent"`
	Now          string     `json:"now"`
	StepIndex    int        `json:"stepIndex"` // 0–8 theo CloneSteps
	ETAMinutes   *float64   `json:"etaMinutes"`
	ThumbnailURL *string    `json:"thumbnailUrl,omitempty"`
	TemplateName string     `json:"templateName,omitempty"`
	DurationSec  *float64   `json:"durationSec,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	PausedAt     *time.Time `json:"pausedAt,omitempty"`
}

// WaitReason giải thích vì sao một mục chưa chạy (§3, hội thoại thiết kế #52).
const (
	WaitQueuePaused   = "queue_paused"
	WaitWorkersBusy   = "workers_busy"
	WaitRotation      = "rotation"
	WaitPausedChannel = "paused_channel"
	WaitPaceFull      = "pace_full"
	WaitNoTemplate    = "no_template"
)

// QueueSkipReason giải thích một kênh bị bỏ lượt.
const (
	SkipPaceFull      = "pace_full"
	SkipNoTemplate    = "no_template"
	SkipPausedChannel = "paused_channel"
)

// QueueSkippedChannel là một kênh trong vòng luân phiên bị bỏ qua kèm lý do.
type QueueSkippedChannel struct {
	ChannelID string     `json:"channelId"`
	Reason    string     `json:"reason"`
	Until     *time.Time `json:"until,omitempty"`
}

// QueueRotation mô tả vòng luân phiên giữa các kênh ở nhóm batch.
type QueueRotation struct {
	Order   []string              `json:"order"`
	Next    *string               `json:"next"`
	Skipped []QueueSkippedChannel `json:"skipped"`
}

// CloneQueueItemView là một dòng Chờ lượt.
type CloneQueueItemView struct {
	ID           string        `json:"id"`
	ChannelID    string        `json:"channelId"`
	Title        string        `json:"title"`
	Source       string        `json:"source"` // tên kênh đối thủ nguồn
	TemplateName string        `json:"templateName"`
	DurationSec  *float64      `json:"durationSec"`
	Priority     ClonePriority `json:"priority"`
	ETAStart     *time.Time    `json:"etaStart"`
	ETADone      *time.Time    `json:"etaDone"`
	WaitReason   string        `json:"waitReason,omitempty"`
	Position     float64       `json:"position"`
	ThumbnailURL *string       `json:"thumbnailUrl,omitempty"`
	SourceURL    string        `json:"sourceUrl,omitempty"`
}

// QueuePace là nhịp hôm nay của một kênh; chỉ đếm job xuất phát từ batch.
type QueuePace struct {
	ChannelID string `json:"channelId"`
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
}

// QueueImageQuota là hạn mức còn lại của một nguồn vẽ. `limit`/`resetsAt`
// nil khi nhà cung cấp không báo; UI ghi "N ảnh hôm nay", không bịa "/50".
type QueueImageQuota struct {
	Provider string     `json:"provider"`
	Used     int64      `json:"used"`
	Limit    *int64     `json:"limit"`
	Period   string     `json:"period"` // "day" | "week"
	ResetsAt *time.Time `json:"resetsAt"`
	Role     string     `json:"role"` // "main" | "fallback"
}

// QueueDoneToday là một video xong hôm nay (sẵn sàng đăng hoặc chờ duyệt).
type QueueDoneToday struct {
	JobID string    `json:"jobId"`
	Title string    `json:"title"`
	At    time.Time `json:"at"`
	State string    `json:"state"` // "ready" | "review"
}

// CloneQueueView là toàn bộ màn Hàng chờ (§3).
type CloneQueueView struct {
	Paused       bool                 `json:"paused"`
	PausedAt     *time.Time           `json:"pausedAt,omitempty"`
	Workers      QueueWorkers         `json:"workers"`
	ImageWorkers int                  `json:"imageWorkers"`
	RescanHours  int                  `json:"rescanHours"`
	Running      []QueueRunningJob    `json:"running"`
	Rotation     QueueRotation        `json:"rotation"`
	Items        []CloneQueueItemView `json:"items"`
	Pace         []QueuePace          `json:"pace"`
	ImageQuota   []QueueImageQuota    `json:"imageQuota"`
	DoneToday    []QueueDoneToday     `json:"doneToday"`
	Removed      *QueueRemovedReceipt `json:"removed,omitempty"`
}

// QueueRemovedReceipt giữ định danh và thứ tự gốc để Hoàn tác bằng restore.
type QueueRemovedReceipt struct {
	ID       string        `json:"id"`
	Position float64       `json:"position"`
	Priority ClonePriority `json:"priority"`
}

// QueueWorkers đếm luồng đang dùng trên tổng số lane.
type QueueWorkers struct {
	Used  int `json:"used"`
	Total int `json:"total"`
}

// QueuePatch là thân PATCH /api/queue: đổi Quy tắc chạy, tạm dừng/chạy tiếp,
// dời, bỏ hoặc khôi phục một mục. Trường nil/0 nghĩa "giữ nguyên".
type QueuePatch struct {
	Paused       *bool `json:"paused,omitempty"`
	Workers      *int  `json:"workers,omitempty"`
	ImageWorkers *int  `json:"imageWorkers,omitempty"`
	RescanHours  *int  `json:"rescanHours,omitempty"`
	// Thao tác trên một mục: đặt ID cùng Position (vị trí số mới) hoặc Action.
	ID       *string  `json:"id,omitempty"`
	Position *float64 `json:"position,omitempty"`
	Action   string   `json:"action,omitempty"` // "remove" | "restore" | "front"
}
