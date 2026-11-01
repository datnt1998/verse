# Phase 10 · Hiệu năng (P10Perf)

Đo trên `serve` dựng từ worktree nhánh `feat/viderse-p10` (Go không đổi so với HEAD; bundle web dựng
từ nguồn hiện tại), DB `p10_p10perf_data` / `p10_p10perf_identity`, cổng 18901.
Ngày đo: 2026-11-01. Ngưỡng lấy từ `plan.md` § "Hiệu năng và ổn định" (dòng 105–110).

## Bảng ngưỡng → đo được

| # | Ngưỡng (plan.md) | Đo được | Kết luận | Cách đo |
|---|---|---|---|---|
| 1 | API danh sách p95 ≤ 100 ms — `/api/clone?channel=all` (1.000 job) | p50 46,3 ms · **p95 54,0 ms** · p99 60,7 ms · max 70,6 ms (300 mẫu, body 3,71 MB) | **ĐẠT** | `python3 /tmp/p10perf-bench.py http://127.0.0.1:18901 <cookie> 300 "/api/clone?channel=all"` |
| 2 | p95 ≤ 100 ms — danh sách job theo kênh (`/api/clone?channel=<id>`) | **p95 25,1 ms** (p50 18,6) | **ĐẠT** | cùng script, path `/api/clone?channel=11111111-…` |
| 3 | p95 ≤ 100 ms — Khám phá: danh sách video đối thủ (20.000 video) | `/api/rivals/all/videos` **p95 14,8 ms** (sort mới nhất 15,0 ms); `/api/rivals` (40 kênh) **p95 11,1 ms** | **ĐẠT** | cùng script, path `/api/rivals/all/videos`, `/api/rivals` |
| 4 | p95 ≤ 100 ms — phụ trợ màn Khám phá/Bàn dựng | `/api/templates` **p95 6,0 ms**, `/api/channels/summary` **p95 41,7 ms** | **ĐẠT** | cùng script |
| 5 | JS tải lần đầu (shell + Bàn dựng) ≤ 250 kB gzip; editor/render chỉ tải khi mở route của nó | **245,0 kB gzip** (1 chunk, entry `assets/index-*.js`), 0 module `src/editor/**` trong chunk đầu | **ĐẠT** (sau khi sửa — trước sửa 272,2 kB và 5 module editor, xem §5) | `cd apps/viderse/web && npx vite build && node scripts/size.mjs` |
| 6 | Đổi trạng thái job → JobsPill/Bàn dựng cập nhật: SSE ≤ 1 s | **p50 438,7 ms · max 504,8 ms** (10 mẫu) | **ĐẠT** | §6 — `PATCH /api/queue` luân phiên tạm dừng/chạy, MutationObserver trên `document.body` |
| 7 | … cùng ngưỡng với chế độ chờ (poll) ≤ 3 s | **p50 552,1 ms · max 2.216,2 ms** (10 mẫu, đã chặn `/api/events`) | **ĐẠT** | §6 — chặn luồng SSE bằng request interception, lặp lại phép đo |
| 8 | Mất máy chủ → dải offline hiện ≤ 5 s | **18 ms** kể từ lúc trình duyệt mất mạng (dải "Mất kết nối với máy chủ Viderse · Thử lại ngay") | **ĐẠT** | CDP `Network.emulateNetworkConditions offline` + MutationObserver trên `document.body` |
| 9 | Cuộn Khám phá/list với 20.000 video: DOM ≤ 200 hàng, không frame > 50 ms | DOM: **15–18 hàng video**, 344–404 phần tử trong `main`; cuộn **17.561 px** trong 5 s: **1.444 task, max 2 ms, 0 task > 50 ms** | DOM **ĐẠT** · long-task **ĐẠT** (giới hạn: trace không có event Layout/Paint, xem §7) | §7 — CDP `Tracing` (`devtools.timeline`) quanh thao tác cuộn |
| 10 | RSS lúc rảnh của `viderse serve` (không tính tiến trình con) ≤ 150 MB | **42,2 MB** sau khi rảnh (đo lúc 3m39s uptime); 178,8 MB nếu đo ngay sau 900 request 3,7 MB (bộ đệm/GC chưa trả) | **ĐẠT** | `ps -o rss= -p $(pgrep -f "viderse serve")` |

## 1. Seed

