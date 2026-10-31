# P4 — Thư viện (template, prompt ảnh, lớp phủ)

Backend Go cho mảng **Thư viện** của Viderse: recipe template + revision, kho
prompt ảnh (7 prompt có sẵn + prompt của bạn), và thư viện lớp phủ dùng chung
(12 hiệu ứng nhúng). Web dựng ở đợt sau đọc §2.

Nguồn đối chiếu: yt-factory `src/editor/engine/features/editing-templates/
{template-schemas,template-limits,template-recipe,background-appearance,
overlay-library}.ts`, `src/server/templates/{registry,settings}.ts`,
`src/server/clone/{template-snapshot,image-prompt}.ts`,
`src/server/editor/{overlay-library,media-background}.ts`,
`assets/overlays/manifest.json`, `cli/{template,overlay}.ts`; thiết kế
`01-ia-flows` W12/L9/L12, `07-data-contract` §13 §15 §24, `08-dev-prompt` V8/V15,
`09-migration-yt-factory` §3, artboards `Library.dc.html` + `Template.dc.html`,
`06-copy`, `11-hoi-dap`, `13-changelog`.

## 1. Đã làm

### 1.1 Tệp

| Tệp | Nội dung |
|---|---|
| `internal/domain/library.go` | Kiểu thuần: `Recipe` và các khối con, `Template`, `PreviewState`, `ChannelRef`, `ImagePrompt`, `Overlay`, `ThumbnailStyles`, bảng `LibraryLimits` (một chỗ cho API + CLI) |
| `internal/library/recipe.go` | `DefaultRecipe`, `StandardRecipe`, `ParseRecipe` (JSON → recipe, đè lên bản hiện tại, cấm field lạ), `ValidateRecipe` (miền giá trị), `CheckRefs` (prompt/lớp phủ/kênh/video nền còn tồn tại), `ImageCountFor` port `recipeImageCount` |
| `internal/library/errors.go` | `Error{Code,Message,Recovery,Fields,Channels,UsedBy,RetryAt}`, mã `invalid_input/not_found/conflict/builtin/in_use/bad_media/tool_missing`, sentinel `ErrNotFound/ErrNameTaken/ErrOverlayExists/ErrInUse` |
| `internal/library/prompts.go` | 7 prompt có sẵn (id cố định), `foldText` (cùng luật `fold_title` của DB), sắp xếp + lọc |
| `internal/library/prompts_service.go` | CRUD prompt, `PromptUsage`, xuất/nhập `.json` (nhập trùng tên tự thêm ` (nhập)`, ` (nhập 2)`…) |
| `internal/library/library.go` | Service: list/get/create/duplicate/patch(CAS)/rename/delete, `CreateStandardTemplate`, `DeleteTemplate` (hợp đồng §4), `Summary`/`Detail`, `Estimate` giá, `PromptUsage` |
| `internal/library/snapshot.go` | `SnapshotTemplate` — phân giải thuần cho job start (L9) |
| `internal/library/assets.go` + `internal/library/assets/**` | `go:embed` 11 tệp `assets/overlays/*.mp4` + `manifest.json` + `assets/standard-template/snow-loop.mp4`; `BuiltinEffects` đối chiếu sha256 manifest |
| `internal/library/media.go` | ffprobe metadata + lấy 8 khung rgb24 64×36 + `classifyBackground` port `media-background.ts`; MIME allowlist |
| `internal/library/overlays.go` | Thêm (tệp/reader), dedupe sha256, tombstone, khôi phục khi thêm lại, `SeedBuiltins` idempotent, đường dẫn tệp |
| `internal/persistence/postgres/queries/library.sql` | 27 truy vấn sqlc (template/prompt/lớp phủ + đọc chéo `owner_channel_templates`, `backdrop_*`) |
| `internal/persistence/postgres/library.go` | `LibraryStore` hiện thực `library.Store`; CAS `UPDATE … WHERE revision=$n`; dịch 23505/23503 sang sentinel |
| `internal/web/library.go` | Route + hình wire đầy đủ + **một** bảng map lỗi `writeLibraryError`; `writePromptByID`/`writeTemplateByID` đọc lại sau ghi |
| `internal/web/library_mix.go` | Điểm nối duy nhất `TemplateBackdropMix` + `IsTemplateBackdropMix`/`ServeTemplateBackdropMix` cho route mix của mảng Video nền |
| `cmd/viderse/library_wiring.go` | `mountLibrary`, `seedLibraryBuiltins`, adapter `tools.Runner`→`library.Runner`, `tools.Ffmpeg`→`library.Binaries` (cache 1 phút), `aiconn`→`library.PriceOf`, `openLibrary` cho CLI |
| `cmd/viderse/template.go`, `cmd/viderse/overlay.go` | CLI `template list\|show\|create\|duplicate\|rename\|set\|delete`, `overlay list\|add\|remove` |
| `cmd/viderse/serve.go` | +2 dòng: `mountLibrary(...)` + `go seedLibraryBuiltins(...)` |
| `cmd/viderse/main.go` | +2 case dispatch + 2 dòng usage |
| Test | `internal/library/{recipe,library,overlays}_test.go` + `memstore_test.go`, `internal/web/library_test.go`, `internal/persistence/postgres/library_pg_test.go` |

