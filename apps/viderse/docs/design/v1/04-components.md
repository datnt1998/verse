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

## Mọi màn dùng UI Kit <a id="c-kit-only"></a> <!-- r37 -->
Từ đợt 37, **không màn nào vẽ tay nút, ô chọn, nhóm chọn, tab hay menu**. Board nào cũng dựng bằng class của `kit.css`; code dùng thành phần tương ứng của `@noverse/ui`. Chuyển động lấy từ kit, không tự đặt:

| Thành phần | Class kit · thành phần code | Chuyển động (như Noverse) |
|---|---|---|
| Nút | `.vv-btn` (`--primary` · mặc định viền · `--quiet` · `--icon` · `--danger(-soft)` · `--sm` · `--m`) · `Button` | hover nền `--vv-hover`, nhấn thu `.98`, `aria-disabled` mờ .42 |
| Nhóm chọn | `.vv-seg` (`--sm` · `--m` · `--fill` · `--bar`) · `Segmented` | **viên trượt** tới mục chọn, 320 ms `cubic-bezier(.3,.7,.2,1)`; đo theo mục thật (rộng, vị trí); nhấn thu `.97` |
| Tab gạch chân | `.vv-tabs` + `.vv-tab` · `Tabs` | màu chữ và gạch đổi 240 ms |
| Chip chọn (khoá : giá trị ▾) | `.vv-chip` · `ChipSelect` | hover viền sáng hơn; mũi tên lật 180° khi mở (180 ms) |
| Ô chọn trong biểu mẫu | `.vv-select` · `Select` | viền focus, mũi tên lật |
| Menu / danh sách chọn | `.vv-pop` + `.vv-pop-in` · `.vv-opt` · `Menu` | mở: trồi 4px + scale .97 → rõ (`--vv-dur-pop`) |
| Toast | `.vv-toast` · `.vv-tcard` · `toast` | xem Toast |

- Nhóm chọn nào cũng có viên trượt: board dùng một đoạn script nhỏ (`data-vv="glide"`) đo mục đang chọn và đặt `--gx`/`--gw`; code dùng `Segmented` của `@noverse/ui` (đã đo bằng JS). Không còn kiểu đổi nền tức thì.
- Không phải nhóm chọn: dải ảnh thu nhỏ, lưới màu, thẻ lựa chọn có mô tả — giữ thành phần riêng (`VideoThumb`, `.vv-choice`…).
- `prefers-reduced-motion`: tắt mọi chuyển động trên, giữ đổi trạng thái.
- Board có kiểm kê tự động (`tools/kit_audit.py`): số nút/ô chọn/nhóm chọn/menu vẽ tay phải bằng 0 trước khi bàn giao.

