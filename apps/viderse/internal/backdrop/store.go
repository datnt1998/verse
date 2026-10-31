package backdrop

import (
	"context"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

// Store là phần lưu trữ của thư viện Video nền: kênh nền, danh mục video của
// kênh, video lẻ, và các truy vấn chỉ đọc trên template + platform_requests.
// Mọi ghi nhiều dòng đi qua WithTx.
type Store interface {
	// WithTx chạy fn trên một store gắn transaction; lỗi từ fn rollback.
	WithTx(ctx context.Context, fn func(store Store) error) error

	// Channel đọc một kênh; found=false khi chưa có.
	Channel(ctx context.Context, id string) (domain.BackdropChannel, bool, error)
	// Channels đọc mọi kênh, mới thêm trước.
	Channels(ctx context.Context) ([]domain.BackdropChannel, error)
	// InsertChannel chèn kênh nếu chưa có; created=false khi đã tồn tại.
	InsertChannel(ctx context.Context, channel domain.BackdropChannel) (bool, error)
	// UpdateChannelList ghi kết quả lần đọc danh sách: errCode rỗng là thành
	// công (đặt listed_at = now) và xoá lỗi cũ.
	UpdateChannelList(ctx context.Context, id string, now time.Time, errCode, errMessage string) error
	// UpdateChannelScan ghi tiến độ đọc dần (Bilibili).
	UpdateChannelScan(ctx context.Context, id string, nextPage *int, stopKnown bool, total int, title *string) error
	// UpdateChannelTitle ghi tên hiển thị khi dò được uploader.
	UpdateChannelTitle(ctx context.Context, id string, title string) error
	// DeleteChannel xoá kênh và danh mục của nó; existed=false khi không có.
	DeleteChannel(ctx context.Context, id string) (bool, error)

	// ChannelVideos đọc mọi dòng danh mục của một kênh theo loại rồi thứ hạng.
	ChannelVideos(ctx context.Context, channelID string) ([]domain.BackdropChannelVideo, error)
	// ChannelVideo đọc một dòng danh mục.
	ChannelVideo(ctx context.Context, channelID, videoID string) (domain.BackdropChannelVideo, bool, error)
	// UpsertVideos ghi một lượt đọc danh sách: trường đã dò giữ nguyên, trường
	// của danh sách thắng, lý do tự hết thì xoá.
	UpsertVideos(ctx context.Context, channelID string, videos []domain.BackdropChannelVideo) error
	// ExpireMissingVods đánh dấu expired cho VOD vắng mặt trong danh sách trọn.
	ExpireMissingVods(ctx context.Context, channelID string, keep []string, now time.Time) error
	// MarkChannelVideoUnavailable đánh dấu một video không dùng được.
	MarkChannelVideoUnavailable(ctx context.Context, channelID, videoID, reason string, now time.Time) error
	// UpdateChannelVideoProbe ghi kết quả dò một video.
	UpdateChannelVideoProbe(ctx context.Context, channelID, videoID string, probe ProbeUpdate) error
	// UsableCounts đếm video còn dùng được của nhiều kênh theo loại.
	UsableCounts(ctx context.Context, channelIDs []string) (map[string]map[domain.PlatformKind]int, error)
	// PageChannelVideos đọc một trang danh mục theo keyset.
	PageChannelVideos(ctx context.Context, query PageQuery) ([]domain.BackdropChannelVideo, error)
	// CountChannelVideos đếm mọi dòng khớp bộ lọc (kể cả không dùng được).
	CountChannelVideos(ctx context.Context, channelID string, kinds []domain.PlatformKind) (int, error)
	// CountUnavailableVideos đếm dòng không dùng được khớp bộ lọc.
	CountUnavailableVideos(ctx context.Context, channelID string, kinds []domain.PlatformKind) (int, error)

	// Videos đọc mọi video lẻ, mới thêm trước.
	Videos(ctx context.Context) ([]domain.BackdropVideo, error)
	// Video đọc một video lẻ.
	Video(ctx context.Context, id string) (domain.BackdropVideo, bool, error)
	// InsertVideo chèn nếu chưa có; created=false khi đã tồn tại.
	InsertVideo(ctx context.Context, video domain.BackdropVideo) (bool, error)
	// UpdateVideo ghi các trường đang đổi của một video lẻ.
	UpdateVideo(ctx context.Context, id string, patch VideoPatch) error
	// DeleteVideo xoá; existed=false khi không có.
	DeleteVideo(ctx context.Context, id string) (bool, error)

	// ClaimDownload giành lượt tải: tăng generation, đặt owner + lease, đưa
	// state về downloading. Chỉ giành được khi chưa ai giữ (lease hết hạn /
	// state khác downloading) hoặc force=true (PATCH đoạn dùng). claimed=false
	// khi process khác đang tải.
	ClaimDownload(ctx context.Context, id, owner string, leaseUntil, now time.Time, force bool) (int64, bool, error)
	// TouchDownload ghi tiến độ + gia hạn lease, chỉ khi còn đúng owner+generation.
	TouchDownload(ctx context.Context, id, owner string, generation int64, progress *float64, leaseUntil, now time.Time) (bool, error)
	// FinishDownload ghi trạng thái ready, chỉ khi còn đúng owner+generation.
	FinishDownload(ctx context.Context, id, owner string, generation int64, filePath string, height int, size int64, now time.Time) (bool, error)
	// FailDownload ghi trạng thái failed, chỉ khi còn đúng owner+generation.
	FailDownload(ctx context.Context, id, owner string, generation int64, code, message string, now time.Time) (bool, error)
	// ClearStaleDownloads đánh dấu failed cho lượt tải mồ côi (lease hết / chưa
	// từng có lease) để người dùng thêm lại được.
	ClearStaleDownloads(ctx context.Context, now time.Time) (int, error)

	// LockResource giữ khoá transaction theo tài nguyên mà template tham chiếu;
	// lượt lưu template phải lấy CÙNG khoá này trong transaction của nó để
	// kiểm tra tồn tại + ghi không chen giữa kiểm và xoá.
	LockResource(ctx context.Context, key string) error
	// NotifyChange phát LISTEN/NOTIFY để process khác (CLI/serve) đánh thức bộ
	// lập lịch ngay thay vì chờ hết giờ ngủ.
	NotifyChange(ctx context.Context, key string) error

	// Templates đọc id, tên và recipe của mọi template (chỉ đọc).
	Templates(ctx context.Context) ([]TemplateRecipeRow, error)
	// RecordPlatformRequest ghi một yêu cầu hỗ trợ nền tảng (idempotent theo host).
	RecordPlatformRequest(ctx context.Context, host, label string, now time.Time) (PlatformRequestRecord, error)
	// PlatformRequests đọc mọi yêu cầu hỗ trợ đã ghi.
	PlatformRequests(ctx context.Context) ([]PlatformRequestRecord, error)
}

// ChannelLockKey là khoá transaction cho một kênh nền.
func ChannelLockKey(id string) string { return "backdrop_channel:" + id }

// VideoLockKey là khoá transaction cho một video lẻ.
func VideoLockKey(id string) string { return "backdrop_video:" + id }

// ProbeUpdate là kết quả dò một video: trường nil giữ nguyên giá trị cũ.
type ProbeUpdate struct {
	Title            *string
	DurationSec      *float64
	ThumbnailURL     *string
	PublishedAt      *time.Time
	ProbedAt         time.Time
	ProbeInfo        []byte // chỉ Bilibili
	ClearUnavailable bool
}

// VideoPatch đổi các trường của một video lẻ; nil giữ nguyên.
type VideoPatch struct {
	State         *domain.BackdropVideoState
	Progress      *float64
	ClearProgress bool
	FilePath      *string
	FileHeight    *int
	FileBytes     *int64
	ErrorCode     *string
	Error         *string
	Segment       *domain.BackdropSegment
	Title         *string
	DurationSec   *float64
}

// PageQuery là một trang danh mục theo keyset.
type PageQuery struct {
	ChannelID string
	Kinds     []domain.PlatformKind
	Cursor    *ChannelVideoCursor
	Limit     int
}

// ChannelVideoCursor là mốc keyset của một trang danh mục: khoá sắp xếp là
// (published_at DESC NULLS LAST, video_id ASC).
type ChannelVideoCursor struct {
	ChannelID   string
	KindsKey    string
	PublishedAt *time.Time
	VideoID     string
}

// TemplateRecipeRow là phần template mà mảng Video nền cần đọc.
type TemplateRecipeRow struct {
	ID     string
	Name   string
	Recipe []byte
}

// PlatformRequestRecord là một dòng yêu cầu hỗ trợ nền tảng.
type PlatformRequestRecord struct {
	Host    string
	Label   string
	Count   int
	FirstAt time.Time
	LastAt  time.Time
}

// IngestParams gói một lượt ghi danh mục.
type IngestParams struct {
	Channel      domain.BackdropChannel
	Insert       bool
	Videos       []domain.BackdropChannelVideo
	VodsComplete bool
	Now          time.Time
}

// BilibiliStatus là trạng thái đọc dần của Bilibili cho Library.
type BilibiliStatus struct {
	Budget      platforms.BudgetReport
	PausedUntil *time.Time
	PauseReason *string
	NextPageAt  *time.Time
	Pending     []PendingChannel
}

// PendingChannel là một kênh Bilibili còn trang chưa đọc.
type PendingChannel struct {
	ChannelID string
	NextPage  *int
	MaxPages  int
}

// ChannelView là một kênh nền kèm số đếm, trạng thái chờ và template đang dùng.
type ChannelView struct {
	Channel     domain.BackdropChannel
	Counts      map[domain.PlatformKind]int
	PausedUntil *time.Time
	PauseReason *string
	UsedBy      []TemplateUsage
	Bilibili    *BilibiliStatus
}

// TemplateUsage là một template đang dùng kênh/video nền.
type TemplateUsage struct {
	TemplateID  string
	Name        string
	Mode        string // "random" | "pick"
	Kinds       []domain.PlatformKind
	ChosenCount int
}

// TemplateRef là một template ghim một video cụ thể.
type TemplateRef struct {
	TemplateID string
	Name       string
}

// VideoItem là một dòng danh mục kèm template ghim nó.
type VideoItem struct {
	Video    domain.BackdropChannelVideo
	ChosenBy []TemplateRef
}

// VideoPage là một trang danh mục của một kênh.
type VideoPage struct {
	Items       []VideoItem
	Counts      map[domain.PlatformKind]int
	Total       int
	Unavailable int
	NextCursor  string
}

// VideoView là một video lẻ kèm template đang dùng.
type VideoView struct {
	Video  domain.BackdropVideo
	UsedBy []TemplateRef
}
