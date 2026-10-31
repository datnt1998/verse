# P4 — Kênh của bạn (backend)

Phạm vi: kênh của chủ, đối thủ liên kết, danh sách template luân phiên, bộ nhận diện
(tên + avatar + ảnh bìa kênh) với lease/phục hồi, HTTP + CLI. Không làm: clone job,
hàng chờ, worker Tự lấy, tự chủ/ưu tiên, bảng Kênh › Video, web UI.

## 1. Đã làm

### Tệp

| Tệp | Nội dung |
|---|---|
| `internal/domain/channels.go` | Kiểu giá trị: `Channel`, `Branding`, `ChannelSource`, `ChannelTemplate`, hằng `IdentityState` (`idle/running/failed`), `IdentityPart`, ngôn ngữ `auto/en/vi`, mã lỗi `interrupted`. |
| `internal/channels/channels.go` | `Service` (List/Get/Create/Update/Delete/Link/Unlink/Templates), `Error` + `CodeOf`, `Patch`/`Field[T]`, giới hạn đầu vào, `Store`/`StandardTemplates`/`Storage`/`Tasks` interface, chuẩn hoá tag/id, `newUUID` (crypto/rand), `Start`/`Close`, notifier SSE. |
| `internal/channels/language.go` | `guessLanguage`: dấu tiếng Việt → `vi`; chỉ chữ Latin → `en`; thiếu dữ liệu → `auto`. |
| `internal/channels/identity.go` | Runner nhận diện: lease 60 s + tim 5 s, fence token + tên, thư mục tạm, tải avatar đối thủ, gọi chuỗi model chữ, vẽ ảnh, công bố một ghi, dọn tệp cũ, ghi lỗi + `retryAt`, phục hồi `interrupted`. |
| `internal/channels/prompt.go` | Port prompt bước chữ + prompt ảnh từ `branding.ts`, parse JSON chặt (chỉ khoá đã biết, trần độ dài), escape `<`/`>`. |
| `internal/channels/files.go` | `AssetFile` (`os.OpenRoot` chặn symlink thoát + regex tên chặt + phải đang là tài sản của kênh), `moveFile` (rename/chép nguyên tử), `coverScale` (crop-resize Catmull-Rom về đúng canvas), `encodeBanner`/`writePNGAtomic` (JPEG thuần Go). |
| `internal/persistence/postgres/queries/channels.sql` | 30 câu sqlc cho kênh/link/template/lease/publish/fail/bối cảnh nhận diện. |
| `internal/persistence/postgres/channels.go` | `ChannelStore` (`var _ channels.Store`), transaction cho create/link/unlink/set-templates, fence lease, `MarkInterruptedIdentities`. |
| `internal/web/channels.go` | Route + wire + một hàm map lỗi; converter export `ChannelToWire` dùng chung CLI. |
| `cmd/viderse/channels_wiring.go` | Adapter `channelAI`/`channelDrawer`/`channelTasks`, `mountChannels` (một dòng trong `serve.go`), `openChannelService` cho CLI, `cliStorage`. |
| `cmd/viderse/channel.go` | CLI `viderse channel …`. |
| `cmd/viderse/serve.go` | +1 dòng `mountChannels(...)`, +4 dòng `channelSvc.Close(shutdown)` khi tắt. |
| `cmd/viderse/main.go` | +1 case `channel`, +1 dòng usage. |

### Quyết định

- **Tạo kênh đúng §4:** `CreateStandardTemplate` trước → **một** transaction (kênh +
  link đối thủ + `owner_channel_templates` position 0) → tx hỏng thì `DeleteTemplate`
  best-effort. Rival không còn trong `rival_channels` làm cả tx hỏng (khoá `FOR UPDATE`
  danh sách rival trước khi kiểm) nên không có link chết.
- **Ngôn ngữ:** đoán từ **đối thủ tham chiếu** = link sớm nhất (`linked_at`): tiêu đề +
  mô tả rival và 20 tiêu đề video gần nhất. Có dấu tiếng Việt → `vi`; chỉ chữ Latin →
  `en`; không đọc được → giữ `auto`. Client gửi `language` thì không đoán. Ghi đè ngôn
  ngữ là best-effort (kênh đã tạo).
- **Một run mỗi kênh:** `identity_lease_owner/token/until`; giành lease chỉ khi không
  còn lease sống; tim mỗi 5 s, mất lease → huỷ run, cấm công bố. Run thứ hai → 409.
