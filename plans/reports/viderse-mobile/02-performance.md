# 02 — Hiệu năng điện thoại: Viderse ↔ Noverse

Phạm vi: `apps/viderse/web` (bản chạy thật `https://vid.aarverse.studio`) so với `apps/noverse/web`
(`https://nov.aarverse.studio`), khung nhìn điện thoại 390×844. Đây là **báo cáo đo + phân tích nguyên
nhân**, không sửa mã sản phẩm.

Mọi con số dưới đây lấy từ CDP thật (Chrome do `agent-browser` mở), tài khoản SSO dùng chung
`qaaudit`, chế độ chỉ đọc (không tạo/sửa/xếp hàng/duyệt bất cứ thứ gì trên production).

---

## 1. Phương pháp đo

| Hạng mục | Cách làm |
|---|---|
| Khung nhìn | `Emulation.setDeviceMetricsOverride` 390×844, `deviceScaleFactor 3`, `mobile: true`, touch 5 điểm |
| CPU | `Emulation.setCPUThrottlingRate { rate: 4 }` |
| Mạng | `Network.emulateNetworkConditions` — latency 40 ms, 9 Mbps xuống / 1.5 Mbps lên, `cellular4g` (preset "Fast 4G" của DevTools) |
| Cache | Mỗi lần "cold": `Storage.clearDataForOrigin { service_workers, cache_storage }` + `Network.setCacheDisabled true` |
| "Warm" (lần mở lại) | Giữ service worker + HTTP cache, vẫn 4x CPU + Fast 4G |
| Số liệu paint | `PerformanceObserver` (buffered) cho `paint`, `largest-contentful-paint`, `layout-shift`, `longtask`, `event` — chèn bằng `Page.addScriptToEvaluateOnNewDocument` trước khi điều hướng |
| TBT | Tổng `max(0, duration − 50)` của mọi long task ≥ 50 ms (đúng định nghĩa Lighthouse) |
| Request/byte | Sự kiện `Network.*` (`encodedDataLength` = byte trên dây, đã nén) |
| Main-thread | `Performance.getMetrics` → `TaskDuration`, `ScriptDuration`, `LayoutDuration`, `RecalcStyleDuration`, `JSHeapUsedSize`, `Nodes`, `JSEventListeners` |
| Tương tác | `Input.dispatchMouseEvent` (chuột thật, không `.click()` trong trang) + `PerformanceObserver` type `event` (đo input→next-paint, tức INP) + một vòng `requestAnimationFrame` ghi lại khung hình để biết **khi nào màn mới thực sự đổi** (`firstChange`) |
| Cuộn | `Input.synthesizeScrollGesture` (cử chỉ chạm) + ghi rAF 3 s → fps, p50/p95/max frame, số khung > 33 ms và > 50 ms, `scrollTop` thực tế đã đi |
| Số lần | Cold: 3 lần/app (lấy trung vị). Warm: 3 lần/app. Tương tác: 3 mẫu/loại. Cuộn: 3 lượt |
| Bản dựng | Ghi lại hash entry của **từng lần chạy** (hash ghi ở tiêu đề mỗi bảng số liệu) |

Công cụ: `agent-browser --session MPerf` + script CDP tự viết (`/tmp/vperf/harness.mjs`). Máy đo:
macOS arm64, 10 nhân.

### 1.1 Điều kiện đo — đọc trước khi so số tuyệt đối

1. **Máy đo bị tải nặng trong suốt phiên**: `uptime` cho `load average ≈ 19–20` trên 10 nhân (5 agent
   khác cùng chạy trình duyệt). Vì vậy mọi con số thời gian tuyệt đối đều **bị thổi phồng** và dao
   động mạnh; chỉ số **ổn định và dùng được để so sánh** là byte, số request, TBT, long task, heap,
   số node DOM.
2. **Origin/CDN dao động 5–10 lần giữa các lần chạy.** Cùng thiết lập throttle, Viderse: 22:31
   (`index-BWVbLy0i.js`) FCP 7.9–11.6 s; 23:00 (`index-D0P3TuAj.js`) 2.0–3.3 s; 23:10 (vẫn
   `D0P3TuAj`) 5.1–15.5 s. `curl` từ máy đo: TTFB tài liệu 0.33–0.94 s, tải entry 0.35–1.23 s ⇒
   **độ trễ máy chủ/đường hầm lớn hơn nhiều so với ngân sách JS**. Vì thế bảng dưới đây tách "cửa sổ
   đo" và nhấn vào phần do client gây ra.
