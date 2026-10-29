# 17 · Bộ thành phần: xác nhận, menu, select, combobox, ô nhập

Artboard `Kit.dc.html` (10 · Bộ thành phần) có 5 tab. Mọi thứ đều bấm được và chạy bằng bàn phím.

Tweak:
- `section`: `confirm` · `menus` · `select` · `inputs` · `overlays`;
- `confirm`: chọn 1 trong 7 tình huống;
- `open`: select đang mở (`voice` · `lang` · `model` · `genres` · `none`).

Token màu và độ bo theo `03-tokens.md`. File này bổ sung các primitive cho `04-components.md`.

---

## 1. Hộp xác nhận (`ConfirmDialog`)

### 1.1 Cấu trúc

`role="alertdialog"`, rộng 460, padding 20, bo 18, nền `#18181A`. Thứ tự từ trên xuống:

1. **Icon tone** 42 px, bo 12 (việc của agent thì dùng orb agent trên quầng màu).
2. **Tiêu đề** 16/600, dạng câu hỏi có tên đối tượng.
3. **Thân bài** 13.5 `#B4B4BE`, nói hậu quả.
4. **Phần thêm** (tuỳ chọn): lựa chọn radio · so sánh 2 bản · danh sách hậu quả · ô gõ để xác nhận.
5. **Lỗi** ngay trong hộp (khi hành động thất bại).
6. **Chân hộp**: “Không hỏi lại” (tuỳ chọn) · Huỷ · nút phụ · nút chính.

### 1.2 Bảy tình huống trong app

| Tình huống | Tone | Nút chính | Đặc biệt |
|---|---|---|---|
| Tạm dừng Người viết | thường | `Tạm dừng` (sáng) | Chọn *Sau bước này* (khuyên dùng) / *Ngay bây giờ*. Có “Không hỏi lại”. Xong có *Hoàn tác* |
| Ghi đè tệp xuất | cảnh báo | `Thay bản cũ` | So sánh bản đang có / bản mới; nút phụ `Giữ cả hai` |
| Đăng xuất một thiết bị | đỏ nhạt | `Đăng xuất thiết bị` (`b-dsoft`) | Làm lại được (đăng nhập lại) |
| Xoá tài khoản | đỏ | `Xoá tài khoản` (`b-dan`) | Danh sách hậu quả nền đỏ nhạt |
| Xoá truyện *(khi backend có API)* | đỏ | `Xoá vĩnh viễn` | **Gõ `01-tra-xanh` để xác nhận**: nút chỉ bật khi khớp, gõ sai thì viền đỏ, khớp thì tick mint |
| Đăng xuất mọi thiết bị | đỏ nhạt | **Giữ 1,2 s** | Nút được tô đầy theo thời gian giữ, nhả sớm là huỷ. Bàn phím: giữ Space / Enter |
| Rời khi chưa lưu | thường | `Lưu rồi rời đi` | Nút phụ `Bỏ thay đổi` đỏ nhạt; Huỷ ghi là `Ở lại` |

### 1.3 Quy tắc

- **Nhãn:**
  - Tiêu đề là câu hỏi có tên đối tượng: “Xoá tài khoản ‘linh’?”.
  - Thân bài nói hậu quả, không hỏi “Bạn có chắc không?”.
  - Nút ghi đúng động từ, không dùng “OK” / “Có”.
- **Thứ tự nút:** Huỷ bên trái, hành động bên phải.
- **Focus khi mở:**
  - Hành động nguy hiểm: focus vào **Huỷ**, và Enter **không** kích hoạt nút đỏ.
  - Hành động thường: Enter = nút chính.
  - Esc = huỷ.
- **Mức xác nhận theo độ nguy hiểm:**
  - dễ hoàn tác → không hỏi, làm luôn rồi hiện toast *Hoàn tác*;
  - làm lại được → hộp thường, nút đỏ nhạt;
  - mất dữ liệu → nút đỏ đặc;
  - mất nhiều dữ liệu → gõ để xác nhận;
  - ảnh hưởng mọi thiết bị → giữ để xác nhận.
