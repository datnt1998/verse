# Viderse — Thành phần dùng chung

Mỗi thành phần ghi: dùng ở đâu, quy tắc, artboard mẫu. Chốt ngày 28/10/2026, cập nhật 29/10/2026 (đợt 19). Tokens ở [tokens.css](tokens.css).

**Từ đợt 19, bộ thành phần lấy nguyên từ UI Kit của Noverse, chỉ đổi theme màu:** chỗ Noverse tím thì Viderse đỏ (`--vv-accent*`), chỗ Noverse đỏ (xoá, lỗi) thì Viderse cam (`--vv-danger*`). Cỡ, bo góc, bóng, chuyển động, cách hỏi xác nhận giống Noverse. Nút chính là nút **sáng**; đỏ không làm nền nút. Từ đợt 20, cả 23 artboard màn hình (15 máy tính, 8 điện thoại) đã vẽ theo kit này. <!-- r19-kit --> <!-- r20-screens -->

**Hình dáng và lớp CSS** của mọi thành phần nằm ở UI Kit: [kit.css](kit.css), tra bảng ở [05-kit.md](05-kit.md), xem trên board `Kit` (máy tính) và `MKit` (điện thoại). File này chỉ ghi luật dùng; khi hai bên khác nhau thì sửa cả hai trong cùng đợt. <!-- r15-kit -->

Thành phần hiện có trong code vẫn dùng lại khi khớp hành vi: `PendingButton`, `LinkPending`, `PageState`, `TechnicalDetails`, `VideoThumb`, `AlertDialog`, toast `sonner`, `Progress`. Phần dưới chỉ ghi những gì đổi hoặc mới.

## Luật hình (bắt buộc) <a id="c-shape-rules"></a> <!-- r26-rules -->

Áp cho mọi màn, artboard và code. Vi phạm thì sửa trước khi bàn giao, không chờ góp ý.

1. **Avatar luôn tròn** (`border-radius: 50%`): kênh của bạn, kênh đối thủ, kênh nền, nền tảng (YT/TW/BL), nhà cung cấp AI, tài khoản. Ô icon đứng chung cột với avatar trong một danh sách cũng tròn. Chỉ **ảnh video / thumbnail / khung hình** là chữ nhật bo góc (7–12px).
2. **Không vạch màu một bên.** Không dùng `border-left/right` từ 2px, `border-left` 1px có màu, `box-shadow: inset Npx 0 0`, hay dải dọc `::before/::after` sát mép để:
   - **báo đang chọn** → dùng nền đậm hơn (`rgba(255,255,255,.08)` hoặc `--vv-hover-row`), có thể thêm viền đủ 4 cạnh 1px (`.vv-row[aria-current]`), chữ đậm hơn;
   - **trích dẫn / lời kể / lời nhắc** → khối nền nhạt bo 10px (`padding: 10px 12px; background: rgba(255,255,255,.035)`), màu theo ngữ cảnh nếu cần (vàng nhạt cho lời nhắc hạn mức);
   - **phân loại bằng màu** (nguồn, kênh, trợ lý) → chấm tròn 8px cạnh tên.
   Được phép: vạch ngăn cột/khung 1px trung tính (`rgba(255,255,255,.06–.11)`), viền hai bên của một vùng chọn trên timeline.
3. **Kiểm tra trước khi bàn giao.** Design chạy `python3 artboards/audit_rules.py` (trong thư mục này) trên mọi artboard, phải ra 0 vi phạm. Dev chạy trong CI:
   ```sh
   rg -n "border-(left|right):\s*[2-9]px|inset\s+-?[1-9][0-9.]*px\s+0\s+0\b" apps/viderse/web/src
   ```
   và với mọi component có chữ cái đầu / ảnh đại diện, radius phải là `50%`.

## AppHeader <!-- c-app-header -->
- Cao 48px; viền dưới `--vv-hairline`.
- **Trái:** logo ống kính 26px + ngữ cảnh: `ChannelSwitcher` ở Bàn dựng; breadcrumb ở Kênh (“Kênh của tôi”), Khám phá (“Khám phá / <đối thủ>”) và trang phụ. <!-- r11-audit -->
- **Giữa:** `MainNav`.
- **Phải:** `JobsPill`, nút Cài đặt có chấm báo, avatar.
- Hẹp hơn 700px thì `MainNav` cuộn ngang và không rớt dòng.
- Thay `NavRail`.
- Artboard: mọi board desktop.

## MainNav <!-- c-main-nav -->
- Gồm 5 mục: Bàn dựng, Duyệt, Kênh, Khám phá, Video mới.
- Mục đang mở có nền `--vv-selected`, icon `--vv-red-ink` và `aria-current="page"`.
- Duyệt có badge vàng hiện số video chờ duyệt; ẩn khi bằng 0.
- Cao 30px, nằm trong nhóm bo 10px.

## ChannelSwitcher <!-- c-channel-switcher --> <!-- r5-scope -->
- Nút cao 30px (điện thoại 36px). Hiện “Mọi kênh” kèm 3 `ChannelAvatar` 20px chồng nhau (lệch −6px, viền tách màu nền), hoặc avatar + tên một kênh.
- Menu rộng 312px (điện thoại: sheet): Mọi kênh · đường kẻ · từng kênh (avatar 30px, “Hôm nay 3/5 · 2 chờ bạn”, chấm vàng, dấu ✓ ở mục đang chọn) · đường kẻ · Tạo kênh (mở `CreateChannel`; điện thoại mở `MCreate`). <!-- r13-create -->
- Vai trò `menuitemradio` + `aria-checked`; ↑/↓ chọn, Enter mở.
- Artboard: `Main` (bấm nút), `MMain` · `screen=switch`, `Dialogs` · `dialog=channel-switch`.

