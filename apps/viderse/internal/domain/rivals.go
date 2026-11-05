package domain

import "time"

// Từ vựng của Trinh sát (Gói V5): kênh đối thủ, video của kênh, việc quét và
// khung đăng lặp lại (§12). Chỉ kiểu giá trị — không I/O.

// AllRivals là id giả của "Tất cả đối thủ" trong đường dẫn video
// (/api/rivals/all/videos); không trùng được id kênh thật (UC…).
const AllRivals = "all"

// OutlierMinScore là ngưỡng điểm để một video được coi là nổi trội (×3).
const OutlierMinScore = 3.0

// LongFormMinSec là độ dài tối thiểu của bộ lọc "Dài ≥ 8 phút".
const LongFormMinSec = 8 * 60

// Giới hạn quét lịch sử: 150–10.000 video, mặc định 2.000.
const (
	HistoryCapMin     = 150
	HistoryCapMax     = 10000
	HistoryCapDefault = 2000
)

// Khoảng quét lại đối thủ cố định một giờ.
const RescanHoursDefault = 1

// RivalChannel là một kênh đối thủ đang theo dõi.
type RivalChannel struct {
	ID              string // UC… (24 ký tự)
	Handle          string // "@nocturnestories"; rỗng khi kênh không có handle
	Title           string
	Description     string
	AvatarURL       string
	URL             string
	SubscriberCount *int64
	MedianViews     *int64 // mức thường; nil khi chưa có lượt xem nào
	MedianAt        *time.Time
	AddedAt         time.Time
	ScannedAt       *time.Time // lần kiểm tra video mới gần nhất xong
}

// ScanOrigin là đường một video vào kho: lần đọc đầu khi thêm kênh, quét lịch
// sử, hay kiểm tra video mới.
type ScanOrigin = string

const (
	OriginBaseline  ScanOrigin = "baseline"
	OriginHistory   ScanOrigin = "history"
	OriginDiscovery ScanOrigin = "discovery"
)

// RivalVideo là một video của kênh đối thủ.
type RivalVideo struct {
	ID            string
	ChannelID     string
	Title         string
	DurationSec   *int
	ViewCount     *int64
	PublishedText string
	PublishedAt   *time.Time // ước lượng từ chữ tương đối
	UploadedAt    *time.Time // giờ đăng chính xác; nil khi chưa đọc hoặc không có
	HasCaptions   *bool      // phụ đề do kênh đăng; nil khi chưa đọc
	Score         *float64   // lượt xem / mức thường; nil khi thiếu một trong hai
	Position      int64
	Origin        ScanOrigin
	FirstSeenAt   time.Time
}

// ScanJobKind là loại việc quét.
type ScanJobKind = string

const (
	ScanHistory   ScanJobKind = "history"
	ScanDiscovery ScanJobKind = "discovery"
)

// ScanJobState là trạng thái của một việc quét (giữ tập của yt-factory).
type ScanJobState = string

const (
	ScanQueued    ScanJobState = "queued"
	ScanRunning   ScanJobState = "running"
	ScanBackoff   ScanJobState = "backoff"
	ScanPaused    ScanJobState = "paused"
	ScanCapped    ScanJobState = "capped"
	ScanComplete  ScanJobState = "complete"
	ScanCancelled ScanJobState = "cancelled"
	ScanError     ScanJobState = "error"
)

// HistoryState là trạng thái quét lịch sử hiển thị cho một kênh: trạng thái
// của việc quét lịch sử nếu có, "idle" khi chưa có việc nào và chưa đọc hết
// tab, "complete" khi đã đọc hết tab.
type HistoryState = string

const HistoryIdle HistoryState = "idle"

// ScanAction là thao tác người dùng gửi tới việc quét của một kênh.
type ScanAction = string

const (
	ScanActionStart  ScanAction = "start"  // bắt đầu hoặc tiếp tục quét lịch sử
	ScanActionPause  ScanAction = "pause"  // tạm dừng quét lịch sử
	ScanActionResume ScanAction = "resume" // tiếp tục sau tạm dừng hoặc lỗi
	ScanActionCancel ScanAction = "cancel" // huỷ quét lịch sử, giữ video đã quét
	ScanActionCheck  ScanAction = "check"  // kiểm tra video mới ngay
)

