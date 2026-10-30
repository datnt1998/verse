# Fidelity — Accounts (Settings › Tài khoản nền tảng)

Phạm vi: `Settings.dc.html?section=accounts` (1440×900) và `MSettings.dc.html?screen=accounts` (390×844), mọi tweak `bili=`, `qr=`, `yt=`.
Báo cáo này gộp **vòng QA** (25 điểm, xem §Lịch sử) và **vòng chuẩn hoá theo artboard** (đo lại từng pixel bằng DOM artboard sống ở `127.0.0.1:18700`).

## Vòng chuẩn hoá: bảng trạng thái (artboard → app)

Nguồn đo: artboard sống (server 18700) + app chạy binary tự build trên DB `viderse_smoke_FidelityAccounts`, Vite 5193, fixture proxy 18795 cho các trạng thái backend không tạo được.

### Máy tính 1440×900

| Artboard | Route/state app | Kết quả |
|---|---|---|
| `Settings.dc.html?section=accounts&bili=saved` | `#/settings?section=accounts`, fixture saved | **Khớp** — so khớp từng từ với artboard, không lệch |
| `…&bili=manual` | `type=cookie_string, method=manual` | **Khớp** — chip “Cookie dán tay”, mask `SESSDATA=8f3a••••c21e`, “Đăng nhập bằng mã QR” + icon `scanner`, “Xoá đăng nhập”, dòng help 12,5 px, disclosure “Dán cookie khác” |
| `…&bili=blocked` | `state=paused`, `failureCount=4`, `pausedUntil` | **Khớp** — tag “Tạm chặn tới 02:10”, banner trong thân thẻ (icon clock, không action), viền thẻ trung tính, chấm cam ở sidebar |
| `…&bili=expired` | `needs_login` + `refreshError` | **Khớp** — banner có action “Quét mã QR”, tag “Cần đăng nhập lại”, hàng tài khoản mờ + chỉ “Xoá đăng nhập”, viền cam |
| `…&bili=empty` | `needs_login`, `saved=false` | **Khớp** — “Đăng nhập tài khoản phụ · tuỳ chọn”, thân 12 px, nút primary “Đăng nhập bằng mã QR”, disclosure |
| `…&yt=bot&bili=saved` | `lastError.code=cookies_failed` | **Khớp cấu trúc**; lệch 1 từ: artboard “phụ đề của **1** video”, app “**một số** video” (lý do: wire không có số video ảnh hưởng — xem hỏi-đáp 2) |
| `yt` chọn hồ sơ “Tắt” (tương tác select) | `profile.profile=off` | **Khớp** — select “Tắt”, dòng chấm “Đang tắt · app tải video không đăng nhập” |
| `…&qr=loading` | open QR, poll chưa trả | **Khớp** — plate skeleton, “Đang tạo mã…”, không overlay |
| `…&qr=waiting` | poll `status=waiting` | **Khớp** — QR thật, progress 200 px + “Mã còn hiệu lực”, status info, 3 bước |
| `…&qr=scanned` | poll `status=scanned` | **Khớp** — overlay `#16161A` + tick, status cam “Đã quét. Bấm xác nhận trên điện thoại để xong.”, 2 bước xong |
| `…&qr=expired` | poll `status=expired` | **Khớp** — không còn track đếm, overlay xám clock, status “Mã đã hết hạn sau 3 phút.”, 0 bước xong, “Đóng” + “Tạo mã mới” (icon refresh) |
| `…&qr=rejected` | poll `status=rejected` | **Khớp** — overlay `#C2410C` + ×, banner tam giác, 2 bước giữ tick, footer như trên |
| `…&qr=error` | mất mạng/5xx | **Khớp** — plate trắng + globe “Chưa có mã”, banner globe |
| `…&qr=success` | poll `status=success` | **Khớp** — overlay xanh, status ok “Đã đăng nhập: nenvideo_02” + “App tự gia hạn đăng nhập mỗi ngày khi quét kênh.”, 3 bước xong, chỉ “Xong” |