- **Công bố nguyên tử:** một `UPDATE` ghi tên + branding + đường tài sản + trạng thái
  `idle`, `WHERE identity_lease_token = token AND lease còn hạn AND name = tên đọc lúc
  bắt đầu`. Đổi tên giữa chừng → giữ tên chủ; xoá giữa chừng → không hồi sinh; sửa
  trường khác → vẫn công bố.
- **Tài sản:** avatar **PNG** (giữ nguyên PNG của nguồn vẽ, 800×800); ảnh bìa
  **JPEG thuần Go** (`image/jpeg`), hạ chất lượng 90 → 80 → 65 tới khi ≤ 6 MB
  (2560×1440; băng an toàn 1546×423 nằm trong prompt). Không dùng ffmpeg/cgo.
  Tên tệp `<uuid>-avatar.png` / `<uuid>-banner.jpg`; ghi tạm rồi rename; tệp cũ chỉ
  xoá **sau** khi công bố thành công.
- **Lỗi nhận diện lưu thật:** `identity_state='failed'`, `identity_error_code`,
  `identity_error` (câu tiếng Việt), `identity_retry_at` (từ `DrawError.RetryAt`;
  hạn mức không có giờ thì hỏi `Drawer().QuotaReset()`). Run chết (lease hết hạn) khi
  khởi động → `failed` + code `interrupted`, **không** tự vẽ lại ảnh trả phí.
- **SSE:** mọi thay đổi kênh phát `channels` (gộp ≤ 4 frame/s qua `web.Notifier`).
- **CLI:** create/link **không** chạy nền (tiến trình CLI không sống đủ lâu); lệnh
  `channel identity` chạy **đồng bộ** và trả lỗi thật. Serve bật `RunInBackground`.
- **Phục hồi không chờ restart:** ngoài lượt `interrupted` lúc khởi động, một timer
  được hẹn tới hạn lease gần nhất (kênh còn lease sống khi tiến trình nhận lại) nên
  kênh chết giữa chừng tự về `failed`, không treo `generating` vô thời hạn.
- **Lease trước, task sau:** task activity chỉ tạo sau khi giữ được lease và gắn vào
  kênh bằng `AttachIdentityTask` (rào theo token) — lệnh thua lease không bao giờ
  tạo rồi xoá task của run đang thắng. Cập nhật task-hỏng chỉ khi bản ghi lỗi được
  chấp nhận.
- **Gate Close:** `admit`/`release` + `closed` — `Close` chờ cả prepare đang giữa
  chừng; gọi identity sau Close → 409.
- **Serialize theo hàng kênh:** `LinkSource`/`UnlinkSource` và `UpdateTemplates`
  (đọc-đổi-ghi bằng closure) chạy trong tx có `SELECT … FOR UPDATE` trên
  `owner_channels` — hai bỏ-link đồng thời không thể cùng thấy "còn 1 nguồn", hai
  add template không mất cập nhật. apply bắt buộc thuần (kiểm tra tồn tại/đầu vào
  chạy trước khi vào apply) để không tự deadlock trên chính khoá hàng.
- **Lỗi không lộ nguyên nhân:** chỉ `channels.Error` và `imagegen.DrawError` giữ
  câu tiếng Việt; mọi lỗi lạ → `internal_error` + câu trung lập, nguyên nhân chỉ đi
  `diag` (log đỏ-masked).
- **Canvas đúng tuyệt đối:** provider trả ảnh lệch size thì vẫn cover-crop +
  resize Catmull-Rom về 800×800 / 2560×1440 trước khi encode — kích thước file là
  hợp đồng, không phải gợi ý trong prompt.
- **Phục vụ ảnh an toàn:** `GET …/branding/{file}` mở file qua `os.OpenRoot(root)`
  nên symlink (trong thư mục kênh hay cha) thoát ra ngoài project bị chặn ngay khi
  mở; phục vụ bằng `http.ServeContent`.
- **needsText đúng nghĩa:** kênh có `brief` chủ tự viết nhưng chưa từng sinh thiết
  kế vẫn chạy bước chữ; "đã có nhận diện" = look/summary/branding-ảnh/tài sản tồn
  tại, không tính brief.

## 2. API cho web

Mọi body JSON; lỗi luôn có hình `{error, code, recovery?, retryAt?}`.

### Hình `channel`

