// Package scan là luồng quét đối thủ của Trinh sát (Gói V5): service cho
// API/CLI, worker nền một goroutine đọc tab Videos từng trang dưới fence kênh,
// và hợp đồng lưu trữ mà persistence hiện thực. Chuyển từ
// scan-repository.ts/scan-worker.ts/upload-times.ts của yt-factory.
package scan

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/youtube"
)

// Hằng số của luồng quét (chuyển nguyên văn từ bản TS).
const (
	LeaseTTL       = 60 * time.Second // TTL của cả fence kênh lẫn lease worker
	Heartbeat      = 15 * time.Second // nhịp gia hạn lease + nhịp đọc hạn kế tiếp duy nhất khi rảnh
	PageMax        = 200              // trần số dòng một trang commit nhận
	DiscoveryBlock = 10_000           // khối khoá trước vị trí nhỏ nhất của một run
	HeadAnchorMax  = 150              // anchors lưu bền sau head check
	MedianSample   = 150              // số hàng mới nhất tính mức thường
	AnchorMax      = 300              // anchors tối đa một run giữ
	KeyLimit       = int64(1) << 52   // miền tuyệt đối của khoá sắp xếp
	CursorMaxBytes = youtube.MaxCursorBytes
	RemainderMax   = youtube.MaxRemainderBytes
	// ProgressInterval là nhịp tối thiểu của một lần ghi tiến độ lên Việc nền.
	ProgressInterval = 250 * time.Millisecond
	// IdleFloor là sàn ngủ của vòng rảnh: tránh quay vòng khi hạn vừa trôi qua
	// mà không có việc nào thật sự chạy được.
	IdleFloor = time.Second
)

// Lease là fence của một kênh: chủ giữ (owner), token bí mật và hạn hiệu lực.
// Mọi ghi của luồng quét kiểm token còn hạn trong cùng giao dịch.
type Lease struct {
	ChannelID string
	Token     string // uuid dạng chuỗi
	Owner     string
	Until     time.Time
}

// Wakeup là kết quả của một lần đọc lịch của vòng rảnh.
type Wakeup struct {
	// Actionable: có việc chạy được ngay — việc queued/running (kể cả recover
	// sau crash), backoff đã đến hạn, hoặc kênh đến hạn rescan chưa có run
	// discovery sống.
	Actionable bool
	// Next: hạn tương lai gần nhất (nil = không có): backoff/rescan tương lai,
	// hết nghỉ thăm dò upload, và scan_state.lease_until tương lai của kênh có
	// việc sống hoặc còn upload candidate — để thức đúng lúc fence hết hạn
	// thay vì spin.
	Next *time.Time
}

// WorkerLease là lease của luồng quét toàn cục (platform_state.youtube).
// Owner là chuỗi ngẫu nhiên do process giữ — không phải tên nền tảng.
type WorkerLease struct {
	Owner string
	Until time.Time
}

// State là một dòng scan_state.
type State struct {
	ChannelID         string
	HistoryCap        int
	IndexedVideos     int
	HistoryEnd        bool
	HistoryCursor     string
	HistoryRemainder  string
	HeadAnchors       []string
	HeadNewestID      string
	HeadCheckedAt     *time.Time
	DiscoveryCaughtUp bool
	Revision          int64
	LeaseOwner        string
	LeaseToken        string
	LeaseUntil        *time.Time
	UpdatedAt         time.Time
}

// Job là một dòng scan_jobs.
type Job struct {
	ID            string
	ChannelID     string
	Kind          domain.ScanJobKind
	State         domain.ScanJobState
	RequestedCap  *int
	Cursor        string
	CursorAt      *time.Time
	Remainder     string
	ReplayTarget  string
	LastVideoID   string
	OrderBase     *int64
	NextOrdinal   int
	Anchors       []string
	NewestID      string
	Pages         int
	Scanned       int
	Added         int
	Attempts      int
	Restarts      int
	NextAttemptAt *time.Time
	ErrorCode     string
	LastError     string
	Recovery      string
	ReachedEnd    bool
	TaskID        string
	StartedAt     *time.Time
	UpdatedAt     time.Time
}

// JobLive khớp điều kiện "còn sống" của schema: mọi trạng thái trừ complete.
func (j *Job) Live() bool { return j != nil && j.State != domain.ScanComplete }

