# V37 · Bàn dựng gọn — đủ bằng chứng để review/merge

Ngày: 04/11/2026. [PR #4](https://github.com/datnt1998/verse/pull/4) **nháp, chưa merge, chưa deploy**.

## Màn → tình trạng

| Màn/phần | Đã sửa và kiểm | Còn lệch/chưa nghiệm thu |
|---|---|---|
| Main desktop | Hai tab, mặc định Thông tin khi đổi video, giữ tab khi SSE nối lại/ẩn bảng; player thật; ZIP; chọn video/phạm vi; nền thiếu/phủ kín với clip công khai thật | Header Thư viện/Làm video thuộc V38/V39 |
| Editor | Project QA bản sao: phát/dừng/tua, caption, timeline, split/delete/undo/redo, phím tắt/focus | Không tuyên bố mọi tổ hợp tweak đã qua |
| Template | Menu nổi desktop, Sheet điện thoại; không đẩy nội dung; viewer đọc media đã lưu | Single default và màn kênh/thư viện thuộc V38 |
| Việc nền / Thông báo | Popover đóng ngoài/Escape; global suggestions lưu/readback/reload/khôi phục; mất kết nối có dải giải thích và khoá thao tác, Thử lại ngay/nối lại thật | Chưa có bằng chứng Safari/iPhone vật lý |
| Main điện thoại | Metadata/clipboard/bốn trợ lý/nhật ký thật | Layout/mobile đầy đủ thuộc V40; không gồm viewer điện thoại round45 |
| Backend | Ba loại đề xuất; bỏ đường recap/assistant/learned đang hoạt động; migration dừng mixed/scoped | Release phải kiểm lại dữ liệu trong cửa sổ quiesced |

## Bằng chứng đã qua

- Source: `5d316eabc0875a71ad380a55a7e582ab17d14cd6`, `b493792d689095a20e01992c40c76462348e0882`, `93545e71cb6d3357a3147385c25db0e862995d53`; thanh phủ token/pill `d615299e2352718ea51ee8c7976f7b4565286c4b`.
- Frontend cuối gồm thanh độ phủ theo token:445 file, **3703 qua /4 bỏ qua**; focused15/15, frontend build/typecheck và native build qua. Build còn cảnh báo chunk lớn; không nhận đây là gate performance đã qua.
- Không thay đổi `platform/ui/src/Button.tsx`: diagnostic wrapper đã gỡ hoàn toàn sau khi phát hiện lớp chặn offline của shell mới là phần nhận pointer. Không sửa component dùng chung để phục vụ một assertion sai.
- Go toàn bộ + vet, PostgreSQL upgrade thật từ schema37 và rollback dữ liệu lỗi đã qua sau commit migration.
- Native desktop/reduced-motion: [run012 player/ZIP](runs/012-reviewed-player-and-zip/owned-ready-studio-restoration.json) **2/2**; [run013 Editor/Template/Main/Việc nền/Thông báo](runs/013-reviewed-editor-feedback/interaction-results.json) **10/10**; [run014 viewer Main](runs/014-canonical-inspector-viewer-regression/interaction-results.json) **2/2**.
- Native phone/reduced-motion: [run016](runs/016-settled-phone-interactions/interaction-results.json) **6/6**.
- Main matrix cuối [run034](runs/034-canonical-coverage-tokens/owned-main-states-restoration.json): **14/14**, gồm10 ca tĩnh và4 ca đang làm, desktop/reduced-motion; không skip/flaky. Pause/resume, offline shield, Thử lại ngay/nối lại thật, các phần/gợi ý Viết lại và ranh giới0phần, nền thiếu/phủ kín thật đều qua. Thanh phủ theo token/pill có giá trị và chiều rộng chặn ở100% khi clip10giây phủ video6giây. Không gửi Viết lại tới AI; kiểm draft/đóng và revision không đổi, không nhận đây là inference proof.
- [Provenance clip công khai](runs/034-canonical-coverage-tokens/public-backdrop-provenance.json), [khôi phục clip/media](runs/034-canonical-coverage-tokens/public-backdrop-restoration.json): import qua API bình thường, ffprobe và giải mã thật. Bản nguồn/job/project/receipt/media bindings không đổi; alias, clip nhập và cả object metadata mới đã dọn, không ghi production.
- Run040: Bỏ qua đủ ba loại đề xuất ở chế độ thường/giảm chuyển động, API thật trả200, biến mất qua reload và cooldown168giờ. Sáu dòng đầu vào QA được dọn; source/project/preferences/receipt và suggestions gốc không đổi. Đây không phải bằng chứng producer hay accept/cleanup.

## Ảnh so sánh

[Manifest ảnh gốc, ảnh code và SHA256](comparisons/manifest.json). Bên trái artboard, bên phải code; giữ nguyên ảnh 1440×900, ghép thành 2880×900, không scale để tạo cảm giác khớp.

- [Chờ duyệt](comparisons/main-review-desktop.png)
- [Cần xử lý](comparisons/main-issue-desktop.png)
- [Kiểm định](comparisons/main-quality-desktop.png)
- [Tạm dừng](comparisons/main-paused-desktop.png)
- [Thành phẩm](comparisons/main-done-desktop.png)
- [Nền phủ kín bằng clip thật](comparisons/main-backdrop-covered-desktop.png), ảnh code run034 và artboard được cuộn để thấy thanh độ phủ.

Các ảnh dùng dữ liệu QA/media đã lưu, không phải nội dung minh hoạ của artboard; không tuyên bố pixel-identical. Header Thư viện/Làm video còn thuộc V38/V39. Ảnh hiện dùng run027: bước chưa chạy là pending và không mang output/trạng thái succeeded của bản nguồn.

## Giới hạn và an toàn

- Người dùng đã cho tải clip công khai vào QA. Run034 kiểm nền phủ kín bằng clip Twitch thật, không gán platform/provenance giả. Kho clip trở về rỗng sau kiểm thử; không commit media clip vào repo.
- Không gọi thêm nhà cung cấp AI; hai receipt ảnh gốc phải giữ nguyên. Không sửa project/job/media gốc.
- Không có kiểm chứng Safari/iPhone vật lý. GitHub Actions bị tắt, không có bằng chứng hosted CI xanh.
- Gate release #83 chưa qua; không deploy bằng báo cáo này.
- Normal git index và `gen/models.go` thuộc người dùng, không staging/reset/sửa chúng. Không sửa artboard.

## Câu hỏi mở

Không còn câu hỏi thiết kế chặn V37. **Design #86 đã chốt A:** tối đa20 Video mỗi ngày trên PC/mobile, không tự clamp; đã cập nhật phase05/backend contract. Quyền tải clip QA đã dùng và dọn xong. V38–V40 cùng gate release#83 vẫn là việc còn lại, không phải phần đã hoàn tất trong PR này.
