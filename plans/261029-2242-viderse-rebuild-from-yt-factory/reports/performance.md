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
| 6 | Đổi trạng thái job → JobsPill/Bàn dựng cập nhật: SSE ≤ 1 s | **Chưa đo được** trong môi trường này (xem §6) | **CHƯA ĐO** | xem §6 (lệnh kèm theo) |
| 7 | … cùng ngưỡng với chế độ chờ (poll) ≤ 3 s | **Chưa đo được** (cùng lý do §6) | **CHƯA ĐO** | xem §6 |
| 8 | Mất máy chủ → dải offline hiện ≤ 5 s | **18 ms** kể từ lúc trình duyệt mất mạng (dải "Mất kết nối với máy chủ Viderse · Thử lại ngay") | **ĐẠT** | CDP `Network.emulateNetworkConditions offline` + MutationObserver trên `document.body` |
| 9 | Cuộn Khám phá/list với 20.000 video: DOM ≤ 200 hàng, không frame > 50 ms | DOM: **18 hàng video** (18 `<img>`, 78 `<button>`, 404 phần tử trong `main`) trên trang 60 video; frame: **chưa đo được** (tab headless không phát `requestAnimationFrame`) | DOM **ĐẠT** · frame **CHƯA ĐO** | xem §7 |
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

## 6. Độ trễ cập nhật khi job đổi trạng thái (SSE và poll) — CHƯA ĐO

Đã thử ba đường trong tab thật (1440×900, Vite/serve cục bộ):

- `POST /api/clone` (enqueue) → 400 `invalid_request` "Có link video không đọc được": máy này không có mạng ra YouTube, nên không tạo được job mới.
- `POST /api/clone/<id> {action:'pause'|'resume'}` trên job seed → 2xx nhưng DOM Bàn dựng không đổi (job seed không ở nhóm đang hiển thị trong khung xem), nên không có mốc so sánh.
- `DELETE /api/clone/<id>` → 204, `document.body` không đổi trong 10 s.

Cần đo lại khi có mạng hoặc khi có job `running` thật:

```sh
# tab đã đăng nhập, đang ở #/ (Bàn dựng), job đang chạy
await tab.evaluate(`(async () => {
  const txt = () => document.body.textContent.replace(/\\s+/g,' ')
  const before = txt(); const t0 = performance.now()
  await fetch('/api/clone/' + JOB_ID, { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({ action:'pause' }) })
  const tPost = performance.now()
  const at = await new Promise(r => { const o = new MutationObserver(() => { if (txt() !== before) { o.disconnect(); r(performance.now()) } }); o.observe(document.body,{subtree:true,childList:true,characterData:true}); setTimeout(() => r(null), 10000) })
  return { sseMs: at - tPost }
})()`)
# poll: tab.route('**/api/events**', r => r.abort()) rồi lặp lại phép đo trên (kỳ vọng ≤ 3 s)
```

Phía máy chủ, luồng SSE có test sẵn (`internal/web` event tests) và các ngưỡng p95 ở §2 không phụ thuộc
đường này; phần thiếu chỉ là con số độ trễ phía trình duyệt.

## 7. Cuộn danh sách 20.000 video

- API: `GET /api/rivals/all/videos?limit=200` trả `total = 20000`, mỗi trang 60 video (mặc định UI).
- DOM sau khi mở Khám phá: `main` có **404 phần tử**, **18 `<img>`**, 78 `<button>` → số hàng video hiển thị
  **18**, thấp hơn hẳn mốc 200 (danh sách phân trang + lưới ảo `useVirtualGrid`).
- Frame: tab điều khiển không phát `requestAnimationFrame` (0 frame trong 30 bước cuộn × 500 px), nên
  **không đo được** độ dài frame. Cần đo bằng trace thật:

```sh
agent-browser --session p10perf open 'http://127.0.0.1:18901/#/explore'
# Chrome DevTools → Performance → Record → cuộn hết danh sách → Stop
# hoặc CDP: Tracing.start(categories: devtools.timeline) quanh thao tác cuộn, đếm frame > 50 ms
```

## 8. Lệnh dựng `dist` cho Main

```sh
cd apps/viderse/web && npm run build   # tsc -b src/editor/tsconfig.json && tsc --noEmit && vite build (outDir ../../internal/web/dist)
node scripts/size.mjs                  # phải in "245.0 kB gzip" và exit 0
```
