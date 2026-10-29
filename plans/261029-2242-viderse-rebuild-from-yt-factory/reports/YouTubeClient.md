# YouTubeClient — `apps/viderse/internal/youtube`

Port của yt-factory `src/server/youtube/{client,channel,channel-pages,parse,upload-time}.ts`
(commit `2c201e6`) sang Go, stdlib only, không trạng thái toàn cục.

## Đã làm

- **Client InnerTube tự chứa** (`*Client`): bootstrap session qua `/sw.js_data`
  (api key + client version + visitor data), cache session chia sẻ + session
  "resumed" theo visitor data (tối đa 4, đuổi cũ nhất), bootstrap lỗi không được
  cache, bootstrap của caller bị huỷ không làm hỏng caller khác.
- **Trang đầu** = channel home → tab Videos → chip "Latest" (chỉ gọi bộ lọc khi
  nó chưa được chọn; mọi trạng thái không chứng minh được thứ tự Mới nhất là lỗi).
- **Trang kế** = phát lại token continuation nguyên vẹn trên session đã phát hành
  nó (cùng visitor data + client name/version).
- **Cursor/remainder**: JSON thuần, kiểm chặt (bỏ khoá lạ, trần 64 KiB / 500 video /
  512 KiB), visitor data `null` hoá khi session chia sẻ phát hành.
- **Lỗi phân loại** giữ đúng enum của bản TS: `rate_limited`, `transient`,
  `blocked`, `cursor_invalid`, `not_found`, `unexpected_page`.
- **Bộ điều tốc `Pacer`**: một request một lúc, nhịp 1–2 s jitter sau khi phản hồi
  trước xong, 429 đẩy lịch chung theo `Retry-After` (30 s nếu thiếu), không tự
  thử lại; ô lịch vượt ngân sách chờ (deadline context, hoặc `MaxWait` mặc định
  5 s) → `*DeferredError{RetryAt}`.
- **Giờ đăng chính xác**: `/player` → `microformat.playerMicroformatRenderer`
  (`publishDate` trước `uploadDate`; chỉ-ngày là nil; thử thách bot là
  `*UploadTimeBlockedError`).
- Fixtures chép sang `testdata/` (5 tệp); toàn bộ ca test của
  `channel-pages.test.ts`, `parse.test.ts`, `upload-time.test.ts` chuyển thành Go
  table test (nguồn ghi ở đầu tệp test). Parser chạy trên mọi fixture.

## API xuất khẩu (cho `internal/scan`)

