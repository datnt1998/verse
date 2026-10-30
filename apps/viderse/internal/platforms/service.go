package platforms

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// LoginInfo là phần đọc của mảng accounts mà platforms cần để dựng
// PlatformStatus.login. Trả về hình đăng nhập với Type để trống — platforms
// điền theo registry. Hiện thực: internal/accounts.
type LoginInfo interface {
	// Login đọc hình đăng nhập của một nền tảng (Type rỗng).
	Login(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error)
}

// LoginActions là mặt thao tác của mảng accounts theo §24.1; platforms gọi
// sau khi đã kiểm id và cách đăng nhập của nền tảng đó trong registry.
type LoginActions interface {
	LoginInfo
	// Profiles liệt kê hồ sơ trình duyệt Chromium đọc được (chỉ đọc).
	Profiles(ctx context.Context, refresh bool) ([]domain.BrowserProfile, error)
	// SetCookieSource bật cookie YouTube từ một hồ sơ trình duyệt; nil tắt.
	SetCookieSource(ctx context.Context, browser, profile string) (domain.PlatformLogin, error)
	// SaveManualCookie kiểm tra rồi lưu cookie Bilibili dán tay.
	SaveManualCookie(ctx context.Context, cookie string) (domain.PlatformLogin, error)
	// ClearLogin xoá mọi đăng nhập của một nền tảng.
	ClearLogin(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error)
	// StartQR tạo một phiên mã QR mới (Bilibili).
	StartQR(ctx context.Context) (domain.QRSession, error)
	// PollQR hỏi trạng thái một phiên mã QR.
	PollQR(ctx context.Context, key string) (domain.QRPoll, error)
	// Refresh gia hạn ngay đăng nhập QR (force).
	Refresh(ctx context.Context) (domain.RefreshOutcome, domain.PlatformLogin, error)
	// Check kiểm tra lại đăng nhập đang lưu của một nền tảng.
	Check(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error)
}

// UsageCounter là bộ đếm số nguồn đang dùng của từng nền tảng (kênh nền,
// video lẻ, đối thủ) — do các mảng Thư viện/Khám phá của phase sau hiện thực.
// Không có nguồn nào thì trả số 0 thật, không phải giá trị giả.
type UsageCounter interface {
	// Usage trả số đếm cho mọi nền tảng một lần gọi (tránh N+1).
	Usage(ctx context.Context) (map[domain.PlatformID]domain.PlatformUsage, error)
}

// zeroUsage là UsageCounter mặc định khi chưa có mảng nào cung cấp số đếm.
type zeroUsage struct{}

func (zeroUsage) Usage(context.Context) (map[domain.PlatformID]domain.PlatformUsage, error) {
	return map[domain.PlatformID]domain.PlatformUsage{}, nil
}

// Deps là phụ thuộc của Service: mọi mặt ngoài (store, accounts, đồng hồ,
// đếm usage, phát sự kiện) đều qua interface/hàm tiêm vào.
type Deps struct {
	Store  StateStore
	Logins LoginActions
	Usage  UsageCounter // nil → mọi số đếm 0
	Now    func() time.Time
	Notify func(ref string) // phát sự kiện "platforms"; nil → không phát
}

// Service là mặt nghiệp vụ của mảng nền tảng: trạng thái bền, pacing và
// điều phối đăng nhập theo registry. An toàn cho goroutine.
type Service struct {
	d      Deps
	usage  UsageCounter
	now    func() time.Time
	notify func(string)
}

