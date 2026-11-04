# V37 · Bàn dựng gọn — nghiệm thu đang thực hiện

Ngày: 04/11/2026. [PR #4](https://github.com/datnt1998/verse/pull/4) **nháp, chưa merge, chưa deploy**.

## Màn → tình trạng

| Màn/phần | Đã sửa và kiểm | Còn lệch/chưa nghiệm thu |
|---|---|---|
| Main desktop | Hai tab, mặc định Thông tin khi đổi video, giữ tab khi SSE nối lại/ẩn bảng; player thật; ZIP; chọn video/phạm vi | Matrix trạng thái mở rộng đang chạy; clip nền phủ kín chưa có dữ liệu thật |
| Editor | Project QA bản sao: phát/dừng/tua, caption, timeline, split/delete/undo/redo, phím tắt/focus | Không tuyên bố mọi tổ hợp tweak đã qua |
| Template | Menu nổi desktop, Sheet điện thoại; không đẩy nội dung; viewer đọc media đã lưu | Single default và màn kênh/thư viện thuộc V38 |
| Việc nền / Thông báo | Popover đóng ngoài/Escape; global suggestions lưu/readback/reload/khôi phục; mất kết nối có dải giải thích và khoá thao tác, Thử lại ngay/nối lại thật | Chưa có bằng chứng Safari/iPhone vật lý |
| Main điện thoại | Metadata/clipboard/bốn trợ lý/nhật ký thật | Layout/mobile đầy đủ thuộc V40; không gồm viewer điện thoại round45 |
| Backend | Ba loại đề xuất; bỏ đường recap/assistant/learned đang hoạt động; migration dừng mixed/scoped | Release phải kiểm lại dữ liệu trong cửa sổ quiesced |

## Bằng chứng đã qua

- Source đã push: `5d316eabc0875a71ad380a55a7e582ab17d14cd6`, `b493792d689095a20e01992c40c76462348e0882`, `93545e71cb6d3357a3147385c25db0e862995d53`.
- Frontend source `93545e71`: 445 file, **3703 qua / 4 bỏ qua**. Frontend build/typecheck và native build qua.
- Không thay đổi `platform/ui/src/Button.tsx`: diagnostic wrapper đã gỡ hoàn toàn sau khi phát hiện lớp chặn offline của shell mới là phần nhận pointer. Không sửa component dùng chung để phục vụ một assertion sai.
- Go toàn bộ + vet, PostgreSQL upgrade thật từ schema37 và rollback dữ liệu lỗi đã qua sau commit migration.
- Native desktop/reduced-motion: [run012 player/ZIP](runs/012-reviewed-player-and-zip/owned-ready-studio-restoration.json) **2/2**; [run013 Editor/Template/Main/Việc nền/Thông báo](runs/013-reviewed-editor-feedback/interaction-results.json) **10/10**; [run014 viewer Main](runs/014-canonical-inspector-viewer-regression/interaction-results.json) **2/2**.
- Native phone/reduced-motion: [run016](runs/016-settled-phone-interactions/interaction-results.json) **6/6**.
- Main matrix [run028](runs/028-rewrite-selection-boundaries/owned-main-states-restoration.json): **14/14**, gồm 10 ca tĩnh và4 ca đang làm, desktop/reduced-motion; không skip/flaky. Pause/resume, lớp chặn offline, Thử lại ngay/nối lại thật, các phần/gợi ý Viết lại và ranh giới0phần đều qua. Không gửi Viết lại tới AI; kiểm draft/đóng và revision không đổi, không nhận đây là inference proof. Source/job/project/media bindings/receipt gốc không đổi, fixture đã dọn. Các lượt trước đã sửa assumption sai trong QA: action POST tại job URL với body action, tab reset khi đổi video, menu hành động là button và offline shield chặn workspace nên không đòi tooltip dưới lớp chặn.

## Ảnh so sánh

[Manifest ảnh gốc, ảnh code và SHA256](comparisons/manifest.json). Bên trái artboard, bên phải code; giữ nguyên ảnh 1440×900, ghép thành 2880×900, không scale để tạo cảm giác khớp.

- [Chờ duyệt](comparisons/main-review-desktop.png)
- [Cần xử lý](comparisons/main-issue-desktop.png)
- [Kiểm định](comparisons/main-quality-desktop.png)
- [Tạm dừng](comparisons/main-paused-desktop.png)
- [Thành phẩm](comparisons/main-done-desktop.png)

Các ảnh dùng dữ liệu QA/media đã lưu, không phải nội dung minh hoạ của artboard; không tuyên bố pixel-identical. Header Thư viện/Làm video còn thuộc V38/V39. Ảnh hiện dùng run027: bước chưa chạy là pending và không mang output/trạng thái succeeded của bản nguồn.

## Giới hạn và an toàn

- Không có clip nền thật trong QA; không thêm nguồn Twitch/Bilibili giả, điểm chất lượng giả hoặc receipt giả để đạt trạng thái phủ kín.
- Không gọi thêm nhà cung cấp AI; hai receipt ảnh gốc phải giữ nguyên. Không sửa project/job/media gốc.
- Không có kiểm chứng Safari/iPhone vật lý. GitHub Actions bị tắt, không có bằng chứng hosted CI xanh.
- Gate release #83 chưa qua; không deploy bằng báo cáo này.
- Normal git index và `gen/models.go` thuộc người dùng, không staging/reset/sửa chúng. Không sửa artboard.

## Câu hỏi mở

Đã ghi **design #86**: giới hạn Video mỗi ngày mâu thuẫn giữa màn Kênh (1–20) và Tạo kênh desktop/mobile (1–50); chờ chốt trước V38. Dữ liệu clip nền để kiểm trạng thái phủ kín vẫn thiếu; chưa tính trạng thái đó đã qua.