- **Đang xử lý:** dùng `useAction` chung của §16: khoá hành động ngay, spinner + “Đang xoá…” xuất hiện sau 150 ms và đã hiện thì giữ ít nhất 400 ms; Huỷ vẫn bấm được. Hộp chỉ đóng sau khi hết thời gian giữ spinner.
- **Lỗi:** hộp không đóng. Hiện khối lỗi đỏ nhạt trong hộp, nút chính đổi thành `Thử lại`.
- **Xong:** hộp đóng (180 ms) và hiện toast ngắn có *Hoàn tác* nếu có thể.
- **Animation:** scrim mờ dần 200 ms. Hộp từ `translateY(10px) scale(.965)` về bình thường trong 300 ms, nảy nhẹ. Đóng 180 ms. Gõ sai rồi bấm thì hộp rung 380 ms.

## 2. Menu

### 2.1 Menu tài khoản (`UserMenu`)

Bấm ảnh đại diện (viền tím 2 px khi đang mở). Popover rộng 320, neo phải, mở từ góc trên phải. Nội dung từ trên xuống:

- **Đầu menu:** ảnh, tên, huy hiệu vai trò, `@username`, thanh phiên “còn 30 ngày”.
- **Tài khoản:** QR điện thoại · Thiết bị (3) · Đổi mật khẩu · Quản lý tài khoản (4).
- **Ứng dụng:** Cài đặt `⌘ ,` · Hướng dẫn lệnh `?`.
- **Chuyển tài khoản:** danh sách tài khoản + Đăng nhập tài khoản khác.
- **Đăng xuất:** đỏ nhạt, **không hỏi lại**.

Bàn phím: `↓` mở / đi xuống, `↑`, `Enter`, `Esc`. Rê chuột cũng đổi mục đang chọn. Chi tiết luồng ở `09-auth-onboard.md` §4.

### 2.2 Menu thao tác ⋯ (`ActionMenu`, `ContextMenu`)

- Nút ⋯ chỉ hiện khi rê chuột vào dòng; trên màn cảm ứng thì luôn hiện. Chuột phải vào dòng mở đúng menu này.
- Mục có icon 16 và phím tắt `kbd` bên phải. Mục nguy hiểm nằm cuối, sau đường kẻ, màu `#FFB3BA`, và mở hộp xác nhận.
- Mục bị khoá có thể truyền `tooltip` giải thích lý do; lớp bọc vẫn nhận rê chuột khi nút `disabled` (ví dụ không xoá truyện khi đang chạy).
- **Menu con** (Chuyển sang tập ›): mở khi rê chuột, `fly-in` 160 ms, tự lật sang trái khi sát mép phải. Mục đang chọn có tick.

### 2.3 Thông số chung của lớp nổi

| Thuộc tính | Giá trị |
|---|---|
| Nền | `#1A1A1D` |
| Viền | `rgba(255,255,255,.1)` |
| Bo | 14 |
| Padding | 6 |
| Shadow | `0 24px 60px -16px rgba(0,0,0,.9)` |
| Mục | cao 34 (có mô tả thì 48), bo 9 |
| Mục hover / đang chọn | `rgba(255,255,255,.06)` / `.075` |
| Mở | `translateY(-4px) scale(.97)` → 0 trong 170 ms `cubic-bezier(.2,.9,.3,1)`, gốc theo phía neo |
| Đóng | bấm ra ngoài, `Esc`, hoặc chọn mục |

## 3. Select & Combobox

| Component | Khi dùng | Chi tiết |
|---|---|---|
| `Select` | ≤ 7 lựa chọn | Trigger cao 36, bo 10, nền `#0F0F11`. Mở / focus: viền `rgba(201,166,245,.55)` + vòng 3 px. Lỗi: viền đỏ nhạt + dòng lỗi. Vô hiệu: mờ 45%. Mũi tên xoay 180° khi mở. Lựa chọn có mã phụ (`vi`, `en`) và tick |
| `RichSelect` | lựa chọn cần ảnh / mô tả | Trigger cao 48. Lựa chọn có ảnh, mô tả, nút phụ (▶ nghe thử giọng — bấm **không đóng** danh sách, đang phát thì hiện cột sóng), chân danh sách “Quản lý giọng đọc…” |
| `Combobox` | danh sách dài / có nhóm | Ô có kính lúp; gõ để lọc; phần khớp tô `rgba(201,166,245,.22)`; nhóm theo nhà cung cấp có số lượng; meta bên phải (`400k · cân bằng`); nút xoá; `↑↓ Enter Esc`. Các trạng thái **Đang tải** (spinner + khung chờ), **Lỗi tải** (giải thích + Thử lại, model đã chọn vẫn dùng được), **Không khớp** (gợi ý thêm nhà cung cấp) |
| `MultiCombobox` | chọn nhiều + tạo mới | Thẻ nằm trong ô; `⌫` khi ô trống xoá thẻ cuối; lựa chọn có hộp kiểm; giới hạn 5 (đếm `3/5`, đủ thì khoá lựa chọn khác); “Thêm ‘…’ Enter” để tạo |

