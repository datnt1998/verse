package accounts

// Cookie YouTube lấy từ một hồ sơ trình duyệt Chromium (port của
// src/server/tools/cookies.ts): liệt kê trình duyệt/hồ sơ đã cài (chỉ đọc),
// chọn một hồ sơ cho yt-dlp --cookies-from-browser, kiểm tra bằng một lần gọi
// yt-dlp không tải media trước khi lưu.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// cookieSettingKey là khoá settings giữ nguồn cookie — trùng khoá "cookies"
// của bản TS để khi port không phải dịch lại khoá.
const cookieSettingKey = "cookies"

// profilesTTL là nhịp quét lại hồ sơ trình duyệt: GET không được duyệt thư
// mục của 5 trình duyệt mỗi lần gọi (luật hiệu năng), nên cache 60 giây và
// cho client bắt buộc quét lại qua ?refresh=1.
const profilesTTL = 60 * time.Second

// cookieSource là giá trị lưu trong settings: hồ sơ trình duyệt đã chọn.
type cookieSource struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
}

// browserDirs là user-data dir của các trình duyệt Chromium mà yt-dlp đọc
// được, tương đối theo thư mục gốc của từng OS (port BROWSER_DIRS).
var browserDirs = map[string]struct {
	darwin, linux, windows string
}{
	"chrome":   {"Google/Chrome", "google-chrome", "Google/Chrome/User Data"},
	"brave":    {"BraveSoftware/Brave-Browser", "BraveSoftware/Brave-Browser", "BraveSoftware/Brave-Browser/User Data"},
	"edge":     {"Microsoft Edge", "microsoft-edge", "Microsoft/Edge/User Data"},
	"chromium": {"Chromium", "chromium", "Chromium/User Data"},
	"vivaldi":  {"Vivaldi", "vivaldi", "Vivaldi/User Data"},
}

// browserOrder giữ thứ tự duyệt ổn định của bản TS (Object.keys).
var browserOrder = []string{"chrome", "brave", "edge", "chromium", "vivaldi"}

// localState là phần "Local State" của trình duyệt mang danh sách hồ sơ.
type localState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name     string `json:"name"`
			UserName string `json:"user_name"`
		} `json:"info_cache"`
	} `json:"profile"`
}

func (s *Service) goos() string {
	if s.d.GOOS != "" {
		return s.d.GOOS
	}
	return runtime.GOOS
}

func (s *Service) home() (string, error) {
	if s.d.Home != "" {
		return s.d.Home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("không xác định được thư mục home: %w", err)
	}
	return home, nil
}

// userDataDir trả thư mục user-data của một trình duyệt trên OS hiện tại.
func (s *Service) userDataDir(browser string) string {
	dirs, ok := browserDirs[browser]
	if !ok {
		return ""
	}
	home, err := s.home()
	if err != nil {
		return ""
	}
	switch s.goos() {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", dirs.darwin)
	case "linux":
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		return filepath.Join(base, dirs.linux)
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, dirs.windows)
	default:
		return ""
	}
}

// listBrowserProfiles đọc hồ sơ của mọi trình duyệt Chromium đã cài — chỉ đọc
// "Local State" và kiểm thư mục tồn tại, không chạm kho cookie.
func (s *Service) listBrowserProfiles() ([]domain.BrowserProfile, error) {
	var out []domain.BrowserProfile
	for _, browser := range browserOrder {
		dir := s.userDataDir(browser)
		if dir == "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "Local State"))
		if err != nil {
			continue // trình duyệt không cài hoặc Local State hỏng → bỏ qua
		}
		var state localState
		if err := json.Unmarshal(raw, &state); err != nil {
			continue
		}
		var names []string
		for profile := range state.Profile.InfoCache {
			if st, err := os.Stat(filepath.Join(dir, profile)); err == nil && st.IsDir() {
				names = append(names, profile)
			}
		}
		sort.Slice(names, func(i, j int) bool { return natLess(names[i], names[j]) })
		for _, profile := range names {
			info := state.Profile.InfoCache[profile]
			name := info.Name
			if name == "" {
				name = profile
			}
			out = append(out, domain.BrowserProfile{
				Browser: browser,
				Profile: profile,
				Name:    name,
				Email:   info.UserName,
			})
		}
	}
	return out, nil
}

// natLess so sánh theo kiểu numeric:true của localeCompare (Profile 2 đứng
// trước Profile 10).
func natLess(a, b string) bool {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			ai, bj := i, j
			for ai < len(ar) && unicode.IsDigit(ar[ai]) {
				ai++
			}
			for bj < len(br) && unicode.IsDigit(br[bj]) {
				bj++
			}
			an, bn := strings.TrimLeft(string(ar[i:ai]), "0"), strings.TrimLeft(string(br[j:bj]), "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			i, j = ai, bj
			continue
		}
		if ar[i] != br[j] {
			return ar[i] < br[j]
		}
		i++
		j++
	}
	return i >= len(ar) && j < len(br)
}

// Profiles trả hồ sơ trình duyệt, lấy từ cache tối đa 60 giây; refresh=true
// bắt buộc quét lại.
func (s *Service) Profiles(ctx context.Context, refresh bool) ([]domain.BrowserProfile, error) {
	s.profilesMu.Lock()
	defer s.profilesMu.Unlock()
	if !refresh && s.profilesOK && s.now().Sub(s.profilesAt) < profilesTTL {
		return append([]domain.BrowserProfile(nil), s.profiles...), nil
	}
	list, err := s.listBrowserProfiles()
	if err != nil {
		return nil, err
	}
	// Cache cả danh sách rỗng — máy không có Chromium nào thì mỗi lượt hỏi
	// không phải duyệt lại 5 user-data dir.
	s.profiles = list
	s.profilesOK = true
	s.profilesAt = s.now()
	return append([]domain.BrowserProfile(nil), list...), nil
}

