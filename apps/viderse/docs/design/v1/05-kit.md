# Viderse — UI Kit

**UI Kit của Viderse là UI Kit của Noverse, chỉ đổi theme màu** (chốt 29/10/2026, đợt 19). Cỡ, bo góc, bóng, chuyển động, cách hỏi xác nhận, menu, select, combobox giống hệt Noverse. Khác duy nhất:

| Noverse | Viderse | Token |
|---|---|---|
| Tím — hộp kiểm, radio, công tắc bật, thanh trượt, focus, dấu tick, link, icon tab đang chọn, thẻ đã chọn | **Đỏ** | `--vv-accent`, `--vv-accent-strong`, `--vv-accent-soft`, `--vv-accent-bg/-tint/-border/-focus/-ring/-mark` |
| Đỏ — nút xoá vĩnh viễn, nút nguy hiểm làm lại được, lỗi, mục menu nguy hiểm | **Cam** | `--vv-danger`, `--vv-danger-fg`, `--vv-danger-text(-2)`, `--vv-danger-soft/-tint/-border/-focus/-ring` |
| Nút chính sáng, nền, chữ, xám, xanh lá, vàng | giữ nguyên | `--vv-primary`, `--vv-bg`, `--vv-text*`, `--vv-st-*` |
| Orb Kiến trúc sư · Người viết · Biên tập · Trọng tài | Orb Trinh sát · Đạo diễn · Hoạ sĩ · Kiểm định | `--vv-orb-*` |

- **Board:**
  - `Kit` (máy tính). Tweak:
    - `section=foundation|confirm|menus|select|inputs|overlays|video|agent`. Năm nhóm `confirm` → `overlays` là 5 nhóm của Kit Noverse; `foundation`, `video`, `agent` là phần riêng Viderse.
    - `confirm=pause|overwrite|logout|video|channel|stopAll|leave` — 7 tình huống xác nhận.
    - `open=source|lang|model|tags|none` — ô chọn đang mở ở nhóm Select.
  - `MKit` (điện thoại, tweak `section=bars|controls|lists|sheets|agent`).
  - Mọi thứ trên hai board đều bấm thử được.
- **Mã:**
  - [tokens.css](tokens.css) là toàn bộ giá trị.
  - [kit.css](kit.css) là lớp thành phần `.vv-*` chỉ dùng biến token.
  - Hai board dựng bằng chính `kit.css`, nên những gì thấy trên board là giá trị thật.
- **Luật dùng từng thành phần** vẫn ở [04-components.md](04-components.md). File này chỉ nói *lấy mảnh nào, tên gì, biến thể nào*.

## Cách dùng trong app

1. **Dùng lại component của Noverse.** Viderse là nhánh của Noverse: lấy nguyên component UI Kit của Noverse (cùng tên, cùng props, cùng hành vi bàn phím). Chỉ thay file token màu bằng [tokens.css](tokens.css). Không vẽ lại.
2. **Nạp:** `tokens.css` rồi `kit.css`, vào `globals.css` (Gói V0). Đặt lớp `.vv-root` ở `<body>`.
3. **Thành phần riêng Viderse** (bảng thứ hai): làm thêm, bọc đúng lớp `.vv-*`, biến thể qua prop.
4. **Trạng thái:**
   - Dùng thuộc tính ARIA đúng nghĩa (`aria-checked`, `aria-selected`, `aria-expanded`, `aria-disabled`, `aria-invalid`, `aria-current`); CSS bám theo các thuộc tính này.
   - `.is-active` là mục đang trỏ bằng phím trong menu / danh sách chọn.
   - `.is-hover`, `.is-focus`, `.is-open`, `.is-error` chỉ để vẽ trạng thái tĩnh trên board.
5. **Trang `/dev/kit`:**
   - Chỉ chạy khi dev.
   - Dựng lại hai board bằng component thật, dùng để so ảnh khi review (Gói V0 nghiệm thu).
