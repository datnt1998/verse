# 04 · Components

Đề xuất thư mục: `web/src/ui/` (primitive), `web/src/features/<màn>/` (component theo màn), `web/src/api/` giữ nguyên và mở rộng. Giá trị pixel chính xác: tra `artboards/*.dc.html`.

## Primitive

> Bản đầy đủ và tương tác: `17-kit.md` + `artboards/Kit.dc.html` — `ConfirmDialog`, `UserMenu`, `ActionMenu`/`ContextMenu` (menu con), `Select`, `RichSelect`, `Combobox`, `MultiCombobox`, `TextField`, `NumberStepper`, `Slider`, `RadioCard`, `Tooltip`, `InfoPopover`, `Banner`, `EmptyState`. Bản điện thoại: `18-mobile.md`. Các dòng dưới đây là tóm tắt.

| Component | Props chính | Ghi chú / trạng thái |
|---|---|---|
| `Icon` | `name`, `variant: 'linear'\|'bulk'`, `size=18` | Render SVG inline từ `icons/` (khuyên dùng `vite-plugin-svgr` hoặc import `?raw`). Màu theo `currentColor` |
| `AgentOrb` | `agent: 'architect'\|'writer'\|'editor'\|'arbiter'`, `size`, `live`, `dim` | `live` → `ai-breathe`; `dim` → opacity .45–.6. Luôn `aria-hidden` trừ khi đứng một mình (khi đó `role="img"` + nhãn) |
| `OrbCluster` | `size=14` | 4 orb chồng `-4px` — logo |
| `Button` | `variant: 'primary'\|'ghost'\|'icon'`, `size: 'sm'\|'md'`, `kbd?` | primary: nền `fg` chữ `bg`, hover `#fff`. ghost: viền `line-4`, hover `rgba(255,255,255,.07)`. icon: 32×32, cần `aria-label`. `kbd` hiển thị phím tắt bên phải |
| `Kbd` | children | Mono 11px, viền `line-4` |
| `Chip` | `icon?`, `label`, `value?`, `tone: 'neutral'\|'accent'\|'ok'\|'warn'\|'err'` | Cao 26px, bo 7px |
| `StatusChip` | `snapshot` | Spinner khi chạy; map `runtimeState`/`statusLabel` (xem `06`) |
| `Badge` | `tone`, children | Cao 20–22px, chữ 11–12px |
| `Card` | `padding`, `as` | Card liquid dark |
| `Tabs` | `items[{id,label,icon,dot?}]`, `value`, `onChange` | `role="tablist"`; tab chọn nền `surface-6` |
| `Segmented` | `options`, `value`, `onChange` | `role="radiogroup"`, item `role="radio"` |
| `Switch` | `checked`, `onChange`, `label` | Track 40×24, vùng chạm 48×44; `role="switch"` |
| `Checkbox` | như trên | Ô 18px, tick bằng `Icon tick-circle bulk` |
| `Field`, `Select`, `TextArea` | chuẩn | Nền `surface-0`, viền `line-4`, focus `line-focus`; `aria-invalid` → viền `err` |
| `Spinner` | `size=14`, `color` | `ai-spin` |
| `Shimmer` | children | Chữ chạy sáng (`ai-shimmer`) |
| `ProgressBar` | `value`, `tone: 'neutral'\|'writer'\|'ok'` | Cao 2–3px |
| `Dialog` | `title`, `icon`, `onClose`, `footer` | Nền `surface-3`, bo 18, shadow `--dialog-shadow`, backdrop `rgba(5,5,7,.62)`; focus trap; `Esc` đóng |
| `Toast` | `tone`, `text`, `action?` | Pill đáy giữa cho phản hồi tại chỗ (có *Hoàn tác*). Thông báo việc chạy nền: xem `14-jobs-dock.md` |
| `Scroll` | `stickToBottom?` | `overflow-y:auto`, thanh mảnh; `stickToBottom` dùng `flex-direction: column-reverse` |

## Shell