```go
// Dựng client — mọi phụ thuộc đi qua Options, không biến toàn cục.
type Options struct {
    HTTPClient *http.Client // mặc định http.DefaultClient; test tiêm transport
    BaseURL    string       // mặc định https://www.youtube.com
    Clock      func() time.Time
    Pacer      *Pacer       // mặc định NewPacer()
}
func NewClient(opts Options) *Client

// Bộ điều tốc dùng chung (một instance cho mỗi Client; chia sẻ giữa mọi request).
type Pacer struct {
    Now                           func() time.Time
    MinGap, MaxGap                time.Duration // cả hai 0 → 1s–2s
    MaxWait                       time.Duration // 0 → 5s
    RateLimitBase                 time.Duration // 0 → 30s
    Rand                          func() float64
}
func NewPacer() *Pacer
func (p *Pacer) Run(ctx context.Context, send func(context.Context) (*http.Response, error)) (*http.Response, error)

// Điểm vào
func (c *Client) ResolveChannelID(ctx context.Context, input string) (string, error)
func (c *Client) ResolveChannelRef(ctx context.Context, channel string) (string, error)
func (c *Client) FirstChannelPage(ctx context.Context, channel string) (*ChannelVideoPage, error)
func (c *Client) NextChannelPage(ctx context.Context, cursor ChannelCursor) (*ChannelVideoPage, error)
func (c *Client) UploadTime(ctx context.Context, videoID string) (*time.Time, error)

// Kiểu dữ liệu
type ScannedVideo struct { ID, Title, ThumbnailURL string; DurationSec, ViewCount *int64;
    PublishedText *string; PublishedAt *time.Time } // JSON camelCase như cursor lưu
type ChannelProfile struct { ID string; Handle, Description, AvatarURL, BannerURL *string;
    Title, URL string; SubscriberCount, VideoCount *int64 }
type ChannelVideoPage struct { Videos []ScannedVideo; NextCursor *ChannelCursor; Profile *ChannelProfile }
type ChannelCursor struct { Version int; ChannelID, Tab, Order string;
    Continuation Continuation; Session SessionContext }
type Continuation struct { Token, Request string }
type SessionContext struct { ClientName, ClientVersion, VisitorData string } // "" → null
func SerializeChannelCursor(ChannelCursor) (string, error)
func ParseChannelCursor(string) (ChannelCursor, error)
func SerializeScannedVideos([]ScannedVideo) (string, error)
func ParseScannedVideos(string) ([]ScannedVideo, error)
const MaxCursorBytes, MaxRemainderVideos, MaxRemainderBytes
func IsChannelID(string) bool
func EncodeVisitorData(id string, timestamp int32) string // cho test/cursor tự dựng

// Lỗi
type ErrorKind string; const KindRateLimited, KindTransient, KindBlocked,
    KindCursorInvalid, KindNotFound, KindUnexpectedPage
type FetchError struct { Kind ErrorKind; Message string; Status int; RetryDelay *time.Duration }
func KindOf(error) (ErrorKind, bool)
type HTTPStatusError struct { Status int; RetryAfter, BodySnippet string }
type ChannelNotFoundError, UploadTimeBlockedError, DeferredError struct{ RetryAt time.Time }
func ParseRetryAfter(string, time.Time) *time.Duration

// Parser thuần
func ParseCompactNumber(string) (int64, bool)
func ParseDuration(string) (int64, bool)
func ParseRelativeTime(string, time.Time) (time.Time, bool)
func Median([]*float64) *float64
func ParseUploadTime(any) (*time.Time, error)
```

Phân loại cho scanner: lỗi ctx (`context.Canceled/DeadlineExceeded`) và
`*DeferredError` đi nguyên vẹn; còn lại `errors.As(err, *FetchError)` để đọc
`Kind`/`Status`/`RetryDelay`. `Message` đã lọc token/visitor/body nên lưu và hiển
thị được.

## Kiểm chứng

| Lệnh | Kết quả |
|---|---|
| `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 go test -p 2 ./apps/viderse/internal/youtube/` | `ok … 1.106s` |
| `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 go test -race -p 1 ./apps/viderse/internal/youtube/` | `ok … 2.46s` |
| `go vet ./apps/viderse/internal/youtube/` | sạch |
| `GOTOOLCHAIN=go1.26.0 gofmt -l` và `GOTOOLCHAIN=go1.27.0 gofmt -l` | không in gì |

Ca test đáng chú ý: thứ tự request (`sw → [resolve] → home → tab → [filter] →
continuation`), cursor round-trip + 12 biến thể hỏng, phân loại 429/5xx/mạng/JSON
cắt/403/bot-challenge/404 (kể cả 429 lúc bootstrap), huỷ giữa request, pacer
xếp hàng + nhịp + phạt 429 + `DeferredError` + huỷ khi chờ, và một ca hồi quy
đường dẫn API (`/youtubei/v1/browse`, không phải `/youtubei/v1browse`).

## Smoke thật (read-only, 5 request)

`VIDERSE_YT_SMOKE=1 go test -p 1 -run TestLiveSmoke -v ./apps/viderse/internal/youtube/`
(kênh `@YouTube`, cổng tắt mặc định):

```
trang đầu: 30 video, profile="YouTube" handle=@YouTube
  Yil32mThG04 "Daddy Yankee reacts to his Watch History" dur=607s views=1300 pub=1 hour ago
  QEM3EHUV5ko "How @haleypham ascended…" dur=677s views=1000000 pub=3 days ago
  yjHvkMeVzK0 "@ldshadowlady created Shadowcraft…" dur=500s views=2500000 pub=10 days ago
trang kế: 30 video, đầu=G7pxAt2tQMs
PASS (4.87s)
```

