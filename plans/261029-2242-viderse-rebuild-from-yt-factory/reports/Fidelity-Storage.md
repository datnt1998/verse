# Fidelity · Settings › Lưu trữ (desktop + điện thoại)

Artboard gốc: `Settings.dc.html?section=storage&store=*` (1440×900) và
`MSettings.dc.html?screen=storage&store=*` (390×844). App: `#/settings?section=storage`.

Screenshot: `/tmp/viderse-fidelity/storage/shots/` — cặp `app-*.png` (app) ↔ `art-*.png` (artboard).
Dữ liệu trạng thái không dựng được bằng backend thật (ổ rút, đang chuyển, xong…) được
mock **trong trình duyệt** bằng `setRequestInterception`, đúng hình JSON của handler
(`internal/web/storage.go`: `storageWire`/`locationWire`/`cleanupPreviewWire`…). Không có
fixture nào trong mã sản phẩm.

## Bảng trạng thái

| Artboard (URL) | App (route/state) | Kết quả |
|---|---|---|
| `Settings…&store=here` | `/settings?section=storage` | **match** — thẻ thư mục (ổ · loại · đường dẫn `~/…`), Mở trong Finder, Đổi thư mục…, dòng “Luôn ở máy này”, biểu đồ + chú giải 5 nhóm, thẻ dọn dẹp, 2 thẻ xoá nhóm |
| `Settings…&store=pick` | bấm “Đổi thư mục…” | **match** — khối chọn tại chỗ: thư mục mới + nút, gợi ý ổ đang cắm (kèm chỗ trống), 2 thẻ radio dữ liệu, checklist 3 dòng, banner “{n} việc nền…”, footer Huỷ / Chuyển sang thư mục mới (36px) |
| `Settings…&store=cloud` | chọn Google Drive | **match** — path `~/Library/CloudStorage/…`, dòng cảnh báo đỏ, radio chọn đỏ, checklist có dòng warn, CTA khoá (opacity .42 như artboard) |
| `Settings…&store=lowspace` | chọn Lexar 128GB | **match** — dòng “Không đủ chỗ: cần 89 GB, ổ chỉ còn 60 GB”, CTA khoá |
| `Settings…&store=moving` | `move.active` + task `storage_move` | **match (trừ 1 nút)** — thẻ thư mục hiện ổ **nguồn** (Macintosh HD · Ổ trong máy · 212 GB) đúng artboard; tiến độ 6px, 4 nhóm ✔/⏳, “Huỷ chuyển · dữ liệu ở Macintosh HD vẫn nguyên”. Thiếu “Xem khi xong ›” — hỏi-đáp **#23** (màn Hàng chờ mới là `ScreenEmpty`, chưa có bảng hàng chờ nên nút sẽ dẫn vào chỗ trống) |
| `Settings…&store=done` | `previous.mode=move` | **match** — banner xanh “Đã chuyển 81,1 GB sang Samsung T7”, 2 nút trong banner (`Xoá bản cũ ngay · 81,1 GB` cam nhạt + `Chuyển về Macintosh HD` quiet), chữ banner 14px sáng. Khác #23: câu “2 video đã chạy tiếp.” bỏ (chưa đếm được video chạy tiếp) |
| `Settings…&store=donenew` | `previous.mode=new_only` | **match** — banner 1 dòng “Video mới lưu ở Samsung T7…”. Số “Ổ còn trống” = chỗ trống của **ổ đang giữ dự án** (1,54 TB) chứ không phải 212 GB tĩnh của artboard (xem đề xuất #B) |
| `Settings…&store=offline` | `projects.online=false` | **match** — banner viền đỏ “Không thấy ổ Samsung T7”, câu đếm việc nền (14:40 → 21:40), 2 nút `Thử lại` (trắng) + `Lưu video mới vào Macintosh HD` (viền). “Ổ còn trống” = chỗ trống ổ trong máy (212 GB) như artboard |
| `Settings…&store=clean` (idle) | thẻ dọn dẹp | **match** — “App tự dọn lúc mở và mỗi 6 giờ · lần cuối 22:14.”, nút “Xem trước những gì sẽ dọn” 32px |
| `Settings…&store=clean` (preview) | bấm Xem trước | **match** — 3 dòng (40px, 12,5px, pad 10/12), “Để sau” quiet, “Dọn 2,4 GB” trắng đặc, chú thích “Chỉ xoá…” |
| `Settings…&store=clean` (đã dọn) | POST cleanup/run | **match** — dòng ✔ xanh “Đã dọn 2,4 GB lúc 13:09” thay danh sách |
| `Settings…&store=reclaim` (xác nhận) | bấm “Xoá video gốc” | **match** — dải cam trong thẻ: ⚠ + câu hỏi + `Huỷ` + `Xoá 31,6 GB` (cam đặc) |
| Artboard không có | hộp chọn thư mục (Dialog) | **deviation có chủ đích** — hỏi-đáp **#21**: web không lấy được đường dẫn thật của HĐH nên “Chọn thư mục…” mở danh sách do máy chủ liệt kê (Dialog/Sheet), xong quay về khối chọn tại chỗ |
| `MSettings…&screen=storage&store=here` | 390×844 | **match** — tiêu đề phụ, thẻ thư mục 16/600 + kind 12,5 + path mono 11,5, nút “Đổi thư mục lưu” 44px, thẻ dung lượng + 2 dòng Xoá 44px |
| `MSettings…&store=pick` | bấm “Đổi thư mục lưu” | **match** — sheet 168→844 (đo 169/675), handle, 4 dòng 56px radio 22px, segmented 48px, 3 dòng check, gợi ý, CTA trắng 48px cách đáy 32px |
| `MSettings…&store=lowspace` | chọn Lexar | **match** — như trên + dòng cảnh báo cam, CTA khoá |
| `MSettings…&store=moving` | đang chuyển | **match** — hộp tiến độ 14/600, stats 12,5, thanh 6px, chú thích “Việc nền tạm dừng…”, “Huỷ chuyển” 44px canh trái (không kéo hết bề ngang) |
| `MSettings…&store=done` | vừa chuyển | **match** — banner “**Đã chuyển 81,1 GB**. Bản cũ ở Macintosh HD giữ tới 04/11 rồi tự xoá.” + “Xoá bản cũ ngay” 44px cam nhạt + “Đổi thư mục lưu” |
| `MSettings…&store=offline` | ổ rút | **match** — banner đỏ (icon giữa khối như artboard), 2 nút 44px xếp dọc, kind “Ổ ngoài USB-C · APFS · còn 1,54 TB” (đọc `volume.bus`), “Ổ còn 212 GB” |

Ghi chú chung: các chỗ “86 GB / 4 lỗi / 21:40 / 13:09” là **dữ liệu sống**, khác artboard là đúng luật copy.

## Ảnh (giữ tới khi nghiệm thu)
`/tmp/viderse-fidelity/storage/shots/`
- desktop: `app-store-{here,pick,cloud,lowspace,moving,done,donenew,offline,clean-idle,clean-preview,clean-done,reclaim-ask}.png` ↔ `art-store-*.png`
- desktop, ngoài artboard: `app-store-picker-dialog.png`
- điện thoại: `app-m-{here,pick,lowspace,moving,done,offline}.png` ↔ `art-m-*.png`

## Tệp đã sửa
- `apps/viderse/web/src/screens/settings/storage/StorageSection.tsx`
- `apps/viderse/web/src/screens/settings/storage/storage.css` (toàn bộ CSS của mục nằm ở đây; `settings.css` không còn luật `vv-sto__/vv-loc/vv-move/vv-cleanlist/vv-usage` nào)
- `apps/viderse/web/src/i18n/storage.ts`
- `apps/viderse/web/src/api/storage.ts` (kiểu `displayPath`)
- `apps/viderse/internal/web/storage.go` (`displayPath` + `tildePath`)
- `apps/viderse/internal/web/storage_test.go` (`TestTildePath`)

## Đề xuất hỏi-đáp (integrator hợp nhất, KHÔNG sửa 11-hoi-dap.md)

### #A · Lưu trữ: đường dẫn in `~/…` thay vì `/Users/<tên>`
- Artboard in `~/Library/Application Support/Viderse/data/projects`; app in đường dẫn tuyệt đối rồi cắt giữa.
- Code làm: `/api/storage` thêm `projects.displayPath` (handler rút gọn thư mục nhà thành `~`, `tildePath`), UI in `displayPath ?? path` ở cả hai bề rộng.
- Hỏi design: giữ `displayPath` trên wire (không đổi `path` cho logic), hay muốn UI tự cắt theo `~`?

### #B · Lưu trữ: “Ổ còn trống” lấy ổ nào
- Artboard dùng **một** con số 212 GB cho mọi trạng thái (`here`, `done`, `donenew`, `offline`).
- Code làm: ổ đang online → chỗ trống của chính ổ giữ dự án (T7: 1,54 TB); ổ rút → chỗ trống của ổ trong máy (212 GB).
- Hỏi design: đồng ý số theo ổ giữ dự án (chỉ 212 GB khi ổ rút), hay luôn hiện ổ trong máy?

### #C · (đã đóng) Nút 32px trong Cài đặt
- Đã đóng: kit có `Button size="compact"` (32 / r8 / 13 / **400**, pad 12) và `variant="warnGhost"` (`.ghost.btn-warn`: nền trong suốt, viền `--kit-errb-45`, chữ `--kit-err-ghost-ink` #FFC09A, hover `--kit-err-8`). Ba nút của mục Lưu trữ dùng kit (`ghost` / `quiet` / `primary` / `warnGhost`) — không còn CSS nút cục bộ.
- Đo lại tại chỗ: “Xoá video gốc” = 32 / 13 / 400 / r8 / nền trong / viền `rgba(255,154,92,.45)` / chữ `#FFC09A` — trùng artboard từng thuộc tính; “Xem trước…” = 32 / 13 / 400 / viền `--kit-wash-12`; “Dọn” = trắng 600.

## Dải xác nhận cam (chuyển từ `settings.css`)
- `.vv-confirm` → `.vv-sto__confirm` / `.vv-sto__confirmQ` trong `storage/storage.css`; nút xác nhận đổi từ `<button class="vv-confirm__btn">` sang kit `<Button variant="danger" size="compact">`, nút Huỷ sang `<Button variant="quiet" size="compact">`.
- Đo tại chỗ (artboard `Settings › store=reclaim`): dải 58px · pad 12 · radius 10 · nền `rgba(255,154,92,.07)` · viền `rgba(255,154,92,.3)` · câu hỏi 13px `#FFC09A`; Huỷ 32/13/400 trong suốt `#A0A0AB`; Xoá 32/r8 · nền `#FF9A5C` · mực `#1A0E06`.
- Kit đã sửa thứ tự style (`styles.compact` đứng trước variant) sau khi mình báo lỗi weight: đo lại nút xác nhận = **32 / 13 / 600**, nền `#FF9A5C`, mực `#1A0E06`; “Dọn 2,4 GB” = 32/13/600 trắng. Trùng artboard.
- `.vv-errline` không bị khai báo lại trong `storage.css` (dùng bản chung ở `styles/components.css`).
- Hàng duyệt thư mục không còn mượn `.vv-mset__row*` của khung: tự khai `.vv-sto__brow` (56px, pad 8/12, hairline, hover) + `.vv-sto__browName`.
- Lớp chết đã bỏ: `vv-banner` (không có CSS) và khối JSX trùng cho ghi chú “việc nền”.

## Vòng QA3 (14:35–14:50) — 1 major + 10 minor trong phạm vi của mình
| # | Lỗi | Sửa | Kết quả đo lại |
|---|---|---|---|
| R0 (major) | Đổi ổ giữa chờ `/api/storage/check`: yêu cầu thứ hai bị `useAsyncAction` **nuốt** (busy), rồi kết quả ổ cũ tới trễ lật lại checklist thành công + mở CTA | `PickPanel` bỏ `useAsyncAction` cho check, chạy `checkStoragePath` trực tiếp + `checkSeq` (chỉ nhận kết quả của yêu cầu mới nhất); `checking` state thay `run.pending` | Test mới `đổi ổ giữa chờ check…` (T7 trả chậm-đạt, Lexar trả nhanh-hỏng): checklist giữ “Không đủ chỗ”, CTA khoá cả sau khi T7 tới trễ |
| R3 | Thẻ chế độ desktop 55px vs artboard 57,5 | `.vv-sto__modeList .kit-rcard { min-height: 57px }` | đo lại **57px** |
| R4 | Path đám mây không đổi màu cam | `vv-sto__pickPath.is-bad { color: --vv-warn-ink }` khi suggestion `cloud_sync` | `#FFC09A` trùng `pathC` |
| R5 | Banner done: icon outline + canh trên | `icon="tick-b"` (đĩa đặc) + `bannerFix` bỏ `align-items:flex-start`, icon 16px + mt 2 | icon `tick-b.svg` 16px, canh giữa Δ1px như artboard; title 14/700, thân 14 `#EDEDEF` |
| R6 | Banner offline: title 13/500, thân 12 muted | `{offline}` bọc `vv-sto__bannerFix` (cả desktop và mobile) | title 14/700, thân 14 `#EDEDEF` — bằng artboard `store=offline` |
| R7 | Điện thoại: heading 14/600, tên ổ 16/600 (đảo vai) | `h2` mobile dùng `vv-sto__h2M`; `.vv-sto__locNameM` 16/600 → 14.5/500 | heading 15/600, tên ổ 14.5/500 — trùng artboard |
| R8 | Nhãn ổ sheet 13px, vòng chọn 2px | CSS `!important` trên `.kit-rcard` trong `vv-sto__drives`: nhãn 15, phụ 12, `kit-rdot` viền 1px `rgba(255,255,255,.28)` | nhãn 15, phụ 12, viền 1px — trùng `.vv-radio--m` |
| R9 | Helper “Muốn chọn…” dính lề sheet (x0/390) | `.vv-sto__pickBody > .vv-help { margin-inline: 4px }` (lề sheet 12 + 4 = x16/358 của artboard) | x16 / w358 — trùng |
| R10 | “(0,2 MB GB)” ở câu stayNote khi dữ liệu <1 GB | `gbOnly(bytes)` (vi-VN, 1 số lẻ) thay `fmtGB().replace(' GB','')` ở 5 chỗ: stayNote, moving stats ×2, deleteConfirm, doneNew | real DB: “(0 GB)” thay “(0,2 MB GB)” |
| R11 | CTA sheet 14/600/r12 vs artboard 15/600/r13 | bỏ nút custom, dùng kit `<Button variant="primary" size="xl">` (48/r13/15-600) + `vv-sto__goBtn` chỉ kéo ngang 100% | 48 / 15 / 600 / r13 / nền `#EDEDEF` — trùng |
| — | R1 sidebar +2px, R2 header co 55→48–51px | **file khung — FidelityMachine đã sửa** (`grp` 11.5px, `vv-header` flex-shrink:0) | Machine xác nhận |

- Kiểm chứng trên app thật (binary mới, DB `viderse_smoke_storage8`, mock `tab.route` đúng wire): `app-store-done-qa3fix.png`, `app-store-cloud-qa3fix.png`, `app-m-pick-qa3fix.png` trong `shots/`.
- `npx vitest run src/screens/settings`: **81/81 pass** (thêm test race). `npx tsc --noEmit`: sạch trên code của mình (Connect đang giữa chừng sửa file riêng — build chung chờ Machine).
- Chunk mục sau vòng này: `StorageSection-*.js` 26,93 kB (gzip 7,04 kB), CSS 12,69 kB (gzip 2,38 kB); first-load vẫn 126,1 kB gzip.

## Vòng QA4 (15:06) — R9 còn sót, đã sửa
- **R9 (minor, lỗi cuối của QA3)**: helper đầu sheet “Chọn ổ đang cắm vào máy…” là `.vv-sto__hintLine` — **con trực tiếp của `.m-sheet-body`** (body không padding) nên giãn x0/390 với pad 4 thay vì x16/358. Sửa trước (`.vv-sto__pickBody > .vv-help`) đặt đúng cho helper *dưới nhóm ổ*, không chạm phần tử này.
- Sửa: `.vv-sto__hintLine { margin-inline: var(--vv-s-16) }` (bỏ `padding: 0 4px` — margin dời cả khối, đúng khung đo của QA). Token `--vv-s-16` vì `design-rules` cấm số cứng.
- Ghim hợp đồng: test mới trong `checks/design-rules.test.ts` (“lời dẫn sheet Đổi thư mục thụt vào lề 16”) đọc `storage.css`, khẳng định `margin-inline: var(--vv-s-16)` và không còn `padding`.
- Kiểm chứng trên trang thật **390×844** (server 18794 + `tab.route` đúng wire; chưa rebuild dist nên phục vụ `storage.css` nguồn qua interception): `hintLine` **x16 / w358 / text x16 w318,3** — artboard x16 / w358 / text x16 w310,5 (độ rộng dòng khác do font fallback ở artboard, khớp cấu trúc). Helper “Muốn chọn…” cũng x16/w358. Ảnh: `shots/app-m-pick-qa4fix.png` ↔ `art-m-pick-qa4.png`.
- `npx tsc --noEmit` sạch; `npx vitest run src/checks src/screens/settings/storage` **28/28 pass**. Không rebuild dist — chờ build chung của Main.

## Kiến trúc & hiệu năng (số đo)
- `npm run size`: **126,1 kB gzip** lần tải đầu (1 chunk, ngưỡng 250 kB).
- Chunk mục mới nhất (sau vòng QA3): `StorageSection-*.js` 26,93 kB (gzip 7,04 kB), `StorageSection-*.css` 12,69 kB (gzip 2,38 kB) — vẫn là lazy chunk của Cài đặt.
- `npx tsc --noEmit`: sạch. `npx vitest run --maxWorkers=2`: **81/81 pass** trên `src/screens/settings` (149/149 toàn suite ở lần chạy trước).
- Go: `go build ./apps/viderse/cmd/viderse` OK; `go vet ./internal/web/` OK; `go test ./internal/web/` OK; `go test -race -p 1 ./internal/web/` OK (68 s); `go test ./archtest/` OK.
- DOM: danh sách dài nhất là 2 thẻ xoá nhóm + 3 dòng preview dọn dẹp (≪ 200 dòng); gợi ý ổ ≤ 4 chip; SSE chỉ `invalidate` `keys.storage()`, `['storage-reclaim']`.
- `GET /api/storage` trên server thật, **giữ kết nối (keep-alive)**, sau 100 lượt warmup:
  **56 308 request trong 12 s** (quét qua ~6 nhịp watch 2 s) → **p50 0,20 ms · p95 0,31 ms · p99 0,46 ms · max 13,5 ms**.
  Request đầu sau boot: 1,24–1,59 ms. Con số p95 52,6 ms trước đây là do `curl` mở kết nối mới mỗi lần (đo 20 mẫu, p95 = mẫu thứ 19) chứ không phải handler.
- Handler không spawn tiến trình / đi mạng: `volumeCache.List/Info` chỉ đọc snapshot trong bộ nhớ (refresh ổ mới chạy ở Start và nhịp watch 2 s), `tildePath` chỉ đọc `$HOME`.

## Việc còn lại (không chặn)
- Hàng chờ thật chưa có → hai câu trong banner (`{n} video đã chạy tiếp`, “Xem khi xong ›”) chờ hỏi-đáp #23 như hiện trạng.

Status: DONE
Summary: Vòng QA3+QA4 đã sửa xong 12/12 finding trong phạm vi của mình và kiểm chứng trên app thật. **R0** (race `/api/storage/check` — `useAsyncAction` nuốt yêu cầu thứ hai, kết quả ổ cũ lật lại CTA) sửa bằng `checkSeq` gating + test hồi quy; **R3** thẻ chế độ 57px, **R4** path đám mây `#FFC09A`, **R5** banner done icon `tick-b` 16px canh giữa, **R6** banner offline 14/700+14 sáng, **R7** heading mobile 15/600 + tên ổ 14.5/500, **R8** nhãn ổ 15 + vòng chọn 1px, **R9** lời dẫn sheet `.vv-sto__hintLine` `margin-inline: var(--vv-s-16)` → x16/w358 (đo live 390×844, ghim bằng design-rules test), **R10** “(0,2 MB GB)” → `gbOnly`, **R11** CTA sheet = kit `Button xl` 48/15/600/r13. R1/R2 thuộc khung — FidelityMachine đã sửa. Mọi trạng thái Lưu trữ khớp artboard từng khối; lệch còn lại do hỏi-đáp #21/#23 + đề xuất #A/#B. Web: tsc sạch, 28/28 test checks+storage (81/81 settings), build + size 126,1 kB gzip, chunk mục 26,93 kB/7,04 gzip. Go xanh. `GET /api/storage` keep-alive: p50 0,20 ms · p95 0,31 ms trên 56 308 request. Chưa rebuild dist — chờ build chung.