## AppHeader <!-- c-app-header -->
- Cao 48px; viền dưới `--vv-hairline`.
- **Trái:** logo ống kính 26px + ngữ cảnh: `ChannelSwitcher` ở Bàn dựng; breadcrumb ở Kênh (“Kênh của tôi”), Khám phá (“Khám phá / <đối thủ>”) và trang phụ. <!-- r11-audit -->
- **Giữa:** `MainNav`.
- **Phải:** `JobsPill`, nút Cài đặt có chấm báo, avatar.
- Dưới 768px là khung điện thoại (một mốc duy nhất, #16). Từ 768px trở lên, `MainNav` không đủ chỗ thì cuộn ngang, không rớt dòng. <!-- r32 -->
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
- Khi hàng chờ tạm dừng: icon `i-pause` màu `--vv-st-queued` (#8E8E99, “Chờ lượt, Tạm dừng”) + “Đã tạm dừng lúc HH:MM · M chờ lượt” (`pausedAt`). <!-- r31-v27 -->
- Mất kết nối: icon `i-danger` màu `--vv-danger` + “Mất kết nối · đang thử lại”, chữ `--vv-danger-text`. Tooltip: “Không kết nối được máy chủ Viderse · số liệu cập nhật lại khi có kết nối”.
- Cùng khung mọi trạng thái: cao 30px, đệm ngang 10px, khe 8px, chữ 12,5px; chiều rộng theo câu. **Điện thoại (đợt 36): không dùng viên chữ** — nút tròn 36px có vòng tiến độ, xem Đầu trang điện thoại › AppBar. <!-- r36 -->
- Bấm mở **popover Việc nền** (desktop, 448px) hoặc sheet Việc nền (điện thoại, cùng hai tab **Đang chạy · Thông báo**); cuối popover có “Mở Hàng chờ”.
- Có thông báo cần bạn chưa đọc → chấm vàng trên viên.
- Máy rảnh: “Máy rảnh”, chấm xám #5E5E68, chữ mờ. <!-- r31-v27 -->
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
- Dưới thẻ là dòng phím tắt và hạn mức ảnh hôm nay. <!-- r32 -->
- Gõ `/` mở danh sách lệnh ngay trên ô (chỉ lệnh hợp lệ với video đang mở; Mọi kênh không có video thì không có lệnh): `/redraw <cảnh>` vẽ lại một cảnh · `/provider <chatgpt|grok|gemini>` đổi nguồn vẽ cho video này · `/pause` · `/resume` · `/export-anyway` xuất dù điểm chưa đạt (chỉ ở Kiểm định) · `/posted [link]` đánh dấu đã đăng. Mỗi dòng: lệnh (mono) + một câu tiếng Việt; ↑↓ chọn, Enter điền. Lệnh lạ → thẻ “Chưa làm được việc này.” như câu không hiểu (#74).
- Màn rộng: tối đa 880px, căn giữa ([Bố cục màn rộng](#c-wide)).
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
- **Bề rộng mục (đợt 34, #80):** <!-- r34 --> mặc định mỗi mục **co theo nhãn** (đệm ngang 12px; `--sm` 10px), nhãn **không bao giờ cắt “…”**. Nhóm hẹp hơn nội dung (thường ở 390) thì **cuộn ngang** trong nhóm, ẩn thanh cuộn, mục đang chọn tự cuộn vào tầm nhìn. Chia đều chỉ khi nhóm chiếm hết bề rộng và các nhãn ngắn ngang nhau: `.vv-seg--fill` (máy tính) hoặc `--m` (điện thoại); nhãn dài hơn phần chia vẫn giữ đủ chữ (`min-width: max-content`). Viên trượt đo theo bề rộng thật của mục, không tính theo `100% / n`. Áp cho Khám phá › Sắp xếp (Nổi trội · Mới nhất · Lượt xem) và Lịch đăng › Vùng giờ (VN · New York) trên máy tính và điện thoại; board đã vẽ co theo nhãn.
- **Tabs trang:** gạch dưới màu nhấn 2px, số đếm xám cạnh tên.
- Trên điện thoại: cuộn ngang, cao 44px.

## Sheet (điện thoại) <!-- c-sheet -->
- Nền mờ `rgba(0,0,0,.6)`, sheet bo trên 22px, có tay cầm.
- Tối đa 86% chiều cao. Đóng bằng chạm nền hoặc vuốt xuống.
- Tiêu đề sheet 18px/600 (`.sheet-t`), lề 16px. <!-- r14-mobile -->
- Nút chính trong sheet: **sáng** (`.vv-btn--primary`, 48px) cho mọi việc chính, kể cả đưa video vào làm hoặc tạo kênh. Nút phụ viền. Xoá vĩnh viễn: cam đặc. <!-- r19-kit -->
- Dùng cho Hàng chờ / Việc nền, Đổi kênh, Góp ý vẽ lại, Ảnh của video, Viết lại / Lịch sử nội dung, chọn kênh, thao tác đối thủ, xác nhận.

## Đầu trang điện thoại (AppBar · NavHeader) <!-- c-mobile-header --> <!-- r14-mobile -->
- **AppBar** (màn gốc) — chuẩn hoá đợt 36 <!-- r36 -->:
  - Một hàng cao 52px, đệm 0 12 0 16. Trái: logo 24px · `ChannelSwitcher` (chỉ Bàn dựng, Kênh). Phải (đẩy sát mép, khe 8px): **nút Việc nền** · **avatar** (mở Tài khoản và cài đặt).
  - **Mọi phần tử cao 36px**, cùng nền `--vv-surface-1` và viền `--vv-card-border`; nút tròn 36px; vùng chạm 44px (`.vv-mbtn::before`). Logo không phải nút chạm riêng.
  - **Nút Việc nền** (`.vv-jobsbtn`, tròn 36px, thay viên chữ “2 · 12”): đang chạy = vòng tiến độ 2px quanh nút (màu `--vv-red`, tiến độ trung bình các việc đang chạy) + số việc đang chạy ở giữa (13px/600, số đều) · máy rảnh = icon `i-activity` xám, **không chữ “Rảnh”** · đã tạm dừng = vòng xám đầy + icon tạm dừng · mất kết nối = icon `i-danger` cam, viền cam nhạt. Câu đầy đủ ở `aria-label` (copy › Hàng chờ và nút việc nền).
  - **Chấm thông báo** (`.vv-notice-dot`): 10px `--vv-st-review`, viền 2px màu nền, đặt `top: -1px; right: -1px` để tâm chấm nằm **trên mép tròn** ở góc trên phải — không đè lên chữ, không lơ lửng ngoài nút. Chỉ hiện khi có thông báo cần bạn **chưa đọc**; ẩn khi mất kết nối. Bấm nút khi có chấm → sheet Việc nền mở ở tab **Thông báo**.
  - **Bộ đổi kênh** (`.vv-mswitch`): co trước mọi thứ (`min-width: 0`), tên cắt “…”; nút Việc nền và avatar không bao giờ bị đẩy ra hay co lại. Mọi kênh: chồng 3 avatar 22px, **mỗi avatar một chữ cái** (M · C · Q) để chữ không bị che; một kênh: avatar 26px hai chữ + tên.
  - **Trường hợp trống:** chưa có kênh → thay bộ đổi kênh bằng nút “+ Tạo kênh” cùng cỡ (mở Tạo kênh); một kênh → vẫn là bộ đổi kênh (sheet có Tạo kênh); máy rảnh và chưa có thông báo → icon xám, không chấm; Duyệt, Khám phá → chỉ logo, hai nút phải giữ đúng chỗ.
  - Dưới thanh là tiêu đề lớn 24px/600 và dòng phụ như cũ.
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

## Toast <!-- c-toast --> <!-- r35 -->
Đợt 35 làm lại theo Noverse (`platform/ui` Toast · `Jobs.dc.html` §1.4 của Noverse). Hai dạng, cùng một chất liệu:

- **Chất liệu chung:** kính mờ `--vv-toast-glass` + `backdrop-filter: blur(18px) saturate(150%)`, viền `--vv-toast-border`, bóng `--vv-toast-shadow`. Màu loại chỉ ở icon, viền nhạt và vệt loang góc trên trái; nền không đổi màu.
- **Đếm ngược = vòng mảnh quanh nút đóng** (`.vv-toast-x`: SVG r 12, nét 1,6, màu loại, `stroke-dashoffset` chạy 160 ms linear). Không vạch ở đáy, không dòng chữ “Tự đóng sau 5 giây…”. `title` của nút: “Tự đóng sau N giây”; rê chuột: vòng mờ 30%, dừng đếm, `title` “Đã dừng tự đóng”. Toast ở lại tới khi đóng thì nút đóng không có vòng.
- **Viên — kết quả thao tác của bạn** (`.vv-toast`, `--ok` · `--review` · `--issue`, trung tính không icon): cao 40px, bo tròn, icon 16px · một câu (`.vv-toast__text`, cắt “…”) · tối đa một nút nhỏ (Hoàn tác / Thử lại / Xem) · nút đóng. Hoàn tác ghi giây còn lại: “Hoàn tác · 4s”.
  - Vị trí: **giữa đáy cột nội dung**, cách mép dưới 18px; màn có ô Nhắn trợ lý hoặc thanh hành động ở đáy thì nằm trên thanh đó 12px. Không đặt ở góc phải.
  - Thời gian: 6 giây; có Hoàn tác thì 5 giây. Viên mới thay viên cũ (cùng `key` thì cập nhật tại chỗ).
- **Thẻ — việc nền cần bạn hoặc vừa xong** (`.vv-tcard` trong `.vv-tstack`): rộng 328px, bo 18px, đệm 12/12/13. Icon tròn 34px trên nền loang màu loại · tiêu đề 13,5px/600 **một dòng** · câu phụ 12px tối đa 2 dòng · nút 28px · nút đóng góc trên phải.
  - Loại: `--review` Chờ duyệt (cam nhạt, cờ) · `--issue` Cần xử lý (cam, `role=alert`, ở lại tới khi đóng) · `--ok` Sẵn sàng · trợ lý làm xong `--scout/--director/--artist/--qa`: orb 22px của trợ lý + dấu tick mint 15px, loang màu trợ lý (`--vv-*-tint`).
  - Vị trí: góc phải dưới, cách mép 12/16px. **Tối đa 2 thẻ**; thẻ mới nhất ở dưới; thẻ cũ lùi lên (`.vv-tcard--back`: dịch 4px, scale .955, mờ 60%, giảm bão hoà). Rê chuột hoặc focus vào chồng thì cả hai bung rõ và dừng đếm. Thẻ thứ ba đến thì thẻ cũ nhất không phải Cần xử lý rời đi (vẫn ở tab Thông báo).
  - Chỉ thẻ trước có nút sáng; thẻ lùi sau dùng nút viền (một nút chính mỗi lúc).
  - Thời gian: 6 giây; Cần xử lý ở lại.
- **Chuyển động:** vào trồi 18px + scale .94 + blur 4px → rõ, 500 ms nảy nhẹ (`.vv-toast-in`); ra rơi xuống mờ dần + blur (`.vv-toast-out`, 420 ms), khung thu chiều cao để thẻ phía trên trượt xuống êm. `prefers-reduced-motion`: tắt.
- **Điện thoại:** viên giữa đáy, trên thanh dưới 12px, tự đóng sau 2,5 giây (Hoàn tác 5 giây). Thẻ việc nền rộng màn trừ lề 12px, **một thẻ**, vuốt xuống để đóng.
- Nút trong toast trắng trung tính, không đỏ. Không toast tiến độ. Gộp trong 10 giây (gộp thì đổi tiêu đề, không thêm dòng “Gộp N thông báo”). Esc đóng toast mới nhất.
- Artboard: `Activity` · `view=toasts` = `Dialogs` · `dialog=toasts` · `Kit` › Gợi ý & trạng thái › Toast · `MKit` › Sheet › Toast · `Editor` · `focus=scene&img=done`.
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

## Lưu trữ trên máy chủ: StorageProviderPicker · S3ConfigForm · ConnectionTest · MigrationCard · AdminStorageStrip <a id="c-storage-server"></a> <!-- r29-s3 -->
- **StorageProviderPicker:** lưới 2 cột, mỗi thẻ là một nút: logo đơn sắc (chữ viết tắt trong vòng tròn 40px), tên, một dòng gợi ý, cách gửi gợi ý, mũi tên. Kho đang dùng có nhãn Đang dùng và nền chọn `rgba(255,255,255,.08)`, không viền màu một bên.
- **S3ConfigForm:** lưới 2 cột; địa chỉ và Account ID chiếm cả hàng. Ô khoá, địa chỉ, tên kho dùng chữ mono. Khoá đã lưu là `FieldBox` chỉ đọc (`••••c41d` · Đã lưu · *Đổi khoá*). Công tắc dùng `vv-switch`. *Lấy ở đâu?* là nút chữ ở đầu thẻ, mở một vùng hướng dẫn tại chỗ. Cách gửi tệp là `vv-choice--lg` dạng radio, hai dòng +/− màu xanh/vàng.
- **ConnectionTest:** danh sách bước, mỗi hàng: biểu tượng trạng thái (chờ: vòng rỗng · chạy: spinner · đạt: tick xanh · không cần: gạch · lỗi: tam giác cam), tên, mô tả, thời gian. Kết quả là một khối dưới danh sách: xanh khi đạt, cam khi lỗi (lý do, Cách sửa, đoạn mã có *Chép*, nút sửa).
- **MigrationCard:** cũng là một dòng trong Việc nền. Hai `vv-progress` (số tệp, dung lượng), bảng nhóm, dòng tốc độ và Đã chép · Đã kiểm tra · Lỗi; khi lỗi thì có bảng Nhóm · Video · Lý do. Huỷ hỏi lại tại chỗ, không mở hộp thoại.
- **AdminStorageStrip:** dải cam nhạt dưới thanh trên cùng, chỉ quản trị viên thấy, ở mọi trang khi kho mất kết nối; một câu + *Mở Lưu trữ*.

## Menu <!-- r28-explore -->
- Mọi menu ⋯ dùng `ActionMenu` của kit: đệm hàng 6/10, cách hàng 0, bo 8; chỉ đổi `width` (menu đối thủ 272px). Hàng có dòng phụ giữ đệm này, không tăng riêng.

## AccountMenu · AppSwitcherRow · AuthShell <a id="c-account"></a> <!-- r30-sso -->
- **AccountMenu:** popover 336px dưới avatar đầu trang (`aria-haspopup="menu"`), đóng khi bấm ngoài hoặc Esc. Thứ tự: thẻ người dùng · Ứng dụng · Tài khoản · Quản lý thành viên (quản trị viên) · Đăng xuất (cam, có dòng phụ). Điện thoại không có popover: avatar mở trang Tài khoản.
- **AppSwitcherRow:** logo app trong vòng tròn 32px, tên, địa chỉ (mono), bên phải nhãn *Đang mở* hoặc biểu tượng mở ở thẻ mới. Dòng đang mở có nền chọn `rgba(255,255,255,.08)`, không viền màu một bên. Không có quyền thì không có dòng.
- **AuthShell:** đầu trang có logo + chip trạng thái máy chủ (xanh: đang hoạt động · cam: không kết nối được / chưa đọc được tài khoản), nội dung một cột 400px giữa màn, chân trang chỉ ở Đăng nhập và Thiết lập. Một nút sáng mỗi màn.

## Bố cục màn rộng <a id="c-wide"></a> <!-- r32 -->
- Artboard vẽ ở 1440px. Từ **1600px** trở lên, khung vẫn bám mép (đầu trang, cột trái, cột phải giữ bề rộng), còn **nội dung cột giữa căn giữa** với bề rộng tối đa theo loại màn. Dưới 1600px không đổi gì.
- Loại màn → bề rộng tối đa:
  - Bàn làm việc, danh sách, bảng (`Main`, `Activity`, `Queue`, `Channel`, khung của `Dialogs`/`System`): `--vv-wide-content` 1280px.
  - Lưới thẻ và xem ảnh (`Explore`, `Library`, `Review`): `--vv-wide-grid` 1760px; lưới tự thêm cột (`repeat(auto-fill, minmax(…))`), thẻ không phình to.
  - Form, trang đọc (`Settings`, `Template`): `--vv-wide-read` 880px; khối đã có bề rộng riêng (Cài đặt 760px, Template 700px) giữ nguyên, chỉ căn giữa.
  - Công cụ (`Editor`) và màn căn giữa sẵn (`Auth`, `Onboard`, `CreateChannel`, `NewVideo`, `Brand`): không giới hạn thêm.
- **Ô Nhắn trợ lý** (Composer) và dòng phím tắt: tối đa `--vv-wide-composer` 880px, căn giữa dưới nội dung; thẻ Kế hoạch mở ngay trên ô dùng cùng bề rộng. Dòng chữ trong Nhật ký tiến độ, thẻ trợ lý không dài quá cột 1280px.
- Không phóng chữ, không tăng khoảng cách theo bề rộng; khoảng trống thừa nằm hai bên cột giữa.
- Cách làm: CSS `@media (min-width: 1600px)` trên con trực tiếp của cột giữa (`width: 100%; max-width: …; margin-inline: auto`), không dùng `zoom`/`scale`. Kit có lớp `.vv-wide`, `.vv-wide--grid`, `.vv-wide--read`, `.vv-composer-dock`. Artboard đánh dấu `r32-wide`; xem ở canvas bằng cách kéo khung artboard rộng ra.

## Badge số (MainNav) <a id="c-badge"></a> <!-- r32 -->
- `.vv-badge`: cao 18px, tối thiểu 18px, đệm ngang 5px, bo tròn, chữ 11px đậm 600 số đều, chữ `--vv-st-review` trên nền `--vv-badge-bg` (vàng nhạt 16%). Không dùng nền vàng đặc ở thanh trên máy tính; nền đặc chỉ là `.vv-tabbar__badge` của thanh dưới điện thoại.

## Ảnh, prompt và phiên bản: ImageStage · VersionStrip · PromptCard · RefList · DevelopEffect <a id="c-image"></a> <!-- r33-img -->
- **ImageStage:** khung 16:9 `width: min(100cqw, <độ rộng gốc>px, 100cqh × 16/9)` trong vùng `container-type: size`, căn giữa. **Không bao giờ lớn hơn kích thước gốc** (`width` của ảnh từ API) để màn rộng không làm mờ ảnh. Huy hiệu góc trái “3 · Cảnh 3 · 12:25–18:40 · bản 2” (bản chưa dùng: chữ vàng). Nút *So với bản N* (giữ hoặc bật, phím V) hiện bản trước tại chỗ.
- **VersionStrip:** ô 64×36 (Editor 52, điện thoại 72), viền 2px đỏ khi đang xem, số bản góc trái dưới, chấm xanh `--vv-st-ready` = bản đang dùng. Bản đang vẽ: ảnh trước mờ + DevelopEffect + vòng chờ. Dưới dải: “Bản 3 · theo góp ý “…” · 00:12”. Xem bản khác bản đang dùng → *Dùng bản N cho video*.
- **PromptCard:** nền `--vv-surface-1`, viền hairline, bo 12. Các phần xếp dọc, cách nhau bằng đường hairline; nhãn 10,5px chữ hoa + nguồn (“· prompt “Đêm xanh rêu””). Chữ prompt 12,5px/1,6 giữ nguyên ngôn ngữ đã gửi. Thay đổi so với bản trước: thêm = nền `rgba(166,237,193,.14)` chữ `--vv-st-ready`; bỏ = gạch, chữ mờ; *Ẩn thay đổi*. *Xem nguyên văn gửi máy chủ* = khối mono 11,5px, các phần nối bằng dòng trống + tham số (kích thước, chất lượng, số ảnh tham chiếu). **Sửa:** mỗi phần một textarea (focus viền đỏ nhấn; đã đổi viền xanh), *Hoàn tác* từng phần. Không tô màu một bên.
- **RefList:** thẻ 84×47 + “1 · Bố cục” + nguồn (“Khung đối thủ · Nocturne Stories 14:02”). Khi sửa có nút bỏ (×). **Thêm ảnh tham chiếu** (popover rộng bằng cột): tab Ảnh trong video · Khung video gốc · Tải lên; lưới 3 cột; *Dùng làm* Nhân vật / Bố cục / Phong cách / Đồ vật; chân “ChatGPT nhận tối đa 4 ảnh tham chiếu” · Huỷ · *Thêm 1 ảnh*. Vai trò sinh một câu trong phần Tham chiếu của prompt (“Image 2: the security guard’s appearance…”).
- **DevelopEffect** (`.vv-develop`, `.vv-dev-base`, `.vv-devcard`, `.vv-ring`, `.vv-reveal`, `.vv-glint`): nền = bản trước `blur(16px) saturate(.7) brightness(.72)`; ảnh xem dần `blur(5px)`; vệt sáng chéo màu Hoạ sĩ (`--vv-artist-ink` 10–20%) chạy 2,8 s; hạt nhiễu 10% overlay; thẻ trạng thái kính mờ ở giữa (orb Hoạ sĩ, tiêu đề, thời gian, 3 bước chấm: xong xanh, đang làm vàng nhấp nháy). Lộ ảnh: `blur(14px)→0`, `scale(1.025)→1`, 1 s; viền sáng `--vv-artist-ink` chạy một lần 1,6 s. Dải ảnh, Phòng vẽ ở Bàn dựng và ô ảnh trong Editor dùng cùng hiệu ứng thu nhỏ. `prefers-reduced-motion`: tắt vệt sáng, hạt, nhấp nháy; lộ ảnh thành đổi tức thì.
- **Chọn nhiều:** Shift + bấm ô trên dải → ô tích 16px góc trái dưới, viền trắng; cột phải thành danh sách ảnh đã chọn (ảnh + dòng đầu phần Cảnh).

## Đường dẫn thư mục <a id="c-path"></a> <!-- r34 -->
- **Đường dẫn trong giao diện** <!-- r34 -->: in `displayPath` của API (thư mục nhà thay bằng `~`), chữ `--vv-font-mono`, một dòng. Hết chỗ thì **cắt giữa theo thư mục**: giữ phần đầu (`~` hoặc `/Volumes/<tên ổ>`) và 2 thư mục cuối, `…` ở giữa (`/Volumes/Samsung T7/…/Kênh YouTube/Viderse Projects`); vẫn dài thì giữ 1 thư mục cuối; tên thư mục cuối quá dài mới cắt ký tự ở đầu tên. Không xuống dòng, không cắt cuối. `title` = `displayPath` đầy đủ; *Chép đường dẫn* và *Mở trong Finder* dùng đường dẫn thật. Bề rộng: dòng đường dẫn lấy hết chỗ còn lại trong thẻ, không đặt số cố định — máy tính 1440 và từ 1600 (khối Cài đặt giữ 760px): ≈ 430px giữa icon và hai nút (≈ 60 ký tự mono 11,5px); điện thoại 390: ≈ 290px (≈ 40 ký tự), nút xuống dòng riêng. FE đo bằng bề rộng thật (canvas `measureText` hoặc thử cắt dần), không đếm ký tự cứng.

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