### 1.2 Quyết định kỹ thuật

1. **Revision CAS ở SQL.** `UPDATE templates SET recipe=$, revision=revision+1
   WHERE id=$ AND revision=$expect RETURNING revision`. Không có hàng → `409
   conflict`. Đua hai người lưu: đúng một thắng (test cả mem-store lẫn
   Postgres thật).
2. **Rename không bump revision.** Tên không đi vào bản xem thử; PATCH đổi tên
   vẫn qua CAS vì nằm chung route. Rename riêng (CLI) nhận `--revision` tuỳ
   chọn.
3. **Recipe là một JSONB, PATCH thay cả khối.** Trường vắng lấy giá trị hiện
   tại (không phải default toàn cục) — nhờ vậy client gửi `{"scenes":{…}}`
   không làm mất `appearance`/`output`.
4. **Một đường kiểm:** `ParseRecipe` → `ValidateRecipe` → `CheckRefs` dùng cho
   HTTP và CLI; lỗi trả theo từng trường (`fields[{field,message,recovery}]`).
5. **Prompt có sẵn là hằng số Go**, không có dòng DB → id `builtin:*` không thể
   trùng uuid người dùng; 403 khi sửa/xoá.
6. **Lớp phủ:** một dòng mỗi sha256; tệp dưới `<dự án>/library/overlays/…`;
   `removed_at` là tombstone — template đang dùng vẫn giữ id, snapshot bỏ lớp
   đó khi dựng job; thêm lại đúng byte sẽ khôi phục dòng cũ.
7. **Seeding** chạy nền lúc serve khởi động, ctx-bound, idempotent: bỏ qua nếu
   `builtin` đã có dòng (kể cả tombstone) hoặc sha256 đã có trong thư viện.
   Không bao giờ chạy trong GET.
8. **Giá ảnh:** `price.perImage` đọc từ `aiconn` (model vẽ chính của Hoạ sĩ,
   `AIModel.PerImage`); `known=false` khi chưa khai giá.
9. **`CreateStandardTemplate`** tạo mới mỗi lần gọi, tên `Mẫu chuẩn · <kênh>`
   (hoặc `Mẫu chuẩn`), recipe Mẫu chuẩn + lớp tuyết **nếu** hiệu ứng đã có
   trong thư viện.
10. **Multipart:** guard JSON dùng chung (`platform/webapp`) chặn mọi POST
    không `application/json`; thêm hook `Options.AllowNonJSON` và chỉ mở cho
    `POST /api/overlays`, `POST /api/image-prompts/import` khi
    `multipart/form-data`. Ranh giới CSRF vẫn là `webguard.CrossSiteRequest`
    (chạy trước) + cookie SameSite.

### 1.3 Quyết định 7 ảnh vs số ảnh template (`11-hoi-dap`)

Dùng **số ảnh của template** (`images.mode/count`), cộng **1 ảnh bìa**:
`totalUSD = (imagesPerVideo + 1) × perImage`. Chế độ `per-minutes` không
biết số ảnh khi chưa có video nguồn nên trả `imagesPerVideo=null`,
`totalUSD=null`, kèm `mode/perMinutes` để UI ghi cách tính. Chế độ `none` vẫn
tính 1 ảnh bìa (video không ảnh vẫn có bìa). Hằng "7" của design chỉ là ví dụ
mock, không phải luật nghiệp vụ.

## 2. API cho web

Tất cả route sau `/api`, cần session cookie. Lỗi dùng chung một hình:

```json
{"error":"câu tiếng Việt","code":"invalid_input|not_found|conflict|builtin|in_use|bad_media|tool_missing|internal",
 "recovery":"câu phục hồi (có thể vắng)",
 "fields":[{"field":"scenes.images.count","message":"…","recovery":"…"}],
 "channels":["Tên kênh"],"usedBy":["<template id>"],
 "reason":"in_use",   // chỉ khi code=in_use (đúng chữ hợp đồng §5)
 "retryAt":"RFC3339"}
```

