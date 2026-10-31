package platforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Ngưỡng chờ bị chặn tăng dần: 15 phút cho lần đầu, nhân đôi mỗi lần
// chặn liên tiếp, trần 120 phút. Retry-After của server chỉ được kéo dài,
// không bao giờ rút ngắn.
const (
	pauseMinMin      = 15
	pauseMaxMin      = 120
	bilibiliSpacing  = 10 * time.Second
	bilibiliJitterMs = 10000
	// bilibiliYtDlpSpacing là khoảng cách giữa hai lượt trích yt-dlp của
	// Bilibili (5–10 giây, hẹp hơn 10–20 giây của lời gọi API).
	bilibiliYtDlpSpacing  = 5 * time.Second
	bilibiliYtDlpJitterMs = 5000
	scannerLease          = 30 * time.Minute
	// bilibiliScannerLease ngắn hơn: một trang danh sách xong trong vài giây.
	bilibiliScannerLease = 5 * time.Minute
	// ytDlpLeaseMargin là phần đệm dọn dẹp khi process chết giữa lệnh yt-dlp.
	ytDlpLeaseMargin = time.Minute
	// requestHistoryWindow là thời gian giữ lịch sử đặt chỗ (trần của mọi cửa sổ).
	requestHistoryWindow = 24 * time.Hour
)

// BilibiliScanLease là thời hạn lease quét của Bilibili (một trang xong trong
// vài giây; lease ngắn để process chết không giữ lâu).
const BilibiliScanLease = bilibiliScannerLease

// BilibiliWindows là hạn mức cuộn của Bilibili: 20 yêu cầu mỗi 10 phút và 200
// mỗi 24 giờ (chuyển từ pacing.ts).
var BilibiliWindows = []BudgetWindow{
	{Window: 10 * time.Minute, Limit: 20},
	{Window: 24 * time.Hour, Limit: 200},
}

// BudgetWindow là một cửa sổ hạn mức cuộn: tối đa Limit trọng số trong Window.
type BudgetWindow struct {
	Window time.Duration
	Limit  int
}

// WindowBudget là trạng thái của một cửa sổ tại thời điểm hỏi.
type WindowBudget struct {
	Window      time.Duration
	Used        int
	Limit       int
	AvailableAt time.Time // lúc cửa sổ nhả đủ chỗ cho yêu cầu kế tiếp
}

// BudgetReport là hạn mức của một nền tảng: từng cửa sổ và mốc sớm nhất được
// gọi tiếp (max của các cửa sổ).
type BudgetReport struct {
	Windows     []WindowBudget
	AvailableAt time.Time
}

// ErrPlatformUnknown đánh dấu id không thuộc registry (web map sang 404).
var ErrPlatformUnknown = errors.New("nền tảng không tồn tại")

// PausedError báo nền tảng đang trong giờ chờ bị chặn — caller hiện nhãn
// "Tạm chặn tới HH:MM" thay vì gọi tiếp. Không lộ chi tiết thô của nền tảng.
type PausedError struct {
	Platform domain.PlatformID
	Label    string
	Until    time.Time
	Reason   string
}

func (e *PausedError) Error() string {
	return fmt.Sprintf("%s đang tạm nghỉ. Có thể thử lại từ %s.",
		e.Label, e.Until.Format("15:04 02/01"))
}

// ErrorCode trả "rate_limited" để mảng gọi phân loại được mà không import
// gói này (cùng mã tools.YtDlpRateLimited).
func (e *PausedError) ErrorCode() string { return ErrCodeRateLimited }

// RetryAt là mốc thử lại sớm nhất để wire/CLI trả kèm 429.
func (e *PausedError) RetryAt() time.Time { return e.Until }

// BudgetError báo hạn mức cuộn đã hết — khác IP block: danh sách đã lưu vẫn
// dùng được và tự gọi tiếp khi cửa sổ nhả chỗ.
type BudgetError struct {
	Platform    domain.PlatformID
	Label       string
	AvailableAt time.Time
}

func (e *BudgetError) Error() string {
	return fmt.Sprintf("%s đã hết lượt yêu cầu an toàn. Có thể thử lại từ %s; danh sách đã lưu vẫn dùng được.",
		e.Label, e.AvailableAt.Format("15:04 02/01"))
}

// ErrorCode trả "rate_limited": hạn mức hết là một dạng bị giới hạn.
func (e *BudgetError) ErrorCode() string { return ErrCodeRateLimited }

// RetryAt là mốc cửa sổ hạn mức nhả chỗ để wire/CLI trả kèm 429.
func (e *BudgetError) RetryAt() time.Time { return e.AvailableAt }

// ScannerBusyError báo một lượt quét của nền tảng đang chạy (lease còn hạn).
// Caller xếp việc lại thay vì gọi song song.
type ScannerBusyError struct {
	Platform domain.PlatformID
	Label    string
}

func (e *ScannerBusyError) Error() string {
	return fmt.Sprintf("Một kênh %s đang được quét. Kênh này sẽ được đọc ở lượt tiếp theo.", e.Label)
}

// ErrorCode trả "failed" (chuyển từ BilibiliScannerBusyError của yt-factory).
func (e *ScannerBusyError) ErrorCode() string { return "failed" }

