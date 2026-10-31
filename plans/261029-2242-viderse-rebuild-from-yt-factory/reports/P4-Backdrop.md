# P4 — Video nền (backend)

Mảng "Video nền": kênh nền Twitch/Bilibili kèm danh mục đã đọc, video lẻ (đoạn dùng tải sẵn), bộ chọn nền dùng chung với editor, yêu cầu hỗ trợ nền tảng, và bộ lập lịch đọc dần Bilibili. Chỉ backend; UI đợt sau đọc mục **API cho web**.

## 1. Đã làm

### Kiểu dùng chung
- `internal/domain/backdrop.go` — `BackdropChannel`, `BackdropChannelVideo`, `BackdropVideo`/`BackdropVideoState`, `BackdropSegment` (To−From ≤ 600 s), `ParsedSource` (+ kind `channel|video|unsupported|invalid`), `RecipeBackdropChannel`, lý do không dùng được (`expired|live|unavailable|login_required`, `SelfClearingBackdropReason`).
- `internal/domain/task.go` — thêm `TaskBackdropDownload = "backdrop_download"`; `TaskBackdropScan` (đã có) dùng cho lần đọc danh mục đầu tiên.

### `internal/backdrop` (một package nghiệp vụ)
| Tệp | Nội dung |
|---|---|
| `links.go` | Nhận link Twitch (kênh, `/videos/<id>` VOD, `/clip/<slug>`, `clips.twitch.tv/<slug>`), Bilibili (`space.bilibili.com/<mid>`, `/video/BV…|av…`), `b23.tv` **một** lần redirect (10 s, doer tiêm). Host chưa hỗ trợ → `unsupported` + nhãn đọc được (Douyin, TikTok, Kuaishou, Xiaohongshu, YouTube, Instagram, Facebook, X, Vimeo, Youku, iQIYI, Niconico, Dailymotion, Rumble, Odysee…). Link sai → `invalid` + câu phục hồi. Không gọi mạng nào khác. |
| `listing.go` | Twitch qua yt-dlp `--flat-playlist -J`; gộp 3 pool `vod`/`highlight`/`clip`, khử trùng theo video id, `VodsComplete` khi pool VOD đọc trọn (để hết hạn VOD vắng mặt), clip không bao giờ hết hạn. |
| `bilibili_wbi.go` | WBI: bảng mixin, ký `w_rid`/`wts`, lấy `img_key`/`sub_key` từ `nav`, cache 24 giờ. |
| `bilibili_device.go` | `device.json` dưới thư mục cấu hình app (0600, ghi nguyên tử) giữ `buvid3/buvid4` + key; cookie tài khoản (SESSDATA) lấy từ `accounts.CookieHeader` khi đã đăng nhập. |
| `bilibili_api.go` | Client `search/type?mid=&pn=&ps=30&order=pubdate`: kiểm `code`, map `-101` (chưa đăng nhập), `-352` (risk control, xoá key cache), HTTP 412 (`Retry-After`), giới hạn 8 MiB, Referer `https://space.bilibili.com`. Mọi lời gọi qua `Deps.Request` (Pacer Bilibili: hạn mức + spacing + giờ chờ). |
| `bilibili_fetch.go` | Parse JSON trang (bvid/av, title, duration `m:ss`/`h:mm:ss`, thumbnail `//` → `https:`, pubdate, tổng số, lỗi riêng của trang). |
| `service.go` | Thêm/đọc/làm mới/xoá kênh, ingest + upsert (coalesce `published_at` khi lần sau thiếu, xoá `unavailable` khi video trở lại, hết hạn VOD khi danh sách trọn), `usedBy`/`chosenBy` đọc `templates.recipe->'backdrop'` bằng SQL chỉ-đọc, đếm usable theo loại, keyset trang danh mục, yêu cầu nền tảng (gộp theo host), `Usage` cho `/api/platforms`. |
| `videos.go` | Video lẻ: probe `yt-dlp -J` (từ chối live), chọn đoạn (mặc định trọn video nhưng ≤ 600 s), tải bằng `--download-sections *from-to` + `-S res:<cao mục tiêu>`, tệp tạm rồi rename vào `library/backdrops/<platform>-<id>.mp4`, tiến độ ≤ 4 lần/giây, trạng thái `downloading|ready|failed`, thêm lại cùng id trả dòng cũ, PATCH đoạn tải lại, xoá 409 khi `fixedVideoIds` còn ghim, `VideoFile` trả đường dẫn tuyệt đối. |
| `pick.go` | Bản thuần của `pickBackdropSources`: mulberry32, round-robin kênh, lượt 1 chỉ video chưa dùng, lượt 2 dùng lại, loại < 10 s (`MinBackdropClipSec`), ghim/kinds/skip, `CutRange` (cửa sổ clip ngẫu nhiên trong độ dài video), trần `MaxBackdropPicks = 40`. |
| `mix.go` | `Mix(templateID, lengthSec)` dùng đúng bộ chia trên; `fixedVideoIds` (chia tốc độ), `clipSec`, `clipCount` ghi đè, `Capped`, `LoopSec`, `perChannel{clips,freshVideos,repeats}`. |
| `scheduler.go` | `RunScheduler` tính mốc sớm nhất (trang kế + `platform_state.next_trickle_at` + hạn mức + giờ chờ) rồi **ngủ tới đúng mốc**; `Wake()` đánh thức sớm; `Trickle` đọc một trang của kênh chờ lâu nhất; lỗi trang không nổi lên scheduler (ghi vào kênh, hẹn lượt sau 3–5 phút). |
| `errors.go`, `cursor.go`, `http.go`, `templates.go`, `store.go` | Mã lỗi ổn định, con trỏ opaque (base64 JSON, gắn channel + kinds), mặt `HTTPDoer`, decode recipe, interface `Store` phía dùng. |

