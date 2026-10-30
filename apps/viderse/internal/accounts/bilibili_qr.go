package accounts

// Đăng nhập Bilibili bằng mã QR và tự gia hạn (V24) — chuyển từ
// src/server/backdrop/bilibili-qr-login.ts: tạo mã kèm buvid3/4, hỏi mã
// (86101 chờ · 86090 đã quét · 86038 hết hạn · 0 xong · khác từ chối), lưu
// cookie + refresh token một lần ghi, chuỗi gia hạn cookie/info → correspond
// → cookie/refresh → confirm/refresh tối đa một lần mỗi 24 giờ trước khi quét.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/platform/identity"
)

const (
	qrTTL         = 180 * time.Second // mã sống 3 phút
	refreshMinGap = 24 * time.Hour    // tự gia hạn tối đa một lần mỗi ngày
	renewError    = "Đăng nhập Bilibili đã hết hạn — quét mã QR lại"
)

// bilibiliPublicKey là khoá RSA công khai mà Bilibili dùng để mã hoá
// timestamp trong bước correspond (nguyên PEM của bản TS).
const bilibiliPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDLgd2OAkcGVtoE3ThUREbio0Eg
Uc/prcajMKXvkCKFCWhJYJcLkcM2DKKcSeFpD/j6Boy538YXnR6VhcuUJOhH2x71
nzPjfdTcqMz7djHum0qSZA0AyCBDABUqCrfNgCiJ00Ra7GmRj+YCK1NJEuewlb40
JNrRuoEUXpabUzGB8QIDAQAB
-----END PUBLIC KEY-----`

var (
	keyPattern   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,256}$`)
	valuePattern = regexp.MustCompile(`^[^;\s\x00-\x1f\x7f]+$`)
	csrfPattern  = regexp.MustCompile(`(?i)<div\b[^>]*\bid\s*=\s*["']1-name["'][^>]*>\s*([^<]+?)\s*</div>`)
)

// qrSessionFile là phiên QR đang mở, giữ trên đĩa để hỏi mã đúng phiên cả
// khi process khởi động lại giữa chừng (port sessionSchema).
type qrSessionFile struct {
	Key       string            `json:"key"`
	ExpiresAt int64             `json:"expiresAt"` // mili-giây, hình bản TS
	Device    map[string]string `json:"device"`
}

type envelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

// bilibiliCorrespondPath mã hoá "refresh_<timestamp>" bằng RSA-OAEP-SHA256
// trên khoá công khai của Bilibili — 128 byte, hex 256 ký tự.
func bilibiliCorrespondPath(timestamp int64) (string, error) {
	block, _ := pem.Decode([]byte(bilibiliPublicKey))
	if block == nil {
		return "", errors.New("khóa Bilibili không đọc được")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("khóa Bilibili không phải RSA")
	}
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, key, []byte(fmt.Sprintf("refresh_%d", timestamp)), nil)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ciphertext), nil
}

// request thực hiện một lời gọi Bilibili với UA/Referer chuẩn, cookie tuỳ
// chọn và form POST tuỳ chọn; timeout đã nằm trong client. Lỗi mạng và lỗi
// trả lời đều là ReasonUnavailable — copy "Chưa kết nối được Bilibili".
func (s *Service) request(ctx context.Context, rawURL, cookie string, form url.Values) (*http.Response, error) {
	var body io.Reader
	method := http.MethodGet
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", bilibiliUserAgent)
	req.Header.Set("Referer", s.hosts.site()+"/")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, unavailablef("Không kết nối được Bilibili. Kiểm tra mạng rồi thử lại.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, unavailablef("Bilibili chưa trả lời. Thử lại sau ít phút.")
	}
	return resp, nil
}

// envelopeOf đọc bao {code, data} của Bilibili.
func (s *Service) envelopeOf(resp *http.Response) (*envelope, error) {
	defer resp.Body.Close()
	var env envelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&env); err != nil {
		return nil, unavailablef("Bilibili trả về dữ liệu không đọc được. Thử lại sau.")
	}
	return &env, nil
}