### Điện thoại 390×844

| Artboard | Route/state app | Kết quả |
|---|---|---|
| `MSettings.dc.html?screen=accounts&bili=saved` | mobile layout | **Khớp** — thẻ 358×287 (artboard 287), tag 22, tile 36, hàng “nenvideo_02 · mã QR” + “Gia hạn” (44) |
| `…&bili=manual` |  | **Khớp** — “nenvideo_02 · cookie dán tay”, “Không tự gia hạn · quét mã để bật”, “Quét mã QR” (44) |
| `…&bili=blocked` |  | **Khớp** — tag “Tạm chặn tới 02:10”, banner clock trong thẻ, vạch ngăn trên hàng tài khoản, “Gia hạn” |
| `…&bili=expired` |  | **Khớp** — tag “Hết hạn”, banner tam giác, “Hết hạn · đang quét không đăng nhập”, nút trắng “Quét lại” |
| `…&bili=empty` |  | **Khớp** — “Tài khoản phụ · tuỳ chọn / Chưa đăng nhập” + “Đăng nhập” |
| `…&qr=waiting/scanned/expired/rejected/error/success` | sheet | **Khớp** — sheet 22 px, handle 36×4, title 18/600, sub 12,5 `#8B8B97`, nút cuối 48/r13/15/600 full width, hint 11,5 `#8B8B97`, khối “Bilibili trên chính máy này?” chỉ ở waiting |
| `yt=bot`, `yt=off`, `qr=loading` | — | **Không có artboard** (MSettings không có tweak `yt`, không có state loading). App vẫn render: banner bot trong thân thẻ (thêm 48 px chiều cao) + dòng hồ sơ chỉ đọc. Xem hỏi-đáp 3 |

So khớp **văn bản từng từ** (word-diff DOM artboard vs DOM app, đã loại giá trị sống):
`diff/{saved,manual,blocked,expired,empty,ytbot}.{board,app}.txt` và `diff/m-*.{board,app}.txt` — lệch duy nhất: `86 GB`→dung lượng thật, `23:40`→giờ tải thật, `1 video`→“một số video”, `4` (badge sidebar) — tất cả là giá trị sống hoặc shell.

Đo lại sau khi sửa (mobile, blocked): thẻ `358×287` = artboard `287`; `meta` 12 px/18 (`1.5`) `#8B8B97`; `tag` `79×22` bo 6; banner `rgba(255,154,92,.07)` + viền `.22`; nút hàng `44×89` bo 12 chữ 14 = `vv-btn--m`; nút cuối sheet `48` bo 13 chữ 15/600 pad 16 = `xl`.

## Nút — kết luận sau khi đo

| Vị trí | Lớp artboard | Số đo thật | Kit dùng |
|---|---|---|---|
| nút trong thẻ (QR/cookie/xoá) | `vv-btn vv-btn--sm` | 30 / r8 / 12 / pad 10 | `size="sm"` |
| footer dialog QR (“Đóng”, “Tạo mã mới”) | `vv-btn` | 36 / r10 / 13 / pad 14 | mặc định (`md`) |
| hàng thẻ điện thoại | `vv-btn--m` | 44 / r12 / 14 / pad 16 | `size="lg"` |
| CTA cuối sheet | `vv-btn--m-lg` | 48 / r13 / 15 / 600 / pad 16 | `size="xl"` + `width:100%` |

`class="btn"` (32/r8/13) xuất hiện **0 lần** trong `Settings.dc.html` (đó là CSS chết của khối Setup) nên Accounts không dùng `compact`. Đã báo Main + FidelityMachine; Main đã chốt lại luật theo từng vị trí.

## Ảnh đối chiếu