ARIA: `role=listbox/option`, `aria-selected`, `aria-expanded`, `aria-autocomplete="list"`, `aria-multiselectable`. Gợi ý thư viện: Radix UI (Select, DropdownMenu, Popover, Tooltip, AlertDialog) + `cmdk` cho combobox, viết lại toàn bộ style theo artboard.

## 4. Ô nhập & nút

- **`TextField`**: các trạng thái mặc định · đang chọn · có nút xoá · lỗi (dòng lỗi có icon) · tiền tố/hậu tố (`exports/` … `.epub`) · không sửa được (kèm chỗ để sửa). Kiểm tra ngay khi gõ (ví dụ tên thư mục chỉ gồm `a-z0-9-`).
- **`TextArea`**: có đếm `0/500` (gần đầy thì cam). `⌘ Enter` gửi, `Shift Enter` xuống dòng.
- **`NumberStepper`**: `− [1] +`, rộng 148; nút mờ khi chạm giới hạn; ghi rõ khoảng hợp lệ.
- **`Slider`**: track 4 px tô tím tới giá trị, nút 16 có quầng; bong bóng giá trị bám theo nút; nhãn mốc (0.8× … 1.5×).
- **`Checkbox`**: các trạng thái bật · tắt · **một phần** (`aria-checked="mixed"`) · vô hiệu.
- **`RadioCard`**: thẻ có nhãn + mô tả, viền tím khi chọn.
- **`Switch`**: 36×22, nút trượt nảy; trạng thái **đang bật** có spinner cạnh (ví dụ bật bộ đọc trên máy mất ~1 s); vô hiệu.
- **`Segmented`**: nhóm chọn có icon.
- **`Button`**: `primary` (1 nút mỗi màn) · `ghost` · `dsoft` (nguy hiểm nhưng làm lại được) · `danger` (chỉ cho xoá vĩnh viễn) · có `kbd` · đang chạy · vô hiệu · `sm`. Nút chỉ có icon luôn có tooltip + `aria-label`.

## 5. Gợi ý & trạng thái

- **`Tooltip`**: hiện sau 400 ms, tắt ngay; nền `#2A2A2E`, có mũi tên, kèm phím tắt; trên hoặc dưới tuỳ chỗ trống. `Button` nhận `tooltip`/`title` ở mọi biến thể, neo theo chính nút qua `asChild` để không thêm lớp bọc làm đổi bố cục flex hoặc nút đặt tuyệt đối.
- **`InfoPopover`**: mở bằng bấm (không phải rê chuột), có thể chứa link, ví dụ giải thích “Cần rảnh”.
- **`Badge`**: có chấm (Đang chạy · Xong · Cần bạn · Lỗi · Nháp · Quản trị). **`Chip`** dạng *khoá giá trị*. **`Kbd`**.
- **`Banner`** trong trang: thông tin / thành công / cảnh báo / lỗi (`role="alert"`), luôn có một hành động.
- **`Skeleton`** · **`Spinner`** 3 cỡ · chữ chạy sáng · **`EmptyState`** (icon, tiêu đề, một câu, tối đa 2 nút).

## 6. Điểm vào triển khai

Gallery phát triển: `#/dev/kit` (chỉ ở Vite dev), có 5 tab và 7 tình huống xác nhận. Query `section=confirm|menus|select|inputs|overlays`, `confirm=pause|overwrite|device|user|story|allDevices|leave`, `open=voice|lang|model|genres|none` chọn trạng thái ban đầu.