3. Production **được deploy lại giữa phiên đo** (3 bản Viderse: `index-BgifezqK.js` 22:10 →
   `index-BWVbLy0i.js` 22:22 → `index-D0P3TuAj.js` 22:35; Noverse: `index-uAcTWqxe.js`). Mỗi dòng
   trong bảng ghi rõ hash.
4. Dữ liệu production của tài khoản `qaaudit` **gần như trống**: 0 kênh, 0 video, 0 job trong hàng
   chờ, 0 mục cần duyệt; Noverse có 2 truyện nháp 0 chương. Vì vậy "danh sách dài" chỉ đo được ở
   **Khám phá** (34 video thật + thumbnail YouTube) của Viderse; các màn Bàn dựng/Duyệt/Hàng chờ/Thư
   viện của Viderse và danh sách chương của Noverse **không có dữ liệu để cuộn** → ghi nhận là
   *không quan sát được*, không suy đoán.

---

## 2. Số liệu

### 2.1 Cold load — cửa sổ A (22:31–22:38, cùng một cửa sổ cho cả hai app)

| Chỉ số | **Viderse** `index-BWVbLy0i.js` | **Noverse** `index-uAcTWqxe.js` |
|---|---|---|
| FCP (trung vị) | **10 644 ms** (7 884 / 10 644 / 11 636) | **16 480 ms** (16 380 / 16 480 / 24 528) |
| LCP | 13 224 ms | 16 480 ms |
| TBT | **540 ms** | 645 ms |
| Long task (số / dài nhất) | 5 / 417 ms | 4 / 418 ms |
| Tổng byte trên dây | **569 kB** | **1 021 kB** |
| JS (byte nén) | **293.5 kB** / 8 file | **744.3 kB** / 2 file |
| JS đã **parse** (decoded) | **908 kB** | **2 547 kB** |
| Tổng byte đã parse (mọi loại) | 1 469 kB | 3 193 kB |
| CSS (byte nén) | 81.4 kB / 1 file | 88.8 kB / 1 file |
| Font (byte nén) | 146.5 kB / **9 file** | 118.7 kB / **9 file** |
| Ảnh | 23.4 kB / 8 | 14.8 kB / 6 |
| Request | **61** | **36** |
| `TaskDuration` (main thread) | 3 175 ms | 3 357 ms |
| `ScriptDuration` | 1 136 ms | 944 ms |
| `LayoutDuration` / `RecalcStyle` | 352 / 98 ms | 238 / 152 ms |
| DOM node / listener | 127 / 204 | 80 / 214 |
| JS heap | 4.95 MB | **15.24 MB** |
| Request khi rảnh (10 s, tab hiện) | **3** (poll 3 s) | **0** |

Phân rã request (lần cold đầu tiên): Viderse `Document 1 · Script 8 · Stylesheet 1 · Manifest 1 ·
XHR 1 · Fetch 31 · Image 8 · Font 9 · Other 1`; Noverse `1 · 2 · 1 · 1 · 1 · 14 · 6 · 9 · 1`.
Con số `Fetch 31` của Viderse **gồm ~17 lần poll `/api/activity`** trong 60 s quan sát; số endpoint
khác nhau thực sự dùng cho màn đầu là **12** (`/api/me`, `/api/queue`, `/api/activity`,
`/api/notifications/settings`, `/api/clone?channel=all`, `/api/channels/summary` ×2, `/api/recap`,
`/api/suggestions`, `/api/status`, `/api/owner-channels`, `/api/events`).

### 2.2 Cold load — cửa sổ B (23:00, chỉ Viderse, máy chủ nhanh hơn)

| Chỉ số | Viderse `index-D0P3TuAj.js` |
|---|---|
| FCP | 2 192 ms (1 960 / 2 192 / 3 336) |
| LCP | 2 688 ms |
| TBT | 615 ms (602 / 615 / 743) |
| Long task dài nhất | 519 ms |
| JS / request / tổng byte | 293.5 kB / 64 / 570.7 kB |
| `TaskDuration` / `ScriptDuration` | 2 847 / 1 270 ms |

→ **Byte, request, TBT giống hệt cửa sổ A** (chênh < 5 %), chỉ FCP/LCP đổi 5 lần ⇒ xác nhận kết luận
ở §1.1: phần client ổn định, phần mạng/máy chủ mới là thứ dao động.