```jsonc
{
  "id": "uuid",
  "name": "string",                  // "" = chưa có tên (nhận diện sẽ đặt)
  "youtubeUrl": "string|null",
  "language": "en|vi|auto",
  "styleNote": "string|null",        // ≤ 8000
  "descriptionFooter": "string|null",// ≤ 1000
  "alwaysTags": ["string"],          // ≤ 10, đã trim/bỏ trùng
  "autoPick": true,
  "pacePerDay": 5,                   // 1..50
  "templateCursor": 0,
  "branding": {                      // rỗng khi chưa có nhận diện
    "brief": "string", "summary": "string", "look": "string",
    "avatar": "string", "banner": "string"
  },
  "avatarUrl": "/api/owner-channels/<id>/branding/<file>|null",
  "bannerUrl": "/api/owner-channels/<id>/branding/<file>|null",
  "sources": [{                      // thứ tự liên kết; [0].reference = true
    "rivalId": "UC…", "title": "string", "handle": "@…|null",
    "linkedAt": "RFC3339", "reference": true
  }],
  "templates": [{ "id": "uuid", "name": "string", "position": 0 }],
  "identity": {
    "state": "ready|generating|failed",   // idle→ready, running→generating
    "error": { "code": "quota", "message": "Hết hạn mức ảnh hôm nay." }, // chỉ khi failed
    "retryAt": "RFC3339",              // khi có
    "startedAt": "RFC3339",            // khi đang chạy
    "part": "all|name|avatar|banner",  // run hiện tại/gần nhất
    "jobId": "task id"                 // id việc nền activity
  },
  "createdAt": "RFC3339", "updatedAt": "RFC3339"
}
```

`identity.error.code` có thể là mã của imagegen (`quota`, `cancelled`, `conflict`,
`internal_error`, `invalid_request`, `tool_missing`, `failed_unknown`, `stale`) hoặc
`interrupted` (run chết), `invalid_input` (câu trả lời model sai hình).

### Route

| Method + path | Body | Thành công | Lỗi |
|---|---|---|---|
| `GET /api/owner-channels` | — | `200 {channels:[channel]}` (tên rỗng cuối, không phân biệt hoa thường) | — |
| `POST /api/owner-channels` | `{name?, youtubeUrl?, competitorIds:[…] (bắt buộc ≥1 ở web), autoPick?=true, pacePerDay?=5, language?}` | `201 {channel, identityJobId: string\|null}` | `400 invalid_input` “Chọn ít nhất 1 đối thủ làm nguồn.” (recovery “Đối thủ là nguồn video mẫu cho kênh.”); `400` pace ngoài 1–50 / ngôn ngữ lạ / tên > 200 / link > 2048 / > 200 đối thủ; `404 not_found` “Đối thủ <id> chưa được theo dõi.” (recovery “Thêm đối thủ ở trang Đối thủ trước.”) |
| `GET /api/owner-channels/{id}` | — | `200 channel` | `404 not_found` “Kênh "<id>" không tồn tại.” |
| `PATCH /api/owner-channels/{id}` | `{name?, youtubeUrl?(null), language?, styleNote?(null), descriptionFooter?(null), alwaysTags?[], autoPick?, pacePerDay?, brief?}` — vắng = giữ, `null` = xoá | `200 channel` | `400` (tên rỗng/>200, pace, ngôn ngữ, tag > 10, brief > 2000, chuỗi quá trần); `404` |
| `DELETE /api/owner-channels/{id}` | — | `204` | `404` |
| `POST /api/owner-channels/{id}/sources` | `{rivalId}` | `200 channel` (idempotent) | `400` “Thiếu rivalId.”; `404` “Đối thủ <id> chưa được theo dõi.” |
| `DELETE /api/owner-channels/{id}/sources/{rivalId}` | — | `200 {autoPickPaused: bool}` — `true` khi bỏ đối thủ cuối (đã tắt `autoPick`) | `404` “Đối thủ <id> chưa được liên kết với kênh.” |
| `POST /api/owner-channels/{id}/templates` | `{action:"add"\|"remove"\|"move"\|"standard", templateId?, position?}` | `200 {templates:[{id,name,position}]}` (position liền mạch từ 0) | `400` action/thiếu templateId/position; `404` template không có / không thuộc kênh |
| `POST /api/owner-channels/{id}/branding` | `{part?="all", brief?, direction?}` | `202 {channel, identityJobId}` (identity chuyển `generating`) | `400` part lạ / brief > 2000 / direction > 1000; `404`; `409 conflict` “Đang có một lượt tạo nhận diện khác cho kênh này.” (recovery “Chờ lượt đang chạy xong rồi thử lại.”) |
| `GET /api/owner-channels/{id}/branding/{file}` | — (thêm `?download=1`) | `200` ảnh (`image/png`/`image/jpeg`, `Cache-Control: private, max-age=31536000, immutable`; `download=1` → `Content-Disposition: attachment`) | `404` “Không tìm thấy ảnh nhận diện.” / “Ảnh nhận diện không còn được kênh dùng.” |