### Lưu trữ
- `internal/persistence/postgres/queries/backdrop.sql` + `postgres/backdrop.go` (`var _ backdrop.Store = (*BackdropStore)(nil)`).
- `queries/platforms.sql` + `postgres/platforms.go`: `request_history` (jsonb), `next_trickle_at`, `ytdlp_owner/ytdlp_lease_until`, `ReserveWeighted` (giờ chờ + hạn mức + ghi lịch sử trong **một** transaction), backoff 15→30→60→120 phút (trần 120; `Retry-After` chỉ kéo dài), lease yt-dlp chéo process, lease quét 5 phút.
- `internal/platforms`: `BudgetOf`/`ReserveWeighted`/`StateStore` mở rộng, `Pacer.RequestCost`, `Pacer.RunYtDlp`, `Pacer.WithScannerLease`, spacing Bilibili API 10–20 s (jitter), yt-dlp Bilibili 5–10 s, `UsageCounter` + `ComposeUsage` + `SetUsage`.
- Migration `00008` dùng nguyên trạng (không sửa khối nào).

### HTTP (`internal/web/backdrop.go`)
`BackdropService` interface nhỏ, handler mỏng, một hàm map lỗi `WriteBackdropError` (hình `{error, code, recovery?, retryAt?}`), wire kiểu riêng và converter export cho CLI. Route mix do Thư viện sở hữu tiền tố và chuyển tiếp qua `web.TemplateBackdropMix` (adapter ở `cmd/viderse/backdrop_wiring.go`).

### Ghép nối / CLI
- `cmd/viderse/backdrop_wiring.go`: dựng store/service, mount route trước `mountPlatforms` (để `POST /api/platforms/requests` thắng tiền tố), `Bind` sau khi mảng nền tảng dựng (pacer, lease, hạn mức, cookie), chạy scheduler trong vòng đời serve + `Close` join, đăng ký `hub.OnPublish` type `platforms` → `Wake()`, và `platformSvc.SetUsage(ComposeUsage(newRivalsUsage, backdrop.Usage))`.
- `cmd/viderse/serve.go`: gọi `mountBackdrop` + `Bind` (+ `newBackdropMix` cho Thư viện).
- `cmd/viderse/main.go`: case `backdrop` (usage dòng 20–22).
- `cmd/viderse/backdrop.go`: CLI đầy đủ (xem §3).

## 2. API cho web (đầy đủ)

Tất cả route cần đăng nhập (cookie phiên như các mảng khác). Lỗi: JSON `{"error": "<câu tiếng Việt>", "code": "<mã>", "recovery"?: "<gợi ý>", "retryAt"?: "RFC3339"}`.
Mã lỗi dùng chung: `invalid_input` (400), `invalid_cursor` (400), `not_found` (404), `conflict`/`in_use`/`unavailable` (409), `login_required` (401), `rate_limited` (429), `unsupported_platform` (422), `failed` (500, không lộ chi tiết).
Bảng mã ↔ HTTP: `invalid_input|invalid_cursor`→400, `not_found`→404, `conflict|in_use|unavailable`→409, `login_required`→401, `rate_limited`→429, `unsupported_platform`→422; lỗi nền tảng đã dịch (`paused`/`budget`/`busy`, yt-dlp có mã) → 409/429/502 tuỳ mã.

### 2.1 `POST /api/sources/parse`
Body: `{"input": "<link hoặc host>"}`. Trả **200** với một trong bốn hình (dùng `kind` để phân nhánh; `invalid` vẫn 200 để UI hiện gợi ý):
```jsonc
// channel
{"kind":"channel","platform":"twitch"|"bilibili","id":"twitch:<login>|bilibili:<mid>","handle":"<login|mid>","url":"https://..."}
// video (title/durationSec chỉ có khi biết mà không gọi mạng; hiện không có)
{"kind":"video","platform":"twitch"|"bilibili","id":"<platform>:<videoId>","url":"https://...","title"?:string,"durationSec"?:number}
// unsupported
{"kind":"unsupported","host":"douyin.com","label":"Douyin"}
// invalid
{"kind":"invalid","message":"Dán link kênh Twitch hoặc trang cá nhân Bilibili","recovery":"Ví dụ: https://www.twitch.tv/<tên kênh> hoặc https://space.bilibili.com/<số>"}
```
`b23.tv` được theo **một** redirect (10 s) rồi phân loại như trên. Không chạy yt-dlp trong route này.

