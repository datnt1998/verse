# P4 · Web Thư viện + Template

Màn Thư viện (`#/library`) và Cài đặt template (`#/templates/<id>`) dựng trên API
thư viện đã xong, theo artboard `Library.dc.html` / `Template.dc.html` và hợp
đồng `p4-web-contract.md`.

## Đã làm

### Thư viện (`screens/library/`)

- Khung trang theo artboard: cột trái (kênh của tôi + 4 mục thư viện kèm số đếm,
  icon), header (tiêu đề, phụ đề, nút hành động theo tab), **tab gạch chân**
  (`role=tablist`) đúng artboard thay vì segmented, cuộn dọc trong `.vlb-scroll`.
- `?tab=templates`: lưới thẻ (sức khoẻ, kiểu ghép, nền, prompt, số ảnh, overlay,
  kênh dùng + avatar, thời điểm sửa; strip nghệ thuật trung tính vì API không trả
  ảnh thu nhỏ). Menu thẻ: Đổi tên (dialog), Nhân bản, Thêm vào kênh (mở
  `AttachChannelDialog` hoặc khoá kèm lý do khi chưa có kênh), Xoá khoá kèm
  `deleteReason` khi template đang được kênh dùng. Nút "Template mới" mở dialog
  đặt tên; menu ⋯ trên mỗi thẻ.
  - Không có nút "Nhập template": backend chưa có endpoint import template.
- `?tab=prompts`: bộ lọc kind dạng radiogroup có số đếm (`listPrompts` += kind
  đếm), tìm kiếm debounce qua `lib/useDebounced.ts` gọi server `q`, lưới
  `PromptCard` (prompt có sẵn badge, usedBy), sửa/tạo trong `PromptEditor`
  (`prompt=new|mine|builtin|<id>`): tên ≤ 60, kind scene|cover, composition cho
  cover, prompt ≤ 2000 kèm đếm ký tự; nhân bản; xoá (409 `in_use` hiện tên
  template, prompt có sẵn khoá); xuất/nhập JSON (nhập là multipart
  `POST /api/image-prompts/import`). "Ảnh mẫu"/"Vẽ thử" khoá kèm lý do.
- `?tab=overlays`: lưới thẻ video (`<video preload="metadata" muted playsInline>`
  trỏ `GET /api/overlays/{id}/file`, bấm để phát), thời lượng, nền, badge có sẵn,
  kênh/template dùng; thêm bằng upload multipart (đọc giới hạn mime/kích thước từ
  lỗi server), xoá tombstone có confirm.
- `?tab=backdrops`: render `<BackdropsTab />` từ `./backdrops/BackdropsTab` (file
  mount do agent Video nền thay nội dung; bản này đã được thay bằng màn thật).

- Tab `projects` trong script artboard là "dự án editor" — không có route, bỏ qua.

### Cài đặt template (`screens/library/template/`)

- `TemplateScreen` + `TemplateNav` (quay lại, nhóm CÀI ĐẶT 4 mục kèm tóm tắt
  sống, thẻ sức khoẻ ghim đáy cột trái), header nama editable + chip `Ổn/Thiếu
  clip nền` + `Dùng ở N kênh`, "Đã lưu · HH:MM" (`aria-live=polite`), "Mở trong
  editor" khoá, menu ⋯ (Nhân bản).
- `useTemplateDraft`: PATCH kèm `revision`; ô chữ lưu khi blur, điều khiển rời rạc
  lưu khi đổi; một request đang chạy, lần gửi sau gộp lại và chạy tiếp tuần tự;
  400 theo trường hiện dưới ô và **giữ nguyên chữ đã gõ**; 409 hiện banner kèm
  "Tải lại", giữ bản đang sửa.
- `PreviewPane`: `fresh|stale|none` từ `detail.preview`; chỉ `stale` có nút
  primary "Dựng lại", nút luôn khoá kèm lý do (chưa có builder ở wave này —
  trạng thái `building` không nằm trong dữ liệu API).
