// Package accounts giữ đăng nhập nền tảng chỉ-nằm-trên-máy (V23 phần tài
// khoản, V24): cookie YouTube đọc từ một hồ sơ trình duyệt, cookie Bilibili
// dán tay đã kiểm tra, và đăng nhập Bilibili bằng mã QR kèm tự gia hạn.
//
// Chuyển từ yt-factory: src/server/tools/cookies.ts, tools/bilibili-cookies.ts,
// backdrop/bilibili-login.ts, backdrop/bilibili-qr-login.ts (+storage).
//
// Bí mật chỉ nằm trong ~/.viderse (tệp quyền 0600); API chỉ trả giá trị đã
// che. Mọi thao tác đăng nhập/gia hạn/kiểm tra chạy dưới một khoá chung.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// Lý do ổn định trả trong body lỗi {reason} của các endpoint đăng nhập.
const (
	ReasonRejected    = "rejected"    // nền tảng không nhận đăng nhập này
	ReasonUnavailable = "unavailable" // không kết nối / nền tảng chưa trả lời
	ReasonBusy        = "busy"        // một thao tác đăng nhập khác đang chạy
	ReasonInvalid     = "invalid"     // đầu vào không hợp lệ
	ReasonUnsupported = "unsupported" // thao tác không áp dụng cho nền tảng này
)

// LoginError là lỗi mặt đăng nhập mang reason ổn định + câu hiển thị.
type LoginError struct {
	Reason string
	Msg    string
}

func (e *LoginError) Error() string { return e.Msg }

func rejectedf(format string, args ...any) *LoginError {
	return &LoginError{Reason: ReasonRejected, Msg: fmt.Sprintf(format, args...)}
}
func unavailablef(format string, args ...any) *LoginError {
	return &LoginError{Reason: ReasonUnavailable, Msg: fmt.Sprintf(format, args...)}
}
func invalidf(format string, args ...any) *LoginError {
	return &LoginError{Reason: ReasonInvalid, Msg: fmt.Sprintf(format, args...)}
}

// errBusy trả khi khoá đăng nhập đang bị giữ bởi một thao tác khác — yêu cầu
// trả 409 thay vì xếp hàng phía sau một lời gọi mạng chậm.
var errBusy = &LoginError{Reason: ReasonBusy, Msg: "Một thao tác đăng nhập khác đang chạy. Thử lại sau ít phút."}

// Settings là phần kho cài đặt accounts cần (nguồn cookie YouTube). Hiện
// thực: postgres.SettingsStore.
type Settings interface {
	Get(ctx context.Context, key string, out any) (bool, error)
	Set(ctx context.Context, key string, value any) error
	Clear(ctx context.Context, key string) error
}

// YouTubeProbe xác nhận một hồ sơ trình duyệt đọc được cookie YouTube: chạy
// yt-dlp ở chế độ không tải media. Hiện thực ở cmd/viderse trên tools.YtDlp;
// lỗi trả về được phân loại qua CodedError (mã tools.YtDlpErrorCode).
type YouTubeProbe interface {
	Check(ctx context.Context, browser, profile string) error
}

// CodedError là lỗi mang mã chuỗi ổn định (khớp tools.YtDlpError.ErrorCode).
type CodedError interface {
	error
	ErrorCode() string
}

// BilibiliHosts là bộ host Bilibili để test thay bằng httptest; trường rỗng
// dùng host thật.
type BilibiliHosts struct {
	API      string // https://api.bilibili.com
	Passport string // https://passport.bilibili.com/x/passport-login/web
	Site     string // https://www.bilibili.com (Referer + correspond)
}

func (h BilibiliHosts) api() string {
	if h.API == "" {
		return "https://api.bilibili.com"
	}
	return h.API
}
func (h BilibiliHosts) passport() string {
	if h.Passport == "" {
		return "https://passport.bilibili.com/x/passport-login/web"
	}
	return h.Passport
}
func (h BilibiliHosts) site() string {
	if h.Site == "" {
		return "https://www.bilibili.com"
	}
	return h.Site
}

// Deps là phụ thuộc của Service — mọi mặt ngoài (file, HTTP, settings,
// platforms, yt-dlp, đồng hồ) đều tiêm qua interface/hàm.
type Deps struct {
	Dir      string        // ~/.viderse — nơi giữ tệp đăng nhập (0600)
	Client   *http.Client  // nil → http.DefaultClient
	Hosts    BilibiliHosts // chỉ test mới đặt
	Settings Settings      // nguồn cookie YouTube
	Probe    YouTubeProbe  // nil → mọi Check YouTube trả unavailable
	Home     string        // home để quét hồ sơ trình duyệt; "" → os.UserHomeDir
	GOOS     string        // "" → runtime.GOOS (test đặt "darwin"…)
	Now      func() time.Time
	Notify   func(ref string) // phát sự kiện "platforms"; nil → không phát
}

