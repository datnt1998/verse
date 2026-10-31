# QA-P4-Library — Thư viện dùng chung + Template (không gồm Video nền)

Đối soát màn Library (4 tab trừ Video nền) và màn Template với artboard
`artboards/Library.dc.html`, `artboards/Template.dc.html` ở 1440×900 và 390×844.

## Cách chạy

- App: `viderse` build từ repo (`apps/viderse/web` + `cmd/viderse`), chạy trên
  `127.0.0.1:18761`, DB `viderse_qa_p4lib` (migrations `00001`–`00023`), admin
  `qa/qa-pass-2026-xyz`.
- Artboard: bản sao của `apps/viderse/docs/design/v1` phục vụ ở
  `127.0.0.1:18762`; mỗi state lật qua `data-props` (ví dụ `tab`, `prompt`,
  `menu`, `section`, `preview`, `picker`). Font Google bị chặn nên nạp Be
  Vietnam Pro + JetBrains Mono từ `internal/web/dist/assets` vào bản sao.
- Seed: 8 prompt người dùng + 7 builtin qua `POST /api/image-prompts`; 5
  template qua `viderse template create|set` (JSON recipe đặt tay; recipe "Cận
  cảnh" để backdrop rỗng bằng SQL vì API từ chối rỗng); 3 kênh +
  `owner_channel_templates` qua SQL; backdrop channel qua SQL (`slowroads`,
  `nightdrivefm` twitch, `3493118` bilibili — không gọi mạng); branding là PNG
  dựng sẵn trong `projects/channels/<id>/`; giá ảnh qua `ai_models.per_image`
  (`0.02` → biết giá; `NULL` → chưa biết giá); `preview_revision` chỉnh SQL cho
  fresh/stale.
- Browser: Chrome headless thật; `init_scripts` xoá `PublicKeyCredential`.
  Đo bằng `getBoundingClientRect`/`getComputedStyle`, không đo mắt.

## Bảng cặp ảnh (mọi tệp trong `/tmp/viderse-qa-p4lib/shots`)

| State | App | Artboard |
| --- | --- | --- |
| tab=templates | app-templates.png | art-templates.png |
| menu ⋯ mở | app-menu.png | art-menu.png |
| dialog Đổi tên | app-rename.png | (kit dialog) |
| dialog Template mới | app-create.png | (kit dialog) |
| tab=prompts list | app-prompts-list.png | art-prompts-list.png |
| tìm "hook" | app-prompts-search.png | — |
| prompt=mine | app-prompt-mine.png | art-prompt-mine.png |
| prompt=builtin | app-prompt-builtin.png | art-prompt-builtin.png |
| prompt=new | app-prompt-new.png | art-prompt-new.png |
| Xoá khoá (đang dùng) | app-prompt-del-disabled.png | — |
| tab=overlays | app-overlays.png | art-overlays.png |
| confirm bỏ overlay | app-overlay-remove-builtin.png | — |
| tpl section=images, fresh | app-tpl-images.png | art-tpl-images.png |
| tpl preview=stale | app-tpl-stale.png | art-tpl-stale.png |
| tpl section=cover | app-tpl-cover.png | art-tpl-cover.png |
| tpl section=output | app-tpl-output.png | art-tpl-output.png |
| autosave 400 | app-tpl-400.png | — |
| autosave 409 | app-tpl-409.png | — |
| giá chưa biết | app-tpl-price-unknown.png | — |
| điều hướng vỡ | app-broken-nav.png | — |
| mở editor từ card | app-prompt-card-open.png | — |
| mobile templates | m-app-templates.png | m-art-templates.png |
| mobile prompts | m-app-prompts.png | — |
| mobile overlays | m-app-overlays.png | — |
| mobile template | m-app-tpl.png | — |
| mobile artboard (ref) | — | m-art-prompt-mine.png |

## Lỗi lớn

### 1. Mọi link nội bộ trong Library/Template điều hướng sai — rơi về Bàn dựng

`paths.*` trả path **không có `#`** (`/library?tab=prompts`,
`/templates/<id>?section=cover`, `/channels/<id>`) trong khi SPA route bằng
`location.hash`. Các `<a>` trong `LibraryScreen.tsx:103,110,119,159`,
`template/TemplateNav.tsx:40,49`, `TemplatesTab.tsx:77` render `href` trần đó —
click → tải lại trang (server 200 tại `/library?tab=templates`), SPA mount với
hash `#/` → parse thành `workbench`. Đo được: click "← Tất cả template" trên màn
Template → URL `http://127.0.0.1:18761/library?tab=templates#/` và hiển thị màn
Bàn dựng "Chưa có video nào" (`app-broken-nav.png`). Cùng kiểu: tab bar, sidebar
(kênh + 4 mục thư viện + "Tạo kênh"), tên template trên card, nav 4 mục trong
màn Template. Các nút `<button onClick={navigate(...)}` vẫn đúng.
Sửa: tiền tố `href={`#${paths…}`}` giống `shell/AppHeader.tsx:42` hoặc đổi
`paths.*` trả luôn dạng hash.

### 2. PreviewPane hiển thị "Chưa có bản xem thử" khi preview đang khớp (fresh)

`template/PreviewPane.tsx`: với `state === 'fresh'` (`preview_revision ==
revision`, API trả `builtAt`), khung vẫn render overlay mờ `"Chưa có bản xem
thử — Dựng thử để xem cảnh và nền trộn lại"` và nút khoá "Dựng lại" — điều kiện
overlay là `state !== 'stale'` nên fresh cũng rơi vào. Artboard
`Template.dc.html` state `preview=fresh` (`pv.idle`) vẽ khung đã dựng xem được
(không overlay), badge "Khớp cài đặt", "dựng lúc 23:52", nút "Dựng lại" bình
thường — xem `art-tpl-images.png` vs `app-tpl-images.png`. Badge "Khớp cài đặt"
của app đúng; chỉ phần khung hiển thị sai trạng thái. Sửa: chỉ dựng overlay
"Chưa có…" khi `state === 'none'`; `fresh` render khung/nội dung đã dựng (ít nhất
không bị mờ và không nói "chưa có").

### 3. PromptEditor giữ nguyên dữ liệu prompt trước khi đổi view

`PromptsTab.tsx:80-101`: `editing` đổi nhưng `PromptEditor` không `key` theo
prompt — `useState` trong `PromptEditor.tsx` (`name`, `prompt`, `kind`, `char
count`) giữ giá trị cũ. Đo được: mở `prompt=mine` ("Đêm xanh rêu") rồi vào
`prompt=new` → tiêu đề "Prompt mới" nhưng Tên vẫn "Đêm xanh rêu", counter
`188/2000` (`app-prompt-new.png`); `prompt=builtin` → tiêu đề "Theo nhận diện
kênh" nhưng ô Tên vẫn "Đêm xanh rêu" (trường readonly cũng giữ chữ cũ). Cùng
nguồn, một lỗi thứ hai: vào thẳng `#/library?tab=prompts&prompt=builtin` khi
`all` chưa tải → `pick` rơi về `{mode:'new'}` và effect không chạy lại → render
"Prompt mới" thay vì prompt có sẵn (tái lập sau `page.reload`). Sửa:
`key={editing.prompt?.id ?? editing.mode}` (hoặc reset state khi `editing`
đổi) và chờ `prompts.initialLoading` xong mới resolve `pick` (hoặc giữ
`editing` là `undefined` cho đến khi có dữ liệu).

## Lỗi nhỏ

1. **Sidebar khác kích thước/cấu trúc**: app 240px, padding `22 14 22 20`,
   nhãn `Kênh của tôi` không số, hàng kênh 34px một dòng, hàng mục 32.1px,
   không chấm vàng, thứ tự kênh theo alpha (`Chuyện Lạ`, `Midnight Tales`,
   `Quiet Hours`); artboard 252px, nhãn có số (`Kênh của tôi 3`), hàng kênh
   ~56.7px hai dòng + chấm vàng, thứ tự MT/CL/QH, có thẻ "Máy sẵn sàng · 2/2
   luồng" đáy sidebar — `Library.dc.html:799-810`. (Sub-line "Hôm nay x/y" là
   phần đã chấp nhận bỏ.)