6. **Icon:**
   - Cùng bộ Iconsax nét 24px với Noverse (mask SVG), dùng qua `<Icon name>`. Bản đặc `-b` chỉ cho mục đang mở ở thanh dưới.
   - Danh sách có ở `Kit` › Nền tảng › Icon.

## Thành phần lấy từ Noverse

| Component | Lớp | Biến thể / trạng thái | ARIA | Board |
|---|---|---|---|---|
| `Button` | `.vv-btn` | `variant`: `primary` (sáng) · `outline` (mặc định) · `quiet` · `danger-soft` (cam nhạt, làm lại được) · `danger` (cam đặc, xoá vĩnh viễn); `size`: `sm` 30 · `md` 36 · `m` 44 · `m-lg` 48 · `icon`; `loading`; `disabled` + `reason`; `hold` (giữ để xác nhận) | `aria-disabled`, `aria-busy`; nút chỉ có icon cần `aria-label` + tooltip | Kit › Ô nhập & nút · MKit › Điều khiển |
| `ConfirmDialog` | `.vv-scrim--center` · `.vv-dialog` · `__head/__icon(--warn/--danger/--ok/--agent)/__text/__title/__sub/__list/__compare/__ver/__err/__actions` | 7 tình huống: thường (lựa chọn radio + Không hỏi lại) · cảnh báo (so hai bản) · làm lại được (cam nhạt) · không hoàn tác (danh sách hệ quả, cam đặc) · gõ tên để xác nhận · giữ để xác nhận · rời khi chưa lưu; kết quả Thành công / Gặp lỗi; toast có Hoàn tác | `role=alertdialog`, `aria-labelledby/-describedby`, focus vào Huỷ khi nguy hiểm, Esc huỷ | Kit › Xác nhận |
| `AccountMenu` | `.vv-pop--float` · `.vv-opt` · `.vv-opt__text/__desc/__count/__chev` · `.vv-menu__group` · `.vv-menu__sep` | đầu menu (avatar + tên + dung lượng dự án), mục có số / phím / mũi tên, mục nguy hiểm | `role=menu/menuitem`, ↑↓ Enter Esc | Kit › Menu |
| `RowMenu` | `.vv-row` + `.vv-row__more` + `.vv-pop` | ⋯ chỉ hiện khi rê dòng (cảm ứng: luôn hiện), menu con lật trái (`.vv-pop-side`), mục nguy hiểm ở cuối mở hộp xác nhận | `aria-haspopup=menu`, `menuitemradio` | Kit › Menu |
| `Select` | `.vv-select` · `__value(.is-placeholder)` · `__chev` + `.vv-pop` `role=listbox` · `.vv-opt` · `.vv-opt__tick` | 6 trạng thái: mặc định, chưa chọn, rê, mở/focus, lỗi, vô hiệu; < 7 lựa chọn | `aria-haspopup=listbox`, `aria-expanded`, `aria-selected` | Kit › Select |
| `RichSelect` | `.vv-select--rich` · `.vv-opt--rich` | ảnh, mô tả, nút phụ (vẽ thử) không làm đóng danh sách | như trên | Kit › Select |
| `Combobox` | `.vv-fieldbox` + `input role=combobox` + `.vv-pop` nhóm `.vv-menu__group` | gõ để lọc, tô phần khớp `.vv-mark`, đang tải, lỗi tải + Thử lại, không khớp, thêm nhà cung cấp | `aria-autocomplete=list` | Kit › Select |
| `MultiSelect` (thẻ) | `.vv-fieldbox--wrap` · `.vv-token` + `.vv-pop` `aria-multiselectable` · `.vv-check` | tối đa N (đếm `.vv-counter`), ⌫ xoá thẻ cuối, tạo thẻ mới | `role=combobox` | Kit › Select |
| `Input` · `Field` | `.vv-field` (`--m`) · `.vv-fieldbox` (`__affix`, `__suffix`, nút xoá) · `.vv-label` · `.vv-label--group` · `.vv-help` · `.vv-err` · `.vv-counter` | mặc định, đang chọn, có nút xoá, lỗi (viền + vòng cam), có tiền tố/hậu tố, không sửa được (ghi nơi đổi) | `aria-invalid`, `aria-describedby` → `.vv-err` | Kit › Ô nhập & nút |
| `Textarea` | `textarea.vv-field` | đếm ký tự, ⌘ Enter gửi | | Kit › Ô nhập & nút |
| `Stepper` | `.vv-stepper` (`--m`) | `min`, `max` (nút mờ ở giới hạn) | `role=group` | Kit, MKit |
| `Slider` | `.vv-range` + `.vv-range__bubble` (`--p` = %) | vạch dưới | `input type=range` | Kit › Ô nhập & nút |
| `Checkbox` · `Radio` | `.vv-check` · `.vv-radio` (`--m`) · `.vv-checkrow` | bật, `mixed`, vô hiệu | `role=checkbox/radio` | Kit, MKit |
| `ChoiceRow` | `.vv-choice` (`--lg` trong hộp thoại) · `__text/__label/__note` | đang chọn: viền + nền màu nhấn | `role=radio` | Kit, MKit › Hàng |
| `Switch` | `.vv-switch` (`--m`) | bật (đỏ), đang bật (spinner cạnh), vô hiệu | `role=switch`, `aria-checked` | Kit, MKit |
| `Segmented` | `.vv-seg` (`--thumb` + `.vv-seg__thumb` trượt · `--bar` thanh tab đầu trang · `--sm` · `--m` · `--fill` chia đều, đợt 34) — mục co theo nhãn, không cắt “…”, hẹp thì cuộn ngang <!-- r34 --> | | `role=radiogroup` hoặc `tablist` | Kit (› Ô nhập & nút › Nhóm chọn), MKit |
| `Tooltip` | `.vv-tipwrap` > `.vv-tip` (`--below`, `--wrap`) | hiện sau 0,4s, kèm phím | `role=tooltip` | Kit › Gợi ý & trạng thái |
| `ExplainPopover` | `.vv-pop--float.vv-pop--note` | bấm để mở, có link `.vv-link` | `role=dialog` | Kit › Gợi ý & trạng thái |
| `Badge` · `Chip` · `Kbd` | `.vv-tag` + `.vv-tag__dot` (`--working/--ok/--review/--issue/--muted/--accent`) · `.vv-chip` · `.vv-kbd` | | | Kit › Gợi ý & trạng thái |
| `Banner` | `.vv-banner` (`--info/--ready/--review/--issue/--director`) · `__body/__title/__text/__actions` | lỗi luôn có cách sửa | `role=status` / `alert` | Kit, MKit |
| `Toast` | `.vv-toast` (`--ok/--issue/--sm`) · `.vv-toast-in` | viên cho kết quả thao tác, một hành động (Hoàn tác / Mở lại / Xem); thẻ 300px cho cần bạn / Cần xử lý | `role=status/alert` | Kit › Xác nhận, Menu; `Activity` · `view=toasts` |
| `Skeleton` · `Spinner` · `Shimmer` · `Empty` | `.vv-skel` · `.vv-spinner` · `.vv-shimmer` · `.vv-empty` (`__icon/__title/__text`) | | `role=progressbar` khi có % | Kit › Gợi ý & trạng thái |
| `Card` · `Stage` | `.vv-card` (`__title`) · `.vv-stage` | | | mọi nơi |
| `Sheet` (điện thoại) | `.vv-sheet` · `__grab` · `__title` | thao tác, xác nhận (nút chính sáng) | `role=dialog aria-modal` | MKit › Sheet |