## ChannelRow · ChannelCard <!-- c-channel-row -->
- Một kênh trong tổng quan Mọi kênh.
- **ChannelRow** (cột trái Bàn dựng, máy tính) gồm 3 dòng:
  - Avatar 26px, tên, nhịp `3/5`.
  - Thanh 3px màu `--vv-muted`.
  - “N chờ bạn” (vàng) và “N sẵn sàng đăng” (xanh). Khi không có gì: “Không có việc chờ bạn” (xám).
- Template cần sửa: icon `danger` cam 13px cạnh tên, kèm tooltip/`aria-label` nói template nào.
- Tooltip của dòng: “Chỉ xem <kênh> · hôm nay 3/5 video · đã đăng 3 video tuần này”.
- **ChannelCard** (điện thoại): cùng nội dung trong thẻ 158px, dải cuộn ngang.

## PromptCard · PromptPicker <!-- c-prompt --> <!-- r7-prompts -->
- **PromptCard** (Thư viện › Prompt ảnh):
  - Ảnh mẫu 16:9 với chip “Có sẵn” (trái trên, chỉ prompt có sẵn) và chip mô tả ngắn (ảnh cảnh, ví dụ “Đêm lạnh, đèn vàng”) hoặc bố cục (ảnh bìa) ở phải dưới. <!-- r9-assets -->
  - Tên + mốc sửa (“sửa 23:40”, “Viderse”).
  - Hai dòng đầu của prompt.
  - “Dùng ở <template>” hoặc “Chưa template nào dùng”.
  - Mở · Nhân bản.
- **PromptPicker** (Cài đặt template):
  - Lưới 4 cột, mỗi ô gồm ảnh mẫu, chip mô tả/bố cục, tên và dấu ✓ khi chọn. Ô cuối là “Prompt mới”, dẫn sang kho. Ở mục Ảnh cảnh, phía trên lưới là ô **Cách ghép ảnh vào video** (3 ô ảnh).
  - Dưới lưới là khung tóm tắt prompt đang chọn: tên, chip, nội dung, template khác cũng dùng, và link “Sửa trong kho” (“Xem trong kho” với prompt có sẵn).
  - Vai trò `radiogroup` / `radio`.
- Ảnh mẫu của prompt có sẵn ở nhóm bìa dùng chung một ảnh, chỉ khác lớp chữ, khoanh và mũi tên (theo quy tắc “cùng chủ thể” ở mục Ảnh trong giao diện).

## JobsPill <!-- c-jobs-pill -->
- Hiện “N đang chạy · M chờ lượt”, có chấm đỏ `.vv-rec` khi N > 0.
- Khi hàng chờ tạm dừng: “Đã tạm dừng · M chờ lượt” với chấm xám.
- Bấm mở **popover Việc nền** (desktop, 448px) hoặc sheet (điện thoại); cuối popover có “Mở Hàng chờ”.
- Có thông báo cần bạn chưa đọc → chấm vàng trên viên.
- Máy rảnh: “Máy rảnh · 0 chờ lượt”, chấm xám.
- Tooltip ghi luồng: “Máy đang chạy 2/2 luồng”.

## Logo ống kính <!-- c-lens -->
- Hình cầu đỏ (gradient `lnSph`) và 4 cung màu trợ lý quanh nó, mỗi cung 17–19% vòng, lệch nhau 25%.
- Bản ≤30px: cung dày hơn, không có vòng kính.
- Xoay 9s mỗi vòng khi có job chạy; đứng yên khi không có job hoặc khi bật giảm chuyển động.
- SVG tĩnh nằm ở artboard `Brand`.

## AgentOrb <!-- c-agent-orb -->
- Hình tròn gradient theo trợ lý: `--vv-orb-*` và `--vv-orb-shadow`.
- Cỡ 14 / 18 / 22 / 24 / 34px.
- Trợ lý đang chạy thì có lớp `.vv-live`.
- **Chỉ dùng cho 4 trợ lý.** Avatar kênh và đối thủ **không bao giờ** dùng orb (xem `ChannelAvatar`).

## AgentCard <!-- c-agent-card -->
- Gồm orb, tên (màu chữ của trợ lý), vai trò một dòng, nhãn trạng thái, một dòng số liệu (“Ảnh đã vẽ 5 / 7 · ChatGPT”).
- Thẻ đang chạy có viền và nền ám màu trợ lý, kèm câu đang làm.
- Bàn dựng tab Trợ lý: 4 thẻ dọc. Điện thoại: lưới 4 ô.

## ChannelAvatar <!-- c-channel-avatar -->
- Ảnh avatar kênh khi đã có; chưa có thì hình tròn phẳng với 2 chữ cái đầu.
- **Kênh của tôi:** nền tối ám màu kênh, viền 1.5px màu kênh.

  | Kênh | Chữ | Nền | Viền |
  |---|---|---|---|
  | Midnight Tales | MT | `#2E2236` | `#D79BFF` |
  | Chuyện Lạ Đêm Khuya | CL | `#302418` | `#F6CB8C` |
  | Quiet Hours | QH | `#1F2C27` | `#A6EDC1` |

- **Đối thủ:** nền đặc, không viền (`#34407A`, `#6E4F25`, `#255E4B`, `#6E3324`, `#3E4659`).
- Cỡ 20 / 26 / 30 / 34 / 56 / 64px. Khi đặt chồng lên ảnh thumbnail hoặc chồng nhau: thêm viền 1.5–2px màu nền để tách.