- `PriceEstimate`: `known=false` → chỉ hiện số ảnh; chế độ theo phút → câu công
  thức; `known` → số ảnh + USD.
- Mục Ảnh cảnh/Ảnh bìa/Xuất & kênh đủ trường recipe (kiểu ghép + khung, prompt
  chọn bằng **lưới thẻ bấm-chọn** như artboard kèm khối chi tiết prompt + "Kho
  prompt ảnh"/"Prompt mới", số ảnh + giá, chuyển động + cường độ/tốc độ, độ trong
  suốt, ảnh đối thủ; bìa: prompt bìa theo kind, nguồn AI/tải lên, vẽ thử khoá, bìa
  mặc định khoá; xuất: nén/gốc, phụ đề, loudness, kênh dùng + thêm kênh, trùng
  template, xoá template, xuất tệp khoá).
- Mục Nền & hiệu ứng render `<BackdropSection template saving onSave />` (file
  mount do agent Video nền thay nội dung; đã được thay bằng mục thật).

### API / i18n

- `api/library.ts`: type + hàm wire đầy đủ (templates, prompts, overlays, draft
  template, attach/detach, duplicate, import/export prompt, upload overlay...).
  Xuất `TemplateDetail`, `Recipe` và các type con.
- `i18n/library.ts`: toàn bộ copy hai màn (artboard + 06-copy), nối vào
  `i18n/vi.ts` một key `library`.
- `data/keys.ts` / `data/eventKeys.ts`: key template/templates/prompts/overlays/
  channels, invalidate theo topic SSE (`template`, `prompt`, `overlay`).

## Kiểm chứng

- `npx tsc --noEmit`: sạch.
- `npx vitest run`: **25 file, 261 test pass** (gồm 9 test thư viện: tự lưu khi
  blur kèm revision; 400 giữ chữ + lỗi trường; 409 giữ bản sửa + "Tải lại"; hai
  lần blur → PATCH tuần tự revision 1→2; preview none/fresh/stale; xoá prompt 409
  hiện tên template; menu xoá template khoá khi đang dùng).
- `design-rules.test.ts`: 14/14 pass (thêm token `--vv-s-232` vào
  `styles/viderse.css`; không dùng border trái/phải để kẻ cột).
- Trình duyệt thật (`viderse serve` + Postgres `viderse_p4_libweb`, dữ liệu seed
  sẵn 6 template / 9 prompt / 12 overlay / 2 kênh): chụp 1440×900 và 390×844 bằng
  Playwright cho templates, prompts, overlays, backdrops và template
  images/cover/output; `scrollWidth == innerWidth` ở cả hai cỡ (không tràn
  ngang), không lỗi console. Ảnh chỉ lưu ở `/tmp/viderse-p4-libweb/shots`
  (không commit). Công cụ screenshot của omp treo trong phiên này nên dùng
  Playwright CLI riêng.

## Ghi chú / còn lại

- Backend chưa có import template → không có nút nhập; import prompt thì có.
- `preview.building` chưa tồn tại trong API wave này; UI chỉ vẽ 3 trạng thái.
- Thẻ template/prompt/overlay không có ảnh thu nhỏ từ API; dùng khối nghệ thuật
  trung tính + glyph theo kind, không bịa ảnh.
- Hai file mount (`backdrops/BackdropsTab.tsx`, `template/BackdropSection.tsx`)
  đã bị agent Video nền thay nội dung thật; hợp đồng props giữ nguyên.

## Sửa theo QA-P4-Library (đợt 2)

Đã sửa cả 3 lỗi lớn và toàn bộ 10 nhóm lỗi nhỏ; kiểm chứng lại trên Chrome thật.

Lớn:

1. **Điều hướng** — mọi `<a>` nội bộ render `#${paths…}` (tab bar, sidebar kênh +
   4 mục + "Tạo kênh", tên template trên card, back/nav 4 mục màn Template,
   picker prompt); thêm test `mọi liên kết nội bộ render href bắt đầu bằng "#/"`.
   Click thử: tab Prompt ảnh → `#/library?tab=prompts`, back → `#/library?tab=templates`.
