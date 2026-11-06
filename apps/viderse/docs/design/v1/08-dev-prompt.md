# Viderse — Gói việc cho dev

Làm theo thứ tự. **V0** là nền cho mọi gói khác; từ **V1** tới **V9** làm song song được sau V0, trừ khi gói ghi phụ thuộc. Khi xong một gói, ghi vào [10-gap-closure.md](10-gap-closure.md).

Nguồn sự thật:
- Hành vi: [01-ia-flows.md](01-ia-flows.md)
- Dữ liệu: [07-data-contract.md](07-data-contract.md)
- Giao diện: artboard trong [artboards/](artboards/README.md)
- Lời văn: [06-copy.md](06-copy.md)
- Đường chuyển từ code hiện tại: [09-migration-yt-factory.md](09-migration-yt-factory.md)

- Dựng trong verse (Go + Postgres + React, `@noverse/ui`): [02-stack-verse.md](02-stack-verse.md). Các đường dẫn `src/server/...`, `src/app/...`, drizzle, Electron, `ytf` trong phần “Việc backend” là nơi logic đang chạy ở bản yt-factory — dùng làm bản tham chiếu khi port, không phải nơi viết code mới.
- Hỏi hoặc thấy lệch: [11-hoi-dap.md](11-hoi-dap.md).

Ưu tiên: **P1** là bắt buộc cho bản Viderse đầu tiên, **P2** làm ngay sau đó.

---

## Gói V0 — Nền tảng Viderse (28/10/2026 · P1) <!-- goi-v0 -->

**Vì sao:** mọi màn mới dùng chung thanh trên, màu, chữ, trợ lý và trạng thái. Làm một lần, đúng một chỗ.
- Artboard: `Brand`, `Kit`, `MKit`, và phần đầu trang của mọi board.
- Spec: [05-kit.md](05-kit.md) + [kit.css](kit.css); [tokens.css](tokens.css); [04-components.md](04-components.md) › AppHeader, MainNav, JobsPill, Logo ống kính, AgentOrb, ChannelAvatar, StatusLabel, ActionButton, Chip, Banner; [07-data-contract.md](07-data-contract.md) › §1, §2, §5.

### Việc FE
- Đưa `tokens.css` rồi `kit.css` vào `globals.css` (`.dark`); `.vv-root` ở `<body>`. Đổi font sang Be Vietnam Pro, giữ JetBrains Mono. <!-- r15-kit -->
- **Dùng lại UI Kit của Noverse** (đợt 19): lấy nguyên component kit của Noverse — Button, ConfirmDialog, Menu, Select, Combobox, MultiSelect, Input, Stepper, Slider, Checkbox, Radio, Switch, Segmented, Tooltip, Popover, Banner, Toast, Skeleton, Empty — cùng props và hành vi bàn phím; chỉ thay token màu bằng [tokens.css](tokens.css) (tím → đỏ `--vv-accent*`, đỏ → cam `--vv-danger*`). <!-- r19-kit -->
- **Làm UI Kit trước mọi màn:** mỗi dòng ở bảng [05-kit.md](05-kit.md) thành một component React bọc lớp `.vv-*`, biến thể qua prop, trạng thái qua ARIA; thành phần riêng Viderse (bảng thứ hai) làm thêm. Không viết lại màu, cỡ, bo góc bằng Tailwind hay style inline.
- Cả 23 artboard màn hình đã vẽ theo kit mới (đợt 20). Code đang có còn theo kit cũ (nút đỏ đặc, công tắc xanh lá, nút 34px…) thì đổi theo bảng “Kit cũ → kit mới” trong 05-kit.md. <!-- r20-screens -->
- Thêm trang `/dev/kit` (chỉ bật khi dev) dựng lại board `Kit` và `MKit` bằng component thật.
- Thay `NavRail` bằng `AppHeader`. Logo ống kính dùng SVG ở `artboards/Brand.dc.html`. Thay `src/app/icon.svg` và tên app thành “Viderse”.
- Thêm `JobsPill`, chấm báo ở nút Cài đặt, badge Duyệt.
- Làm các thành phần trong `04-components.md` có nhãn **V0**: AgentOrb, ChannelAvatar (chữ cái đầu khi chưa có ảnh), StatusLabel, ActionButton, Chip, Banner.
- Thêm luật lint/kiểm tra: tối đa một `ActionButton variant="primary"` mỗi route.

### Việc backend
- Thêm `agentView()` và `job.subline` (data-contract §1, §2).
- `PATCH /api/clone/:id/metadata` và `POST /api/clone/:id/metadata/rewrite` (data-contract §16).
- Thêm `GET /api/status` (§5).
- Thêm cột `clone_jobs.paused_at` và migration; API và CLI `pause`/`resume`.