`/tmp/viderse-fidelity/FidelityAccounts/shots/` — 26 cặp (52 ảnh):
- Desktop 1440×900: `saved, manual, blocked, expired, empty, ytbot, ytoff` và `qr-{loading,waiting,scanned,expired,rejected,error,success}`.
- Mobile 390×844 (488×1055 @1,25): `saved, manual, blocked, expired, empty` và `qr-{waiting,scanned,expired,rejected,error,success}`.
- Không có board cho `yt=bot`/`yt=off`/`qr=loading` ở điện thoại (artboard không định nghĩa state đó).

Quy trình chụp (để QA2 dựng lại): artboard `http://127.0.0.1:18700/artboards/<File>.dc.html?...` chờ **marker của đúng state** rồi thêm 1,2 s (artboard render lại sau hydrate — chụp sớm sẽ ra state mặc định); app `?t=<n>#/settings?section=accounts` + `fetch('/api/platforms')` đối chiếu `state/saved/method` trước khi chụp.

## Tệp đã đổi (vòng chuẩn hoá)

- `screens/settings/accounts/accounts.css`
  - màu chữ phụ: `--color-fg-4` → `--kit-fg-sub` (`#8B8B97`) ở `.sacc-meta`, `.sacc-acct__sub`, `.sacc-empty__title span`, `.sacc-empty__body`, `.sacc-manual__label span`, `.sacc-yt__label span`, `.sacc-more > .vv-ic`, `.sacc-more__body`, `.sacc-secure`, `.sacc-mfoot`, `.sacc-mrow__v`, `.sacc-qr__timer-label`;
  - `.sacc-qr__hint` → `--color-fg-3` (`#A0A0AB`), `.sacc-sheet__sub`/`.sacc-qr__same-body` → `kit-fg-sub`;
  - `.sacc-sheet__help` 12 → **11,5 px**, `kit-fg-sub`;
  - chữ trong thẻ điện thoại giãn dòng **1,5** (`.sacc-card`), status sheet 14/1,5;
  - xoá `.sacc-btn-lg`, `.sacc-btn-m`, biến `--sacc-r-13` (đã thay bằng kit `xl`/`lg`).
- `screens/settings/accounts/YouTubeLogin.tsx` — tách `BotBanner` dùng chung; **hàng mobile giờ có banner bot** (artboard không có state, app cần lời giải thích cho tag “Cần cookie”).
- `screens/settings/accounts/QrLogin.tsx` — CTA sheet → `Button size="xl" style={{width:'100%'}}`; nút “Mở app” → `size="lg"`.
- `screens/settings/accounts/BilibiliLogin.tsx` — 4 nút hàng → `size="lg"`.
- `screens/settings/accounts/AccountsSection.test.tsx` — bỏ assertion class `sacc-btn-lg` (giữ test hành vi: nút chỉ chữ, không icon).
- Không đổi Go, không đổi `i18n/accounts.ts`, không đổi kit.

## Lịch sử vòng QA (đã xong trước đó)

Đã sửa 25 điểm của `QA-Accounts.md`: banner sự cố 7%/22% + icon `#FF9A5C`; bỏ action khỏi banner bị chặn + “từ 00:10”; mapping lỗi QR (409/429 = bận, còn lại = kết nối, chỉ poll `rejected` là “từ chối”); scrim blur 3 px; title 16; close 36; icon `scanner`; giữ tick khi bị từ chối; dòng “tự gia hạn” khi xong; sheet title 18 + nút 48 + same-device chỉ khi chờ; YouTube select gốc + banner trên vạch ngăn; chevron disclosure (bug selector `.vv-ic` cũ); tile mono. Chi tiết từng điểm nằm trong lịch sử git của tệp này.

## Hỏi-đáp đề xuất

1. **Q:** Banner bị chặn cần “từ HH:MM” nhưng DB chỉ có `paused_until`. Dùng `lastError.at` được không?
   **A (đề xuất):** Dùng `lastError.at` (lần lỗi đẩy sang tạm chặn); thêm cột `paused_since` chỉ khi cần chính xác tuyệt đối.