### 2.3 Lần mở lại (warm: giữ SW + HTTP cache, vẫn 4x CPU + Fast 4G)

| Chỉ số | Viderse `D0P3TuAj` | Noverse `uAcTWqxe` |
|---|---|---|
| FCP | 3 256 ms (1 864 / 3 256 / 5 720) | 2 064 ms (2 036 / 2 064 / 5 752) |
| TBT | 407 ms | **1 202 ms** (có một long task 1 556 ms) |
| Byte trên dây | 14.4 kB | 12.3 kB |
| `TaskDuration` | 3 025 ms | **7 535 ms** |
| JS heap | 4.92 MB | **18.93 MB** |

Noverse mở lại nhanh hơn về khung hình đầu nhưng **đốt main thread gấp 2.5 lần** vì mỗi lần mở lại
vẫn phải nạp/parse 2.5 MB JS (service worker chỉ bỏ phần mạng, không bỏ phần parse).

### 2.4 Độ trễ tương tác (4x CPU + Fast 4G, 3 mẫu, trung vị)

| Thao tác | App | Đổi màn thấy được (`firstChange`) | Event dài nhất (INP-ish) |
|---|---|---|---|
| Tab **Bàn dựng → Duyệt** | Viderse | **51 ms** | 64 ms |
| Tab **Bàn dựng → Khám phá** (lần đầu, phải tải chunk) | Viderse | **151 ms** | 64 ms |
| Mở **sheet "Việc nền"** | Viderse | **94 ms** | 104 ms |
| Tab **Viết → Thêm** | Noverse | 148 ms | 168 ms |
| Mở **sheet danh sách lệnh `/`** | Noverse | 162 ms | 72 ms |

Ghi chú: lần chuyển tab **đầu tiên** sang một màn chưa từng mở của Viderse có mẫu 315 ms
(`pointerdown` 112 ms) — chi phí tải chunk lazy; các lần sau 37–76 ms.

### 2.5 Cuộn danh sách dài (4x CPU, cử chỉ chạm 3 s)

| Màn | App | fps | p95 frame | max | khung > 33 ms | Đã cuộn |
|---|---|---|---|---|---|---|
| **Khám phá** (34 video, thumbnail thật, lưới ảo hoá) | Viderse | **45–51** | **34–46 ms** | 66–95 ms | 8–15 / ~140 | 1 812 px |
| **Workspace** (nội dung ngắn) | Noverse | 54–59 | 25–32 ms | 34–145 ms | 2–6 / ~160 | 255 px (hết nội dung) |

Hai màn không cùng khối lượng nội dung ⇒ **không kết luận được Noverse cuộn mượt hơn**; chỉ kết luận
được: cuộn lưới ảnh của Viderse **rơi khung** (p95 34–46 ms > ngân sách 60 fps là 16.7 ms; 5 khung
vượt 50 ms trong 3 s).

### 2.6 Tương tác nền khi ngồi yên (10 s, tab đang hiện)

| App | Request | SSE event | Nhận xét |
|---|---|---|---|
| Viderse | **3** | 0 | `/api/activity` mỗi 3 s — vĩnh viễn, kể cả khi SSE đang mở |
| Noverse | **0** | 0 | không poll nền |

---

## 3. Thành phần bundle (đo bằng build thật + sourcemap)

Lệnh dựng (không đụng `dist` của repo):

```
cd apps/viderse/web && npx vite build --outDir /tmp/vperf-dist --mode production
# + bản có sourcemap để quy byte về từng module (plugin tạm, xem /tmp/vperf/analyze.config.mts)
```

### 3.1 Entry Viderse — 860 kB thô / **258 kB nén** (nội dung minify 826 225 byte, quy byte về module bằng sourcemap)

| Khối | Byte | % | Ghi chú |
|---|---|---|---|
| `react-dom` | 209 949 | 25.4 % | không tránh được |
| **kit `platform/ui`** | 166 997 | **20.2 %** | `tokenDefaults.ts` 44 920 · `Toast.tsx` 17 950 · `Selects.tsx` 15 216 · `ConfirmDialog.tsx` 11 801 |
| **i18n toàn bộ màn** | 138 466 | **16.8 %** | `i18n/vi.ts` 23 898 + `library` 17 531 + `channels` 16 258 + `storageServer` 14 279 + `newScreen` 11 313 + … |
| **`zod` v4** | 99 256 | **12.0 %** | chỉ để `parse()` vài schema wire |
| `screens/workbench` | 43 147 | 5.2 % | `WorkbenchPanels.tsx` 23 555 |
| `components/assistant` | 22 348 | 2.7 % | |
| `react` + `scheduler` | 12 593 | 1.5 % | |