// dataOf giống envelopeOf nhưng bắt buộc code=0.
func (s *Service) dataOf(resp *http.Response) (json.RawMessage, error) {
	env, err := s.envelopeOf(resp)
	if err != nil {
		return nil, err
	}
	if env.Code != 0 {
		return nil, rejectedf("Bilibili chưa chấp nhận yêu cầu. Quét mã QR lại hoặc thử sau.")
	}
	return env.Data, nil
}

// cookieString ghép map cookie thành chuỗi "a=b; c=d" theo thứ tự ổn định
// của map gốc (device trước, tham số/Set-Cookie ghi đè sau).
func cookieString(cookies map[string]string, order []string) string {
	cookies = mapCopy(cookies)
	var b strings.Builder
	seen := 0
	emit := func(name, value string) {
		if seen > 0 {
			b.WriteString("; ")
		}
		b.WriteString(name + "=" + value)
		seen++
	}
	for _, name := range order {
		if v, ok := cookies[name]; ok {
			emit(name, v)
			delete(cookies, name)
		}
	}
	// Các khoá còn lại (Set-Cookie ngoài order) theo thứ tự tên.
	var rest []string
	for k := range cookies {
		rest = append(rest, k)
	}
	sortStrings(rest)
	for _, name := range rest {
		emit(name, cookies[name])
	}
	return b.String()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// mergeResponseCookies gộp Set-Cookie của một response vào map (port
// mergeResponseCookies): giá trị hợp lệ thì set, không hợp lệ thì xoá.
func mergeResponseCookies(cookies map[string]string, resp *http.Response) {
	for _, c := range resp.Cookies() {
		if valuePattern.MatchString(c.Value) {
			cookies[c.Name] = c.Value
		} else {
			delete(cookies, c.Name)
		}
	}
}

// requiredCookie bắt buộc có SESSDATA và bili_jct rồi chuẩn hoá — thiếu là
// lỗi "đăng nhập thiếu thông tin".
func requiredCookie(cookies map[string]string, order []string) (string, error) {
	if cookies["SESSDATA"] == "" || cookies["bili_jct"] == "" {
		return "", rejectedf("Bilibili trả về đăng nhập thiếu thông tin. Quét mã QR lại.")
	}
	return NormalizeBilibiliCookie(cookieString(cookies, order))
}

func mapCopy(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// StartQR tạo phiên đăng nhập QR: lấy buvid3/4 (best-effort), gọi
// qrcode/generate, kiểm loginUrl phải thuộc bilibili.com, ghi phiên 0600 và
// render QR bằng cùng bộ mã hoá QR của identity (ECC M, quiet zone 2 — 底板有 sẵn padding).
func (s *Service) StartQR(ctx context.Context) (domain.QRSession, error) {
	var out domain.QRSession
	err := s.withLock(func() error {
		device := map[string]string{}
		var order []string
		if resp, err := s.request(ctx, s.hosts.api()+"/x/frontend/finger/spi", "", nil); err == nil {
			var d struct {
				B3 string `json:"b_3"`
				B4 string `json:"b_4"`
			}
			if data, err := s.dataOf(resp); err == nil && json.Unmarshal(data, &d) == nil &&
				valuePattern.MatchString(d.B3) && valuePattern.MatchString(d.B4) {
				device = map[string]string{"buvid3": d.B3, "buvid4": d.B4}
				order = []string{"buvid3", "buvid4"}
			}
		}
		// Tra cứu thiết bị là best-effort: Bilibili vẫn phát mã không cần buvid.
		resp, err := s.request(ctx, s.hosts.passport()+"/qrcode/generate", cookieString(device, order), nil)
		if err != nil {
			return err
		}
		data, err := s.dataOf(resp)
		if err != nil {
			return err
		}
		var gen struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		}
		if err := json.Unmarshal(data, &gen); err != nil || gen.URL == "" || !keyPattern.MatchString(gen.QRCodeKey) {
			return unavailablef("Chưa tạo được mã QR Bilibili. Thử tạo mã mới.")
		}
		loginURL, err := url.Parse(gen.URL)
		if err != nil || loginURL.Scheme != "https" ||
			(loginURL.Hostname() != "bilibili.com" && !strings.HasSuffix(loginURL.Hostname(), ".bilibili.com")) {
			return invalidf("Đường dẫn đăng nhập Bilibili không hợp lệ.")
		}
		expiresAt := s.now().Add(qrTTL)
		session := qrSessionFile{Key: gen.QRCodeKey, ExpiresAt: expiresAt.UnixMilli(), Device: device}
		if err := writePrivateAtomic(s.qrFile(), session); err != nil {
			return errors.New("chưa lưu được phiên đăng nhập Bilibili. Thử tạo mã mới")
		}
		// Khung QR trong dialog có sẵn 14px padding: 静区 2 模块已让墨迹放大
		// 而仍保持 ≥2 模块白边（规范下限），匹配 artboard 墨迹顶到板内边的观感。
		svg, err := identity.QRSVGQuiet(gen.URL, 2)
		if err != nil {
			return errors.New("chưa vẽ được mã QR Bilibili. Thử tạo mã mới")
		}
		out = domain.QRSession{
			QRCodeKey: gen.QRCodeKey,
			QRImage:   "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)),
			LoginURL:  gen.URL,
			ExpiresAt: expiresAt,
		}
		return nil
	})
	return out, err
}

// readQRSession đọc phiên đang mở; nil khi chưa tạo mã.
func (s *Service) readQRSession() (*qrSessionFile, error) {
	raw, err := os.ReadFile(s.qrFile())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("chưa đọc được mã QR Bilibili. Tạo mã mới")
	}
	var session qrSessionFile
	if err := json.Unmarshal(raw, &session); err != nil || !keyPattern.MatchString(session.Key) {
		return nil, errors.New("chưa đọc được mã QR Bilibili. Tạo mã mới")
	}
	return &session, nil
}

