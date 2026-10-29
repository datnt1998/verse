package youtube

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// defaultBaseURL là gốc endpoint InnerTube web; giống PRODUCTION_1 của youtubei.js.
	defaultBaseURL  = "https://www.youtube.com"
	apiVersion      = "v1"
	webClientNameID = "1"
	// maxBodyBytes giới hạn đọc thân phản hồi; trang kênh thật vài trăm KiB.
	maxBodyBytes       = 16 << 20
	statusSnippetBytes = 2048
	maxResumedSessions = 4
	// userAgent giữ vai trò getRandomUserAgent('desktop'): một UA desktop cố định,
	// vì phân tích UA không nằm trong phạm vi parser này.
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
)

// Options tuỳ biến Client. Trường nil/0 dùng mặc định sản xuất.
type Options struct {
	// HTTPClient thực hiện request; mặc định http.DefaultClient.
	// Test tiêm client với Transport là fixture transport.
	HTTPClient *http.Client
	// BaseURL gốc endpoint (mặc định https://www.youtube.com); test trỏ tới httptest.
	BaseURL string
	// Clock đồng hồ cho bộ điều tốc và parseRetryAfter; mặc định time.Now.
	Clock func() time.Time
	// Pacer bộ điều tốc chia sẻ; mặc định NewPacer() (nhịp 1–2s, chờ tối đa 5s).
	// Test thường truyền pacer với MinGap=MaxGap=0.
	Pacer *Pacer
}

// Client là một phiên InnerTube tự chứa: session cache + bộ điều tốc + HTTP.
type Client struct {
	http    *http.Client
	baseURL string
	now     func() time.Time
	pacer   *Pacer

	mu       sync.Mutex
	shared   *session
	resumed  map[string]*session
	resumeQ  []string                  // FIFO cắt giữ maxResumedSessions
	inflight map[string]*sessionFlight // bootstrap đang chạy theo khoá ("" = shared)
}

// sessionFlight là một lần bootstrap đang chạy; waiter chờ kênh done bằng ctx
// của chính nó chứ không bị kẹt theo deadline của caller khởi tạo.
type sessionFlight struct {
	done chan struct{}
	s    *session
	err  error
}