2. **Q:** Banner bot artboard ghi “phụ đề của **1** video”, wire không có số video ảnh hưởng. Giữ “một số video” hay thêm trường đếm?
   **A (đề xuất):** Giữ “một số video” (không bịa số); muốn khớp artboard thì thêm `videosAffected` vào `lastError`/job.
3. **Q:** Điện thoại không có artboard cho `yt=bot` / hồ sơ “Tắt” / `qr=loading`. App có nên hiện banner bot trong thẻ (thêm 48 px) và plate skeleton không?
   **A (đề xuất):** Có — banner trong thân thẻ đã có tiền lệ (Bilibili blocked/expired) và không có nó thì tag “Cần cookie” vô nghĩa; skeleton dùng lại của desktop.
4. **Q:** Artboard MSettings dùng giãn dòng 1,5 cho chữ trong thẻ, còn artboard desktop và app dùng 1,55. Chuẩn nào?
   **A (đề xuất):** Giữ 1,55 toàn app, chỉ hạ 1,5 trong `.sacc-card` + status sheet (đang làm) để khớp ảnh điện thoại; nếu shell chốt 1,5 cho toàn bộ MSettings thì bỏ rule cục bộ này.
5. **Q:** Mục sidebar “Giới hạn từng kênh” (artboard có, app chưa có route Channel) — thêm khi nào?
   **A (đề xuất):** Chờ trang Channel có route rồi mới thêm (không thêm link chết). Ngoài phạm vi Accounts.

## Kiến trúc & hiệu năng (số đo cuối)

- First-load JS: **126,1 kB gzip** (`npm run size`, 1 chunk, ngưỡng 250).
- Chunk `AccountsSection`: **21,69 kB / 6,49 kB gzip**; agent khác: Agents 18,50/5,58, Storage 26,90/6,99. Settings vẫn lazy theo section.
- DOM: 3 thẻ nền tảng + 1 thẻ “Nền tảng khác”; dialog QR ~30 node; không danh sách nào > 200 hàng, không cần virtual hoá, không search.
- Poll QR 2 s/lần, dừng khi đóng/hết hạn; chỉ invalidate `platforms` khi `success`; GET không spawn/tree-walk/network.
- `npx tsc --noEmit` sạch; `vitest` accounts **19/19**; `npm run build` OK.
- Toàn bộ suite: **147/148**, 1 fail thuộc `src/checks/design-rules.test.ts` do `connect/AddProvider.tsx` (`borderRadius: 12` ×2, file của FidelityConnect — đã báo). `App.test.tsx` (lazy-chunk) flaky khi chạy song song, pass khi chạy riêng.
- Go: **không đổi** → không chạy lại test Go; `internal/web/platforms.go` đã đủ trường cho UI (`usage`, `state`, `pausedUntil`, `failureCount`, `lastError{code,at,message}`, `login{…masked/account/lastRefreshAt/refreshError}`, `profiles`).

## Vòng QA3 (sửa theo `QA3-Accounts.md` §2)