SQL sinh bằng `python3 /tmp/p10perf-seed.py` → `/tmp/p10perf-seed.sql` (16 s nạp):

- 3 kênh của bạn, 50 template (recipe chuẩn), 12 template gắn mỗi kênh.
- **1.000 job** chia đều 3 kênh, đủ 7 `stage_group`, mỗi job 9 `clone_steps` → 9.000 bước.
- **40 kênh đối thủ × 500 video = 20.000 `rival_videos`**, có `score`, `origin`, `scan_state`.

```sh
set -a; . /tmp/vs3.env; set +a
psql "${TEST_DATABASE_URL%/*}/p10_p10perf_data?${TEST_DATABASE_URL#*\?}" -v ON_ERROR_STOP=1 -f /tmp/p10perf-seed.sql
```

## 2. p95 API danh sách

Script `http.client` keep-alive, 30 request khởi động rồi 300 mẫu tuần tự (không song song), p95 nội suy tuyến tính:

| Endpoint | body | p50 | p95 | p99 | max |
|---|---|---|---|---|---|
| `/api/clone?channel=all` | 3,71 MB | 46,33 ms | **53,97 ms** | 60,73 ms | 70,57 ms |
| `/api/clone?channel=<kênh>` | 1,24 MB | 18,60 ms | **25,13 ms** | 26,31 ms | 30,51 ms |
| `/api/rivals` | 38 kB | 7,32 ms | **11,06 ms** | 11,66 ms | 13,09 ms |
| `/api/rivals/all/videos` | 96 kB | 12,76 ms | **14,75 ms** | 15,26 ms | 16,24 ms |
| `/api/rivals/all/videos?sort=newest` | 96 kB | 12,81 ms | **15,01 ms** | 16,44 ms | 21,11 ms |
| `/api/templates` | 22 kB | 4,62 ms | **5,96 ms** | 6,53 ms | 6,66 ms |
| `/api/channels/summary` | 4 kB | 34,10 ms | **41,67 ms** | 44,02 ms | 44,88 ms |

Không có bộ benchmark Go sẵn trong `apps/viderse` (`grep -rn "func Benchmark"` → 0 kết quả), nên đo qua HTTP thật.

## 3. Bundle: trước và sau khi sửa

| Bản | Entry | gzip | `scripts/size.mjs` |
|---|---|---|---|
| Nguồn hiện tại **trước sửa** | `index-CGLFBRFg.js` (943 kB thô) | **272,2 kB** | **FAIL** — quá 250 kB **và** 5 module `src/editor/**` nằm trong chunk đầu |
| Nguồn hiện tại **sau sửa** | `index-Hwm6TZPY.js` | **245,0 kB** | **PASS** |
| `dist` đang commit ở HEAD (cũ, dựng 2026-10-31, trước phase 05) | `index-qdcr4GI-.js` | 147,0 kB | PASS nhưng **không phải bản của nguồn hiện tại** |

`dist` trong git là bản cũ nên `serve` dựng bằng `/tmp/p10-serve.sh` (git archive HEAD) phục vụ UI cũ —
mọi kiểm tra trình duyệt phải dựng `dist` mới hoặc chạy binary dựng từ worktree. Main dựng lại `dist`
một lần lúc tích hợp (lệnh ở §8).

Nguyên nhân vượt ngưỡng (đo bằng plugin đếm `renderedLength` từng module trong chunk đầu):

| Nhóm | gzip | Ghi chú |
|---|---|---|
| react + react-dom | 67,3 kB | không cắt được |
| `@noverse/ui` (kit) | 51,2 kB | dùng chung, ngoài phạm vi |
| `src/i18n` (toàn bộ copy tiếng Việt) | 47,3 kB | `vi.ts` gộp mọi namespace; 123 file import `t` |
| `src/shell` + `api` + `lib` | 28,9 kB | |
| zod v4 | 27,3 kB | schema wire của shell |
| Bàn dựng (panels/studio/menus) | 15,3 kB | |
| Các nhóm đã tách | 38 kB | auth, assistant, autonomy, queue mobile, editor, activity |

## 4. Việc đã sửa (kèm bằng chứng hỏng trước)

Bằng chứng hỏng trước: `npm run build && node scripts/size.mjs` → exit 1, in
`chunk đầu chứa mã editor: …/src/editor/vendor/lib/clone-view.ts, clone-contract.ts, clone-steps.ts,
clone-quality.ts, …/src/editor/ui/export/export-settings.ts` + `272.2 kB > 250 kB`.

