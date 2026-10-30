package platforms

// Mặt HTTP của đăng nhập theo registry (§24.1): platforms kiểm nền tảng có áp
// dụng thao tác không rồi mới gọi accounts — web không phải biết nền tảng nào
// dùng cookie tay, hồ sơ trình duyệt hay mã QR.

import (
	"context"
	"errors"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// LoginInput là body của PUT /api/platforms/:id/login: cookie dán tay (chuỗi
// cookie nền tảng) hoặc hồ sơ trình duyệt (browser + profile).
type LoginInput struct {
	Cookie  string
	Browser string
	Profile string
}

// SetLogin lưu đăng nhập của một nền tảng theo cách đăng nhập trong registry.
func (s *Service) SetLogin(ctx context.Context, id domain.PlatformID, in LoginInput) (domain.PlatformLogin, error) {
	d, ok := Lookup(id)
	if !ok {
		return domain.PlatformLogin{}, ErrPlatformUnknown
	}
	var (
		login domain.PlatformLogin
		err   error
	)
	switch d.Login {
	case domain.LoginBrowserCookie:
		if in.Browser == "" || in.Profile == "" || in.Cookie != "" {
			return domain.PlatformLogin{}, ErrInvalidLoginBody
		}
		login, err = s.d.Logins.SetCookieSource(ctx, in.Browser, in.Profile)
	case domain.LoginCookieString, domain.LoginQR:
		if in.Cookie == "" || in.Browser != "" || in.Profile != "" {
			return domain.PlatformLogin{}, ErrInvalidLoginBody
		}
		login, err = s.d.Logins.SaveManualCookie(ctx, in.Cookie)
	default:
		return domain.PlatformLogin{}, ErrLoginUnsupported
	}
	if err != nil {
		s.noteLoginFailure(ctx, id, err)
		return domain.PlatformLogin{}, err
	}
	s.afterLogin(ctx, id)
	return login, nil
}

// ClearLogin xoá đăng nhập của một nền tảng.
func (s *Service) ClearLogin(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	d, ok := Lookup(id)
	if !ok {
		return domain.PlatformLogin{}, ErrPlatformUnknown
	}
	if d.Login == domain.LoginNone {
		return domain.PlatformLogin{}, ErrLoginUnsupported
	}
	return s.d.Logins.ClearLogin(ctx, id)
}

// Profiles liệt kê hồ sơ trình duyệt Chromium (chỉ YouTube dùng).
func (s *Service) Profiles(ctx context.Context, id domain.PlatformID, refresh bool) ([]domain.BrowserProfile, error) {
	d, ok := Lookup(id)
	if !ok {
		return nil, ErrPlatformUnknown
	}
	if d.Login != domain.LoginBrowserCookie {
		return nil, ErrLoginUnsupported
	}
	return s.d.Logins.Profiles(ctx, refresh)
}

// StartQR tạo mã QR đăng nhập (chỉ nền tảng dùng QR).
func (s *Service) StartQR(ctx context.Context, id domain.PlatformID) (domain.QRSession, error) {
	d, ok := Lookup(id)
	if !ok {
		return domain.QRSession{}, ErrPlatformUnknown
	}
	if d.Login != domain.LoginQR {
		return domain.QRSession{}, ErrLoginUnsupported
	}
	return s.d.Logins.StartQR(ctx)
}

// PollQR hỏi trạng thái một phiên mã QR.
func (s *Service) PollQR(ctx context.Context, id domain.PlatformID, key string) (domain.QRPoll, error) {
	d, ok := Lookup(id)
	if !ok {
		return domain.QRPoll{}, ErrPlatformUnknown
	}
	if d.Login != domain.LoginQR {
		return domain.QRPoll{}, ErrLoginUnsupported
	}
	poll, err := s.d.Logins.PollQR(ctx, key)
	if err != nil {
		return domain.QRPoll{}, err
	}
	if poll.Status == domain.QRSuccess {
		_ = s.ClearPause(ctx, id)
	}
	return poll, nil
}

// Refresh gia hạn ngay đăng nhập QR của một nền tảng.
func (s *Service) Refresh(ctx context.Context, id domain.PlatformID) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	d, ok := Lookup(id)
	if !ok {
		return "", domain.PlatformLogin{}, ErrPlatformUnknown
	}
	if d.Login != domain.LoginQR {
		return "", domain.PlatformLogin{}, ErrLoginUnsupported
	}
	outcome, login, err := s.d.Logins.Refresh(ctx)
	if err != nil {
		return "", domain.PlatformLogin{}, err
	}
	if outcome == domain.RefreshRefreshed {
		_ = s.ClearPause(ctx, id)
	}
	return outcome, login, nil
}

// Check kiểm tra lại đăng nhập đang lưu của một nền tảng.
func (s *Service) Check(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	d, ok := Lookup(id)
	if !ok {
		return domain.PlatformLogin{}, ErrPlatformUnknown
	}
	if d.Login == domain.LoginNone {
		return domain.PlatformLogin{}, ErrLoginUnsupported
	}
	login, err := s.d.Logins.Check(ctx, id)
	if err != nil {
		s.noteLoginFailure(ctx, id, err)
		return domain.PlatformLogin{}, err
	}
	s.afterLogin(ctx, id)
	return login, nil
}

// noteLoginFailure ghi lỗi YouTube "xác nhận bạn không phải bot" khi một lần
// đăng nhập/kiểm tra bị nền tảng từ chối — banner và chấm bánh răng dựa vào
// lastError này.
func (s *Service) noteLoginFailure(ctx context.Context, id domain.PlatformID, err error) {
	if id == domain.PlatformYouTube && rejectedByPlatform(err) {
		_ = s.RecordError(ctx, id, "cookies_failed", nil)
	}
}

// loginReason là interface mà accounts.LoginError hiện thực, để platforms
// nhận ra "nền tảng từ chối" mà không phải import gói accounts.
type loginReason interface {
	LoginReason() string
}

// afterLogin áp tác dụng phụ của một thao tác đăng nhập thành công: đăng nhập
// Bilibili mới là danh tính mới nên bỏ giờ chờ bị chặn; YouTube hết lỗi thì
// xoá lastError. Lỗi phụ không làm hỏng kết quả đăng nhập đã lưu.
func (s *Service) afterLogin(ctx context.Context, id domain.PlatformID) {
	switch id {
	case domain.PlatformBilibili:
		_ = s.ClearPause(ctx, id)
	case domain.PlatformYouTube:
		_ = s.ClearError(ctx, id)
	}
}

// rejectedByPlatform đúng khi lỗi là "nền tảng không nhận đăng nhập này".
func rejectedByPlatform(err error) bool {
	var le loginReason
	return errors.As(err, &le) && le.LoginReason() == "rejected"
}