// Runnable là trạng thái worker được nhặt (cùng điều kiện truy vấn runnable).
func (j *Job) RunnableAt(now time.Time) bool {
	switch j.State {
	case domain.ScanQueued, domain.ScanRunning:
		return true
	case domain.ScanBackoff:
		return j.NextAttemptAt != nil && !j.NextAttemptAt.After(now)
	}
	return false
}

// Page là một trang video đã đọc từ YouTube kèm cursor sang trang kế.
type Page struct {
	Videos     []youtube.ScannedVideo
	NextCursor *youtube.ChannelCursor
}

// CommitPageInput là đầu vào của CommitPage — ngữ nghĩa của commitScanPage
// bản TS: một giao dịch fenced ghi hồ sơ + video + trạng thái + việc.
type CommitPageInput struct {
	ChannelID  string
	Videos     []youtube.ScannedVideo
	NextCursor *youtube.ChannelCursor
	Remainder  []youtube.ScannedVideo
	Origin     domain.ScanOrigin
	Lease      Lease
	// JobID khác rỗng khi trang thuộc một việc đang chạy (luôn như vậy với
	// discovery; lịch sử chỉ có việc khi quét lịch sử chạy).
	JobID string
	// NewestVideoID phân biệt "được đặt" (kể cả rỗng) với "không truyền".
	NewestVideoID *string
	// Profile kênh của nguồn (chỉ baseline mới được tạo dòng kênh).
	Profile *youtube.ChannelProfile
	// Now phục vụ test (mặc định đồng hồ hệ thống).
	Now time.Time
}

// CommitPageResult kể lại trang vừa ghi.
type CommitPageResult struct {
	Scanned     int
	Added       int
	Overlap     bool // trang chứa một anchor của run (đo trước upsert)
	NextOrdinal int
	JobID       string
}

// HeadCheckInput của CompleteHeadCheck.
type HeadCheckInput struct {
	CheckedAt time.Time
	NewestID  *string  // nil = giữ mốc cũ
	Anchors   []string // nil = suy từ hàng mới nhất origin baseline|discovery
	Lease     Lease
	JobID     string // việc discovery kết thúc trong cùng tx (rỗng = không)
	Now       time.Time
}

// HeadCheckResult là kết quả head check.
type HeadCheckResult struct {
	MedianViews *int64
	CheckedAt   time.Time
}

// Backoff là ghi backoff của một việc đang chạy.
type Backoff struct {
	NextAttemptAt time.Time
	Attempts      int
	ErrorCode     string
	Err           error // làm sạch rồi lưu last_error
	Recovery      string
}

// JobError là ghi lỗi (hoặc tạm dừng kèm chẩn đoán) của một việc đang chạy.
type JobError struct {
	ErrorCode string
	Err       error
	Recovery  string
	Attempts  *int // nil = giữ nguyên
}

// UploadCandidate là một video chờ đọc giờ đăng chính xác.
type UploadCandidate struct {
	VideoID   string
	ChannelID string
}

// StatusSnapshot là dữ liệu thô mà Status dựng (chưa gồm RescanHours — cài đặt
// nằm ở service).
type StatusSnapshot struct {
	Channel      domain.RivalChannel
	State        State
	HasState     bool
	HistoryJob   *Job // việc lịch sử còn sống, hoặc mới nhất khi không còn
	DiscoveryJob *Job // việc discovery còn sống
	WorkerLive   bool
}

