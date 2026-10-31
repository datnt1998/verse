# Viderse — Design v1 (bàn giao cho dev)

Bộ thiết kế chính thức của Viderse: vận hành nhiều kênh YouTube bằng bốn trợ lý (Trinh sát, Đạo diễn, Hoạ sĩ, Kiểm định), dựng trên kiến trúc web của Noverse và kit `@noverse/ui`. Thư mục này là **nguồn sự thật cho hành vi và giao diện**; mọi cập nhật thiết kế và trao đổi với dev diễn ra ở đây.

- Canvas thiết kế (bấm được, có tweak): https://claude.ai/artifact/FX1i2msq4xn9L9HdSrPmw6 — phiên bản 57. Link đang riêng tư; chủ canvas cần chia sẻ cho team.
- Bản sao mã nguồn từng màn nằm trong `artboards/` để tra giá trị chính xác và mở trạng thái bằng tweak.
- Viderse trước đây là giao diện mới của yt-factory; trong verse được dựng lại bằng Go + Postgres + React. Đọc [02-stack-verse.md](02-stack-verse.md) trước khi làm.

## Đọc theo thứ tự

| Tệp | Nội dung |
|---|---|
| `01-ia-flows.md` | **Logic sản phẩm**: mô hình Assets tập trung, đối tượng, hành trình A→Z, vòng đời video, luật chạy, điều hướng, workflow W1–W15, danh sách đã cắt |
| `02-stack-verse.md` | **Dựng trong verse**: phần nào vẫn là chuẩn, brand (`defineBrand`), component Viderse → `@noverse/ui`, route hash, cách xem artboard |
| `03-tokens.md` + `tokens.css` | Màu, chữ, bo góc, khoảng cách, chuyển động, màu trạng thái và trợ lý |
| `04-components.md` | Component và luật dùng: trạng thái, a11y, chỗ dùng, artboard tương ứng |
| `05-kit.md` + `kit.css` | **UI Kit** = kit Noverse đổi theme màu: component → lớp `.vv-*` → board `Kit`/`MKit`; thành phần riêng Viderse |
| `06-copy.md` | Thuật ngữ chuẩn (dùng / không dùng), mẫu câu, mã lỗi → câu |
| `07-data-contract.md` | Dữ liệu và API mà UI cần (§1–§24.1) |
| `08-dev-prompt.md` | **Gói việc** V0–V24: việc FE, backend, nghiệm thu |
| `09-migration-yt-factory.md` | Bản đồ route, thành phần, thiết lập đổi so với giao diện yt-factory cũ — tham chiếu khi port logic |
| `10-gap-closure.md` | Dev ghi gói đã xong |
| `11-hoi-dap.md` | **Hỏi đáp design ↔ dev**: câu hỏi mở, đề nghị đổi, quyết định |
| `12-brand.md` + `brand/` | Logo ống kính, biểu tượng app, favicon, logo dài, luật dùng |
| `13-changelog.md` | Lịch sử các đợt thiết kế (mới nhất ở trên) |
| `icons/` | Icon Iconsax (nét Linear 24px, bản đặc `-b`) dạng SVG, cùng bộ với Noverse |
| `img/` | Ảnh minh hoạ JPG 1280×720 dùng trong artboard (đóng vai ảnh Hoạ sĩ vẽ, thumbnail, clip nền) |
| `artboards/` | Mã `.dc.html` của 28 artboard (18 máy tính, 10 điện thoại) + `canvas.json` + `support.js`; bảng mọi tweak ở `artboards/README.md` |

## Nguyên tắc