| Component | Mô tả |
|---|---|
| `AppShell` | `height:100vh; overflow:hidden; display:flex; flex-direction:column`. Dưới 1100px: `height:auto` |
| `AppHeader` | Trái: (nút ẩn panel trái, chỉ Bàn viết) · `BrandMark` · `ContextCrumb`. Giữa: `MainNav`. Phải: Cài đặt · Avatar · (nút ẩn panel phải, chỉ Bàn viết). **Dựng một lần ở layout gốc**, không dựng lại theo trang. Lưới `minmax(0,1fr) auto minmax(0,1fr)`, cao 54 px, lề 16 px; `ContextCrumb` cắt “…”. `MainNav`: mọi mục 500, mục chọn chỉ đổi màu chữ + icon `#C9A6F5`; một khối nền trượt (transform + width 280 ms). Xem `Nav.dc.html` và Gói Q. |
| `ContextCrumb` | `[folder] dự án ▾ / NN · tên truyện / Tập N` — mỗi phần là link; phần dự án mở `ProjectSwitcher` |
| `ProjectSwitcher` | Popover 320px: label thư viện (mono), danh sách dự án (đang chọn có tick), “Dự án mới”, “Đổi thư mục thư viện” |
| `MainNav` | Segmented 4 link; mục hiện tại dùng icon Bulk màu accent |
| `ConnectionBanner` | Pill trên cùng: spinner vàng + “Mất kết nối… sau Ns” + Thử ngay |

## Workspace

| Component | Dữ liệu | Hành vi |
|---|---|---|
| `OutlineRail` | `snapshot.outline`, `completedCount`, `inProgressChapter`, `pendingRewrites` | Mỗi chương: số · tên · trạng thái (tick ok / orb writer live / icon refresh = chờ viết lại). Click → Trình đọc |
| `RunHeader` | `bookTitle`, chips | Tiêu đề 1 dòng ellipsis; hàng chip: `StatusChip`, Giai đoạn, Quy trình, Chế độ, Đang viết/Đang viết lại, `recoveryLabel` |
| `GateCard` | `advanceMode`, `advancePermitChapter`, `hasAdvanceHold` | Viền vàng; nút `/next` |
| `ActivityFeed` | `HostEvent[]` | Xem `06` › Event rendering. Header có link “Toàn bộ” |
| `ActivityRow` | `HostEvent` | Giờ (mono 11) · icon/spinner/`├` · `AGENT` màu agent + `（nhiệm vụ）` · thời lượng nghiêng |
| `LiveStream` | `streamText`, `isRunning` | Thanh `grad-writer-progress` trên cùng; header: orb WRITER · “Luồng viết trực tiếp” · ✦✧ · tool đang chạy (`Shimmer` mono) · đồng hồ; thân: danh sách `StreamRound` bám đáy; rỗng → “Không có agent nào đang viết” |
| `StreamRound` | `{tool, labelVi, fields, live}` | Nút header (✻ vàng · nhãn xanh ngọc · tên tool mono · gạch · spinner/tick · chevron), mở/đóng; thân theo loại (xem `06` › Stream) |
| `ThinkingLine` | elapsed | Icon ∞ (vệt `grad-thinking` chạy) + “Đang suy nghĩ” shimmer + đồng hồ |
| `Composer` | `placeholder` theo trạng thái, `running` | Card bo 18; icon tin nhắn · input · đếm `n/5000` · nút gửi tròn. Khi `running`: lớp `RunRing` (viền conic chạy + bản blur). `/` mở palette. `Enter` gửi, `Ctrl+J`/`Alt+Enter` xuống dòng (textarea tự cao tối đa 6 dòng) |
| `RunRing` | — | 2 lớp absolute: ring (mask xor 1.5px) + glow blur 16px; `ai-run` 4.5s. **Chỉ dùng cho `Composer`**, mỗi màn tối đa một vật phát sáng. |
| `ShortcutHints` | — | Dòng dưới ô chat: `/ lệnh · Enter gửi · Ctrl J xuống dòng · Esc tạm dừng · [ ] ẩn/hiện panel` + số liệu nhanh `↑in ↓out ⚡cache% $cost` |
| `InspectorTabs` | — | Tabs Agent · Chi phí · Truyện; chấm orb writer trên tab Agent khi đang chạy & đang xem tab khác |
| `AgentCard` | `AgentSnapshot`, chi phí vai trò | Orb 26 · tên · `tokens · $` · badge (Đang chạy/Tạm dừng/Chờ lệnh/Gọi khi cần). **Thẻ trơn, không quầng sáng** kể cả khi chạy (chốt 27/10): dấu hiệu chạy là badge tím và orb “thở”. Khi chạy: dòng `summary` (font thường, **không mono** vì có chữ Việt) + thanh ctx (`context.percent`, màu ok <70, warn 70–85, err ≥85) + `context.strategy` |
| `RewriteQueueCard` | `pendingRewrites`, `rewriteReason` | Chip số chương; chương đang viết tô accent |
| `CostCard` | totals, `cachePerAgent`, `cachePerModel` | `$` lớn + badge tiết kiệm; thanh chia theo vai trò (orb gradient); bảng vai trò & model |
| `CacheCard` | cache stats | % tổng, gần N lần, theo vai trò (thanh ok), đọc/ghi, đứt gãy (warn) |

