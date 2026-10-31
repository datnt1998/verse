package domain

import "time"

// Từ vựng của mảng nền tảng nguồn (hợp đồng dữ liệu §24, §24.1): nền tảng app
// lấy video từ đó, trạng thái tạm chặn/tự phục hồi của từng nền tảng, và hình
// đăng nhập chỉ nằm trên máy này. Chỉ kiểu giá trị — không I/O.

// PlatformID định danh một nền tảng nguồn; tập giá trị sinh từ registry của
// gói platforms (thêm nền tảng = thêm một mục registry).
type PlatformID = string

const (
	PlatformYouTube  PlatformID = "youtube"
	PlatformTwitch   PlatformID = "twitch"
	PlatformBilibili PlatformID = "bilibili"
)

// PlatformUse là việc app dùng nền tảng cho: đối thủ (kênh mình đối chiếu) hay
// nguồn nền (video lấy cảnh).
type PlatformUse = string

const (
	PlatformUseCompetitor PlatformUse = "competitor"
	PlatformUseBackdrop   PlatformUse = "backdrop"
)

// PlatformKind là loại video một nền tảng cung cấp cho Thư viện › Video nền.
type PlatformKind = string

const (
	PlatformKindClip      PlatformKind = "clip"
	PlatformKindHighlight PlatformKind = "highlight"
	PlatformKindVod       PlatformKind = "vod"
	PlatformKindVideo     PlatformKind = "video"
)

// PlatformLoginType là cách đăng nhập của một nền tảng (§24.1): không cần,
// cookie đọc từ hồ sơ trình duyệt, cookie dán tay, hoặc mã QR.
type PlatformLoginType = string

const (
	LoginNone          PlatformLoginType = "none"
	LoginBrowserCookie PlatformLoginType = "browser_cookie"
	LoginCookieString  PlatformLoginType = "cookie_string"
	LoginQR            PlatformLoginType = "qr"
)

// PlatformCondition là trạng thái hiển thị của một nền tảng (§24): sẵn sàng,
// đang tạm chặn tới một giờ, hay cần đăng nhập lại.
type PlatformCondition = string

const (
	PlatformReady      PlatformCondition = "ready"
	PlatformPaused     PlatformCondition = "paused"
	PlatformNeedsLogin PlatformCondition = "needs_login"
)

// PlatformState là bản ghi trạng thái bền của một nền tảng (chuyển từ
// backdrop_platform_state): nhịp gọi, giờ chờ bị chặn tăng dần, số lần chặn
// liên tiếp, lease của luồng quét, lịch sử yêu cầu trong một ngày (hạn mức
// cuộn), mốc đọc dần trang kế tiếp, lease yt-dlp và lỗi gần nhất cần người xử lý.
type PlatformState struct {
	Platform          PlatformID
	NextRequestAt     time.Time
	PausedUntil       *time.Time
	PauseReason       *string
	FailureCount      int
	ScannerOwner      *string
	ScannerLeaseUntil *time.Time
	LastErrorCode     string
	LastErrorAt       *time.Time
	LastErrorJobID    *string
	// RequestHistory là các lần đặt chỗ trong 24 giờ gần nhất, mỗi phần tử
	// {At, Cost} — hạn mức cuộn của Bilibili tính trên đây.
	RequestHistory []PlatformRequest
	// NextTrickleAt là mốc sớm nhất được đọc trang danh sách kế tiếp (Bilibili).
	NextTrickleAt time.Time
	// YtdlpOwner/YtdlpLeaseUntil là lease chéo process cho lệnh yt-dlp trên
	// nền tảng đó (một lệnh mỗi lúc).
	YtdlpOwner      *string
	YtdlpLeaseUntil *time.Time
}

// PlatformRequest là một lần đặt chỗ gọi nền tảng: thời điểm bắt đầu và trọng
// số (một trang = 1, một lượt trích yt-dlp = 4, đọc lại info đã lưu = 1).
type PlatformRequest struct {
	At   time.Time
	Cost int
}

// PlatformLastError là lỗi nền tảng gần nhất cần người xử lý (§24): hiện giờ
// chỉ có YouTube báo "xác nhận bạn không phải bot" (cookies_failed).
type PlatformLastError struct {
	Code  string
	At    time.Time
	JobID *string
}

// PlatformLogin là hình đăng nhập của một nền tảng (§24.1). Saved=false với
// login không cần thiết (Twitch) vẫn hợp lệ; Method rỗng nghĩa là chưa đăng
// nhập; Renewable = đăng nhập bằng QR có refresh token để tự gia hạn.
type PlatformLogin struct {
	Type          PlatformLoginType
	Saved         bool
	Masked        *string
	Account       *string
	Browser       string
	Profile       string
	Method        string // "qr" | "manual" | ""
	Renewable     bool
	LastRefreshAt *time.Time
	RefreshError  *string
}

// PlatformUsage là số nguồn đang dùng của một nền tảng (§24.usage). Rivals là
// con trỏ vì chỉ một số nền tảng có khái niệm đối thủ.
type PlatformUsage struct {
	Channels int
	Videos   int
	Rivals   *int
}

// PlatformStatus là view-model đầy đủ của một nền tảng cho GET /api/platforms:
// danh tính registry + đăng nhập + trạng thái bền + số nguồn đang dùng.
type PlatformStatus struct {
	ID           PlatformID
	Label        string
	Uses         []PlatformUse
	Kinds        []PlatformKind
	TargetHeight int
	Login        PlatformLogin
	State        PlatformCondition
	PausedUntil  *time.Time
	PauseReason  *string
	FailureCount int
	LastError    *PlatformLastError
	Usage        PlatformUsage
}

// BrowserProfile là một hồ sơ trình duyệt Chromium đọc được trên máy (§24,
// cookie YouTube): Profile là tên thư mục dưới user-data dir ("Default",
// "Profile 1"), Name là tên hiển thị, Email là tài khoản đã đăng nhập nếu có.
type BrowserProfile struct {
	Browser string
	Profile string
	Name    string
	Email   string
}

// QRSession là phiên đăng nhập bằng mã QR vừa tạo (§24.1).
type QRSession struct {
	QRCodeKey string
	QRImage   string // data:image/svg+xml;base64,…
	LoginURL  string
	ExpiresAt time.Time
}

// QRPollStatus là trạng thái của một lần hỏi mã QR (§24.1).
type QRPollStatus = string

const (
	QRWaiting  QRPollStatus = "waiting"
	QRScanned  QRPollStatus = "scanned"
	QRExpired  QRPollStatus = "expired"
	QRSuccess  QRPollStatus = "success"
	QRRejected QRPollStatus = "rejected"
)

// QRPoll là kết quả hỏi mã QR; Account chỉ có khi đăng nhập thành công và
// Bilibili xác nhận được tên tài khoản.
type QRPoll struct {
	Status  QRPollStatus
	Account *string
}

// RefreshOutcome là kết quả của một lần gia hạn đăng nhập (§24.1).
type RefreshOutcome = string

const (
	RefreshRefreshed RefreshOutcome = "refreshed"
	RefreshCurrent   RefreshOutcome = "current"
	RefreshSkipped   RefreshOutcome = "skipped"
	RefreshFailed    RefreshOutcome = "failed"
)