// PollQR hỏi trạng thái một phiên QR (port pollBilibiliQrLogin). Phiên hết
// hạn cục bộ trả expired mà không gọi Bilibili — đây cũng là cơ chế "đóng
// hộp thoại thì dừng hỏi": server chỉ gọi mạng khi client hỏi.
func (s *Service) PollQR(ctx context.Context, key string) (domain.QRPoll, error) {
	var out domain.QRPoll
	err := s.withLock(func() error {
		if !keyPattern.MatchString(key) {
			return invalidf("Mã đăng nhập Bilibili không hợp lệ. Tạo mã QR mới.")
		}
		session, err := s.readQRSession()
		if err != nil {
			return err
		}
		if session == nil || session.Key != key || !s.now().Before(time.UnixMilli(session.ExpiresAt)) {
			out = domain.QRPoll{Status: domain.QRExpired}
			return nil
		}
		var order []string
		deviceCookie := map[string]string{}
		for k, v := range session.Device {
			deviceCookie[k] = v
			order = append(order, k)
		}
		resp, err := s.request(ctx,
			s.hosts.passport()+"/qrcode/poll?"+url.Values{"qrcode_key": {key}, "source": {"main-fe-header"}}.Encode(),
			cookieString(deviceCookie, order), nil)
		if err != nil {
			return err
		}
		data, err := s.dataOf(resp)
		if err != nil {
			return err
		}
		var polled struct {
			Code         int    `json:"code"`
			URL          string `json:"url"`
			RefreshToken string `json:"refresh_token"`
			Timestamp    int64  `json:"timestamp"`
			Message      string `json:"message"`
		}
		if err := json.Unmarshal(data, &polled); err != nil {
			return unavailablef("Chưa đọc được trạng thái QR Bilibili. Tạo mã mới.")
		}
		switch polled.Code {
		case 86101:
			out = domain.QRPoll{Status: domain.QRWaiting}
			return nil
		case 86090:
			out = domain.QRPoll{Status: domain.QRScanned}
			return nil
		case 86038:
			out = domain.QRPoll{Status: domain.QRExpired}
			return nil
		}
		if polled.Code != 0 {
			out = domain.QRPoll{Status: domain.QRRejected}
			return nil
		}
		// code=0: gộp cookie từ URL trả về + Set-Cookie, bắt buộc SESSDATA +
		// bili_jct, xác nhận tài khoản rồi commit một lần ghi.
		cookies := mapCopy(deviceCookie)
		if polled.URL != "" {
			u, err := url.Parse(polled.URL)
			if err != nil {
				return invalidf("Bilibili trả về đăng nhập không hợp lệ. Tạo mã mới.")
			}
			for _, name := range []string{"SESSDATA", "bili_jct", "DedeUserID", "DedeUserID__ckMd5"} {
				if v := u.Query().Get(name); v != "" {
					cookies[name] = v
				}
			}
		}
		mergeResponseCookies(cookies, resp)
		cookie, err := requiredCookie(cookies, order)
		if err != nil {
			return err
		}
		var account *string
		var refreshError *string
		uname, err := s.verifyBilibiliCookie(ctx, cookie)
		switch {
		case err == nil:
			if uname != "" {
				account = &uname
			}
		default:
			le, isLoginErr := asLoginError(err)
			if isLoginErr && le.Reason == ReasonRejected {
				out = domain.QRPoll{Status: domain.QRRejected}
				return nil
			}
			if !isLoginErr || le.Reason != ReasonUnavailable {
				return err
			}
			msg := "Đã nhận đăng nhập; chưa kiểm tra được tài khoản do kết nối Bilibili. Thử gia hạn khi mạng ổn định."
			refreshError = &msg
		}
		nowMs := s.now().UnixMilli()
		st := &bilibiliLoginState{
			Cookie:        cookie,
			Method:        "qr",
			Account:       account,
			RefreshError:  refreshError,
			LastCheckedAt: &nowMs,
		}
		if valuePattern.MatchString(polled.RefreshToken) {
			st.RefreshToken = &polled.RefreshToken
			csrf := cookies["bili_jct"]
			if secretPattern.MatchString(csrf) {
				st.CSRF = &csrf
			}
		}
		if _, err := s.commitBilibiliLogin(st); err != nil {
			return err
		}
		_ = os.Remove(s.qrFile())
		out = domain.QRPoll{Status: domain.QRSuccess, Account: account}
		return nil
	})
	if err == nil && out.Status == domain.QRSuccess {
		s.notify(domain.PlatformBilibili)
	}
	return out, err
}