## Thành phần riêng Viderse

| Component | Lớp | Biến thể / trạng thái | ARIA | Board |
|---|---|---|---|---|
| `StatusLabel` | `.vv-status--queued/working/review/issue/ready/posted/paused` | Đang làm = vòng xoay | | Kit › Video & thẻ |
| `Tag` (riêng Viderse) | `.vv-tag--ref/--next/--auto` | | | Kit › Video & thẻ |
| `Count` · `Score` | `.vv-count` (`--solid/--info`) · `.vv-score` (`--low`) | | | Kit › Video & thẻ |
| `Progress` · `WaitTile` | `.vv-progress` (`--working/--artist/--ready/--issue`) · `.vv-wait` | mặc định màu nhấn | `role=progressbar` | Kit › Video & thẻ |
| `ChipSelect` | `.vv-chip` | `aria-expanded`, `.is-custom` (nhãn “riêng”), chỉ hiển thị | `aria-haspopup` | Kit › Video & thẻ |
| `FilterPill` · `Tabs` | `.vv-pill` · `__dot` · `.vv-tabs` · `.vv-tab` · `__n` | gạch tab màu nhấn | `role=radio` · `tab` | Kit › Video & thẻ, MKit |
| `Row` · `Avatar` · `Thumb` | `.vv-row` (`--m` 56px, `.is-on`) · `.vv-avatar` (`--s`, `--bg`, `--ring`, `--fg`) · `.vv-thumb` (`__bl`, `__br`, `__dur`) | nhãn ảnh chỉ ở cạnh dưới | `aria-current` | Kit › Video & thẻ |
| `VideoCard` · `VideoRow` · `ChannelRow` · `ChannelSwitcher` · `AgentCard` · `StatTile` | ghép từ các lớp trên | xem board | | Kit › Video & thẻ |
| `Orb` | `.vv-orb--scout/director/artist/qa` + `.vv-live` | | `aria-hidden` | Kit › Nền tảng |
| `AppBar` · `NavHeader` | `.vv-appbar` + `.vv-title-m` · `.vv-navbar` · `__title` · `.vv-ib` | gốc / con (‹ hoặc ×) | | MKit › Đầu trang |
| `MobileTabBar` | `.vv-tabbar` · `__cap/__ind(--i)/__item/__label/__new/__badge` · `.is-mini` | 5 mục, mục giữa nút tròn đỏ (chỗ duy nhất đỏ làm nền) | `aria-current=page` | MKit › Đầu trang |
| `WhyLink` · `ReasonPopover` | `.vv-why` · `.vv-reason` · `__head/__text/__facts/__fact/__src/__fix` | màu trợ lý qua `--ag` | `aria-haspopup=dialog`, `aria-expanded` | Kit › Trợ lý, MKit › Trợ lý |
| `SuggestionCard` | `.vv-suggest` (`--compact` · `--m`) · `__who/__title/__ev/__actions` | nút sáng nhỏ, Bỏ qua, ⋯ | `article`; ⋯ có `aria-label` | Kit › Trợ lý, MKit › Trợ lý |
| `RecapView` | `.vv-recap` · `__when/__lead/__sec/__line/__text/__tiles/__tile` · `.vv-tag--auto` | máy tính / điện thoại | `role=region aria-label="Bản tin"` | Kit › Trợ lý, `Main` · `agent=recap` |
| `AutonomyControl` | `.vv-autonomy` · `.vv-arow` (`--stack` · `--m`) · `__label/__hint` · `.vv-guard` | 3 mức + Tuỳ chỉnh | `role=radiogroup`, dòng chọn `aria-haspopup=listbox` | Kit › Trợ lý, `Channel` · `tab=sources` |
| `LearnHint` · `LearnedRuleRow` | `.vv-learn` (`--artist`) · `__text/__actions` · `.vv-rule` · `__text/__src/__actions` | | `role=status` | Kit › Trợ lý, `Channel` · `tab=settings` |
| `PlanCard` | `.vv-plan` · `__head/__step/__cost/__foot/__ask` | bước bỏ tích được; Làm N bước là nút chính | bước `role=checkbox aria-checked` | Kit › Trợ lý, `Main` · `agent=plan` |
| `InsightRow` · `ImageScore` | `.vv-insight` · `__text/__bars/__bar` (`--w`, `--c`) · `.vv-imgscore` (`--low`) | Sơ bộ < 20 video | | Kit › Trợ lý, `Review` · `score=true` |