2. **Header**: h1 app 21px/600 (artboard 22px/600, `h1` kit); dòng phụ app
   12px `#7E7E8A` (artboard 13.5px `#A0A0AB`); thiếu nút "Nhập template"
   (artboard `Library.dc.html:839`) — chưa có API nhập, hợp lý để ẩn nhưng
   ghi nhận. "Template mới" đúng 36px/white-pill.
3. **Tab bar**: app cao 40.1px, font 13/400 (artboard 38px, 13.5/500 active —
   `Library.dc.html` `[role=tab]`); gạch đỏ active đúng.
4. **Thẻ template**: app nền phẳng `#141416` bo 12, strip 96px → thẻ 372×298.5;
   artboard gradient `#171719→#131315` bo 14, thumb 366×146.4 → thẻ 368×312.4
   (`Library.dc.html:849-877`). App đặt health/style thành chip trong thân;
   artboard để badge phủ lên strip (health ở góc trái ~y+10, style badge phải
   dưới mép strip). App thiếu nút icon Xuất tệp và ⋯ đặt góc phải trên
   (artboard ⋯ cuối hàng action). Card "Cận cảnh" hiển thị đúng chip vàng "Cần
   sửa · thiếu clip nền" nhưng nút vẫn "Mở" (artboard "Mở để sửa"). Dòng tóm
   tắt đúng nội dung, thiếu chi tiết chuyển động/`2 video lẻ` (API
   `TemplateSummary` không trả `counts`) — "Prompt Đêm xanh rêu · Tách nền ·
   bìa Hook đỏ Midnight · 6 ảnh · 1 lớp phủ" vs artboard "… · Lang thang · 2
   kênh nền, 2 video lẻ · 1 lớp phủ". Tile "Kéo tệp template vào đây…" vắng —
   do không có API nhập.