## Dự án

| Component | Mô tả |
|---|---|
| `ProjectTree` | Dự án (Tổng quan) · Ý tưởng · Tài sản · KỊCH BẢN › truyện (orb trạng thái) › Tập (thu/mở) › chương · “+ Tập mới (/reopen)” · “+ Truyện mới”. Chân: đường dẫn + “Cấu trúc đúng chuẩn” |
| `ProjectOverview` | Form `project.json`, lưới thẻ truyện, kho ý tưởng (“Viết thành truyện”), tài sản. Ô chat đáy = thêm ý tưởng |
| `StoryView` | Tabs Đại cương · Nhân vật · Phục bút · Xuất bản · Thông tin. Ô chat đáy = can thiệp |
| `VolumeOutline` | Bảng chương có hàng mở rộng (tóm tắt, sự kiện, kế hoạch, hợp đồng, link Trình đọc, đường dẫn tệp) |
| `ExportMatrix` | Hàng = tập, cột = .txt/.epub/Audio, ô = trạng thái + dung lượng |
| `FolderTree` | Cây thư mục chuẩn có nhãn vai trò; màu: ok (đúng chuẩn), info (engine quản lý, không sửa tay), fg-4 (tùy chọn); nút “Kiểm tra chuẩn” |

## Trình đọc

`ChapterPicker`, `ReaderToolbar` (Bản nộp/Bản nháp, Thay đổi gần nhất, A−/A+), `ReaderBody` (Literata), `DiffInline` (dòng cũ gạch đỏ nền err 7%, dòng mới nền ok 8%), `ReviewPanel` (verdict badge, tiêu chí 0–100, `IssueCard` có nút “Yêu cầu sửa” → điền ô chat), `ExportPanel`, `QuickActions` (điền sẵn lệnh vào ô chat, không tự gửi).

## Tạo truyện

`ModeSwitch` (Bắt đầu nhanh / Đồng sáng tác, phím Tab), `PromptBox` (textarea 4 dòng, chip văn phong/ngôn ngữ/model/thư mục đích, đếm ký tự, nút Bắt đầu viết + Kbd Enter), `ExampleCards` (3 ví dụ thật từ `panels.go`), `FlowPreview`, `AltEntries` (`/start`, `/import`, `/simulate`), `CoCreateChat` + `SuggestionChips` (❶❷❸) + `DraftPanel`.

## Cài đặt

`SettingsNav` (anchor), `SettingsSection`, `SettingRow` (nhãn + `key` mono + mô tả | control), `DirNameRow` (kiểm tra hợp lệ trực tiếp), `SlugPreview`, `PatternField` (xem trước đường dẫn), `InheritChain`, `ReadOnlyValue` (viền nét đứt), `SaveBar` (neo đáy khi có thay đổi).

## Hộp thoại

`CommandPalette` (nhóm lệnh, lọc, nhãn “Cần rảnh”/“Chưa có API”), `NewProjectDialog`, `ReopenDialog`, `ImportDialog` (9 giai đoạn + xác nhận `y` + log ✓ ✗ ↻), `LockedDialog`. Đăng nhập, menu tài khoản, QR, thiết bị, quản lý tài khoản, onboard: `09-auth-onboard.md` §4.

## Không vỡ chữ trong nút & chip