// New dựng service; các phụ thuộc bắt buộc thiếu thì panic sớm ở bootstrap.
func New(d Deps) *Service {
	if d.Store == nil || d.Logins == nil {
		panic("platforms.New: thiếu Store hoặc Logins")
	}
	s := &Service{d: d, usage: d.Usage, now: d.Now, notify: d.Notify}
	if s.usage == nil {
		s.usage = zeroUsage{}
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.notify == nil {
		s.notify = func(string) {}
	}
	return s
}

// Status dựng view-model của mọi nền tảng trong registry cho GET
// /api/platforms: một truy vấn trạng thái + một lượt đếm usage, đăng nhập
// lấy theo nền tảng từ accounts.
func (s *Service) Status(ctx context.Context) ([]domain.PlatformStatus, error) {
	states, err := s.d.Store.All(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[domain.PlatformID]domain.PlatformState, len(states))
	for _, st := range states {
		byID[st.Platform] = st
	}
	usage, err := s.usage.Usage(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	descs := Registry()
	out := make([]domain.PlatformStatus, 0, len(descs))
	for _, d := range descs {
		st := byID[d.ID]
		// Giờ chờ đã qua thì không còn nghĩa: chuẩn hoá trong read model theo
		// đồng hồ tiêm, không ghi ngược — nền tảng hiện "sẵn sàng"/cần đăng
		// nhập ngay lập tức dù chưa có lượt gọi nào dọn trường.
		if st.PausedUntil != nil && !st.PausedUntil.After(now) {
			st.PausedUntil, st.PauseReason = nil, nil
		}
		login, err := s.d.Logins.Login(ctx, d.ID)
		if err != nil {
			return nil, fmt.Errorf("đọc đăng nhập %s: %w", d.ID, err)
		}
		login.Type = d.Login
		out = append(out, domain.PlatformStatus{
			ID:           d.ID,
			Label:        d.Label,
			Uses:         append([]domain.PlatformUse(nil), d.Uses...),
			Kinds:        append([]domain.PlatformKind(nil), d.Kinds...),
			TargetHeight: d.TargetHeight,
			Login:        login,
			State:        conditionOf(st, login),
			PausedUntil:  st.PausedUntil,
			PauseReason:  st.PauseReason,
			FailureCount: st.FailureCount,
			LastError:    lastErrorOf(st),
			Usage:        usage[d.ID],
		})
	}
	return out, nil
}

// conditionOf suy trạng thái hiển thị: đang chờ chặn thì paused; còn lại cần
// đăng nhập lại khi đăng nhập có refreshError hoặc YouTube báo cookies_failed.
func conditionOf(st domain.PlatformState, login domain.PlatformLogin) domain.PlatformCondition {
	if st.PausedUntil != nil {
		return domain.PlatformPaused
	}
	if login.RefreshError != nil {
		return domain.PlatformNeedsLogin
	}
	if st.LastErrorCode != "" {
		return domain.PlatformNeedsLogin
	}
	return domain.PlatformReady
}

func lastErrorOf(st domain.PlatformState) *domain.PlatformLastError {
	if st.LastErrorCode == "" || st.LastErrorAt == nil {
		return nil
	}
	return &domain.PlatformLastError{Code: st.LastErrorCode, At: *st.LastErrorAt, JobID: st.LastErrorJobID}
}

// RecordBlocked ghi một lần bị chặn của nền tảng: tăng đếm và đặt giờ chờ
// tăng dần 15 → 120 phút; retryAfter (Retry-After của server) chỉ được kéo
// dài. Gọi bởi pacing/scan khi nền tảng trả rate limit.
func (s *Service) RecordBlocked(ctx context.Context, id domain.PlatformID, retryAfter time.Duration) error {
	d, ok := Lookup(id)
	if !ok {
		return ErrPlatformUnknown
	}
	st, err := s.d.Store.RecordBlocked(ctx, id, s.now(), pauseReason(d), retryAfter)
	if err != nil {
		return err
	}
	_ = st
	s.notify(id)
	return nil
}

// pauseReason là câu lý do của lần tạm chặn (W13): "<Nền tảng> đang giới hạn
// yêu cầu" — trùng nguyên văn của yt-factory cho Bilibili.
func pauseReason(d Descriptor) string {
	return d.Label + " đang giới hạn yêu cầu"
}

// PauseMinutes tính số phút chờ cho lần chặn thứ n liên tiếp (15·2^(n-1),
// trần 120) — chuyển từ pausePlatform của yt-factory; dùng trong câu SQL và
// test dùng lại công thức này.
func PauseMinutes(failureCount int) float64 {
	exp := failureCount - 1
	if exp > 3 {
		exp = 3
	}
	if exp < 0 {
		exp = 0
	}
	return math.Min(pauseMaxMin, pauseMinMin*math.Pow(2, float64(exp)))
}

// RecordSuccess ghi một lần gọi thành công: xoá đếm chặn và giờ chờ.
func (s *Service) RecordSuccess(ctx context.Context, id domain.PlatformID) error {
	if err := s.d.Store.ClearPause(ctx, id); err != nil {
		return err
	}
	s.notify(id)
	return nil
}

// ClearPause xoá giờ chờ và đếm chặn của một nền tảng — dùng khi đăng nhập
// mới thành công (danh tính mới thì lượt chặn cũ không còn ý nghĩa).
func (s *Service) ClearPause(ctx context.Context, id domain.PlatformID) error {
	return s.RecordSuccess(ctx, id)
}

// Pause trả giờ chờ còn hiệu lực của một nền tảng; dọn trường chờ đã hết hạn.
func (s *Service) Pause(ctx context.Context, id domain.PlatformID) (until *time.Time, reason *string, err error) {
	return s.d.Store.Pause(ctx, id, s.now())
}

// RecordError ghi lỗi nền tảng cần người xử lý (hiện chỉ cookies_failed của
// YouTube — "xác nhận bạn không phải bot").
func (s *Service) RecordError(ctx context.Context, id domain.PlatformID, code string, jobID *string) error {
	if _, ok := Lookup(id); !ok {
		return ErrPlatformUnknown
	}
	if err := s.d.Store.RecordError(ctx, id, code, s.now(), jobID); err != nil {
		return err
	}
	s.notify(id)
	return nil
}

// ClearError xoá lỗi gần nhất (đăng nhập lại thành công hoặc tải kế tiếp ổn).
func (s *Service) ClearError(ctx context.Context, id domain.PlatformID) error {
	if err := s.d.Store.ClearError(ctx, id); err != nil {
		return err
	}
	s.notify(id)
	return nil
}

// Attention liệt kê các mục cần xử lý của mảng nền tảng cho chấm bánh răng:
// đăng nhập Bilibili hết hạn tự gia hạn, hoặc YouTube báo cookies_failed —
// cả hai dẫn về mục Tài khoản nền tảng.
func (s *Service) Attention(ctx context.Context) ([]AttentionItem, error) {
	statuses, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]AttentionItem, 0, 2)
	for _, st := range statuses {
		switch {
		case st.Login.RefreshError != nil:
			items = append(items, AttentionItem{
				Kind: domain.AttentionLoginExpired,
				Text: "Đăng nhập Bilibili đã hết hạn",
				Href: "#/settings?section=accounts",
			})
		case st.LastError != nil && st.LastError.Code == "cookies_failed":
			items = append(items, AttentionItem{
				Kind: domain.AttentionLoginExpired,
				Text: "YouTube yêu cầu đăng nhập để tải video",
				Href: "#/settings?section=accounts",
			})
		}
	}
	return items, nil
}

// AttentionItem là một mục cần xử lý do mảng này đóng góp; ánh sang
// status.Attention ở chỗ ráp (cmd) để gói không import tầng status.
type AttentionItem struct {
	Kind domain.AttentionKind
	Text string
	Href string
}