⇒ **22.2 % entry là dữ liệu tĩnh** (i18n 138 kB + `tokenDefaults` 45 kB) và **12 % là zod** — hai thứ
đều có thể cắt mà không đổi tính năng.

### 3.2 Chunk tải trên màn đầu nhưng người dùng không mở

Màn Bàn dựng tải 8 file script; 6 file trong đó thuộc sheet "Việc nền" **chưa được bấm**:

| Chunk | Thô | Nén |
|---|---|---|
| `QueueSummarySheet` | 7 171 | 2 806 |
| `Sheet` | 2 767 | 1 275 |
| `Segmented` | 5 841 | 2 449 |
| `MobileQueue` | 25 817 | 7 683 |
| `fmt` | 1 226 | 638 |
| `notificationList` | 4 602 | 2 073 |
| **Tổng** | **47.4 kB** | **16.9 kB** + 6 round-trip |

### 3.3 CSS: một tệp cho **mọi** màn

`assets/index-*.css` = **320 115 byte thô / 80 868 byte nén**, chặn render (render-blocking):

- 224 985 byte là atom StyleX (`@layer priority…`) — **có cả atom của màn chỉ dùng trên máy tính**;
  kiểm chứng: class `xgx3art`, `x1ouywsu`, `xyreu3s` lấy từ `settings.stylex-*.js` (màn Cài đặt,
  desktop) đều nằm trong tệp CSS này.
- 95 130 byte còn lại: CSS cũ của kit + preflight Tailwind + **26 khối `@font-face`**.
- 4 254 rule, 138 `@media`.

### 3.4 Font: 9 file / 146.5 kB cho màn đầu

`styles/viderse.css:11-16` import 4 độ đậm Be Vietnam Pro + 2 JetBrains Mono; mỗi độ đậm có 3 subset
(`latin`, `latin-ext`, `vietnamese`) ⇒ trình duyệt tải **3 subset × 3 độ đậm = 9 tệp** (146.5 kB) ngay
lần vẽ đầu. Noverse y hệt (3 độ đậm → 9 tệp / 118.7 kB).

### 3.5 So với Noverse: **một chunk 2.48 MB**

| Khối | Byte | % |
|---|---|---|
| `screens/settings` | 256 157 | 10.3 % |
| `screens/audio` | 245 178 | 9.9 % |
| kit `platform/ui` | 212 854 | 8.6 % |
| `react-dom` | 211 974 | 8.6 % |
| `screens/project` | 162 261 | 6.5 % |
| `screens/workspace` | 145 979 | 5.9 % |
| `screens/reader` | 101 250 | 4.1 % |
| `screens/cmdresults` | 95 329 | 3.8 % |
| `screens/onboard` | 92 759 | 3.7 % |
| `screens/newstory` | 76 828 | 3.1 % |
| `i18n/vi.ts` | 70 523 | 2.8 % |

**Noverse không tách chunk nào** (`App.tsx` import tĩnh toàn bộ màn hình desktop + tablet + mobile).
Đó là lý do entry 2.5 MB, heap 15–19 MB, và mỗi lần mở lại vẫn có long task 1.5 s. Viderse tách chunk
tốt hơn hẳn — nhưng bù lại mất "keep-alive" (xem C6) và trả giá ở các mục C1–C5.

---

## 4. Nguyên nhân gốc, xếp theo tác động

> Cách xếp: (a) số byte/CPU tiết kiệm được trên **màn đầu điện thoại**, (b) có bằng chứng đo, (c) mức
> độ đơn giản của cách sửa. Không có mục nào yêu cầu đổi hợp đồng API.

### C1 — Entry 258 kB nén, trong đó 22 % là dữ liệu tĩnh và 12 % là zod · **P1**
- **Bằng chứng**: §3.1. `src/i18n/vi.ts:4-16` import **tất cả** 13 module copy của mọi màn;
  `platform/ui/src/brand.ts:11` kéo `tokenDefaults.ts` (45 kB) và được gọi ngay trong
  `src/main.tsx:13` (`applyViderseBrand()`); `src/api/activity.ts:61` dùng `zod` và được kéo vào entry
  qua `shell/MobileShell.tsx:12` + `shell/ActivityBridge.tsx:6`.