1. **Bốn việc của người dùng:** biết việc chờ mình, đưa video vào làm, tin máy tự chạy, lấy video đi đăng. Mọi màn phục vụ một trong bốn việc này.
2. **Chỉ dừng khi cần người:** duyệt ảnh và sự cố. Còn lại là thông tin.
3. **Một hành động bắt đầu: “Làm video”.** Video vào một hàng chờ chung; video bạn chọn đi trước, hàng loạt theo nhịp kênh.
4. **9 bước máy được nói thành 4 trợ lý:** Trinh sát, Đạo diễn, Hoạ sĩ, Kiểm định.
5. **Một trạng thái, một hành động chính** cho mỗi video tại mỗi thời điểm. Tên trạng thái giống nhau ở mọi nơi.
6. **Một nút chính mỗi màn** (nút sáng, như Noverse). Đỏ là màu nhấn (đang chọn, đang bật, focus); cam là lỗi và xoá; vàng là tới lượt bạn. <!-- r19-kit -->
7. **Mỗi thiết lập có một nhà.** Nơi khác chỉ hiển thị và dẫn link.
8. **Nói trước khi tốn:** giờ bắt đầu dự kiến, số ảnh sẽ vẽ, phần sẽ làm lại.
9. **Bị khoá thì nói vì sao**, ngay cạnh chỗ bị khoá.
10. **Không lộ kỹ thuật** ngoài “Chi tiết”.
11. **Hình gọn:** avatar luôn tròn; đang chọn là nền đậm hơn, không bao giờ là vạch màu một bên; trích dẫn là khối nền nhạt. Luật và lệnh kiểm tra: [04-components › Luật hình](04-components.md#c-shape-rules). <!-- r26-rules -->

## Cách làm việc

- **Design cập nhật thiết kế:** sửa canvas, xuất artboard và tài liệu vào thư mục này, ghi một mục ở `13-changelog.md` (đợt, ngày, thay đổi, phiên bản canvas).
- **Dev hỏi hoặc thấy lệch:** ghi vào `11-hoi-dap.md` trước khi tự đổi. Design trả lời ngay trong mục và sửa tài liệu gốc cùng lúc.
- **Dev làm xong một gói:** ghi vào `10-gap-closure.md`.
- **Thứ tự ưu tiên khi các nguồn lệch nhau:** `01-ia-flows.md` (hành vi) → artboard (giao diện, giá trị) → các tài liệu còn lại. Thấy lệch thì ghi hỏi đáp.

## Quyết định cần chủ sản phẩm chốt

| # | Câu hỏi | Đề xuất | Ảnh hưởng |
|---|---|---|---|
| D1 | Điện thoại vào app bằng cách nào khi app chỉ nghe `127.0.0.1`? | Nghe trên LAN, ghép thiết bị một lần bằng mã. Trong verse đã có ghép thiết bị bằng QR (`platform/identity`) | Gói V10 backend |
| D2 | Nhắn trợ lý v1 gồm những intent nào? | Bảng ở Gói V11 (10 intent, luôn xác nhận) | Gói V11 |
| D3 | ~~Đổi tên app, cửa sổ desktop và icon thành “Viderse”? CLI giữ `ytf`?~~ | Hết hiệu lực: trong verse app tên `viderse` từ đầu | — |
| D4 | ~~DESIGN.md cũ của yt-factory~~ | Hết hiệu lực: trong verse thư mục này là chuẩn thiết kế duy nhất | — |
| D5 | Lấy số liệu sau đăng từ đâu? | Khoá YouTube Data API tuỳ chọn ở Kết nối AI; không có khoá thì Trinh sát đọc trang công khai như khi quét đối thủ | Gói V21 | <!-- r16-agent -->
| D6 | Ngưỡng Duyệt theo điểm mặc định và có cho “Tự duyệt hết” không? | 85; có, nhưng chỉ qua Tuỳ chỉnh và luôn hỏi xác nhận | Gói V20 |
| D7 | Máy được tự tiêu ảnh trả phí bao nhiêu mỗi ngày? | Mặc định 50 ảnh/ngày cho mỗi nhà cung cấp trả theo ảnh; 0 = không tự tiêu | Gói V20 |
| D8 | Yêu cầu hỗ trợ nền tảng mới (bấm ở Thư viện › Video nền) gửi về đâu? | Ghi trên máy, gửi kèm khi người dùng Báo lỗi; chưa gửi thẳng lên máy chủ | Gói V23 | <!-- r22-sources -->