### 2.2 `GET /api/backdrop/channels`
200: mảng `ChannelWire`:
```jsonc
{
  "id": "twitch:nightdrivefm",            // "<platform>:<handle>"
  "platform": "twitch"|"bilibili",
  "handle": "nightdrivefm",
  "url": "https://www.twitch.tv/nightdrivefm",
  "title": string|null,                    // chưa đọc được → null (Twitch tạm dùng handle)
  "avatarUrl": string|null,                // Bilibili lấy từ API nếu có
  "addedAt": "RFC3339",
  "listedAt": "RFC3339"|null,              // null = đang đọc lần đầu
  "listError": {"code":string,"message":string}|null,  // lần đọc hỏng gần nhất
  "scanNextPage": number|null,             // Bilibili: trang kế; null = đã đọc hết
  "scanTotal": number,                     // tổng video nền tảng báo (0 = chưa biết)
  "scanStopKnown": boolean,                // trang kế đã toàn video đã biết
  "counts": {"vod":number,"highlight":number,"clip":number,"video":number}, // chỉ video dùng được
  "pausedUntil": "RFC3339"|null,
  "pauseReason": string|null,
  "usedBy": [{"templateId":"uuid","name":string,"mode":"random"|"pick","kinds":["vod",…],"chosenCount":number}],
  "bilibili"?: {                            // chỉ kênh Bilibili
    "budget": {"short":{"used":n,"limit":20,"availableAt":"RFC3339"},
               "day":{"used":n,"limit":200,"availableAt":"RFC3339"}},
    "pausedUntil": "RFC3339"|null, "pauseReason": string|null,
    "nextPageAt": "RFC3339"|null,           // mốc được đọc trang kế
    "pending": [{"channelId":"bilibili:<mid>","nextPage":number|null,"maxPages":number}]
  }
}
```
`usedBy`: template có `recipe.backdrop.channels[]` chứa kênh; `mode = "pick"` khi `pinned` không rỗng, `chosenCount = |pinned|`; `kinds` là bộ lọc loại của dòng đó.

### 2.3 `POST /api/backdrop/channels`
Body: `{"input": "<link>"}` hoặc `{"url": "<link>"}`.
- 201 khi tạo mới: dòng kênh được ghi trước, danh mục đọc ở việc nền (`backdrop_scan`); response là `ChannelWire` ngay (thường `listedAt=null`, `counts=0`).
- 200 khi kênh đã có: làm mới danh mục rồi trả view.
- 400 `invalid_input` (link sai), 422 `unsupported_platform` (host chưa hỗ trợ), 401 `login_required` (Bilibili cần đăng nhập lúc đọc).

### 2.4 `GET /api/backdrop/channels/{id}`
200: `ChannelWire` như 2.2 (một kênh).

### 2.5 `DELETE /api/backdrop/channels/{id}`
204. 404 `not_found`. **409 `in_use`** khi còn template dùng, câu: `Kênh nền đang dùng trong N template: <tên, …>`, `recovery`: `Bỏ kênh khỏi các template này trước`.

### 2.6 `POST /api/backdrop/channels/{id}/refresh`
Đọc lại ngay (force), 200 `ChannelWire`. Bilibili đọc một trang. Nếu lời gọi hỏng, trạng thái lỗi được ghi vào kênh rồi lỗi mang mã được trả về (401 `login_required`, 409 `unavailable`, 429 `rate_limited`); danh mục đã đệm và con trỏ còn nguyên.

### 2.7 `GET /api/backdrop/channels/{id}/videos`
Query: `kind` (lặp hoặc phẩy: `vod,clip`; mặc định mọi loại của nền tảng), `sort=newest` (chỉ hỗ trợ `newest`; khác → 400 `invalid_input`), `cursor` (opaque từ `nextCursor`), `limit` (mặc định 24, trần 500).
200:
```jsonc
{
  "items": [{
    "videoId": "v123", "kind": "vod"|"highlight"|"clip"|"video",
    "title": string, "durationSec": number|null, "publishedAt": "RFC3339"|null,
    "thumbnailUrl": string|null, "url": string,
    "unavailable"?: {"reason":"expired"|"live"|"unavailable"|"login_required","at":"RFC3339"},
    "chosenBy": [{"templateId":"uuid","name":string}]   // pinned chứa video này
  }],
  "counts": {"vod":n,"clip":n,…},   // usable theo loại (không tính unavailable)
  "total": number,                 // tổng theo bộ lọc kind (gồm unavailable)
  "unavailable": number,
  "nextCursor": string             // vắng khi hết
}
```
Thứ tự: `published_at DESC NULLS LAST, video_id ASC`. Con trỏ gắn kênh + bộ lọc kinds; lệch → 400 `invalid_cursor`. `kind` không thuộc nền tảng → 400 `invalid_input`.

### 2.8 `GET /api/backdrop/videos`
200: mảng `VideoWire` (mọi video lẻ), mới thêm trước:
```jsonc
{
  "id": "twitch:Abc", "platform":"twitch"|"bilibili", "url": string, "title": string,
  "durationSec": number|null,
  "segment": {"fromSec": number, "toSec": number},
  "file"?: {"path":"library/backdrops/twitch-Abc.mp4","height":480|1080,"bytes":number},
  "state": "downloading"|"ready"|"failed",
  "progress"?: number,                 // 0..1 khi đang tải
  "errorDetail"?: {"code":string,"message":string},
  "usedBy": [{"templateId":"uuid","name":string}],   // recipe.backdrop.fixedVideoIds
  "addedAt": "RFC3339", "updatedAt": "RFC3339"
}
```

### 2.9 `POST /api/backdrop/videos`
Body: `{"url": "<link video>", "segment"?: {"fromSec":n,"toSec":n}}`.
201 `VideoWire`. Không có `segment` → trọn video nhưng tối đa 600 s. Tải chạy nền (`backdrop_download`), response thường `state:"downloading"`; cùng video id thêm lại → trả dòng cũ (không dò lại). Lỗi: 400 `invalid_input` (link kênh thay vì video, đoạn rỗng/âm/> 600 s/vượt độ dài video, thiếu `fromSec`/`toSec`), 409 `unavailable` (video đang phát trực tiếp), 401 `login_required`, 429 `rate_limited`.

### 2.10 `PATCH /api/backdrop/videos/{id}`
Body: `{"segment":{"fromSec":n,"toSec":n}}` (bắt buộc đủ hai số). 200 `VideoWire` (state về `downloading`, tải lại đoạn mới). Cùng ràng buộc đoạn như 2.9. 404 `not_found`.

