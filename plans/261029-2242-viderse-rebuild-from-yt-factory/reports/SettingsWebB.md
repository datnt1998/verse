# SettingsWebB — Settings › Kết nối AI · Trợ lý AI · Tài khoản nền tảng

Phạm vi V17, V23 (phần accounts), V24 + variants mobile của `Settings`/`MSettings`.

## Đã làm

**API client (mới)**
- `web/src/api/ai.ts` — `AiProvider`/`AiModel`/`AiLogin`/`AiAgentSettings` theo §15 + wire AIConn xác nhận qua IRC: `GET/POST /api/ai/providers`, `PATCH/DELETE :id`, `POST :id/test`, `GET :id/models?q&filter`, `POST :id/models/check`, `GET/PUT /api/ai/agents`, login `POST :id/login` (202), `GET :id/login` (phase `idle|authorize|code|exchanging|done|failed|cancelled`, `authUrl|code|verifyUrl|account`), `POST :id/login/cancel`, `POST :id/login/code`, `POST :id/logout`.
- `web/src/api/platforms.ts` — `Platform`/`PlatformLogin`/`QrPayload`, `GET /api/platforms`, `GET /api/platforms/youtube/login/profiles`, `PUT :id/login`, `POST :id/login/refresh`, `POST/GET :id/login/qr`, `DELETE :id/login` (§24/§24.1 + xác nhận Platforms).
- `web/src/api/request.ts` — thêm `mutate(method)` chung (POST/PATCH/PUT/DELETE), cùng hợp đồng lỗi `ApiError` (status 0 = offline, payload giữ `reason`).

**Kết nối AI** (`screens/settings/connect/`)
- `ConnectSection.tsx` — danh sách thẻ, thẻ thêm (desktop) / sheet (mobile), thanh dính "N thay đổi chưa lưu · Huỷ · Lưu" → PATCH `selected` từng provider đổi; provider mới lên đầu + nhắc chọn model + catalog mở sẵn.
- `ProviderCard.tsx` — tag năng lực (hợp nhất caps model đã chọn, rỗng → theo loại), plan/quota ngày + `resetAt`, Model · N, Thử kết nối (`/test`, hiện ms), Đăng xuất/Đổi khoá/Đổi địa chỉ theo `auth`, Chi tiết mở `lastError`; thẻ cam khi `status=failed` (Đổi khoá · Thử lại · Thêm theo tên); `409 in_use` → câu riêng; subscription: `logged_out` → nút đăng nhập, vùng `LoginFlow` poll `GET login` 2 s (mở `authUrl` một lần, hiện mã + `verifyUrl` cho grok, "Mở trang xAI" là nút primary duy nhất, dán mã cho máy khác).
- `ModelCatalog.tsx` — tìm debounce 250 ms, Đã chọn/Gợi ý/Tất cả (server-side `q`/`filter`), tick chọn, context/giá, render tối đa 200 hàng + "Hiện thêm", "Thêm theo tên" qua `models/check`.
- `AddProvider.tsx` — 8 loại, ô khoá che / ô địa chỉ theo loại, "Kết nối và lấy model".
- `ChatGPTLogin.tsx` — `ChatGPTLoginAction` (`variant: primary|ghost`, `onDone?`), poll 2 s, `CHATGPT_PROVIDER_ID = 'chatgpt'`.

**Trợ lý AI** (`screens/settings/agents/`)
- `AgentsSection.tsx` — Trinh sát/Kiểm định "Không dùng model"; Đạo diễn: picker + effort (Tự động/Thấp/Vừa/Cao) + chuỗi dự phòng có thứ tự; Hoạ sĩ: model viết prompt (needs vision), lưới "Vẽ ảnh bằng", switch "Vẽ lại bằng nguồn khác khi lỗi" + dự phòng; auto-save PUT; 422 `capability_missing {field,need}` → câu inline dưới đúng trường.
- `ModelPicker.tsx` — popover 400 px (`usePopup`), nhóm theo provider, tên + id + tag; model thiếu năng lực `disabled` + lý do ("Không đọc được ảnh"/"không vẽ được ảnh"/"không viết được chữ"); mobile dùng cùng `ModelList` trong `Sheet`.

