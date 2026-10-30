package accounts

// Chuyển từ src/server/tools/bilibili-cookies.ts và
// src/server/backdrop/bilibili-qr-login-storage.ts của yt-factory: chuẩn hoá
// cookie dán tay, kiểm tra đăng nhập qua nav, tệp bí mật 0600, snapshot đăng
// nhập nguyên tử, và khoá tệp chéo process cho mọi thao tác đăng nhập.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
)

// bilibiliUserAgent là UA của app 哔哩哔哩 web mà yt-factory đã chạy ổn.
const bilibiliUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

const requestTimeout = 20 * time.Second

// secretPattern là mẫu giá trị bí mật được ghi tệp: ASCII thấy được, không
// khoảng trắng, không điều khiển (chuyển từ `secret` của qr-login-storage).
var secretPattern = regexp.MustCompile(`^[\x21-\x7e]+$`)

// bilibiliLoginState là snapshot đăng nhập Bilibili — file chuẩn; bản gương
// cookie.txt chỉ để tương thích đọc. JSON giữ hình của bản TS (mili-giây).
type bilibiliLoginState struct {
	Cookie        string  `json:"cookie"`
	Method        string  `json:"method"` // "qr" | "manual"
	Account       *string `json:"account,omitempty"`
	RefreshToken  *string `json:"refreshToken,omitempty"`
	CSRF          *string `json:"csrf,omitempty"`
	LastRefreshAt *int64  `json:"lastRefreshAt,omitempty"`
	LastCheckedAt *int64  `json:"lastCheckedAt,omitempty"`
	RefreshError  *string `json:"refreshError,omitempty"`
}

func (st *bilibiliLoginState) valid() bool {
	if st == nil || st.Cookie == "" || len(st.Cookie) > 16384 {
		return false
	}
	if st.Method != "qr" && st.Method != "manual" {
		return false
	}
	for _, s := range []*string{st.RefreshToken, st.CSRF} {
		if s != nil && !secretPattern.MatchString(*s) {
			return false
		}
	}
	return true
}

// clone trả bản sao sâu của snapshot — bản công bố trong cache phải
// immutable để đọc song song không đuổi nhau, và sửa bản sao không chạm bản
// đã công bố.
func (st *bilibiliLoginState) clone() *bilibiliLoginState {
	if st == nil {
		return nil
	}
	out := *st
	cp := func(p *string) *string {
		if p == nil {
			return nil
		}
		return new(*p)
	}
	out.Account = cp(st.Account)
	out.RefreshToken = cp(st.RefreshToken)
	out.CSRF = cp(st.CSRF)
	out.RefreshError = cp(st.RefreshError)
	if st.LastRefreshAt != nil {
		out.LastRefreshAt = new(*st.LastRefreshAt)
	}
	if st.LastCheckedAt != nil {
		out.LastCheckedAt = new(*st.LastCheckedAt)
	}
	return &out
}

// readLoginState đọc snapshot; nil khi chưa có tệp. Kết quả được cache trong
// service theo mtime; caller nhận một BẢN SAO — bản công bố không bao giờ bị
// sửa tại chỗ. GET nhiều lượt đọc song song dưới cacheMu mà không giữ mu.
func (s *Service) readLoginState() (*bilibiliLoginState, error) {
	file := s.loginFile()
	info, err := os.Stat(file)
	if errors.Is(err, os.ErrNotExist) {
		s.cacheMu.Lock()
		s.loginSnap = nil
		s.loginFileMod = time.Time{}
		s.cacheMu.Unlock()
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chưa đọc được đăng nhập Bilibili. Xoá đăng nhập rồi quét mã QR lại: %w", err)
	}
	s.cacheMu.RLock()
	if s.loginSnap != nil && info.ModTime().Equal(s.loginFileMod) {
		cached := s.loginSnap
		s.cacheMu.RUnlock()
		return cached.clone(), nil
	}
	s.cacheMu.RUnlock()
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("chưa đọc được đăng nhập Bilibili. Xoá đăng nhập rồi quét mã QR lại: %w", err)
	}
	var st bilibiliLoginState
	if err := json.Unmarshal(raw, &st); err != nil || !st.valid() {
		return nil, errors.New("chưa đọc được đăng nhập Bilibili. Xoá đăng nhập rồi quét mã QR lại")
	}
	s.cacheMu.Lock()
	s.loginSnap = &st
	s.loginFileMod = info.ModTime()
	s.cacheMu.Unlock()
	return st.clone(), nil
}