// NewClient tạo client; mọi phụ thuộc đi qua Options (không trạng thái toàn cục).
func NewClient(opts Options) *Client {
	hc := opts.HTTPClient
	if hc == nil {
		// Timeout tổng 60s: YouTube trả thân lớn theo lát nên chỉ cần rào
		// tiến trình không treo vô hạn trên một request.
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	now := opts.Clock
	if now == nil {
		now = time.Now
	}
	pacer := opts.Pacer
	if pacer == nil {
		pacer = NewPacer()
	}
	if pacer.Now == nil && opts.Clock != nil {
		pacer.Now = now
	}
	return &Client{
		http:     hc,
		baseURL:  strings.TrimRight(base, "/"),
		now:      now,
		pacer:    pacer,
		resumed:  map[string]*session{},
		inflight: map[string]*sessionFlight{},
	}
}

// session là một phiên InnerTube đã bootstrap: api key + bối cảnh client.
type session struct {
	client      *Client
	ctx         jmap // context.client gửi trong mọi request body
	visitorData string
	clientName  string
	clientVer   string
}

// sessionFor trả session cho visitorData ("" = session chia sẻ). Bootstrap lỗi
// không được cache: lần gọi sau bootstrap lại. Bootstrap của caller khác bị huỷ
// giữa chừng không làm caller này lỗi — nó tự bootstrap lại một lần.
func (c *Client) session(ctx context.Context, visitorData string) (*session, error) {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		var s *session
		if visitorData == "" {
			s = c.shared
		} else {
			s = c.resumed[visitorData]
		}
		if s != nil {
			c.mu.Unlock()
			// Cursor do session chia sẻ phát cũng phải tìm lại được qua visitorData của nó.
			if visitorData == "" && s.visitorData != "" {
				c.mu.Lock()
				if _, ok := c.resumed[s.visitorData]; !ok {
					c.rememberLocked(s.visitorData, s)
				}
				c.mu.Unlock()
			}
			return s, nil
		}
		if fl, ok := c.inflight[visitorData]; ok {
			c.mu.Unlock()
			// Waiter tôn trọng ctx của chính nó: huỷ/deadline của waiter trả
			// ngay, chủ flight vẫn hoàn tất và cache kết quả cho caller sau.
			select {
			case <-fl.done:
				if fl.err != nil {
					if attempt == 0 && errors.Is(fl.err, context.Canceled) && ctx.Err() == nil {
						continue // bootstrap của caller đã huỷ không làm hỏng waiter còn sống
					}
					return nil, fl.err
				}
				continue // session đã cache; vòng kế đọc lại
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		fl := &sessionFlight{done: make(chan struct{})}
		c.inflight[visitorData] = fl
		c.mu.Unlock()

		s, err := c.bootstrap(ctx, visitorData)

		c.mu.Lock()
		delete(c.inflight, visitorData)
		if err == nil {
			if visitorData == "" {
				c.shared = s
				// Cursor do session chia sẻ phát phải resume trên chính session
				// đó (cùng visitor data), không bootstrap lần hai.
				if s.visitorData != "" {
					c.rememberLocked(s.visitorData, s)
				}
			} else {
				c.rememberLocked(visitorData, s)
			}
		}
		fl.s, fl.err = s, err
		close(fl.done)
		c.mu.Unlock()

		if err != nil {
			// Bootstrap của một caller đã huỷ không được làm hỏng caller còn sống.
			if attempt == 0 && errors.Is(err, context.Canceled) && ctx.Err() == nil {
				continue
			}
			return nil, err
		}
		return s, nil
	}
}

// rememberLocked cất session resumed, đuổi khóa cũ nhất quá maxResumedSessions.
func (c *Client) rememberLocked(key string, s *session) {
	if _, ok := c.resumed[key]; !ok {
		c.resumeQ = append(c.resumeQ, key)
	}
	c.resumed[key] = s
	for len(c.resumeQ) > maxResumedSessions {
		oldest := c.resumeQ[0]
		c.resumeQ = c.resumeQ[1:]
		delete(c.resumed, oldest)
	}
}

// sessionContext trả bối cảnh client để dập vào cursor.
func (s *session) context() SessionContext {
	return SessionContext{
		ClientName:    s.clientName,
		ClientVersion: s.clientVer,
		VisitorData:   s.visitorData,
	}
}

// bootstrap gọi sw.js_data để lấy api key, client version và visitor data.
// visitorData != "" thì gửi id đã giải mã của nó trong cookie VISITOR_INFO1_LIVE
// và session nhận lại đúng visitor data đó.
func (c *Client) bootstrap(ctx context.Context, visitorData string) (*session, error) {
	visitorID := randomString(11)
	if visitorData != "" {
		if id, err := decodeVisitorDataID(visitorData); err == nil {
			visitorID = id
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sw.js_data", nil)
	if err != nil {
		return nil, err
	}
	tz := "UTC"
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", c.baseURL+"/sw.js")
	req.Header.Set("Cookie", fmt.Sprintf("PREF=tz=%s;VISITOR_INFO1_LIVE=%s;", strings.ReplaceAll(tz, "/", "."), visitorID))

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, &errSession{msg: "session data body unreadable"}
	}
	text := string(raw)
	// Prefix JSPB gồm ")]}'" + một ký tự xuống dòng; bản thân prefix còn
	// thiếu cũng là lỗi format, không được cắt mảng mù (panic từng có thể xảy ra).
	if len(text) <= len(")]}'") || !strings.HasPrefix(text, ")]}'") {
		return nil, &errSession{msg: "incorrect JSPB formatting"}
	}
	var data []any
	if err := json.Unmarshal([]byte(text[len(")]}'"):]), &data); err != nil {
		return nil, &errSession{msg: "session data is not JSON"}
	}
	device, err := parseSWData(data)
	if err != nil {
		return nil, err
	}
	vd := visitorData
	if vd == "" {
		vd = devField(device, 13)
	}
	clientVer := devField(device, 16)
	client := jmap{
		"hl":                 "en",
		"gl":                 "US",
		"remoteHost":         devField(device, 3),
		"visitorData":        vd,
		"clientName":         "WEB",
		"clientVersion":      clientVer,
		"osName":             devField(device, 17),
		"osVersion":          devField(device, 18),
		"userAgent":          userAgent,
		"platform":           "DESKTOP",
		"clientFormFactor":   "UNKNOWN_FORM_FACTOR",
		"timeZone":           devField(device, 79),
		"deviceMake":         devField(device, 11),
		"deviceModel":        devField(device, 12),
		"browserName":        devField(device, 86),
		"browserVersion":     devField(device, 87),
		"originalUrl":        c.baseURL,
		"utcOffsetMinutes":   -utcOffsetMinutes(c.now()),
		"screenDensityFloat": 1,
		"screenWidthPoints":  2560,
		"screenHeightPoints": 1440,
		"screenPixelDensity": 1,
		"userInterfaceTheme": "USER_INTERFACE_THEME_LIGHT",
		"memoryTotalKbytes":  "8000000",
	}
	return &session{
		client:      c,
		ctx:         client,
		visitorData: vd,
		clientName:  "WEB",
		clientVer:   clientVer,
	}, nil
}

// parseSWData tách device_info khỏi thân sw.js_data:
// data[0][2] = [[device_info], api_key] (cùng chỉ số youtubei.js dùng). Api key
// không cần gửi đi (InnerTube web nhận cookie/visitor data) nhưng thiếu nó nghĩa
// là layout lạ, không đọc tiếp được.
func parseSWData(data []any) ([]any, error) {
	fail := func() ([]any, error) {
		return nil, &errInnertube{msg: "unexpected sw.js_data layout"}
	}
	if len(data) == 0 {
		return fail()
	}
	first := jarr(data[0])
	if len(first) < 3 {
		return fail()
	}
	ytcfg := jarr(first[2])
	if len(ytcfg) < 2 {
		return fail()
	}
	inner := jarr(ytcfg[0])
	if len(inner) == 0 {
		return fail()
	}
	device := jarr(inner[0])
	if len(device) < 17 {
		return fail()
	}
	apiKey := jstr(ytcfg[1])
	if apiKey == "" {
		return fail()
	}
	return device, nil
}

// devField đọc một phần tử chuỗi của device_info ở chỉ số i; mảng ngắn hơn
// hoặc phần tử không phải chuỗi thì trả "" (không panic), để layout lạ rơi vào
// lỗi đã phân loại thay vì crash tiến trình.
func devField(device []any, i int) string {
	if i < 0 || i >= len(device) {
		return ""
	}
	return jstr(device[i])
}

func utcOffsetMinutes(t time.Time) int {
	_, off := t.Zone()
	return off / 60
}

const visitorAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"

func randomString(n int) string {
	var b strings.Builder
	b.Grow(n)
	for b.Len() < n {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(visitorAlphabet))))
		if err != nil {
			break
		}
		b.WriteByte(visitorAlphabet[i.Int64()])
	}
	return b.String()
}