## Kit cũ → kit mới (khi sửa code đang có) <!-- r20-screens -->

Từ đợt 20, cả 23 artboard màn hình đã vẽ theo kit mới. Bảng này dùng khi sửa code còn theo kit đợt 15 (hoặc giao diện yt-factory cũ):

| Code / giao diện cũ | Đổi thành |
|---|---|
| Nút đỏ đặc (Làm video, Tạo kênh, Chạy tiếp, Duyệt N video, Xuất…) | `.vv-btn--primary` (sáng) |
| Nút viền cam “Xoá …” trong hộp thoại | `.vv-btn--danger` (cam đặc); việc làm lại được (Bỏ liên kết, Đăng xuất, Bỏ thay đổi) dùng `--danger-soft` |
| Công tắc bật màu xanh lá | `.vv-switch` bật màu nhấn đỏ |
| Hộp kiểm / radio đang chọn màu trắng | màu nhấn đỏ |
| Nút 34px bo 9px · ô nhập bo 10px viền trắng khi focus | nút 36px bo 10px · viền + vòng đỏ khi focus |
| Menu `.vv-menu` bo 12px, mục cao 8px đệm | `.vv-pop` bo 14px, `.vv-opt` cao ≥ 34px, mục đang trỏ `.is-active` |
| Hộp thoại bo 16px, nền `#161618` | `.vv-dialog` bo 18px, nền `--vv-dialog`, icon 42px |
| Banner câu một dòng, chữ màu loại | `.vv-banner` tiêu đề + câu xám + nút nhỏ bên phải |
| Toast thẻ chữ nhật cho kết quả thao tác | `.vv-toast` hình viên; thẻ 300px chỉ cho “cần bạn” / Cần xử lý |
| Chip 26px bo 7px | `.vv-chip` 28px bo 8px |
| Thẻ bo 14px | `.vv-card` bo 16px (thẻ con trong danh sách 14px) |