// writeLoginState ghi snapshot nguyên tử (tạm + đổi tên, quyền 0600) — một
// lần ghi cho cookie + refresh token, hỏng thì không để nửa trạng thái.
func (s *Service) writeLoginState(st *bilibiliLoginState) error {
	if !st.valid() {
		return errors.New("trạng thái đăng nhập Bilibili không hợp lệ")
	}
	if err := writePrivateAtomic(s.loginFile(), st); err != nil {
		return err
	}
	s.cacheMu.Lock()
	s.loginSnap = st.clone()
	info, err := os.Stat(s.loginFile())
	if err == nil {
		s.loginFileMod = info.ModTime()
	}
	s.cacheMu.Unlock()
	return nil
}

// writePrivateAtomic ghi một giá trị JSON/text ra tệp quyền 0600 qua tệp tạm
// rồi đổi tên — tương đương writeFile(temp, {mode:0o600, flag:'wx'}) + rename
// của bản TS.
func writePrivateAtomic(file string, value any) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	var raw []byte
	var err error
	switch v := value.(type) {
	case string:
		raw = []byte(v)
	default:
		raw, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, file)
}

// ── chuẩn hoá cookie dán tay (port NormalizeBilibiliCookie) ────────────────

var (
	cookieEntryPattern = regexp.MustCompile(`^\s*([\w-]+)=([^;\s]*)\s*$`)
	cookieInputPattern = regexp.MustCompile(`^[\x20-\x7e]+$`)
	sessdataPattern    = regexp.MustCompile(`(?:^|;)\s*SESSDATA=`)
)

// NormalizeBilibiliCookie nhận SESSDATA thô hoặc cả chuỗi cookie; trả chuỗi
// chuẩn "a=b; c=d". Mọi lỗi nhập là ReasonInvalid — không bao giờ echo nguyên
// liệu chứa ký tự lạ vào thông báo.
func NormalizeBilibiliCookie(input string) (string, error) {
	text := strings.TrimSpace(input)
	if text == "" || len(text) > 16384 || !cookieInputPattern.MatchString(text) {
		return "", invalidf("Cookie Bilibili không hợp lệ. Dán SESSDATA hoặc chuỗi cookie, không kèm xuống dòng.")
	}
	text = regexp.MustCompile(`(?i)^Cookie:\s*`).ReplaceAllString(text, "")
	full := sessdataPattern.MatchString(text) || strings.Contains(text, ";")
	if !full {
		text = "SESSDATA=" + text
	}
	type kv struct{ k, v string }
	var entries []kv
	seen := map[string]bool{}
	for _, part := range strings.Split(text, ";") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		m := cookieEntryPattern.FindStringSubmatch(part)
		if m == nil || m[2] == "" {
			return "", invalidf("Cookie Bilibili không hợp lệ. Hãy sao chép lại từ tài khoản phụ.")
		}
		if seen[m[1]] {
			return "", invalidf("Cookie Bilibili bị trùng tên. Hãy sao chép lại.")
		}
		seen[m[1]] = true
		entries = append(entries, kv{m[1], m[2]})
	}
	if !seen["SESSDATA"] {
		return "", invalidf("Chuỗi cookie cần có SESSDATA của tài khoản Bilibili phụ.")
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e.k)
		b.WriteString("=")
		b.WriteString(e.v)
	}
	return b.String(), nil
}

// maskedSessdata trả giá trị đã che duy nhất API được phép trả: chỉ 4 ký tự
// cuối của SESSDATA (và chỉ khi nó đủ dài).
func maskedSessdata(cookie string) *string {
	m := regexp.MustCompile(`(?:^|; )SESSDATA=([^;]+)`).FindStringSubmatch(cookie)
	if m == nil {
		return nil
	}
	masked := "SESSDATA=••••"
	if len(m[1]) > 8 {
		masked += m[1][len(m[1])-4:]
	}
	return &masked
}

// cookieHeader đọc chuỗi cookie đang có hiệu lực: snapshot login.json thắng,
// rồi tới gương cookie.txt; "" khi chưa đăng nhập.
func (s *Service) cookieHeader() (string, error) {
	st, err := s.readLoginState()
	if err != nil {
		return "", err
	}
	if st != nil {
		return NormalizeBilibiliCookie(st.Cookie)
	}
	raw, err := os.ReadFile(s.cookieFile())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("chưa đọc được cookie Bilibili. Lưu lại hoặc xoá cookie trong Cài đặt")
	}
	return NormalizeBilibiliCookie(string(raw))
}