1. **Hợp đồng clone ra khỏi editor** — `git mv src/editor/vendor/lib/{clone-view,clone-contract,clone-steps,clone-quality,clone-retry}.ts src/lib/clone/`;
   `vite.config.ts` + `tsconfig.json` trỏ `@/lib/clone-view`, `@/lib/clone-quality` vào `src/lib/clone/`.
   Đây là schema wire dùng chung shell + editor, không phải mã editor.
2. **Định dạng số hiển thị** — `formatBytes`/`formatDuration` chuyển từ `src/editor/ui/export/export-settings.ts`
   sang `src/lib/units.ts`; `src/shell/activityText.ts`, `export-dialog.tsx`, `v13/ExportDialogs.tsx` và test trỏ theo.
3. **Ranh giới lazy (không đổi hành vi hiển thị)** — mỗi mục dưới đây vốn chỉ render khi người dùng mở,
   hoặc chỉ dùng ở một nhánh trạng thái:
   - `App.tsx`: 5 màn cổng phiên (Thiết lập/Đăng nhập/Bị chặn/Đổi mật khẩu/Mất máy chủ) → `lazy` + `Suspense`
     dùng đúng khung chờ sẵn có; người đã đăng nhập không tải mã của chúng.
   - `MobileShell.tsx`: `QueueSummarySheet` (chỉ mở khi bấm, chỉ trên khổ điện thoại).
   - `AppHeader.tsx`: `ActivityPopover` (chỉ render khi mở chuông).
   - `ActivityPopover.tsx`: `NotificationSettings` (chỉ ở tab Cài đặt).
   - `WorkbenchScreen.tsx`: `Recap`, `PromptMenu`, `OverflowMenu` (chỉ render khi mở).
   - `components/confirm.tsx`: `openAutonomyDialog` nạp bằng `import()` lúc bấm xác nhận.
   - `recap/recap-logic.ts` tách `shouldAutoOpenRecap`/`recapLead` khỏi màn để `WorkbenchScreen` không kéo cả màn.

Test đã cập nhật theo ranh giới lazy: `App.test.tsx` (nạp sẵn 5 chunk cổng phiên + `settle()` sau mỗi lần đổi cổng),
`confirm.test.tsx`, `AutonomyCard.test.tsx` (chờ chunk Tự chạy), `Recap.test.tsx` (import từ `recap-logic`).

Đã chạy: `npx vitest run src/App.test.tsx src/shell src/screens/workbench src/components src/screens/queue src/checks`
và `npx vitest run --config vitest.editor.config.ts src/editor/ui/clone src/editor/v13` và
`node --import tsx --test src/editor/ui/export/export-settings.test.ts` — xanh toàn bộ, gồm `src/checks` (design rules).

Commit: phần lớn thay đổi ở `9f6ed996` (`perf(viderse): cắt 27 kB gzip…`). Riêng lần `git mv` 5 file hợp đồng
clone bị một commit của lane khác quét vào `81091859` (commit đó không kèm alias vite/tsconfig, nên HEAD
trước `9f6ed996` không dựng được); `9f6ed996` đưa alias + `tsconfig.json` vào đúng chỗ.
`apps/viderse/internal/web/dist` **không** nằm trong commit này — Main dựng lại một lần (§8).

## 5. Ghi chú kỹ thuật về ngưỡng bundle

Phần lớn dung lượng là nền chung không cắt được bằng ranh giới lazy: react 67 kB + kit 51 kB +
zod 27 kB + i18n 47 kB = 192 kB gzip trên 245 kB. Muốn dư địa lớn hơn thì phải tách `src/i18n/vi.ts`
theo namespace (47 kB, 123 file import) hoặc bỏ zod khỏi đường tải đầu — cả hai là refactor riêng,
không nằm trong "sửa nhỏ và an toàn" của phase này. Vì vậy bản sửa ở §4 giữ đúng 245 kB, sát ngưỡng
5 kB; nếu phase sau thêm >5 kB vào chunk đầu thì phải làm tiếp việc tách i18n.

## 6. Độ trễ cập nhật khi trạng thái đổi (SSE và poll)