Các primitive nằm trong `web/src/ui`: `ConfirmDialog` và `confirm()` trả `Promise<boolean>` (thành công = `true`, huỷ = `false`), preset trong `confirmPresets.ts`, menu trong `Menus.tsx`, lớp nổi trong `Overlays.tsx`, lựa chọn trong `Selects.tsx`, ô nhập trong `Inputs.tsx`. `Field.tsx` giữ `Select` native tương thích các màn hiện có; `KitSelect` là bản listbox tùy biến. `AccountMenu` cũng xuất tên `UserMenu`. Checkbox “Không hỏi lại” chỉ ghi sau hành động thành công, theo tài khoản và loại hành động.

Biểu tượng `more`, `trash`, `profile-delete`, `search-normal-1` dùng SVG từ artboard trong `web/src/assets/icons`; bản sao dev riêng cần đồng bộ cả asset vì `Icon` lập danh sách bằng `import.meta.glob`. Combobox dùng tên canonical `search-normal-1`, `arrow-down`, `refresh-2` cho tìm kiếm, mở danh sách và thử tải lại. URL mask được đặt trong dấu nháy để hỗ trợ SVG nhỏ do Vite nhúng thành data URI có dấu nháy đơn.

`Dialog` thông thường dùng `role="dialog"` (`alertDialog` đổi thành `alertdialog`), `aria-modal="true"` và `aria-labelledby` trỏ tới tiêu đề, kể cả tiêu đề là ReactNode. `ConfirmDialog` luôn dùng `alertdialog`. Chuyển động giống hộp xác nhận (Kit.dc.html): scrim `rgba(5,5,7,.62)` mờ dần 200 ms ease-out, hộp từ `translateY(10px) scale(.965)` về bình thường trong 300 ms `cubic-bezier(.2,.9,.25,1.08)`; đóng bằng mọi đường (Esc, nút đóng, bấm scrim, caller bỏ `?dlg=`) thì hộp mờ đi 180 ms `cubic-bezier(.5,0,.75,0)` về `translateY(6px) scale(.97)` và scrim mờ đi 200 ms ease-in. Bản đang đóng là lớp trơ (`inert`, `aria-hidden`, không còn `role`/`aria-modal`/`id`), nền được gỡ `inert` ngay. Bảng kết quả lệnh có `.cr-dlg-in` riêng (CmdResults.dc.html, 380 ms) nên hộp ngoài không chạy thêm chuyển động mở. `prefers-reduced-motion: reduce` tắt cả mở lẫn đóng.

Caller truyền hành động thật qua `onConfirm`, việc nghe thử qua `RichSelect.onPreview`, và thao tác thứ hai qua `secondary`; lỗi Promise giữ hộp mở để thử lại. Gallery dùng kết quả mô phỏng để tái hiện cả thành công và lỗi, không gọi hành động phá huỷ thật.

## QrCode — mã QR đăng nhập điện thoại (bổ sung 25/10)

Dùng ở: Thiết lập lần đầu › Điện thoại & người cùng viết (`Onboard.dc.html` step `mobile`) và hộp thoại “Đăng nhập điện thoại bằng QR” (`Login.dc.html` / `LoginQR.dc.html` state `qr`).

**Lỗi đang thấy trên bản dev:** mã nằm trong ô trắng 172 px nhưng chỉ vẽ khoảng 84 px. Thư viện tự thêm lề, cộng thêm padding CSS, nên điện thoại phải dí sát mới quét được.

| Quy tắc | Giá trị |
|---|---|
| Kích thước hiển thị | **≥ 232 px** ở thẻ Thiết lập lần đầu; **264–300 px** trong hộp thoại QR; **không bao giờ < 200 px** (kể cả máy tính bảng) |
| Lề trắng (quiet zone) | đúng **4 ô (module)**, nằm **trong** SVG. Không thêm `padding` CSS lên ô trắng |
| Cỡ ô | số nguyên px: `size = (số ô + 8) × k`, `k ≥ 7`. Ví dụ mã 25 ô → 33 × 7 = 231 ≈ 232; 29 ô → 37 × 8 = 296 |
| Nội dung mã | URL ngắn chứa mã ghép (`/p/<code>`) để mã ở phiên bản ≤ 4 (≤ 33 ô); mức sửa lỗi **M** |
| Vẽ | SVG `shape-rendering: crispEdges`, đen `#000` trên trắng `#FFF`, **không** opacity/filter (trừ lớp “Mã đã hết hạn”), **không** đảo màu ở giao diện tối |
| Không che mã | Mọi nút (phóng to, làm mới) đặt **ngoài** mã. Bo góc ô trắng theo artboard: **16** px thẻ Thiết lập, **18** px hộp thoại, **24** px lớp phóng to |
| Phóng to | Nút cạnh đồng hồ đếm ngược → lớp phủ toàn màn, mã **464 px**, câu “Tăng độ sáng màn hình nếu điện thoại khó quét · bấm bất kỳ đâu để đóng”. Đóng bằng Esc hoặc bấm ra ngoài |
| Đếm ngược | Viên xanh `04:37`; hết giờ → lớp trắng 70 % + “Mã đã hết hạn” + **Tạo mã mới** |
| Chọn mạng | Hai tab dưới mã: **Ở nhà** (cùng Wi-Fi) · **Ra ngoài** (qua Tailscale), đổi `links[]` |
| Hướng dẫn | 2 bước đánh số (mở Camera → chạm đường dẫn, bấm Đăng nhập · phiên 30 ngày) |
| Dự phòng | “Đang chờ quét…” + **Chép đường dẫn** (để mở bằng tay trên điện thoại) |