- **Ảnh hưởng**: ~183 kB (dữ liệu tĩnh) + ~99 kB (zod) trên 826 kB; ở 4x CPU phần parse này nằm trong
  `ScriptDuration` 1.1–1.3 s và long task 400–700 ms đầu tiên.
- **Hướng sửa**: (1) tách `t` theo màn (`i18n/workbench.ts`, `i18n/explore.ts`…) và chỉ import trong
  chunk của màn; (2) bỏ `applyViderseBrand()` khỏi đường boot — đưa bảng token vào CSS `:root` sinh
  lúc build thay vì gọi hàm lúc chạy; (3) thay `zod` ở đường đọc bằng bộ kiểm tra viết tay (hoặc
  `zod/mini`/`valibot`) cho 4–5 schema thực dùng, hoặc chuyển sang chunk riêng của màn cần.
- **Ngân sách**: entry ≤ **150 kB nén**.

### C2 — Một tệp CSS 81 kB nén cho mọi màn, chặn render · **P1**
- **Bằng chứng**: §3.3; `index.html` chỉ có một `<link rel="stylesheet">`; atom của màn desktop-only
  (Cài đặt) nằm trong đó.
- **Ảnh hưởng**: `RecalcStyleDuration` 98–172 ms, `LayoutDuration` 213–1 136 ms; CSS là tài nguyên
  chặn vẽ đầu tiên nên nó cộng thẳng vào FCP.
- **Hướng sửa cần kiểm chứng**: tách CSS theo route khi công cụ hỗ trợ. Kiểm tra bản cài đặt
  `@stylexjs/unplugin` 0.19.1 ngày 2026-11-02 cho thấy plugin gom metadata StyleX vào CSS chung,
  không có tuỳ chọn `splitCss`; chỉ đổi import sang lazy không tách được các atom này. Nhận định
  trước đó rằng plugin hỗ trợ tách theo entry động là sai. Không thêm tuỳ chọn không tồn tại hay
  sửa nội bộ plugin; giữ ngân sách bên dưới để đối chiếu số đo bản dựng cuối, không coi là đã đạt.
- **Ngân sách**: CSS màn đầu ≤ **40 kB nén**.

### C3 — 9 tệp font / 146.5 kB ngay lần vẽ đầu · **P1**
- **Bằng chứng**: `src/styles/viderse.css:11-16`; đo được 9 request font ở cả 3 lần cold.
- **Ảnh hưởng**: 146 kB = 26 % tổng byte màn đầu của Viderse; mỗi file ~20 kB × 3 subset × 3 độ đậm.
  Text tiếng Việt khớp cả `vietnamese` và `latin-ext` nên tải cả hai.
- **Hướng sửa**: subset lại đúng dải ký tự đang dùng (một tệp `vietnamese+latin` ~25 kB/độ đậm), chỉ
  giữ 400/600, `font-display: swap` + `<link rel=preload>` cho đúng 2 tệp; hoặc dùng biến thể
  variable của Be Vietnam Pro.
- **Ngân sách**: ≤ **4 tệp / 80 kB**.

### C4 — Chunk của sheet chưa mở vẫn bị tải lúc khởi động · **P2**
- **Bằng chứng**: `src/shell/MobileShell.tsx:21` (`lazy(...)`) và `:126`
  (`<QueueSummarySheet open={jobsOpen} … />` **luôn được render**); component trả `null` khi đóng
  (`screens/queue/QueueSummarySheet.tsx:38`) nhưng `React.lazy` đã tải module + 5 module con.
- **Ảnh hưởng**: 6 request / 16.9 kB nén thừa trên mọi lần mở app.
- **Hướng sửa**: `{jobsOpen && <QueueSummarySheet … />}` (hoặc `Suspense` chỉ mount khi mở).
- **Ngân sách**: 0 chunk ngoài màn đầu.