**Tài khoản nền tảng** (`screens/settings/accounts/`)
- `AccountsSection.tsx` — thẻ từ `GET /api/platforms` (name/use/quality/usage+link/state Sẵn sàng · Tạm chặn tới HH:MM · Cần cookie), YouTube chọn browser+profile từ `login/profiles` → PUT, banner bot + Thử lại → `POST login/refresh`, Twitch "không cần đăng nhập", Bilibili: QR · "Dán cookie thủ công" (gập, kiểm SESSDATA trước khi PUT, 401 → câu riêng) · blocked banner + pausedUntil, hàng "Nền tảng khác" → `#/library`.
- `QrLogin.tsx` — `Dialog` desktop / `Sheet` mobile, poll 2 s **chỉ khi mở** (cleanup dừng), đếm ngược 1 s từ `expiresAt` (hết giờ phía client → expired), 3 bước có tick, overlay scanned/confirmed/expired, banner network/busy/failed, "Mở app" (mobile) mở `loginUrl`, một nút primary mỗi trạng thái; xong → toast `Đã đăng nhập Bilibili: <account>` + invalidate platforms.

**Shared**
- `data/keys.ts`: `aiProviders`, `aiProviderModels(id,q,filter)`, `aiAgents`, `platforms`, `platformProfiles`.
- `data/eventKeys.ts`: `SERVER_EVENT_TYPES` + `ai` → `['ai']`, `platforms` → `['platforms']`.
- `i18n/ai.ts`, `i18n/accounts.ts` + 2 dòng trong `vi.ts`; `common.copy`, `common.cancel` giữ nguyên.
- `screens/settings/settings-ai.css` — CSS riêng, tiền tố `sai-`, token kit.

## Kiến trúc & hiệu năng
- Catalog chỉ render ≤ 200 hàng/lần + "Hiện thêm"; tìm server-side có debounce 250 ms → không mount hàng trăm DOM row (OpenRouter 400+).
- Đếm ngược QR: interval 1 s chỉ khi `pending|scanned`; poll QR/login: interval 2 s chỉ khi dialog/thẻ đang mở, cleanup dừng hẳn; `confirmed/expired/failed` clear ngay.
- Không thêm timer dự phòng: dữ liệu đi qua `useResource` + SSE invalidate theo prefix (`['ai']`, `['platforms']`).
- Catalog của picker trợ lý tải lười 1 lần/provider có model đã chọn (song song, cache state), không N+1 theo model.
- Đo được: 12/12 test pass trong 3.5 s; `tsc --noEmit` sạch cho toàn bộ file tôi sở hữu.

## Kiểm chứng
- `npx tsc --noEmit -p tsconfig.json` → sạch cho `screens/settings/{connect,agents,accounts}/**`, `api/{ai,platforms,request}.ts`, `i18n/*`.
- `npx vitest run src/screens/settings/{connect,agents,accounts}/*.test.tsx --maxWorkers=2` → **12 passed** (3 file).
  - V17: thêm OpenRouter → tick 2 model → thanh chưa lưu → Lưu → PATCH `{selected:[…]}`; key sai → thẻ cam + 3 hành động; grok logged_out → nút đăng nhập.
  - Trợ lý: picker chỉ có model đã chọn; model thiếu vision bị disable + lý do; PUT fallback; 422 → `.sai-caperr` inline.
  - Nền tảng: 3 thẻ + trạng thái + usage; profiles; countdown `[23]:dd`; poll dừng sau Đóng; confirmed → chỉ "Xong"; dán cookie thiếu SESSDATA không PUT; paused → "Tạm chặn tới HH:MM" + banner.
- `vite build`: **FAIL do file của người khác** — `src/screens/setup/SetupScreen.tsx:87` thiếu `)` (`Expected ")" but found "const"`). Đã báo SettingsWebA; không sửa file của họ.

## Lệch thiết kế / chưa làm
- Chưa chụp ảnh so artboard: backend chưa mount `/api/ai/*` (`internal/web/ai.go` chưa có) nên không dựng được `viderse serve` có dữ liệu thật cho Kết nối/Trợ lý; kiểm tra bằng DOM state trong test thay thế. Cần chạy lại bước ảnh khi AIConn mount xong (desktop + `MSettings`).
- `AiModel` không có trường `caps` cấp provider trong §15 → tag năng lực của thẻ suy ra từ caps model đã chọn, fallback theo loại.
- Quota chỉ có tín hiệu ngày (AIConn: `{used,limit,period:'day',resetAt}`); `spend` tháng đã bỏ khỏi UI (server không có).
- Ảnh QR: ưu tiên `svg` server trả; nếu chỉ có `loginUrl` thì client vẽ bằng `qrMatrix` của kit.