// Store là hợp đồng lưu trữ của luồng quét; hiện thực Postgres ở
// internal/persistence/postgres. Mỗi phương thức là một đơn vị nguyên tử
// (transaction) — ghi dưới fence kiểm token trong cùng tx như bản TS.
type Store interface {
	// ── kênh ──
	Channel(ctx context.Context, id string) (domain.RivalChannel, error)
	DeleteChannel(ctx context.Context, id string) error
	EnsureState(ctx context.Context, channelID string) (State, error)
	FindKnownVideoIDs(ctx context.Context, channelID string, ids []string) (map[string]bool, error)

	// ── fence kênh (scan_state.lease_*) ──
	AcquireChannelLease(ctx context.Context, channelID, owner string, ttl time.Duration) (*Lease, error)
	RenewChannelLease(ctx context.Context, lease Lease) (*Lease, error)
	ReleaseChannelLease(ctx context.Context, lease Lease) error

	// ── commit + head check (fenced) ──
	CommitPage(ctx context.Context, in CommitPageInput) (CommitPageResult, error)
	CompleteHeadCheck(ctx context.Context, channelID string, in HeadCheckInput) (HeadCheckResult, error)
	// RecordUploadTime ghi kết quả đọc giờ đăng dưới fence kênh: token còn hạn
	// và video vẫn thuộc kênh trong cùng giao dịch.
	RecordUploadTime(ctx context.Context, lease Lease, videoID string, at *time.Time, hasCaptions bool, now time.Time) error

	// ── điều khiển (mặt API/CLI) ──
	Status(ctx context.Context, channelID string) (StatusSnapshot, error)
	StartBackfill(ctx context.Context, channelID string) (*Job, error)
	SetHistoryCap(ctx context.Context, channelID string, cap int) error
	ControlBackfill(ctx context.Context, channelID string, action domain.ScanAction) error
	// StartDiscovery xếp/khoi phục một lượt kiểm tra video mới. replaceStopped
	// (nút check/Tiếp tục/Thử lại) đưa việc paused/error/capped về hàng chờ;
	// lịch tự động chỉ tạo run mới khi kênh không có việc discovery còn sống.
	StartDiscovery(ctx context.Context, channelID string, replaceStopped bool) (bool, error)

	// ── mặt worker ──
	NextRunnableJob(ctx context.Context, skip map[string]bool) (*Job, error)
	// NextRunnableJobForChannel chỉ xét một kênh (CLI inline): discovery trước
	// lịch sử trong cùng kênh, nil khi kênh không còn việc chạy được.
	NextRunnableJobForChannel(ctx context.Context, channelID string) (*Job, error)
	Job(ctx context.Context, jobID string) (*Job, error)
	ClaimJob(ctx context.Context, jobID string, lease Lease) (*Job, error)
	SetJobBackoff(ctx context.Context, jobID string, lease Lease, b Backoff) error
	SetJobError(ctx context.Context, jobID string, lease Lease, e JobError) error
	SetJobCapped(ctx context.Context, jobID string, lease Lease) error
	SetJobPaused(ctx context.Context, jobID string, lease Lease, e JobError) error
	CompleteJob(ctx context.Context, jobID string, lease Lease) error
	RestartJobCursor(ctx context.Context, jobID string, lease Lease, cause error) error
	// SyncScanTask tạo/nhận lại dòng Việc nền rồi nối vào việc trong MỘT giao
	// dịch (khoá parent → state → job). Kênh/việc đã bị gỡ → channel_not_found/
	// job_stopped; việc đã dừng hẳn → job_stopped, không ghi dòng nào. Kho tự
	// suy state/phase/action từ việc ĐANG KHOÁ qua scan.TaskPresentation; các
	// trường đó trong spec bị bỏ qua. spec.Progress nil giữ tiến độ đang hiện.
	// Khi chính lần ghi này đưa việc sang failed, kho ghi thông báo + error_event
	// trong cùng giao dịch và trả id thông báo ở TaskSync.NotificationID.
	SyncScanTask(ctx context.Context, jobID, channelID string, spec domain.TaskSpec) (TaskSync, error)
	// ClearTaskLink gỡ liên kết khi dòng Việc nền đã bị xoá (kết thúc việc).
	ClearTaskLink(ctx context.Context, jobID string) error

	// ── lịch + thăm dò giờ đăng ──
	DueChannels(ctx context.Context, rescanHours int, now time.Time) ([]string, error)
	// NextWakeup là lần đọc lịch duy nhất của vòng rảnh: Actionable báo có việc
	// làm được ngay (queued/running, backoff/rescan đến hạn) — khiến worker rời
	// rảnh; Next là hạn tương lai gần nhất để ngủ tới (backoff/rescan tương lai,
	// hết nghỉ thăm dò upload, hết hạn fence của kênh đang chờ).
	NextWakeup(ctx context.Context, rescanHours int, now time.Time) (Wakeup, error)
	NextUploadCandidate(ctx context.Context, now time.Time) (*UploadCandidate, error)
	PauseUploads(ctx context.Context, until time.Time) error
	BumpUploadFailures(ctx context.Context) (int, error)
	ResetUploadFailures(ctx context.Context) error

	// ── lease worker (platform_state 'youtube') ──
	AcquireWorkerLease(ctx context.Context, owner string, ttl time.Duration) (*WorkerLease, error)
	RenewWorkerLease(ctx context.Context, lease WorkerLease, ttl time.Duration) (*WorkerLease, error)
	ReleaseWorkerLease(ctx context.Context, lease WorkerLease) error
}
