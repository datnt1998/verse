# 05 · Screens

Kích thước chuẩn 1440×900. Tối thiểu desktop 1100×700. Mọi màn dùng `AppShell` (không cuộn trang ở desktop).

## 00 · Tạo truyện mới — `NewStory.dc.html`

- **Bắt đầu nhanh** (mặc định): cột giữa `max-width: 760px`, căn giữa dọc. Thứ tự: OrbCluster lớn (trôi nhẹ) → H1 “Bạn muốn viết câu chuyện nào?” → mô tả → `ModeSwitch` → `PromptBox` → “Thử một ý tưởng” (3 thẻ) → “Điều sẽ diễn ra” → `AltEntries`. Dòng phím tắt đáy.
- **Đồng sáng tác**: chia 2 cột — hội thoại (bám đáy, gợi ý ❶❷❸, ô chat) | `DraftPanel` 360px (trạng thái Đang làm rõ/Sẵn sàng viết, nút Bắt đầu sáng tác + Ctrl S).
- Dữ liệu: dự án hiện tại; số thứ tự truyện kế tiếp; config `style`, `language`, `model`.
- Trạng thái lỗi: khởi tạo thất bại → banner đỏ “Khởi tạo thất bại, vui lòng kiểm tra cấu hình hoặc dùng /model để đổi model”.

## 01 · Workspace — `Main.dc.html` (tweak `phase`: rewrite · gate · done)

```
┌ Header 48 ───────────────────────────────────────────────────────────────┐
├ Đại cương 236 ┬──────────── Cột giữa (flex) ─────────────┬ Inspector 340 ┤
│ tiến độ       │ Tiêu đề · [Tạm dừng Esc] | [/export][/reopen]│ Agent│Chi phí│Truyện
│ 8 chương      │ chips: trạng thái·giai đoạn·quy trình·…  │                │
│ (cuộn)        │ [GateCard khi nghiệm thu]                │ (cuộn)         │
│ số chữ        │ Nhật ký tiến độ   (flex 4, cuộn, bám đáy) │                │
│ bản nộp       │ Luồng viết trực tiếp (flex 6, cuộn, bám đáy)│               │
│               │ Composer (neo) + ShortcutHints            │                │
└───────────────┴──────────────────────────────────────────┴────────────────┘
```

| Trạng thái | Điều kiện (`Snapshot`) | Khác biệt UI |
|---|---|---|
| Đang chạy | `isRunning` | StatusChip spinner tím; RunRing; orb live; ThinkingLine; round cuối có spinner |
| Đã tạm dừng | `runtimeState=paused` | Chip vàng “Đã tạm dừng”; không animation; nút Tiếp tục; placeholder “Sáng tác đã tạm dừng, nhập nội dung bất kỳ để tiếp tục” |
| Viết lại / gọt giũa | `flow=rewriting\|polishing`, `pendingRewrites` | Chip Quy trình tím; chip khôi phục; Đại cương đánh dấu chờ; tab Agent có RewriteQueueCard |
| Nghiệm thu | `advanceMode=review` & chờ | GateCard; placeholder “Chờ nghiệm thu: Nhập ý kiến chỉnh sửa, hoặc /next để duyệt chương tiếp” |
| Hoàn thành | `runtimeState=completed` | Chip ok; nút `/export` `/reopen`; luồng viết rỗng; tab mặc định Truyện |
| Đang rảnh | `idle` | Chip trung tính “Sẵn sàng” |

- Hai panel bên ẩn/hiện bằng nút ở 2 góc header và phím `[` `]`; trạng thái nhớ trong `localStorage`.
- Mobile (<760px): header thu gọn (crumb rút còn tên truyện), cột giữa full width; Đại cương + Inspector chuyển thành **thanh tab đáy** phía trên Composer: `Nhật ký · Luồng viết · Agent · Chi phí · Truyện`; Composer `sticky`.

## 01b · Nhật ký tiến độ — `Activity.dc.html`

Trang cuộn được (ngoại lệ duy nhất cùng Cài đặt), `max-width: 920px`. Bộ lọc pill: Tất cả · Agent · Công cụ · Phán quyết · Hệ thống · Bạn · Lỗi (kèm số). Nút ẩn/hiện lệnh gọi công cụ. Nhóm theo phiên/chương. Dòng có thể kèm: khối trích (`sub`), đường dẫn + Mở thư mục, nút hành động (Thử lại ngay, /model, /next).

## 02 · Dự án — `Outline.dc.html` (tweak `view`: story · project)

