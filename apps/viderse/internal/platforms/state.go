package platforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Ngưỡng chờ bị chặn tăng dần (W13): 15 phút cho lần đầu, nhân đôi mỗi lần
// chặn liên tiếp, trần 120 phút. Retry-After của server chỉ được kéo dài,
// không bao giờ rút ngắn.
const (
	pauseMinMin      = 15
	pauseMaxMin      = 120
	bilibiliSpacing  = 5 * time.Second
	bilibiliJitterMs = 1000
	scannerLease     = 30 * time.Minute
)

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
	// AcquireScanner giữ lease quét của một nền tảng; trả false khi một
	// scanner khác còn hạn lease.
	AcquireScanner(ctx context.Context, id domain.PlatformID, owner string, now, until time.Time) (bool, error)
	// ReleaseScanner nhả lease chỉ khi vẫn thuộc owner.
	ReleaseScanner(ctx context.Context, id domain.PlatformID, owner string) error
}

// ErrInvalidLoginBody báo body PUT .../login không khớp cách đăng nhập của
// nền tảng (web map sang 400 reason=invalid).
var ErrInvalidLoginBody = errors.New("body đăng nhập không khớp nền tảng")

// ErrLoginUnsupported báo nền tảng không có thao tác đăng nhập này (web map
// sang 400 reason=unsupported).
var ErrLoginUnsupported = errors.New("nền tảng không có thao tác đăng nhập này")