### 2.11 `DELETE /api/backdrop/videos/{id}`
204. 409 `in_use` khi còn template ghim trong `fixedVideoIds` (`Video nền đang dùng trong N template: …`). Xoá kèm tệp media.

### 2.12 `GET /api/backdrop/videos/{id}/file`
200 nhị phân, `Content-Type: video/mp4|video/webm|video/x-matroska`, hỗ trợ Range qua `http.ServeContent`. 404 khi chưa có/chưa tải xong (`conflict` → 409 `Video nền chưa tải xong`).

### 2.13 `GET /api/templates/{id}/backdrop/mix?lengthSec=`
Route do Thư viện sở hữu tiền tố, chuyển tiếp sang mảng này (`web.TemplateBackdropMix`). `lengthSec` tuỳ chọn: không truyền = 0 (chỉ báo phần đã biết: `fixedSec`, `freshVideos`); số âm hoặc không phải số → 400 `invalid_input` (`Độ dài video không hợp lệ`).
200:
```jsonc
{
  "fixedSec": number,       // tổng giây phát của fixedVideoIds (đã chia tốc độ)
  "perChannel": [{"channelId":"twitch:slowroads","clips":36,"freshVideos":50,"repeats":false}],
  "loopSec": number,        // còn thiếu bao nhiêu giây phải lặp
  "need": number,           // số clip cần (≤ 40)
  "capped": boolean         // true khi nhu cầu vượt trần 40 clip
}
```
Ví dụ chấp nhận (V23): 2 video cố định 85 s + slowroads ngẫu nhiên + nightdrivefm 4 video ghim, clip 60 s, video 42 phút → `fixedSec 170`, `need 40`, slowroads 36 clip, nightdrivefm 4 clip, `loopSec 0`, `capped false`.
Lỗi: 404 `not_found` (`Template không tồn tại`, hoặc `Template chưa gắn kênh nền nào`).

### 2.14 `POST /api/platforms/requests`
Body: `{"host":"douyin.com"}` (nhận cả URL). 200:
```jsonc
{"host":"douyin.com","label":"Douyin","count":3,"firstAt":"RFC3339","lastAt":"RFC3339"}
```
Bấm lại **không tạo dòng mới**: `count` tăng, `lastAt` cập nhật, `firstAt` giữ nguyên (UI có thể hiện "đã ghi nhận N lần"). Host không thuộc danh sách chưa hỗ trợ hoặc đã hỗ trợ → 422 `unsupported_platform` (`Nền tảng này không có trong danh sách chưa hỗ trợ`).

### 2.15 `GET /api/platforms` (mảng nền tảng, dùng số của mảng này)
`backdrop.Service` hiện thực `platforms.UsageCounter`: `channels` = số kênh nền, `videos` = số video lẻ theo nền tảng, cộng với số đối thủ của mảng rivals qua `ComposeUsage`.

### SSE
Kênh SSE chung (`platform/events`), sự kiện `type: "backdrop"`, `data: {"rev": <số tăng>, "ref": "<channelId|videoId>"}`; gộp tối đa 4 frame/giây (`web.Notifier`, cửa sổ 250 ms). Bắn khi:
- thêm/gỡ kênh;
- đọc danh mục xong hoặc lỗi đọc (Twitch/Bilibili, kể cả trickle nền);
- thêm video lẻ, mỗi lần ghi tiến độ (đã bị chặn ≤ 4 lần/giây), khi xong (ready) hoặc thất bại, khi đổi đoạn, khi gỡ.
UI nên refetch danh sách/kênh tương ứng khi nhận `ref` (rỗng = nhiều đối tượng trong cùng frame).

## 3. CLI `viderse backdrop …`

Mọi lệnh in **một dòng JSON** ở stdout; lỗi in JSON `{"error":…,"code":…,"recovery"?:…}` ra stderr, exit 1. Không cần serve; dùng `VIDERSE_DATABASE_URL` như serve; việc nền chạy đồng bộ (lệnh trả khi đọc/tải xong).

| Lệnh | Output |
|---|---|
| `backdrop list` | `{"channels":[ChannelWire…]}` |
| `backdrop add <link>` | `{"created":bool,"channel":ChannelWire}` |
| `backdrop show <id>` | `ChannelWire` |
| `backdrop videos <id> [--kind k] [--limit n] [--cursor c]` | `VideoPageWire` (như 2.7) |
| `backdrop refresh <id>` | `ChannelWire` |
| `backdrop remove <id>` | `{"removed":"<id>"}` |
| `backdrop video add <link> [--from s] [--to s]` | `VideoWire` |
| `backdrop video list` | `{"videos":[VideoWire…]}` |
| `backdrop video remove <id>` | `{"removed":"<id>"}` |
| `backdrop parse <link>` | `ParsedSourceWire` (như 2.1) |

## 4. Test

Chạy không mạng, fixture tiêm (`RunYtDlp`, `BilibiliPage`, `HTTPDoer`). Không test nào gọi mạng.