**Thẻ “Mời người cùng viết”** (cạnh thẻ QR, chỉ quản trị):
- Mỗi ô có nhãn riêng: Tên đăng nhập (chữ thường, có gợi ý quy tắc, báo đỏ khi sai) · Vai trò (Người viết “viết & góp ý” / Người đọc “chỉ đọc, xuất”) · Mật khẩu tạm (chữ đậm, nút **Chép** + **Tạo mật khẩu khác**) · Được viết (chọn theo **tên truyện/dự án**, không slug).
- Không dùng font mono, không có nhãn kiểu `mat-khau-tam:`, không hiện `demo-audio`.
- Bấm **Tạo tài khoản** → khung xanh “Đã tạo tài khoản cho linh” + **Chép lời mời (địa chỉ + tên + mật khẩu tạm)**.
- Bố cục 2 cột bằng nhau. Người không phải quản trị chỉ thấy thẻ QR (340 px).

**Triển khai:** `web/src/ui/QrCode.tsx` tự vẽ SVG (qrcode-generator, ECC M, `shape-rendering: crispEdges`): `qrMetrics(modules, range)` chọn k nguyên lớn nhất trong `QR_RANGE` (card 232–300 / dialog 264–300, sàn 7 px/ô và phần mã ≥ 200 px); mã ghép `/p/<32 hex>` ra phiên bản 4 ⇒ 33 ô → k=7, ô trắng 287 px (bo 16 thẻ / 18 hộp thoại). `QrZoom` mở portal `role="dialog"`: hộp trắng 464 px bo 24, mã k=11 (451 px) canh giữa, Esc/click đóng. Nút `QrZoomButton` (icon `scan`, 28 px) đặt cạnh `QrCountdown` trong thẻ onboard và cạnh mm:ss trong `screens/account/PairingDialog.tsx`; thẻ onboard là `screens/onboard/MobileStep.tsx` (`PhoneCard` + `InviteCard`, lớp không-mạng `QrNoLan.tsx`).

## Chuyển động của điều khiển — công tắc, radio, hộp kiểm, nhóm chọn (bổ sung 25/10)

CSS mẫu nằm ở đầu `<style>` của mọi artboard (khối `/* motion:controls v1 */`). Bản gốc: `src/_motion.css` trong canvas.
- Bám theo thuộc tính ARIA: `[role=switch|radio|checkbox][aria-checked]`. Component nào đúng ARIA là tự có chuyển động.
- Không cần JS riêng. Không đổi kích thước bố cục.

