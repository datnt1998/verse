package backdrop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

// bilibiliUserAgent trùng user-agent accounts dùng cho đăng nhập, để Bilibili
// thấy một trình duyệt duy nhất cho cả app.
const bilibiliUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

const (
	bilibiliAPIBase  = "https://api.bilibili.com"
	bilibiliHTTPTime = 20 * time.Second
	// bilibiliKeysTTL là hạn đệm khoá ký WBI.
	bilibiliKeysTTL = 24 * time.Hour
	// bilibiliPageSize là số mục mỗi trang danh mục không gian.
	bilibiliPageSize = 30
)

var bilibiliBVPattern = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)
var bilibiliLengthPattern = regexp.MustCompile(`^\d+(?::\d{1,2}){1,2}$`)
var bilibiliPicPattern = regexp.MustCompile(`(?i)^(?:https?:)?//[^\s]+$`)
var bilibiliMidPattern = regexp.MustCompile(`^\d{1,20}$`)

// BilibiliPage là một trang danh mục video của một không gian.
type BilibiliPage struct {
	Videos   []ListedVideo
	Total    int
	PageSize int
	Title    *string
}

// BilibiliAPIError là lỗi Bilibili trả về: mã API quyết định nhóm lỗi dùng
// chung, Retry-After (nếu có) chỉ kéo dài giờ chờ.
type BilibiliAPIError struct {
	APICode int
	Retry   time.Duration
}

func (e *BilibiliAPIError) Error() string {
	switch e.ErrorCode() {
	case CodeRateLimited:
		return "Bilibili đang giới hạn yêu cầu"
	case CodeUnavailable:
		return "Không tìm thấy kênh Bilibili"
	default:
		return "Chưa đọc được danh sách Bilibili"
	}
}

func (e *BilibiliAPIError) ErrorCode() string {
	switch e.APICode {
	case 412, 429, -412, -352, -401, -403, -799:
		return CodeRateLimited
	case -404, -400, -62002:
		return CodeUnavailable
	default:
		return CodeFailed
	}
}

// RetryAfter cho platforms.Pacer biết server muốn chờ bao lâu.
func (e *BilibiliAPIError) RetryAfter() time.Duration { return e.Retry }