// decodeVisitorDataID lấy visitor id từ chuỗi visitor data (protobuf
// {id: string field1, timestamp: int32 field5}, base64 URL-safe đã %-encode).
func decodeVisitorDataID(visitorData string) (string, error) {
	unescaped, err := url.QueryUnescape(visitorData)
	if err != nil {
		unescaped = visitorData
	}
	raw, err := base64.URLEncoding.DecodeString(unescaped)
	if err != nil {
		if raw, err = base64.RawURLEncoding.DecodeString(unescaped); err != nil {
			return "", err
		}
	}
	// Đọc tối thiểu: field 1 length-delimited = id.
	for len(raw) > 0 {
		tag := int(raw[0])
		raw = raw[1:]
		switch tag {
		case 0x0a: // field 1, string
			if len(raw) < 1 {
				return "", errors.New("truncated visitor data")
			}
			l := int(raw[0])
			raw = raw[1:]
			if len(raw) < l {
				return "", errors.New("truncated visitor data")
			}
			return string(raw[:l]), nil
		default:
			// bỏ qua field khác: chỉ cần varint cho field 5 (0x28)
			if tag == 0x28 {
				for len(raw) > 0 && raw[0]&0x80 != 0 {
					raw = raw[1:]
				}
				if len(raw) > 0 {
					raw = raw[1:]
				}
				continue
			}
			return "", errors.New("unsupported visitor data field")
		}
	}
	return "", errors.New("empty visitor data")
}