| Điều khiển | Khi đổi trạng thái | Khi nhấn giữ | Thời gian · easing |
|---|---|---|---|
| Công tắc | Nút tròn trượt bằng `transform`/`left` và hơi vượt đích rồi về. Nền rãnh đổi màu mượt. | Nút tròn giãn ngang ×1.22 về phía sẽ đi: tắt → giãn sang phải, bật → giãn sang trái. | trượt 300ms `cubic-bezier(.34,1.45,.5,1)` · màu 200ms ease |
| Radio | Viền và nền đổi màu. Chấm trong nở 0 → 1.1 → 1. Dấu ✓ (danh sách chọn) nở và hiện dần. | Cả lựa chọn thu ×0.97 (thẻ lớn ×0.985). | chấm 320ms `cubic-bezier(.34,1.56,.64,1)` · màu 200ms |
| Hộp kiểm | Ô đổ màu. Ô nảy 0.82 → 1.08 → 1. Dấu ✓ vẽ nét (`stroke-dashoffset`) sau 60ms. Bỏ chọn: ✓ mờ đi 180ms. | ×0.97 | nảy 340ms · vẽ 260ms ease-out |
| Hộp kiểm gốc `<input type=checkbox>` | Thay bằng ô tự vẽ (`appearance: none`) để chuyển động giống các hộp kiểm khác. ✓ bằng `clip-path`, phóng từ 0. | ×0.97 | 220ms `cubic-bezier(.34,1.56,.64,1)` |
| Nhóm chọn (segmented) | Một **khối nền trượt** chạy sang lựa chọn mới: `transform: translateX(i × (100% + gap))`, rộng `(100% − padding − gap×(n−1)) / n`. Nút bên trên nền trong suốt. | ×0.97 | 300ms `cubic-bezier(.34,1.25,.5,1)` |

**Quy tắc:**
- Chỉ animate `transform`, `opacity` và màu. Không animate `width`, `height` hay `margin`, vì những thuộc tính này xô bố cục (xem `07-copy.md` › Kiểu chữ).
- Chuyển động chạy khi trạng thái đổi, kể cả đổi từ phía máy chủ (SSE). Không chạy lại khi component re-render vì lý do khác: phải giữ nguyên DOM node, đặt `key` ổn định cho danh sách.
- Mở trang có sẵn mục đã chọn thì có thể nảy một lần, chấp nhận được. Không được nảy lặp mỗi giây khi có đồng hồ chạy.
- Công tắc đang chờ máy chủ (vd “Bộ đọc trên máy”): nút tròn thay bằng spinner nhỏ, rãnh giữ màu cũ đến khi có kết quả, rồi mới trượt.
- `prefers-reduced-motion: reduce`: tắt trượt, nảy và giãn. Chỉ giữ đổi màu ≤ 200ms.
- Bấm bàn phím (Space/Enter, phím mũi tên trong radiogroup) phải ra cùng chuyển động như bấm chuột.

**Triển khai:** `web/src/ui/controls.tsx` và `Tabs.tsx` dùng chung khối `motion:controls v1` trong `kit.css`, giữ đúng thời gian/easing của artboard; `Switch` giữ nút DOM khi chờ, thay bằng spinner và giữ màu cũ đến khi có kết quả. `Checkbox` giữ SVG để vẽ ✓ rồi mờ đi khi bỏ chọn; `Radio`/`RadioCard` và `RadioDot` nở theo trạng thái; `Segmented` đo vị trí/kích thước nút để khối nền trượt đúng cả lưới `columns` lẫn nút rộng theo nội dung (nhóm đều tương đương `i × (100% + gap)`). Phím gốc của button và mũi tên/Home/End trong radiogroup dùng cùng thay đổi trạng thái, không gắn `key` theo lần render; Giảm chuyển động chỉ giữ đổi màu 200ms. Biến thể điện thoại: công tắc 51×31; Reader Kiểu chữ/Giãn dòng 44px/bo11 (Cỡ chữ giữ thanh trượt); MMore cấp áp dụng 40px/bo10, mức suy nghĩ 42px/bo10; định dạng âm thanh 54px/bo11. Theo artboard, MLog giữ các chip tablist cuộn riêng, không thêm khối trượt.

Kiểm chứng bàn phím: Vitest mô phỏng chuỗi kích hoạt gốc (Happy DOM không tự tổng hợp click từ Space); trên Chrome thật, Space đổi công tắc tắt→bật và chạy transform 300ms, Enter bật→tắt, ArrowRight chuyển focus/chọn radio kế tiếp với `mo-dot` 320ms. Đo `getAnimations()` sau nhịp render khác: giữ nguyên DOM, không tạo chuyển động điều khiển mới; giả lập Giảm chuyển động chỉ còn đổi màu 200ms.

### ThinkingOrb — quả cầu chấm “đang nghĩ” cuối luồng chữ (bổ sung 25/10, bản chốt v3)

Một **quả cầu chấm** gọn gàng: chỉ có các chấm tròn đều nhau trên mặt cầu. Chuyển động như thinking orb.