Status: DONE_WITH_CONCERNS
Summary: Ba mục Cài đặt (Kết nối AI, Trợ lý AI, Tài khoản nền tảng + QR Bilibili) đã chạy thật với client API, i18n riêng, keys/SSE và 12 test vitest xanh; typecheck sạch cho toàn bộ file tôi sở hữu.
Concerns: `vite build` đang hỏng vì lỗi cú pháp trong `screens/setup/SetupScreen.tsx` (SettingsWebA, đã báo); ảnh so artboard chưa chụp được vì `/api/ai/*` chưa mount ở backend.

## Follow-up (integrator items 1–3)

**1. Design guard (src/checks) — green.** Mọi khoảng cách/bo góc/màu trong file tôi sở hữu giờ đi qua token:
- `settings-ai.css`: `gap|padding|margin` → `var(--vv-s-N)`, `border-radius` → `var(--vv-r-N)`/`--vv-r-pill`, hex → `--vv-qr-plate|ink|ok`, `--vv-skel-a|b`.
- `viderse.css` (SettingsWebA cho phép khi thêm bước mới): thêm `--vv-r-8|9|10|12` và bộ màu QR/skeleton.
- JSX inline (`gap`, `paddingTop`, `borderRadius`) → giá trị `var(--…)`; không còn số px rời.

**2. Hiệu năng (có test).**
- `ModelCatalog` render tối đa `PAGE = 200` hàng, "Xem thêm (N)" mở tiếp; tìm server-side `q` + `filter=selected|recommended` debounce 250 ms. Test 500 model: 200 hàng → `200 / 500 model` → bấm Xem thêm = 400 hàng.
- QR: đúng **một** `setInterval(2000)` sau khi phiên có `qrcodeKey`; test khẳng định 1 request/2 s, và sau "Đóng" + 8 s không tăng request. `LoginFlow`/`ChatGPTLoginAction` poll 2 s và clear khi `done|failed|cancelled` hoặc unmount.

**3. Đối chiếu endpoint với `internal/web/ai.go` + `internal/web/platforms.go` — đã sửa hết lệch.**
| Client trước | Backend thật | Xử lý |
|---|---|---|
| provider `.name` | `.label` | đổi toàn bộ UI + test |
| `status ok/failed` | `ready/logged_out/login_pending/auth_failed/unreachable` | thẻ cam = `auth_failed|unreachable` |
| `auth key/url` | `api_key/local/subscription` | Đổi khoá/Địa chỉ/Đăng xuất |
| `quota {used,limit,resetsIn}`, `spend`, `plan/account` | `quota {used,limit,period,resetAt}`, `catalogAt/catalogCount`, `keyHint` | bỏ plan/spend; hiện `keyHint` + hạn mức ngày; "Chi tiết" = `catalogAt` |
| model `{name,caps{...},suggested,priceIn/Out/Img}` | `{id,caps:[text|vision|image],contextK,price{inPerM,outPerM,perImage},recommended,manual}` | đổi type + hiển thị (id làm tên) |
| filter `suggested` | `recommended` | đổi nhãn gọi API |
| agents `{artist.promptModel,drawProvider,drawFallback,retryOnOtherSource}` | `{artist.promptModel,draw,fallbackOn,drawFallback}` + `ModelRef{provider,model}` | viết lại AgentsSection/ModelPicker theo ModelRef |
| platforms `{name,use,quality,usage.text,login{hint,browser,profile,method},lastError:string}` | `{label,uses[],kinds[],targetHeight,usage{channels,videos,rivals},login{type,saved,masked,account,browserProfile,method,renewable},lastError{code,at}}` | đổi mapping + copy usage/chất lượng |
| Youtube retry `POST login/refresh` | `POST login/check` | dùng `check` cho "Thử lại" |
| QR `POST → {svg,status}`, `GET ?` (không key) | `POST → {qrcodeKey,qrImage,loginUrl,expiresAt}`, `GET /qr/:key → {status:waiting|scanned|expired|success|rejected,account?}` | poll theo key, ảnh `qrImage` (data URL) |
| `POST /providers` 401 không có id | body có `{id,keyHint}` | 401 → đưa thẳng thẻ cam lên đầu |

Không còn lệch nào giữa `api/ai.ts`/`api/platforms.ts` và hai handler; đã báo Platforms/AIConn trong lúc làm.