Mã → HTTP: `invalid_input` 400 · `not_found` 404 · `conflict` 409 ·
`in_use` 409 · `builtin` 403 · `bad_media` 400 · `tool_missing` 503 ·
`internal` 500. Body JSON hỏng/field lạ → 400 kèm `code:"invalid_input"`.

### 2.1 Kiểu dùng chung

```ts
type TemplateImages = { mode: "fixed"|"per-minutes"|"none"; count: number; minutes: number;
                        perVideo: number|null; known: boolean }
type TemplatePrompts = { sceneId: string; sceneName: string; coverId: string; coverName: string;
                         summary: string }   // "Prompt <tên> · Tách nền[ · Cận cảnh] · bìa <tên>"
type TemplateBackdrop = { channelCount: number; videoCount: number; clipCount: number|null;
                          clipSec: number; enabled: boolean }
type TemplatePreview = { state: "none"|"fresh"|"stale"; builtAt: string|null }
type ChannelRef = { id: string; name: string; position?: number }
type TemplateSummary = {
  id: string; name: string; revision: number;
  style: "full"|"cutout"; framing?: "close";
  images: TemplateImages; prompts: TemplatePrompts;
  backdrop: TemplateBackdrop; overlayCount: number;
  channels: ChannelRef[]; preview: TemplatePreview;
  createdAt: string; updatedAt: string
}
type PriceEstimate = { perImage: number; known: boolean; imagesPerVideo: number|null;
                       includesCover: boolean; mode: "fixed"|"per-minutes"|"none";
                       perMinutes?: number; totalUSD: number|null }
type TemplateDetail = TemplateSummary & { recipe: Recipe; price: PriceEstimate; savedAt: string }
```

### 2.2 Template

**`GET /api/templates`** → `200 {"items": TemplateSummary[]}` (mới sửa trước).

**`POST /api/templates`** body `{"name"?: string, "fromId"?: string}`
- `fromId` có → nhân bản template đó (tên mặc định `<tên> — Bản sao`), 404 nếu
  nguồn không có.
- không `fromId` → tạo mới với recipe Mẫu chuẩn; `name` rỗng/blank → 400
  (`fields[{field:"name"}]`).
→ `201 TemplateDetail`.

**`GET /api/templates/{id}`** → `200 TemplateDetail`; 404 `not_found` (kể cả
id không phải UUID — store sentinel được map 404, không còn 500).

**`PATCH /api/templates/{id}`** body
```json
{"revision": 3, "name": "tuỳ chọn", "recipe": { …Recipe… }}
```
- `revision` bắt buộc (thiếu/≤0 → 400 "Thiếu revision khi lưu template").
- `recipe` vắng = giữ nguyên; có = thay cả khối (trường thiếu lấy giá trị hiện
  tại), rồi `ValidateRecipe` + `CheckRefs`.
- Revision lệch → `409 conflict` `{"code":"conflict","error":"Template vừa
  được sửa ở nơi khác","recovery":"Tải lại trang rồi thử lại"}`.
- Lỗi trường → 400 + `fields`.
→ `200 TemplateDetail` (revision mới; `savedAt` = `updatedAt` = giờ lưu).

**`DELETE /api/templates/{id}`** → `204`; nếu kênh còn dùng →
`409 {"code":"conflict","error":"Đang dùng ở N kênh. Bỏ khỏi kênh trước rồi
xoá.","recovery":"Mở kênh đang dùng, bỏ template khỏi danh sách luân phiên rồi
xoá lại","channels":["Tên kênh"]}`. 404 khi không có.

**`GET /api/templates/{id}/backdrop/mix?lengthSec=N`** → mảng Video nền phục vụ
(route đăng ký cùng tiền tố, `internal/web/library_mix.go` chuyển tiếp). Body
theo báo cáo mảng Video nền.

### 2.3 Prompt ảnh

```ts
type ImagePrompt = { id: string; name: string; kind: "scene"|"cover"; builtIn: boolean;
  composition?: "clean"|"hook"|"circle-arrow"|"hook-arrow"|"split"|"closeup";
  prompt: string; updatedAt: string;
  usedBy: { id: string; name: string }[];
  samples: string[]   // LUÔN rỗng ở đợt này (vẽ mẫu cần video đã xong)
}
```