Request: `sw.js_data`, `navigation/resolve_url`, `browse` (home), `browse` (tab),
`browse` (continuation). Giờ đăng tương đối và số đếm compact đọc đúng trên dữ
liệu thật; continuation trang 2 phân trang được.

## Còn lại / lưu ý

- `Pacer` là trong-process (một instance mỗi `Client`). Bản TS dùng lease
  Postgres để chia nhịp giữa nhiều process; phase `internal/scan` nên sở hữu
  `*Client` duy nhất (hoặc bọc `Pacer` bằng store chung) nếu chạy nhiều process.
- `Pacer.Run` truyền thẳng ctx của caller làm tín hiệu cho send; chưa có tín
  hiệu "mất lease" riêng — khi scan cần huỷ request đang bay vì mất lease, hãy
  huỷ ctx (hành vi tương đương, khác cơ chế).
- `Retry-After` không có giây và ngày đều đọc được; ngày HTTP chỉ nhận các layout
  RFC1123/RFC850/ANSI C.

## Review fixes (Review-leaf.md: H5, M3, M4, M5, M6, M10)

- **H5 — không panic trên session data hỏng**: kiểm `len(text) <= len(")]}'")`
  trước khi cắt (prefix thiếu dấu xuống dòng từng panic ở `text[5:]`); trường
  đọc từ `device_info` giờ đi qua `devField` trả `""` khi mảng ngắn — layout
  xấu thành lỗi đã phân loại (transient/unexpected_page) thay vì crash.
- **M3 — waiter bootstrap tôn trọng ctx riêng**: thay `sync.Cond` (không huỷ
  được) bằng `sessionFlight{done, s, err}`; waiter `select` trên `fl.done` và
  `ctx.Done()`, nên waiter có deadline ngắn rời sớm trong khi owner hoàn tất
  và cache. Client mặc định giờ có `Timeout: 60s` (không còn treo vô hạn).
- **M4 — MaxWait tích luỹ**: `Run` tính một `cutoff` duy nhất lúc bắt đầu
  (min(deadline ctx, now+MaxWait)); mọi nhịp xếp hàng cộng dồn vào nó thay vì
  xin ngân sách mới mỗi 250ms — trước đây chờ vô hạn khi ô lịch bận.
- **M5 — lỗi mạng khi đọc thân → transient**: `isTransportError` nhận thêm
  `net.Error` và errno socket (ECONNRESET/REFUSED/ETIMEDOUT/EPIPE/EHOSTUNREACH/
  ENETUNREACH); lỗi không rõ loại luôn là `unexpected_page` — chỉ `errInnertube`
  (dữ liệu continuation không dùng được) mới là `cursor_invalid`, khớp upstream.
- **M6 — parse dữ liệu lưu trữ chặt**: `ChannelCursor`, `Continuation`,
  `SessionContext` bắt buộc đúng bộ khoá (visitorData phải hiện diện: null hoặc
  chuỗi); `strictUnmarshal` đọc hết input (decode thứ hai phải `io.EOF`);
  `ParseScannedVideos` đòi mảng thật (không null/object) và mỗi phần tử đủ 7 khoá.
- **M10 — jitter mặc định**: `NewPacer()` và pacer literal đều jitter qua
  `math/rand/v2` khi `Rand` nil (nhịp 1–2s thật như upstream `Math.random`);
  test vẫn tiêm `Rand` cho nhịp cố định.

Ca test hồi quy mới (`review_regressions_test.go`): prefix/dệt JSON/device array
ngắn không panic; thiếu trường đuôi device_info đọc an toàn; waiter huỷ/deadline
không kẹt và không làm hỏng flight của owner; MaxWait cộng dồn thật; reset khi
đọc thân continuation → transient; cursor thiếu `visitorData`, JSON đuôi rác,
`null`/object/thiếu-khoá remainder đều bị từ chối; `NewPacer`/client mặc định có
jitter thật + timeout HTTP.

Kiểm chứng sau sửa: `go test -count=1 -p 2` ok (1.34s); `-race -p 1` ok (2.92s);
`go vet` sạch; `gofmt -l` không in gì trên go1.26.0 và go1.27.0. Không gọi mạng thật.