**Kiểm chứng lại (heavy lock):**
- `npx vitest run src/checks` → **6 passed**.
- `npx vitest run src/screens/settings --maxWorkers=2` → **7 files / 38 tests passed** (14 của tôi + các file khác trong thư mục).
- `npx tsc --noEmit` → sạch cho toàn bộ file tôi sở hữu.
- `npx vite build` → **✓ built in 1.77s**.

**4. Visual check — chưa hoàn tất.** Đã dựng `viderse serve` trên 18796 với DB tạm (`viderse_smoke_settingsb`, migrate 00001→00004 OK) và vite 5192 proxy `VITE_API_TARGET=18796`; API trả 401 (chưa có tài khoản) nên cần đi qua luồng tạo tài khoản trong UI. Trình duyệt managed của môi trường này không mở được `127.0.0.1:5192` (goto timeout, screenshot timeout) nên chưa chụp được ảnh so artboard. Đã dọn: dừng serve + vite, `dropdb viderse_smoke_settingsb` OK. `/tmp/viderse-settingsB` còn lại `home/go/pkg/mod` do mod cache của toolchain thuộc user khác (chỉ đọc) — vô hại, cần `sudo` mới xoá được.
Việc còn lại: chụp `Settings` + `MSettings` (connect/agents/accounts, kèm `qr=`/`conn=`/`ai=` variants) khi có phiên đăng nhập, đối chiếu artboard và ghi khác biệt.

## Review fixes (Review-p2-web + Review-p2-secrets)

Mọi sửa có regression test đi kèm; wire mới theo đúng Go handler thật.

| Finding | Sửa | Regression test |
|---|---|---|
| web H4 — save trợ lý đè nhau | `AgentsSection`: mọi mutation ghi vào MỘT `pending` draft rồi `flush()` xả PUT nối tiếp (PUT2 gửi doc mới nhất, không chồng request, không đọc snapshot cũ); hint hiện Đang lưu/lỗi thay vì "đã áp dụng" tĩnh | PUT đầu bị giữ + thay đổi thứ hai → đúng 2 PUT, PUT2 chứa `effort:low` + fallback mới |
| web H5 — bịa caps model đã chọn | `pickedFrom` → `caps: string[] \| undefined` (chưa tải = không biết, KHÔNG bịa 'text'; `blockedReason` không chặn khi chưa rõ). Metadata lấy từ `filter=selected` qua `listSelectedModels` (đi hết mọi trang, limit 500). `draftOf`/`toggleModel` ở ConnectSection cũng ngừng bịa `['text']` | `pickedFrom(catalogOf→undefined)` → caps undefined, không dim; nano-banana giữ `['image']` thật (dim cho vision, có mặt trong lưới Vẽ ảnh) |
| web H6 — xoá provider dán nhãn Đăng xuất | Nút "Gỡ kết nối" riêng cho api_key/local → `confirm(tone:'warn')` (Gỡ X? · mất khoá/địa chỉ + model đã chọn) trước `DELETE`; subscription giữ "Đăng xuất" | Click Gỡ → không DELETE; Huỷ giữ nguyên; Gỡ → đúng 1 DELETE |
| web H7 — authUrl mở ngoài gesture | `startLogin`/`ChatGPTLoginAction` mở cửa sổ trắng NGAY trong click rồi LoginFlow điều hướng tới authUrl; link `sai-authlink` "Mở trang đăng nhập" luôn hiện khi phase=authorize (fallback popup bị chặn) | `window.open→null` vẫn gọi 1 lần trong click + link href đúng authUrl |
| web M1 — field lỗi không khớp Go | Error map theo đúng `director.model`, `director.fallback`, `artist.promptModel`, `artist.draw`, `artist.drawFallback` | `it.each` 5 payload 422 thật → đúng `.sai-caperr` kề trường |
| web M3 — poll chồng/không hết hạn | `poll.ts` `sequentialPoll` mới (setTimeout sau mỗi request, AbortController; QR dừng theo `expiresAt` máy, 5 lỗi liên tiếp → trạng thái network + dừng). Áp cho QrLogin, LoginFlow, ChatGPTLoginAction | Treo 1 poll 3 nhịp → vẫn 1 request; quá expiresAt lúc treo → "Mã hết hạn" + dừng hẳn; đóng dialog → 0 request thêm |
| web M4 — fetch catalog trong render | Bỏ `useMemo`-fetch + Map riêng: `SelectedCatalog` con per-provider `useResource(['ai',providers,id,'selected'], listSelectedModels)` — fetch trong effect của cache dùng chung, abort qua signal, SSE 'ai' invalidate đúng key, lỗi không bị cache thành [] | Test filter=selected trả caps thật — thấy trong bài "catalog filter=selected" |
| web M5 — bỏ qua renewable/refreshError | Bilibili: `renewable` → "Gia hạn ngay" (primary) → POST login/refresh + ghost "Quét mã QR"; `refreshError` → trạng thái "Hết hạn" + banner cam (giờ từ lastRefreshAt, không lộ lỗi thô) + "Quét lại"; cookie dán tay → "Không tự gia hạn · quét mã để bật". "Xoá đăng nhập" → cam xác nhận → DELETE. YouTube: select phẳng ["Tắt", "Chrome · Kênh chính", …], Tắt → cam xác nhận → DELETE login | 4 test: renewable/refresh/manual/expired + clear qua confirm |
| web M7 — enum thô ra UI + nhiều primary | `uses`/`kinds` map sang nhãn Việt ("Video đối thủ", "Video nền: clip, highlight, phát lại", "video đã đăng"); meta `Tải 1080p · 1 kênh, 2 video lẻ`; refreshError không in chuỗi lỗi. Bilibili chỉ một primary theo ngữ cảnh (Gia hạn XOR Quét mã QR); YouTube Lưu chỉ khi đổi | Test assert không chứa 'competitor'/'refresh failed', một nút chính |
| web M8 — thiếu tên/keyboard | Switch `label`, Field/PasswordField `aria-label` khắp nơi; `ModelPicker` là listbox thật: focus vào box, Arrow/Home/End bỏ qua mục mờ, Enter/Space chọn, Esc đóng + trả focus nút, `aria-activedescendant`/`aria-disabled`/`aria-selected` | Test role=switch có `aria-label` đúng; option mờ `disabled`+`aria-disabled` |
| secrets H5 — openai_compat thiếu trường | `AddKind.auth:'both'` → hiện + gửi CẢ `apiKey` và `baseUrl`; Ollama vẫn URL-only | Test assert POST body `{kind,apiKey,baseUrl}` |
| secrets M5 — cắt trang giấu model | `listModels` → `{models,total,nextCursor}` (`cursor` keyset id cuối trang); ModelCatalog hiển thị đúng MỘT trang ≤200 hàng với Trang trước/sau + tổng thật; chọn giữ qua mọi trang | 500 model: 200/500 hiển thị, Trang sau → model-200, Trang trước về; tick trang 1 sống qua điều hướng |