// bilibiliLogin dựng hình đăng nhập Bilibili cho PlatformStatus.login (§24.1).
func (s *Service) bilibiliLogin(ctx context.Context) (domain.PlatformLogin, error) {
	cookie, err := s.cookieHeader()
	if err != nil {
		return domain.PlatformLogin{}, err
	}
	st, err := s.readLoginState()
	if err != nil {
		return domain.PlatformLogin{}, err
	}
	login := domain.PlatformLogin{
		Saved:  cookie != "",
		Masked: maskedSessdata(cookie),
		Method: "manual",
	}
	if !login.Saved {
		login.Method = ""
		login.Masked = nil
		return login, nil
	}
	if st != nil {
		login.Method = st.Method
		login.Account = st.Account
		login.RefreshError = st.RefreshError
		if st.LastRefreshAt != nil {
			t := time.UnixMilli(*st.LastRefreshAt)
			login.LastRefreshAt = &t
		}
	}
	login.Renewable = login.Method == "qr" && st != nil && st.RefreshToken != nil && st.CSRF != nil
	return login, nil
}

// ── kiểm tra đăng nhập với Bilibili (port verifyBilibiliCookie) ────────────

type navResponse struct {
	Code int `json:"code"`
	Data *struct {
		IsLogin bool   `json:"isLogin"`
		Uname   string `json:"uname"`
	} `json:"data"`
}

// verifyBilibiliCookie hỏi /x/web-interface/nav; trả tên tài khoản khi
// Bilibili nhận đăng nhập, ReasonRejected khi không, ReasonUnavailable khi
// không gọi được/không đọc được trả lời.
func (s *Service) verifyBilibiliCookie(ctx context.Context, cookie string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.hosts.api()+"/x/web-interface/nav", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", bilibiliUserAgent)
	req.Header.Set("Referer", s.hosts.site()+"/")
	req.Header.Set("Cookie", cookie)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", unavailablef("Không kết nối được Bilibili để kiểm tra cookie. Kiểm tra mạng rồi thử lại.")
	}
	defer resp.Body.Close()
	var parsed navResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", unavailablef("Bilibili chưa trả lời khi kiểm tra cookie. Thử lại sau ít phút.")
	}
	if parsed.Code != 0 || parsed.Data == nil || !parsed.Data.IsLogin {
		return "", rejectedf("Cookie này chưa đăng nhập được Bilibili (bilibili.com): có thể đã đăng xuất, hết hạn hoặc copy thiếu. Tài khoản bilibili.tv không dùng được. Lấy lại cookie hoặc quét mã QR.")
	}
	return parsed.Data.Uname, nil
}

// httpClient trả client đã cấu hình (timeout 20s + không tự redirect, dựng
// trong New) cho mọi request Bilibili.
func (s *Service) httpClient() *http.Client { return s.client }

// ── lưu / xoá cookie dán tay và gương tệp (port setBilibiliCookie) ─────────

// setCookieMirror ghi gương cookie.txt (0600) hoặc xoá khi value rỗng; mirror
// còn để tương thích đọc — file chuẩn vẫn là login.json.
func (s *Service) setCookieMirror(cookie string) error {
	if cookie == "" {
		err := os.Remove(s.cookieFile())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writePrivateAtomic(s.cookieFile(), cookie)
}

// commitBilibiliLogin ghi cookie + trạng thái gia hạn cùng một lần (port
// commitBilibiliLogin): gương trước, snapshot sau; hỏng giữa chừng thì trả
// gương về cookie cũ — đăng nhập mới chưa được coi là có hiệu lực.
func (s *Service) commitBilibiliLogin(st *bilibiliLoginState) (domain.PlatformLogin, error) {
	previous, err := s.cookieHeader()
	if err != nil {
		previous = ""
	}
	cookie, err := NormalizeBilibiliCookie(st.Cookie)
	if err != nil {
		return domain.PlatformLogin{}, err
	}
	if err := s.setCookieMirror(cookie); err != nil {
		return domain.PlatformLogin{}, errors.New("chưa lưu được đăng nhập Bilibili. Kiểm tra dung lượng ổ đĩa rồi thử lại")
	}
	err = s.writeLoginState(st)
	if err != nil {
		// Commit hỏng → trả gương về cookie cũ để đăng nhập mới không bị
		// coi là có hiệu lực, kể cả lần đăng nhập đầu tiên.
		_ = s.setCookieMirror(previous)
		return domain.PlatformLogin{}, errors.New("chưa lưu được đăng nhập Bilibili. Kiểm tra dung lượng ổ đĩa rồi thử lại")
	}
	return s.bilibiliLogin(context.Background())
}

// SaveManualCookie kiểm tra với Bilibili rồi lưu cookie dán tay; xoá phiên QR
// đang treo. Cookie chỉ được ghi sau khi nền tảng xác nhận nó là một đăng
// nhập sống — cookie chết không bao giờ lặng lẽ quét ẩn danh.
func (s *Service) SaveManualCookie(ctx context.Context, cookie string) (domain.PlatformLogin, error) {
	var out domain.PlatformLogin
	err := s.withLock(func() error {
		_ = os.Remove(s.qrFile())
		normalized, err := NormalizeBilibiliCookie(cookie)
		if err != nil {
			return err
		}
		account, err := s.verifyBilibiliCookie(ctx, normalized)
		if err != nil {
			return err
		}
		st := &bilibiliLoginState{Cookie: normalized, Method: "manual"}
		if account != "" {
			st.Account = &account
		}
		out, err = s.commitBilibiliLogin(st)
		return err
	})
	return out, err
}

// clearBilibiliLocked xoá đăng nhập Bilibili (gương + snapshot + phiên QR).
func (s *Service) clearBilibiliLocked() error {
	var first error
	for _, f := range []string{s.cookieFile(), s.loginFile(), s.qrFile()} {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) && first == nil {
			first = err
		}
	}
	s.cacheMu.Lock()
	s.loginSnap = nil
	s.loginFileMod = time.Time{}
	s.cacheMu.Unlock()
	return first
}