// RateLimited là giao diện mà lỗi "bị giới hạn yêu cầu" của một client nền
// tảng nào đó hiện thực để pacing nhận ra (kèm Retry-After nếu server trả).
// tools.YtDlpError không có Retry-After — pacing cũng kiểm mã "rate_limited"
// của nó qua interface CodedError bên dưới.
type RateLimited interface {
	error
	RetryAfter() time.Duration
}

// CodedError là lỗi mang mã chuỗi ổn định; pacing nhận "rate_limited" như một
// dạng bị chặn không kèm Retry-After (trường hợp yt-dlp).
type CodedError interface {
	error
	ErrorCode() string
}

// ErrCodeRateLimited là mã lỗi pacing nhận diện là bị giới hạn (trùng mã
// tools.YtDlpRateLimited — khai lại để không import gói công cụ).
const ErrCodeRateLimited = "rate_limited"

// SlotRequest là một lần xin đặt chỗ gọi nền tảng: khoảng cách tối thiểu tới
// lần kế tiếp, trọng số của lần gọi, và các cửa sổ hạn mức phải kiểm.
type SlotRequest struct {
	Spacing time.Duration
	Cost    int
	Windows []BudgetWindow
}

// SlotDecision là kết quả đặt chỗ: giờ được bắt đầu, hoặc lý do từ chối
// (đang chờ bị chặn / hết hạn mức). Không ghi gì khi từ chối.
type SlotDecision struct {
	StartAt     time.Time
	PausedUntil *time.Time
	PauseReason *string
	BudgetAt    *time.Time
}

// StateStore là phần lưu trữ mà platforms cần: một dòng trạng thái mỗi nền
// tảng. Hiện thực Postgres ở internal/persistence/postgres.
type StateStore interface {
	// All đọc trạng thái của mọi nền tảng đã có dòng (một truy vấn).
	All(ctx context.Context) ([]domain.PlatformState, error)
	// One đọc trạng thái một nền tảng; found=false khi chưa có dòng.
	One(ctx context.Context, id domain.PlatformID) (domain.PlatformState, bool, error)
	// Pause đọc giờ chờ còn hiệu lực; tự dọn các trường chờ đã hết hạn.
	Pause(ctx context.Context, id domain.PlatformID, now time.Time) (until *time.Time, reason *string, err error)
	// RecordBlocked tăng failureCount và đặt giờ chờ tăng dần; retryAfter chỉ
	// được kéo dài chứ không rút ngắn. Trả giờ chờ mới.
	RecordBlocked(ctx context.Context, id domain.PlatformID, now time.Time, reason string, retryAfter time.Duration) (domain.PlatformState, error)
	// ClearPause xoá giờ chờ và đếm chặn (đăng nhập mới hoặc thành công).
	ClearPause(ctx context.Context, id domain.PlatformID) error
	// RecordError ghi lỗi gần nhất cần người xử lý (vd cookies_failed).
	RecordError(ctx context.Context, id domain.PlatformID, code string, at time.Time, jobID *string) error
	// ClearError xoá lỗi gần nhất.
	ClearError(ctx context.Context, id domain.PlatformID) error
	// ReserveSlot đặt chỗ gọi kế tiếp; trả thời điểm được phép bắt đầu.
	ReserveSlot(ctx context.Context, id domain.PlatformID, now time.Time, spacing time.Duration) (startAt time.Time, err error)
	// ReserveWeighted đặt chỗ có trọng số: kiểm giờ chờ và hạn mức cuộn trong
	// cùng một transaction, ghi lịch sử khi nhận. Không ghi gì khi từ chối.
	ReserveWeighted(ctx context.Context, id domain.PlatformID, now time.Time, req SlotRequest) (SlotDecision, error)
	// AcquireScanner giữ lease quét của một nền tảng; trả false khi một
	// scanner khác còn hạn lease.
	AcquireScanner(ctx context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error)
	// ReleaseScanner nhả lease chỉ khi vẫn thuộc owner.
	ReleaseScanner(ctx context.Context, id domain.PlatformID, owner string) error
	// AcquireYtDlp giữ lease yt-dlp chéo process; false khi lease khác còn hạn.
	AcquireYtDlp(ctx context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error)
	// ExtendYtDlp kéo dài lease của mình tới mốc mới; false khi lease đã mất.
	ExtendYtDlp(ctx context.Context, id domain.PlatformID, owner string, until time.Time) (bool, error)
	// ReleaseYtDlp nhả lease chỉ khi vẫn thuộc owner.
	ReleaseYtDlp(ctx context.Context, id domain.PlatformID, owner string) error
	// SetNextTrickle ghi mốc sớm nhất được đọc trang danh sách kế tiếp.
	SetNextTrickle(ctx context.Context, id domain.PlatformID, at time.Time) error
}

// ErrInvalidLoginBody báo body PUT .../login không khớp cách đăng nhập của
// nền tảng (web map sang 400 reason=invalid).
var ErrInvalidLoginBody = errors.New("body đăng nhập không khớp nền tảng")

// ErrLoginUnsupported báo nền tảng không có thao tác đăng nhập này (web map
// sang 400 reason=unsupported).
var ErrLoginUnsupported = errors.New("nền tảng không có thao tác đăng nhập này")