> **Đã thiết kế lại** (bỏ cây thư mục, thêm “Đang áp dụng” và Tình trạng dự án): xem `20-settings-rules.md` §4. Phần dưới giữ để tham khảo dữ liệu.
>
> **Điều hướng (Gói AG, 28/10):** đổi dự án ở thanh trên, đổi truyện ở thẻ đầu cột trái, cột trái chỉ là mục lục của truyện đang mở. Xem `20-settings-rules.md` §4. <!-- ag-screens -->

```
├ ProjectTree 272 ┬──────── Nội dung (flex) ────────┬ FolderTree 360 ┤
```
- **view project**: form `project.json`, thẻ truyện (trạng thái, tập/chương/chữ, thể loại, bản xuất), kho ý tưởng, tài sản; Composer = “Ghi nhanh một ý tưởng…”.
- **view story**: đường dẫn mono + tiêu đề + “Mở Workspace”; Tabs:
  - Đại cương: card Tập N (số chương, kiểu đại cương, badge viết lại x/y, `output/novel/outline.json`), hàng chương mở rộng; card nét đứt “Tập N+1 · /reopen”.
  - Nhân vật: lưới thẻ (vai trò có màu tag).
  - Phục bút: danh sách `plant` (info) / `advance` (warn) / `resolve` (ok) + id + mô tả + chương.
  - Xuất bản: ExportMatrix + quy tắc xuất tự động + “Xuất Tập N ngay”.
  - Thông tin: form `story.json`.
- FolderTree luôn hiển thị; mục đang chọn tô accent nhạt.

## 03 · Trình đọc & duyệt — `Reader.dc.html`

```
├ ChapterPicker 236 ┬──── Toolbar + ReaderBody (Literata, 66ch) + Composer ────┬ Thẩm duyệt│Xuất 360 ┤
```
- Toolbar: “Chương N · số chữ”, tiêu đề, Bản nộp/Bản nháp, Thay đổi gần nhất, A−/A+ (14–24px, nhớ localStorage).
- Composer có QuickActions: Viết lại chương N · Gọt giũa chương N · Nhờ EDITOR thẩm duyệt · `/sync`.
- Thẩm duyệt: verdict (`accept` ok / `polish` warn / `rewrite` err), phạm vi (chapter/arc/global), hợp đồng (met/partial/missed), tóm tắt, 7 tiêu chí 0–100, IssueCard (type, severity critical/error/warning, description, evidence, suggestion, requires_change).
- Xuất: định dạng, từ/đến chương, ghi đè, nút xuất, lần xuất gần nhất.

## 03b · Góp ý theo đoạn — `Feedback.dc.html` · `MFeedback.dc.html` (28/10, Gói AI) <!-- ai-spec -->

Tweak máy tính `fb`: pending · sent · after · fixed · unsure · relink · history. Điện thoại `fb`: pending · sheet · sent · after · afterSheet · fixed · unsure · relink. Hai artboard dùng chung một bộ dữ liệu (cùng chương 8, cùng 5 góp ý, cùng vị trí).

Cách tạo góp ý giữ nguyên (chọn đoạn hoặc bôi đen câu → chip + ô nhập). Không bao giờ hiện số thứ tự đoạn: “Góp ý cho đoạn này”, “Cả chương”, câu trích.

### Vòng đời

| Trạng thái | Khi nào | Trong bài | Trên thẻ |
|---|---|---|---|
| **Chờ gửi** | mới tạo; hoặc “Chưa ổn”; hoặc vừa gắn lại; hoặc Người viết chưa sửa chỗ này | gạch chân tím, chấm tím ở lề | Xoá |
| **Đã gửi** | đã gửi, Người viết đang sửa | gạch chấm xanh; cả bài mờ nhẹ, dải “Người viết đang sửa…” | vòng quay nhỏ, không thao tác |
| **Đã sửa** | Người viết báo đã sửa, hoặc câu trích cũ không còn và tìm được đoạn thay thế | gạch xanh lá mờ ở câu mới | mở ra thấy **Trước / Sau** + *Ổn* · *Chưa ổn* |
| **Đã xử lý** | bấm *Ổn*; hoặc tự động (xem dưới) | không còn đánh dấu | dòng thu gọn “Đã xử lý · n” cuối ngăn |
| **Mất vị trí** | câu trích cũ không còn, không tìm được đoạn nào đủ giống | không đánh dấu | câu trích gạch ngang + *Gắn lại* · *Gửi lại* · *Xoá* |
| **Không chắc vị trí** | có 2–3 đoạn cùng giống | các đoạn gợi ý viền vàng, nhãn “Gợi ý 1/2” khi thẻ mở | danh sách gợi ý + *Gắn vào đây*; *Chọn đoạn khác* · *Gửi lại* · *Xoá* |

