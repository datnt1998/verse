package domain

import "time"

// Từ vựng của "Video nền" (hợp đồng dữ liệu §24): kênh nền (danh mục video
// đọc từ Twitch/Bilibili), video lẻ (một đoạn dùng của một video cụ thể) và
// kết quả phân loại một link dán vào. Chỉ kiểu giá trị — không I/O.

// BackdropUnavailableReason là lý do một video trong danh mục không dùng được.
// expired và live tự xoá khi lần đọc danh sách sau thấy video đó trở lại.
type BackdropUnavailableReason = string

const (
	BackdropExpired       BackdropUnavailableReason = "expired"
	BackdropLive          BackdropUnavailableReason = "live"
	BackdropUnavailable   BackdropUnavailableReason = "unavailable"
	BackdropLoginRequired BackdropUnavailableReason = "login_required"
)

// SelfClearingBackdropReason đúng khi lý do không dùng được tự hết ở lần đọc
// danh sách kế tiếp (video còn trong danh sách thì lại dùng được).
func SelfClearingBackdropReason(reason BackdropUnavailableReason) bool {
	return reason == BackdropExpired || reason == BackdropLive
}

// BackdropChannel là một kênh nền trong thư viện: kênh Twitch hoặc trang cá
// nhân Bilibili kèm trạng thái lần đọc danh sách gần nhất.
type BackdropChannel struct {
	ID            string     // "twitch:<login>" | "bilibili:<mid>"
	Platform      PlatformID // twitch | bilibili
	Handle        string     // login Twitch (chữ thường) hoặc mid Bilibili
	URL           string     // địa chỉ gốc của kênh
	Title         *string    // tên hiển thị; nil = chưa đọc được
	AvatarURL     *string
	AddedAt       time.Time
	ListedAt      *time.Time // lần đọc danh sách thành công gần nhất
	ListErrorCode string     // mã lỗi của lần đọc gần nhất (rỗng = không lỗi)
	ListError     string     // câu lỗi của lần đọc gần nhất
	ScanNextPage  *int       // trang kế tiếp cần đọc (Bilibili); nil = đã đọc hết
	ScanTotal     int        // tổng số video nền tảng báo (0 = chưa biết)
	ScanStopKnown bool       // trang kế đã gặp toàn video đã biết
}

// BackdropChannelVideo là một dòng danh mục của một kênh nền. Không có tệp
// media: việc tải clip thuộc bước dựng video.
type BackdropChannelVideo struct {
	ChannelID         string
	VideoID           string
	Kind              PlatformKind
	URL               string
	Title             string
	DurationSec       *float64
	ThumbnailURL      *string
	PublishedAt       *time.Time
	Position          int
	ListedAt          time.Time
	ProbedAt          *time.Time
	ProbeInfo         []byte // JSON yt-dlp đã kiểm (chỉ giữ cho Bilibili)
	UnavailableAt     *time.Time
	UnavailableReason BackdropUnavailableReason
}

// BackdropVideoState là trạng thái tải của một video lẻ.
type BackdropVideoState = string

const (
	BackdropDownloading BackdropVideoState = "downloading"
	BackdropReady       BackdropVideoState = "ready"
	BackdropFailed      BackdropVideoState = "failed"
)

// BackdropSegment là đoạn dùng của một video lẻ, tính bằng giây nguồn.
// ToSec−FromSec ≤ 600 (BackdropMaxSegmentSec).
type BackdropSegment struct {
	FromSec float64
	ToSec   float64
}

// Duration trả độ dài đoạn dùng tính bằng giây.
func (s BackdropSegment) Duration() float64 {
	if s.ToSec <= s.FromSec {
		return 0
	}
	return s.ToSec - s.FromSec
}

// BackdropVideo là một "Video lẻ": một video cụ thể kèm đoạn dùng, tải về
// bằng yt-dlp để luôn có mặt trong nền của template đang gắn nó.
type BackdropVideo struct {
	ID          string // "<platform>:<videoId>"
	Platform    PlatformID
	URL         string
	Title       string
	DurationSec *float64
	Segment     BackdropSegment
	FilePath    string // đường dẫn tương đối dưới thư mục dự án; rỗng = chưa có
	FileHeight  int
	FileBytes   int64
	State       BackdropVideoState
	Progress    *float64 // 0..1 khi đang tải
	ErrorCode   string
	Error       string
	AddedAt     time.Time
	UpdatedAt   time.Time

	// Chủ sở hữu lượt tải (dùng nội bộ store): mỗi lượt tăng generation; ghi
	// tiến độ/kết thúc phải khớp owner + generation.
	DownloadGeneration int64
	DownloadOwner      *string
	DownloadLeaseUntil *time.Time
}

// ParsedSourceKind là loại kết quả của một link dán vào ô thêm nguồn nền
// (§24 ParsedSource).
type ParsedSourceKind = string

const (
	SourceChannel     ParsedSourceKind = "channel"
	SourceVideo       ParsedSourceKind = "video"
	SourceUnsupported ParsedSourceKind = "unsupported"
	SourceInvalid     ParsedSourceKind = "invalid"
)

// ParsedSource là kết quả phân loại một link: kênh nền, video lẻ, nền tảng
// chưa hỗ trợ (kèm tên dễ đọc để mời gửi yêu cầu), hoặc link lỗi kèm gợi ý.
// Một kết quả chỉ mang các trường của loại mình.
type ParsedSource struct {
	Kind ParsedSourceKind
	// channel | video
	Platform    PlatformID
	ID          string
	Handle      string  // channel: login/mid
	URL         string  // channel: địa chỉ gốc; video: địa chỉ video
	Title       *string // video: chỉ khi biết mà không cần gọi mạng
	DurationSec *float64
	// unsupported
	Host  string
	Label string
	// invalid
	Message  string
	Recovery string
}

// RecipeBackdropChannel là một dòng "Lấy nền từ" trong recipe.backdrop của
// template: kênh nền + loại video nhận + video ghim (rỗng = Ngẫu nhiên).
type RecipeBackdropChannel struct {
	ChannelID string   `json:"channelId"`
	Kinds     []string `json:"kinds"`
	Pinned    []string `json:"pinned"`
}

// RecipeBackdrop là phần recipe.backdrop của một template (hình do internal
// library sở hữu; backdrop chỉ đọc để tính usedBy/chosenBy và mix).
type RecipeBackdrop struct {
	Channels      []RecipeBackdropChannel `json:"channels"`
	ClipCount     *int                    `json:"clipCount"`
	ClipSec       int                     `json:"clipSec"`
	FixedVideoIDs []string                `json:"fixedVideoIds"`
}
