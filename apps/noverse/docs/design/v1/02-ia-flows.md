# 02 · Kiến trúc thông tin & user flow

## Mô hình dữ liệu (cấp tổ chức)

```
Thư viện (--root, vd ~/Truyen)
└── Dự án (kênh / series)            ← project.json, .ainovel/config.json, .ainovel/rules/
    ├── Ý tưởng (y-tuong/*.md)
    ├── Tài sản (tai-san/)
    └── Truyện (kich_ban/NN-slug/)   ← story.json, output/novel/ (engine), xuat-ban/
        └── Tập (volume)             ← engine: layered → Tập · Hồi · Chương; phẳng → 1 tập
            └── Chương
```

Hiện tại `serve --root` coi **mỗi thư mục con của root là một truyện** (`/api/books`). v1 cần backend chuyển sang 3 cấp (xem `06-data-contract.md` › GAP-1). Trong lúc chờ, frontend coi root hiện tại là **một dự án ẩn danh** và `books` là danh sách truyện của dự án đó.

## Route (hash, giữ cách của `App.tsx`)

| Route | Màn | Artboard |
|---|---|---|
| `#/` | Dự án (tổng quan dự án mặc định) | `Outline.dc.html` (view `project`) |
| `#/p/:project` | Dự án | `Outline.dc.html` (view `project`) |
| `#/p/:project/s/:story` | Dự án › Truyện (tab Đại cương mặc định) | `Outline.dc.html` (view `story`) |
| `#/p/:project/s/:story/work` | Workspace | `Main.dc.html` |
| `#/p/:project/s/:story/activity` | Nhật ký tiến độ đầy đủ | `Activity.dc.html` |
| `#/p/:project/s/:story/read/:chapter?` | Trình đọc & duyệt | `Reader.dc.html` |
| `#/p/:project/new` | Tạo truyện mới | `NewStory.dc.html` |
| `#/settings/:section?` | Cài đặt (anchor theo mục) | `Settings.dc.html` |
| `#/login` | Đăng nhập (chọn tài khoản / mật khẩu / hết hạn) | `Login.dc.html` |
| `#/pair/:code` | Xác nhận đăng nhập điện thoại bằng QR | `Phone.dc.html` |
| `#/p/:project/s/:story/audio/:volume?` | Audio · bản đọc (duyệt & chốt) | `Transcript.dc.html` |
| `#/p/:project/s/:story/audio/:volume/make` | Audio · xác nhận & đang tạo | `AudioExport.dc.html` (confirm · rendering) |
| `#/p/:project/s/:story/audio/:volume/listen` | Audio · nghe, đọc lại đoạn, ghép & xuất | `AudioExport.dc.html` (listen · exported) |
| `#/help/commands/:name?` | Hướng dẫn lệnh (mặc định: Cách dùng lệnh). Mở bằng `/help`, phím `?`, nút Hướng dẫn | `Commands.dc.html` |
| *(popover trong Workspace)* | Gõ `/` trong ô chat: gợi ý & kiểm tra lệnh | `Slash.dc.html` |
| *(lớp phủ ở app shell, mọi màn)* | Dock việc chạy nền góc dưới phải; `⌥J` mở/đóng | `Jobs.dc.html` |
| `#/welcome/:step?` | Thiết lập lần đầu (quản trị 8 bước · người viết/đọc 4 bước) | `Onboard.dc.html` |

Menu tài khoản, QR, Thiết bị & bảo mật, Quản lý tài khoản là hộp thoại phủ lên route hiện tại (`?dlg=qr\|devices\|users`), xem `Login.dc.html` › menu/qr/devices/users.

Tương thích ngược: `#/book/:id` → redirect `#/p/_/s/:id/work`.

Nav chính (header): **Workspace · Dự án · Trình đọc · Truyện mới**, cộng nút Cài đặt (icon) và avatar. Workspace/Trình đọc disabled nếu chưa chọn truyện.

## User flows

Ký hiệu: → hành động người dùng, ⇢ phản hồi hệ thống, `API` gọi máy chủ.

### F1. Mở ứng dụng
1. `GET /api/me`. 401 ⇢ `#/login` (A1). Đăng nhập bằng tài khoản web (`POST /api/login`), phiên 30 ngày. Chi tiết: `09-auth-onboard.md`.
2. Tài khoản chưa thiết lập (GAP-15) ⇢ `#/welcome` (A3). Xong ⇢ route lưu trong `ainovel.next` hoặc `#/`.
3. `GET /api/books` ⇢ danh sách (đã lọc theo quyền tài khoản). Không có truyện ⇢ màn Dự án, trạng thái rỗng, CTA “Truyện mới” (ẩn với Người đọc).
4. 401 bất kỳ lúc nào ⇢ lưu route ⇢ `#/login` trạng thái **Phiên đăng nhập đã hết hạn**. Mất mạng chỉ hiện banner (Dialogs › offline).
5. Điện thoại: quét QR ⇢ `#/pair/:code` (A2) ⇢ xác nhận ⇢ `POST /api/pair`.

### F2. Chuyển dự án / truyện / tập
1. → Breadcrumb thanh trên `Tên dự án ▾` ⇢ popover dự án (danh sách + “Dự án mới”; “Đổi thư mục thư viện” chỉ trong Nâng cao). Đổi dự án mở Tổng quan dự án đó. <!-- ag-f2 -->
2. → Thẻ truyện ở đầu cột trái màn Dự án ⇢ popover truyện của dự án hiện tại (Tổng quan dự án · các truyện · Truyện mới) ⇢ view truyện; → “Mở Bàn viết”. Cột trái chỉ hiện mục lục của truyện đang mở (Gói AG), nên không phải đóng tập nào để đổi truyện.
3. Chuyển truyện khi truyện hiện tại đang chạy: **không dừng** truyện cũ (worker riêng); breadcrumb hiện chấm tím ở truyện đang chạy.