- Nút, chip, tab, mục điều hướng: luôn **một dòng** (`white-space: nowrap; flex: 0 0 auto`). Không cho chữ trong nút xuống dòng ở bất kỳ độ rộng nào.
- Khi thiếu chỗ: chữ mô tả bên cạnh co lại trước (`min-width: 0; overflow: hidden; text-overflow: ellipsis` + `title` đầy đủ), rồi mới rút gọn nhãn nút (vd “Soạn 3 chương còn lại”, “Chỗ cần xem tiếp”).
- Nhãn có số ghép từ dữ liệu: dựng cả chuỗi ở logic (`'Soạn ' + n + ' chương còn lại'`), không ghép nhiều mảnh chữ trong template (dễ thừa khoảng trắng).
- Kiểm tra ở 1440, 1280, 1180 px và 390 px (điện thoại): không phần tử bấm được nào có chữ bị ngắt dòng.

## Avatar nhân vật (chốt 26/10) <!-- avatar-nhan-vat -->

Dùng ở danh sách Nhân vật (Bàn viết, Dự án & truyện, Não truyện, Não dự án, người nói ở Audio).

**Chữ trong avatar** lấy từ `name` (chỉ tên, không gồm vai), viết hoa, giữ dấu của chữ cái đầu (Đ):

| Tên có | Lấy | Ví dụ |
|---|---|---|
| 1 chữ | chữ cái đầu | Lụa → L |
| 2 chữ | chữ đầu của cả hai | Lâm An → LA · Phùng Chấn → PC · Đỗ Mẫn → ĐM |
| 3 chữ trở lên (không có danh xưng) | bỏ họ (chữ đầu tiên), lấy chữ đầu của **2 chữ cuối** | Lâm Tri Hạ → TH · Tống Nhược Vy → NV |
| Bắt đầu bằng danh xưng (Ông, Bà, Cô, Chú, Bác, Dì, Cậu, Mợ, Thầy, Luật sư, Bác sĩ, Giám đốc…) | chữ đầu của danh xưng + chữ đầu của chữ cuối | Luật sư Hàn → LH · Cô lao công Lụa → CL · Bà Tư → BT |

- Tính ở client từ `name`, không lấy từ dữ liệu khác. Nếu nhân vật có `ini` người dùng tự đặt thì ưu tiên `ini`.
- Hai nhân vật trùng chữ viết tắt vẫn giữ nguyên; người đọc phân biệt bằng tên bên cạnh.
- Hình: tròn, chữ 10/600; **màu theo loại vai** (bảng dưới). Không dùng gradient trợ lý.

**Màu theo loại vai (chốt 28/10)** <!-- cav-loai -->

Thay cho avatar xám chung trước đây. Lấy từ `kind` của nhân vật (xem `06-data-contract.md` › *Bổ sung §S*), không đoán từ chữ `role`.

| Loại (`kind`) | Nhãn | Viền | Nền | Chữ | Huy hiệu góc |
|---|---|---|---|---|---|
| `lead` | Chính | `#C9A6F5` | `#2A2336` | `#E2D0FF` | ngôi sao |
| `antagonist` | Phản diện | `#FF9AA2` | `#302225` | `#FFC2C8` | tam giác cảnh báo |
| `ally` | Đồng minh | `#A6EDC1` | `#1F2C27` | `#C9F5DA` | khiên |
| `support` | Phụ | `rgba(255,255,255,0.18)` | `#1E1E22` | `#C9C9D1` | không có |