Port từ yt-factory (ghi nguồn trong comment đầu tệp):
- `links_test.go` — `catalogue.test.ts`/`links.test.ts`: `TestParseChannelLink`, `TestParseChannelLinkRecoveryCopy`, `TestParseChannelLinkB23SingleRedirect`, `TestChannelOfIDRoundTrip`, `TestParseAssetLink`, `TestParseSourceKinds`.
- `listing_test.go` — `listing.test.ts`: `TestListChannelVideosPoolsAndDedupe`, `TestListChannelVideosVodsTruncated`, `TestListChannelVideosOnePoolFailureFailsListing`, `TestListChannelVideosBadJSON`.
- `pick_test.go` — `pick.test.ts` + `background-clip.test.ts` (CutRange): `TestMulberry32Deterministic`, `TestCutRange`, `TestPickRoundRobinAcrossChannels`, `TestPickFreshBeforeReuse`, `TestPickStopsWhenOnlyShortVideosRemain`, `TestPickRespectsPinsKindsAndSkips`, `TestPickHonorsExcludedAndUsed`.
- `bilibili_wbi_test.go` — `bilibili-api.test.ts`: `TestSignWBIWithKnownVector` (đúng vector 1702204169), `TestParseBilibiliPage`.
- `bilibili_api_test.go` — client: `TestBilibiliClientUsesCachedKeysAndDeviceCookies` (2 lời gọi, không nav lại; Referer; cookie SESSDATA+buvid; file không chứa SESSDATA), `TestBilibiliClientBootstrapsAnonymousNavAndPersistsKeys`, `TestBilibiliClientBlockedPageCarriesRetryAfterWithoutRetry` (412, Retry-After 30 phút, không lộ body), `TestBilibiliClientRiskControlInvalidatesKeys` (-352), `TestBilibiliErrorCodesMapToSharedContract`, `TestParseBilibiliPageRejectsBadUploadID`, `TestReadAllLimitedRejectsOversize`.

Nghiệp vụ (`service_test.go`, store trong bộ nhớ): `TestAddTwitchChannelIngestsAndExpiresMissingVods` (ingest, hết hạn VOD vắng mặt, clip không hết hạn, video trở lại dùng được), `TestChannelVideosKeysetPagination` (7/page × 31 video, không lặp, NULL cuối, con trỏ lệch bộ lọc bị từ chối, kind sai nền tảng), `TestChannelViewsReportUsedByAndChosenBy` + 409 in_use kèm tên template, `TestRemoveChannelWhenUnused`, `TestAddVideoProbesDownloadsAndThrottlesProgress` (probe + tải, tiến độ 2 lần ghi với đồng hồ đứng yên = ≤ 4/s, file 480, cùng id trả dòng cũ), `TestAddVideoSegmentValidation`, `TestUpdateVideoSegmentRedownloadsAndDeleteGuard`, `TestDownloadFailureMarksFailed`, `TestMixAcceptanceExample` (ví dụ V23: 170 s + 36 + 4 clip, need 40), `TestMixCappedAndMissingTemplate`, `TestTrickleAdvancesPagesThenStops`, `TestTrickleStopsOnRiskControlAndSchedulesNext`, `TestNextDueHonorsTricklePauseAndBudget`, `TestRecordPlatformRequestIsIdempotentPerHost`, `TestUsageCountsChannelsAndVideos`, `TestAddBilibiliChannelQueuesWhenBlocked`, `TestNotifyFiresOnCatalogueAndDownload`, `TestRunSchedulerSleepsUntilDueThenWakes`.

Postgres thật (`TEST_DATABASE_URL`): `backdrop_pg_test.go` — `TestBackdropCatalogueUpsertKeepsPublishedAndExpiresVods`, `TestBackdropChannelVideosKeysetNullsLast`, `TestBackdropVideoStateUpdates`, `TestBackdropPlatformRequestUpsert`; `platforms_pg_test.go` — `TestPlatformReserveWeightedBudgetAndPause` (20 yêu cầu, lần 21 hết hạn mức 10 phút, chặn 15→30→60→120→120, hết chặn về 15), `TestPlatformYtDlpLeaseIsExclusive` và các test cũ.

CLI/gói lỗi (`cmd/viderse/machine_wiring_test.go`): `TestPlatformRequestsExtra`.

Web (`internal/web/backdrop_test.go`): `TestParseSourceWire`, `TestBackdropChannelListWire`, `TestBackdropChannelAddAndRefresh`, `TestBackdropChannelVideosWire`, `TestBackdropVideoAddPatchDeleteAndFile` (kể cả `Accept-Ranges`), `TestBackdropInUseConflictCarriesRecovery`, `TestPlatformRequestWire`, `TestBackdropVideoListWire`.

Đã chạy (sau review): `go test -race -count=1 ./internal/backdrop ./internal/platforms` (pass), `go test -race -count=1 ./internal/web` (pass, 136 s), `go test -count=1 ./cmd/viderse ./internal/web` (pass), `TEST_DATABASE_URL=… go test -count=1 ./internal/persistence/postgres/` (pass **không overlay**, 56 s — mảng Kênh đã xong test), `TEST_DATABASE_URL=… go test -race -run 'TestBackdrop|TestPlatform' ./internal/persistence/postgres/ ./cmd/viderse/` (pass), `go vet` các package của mảng + `./cmd/viderse` (sạch), `gofmt -l internal cmd` (sạch), `go test ./archtest/` (pass), `go tool sqlc generate` + so sha256 `internal/persistence/postgres/gen/*.go` (không diff). Test review thêm: b23 không đọc body/không follow, publish EXDEV hai nhánh, refresh đọc trang mới, mã lỗi 401/429+retryAt, segment thiếu đầu → 400, claim/generation/dọn lease + khoá xoá–lưu trên Postgres, adapter cookie/nil, listener NOTIFY. Bản đo nhịp rảnh: `TestRunSchedulerSleepsUntilDueThenWakes` (mục 5). Sau QA: thêm fixture VOD Twitch không login + hai test httptest cho b23; chạy lại `go test -race -count=1 ./internal/backdrop ./internal/web ./cmd/viderse` (pass, 140 s), `gofmt`/`go vet` sạch, `go test ./archtest/` (pass).