5. **Tab Prompt**: artboard chia 2 cụm có đề mục "Ảnh cảnh 7 · …" và "Ảnh bìa
   8 · …", mỗi grid kết thúc bằng tile "Prompt mới" (`Library.dc.html:937-999`);
   app gộp một grid chung (`grid cols: 296×3`), đề mục thành một dòng legend
   nén "Ảnh cảnh 7 · Vẽ cho từng đoạn lời kể Ảnh bìa 8 · Một ảnh mỗi video",
   chỉ 1 tile "Prompt mới". Thẻ prompt: artboard tên nằm trên ảnh mẫu + pill
   kind + nút "Mở"/"Nhân bản" trong thẻ; app tên dưới ảnh, có dòng meta
   `sửa <giờ> · Dùng ở <template>` (artboard chỉ "Dùng ở …"), thiếu nút "Mở"
   hiển thị (click card vẫn mở được — `app-prompt-card-open.png`). Tìm "hook"
   lọc đúng 4 thẻ.
6. **Editor prompt**: chân app `[Xoá khoá] [Huỷ] [Lưu]` — artboard `prompt=mine`
   có `[Xoá] [Nhân bản] [Lưu]` (thiếu Nhân bản; thừa Huỷ — vô hại). Khu "Ảnh
   mẫu"/"Vẽ thử với [video]" rút còn card khoá (đã chấp nhận: chưa có video để
   vẽ). "Xoá" khoá đúng cho prompt đang dùng (moss/`aria-disabled`,
   `app-prompt-del-disabled.png`). Dùng đúng: counter `188/2000`, dòng "Enter
   gửi · Shift+Enter xuống dòng", block "Luôn giữ, mọi prompt".
