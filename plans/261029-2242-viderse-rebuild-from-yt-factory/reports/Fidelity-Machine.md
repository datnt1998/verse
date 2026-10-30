# Fidelity-Machine — Khung Cài đặt · Công cụ · Báo lỗi · Chuẩn bị máy · MSettings

Vòng QA: `reports/QA2-Machine.md` (16 major + ~40 minor). Artboard là nguồn đúng.
Ảnh chụp 1440×900 (desktop) và 390×844 (MSettings) trong
`/tmp/viderse-fidelity/FidelityMachine/shots/`.

Cách chụp lần này khác vòng trước: **server thật** (DB `viderse_smoke_machine` dựng rồi xoá,
binary `go build`, đăng nhập bằng form thật), và chỉ **vá đúng những trường không dựng được
offline** bằng `page.setRequestInterception` trong một `tab.run` — payload lấy từ
`/api/tools` thật rồi sửa `state/hint/progress/sizeBytes`. Không còn fixture bịa cả khối
JSON, nên các lỗi “copy do mock” của vòng trước (Chrome purpose, phiên bản, câu đã cài) hết
hẳn: những câu đó giờ là chuỗi thật của `internal/toolsvc`.

## Bảng trạng thái (artboard → app → kết quả)

| Artboard | App | Kết quả |
|---|---|---|
| `Settings.dc.html?section=tools&tools=ok` | `#/settings?section=tools` (vá `state=ready`) | **Khớp**: khung, sidebar nhóm/đếm, thẻ tổng “Máy sẵn sàng”, 6 hàng (tên + chấm 10px + việc + dung lượng + phiên bản), “Nâng cao · thư mục dữ liệu và phiên bản” là **hàng cuối trong cùng thẻ** (khe 0px, viền trên 1px `rgba(255,255,255,.06)`, thẻ bo 14). `tools-ok.png` |
| `…&tools=update` | vá `state=update` | **Khớp**: pill “Có bản mới”, câu “Có bản 2026.10.22 · bạn đang dùng 2026.10.09. YouTube hay đổi cách tải, nên cập nhật.”, nút ghost “Cập nhật”, thẻ tổng “Máy sẵn sàng · có 1 bản cập nhật”, chấm sidebar vàng. `tools-update.png` |
| `…&tools=failed` | vá `state=failed` | **Khớp**: hàng nền cam, icon cảnh báo, câu lỗi + hậu quả theo bước, pill “Chưa cài được”, nút “Tải tiếp”, thẻ tổng “1 công cụ cần sửa”, **chấm sidebar cam `--color-err`** (trước là vàng nhạt). `tools-failed.png` |
| `…&tools=installing` | vá `state=installing` | **Khớp**: “Đang cài 3/6 · còn khoảng 1 phút”, dải trượt + phase **viết hoa** “Đang tải mô hình nhận giọng nói · 806 MB / 1,3 GB”, pill “Đang cài” có spinner, hàng sau lượt cài là pill **“Chờ”** (không nút), sidebar đếm “3/6”. `tools-installing.png` |
| `…&section=logs` | `#/settings?section=logs` (thật) | **Khớp** cấu trúc/copy: “Xuất log để báo lỗi”, “Lỗi gần đây · 7 ngày qua”, thẻ phiên bản, rail “Gửi log thế nào” + “Mẹo”. Thẻ phiên bản giờ hiện **semver** như chân cột (“Viderse 0.0.0”). Thiếu ngày phát hành — xem hỏi-đáp #3. `frame-logs.png` |
| `…&section=storage|connect|agents|accounts` | cùng route (thật) | Khung khớp từng pixel (bảng đo dưới). Nội dung thuộc Storage/Connect/Accounts. `frame-{storage,connect,agents,accounts}.png` |
| `Onboard.dc.html?step=setup&setup=running` | `#/setup` (vá state) | **Khớp**: header “Viderse · Lần đầu mở” + “Cài đặt nâng cao”, hero **lens hero** (cung dày 4,6, cầu 16, không vòng kính), 4 chip, thẻ tiến độ, thẻ 6 bước, **chấm 10px sau mỗi tên bước**, hàng đang chạy nền xanh + dải xanh + “62%”, “Có sẵn” cho Chrome, hàng ChatGPT ghost “Đăng nhập ngay”, câu chân. `setup-running.png` |
| `…&setup=login` | vá state=login | **Khớp**: “Còn một bước cần bạn”, 5/6, hàng ChatGPT focus nền hồng + nút primary “Đăng nhập ChatGPT”. `setup-login.png` |
| `…&setup=failed` | vá state=failed | **Khớp**: “Một bước chưa cài được” + 4/6, hàng lỗi nền cam **không còn dải tiến độ bên trong** (artboard `hasBar:false`), lý do + “Đã giữ 806 MB”, nút “Thử lại”. `setup-failed.png` |
| `MSettings.dc.html?screen=list` (390×844) | `#/settings` (vá update) | **Khớp**: header 52px, **back 44×44 góc 12 → tiêu đề x=52**, thẻ tổng, 6 hàng icon/nhãn/giá trị + chấm, thẻ “Luồng chạy…”, chân “Viderse 0.0.0 · cài đặt áp dụng cho máy này”. `msettings-list.png` |
| `?screen=tools` (390×844) | `#/settings?section=tools` | **Khớp**: câu dẫn, hàng update với câu nhắc **vàng ấm** “YouTube hay đổi cách tải, nên cập nhật để tải không lỗi.” (2 dòng), nút trải ngang 44px “Cập nhật · 52,4 MB”, mô tả mobile ngắn (“Chép lời thoại thành chữ ngay trên máy”, “Dùng Chrome có sẵn trên máy, chạy ngầm”, “Vẽ ảnh cho từng cảnh, chạy tách riêng”) ghim 2 dòng, thẻ “Chi tiết · phiên bản và thư mục” **tách thẻ**. `msettings-tools.png` |
| `Settings` nút desktop | `#/settings?section=tools` + `logs` | **Khớp**: “Kiểm tra lại” và hành động hàng đo `32 / r8 / 13 / fw400 / pad12` (artboard `ghost btn` 32/8/13/400/12); “Xuất log” giữ `36 / r10 / 13 / 600 / pad14` (artboard `.btn primary`). `tools-ok.png`, `frame-logs.png` |
| `System › offline` (khung có dải mất kết nối) | `#/settings?section=tools` rồi tắt server | **Khớp**: dải 37px ở y=55, `.vv-set` co lại còn `y=92, h=808` và **không đẩy cột nào khỏi màn hình**; nội dung mờ 42% như artboard. Ảnh: `frame-offline.png` (nếu thiếu, số đo ở mục “Khung / điều hướng” là bằng chứng). |