// parseBilibiliPage kiểm hình dữ liệu trang danh mục; một mục sai hình làm hỏng
// cả trang (trang dở khiến video còn sống trông như đã mất).
func parseBilibiliPage(raw json.RawMessage) (BilibiliPage, error) {
	var payload struct {
		Page *struct {
			Count *int `json:"count"`
			PS    *int `json:"ps"`
		} `json:"page"`
		List *struct {
			Vlist []struct {
				Bvid    *string `json:"bvid"`
				Title   *string `json:"title"`
				Length  *string `json:"length"`
				Pic     *string `json:"pic"`
				Created *int64  `json:"created"`
				Author  *string `json:"author"`
			} `json:"vlist"`
		} `json:"list"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
	}
	if payload.Page == nil || payload.Page.Count == nil || *payload.Page.Count < 0 || payload.List == nil {
		return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
	}
	pageSize := bilibiliPageSize
	if payload.Page.PS != nil {
		if *payload.Page.PS < 1 || *payload.Page.PS > 100 {
			return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
		}
		pageSize = *payload.Page.PS
	}
	out := BilibiliPage{Total: *payload.Page.Count, PageSize: pageSize, Videos: make([]ListedVideo, 0, len(payload.List.Vlist))}
	for _, entry := range payload.List.Vlist {
		if entry.Bvid == nil || entry.Title == nil || entry.Length == nil || entry.Pic == nil || entry.Created == nil {
			return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
		}
		if !bilibiliBVPattern.MatchString(*entry.Bvid) || !bilibiliLengthPattern.MatchString(*entry.Length) {
			return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
		}
		if *entry.Pic != "" && !bilibiliPicPattern.MatchString(*entry.Pic) {
			return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
		}
		if *entry.Created < 0 {
			return BilibiliPage{}, &tools.YtDlpError{Message: "Bilibili trả về danh sách không đọc được", Code: tools.YtDlpFailed}
		}
		duration := bilibiliDuration(*entry.Length)
		video := ListedVideo{
			VideoID: *entry.Bvid,
			Kind:    domain.PlatformKindVideo,
			URL:     "https://www.bilibili.com/video/" + *entry.Bvid,
			Title:   entry.Title,
			// Độ dài nguồn: giây, làm tròn như bản TS giữ nguyên số nguyên.
			DurationSec: &duration,
		}
		if *entry.Pic != "" {
			video.ThumbnailURL = bilibiliThumbnail(*entry.Pic)
		}
		at := time.Unix(*entry.Created, 0).UTC()
		video.PublishedAt = &at
		out.Videos = append(out.Videos, video)
		if out.Title == nil && entry.Author != nil && *entry.Author != "" {
			author := *entry.Author
			out.Title = &author
		}
	}
	return out, nil
}

// bilibiliDuration đổi "mm:ss" / "hh:mm:ss" sang giây.
func bilibiliDuration(length string) float64 {
	total := 0.0
	for _, part := range strings.Split(length, ":") {
		value, err := strconv.Atoi(part)
		if err != nil {
			return 0
		}
		total = total*60 + float64(value)
	}
	return total
}

func bilibiliThumbnail(pic string) *string {
	value := pic
	switch {
	case strings.HasPrefix(value, "//"):
		value = "https:" + value
	case strings.HasPrefix(strings.ToLower(value), "http:"):
		value = "https://" + value[len("http:"):]
	}
	return &value
}

// bilibiliClient là client API Bilibili: ký WBI, đệm khoá 24 giờ, danh tính
// thiết bị 0600, cookie đăng nhập khi đã lưu. Mọi lời gọi đi qua `request`
// (pacing) nên nhịp và giờ chờ bị chặn do một nơi quyết định.
type bilibiliClient struct {
	doer    HTTPDoer
	now     func() time.Time
	request func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error
	device  *deviceStore
	cookie  func(ctx context.Context) (string, error)
	mu      sync.Mutex
	state   deviceState
}

func newBilibiliClient(doer HTTPDoer, now func() time.Time, request func(ctx context.Context, resetBackoff bool, fn func(context.Context) error) error, device *deviceStore, cookie func(ctx context.Context) (string, error)) (*bilibiliClient, error) {
	state, err := device.load()
	if err != nil {
		return nil, err
	}
	return &bilibiliClient{doer: doer, now: now, request: request, device: device, cookie: cookie, state: state}, nil
}

func (c *bilibiliClient) saveLocked() error {
	return c.device.save(c.state)
}

// get gọi một endpoint API và trả `data` khi mã 0 (nav ẩn danh chấp nhận -101).
func (c *bilibiliClient) get(ctx context.Context, endpoint, mid string, allowAnonymousNav bool) (json.RawMessage, error) {
	owner := ""
	if c.cookie != nil {
		header, err := c.cookie(ctx)
		if err != nil {
			return nil, err
		}
		owner = header
	}
	deviceCookie := fmt.Sprintf("buvid3=%s; buvid4=%s", c.state.Buvid3, c.state.Buvid4)
	ownerWithoutDevice := stripDeviceCookies(owner)
	cookie := deviceCookie
	if ownerWithoutDevice != "" {
		cookie = deviceCookie + "; " + ownerWithoutDevice
	}
	var data json.RawMessage
	err := c.request(ctx, strings.HasPrefix(endpoint, "/x/space/wbi/arc/search"), func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, bilibiliAPIBase+endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", bilibiliUserAgent)
		req.Header.Set("Referer", "https://space.bilibili.com/"+mid+"/video")
		req.Header.Set("Origin", "https://space.bilibili.com")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Cookie", cookie)
		resp, err := c.doer.Do(req)
		if err != nil {
			return &tools.YtDlpError{Message: "Không kết nối được Bilibili. Kiểm tra mạng rồi thử lại.", Code: tools.YtDlpFailed}
		}
		defer resp.Body.Close()
		body, err := readAllLimited(resp.Body)
		if err != nil {
			return &tools.YtDlpError{Message: "Bilibili trả về dữ liệu không đọc được", Code: tools.YtDlpFailed}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &BilibiliAPIError{APICode: resp.StatusCode, Retry: parseRetryAfter(resp.Header.Get("Retry-After"), c.now())}
		}
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return &tools.YtDlpError{Message: "Bilibili trả về dữ liệu không đọc được", Code: tools.YtDlpFailed}
		}
		if envelope.Code != 0 && !(allowAnonymousNav && envelope.Code == -101) {
			return &BilibiliAPIError{APICode: envelope.Code, Retry: parseRetryAfter(resp.Header.Get("Retry-After"), c.now())}
		}
		data = envelope.Data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ensureKeys nạp khoá ký WBI, dùng đệm khi còn hạn.
func (c *bilibiliClient) ensureKeys(ctx context.Context, mid string) error {
	c.mu.Lock()
	fresh := c.state.keysFresh(c.now())
	c.mu.Unlock()
	if fresh {
		return nil
	}
	raw, err := c.get(ctx, "/x/web-interface/nav", mid, true)
	if err != nil {
		return err
	}
	var nav struct {
		WbiImg *struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	}
	if err := json.Unmarshal(raw, &nav); err != nil || nav.WbiImg == nil {
		return &tools.YtDlpError{Message: "Chưa đọc được khoá truy cập Bilibili", Code: tools.YtDlpFailed}
	}
	img, ok := wbiKeyFromURL(nav.WbiImg.ImgURL)
	if !ok {
		return &tools.YtDlpError{Message: "Khoá truy cập Bilibili không hợp lệ", Code: tools.YtDlpFailed}
	}
	sub, ok := wbiKeyFromURL(nav.WbiImg.SubURL)
	if !ok {
		return &tools.YtDlpError{Message: "Khoá truy cập Bilibili không hợp lệ", Code: tools.YtDlpFailed}
	}
	c.mu.Lock()
	c.state.ImgKey, c.state.SubKey = img, sub
	c.state.KeysExpireAt = c.now().Add(bilibiliKeysTTL).UnixMilli()
	err = c.saveLocked()
	c.mu.Unlock()
	return err
}

func wbiKeyFromURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	segments := strings.Split(u.Path, "/")
	stem := strings.Split(segments[len(segments)-1], ".")[0]
	if !wbiKeyPattern.MatchString(stem) {
		return "", false
	}
	return stem, true
}

// Page đọc một trang danh mục của không gian `mid`.
func (c *bilibiliClient) Page(ctx context.Context, mid string, pn int) (BilibiliPage, error) {
	if !bilibiliMidPattern.MatchString(mid) || pn < 1 {
		return BilibiliPage{}, &tools.YtDlpError{Message: "Mã kênh hoặc trang Bilibili không hợp lệ", Code: tools.YtDlpFailed}
	}
	if err := c.ensureKeys(ctx, mid); err != nil {
		return BilibiliPage{}, err
	}
	c.mu.Lock()
	imgKey, subKey := c.state.ImgKey, c.state.SubKey
	c.mu.Unlock()
	params := map[string]string{
		"mid": mid, "pn": strconv.Itoa(pn), "ps": strconv.Itoa(bilibiliPageSize), "order": "pubdate",
		"tid": "0", "keyword": "", "platform": "web", "order_avoided": "true",
		"web_location": "333.1387", "special_type": "", "index": "0",
		"dm_img_list": "[]", "dm_img_str": bilibiliNoise(16, 64), "dm_cover_img_str": bilibiliNoise(32, 128),
		"dm_img_inter": bilibiliInter(),
	}
	signed := SignWBI(params, imgKey, subKey, c.now().Unix())
	raw, err := c.get(ctx, "/x/space/wbi/arc/search?"+signed, mid, false)
	if err != nil {
		// Hỏng khoá: lần sau nạp lại, nhưng không thử lại trong cùng lượt với
		// phản hồi risk-control.
		if errorCode(err) == CodeRateLimited {
			c.mu.Lock()
			c.state.KeysExpireAt = 0
			_ = c.saveLocked()
			c.mu.Unlock()
		}
		return BilibiliPage{}, err
	}
	return parseBilibiliPage(raw)
}

// bilibiliNoise tạo chuỗi base64 ngẫu nhiên giống nhiễu ảnh thiết bị yt-dlp
// gửi, thay cho vector 0.
func bilibiliNoise(minLength, maxLength int) string {
	printable := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ \t\n\r\u000b\f"
	n := minLength + rand.IntN(maxLength-minLength+1)
	var b strings.Builder
	for range n {
		b.WriteByte(printable[rand.IntN(len(printable))])
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(b.String()))
	if len(encoded) > 2 {
		encoded = encoded[:len(encoded)-2]
	}
	return encoded
}

func bilibiliInter() string {
	r := rand.IntN(114)
	top := rand.IntN(101)
	s := rand.IntN(514)
	payload := map[string]any{
		"ds": []any{},
		"wh": []int{6000 + 3*r, 6600 + r, r},
		"of": []int{3*top + s, 4*top + 2*s, s},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return `{"ds":[],"wh":[],"of":[]}`
	}
	return string(raw)
}

// stripDeviceCookies bỏ buvid do cookies đăng nhập mang theo: danh tính thiết
// bị của app phải thắng.
func stripDeviceCookies(header string) string {
	parts := strings.Split(header, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || strings.HasPrefix(trimmed, "buvid3=") || strings.HasPrefix(trimmed, "buvid4=") {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "; ")
}

// parseRetryAfter đọc Retry-After dạng giây hoặc ngày HTTP.
func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds * float64(time.Second))
	}
	if at, err := http.ParseTime(value); err == nil {
		if d := at.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// readAllLimited đọc body API với trần kích thước để một phản hồi lạ không ăn
// hết bộ nhớ.
func readAllLimited(body io.Reader) ([]byte, error) {
	const limit = 8 << 20
	buf, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(buf) > limit {
		return nil, fmt.Errorf("phản hồi quá lớn")
	}
	return buf, nil
}