7. **Tab Lớp phủ**: 12 tile builtin + tile "Thêm hiệu ứng" đúng; tên builtin
   từ manifest lệch artboard ("Sao lấp lánh/Tuyết rơi/Mưa nhẹ" vs "Tia
   sáng/Tuyết/Mưa" — sai khác dữ liệu, ghi nhận). Nút bỏ (thùng rác) hiện trên
   **mọi** tile kể cả builtin: confirm → server 404 `Lớp phủ không tồn tại` —
   artboard nói rõ "Hiệu ứng có sẵn không bỏ được" (`Library.dc.html:1190`).
   Sửa: ẩn/khoá nút bỏ cho `builtIn !== ''` (và khóa khi `usedBy.length > 0`
   nếu chưa). Tile upload không nhắc "dán link" (không có API — chấp nhận).
8. **Màn Template**: thanh mục `vt-nav` rút còn "2 kênh nền" (artboard "2 kênh
   · 2 video lẻ · tuyết" — `Template.dc.html:1435` navDefs, lại cần `counts`).
   Mục Xuất & kênh: hàng kênh thiếu avatar + nút "Bỏ khỏi kênh" + dòng "Thứ 1
   trong 3 template luân phiên…" (app chỉ "vị trí 1"); mục Bố cục thiếu nút
   "Mở trong editor" (artboard `Template.dc.html` phần output — có thể xếp
   cùng nhóm khoá chưa có editor). Nút Xuất tệp khoá kèm lý do đúng.
9. **Sau 409 → "Tải lại"**: banner đúng (`app-tpl-409.png`: "Bản lưu bị
   lệch… giữ chữ vừa gõ"), server giữ nguyên (rev 4), nhưng sau reload ô Tên
   hiện lại giá trị draft cũ "Mẫu chuẩn · Tách nền QA" thay vì tên server
   "…(server)" — `useTemplateDraft.reload` xong, `setName(draft.detail.name)`
   đọc `draft.detail` đã cũ (`TemplateScreen.tsx` handler `conflictAction`).
   Sửa: đọc detail mới trả về từ `reload()` (hoặc để useEffect theo
   `detail.name` tự set lại input).
10. **Mobile 390×844**: một cột, không tràn ngang, preview xếp trước settings —
    đạt "usable"; nhưng nhiều điểm chạm < 44px: nút card 30px, tab 39.4px,
    search 30px, seg ~34px (artboard breakpoint `<700` vẫn giữ hàng ~44+).
    Sidebar ẩn đúng, kênh có trong drawer.

## Các điểm đã kiểm đúng

- Blur trên ô Tên tự lưu: PATCH đúng, revision 2→3, indicator "Đã lưu ·
  HH:MM" cập nhật.
- 400 (tên rỗng): chữ nhập giữ nguyên, ô Tên viền đỏ, lỗi "Tên template phải
  có ít nhất 1 ký tự" hiện dạng banner đầu card thay vì dưới trường (khác
  vị trí so `fieldErrors.name → vt-name-error`, thông tin vẫn đầy đủ).
- 409: như trên — draft giữ, server không ghi đè, có "Tải lại".
- Preview stale (preview_revision < revision): overlay đúng "Bản xem thử đã
  cũ / Dựng lại để thấy cài đặt vừa đổi", badge vàng "Cần dựng lại", nút
  primary "Dựng lại bản xem thử" — khớp artboard `preview=stale`.
- Health card: "Sẵn sàng làm video" + 3 check, nội dung khớp dữ liệu.
- Ước giá: `per_image=0.02` → "6 ảnh mỗi video · khoảng $0.14 tiền vẽ ảnh.";
  `NULL` → "6 ảnh mỗi video. Chưa khai giá ảnh của model đang dùng."
- Dialog create/rename/menu: vị trí, scrim mờ, disabled-with-reason trong menu
  ("Đang dùng ở Chuyện Lạ Đêm Khuya, Midnight Tales — bỏ khỏi từng kênh trước
  rồi xoá") đúng bản app dùng `aria-disabled`.
- Trạng thái đã chấp nhận không báo: khoá "Vẽ thử"/xuất tệp/nhập, không ảnh
  mẫu, không "Hôm nay x/y" trong sidebar, không dựng thử preview.

Status: 3 lỗi lớn, 10 nhóm lỗi nhỏ; 18 cặp ảnh desktop + 5 mobile.