| Điểm | Sửa | Bằng chứng đo lại |
|---|---|---|
| **[major]** Select hồ sơ trình duyệt YouTube inline 220px bên phải | `.sacc-select` thêm `width: 100%` (artboard `.vv-select` là `width:100%` + wrap) | Đo live: `x=297, w=726, h=36` — đúng y hệt số QA3 đo trên artboard |
| Step badge `--done`/`--now` dùng token không tồn tại (`--sacc-ok-12`, `--sacc-warn-16` → `rgba(0,0,0,0)`) | `--kit-ok-12` (đúng `.12` artboard) và `--frame-warn-14` (gần nhất với `.16`; không có `--kit-warn-16`); thêm `.sacc-qr__step--done { color: --kit-fg-sub }` và `--now { color: --color-fg }` đúng mực chữ artboard | Đo live scanned: done `rgba(166,237,193,.12)`/ink `#A6EDC1`/text `rgb(139,139,151)`; now `rgba(246,203,140,.14)`/ink `#F6CB8C`/text `rgb(237,237,239)` = `#EDEDEF` |
| Glyph `scanner` → `scan` (i-scan góc quét) | 3 chỗ: `BilibiliLogin` ×2 (manual + empty), `QrLogin` head | `mask-image: …/icons/scan.svg` |
| Close glyph 16 → 14 (nút vẫn 36) | `QrLogin.tsx` `Icon name="close" size={14}` | Đo live: `--x-width: 14px`, nút 36 |
| Mực QR nhỏ hơn artboard (quiet zone 4 trong khung đã có padding 14px) | `identity.QRSVGQuiet(text, quiet)` mới trong `platform/identity/qr.go`; `StartQR` gọi `quiet=2` (khung 200 + padding 14 → tổng lề trắng ≥2 module vẫn quét được). `QRSVG` mặc định vẫn 4 → Noverse không đổi | `TestQRSVGQuietZone` khẳng định ink bắt đầu đúng module `quiet` (assert margin parameter — `rsc.io/qr` chỉ encode, không có decoder trong go.mod) |

Font provider (SF vs Be Vietnam Pro) — không xử lý theo chỉ dẫn QA3.

**Tệp đổi vòng QA3:** `accounts.css` (select 100%, badge tokens, mực step); `BilibiliLogin.tsx`, `QrLogin.tsx` (icon `scan`, close 14); `platform/identity/qr.go` + `qr_test.go` (API mới `QRSVGQuiet`, test margin); `internal/accounts/bilibili_qr.go` (dùng `quiet=2`); `AccountsSection.test.tsx` (+2 test hợp đồng CSS: select full-width, badge token + guard “không var() mồ côi --sacc-*”).

**Checks QA3:** `tsc` sạch; `vitest` accounts **21/21** (gồm 2 test mới); Go: `gofmt` (go1.26.0), `build`, `vet`, `go test` trên `platform/identity`, `internal/accounts`, `archtest` — xanh; `-race -p 1` trên 2 gói chạm xanh.

Status: DONE
Summary: [QA3] Đã sửa hết 6 điểm: select YouTube thành hàng riêng rộng 726px đúng artboard (đo lại khớp số QA3), badge step QR dùng token thật (done/now đo lại đúng .12/.14 với mực chữ artboard), icon scanner→scan, close 16→14, quiet zone QR 4→2 module qua API mới QRSVGQuiet (Noverse giữ 4). tsc sạch, 21/21 test accounts, Go test+race xanh.

[Trước] Accounts đã khớp artboard ở cả 1440×900 và 390×844 cho toàn bộ tweak `bili` (5), `yt` (bot + hồ sơ Tắt) và `qr` (7 desktop / 6 mobile): word-diff DOM app vs DOM artboard bằng 0 khác biệt ngoài giá trị sống; thẻ điện thoại cao đúng 287 px, tag 22, tile 36, nút hàng 44, CTA sheet 48/r13/15/600. Vòng này sửa lệch mực chữ phụ (`#7E7E8A` → `#8B8B97` đúng 13 chỗ), giãn dòng 1,5 trong thẻ điện thoại, hint sheet 11,5 px, thêm banner bot cho hàng YouTube điện thoại, và thay CSS nút tự chế bằng kit `xl`/`lg`. Kích thước nút desktop giữ `sm` (30) + `md` (36) theo đúng DOM artboard — đã phản biện và được Main chốt lại luật per-spot. 52 ảnh cặp trong `/tmp/viderse-fidelity/FidelityAccounts/shots/`. Test/build/size xanh; 1 test `design-rules` fail thuộc file của FidelityConnect.