### F3. Tạo truyện mới (Bắt đầu nhanh)
1. → Màn Truyện mới (trong dự án đang chọn) → gõ yêu cầu (≤5000 ký tự) hoặc bấm 1 trong 3 ví dụ.
2. ⇢ Chip “Thư mục: `kich_ban/NN-…`” hiện tên tạm. Nếu người dùng nhập tên làm việc → slug từ tên đó.
3. → `Enter`/“Bắt đầu viết” ⇢ `POST /api/books {name}` (tạo thư mục) ⇢ `POST /api/books/:id/input {text}` (action `start`) ⇢ chuyển Workspace.
4. ⇢ Nhật ký: `Yêu cầu sáng tác…` → `ARBITER (Phán quyết khởi động)` → `Bắt đầu sáng tác (người lập kế hoạch: architect_long — …)` → `ARCHITECT …`.
5. Khi `snapshot.bookTitle` có giá trị và thư mục còn tên tạm ⇢ toast “Đổi tên thư mục theo tên truyện?” (GAP-9).

### F3b. Đồng sáng tác (GAP-6)
Tab → chế độ Đồng sáng tác ⇢ hội thoại + gợi ý ❶❷❸ + “Bản nháp yêu cầu” ⇢ `Ctrl+S` gửi bản nháp như F3.3.

### F4. Theo dõi & can thiệp khi đang viết (Workspace)
1. SSE `snapshot` cập nhật chip trạng thái, Agent tab, chi phí, cache, đại cương.
2. SSE `event` thêm/cập nhật dòng nhật ký (theo `id`).
3. SSE `delta` nối vào luồng viết; `clear` xóa; parser tách round theo `✻`.
4. → Gõ can thiệp + Enter ⇢ `POST /input` (action `steer`) ⇢ nhật ký `[Can thiệp] …` → `ARBITER (Phán quyết can thiệp)` → `Phán quyết: …`.
5. → Gõ `/` ⇢ command palette (F8).
6. → `Esc` / Tạm dừng ⇢ `POST /abort` ⇢ trạng thái Đã tạm dừng; viền chạy và shimmer dừng.
7. → Tiếp tục ⇢ `POST /resume`. Hoặc gõ nội dung bất kỳ ⇢ `POST /input` (action `continue`).

### F5. Nghiệm thu từng chương
1. → `/review on` hoặc Cài đặt › Sáng tác ⇢ `POST /review {on:true}`.
2. Engine viết xong chương ⇢ `snapshot.advanceMode=review`, chờ ⇢ thẻ vàng “Nghiệm thu từng chương · chờ bạn duyệt”.
3. → `/next` ⇢ `POST /next`; hoặc gõ ý kiến sửa ⇢ `POST /input`.

### F6. Viết lại / gọt giũa
1. Kết quả thẩm duyệt `polish`/`rewrite` hoặc can thiệp ⇢ `snapshot.pendingRewrites=[…]`, `flow=rewriting|polishing`.
2. ⇢ Chip Quy trình “Viết lại”, chip khôi phục `recoveryLabel`, tab Agent hiện hàng đợi, Đại cương đánh dấu chương chờ.
3. Từ Trình đọc → “Viết lại chương N” ⇢ điền sẵn `viết lại chương N` vào ô nhập (người dùng vẫn bấm gửi).

### F7. Hoàn thành, xuất bản, viết tiếp
1. `snapshot.runtimeState=completed` ⇢ chip Hoàn thành; nút `/export`, `/reopen`.
2. → `/export` ⇢ chọn định dạng ⇢ `GET /export?format=txt|epub` tải về. Xuất tự động vào `xuat-ban/` là GAP-4.
3. → `/reopen` ⇢ hộp thoại Viết tiếp tập mới ⇢ `POST /reopen {direction}` ⇢ Tập mới xuất hiện ở cây Dự án.

### F8. Command palette
- Mở: gõ `/` ở ô nhập trống hoặc `Ctrl/⌘+K`. Lọc theo lệnh và mô tả. `↑↓` chọn, `Tab/Enter` chấp nhận, `Esc` đóng.
- Lệnh có API → thực thi trực tiếp hoặc mở hộp thoại; lệnh điều hướng (`/model`, `/config`) → mở Cài đặt đúng mục; lệnh GAP → hiện nhãn “Chưa có API”, disabled.

### F9. Dự án mới
→ Menu dự án › Dự án mới ⇢ hộp thoại: Tên hiển thị → slug tự sinh (kiểm tra hợp lệ + trùng) → xem trước cây sẽ tạo → Tạo (GAP-1).

### F10. Cấu hình
Cài đặt chọn phạm vi lưu **Dự án** (`<dự án>/.ainovel/config.json`) hoặc **Toàn cục** (`~/.ainovel/config.json`). Thay đổi hiện thanh “Có thay đổi chưa lưu” neo đáy. Đọc/ghi là GAP-5.

### F11. Lỗi & kết nối
- SSE lỗi ⇢ banner “Mất kết nối máy chủ · tự kết nối lại sau Ns” (backoff 1s → 15s như `useBookEvents`), nút Thử ngay.
- 409 khi mở truyện ⇢ hộp thoại “Truyện đang được mở ở nơi khác” (Chỉ đọc / Thử lại).
- Sự kiện `ERROR` hoặc `failed` ⇢ dòng đỏ trong nhật ký; retry (`retryAt`) ⇢ đếm ngược “Thử lại sau Ns”.