- **Đã sửa không bắt xác nhận từng thẻ.** Thẻ hiện ở nhóm “Đã sửa · n — Tự ẩn khi bạn duyệt chương sau”. Người dùng có thể bấm *Ổn* (ẩn ngay) hoặc *Chưa ổn* (mở lại thành Chờ gửi, giữ nội dung để sửa). Không làm gì thì tự thành Đã xử lý ở **lần duyệt kế tiếp của truyện** (thường là chương sau): luồng hiện tại duyệt *trước* khi Người viết sửa, nên không có lần duyệt lại chính chương đó để làm mốc.
- **Mất vị trí / Không chắc vị trí không bao giờ tự ẩn.**
- **Người viết chưa sửa chỗ này:** câu trích vẫn còn nguyên và Người viết không báo đã sửa → về Chờ gửi, nhãn “Người viết chưa sửa chỗ này”.
- **Gửi lại** góp ý mất vị trí: gửi cho cả chương, kèm câu trích cũ làm ngữ cảnh (nhãn “Gửi cho cả chương, kèm câu trích cũ”).
- **Gắn lại:** máy tính hiện dải vàng trên bài “Chọn chỗ mới cho góp ý … — bấm vào một đoạn, hoặc bôi đen một câu” + *Huỷ*; rê chuột lên đoạn hiện *Gắn góp ý vào đoạn này*. Điện thoại: đóng ngăn, dải “Chạm vào đoạn mới…”, chạm đoạn → nút *Gắn góp ý vào đoạn này*. Gắn xong → Chờ gửi, nhãn “Bạn đã gắn lại”.

### Khu “Cần gắn lại” ở đâu

Trong **ngăn Góp ý** (máy tính: tab *Góp ý* ở bảng phải, cạnh *Thẩm duyệt* · *Xuất*; điện thoại: sheet “Góp ý chương 8” mở từ nút nổi “Góp ý · n”). Là **nhóm đầu tiên** của ngăn, viền vàng. Không đặt ở đầu/cuối bài vì đó là chỗ đọc, và vì hai nền tảng phải cùng một chỗ.

Thứ tự nhóm: Cần gắn lại → Chờ gửi → Đã gửi → Đã sửa → Đã xử lý (thu gọn). Số trên tab/nút nổi: số cần gắn lại (vàng) nếu có, không thì số chờ gửi (tím). Danh sách chương bên trái cũng hiện số đó.

### Báo sau khi viết lại

- **Dải trên đầu bài** (không tự tắt, có ×): “Người viết đã viết lại chương 8 theo 5 góp ý” + các viên “2 đã sửa” · “1 chưa sửa” · “2 cần gắn lại”; bấm viên nào mở ngăn Góp ý đúng nhóm đó.
- Đang ở chương khi sửa xong: thêm toast “Đã viết lại chương 8 · 2 góp ý cần gắn lại” + *Xem* (điện thoại hiện ở trên, dưới thanh đầu).
- Đang ở màn khác: dùng thông báo/dock việc chạy nền như hiện có, cùng câu chữ.

### Đánh dấu “đã dời”

Có. Góp ý được máy tự gắn sang đoạn khác (không phải người dùng chọn): gạch chân **nét đứt**, chấm lề **rỗng** + chữ nhỏ “đã dời” (máy tính), thẻ có nhãn “Đã tự gắn sang đoạn mới”. Mất khi người dùng mở thẻ hoặc khi góp ý chuyển trạng thái. Góp ý gốc dùng gạch liền, chấm đặc.

### Chân ngăn

- Trước khi duyệt: **Duyệt & gửi n góp ý cho Người viết** — “Chương 8 được viết lại theo góp ý, rồi viết tiếp chương 9.”
- Sau khi viết lại mà còn Chờ gửi: **Gửi n góp ý · viết lại chương 8** — “Chỉ sửa chỗ có góp ý; phần còn lại giữ nguyên.”

## 04 · Cài đặt — `Settings.dc.html`

> **Đã thiết kế lại thành “Tuỳ chỉnh”** (3 cấp áp dụng, Quy tắc viết, ngăn Nâng cao cho quản trị): xem `20-settings-rules.md` §2–3. Phần dưới đã cũ.

Nav trái (anchor) + nội dung `max-width: 900px` cuộn nội bộ + `SaveBar` neo đáy. Mục: Thư viện & thư mục · Nhà cung cấp · Model & vai trò · Sáng tác · Ngân sách · Thông báo · Quy tắc viết · Hệ thống (chỉ đọc) · Giao diện ứng dụng (chỉ GUI). Giá trị và key: xem artboard + `06-data-contract.md` › Config.

## 05 · Lệnh & hộp thoại — `Dialogs.dc.html` (tweak `state`)

`palette` · `newProject` · `reopen` · `import` · `offline` (banner + màn nhập mã) · `locked`.