// cookieSourceOf đọc nguồn cookie đã lưu; nil khi chưa chọn.
func (s *Service) cookieSourceOf(ctx context.Context) (*cookieSource, error) {
	if s.d.Settings == nil {
		return nil, nil
	}
	var src cookieSource
	found, err := s.d.Settings.Get(ctx, cookieSettingKey, &src)
	if err != nil {
		return nil, err
	}
	if !found || src.Browser == "" || src.Profile == "" {
		return nil, nil
	}
	return &src, nil
}

// youtubeLogin dựng hình đăng nhập YouTube: browserProfile "chrome:Default".
func (s *Service) youtubeLogin(ctx context.Context) (domain.PlatformLogin, error) {
	src, err := s.cookieSourceOf(ctx)
	if err != nil {
		return domain.PlatformLogin{}, err
	}
	login := domain.PlatformLogin{}
	if src == nil {
		return login, nil
	}
	login.Saved = true
	login.Browser = src.Browser
	login.Profile = src.Profile
	return login, nil
}

// classifyProbeError xếp lỗi của YouTubeProbe: mã cookies_failed /
// login_required là đăng nhập không dùng được (rejected), còn lại là nền
// tảng/dịch vụ tạm lỗi (unavailable). nil → chấp nhận.
func classifyProbeError(err error) error {
	if err == nil {
		return nil
	}
	var coded CodedError
	if errors.As(err, &coded) {
		switch coded.ErrorCode() {
		case "cookies_failed":
			return rejectedf("Không đọc được cookie của hồ sơ trình duyệt này. Hồ sơ cần đang đăng nhập tài khoản Google phụ; mở trình duyệt đó một lần rồi thử lại.")
		case "login_required":
			return rejectedf("Hồ sơ trình duyệt này chưa đăng nhập tài khoản Google phụ.")
		}
	}
	return unavailablef("Chưa kiểm tra được hồ sơ trình duyệt (yt-dlp không chạy được). Cài công cụ ở Cài đặt › Công cụ rồi thử lại.")
}

// SetCookieSource bật cookie YouTube từ một hồ sơ trình duyệt (port
// setCookieSource + kiểm tra yt-dlp của V23): hồ sơ phải tồn tại và yt-dlp
// đọc được cookie của nó trước khi lưu.
func (s *Service) SetCookieSource(ctx context.Context, browser, profile string) (domain.PlatformLogin, error) {
	if !s.mu.TryLock() {
		return domain.PlatformLogin{}, errBusy
	}
	defer s.mu.Unlock()
	if _, ok := browserDirs[browser]; !ok || profile == "" {
		return domain.PlatformLogin{}, invalidf("Hồ sơ trình duyệt không hợp lệ.")
	}
	profiles, err := s.listBrowserProfiles()
	if err != nil {
		return domain.PlatformLogin{}, err
	}
	found := false
	for _, p := range profiles {
		if p.Browser == browser && p.Profile == profile {
			found = true
			break
		}
	}
	if !found {
		return domain.PlatformLogin{}, invalidf("Không thấy hồ sơ trình duyệt %s:%s trên máy này.", browser, profile)
	}
	if s.d.Probe == nil {
		return domain.PlatformLogin{}, unavailablef("Chưa kiểm tra được hồ sơ trình duyệt (yt-dlp không chạy được). Cài công cụ ở Cài đặt › Công cụ rồi thử lại.")
	}
	if err := classifyProbeError(s.d.Probe.Check(ctx, browser, profile)); err != nil {
		return domain.PlatformLogin{}, err
	}
	if err := s.d.Settings.Set(ctx, cookieSettingKey, cookieSource{Browser: browser, Profile: profile}); err != nil {
		return domain.PlatformLogin{}, err
	}
	// Profile vừa đọc xong → cache còn tươi, không cần vô hiệu.
	s.notify(domain.PlatformYouTube)
	return s.youtubeLogin(ctx)
}

// checkYouTube kiểm tra lại hồ sơ đã lưu (nút "Thử lại" ở banner): thành công
// xoá lastError; cookies_failed ghi lỗi để banner/chấm hiện tiếp.
func (s *Service) checkYouTube(ctx context.Context) (domain.PlatformLogin, error) {
	src, err := s.cookieSourceOf(ctx)
	if err != nil {
		return domain.PlatformLogin{}, err
	}
	if src == nil {
		return domain.PlatformLogin{}, invalidf("Chưa chọn hồ sơ trình duyệt nào.")
	}
	if s.d.Probe == nil {
		return domain.PlatformLogin{}, unavailablef("Chưa kiểm tra được hồ sơ trình duyệt (yt-dlp không chạy được). Cài công cụ ở Cài đặt › Công cụ rồi thử lại.")
	}
	probeErr := s.d.Probe.Check(ctx, src.Browser, src.Profile)
	if probeErr == nil {
		s.notify(domain.PlatformYouTube)
		return s.youtubeLogin(ctx)
	}
	return domain.PlatformLogin{}, classifyProbeError(probeErr)
}

// CookieArgs trả tham số yt-dlp cho nguồn cookie đã chọn (port cookieArgs) —
// [] khi đang tải ẩn danh. Chỉ gói quét/tải dùng.
func (s *Service) CookieArgs(ctx context.Context) ([]string, error) {
	src, err := s.cookieSourceOf(ctx)
	if err != nil || src == nil {
		return nil, err
	}
	return []string{"--cookies-from-browser", src.Browser + ":" + src.Profile}, nil
}