## StatusLabel <!-- c-status -->
- Chấm 7px (hoặc spinner 11px khi Đang làm) + tên trạng thái theo màu `--vv-st-*`.
- Dòng phụ 12px màu `--vv-muted`.
- Tên và màu đúng bảng [ia-flows §3.1](01-ia-flows.md#31-bảng-trạng-thái); không tự đặt tên mới.

## ActionButton <!-- c-buttons --> <!-- r19-kit -->
- Cao 36px (nhỏ 30px), bo 10px (nhỏ 8px), chữ 13px/500 — như Noverse.
- **Chính (sáng):** `.vv-btn--primary`, nền `--vv-primary`, chữ tối 600, có thể kèm phím `kbd`. **Một màn một nút chính** — kể cả “Làm video”, “Tạo kênh”, “Chạy tiếp”. Đỏ không làm nền nút.
- **Viền (mặc định):** viền `--vv-ghost-border`, nền trong suốt. Dùng cho mọi hành động phụ, kể cả hành động theo trạng thái (Duyệt ảnh có viền vàng, Xử lý có viền cam).
- **Nhẹ:** `--quiet`, không viền, chữ `--vv-text-3` (Để sau, Bỏ qua).
- **Nguy hiểm làm lại được:** `--danger-soft`, nền cam nhạt, chữ `--vv-danger-text-2` (Đăng xuất, Bỏ thay đổi, Bỏ liên kết, Dừng tất cả).
- **Xoá vĩnh viễn:** `--danger`, nền cam đặc `--vv-danger`, chữ tối `--vv-danger-fg`. Chỉ trong hộp xác nhận.
- **Gửi:** tròn, nền `--vv-text`, chữ tối. Dùng trong composer.
- **Khoá:** `disabled` / `aria-disabled="true"` (mờ 42%), tooltip hoặc dòng gần đó nói lý do.
- **Chỉ icon:** `--icon`, luôn có tooltip và `aria-label`.

## Chip <!-- c-chip -->
- Cao 28px, bo 8px, nền `--vv-surface-1`, viền `--vv-field-border`; icon 13px. <!-- r19-kit -->
- Dạng “nhãn **giá trị**” (Template **Mẫu chuẩn · Tách nền**). Chip bấm được thì có mũi tên xuống.
- Không xuống dòng.

## Banner <!-- c-banner -->
- Như Noverse: icon 17px màu loại + tiêu đề 13px/500 + một câu 12px xám + nút nhỏ viền bên phải (tối đa 2). Bo 12px. <!-- r19-kit -->
- **Thông tin:** xanh dương nhạt (`--vv-banner--info`). **Xong / đã kết nối:** xanh lá. **Chờ bạn / sắp hết hạn mức:** vàng. **Cần xử lý / lỗi:** cam, `role="alert"`, luôn có cách sửa.
- Đặt ngay dưới tiêu đề video, trên nhật ký.

## ProgressLog <!-- c-progress-log -->
- Thay terminal panel.
- Mỗi dòng gồm giờ (mono, mờ), dấu (✓ / spinner / cảnh báo), **tên trợ lý** (màu chữ của trợ lý), câu tiếng Việt, và thời gian bước bên phải.
- `role="log"`, `aria-live="polite"`.
- Mặc định 5 dòng gần nhất; “Toàn bộ ›” mở hết.

## LiveStudio (Phòng vẽ) <!-- c-live-studio -->
- Lưới ảnh bìa (rộng gấp đôi) + 6 cảnh.
- Mỗi ô ảnh ở một trạng thái:
  - **đã vẽ:** ảnh.
  - **đang vẽ:** skeleton + orb Hoạ sĩ + chữ shimmer.
  - **chờ lượt:** sọc chéo + “Chờ lượt vẽ”.
  - **lỗi:** nền cam nhạt + câu.
  - **bị chặn:** “Bị chặn · dùng ảnh bên cạnh”.
- Nhãn góc gồm số và tên cảnh. Dưới ô là mô tả (cắt …) và mốc thời gian (luôn hiện đủ).

## Composer (Nhắn trợ lý) <!-- c-composer -->
- Thẻ bo 18px gồm icon tin nhắn, ô nhập, chip trợ lý nhận (“Bàn với Trinh sát”), và nút gửi trắng.
- Dưới thẻ là dòng phím tắt và hạn mức ảnh hôm nay.
- Cố định đáy vùng giữa.
- Kết quả hiện dưới dạng thẻ xác nhận ([Gói V11](08-dev-prompt.md#goi-v11)).

## BulkBar <!-- c-bulk-bar -->
- Thay Composer khi bật chọn nhiều.
- Gồm “Đã chọn N”, các nút viền có đếm số video hợp lệ, ⋯ (Huỷ, Xoá), và “Thoát chọn” (Esc).
- Nút có số 0 thì khoá và có tooltip lý do.
- Artboard: `Channel` · `select=true`. Khám phá dùng biến thể có một nút chính “Làm N video cho <kênh>”.

## Stepper <!-- c-stepper -->
- Như Noverse: một ô 148×36 (điện thoại 160×44), nút − / + 36px (điện thoại 44px) hai đầu, số mono ở giữa có vạch ngăn. Nút mờ khi chạm giới hạn. <!-- r19-kit -->
- Dùng cho luồng, nhịp mỗi ngày, độ dài clip.

## Switch <!-- c-switch -->
- 36×22 (máy tính) hoặc 50×30 (điện thoại); bật màu nhấn `--vv-accent-strong` (đỏ), núm sáng nảy khi bấm. <!-- r19-kit -->
- `role="switch"`, `aria-checked`.
- Bật/tắt có hệ quả lớn (chuyển Mức tự chủ sang Tự chạy, Tự duyệt hết) thì hỏi xác nhận.

## SegmentedTabs / Tabs <!-- c-tabs -->
- **Nhóm chọn** (`.vv-seg`): như Noverse, nhóm bo 10px nền ô nhập, viên chọn `--vv-selected-2` trượt tới mục chọn (`.vv-seg__thumb`). Thanh tab đầu trang dùng `--bar` (nền `--vv-surface-1`, icon mục đang chọn màu nhấn). <!-- r19-kit -->
- **Tabs trang:** gạch dưới màu nhấn 2px, số đếm xám cạnh tên.
- Trên điện thoại: cuộn ngang, cao 44px.

## Sheet (điện thoại) <!-- c-sheet -->
- Nền mờ `rgba(0,0,0,.6)`, sheet bo trên 22px, có tay cầm.
- Tối đa 86% chiều cao. Đóng bằng chạm nền hoặc vuốt xuống.
- Tiêu đề sheet 18px/600 (`.sheet-t`), lề 16px. <!-- r14-mobile -->
- Nút chính trong sheet: **sáng** (`.vv-btn--primary`, 48px) cho mọi việc chính, kể cả đưa video vào làm hoặc tạo kênh. Nút phụ viền. Xoá vĩnh viễn: cam đặc. <!-- r19-kit -->
- Dùng cho Hàng chờ / Việc nền, Đổi kênh, Góp ý vẽ lại, Ảnh của video, Viết lại / Lịch sử nội dung, chọn kênh, thao tác đối thủ, xác nhận.

## Đầu trang điện thoại (AppBar · NavHeader) <!-- c-mobile-header --> <!-- r14-mobile -->
- **AppBar** (màn gốc): cao 52px; logo 24px · `ChannelSwitcher` (chỉ Bàn dựng, Kênh) · khoảng trống · `JobsPill` · avatar 34px (mở Cài đặt). Dưới là tiêu đề lớn 24px/600, chữ phụ phải (ví dụ “Video 1 / 3 ›”).
- **NavHeader** (màn con): cao 52px; nút 44px Quay lại `‹` (đi sâu) hoặc Đóng `×` (luồng tạo mới: Video mới, Tạo kênh); tiêu đề 16px/600 căn trái, cắt “…”; dòng phụ 12px xám khi cần; nút 44px bên phải (⋯, sao chép).

## Hàng video (điện thoại) <!-- c-mobile-videorow --> <!-- r14-mobile -->
- Thumbnail 80×45, bo 8px (Bàn dựng, Kênh); tiêu đề 13,5–14px, dòng trạng thái có chấm màu / vòng xoay; nút hành động viền cao 32–36px (Duyệt, Xử lý, Tải gói đăng).
- Thẻ “Đang làm” ở Bàn dựng là thẻ riêng (thumbnail 92×52 + thanh tiến độ).
- Tab trong trang Kênh dùng nhãn ngắn chia đều 4 cột: Video · Nguồn · Template · Hồ sơ (tên đầy đủ trong `aria-label`).

## MobileTabBar <!-- c-tabbar -->
- Viên nổi (giống Noverse): cách mép trái/phải 14px, cách đáy 30px (trên home indicator), cao 64px, bo 32px. Nền kính `rgba(26,26,29,.74)` + `backdrop-filter: blur(26px) saturate(180%)`, viền `rgba(255,255,255,.1)`, bóng `0 22px 44px -14px rgba(0,0,0,.85)`, ánh sáng mảnh ở mép trên. <!-- r13-tabbar -->
- 5 ô chia đều. Mục đang mở có **viên chỉ báo** đỏ nhạt trượt tới (`rgba(255,51,82,.13)`, viền `rgba(255,77,99,.3)`, quầng đỏ mờ; vào bằng hiệu ứng nảy 0,55s), icon chuyển sang bản **đặc** (`i-workbench-b`, `i-gallery-b`, `i-channel-b`, `i-radar-b`) màu `--vv-red-ink`, nhích lên 1px, chữ `#FFE3E8` đậm.
- Mục giữa **Video mới**: nút tròn 46px đỏ (gradient + bóng đỏ), dấu + trắng, không chữ (có `aria-label`).
- Badge Duyệt: viên vàng 17px, viền tách nền 2px.
- Cuộn xuống quá 60px thì thu nhỏ (`scale(.92)`, ẩn chữ); cuộn lên thì hiện lại. Ẩn khi mở bàn phím, khi đang ở chế độ chọn nhiều (thanh thao tác thay chỗ) và ở trang con không thuộc 5 mục (chi tiết video, Tạo kênh, Video mới).
- Nội dung cuộn chừa 118px dưới cùng; thanh thao tác cố định của màn (ví dụ Duyệt: Vẽ lại · Duyệt cả video & dựng) nằm ngay trên viên.
- Icon Bàn dựng dùng bản nét (`i-workbench` = Category linear) cho đồng bộ với các icon khác; bản đặc chỉ khi đang mở.

## EmptyState <!-- c-empty -->
- Khung viền gạch bo 12px (`.vv-empty`). Đầu khối: orb của trợ lý liên quan khi trống vì trợ lý (Đã duyệt hết), hoặc ô icon 40px nền nhấn nhạt như Noverse khi trống vì chưa có dữ liệu (Chưa có video). Tiêu đề một dòng, một câu, đúng một lối đi tiếp (nút chính nhỏ + tối đa một nút viền). <!-- r19-kit -->
- Ví dụ: “Đã duyệt hết” ở `Review` · `state=empty`; Hàng chờ `state=empty`.

## ActivityPopover · TaskRow <!-- c-activity -->
- Popover hai tab: **Đang chạy** (nhóm Video · Khác) và **Thông báo** (lịch sử, chưa đọc có chấm).
- TaskRow một dạng cho mọi việc nền: orb/icon · tên + đối tượng · câu pha · thanh (%/x/y/không xác định) · thời gian còn lại · đúng một hành động.
- Artboard: `Activity` · `view=running|inbox|empty`.

## Toast <!-- c-toast -->
- Hai dạng: <!-- r19-kit --> <!-- r20-screens -->
  - **Kết quả thao tác** (Đã xoá, Đã lưu, Đã chuyển kênh…): hình viên như Noverse (`.vv-toast`), cao 40px, bo tròn, icon màu loại, một câu, một nút nhỏ (Hoàn tác / Mở lại / Xem).
  - **Cần bạn** (Duyệt ảnh, Tải gói đăng) và **Cần xử lý**: thẻ 300px bo 14px, tiêu đề + một câu + nút, vì có tên video và việc phải làm.
- Góc phải dưới, cách mép 12/16px, tối đa 3, mới nhất trên cùng.
- Loại: kết quả thao tác (5 giây, Hoàn tác/Xem, thanh thời gian) · cần bạn (Duyệt ảnh, Tải gói đăng) · Cần xử lý (cam, `role=alert`, ở lại tới khi đóng).
- Nút trong toast màu trắng trung tính, không đỏ. Không toast tiến độ. Gộp trong 10 giây.
- Artboard: `Activity` · `view=toasts` = `Dialogs` · `dialog=toasts`.
<!-- r3-comp -->

## Ảnh trong giao diện <a id="c-imagery"></a> <!-- r6-imagery -->
- **Ảnh luôn là nội dung thật** (ảnh bìa, ảnh cảnh, khung video, clip nền), không dùng vệt gradient làm ảnh giả. Khi ảnh chưa có thì dùng `skel-img` (đang tải), `wait-tile` (chờ lượt vẽ) hoặc ô lỗi; không dùng hình mờ ngẫu nhiên.
- Khung 16:9, `object-fit: cover`, bo theo cỡ (6 / 8 / 10 / 12px), viền `rgba(255,255,255,.06–.07)`.
- **Thumbnail video = ảnh bìa đã xuất** (có chữ hook nếu template có). Nhãn trên ảnh (điểm nổi `×4,2`, thời lượng, số cảnh) là lớp UI nằm trên ảnh, nền tối 75–78%.
- **Nhãn trên thumbnail chỉ nằm ở cạnh dưới**, không che chữ hook (chữ hook gần như luôn ở nửa trên): điểm nổi ở góc dưới trái (vàng khi ≥ ×3, tối khi thấp hơn); phụ đề (icon `i-subtitle` xanh) + thời lượng ở góc dưới phải. Cùng chiều cao (18–22px theo cỡ thẻ), cùng khoảng cách tới mép. Chỉ ô chọn nhiều nằm góc trên phải. <!-- r12-small -->
- **VideoCard** (Khám phá, gợi ý ở Video mới): tiêu đề luôn giữ chỗ 2 dòng để hàng nút dưới thẳng hàng giữa các thẻ; dòng dưới là nút “Làm cho <kênh>” hoặc trạng thái (chấm + “Kênh · trạng thái”, bấm mở Bàn dựng), và nút sao chép link.
- **Video đã chọn** (cột phải Khám phá): kênh đã làm video này nằm ngay trong danh sách “Làm cho kênh” — dòng nền tối, chấm màu trạng thái, “Đã làm · <trạng thái>”, link “Mở ›”; không có thẻ trạng thái riêng. Nút chính hai dòng: “Làm video” + “cho <kênh>” (không cắt tên kênh). Chương là của video gốc, giữ nguyên ngôn ngữ gốc, giờ dạng `m:ss`.
- **Ảnh cảnh** (Phòng vẽ, Duyệt, Editor) không có chữ.
- **Khung video** (trình phát Thành phẩm, Editor, Xem thử template) là một khung hình của video đã dựng, không phải ảnh bìa.
- **Ô chọn kiểu** (Cách ghép ảnh vào video, Bố cục bìa, Lớp phủ): mọi lựa chọn trong cùng một nhóm dùng **cùng một chủ thể**, chỉ khác cách xử lý. Ví dụ Cách ghép ảnh vào video: cùng nhân vật y tá ở Toàn khung, Tách nền (viền trắng trên video nền) và Cận cảnh. Lớp phủ: mỗi hiệu ứng chạy trên một khung cảnh thật.
- **Ảnh đối thủ** (Khám phá, Video mới, Nguồn) là thumbnail gốc của kênh đối thủ, khác ảnh bìa Viderse vẽ lại.
- Ảnh trong artboard (`img/`) là mẫu minh hoạ do thiết kế vẽ. Mỗi kênh mẫu có một bảng màu: Midnight Tales xanh rêu + đèn vàng; Chuyện Lạ Đêm Khuya đỏ cam đèn lồng; Quiet Hours xanh chàm dịu. App không dùng các ảnh này.

## PromptMenu <a id="c-promptmenu"></a> <!-- r8-pm -->
- Thành phần chọn ngoại lệ ảnh của một video, chỉ dùng ở chip Ảnh (Bàn dựng). <!-- r9-assets --> Máy tính: popover 344px; điện thoại: sheet (dòng cao 56px, ảnh mẫu 68×38).
- Phần: tiêu đề · một câu phạm vi (“Chỉ đổi cho video này. Template giữ nguyên.”) · tab Ảnh cảnh / Ảnh bìa (tuỳ chỗ) · danh sách radio · ô giá (tuỳ chỗ) · chân “Kho prompt ảnh” (điện thoại: “Thêm hoặc sửa prompt trong Thư viện trên máy tính”) · nút (tuỳ chỗ).
- Dòng đặc biệt dùng icon thay ảnh mẫu: Theo template (`i-layer`), Không tạo ảnh (`i-gallery-edit`).
- Dòng không chọn được: mờ 45%, `aria-disabled`, dòng phụ nói lý do.
- Ô giá: nền xám; vượt hạn mức thì nền vàng. Nút chính ghi đúng việc (“Đổi và vẽ lại 6 ảnh”); khi chưa chọn gì khác thì xám, chữ “Chọn prompt khác”.
- Artboard: `Main` `promptPop`, `MMain` `promptPop` (cần `screen=detail`).

## ProviderCard · ModelCatalog · ModelPicker <a id="c-ai"></a> <!-- r8-ai -->
- **ProviderCard:** ô chữ cái 36px màu nhà cung cấp, tên, nhãn loại kết nối, nhãn việc làm được, một dòng trạng thái (xanh khi đăng nhập gói ổn, vàng khi chưa chọn model, cam khi lỗi), thanh hạn mức cho gói đăng nhập. Nút: **Model · N** (viền màu nhấn khi N = 0, chữ “Chọn model”), Thử kết nối, và một nút chữ (Đăng xuất / Đổi khoá / Đổi địa chỉ). Lỗi: khối cam `role="alert"` + ba nút.
- **ModelCatalog** (mở trong thẻ): ô tìm “Tìm trong N model”, lọc Đã chọn · N / Gợi ý / Tất cả, dòng model = ô chọn + id mono + nhãn Gợi ý + ghi chú + nhãn khả năng (Chữ xanh, Chữ · đọc ảnh tím, Vẽ ảnh vàng) + giá; chân ghi nguồn và giờ lấy danh mục; “Không thấy model? Thêm theo tên”; tóm tắt + “Gán cho trợ lý”.
- **ModelPicker:** popover 400px dưới ô model; nhóm theo nhà cung cấp; model không đủ khả năng mờ kèm lý do; chân “Thêm model ở Kết nối AI”.
- Thanh **Thay đổi chưa lưu**: dính ở đầu mục, icon thông tin vàng, Huỷ · **Lưu** (nút sáng).
- Artboard: `Settings` `section=connect|agents`, `conn`, `ai` · `MSettings` `screen`, `sheet`.

## PlatformCard · SourceLinkInput · SourceRail · BackdropGrid · BackdropSourceList · BackdropMixBar · AttachBackdropDialog <a id="c-sources"></a> <!-- r22-sources -->
- **PlatformCard** (Cài đặt › Tài khoản nền tảng): ô chữ cái 36px trung tính (không dùng logo nền tảng), tên, một câu “dùng cho”, dòng phụ “Tải 480p · 3 kênh, 1 video lẻ · Mở Video nền”, nhãn trạng thái (`.vv-tag` xanh Sẵn sàng · cam Tạm chặn tới HH:MM · cam Cần cookie). Có sự cố: thẻ viền cam + `.vv-banner--issue`. Phần đăng nhập ngăn bằng vạch mảnh. Một mẫu cho mọi nền tảng; nền tảng mới chỉ thêm dữ liệu.
- **SourceLinkInput** (Thư viện › Video nền): một ô dán link + nút Thêm; dòng dưới ô nói app nhận ra gì (icon tick xanh) hoặc lỗi cam + nút quiet gửi yêu cầu. Không tách ô kênh với ô video.
- **SourceRail** (đợt 25) <!-- r25-backdrops -->: thẻ 272px bên trái. Tiêu đề nhóm chữ hoa 11px (Kênh nền · Video lẻ, có số), tiêu đề phụ theo nền tảng (chấm màu + tên + số). Dòng 48px: avatar tròn 30px (dòng Video lẻ: ô icon tròn), tên, dòng phụ số; đang chọn = nền `rgba(255,255,255,.08)` (không vạch trái); kênh bị chặn = icon đồng hồ cam + dòng phụ cam; đang đọc = spinner. `aria-current` cho dòng đang chọn. Dùng cả ở Thư viện và trong hộp thoại gắn.
- **BackdropGrid**: lưới 4 cột `.vv-thumb` (bo 10px), nhãn loại góc trên trái, thời lượng góc dưới phải, nhãn tick xanh “<template>” góc trên phải khi video được template chọn; tên 2 dòng + “Loại · đăng khi nào”. Không còn xem được: mờ 55% + chữ đè. Trên đầu: `.vv-seg--sm` loại (loại 0 video mờ, không bấm được) + “Đang hiện 12 / 312” + sắp xếp. Cuối: **Hiện thêm 24 video**.
- **BackdropSourceList** (Template): mỗi dòng 52px, kênh = avatar tròn 32px, video lẻ = ảnh 56×32 bo 7px (cùng một cột 56px), chấm tròn 8px màu của nguồn trước tên (khớp BackdropMixBar), tên + nhãn **Ngẫu nhiên** (xanh) / **Luôn có** (vàng), dòng phụ cách lấy; kênh có **Chọn video**; nút ✕ có `aria-label` “Bỏ … khỏi template”.
- **BackdropMixBar**: thanh 30px bo 8px, đoạn rộng theo số giây; vàng = Luôn có, màu kênh = clip của kênh, xám = lặp lại; chữ trong đoạn chỉ hiện khi đoạn ≥ 10%. Chú thích bên dưới (ô màu + tên + số clip). `role=img` + `aria-label` đọc đủ.
- **AttachBackdropDialog**: `.vv-dialog` 960×680, không padding; đầu (tiêu đề “Gắn video nền vào <template>”, một câu), thân hai cột (SourceRail có ô dán link + nhãn “Đã gắn” · chi tiết nguồn), chân (tóm tắt + lý do khoá · Huỷ · **Gắn vào template** / **Lưu**). Kênh: hai `.vv-choice` radio (Ngẫu nhiên từ mọi video · Chỉ từ video tôi chọn), chip loại `aria-pressed`, lưới 4 cột; chế độ chọn thì mỗi ảnh là `role=checkbox` có `.vv-check` và viền trắng khi chọn, ảnh khác loại đang tắt mờ và không chọn được. Video lẻ: ảnh lớn + đoạn dùng + link đổi đoạn ở Thư viện. Thay cho SourcePicker (popover) của đợt 22.
- **Phần đăng nhập trên PlatformCard** (Bilibili, đợt 24) <!-- r24-biliqr -->: đã đăng nhập = vòng chữ cái 32px + tên tài khoản + nhãn cách đăng nhập (`.vv-tag` xanh **Mã QR** · xám **Cookie dán tay** · cam **Mã QR · hết hạn**) + dòng phụ số (tự gia hạn, lần cuối, cookie đã che) + **Gia hạn ngay** / **Đăng nhập bằng mã QR** + **Xoá đăng nhập** (`--danger-soft`). Chưa đăng nhập = một câu lợi ích + nút chính **Đăng nhập bằng mã QR**. **Dán cookie thủ công** là nút quiet có mũi tên, mở ô cookie ngay dưới.
- **QrLoginDialog** <!-- r24-biliqr -->: `.vv-dialog` 620px, `role=dialog`. Đầu: icon `i-scan`, tiêu đề “Đăng nhập <nền tảng> bằng mã QR”, một câu app/tài khoản dùng được. Thân: ô mã nền trắng 200px (mã 172px, bo 14px) + thanh đếm ngược `.vv-progress--working` + “Mã còn hiệu lực m:ss”; bên phải 3 bước đánh số (bước xong đổi tick xanh, bước đang chờ viền vàng) và ô trạng thái `role=status aria-live=polite` (spinner xanh Đang chờ quét · chấm vàng Đã quét · chấm xanh Đã đăng nhập). Mã đã quét / hết hạn / lỗi: mã mờ (hết hạn, lỗi thêm `blur`) + vòng 48px có icon và một dòng chữ đè lên. Lỗi dùng `.vv-banner--issue` thay ô trạng thái. Chân: “Không quét được? Dán cookie thủ công” (quiet) · **Đóng** · **Tạo mã mới** / **Xong** (nút chính). Dòng cuối `.vv-help` nói cookie chỉ lưu trên máy. Điện thoại: cùng nội dung trong sheet, mã 196px ở giữa, nút chính rộng hết sheet. Một mẫu cho mọi nền tảng đăng nhập bằng mã.
- Artboard: `Settings` `section=accounts`, `bili`, `qr`, `yt` · `MSettings` `screen=accounts`, `bili`, `qr` · `Library` `tab=backdrops`, `nguon`, `kenh` · `Template` `section=backdrop`, `picker=none|add|choose|video`, `fewClips`.

## Editor: EditorRail · FixPanel · CaptionSyncInspector · ExportGateDialog <a id="c-editor"></a> <!-- r27-editor -->
- **EditorRail:** 68px, nút 56×54 có icon 19px + chữ 10,5px (Cần sửa · Cảnh · Phụ đề · Nền · Âm thanh · Media); đang chọn = nền `rgba(255,255,255,.08)`; số việc là huy hiệu cam tròn trên icon. Không có icon trơn không chữ.
- **FixPanel:** điểm 22px + thanh 6px có vạch 90; nhóm (tiêu đề chữ hoa 11px): Chưa đạt (thẻ viền cam nhạt, dòng đoạn 40px có giờ mono · câu · trạng thái cam/xanh), Cần đo lại, Đã đạt (gập); chân dính: **Đo lại độ hoàn thiện** + một câu thời điểm/ước lượng. Mục lấy từ `CLONE_QUALITY_ITEM_LABELS` (label + fix) của code.
- **CaptionSyncInspector:** đầu (icon, “Phụ đề”, “Đoạn lệch n/N”, ‹ ›), ô trạng thái (cam lệch / xanh khớp), dải 3 giây: hàng **Tiếng** (sóng), hàng **Chữ** (mỗi từ một thanh, `title` = từ), trục giờ mono; **Khớp theo giọng** rộng hết bảng (nút chính khi còn lệch, khoá khi đã khớp); Dời cả đoạn ±0,1 s; Nghe lại (L); ô chữ của đoạn. Phím: F / ⇧F đoạn lệch sau/trước, `,` `.` dời ±0,1 s.
- **Bảng phải theo lựa chọn:** một tiêu đề nói thứ đang chọn; việc hay làm lên đầu (nút 2 cột), thông số kỹ thuật vào **Nâng cao** gập.
- **ExportGateDialog:** `.vv-dialog` `role=alertdialog`, icon cảnh báo, liệt kê mục còn lại; **Sửa tiếp** (chính) · **Vẫn xuất**.
- Một nút chính mỗi lúc: còn lệch → Khớp theo giọng; sửa xong → Đo lại; đạt → Xuất.

## Menu <!-- r28-explore -->
- Mọi menu ⋯ dùng `ActionMenu` của kit: đệm hàng 6/10, cách hàng 0, bo 8; chỉ đổi `width` (menu đối thủ 272px). Hàng có dòng phụ giữ đệm này, không tăng riêng.

## Icon <!-- r8-icons -->
- Mọi icon trong nút dùng bộ icon (mask) của Viderse, không dùng ký tự (↶ ↷ − +). Thêm `i-undo`, `i-redo` (Editor Hoàn tác / Làm lại), `i-minus` (nút giảm), `i-key` (Kết nối AI, khoá API). Nút tăng dùng `i-add`.

## PublishInfo (Nội dung đăng) <a id="c-publish"></a> <!-- r10-channel -->
- Ba ô sửa trực tiếp (`role="textbox"`): Tiêu đề (14px, đậm), Mô tả, Tag (chip có nút bỏ). Mỗi ô có nút sao chép và bộ đếm theo giới hạn YouTube.
- Đoạn cuối của kênh nằm trong cùng khung mô tả, ngăn bằng viền đứt, chữ xám, có link “Sửa ở Hồ sơ kênh”. Tag luôn thêm: biểu tượng `i-channel`, không có nút bỏ.
- Nhãn trạng thái phải trên: “Đạo diễn viết · giờ” (xám), “Bạn đã sửa · giờ” (vàng), “Chưa viết” (xám), “Đang viết…” (tím), “Chưa viết được” (cam). <!-- r11-audit -->
- **Viết lại** mở khung (điện thoại: sheet): chip chọn phần (checkbox), ô góp ý, gợi ý nhanh, nút “Viết lại N phần” (khoá khi chưa chọn phần).
- **Lịch sử** (nút đồng hồ + số bản): danh sách bản, bản đang dùng có nhãn xanh, bản cũ có **Dùng lại**.
- Chưa viết: khung viền đứt + câu giải thích. Đang viết: dòng shimmer + 3 khối xương. Lỗi: khối cam `role="alert"`, Thử lại, Đăng nhập lại. Ba trạng thái này khoá Viết lại và Lịch sử.
- Artboard: `Main` · `panel=content`, `meta=pending|writing|failed|ai|edited|rewrite|history` · `MMain` · `screen=detail`, `meta` (cuộn tới Nội dung đăng).

## ChannelProfile <a id="c-profile"></a>
- Khối Nhận diện (ảnh bìa + avatar + nút vẽ lại từng phần, mỗi nút ghi giá), Mô tả kênh, Phong cách nhận diện (nền `#0F0F11`, icon `i-magic` vàng, nhãn “app tự rút”), Góp ý + Tạo lại cả bộ.
- Khối “Khi viết tiêu đề, mô tả, tag”: segmented Ngôn ngữ, textarea Giọng văn, textarea Đoạn cuối mô tả (bộ đếm 1.000), chip Tag luôn thêm + “Thêm tag”.
- Trạng thái lỗi: khối cam `role="alert"` ở đầu, nút Thử lại viền cam. Đang vẽ: khung xương shimmer trên ảnh bìa và avatar, nút vẽ lại khoá (`aria-disabled`).
- Nút vẽ lại luôn ghi giá: “Vẽ lại avatar · 1 ảnh”, “Vẽ lại ảnh bìa kênh · 1 ảnh”, “Tạo lại cả bộ · 2 ảnh”. <!-- r11-audit -->
- Đối thủ tham chiếu: chip vàng nhạt “Tham chiếu nhận diện” cạnh tên đối thủ.
- Artboard: `Channel` · `tab=settings|sources`, `identity`, `autonomy` · `MChannel` · `tab=settings|sources|templates`, `identity`, `autonomy`, `sheet`.

## Trợ lý chủ động <a id="c-agent"></a> <!-- r16-agent -->
Luật ở [ia-flows §9](01-ia-flows.md); lớp CSS ở [05-kit.md › Trợ lý](05-kit.md); board `Kit` › Trợ lý và `MKit` › Trợ lý.

- **WhyLink + ReasonPopover** (`.vv-why`, `.vv-reason`): link chữ “Vì sao?” 12px, gạch chân chấm, cạnh quyết định của trợ lý. Popover 320px (điện thoại: sheet): đầu “<Trợ lý> giải thích” màu của trợ lý, một câu lý do, 2–4 dòng số + nguồn, lối sửa. Không có số thì không hiện link.
- **SuggestionCard** (`.vv-suggest`, `--compact` cột trái 248px, `--m` 288px cuộn ngang): orb + “<Trợ lý> đề xuất” + tiêu đề ≤ 2 dòng + một dòng bằng chứng + nút chính **sáng** + Bỏ qua + ⋯ (Đừng đề xuất kiểu này). Nút trong thẻ là nút sáng cỡ nhỏ; nút chính của màn đứng riêng, không nằm trong thẻ đề xuất.
- **RecapView** (`.vv-recap`): câu đầu 20px (điện thoại 19px), rồi Cần bạn (nút chính Duyệt N video) → Máy đã tự làm (dòng + nhãn **Tự làm** + Xem / Hoàn tác) → Đề xuất (3) → Hôm nay (4 ô; điện thoại 2×2) → Theo kênh. Máy tính thay cột giữa Bàn dựng và ẩn bảng phải; điện thoại là màn con.
- **AutonomyControl** (`.vv-autonomy`, `.vv-arow`, `--stack` trong cột hẹp, `--m`): segmented 3 mức + 4 dòng; chỉnh một dòng thì mức thành **Tuỳ chỉnh** (nhãn). Lên Tự chạy hoặc Tự duyệt hết → hộp xác nhận (`dialog=autonomy`). Cuối thẻ luôn có `.vv-guard` liệt kê việc không bao giờ tự làm.
- **LearnHint** (`.vv-learn`, `--artist`): ngay dưới ô vừa sửa (Nội dung) hoặc dưới ô góp ý (Duyệt), chỉ ở lần lặp thứ 3 (viết) / thứ 2 (vẽ). Lưu → banner xanh có Hoàn tác.
- **LearnedRuleRow** (`.vv-rule`): công tắc + câu quy tắc + nguồn (“Học từ 3 lần bạn sửa · 26–28/10”) + Xem ví dụ + Xoá. Trong Hồ sơ kênh › Khi viết…, tối đa 20.
- **PlanCard** (`.vv-plan`): đầu “Kế hoạch · N bước” + câu gốc trong ngoặc kép; mỗi bước một dòng có ô tích (bỏ được), orb, giá; chân: tổng giá + Sửa câu + Huỷ + **Làm N bước** (nút chính, sáng). Thiếu thông tin → `__ask` với chip lựa chọn. Điện thoại: sheet.
- **InsightRow** (`.vv-insight`): câu nhận xét + 2 thanh so sánh nhóm (số video, trung vị 7 ngày) + nút Áp dụng; khối có nhãn **Sơ bộ** khi dưới 20 video.
- **ImageScore** (`.vv-imgscore`, `--low` dưới ngưỡng): góc dưới trái ảnh; ảnh dưới ngưỡng viền cam.