- **Chưa lộ với người nghe** (chương đang xem nhỏ hơn `revealAt`): viền **nét đứt**, vẫn giữ màu loại. Thẻ rê và ngăn chi tiết ghi “· chưa lộ”.
- **Huy hiệu góc** dưới phải: 13 px (16 px ở cỡ lớn ≥ 40 px), nền là màu viền, biểu tượng màu nền. **Ẩn ở cỡ ≤ 20 px** (chip trong kế hoạch, chip nộp chương) vì quá nhỏ để đọc; lúc đó chỉ còn màu viền.
- **Cỡ:** danh sách 26 px · chip 20 px / 18 px · nút đồ thị trong ngăn kéo · ngăn chi tiết 40 px kèm chip loại.
- **Mờ đi** (nút không liên quan khi chọn một nút ở đồ thị): giữ nền đặc, làm mờ bằng `filter: saturate(0) brightness(.55)`. Không giảm `opacity` (sẽ lộ cạnh phía sau).
- **Thứ tự danh sách:** Chính → Phản diện → Đồng minh → Phụ; trong cùng loại giữ thứ tự xuất hiện.
- **Chú giải** một dòng dưới danh sách Nhân vật và trong ngăn kéo Não truyện: “Chính · Phản diện · Đồng minh · Phụ · Chưa lộ”. Không cần ở chip.
- **Trợ năng:** `aria-label` = “{tên}, {nhãn loại}” (+ “, chưa lộ”). Màu không phải tín hiệu duy nhất: còn huy hiệu, nhãn ở thẻ rê và chú giải.
- **Não truyện / Não truyện điện thoại:** màu nút nhân vật cũng đổi theo loại vai (cùng bảng), bỏ màu nhận diện riêng từng người.
- Áp dụng ở mọi nơi có avatar nhân vật: Bàn viết (danh sách, thẻ rê, chip kế hoạch, chip nộp chương, ngăn kéo), Dự án & truyện, Não truyện, Não dự án, người nói ở Audio.

**Dòng tên:** `name` đậm 13/500, sau đó `role` 12 px `#7E7E8A` trên cùng một dòng, cắt “…” theo chiều rộng.
- `role` là trường riêng, các ý nối bằng “ · ”, vd. “Nữ chính · người kể xưng tôi”.
- Không gộp vai vào tên trong ngoặc, vd. không viết “Lâm An (Nữ chính; …)”. Nếu dữ liệu cũ gộp chung thì tách phần trong ngoặc ra làm `role` và đổi “;” thành “ · ”.

## Nút lên đầu (chốt 28/10) <!-- nut-len-dau -->

Artboard: `ScrollTop.dc.html` (10c, có hai khung cuộn thử), gắn sẵn ở `Reader`, `MReader`, `Outline`, `Transcript` (tweak `scroll` = deep, ở `MReader` là `state` = readDeep).

**Khi nào hiện**
- Chỉ gắn cho vùng cuộn có nội dung cao ít nhất **3 màn hình**; đo lại khi đổi chương, đổi tab, đổi cỡ chữ.
- Hiện khi đã cuộn quá **1 màn hình** (chiều cao vùng cuộn); ẩn khi về dưới **nửa màn hình**. Khoảng đệm này để nút không nhấp nháy.
- **Điện thoại:** chỉ hiện khi thanh trên/dưới đang hiện (cuộn ngược lên, chạm, hoặc tới cuối chương); đang cuộn xuống đọc thì ẩn cùng thanh. **Máy tính:** hiện liên tục khi đã qua ngưỡng.
- Ẩn khi mở bottom sheet, bàn phím, menu, hoặc đang chọn một đoạn văn.

**Khi bấm**
- Cuộn mượt về đầu, tối đa khoảng 400 ms. Nếu đang cách đầu hơn 6 màn hình thì nhảy tới cách đầu 1 màn hình rồi mới cuộn mượt. Bật “giảm chuyển động” thì nhảy thẳng.
- Chuyển focus tới tiêu đề chương (`tabindex="-1"`, `preventScroll`), hoặc tới chính vùng cuộn nếu không có tiêu đề. Nút tự ẩn.
- Phím Home vẫn hoạt động như cũ; nhãn khi rê chuột nhắc phím này.

**Hình**

| | Máy tính | Điện thoại |
|---|---|---|
| Cỡ | tròn 40 px | tròn 44 px |
| Icon | mũi nhọn lên (`arrow-up-2`) 18 px | 20 px |
| Nền | `#1A1A1D` 90 % + làm mờ phía sau 12 px | như máy tính |
| Viền, bóng | viền trong 1 px trắng 10 %, bóng `0 8px 24px` đen 45 % | như máy tính |
| Rê chuột | nền `#26262A` + nhãn bên trái (“Lên đầu chương · đã đọc 62%”, “Lên đầu · Home”) | không có nhãn |
| Focus | viền tím `#C9A6F5` 2 px, cách 3 px | như máy tính |