## Đã sửa theo QA2

**Khung / điều hướng**
- Màn Cài đặt **không** sáng mục nào ở nav trên (đo: `.vv-nav__item--active` = 0) — ảnh QA2
  nhìn thấy “Bàn dựng” sáng là ảnh cũ; đo lại ở cả 6 mục đều 0.
- Nav trên là một nhóm kín: `[464,11,512,33]` (giữa khung đúng 720), viền 1px, nền `#131315`.
- Sidebar: icon đủ (ảnh cũ chụp trước khi mask SVG tải xong — harness mới đợi mọi
  `mask-image` trong DOM giải mã xong mới chụp), nhịp nhóm 16px, hàng 34px, chấm attention
  **có kiểu**: `login_expired`/`tool_failed` → `--color-err` (cam #FF9A5C), còn lại
  `--color-warn`. `useSectionDots()` trả `Map<section,'warn'|'err'>` (một chấm/mục).
- Chân cột: chỉ **semver** (`lib/version.ts:semverOf`) nên không còn xuống 3 dòng vì chuỗi
  `v0.0.0-…+dirty`; thẻ phiên bản ở Báo lỗi dùng chung hàm đó.
- `.vv-set` giữ `height: calc(100dvh - var(--vv-h-topbar))`; với dải offline (37px) đo được
  `y=92, h=808` — cột trái/phải vẫn đủ cao, không tràn.
- Skeleton: `App.tsx` chỉ dùng `SystemLoading` (3 thẻ Bàn dựng) cho route workbench; route
  khác (Cài đặt, Chuẩn bị máy) chờ bằng spinner giữa khung → không còn “Đang tải Bàn dựng”
  trên màn Cài đặt.

**Công cụ**
- **Nâng cao nằm trong thẻ danh sách** (nguyên nhân 16px: nó là sibling trong flex `gap:16px`;
  `margin-top:-1px` chỉ bù được 1px). Giờ desktop render `<Advanced>` là con cuối của
  `.vv-tools__list` (đo: khe 0px, list bo 14, `border-top` 1px); mobile vẫn là thẻ riêng.
- `phaseLabel` mới trên wire (`installProgressWire.PhaseLabel`, tính từ
  `toolsvc.PhaseLabel`): web hiện thẳng câu tiếng Việt thay vì enum `downloading`.
- Hàng `missing` **đứng sau** lượt đang cài → pill “Chờ”, không nút (đúng `tools=installing`);
  hàng trước lượt cài vẫn “Chưa cài” + nút “Cài”. Có test mới.
- Mobile: bộ câu `descsMobile` (không phiên bản — artboard để phiên bản ở thẻ Chi tiết),
  câu update dùng `--vv-note-ink` (#F3D9AE) thay xám, mô tả ghim 2 dòng, màu phụ
  `--vv-fg-note` (#8B8B97).
- Desktop giữ nguyên câu server (đã có “· bản X” / “· mô hình giọng nói đã tải”), nên các
  lỗi “thiếu câu đã cài”, “thay purpose của Chrome bằng câu khác” của vòng trước biến mất —
  chúng đến từ payload mock, không từ code.

**Chuẩn bị máy**
- Hero dùng biến thể `LensLogo hero`: cung 4,6 (trước 2,6), cầu 16, bỏ vòng kính + vệt sáng
  → khớp “cung dày” của artboard.
- **Chấm 10px sau tên bước**: `AgentOrb` chỉ đặt `width/height`, mà span mặc định `display:
  inline` nên kích thước bị bỏ qua (chỉ hiện khi nằm trong flex). `Orb` giờ đặt
  `display:inline-block` → cả 6 chấm hiện đúng màu (lam/lục/lam/lục/vàng/vàng).
- Hàng lỗi bỏ dải tiến độ bên trong (và xoá CSS `.is-fail` chết theo).
- Phase trong hàng đang chạy dùng `phaseLabel` → “Đang tải mô hình nhận giọng nói” (viết hoa).

**MSettings**
- Back 32 → **44×44, góc 12** (`.ib` của artboard), tiêu đề lùi về x=52.
- (Đã có từ vòng trước) bỏ câu dẫn ở màn danh sách, nút 44px trải ngang, thẻ Chi tiết riêng.

## Kit (`platform/ui`) — thay đổi và bằng chứng Noverse

- `Sheet` thêm `size="md"` (góc 22, `--color-card`, viền `--line-3`, cao tối đa 86%,
  grabber 36×4 trên hàng 8/2, tiêu đề 16/600 khối `6/16/10` gap 3), `subtitle` (12,5px,
  token mới `--kit-fg-sub` #8B8B97), `homeIndicator` (134×5). Mặc định `size="lg"` giữ
  nguyên tấm Noverse (26/#18181A/18); mọi selector biến thể có tiền tố `.m-sheet--md`.
  FidelityConnect đang dùng cho sheet Kết nối AI + Trợ lý AI.
- `Banner` thêm `align="top"` (nút 44px canh đầu khối) và `size` cho action.
- `Button` thêm `size="compact"` (32 / pad 12 / r8 / 13px / fw 400 / gap 7) = `.btn` của
  `Settings.dc.html` dùng cho nút `ghost btn` (Công cụ “Kiểm tra lại” + hành động hàng, Báo lỗi
  “Kiểm tra bản mới”, Connect toàn bộ, dọn dẹp Lưu trữ) và `size="xl"` (48 / pad 16 / r13 /
  15px / 600 / gap 8) = CTA chính của điện thoại. `sm` (30/r8/12) vẫn là `vv-btn--sm` của
  artboard, `md` (36/r10/13) là `.vv-btn`/hộp thoại — **chọn theo class của artboard tại chỗ**,
  không theo mục.
- `Button variant="warnGhost"` = `.ghost.btn-warn` (nền trong, viền `--kit-errb-45`, chữ
  `--kit-err-ghost-ink`, hover `--kit-err-8`); `defineBrand.danger.ghostInk` (Viderse #FFC09A).
- `ConfirmDialog` thêm `size` (mặc định `'md'` — Noverse không đổi) cho chân hộp thoại.
- `Button` `size="lg"`, `--kit-err-deep-fg`, `defineBrand.danger` (remap thang lỗi sang cam
  Viderse; không truyền thì Noverse không đổi một biến).
- `hardcodedColors.baseline.json`: +2 mã ở `kit.css` (grabber `.22` và home indicator
  `#EDEDEF` — cả hai là giá trị artboard, đã có sẵn cùng dạng trong file).
- Kit: **147/147 test xanh**; Noverse không đổi vì mọi thay đổi là opt-in.

## Đề xuất hỏi-đáp

1. **“Máy sẵn sàng · 2/2 luồng”** — thuộc phase Hàng chờ (W9/L5): `/api/status` chưa có
   `lanes` nên thẻ tổng không hiện số luồng (không bịa). Khi có `lanes`, khôi phục câu.
2. **“Giới hạn từng kênh”** (sidebar, nhóm “Ở trang khác”) — thuộc phase Kênh; router chưa
   có `#/channels/:id?tab=sources` nên không dựng, không thay bằng link giả.
3. **Ngày phát hành ở thẻ phiên bản Báo lỗi** — artboard “Bản mới nhất · cập nhật
   20/10/2026”; `/api/version` chưa trả ngày phát hành nên chỉ hiện “Bản mới nhất”.
4. **Chấm attention theo tông** — cần `kind` trong `/api/status.attention` (đã có:
   `tool_failed`, `login_expired`, `tool_update`, `quota_low`); mình đang suy tông từ
   `kind` ở client. Nếu sau này thêm loại mới, cần chốt bảng kind → tông trong hỏi-đáp.
5. **Dải offline** — artboard System không đặt dải trong khung Cài đặt; mình cho dải chen
   giữa header và nội dung (`.vv-set` co lại). Nếu muốn dải phủ lên nội dung thay vì đẩy
   xuống, cần một artboard khác.
6. **`phaseLabel` trên wire** — thêm trường vào `installProgressWire` (dùng
   `toolsvc.PhaseLabel`) để web không phải dịch enum. Nếu sau này có client khác, cần chốt
   rằng nhãn pha là **copy tiếng Việt của server**, không phải enum.

## Kiến trúc & hiệu năng (số đo)

- `npx tsc --noEmit`: sạch. `vitest`: **148/148** (15 file) — có 1 test mới cho hàng “Chờ”,
  1 test Nâng cao sửa lại theo cấu trúc mới. `design-rules` 6/6 (hex trong comment đã bỏ;
  lỗi `margin: -1px` còn lại thuộc `connect.css` — Connect đã sửa trong lúc mình chạy).
- Kit: **152/152** (thêm test hợp đồng `compact`, `xl`, `ConfirmDialog size`, `warnGhost`, và thứ tự weight của `compact`).
- `npm run build` xanh; **JS tải lần đầu 125,9 kB gzip** (1 chunk, ngưỡng 250 kB);
  tổng `index` 428,66 kB → 129,19 kB gzip.
- Chunk lazy của mục Cài đặt: `SettingsScreen` 11,5 kB (3,4 gzip), `ToolsSection` 7,3 (2,5),
  `SetupScreen` 7,4 (2,6), `ConnectSection` 8,2 (2,9), `AgentsSection` 18,0 (5,4),
  `AccountsSection` 21,2 (6,3), `StorageSection` 26,5 (6,9) — mỗi mục vẫn một chunk riêng,
  mở Cài đặt không kéo nội dung mục khác.
- DOM: bảng công cụ 6 hàng, màn logs 131 node, danh sách mobile 6 hàng + 1 thẻ queue — dưới
  ngưỡng 200. Không thêm subscription nào: `useFrameFacts` đọc 7 khoá cache có sẵn.
- GET: mở 6 mục không phát sinh request lặp (khoá `tools/storage/errors/version/aiProviders/
  aiAgents/platforms` dùng chung). Không thêm GET mới.
- Go: `internal/toolsvc/install.go` (export `PhaseLabel`), `internal/web/progress.go`
  (+`phaseLabel` trong wire). `go build ./internal/...` sạch, `go vet` sạch,
  `go test -p 2 ./internal/toolsvc/... ./internal/web/...` xanh,
  `-race -p 1` xanh (toolsvc 1,6s · web 72,5s), `./archtest/...` xanh.
- Smoke: DB `viderse_smoke_machine` (đã `dropdb`), server 18791 (đã dừng), Chrome headless
  riêng (đã đóng). Không đụng cổng/DB của agent khác.

## Ảnh cặp

`/tmp/viderse-fidelity/FidelityMachine/shots/` (giữ tới khi integrator xác nhận):
`frame-{tools,connect,agents,accounts,storage,logs,offline}.png`,
`tools-{ok,update,failed,installing}.png`, `setup-{running,login,failed}.png`,
`msettings-{list,tools}.png` — mỗi ảnh app có ảnh artboard cùng tên `qa-art-*.png`.

## Tệp đã sửa (vòng này)

- web: `App.tsx`, `components/{LensLogo,Orb}.tsx`, `lib/version.ts` (mới),
  `screens/settings/SettingsScreen.tsx`, `screens/settings/tools/{ToolsSection.tsx,tools.css,ToolsSection.test.tsx}`,
  `screens/settings/logs/LogsSection.tsx`, `screens/setup/{SetupScreen.tsx,setup.css}`,
  `i18n/tools.ts`, `api/{tools,setup}.ts` (kiểu `phaseLabel`).
- web (QA3 vòng 3): `screens/setup/{SetupScreen.tsx,SetupScreen.test.tsx}`, `i18n/setup.ts`,
  `screens/settings/tools/{ToolsSection.tsx,ToolsSection.test.tsx,tools.css}`,
  `screens/settings/logs/{LogsSection.tsx,LogsSection.test.tsx,logs.css}`,
  `screens/settings/{SettingsScreen.tsx,SettingsScreen.test.tsx,settings.css}`,
  `styles/components.css`, `api/setup.ts`, `shell/AppShell.tsx`, `checks/design-rules.test.ts`.
- web (vòng cỡ nút + dọn CSS): `screens/settings/settings.css` (chỉ còn frame),
  `screens/settings/tools/{ToolsSection.tsx,tools.css}`, `screens/setup/SetupScreen.tsx`,
  `styles/components.css` (`.vv-errline`, `.vv-grow`), `brand.ts` (`danger.ghostInk`).
- kit: `platform/ui/src/{Sheet.tsx,kit.css,tokens.stylex.ts,tokenDefaults.ts,hardcodedColors.baseline.json,Button.tsx,ConfirmDialog.tsx,brand.ts,kitAdditions.test.tsx}`.
- Go: `internal/toolsvc/install.go`, `internal/web/progress.go`.
- Ảnh hưởng từ agent khác trong lúc chạy: `settings.css`/`SettingsScreen.tsx` (Connect rút
  gọn chân cột — mình giữ và chỉnh lại indent), `connect/connect.css` (Connect tự sửa
  `margin` số cứng).

## CSS: `settings.css` còn frame-only (bổ sung)

Khi FidelityStorage dọn CSS cục bộ xong, mình dọn nốt phần của mình để `settings.css` chỉ còn
khung Cài đặt (`vv-set*`, `vv-dot*`, `vv-mset*`) — mỗi mục một file cạnh nó:

| Rule cũ trong `settings.css` | Xử lý |
|---|---|
| `.vv-set__card*` | → `tools/tools.css` đổi tên `.vv-tools__card*` (chỉ Công cụ dùng; là thẻ báo lỗi tải) |
| `.vv-set__grow` | → `.vv-grow` trong `styles/components.css` (Tools + Setup cùng dùng) |
| `.vv-errline` | → `styles/components.css` (Tools + Setup + Lưu trữ cùng dùng; nạp toàn cục ở `main.tsx`) |
| `.vv-row*`, `.vv-details*` | xoá — không còn file nào dùng (đã đối chiếu toàn `web/src`) |
| `.vv-confirm*` | đã xoá — FidelityStorage chuyển thành `.vv-sto__confirm*` trong `storage.css` và thay `__btn` bằng `Button variant="danger" size="compact"` (đo lại 32/13/600). |

Hệ quả: `SetupScreen.tsx` không còn `import '../settings/settings.css'` — trước đó màn Chuẩn bị
máy phụ thuộc CSS của khung Cài đặt. Kiểm lại import graph: chỉ `SettingsScreen.tsx` nạp
`settings.css`; mỗi mục có chunk CSS riêng (`ToolsSection-*.css`, `StorageSection-*.css`, …).

Đo lại sau khi chuyển (probe trong DOM, app thật): `.vv-tools__card` = bo 14 / pad 16 / gap 12 /
column (y hệt `.vv-set__card` cũ), `.vv-grow` = flex-grow 1, `.vv-errline` = `#FFC09A` trên
`rgba(255,154,92,.07)` / pad 10 / bo 10. Ảnh `setup-running.png` (chụp lại sau khi bỏ import)
vẫn đủ khung: header, orb, 4 chip, thẻ tiến độ, 6 hàng, footnote.

## QA3 vòng 3 — sửa 2 major + 4 minor (+2 frame)

### Major 1 — máy mới không tự khởi lượt cài
`SetupScreen.tsx` chặn bằng `!data.complete`, tức chặn đúng máy mới (máy mới luôn
`complete=false`). Điều kiện đúng: `mobile || !data || data.complete || !data.disk.enough` → return;
`state === 'idle'` thì POST start. Thêm hai việc đi kèm:
- **Thẻ tiến độ không còn giả “Đang cài”**: `idle` → “Đang bắt đầu…” (đang gọi) hoặc “Chưa bắt
  đầu”; POST start lỗi → “Không bắt đầu được” + nguyên văn lỗi, tông cảnh báo.
- **Ảnh chụp POST start ghi thẳng vào cache** (`setData(keys.setup(), next)`): trước đó server đã
  `running` mà thẻ vẫn “Chưa bắt đầu” và các hàng vẫn “Chờ lượt” cho tới lần SSE sau.

Test (đều đỏ trước khi sửa): idle+chưa xong+đủ chỗ → đúng **một** POST start; đang chạy/đã xong →
không POST; mobile → không; thiếu chỗ → không; ảnh chụp POST start đổi thẻ sang “Đang cài”.

**Repro trên binary thật** (HOME mới, DB `viderse_smoke_fm`, Chrome headless `--use-mock-keychain
--password-store=basic`, profile riêng, 1440×900, **không** chặn request nào):
1. Trước khi mở `#/setup`: `GET /api/setup/state` = `idle / running=false / complete=false /
   disk.enough=true` (free 9,6 GB ≥ cần 4,7 GB).
2. Đăng nhập → app tự vào `#/setup` → đọc lại ngay: **`running / running=true`**, bước đang chạy
   `ffmpeg`. Không hề POST tay.
3. Sau 1,5 s và 6 s: vẫn `running`; giao diện “Đang cài”, 3/6 xong; khi SSE chạy đúng (proxy
   streaming) các hàng khớp server: `done,done,running,done,done,needs_user`.
Bằng chứng: `shots/verify-autostart-running.png` (thẻ “Đang cài 3/6 xong”, hàng ffmpeg chạy
50% với “Đang tải · 45,6 MB / 91,1 MB”) + `shots/verify-autostart.json` (đọc trước/sau và
`appStartRequests: 1` — đúng một request do app phát).

### Major 2 — MSettings: hàng ChatGPT đã đăng nhập dù server nói cần đăng nhập
`hintMobile` thay câu của **mọi** trạng thái bằng `descsMobile`, kể cả `needs_login`. Giờ
`needs_login` giữ câu server (`tool.hint`) — đo trong app 390×844: hint “Ảnh được vẽ bằng tài
khoản ChatGPT của bạn, nên bước này cần bạn tự đăng nhập.” + nhãn “Cần bạn”, không còn câu
“Đã đăng nhập · quản lý ở Kết nối AI”. Ảnh: `shots/verify-msettings-tools.png` (390×844,
thẻ Chi tiết mở: “Thư mục dữ liệu · yt-dlp · ffmpeg · ffprobe · faster-whisper · Chrome · Codex”).

### Minor
- **tools=failed**: thêm `.vv-tools__hint.is-err` (mực `--vv-warn-ink`, cùng tông chữ cam của
  artboard `toolsBad`) — component đã gắn `is-err`, CSS thiếu.
- **logs**: lỗi chưa xử lý đổi từ vàng warn sang **cam lỗi** — chấm `vv-dot--err`, nhãn
  `.vv-logs__err-st.is-err` với `var(--color-err)` (#FF9A5C); không hex cứng.
- **Sidebar, chấm lệch 11px**: artboard luôn render span số cuối hàng (kể cả rỗng) nên chấm
  không trôi vào chỗ đó. Giờ luôn render `<span class="vv-set__nav-num">`; đo lại dot x = **211**
  (artboard 210–217, trước 221).
- **Sidebar, nhịp hàng lệch 2px** (R1): `.vv-set__grp` 11px → **11.5px** như artboard `.grp`
  (mỗi nhãn cao thêm 1px; hai nhóm trên Lưu trữ = 2px). Đo lại ở 1440×900: hàng chọn
  tools/connect/agents/accounts/storage/logs = **97/133/169/205/281/317**, trùng artboard
  từng trị số (trước: 96/132/168/204/279/315).
- **MSettings, Chi tiết mở rộng**: bỏ `.vv-tools__adv-k { display:none }` ở mobile — tên công cụ
  hiện cạnh phiên bản (`yt-dlp 2026.08.19 · ffmpeg · ffprobe 6.0 · faster-whisper 1.2.1 ·
  Chrome 156.0.8078.74 · Codex 0.162.1`), như artboard.

### Frame R2 — header co 55 → 48–51px (Storage + Connect)
`.vv-header` chỉ có `min-height`, là flex item trong cột cao `100dvh` nên bị co khi nội dung
dưới tràn. Thêm `flex-shrink: 0`. Đo ở 1440×**900 / 500 / 420** trên cả 6 mục: header luôn
**55px** (trước: 48–51 khi tràn). Nội dung dài giờ do cột tự cuộn, không kéo header.

### Kiểm tra
`tsc --noEmit` sạch · vitest **167/167** (gộp cả nguồn Storage/Accounts/Connect) · CSS pin trong `checks/design-rules.test.ts` (grp
11.5px, header `flex-shrink: 0`, `.vv-tools__hint.is-err`, adv-k không bị ẩn ở mobile, logs
`is-err` dùng `--color-err`). Ba test mới của 2 major đã được chứng minh **đỏ trước khi sửa**
(tạm hoàn nguyên code rồi chạy lại).

**Dist dùng chung** (một lần, sau khi Machine + Storage + Accounts + Connect đều báo
“source ready”): `internal/web/dist/assets/index-CndEvHKK.js`, **14:48:28**, first-load
**126,1 kB gzip** (1 chunk).

**Giới hạn bằng chứng**: đĩa máy đang 100% (còn 4,0 GB). Lần đọc PRE của repro rơi đúng lúc
`enough=false` (4,0 GB < 4,71 GB cần) nên màn **không** tự khởi — đúng nhánh `!disk.enough`.
Vài giây sau dung lượng trống lên 5,24 GB (`enough=true`) và màn tự khởi như mô tả ở trên;
`verify-autostart.json` ghi cả hai lần đọc, không giấu chỗ đổi. Không chụp được ảnh “PRE =
idle” vì lúc đó app chưa mount.

## QA3: đo lại trên bản build sau đóng băng

Chụp lại toàn bộ trên binary dựng từ dist `index-hsmX0DF6.js` (13:56), server thật, DB mới:

| Artboard | Đo trong app | Artboard | Kết luận |
|---|---|---|---|
| `Settings?section=tools&tools=ok` → “Kiểm tra lại” | 32 / r8 / 13 / 400 / pad12 | `.ghost btn` 32 / r8 / 13 / 400 / pad12 | khớp |
| `Settings?section=tools` → hành động hàng | 32 / r8 / 13 / 400 | 32 / r8 / 13 / 400 | khớp |
| `Settings?section=logs` → “Xuất log” | 36 / r10 / 13 / 600 / pad14 | `.btn primary` 36 / r10 / 13 / 600 / pad14 | khớp |
| `Settings?section=logs` → “Kiểm tra bản mới” | 32 / r8 / 13 / 400 | 32 / r8 / 13 / 400 | khớp |
| `Onboard?setup=failed` → “Thử lại” | 32 / 13 / 400 | `ghost` 32 / 13 / 400 / r9 | khớp (r8 vs r9: 1px) |
| `Onboard?setup=running` → “Đăng nhập ngay” | 32 / 13 / 400 / r8 | `ghost` 32 / 13 / 400 / r9 | khớp (1px) |
| `Onboard?setup=login` → “Đăng nhập ChatGPT” | **32** / 13 / 600 / r8 | `primary` **36** / 13.5 / 600 / r10 | **lệch 4px** — xem “Nút đăng nhập ChatGPT” bên dưới |

`Onboard?setup=running` trước đó mình chụp thiếu nút “Đăng nhập ngay” là lỗi harness (mình đặt
`codex-login: pending` thay vì `needs_user`); backend thật trả `needs_user` song song với các
bước đang cài, và app render đúng như artboard. Đã chụp lại bằng payload đúng hình `setupWire`.

## Nút đăng nhập ChatGPT: một component, hai khung

FidelityConnect đã cho `ChatGPTLoginAction` nhận `size` (mặc định `compact`) thay vì đổi cứng
một cỡ — đúng, vì artboard dùng **hai** cỡ cho cùng một hành động:

| Chỗ | Artboard (đo) | Cần |
|---|---|---|
| `Settings?section=connect&grok=code` → “Mở trang xAI” (`class="primary"`) | 32 · r8 · 13 · 600 · pad13 | `compact` (mặc định của họ) |
| `Onboard?step=setup&setup=login|failed` → “Đăng nhập ChatGPT” (`class="primary"`) | 36 · r10 · 13.5 · 600 · pad16 | `md` |
| `Onboard?setup=running` → “Đăng nhập ngay” (`class="ghost"`) | 32 · r9 · 13 · 400 | `compact` |

Đã áp `screens/setup/SetupScreen.tsx:108`:
`size={focus ? 'md' : 'compact'}` (nhánh `primary` = `md`, ghost giữ `compact`). `tsc` sạch,
vitest **148/148**. Ảnh trong `shots/` của trạng thái `setup=login|failed` là **trước** bản sửa
(32px); Main rebuild dist và QA3Machine chụp lại `#/setup` — kỳ vọng `primary` 36 · r10 · pad14
· 13 · 600 (artboard 36 · r10 · pad16 · 13.5 · 600), ghost vẫn 32 · r8 · 13 · 400 (artboard r9,
lệch 1px).

## Sửa thứ tự style của `Button` (FidelityStorage báo)

`compact` khai `fontWeight: 400` nhưng đứng **sau** variant trong `stylex.props`, nên
`primary`/`danger` + `compact` bị đè về 400 (artboard: `.btn.primary` và nút xác nhận phá huỷ
là 600). Đã chuyển `size === 'compact' && styles.compact` lên ngay sau `styles.base`; `sm/lg/xl`
giữ nguyên chỗ cũ (chúng không đặt weight nên không có xung đột).

Kết quả: `primary|danger` + `compact` = 600, `ghost|quiet|warnGhost` + `compact` = 400,
`sm/md/lg/xl` không đổi, Noverse không đổi (không dùng `compact`). Test mới trong
`kitAdditions.test.tsx` dựng atom `fontWeight 600/400` bằng `stylex.create` rồi khẳng định
đúng atom có mặt — đỏ nếu ai đó đảo lại thứ tự. Kit **152/152**, `tsc` sạch.

FidelityStorage đo lại trong app: nút xác nhận `32/13/600` nền `#FF9A5C` mực `#1A0E06`,
“Dọn” `32/13/600` trắng — khớp artboard.

## Cỡ nút (bổ sung sau QA2)

`Settings.dc.html` có **hai** hệ nút, và `.btn{32}` không phải “nút duy nhất”: nó là hệ của
`class="ghost btn"` (19 chỗ), còn `class="vv-btn …"` là hệ 30/36. Chọn theo đúng class tại chỗ:

| Artboard | Đo được | Kit |
|---|---|---|
| `ghost btn`, `ghost btn btn-warn` | 32 · pad 12 · r8 · 13px · fw400 | `size="compact"` (+ `variant="warnGhost"` cho bản warn) |
| `vv-btn vv-btn--sm` | 30 · pad 10 · r8 · 12px | `size="sm"` |
| `vv-btn`, `.btn primary` | 36 · pad 14 · r10 · 13px | mặc định `md` |
| `vv-btn--m` (điện thoại) | 44 · r12 · 14px | `size="lg"` |
| `vv-btn--m-lg` (điện thoại, primary) | 48 · pad 16 · r13 · 15px/600 | `size="xl"` |

Đã chuyển: Công cụ (“Kiểm tra lại”, hành động hàng) và Báo lỗi (“Kiểm tra bản mới”) →
`compact`; “Xuất log” giữ `md` (artboard là `.btn primary` 36). Đo lại trên app:
`Kiểm tra lại` = 32/r8/13/400/12 (artboard 32/r8/13/400/12, rộng 120 vs 118 do metric chữ),
`Xuất log` = 36/r10/13/600/14 (khớp). Chuẩn bị máy: nút ghost 32 → `compact`, nút primary
“Đăng nhập ChatGPT” giữ `md` (artboard 36/r10/16).

## Vòng QA3 thứ ba (2 major + 4 minor + 2 frame)

Tất cả đã sửa, mỗi lỗi có test đỏ-trước-khi-sửa (đã chứng minh bằng cách tạm hoàn nguyên):
guard tự-khởi bị đảo (máy mới không bao giờ bắt đầu cài), thẻ tiến độ giả “Đang cài” khi idle,
ảnh chụp POST start không vào cache, `hintMobile` đè câu `needs_login`, thiếu
`.vv-tools__hint.is-err`, logs dùng vàng thay cam lỗi, chấm sidebar lệch 11px, nhịp sidebar lệch
2px, `.vv-tools__adv-k` bị ẩn ở mobile, header co 55→48px khi nội dung tràn.

Đo lại trong app thật: header **55px** ở 900/500/420 cho cả 6 mục; hàng chọn sidebar
**97/133/169/205/281/317** (artboard y hệt); chấm x=**211** (artboard 210–217); máy mới
`idle→running` không cần POST tay.

Status: DONE_WITH_CONCERNS
Summary: 16 major của QA2 đều xử lý: “Nâng cao” giờ là hàng cuối **trong** thẻ danh sách
(khe 0px — nguyên nhân là flex gap 16, không phải margin), chấm sidebar đúng tông (cam khi
công cụ lỗi), phase tiếng Việt viết hoa qua trường `phaseLabel` mới trên wire, hàng “Chờ”
trong lúc cài, hero lens dày + chấm 10px từng bước (bug `display:inline` của orb), bỏ dải
tiến độ ở hàng lỗi, mobile có bộ câu ngắn riêng + tông vàng ấm cho câu cập nhật + ghim 2 dòng,
back 44×44, và skeleton “Đang tải Bàn dựng” không còn xuất hiện trên màn Cài đặt.
Cỡ nút đã chốt theo class artboard: kit thêm `compact` (32/r8/13/400) và `warnGhost`, đổi
Công cụ/Báo lỗi sang `compact`, giữ `md` cho “Xuất log” (`.btn primary` 36) — đo trên app khớp
từng trị số. `settings.css` giờ chỉ còn frame; `.vv-errline`/`.vv-grow` lên `components.css`,
thẻ Công cụ sang `tools.css`, `SetupScreen` không còn nạp CSS của khung Cài đặt.
QA3: chụp lại toàn bộ sau bản build 13:56 trên server thật; 6/7 phép đo nút khớp artboard,
và lệch 4px cuối cùng ở nút “Đăng nhập ChatGPT” (`setup=login|failed`) đã sửa: Connect tách
prop `size` trên `ChatGPTLoginAction` (mặc định `compact` cho Cài đặt), mình truyền `md` ở
call site Onboard → 36/r10/13/600 thay vì 32. Ảnh `setup=login|failed` trong `shots/` là bản
trước khi sửa; dist dùng chung lần cuối: `index-CndEvHKK.js` 14:48:28, first-load 126,1 kB
gzip. Kit 152/152, web 167/167, tsc sạch. Ảnh QA2
“kẹt skeleton/spinner/thiếu icon/không sáng nav” đều là lỗi harness cũ: harness mới đợi DOM
+ giải mã mọi mask icon + network idle, chạy trên server thật và chỉ vá trường không dựng
được offline. Concerns: 3 điểm chờ quyết định (số luồng #18, “Giới hạn từng kênh” #19, ngày
phát hành ở thẻ phiên bản) và dải offline đang đẩy nội dung xuống thay vì phủ — đều ghi ở
mục hỏi-đáp.

## Follow-up: kích thước hàng công cụ + “khoảng X GB” (đo vs dự trữ)

`RequiredBytes` từng là con số hiện lên cả hàng công cụ **lẫn** câu “khoảng X GB”, nhưng nó
vốn là **chỗ trống dự phòng** của hàm kiểm ổ (`disk.enough`), lớn hơn kích thước cài thật 2–4
lần → hiển thị phình to và câu intro “khoảng 4,7 GB” thay vì ~1,8 GB thiết kế. Đã tách:

- **`ToolPin.ApproxBytes`** (manifest) — dung lượng cài **xấp xỉ** của bản ghim, đo thật trên
  máy: ffmpeg **88 MB** (darwin-arm64), Chrome **370 MB**, Codex **299 MB**; yt-dlp
  `YtDlpApproxBytes` **124 MB**; môi trường tách tiếng `PythonEnvApproxBytes` **1,2 GB**.
  `RequiredBytes` giữ nguyên — chỉ cho `disk.enough`.
- **`Row.SizeBytes`** giờ qua `sizeForRow`: hàng `ready|update` dùng **số đã đo** ghi lại,
  0 nếu chưa có (UI hiện “Đã cài” trần); hàng `pending|installing|failed` dùng **xấp xỉ**;
  hàng `provided` luôn **0** (“Có sẵn”).
- **Đo và ghi một lần sau khi cài**: `measureInstalled(id)` chạy trong goroutine cài
  (`install.go`), tính `ToolRoot` cho công cụ phẳng (yt-dlp), `ActiveVersionDir` cho
  ffmpeg/codex/chrome, và `python` + `envs/transcription` + `models/hf` cho môi trường
  tách tiếng; ghi `bin/<id>/installed-size` (`internal/tools/installer.go`,
  `ReadInstalledSize`/`WriteInstalledSize`/`DirSizeBytes`). **GET không đi cây** — đọc đúng
  tệp đã ghi.
- **`domain.SetupCheck.SizeBytes`** (mới) đưa con số này sang `Step.SizeBytes`; `Disk` và
  `setupDisk` thêm **`ApproxBytes`** = tổng xấp xỉ các bước còn thiếu, cho câu “khoảng X GB”.
- **Web**: `api/setup.ts` thêm `approxBytes`; hero `SetupScreen` dùng `approxBytes`
  (không phải `requiredBytes`).

**Đo trên máy thật** (install thật): `yt-dlp` cài xong đo **129.401.729 B (123,4 MB)** ghi vào
`installed-size`, và `/api/tools` trả `sizeBytes = 129401729` — không phải `YtDlpApproxBytes`
(124 MB) lẫn `YtDlpRequiredBytes` (50 MB). Ghi thêm 200 MB vào thư mục rồi GET lại vẫn
**129.401.729** → GET đọc số đo, không đi cây. `codex`/`chrome` đo được 299 MB / 370 MB.

**Test đỏ-trước-khi-sửa**: `TestInstalledSizeIsMeasuredNotReserved`,
`TestSnapshotReadsPersistedSizeNotTreeWalk` (Go), và test hero “khoảng X GB” chỉ xuất
`approxBytes`, không `requiredBytes` (web). Vitest **171/171** (4 test mới), Go `toolsvc`/
`setup`/`tools`/`web`/`domain` + `archtest` xanh, race `-p 1` xanh, `tsc` sạch, gofmt
go1.26.0+go1.27.0 sạch.