2. **PreviewPane** — overlay "Chưa có bản xem thử" chỉ còn khi `state === 'none'`;
   `fresh` render khung không mờ + "dựng lúc HH:MM" + nút "Dựng lại" thường.
3. **PromptEditor** — `key` theo prompt/mode nên đổi view reset state (mine → new
   ra ô trống, counter 0/2000); deep link `prompt=builtin|mine` chờ
   `prompts.initialLoading` xong mới chọn nên không rơi về "Prompt mới".

Nhỏ:

1. Sidebar: 252px, hàng kênh 46px + avatar 30px, nhãn có số ("Kênh của tôi 2"),
   thẻ "Máy sẵn sàng / N đang chạy · N chờ lượt" đáy sidebar (lấy `/api/status`).
   Artboard ghi "2/2 luồng" — API không trả số luồng nên chỉ hiện trạng thái + hàng chờ.
2. Header: h1 22px, dòng phụ 13.5px `fg-3`; "Nhập template" vẫn ẩn vì chưa có API nhập.
3. Tab bar: cao 38px, 13.5px, active weight 500.
4. Thẻ template: nền gradient, bo `--vv-r-14`, strip 140px; health/style thành badge
   phủ trên strip (trái trên / phải dưới); health vàng → nút "Mở để sửa"; hàng thao
   tác có icon Xuất tệp (khoá + lý do) và menu ⋯ ở cuối hàng.
5. Tab Prompt: tách hai cụm **Ảnh cảnh N · …** và **Ảnh bìa N · …**, mỗi cụm kết
   thúc bằng tile "Prompt mới"; tìm/lọc thì về một lưới ảo hoá. Card: tên phủ trên
   ảnh, pill kind, badge Có sẵn, nút "Mở" + "Nhân bản", dòng meta chỉ còn "Dùng ở …".
6. Editor prompt: thêm "Nhân bản" cạnh Xoá/Lưu cho prompt đã có (builtin nhân bản
   được như cũ); nút Huỷ giữ nguyên (vô hại).
7. Lớp phủ: builtin (`builtIn !== ''`) không còn nút xoá; ghi chú dưới lưới đúng
   artboard: "Hiệu ứng có sẵn không xoá được. Bỏ một hiệu ứng tự thêm thì template
   đang dùng vẫn giữ nó." (máy chủ giờ trả 403 `builtin`). Tên builtin lấy từ API
   mới (Tuyết, Mưa, …).
8. Màn Template: tóm tắt mục Nền & hiệu ứng thêm tên lớp phủ ("2 kênh nền · 2 video
   lẻ · Tuyết"); mục Xuất & kênh có avatar, nút "Bỏ khỏi kênh"
   (`POST /api/owner-channels/{id}/templates {action:'remove'}` + invalidate),
   dòng "Thứ N trong danh sách luân phiên của kênh"; mục Bố cục có nút "Mở trong
   editor" khoá kèm lý do.
9. Sau 409 → "Tải lại": `reload()` trả `TemplateDetail` mới, ô Tên lấy tên máy chủ.
10. Mobile `(pointer: coarse)`: mọi nút/link/input trong hai màn ≥ 44px (đo bằng
    Chromium `hasTouch`+`isMobile`: templates/prompts/overlays/template đều sạch,
    chỉ còn logo 18px ở appbar ngoài phạm vi).

Kiểm chứng lại: `tsc --noEmit` sạch (file của agent khác đang sửa dở không tính),
`vitest run` **25 file / 263 test pass** (gồm 10 test thư viện, có test href),
`design-rules` 14/14. Chạy `viderse serve` + Postgres `viderse_p4_libweb` (migrate
00001–00023), seed 2 kênh, 5 template, 4 prompt của bạn + builtin, preview fresh
và stale; chụp 1440×900 + 390×844 (coarse) — không tràn ngang, không lỗi console.
DB và server kiểm chứng đã dừng/xoá sau khi chụp.