// Refresh gia hạn đăng nhập QR (port refreshBilibiliLogin): force bỏ qua
// ngưỡng 24 giờ của lastCheckedAt. Thất bại ghi refreshError vào snapshot
// (thẻ "Cần đăng nhập lại") và giữ nguyên cookie/token cũ.
func (s *Service) Refresh(ctx context.Context) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	return s.refresh(ctx, true)
}

func (s *Service) refresh(ctx context.Context, force bool) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	var outcome domain.RefreshOutcome
	var login domain.PlatformLogin
	err := s.withLock(func() error {
		state, err := s.readLoginState()
		if err != nil {
			return err
		}
		current := func() error {
			var lerr error
			login, lerr = s.bilibiliLogin(ctx)
			return lerr
		}
		if state == nil || state.Method != "qr" || state.RefreshToken == nil || state.CSRF == nil {
			outcome = domain.RefreshSkipped
			return current()
		}
		if !force && state.LastCheckedAt != nil && s.now().UnixMilli()-*state.LastCheckedAt < int64(refreshMinGap/time.Millisecond) {
			outcome = domain.RefreshSkipped
			return current()
		}
		outcome = domain.RefreshFailed
		if rerr := s.renew(ctx, state); rerr != nil {
			if errors.Is(rerr, errStillCurrent) {
				outcome = domain.RefreshCurrent
				return current()
			}
			nowMs := s.now().UnixMilli()
			msg := renewError
			state.LastCheckedAt = &nowMs
			state.RefreshError = &msg
			_ = s.writeLoginState(state)
			s.notify(domain.PlatformBilibili)
			return current()
		}
		outcome = domain.RefreshRefreshed
		s.notify(domain.PlatformBilibili)
		return current()
	})
	return outcome, login, err
}