Giữ lại: `POST /providers/:id/login/code` (paste-code ChatGPT) — AIConn xác nhận route ở lại.

### Wire notes
- Models: `GET /api/ai/providers/:id/models?q&filter=all|selected|recommended&limit≤500&cursor` → `{models:[{id,caps[],contextK,price{inPerM,outPerM,perImage},recommended,manual}],total,nextCursor}`.
- `filter=selected` trả metadata model đã chọn (để Trợ lý AI không đoán caps).
- Platforms login: `{type,saved,masked,account,browserProfile,method,renewable,lastRefreshAt,refreshError}`; `POST login/refresh` gia hạn QR; `DELETE login` xoá (cam xác nhận trước).
- `GET .../login` provider: `{phase,authUrl,code,verifyUrl,account,error}` — poll tuần tự 2 s, cửa sổ giữ chỗ trong gesture.

### Verification (heavy lock)
- `npx tsc --noEmit` → sạch.
- `npx vitest run src/screens/settings src/checks --maxWorkers=2` → **8 files / 67 tests passed** (gồm design-rules 6).
- `npx vite build` → **✓ built in 1.78s**.

### Lệch/giới hạn còn lại
- `ModelPicker` dùng `aria-activedescendant` (không roving tabindex) — đúng pattern ARIA cho listbox; mobile sheet vẫn list nút thường.
- `ModelPage` đọc `nextCursor` — back-nav dùng mảng cursor client (server chỉ cho nextCursor; không có previousCursor).
- Visual check (bước 4) vẫn chưa làm: trình duyệt managed của môi trường không mở được 127.0.0.1:5192 (goto/screenshot timeout); serve 18796 + DB tạm đã chạy được, cần luồng tạo tài khoản để qua 401 — chờ hướng dẫn bypass hoặc làm lại khi phiên tiếp theo.