### Nghiệm thu
- Mọi trang có thanh trên giống artboard. Không còn rail. Từ 768px trở lên, nav không đủ chỗ thì cuộn ngang, không rớt dòng; dưới 768px là khung điện thoại (#16). <!-- r32 -->
- `JobsPill` cập nhật trong vòng 3 giây sau khi một job bắt đầu hoặc xong. Khi tạm dừng hàng chờ, viên ghi “Đã tạm dừng”.
- Chấm ở bánh răng hiện khi đăng nhập ChatGPT hết hạn; bấm vào mở đúng mục Cài đặt.
- Bật `prefers-reduced-motion` thì logo, orb và chấm đỏ đều đứng yên.
- Chữ phụ không dùng màu dưới `#8B8B97`.
- `/dev/kit` khớp board `Kit` và `MKit` từng nhóm (so ảnh chụp cùng khung). Nút đủ 5 kiểu (chính, viền, nhẹ, nguy hiểm làm lại được, xoá vĩnh viễn) và các trạng thái; hộp xác nhận đủ 7 tình huống ở `Kit` › Xác nhận (kể cả gõ tên và giữ để xác nhận); Tab/Shift+Tab đi qua mọi điều khiển với vòng focus màu nhấn. Đặt cạnh kit Noverse: chỉ khác màu.
- Ngoài `tokens.css` và `kit.css`, mã giao diện không có mã màu hex và bo góc viết cứng (kiểm tra bằng lint).
- Tạm dừng một job đang vẽ ảnh: ảnh đang vẽ xong rồi dừng; job không giữ luồng; Chạy tiếp làm tiếp đúng chỗ.
- Ghi “Gói V0” vào `10-gap-closure.md`.

## Gói V1 — Bàn dựng (P1) <!-- goi-v1 -->

**Vì sao:** người dùng phải mở ba trang (Tổng quan, Video, chi tiết job) mới biết việc của mình. Bàn dựng gộp lại thành một.
- Artboard: `Main` (`scope=all|channel`, `stage=images|review|render|issue|quality|done|paused`, `panel=agents|content|source|backdrop`, `backdropShort`, `promptPop`, `meta=pending|writing|failed|ai|edited|rewrite|history`) · `MMain` (`scope`, `screen=list|detail|jobs|switch|recap|plan`, `agent`, `stage`, `promptPop`, `meta`). <!-- r11-audit -->
- Spec: [ia-flows](01-ia-flows.md) › W5, W7, W8, W12 (Nền); [components](04-components.md) › ProgressLog, LiveStudio, AgentCard, Composer (chỉ giao diện, chưa parse); data-contract §1, §2, §6.

### Việc FE
- Route `/` theo migration §1; redirect `/clone`, `/clone/[jobId]`.
- **Phạm vi** <!-- r5-scope -->
  - `?scope=all` là mặc định; `?channel=<id>` là một kênh. Lưu lựa chọn cuối vào `localStorage`.
  - Bộ đổi kênh ở đầu trang có thêm mục **Mọi kênh**.
- Cột trái:
  - Mọi kênh: khối **Kênh** gồm 3 dòng (`ChannelRow`: avatar, tên, nhịp `done/cap` kèm thanh, “N chờ bạn · N sẵn sàng đăng”, icon cam khi template cần sửa). Bấm dòng thì chuyển sang `?channel=<id>`.
  - Nhóm **Chờ bạn · Đang làm · Sẵn sàng đăng · Chờ lượt** (thêm **Đã đăng** khi xem một kênh). Nhóm rỗng thì ẩn. Mỗi mục kèm `subline`; ở Mọi kênh mỗi mục có avatar kênh 20px.
  - Chờ lượt hiện 3 video (Mọi kênh) hoặc 2 video (một kênh) và link “Xem cả N trong Hàng chờ”. Đã đăng hiện 1 video và link tới trang kênh.
  - Chân cột: Mọi kênh ghi “Tự lấy video: bật ở x/3 kênh” và “Hàng chờ chung: N video”; một kênh ghi Tự lấy video và Template luân phiên.
- Điện thoại (`MMain`): ở Mọi kênh có dải thẻ kênh cuộn ngang. Bộ đổi kênh là sheet. Các nhóm theo cùng thứ tự với máy tính.
- Giữa:
  - Tiêu đề, chip, banner theo trạng thái, ProgressLog, LiveStudio, Composer.
  - Composer chỉ giao diện: gửi thì báo “Sắp có”, cho tới khi V11 xong.
- Phải:
  - Tab **Trợ lý:** 4 AgentCard.
  - Tab **Nội dung:** tiêu đề, mô tả, tag sửa trực tiếp (bộ đếm, nhãn “bạn đã sửa”), đoạn cuối của kênh và tag luôn thêm hiện riêng, **Viết lại** (chọn phần + góp ý), **Lịch sử** (bản trước + Dùng lại), sao chép từng phần / cả gói. Trạng thái: Chưa viết (trước khi có lời thoại) · Đang viết (khung xương) · Chưa viết được (khối cam, Thử lại, Đăng nhập lại). Data-contract §16. <!-- r10-channel --> <!-- r11-audit -->
  - Điện thoại: mục **Nội dung đăng** trong chi tiết video, cùng trạng thái; Viết lại và Lịch sử là sheet.
  - Tab **Nguồn:** Mở trên YouTube · Khám phá (không còn trang chi tiết video đối thủ).
  - Tab **Nền:** độ phủ, danh sách clip và các thao tác. Khoá khi Kiểm định đang dựng.
- Đầu trang video: Tạm dừng (Esc) / Chạy tiếp, và menu ⋯ theo W8. Khi tạm dừng (`stage=paused`): video chuyển sang nhóm Chờ bạn, banner xám, “Chạy tiếp” là nút chính, Hoạ sĩ ở trạng thái Dừng, ô nhắn mời góp ý trước khi chạy tiếp.
- Theo trạng thái:
  - `quality`: danh sách mục chưa đạt, Sửa trong editor (đỏ), Vẫn xuất (viền).
  - `done`: trình phát, Tải gói đăng (đỏ), Đánh dấu đã đăng, gợi ý xoá MP4.

### Việc backend
- `pickDefaultJob`. Thêm nhóm `paused` vào list API.
- Đọc `exportBytes` trước khi xoá MP4.
- Không đổi API Nền (dùng `backdrop-*` có sẵn).

### Nghiệm thu
- Mở `/` khi có 1 video Cần xử lý và 2 video Chờ duyệt → mở đúng video Cần xử lý cũ nhất.
- Lần đầu mở `/` thì phạm vi là Mọi kênh. Tổng “Chờ bạn” trên các dòng kênh bằng số mục của nhóm Chờ bạn. Badge Duyệt bằng số mục Chờ duyệt của cả 3 kênh.
- Bấm dòng “Quiet Hours” → danh sách chỉ còn video Quiet Hours; nhóm rỗng ẩn; bộ đổi kênh ghi “Quiet Hours”; video đang mở vẫn giữ và có avatar kênh cạnh tiêu đề.
- `/clone/abc` → `/?video=abc`, nút quay lại vẫn về trang trước.
- Video đang vẽ cảnh 5: ô cảnh 5 có orb Hoạ sĩ nhấp nháy, cảnh 6 ghi “Chờ lượt vẽ”, thẻ Hoạ sĩ nổi lên.
- Chờ duyệt: có đúng một nút “Duyệt ảnh” trên banner; không có nút chính thứ hai trên màn.
- Tab Nền khi đang dựng: nút khoá, lý do nằm ngay cạnh. Bỏ một clip thì độ phủ giảm ngay; thiếu thì thanh chuyển vàng.
- Mỗi trạng thái (`stage`) có tối đa một nút chính.
- Ghi “Gói V1” vào `10-gap-closure.md`.

## Gói V2 — Làm video (một hành động) (P1) <!-- goi-v2 -->

**Vì sao:** “Bắt đầu ngay” và “Thêm vào hàng chờ” làm người dùng không biết video chạy khi nào. Giờ chỉ còn một hành động, và có kế hoạch nói trước.
- Artboard: `NewVideo` (`input=empty|pasted|duplicate|error|multi`), `MNew`.
- Spec: ia-flows L1, L2, L4, W4; data-contract §3, §4; copy › Kế hoạch.

### Việc FE
- Hai chế độ **Một link · Nhiều link** (đã cắt “Chọn video nổi trội”, trùng Khám phá). Chip Kênh / Template đã điền sẵn (không có chip Ảnh ở đây — L14, Gói V16; sửa đợt 31); thẻ gợi ý video ở dưới điền link hoặc dẫn sang Khám phá.
- Dòng **kế hoạch** lấy từ `POST /api/clone/plan` (debounce 300ms sau khi dán).
- Nút chính duy nhất “Làm video” / “Làm N video” (Enter). Nhiều link có công tắc “Ưu tiên cả N”.
- Trùng link → khối vàng + “Mở video đang làm”. Link lỗi → khối cam.
- Đổi nút ở Video đối thủ, Khám phá (chọn nhiều) và “Làm mới nhất” của đối thủ sang cùng luồng `enqueue`.

### Việc backend
- `clone_queue.priority`, chèn `pick` sau mục `pick` cuối cùng.
- Scheduler lấy `pick` trước, rồi luân phiên `batch`. Chạy scheduler ngay khi enqueue.
- Nhịp chỉ đếm `batch`. Migration: mục đang có → `batch`.
- `POST /api/clone/plan`, `POST /api/clone/enqueue`.
- CLI: `clone batch --priority pick|batch`; `clone start` = `pick`.

### Nghiệm thu
- Máy rảnh, dán 1 link, Enter → video vào Đang làm trong vòng 2 giây; dòng kế hoạch trước đó ghi “Bắt đầu ngay khi bấm”.
- Máy bận 2/2, dán 1 link → “Lên đầu hàng chờ · bắt đầu khoảng …”. Trong Hàng chờ, video đứng đầu với nhãn “Ưu tiên · bạn chọn”.
- Dán 4 link (1 trùng), nhịp còn 2 → “3 link đọc được · 1 đã làm cho kênh này · sẽ bỏ qua”, “video thứ 3 chạy ngày mai”. Bật “Ưu tiên cả 3” thì kế hoạch đổi thành lên đầu hàng chờ.
- Video chọn tay không làm giảm “Nhịp hôm nay”.
- Ghi “Gói V2” vào `10-gap-closure.md`.

## Gói V3 — Duyệt (P1) <!-- goi-v3 -->

**Vì sao:** duyệt là việc lặp nhiều nhất, nên cần nhanh hơn và cho người dùng thấy đủ ngữ cảnh để quyết.
- Artboard: `Review` (`state=review|redrawing|blocked|empty`, `view=full|cutout`). <!-- r2-v3 -->
- Bổ sung đợt 2: nút **Vẽ lại cả 7 ảnh** (hỏi xác nhận, hộp thoại `Dialogs` · `dialog=redraw-all`); lớp **Ảnh gốc / Tách nền** trên ảnh cảnh, nền ô caro để soát mép tóc và tay; ảnh **bị chặn** hai lần hiện “Bị chặn · dùng ảnh cảnh 4”, ô trong dải có dấu cam, không tính vào Đã duyệt.
- Spec: ia-flows L7, W6.

### Việc FE
- Hàng video chờ duyệt bên trái, ảnh lớn có lớp chạm trái/phải, dải ảnh, ô góp ý.
- Các nút: Vẽ lại (R), Bỏ ảnh này, Duyệt cả video & dựng (A, đỏ).
- Cột phải: lời kể của cảnh, khung đối thủ làm mẫu, mô tả Hoạ sĩ đã dùng, lần vẽ.
- Duyệt xong tự chuyển video kế. Badge Duyệt cập nhật ngay.

### Việc backend
- Không đổi API. Dùng regenerate, skip, approve có sẵn.

### Nghiệm thu
- Phím ←/→/J/K/R/A chạy đúng. Phím tắt không chạy khi đang gõ.
- Đang vẽ lại 1 ảnh → nút duyệt khoá với lý do; ảnh khác vẫn xem và vẽ lại được.
- Duyệt video cuối cùng → màn “Đã duyệt hết”, badge ẩn.
- Ghi “Gói V3” vào `10-gap-closure.md`.

## Gói V4 — Kênh (P1) <!-- goi-v4 -->

**Vì sao:** mỗi thiết lập của kênh có một nhà, và thao tác hàng loạt nằm cạnh danh sách video.
- Artboard: `Channel` (`tab=videos|sources|templates|settings`, `filter=all|working|waiting|queued|ready|posted`, `select`, `identity=ready|generating|failed`, `link=none|picker|empty`, `autonomy=balanced|careful|auto|custom|confirm`) · `MChannel` (`tab=videos|sources|templates|settings`, `identity`, `autonomy`, `sheet=none|rival|unlink|row`). <!-- r2-v4 --> <!-- r10-channel --> <!-- r11-audit -->
- Bổ sung đợt 2–3: **không có** bộ lọc Bản cũ (xem 09-migration-yt-factory.md); tab **Hồ sơ kênh** gộp nhận diện + thông tin kênh; nhận diện có công tắc **Vùng hiện trên điện thoại 1546×423** và trạng thái Hoạ sĩ đang vẽ (ảnh cũ giữ tới khi xong); kênh mới rỗng xem `Channel` · `fresh=true` (đợt 21); xoá kênh dùng `Dialogs` · `dialog=delete-channel` (gõ đúng tên mới mở khoá).
- Spec: ia-flows L4, L12, W9, W11; data-contract §7; components › BulkBar, Stepper, Switch.

### Việc FE
- Cột trái: “Mọi video của 3 kênh”, kênh của tôi (chấm vàng khi có Chờ bạn), Tạo kênh, Thư viện dùng chung.
- Đầu kênh: avatar, chip, Mở YouTube, “Video mới cho kênh” (đỏ), 4 số liệu.
- Tab **Video:**
  - Bộ lọc, “Chọn nhiều” → BulkBar có đếm số video hợp lệ cho từng nút.
  - Tải gói đã xong.
  - Dưới 1380px thì ẩn cột Template.
- Tab **Nguồn & tự động:** đối thủ (Làm mới nhất = `pick`; **Chọn video nổi trội** = mở Khám phá ở đối thủ đó, sắp theo Nổi trội, chế độ chọn nhiều, rồi `batch`; **Bỏ liên kết** = `unlinkCompetitor` sau hộp xác nhận; nhãn **Tham chiếu nhận diện** ở đối thủ đầu tiên), **Liên kết đối thủ** chọn từ danh sách đang theo dõi (`link=picker`; chưa có đối thủ: `link=empty`), **Nhịp mỗi ngày** + **Ưu tiên kênh**, **Mức tự chủ** (Gói V20; lên Tự chạy / Tự duyệt hết thì hỏi xác nhận, `autonomy=confirm`; hạ mức không hỏi), Lịch đăng của đối thủ. <!-- r11-audit -->
- Tab **Template luân phiên** (tách từ tab Thiết kế cũ).
- Tab **Hồ sơ kênh** (ia-flows W11):
  - Nhận diện: ảnh bìa + công tắc vùng điện thoại, avatar, Vẽ lại avatar / ảnh bìa (`part`), Đặt tên khác (`part=name`), Tải về; mô tả kênh sửa được (`brief`); khối **Phong cách nhận diện** chỉ đọc (`summary`, nguồn = đối thủ tham chiếu); Góp ý (`direction`) + Tạo lại cả bộ (`part=all`).
  - Trạng thái `generating` (ảnh cũ giữ) và `failed` (khối cam, Thử lại, giờ tự thử lại).
  - Khối **Khi viết tiêu đề, mô tả, tag:** ngôn ngữ, giọng văn (`styleNote`), đoạn cuối mô tả, tag luôn thêm.
  - Tên, link; liên kết sang Cài đặt › Trợ lý AI; Xoá kênh (cam, khoá khi có video đang chạy).
- Điện thoại: đủ 4 tab, cùng thao tác (sắp xếp template bằng nút lên/xuống; ⋯ của đối thủ là sheet; Mức tự chủ: mỗi dòng mở sheet chọn, Tự chạy / Tự duyệt hết hỏi xác nhận bằng sheet — Gói V20). Chỉ máy tính: Chọn nhiều, Dặn trợ lý về kênh này (ia-flows §6.2).
- Cột phải “Đang áp dụng” chỉ đọc; mỗi dòng có **Tuỳ chỉnh** mở đúng tab; nhãn “riêng” chỉ khi khác mặc định kênh mới (ia-flows W11). <!-- r11-audit -->

### Việc backend
- `bulkEligibility()`.
- Đổi nghĩa giới hạn ngày thành nhịp (cùng gói V2).
- `owner_channels` thêm `language`, `description_footer`, `always_tags` (data-contract §16); step `metadata` dùng ba trường này.
- Branding API nhận `part`, `brief`, `direction` (đã có trong `brandingInputSchema`); trả trạng thái `failed` + lý do + giờ thử lại.

### Nghiệm thu
- Chọn 3 video (1 Chờ duyệt, 1 Cần xử lý, 1 Sẵn sàng) → BulkBar hiện Thử lại 1 · Duyệt & dựng 1 · Tải gói 1 · Đánh dấu đã đăng 1.
- Shift-click chọn được một khoảng. Esc thoát chọn.
- Không có ô sửa nhịp nào ngoài tab Nguồn & tự động; Hàng chờ chỉ hiển thị nhịp và có link sang đây.
- Kênh đang có video chạy → Xoá kênh khoá, có lý do và link tới các video đó.
- Thêm đoạn cuối mô tả và 3 tag luôn thêm → video viết sau đó có đoạn cuối nguyên văn và 3 tag đứng đầu; tổng vẫn ≤ 5.000 ký tự / 30 tag / 500 ký tự (Đạo diễn rút bớt tag của mình).
- Bỏ liên kết đối thủ tham chiếu → đối thủ kế tiếp có nhãn Tham chiếu nhận diện; nhận diện hiện có không đổi tới khi bấm tạo lại.
- Chọn Tự chạy hoặc Tự duyệt hết → hộp xác nhận; Huỷ giữ nguyên mức cũ. Hạ mức → không hỏi. <!-- r11-audit --> <!-- r16-agent -->
- Bỏ liên kết đối thủ cuối cùng → Tự lấy tự tắt và ghi lý do; video đã vào hàng chờ vẫn chạy.
- Ghi “Gói V4” vào `10-gap-closure.md`.

## Gói V5 — Khám phá (xem trước video, không có trang chi tiết) (P1) <!-- goi-v5 -->

**Vì sao:** từ lúc thấy video hay tới lúc nó vào hàng chờ chỉ còn hai bấm.
- Artboard: `Explore` (`preview`, `select`, `rivalMenu`, `scan=running|paused|limited|done`). <!-- r2-v5 -->
- Bổ sung đợt 2: menu ⋯ của đối thủ (Kiểm tra video mới · Mở trên YouTube · Liên kết với kênh của tôi · **Bỏ theo dõi** màu cam, nêu rõ video đã làm giữ nguyên và số video trong hàng chờ bị bỏ). Chưa có đối thủ nào: `Explore` · `empty=true` (đợt 21).
- Spec: ia-flows W3.

### Việc FE
- Cột trái đối thủ dùng ChannelAvatar (không orb). Thêm đối thủ bằng @handle hoặc link.
- Lưới video có bộ lọc, chọn nhiều, BulkBar với “Làm N video cho <kênh>” (đỏ).
- Cột phải: thông tin đối thủ, quét lịch sử, **Lịch đăng**: toggle VN / New York, danh sách **mọi khung đăng lặp lại** (thứ · khoảng giờ · n/30 · độ đều, khung nhiều nhất có nhãn), bấm khung → làm nổi ô trên bảng thứ × giờ, dòng “n video lẻ ngoài các khung”. Trang Kênh (cột phải) có khối **Lịch đăng** gộp khung của mọi đối thủ liên kết kèm avatar đối thủ.
- Bấm một thẻ → cột phải thành **Video đã chọn**: ảnh, điểm nổi trội, chương, chọn kênh (kênh đã làm video này hiện “Đã làm · trạng thái” + Mở, không chọn được; components › Ảnh trong giao diện), <!-- r12-small --> dòng kế hoạch, nút chính “Làm video cho <kênh>”, “Mở trên YouTube”, “← Thông tin đối thủ”.
- **Đã cắt** trang `/videos/[id]` và việc lấy/dán transcript trên UI (app tự chép lời). Redirect theo 09-migration-yt-factory.md; transcript vẫn có ở CLI `ytf video`.

### Việc backend
- Không đổi API ngoài `enqueue` (V2).

### Nghiệm thu
- Chọn Top 10 → “Làm 10 video cho Midnight Tales” → 10 video vào cuối hàng chờ theo nhịp.
- Bấm một video → panel Video đã chọn; Làm video → video vào đầu hàng chờ (`pick`), dòng Midnight Tales trong “Làm cho kênh” đổi thành “Đã làm · Chờ lượt” + Mở.
- Ghi “Gói V5” vào `10-gap-closure.md`.

## Gói V6 — Hàng chờ (P1) <!-- goi-v6 -->

**Vì sao:** người dùng hỏi “sao video này chưa chạy?”, và câu trả lời phải nằm cạnh hàng chờ.
- Artboard: `Queue` (`state=running|paused|limited|empty`); điện thoại `MMain` · `screen=jobs`.
- Spec: ia-flows L1–L6, W10; data-contract §3.

### Việc FE
- Hai thẻ Đang chạy (thanh 9 đoạn theo step).
- Thẻ Luân phiên có kênh kế tiếp và lý do bỏ lượt.
- Bảng Chờ lượt có lọc theo kênh, kéo thả, Lên đầu, ⋯.
- Cột Quy tắc chạy: 3 stepper, nhịp (chỉ đọc), hạn mức ảnh, xong hôm nay.
- Tạm dừng / Chạy tiếp; `limited` có banner cam.

### Việc backend
- `GET /api/queue` theo §3; chuyển 3 khoá thiết lập sang PATCH của Hàng chờ.
- Lý do bỏ lượt và `etaStart`.

### Nghiệm thu
- Kênh QH đủ nhịp → thẻ Luân phiên ghi QH bị bỏ tới 00:00. Các mục của QH ghi “Ngày mai”.
- Hết hạn mức mọi nguồn vẽ (chính + dự phòng) → banner cam, nút “Thêm nguồn vẽ dự phòng”. Video đang vẽ ghi “Chờ hạn mức ảnh · 07:00”; video ở bước khác vẫn chạy.
- Kéo một mục lên đầu → giờ dự kiến của mọi mục được tính lại.
- Ghi “Gói V6” vào `10-gap-closure.md`.

## Gói V7 — Cài đặt (P1) <!-- goi-v7 -->

**Vì sao:** Cài đặt chỉ còn công cụ, tài khoản và dữ liệu; thiết lập chạy máy đã chuyển sang Hàng chờ.
- Artboard: `Settings` (`section=tools|connect|agents|accounts|storage|logs`, `tools=ok|update|failed|installing`, `grok=signed|code|out`, `bili=saved|empty`, `clean=idle|preview|done`). Mục Kết nối AI và Trợ lý AI thay mục Vẽ ảnh cũ: xem Gói V17.
- Spec: ia-flows W13.

### Việc FE
- Cột trái 5 mục, giữa là nội dung, phải là thẻ “Khi nào có hiệu lực”.
- Công cụ gọi theo việc làm, mỗi công cụ một nhãn trạng thái; chỉ khi có vấn đề mới hiện một nút.
- Lưu trữ (trước là Dọn dữ liệu): thư mục lưu dự án (Gói V22), biểu đồ dung lượng, Xem trước → Dọn.
- Mọi xoá đều có xác nhận cam ngay tại chỗ.

### Việc backend
- Không đổi API (dùng `tools`, `cleanup --dry-run`, `logs` có sẵn).

### Nghiệm thu
- yt-dlp có bản mới → dòng “Tải video và phụ đề” hiện “Có bản mới” + Cập nhật. Phiên bản chỉ nằm trong “Chi tiết”.
- Đăng nhập Grok bằng mã: hiện mã, và nút “Mở trang xAI” là nút chính duy nhất.
- Ghi “Gói V7” vào `10-gap-closure.md`.

## Gói V8 — Thư viện + Cài đặt template (P1) <!-- goi-v8 -->

**Vì sao:** thứ dùng chung nằm một chỗ. Template tự lưu, có xem thử và có giá.
- Artboard: `Library` (`tab=templates|backdrops|overlays|projects`, `menu`, `nguon=channels|videos|adding|addvideo|unsupported` — đợt 22, xem Gói V23), <!-- r2-v8 --> `Template` (`section=images|cover|backdrop|output`, `preview=fresh|stale|building`, `fewClips`, `picker`).
- Spec: ia-flows W12.

### Việc FE
- Thư viện có 3 tab, khung cột trái giống trang Kênh.
- Template:
  - Tự lưu, có chỉ báo “Đã lưu · 23:52”.
  - Bản xem thử thành cũ khi đổi cài đặt; “Dựng lại bản xem thử” (đỏ) chỉ hiện khi cũ.
  - Vẽ thử ảnh bìa là nút trung tính, ghi “1 ảnh”.
- Ước tính số clip nền tính thật: độ dài video ÷ độ dài clip, tối đa 40; vượt thì báo nền cuối sẽ lặp và gợi ý tăng độ dài clip.
- Xoá template đang dùng → bị chặn kèm lý do.

### Việc backend
- `GET /api/templates/overlays` (kho lớp phủ có sẵn qua CLI).
- Không đổi recipe.

### Nghiệm thu
- Đổi prompt ảnh cảnh → chỉ báo “Đã lưu”, bản xem thử chuyển “Cần dựng lại”, nút chính hiện. (Kho prompt: Gói V15.)
- Kênh nền Bilibili bị giới hạn → Làm mới khoá tới giờ thử lại (Video nền: Gói V23).
- Ghi “Gói V8” vào `10-gap-closure.md`.

## Gói V9 — Lần đầu mở và Tạo kênh (P1) <!-- goi-v9 -->

**Vì sao:** kênh mới làm được video ngay sau một luồng, không phải đi qua tour.
- Artboard: `Onboard` (`step=setup|create|done`, `setup=running|login|failed`, `identity=running|failed`, `rivals=some|none|empty`) · `CreateChannel` (`step=create|done`, `rivals`, `identity`) · `MCreate` (cùng tweak) · `Channel` (`fresh=true`). <!-- r11-audit --> <!-- r13-create --> <!-- r21-empty -->
- Spec: ia-flows W1, W2; data-contract §8.

### Việc FE
- `/setup` đổi giao diện thành Chuẩn bị máy: 6 dòng, %, pha.
- `/my-channels/new` chỉ **một màn Tạo kênh**, cả máy tính và điện thoại (đối thủ bắt buộc, có ô theo dõi đối thủ mới ngay trong danh sách; tên, link tuỳ chọn; khối “Cách kênh chạy” điền sẵn Mẫu chuẩn tự tạo = Tách nền + Theo nhận diện kênh + bìa Chữ hook, Tự lấy bật, Nhịp 5, Tự duyệt tắt; ngôn ngữ đoán từ đối thủ tham chiếu) → màn **Xong** với thẻ Đang chạy nền (đặt tên, vẽ nhận diện, tạo Mẫu chuẩn) và nút chính “Làm video mới nhất của <đối thủ>”. Không bắt đợi; xong có thông báo (V14).
- Xoá tour và hướng dẫn 5 bước.

### Việc backend
- Tạo kênh trả `identityJobId` và chạy tạo nhận diện ngay.

### Nghiệm thu
- Máy mới: cài tự chạy; tới bước đăng nhập ChatGPT thì nút chính duy nhất là Đăng nhập.
- Tạo kênh không chọn đối thủ → nút Tạo kênh khoá kèm lý do.
- Chưa theo dõi đối thủ nào → dán @handle ở màn Tạo kênh → đối thủ được theo dõi (bắt đầu quét) và tick sẵn; không phải rời màn. <!-- r13-create -->
- Xong → “Làm video mới nhất” tạo video `pick` cho kênh mới.
- Ghi “Gói V9” vào `10-gap-closure.md`.

## Gói V10 — Điện thoại (P2, phụ thuộc V0–V3; D1 đã chốt: LAN + ghép thiết bị) <!-- goi-v10 -->

**Vì sao:** duyệt ảnh và theo dõi được khi không ngồi máy.
- Artboard: `MMain` (`scope`, `screen=list|detail|jobs|switch`, `stage=images|review`, `promptPop`, `meta`), `MReview` (`state=review|feedback|redrawing|empty|learn`, `score`), `MChannel` (`tab=videos|sources|templates|settings`, `identity`, `autonomy`, `sheet`), `MNew` (`input=empty|pasted|multi`), `MExplore` (`mode=browse|select|info|add`), `MCompVideo` — Chi tiết video (`sheet=none|picker|why`), `MSettings` (`screen=list|tools|connect|agents`, `sheet=none|add|models|director|artist`). <!-- r2-v10 --> <!-- r11-audit -->
- Spec: ia-flows §6.2; components › MobileTabBar, Sheet; data-contract §10.

### Việc FE
- Dưới 768px:
  - Thanh dưới 5 mục; Hàng chờ và Góp ý là Sheet.
  - Mọi vùng bấm ≥ 44px.
  - Trang chỉ có trên máy tính hiện “Mở trên máy tính” (danh sách ở ia-flows §6.2). Tạo kênh có bản điện thoại `MCreate`.

### Việc backend
- D1 đã chốt cách (a): nghe trên LAN, ghép thiết bị một lần bằng mã QR, rồi dùng cookie phiên (data-contract §10, #11). <!-- r32 -->

### Nghiệm thu
- 390×844: không cuộn ngang ở mọi màn; duyệt được một video chỉ bằng chạm.
- Ghi “Gói V10” vào `10-gap-closure.md`.

## Gói V11 — Nhắn trợ lý (P2, phụ thuộc V1, V4) <!-- goi-v11 --> <a id="goi-v11"></a>

**Vì sao:** sai khiến trợ lý bằng câu tự nhiên thay vì tìm nút. Mỗi câu thành một thao tác có sẵn, và luôn xác nhận trước khi chạy.
- Artboard: `Main` (Composer), `Channel` (Composer kênh).
- Spec: ia-flows W14; data-contract §9.

**Intent v1:**

| Phạm vi | Intent | Ví dụ câu | Service |
|---|---|---|---|
| video | `redraw_image` | “cảnh 5 cho y tá cầm đèn pin, tông lạnh hơn” | `clone regenerate --prompt` |
| video | `redraw_all` | “vẽ lại cả bộ theo phong cách mới” | vẽ lại tất cả (hỏi giá) |
| video | `skip_image` | “bỏ ảnh cảnh 6” | `clone skip` |
| video | `edit_metadata` | “đổi tiêu đề ngắn hơn” | sửa metadata |
| video | `reroll_backdrop` | “đổi nền khác cho đoạn đầu” | `backdrop-reroll` |
| video | `no_images` | “video này không cần ảnh” | `images-mode none` |
| video | `pause` / `resume` | “tạm dừng video này” | §2 |
| kênh | `set_pace` | “mỗi ngày làm 3 video thôi” | nhịp |
| kênh | `filter_auto` | “chỉ lấy video trên 20 phút từ Nocturne Stories” | bộ lọc Tự lấy (**mới**: `minDurationSec`, theo đối thủ) |
| kênh | `make_latest` | “làm video mới nhất của Late Archive” | enqueue `pick` |

**Cập nhật đợt 16 — kế hoạch nhiều bước** <!-- r16-agent -->: câu → kế hoạch 1–5 bước (`assistant/plan`, data-contract §22), thêm intent `prioritize`, `set_autonomy`, `add_rule`, `make_videos`, và phạm vi **mọi kênh** (ô nhắn ở Bản tin). Thiếu thông tin → `ask` với lựa chọn. Artboard `Main` · `agent=plan`, `MMain` · `screen=plan`.

### Việc FE
- Gửi → **thẻ Kế hoạch** ngay trên ô nhắn: mỗi bước một dòng có ô tích, orb trợ lý, giá; dòng tổng; nút **Làm N bước** (luôn sáng như board, #74) và **Sửa câu**. Câu không hiểu → “Chưa làm được việc này” + 2 gợi ý.

### Việc backend
- `parse` và `run` theo §9. Phân loại bằng ChatGPT qua đường Codex hiện có, đầu ra JSON theo schema, chỉ nhận đúng các intent trong bảng. Câu người dùng là dữ liệu, không phải chỉ dẫn hệ thống.
- Thêm bộ lọc Tự lấy `minDurationSec` theo từng đối thủ.

### Nghiệm thu
- “cảnh 5 cho y tá cầm đèn pin” → thẻ “Vẽ lại cảnh 5 · 1 ảnh” → bấm Làm → cảnh 5 vào trạng thái đang vẽ.
- “xoá kênh” → không thành intent nào (không có trong danh sách), trả gợi ý.
- Ghi “Gói V11” vào `10-gap-closure.md`.

## Gói V12 — Hộp thoại, thông báo, trạng thái hệ thống (P1, làm cùng V0–V1) <!-- goi-v12 -->

**Vì sao:** mọi thao tác tốn kém hoặc không hoàn tác được phải nói trước hệ quả; mọi trang phải có trạng thái đang tải, rỗng, lỗi, mất kết nối.
- Artboard: `Dialogs` (`dialog=delete-video|delete-channel|autonomy|redraw-all|restart|clip-picker|shortcuts|channel-switch|assistant-confirm|assistant-unknown|toasts`, `typed`), `System` (`state=loading|offline|not-found|empty-workbench`); trạng thái trống của trang khác nằm ở board của trang đó: `Channel` · `fresh=true`, `Explore` · `empty=true`. Nền phía sau `Dialogs`, `Activity`, `System` là `Main` hiện tại (đợt 21). <!-- r21-empty -->
- Spec: ia-flows L11, W7; 06-copy.md › Mẫu câu; components › ActionButton (Phá huỷ), EmptyState.

### Việc FE
- Một `ConfirmDialog` dùng chung: tên đối tượng, hai cột **Sẽ xoá / Giữ lại**, có hoàn tác được không, Esc đóng.
  - Hộp thoại phá huỷ (xoá video, xoá kênh) focus mặc định vào **Huỷ**; nút xoá màu cam.
  - Hộp thoại có giá (vẽ lại cả bộ, làm lại từ đầu) ghi số ảnh / lượt sẽ tốn trước khi bấm.
- Xoá kênh: phải gõ đúng tên mới mở khoá; có video đang chạy thì khoá với lý do và link tới các video đó.
- Chuyển Mức tự chủ sang Tự chạy (`dialog=autonomy`): nói rõ ngưỡng 85, ảnh dưới ngưỡng vẫn hỏi, video vẫn qua độ hoàn thiện, không bao giờ tự đăng/xoá/đổi template.
- **Chọn đoạn khác** cho một clip nền: danh sách video của kênh nền (lọc theo kênh, loại), xem trước, thanh chọn đoạn Từ–Đến, “Giữ clip này khi đổi ngẫu nhiên tất cả”.
- **Phím tắt** (`?`): nhóm Chung · Bàn dựng · Duyệt · Chọn nhiều.
- **Đổi kênh** trên đầu trang: Mọi kênh (3 avatar chồng nhau, “Hôm nay 6/13 · 4 chờ bạn”), 3 kênh có nhịp hôm nay và chấm Chờ bạn, Tạo kênh. Ở Bàn dựng mục này đổi phạm vi (Gói V1).
- **Nhắn trợ lý**: thẻ xác nhận ngay trên ô nhắn (V11) và trạng thái không hiểu.
- **Toast** theo chuẩn ở Gói V14 (cùng thành phần, cùng vị trí): `Dialogs` · `dialog=toasts` dùng chung với `Activity` · `view=toasts`.
- `loading.tsx` dạng khung Bàn dựng; trang **Không tìm thấy** dùng `PageState`; **mất kết nối**: dải cam dưới đầu trang, tự thử lại sau 5 giây, khoá thao tác ghi, nút Thử lại ngay.
- Trang rỗng: Bàn dựng chưa có video, kênh mới chưa có video, Khám phá chưa có đối thủ — mỗi trang một lối đi tiếp.

### Việc backend
- `GET /api/health` để client phát hiện mất kết nối (poll cùng `GET /api/status`).
- Không đổi API xoá/làm lại (dùng `clone delete`, `clone restart`, `owner-channels` có sẵn); hộp thoại cần đếm trước: số video của kênh, số video đang chạy, dung lượng MP4.

### Nghiệm thu
- Xoá video đang chạy → nút trên menu ⋯ khoá, có lý do “Tạm dừng trước rồi xoá”.
- Tắt máy chủ khi đang mở Bàn dựng → dải cam hiện trong 5 giây, nút ghi bị khoá; bật lại → dải tự biến mất, dữ liệu tự làm mới.
- Mở `/?video=khong-ton-tai` → trang Không tìm thấy có một nút về Bàn dựng.
- Ghi “Gói V12” vào `10-gap-closure.md`.

## Gói V13 — Editor theo Viderse (P2, phụ thuộc V0) <!-- goi-v13 -->

**Vì sao:** editor là nơi sửa khi video chưa đạt độ hoàn thiện; phải cùng ngôn ngữ thiết kế và dẫn thẳng tới lỗi cần sửa.
- Artboard: `Editor` (`state=editing|export-gate|exporting|export-done|conflict|narrow`, `fix=todo|partial|done|measuring|passed`, `panel=fix|scenes|subs|bg|audio|media`, `focus=caption|scene|bg|none`). Bố cục đợt 27: ia-flows W15, components › Editor. <!-- r27-editor -->
- Spec: DESIGN.md › Editor (bố cục, mật độ 28px/24px, phím tắt, trợ năng) vẫn là luật; Viderse chỉ đổi bề mặt, chữ, màu và luồng vào/ra.

### Việc FE
- Đổi tokens sang Viderse (Be Vietnam Pro, bề mặt, đỏ `#E6002E` cho Xuất và playhead); giữ track tokens `--ed-track-*`.
- Thanh trên: “← Bàn dựng”, tên + dòng kênh · thời lượng · số cảnh · template, Đã lưu, hoàn tác/làm lại, `?` phím tắt, chip **Độ hoàn thiện** (bấm mở Cần sửa), **Xuất** (chỉ là nút chính khi đã đạt).
- Thanh trái **có chữ** theo việc: Cần sửa · Cảnh · Phụ đề · Nền · Âm thanh · Media (thay Media/Chữ/Hiệu ứng/Chuyển cảnh/Phụ đề/Mẫu). Chữ/hiệu ứng/chuyển cảnh/mẫu của NLE vẫn có trong code nhưng không đưa lên thanh; chuyển cảnh sửa ở bảng phải khi chọn cảnh.
- **FixPanel** từ `GET /api/editor/:id/quality`; **Khớp theo giọng** (từng đoạn / cả N đoạn) gọi `captions/align`; **Đo lại** gọi `measure` và theo dõi qua Việc nền.
- **Bảng phải theo lựa chọn** thay 3 tab Clip/Chuyển cảnh/Khung hình: CaptionSyncInspector, cảnh (Vẽ lại → hộp thoại góp ý như Duyệt, Thay ảnh, chuyển động, sang cảnh sau, Nâng cao), clip nền (dùng lại thao tác Bàn dựng › Nền), không chọn gì.
- Timeline: Tách · Xoá · Bắt dính · chip Đoạn lệch n/N; bỏ công cụ “V/T”, ẩn In–Out khi chưa đặt. Đoạn lệch cam; đã khớp về màu track.
- **ExportGateDialog** khi bấm Xuất lúc chưa đạt; Vẫn xuất gửi `accept: true`.
- Xuất xong: “Quay lại Bàn dựng” (đỏ) — Kiểm định tự đo lại độ hoàn thiện.
- Xung đột: Tải lại bản mới · Tiếp tục ở đây (tạm tắt tự lưu) · Lưu thay đổi thành bản sao (đỏ).
- Dưới 1024px: “Cần màn hình rộng ≥ 1024px” + một nút về.

### Việc backend
- Trả danh sách đoạn chưa đạt của checklist (`clone-quality`) theo mốc thời gian để editor đánh dấu. Có sẵn trong score; cần thêm `ranges`.

### Nghiệm thu
- Mở editor từ trạng thái Chưa đạt → bảng Cần sửa mở, đoạn lệch đầu tiên được chọn, playhead ở đó, chip đếm đúng số đoạn.
- Bấm **Khớp theo giọng** ở đoạn đang chọn → dải Chữ thẳng hàng với Tiếng, trạng thái “Đã khớp tiếng”, đoạn trên timeline về màu track; ⌘Z hoàn tác được.
- **Khớp theo giọng cả 3 đoạn** → Cần đo lại hiện 2 mục; **Đo lại** thành nút chính; đo xong ≥ 90 → chip xanh, Xuất thành nút chính.
- Bấm Xuất lúc 84/100 → hộp thoại hỏi; Sửa tiếp quay về Cần sửa; Vẫn xuất bắt đầu xuất và ghi accepted.
- Thanh trái đọc được bằng trình đọc màn hình (tab có tên), không còn nút chỉ có icon.
- Ghi “Gói V13” vào `10-gap-closure.md`.

## Gói V14 — Việc nền & thông báo (P1, làm cùng V0) <!-- goi-v14 -->

**Vì sao:** việc chạy nền phải có một chỗ, một dạng, và thông báo chỉ khi máy cần bạn, để người dùng không phải ngồi canh và không bị làm phiền.
- Artboard: `Activity` (`view=running|inbox|toasts|system|settings|empty`).
- Spec: ia-flows L13, §0 (thông báo đưa về đúng bước); components › JobsPill, Toast.

### Việc FE
- Viên ở đầu trang mở **popover Việc nền** (448px, neo dưới viên): tab **Đang chạy** (nhóm Video · Khác) và **Thông báo** (lịch sử, nhóm Hôm nay/Hôm qua, chấm chưa đọc, Đánh dấu đã đọc); chân popover: Mở Hàng chờ · Cài đặt thông báo. Viên có chấm vàng khi có thông báo cần bạn chưa đọc.
- Mỗi việc nền một dạng: orb trợ lý hoặc icon hệ thống · tên + đối tượng · câu pha · thanh tiến độ (% hoặc x/y, hoặc không xác định) · thời gian còn lại khi đo được · đúng một hành động (Mở / Huỷ / Tiếp tục / Thử lại).
- Toast: góc phải dưới, 300px, tối đa 3, mới nhất trên cùng; 5 giây cho kết quả thao tác (có Hoàn tác/Xem), ở lại tới khi đóng cho Cần xử lý (`role=alert`); rê chuột dừng đếm; Esc đóng; gộp kết quả trong 10 giây. **Không toast tiến độ.**
- Thông báo hệ thống khi cửa sổ ẩn (Chờ duyệt, Cần xử lý, Sẵn sàng) + số trên biểu tượng app = số Chờ bạn.
- Cài đặt thông báo: loại × (Trong app / Hệ thống), gộp 10 giây, Không làm phiền (vẫn ghi lịch sử), số trên biểu tượng.

### Việc backend
- Bảng `background_tasks` (hoặc view hợp nhất) cho mọi việc dài không phải video: `kind`, `objectRef`, `phase`, `progress {done,total}|null`, `etaSec|null`, `action`, `state`. Các service hiện có (identity, channel scan, backdrop scan, thumbnail preview, template preview, editor export, setup, cleanup, logs) ghi vào đây.
- Bảng `notifications`: `kind` (review_ready | needs_attention | ready_to_post | task_done | action_result), `objectRef`, `createdAt`, `readAt`, `batchKey`. Gộp theo `batchKey` trong 10 giây.
- `GET /api/activity` (đang chạy + thông báo chưa đọc), `POST /api/notifications/read`.
- Desktop app: `Notification` của Electron/macOS + `app.dock.setBadge`.
- Cài đặt thông báo lưu trong settings.

### Nghiệm thu
- 3 video chuyển Chờ duyệt trong 10 giây → **một** toast “3 video chờ duyệt · Duyệt ảnh”, và một dòng trong Thông báo.
- Không có toast nào khi một video chuyển từ Trinh sát sang Đạo diễn.
- Cửa sổ ẩn, một video Cần xử lý → thông báo hệ thống; số trên biểu tượng tăng 1; bấm vào mở đúng video trên Bàn dựng.
- Trong giờ Không làm phiền: không thông báo hệ thống, nhưng lịch sử vẫn có.
- Ghi “Gói V14” vào `10-gap-closure.md`.

## Gói V15 — Kho prompt ảnh (P1, phụ thuộc V8) <!-- r7-prompts --> <a id="goi-v15"></a>

**Vì sao:** cách vẽ ảnh là thứ người dùng muốn tự viết và dùng lại giữa nhiều template. Ba kiểu ảnh cố định không đủ, còn ô mô tả nằm trong từng template thì phải chép tay khi muốn dùng lại.
- Artboard: `Library` (`tab=prompts`, `prompt=list|mine|builtin|new`) · `Template` (`section=images|cover`).
- Spec: ia-flows › W12 (Prompt ảnh), L12; data-contract §13; components › PromptCard, PromptPicker.

### Việc FE
- Thư viện có tab **Prompt ảnh** (cột trái và tab ngang, thứ tự: Template · Prompt ảnh · Video nền · Lớp phủ — tên Video nền từ đợt 22). Trang Kênh cũng thêm mục này ở cột trái.
- Danh sách: lọc Tất cả / Ảnh cảnh / Ảnh bìa, tìm theo tên hoặc nội dung; hai nhóm Ảnh cảnh và Ảnh bìa; PromptCard; ô “Prompt mới”. Đầu trang: Nhập prompt (viền) và **Prompt mới** (đỏ).
- Trang sửa prompt (thay danh sách, có “‹ Prompt ảnh” để quay lại):
  - Trái: Tên · Dùng cho · (chỉ ảnh bìa) Bố cục bìa · Prompt · Luôn giữ. Prompt ảnh cảnh không có cách đặt hay cách dùng. <!-- r9-assets -->
  - Phải: Ảnh mẫu + Vẽ thử với (chọn video và cảnh) · Template đang dùng.
  - Thanh dưới cố định: trạng thái lưu · Xoá (cam, khoá khi đang dùng, tooltip lý do) · Nhân bản · **Lưu** (đỏ).
  - Prompt có sẵn: chỉ đọc, banner giải thích, nút chính **Nhân bản để sửa**.
  - Prompt mới: Lưu khoá tới khi có tên và nội dung, ghi lý do.
- Cài đặt template: mục Ảnh cảnh có **Cách ghép ảnh vào video** (3 ô ảnh: Toàn khung / Tách nền / Cận cảnh) rồi **Prompt ảnh cảnh** (PromptPicker); mục Ảnh bìa có **Prompt ảnh bìa**; bỏ ô Mô tả phong cách ảnh. Cột trái tóm tắt tên prompt (“Đêm xanh rêu · 6 ảnh”).
- Chỗ chỉ hiển thị: card template ở Thư viện và Kênh ghi “Prompt <tên> · … · bìa <tên>”; chip Ảnh ở Bàn dựng ghi tên prompt ảnh cảnh (Gói V16).

### Việc backend
- Bảng/tệp `image_prompts` và API ở data-contract §13. Seed 7 prompt có sẵn (1 cảnh “Theo nhận diện kênh”, 6 bìa) với id cố định.
- Recipe template giữ `style` (cách ghép: full / cutout, + khung cận) và lưu `scenePromptId`, `coverPromptId` thay cho `thumbnail`, `imagePrompt`, `imagePromptMode`.
- **Pipeline ảnh không đổi:** khi video bắt đầu, app chép prompt vào các trường recipe hiện có (`imagePrompt`, `imagePromptMode` = `blend` với prompt có sẵn / `custom` với prompt của bạn, `thumbnail`) rồi chạy như cũ, nên manifest và fingerprint giữ nguyên. Mới: `coverPrompt` riêng cho ảnh bìa (hiện ảnh bìa dùng chung `imagePrompt` với ảnh cảnh).
- Vẽ thử: dùng `drawThumbnailPng` / đường vẽ ảnh cảnh hiện có với một câu chuyện đã phân tích của video được chọn; ghi việc nền (V14) và lưu ảnh làm `samples`.
- Migration: mỗi template cũ sinh một prompt “<tên template> · ảnh” nếu có `imagePrompt`; prompt trùng nội dung thì gộp. Template không có `imagePrompt` trỏ tới “Theo nhận diện kênh”; `style` giữ nguyên trong recipe; prompt bìa có sẵn chọn theo `thumbnail`.

### Nghiệm thu
- Thư viện › Prompt ảnh hiện 7 prompt có sẵn + prompt từ migration; prompt có sẵn không có nút Lưu/Xoá.
- Sửa “Đêm xanh rêu” rồi Lưu → template Mẫu chuẩn hiện nội dung mới; video đang chạy vẫn vẽ bằng nội dung cũ; video bắt đầu sau đó dùng nội dung mới.
- Xoá prompt đang dùng → bị chặn với tên template; bỏ chọn ở template rồi xoá được.
- Template chọn Cách ghép Tách nền → video vẽ ảnh tách nền như `style=cutout` hiện tại, với bất kỳ prompt ảnh cảnh nào; chọn prompt bìa “Chữ hook” → ảnh bìa có chữ hook do Đạo diễn viết.
- Ghi “Gói V15” vào `10-gap-closure.md`.

## Gói V16 — Ngoại lệ ảnh của một video (P2, phụ thuộc V15) <!-- r8-pm --> <!-- r9-assets --> <a id="goi-v16"></a>

**Vì sao:** đôi khi một video cần ảnh khác template (thử bìa khác, hoặc không vẽ ảnh) mà không phải tạo template mới. Chỉ có **một** chỗ làm việc này để không phân tán (ia-flows §0.2, L14).
- Artboard: `Main` (`promptPop=none|scene|cover`, kèm `stage`) · `MMain` (`screen=detail`, `promptPop`) · `Dialogs` (`dialog=redraw-all`).
- Spec: ia-flows L14, W5; data-contract §14; components › PromptMenu.

### Việc FE
- Chip **Ảnh <prompt cảnh>** cạnh chip Template ở Bàn dựng (điện thoại: dòng “Ảnh của video”). Khác template thì có chữ “riêng video này”.
- Tab Ảnh bìa có dòng **Dùng ảnh của bạn** (dùng `POST /api/clone/:id/cover` có sẵn); bỏ “Tải ảnh bìa của bạn” khỏi menu ⋯.
- PromptMenu hai tab Ảnh cảnh / Ảnh bìa; tab Ảnh cảnh có dòng cuối “Không tạo ảnh cho video này”. Chọn khác thì gọi `plan` và hiện giá; nút chính ghi đúng việc; Huỷ bỏ lựa chọn.
- Bỏ khỏi menu ⋯ dòng “Không tạo ảnh cho video này” (đã nằm trong chip Ảnh).
- Hộp thoại Vẽ lại cả bộ ghi prompt đang dùng + link “Đổi ở Bàn dựng”.
- **Không làm:** chip ảnh ở Video mới. (Prompt riêng cho từng ảnh ở Duyệt: chuyển sang làm ở Gói V29, đợt 33.) <!-- r33-img -->

### Việc backend
- Data-contract §14. `resolveJobRecipe` đè `scenePromptId` / `coverPromptId` / `no_images` của job lên recipe template, rồi phân giải như V15.
- `plan`: đổi prompt cảnh → vẽ lại ảnh cảnh đã vẽ, ảnh chưa vẽ dùng luôn prompt mới; đổi prompt bìa → 1 ảnh; không tạo ảnh → 0 ảnh. Kèm `rebuild` khi video đã dựng. Video còn Chờ lượt: không tốn gì.

### Nghiệm thu
- Bàn dựng, video đang vẽ 4/6 ảnh: đổi Đêm xanh rêu → Đèn lồng phố cổ → giá “Vẽ lại 4 ảnh cảnh đã vẽ; 2 ảnh còn lại vẽ luôn theo prompt mới. Tốn 4 ảnh”.
- Chọn “Không tạo ảnh cho video này” → “Đổi · không tạo ảnh”, không tốn ảnh; video dựng từ nền, hiệu ứng, phụ đề.
- Đổi prompt bìa → chỉ ảnh bìa vẽ lại; template không đổi.
- Ghi “Gói V16” vào `10-gap-closure.md`.

## Gói V17 — Kết nối AI và Trợ lý AI (P2, phụ thuộc V7) <!-- r8-ai --> <a id="goi-v17"></a>

**Vì sao:** hiện app chỉ vẽ bằng gói ChatGPT (Codex) hoặc Grok và mọi chữ đi qua Codex. Người dùng muốn dùng thêm khoá API (Gemini, OpenRouter…) hoặc model chạy trên máy (Ollama), và chọn model cho từng trợ lý như ở Noverse.
- Artboard: `Settings` (`section=connect|agents`, `conn=list|add|added|models|manual|failed`, `grok=signed|code|out`, `ai=none|director|artist`) · `MSettings` (`screen=connect|agents`, `sheet=none|add|models|director|artist`).
- Spec: ia-flows W13, L6, L15; data-contract §15; components › ProviderCard, ModelCatalog, ModelPicker.

### Việc FE
- Cột trái Cài đặt: Công cụ trên máy · **Kết nối AI** · **Trợ lý AI** · Tài khoản nền tảng (tên từ đợt 22, Gói V23). Mục Vẽ ảnh cũ bỏ; link cũ `?section=images` chuyển sang `agents`.
- Kết nối AI: ProviderCard cho từng nhà cung cấp; ModelCatalog mở trong thẻ; thẻ thêm nhà cung cấp; thẻ lỗi; thanh Lưu dính đầu mục.
- Trợ lý AI: bốn trợ lý; ModelPicker (popover) cho Đạo diễn và Hoạ sĩ; lưới “Vẽ ảnh bằng”; công tắc + chuỗi dự phòng.
- Dòng “Đăng nhập ChatGPT” ở Công cụ trên máy dẫn sang Kết nối AI. Banner hết hạn mức ở Hàng chờ có nút “Thêm nguồn vẽ dự phòng”.

### Việc backend
- Data-contract §15. Bảng `ai_providers`; khoá API lưu trong keychain của máy (hoặc mã hoá bằng khoá máy), API chỉ trả `keyHint`.
- Danh mục model: gọi endpoint liệt kê model của từng nhà cung cấp (OpenAI-compatible `/models`, Gemini `models.list`, Ollama `/api/tags`, Codex `debug models`). Lưu `catalogAt`. `models/check` gọi thử một lượt ngắn.
- Adapter gọi model theo việc: `text` (metadata, scenes, branding), `vision` (mô tả ảnh có khung mẫu), `image` (vẽ). Codex và Grok giữ đường hiện có; thêm adapter OpenAI-compatible, Gemini, Ollama.
- `clone.imageProvider` → `agents.artist.draw`, `clone.imageFallback` → `agents.artist.fallbackOn`; thêm `drawFallback[]`. Snapshot model của mỗi step vào job khi step bắt đầu.
- Kiểm tra khả năng: từ chối gán model thiếu `text`/`vision`/`image` (`422 capability_missing`). Xoá nhà cung cấp đang dùng → `409 in_use { agents }`.

### Nghiệm thu
- Thêm OpenRouter bằng khoá đúng → thẻ mới đầu danh sách, nhắc chọn model; chọn 2 model rồi Lưu → hai model hiện trong ô chọn model của Trợ lý AI.
- Khoá sai → thẻ cam “Không lấy được danh sách model… (401)”, có Đổi khoá · Thử lại · Thêm theo tên.
- Hoạ sĩ: qwen3:14b (chỉ chữ) hiện mờ ở “Model viết mô tả ảnh” với lý do “Không đọc được ảnh”.
- Nguồn vẽ ChatGPT hết hạn mức, dự phòng Grok → Gemini: ảnh tiếp theo vẽ bằng Grok; Grok cũng hết → Gemini; video không vào Cần xử lý.
- Ghi “Gói V17” vào `10-gap-closure.md`.

## Gói V18 — Vì sao? và Bản tin (P1, phụ thuộc V1, V14) <!-- r16-agent --> <a id="goi-v18"></a>

**Vì sao:** người dùng chỉ giao việc cho máy khi hiểu máy quyết định thế nào, và cần biết máy đã làm gì khi họ vắng.
- Artboard: `Main` (`agent=recap|why`), `Explore` (`preview=true`, `why=true`), `MMain` (`screen=recap`, `agent=why`), `MCompVideo` (`sheet=why`), `Activity` (`view=settings` › Trợ lý chủ động).
- Spec: ia-flows §9.1, §9.6; data-contract §17, §22; components › ReasonPopover, RecapCard (05-kit.md › Trợ lý).

### Việc FE
- `ReasonPopover` (máy tính) / sheet (điện thoại) mở từ link **Vì sao?** ở điểm nổi trội, Nội dung, điểm ảnh, quyết định Tự lấy.
- Bản tin ở cột giữa Bàn dựng: tự mở theo §9.6, nút **Bản tin** đầu cột trái, **Vào việc**, ô **Nhắn trợ lý · mọi kênh**. Điện thoại: thẻ đầu Bàn dựng → màn con.

### Việc backend
- `reason` cho điểm nổi trội, Tự lấy, metadata (§17). `GET /api/recap`, `POST /api/recap/seen`, `lastSeenAt` của máy.

### Nghiệm thu
- Mở app sau 3 giờ có việc mới → Bản tin hiện, câu đầu đúng số video Chờ bạn ở cột trái; không có gì mới → không tự mở.
- **Vì sao?** ở điểm ×4,2 ghi trung vị và nguồn số; không có số thì không hiện link.
- Ghi “Gói V18” vào `10-gap-closure.md`.

## Gói V19 — Đề xuất và Quy tắc đã học (P2, phụ thuộc V18, V4) <!-- r16-agent --> <a id="goi-v19"></a>

**Vì sao:** trợ lý chủ động nói việc nên làm, và lần sau không mắc lại lỗi bạn đã sửa.
- Artboard: `Main` (`agent=suggest|learn`), `Channel` (`tab=settings` › Quy tắc đã học, `tab=videos` › Trợ lý nhận ra), `Review` (`state=learn`), `MMain` (`screen=list`, `agent=learn`), `MChannel` (`tab=settings`), `MReview` (`state=learn`).
- Spec: ia-flows §9.2, §9.3; data-contract §18, §19; components › SuggestionCard, LearnedRuleRow.

### Việc FE
- Nút **Đề xuất N** đầu cột trái Bàn dựng mở nhóm thẻ gọn (tối đa 3 + “Xem cả N trong Bản tin”); 3 thẻ trong Bản tin. Điện thoại: thẻ cuộn ngang dưới thẻ Bản tin.
- Gợi ý **Lưu thành quy tắc** ngay dưới ô Nội dung vừa sửa (lần thứ 3) và dưới ô góp ý ở Duyệt.
- Hồ sơ kênh › Khi viết…: danh sách **Quy tắc đã học** (công tắc, nguồn, Xem ví dụ, Xoá). Thư viện › Prompt ảnh: **Ghi chú đã học** cuối nội dung.

### Việc backend
- Bảng `suggestions`, `learned_rules`; bộ sinh đề xuất 7 loại (§18); phát hiện kiểu sửa (§19); step `metadata` và prompt ảnh nhận quy tắc.

### Nghiệm thu
- Bỏ phần trong ngoặc ở cuối tiêu đề của 3 video Midnight Tales → lần thứ 3 hiện “Lưu thành quy tắc”; lưu → video kế tiếp không có phần trong ngoặc; quy tắc nằm ở Hồ sơ kênh với nguồn “Học từ 3 lần bạn sửa”.
- Bỏ qua một đề xuất → không hiện lại trong 7 ngày; tối đa 5 đề xuất mở; đề xuất không bao giờ toast.
- Ghi “Gói V19” vào `10-gap-closure.md`.

## Gói V20 — Mức tự chủ và điểm ảnh (P2, phụ thuộc V3, V4) <!-- r16-agent --> <a id="goi-v20"></a>

**Vì sao:** thay công tắc Tự duyệt bật/tắt bằng mức tin cậy có lưới an toàn, để người dùng giao dần việc cho máy.
- Artboard: `Channel` (`tab=sources`, `autonomy=careful|balanced|auto|custom|confirm`), `Review` (`score=true`), `CreateChannel`/`MCreate` (khối Cách kênh chạy), `MChannel` (`tab=sources`, `autonomy`), `MReview` (`score=true`).
- Spec: ia-flows §9.4, L7; data-contract §20; components › AutonomyControl, ScoreBadge.

### Việc FE
- Kênh › Nguồn & tự động: thẻ **Mức tự chủ** — segmented 3 mức + 4 dòng chọn; “Tuỳ chỉnh” khi khác mức sẵn; xác nhận khi chọn Tự chạy hoặc Tự duyệt hết. **Ưu tiên kênh** Đều đặn / Lượt xem.
- Duyệt: nhãn điểm ở góc mỗi ảnh khi có điểm; ảnh dưới ngưỡng viền cam; dòng tóm tắt trên dải ảnh có nút tới ảnh đó; cột phải **Điểm của Kiểm định** + **Vì sao?**.
- Tạo kênh: dòng **Mức tự chủ** (3 mức, mặc định Cân bằng) thay hai công tắc Tự lấy video và Tự duyệt ảnh. Hộp `auto-approve` cũ thành `Dialogs` · `dialog=autonomy`.
- Cài đặt › Kết nối AI: nhà cung cấp trả theo ảnh ghi “máy tự tiêu tối đa N ảnh/ngày” (D7); Cài đặt thông báo có mục Trợ lý chủ động (Bản tin, loại đề xuất).

### Việc backend
- `owner_channels.autonomy`, `priority`, migration từ `autoApprove`; `image.score`; `retry_once`, `fix_then_ask`; `dailyPaidImageLimit`.

### Nghiệm thu
- Kênh ở Tự chạy, video có 7 ảnh ≥ 85 → không vào Chờ duyệt; thông báo “Tự duyệt theo điểm” có Xem; có 1 ảnh 72 → cả video Chờ duyệt, ảnh đó viền cam, dòng tóm tắt “cảnh 4 được 72”.
- Lỗi tải nguồn ở Cân bằng → tự thử lại 1 lần; vẫn lỗi → Cần xử lý với dòng “Đã tự thử lại lúc 02:14”.
- Không có đường nào để máy tự đăng, tự xoá hay đổi template.
- Ghi “Gói V20” vào `10-gap-closure.md`.

## Gói V21 — Hiệu quả sau đăng và Ưu tiên lượt xem (P2, phụ thuộc V20, quyết định D5) <!-- r16-agent --> <a id="goi-v21"></a>

**Vì sao:** khép vòng: biết video nào hiệu quả để chọn video, template và cách viết tốt hơn.
- Artboard: `Channel` (`tab=videos`, `filter=posted`), `MChannel` (`tab=videos`).
- Spec: ia-flows §9.5; data-contract §21; components › InsightRow.

### Việc FE
- Đánh dấu đã đăng có ô dán link YouTube (tuỳ chọn). Đã đăng hiện lượt xem 7 ngày + “×1,4 so với trung vị”.
- Đầu tab Video: **Trợ lý nhận ra** (tối đa 3 nhận xét, nhãn Sơ bộ khi < 20 video, nút Áp dụng).

### Việc backend
- Đọc số liệu ở mốc 24 giờ / 7 ngày / 28 ngày; tính nhận xét theo §9.5; Ưu tiên lượt xem trong Tự lấy và Template luân phiên.

### Nghiệm thu
- Kênh có 9 video đã đăng có số → hiện tối đa 3 nhận xét gắn “Sơ bộ”; dưới 6 → “Cần thêm N video có số liệu”.
- Áp dụng nhận xét tiêu đề → tạo quy tắc đã học có nguồn “Từ số liệu 9 video”.
- Ghi “Gói V21” vào `10-gap-closure.md`.

## Gói V22 — Thư mục lưu dự án (P2, phụ thuộc V7, V14) <!-- r17-storage --> <a id="goi-v22"></a>

**Vì sao:** assets của video (video gốc, ảnh, MP4, project editor) chiếm phần lớn dung lượng (81 GB ở dữ liệu mẫu) và hiện nằm cứng trong `YTF_DATA_DIR`. Người dùng cần chọn thư mục khác, thường là ổ ngoài.
- Artboard: `Settings` (`section=storage`, `store=here|pick|lowspace|cloud|moving|done|donenew|offline`), `MSettings` (`screen=storage`, `store=here|pick|lowspace|moving|done|offline`).
- Spec: ia-flows W13 › Lưu trữ, L12; data-contract §23.

### Việc FE
- Thẻ **Thư mục lưu dự án** đầu mục Lưu trữ: tên ổ, loại ổ, chỗ trống, đường dẫn (mono, cắt giữa kèm `title`), **Mở trong Finder**, **Đổi thư mục…**.
- Chọn: ô đường dẫn + **Chọn thư mục…** (Electron `dialog.showOpenDialog`, `openDirectory` + `createDirectory`), gợi ý ổ đang cắm, 2 lựa chọn dữ liệu đang có, danh sách kiểm tra, nút sáng; không đạt thì khoá nút và nói lý do ngay cạnh.
- Đang chuyển: thanh tiến độ theo nhóm, Huỷ chuyển; cũng là một dòng trong Việc nền. Xong: banner xanh + Xoá bản cũ ngay + Chuyển về. Ổ bị rút: banner cam + thông báo Cần xử lý.
- Điện thoại: màn Lưu trữ (trước bấm không có gì), sheet chọn ổ đang cắm.

### Việc backend
- Thêm `YTF_PROJECTS_DIR` (mặc định `YTF_DATA_DIR/projects`; với dữ liệu cũ là chính `YTF_DATA_DIR`) cho `clone-jobs/`, `editor-workspace/` (thay `YTF_EDITOR_WORKSPACE`), `export-requests/`, media của `owner-channels/`. Giữ ở `YTF_DATA_DIR`: `app.db`, `bin/`, `python/`, `models/`, `envs/`, `chrome-profiles/`, `cache/`, `process-groups/`.
- `desktop/main.mjs` đọc `storage.json` trong `userData` trước khi khởi động server và truyền `YTF_PROJECTS_DIR`; đổi thư mục = ghi `storage.json` rồi khởi động lại server trong app (không thoát app).
- DB: đường dẫn tệp lưu **tương đối** với `YTF_PROJECTS_DIR`. Không có migration dữ liệu yt-factory (#9). <!-- r32 -->
- Chuyển: tạm dừng hàng chờ (`paused_at` như V0), chép theo nhóm có kiểm tra kích thước + hash mẫu, đổi `storage.json` khi chép xong, chạy tiếp hàng chờ; cùng ổ thì `rename`. Bản cũ giữ 7 ngày (`storage.json.previous` + hạn xoá).
- Kiểm tra trước: chỗ trống ≥ dữ liệu × 1,1 (chỉ video mới: ≥ 20 GB), quyền ghi (ghi thử tệp tạm), không nằm trong `~/Library/CloudStorage`, iCloud Drive hay Dropbox, hệ tệp (FAT32 từ chối: tệp > 4 GB).
- Theo dõi ổ: thư mục biến mất → tạm dừng hàng chờ, `needs_attention` “Không thấy ổ …”; thư mục xuất hiện lại → tự chạy tiếp.

### Nghiệm thu
- Chọn `/Volumes/Samsung T7` → thư mục “Viderse Projects” được tạo; chuyển 81 GB xong thì video đang làm chạy tiếp, mở được project editor cũ; Finder mở đúng thư mục mới.
- Chọn thư mục trong Google Drive → nút khoá, lý do “Thư mục đồng bộ đám mây: không dùng được”.
- Rút ổ khi máy đang chạy → không hỏng video nào; Cần xử lý hiện trong 10 giây; cắm lại → tự chạy tiếp.
- Huỷ giữa chừng → thư mục cũ nguyên vẹn, `storage.json` không đổi.
- Ghi “Gói V22” vào `10-gap-closure.md`.

## Gói V23 — Video nền và Tài khoản nền tảng (P2, phụ thuộc V8, V17) <!-- r22-sources --> <a id="goi-v23"></a>

**Vì sao:** nguồn bên ngoài đang rải ở ba chỗ (Thư viện › Kênh nền chỉ nhận kênh; clip cố định dán riêng trong từng template; Cài đặt › Tài khoản phụ chỉ có cookie YouTube + Bilibili) và code chốt cứng hai nền tảng (`BACKDROP_PLATFORMS = twitch, bilibili`). Trạng thái từng nền tảng (`backdrop_platform_state`: tạm chặn tới, số lần lỗi) chưa hiện ở đâu. Sẽ có thêm nền tảng.
- Quy ước: **thêm kênh hay video ở Thư viện › Video nền** (asset dùng chung); **đăng nhập và trạng thái ở Cài đặt › Tài khoản nền tảng** (chỉ nằm trên máy này).
- Artboard: `Library` (`tab=backdrops`, `nguon=channels|videos|adding|addvideo|unsupported`, `kenh=slowroads|nightdrivefm|bilibili|lofilanes`) · `Template` (`section=backdrop`, `picker=none|add|choose|video`, `fewClips`) · `Settings` (`section=accounts`, `bili`, `yt=ok|bot`) · `MSettings` (`screen=accounts`, `bili`). Bố cục Thư viện và Template theo đợt 25.
- Spec: ia-flows W12 › Video nền, W13 › Tài khoản nền tảng; data-contract §24; copy (Video nền, Video lẻ, Tài khoản nền tảng, `unsupported_platform`).

### Việc FE
- Thư viện: tab **Video nền** (thay Kênh nền). Ô dán link ở trên: gọi `POST /api/sources/parse` khi dán, ghi kết quả dưới ô; nút chính “Thêm”. Dưới là **SourceRail** (Kênh nền chia theo nền tảng sinh từ `GET /api/platforms` · Video lẻ) và nội dung nguồn đang chọn: <!-- r25-backdrops -->
  - Kênh: đầu trang (Làm mới, ⋯), **Dùng ở** từ `usedBy`, banner khi nền tảng bị chặn, **BackdropGrid** từ `GET /api/backdrop/channels/:id/videos` (lọc loại, Hiện thêm theo `cursor`, nhãn tick theo `chosenBy`). Route gợi ý `#/library/backdrops/<channelId>` để quay lại đúng kênh.
  - Video lẻ: lưới thẻ (đoạn dùng + **Chọn đoạn khác** dùng lại thanh chọn đoạn của Gói V8, dùng ở, tiến độ tải); bỏ khỏi thư viện có chặn.
- Template › Nền & hiệu ứng: **BackdropSourceList** + **AttachBackdropDialog** (một hộp thoại cho cả thêm kênh, thêm video lẻ và **Chọn video** của kênh đã gắn) + **BackdropMixBar** + **Cách cắt clip** gập. Bỏ khối Clip cố định, hai popover và ô dán clip cố định cũ. Code yt-factory hiện có các mảnh tương ứng (`backdrop-channels-block.tsx`, `backdrop-video-picker.tsx`, `add-backdrop-channel-dialog.tsx`) — gộp theo bố cục mới.
- Cài đặt: mục **Tài khoản nền tảng** = danh sách thẻ sinh từ `GET /api/platforms`; YouTube giữ chọn hồ sơ trình duyệt, Bilibili giữ ô SESSDATA. Điện thoại: màn mới cùng thẻ.
- Đổi chữ: “Kênh nền” (tên tab) → “Video nền”; “Tài khoản phụ” → “Tài khoản nền tảng”; link cũ `?tab=backdrops` giữ nguyên.

### Việc backend
- **Danh sách nền tảng một chỗ:** thay mảng `BACKDROP_PLATFORMS` bằng registry `src/server/platforms/` — mỗi nền tảng khai báo `id`, `label`, `uses` (`competitor` | `backdrop`), loại video (`clip | highlight | vod | video`), `targetHeight` (Twitch 480, Bilibili 1080, YouTube 360), cách đăng nhập (`none | browser_cookie | cookie_string`), hàm nhận link kênh và link video (gộp `parseBackdropChannelLink` và `parseAssetLink`). Thêm nền tảng = thêm một mục registry + bộ đọc danh sách.
- **Video lẻ vào thư viện:** bảng `backdrop_videos` (id `<platform>:<videoId>`, url, title, durationSec, `segmentFromSec`/`segmentToSec` ≤ 600 giây, đường dẫn tệp, height, bytes, state). Template lưu `fixedVideoIds` thay cho clip cố định nhúng trong recipe; migration chuyển clip cố định cũ của từng template thành `backdrop_videos` (trùng url thì gộp).
- `POST /api/sources/parse { input }` → loại link; `b23.tv` mở một lần như hiện tại. Nền tảng chưa hỗ trợ trả `unsupported_platform` kèm tên miền dễ đọc.
- `GET /api/platforms` → trạng thái từng nền tảng từ `backdrop_platform_state` (YouTube: từ lần tải gần nhất lỗi `cookies_failed`), số nguồn đang dùng.
- Yêu cầu hỗ trợ nền tảng: ghi `platform_requests { host, count, lastAt }`; gửi kèm khi người dùng **Báo lỗi** (Cài đặt › Báo lỗi). Cần chốt: có gửi thẳng về đâu không (D8).

### Nghiệm thu
- Dán `https://www.twitch.tv/rainyarchive` → “Link kênh Twitch · đang đọc…”, kênh lên đầu bảng Kênh. Dán `https://b23.tv/…` của một video → vào nhóm Video lẻ, trạng thái “Đang tải 45%”.
- Dán `https://www.douyin.com/user/…` → báo chưa hỗ trợ Douyin, có nút gửi yêu cầu; bấm hai lần chỉ ghi một yêu cầu.
- Bilibili bị chặn → Cài đặt hiện “Tạm chặn tới 02:10” + banner; Thư viện hiện banner cùng giờ; tới giờ tự quét lại, không cần bấm gì.
- Template cũ có clip cố định → sau migration thấy clip đó ở Video lẻ, “Dùng ở” ghi đúng template; nền của video mới vẫn có clip đó.
- Thư viện › Video nền › bấm từng kênh trong danh sách trái → thấy lưới video của kênh đó, lọc Clip/Highlight đổi số đúng, “Hiện thêm” tải tiếp; video template đã chọn có nhãn tick.
- Template › **Gắn kênh hoặc video** → chọn lofilanes → Ngẫu nhiên → **Gắn vào template**: dòng mới có nhãn Ngẫu nhiên, thanh Nền của một video thêm phần lofilanes. Chọn video của nightdrivefm → bỏ hết tick → nút Lưu khoá kèm câu “Chọn ít nhất 1 video…”.
- Thanh Nền của một video khớp video làm ra: với Mẫu chuẩn (2 clip cố định 2:50, slowroads ngẫu nhiên, nightdrivefm 4 video, mỗi clip 60 giây) ra 2 clip cố định + 36 clip slowroads + 4 clip nightdrivefm.
- Ghi “Gói V23” vào `10-gap-closure.md`.

## Gói V24 — Đăng nhập Bilibili bằng mã QR (P2, phụ thuộc V23) <!-- r24-biliqr --> <a id="goi-v24"></a>

**Vì sao:** cookie dán tay hết hạn sau vài tuần và người dùng phải tự lấy SESSDATA từ trình duyệt. yt-factory đã có đăng nhập bằng mã QR và tự gia hạn (`yt-factory`: `src/server/backdrop/bilibili-qr-login.ts`, `bilibili-login.ts`, `src/app/settings/bilibili-cookie-controls.tsx`); gói này đưa vào thiết kế và verse.
- Artboard: `Settings` (`section=accounts`, `bili=blocked|saved|manual|expired|empty`, `qr=none|loading|waiting|scanned|expired|rejected|error|success`) · `MSettings` (`screen=accounts`, `bili`, `qr`).
- Spec: ia-flows W13 › Bilibili; data-contract §24.1; components › QrLoginDialog; copy (Đăng nhập bằng mã QR, Cookie dán tay, mã lỗi QR).

### Việc FE
- Thẻ Bilibili: phần đăng nhập 4 dạng (QR + tự gia hạn · cookie dán tay · hết hạn · chưa đăng nhập) như artboard. **Dán cookie thủ công** gập sẵn.
- `QrLoginDialog` (máy tính) và sheet (điện thoại): gọi `POST …/login/qr`, hỏi trạng thái mỗi 2 giây, đếm ngược từ `expiresAt`, đổi bước và ô trạng thái theo `status`. Thành công → **Xong** đóng hộp thoại, làm mới `GET /api/platforms`, toast “Đã đăng nhập Bilibili: <tài khoản>”.
- Bấm vào mã không làm gì (trên artboard bấm mã để xem trạng thái kế tiếp, chỉ để demo).
- Điện thoại: dòng **Mở app** mở `loginUrl` nếu thử trên máy thật mở được app 哔哩哔哩; không thì thay bằng câu “Quét bằng điện thoại khác, hoặc mở Cài đặt trên máy tính” (#6). <!-- r32 -->

### Việc backend
- Port `bilibili-qr-login.ts` sang Go trong verse (`apps/viderse/internal/...`): tạo mã (`passport.bilibili.com/x/passport-login/web/qrcode/generate`, kèm `buvid3/4` nếu lấy được), hỏi mã (86101 chờ · 86090 đã quét · 86038 hết hạn · 0 xong · khác = từ chối), lưu cookie + refresh token, gia hạn (`cookie/info` → `correspond` → `cookie/refresh` → `confirm/refresh`), kiểm tra tài khoản.
- Thành công thì xoá giờ chờ bị chặn của Bilibili. Một khoá cho mọi thao tác đăng nhập. Tệp quyền 600.
- API theo §24.1. `startBilibiliQrLoginAction` của yt-factory hiện không trả `loginUrl` về giao diện; bản verse trả (hỏi đáp #7).

### Nghiệm thu
- Chưa đăng nhập → **Đăng nhập bằng mã QR** → quét bằng app 哔哩哔哩 → ô trạng thái đổi “Đã quét…” trong ≤ 2 giây → xác nhận trên điện thoại → “Đã đăng nhập: <tài khoản>”; thẻ hiện nhãn Mã QR, “Tự gia hạn mỗi ngày”.
- Đang “Tạm chặn tới 02:10” mà đăng nhập xong → nhãn về Sẵn sàng, kênh Bilibili được quét lại ngay.
- Để mã quá 3 phút → “Mã đã hết hạn”, **Tạo mã mới** ra mã khác. Bấm Huỷ trên điện thoại → “Bilibili chưa chấp nhận đăng nhập”. Rút mạng → “Chưa kết nối được Bilibili”.
- Đóng hộp thoại giữa chừng → không còn request hỏi mã.
- Giả lập gia hạn lỗi → thẻ viền cam “Cần đăng nhập lại”, chấm cam ở mục bên trái; quét kênh vẫn chạy không đăng nhập.
- Dán cookie bilibili.tv → báo không đăng nhập được, không lưu.
- Ghi “Gói V24” vào `10-gap-closure.md`.

## Gói V25 — Lưu trữ khi chạy trên máy chủ: kho S3 dùng chung Garage (P2, phụ thuộc V22, Noverse Gói AN) <!-- r29-s3 --> <a id="goi-v25"></a>

**Vì sao:** Viderse chạy trên Mac mini (`vid.aarverse.studio`), đăng nhập chung với Noverse. Tệp dự án đang nằm trên ổ máy chủ (V22). Gói này thêm Kho S3; Viderse và Noverse dùng chung máy chủ Garage nhưng mỗi app một kho và một khoá. Màn phải giống Noverse Gói AN để người quản trị cả hai app thấy cùng một kiểu.
- Artboard: `Settings` (`section=storage`, `store=server|s3pick|s3form|s3testing|s3testok|s3testfail|s3envlocked|s3nokey|s3confirm|s3moving|s3moveerr|s3active|s3down`, `sprov=garage|r2|aws|other`, `sfail=key|bucket|perm|net|clock|cors|reach`, `skey=new|saved`, `role=admin|member`, `usage=full|partial`; `clean=preview` với `store=s3active` là Dọn khi dùng kho S3) · `MSettings` (`screen=storage`, `store=server|s3active|s3moving|s3down`, `usage`). `usage=partial` là bản tạm khi chỉ Thư viện có số. Các giá trị V22 giữ nguyên.
- Spec: ia-flows W13 › Lưu trữ › Chạy trên máy chủ; data-contract §23.1; components › Lưu trữ trên máy chủ; copy › Lưu trữ trên máy chủ.

### Trả lời câu hỏi mở của prompt
1. **Đang chuyển sang S3 có cho sửa video không?** Có. Video đang làm vẫn chạy, Bàn dựng và Duyệt sửa bình thường; tệp mới ghi vào chỗ cũ tới khi chuyển xong rồi được chép nốt. Hộp xác nhận và thẻ đang chuyển nói rõ điều này. Khác V22 (phải khoá vì đổi thư mục gốc trên máy).
2. **Dòng “Dùng chung máy chủ lưu trữ với Noverse”:** bỏ, ghi GAP. App không biết kho của app kia; ghi chú tay dễ sai mà không ai sửa. Thay bằng một dòng ở form Garage: “Nên dùng kho và khoá riêng, không dùng chung kho với Noverse.” và một mẹo ở cột phải.

### Việc FE
- Mục Lưu trữ khi `GET /api/storage/server` có dữ liệu: đầu trang “Cài đặt · máy chủ”; quản trị viên thấy thẻ nơi lưu, mời chuyển (khi `kind = disk`), Dung lượng theo nhóm, Bộ nhớ đệm làm việc, Hoạt động từ lần khởi động, Chỗ cũ; thành viên chỉ thấy Dung lượng theo nhóm và Dọn.
- `StorageProviderPicker` → `S3ConfigForm` (ô theo nhà cung cấp, *Lấy ở đâu?*, công tắc http, Cách gửi tệp, Địa chỉ kho cho trình duyệt) → `ConnectionTest` → thanh lưu → hộp xác nhận → `MigrationCard`. Sửa ô nào thì xoá kết quả kiểm tra. Một nút sáng mỗi lúc.
- Kiểm tra với `delivery = redirect`: sau khi server trả `probeUrl`, trình duyệt `fetch(probeUrl, { headers: { Range: "bytes=0-1023" } })`; lỗi CORS hay lỗi mạng → gửi `probe` để server dịch thành `cors_blocked` / `browser_unreachable`.
- `s3envlocked`: mọi giá trị chỉ đọc, không có Đổi kho; `s3nokey`: banner vàng, Lưu khoá. Khối “Chi tiết cho người quản trị máy chủ” gập sẵn là chỗ duy nhất có tên biến, tệp hay `key=value`.
- Mất kết nối: banner ở mục Lưu trữ, `AdminStorageStrip` ở đầu mọi trang (quản trị viên), thông báo Cần xử lý; thành viên thấy một dòng thông tin.
- Điện thoại: hàng Lưu trữ theo trạng thái; trang chỉ đọc + Kiểm tra kết nối; đổi kho chỉ trên máy tính.
- Màu: lỗi dùng cam `#FF9A5C` của Viderse (prompt ghi “đỏ”, nhưng Viderse không có đỏ cho lỗi); không thêm màu mới. Biểu tượng mới `i-cloud`, `i-drive`, `i-lock` nằm trong CSS riêng của board (data-URI nét 1.5) cho tới khi vào bộ icon.

### Việc backend
- Lớp lưu trữ chung cho ổ máy chủ và S3 (đọc theo đoạn để tua MP4 ở cả hai cách gửi). Có thể dùng chung `blobstore` của Noverse (`apps/noverse/internal/blobstore/s3.go`, `bootstrap/storage.go`).
- Biến môi trường đúng tên Noverse (§23.1, hỏi đáp #49), dùng chung bộ đọc cấu hình lưu trữ; giá trị nào đặt bằng biến thì `sources[k] = "env"` và không cho sửa. Ô Địa chỉ kho cho trình duyệt = `S3_PRESIGN_ENDPOINT`.
- Khoá bí mật mã hoá bằng `secretbox` như khoá API (`<thư mục cấu hình>/secret.key`, 0600). `encryptionKey.ok = false` khi thư mục cấu hình chỉ đọc, tệp sai quyền hoặc tệp hỏng; trả `path` và `issue` để FE ghi khối chi tiết.
- Dung lượng: Thư viện (lớp phủ, video nền, ảnh kênh) đếm ngay; bốn nhóm còn lại trả `null` tới khi có luồng dựng video.
- Bộ nhớ đệm làm việc: dựng video thì tải tệp cần dùng từ kho về, dựng xong đẩy kết quả lên kho; giới hạn dung lượng, bỏ tệp cũ nhất trước, không bỏ tệp của video đang dựng.
- Đếm `OpStat` trong bộ nhớ từ lần khởi động. Chuyển dữ liệu chạy nền, chép rồi đọc lại từng tệp (so kích thước và hash), tạm dừng/tiếp tục/huỷ/thử lại; chỗ cũ không xoá.
- Thông báo và `storageDown` theo §23.1; tự thử lại kết nối mỗi phút, kết nối lại thì bỏ hold.

### Nghiệm thu
- Ổ máy chủ → *Chọn kho S3* → Garage → điền `http://garage:3900`, `viderse-media`, khoá → *Kiểm tra kết nối*: 6 bước đạt, “Tạo link ký” ghi “không cần” (qua máy chủ) → *Lưu…* → hộp “Chuyển 412 tệp · 18,6 GB sang Garage?” → *Bắt đầu chuyển*: việc nền chạy, mở Bàn dựng vẫn sửa được video và lưu được.
- Tắt mạng giữa chừng → thẻ chuyển “3 tệp lỗi”, bảng Nhóm · Video · Lý do; app vẫn dùng ổ máy chủ; *Thử lại 3 tệp lỗi* xong thì đổi kho, thẻ Chỗ cũ ghi đúng số tệp và dung lượng.
- R2, Trực tiếp từ kho, chưa có CORS → bước “Trình duyệt tải được (CORS)” lỗi, khối lỗi có đoạn JSON điền sẵn `https://vid.aarverse.studio`; dán vào R2 rồi *Kiểm tra lại* → đạt. *Chuyển sang gửi qua máy chủ* → kiểm tra lại đạt mà không cần CORS.
- Garage, Trực tiếp, địa chỉ nội bộ, không điền Địa chỉ kho cho trình duyệt → “Trình duyệt không tới được địa chỉ kho”.
- Tua một MP4 30 phút ở cả hai cách gửi: nhảy tới phút 25 trong ≤ 2 giây (đọc theo đoạn).
- Đặt kho bằng biến môi trường → mọi giá trị chỉ đọc, không có Đổi kho, *Kiểm tra kết nối* vẫn chạy.
- Dừng Garage → trong ≤ 1 phút: banner mục Lưu trữ, dòng đầu trang cho quản trị viên, thông báo Cần xử lý; video đang làm tạm dừng; bật lại thì tự chạy tiếp. Thành viên chỉ thấy dòng thông tin.
- Thành viên mở Lưu trữ → không thấy nơi lưu, bộ nhớ đệm, hoạt động; `GET /api/storage/server` không trả `storage`.
- Chưa có luồng dựng video → Dung lượng theo nhóm: Thư viện có số, bốn nhóm còn lại ghi “chưa có số”, tổng ghi “Đã đếm …” (như `usage=partial`).
- Huỷ chuyển giữa chừng rồi chuyển lại → tệp đã chép không chép lại (tốc độ ở đầu lần sau tăng vọt, số “Đã chép” nhảy ngay).
- Thư mục cấu hình chỉ đọc → `s3nokey`, khối chi tiết ghi đúng đường dẫn `secret.key`; cấp quyền ghi, khởi động lại → lưu được.
- Không màn nào hiện `key=value`, tên biến hay mã lỗi thô ngoài khối chi tiết.
- Ghi “Gói V25” vào `10-gap-closure.md`.

## Gói V26 — Một tài khoản cho Noverse và Viderse (P1, phụ thuộc đăng nhập chung của Noverse) <!-- r30-sso --> <a id="goi-v26"></a>

**Vì sao:** hai app dùng chung kho tài khoản và một phiên. Code Viderse đã có các màn xác thực và `#/account` nhưng chưa có thiết kế; câu chữ hiện tại nói sai (đăng nhập ghi “tài khoản quản trị viên đã tạo lúc thiết lập”), chưa có lối sang app kia, quản trị viên không biết cấp quyền ở đâu.
- Artboard: `Auth` (`screen=login|denied|password|setup|down|account|menu`, `kicked`, `err`, `role`, `other`, `app`, `step`, `down`, `pwerr`) · `MAuth` (cùng tweak, `screen=account`, `app=noverse`, `sheet=password`) · `Settings` (cột trái › Ở trang khác › *Thành viên · ở Noverse*).
- Spec: ia-flows W0; data-contract §25; components › AccountMenu; copy › Đăng nhập và tài khoản. Phía Noverse: `apps/noverse/docs/design/v1/09-auth-onboard.md` §12, `07-copy.md`, `21-dev-prompt.md` › Gói AO.

### Việc FE
- `AuthShell`, `LoginScreen`, `AccessDeniedScreen`, `ChangePasswordScreen`, `SetupScreen`, `ServerDownScreen`, `AccountScreen` theo artboard; câu theo 06-copy (thay `loginSub`, `accessDenied`, `sharedLogoutNote`, `setup.codeHint`).
- Đăng nhập: khối lý do theo `reason`/`app` của 401 hoặc `?bye=1`, `?pw=1`; ẩn khi không có.
- Chưa được cấp quyền: *Mở Noverse* khi `apps` có Noverse (nút sáng), không thì *Thử lại* sáng; *Chép lời nhờ quản trị viên cấp quyền*.
- Đầu trang: avatar mở `AccountMenu` thay vì đi thẳng `#/account`. Điện thoại: avatar vẫn mở `#/account`, trang có nhóm Ứng dụng.
- `AccountScreen`: thêm Ứng dụng của bạn, Thành viên (quản trị viên), Thiết bị đang đăng nhập, Đăng xuất có câu nói cả hai app.
- Cài đặt › Ở trang khác: *Thành viên · ở Noverse* (chỉ quản trị viên).
- Nhóm Ứng dụng và *Mở Noverse* chỉ có khi có quyền; không vẽ trạng thái khoá.

### Việc backend
- GAP-V26a: `apps: AppLink[]` trong `GET /api/me` và 403 `app_access_denied` (địa chỉ gốc từng app máy chủ đã biết).
- GAP-V26b: 401 trả `reason` và `app` của lần thu hồi phiên (đăng xuất, đổi mật khẩu, quản trị viên đổi, hết hạn). Xem hỏi đáp #50.

### Nghiệm thu
- Đăng nhập ở Noverse rồi mở `vid.aarverse.studio` → vào thẳng, không hỏi lại. Menu tài khoản ở Viderse có dòng Noverse; bấm → Noverse mở ở thẻ mới, đã đăng nhập.
- Đăng xuất ở Viderse → màn đăng nhập ghi “Đã đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này.”; tải lại tab Noverse → màn đăng nhập Noverse ghi “Bạn vừa đăng xuất ở Viderse…”.
- Thành viên chỉ có Noverse đăng nhập Viderse → màn Chưa được cấp quyền có *Mở Noverse*; quản trị viên bật ô Viderse → bấm *Thử lại* → vào app. Không có quyền Noverse → không có nút *Mở Noverse*, menu không có nhóm Ứng dụng.
- Mật khẩu tạm → màn Đặt mật khẩu, không bỏ qua được; đổi xong → đăng nhập lại với khối “Mật khẩu vừa được đổi”.
- Quản trị viên: menu và trang Tài khoản có *Quản lý thành viên* mở `Noverse › Thành viên` ở thẻ mới; thành viên không thấy.
- Dừng máy chủ tài khoản (503) → màn “Chưa đọc được tài khoản”, không mất phiên khi máy chủ trở lại.
- Không màn nào hiện cookie, mã lỗi, tên biến.
- Ghi “Gói V26” vào `10-gap-closure.md`.

## Gói V27 — Lấp chỗ thiếu artboard sau V25, V26 và đợt Làm video (P1, phụ thuộc V2, V6, V26) <!-- r31-v27 --> <a id="goi-v27"></a>

**Vì sao:** dev đo 1:1 và gặp chỗ không có board hoặc board tự mâu thuẫn (hỏi đáp #51, #55–#63). Gói này thay các phương án tạm bằng thiết kế đã chốt.
- Artboard: `Main` / `MMain` (`jobs=running|idle|paused|offline`) · `Queue` · **`MQueue` mới** (`state=running|paused|limited|empty`, `sheet=none|rules|row`) · `MNew` (`input=multi`, `prio=off|on`) · `Explore` (xem trước › nút Làm video) · `MAuth` (`sheet=devices`, `fail=none|network|copy|norecovery`, `busy=no|yes`, `screen=password`) · `Auth` (cùng `fail`, `busy`).
- Spec: ia-flows L13 (viên việc nền), W4, W10; components › JobsPill; copy › Hàng chờ và viên việc nền, Đăng nhập › trạng thái bổ sung; data-contract §3, §4, §25; tokens › Token chốt ở đợt 31.

### Việc FE
1. **Viên việc nền** (#60): 4 trạng thái cùng khung 30px; điện thoại chữ gọn. Đã tạm dừng dùng `pausedAt`. Mất kết nối theo luật ở data-contract §3.
2. **Hàng chờ điện thoại** (#59): màn `MQueue` thay Dialog tạm. Sheet Hàng chờ ở `MMain` có *Mở cả hàng chờ* và *Quy tắc chạy* dẫn tới `MQueue`; sheet Việc nền ở màn gốc khác có *Mở hàng chờ*. Quy tắc chạy là bottom sheet (stepper 44px), áp dụng ngay, nút *Xong* chỉ đóng.
3. **Ưu tiên cả N** (#61): duyệt hàng công tắc 52px cuối nhóm Kênh/Template của `MNew`; công tắc đỏ `--vv-red-button` như chip máy tính.
4. **Không cần ảnh** (#58): **bỏ** khỏi Video mới cả hai bản (L14 · Gói V16 “Không làm: chip ảnh ở Video mới”). Dòng “Chip Kênh / Template / Ảnh” cũ ở Gói V2 đã sửa. Video không ảnh: template không ảnh, hoặc chip Ảnh ở Bàn dựng.
5. **Nút + ở thanh dưới** (#62): dùng token mới `--vv-red-button-fill` (gradient 3 stop của artboard).
6. **Xem trước ở Khám phá** (#63): phụ đề “cho <kênh>” chữ tối `--vv-bg` 82% trên nút sáng; board đã sửa.
7. **Tài khoản điện thoại** (#51): sheet *Thiết bị đang đăng nhập* (thiết bị này không có nút, mỗi máy khác *Đăng xuất*, *Đăng xuất mọi thiết bị khác* có xác nhận tại chỗ, câu nhắc cả hai app); màn mật khẩu tạm có *Đăng xuất*; lỗi mạng, chép mã thất bại, máy chủ không trả mã khôi phục, nút đang chờ; `kicked=none` theo cờ `vv.signedInBefore`. Máy tính cùng bộ trạng thái.
8. **Token** (#51): chữ huy hiệu `--vv-accent-soft` (thay `--vv-tabbar-on`); icon băng thông tin `--vv-info` (thay `--kit-info-fg`, map trong `defineBrand`); ô logo app `--vv-surface-3`.
9. **48px** (#51): giữ nút 48px; board `MAuth` đã cho con của cột cuộn `flex-shrink: 0`, không còn 40,8px.
10. **Hàng chờ máy tính** (#55, #56): bỏ câu đo sức máy, thay câu trung tính; viên “Đã tạm dừng lúc 00:04 · N chờ lượt”; luồng 1–6.

### Việc backend
- Đã có theo #50, #52, #53: `pausedAt`, `waitReason` 6 giá trị, `position`, `thumbnailUrl`, `sourceUrl`, `PATCH /api/queue` v2, `imageCalls`, `priceUSD: null`, `templateId`, `AppLink { id, name, origin }`, 401 `reason`. Gói này không thêm API mới.
- `GET /api/me/sessions` cần trả `{ id, device, browser, lastActiveAt, expiresAt, current }`; `POST /api/sessions/:id/revoke`, `POST /api/me/sessions/revoke-all` (giữ phiên hiện tại).

### Nghiệm thu
- Tạm dừng ở `MQueue` → viên đầu trang mọi màn đổi “Đã tạm dừng lúc HH:MM · N chờ lượt” (điện thoại `⏸ N`), mọi dòng chờ ghi “Hàng chờ đang tạm dừng”; Chạy tiếp → về chấm đỏ.
- Tắt máy chủ → sau 2 lần poll viên ghi “Mất kết nối · đang thử lại”, sheet giữ danh sách cũ có giờ cập nhật, Tạm dừng khoá; bật lại → tự về đúng trạng thái.
- Hàng chờ trống và không job nào chạy → viên “Máy rảnh”, `MQueue` hiện khối Máy rảnh + *Video mới*.
- `MQueue` › ⋯ › *Lên đầu hàng chờ* → mục thành `pick`, đứng đầu; *Bỏ khỏi hàng chờ* → có *Hoàn tác*.
- Quy tắc chạy: luồng dừng ở 1 và 6, ảnh ở 1 và 8, đối thủ ở 1 và 24 giờ (nút giảm/tăng khoá ở biên); đổi trên điện thoại thấy ngay ở `Queue` máy tính.
- Video mới nhiều link: bật Ưu tiên cả 3 → kế hoạch đổi câu, enqueue `priority: "pick"`. Không có tuỳ chọn Không cần ảnh ở cả hai bản.
- Tài khoản điện thoại › Thiết bị: đăng xuất Mac · Chrome → máy đó gặp màn đăng nhập ở cả hai app; *Đăng xuất mọi thiết bị khác* hỏi lại trước khi làm.
- Rút mạng ở màn đăng nhập → băng cam, chip đầu trang cam; không hiện mã lỗi.
- Ghi “Gói V27” vào `10-gap-closure.md`.

## Gói V28 — Màn rộng và chốt hỏi đáp cuối phase nghiệm thu (P1) <!-- r32 --> <a id="goi-v28"></a>

**Vì sao:** trên màn rộng hơn 1440px, cột giữa (Nhật ký tiến độ, ô Nhắn trợ lý, bảng) kéo dãn theo cả bề ngang, dòng chữ dài khó đọc. Cùng đợt, design chốt các mục hỏi đáp còn mở để đóng nghiệm thu.
- Artboard: `Main`, `Activity`, `Dialogs`, `System`, `Queue`, `Channel`, `Explore`, `Library`, `Review`, `Settings`, `Template` (khối CSS `r32-wide`); `Settings` / `MSettings` (câu chữ #20, #22, #25, #26, #28).
- Spec: components › Bố cục màn rộng, Badge số, Composer; tokens › đợt 32; hỏi đáp #6, #8–#23, #25–#30, #73–#76.

### Việc FE
- **Màn rộng:** từ 1600px, con trực tiếp của cột giữa có `max-width` theo loại màn (1280 · 1760 · 880) và căn giữa; ô Nhắn trợ lý + dòng phím tắt tối đa 880px. Khung (đầu trang, cột trái, cột phải) không đổi. Dưới 1600px không đổi gì.
- **Badge** (#14): `.vv-badge` theo kit (chữ `--vv-st-review` trên `--vv-badge-bg`), bỏ `--vv-badge-ink` nâu.
- **Mốc điện thoại** (#16): một mốc 768px; bỏ luật 700px.
- **Cài đặt** (#20, #22, #25, #26, #28): bỏ ngày phát hành; “Ổ ngoài USB”; banner bot ghi “một số video”, “từ HH:MM” = lần lỗi cuối; thẻ Grok không ghi gói; ẩn “tháng này $…”; không ghi “≈ $… mỗi video” ở Cài đặt; 4 nguồn vẽ xếp lưới 2 × 2.
- **Kết nối AI** (#27): thẻ lỗi lên đầu, còn lại theo thứ tự thêm (cũ trước); thẻ vừa thêm đứng đầu tới khi rời trang. Không tạo thẻ trùng loại: chọn lại nguồn đã có → cuộn tới thẻ đó và mở *Đổi khoá* (trừ Tương thích OpenAI và Ollama khác địa chỉ).
- **Khung Cài đặt** (#30): dải mất kết nối đẩy nội dung; chấm theo loại `tool_failed`/`login_expired` cam (`--vv-danger`), `tool_update` xanh (`--vv-info`), `quota_low` vàng (`--vv-st-review`); pha cài không biết tổng dùng thanh chạy vô định `.vv-progress--working`.
- **Composer** (#74): danh sách lệnh `/` theo components › Composer.
- Các mục còn lại: làm như dev đề xuất trong hỏi đáp (đã đồng ý).

### Nghiệm thu
- Mở Bàn dựng ở 2560 × 1440: cột giữa rộng 1280px căn giữa, ô Nhắn trợ lý 880px; ở 1440 × 900 không lệch artboard.
- Khám phá ở 2560: lưới thêm cột, thẻ giữ cỡ; Cài đặt ở 2560: khối 760px nằm giữa.
- Thanh trên: badge Duyệt nền vàng nhạt, chữ vàng, khớp `Main`.
- Ghi “Gói V28” vào `10-gap-closure.md`.

## Gói V29 — Ảnh có prompt đã gửi, sửa prompt và vẽ lại đúng ảnh (P1, phụ thuộc V2, V6, V13) <!-- r33-img --> <a id="goi-v29"></a>

**Vì sao:** người dùng chỉ thấy một câu tóm tắt tiếng Việt về ảnh, không biết Hoạ sĩ đã gửi gì và kèm ảnh tham chiếu nào, nên góp ý mò mẫm. Sau khi vẽ không sửa được prompt của đúng một ảnh, và trong editor muốn vẽ lại phải quay về Duyệt. Ảnh lớn ở màn rộng còn bị phóng quá kích thước gốc nên mờ.
- Artboard: `Review` (`state=review|redrawing|reveal|failed|batch`, `prompt=view|edit|raw`, `refs=view|add`) · `MReview` (`state=prompt|feedback|edit|redrawing|reveal`) · `Editor` (`focus=scene`, `img=view|edit|drawing|done`) · `Main` (ô Phòng vẽ đang vẽ).
- Spec: ia-flows W6, W15, L14; components › Ảnh, prompt và phiên bản; copy › Ảnh, prompt và vẽ lại; data-contract §26.

### Việc FE
- Cột phải Duyệt: Phiên bản → Ảnh tham chiếu → Prompt đã gửi (theo phần, có thay đổi so với bản trước, nguyên văn) → meta → Lời kể. 360px; 440px khi sửa; 420/520px từ 1600px.
- **Sửa prompt** (E): textarea từng phần, *Hoàn tác* từng phần, thêm/bỏ ảnh tham chiếu, thanh dưới chế độ sửa với **Vẽ lại với prompt này** (⌘↵). Esc huỷ.
- **Góp ý → Vẽ lại** (R) giữ như cũ; bản mới ghi `feedback` và hiện thay đổi.
- **Hiệu ứng** theo components › DevelopEffect: đang vẽ (ảnh cũ mờ + vệt sáng + hạt + 3 bước), xem dần khi có `previewUrl`, lộ ảnh + viền sáng khi xong, nhãn MỚI trên dải ảnh; giảm chuyển động thì tắt.
- **Khung ảnh không phóng quá `width` gốc**; căn giữa; dùng container query, không `object-fit: cover` phóng to.
- Chọn nhiều (Shift + bấm) → Vẽ lại N ảnh với góp ý chung.
- **Editor:** bảng phải của Cảnh có ảnh 16:9, Bản, Prompt đã gửi (4 dòng, Xem đủ, Sửa phần Cảnh), ảnh tham chiếu thu nhỏ, góp ý + Vẽ lại, Thay ảnh. Vẽ xong tự thay media của clip (giữ chuyển động/keyframe/thời lượng), toast + Hoàn tác ⌘Z, ảnh đánh dấu cần duyệt lại.
- Điện thoại: hàng Prompt đã gửi mở sheet; sheet Vẽ lại hai tab Góp ý · Sửa prompt.
- Bàn dựng › Phòng vẽ: ô đang vẽ dùng cùng hiệu ứng.

### Việc backend
- Lưu `ImageVersion` cho mỗi lần vẽ (`parts`, `sentText`, `refs`, `provider`, `model`, `params`, thời gian, `costImages`, `error`), giữ mọi bản (ảnh của bản cũ không bị GC khi còn trong lịch sử của video).
- `POST …/images/redraw` nhận `feedback`, `parts`, `refs`, `provider`; `…/use`; `…/cancel`; SSE `image.progress` (có `previewUrl` khi nhà cung cấp hỗ trợ ảnh xem dần).
- Editor: đổi bản đang dùng → cập nhật binding media của clip trong project, ghi vào lịch sử hoàn tác.

### Nghiệm thu
- Duyệt cảnh 3: cột phải hiện đúng nguyên văn đã gửi (so với log máy chủ) và 2 ảnh tham chiếu đã gửi.
- Sửa phần Cảnh, bỏ ảnh tham chiếu 2 → Vẽ lại → bản 3 dùng đúng prompt đã sửa, chỉ 1 ảnh tham chiếu; template và prompt Đêm xanh rêu không đổi; ảnh khác không đổi.
- Góp ý “thêm biển…” → bản mới hiện thay đổi (thêm xanh, bỏ gạch) so với bản trước.
- Trong lúc vẽ: duyệt ảnh khác được; Huỷ lần vẽ → bản cũ giữ nguyên, không tốn ảnh nếu nhà cung cấp chưa nhận.
- Hết hạn mức → thẻ lỗi có giờ đặt lại thật, *Vẽ bằng Grok* chạy đúng prompt đã sửa.
- Màn 2560 × 1440: ảnh 1280×720 hiện đúng 1280px, không phóng to.
- Editor: vẽ lại cảnh 5 trong lúc kéo phụ đề → không mất thao tác; xong ảnh thay vào timeline, ⌘Z quay lại bản 1.
- Ghi “Gói V29” vào `10-gap-closure.md`.

## Gói V30 — Đường dẫn, chỗ trống theo ổ, nhóm chọn không cắt chữ (P2) <!-- r34 --> <a id="goi-v30"></a>

**Vì sao:** báo cáo QA nêu ba luật hiển thị chưa chốt (#78–#80). Không thêm màn; board chỉ sửa chữ và kit.
- Artboard: `Settings` · `section=storage`, `store=here|done|donenew|offline|cloud` · `MSettings` · `screen=storage`, cùng `store` · `Kit` › Ô nhập & nút › Nhóm chọn · `Explore` / `MExplore` (Sắp xếp, Vùng giờ — đã co theo nhãn).
- Spec: components › Đường dẫn thư mục, SegmentedTabs; copy › Lưu trữ trên máy, Nhóm chọn: nhãn; 05-kit › Segmented.

### Việc FE
- Đường dẫn: in `displayPath`, một dòng, cắt giữa theo thư mục theo bề rộng thật; `title` đầy đủ.
- Chỗ trống: theo ổ đang giữ thư mục lưu dự án, kèm tên ổ; ổ ngoài rút → “chưa cắm”, không thay bằng số của ổ trong máy; không thêm dòng cảnh báo ở `offline`.
- Kit `Segmented`: mục co theo nhãn (`flex: 0 0 auto`), không `text-overflow: ellipsis`; nhóm `max-width: 100%`, cuộn ngang ẩn thanh cuộn; `--fill` / `--m` chia đều với `min-width: max-content`; viên trượt đo theo mục thật. Bỏ ngoại lệ tạm cho hai nhóm Sắp xếp / Vùng.

### Nghiệm thu
- Thư mục `/Volumes/Samsung T7/Studio 2026/Kênh YouTube/Viderse Projects` ở 1440 hiện `/Volumes/Samsung T7/…/Kênh YouTube/Viderse Projects`; ở 390 hiện `/Volumes/Samsung T7/…/Viderse Projects`; rê chuột thấy đủ.
- Rút ổ T7: thẻ dung lượng ghi “Samsung T7 chưa cắm · chưa đọc được chỗ trống”.
- Khám phá ở 390: nhóm Sắp xếp đủ chữ “Mới nhất”, cuộn ngang được khi chật; không chỗ nào trong app có nhóm chọn bị “…”.
- Ghi “Gói V30” vào `10-gap-closure.md`.

## Gói V31 — Toast theo Noverse (P2) <!-- r35 --> <a id="goi-v31"></a>

**Vì sao:** toast của Viderse lệch hẳn Noverse: ba thẻ phẳng 300px chồng góc phải (kể cả kết quả thao tác), vạch đếm ở đáy, dòng chữ “Tự đóng sau 5 giây…”, tiêu đề hai dòng. Đợt 35 đưa về cùng chất liệu và hành vi với Noverse.
- Artboard: `Activity` · `view=toasts` (= `Dialogs` · `dialog=toasts`) · `Kit` › Gợi ý & trạng thái › Toast · `Kit` › Xác nhận (toast sau khi xác nhận) · `Editor` · `focus=scene&img=done` · `MKit` › Sheet › Toast.
- Spec: components › Toast; copy › Toast: câu chữ; 05-kit › Toast; tokens `--vv-toast-glass`, `--vv-toast-border`, `--vv-toast-shadow` (mới), `--vv-toast-ring`, `--vv-*-tint`; luật L13 ở 01.

### Thấy trong code hiện tại
- `apps/viderse/web/src/App.tsx` dùng `<ToastHost activity />` của `@noverse/ui`: mọi toast (cả viên) nằm cột phải, thẻ 300px bo 12–14px, tối đa 3.
- `shell/ActivityBridge.tsx` ghi `meta` “Gộp N thông báo trong 10 giây” / “Giữ tới khi bạn đóng” / “Tự đóng sau 5 giây…” và tiêu đề là tên video.
- `editor/v13/Editor.tsx` dùng `sonner` riêng (`<Toaster>` + ~20 file `import { toast } from 'sonner'`), nên toast ở Editor khác phần còn lại.

### Việc FE
- Viên kết quả thao tác (`toast(text, tone, action)`): **giữa đáy cột nội dung**, trên ô Nhắn trợ lý / thanh đáy 12px; 6 giây, Hoàn tác 5 giây và ghi “· Ns” trên nút.
- Thẻ việc nền (`toast.card`): góc phải dưới, rộng 328px, bo 18px, kính mờ, icon tròn 34px trên nền loang màu loại (trợ lý: orb + tick mint), tiêu đề một dòng, câu phụ tối đa 2 dòng; **tối đa 2**, thẻ cũ lùi sau (scale .955, mờ 60%), rê chuột bung + dừng đếm. Chỉ thẻ trước có nút sáng.
- Đếm ngược là vòng quanh nút đóng như `platform/ui` Toast đang có cho viên; áp cả cho thẻ. Bỏ `meta`, bỏ vạch đáy. Cần xử lý: không vòng, ở lại.
- Câu chữ theo copy › Toast: tiêu đề = việc, tên video vào câu phụ.
- Editor: thay `sonner` bằng `toast` của `@noverse/ui` để cùng một chồng toast và cùng kiểu.
- Nếu sửa trong `platform/ui/src/Toast.tsx` (dùng chung với Noverse) thì chỉ đổi nhánh `activity` / thêm tuỳ chọn vị trí; viên của Noverse giữ nguyên.

### Nghiệm thu
- Xoá 2 video ở Kênh: viên “Đã xoá 2 video · Hoàn tác · 5s” hiện giữa đáy, không ở góc phải; vòng quanh nút đóng chạy hết trong 5 giây.
- Ba thông báo việc nền đến liền nhau: chỉ thấy 2 thẻ, thẻ cũ lùi sau; rê chuột thì bung và vòng dừng.
- Không còn chữ “Tự đóng sau…”, “Gộp N thông báo…” trong toast; Editor và các màn khác cùng một kiểu toast.
- Ghi “Gói V31” vào `10-gap-closure.md`.

## Gói V32 — Thanh trên điện thoại chuẩn hoá, Thông báo trên điện thoại (P1) <!-- r36 --> <a id="goi-v32"></a>

**Vì sao:** ảnh bản thật cho thấy viên “Rảnh” và chấm vàng đè lên mép viên, avatar khác cỡ viên, chồng avatar “Mọi kênh” che mất chữ. Thiết kế điện thoại trước đây cũng không có nơi xem thông báo nên chấm vàng không có đích.
- Artboard: `MMain` · `jobs=running|idle|paused|offline`, `notice=unread|read|none`, `screen=inbox` (sheet Việc nền mở ở tab Thông báo) · `MKit` › Đầu trang › Thanh trên · mọi trường hợp · `MChannel`, `MReview`, `MExplore` (thanh trên dùng cùng thành phần) · `Main` (chồng avatar một chữ).
- Spec: components › Đầu trang điện thoại › AppBar, JobsPill; copy › Thanh trên điện thoại và Thông báo, Hàng chờ và nút việc nền; 05-kit › AppBar; 01 › thanh trên.

### Thấy trong code hiện tại
- `shell/JobsPill.tsx` (`mobileBar`): viên 36px có chữ “Rảnh” khi rảnh; chấm `unread` `top/right: 3px` nên nằm đè mép viên; avatar cao khác viên.
- `shell/MobileShell.tsx`: chưa có chỗ xem thông báo trên điện thoại (sheet chỉ có việc đang chạy).

### Việc FE
- `MobileShell` thanh trên: logo 24 · `ChannelSwitcher` · (đẩy phải, khe 8) nút Việc nền · avatar. Mọi phần tử cao 36px, nền `--vv-surface-1`, viền `--vv-card-border`, vùng chạm 44px.
- `JobsPill mobileBar` → nút tròn 36px: đang chạy = vòng 2px (`--vv-red`, tiến độ trung bình các việc đang chạy) + số việc đang chạy; rảnh = icon `activity`; tạm dừng = vòng xám + icon `pause`; mất kết nối = icon `danger` cam. Bỏ chữ trên điện thoại; `aria-label` câu đầy đủ.
- Chấm chưa đọc: 10px, viền 2px màu nền, `top: -1px; right: -1px` (tâm trên mép tròn); ẩn khi mất kết nối.
- `ChannelSwitcher mobileBar`: `min-width: 0`, tên cắt “…”; Mọi kênh = 3 avatar 22px **một chữ cái**, chồng −7px, viền 2px màu thanh. Chưa có kênh → nút “+ Tạo kênh”. Máy tính: chồng avatar cũng một chữ cái.
- Sheet Việc nền điện thoại: tiêu đề “Việc nền”, `Segmented --m --fill` hai tab Đang chạy · Thông báo (huy hiệu số); bấm nút khi có chấm thì mở tab Thông báo. Tab Thông báo dùng cùng dữ liệu với popover máy tính, *Đánh dấu đã đọc*, trạng thái trống.

### Nghiệm thu
- 390px, Mọi kênh + 2 việc + có thông báo: logo, bộ đổi kênh, nút Việc nền, avatar thẳng hàng giữa, cùng cao 36; chấm vàng nằm trên mép tròn.
- Kênh tên dài (“Chuyện Lạ Đêm Khuya”) ở 360px: tên cắt “…”, hai nút phải không xê dịch.
- Máy rảnh: không còn chữ “Rảnh”. Chưa có kênh: thấy “+ Tạo kênh”.
- Bấm nút Việc nền khi có chấm → sheet mở ở Thông báo; đánh dấu đã đọc → chấm mất.
- Ghi “Gói V32” vào `10-gap-closure.md`.

## Gói V33 — Mọi màn dùng UI Kit, chuyển động theo Noverse (P1) <!-- r37 --> <a id="goi-v33"></a>

**Vì sao:** nhiều màn (Video mới, Bàn dựng, Duyệt, Kênh, Thư viện, Cài đặt…) còn nút, ô chọn, nhóm chọn, menu vẽ tay: kích thước lệch, chữ trong nút lệch dọc, nhóm chọn “Một link / Nhiều link” đổi nền tức thì không có viên trượt, menu mở không hiệu ứng. Đợt 37 đưa toàn bộ board về class kit (423 nút, 33 nhóm chọn, 11 chip, 7 ô chọn, 12 menu nổi với 30 mục, 1 nhóm tab).
- Spec: components › **Mọi màn dùng UI Kit** (bảng thành phần → class → chuyển động); 05-kit › Segmented, ChipSelect.
- Artboard: mọi board. Xem nhanh: `NewVideo` (nhóm chọn Một link/Nhiều link trượt; chip Kênh, Template) · `Main` (menu ⋯ và Đổi kênh mở có hiệu ứng) · `Review` (nút, nhóm chọn) · `Template`, `Settings` (nhiều nhóm chọn) · `Kit` › Ô nhập & nút.

### Thấy trong code hiện tại (đếm ngày 01/11)
- `<button>` tự style: 310 chỗ trong 92 tệp (so với 203 chỗ dùng `Button`). `role="radiogroup"/"tablist"` tự dựng: 32 chỗ, `Segmented` mới dùng 20 chỗ. Ô chọn của kit (`KitSelect`/`RichSelect`/`Combobox`) chỉ 3 chỗ; `role="menu"/"listbox"` tự dựng 9 chỗ.
- Ví dụ anh/chị thấy trên bản thật:
  - `screens/new/NewScreen.tsx:102` nhóm Một link / Nhiều link tự dựng → không có viên trượt. Chip Kênh / Template tự dựng.
  - `screens/review/ReviewScreen.tsx:89` trạng thái “Đã duyệt hết”: liên kết “Xem tiến độ trên Bàn dựng” dùng `sx.button` tự đặt → chữ lệch dọc. Dùng `Button` (`as="a"` / `href`).
  - `screens/library/TemplateImportDialog.tsx`: đoạn mô tả không có lề (thân `Dialog` không tự đệm), mô tả lặp lại chữ trong vùng thả, “Nhập tệp khác” hiện cả khi chưa nhập gì, “Đóng” là nút chính. Làm lại theo `Library` · `import=…` (dưới).
- `@noverse/ui` `Dialog`: `padX` chỉ áp cho đầu và chân, thân không có lề → dễ dính mép. Đề xuất: thân mặc định đệm 0 `padX` (giống `.vv-dialog` của kit), màn nào cần tràn mép (danh sách) thì tắt.

### Hộp thoại Nhập template cũ (mới vẽ) <a id="import-template"></a>
- Artboard: `Library` · tab Template · bấm *Nhập template* hoặc tweak `import=ready|drag|importing|done|partial|zip|offline`.
- `Dialog` 520px, icon đầu theo kết quả (thường · xong mint · một phần cam nhạt · mất kết nối cam). Dòng phụ nói tệp là gì; **vùng thả** (`.vv-drop`, mới trong kit) nói việc cần làm: “Kéo tệp .zip vào đây” · “Thả nhiều tệp một lúc được” · nút *Chọn tệp*. Kéo tệp vào: viền và nền màu nhấn, “Thả để nhập 3 tệp”.
- Đang nhập / kết quả: mỗi tệp một dòng (`.vv-filerow`): đã nhập (tên template + “14 media vào kho media”), đang nhập (vòng quay + vạch %), chờ, chưa nhập được (lý do màu cam), “2 điều cần biết” mở danh sách cảnh báo tại chỗ.
- Nút: chưa nhập → chỉ *Đóng*; đang nhập → *Huỷ phần chưa nhập*; xong / một phần → *Nhập tệp khác* + **Xong**; mất kết nối → *Đóng* + **Thử lại** (tệp đã chọn vẫn giữ). Không .zip → báo lỗi ngay trên vùng thả.

### Hộp thoại có trong code nhưng chưa có thiết kế
- “Nguồn đã gắn không còn trong Video nền” (`template/BackdropSection.tsx`), “Chọn hiệu ứng từ thư viện” (`template/BackdropSection.tsx`), “Chọn đoạn của video” (`backdrops/VideosPage.tsx`). Design vẽ ở đợt sau; trong lúc chờ, dựng bằng `Dialog` + thành phần kit theo bảng ở components › Mọi màn dùng UI Kit, không tự style.

### Việc FE
- Nhóm chọn chữ → `Segmented` (`size`, `fit`), có viên trượt; tab gạch chân → `Tabs`. Lưới thẻ lựa chọn có mô tả, dải ảnh thì giữ.
- Nút → `Button` đúng biến thể; bỏ style tự đặt (cao, đệm, viền, nền, cỡ chữ). Nút chỉ có icon → `variant="icon"` + `aria-label`.
- Chip “khoá : giá trị ▾” → `ChipSelect`; ô chọn trong biểu mẫu → `Select`; menu ⋯ / Đổi kênh → `Menu` (có hiệu ứng mở).
- Không thêm `transition` riêng ngoài kit; tôn trọng `prefers-reduced-motion`.
- `TemplateImportDialog` làm lại theo artboard `Library` · `import=…`; thêm `.vv-drop` / `.vv-filerow` (hoặc tương đương trong `@noverse/ui`) để Nhập prompt, Thêm hiệu ứng dùng chung.

### Nghiệm thu
- Video mới: bấm Một link ↔ Nhiều link thấy viên nền trượt; bấm chip Kênh / Template mũi tên lật.
- Không còn nút có chữ lệch dọc (ví dụ “Xem tiến độ trên Bàn dựng” ở Duyệt khi đã duyệt hết).
- `grep -rn 'role="radiogroup"\|role="tablist"' src` chỉ còn ở thành phần kit hoặc lưới thẻ lựa chọn.
- Nhập template: chưa chọn tệp chỉ có *Đóng*; mô tả có lề như tiêu đề; nhập 3 tệp thấy từng dòng tiến độ.
- Ghi “Gói V33” vào `10-gap-closure.md`.

## Gói V34 — Màn còn thiếu artboard (P1) <!-- r38 --> <a id="goi-v34"></a>

**Vì sao:** năm chỗ code đã có hoặc sắp làm nhưng chưa có thiết kế: Thư viện và Sửa template trên điện thoại, kênh mới chưa có video (điện thoại), Khám phá khi rỗng hoặc lỗi, thẻ YouTube Data API, bước Xong khi Đạo diễn lỗi. Đợt này vẽ đủ, cùng một bộ tuỳ chọn giữa máy tính và điện thoại.

| Màn | Artboard · tweak | Ghi chú cho code |
|---|---|---|
| Thư viện dùng chung (điện thoại) | `MLibrary` · `tab=templates\|prompts\|backdrops\|overlays` · `sheet=none\|actions\|new` · `empty` | Lối vào: `MChannel` › Template › *Mở Thư viện dùng chung* (thay dòng “Tạo và sửa template trên máy tính”). Tab là `Segmented`; ⋯ của template mở sheet: Sửa template, Đổi tên, Nhân bản, Thêm vào kênh, Xuất tệp, Xoá (tắt khi còn kênh dùng, ghi lý do). Template có nhãn sức khoẻ “Ổn” / “Cần sửa · thiếu clip nền”. |
| Sửa template (điện thoại) | `MTemplate` · `screen=list\|images\|cover\|backdrop\|output` · `preview=fresh\|stale\|building` · `sheet=none\|more` | Danh sách 4 mục (ảnh cảnh, ảnh bìa, nền & hiệu ứng, xuất) + bản xem thử + danh sách sẵn sàng; mỗi mục một màn con, tự lưu (thanh “Đã lưu · 23:52”). Cùng tuỳ chọn với `Template` máy tính. |
| Kênh mới chưa có video (điện thoại) | `MChannel` · `fresh=true` | Như `Channel` · `fresh=true`: đầu trang tên kênh mới, số liệu 0, tab Video 0 · Nguồn 2 · Template 1; khối trống có video mới nhất của đối thủ, Tự lấy video, Khám phá. |
| Khám phá — danh sách | `Explore` · `list=ok\|filterEmpty\|firstScan\|error\|offline` · `MExplore` · `list=…\|noRivals` | Hiện `ExploreScreen.tsx` chỉ in một dòng chữ cho lỗi và lọc rỗng. Cần: lọc rỗng có *Bỏ lọc* (đưa Dài, Đăng trong, Trạng thái về mặc định); quét lần đầu có banner + ô xương (đừng báo “không khớp bộ lọc” khi đối thủ còn đang quét); lỗi tải có *Thử lại* (gọi lại danh sách); mất kết nối dùng câu `retryingIn` + *Thử lại ngay*. Điện thoại thêm chip **Đăng trong** cho khớp máy tính, và trạng thái chưa theo dõi đối thủ. |
| Thẻ YouTube Data API | `Settings` · `section=connect` · `ytkey=empty\|ok\|edit\|rejected\|failed\|offline` · `MSettings` · `screen=connect` · `ytkey=…` | `YoutubeDataKeyCard` đã có. Đổi: (1) tách khỏi danh sách nhà cung cấp AI thành nhóm **Số liệu YouTube** đặt dưới nút Thêm nhà cung cấp; (2) khi ở chế độ sửa thêm dòng gợi ý dưới ô (tạo khoá ở Google Cloud Console, bật YouTube Data API v3); (3) `failed` hiện *Thử lại* thay cho *Thêm khoá* (khi chưa đọc được trạng thái, chưa biết đã có khoá hay chưa). Câu chữ giữ `performanceCopy.dataKey`. |
| Bước Xong khi Đạo diễn lỗi | `CreateChannel` · `Onboard` · `MCreate` · `step=done` · `identity=director` | Dùng `channels.directorFailed(reason)`, `waitDirector`, `noName`. Thẻ “Một việc cần bạn”, nút *Thử lại* ở dòng Đạo diễn (gọi lại bước đặt tên), nút *Đặt tên ở Hồ sơ kênh*. Avatar chữ “K” nền trung tính tới khi có tên. |

### Việc FE
- Màn điện thoại mới dùng thành phần kit như các màn điện thoại khác (`Segmented`, `Sheet`, hàng `button.vv-row`), không vẽ tay.
- Khám phá: tách 4 nhánh trạng thái (lọc rỗng / quét lần đầu / lỗi / mất kết nối) thay cho `vx-side-empty`; khối trống dùng `.vv-empty` của kit.
- Kết nối AI: chuyển `<YoutubeDataKeyCard />` ra ngoài `conn-list`, thêm tiêu đề nhóm (cả máy tính và điện thoại).
- Tạo kênh: khi Đạo diễn lỗi vẫn tạo kênh với tên tạm, không chặn bước Xong.

### Nghiệm thu
- Điện thoại: Kênh › Template › *Mở Thư viện dùng chung* → Thư viện; chạm một template → Sửa template; sửa một mục thấy “Đã lưu”.
- Khám phá: lọc Dài ≥ 40 phút + 7 ngày ra rỗng → thấy câu nêu bộ lọc + *Bỏ lọc*; tắt mạng → *Thử lại ngay*; thêm đối thủ mới → banner quét + ô xương.
- Cài đặt › Kết nối AI: nhóm Số liệu YouTube nằm dưới nút Thêm nhà cung cấp; dán khoá sai → viền cam + câu lỗi.
- Tạo kênh khi ChatGPT không phản hồi: vẫn tới bước Xong, tên “Kênh mới”, *Thử lại* chạy lại Đạo diễn.
- Ghi “Gói V34” vào `10-gap-closure.md`.

## Gói V35 — Trạng thái hệ thống và xác nhận trên điện thoại (P2) <!-- r39 --> <a id="goi-v35"></a>

**Vì sao:** `System` và `Dialogs` (máy tính) chưa có bản điện thoại. Board mới `MSystem` · tweak `state`, cùng câu chữ với máy tính, hộp thoại thành sheet đáy.

| `state` | Khớp máy tính | Ghi chú điện thoại |
|---|---|---|
| `loading` | `System` · `loading` | Ô xương theo bố cục Bàn dựng, giữ thanh trên và thanh dưới |
| `offline` | `System` · `offline` | Banner cam + *Thử lại ngay*; danh sách cũ mờ, không bấm được; nút Việc nền đổi icon cam |
| `notfound` | `System` · `not-found` | Bỏ dòng mã kỹ thuật (chỉ máy tính có ở *Chi tiết*); một nút **Về Bàn dựng** |
| `empty` | `System` · `empty-workbench` | **Làm video đầu tiên** + *Mở Khám phá*, 4 trợ lý “Chờ việc” |
| `delVideo` · `delChannel` (+ `typed`) · `redrawAll` · `restart` | `Dialogs` cùng tên | Sheet đáy, hai nút chia đôi: Huỷ + nút chính; xoá dùng nút cam. Xoá kênh mở từ Kênh › Hồ sơ kênh, nút chỉ mở khi gõ đúng tên |
| `ask` · `unknown` | `Dialogs` · `assistant-confirm` · `assistant-unknown` | Gợi ý là hàng bấm được 52px |
| `clip` | `Dialogs` · `clip-picker` | Mở từ chi tiết video › Nền video; chip lọc cuộn ngang, danh sách clip, dải Từ–Đến |

### Nghiệm thu
- Điện thoại: tắt mạng ở Bàn dựng thấy banner + danh sách mờ; mở link video đã xoá thấy “Không tìm thấy video này”, không lộ mã lỗi.
- Mọi xác nhận là sheet đáy, Huỷ bên trái; xoá kênh khoá nút tới khi gõ đúng tên.
- Ghi “Gói V35” vào `10-gap-closure.md`.

## Gói V36 — Workflow gọn, Trình xem ảnh, nối nút (P1) <!-- r40 --> <a id="goi-v36"></a>

**Vì sao:** người dùng gặp nhiều chỗ bấm không ra gì (hộp thoại không mở, bấm ảnh không xem được). Máy bấm thử 557 điều khiển trên 16 board máy tính: 120 không phản hồi. Workflow gọn đã chốt ở board **`Workflow`** và `01-ia-flows.md` § 0.3; gói này là đợt đầu.

### Làm ngay (đợt 40)
- **Trình xem ảnh** (04 › Trình xem ảnh): thành phần dùng chung. Bàn dựng › Phòng vẽ: bấm ảnh mở chế độ Xem; nút Vẽ lại trên ô ảnh mở luôn ô góp ý. Video chờ duyệt: bấm ảnh sang Duyệt đúng ảnh đó. Artboard `Main` · `viewer=view|redraw`.
- **Duyệt:** nút “Vẽ lại cả 7 ảnh” mở hộp xác nhận (trước đây chỉ có ở `Dialogs`). Artboard `Review` · `confirm=redrawAll`.
- **Nối nút:** Thư viện (Nhân bản, Xuất tệp → toast), Khám phá (Sao chép link → toast “Đã chép link YouTube”), Cài đặt (Kiểm tra lại), Template (Mở trong editor là link; ⋯ mở menu Nhân bản · Xuất tệp · Về Thư viện · Xoá (khoá khi đang dùng); nút phát xem thử), Việc nền (dòng video và nút Mở tới Bàn dựng; Cài đặt thông báo mở đúng màn), hộp thoại ở `Dialogs` đóng được bằng Huỷ / ×.
- Toast kết quả dùng `.vv-toast` của kit, giữa đáy, tự đóng sau 4 giây.

### Ảnh hưởng tới backend (đã chốt, áp khi tới đợt)
- Bỏ: đọc số liệu sau đăng và `perf_insight`, khoá YouTube Data API (`YoutubeDataKeyCard`, `/api/performance/data-key`), `learn_rule`, `raise_autonomy`, `pace`, Ưu tiên kênh “Lượt xem”, ngoại lệ ảnh theo video (prompt riêng video), danh sách luân phiên template của kênh, cờ ưu tiên khi thêm nhiều link.
- Đổi: Mức tự chủ của kênh còn một cờ `autoApprove` (ngưỡng cố định 85); kênh có một `defaultTemplateId`; video người dùng tự thêm luôn vào phần đầu hàng.
- Anh/chị dev xem lại, chỗ nào khó bỏ (đã có dữ liệu) thì ghi hỏi đáp trước khi xoá.

### Tạo kênh: Cách kênh chạy (đợt 41b) <!-- r41b -->
- Ba dòng: **Template** (ô chọn mở danh sách nổi — `Select`/`Popover`, không đẩy nội dung; điện thoại mở sheet đáy — bỏ link “Chọn template khác” sang Thư viện), **Tự duyệt ảnh** (công tắc → `autoApprove`), **Video mỗi ngày** (bước chỉnh → nhịp kênh). Bỏ Mức tự chủ ba mức ở màn này.
- Artboard `CreateChannel` · `Onboard` (bước Tạo kênh) · `MCreate` · tweak `tplMenu`.

### Đợt sau (đã chốt, chưa vẽ lại)
- 41 Bàn dựng gọn (xong, xem Gói V37) · 42 Kênh chỉ còn thiết lập (xong, xem Gói V38), Thư viện lên thanh trên · 43 Làm video thành hộp thoại (xong, xem Gói V39) · 44 điện thoại (xong, xem Gói V40) · 45 Trình xem ảnh bản điện thoại. Chi tiết ở board `Workflow` mục 05 và 07.

### Nghiệm thu
- Bàn dựng: bấm một ảnh đã vẽ ở Phòng vẽ → Trình xem ảnh; ← → đổi ảnh, Esc đóng; Góp ý & vẽ lại hiện giá trước khi gửi.
- Duyệt: Vẽ lại cả 7 ảnh → hộp xác nhận có số ảnh tốn.
- Không còn nút nào bấm không có phản hồi ở các chỗ liệt kê trên.
- Ghi “Gói V36” vào `10-gap-closure.md`.

## Gói V37 — Bàn dựng gọn, Editor & Template mở ảnh, nền mới cho Việc nền/Hộp thoại/Trạng thái (P1) <!-- r41g --> <a id="goi-v37"></a>

**Vì sao:** đợt 41 của workflow gọn (01 § 0.3). Bàn dựng còn nhiều khối ít dùng (Bản tin, Nhắn trợ lý, chip Ảnh, cột phải 4 tab) và nhiều chỗ bấm không ra gì.

### Màn và artboard
- **4.1 Bàn dựng** (`Main`) — 01 › W5 (viết lại):
  - Thanh trên: Bàn dựng · Duyệt · Kênh · Khám phá · **Thư viện** + nút **Làm video** (N).
  - Cột trái: bấm dòng mở video; **Chọn** → hộp kiểm + thanh **Tải gói N video**; **đề xuất là dòng trong Chờ bạn** (3 loại, có ×); chân cột “Template mặc định”.
  - Cột giữa: khối **Tiến độ** (2 dòng nhật ký + “Nhật ký · N bước”) gộp vào khối chính; bỏ dock (ô nhắn, kế hoạch, hàng phím tắt); bố cục tóm tắt cho video không có bộ ảnh; ảnh thành phẩm 16:9; toast góc dưới phải.
  - Cột phải: hai tab **Nội dung đăng · Thông tin** (mặc định Thông tin). Sao chép, bỏ/thêm tag đã nối.
  - Bỏ: Bản tin, nút Đề xuất N, chip Ảnh (ảnh riêng video), Vì sao?, Lưu thành quy tắc, thẻ Kế hoạch, ô Nhắn trợ lý.
  - Tweak: `video`, `select`, `panel=info|content`; bỏ `agent`, `promptPop`.
- **3.2 Editor** (`Editor`): bấm ảnh cảnh mở Trình xem ảnh **Xem + Thay**; thanh trái là cột dọc (trước bị vẽ thành nhóm ngang); nối hoàn tác/làm lại, Phím tắt (popover nổi), Về đầu/Phát/Tới cuối, tắt tiếng, ±0,1 s, Nghe lại, Tách/Xoá, thu phóng timeline, ẩn từng track.
- **5.3 Sửa template** (`Template`): nút mắt trên mỗi prompt mở Trình xem ảnh **Mẫu**; ô “Dựng từ video đã làm” và ô video của Vẽ thử ảnh bìa là **danh sách nổi** (`Select`/`Popover`, không đẩy nội dung); thanh Độ mạnh/Tốc độ kéo được; Chọn ảnh, Đặt lại, Bỏ hiệu ứng, Thêm vào kênh, Bỏ khỏi kênh có phản hồi.
- **2.3 Việc nền** (`Activity`), **6.1 Hộp thoại** (`Dialogs`), **6.2 Trạng thái hệ thống** (`System`): nền phía sau dựng lại từ `Main` mới.
  - Việc nền: bấm viên đóng/mở popover; nối mọi nút trong popover; công tắc Cài đặt thông báo bật/tắt được; bỏ “Bản tin khi bạn quay lại”; “Đề xuất của trợ lý” một công tắc cho 3 loại.
  - Hộp thoại: bỏ `assistant-confirm` / `assistant-unknown` (ô Nhắn trợ lý đã bỏ); Phím tắt “N Làm video” thay “/ Nhắn trợ lý”, bỏ “Gửi tin nhắn”; “template mặc định” thay “template luân phiên”.
  - Trạng thái: khung đang tải theo bố cục mới; Mất kết nối có Thử lại ngay (“Đang kết nối lại…”) và Chi tiết; Bàn dựng chưa có video: cột phải “Bốn trợ lý”, chân trái “Template mặc định”, ẩn số Duyệt khi 0.

### Việc FE
- Bàn dựng: `VideoList` dòng là nút; thêm chế độ chọn (`selection` state, thanh dính); `SuggestionRow` (orb, câu, trợ lý · việc, nút ×) trong nhóm Chờ bạn; bỏ `RecapView`, `SuggestionCard`, `PlanCard`, `AgentComposer`, `ExplainPopover`, `LearnRulePrompt`, `VideoImagePopover`.
- Cột phải: `Tabs` 2 mục; tab Thông tin gom Bốn trợ lý + Nguồn + Template + Nền.
- Trình xem ảnh dùng lại ở Editor (`mode="view-replace"`) và Template (`mode="sample"`).
- Lớp phủ Việc nền/Hộp thoại/Trạng thái dùng chung khung Bàn dựng, không giữ bản sao riêng.

### Ảnh hưởng tới backend
- Không còn gọi: tạo Bản tin, `explain`, `learn_rule`, kế hoạch từ câu nhắn, ngoại lệ ảnh theo video. Đề xuất chỉ 3 loại (Gói V36).
- Cài đặt thông báo: bỏ cờ “Bản tin khi bạn quay lại”; “Đề xuất của trợ lý” là một cờ bật/tắt.

### Nghiệm thu
- Bàn dựng: bấm từng dòng ở cột trái → cột giữa và cột phải đổi theo video; Chọn → chọn 2 video sẵn sàng → “Tải gói 2 video”; × trên dòng đề xuất → dòng ẩn.
- Editor: bấm ảnh cảnh → Trình xem ảnh, ← → đổi ảnh, Esc đóng.
- Template: bấm mắt trên prompt → Trình xem ảnh Mẫu; mở ô “Dựng từ video đã làm” → danh sách nổi, nội dung bên dưới không dịch.
- Việc nền / Hộp thoại / Trạng thái: nền giống hệt Bàn dựng mới; không còn Bản tin, chip Ảnh, ô nhắn.
- Bấm thử: các board trên về 0 nút không phản hồi (trừ phần nằm dưới hộp thoại/popover).

## Gói V38 — Kênh chỉ còn thiết lập, Thư viện lên thanh trên, template mặc định (P1) <!-- r42 --> <a id="goi-v38"></a>

**Vì sao:** đợt 42 của workflow gọn (01 § 0.3). Trang Kênh có bảng video thứ hai (trùng Bàn dựng), cột tóm tắt và lịch đăng ít dùng, Mức tự chủ quá nhiều lựa chọn, luân phiên template khó đoán video nào ra kiểu gì; Thư viện bị giấu trong cột trái trang Kênh.

### Màn và artboard
- **Thanh trên mọi màn máy tính** (`Main`, `Review`, `Channel`, `Explore`, `Library`, `Template`, `Queue`, `Settings`, `NewVideo`, `CreateChannel`, `Onboard`, `Auth`, `Activity`, `Dialogs`, `System`): Bàn dựng · Duyệt · Kênh · Khám phá · **Thư viện** + nút **Làm video (N)**. Sửa template sáng ở Thư viện.
- **5.1 Kênh** (`Channel`) — 01 › W11 (viết lại):
  - Cột trái chỉ còn Kênh của tôi (bấm đổi kênh) + Tạo kênh.
  - Ba tab Nguồn & tự động · Template · Hồ sơ kênh. Bỏ tab Video, cột Đang áp dụng, Lịch đăng.
  - Số liệu 4 ô + “Xem trong Bàn dựng”.
  - Khối **Tự động**: Tự lấy video mới (công tắc), Video mỗi ngày (1–20), Tự duyệt ảnh (công tắc, ≥ 85).
  - Tab Template: một **Template mặc định** + Đổi template (danh sách nổi) + Sửa template.
  - Hồ sơ kênh: bỏ Quy tắc đã học; nối Vẽ lại, Tải, Đặt tên khác, Tạo lại cả bộ, ngôn ngữ, tag, Xoá kênh.
  - Bỏ liên kết đối thủ hỏi xác nhận; Liên kết N chạy.
  - Tweak: `tab=sources|templates|settings`, `channel`, `autoApprove`, `tplMenu`, `identity`, `link`, `fresh`.
- **5.2 Thư viện** (`Library`): bỏ cột trái; template ghi “Mặc định của …”; menu ⋯ “Đặt làm mặc định cho kênh…”; nối Đổi tên, Nhập prompt, Vẽ thử, Nhân bản, Lưu, Làm mới, Hiện thêm; danh sách nổi cho Vẽ thử với và Sắp xếp.
- **5.3 Sửa template** (`Template`): chip “Mặc định của 2 kênh” (MT, QH); mục **Kênh dùng làm mặc định** + Đổi ở Kênh; nối Xuất tệp, Nhân bản.
- **6.1 Hộp thoại** (`Dialogs`): bỏ `autonomy` (Chuyển sang Tự chạy).

### Việc FE
- `TopNav` một bộ cho mọi màn; `NewVideoButton` (đợt 43 mở hộp thoại).
- Trang Kênh: bỏ `ChannelVideosTab`, `AppliedSummary`, `RivalSchedule`, `AutonomyPresets`, `ChannelPriority`, `LearnedRules`, `TemplateRotationList`. Thêm `AutomationCard` (3 dòng) và `DefaultTemplateCard` (`Select`/`Popover` không đẩy nội dung).
- Thư viện: route riêng `/library` trên thanh trên; bỏ `LibrarySideNav`.

### Ảnh hưởng tới backend
- Kênh: `defaultTemplateId` (bắt buộc) thay danh sách luân phiên; `autoApprove: boolean` (ngưỡng 85 cố định) thay `autonomy` (pick/approve/onError/below + ngưỡng); `autoFetch: boolean`; `perDay` 1–20.
- Bỏ: `priority` (steady/views), `learned_rules`, số liệu sau đăng, lịch đăng đối thủ theo kênh (lịch vẫn có ở Khám phá).
- Đổi template mặc định áp cho job tạo sau thời điểm đổi; job đang chạy giữ template cũ.
- Dữ liệu cũ: kênh có luân phiên thì lấy template đầu danh sách làm mặc định; `autonomy.approve = score|all` → `autoApprove = true`.

### Nghiệm thu
- Kênh: bấm Chuyện Lạ Đêm Khuya ở cột trái → số liệu, đối thủ (Hồ Sơ Bí Ẩn), template (Truyện kể đêm), hồ sơ đổi theo; Đổi template → danh sách nổi, chọn → toast; Tự duyệt ảnh bật → chip ở đầu trang.
- Không còn chữ “luân phiên”, “Mức tự chủ”, “Ưu tiên kênh” ở máy tính.
- Thư viện mở từ thanh trên, mục Thư viện sáng; Sửa template cũng sáng Thư viện.
- Bấm thử Kênh, Thư viện, Template: 0 nút không phản hồi.

## Gói V39 — Làm video thành hộp thoại, Khám phá nối hộp, Hàng chờ gọn (P1) <!-- r43 --> <a id="goi-v39"></a>

**Vì sao:** đợt 43 của workflow gọn. Video mới là một trang riêng chỉ để dán link; người dùng phải rời màn đang làm. Hộp thoại Nhập nhanh làm việc này tại chỗ.

### Màn và artboard
- **2.1 Làm video** (`NewVideo`, nền Bàn dựng) — 01 › W4 (viết lại): ô dán một/nhiều link, thẻ xem trước hoặc danh sách, Kênh + Template (danh sách nổi; nhãn “mặc định” / “chỉ video này”), dòng kế hoạch, Làm video / Làm N video. Tweak `input`, `from=topbar|explore`, `picker`.
- **1.1 Khám phá** (`Explore`): Làm N video và Làm video cho <kênh> mở hộp; chip lọc + Giới hạn quét là danh sách nổi; Top 10/20 chọn thật; menu ⋯ đối thủ nối đủ; bỏ Vì sao? (tweak `why`); xem trước ghi template mặc định.
- **2.2 Hàng chờ** (`Queue`): Quy tắc chạy còn Luồng chạy cùng lúc + Nhịp kênh; template trong Chờ lượt theo mặc định của kênh.

### Việc FE
- `NewVideoDialog` (Nhập nhanh) dùng chung; mở bằng nút thanh trên, phím N, Khám phá (truyền danh sách video đã chọn), Hàng chờ trống, Bàn dựng chưa có video. Route `/new` (nếu còn) chuyển sang mở hộp trên trang trước đó.
- Một `textarea` nhận một hoặc nhiều link (tách theo dòng/khoảng trắng); đọc từng link song song, hiện trạng thái từng dòng.
- `Select` Kênh / Template: `Popover` nổi, không đẩy nội dung.
- Bỏ: trang `NewVideo` cũ, `ModeTabs` (Một link / Nhiều link), công tắc Ưu tiên cả N, khối gợi ý.
- Khám phá: bỏ `ExplainPopover` (Vì sao nổi trội?).

### Ảnh hưởng tới backend
- `POST /api/jobs` nhận `templateId` tuỳ chọn cho từng lần thêm (mặc định = `defaultTemplateId` của kênh); không còn cờ ưu tiên — job người dùng thêm luôn vào phần đầu hàng.
- Kiểm tra trùng theo (video nguồn, kênh) trước khi tạo; trả về job đã có để mở ở Bàn dựng.
- Quy tắc chạy: chỉ còn `workers` (1–6) cho người dùng chỉnh; `imageConcurrency` (3) và chu kỳ quét (1 giờ) cố định.

### Nghiệm thu
- Từ Kênh bấm Làm video → hộp mở trên Kênh; dán 4 link trong đó 1 link đã làm → “Làm 3 video”; đổi template → nhãn “chỉ video này”; bấm Làm → toast, hộp đóng, vẫn ở Kênh.
- Khám phá: chọn 3 video → Làm 3 video → hộp mở với 3 video, không có ô dán.
- Hàng chờ: chỉ còn 2 điều khiển ở Quy tắc chạy.
- Bấm thử `NewVideo`, `Explore`, `Queue`: 0 nút không phản hồi (trừ phần nằm dưới hộp thoại).

## Gói V40 — Điện thoại theo workflow, 0 nút không phản hồi (P1) <!-- r44 --> <a id="goi-v40"></a>

**Vì sao:** đợt 44. Máy tính đã gọn ở đợt 41–43 nhưng điện thoại còn phần đã bỏ (Bản tin, Vì sao?, học quy tắc, luân phiên template, thẻ YouTube Data API) và nhiều nút bấm không có phản hồi; nhóm chọn có viên trượt lệch vài px.

### Màn và artboard
Chi tiết từng màn ở 01 › §6.2 (khối Đợt 44). Board: `MMain`, `MNew`, `MChannel`, `MExplore`, `MCompVideo`, `MReview`, `MQueue`, `MLibrary`, `MTemplate`, `MSettings`, `MSystem`, `MAuth`, `MCreate`. Máy tính sửa kèm: `Settings` (bỏ thẻ YouTube Data API), `Review` (bỏ Vì sao?, học quy tắc), `CreateChannel` (nút Theo dõi & chọn).

### Việc FE
- Điện thoại: mọi danh sách thả, menu ⋯ và chi tiết phụ là `BottomSheet` (scrim đóng được, kể cả khi mở sẵn); kết quả là `Toast` đặt trên thanh dưới / thanh thao tác; xoá, đăng xuất dùng `ConfirmSheet` nói hệ quả (số GB, việc bị ảnh hưởng).
- Bỏ khỏi điện thoại: `RecapCard`, `SuggestionStrip` (đề xuất thành dòng trong Chờ bạn), `WhyButton`/`ExplainSheet`, `LearnPrompt` (Ghi chú đã học), `AssistantComposer` và hai sheet trợ lý ở `MSystem`, `YoutubeDataKeyCard` (cả máy tính).
- `Segmented` có viên trượt: đo lại bằng `ResizeObserver` trên nhóm và từng nút, và khi `document.fonts` tải xong — không chỉ lúc mount.
- Tab Thư viện điện thoại: 4 cột bằng nhau, không cuộn ngang; số lượng chuyển xuống mô tả tab.
- Template trong Hàng chờ, Thư viện, Sửa template, Video đối thủ lấy từ `defaultTemplateId` của kênh (MT, QH: Mẫu chuẩn · Tách nền; CL: Truyện kể đêm).

### Ảnh hưởng tới backend
- Không thêm API. Bỏ `/api/performance/data-key` (khoá YouTube Data API) và phần prompt `learned` khỏi prompt đã gửi.
- Đăng xuất một nhà cung cấp gói (ChatGPT, Grok): trợ lý không dùng nhà cung cấp đó; ảnh đang vẽ chuyển nguồn dự phòng nếu bật, không thì chờ.

### Nghiệm thu
- Bấm thử tự động 13 board điện thoại ở mọi giá trị tweak: 0 nút không phản hồi (trừ phần nằm dưới sheet đang mở), mọi sheet nằm trong màn, không tràn ngang.
- Nhóm chọn có viên trượt: lệch ≤ 2px so với nút đang chọn sau khi font tải xong.
- Không còn chữ “luân phiên” (template), “Mức tự chủ”, “Vì sao?”, “Ghi chú đã học”, “YouTube Data API” ở màn điện thoại.

## Tổng hợp backend

| Gói | Thay đổi DB/API |
|---|---|
| V0 | `clone_jobs.paused_at`; `pause`/`resume`; `GET /api/status`; `agentView`, `subline` |
| V1 | `pickDefaultJob`; nhóm `paused`; `exportBytes` |
| V2 | `clone_queue.priority`; scheduler `pick` trước; nhịp chỉ đếm `batch`; `plan`, `enqueue`; CLI `--priority` |
| V4 | `bulkEligibility` |
| V6 | `GET /api/queue` mở rộng; chuyển 3 khoá thiết lập |
| V8 | `GET /api/templates/overlays` |
| V9 | tạo kênh trả `identityJobId` |
| V10 | LAN + ghép thiết bị bằng QR, cookie phiên (D1 cách a) |
| V11 | `assistant/parse`, `assistant/run`; bộ lọc Tự lấy `minDurationSec` |
| V12 | `GET /api/health`; đếm trước cho hộp thoại xoá |
| V13 | `ranges` cho các mục chưa đạt độ hoàn thiện; `GET /api/editor/:id/quality`, `POST …/captions/align` (khớp mốc từng từ), `POST /api/clone/:id/measure`, export `accept` | <!-- r2-backend --> <!-- r27-editor -->
| V14 | `background_tasks`, `notifications`, `GET /api/activity`, thông báo hệ thống + dock badge |
| V15 | `image_prompts` + API, seed 7 prompt có sẵn, recipe giữ `style` + thêm `scenePromptId`/`coverPromptId`, `coverPrompt`, migration | <!-- r7-prompts -->
| V16 | `clone_jobs.scene_prompt_id`/`cover_prompt_id` (overlay trong `resolveJobRecipe`, cùng `no_images`), `POST /api/clone/:id/prompts/plan`, `PATCH /api/clone/:id/prompts` | <!-- r8-pm -->
| V4 (đợt 9) | `owner_channels.language`, `description_footer`, `always_tags`; branding `part`/`brief`/`direction` + trạng thái `failed`; `PATCH/rewrite metadata` | <!-- r10-channel -->
| V22 | `YTF_PROJECTS_DIR`, `storage.json`, `GET/POST /api/storage/*`, đường dẫn tương đối trong DB, theo dõi ổ | <!-- r17-storage -->
| V18 | `reason` (§17), `GET /api/recap`, `lastSeenAt` | <!-- r16-agent -->
| V19 | `suggestions`, `learned_rules`, bộ sinh đề xuất, phát hiện kiểu sửa |
| V20 | `owner_channels.autonomy`/`priority`, `image.score`, `retry_once`, `fix_then_ask`, `dailyPaidImageLimit` |
| V21 | `posted_stats`, `insights`, `POST /api/clone/:id/posted { youtubeUrl }` |
| V23 | registry `platforms/`, `backdrop_videos`, template `fixedVideoIds` + migration clip cố định, `POST /api/sources/parse`, `GET /api/platforms`, `platform_requests` | <!-- r22-sources -->
| V24 | `POST/GET /api/platforms/:id/login/qr`, `POST …/login/refresh`, `PUT/DELETE …/login`; lưu cookie + refresh token; tự gia hạn trước khi quét; đăng nhập xong xoá giờ chờ bị chặn | <!-- r24-biliqr -->
| V25 | `GET /api/storage/server`, `POST …/test` (+ `probe` từ trình duyệt), `PUT …/config`, `POST …/migrate` (+ pause/resume/cancel/retry); BackgroundTask `storage_migrate`; khoá bí mật qua `secretbox`; bộ nhớ đệm làm việc; `OpStat` từ lần khởi động; `storageDown` trong `/api/status` | <!-- r29-s3 -->
| V26 | `apps: AppLink[]` trong `GET /api/me` và 403 `app_access_denied`; 401 `session_ended` có `reason`, `app` | <!-- r30-sso -->
| V27 | Không API mới; dùng `pausedAt`, `waitReason`, `PATCH /api/queue` v2 (#52) và phiên thiết bị `GET /api/me/sessions` | <!-- r31-v27 -->
| V28 | Không API mới; `/api/events` (SSE) đã có (#8) | <!-- r32 -->
| V29 | `ImageVersion` (prompt đã gửi theo phần, refs, provider, thời gian), `POST …/images/redraw` (`feedback`, `parts`, `refs`), `…/use`, `…/cancel`, SSE `image.progress` | <!-- r33-img -->
| V30 | `displayPath` (đã có, #78); chỗ trống theo ổ đang giữ thư mục lưu dự án, `null` khi ổ không cắm | <!-- r34 -->
| V31 | Không đổi API (chỉ giao diện toast) | <!-- r35 -->
| V32 | Tiến độ trung bình các việc đang chạy cho vòng của nút Việc nền (có thể tính ở FE từ `status`); số thông báo chưa đọc (đã có `attention`) | <!-- r36 -->
| V33 | Không đổi API (chỉ giao diện) | <!-- r37 -->
| V34 | Không đổi API; Khám phá cần gọi lại danh sách khi bấm Thử lại; Tạo kênh trả kênh dù Đạo diễn lỗi (tên tạm) | <!-- r38 -->
| V35 | Không đổi API (chỉ giao diện điện thoại) | <!-- r39 -->
| V36 | Không đổi API ở đợt 40; đợt 43 cần điểm mở hộp Làm video từ mọi màn (chỉ FE) | <!-- r40 -->
| V11 (đợt 16) | `assistant/plan`, `assistant/run-plan`, intent `prioritize`/`set_autonomy`/`add_rule`/`make_videos` |
| V17 | `ai_providers` (+ khoá trong keychain), danh mục model, `GET/PUT /api/ai/agents`, `clone.imageProvider`/`imageFallback` → `artist.draw`/`drawFallback`, gọi model qua adapter (Codex, Grok CLI, OpenAI-compatible, Gemini, Ollama) | <!-- r8-ai -->
<!-- r3-dev -->
<!-- r4-slots -->