Tên tệp chỉ nhận `^[0-9a-f]{8}-…-(avatar\.png|banner\.jpg)$` **và** phải đang là
avatar/ảnh bìa của chính kênh; mọi thứ khác là 404. File mở dưới `os.Root` của
project nên symlink thoát không đọc được.

### Map lỗi

| `code` | HTTP |
|---|---|
| `invalid_input` | 400 |
| `not_found` | 404 |
| `conflict`, `cancelled`, `interrupted` | 409 |
| `quota` | 429 + `retryAt` |
| `tool_missing` | 503 |
| `internal_error` / lỗi lạ | 500 `{error:"lỗi nội bộ", code:"internal"}` (nguyên nhân chỉ vào log) |

### SSE

- Loại `channels`, data `{"rev":N,"ref":"<channelId>"}` — phát khi: tạo, sửa, xoá,
  liên kết/bỏ liên kết, thêm/bỏ/dời/chuẩn-hoá template, bắt đầu run nhận diện, công bố,
  hỏng, phục hồi `interrupted` lúc khởi động.
- Tiến độ nhận diện đi qua loại `activity` của `activity.Service` (kind `identity`):
  `title "Tạo bộ nhận diện"`, agent `artist`, `objectRef {type:"channel", id, label:<tên>}`,
  phase lần lượt “Đang đặt tên và viết mô tả kênh” → “Đang vẽ avatar 800×800” →
  “Đang vẽ ảnh bìa kênh 2560×1440” → “Đang công bố nhận diện”, `progress {done,total}`
  (total = 1 nếu có bước chữ + số ảnh + 1), action `{label:"Mở kênh", href:"/channels/<id>?tab=settings"}`;
  hỏng → `state:"failed"`, action `{label:"Thử lại"}`; xong → task bị xoá + notification
  `task_done` “Đã tạo nhận diện cho <tên>.”.

## 3. CLI

`viderse channel list|show|create|update|delete|link|unlink|template|identity`
(JSON ra stdout; lỗi `{error, code?, recovery?, retryAt?}` ra stderr, exit 1).
Cần `VIDERSE_DATABASE_URL`. `create` giữ tuỳ chọn 0 đối thủ; không chạy nền — dùng
`viderse channel identity <id> [--part all|name|avatar|banner] [--brief …] [--direction …]`
(đồng bộ, cần cấu hình Trợ lý AI). `update` dùng cờ đã đưa (`--url ""` để xoá);
`template add|remove|move|standard`; `link`/`unlink`; `show` in đúng hình wire.

## 4. Test

- `internal/channels` (15 ca service + 21 ca identity, có `-race`): CRUD/thứ tự tên,
  bù trừ tx, validation, tag trim/dedupe, xoá kèm tệp, link chỉ đụng link, rival lạ
  404, bỏ đối thủ cuối tắt Tự lấy, tham chiếu dời, template add/move/remove/standard
  + renumber, đoán ngôn ngữ, AssetPath chặt.
- Identity (port `yt-factory src/server/channels/branding.test.ts`): công bố tên +
  tài sản + branding; JSON hỏng không đụng gì; `part=name` không vẽ; `part=avatar`
  giữ tên/banner; canvas + băng an toàn 1546×423; gửi avatar tham chiếu; run thứ hai
  409 **và không tạo task thừa**; mất lease không công bố; quota lưu `retryAt`;
  phục hồi `interrupted`; **timer reconcile đánh dấu lease chết**; đổi tên giữa
  chừng; xoá giữa chừng không hồi sinh; sửa trường khác vẫn công bố; tệp cũ giữ
  tới khi thành công; **lỗi lạ không lộ nguyên nhân**; **kênh có brief chủ viết
  vẫn chạy bước chữ**; **file công bố đúng canvas dù provider trả lệch size**;
  **Close chờ prepare giữa chừng, gọi sau Close → 409**; 8 run song song dưới `-race`.