## 5. Nhịp rảnh của bộ lập lịch (đo)

- `RunScheduler` không hỏi vòng: mỗi vòng tính **một** mốc `due = max(now, next_trickle_at, budget.availableAt, pausedUntil)` từ `platform_state` + các kênh Bilibili còn `scan_next_page`, rồi `select` trên timer tới mốc, `s.wake`, và `ctx.Done()`.
- Không kênh nào còn trang để đọc → chờ `wake` vô hạn (0 vòng/giây, không timer).
- Đo bằng `TestRunSchedulerSleepsUntilDueThenWakes` (đồng hồ thật): chưa tới mốc 40 ms → 0 trang đọc; tới mốc (~120 ms) → đúng 1 trang; 120 ms sau đó → vẫn 1 trang; hạ `next_trickle_at` về `now` + `Wake()` → đọc trang kế trong < 5 ms kiểm tra (vòng chờ 5 ms trong test). Tức là 0 lần đọc khi rảnh và 1 lần đọc cho mỗi mốc.
- Thức dậy bởi: timer tới mốc kế; `Service.Wake()` từ: thêm kênh, làm mới, xong một trang (tự đặt mốc kế), và hub sự kiện `"platforms"` (đăng nhập mới/gỡ chặn) trong `backdrop_wiring.go`.
- Khi bị chặn: `Trickle` trả `served=false`, ghi lỗi vào kênh, đặt `next_trickle_at = now + 3–5 phút`; hạn mức đầy thì `budget.AvailableAt` đẩy mốc; `pausedUntil` đẩy mốc.

## 6. Lệch so với design / yt-factory

1. **Backoff chặn**: 15 → 30 → 60 → 120 phút (trần 120, `Retry-After` chỉ kéo dài) theo verse/W13, không dùng 30/60/180/360 của yt-factory. Test `TestBackoffEscalatesAndResets` + pg test đã viết lại theo dãy này.
2. **Spacing**: API Bilibili 10–20 s (jitter 10 s) và yt-dlp Bilibili 5–10 s — yt-factory dùng 10 s + jitter 10 s cho cả hai; tách theo loại lời gọi để trickle chậm mà tải clip không quá chậm. Hạn mức giữ 20/10 phút + 200/24 giờ.
3. **Chiều cao tải** lấy từ registry: Twitch **480**, Bilibili **1080**; yt-factory cứng 720. Ghi để controller xác nhận (nếu muốn 720/1080 khác chỉ sửa `Descriptor.TargetHeight`).
4. **Thêm kênh là bất đồng bộ**: ghi dòng trước, đọc danh mục trong việc nền `backdrop_scan`, HTTP trả ngay (thẻ kênh hiện `listedAt=null`/`listError`). yt-factory đọc đồng bộ trong request thêm. Lỗi link vẫn trả ngay ở bước parse.
5. **Parse không gọi yt-dlp** (yt-factory dò `-J` ngay khi dán link): chỉ `b23.tv` redirect một lần; `title`/`durationSec` của `kind:"video"` vì vậy thường vắng — UI hiện sau khi thêm (probe chạy ở bước thêm video lẻ).
6. **`platform_requests`**: giữ một dòng/host, `count`/`first_at`/`last_at`; bấm lại không tạo dòng. yt-factory không có bảng này.
7. **Delete kênh/video**: 409 `in_use` kèm **tên** template đang dùng (đọc recipe bằng SQL chỉ-đọc), không chỉ "đang dùng".
8. **Mix**: `need` lấy từ `lengthSec`/`clipSec` (hoặc `clipCount` ghi đè), trần 40; `fixedSec` chia tốc độ chung `appearance.speed`; `loopSec` = max(0, target − picks×clip phát).
9. **Kênh Bilibili**: dùng `search/type` theo `mid` (yt-factory), không dùng API space khác; cache WBI 24 h; `device.json` 0600.

## 7. Việc còn lại / lưu ý cho controller

- **Gói Báo lỗi**: đã nối. `cmd/viderse/machine_wiring.go` có `platformRequestsExtra` đọc `backdrop.Service.PlatformRequests` → `platform-requests.json` (`{"requests":[{host,label,count,firstAt,lastAt}]}`), truyền từ `serve.go` (`backdropRuntime.svc.PlatformRequests`); test `TestPlatformRequestsExtra` (cmd) kiểm hình JSON, trường hợp không có bộ đọc (bỏ tệp), lỗi đọc giữ nguyên, và gói zip thật chứa tệp. Không có route GET cho danh sách này — chỉ POST ghi + tệp phụ.
- **Bước clone/dựng video** (manifest, tải clip theo job) không làm theo phân công; backend chỉ có Video lẻ và ước lượng mix.
- **`go build ./cmd/viderse`**, `go vet ./cmd/viderse`, `go test ./cmd/viderse` sạch; `go test ./internal/persistence/postgres` **không overlay** nay xanh (mảng Kênh đã hoàn tất `ChannelStore.SetTemplates`), `go test -race ./internal/web` xanh.
- **Đo p95 GET**: chưa đo bằng tải thật (không có harness tải trong đợt). Các GET chỉ một truy vấn chính + ghép trong bộ nhớ (`usedBy`/`chosenBy` đọc một lần `templates`, `counts` một truy vấn nhóm), không duyệt cây, không chạy tiến trình, không gọi mạng.
- **Kiểm tra mạng thật**: không chạy (chỉ fixture). Nếu controller muốn smoke: 1 link Twitch (yt-dlp) và 1 link Bilibili đã đăng nhập là đủ; hạn mức Bilibili 20/10 phút còn nguyên.