// HistoryStatus là tiến độ quét lịch sử của một kênh.
type HistoryStatus struct {
	State         HistoryState
	JobID         string
	Scanned       int
	Added         int
	Pages         int
	ReachedEnd    bool
	StartedAt     *time.Time
	UpdatedAt     *time.Time
	NextAttemptAt *time.Time // backoff: lúc tự thử lại
	ErrorCode     string
	Error         string
	Recovery      string // câu hướng dẫn tiếng Việt khi State = error|paused do lỗi
}

// DiscoveryStatus là trạng thái kiểm tra video mới của một kênh.
type DiscoveryStatus struct {
	CheckedAt *time.Time
	CaughtUp  bool
	Running   bool
	NextAt    *time.Time // lần kiểm tra theo lịch tiếp theo
}

// ScanStatus là trạng thái quét của một kênh cho Khám phá và CLI.
type ScanStatus struct {
	ChannelID     string
	HistoryCap    int
	IndexedVideos int
	History       HistoryStatus
	Discovery     DiscoveryStatus
	RescanHours   int
}

// PostingZone là múi giờ xem khung đăng.
type PostingZone = string

const (
	ZoneVN PostingZone = "vn" // Asia/Ho_Chi_Minh
	ZoneNY PostingZone = "ny" // America/New_York
)

// PostingSample là số video mới nhất mỗi kênh được đọc giờ đăng chính xác.
const PostingSample = 30

// PostingRhythm là độ đều của một khung đăng (§12).
type PostingRhythm = string

const (
	RhythmWeekly    PostingRhythm = "weekly"
	RhythmMostWeeks PostingRhythm = "most_weeks"
	RhythmBiweekly  PostingRhythm = "biweekly"
	RhythmNew       PostingRhythm = "new"
)

// PostingSlot là một khung đăng lặp lại (§12): các thứ cùng khoảng giờ, cùng
// độ đều. Weekdays 0 = T2 … 6 = CN theo múi giờ đang xem; Start/End "HH:MM".
type PostingSlot struct {
	Weekdays   []int
	Start      string
	End        string
	Count      int
	WeeksHit   int
	WeeksTotal int
	SinceWeeks int // RhythmNew: số tuần gần nhất khung xuất hiện
	Rhythm     PostingRhythm
	IsTop      bool
}

// PostingSchedule là lịch đăng của một kênh theo một múi giờ (§12).
type PostingSchedule struct {
	ChannelID   string
	Zone        PostingZone
	Sample      int        // PostingSample
	Total       int        // số video trong mẫu đã có giờ đăng chính xác
	Pending     int        // số video trong mẫu chưa đọc giờ đăng
	PausedUntil *time.Time // việc đọc giờ đăng đang tạm nghỉ tới
	PerWeek     *float64   // số video mỗi tuần trong mẫu; nil khi < 2 video
	WeeksTotal  int
	Slots       []PostingSlot
	OffSlot     int
	Grid        [7][12]int // thứ × khối 2 giờ (0 = 00:00–02:00)
}

// VideoUsage là một kênh của bạn đã làm hoặc đang làm một video đối thủ —
// nguồn thật cho trạng thái trên thẻ Khám phá và cho dòng kênh bị khoá ở
// Video đã chọn. Chỉ job chưa bị thay thế/chưa huỷ và mục hàng chờ còn sống.
type VideoUsage struct {
	VideoID     string
	ChannelID   string
	ChannelName string
	Stage       string // queued|working|review|attention|paused|ready|posted
	ActiveStep  string // bước đang chạy khi Stage = working (vd "images")
	JobID       string // rỗng khi video mới ở hàng chờ
	QueueItemID string // rỗng khi mục hàng chờ đã bị dọn
	UpdatedAt   time.Time
}

// PickNote là lý do Tự lấy không lấy một video, chỉ dựng khi dữ liệu chứng
// minh được: video có giờ đăng chính xác trước mốc bật Tự lấy của kênh.
type PickNote struct {
	ChannelID   string
	ChannelName string
	BaselineAt  time.Time
	PublishedAt time.Time
}

// VideoChapter là một chương thật của video gốc (từ source.info.json).
type VideoChapter struct {
	StartSec float64
	Title    string
}