- Không dùng icon mũi tên của nút Gửi, để khỏi nhầm khi hai nút đứng gần nhau.
- **Vòng tiến độ** (chỉ ở chỗ đọc chương: Trình đọc, Đọc trên điện thoại): viền 2 px tím `#C9A6F5` trên nền trắng 12 %, bằng phần trăm đã cuộn trong chương.
- **Chuyển động:** hiện 180 ms (mờ → rõ, nhích lên 8 px, 0,9 → 1); ẩn 120 ms; nhấn thu về 0,94. Ở điện thoại, nút trượt cùng thanh dưới (0,38 s).

**Vị trí**

| Màn | Vùng cuộn | Vị trí | Vòng tiến độ | Nhãn |
|---|---|---|---|---|
| Trình đọc | nội dung chương | phải 20 px, trên ô nhập 16 px | có | Lên đầu chương |
| Đọc (điện thoại) | nội dung chương | phải 16 px, trên thanh dưới 16 px; trên nút “Duyệt & viết tiếp” nếu nút đang hiện; khi thanh ẩn thì cách đáy 24 px | có | chỉ `aria-label` |
| Dự án & truyện | nội dung tab | thẳng mép phải ô dặn hướng, trên ô 26 px | không | Lên đầu |
| Audio | danh sách đoạn của chương | phải 20 px, dưới 16 px (trên thanh “Chốt”) | không | Lên đầu chương |
| Nhật ký tiến độ | cả trang | phải 24 px, dưới 24 px của khung nội dung | không | Lên đầu |

**Trợ năng:** là `<button>` thật, `aria-label` đọc được, vd. “Lên đầu chương, đã đọc 62%”. Khi ẩn dùng `visibility: hidden` để ra khỏi thứ tự Tab.

**Không dùng ở:** luồng trực tiếp của Bàn viết (luồng bám ở cuối); nhật ký điện thoại (đã có nút “N sự kiện mới”, không hiện hai nút cùng lúc); cột bên hẹp; danh sách ngắn.

## Đồ thị Não truyện dùng chung (chốt 28/10) <!-- do-thi-chung -->

Chỉ có **một** component đồ thị (`StoryGraph`), dùng ở màn Não truyện (đầy đủ) và ngăn kéo ở Bàn viết (`compact`). Artboard `Brain.dc.html` và `Main.dc.html` (`drawer=brain`) chạy cùng một mã vẽ.

| Phần | Đầy đủ (Não truyện) | Gọn (ngăn kéo) |
|---|---|---|
| Thẻ, cạnh, nhãn, màu, quầng chọn | có | giống hệt |
| Cột lọc loại và kiểu quan hệ | có | không |
| Tìm, Đồ thị/Danh sách, “Chỉ quanh mục chọn”, phóng to | có | không |
| Dòng thời gian chương | có | giống hệt |
| Chú giải | ở cột lọc | một dòng loại vai ở góc dưới trái |
| Bảng chi tiết | sửa được khi tạm dừng | chỉ xem; “Mở toàn màn” để sửa |

Không vẽ đồ thị bằng vòng tròn rỗng và đường thẳng, không đặt nhãn cố định giữa mọi cạnh, không dùng hàng ô số chương.

## Ô chọn: chữ dài phải cắt “…” (chốt 28/10) <!-- select-cat-chu -->

- Nút của `KitSelect` / `Combobox`: nhãn nằm trong một `span` có `min-width: 0; overflow: hidden; white-space: nowrap; text-overflow: ellipsis`, mũi tên `flex: none`. Không bao giờ để chữ bị xén giữa ký tự như “viết hay nhấ”. Rê chuột hiện đủ nhãn (`title`).
- **Ô chọn model** hiện hai phần: tên model (chữ mã) + mô tả ngắn (chữ thường, xám). Chỉ phần mô tả bị cắt “…”, tên model luôn đủ.
- Danh sách model: mỗi dòng có tên model, mô tả ở dòng dưới, chip ngữ cảnh bên phải (“272K”, “1M”), dấu tick ở mục đang chọn; cuối danh sách có “Thêm nhà cung cấp ở Kết nối AI”.
- Ô chọn hay popover nằm gần đáy khung thì mở lên trên.