## 8. Sửa theo review (`Review-p4-backdrop.md`)

Đã đọc review và xử lý **toàn bộ** finding; không có mục nào bị từ chối. Tóm tắt theo mã:

| # | Việc đã sửa |
|---|---|
| 1 (major) | `POST /api/platforms/requests` chỉ nhận host **chưa hỗ trợ**; host đã hỗ trợ hoặc lạ trả `422 unsupported_platform`. Service vẫn idempotent (một dòng/host, `count`/`firstAt`/`lastAt`), CLI in `code`; test web kỳ vọng 422. |
| 1 (major, atomic delete) | `RemoveChannel`/`RemoveVideo` chạy trong `WithTx`: `pg_advisory_xact_lock(hashtextextended(key,0))` theo kênh/video → **kiểm lại** recipe + `fixedVideoIds` trong cùng transaction → mới xoá. `LibraryStore.UpdateTemplate` (mảng Library, đã xác nhận qua IRC) lấy đúng khoá đó rồi kiểm tồn tại trong transaction trước khi ghi, nên khe "lưu tham chiếu sau khi xoá" bị đóng. Khoá: `backdrop.ChannelLockKey` = `backdrop_channel:<id>`, `VideoLockKey` = `backdrop_video:<id>`; pg test `TestBackdropDeleteLockSerializesTemplateSave` chứng minh lượt "lưu" chờ và thấy kênh đã mất. |
| 2 | `resolveB23` dùng client `CheckRedirect: ErrUseLastResponse`, `Timeout: 10s`, đóng body **không đọc**, deadline ≤10 s; chỉ đúng host `b23.tv` (không nhận `www.b23.tv`). Test `TestResolveB23RedirectDoesNotFollowOrReadBody`. |
| 3 | (Lưu template) thuộc mảng Library — đã phối hợp như trên. |
| 4 | Khoá xoá như #1; khoá chung một khoá; không còn `SELECT templates` ngoài transaction ở đường xoá. |
| 5 | **Body API có trần**: thêm `readAllLimited` (cap + 1 byte, chặn body quá lớn, lỗi thân đọc) dùng cho mọi phản hồi WBI/trang Bilibili; trang HTML fetch giữ `LimitReader` 1 MiB sẵn có; b23 không đọc body. Test `TestReadAllLimitedRejectsOversize`. |
| 6 | Ghi **tiến độ** dùng ctx của worker (`progressWriter.flush` dừng khi ctx huỷ); lần chốt trạng thái cuối (`Finish/Fail`) dùng `context.WithoutCancel` + hạn 5 s để shutdown vẫn ghi được kết quả; `close()` của runner có hạn 5 s; tắt theo `backdropRuntime.Close`: `stop()` (huỷ ctx) → join scheduler + listener + việc tải (trần 10 s) → serve đóng pool sau; `defer` trong serve gọi lần hai idempotent. Test `TestBackdropWakeListenerReceivesNotify` lộ đúng bẫy "Close pool trước cancel" và đã sửa thứ tự trong test. |
| 7 | `rate_limited` nhận diện theo **mã**, không chỉ `RetryAfter()`: `tools` có `ErrorCode()`; `platforms` chỉ coi là chặn khi `ErrorCode() == "rate_limited"` **hoặc** (không có mã và có `RetryAfter()`), và chỉ tin `RetryAfter()` cho lỗi chặn; test `TestRetryAfterOnlyCountsForRateLimited`. |
| 8 | `RunYtDlp` bọc lời gọi trong ctx deadline = `timeout + ytDlpLeaseMargin` tính từ mốc được cấp lượt; gia hạn lease nếu mốc bắt đầu trượt quá hạn ban đầu. Test `TestYtDlpRunsUnderLeaseDeadline`. |
| 9 | `LISTEN backdrop_changed` (một kết nối, tự kết nối lại, gọi `wake()` ngay sau LISTEN để bù NOTIFY trước đó) chạy trong serve; service `publish()` NOTIFY sau mọi thay đổi kênh/video (thêm/khoá/xoá/quét/chặn). Test `TestBackdropWakeListenerReceivesNotify`. |
| 10 | `RefreshChannel` Bilibili luôn đọc **trang 1 (mới nhất)**, không đụng `scan_next_page`; test `TestRefreshReadsNewestPageKeepsBackfill`. |
| 11 | Thêm kênh đã tồn tại → `RefreshChannel` ngay (không thêm dòng trùng, không trả "added" sai). |
| 12 | Độ dài clip chưa biết trong mix: `Pick` lọc video ngắn theo `clipSec`; clip chưa probe đếm là đủ khi chọn và `loopSec` tính nó bằng **`clipSec` cấu hình** (không coi là trọn video), có ghi chú trong `mix.go` + report mục 6. |
| 13 | `segment` có mặt nhưng thiếu `fromSec`/`toSec` → 400 `invalid_input`; test web `TestBackdropAddVideoPartialSegmentRejected`. |
| 14 | Cookie Bilibili chỉ đi qua host Bilibili: adapter `makeBackdropYtdlpRun` cấp tệp tạm qua `WithBilibiliCookieJar(rawURL)`; test `TestYtdlpAdaptersAreNilSafeAndCookieAware`. |
| 15 | Khôi phục lượt tải mồ côi: `RunScheduler` dọn `downloading` hết lease lúc khởi động (`ClearStaleDownloads` → `failed/interrupted`); thêm lại cùng id khi đang tải dở mà lease hết hạn sẽ **giành lượt** thay vì trả dòng kẹt. Test `TestBackdropDownloadClaimsGuardAndReconcile`, `TestAddVideo…` (thêm lại). |
| 16 | Atomically publish tệp: `publishFile` rename → EXDEV thì copy sang `.backdrop-*.part` cùng thư mục đích rồi rename; lỗi giữa chừng giữ tệp cũ, dọn tệp tạm. Test `TestPublishFileCrossVolumeKeepsOldUntilRename`, `TestPublishFileFailureLeavesOldAndCleansStaging`. |
| 17 | Bớt N+1: `Channel()`/danh sách đọc `platform_state` **một lần** và dùng chung (không còn `PlatformPause` từng dòng); parse `templates.recipe` **một lần** cho cả `usedBy` lẫn `chosenBy`; truy vấn danh mục đã dùng index sẵn có `backdrop_channel_videos_newest_idx` / `..._kind_idx` (không thêm index mới). |
| 18 | CLI: thêm `code` (và `retryAt` khi có) vào lỗi JSON; `invalid_input` tách khỏi `unsupported`; `tools`/`platforms` `login_required` → HTTP 401 (web `TestBackdropCodedErrorsMapToWire` kiểm cả `retryAt`). |
| 19 | Chỉ `dest` gây đổi `usedBy` mới khoá; chọn/xoá kênh dùng khoá đúng mục tiêu (khoá kênh cho kênh, khoá video cho video) — không khoá cả bảng. |
| 20 | `allowedRedownloadStates` là hàm có tên, không còn danh sách state rải rác trong `AddVideo`. |