### C5 — Poll `/api/activity` mỗi 3 s, vĩnh viễn · **P1** (pin, không phải băng thông)
- **Bằng chứng**: `src/shell/ActivityBridge.tsx:39`
  `setInterval(() => void activity.refresh(), 3000)` — không phụ thuộc SSE, không phụ thuộc
  `visibilityState`; mỗi lần `refresh()` làm mới resource `['activity']` ⇒ mọi consumer (JobsPill,
  badge tab Duyệt) render lại. Đo được **3 request/10 s** khi ngồi yên (Noverse: 0).
  Cộng thêm `src/data/status.ts:33` (`POLL_STALE_MS = 2500`, dùng ở `:35`) khiến mỗi lần mount màn là một lần gọi
  `/api/status`, và `src/data/events.ts:64` poll 2.5 s khi SSE rớt.
- **Ảnh hưởng**: pin, radio, và render lại nền trên điện thoại; **18 request/phút** (đo 3 req/10 s).
- **Hướng sửa**: bỏ hẳn interval — `/api/activity` đã có sự kiện SSE; chỉ poll khi
  `events.status() !== 'open'` **và** tab hiện, với backoff 10→60 s; dừng khi `document.hidden`.
- **Ngân sách**: ≤ **2 request/phút** khi tab hiện, 0 khi ẩn.

### C6 — Không keep-alive: mỗi lần đổi tab là tháo màn + dựng lại · **P2**
- **Bằng chứng**: `src/shell/AppShell.tsx:99-105` render đúng `{children}` của route hiện tại;
  `src/App.tsx:59-81` trả về component khác nhau ⇒ unmount. Đối chiếu Noverse:
  `apps/noverse/web/src/mobile/tabs.tsx:4,158` (`MobilePanes` + `KeepAlive`) + `mobile/index.tsx:49,166`
  (`PaneView` bọc `memo`, `paneSignature`).
- **Ảnh hưởng đo được**: chuyển tab 51–151 ms (Viderse) — chấp nhận được, **nhưng** mất vị trí cuộn,
  mất state, và mỗi lần quay lại phải đọc lại resource đã hết `staleTime` (2.5 s) ⇒ thêm request.
- **Hướng sửa**: bọc 5 tab trong `<Activity mode="visible|hidden">` (React 19.3 — xem
  `local://react-render-guide.md §1`); pane ẩn tự dừng effect (SSE/poll) nhưng giữ DOM + vị trí cuộn.
- **Ngân sách**: chuyển tab đã từng mở ≤ **100 ms**, 0 request mới khi dữ liệu còn hạn.

### C7 — Tab bar kính mờ dán đáy, blur 26 px trên nội dung đang cuộn · **P1** (paint)
- **Bằng chứng**: `src/shell/MobileShell.tsx:147`
  `backdropFilter: 'blur(26px) saturate(180%)'` + 3 lớp `boxShadow` trên phần tử `position: fixed`.
- **Ảnh hưởng**: mỗi khung hình cuộn, compositor phải làm mờ lại vùng 390×(64+inset) phía sau thanh.
  Đo cuộn Khám phá: **45–51 fps, p95 34–46 ms, 8–15 khung > 33 ms**. (Noverse cũng dùng đúng hiệu ứng
  này ở `apps/noverse/web/src/mobile/mobile.css:17` — nên đây là **vấn đề của chuẩn chung**, không
  riêng Viderse.)
- **Hướng sửa**: giảm `blur(26px)` → `blur(10–12px)` hoặc nền đặc `--vv-glass` khi
  `prefers-reduced-transparency`/máy yếu; bỏ `saturate`; thêm `contain: paint` cho vùng cuộn.
- **Ngân sách**: cuộn ≥ **55 fps, p95 ≤ 20 ms**.

### C8 — Bàn dựng không có bản điện thoại; kéo theo panel + composer vào entry · **P2**
- **Bằng chứng**: `src/screens/workbench/WorkbenchScreen.tsx` **không có nhánh `mobile`/`narrow`**
  (đối chiếu `screens/explore/ExploreScreen.tsx:55` có `useMediaQuery('(max-width: 767px)')`);
  nó import tĩnh `WorkbenchPanels` (23.5 kB) + `AssistantComposer` (4.4 kB) + `SuggestionPanels` (2.8 kB).