// EncodeVisitorData tạo visitor data protobuf {id, timestamp} — dùng cho test
// và cho caller tự dựng cursor; dạng URL-safe đã %-encode như youtubei.js.
func EncodeVisitorData(id string, timestamp int32) string {
	var out []byte
	out = append(out, 0x0a, byte(len(id)))
	out = append(out, id...)
	out = append(out, 0x28)
	// varint int32
	v := uint64(uint32(timestamp))
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v == 0 {
			out = append(out, b)
			break
		}
		out = append(out, b|0x80)
	}
	enc := base64.URLEncoding.EncodeToString(out)
	return url.QueryEscape(enc)
}

// do chạy request qua bộ điều tốc; phản hồi không-2xx thành *HTTPStatusError
// (đọc tối đa 2 KiB thân để nhận diện trang thử thách).
func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.pacer.Run(req.Context(), func(ctx context.Context) (*http.Response, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return c.http.Do(req.WithContext(ctx))
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, statusSnippetBytes))
		resp.Body.Close()
		return nil, &HTTPStatusError{
			Status:      resp.StatusCode,
			RetryAfter:  resp.Header.Get("Retry-After"),
			BodySnippet: string(snippet),
		}
	}
	return resp, nil
}

// execute POST một endpoint InnerTube với context của session; body =
// {…args sau khi bỏ "parse"/"request" và đổi "token"→"continuation", context}.
func (s *session) execute(ctx context.Context, endpoint string, args jmap) (jmap, error) {
	data := jmap{}
	for k, v := range args {
		switch k {
		case "parse", "request":
			// khoá điều khiển client-side, không gửi lên dây
		case "token":
			data["continuation"] = v
		default:
			data[k] = v
		}
	}
	data["context"] = jmap{
		"client": s.ctx,
		"user":   jmap{"enableSafetyMode": false},
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	path := endpoint
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := fmt.Sprintf("%s/youtubei/%s%s?prettyPrint=false&alt=json", s.client.baseURL, apiVersion, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "*")
	req.Header.Set("X-Goog-Visitor-Id", s.visitorData)
	req.Header.Set("X-Youtube-Client-Version", s.clientVer)
	req.Header.Set("X-Youtube-Client-Name", webClientNameID)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", s.client.baseURL)

	resp, err := s.client.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err // lỗi net/OpError/EOF: isTransportError xếp transient
	}
	var parsed jmap
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, &jsonError{err: err}
	}
	return parsed, nil
}

// jsonError là body JSON không đọc được — xếp transient như SyntaxError bên TS.
type jsonError struct{ err error }

func (e *jsonError) Error() string { return e.err.Error() }

// isTransportError: lỗi lớp vận chuyển (mạng, đọc thân bị cắt, JSON hỏng) →
// transient. Bao phủ cả *net.OpError/net.Error và errno socket vì lỗi đọc thân
// phản hồi không nhất thiết đi kèm bọc *url.Error.
func isTransportError(err error) bool {
	var je *jsonError
	if errors.As(err, &je) {
		return true
	}
	var se *errSession
	if errors.As(err, &se) {
		return true
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	for _, errno := range []error{
		syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.ETIMEDOUT,
		syscall.EPIPE, syscall.EHOSTUNREACH, syscall.ENETUNREACH,
	} {
		if errors.Is(err, errno) {
			return true
		}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	return false
}