// renew chạy chuỗi gia hạn một lần: cookie/info → (correspond → refresh →
// confirm) → nav kiểm → commit. Mọi lỗi trả ra ngoài để caller ghi
// refreshError — bản thân không đổi trạng thái đã lưu.
func (s *Service) renew(ctx context.Context, state *bilibiliLoginState) error {
	// cookie/info?csrf=<csrf> — cookie/Referer của phiên cũ.
	resp, err := s.request(ctx,
		s.hosts.passport()+"/cookie/info?"+url.Values{"csrf": {*state.CSRF}}.Encode(),
		state.Cookie, nil)
	if err != nil {
		return err
	}
	env, err := s.envelopeOf(resp)
	if err != nil {
		return err
	}
	var timestamp int64
	if env.Code == -101 {
		// Phiên đã chết theo Bilibili: vẫn phải chạy correspond/refresh để
		// thử cứu — timestamp lấy đồng hồ máy như bản TS.
		timestamp = s.now().UnixMilli()
	} else {
		if env.Code != 0 {
			return errors.New(renewError)
		}
		var info struct {
			Refresh   bool  `json:"refresh"`
			Timestamp int64 `json:"timestamp"`
		}
		if err := json.Unmarshal(env.Data, &info); err != nil {
			return errors.New(renewError)
		}
		if !info.Refresh {
			nowMs := s.now().UnixMilli()
			state.LastCheckedAt = &nowMs
			state.RefreshError = nil
			if err := s.writeLoginState(state); err != nil {
				return err
			}
			return errStillCurrent
		}
		timestamp = info.Timestamp
	}
	path, err := bilibiliCorrespondPath(timestamp)
	if err != nil {
		return err
	}
	// correspond trả HTML chứa refresh_csrf trong <div id="1-name">…</div>.
	resp, err = s.request(ctx, s.hosts.site()+"/correspond/1/"+path, state.Cookie, nil)
	if err != nil {
		return err
	}
	html, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	m := csrfPattern.FindSubmatch(html)
	if m == nil || !valuePattern.MatchString(strings.TrimSpace(string(m[1]))) {
		return errors.New(renewError)
	}
	refreshCsrf := strings.TrimSpace(string(m[1]))
	// cookie/refresh — form với csrf cũ + refresh_csrf mới + refresh_token cũ.
	resp, err = s.request(ctx, s.hosts.passport()+"/cookie/refresh", state.Cookie, url.Values{
		"csrf":          {*state.CSRF},
		"refresh_csrf":  {refreshCsrf},
		"source":        {"main_web"},
		"refresh_token": {*state.RefreshToken},
	})
	if err != nil {
		return err
	}
	data, err := s.dataOf(resp)
	if err != nil {
		return err
	}
	var refreshed struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(data, &refreshed); err != nil || !secretPattern.MatchString(refreshed.RefreshToken) {
		return errors.New(renewError)
	}
	// Set-Cookie của refresh PHẢI có đủ SESSDATA + bili_jct mới.
	issued := map[string]string{}
	mergeResponseCookies(issued, resp)
	if _, err := requiredCookie(issued, nil); err != nil {
		return err
	}
	// Gộp cookie mới lên cookie cũ — csrf mới = bili_jct mới.
	cookies := map[string]string{}
	for _, part := range strings.Split(state.Cookie, ";") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, '='); i > 0 {
			cookies[part[:i]] = part[i+1:]
		}
	}
	var order []string
	for k := range cookies {
		order = append(order, k)
	}
	sortStrings(order)
	mergeResponseCookies(cookies, resp)
	cookie, err := requiredCookie(cookies, order)
	if err != nil {
		return err
	}
	newCsrf := cookies["bili_jct"]
	// confirm/refresh — cookie MỚI + csrf MỚI + refresh_token CŨ.
	resp, err = s.request(ctx, s.hosts.passport()+"/confirm/refresh", cookie, url.Values{
		"csrf":          {newCsrf},
		"refresh_token": {*state.RefreshToken},
	})
	if err != nil {
		return err
	}
	if _, err := s.dataOf(resp); err != nil {
		return err
	}
	account, err := s.verifyBilibiliCookie(ctx, cookie)
	if err != nil {
		return err
	}
	nowMs := s.now().UnixMilli()
	st := &bilibiliLoginState{
		Cookie:        cookie,
		Method:        "qr",
		RefreshToken:  &refreshed.RefreshToken,
		CSRF:          &newCsrf,
		LastCheckedAt: &nowMs,
		LastRefreshAt: &nowMs,
	}
	if account != "" {
		st.Account = &account
	}
	_, err = s.commitBilibiliLogin(st)
	return err
}

// errStillCurrent đánh dấu nhánh "đăng nhập vẫn còn hạn" — không phải lỗi.
var errStillCurrent = errors.New("current")