## Luật chung (tóm tắt, chi tiết ở 04-components.md)

- **Nút:**
  - Một màn một nút chính (sáng) — kể cả “Làm video”, “Tạo kênh”.
  - Cam đặc chỉ cho xoá vĩnh viễn, và chỉ trong hộp xác nhận; cam nhạt cho việc nguy hiểm làm lại được.
  - Đỏ không làm nền nút, trừ nút + tròn ở thanh dưới điện thoại.
- **Hỏi xác nhận:** tiêu đề là câu hỏi có tên đối tượng; thân nói hậu quả; nút ghi đúng động từ; Huỷ bên trái; Esc huỷ; Enter chỉ chạy khi không nguy hiểm; việc dễ hoàn tác thì làm luôn kèm Hoàn tác.
- **Khoá:** thứ bị khoá luôn nói lý do, ngay cạnh chỗ bị khoá.
- **Màu:**
  - Đỏ là màu nhấn: đang chọn, đang bật, focus, dấu tick, link.
  - Cam là lỗi và xoá.
  - Vàng là “tới lượt bạn”.
  - Xanh lá là sẵn sàng, xong, đã kết nối.
- **Không xuống dòng:** chữ trên nút, tab, chip, kbd không bao giờ xuống dòng; chữ dài cắt “…” kèm `title`.
- **Vùng bấm:**
  - Máy tính ≥ 24px (nút 30–36px).
  - Điện thoại ≥ 44px; hàng 56px; nút lớn 48px.

## Khi thêm thành phần mới

1. Kiểm tra Kit Noverse đã có chưa. Có thì dùng lại, chỉ đổi màu theo bảng đầu file.
2. Chưa có ở Noverse thì kiểm tra `Kit`/`MKit` của Viderse.
3. Nếu chưa có ở cả hai:
   - Thêm lớp vào `kit.css`, chỉ dùng token.
   - Thêm một ô trên board `Kit` (qua `tools/gen_kit.py`) hoặc `MKit` (`tools/gen_mkit.py`).
   - Thêm một dòng vào bảng trên và luật vào `04-components.md`.
4. Artboard màn hình chỉ được dùng thành phần đã có trong kit.