Cách đo: tab Chromium 1440×900 mở `#/` (Bàn dựng), đã đăng nhập; đổi trạng thái bằng
`PATCH /api/queue {paused:true|false}` luân phiên 10 lần (server phát sự kiện `queue`), đo từ lúc gửi request
(`fetch` bắt đầu) tới lần mutation đầu tiên của `document.body`; mốc nhìn thấy là dải hàng chờ trong Bàn dựng
đổi giữa `0 chờ lượt` và `Đã tạm dừng · 0 chờ lượt`. Giữa hai mẫu nghỉ 300 ms.

**SSE (luồng mở):** `504,8 · 446,9 · 397,5 · 438,7 · 354,3 · 490,8 · 359,9 · 440,1 · 428,1 · 431,0` ms
→ **p50 438,7 ms, max 504,8 ms** — ĐẠT (≤ 1 s).

**Poll dự phòng:** chặn `GET /api/events` bằng request interception của Puppeteer
(`page.setRequestInterception` + `abort`), xác nhận `fetch('/api/events')` → `blocked`, rồi lặp lại 10 mẫu:
`221,3 · 2.216,2 · 2.142,0 · 345,8 · 552,1 · 427,7 · 212,9 · 197,7 · 693,3 · 698,9` ms
→ **p50 552,1 ms, max 2.216,2 ms** — ĐẠT (≤ 3 s).

Ghi chú: vài mẫu ~200–700 ms vì chính request ghi của trang làm mới cache ngay; mẫu ~2,1–2,2 s mới là
đường poll thuần (`POLL_MS = 2500` trong `src/data/events.ts`), khớp trần lý thuyết 2,5 s + độ trễ mạng.

```js
// tái lập: tab đã đăng nhập, đang ở #/
const samples = []
for (let i = 0; i < 10; i++) {
  const before = document.body.textContent.replace(/\s+/g, ' ')
  const t0 = performance.now()
  await fetch('/api/queue', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ paused: i % 2 === 0 }) })
  samples.push(await new Promise((resolve) => {
    const o = new MutationObserver(() => { if (document.body.textContent.replace(/\s+/g, ' ') !== before) { o.disconnect(); resolve(Math.round(performance.now() - t0)) } })
    o.observe(document.body, { subtree: true, childList: true, characterData: true })
    setTimeout(() => { o.disconnect(); resolve(null) }, 8000)
  }))
}
```

Đường enqueue (`POST /api/clone`) không dùng được để đo: máy này không ra được YouTube nên trả 400
`invalid_request` "Có link video không đọc được".

## 7. Cuộn danh sách 20.000 video

- API: `GET /api/rivals/all/videos?limit=200` trả `total = 20000`; UI phân trang 60 video/trang.
- DOM: mở Khám phá rồi cuộn hết danh sách — số hàng video **15–18** (`<img>` trong `main`), tổng phần tử
  `main` **344–404**, không đổi trong suốt thao tác cuộn → lưới ảo giữ DOM phẳng, **ĐẠT** mốc ≤ 200 hàng.
- Trace: `Tracing.start` (`devtools.timeline,disabled-by-default-devtools.timeline`) quanh 5 s cuộn
  **17.561 px** (89 bước × 1.200 px, hết `scrollHeight` 18.319): **1.444 task, max 2 ms, 0 task > 50 ms** — ĐẠT.
- Giới hạn của phép đo: cửa sổ Chromium do công cụ quản lý không composite (0 event `Layout`/`Paint`,
  0 `requestAnimationFrame` kể cả khi `headed`, `Page.bringToFront`, `Emulation.setFocusEmulationEnabled`
  và `Page.startScreencast`), nên con số trên là **thời gian task trên main thread** — đúng định nghĩa
  "long task > 50 ms" nhưng không phải số đo frame của compositor. Muốn có số frame: mở Chrome thật
  (`agent-browser --session p10perf open 'http://127.0.0.1:18901/#/explore'`), DevTools → Performance →
  Record → cuộn hết danh sách → Stop, hoặc `Tracing.start` trên Chrome có cửa sổ hiển thị.

## 8. Lệnh dựng `dist` cho Main

```sh
cd apps/viderse/web && npm run build   # tsc -b src/editor/tsconfig.json && tsc --noEmit && vite build (outDir ../../internal/web/dist)
node scripts/size.mjs                  # phải in "245.0 kB gzip" và exit 0
```