Không có:
- quầng hay vòng sáng bọc quanh chấm;
- lõi phát sáng;
- vệt đuôi;
- chấm to nhỏ lộn xộn.

Các bản trước đã loại vì những lý do sau:
- Cầu chất lỏng WebGL: quá nặng khối.
- Hạt có quầng sáng: trông như virus.
- Hạt có đuôi: trông như nòng nọc.

**Chấm**
- 150 chấm rải đều theo Fibonacci sphere, cùng một cỡ.
- Ở 22px, mỗi chấm khoảng 0,4–0,6px CSS. Khi phóng to, chấm to chậm theo `(size/22)^0,45`, nên luôn mảnh.
- Chiều sâu chỉ bằng phối cảnh (F = 3,8) và độ mờ:
  - mặt sau mờ (opacity 0,1, xanh lilac `#9696FF`);
  - mặt trước rõ (≈0,85, `#F0DEFF`).

**Chuyển động “đang nghĩ”**
- Cả cầu quay đều khoảng 18 giây một vòng, trục nghiêng −20°.
- **Sóng lan trên bề mặt:**
  - Một vòng sóng tỏa từ một điểm trên cầu ra hết bề mặt, khoảng 1,8 giây một nhịp, mờ dần khi lan xa.
  - Chấm nằm trên vòng sóng sáng lên và nhô ra ngoài 7%.
  - Điểm phát sóng tự đi vòng quanh cầu, nên mỗi nhịp sóng bắt đầu ở một chỗ khác.
- Không có nhấp nháy ngẫu nhiên, không đổi màu loạn.

**Dùng**
- Inline 22px ngay sau chữ cuối của luồng (`vertical-align: -6px`).
- Dùng được ở 44px hoặc lớn hơn, ví dụ trạng thái chờ hoặc màn trống.

**Hiệu năng**
- Canvas 2D, khoảng 30 fps, DPR tối đa 3.
- Bỏ qua canvas ngoài màn hình, tự dừng khi tab ẩn.

**Truy cập**
- `prefers-reduced-motion` → một khung đứng yên.
- `role="img"`, aria-label “Người viết đang viết”.
- Chỉ hiện khi trợ lý đang chạy.

**Mã mẫu:** `startOrbit()` + `canvas.orbit-cv` trong `artboards/Main.dc.html`. Khi làm React, gói thành `<ThinkingOrb size state="thinking|idle" />`. Ở trạng thái `idle`: tắt sóng, chỉ quay chậm.

**Triển khai:** `web/src/ui/ThinkingOrb.tsx` (xuất từ `ui/index.ts`), chuyển từ `startOrbit()`. Màu chấm lấy từ token `--orb-dot-back` / `--orb-dot-front`. Vẽ khung đầu ngay khi mount; mỗi khung đọc lại `prefers-reduced-motion` nên đổi cài đặt là có hiệu lực ngay. Dùng ở Luồng viết (máy tính, điện thoại) và luồng đồng sáng tác, thay `✦✧`/ký tự nhảy (`.m-caret`, `.mc-caret` đã xoá). Đo trên mock: 22×22, `vertical-align:-6px`, trong 784 điểm ảnh chỉ 8 điểm đặc (không quầng); giảm chuyển động thì hai khung cách 1 giây giống hệt nhau.

### Phím Esc (chốt 25/10)

- Bấm **một lần** là đóng lớp trên cùng: menu, popover, bottom sheet, hộp thoại. Không có kiểu "bấm lần nữa để đóng".
- Bản nháp đang gõ trong lớp đó được giữ lại, không hỏi.
- Hộp "gõ để xác nhận" và "giữ để xác nhận": Esc = Huỷ.
- Chỉ khi không còn lớp nào mở, Esc ở Bàn viết mới là **Tạm dừng** (đúng như dòng phím tắt dưới ô chat).

### Thẻ thẩm duyệt có số mục luật — `ReviewIssue` (bổ sung 25/10 · D4, D9) <!-- brief-2510 -->

Một component dùng chung cho Trình đọc, Bàn viết (phase gate), Nhật ký, nhật ký điện thoại.