- **Ảnh hưởng**: 43 kB của entry + toàn bộ cây DOM desktop dựng trong 390 px (đúng triệu chứng "không
  giống artboard" mà chủ dự án nêu); khi có job thật, `LiveStudio` còn gắn thẻ `<video>`.
- **Hướng sửa**: màn `MMain` riêng cho < 768 px; panel/composer `lazy()`.
- **Ngân sách**: entry không còn module `screens/workbench/panels`.

### C9 — Danh sách không ảo hoá (trừ Khám phá) · **P2**
- **Bằng chứng**: `@tanstack/react-virtual` chỉ dùng ở `editor/ui/timeline/timeline-panel.tsx`;
  lưới Khám phá ảo hoá thủ công (`lib/useVirtualGrid.ts`, dùng ở `ExploreScreen.tsx:38,145`); Thư viện
  (`screens/library/*`) và Hàng chờ render hết.
- **Ảnh hưởng hiện tại**: Khám phá 34 video → 2 412 node DOM (đo `warmMetrics`), cuộn vẫn rơi khung.
  Với 200–500 video thì chi phí dựng DOM + ảnh sẽ tăng tuyến tính.
- **Hướng sửa**: dùng lại `useVirtualGrid` cho Thư viện/Review; `content-visibility: auto` cho thẻ
  ngoài khung nhìn.
- **Ngân sách**: ≤ 120 node DOM cho mỗi 20 thẻ danh sách (đúng như comment đầu `ExploreScreen.tsx`).

### C10 — Vụn vặt có bằng chứng · **P2**
- `static.cloudflareinsights.com/beacon.min.js` 28.7 kB chạy trên mọi lần mở (cả hai app) — không cần
  cho điện thoại, tải sau `load` hoặc bỏ.
- `/api/channels/summary` bị gọi **2 lần** trong 60 s đầu (cùng key `['channels','summary']` ở
  `shell/AppShell.tsx:46` và `screens/workbench/WorkbenchScreen.tsx`), nghi do SSE `channels` làm
  invalidate giữa hai lần mount `[INFERENCE]`.
- `LensLogo` xoay vô hạn 9 s (`components/LensLogo.tsx:12-15`) khi có job chạy — animation transform
  nên rẻ, nhưng nên tôn trọng `prefers-reduced-motion` (đã có ở `styles/viderse.css`).

---

## 5. Ngân sách đề xuất (điện thoại, 390×844)

Đo ở **4x CPU + Fast 4G**, máy đo rảnh (không có 5 trình duyệt khác chạy):

| Hạng mục | Viderse hiện tại | Noverse hiện tại | **Ngân sách** |
|---|---|---|---|
| Entry JS (nén) | 258 kB | 748 kB | **≤ 150 kB** |
| Tổng JS màn đầu (kể cả chunk lazy) | 275 kB | 748 kB | **≤ 200 kB** |
| CSS màn đầu (nén) | 81 kB | 89 kB | **≤ 40 kB** |
| Font màn đầu | 9 tệp / 147 kB | 9 tệp / 119 kB | **≤ 4 tệp / 80 kB** |
| Request màn đầu | 61 | 36 | **≤ 25** |
| TBT @4x | 540–615 ms | 645–1 202 ms | **≤ 200 ms** |
| Long task dài nhất | 417–590 ms | 418–1 556 ms | **≤ 150 ms** |
| FCP @4x + Fast 4G | 2.2–10.6 s (theo máy chủ) | 2.1–16.5 s | **≤ 2.0 s** (khi origin ≤ 300 ms TTFB) |
| INP (p75) | 64–104 ms | 72–168 ms | **≤ 200 ms** |
| Chuyển tab (đã mở) | 51–151 ms | 148 ms | **≤ 100 ms** |
| Cuộn danh sách | 45–51 fps, p95 34–46 ms | 54–59 fps, p95 25–32 ms | **≥ 55 fps, p95 ≤ 20 ms** |
| Request nền khi tab hiện | 18/phút | 0 | **≤ 2/phút** |
| JS heap sau 60 s | 4.9 MB | 15–19 MB | **≤ 8 MB** |

---

## 6. Safe area (phần liên quan hiệu năng)

Đo trên khung 390×844 (không có notch giả lập), đọc thẳng CSS sẽ áp:

| Chỗ | Công thức | File:line |
|---|---|---|
| Vỏ app | `paddingTop: env(safe-area-inset-top)` | `shell/MobileShell.tsx:134` |
| Thanh tab dán đáy | `bottom: calc(--vv-s-4 + env(safe-area-inset-bottom))`, cao 64 px | `shell/MobileShell.tsx:147` |
| Đệm đáy vùng nội dung | `paddingBottom: var(--vv-tabbar-pad)` = **110 px cố định** | `shell/AppShell.tsx:102`, `styles/viderse.css:247` |
| Sheet hàng chờ | `paddingBottom: env(safe-area-inset-bottom, 0px)` | `screens/queue/queueMobile.stylex.ts:70` |
| Thanh hành động màn Video mới | `bottom: env(safe-area-inset-bottom)` | `screens/new/NewScreen.tsx:205` |

Hệ quả hiệu năng/hiển thị:
- iPhone inset đáy 34 px ⇒ mép trên thanh tab ở `4 + 34 + 64 = 102 px`; đệm nội dung 110 px ⇒ **chỉ
  còn 8 px** trống. Máy có inset lớn hơn (hoặc khi thanh tab đổi chiều cao) nội dung sẽ chui dưới
  thanh — và vùng blur phải phủ thêm đúng phần inset đó mỗi khung hình.
- Vùng blur = 390 × (64 + inset) px dán đáy, vẽ lại mỗi khung hình khi cuộn ⇒ xem C7. Khi thu nhỏ
  blur, giữ nguyên `bottom` để không đổi bố cục.

---

## 7. Không quan sát được / hạn chế

1. **Danh sách dài thật** (Bàn dựng có 20–50 video, Hàng chờ có job, Review có ảnh, Thư viện có
   template) — tài khoản `qaaudit` trống; không tạo dữ liệu trên production. Chỉ Khám phá (34 video)
   là danh sách thật. Cụ thể **chưa đo được**: cuộn danh sách Bàn dựng, Hàng chờ (0 job), Duyệt
   (0 mục), Thư viện (7 prompt + 12 lớp phủ — quá ngắn để cuộn có ý nghĩa); chuyển tab sang **Kênh**
   và **Video mới** không đo riêng (cùng cơ chế lazy chunk + remount như Duyệt/Khám phá).
2. **Danh sách chương Noverse** — 2 truyện nháp 0 chương ⇒ chỉ đo được workspace (nội dung 255 px) và
   màn "Thêm" (không có vùng cuộn); danh sách thư viện Noverse chỉ 2 thẻ nên không đo cuộn.
3. **Máy thật** (iPhone/Android) — mọi số đo là Chrome trên macOS với throttle; GPU thật, nhiệt, và
   Safari/WebKit chưa được đo. `backdrop-filter` trên WebKit iOS tốn kém hơn Chromium.
4. **INP p75 thật** cần nhiều phiên người dùng; số ở §2.4 là mẫu tổng hợp 3 lần/thao tác.
5. Ảnh hưởng của **5 agent khác chạy trình duyệt cùng lúc** lên mọi số tuyệt đối (xem §1.1).

---

## 8. Việc nên làm ở giai đoạn triển khai (thứ tự)

1. C4 (một dòng) + C5 (bỏ interval) — rẻ nhất, cắt 6 request/lần mở và 18 request/phút.
2. C3 (font) — cắt 90 kB khỏi màn đầu.
3. C1 (i18n theo màn + bỏ `tokenDefaults` khỏi boot + bỏ zod ở đường đọc) — cắt ~250 kB thô.
4. C2 (CSS theo chunk) — cắt ~40 kB nén.
5. C6 (`<Activity>` cho 5 tab) — cắt request lặp, giữ vị trí cuộn.
6. C7 (giảm blur) + C8 (màn Bàn dựng riêng cho điện thoại) — đi cùng đợt sửa giao diện MMain.
7. Đo lại bằng đúng harness này (`/tmp/vperf/harness.mjs`) trên máy rảnh, ghi lại 3 lần/app.

---

## 9. Tệp bằng chứng

- Số liệu thô: `/tmp/vperf/vid-cold.json`, `nov-cold.json`, `vid-cold2.json`, `vid-warm.json`,
  `nov-warm.json`, `vid-inter2.json`, `nov-inter.json`, `nov-scroll1.json`,
  `v3-{1,2,3}.json`, `il-*.json`.
- Quy byte theo module: `/tmp/vperf/attr-vid-entry.json`, `/tmp/vperf/attr-nov-entry.json`.
- Bản dựng phân tích: `/tmp/vperf-dist` (Viderse), `/tmp/vperf-dist3` (+sourcemap),
  `/tmp/vperf-dist-nov` (Noverse).
- Ảnh: `/tmp/vmob-MPerf-vid-explore.png`.