// Service gom các thao tác tài khoản nền tảng; an toàn cho goroutine — một
// khoá (mu + khoá tệp Bilibili) tuần tự hoá mọi đăng nhập/gia hạn/kiểm tra.
type Service struct {
	d      Deps
	now    func() time.Time
	hosts  BilibiliHosts
	client *http.Client
	mu     sync.Mutex // khoá thao tác đăng nhập — chỉ giữ qua TryLock (busy)

	// Bộ nhớ đệm đọc: hồ sơ trình duyệt quét lại tối đa mỗi 60 giây, snapshot
	// Bilibili đọc lại chỉ khi tệp đổi (GET không được quét thư mục mỗi lần).
	profilesMu sync.Mutex
	profilesAt time.Time
	profiles   []domain.BrowserProfile
	profilesOK bool

	// cacheMu bảo vệ snapshot đăng nhập Bilibili đã công bố: mọi đọc/ghi đi
	// qua RWMutex này và bản công bố là immutable (clone trước khi sửa).
	// GET đọc dưới nó mà không giữ mu — không bị chặn bởi chuỗi gia hạn mạng.
	cacheMu      sync.RWMutex
	loginFileMod time.Time
	loginSnap    *bilibiliLoginState
}

// New dựng service trên các phụ thuộc tiêm vào.
func New(d Deps) *Service {
	if d.Dir == "" {
		panic("accounts.New: thiếu Dir")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	// Bản TS chạy với redirect:'error' và timeout 20s trên mọi request Bilibili.
	scoped := *client
	scoped.Timeout = requestTimeout
	scoped.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Service{d: d, now: d.Now, hosts: d.Hosts, client: &scoped}
}

func (s *Service) notify(ref string) {
	if s.d.Notify != nil {
		s.d.Notify(ref)
	}
}

// ── đường dẫn tệp (chuyển từ bilibili-cookies.ts / qr-login-storage.ts) ────

func (s *Service) cookieFile() string { return filepath.Join(s.d.Dir, "bilibili-cookie.txt") }
func (s *Service) loginFile() string  { return filepath.Join(s.d.Dir, "bilibili-login.json") }
func (s *Service) qrFile() string     { return filepath.Join(s.d.Dir, "bilibili-qr-session.json") }
func (s *Service) lockFile() string   { return s.cookieFile() + ".lock" }

// Login trả hình đăng nhập của một nền tảng (Type do platforms điền). YouTube
// đọc nguồn cookie từ settings; Bilibili đọc snapshot tệp; nền tảng khác
// trả đăng nhập rỗng.
func (s *Service) Login(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	switch id {
	case domain.PlatformYouTube:
		return s.youtubeLogin(ctx)
	case domain.PlatformBilibili:
		return s.bilibiliLogin(ctx)
	default:
		return domain.PlatformLogin{}, nil
	}
}

// Check kiểm tra lại đăng nhập đang lưu của một nền tảng (nút "Thử lại"):
// YouTube chạy yt-dlp trên hồ sơ đã chọn, Bilibili gọi nav với cookie đang
// có. Thành công xoá lastError; thất bại ghi lỗi và trả reason.
func (s *Service) Check(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	switch id {
	case domain.PlatformYouTube:
		if !s.mu.TryLock() {
			return domain.PlatformLogin{}, errBusy
		}
		defer s.mu.Unlock()
		return s.checkYouTube(ctx)
	case domain.PlatformBilibili:
		// checkBilibili tự lấy khoá đăng nhập chung (mutex + khoá tệp).
		return s.checkBilibili(ctx)
	default:
		return domain.PlatformLogin{}, &LoginError{Reason: ReasonUnsupported, Msg: "Nền tảng này không có đăng nhập."}
	}
}

// ClearLogin xoá đăng nhập của một nền tảng.
func (s *Service) ClearLogin(ctx context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	switch id {
	case domain.PlatformYouTube:
		if !s.mu.TryLock() {
			return domain.PlatformLogin{}, errBusy
		}
		defer s.mu.Unlock()
		if s.d.Settings != nil {
			if err := s.d.Settings.Clear(ctx, cookieSettingKey); err != nil {
				return domain.PlatformLogin{}, err
			}
		}
		s.notify(id)
		return s.youtubeLogin(ctx)
	case domain.PlatformBilibili:
		if err := s.withLock(func() error { return s.clearBilibiliLocked() }); err != nil {
			return domain.PlatformLogin{}, err
		}
		s.notify(id)
		return s.bilibiliLogin(ctx)
	default:
		return domain.PlatformLogin{}, &LoginError{Reason: ReasonUnsupported, Msg: "Nền tảng này không có đăng nhập."}
	}
}

// errors.As cho *LoginError — web đọc Reason và Msg để map status/body.
func asLoginError(err error) (*LoginError, bool) {
	var le *LoginError
	if errors.As(err, &le) {
		return le, true
	}
	return nil, false
}

// LoginReason trả reason ổn định của lỗi đăng nhập; web dùng interface này để
// map sang status/`reason` mà không phải import gói accounts.
func (e *LoginError) LoginReason() string { return e.Reason }