Ghi chú thêm:
- **Migration `00008`**: mảng này dùng thêm cột/bảng đã khai trong migration đó (`platform_requests`, `backdrop_download_owner/generation/download_lease_until`, `platform_state.request_history`, `ytdlp_lease_owner/until`, `backdrop_videos`); không có migration mới, không sửa file của mảng khác.
- **sqlc**: đợt review không đổi `queries/*.sql`; đã chạy `lockf … go tool sqlc generate` rồi so sha256 toàn bộ `internal/persistence/postgres/gen/*.go` — **không diff**.
- Test mới sau review: `publish_test.go`, `links_test.go` (b23), `service_test.go` (refresh trang mới, mix unknown/short, khoá lượt tải), `pacing_test.go` (phân loại `rate_limited`, deadline lease), `web/backdrop_test.go` (mã lỗi 401/429+retryAt, segment thiếu đầu), `postgres/backdrop_pg_test.go` (claim/generation/reconcile, khoá xoá–lưu), `cmd/viderse/backdrop_wiring_test.go` (adapter cookie/nil, listener NOTIFY).


## 9. Sửa theo QA (`QA-P4-Backdrops.md`)

Hai lỗi major đều đã sửa và có test hồi quy:

1. **Link VOD Twitch dạng thật `https://www.twitch.tv/videos/<id>`**: `twitchVodPath` đổi thành `^/(?:[^/]+/)?videos/(\d+)/?$` — nhận cả dạng có login (link cũ) và không login (dạng thật). Fixture thêm vào `TestParseAssetLink` (`videos/2233445`, `videos/<id>?filter=all`) và `TestParseChannelLink` khẳng định cả hai dạng là **video**, không phải kênh.
2. **b23.tv bị theo redirect rồi chạm bilibili.com**: gốc lỗi là `transportOf` lấy `client.Do` của `*http.Client` làm `RoundTripper`; `Do` bên trong tự theo redirect nên `CheckRedirect` của client ngoài không có tác dụng — request thứ hai tới bilibili.com nhận 412 và đường parse lại đi qua `d.request` nên ghi giờ chờ 2 giờ.
   - `transportOf` nay lấy **transport thật** của client (`client.Transport`, rỗng thì `http.DefaultTransport`); fallback `roundTripDoer` chỉ còn cho doer test.
   - `pacedBilibiliDoer.Location` **không đi qua `d.request`/pacer** nữa: client riêng cấm follow redirect, deadline 10 s, chỉ đọc header `Location`, không đọc body, không request tới đích. Vì vậy 412/429 của riêng link rút gọn trả `RateLimitError` cho UI nhưng **không** được ghi thành trạng thái chặn nền tảng.
   - Test `TestB23LocationOneRequestNoFollowNoPause` (httptest): xác nhận đúng **1** request tới b23.tv, không có request nào bị theo sang host khác (transport lỗi nổi nếu bị theo), pacer không được gọi.
   - Test `TestB23Location412ReturnsErrorWithoutPause`: 412 → `*RateLimitError` trả về cho UI, đúng 1 request, `pacerCalls == 0` → không có pause nào bị ghi.
   - Test cũ `TestResolveB23OneRequestNoRedirectNoBody` vẫn xanh (doer test, không đọc body, deadline ~10 s).

Các lời gọi Bilibili thật (nav/trang danh mục) vẫn đi qua `d.request` và vẫn pause đúng khi bị chặn 412; chỉ đường **parse** là miễn nhiễm.


Status: DONE — backend đầy đủ theo phân công và đã xử lý toàn bộ review; build/vet/gofmt/archtest/sqlc sạch, test của mảng (kể cả Postgres thật, race, toàn bộ suite postgres không overlay) xanh, gói Báo lỗi kèm `platform-requests.json`, nhịp rảnh 0 lần đọc khi idle. Không chạy smoke mạng thật (theo hợp đồng test không gọi mạng; hạn mức Twitch/Bilibili còn nguyên cho controller) và chưa đo p95 bằng tải thật. Đã sửa 2 major của QA (VOD Twitch không login, b23 theo redirect/412 làm pause oan) kèm test httptest chứng minh 1 request – không follow – không pause.