| Phần | Quy cách |
|---|---|
| Nhãn mức | Bắt sửa: nền `rgba(255,154,162,.12)`, chữ `#FFB3BA`, viền thẻ `rgba(255,154,162,.22)` · Cảnh báo: nền `rgba(246,203,140,.12)`, chữ `#F3D9AE`, viền thẻ `rgba(255,255,255,.07)`. Cao 20 px, bo 6, chữ 11,5 px 500 |
| Chip mục | `Master §7`, `Bộ A §4.3`, `Xưng hô`, `Quy tắc của bạn` — nền `rgba(255,255,255,.05)`, chữ `#C9C9D1` |
| Tên mục | 12 px `#7E7E8A` (“Đầu chương và cuối chương”, với xưng hô là chiều “Tri Hạ → Nhược Vy” hoặc “Chưa rõ ai nói”) |
| Câu lỗi | 13 px `#EDEDEF`, line-height 1,5 |
| Đoạn trích | Literata 13 px, gạch trái 2 px màu theo mức (`.4`/`.35`) |
| Gợi ý | “Gợi ý:” `#C9C9D1` + nội dung `#A0A0AB` 12,5 px |
| Hành động (Cảnh báo) | trạng thái trái (“Tuỳ bạn” / “Giữ nguyên · đã ghi lý do”), “Giữ nguyên”, “Yêu cầu sửa” (điền sẵn ô góp ý). Bắt sửa chỉ hiện “Đã vào hàng chờ sửa” |
| Bản thu gọn (điện thoại, Nhật ký) | bỏ đoạn trích/gợi ý hoặc bỏ hành động; giữ dòng nhãn + câu lỗi |

### Đồ thị Não truyện — `StoryGraph` <!-- brief-2510 -->

Quy cách đầy đủ ở `22-brief-2510.md` › D10 “Đồ thị v3”. Mã mẫu: `build()` + `paint()` trong `artboards/Brain.dc.html`. Điểm bắt buộc: thẻ nút đo chữ bằng font thật sau khi font tải xong và cắt “…” theo bề rộng tối đa; nhãn cạnh đặt né thẻ và nhãn khác; thẻ che cạnh đi sau; `prefers-reduced-motion` tắt hạt chạy, sóng lan, chuyển viewBox.

## Vòng tiến độ và sóng âm (sửa 25/10) <!-- ring-v2 -->

Áp dụng ở mọi nơi có vòng tiến độ và sóng âm: `AudioExport`, `Transcript`, `Jobs`, `MAudio`, `MCmd`, `MWork`, `MTablet`.

- **Vòng tiến độ**
  - Hai đầu bo tròn. Vẽ bằng SVG `circle` với `stroke-linecap: round`, `pathLength="100"`, `stroke-dasharray: <phần trăm> 100`, xoay −90° để bắt đầu từ 12 giờ.
  - Không dùng `conic-gradient` + `mask`: cách đó cho hai đầu vuông.
  - Vòng nền cùng độ dày. Màu vòng nền theo màn: thường `rgba(255,255,255,.08)`.
  - Độ dày theo cỡ:

    | Cỡ vòng | Độ dày |
    |---|---|
    | 22 px | 2,5 px |
    | 28–42 px | 2,5–3 px |
    | 72–84 px | 5 px |
    | 96 px | 7,5 px |
    | 148 px | 7 px |

  - Vòng lớn (Tạo giọng, Nhập truyện) dùng gradient tím. Vòng nhỏ dùng một màu theo loại việc: xanh = xuất, tím = giọng, cam = chờ / không xác định, đỏ = lỗi.
  - Việc không xác định được tiến độ: cung 25% quay vòng; tắt khi bật “Giảm chuyển động”.
  - 100% là vòng kín. 0% thì ẩn cung, không để lại chấm tròn.
  - Đổi giá trị thì cung chạy mượt 0,5 s.
- **Sóng âm:** mỗi thanh là viên thuốc (`border-radius: 999px`), cao tối thiểu 4 px. Có hai bộ màu (chốt 26/10):
  - nghe: đã phát `#C9A6F5`, chưa phát `#2A2A2E` (điện thoại `rgba(255,255,255,.16)`);
  - tạo giọng: đã tạo `#C9C9D1`, 3 cột mới nhất `#C9A6F5`, chưa tạo `#2A2A2E`.
- **Thời gian còn lại:** ghi “còn khoảng 11 phút” ở dòng chữ, “còn 4 phút” ở giữa vòng lớn. Không dùng dấu “~”, vì ở cỡ chữ nhỏ trông giống dấu trừ.