**`GET /api/image-prompts?kind=scene|cover&q=<từ khoá>`** →
`200 {"items": ImagePrompt[]}`. Prompt có sẵn trước (scene rồi 6 bìa theo
`ThumbnailStyles`), rồi prompt của bạn theo `updatedAt` giảm dần. `q` tìm trên
tên + nội dung, không phân biệt hoa thường/**, bỏ dấu tiếng Việt**.
`kind` lạ → 400.

**`POST /api/image-prompts`** body
`{"name": string, "kind": "scene"|"cover", "composition"?: ThumbnailStyle, "prompt"?: string, "fromId"?: string}`
- `fromId` (nhân bản) → trường vắng lấy của prompt nguồn; nguồn có sẵn cũng
  được.
- `kind:"cover"` bắt buộc `composition`; `kind:"scene"` cấm `composition`.
- `prompt` ≤ 2000 ký tự, `name` 1–60.
- Tên trùng (kể cả trùng tên prompt có sẵn, không phân biệt hoa thường) →
  `409 {"code":"conflict","error":"Đã có prompt tên \"…\"","recovery":"Đặt tên
  khác, hoặc nhân bản prompt có sẵn rồi sửa"}`.
→ `201 ImagePrompt`.

**`GET /api/image-prompts/{id}`** → `200 ImagePrompt` (id có sẵn cũng đọc được).

**`PATCH /api/image-prompts/{id}`** body `{"name"?, "kind"?, "composition"?, "prompt"?}`
- Prompt có sẵn → `403 {"code":"builtin","error":"Prompt có sẵn của Viderse
  không sửa và không xoá được","recovery":"Nhân bản để có bản của bạn rồi sửa"}`.
- Đổi `kind` khi template còn dùng → `409 {"code":"in_use","reason":"in_use",
  "usedBy":["<template id>"],"error":"Không đổi được loại prompt khi template
  còn dùng. Đang dùng ở <tên>."}`.
→ `200 ImagePrompt`.

**`DELETE /api/image-prompts/{id}`** → `204`; có sẵn → 403; đang dùng →
`409 {"code":"in_use","reason":"in_use","usedBy":[…],"error":"Đang dùng ở
<danh sách tên>. Đổi prompt ở template đó trước rồi xoá."}`.

**`GET /api/image-prompts/{id}/export`** → `200` body
`{"format":"viderse-image-prompt","version":1,"name","kind","composition"?,"prompt","exportedAt"}`
+ `Content-Disposition: attachment; filename="<tên>.json"` (tên không ASCII
dùng `filename*=UTF-8''…` của chuẩn MIME).

**`POST /api/image-prompts/import`** nhận `application/json` thân tệp **hoặc**
`multipart/form-data` trường `file`; trần 64 KiB. Tệp sai dấu → 400 "Tệp không
phải prompt của Viderse". Trùng tên → tự đặt `"<tên> (nhập)"`, `"<tên> (nhập
2)"`… → `201 ImagePrompt`.

### 2.4 Lớp phủ

```ts
type Overlay = { id: string; name: string; mime: string; bytes: number;
  durationSec: number|null; width: number|null; height: number|null;
  background: "black"|"green"|"none";
  builtIn: string;        // "snow","rain",… hoặc "" (tệp của bạn)
  createdAt: string;
  usedBy: { id: string; name: string }[];
  url: string             // "/api/overlays/<id>/file"
}
```

**`GET /api/overlays`** → `200 {"items": Overlay[]}` (mới nhất trước; không
probe trong GET).

**`POST /api/overlays`** `multipart/form-data`: `file` (bắt buộc), `name` (tuỳ
chọn — **chỉ nhãn hiển thị**, mặc định = tên tệp bỏ đuôi, tối đa 200 ký tự).
Đuôi/MIME suy từ `filename` của part `file`, không từ `name`.
- Đuôi nhận: `.mp4 .m4v .mov .webm .png .jpg .jpeg .webp`; sai → 400 kèm danh
  sách đuôi.
- **Sniff nội dung:** chữ ký bytes phải khớp đuôi (`ftyp`/box BMFF → mp4/mov/m4v;
  EBML → webm; magic ảnh tương ứng). Sai → 400 `bad_media` "Nội dung tệp không
  khớp loại …". ffprobe bắt buộc thấy một stream video/ảnh có kích thước —
  audio-only bị từ chối.
- Trần 256 MiB (413-style → dùng 400 `invalid_input` "Tệp lớp phủ quá lớn").
- Trùng sha256 → `201` dòng cũ (thêm lại dòng đã bỏ = khôi phục, đổi về tên vừa
  tải). Nếu **tệp của dòng cũ đã mất khỏi đĩa**, bytes vừa tải được công bố
  lại vào đúng đường dẫn cũ → `GET /file` tự phục hồi.
- Máy chưa có ffmpeg/ffprobe → `503 {"code":"tool_missing","recovery":"Cài
  ffmpeg trong Cài đặt → Công cụ rồi thử lại"}`.
- Tệp hỏng → 400 `bad_media` "Không đọc được tệp: …".
→ `201 Overlay`. Trường `name` trả về **không có đuôi tệp** — web đọc `mime`
thay vì suy từ tên.

**`DELETE /api/overlays/{id}`** → `204` (tombstone; template đang dùng vẫn phủ
được; hiệu ứng có sẵn cũng bỏ được và **không** tự quay lại). 404 nếu đã bỏ.

**`GET /api/overlays/{id}/file`** → tệp với `Content-Type` của dòng,
`Cache-Control: private, max-age=3600`, hỗ trợ `Range`/`If-Range`,
`?download=1` thêm `Content-Disposition: attachment`.

### 2.5 SSE

Đều gửi cho mọi phiên đăng nhập, `data {"rev":N,"ref":"<id>"}` (`ref` rỗng khi
gộp nhiều thay đổi khác nhau trong 250 ms):

| Loại | Khi nào |
|---|---|
| `library.templates` | tạo/nhân bản/PATCH/rename/delete template |
| `library.prompts` | tạo/sửa/xoá/nhập prompt |
| `library.overlays` | thêm/khôi phục/bỏ lớp phủ; seeding thêm hiệu ứng (`ref:""`) |

## 3. Recipe JSON — schema, giới hạn, mặc định

```jsonc
{
  "scenes": {
    "style": "full" | "cutout",                  // bắt buộc, mặc định "full"
    "framing": "close",                          // tuỳ chọn; chỉ hợp lệ khi style="cutout"
    "images": {                                  // mặc định {"mode":"fixed","count":5}
      "mode": "fixed" | "per-minutes" | "none",
      "count": 1..60,                            // khi mode=fixed
      "minutes": 1..60                           // khi mode=per-minutes
    },
    "motion": {                                  // mặc định {"kind":"ken-burns","intensity":1,"speed":1}
      "kind": "ken-burns" | "wander",
      "intensity": 0.25..2,
      "speed": 0.25..3
    },
    "scenePromptId": "<uuid|builtin:scene>",     // mặc định "" → builtin:scene khi chốt
    "coverPromptId": "<uuid|builtin:cover-*>",   // mặc định "builtin:cover-clean"
    "coverSource": "upload" | "ai",              // mặc định "upload"
    "coverMediaId": string,                      // tuỳ chọn
    "referenceImages": boolean,                  // mặc định true
    "referenceStyle": "composition" | "master",  // mặc định "composition"
    "sceneOpacity": 0..1                         // mặc định 1
  },
  "backdrop": null | {
    "channels": [ {                              // 1..20, không trùng channelId
      "channelId": "twitch:[a-z0-9_]{1,25}" | "bilibili:[0-9]{1,20}",
      "kinds": ["clip"|"highlight"|"vod"|"video"], // 1..3, phải đúng nền tảng
      "pinned": [string ≤64]                     // ≤200; rỗng = mọi video của kinds
    } ],
    "clipCount": 1..40 | null,                   // null = tự phủ kín (mặc định null)
    "clipSec": 10..600,                          // mặc định 60
    "fixedVideoIds": ["<platform>:<videoId>"]    // video lẻ, mỗi id phải tồn tại
  },
  "overlays": [ "<overlay uuid>" ],              // mặc định []
  "appearance": {                                // "Chỉnh nền"
    "opacity": 0..1,          // 1
    "blurRadius": 0..50,      // 0
    "brightness": -1..1,      // 0
    "contrast": 0..3,         // 1
    "saturation": 0..3,       // 1
    "hueShift": 0..1,         // 0
    "speed": 0.25..3,         // 1
    "zoom": 1..3,             // 1
    "flipX": false, "flipY": false
  },
  "output": { "compressExport": true }
}
```

Ràng buộc tham chiếu (SQL đọc bảng của mảng khác, **chỉ đọc**):
`scenePromptId` phải tồn tại và `kind="scene"`; `coverPromptId` phải tồn tại và
`kind="cover"`; mỗi `overlays[i]` phải còn trong thư viện; `backdrop.channels[].channelId`
phải có trong `backdrop_channels`; `backdrop.channels[].pinned` phải có trong
`backdrop_channel_videos` của đúng kênh; `backdrop.fixedVideoIds` phải có trong
`backdrop_videos`. `kinds` theo nền tảng: twitch `clip|highlight|vod`, bilibili
`video`.

Lỗi trường trả đường dẫn JSON: `scenes.style`, `scenes.framing`,
`scenes.images.mode|count|minutes`, `scenes.motion.kind|intensity|speed`,
`scenes.coverSource`, `scenes.referenceStyle`, `scenes.sceneOpacity`,
`scenes.scenePromptId`, `scenes.coverPromptId`, `overlays[i]`,
`backdrop.channels`, `backdrop.channels[i].channelId|kinds|pinned`,
`backdrop.clipCount`, `backdrop.clipSec`, `backdrop.fixedVideoIds[i]`,
`appearance.opacity|blurRadius|brightness|contrast|saturation|hueShift|speed|zoom`.

### `SnapshotTemplate` (cho job start, L9)

```jsonc
{
  "templateId","templateName","revision",
  "style","framing?",
  "scenePrompt": {id,name,kind,builtIn,composition?,prompt},
  "coverPrompt": { … },
  "imagePrompt": "<nội dung prompt cảnh>",
  "imagePromptMode": "blend" | "custom",   // blend = prompt có sẵn (house style kênh)
  "thumbnail": "clean|hook|circle-arrow|hook-arrow|split|closeup",
  "coverSource","coverMediaId?","referenceImages","referenceStyle",
  "images": {mode,count?,minutes?}, "motion": {kind,intensity,speed}, "sceneOpacity",
  "overlays": [{id,name,filePath}],         // bỏ lớp đã tombstone
  "backdrop": null | {…}, "appearance": {…}, "compressExport": bool
}
```

## 4. CLI

```text
viderse template list                                  # {"templates":[TemplateSummary]}
viderse template show <id>                             # TemplateDetail
viderse template create <tên>                          # {"template":TemplateDetail,"templateId":id}
viderse template duplicate <id> [tên]                  # như trên
viderse template rename <id> <tên> [--revision N]      # {"template":TemplateDetail}
viderse template set <id> --revision N '<json>'        # hoặc --recipe-file f / --name tên
viderse template delete <id>                           # {"removed":"<id>"}

viderse overlay list                                   # {"overlays":[Overlay]}
viderse overlay add <tệp>                              # {"overlay":Overlay}
viderse overlay remove <id>                            # {"removed":"<id>","overlays":[…]}
```

Cùng đường kiểm với HTTP; lỗi JSON `{error,code,recovery,fields,channels,usedBy}`
ra **stderr** + exit 1; `--help` in usage ra stdout, exit 0; thiếu lệnh con =
exit 1 (vỏ JSON, recovery chứa usage). CLI mở đúng thư mục dự án đang chọn
(`settings.storage.location`, mặc định `<config>/projects`) nên `library/overlays/…`
khớp với serve.

## 5. Test

| Test | Nguồn port / kiểm gì |
|---|---|
| `TestDefaultRecipeMatchesYtFactory`, `TestStandardRecipeIsCutoutWithBuiltinPrompts` | `template-recipe.ts` DEFAULT_RECIPE + Mẫu chuẩn của kênh mới |
| `TestValidateRecipeLimits` (37 ca), `TestValidateRecipeBoundariesPass`, `TestParseRecipe*` | `template-limits.ts`/`template-schemas.ts`: mọi biên, field lạ, JSON hỏng, dedupe overlays/kinds/pinned, `backdrop:null` |
| `TestCheckRefs` | prompt sai loại/không có, lớp phủ đã bỏ, kênh/video nền thiếu (6 lỗi trường) |
| `TestImageCountFor` | `recipeImageCount` (none/fixed/per-minutes, clamp 1..60) |
| `TestBuiltinPromptsShape`, `TestBuiltinAssetsMatchManifest` | 7 prompt (id = `builtin:cover-<composition>`), 12 tệp nhúng khớp sha256 manifest |
| `TestCreateTemplate…`, `TestCreateStandardTemplateName`, `TestDuplicateTemplate`, `TestPatchRevisionRaceOneWins`, `TestPatchValidatesRecipeAndRefs`, `TestRenameTemplate`, `TestDeleteTemplateBlockedWhileChannelUsesIt`, `TestPriceEstimateUsesTemplateImageCount`, `TestListSummaryPromptText` | nghiệp vụ template + giá + tóm tắt |
| `TestListPromptsBuiltinsFirstAndSearch`, `TestCreatePromptValidationAndDuplicates`, `TestPatchAndDeleteBuiltinPromptForbidden`, `TestPromptInUseBlocksKindChangeAndDelete`, `TestPromptExportImportRoundTrip`, `TestPromptUsage` | prompt CRUD, 403/409, tên duy nhất + tên có sẵn, tìm không dấu, xuất/nhập |
| `TestClassifyBackground` | `media-background.ts` (đen 85/50%, lục 70%, còn lại none) |
| `TestAddOverlayFileDetectsMetadataAndDedupes`, `TestListOverlaysNewestFirst`, `TestAddOverlayRejectsBadInput`, `TestAddOverlayWithoutFfmpegIsToolMissing`, `TestRemoveOverlayTombstonesAndRestoresOnReAdd`, `TestSeedBuiltinsIdempotentAndNeverResurrectsRemoved` | `overlay-library.ts` (dedupe theo byte, thứ tự, bỏ = tombstone) + seeding idempotent, hiệu ứng đã bỏ không quay lại |
| `TestSnapshotResolvesBuiltinAndCustomPrompts`, `TestSnapshotMissingPromptIsInvalidAndTombstonedOverlaySkipped` | `template-snapshot.ts` (imagePrompt ← prompt, mode blend/custom, thumbnail ← composition, coverPrompt) |
| `TestLibrary*` (web) | map lỗi → status (`conflict` 409 + recovery, `builtin` 403, `in_use` 409 + `usedBy`+`reason`, `invalid_input` 400 + `fields`, 404, 500 + diag), hình wire list/detail/prompt/overlay, multipart 201, export header, mix delegation |
| `TestLibraryTemplate…`, `TestLibraryPromptStore`, `TestLibraryOverlayStoreTombstone`, `TestLibraryBackdropReferenceChecks`, `TestLibraryOverlayUsageFromTemplates`, `TestLibraryPromptRefLockedOnTemplateWrite`, `TestLibraryBackdropRefLockedOnTemplateWrite` (Postgres) | CAS thật, hai writer một thắng, FK chặn xoá (23503 **và** 23001 → `in_use`), unique tên `lower(name)`, tombstone, `fold_title` tìm không dấu, kiểm kênh/video nền, usage từ recipe, khoá tham chiếu prompt (FOR UPDATE) + khoá advisory kênh/video nền chống đua xoá/đổi loại |
| `TestPatchBadBackdropKeepsRecipeAndRevision`, `TestUpdateTemplateRejectsMissingOrKindMismatchPrompt`, `TestDeletePromptBlockedByTemplateAtStore`, `TestGetMalformedIDIsNotFound`, `TestReAddRepairsMissingFile`, `TestSeedBuiltinsConcurrentKeepsFiles`, `TestImageCountForCeilBoundaries` | regression review: backdrop sai → 400 + recipe/revision nguyên; đua xoá/đổi loại prompt; id xấu → 404; tải lại phục hồi tệp mất; seed đua không xoá tệp; ceil per-minutes |

Chạy: `go test ./internal/library ./internal/web`; Postgres cần
`TEST_DATABASE_URL`. Test Postgres tự provision schema riêng nên chạy song song
với suite khác được.

Chưa đo p95 (chưa có serve + dữ liệu thật); GET list là 3 truy vấn
(templates + gộp kênh bằng LATERAL + prompts cho tên), GET detail 3–4 truy vấn.

## 6. Lệch so với design / yt-factory

1. **Số ảnh khi tính giá** dùng số của template (không dùng hằng 7) — lý do ở
   §1.3; cần controller ghi vào `11-hoi-dap`.
2. **Tên hiển thị của 12 hiệu ứng** theo artboard Library (`OV`: "Tuyết", "Mưa",
   "Bụi", "Phim cũ", "Loé sáng", "Bokeh", "Tàn lửa", "Sương mù", "Đom đóm",
   "Lá rơi", "Hoa anh đào", "Sao") — controller đổi trong
   `assets/overlays/manifest.json` sau báo cáo (artboard đứng trên tên của pack
   script yt-factory).
3. **Prompt nhập trùng tên** không bị chặn mà tự đổi thành `"<tên> (nhập)"` —
   design không nói; chọn để nhập cùng tệp hai lần vẫn được.
4. **Hiệu ứng có sẵn không bỏ được** (sửa sau báo cáo, theo artboard Library
   dòng 1204): `DELETE /api/overlays/{id}` trên hiệu ứng có sẵn trả 403
   `builtin`; tombstone chỉ còn cho lớp phủ tự thêm.
5. **`samples` luôn rỗng** ở đợt này (cần video đã xong để vẽ mẫu) — đúng
   non-goal, web hiện "Chưa có mẫu".
6. **`preview` chỉ đọc** (`preview_revision`/`preview_built_at`); chưa có route
   dựng bản xem thử (non-goal đợt này).
7. **Khoá `appearance`** (không phải `backgroundAppearance` như yt-factory):
   theo gợi ý hợp đồng §3 "appearance/output… do library định nghĩa"; đã báo
   mảng Video nền đọc `recipe.appearance.speed`.
8. **`coverMediaId`** không kiểm tồn tại: đợt này chưa có kho media cho ảnh bìa
   tải lên; trường được giữ để đợt editor điền.
9. **Rename không tăng revision** (yt-factory coi mọi lần lưu là làm cũ bản xem
   thử); tên không ảnh hưởng bản dựng nên giữ nguyên revision, tránh làm bản
   xem thử "cũ" oan.
10. **`code` thay vì `reason`** cho 409 in_use: trả **cả hai** (`code:"in_use"`,
    `reason:"in_use"`) để hợp đồng §5 và phần còn lại của API cùng đúng.
11. **`PATCH /api/templates/{id}` nhận `name`** để một route lo cả đổi tên (hợp
    đồng chỉ có PATCH); CLI tách lệnh `rename`.
12. **Hook `Options.AllowNonJSON` trong `platform/webapp`**: thay đổi ngoài
    mảng (1 điều kiện + 1 trường Options) vì guard JSON dùng chung chặn
    multipart; chỉ mở cho đúng 2 route tải tệp khi `Content-Type` là
    `multipart/form-data`.
13. **Đọc chéo bảng**: `owner_channel_templates`, `backdrop_channels`,
    `backdrop_channel_videos`, `backdrop_videos` — chỉ `SELECT`, đúng ranh giới
    hợp đồng.
14. **`name` của overlay** bỏ đuôi tệp khi `name` không được gửi (multipart
    `file.filename` chỉ quyết định đuôi/MIME). Trước review `name` bị lấy làm
    nguồn đuôi — đã tách hai tham số (review A4).
15. **Store lock**: `UpdateTemplate`/`UpdatePrompt`/`DeletePrompt` mở
    transaction và `SELECT … FOR UPDATE` hàng prompt recipe trỏ tới, để xoá/
    đổi loại prompt không đua tạo tham chiếu hỏng (review A2); `memStore`
    test mô phỏng cùng ngữ nghĩa dưới mutex. Cùng transaction đó còn giữ
    **advisory lock dùng chung** cho kênh/video nền recipe trỏ tới
    (`pg_advisory_xact_lock(hashtextextended(key,0))`, key
    `backdrop_channel:<id>`/`backdrop_video:<id>` — chuỗi lạnh, trùng
    `backdrop.ChannelLockKey`/`VideoLockKey`), rồi kiểm tồn tại dưới khoá:
    xoá bên mảng Video nền không thể đua "kiểm rồi ghi" (yêu cầu của
    BackdropBackend; `ErrBackdropRef` → web kiểm lại refs và trả
    `invalid_fields` đúng trường).
16. **Sniff nội dung** thay vì tin đuôi tên + bắt buộc stream hình trong
    ffprobe (review A3); lỗi `bad_media` không còn chứa đường dẫn tạm tuyệt
    đối.

## 7. Việc còn lại (đợt sau)

- Dựng bản xem thử + trạng thái `fresh/stale` ghi thật (route preview).
- `samples` của prompt ảnh: vẽ 1 ảnh mẫu từ video đã xong.
- `coverMediaId` gắn kho media ảnh bìa; `POST /api/templates` chọn nền tảng
  soạn bìa.
- Web UI (đợt sau) đọc §2; artboard cần chốt tên ngắn của 12 hiệu ứng (§6.2).
- Đo p95 GET sau khi có serve + dữ liệu mẫu.

Status: DONE
Backend Thư viện xong: recipe + validator dùng chung, template CAS revision,
7 prompt có sẵn + CRUD + xuất/nhập, 12 lớp phủ nhúng + upload + tombstone +
seeding, snapshot cho job start, đủ route/CLI/SSE/test; 4 điểm cần controller
chốt nằm ở §6.