// checkBilibili kiểm tra lại cookie đang lưu bằng một lần gọi nav; thành công
// cập nhật tên tài khoản trong snapshot.
func (s *Service) checkBilibili(ctx context.Context) (domain.PlatformLogin, error) {
	var out domain.PlatformLogin
	err := s.withLock(func() error {
		cookie, err := s.cookieHeader()
		if err != nil {
			return err
		}
		if cookie == "" {
			return &LoginError{Reason: ReasonInvalid, Msg: "Chưa có đăng nhập Bilibili để kiểm tra."}
		}
		account, err := s.verifyBilibiliCookie(ctx, cookie)
		if err != nil {
			return err
		}
		st, err := s.readLoginState()
		if err != nil {
			return err
		}
		if st != nil && account != "" {
			st.Account = &account
			st.RefreshError = nil
			if err := s.writeLoginState(st); err != nil {
				return err
			}
		}
		out, err = s.bilibiliLogin(ctx)
		return err
	})
	return out, err
}

// CookieHeader trả chuỗi cookie Bilibili đang lưu cho tầng quét — chỉ dùng
// nội bộ, không bao giờ serialize ra API. Đọc tệp qua cacheMu, không giữ mu:
// một lượt gia hạn đang chạy không chặn lượt quét đọc cookie hiện có.
func (s *Service) CookieHeader(ctx context.Context) (string, error) {
	return s.cookieHeader()
}

// EnsureFresh tự gia hạn trước một lượt quét (tối đa một lần mỗi 24 giờ qua
// lastCheckedAt): lỗi gia hạn không chặn lượt quét — trả cookie đang có.
func (s *Service) EnsureFresh(ctx context.Context) string {
	_, _, _ = s.refresh(ctx, false)
	cookie, err := s.CookieHeader(ctx)
	if err != nil {
		return ""
	}
	return cookie
}

// WithBilibiliCookieJar chạy fn với đường dẫn tệp cookie Netscape riêng
// (quyền 0600, xoá ngay cả khi lỗi) — chỉ áp cho URL *.bilibili.com.
func (s *Service) WithBilibiliCookieJar(ctx context.Context, rawURL string, fn func(jar string) error) error {
	u, err := url.Parse(rawURL)
	if err != nil || u == nil {
		return fn("")
	}
	host := u.Hostname()
	if host != "bilibili.com" && !strings.HasSuffix(host, ".bilibili.com") {
		return fn("")
	}
	cookie, err := s.CookieHeader(ctx)
	if err != nil || cookie == "" {
		return fn("")
	}
	dir, err := os.MkdirTemp("", "viderse-bilibili-cookie-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	jar := filepath.Join(dir, "cookies.txt")
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	for _, part := range strings.Split(cookie, "; ") {
		i := strings.IndexByte(part, '=')
		if i <= 0 {
			continue
		}
		fmt.Fprintf(&b, ".bilibili.com\tTRUE\t/\tTRUE\t0\t%s\t%s\n", part[:i], part[i+1:])
	}
	if err := os.WriteFile(jar, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return fn(jar)
}
