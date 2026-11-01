package domain

// Việc nền (hợp đồng dữ liệu §11): mỗi mảng nghiệp vụ chạy lâu (cài công cụ,
// đăng nhập, chuyển thư mục, dọn dữ liệu…) ghi một dòng để popover Việc nền,
// màn Chuẩn bị máy và Cài đặt cùng thấy tiến độ. Gói nghiệp vụ khai interface
// nhỏ của riêng nó trên các kiểu dưới đây (xem Tasks), không import activity.

// TaskKind là loại việc nền — tập giá trị cố định theo §11 (thêm storage_move
// của §23).
type TaskKind = string

const (
	TaskVideo            TaskKind = "video"
	TaskIdentity         TaskKind = "identity"
	TaskChannelScan      TaskKind = "channel_scan"
	TaskBackdropScan     TaskKind = "backdrop_scan"
	TaskBackdropDownload TaskKind = "backdrop_download"
	TaskThumbnailPreview TaskKind = "thumbnail_preview"
	TaskTemplatePreview  TaskKind = "template_preview"
	TaskEditorExport     TaskKind = "editor_export"
	TaskSetup            TaskKind = "setup"
	TaskCleanup          TaskKind = "cleanup"
	TaskLogs             TaskKind = "logs"
	TaskStorageMove      TaskKind = "storage_move"
	TaskStorageMigrate   TaskKind = "storage_migrate"
	TaskAssistant        TaskKind = "assistant_plan"
)

// TaskState là trạng thái của một việc nền đang hiện.
type TaskState = string

const (
	TaskRunning TaskState = "running"
	TaskPaused  TaskState = "paused"
	TaskWaiting TaskState = "waiting"
	TaskFailed  TaskState = "failed"
)

// TaskOp là thao tác mà nút của dòng việc nền gọi khi không dẫn sang trang khác.
type TaskOp = string

const (
	TaskOpCancel TaskOp = "cancel"
	TaskOpResume TaskOp = "resume"
	TaskOpRetry  TaskOp = "retry"
)

// ObjectRef trỏ tới thứ mà việc nền đang làm (một công cụ, một kênh, thư mục
// dự án…). Label là chữ hiện cho người dùng.
type ObjectRef struct {
	Type  string
	ID    string
	Label string
}

// TaskProgress là tiến độ đếm được; nil ở các trường dùng nó nghĩa là không xác
// định. Groups chỉ dùng cho việc có nhiều nhóm (chuyển thư mục: images, editor,
// sources, exports).
type TaskProgress struct {
	Done   int64
	Total  int64
	Groups []TaskGroupProgress
}

// TaskGroupProgress là tiến độ của một nhóm trong việc nền nhiều nhóm.
type TaskGroupProgress struct {
	Group string
	Done  int64
	Total int64
}

// TaskAction là nút của dòng việc nền: Href dẫn sang trang, Op gọi thao tác.
type TaskAction struct {
	Label string
	Href  string
	Op    TaskOp
}

// TaskSpec là dữ liệu mở một việc nền mới (trạng thái ban đầu running).
type TaskSpec struct {
	Kind     TaskKind
	Agent    string // AgentId của §1 khi có: scout | director | artist | qa
	Title    string
	Object   ObjectRef
	Phase    string // câu tiếng Việt của pha hiện tại
	Progress *TaskProgress
	Action   TaskAction
}

// TaskUpdate đổi phần đang thay đổi của một việc nền. Trường nil giữ nguyên
// giá trị cũ; State rỗng giữ nguyên trạng thái.
type TaskUpdate struct {
	Phase    *string
	Progress *TaskProgress
	ETASec   *int
	State    TaskState
	Action   *TaskAction
}
