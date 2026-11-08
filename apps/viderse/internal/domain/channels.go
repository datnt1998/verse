package domain

import "time"

// Từ vựng của Kênh của bạn (07 §8, §16): kênh do chủ sở hữu, đối thủ liên
// kết, template mặc định của Thư viện và bộ nhận diện (tên + avatar + ảnh bìa
// kênh) do trợ lý tạo. Chỉ kiểu giá trị — không I/O.

// ChannelLanguage là ngôn ngữ tiêu đề/mô tả/tag của kênh; "auto" = theo ngôn
// ngữ video nguồn.
type ChannelLanguage = string

const (
	LangAuto ChannelLanguage = "auto"
	LangEN   ChannelLanguage = "en"
	LangVI   ChannelLanguage = "vi"
)

// IdentityState là trạng thái của lần tạo nhận diện: idle = không có run đang
// sống (nhận diện có thể đã có hoặc chưa), running = đang tạo, failed = run
// gần nhất hỏng (kèm code + câu lỗi + giờ tự thử lại nếu có).
type IdentityState = string

const (
	IdentityIdle    IdentityState = "idle"
	IdentityRunning IdentityState = "running"
	IdentityFailed  IdentityState = "failed"
)

// IdentityPart là phần nhận diện được tạo lại: "all" = tên + avatar + ảnh bìa,
// "name" = chỉ đặt tên + làm mới brief ảnh, "avatar"/"banner" = vẽ lại một ảnh
// từ brief đã lưu.
type IdentityPart = string

const (
	IdentityAll    IdentityPart = "all"
	IdentityName   IdentityPart = "name"
	IdentityAvatar IdentityPart = "avatar"
	IdentityBanner IdentityPart = "banner"
)

// Mã lỗi của lần tạo nhận diện (identity_error_code), theo các code ổn định
// của đường ảnh; thêm "interrupted" cho run chết giữa chừng (mất điện, tắt
// app, mất lease).
const (
	IdentityErrInterrupted = "interrupted"
)

// Bước của một run nhận diện, ghi lại khi run hỏng để UI chỉ đúng trợ lý.
const (
	IdentityStepText   = "text"   // Đạo diễn đặt tên + viết mô tả
	IdentityStepImages = "images" // Hoạ sĩ vẽ avatar / ảnh bìa
)

// Branding là bối cảnh nhận diện đã công bố (07 §16): brief = Mô tả kênh (chủ
// sửa được), summary = câu tiếng Việt hiện ở "Phong cách nhận diện", look =
// phong cách dùng chung cho avatar, ảnh bìa và ảnh video; avatar/banner là
// brief ảnh tiếng Anh mà mỗi tài sản được vẽ ra.
type Branding struct {
	Brief   string `json:"brief,omitempty"`
	Summary string `json:"summary,omitempty"`
	Look    string `json:"look,omitempty"`
	Avatar  string `json:"avatar,omitempty"`
	Banner  string `json:"banner,omitempty"`
}

// Channel là một kênh của chủ (dòng owner_channels).
type Channel struct {
	ID                string
	Name              string
	YoutubeURL        *string
	Language          ChannelLanguage
	StyleNote         *string
	DescriptionFooter *string
	AlwaysTags        []string
	// DefaultTemplateID là template mặc định của kênh trong Thư viện (FK bắt
	// buộc; nhiều kênh được dùng chung một template).
	DefaultTemplateID string
	// AutoFetch bật/tắt Tự lấy video mới của đối thủ; AutoApprove bật/tắt tự
	// duyệt ảnh khi mọi ảnh đạt ngưỡng cố định 85.
	AutoFetch   bool
	AutoApprove bool
	PerDay      int
	// AgentInstructions là chỉ dẫn riêng của chủ kênh cho từng trợ lý
	// (scout|director|artist|qa). Khoá vắng = không có chỉ dẫn. Đây KHÔNG phải
	// Giọng văn (StyleNote) hay Mô tả kênh (Branding.Brief).
	AgentInstructions  AgentInstructions
	Branding           Branding
	AvatarPath         *string // tương đối thư mục dự án: channels/<id>/<tệp>
	BannerPath         *string
	IdentityState      IdentityState
	IdentityPart       *string
	IdentityTaskID     *string
	IdentityErrorCode  *string
	IdentityError      *string
	IdentityFailedStep *string // IdentityStepText | IdentityStepImages; nil = không rõ
	IdentityRetryAt    *time.Time
	IdentityStartedAt  *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ChannelSource là một đối thủ liên kết với kênh (link order = linked_at; dòng
// đầu là đối thủ tham chiếu nhận diện).
type ChannelSource struct {
	RivalID   string
	Title     string
	Handle    *string
	LinkedAt  time.Time
	Reference bool
	// MinDurationSec là bộ lọc Tự lấy theo đối thủ: chỉ lấy video dài từ ngần
	// này giây; nil = không lọc.
	MinDurationSec *int
}

// ChannelTemplate là một template của Thư viện được kênh tham chiếu (mặc định
// của kênh hoặc override cho một lần tạo).
type ChannelTemplate struct {
	ID   string
	Name string
}