- `internal/web` (12 ca): wire list/identity failed+retryAt, create bắt buộc đối thủ,
  `identityJobId`, PATCH null tường minh, unlink `{autoPickPaused}` đúng một khoá,
  template move, branding 202, map lỗi 400/404/409/429/503/500, tải ảnh + attachment,
  route lạ 404, collection rỗng là `[]`.
- `internal/persistence/postgres/channels_pg_test.go` (10 ca, `TEST_DATABASE_URL`):
  CRUD + cascade, rival lạ không tạo kênh, bỏ đối thủ cuối + dời tham chiếu,
  **hai unlink đồng thời đúng một paused**, **hai add template đồng thời serialize
  qua khoá hàng**, mẫu ngôn ngữ/tham chiếu, lease (giành hai lần, heartbeat,
  publish fence theo tên, attach task), fail + phục hồi `interrupted`, patch từng
  trường + chỉ-brief không phá trường khác.
- Nguồn port: `yt-factory src/server/channels/{branding,owner-channels}.test.ts`.

Đã chạy: `go build ./...`, `go vet` (các gói kênh), `gofmt -l` (sạch), `go test
./archtest/`, `go test -race ./internal/channels/` (xanh, có ca đua),
`go test ./internal/web -run TestChannel`,
`TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test go test ./internal/persistence/postgres -run TestChannel`,
`sqlc generate` (md5 `channels.sql.go` không đổi). Không đo p95 (không mở serve).

## 5. Lệch so với design/yt-factory

1. **Định dạng tài sản:** yt-factory mã hoá WebP bằng ffmpeg+libwebp; ở đây avatar giữ
   PNG, ảnh bìa JPEG thuần Go (yêu cầu không cgo). Không còn lỗi `tool_missing` do
   thiếu libwebp.
2. **Lỗi nhận diện được lưu:** yt-factory chỉ ném lỗi; đợt này lưu `identity_state/
   error/retry_at` trên `owner_channels` (theo hợp đồng) để UI `failed` + `retryAt`
   sống qua tải lại trang.
3. **CLI `create` không chạy nền** (tiến trình không giữ được lease); `identity`
   đồng bộ. Trên serve, create/link tự chạy nền như §4/§8.
4. **Tham chiếu chỉ có avatar:** `rival_channels` không có `banner_url`, nên prompt
   chữ nhận avatar tham chiếu (không nhận ảnh bìa đối thủ); prompt ảnh vẫn yêu cầu
   bám định dạng avatar.
5. **Tự lấy khi liên kết:** chỉ tự chạy nhận diện khi kênh **chưa có** nhận diện đã
   công bố và đây là link đầu tiên (không dựa vào tên chủ tự đặt).
6. **Template rotation** chạy trong khoá hàng kênh (`UpdateTemplates` + closure)
   thay vì `SetTemplates` read-modify-write trần — serialize đúng cả khi hai tab
   thêm template đồng thời.
7. **`nextTemplate`** (con trỏ luân phiên) chưa làm — chưa cần.
8. **DELETE sources** trả đúng `{autoPickPaused}` như hợp đồng; UI đọc lại kênh bằng GET.
9. **SSE thư viện:** template “Mẫu chuẩn” do mảng kênh tạo vẫn phát
   `library.templates` qua một `library.Service` tối giản ghép trong
   `channels_wiring.go` (không dựng route/runner của thư viện).

## 6. Việc còn lại

- Kênh › tab Video (bảng job, 4 số liệu, chấm Chờ bạn), khoá xoá khi có video đang
  chạy, hàng chờ + worker Tự lấy, tự chủ/ưu tiên — phase sau.
- `GET/PATCH` lịch đăng của kênh (07 §12) không nằm trong hợp đồng §5 nên chưa làm.
- Ảnh bìa tham chiếu của đối thủ cần cột `rival_channels.banner_url` (migration sau).
- Chưa smoke `channel identity` thật (không gọi model trả phí theo hợp đồng §0).

Status: DONE
Backend Kênh của bạn đã đủ route/CLI/lease/nhận diện theo hợp đồng, test xanh trên
Postgres thật; các mục còn lại đều thuộc phase clone-job/queue/editor.
