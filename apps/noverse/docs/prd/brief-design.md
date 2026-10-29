# Brief cho design: màn hình cần vẽ cho đợt tối ưu tiếng Việt và Não truyện

- **Ngày:** 25/10/2026
- **Nguồn:** `toi-uu-ainovel-cli-tieng-viet.md` (PRD, Quyết định 1–11 đã chốt), `nao-truyen.md` (*Cần design*), `docs/design/v1/19-gap-closure.md` §E.
- **Backend đã sẵn (25/10):** API cho D1–D12 đã có, xem `ke-hoach-be.md` › *Triển khai*. Gồm:
  - D1: `/api/system` có `latestVersion`, `hasUpdate`.
  - D2: `presets.packs` có `id`, `ten`, `so_chuong`, `nguoi_nghe`.
  - D3: định dạng `narration`, tuỳ chọn `readNumbers`.
  - D4: issue có `rule_ref`, `rule_text`, `quote`.
  - D6: violation có `per_1000` và các phép đo mới.
  - D5: trường giữ chân trong outline, `meta/story_retention.json`.
  - D7–D10: `address_terms.json`, `relationship_log.jsonl`, kho ghi chú `nao/`.

  Design chỉ cần vẽ; dev nối dây khi có artboard.
- **Quy ước:**
  - Dev không dựng màn nào khi chưa có artboard (chỉ thị "không tự chế").
  - Nguyên tắc bất biến số 7 vẫn áp dụng: không hiện đường dẫn, tên tệp, khoá cấu hình hay tên công cụ, trừ ngăn Nâng cao.
  - Số mục luật như "Master §7" hay "Bộ A §3" là nội dung của người dùng, được hiện.

Xếp theo sprint cần dùng. Design nên giao trước khi dev bắt đầu sprint đó.

## Sprint 1

| # | Nhu cầu | Yêu cầu nguồn | Màn đang có bị ảnh hưởng |
|---|---|---|---|
| D1 | **Phiên bản đang chạy và báo có bản mới.** Hiện phiên bản (tag, ngày build) ở đâu; báo "có bản mới hơn" thế nào (banner, chấm trên menu tài khoản…) | P0-2 | `Settings.dc.html` mục Nâng cao, header. Web hiện chưa hiện phiên bản ở đâu |

## Sprint 2

| # | Nhu cầu | Yêu cầu nguồn | Màn đang có bị ảnh hưởng |
|---|---|---|---|
| D2 | **Chọn gói thể loại.** Gói được chọn qua khoá `style` sẵn có (Quyết định 9). Hiện Cách viết chỉ có 5 thẻ dựng sẵn và một thẻ "Văn phong của bạn", nên một gói sẽ rơi vào thẻ "của bạn". Cần thẻ cho gói: tên ("Nữ tần báo thù"), mô tả, số chương chuẩn, dấu hiệu đây là bộ luật có chấm | P0-3, Quyết định 1, 9 | `Settings.dc.html` mục `writing`; `MMore.dc.html` storyset (danh sách "Văn phong"); chip "Văn phong" ở `NewStory.dc.html`; dòng "Văn phong" ở màn Dự án |
| D3 | **Xuất bản đọc cho TTS** (định dạng thứ ba bên cạnh văn bản và ebook): không tiêu đề, không Markdown; tuỳ chọn "Đọc số thành chữ" | P0-5 | Ô chọn `.txt`/`.epub` ở bảng xuất của Trình đọc (`Reader.dc.html`); ma trận Xuất ở màn Dự án (`Outline.dc.html` tab export); ô chọn định dạng tự xuất ở `Onboard.dc.html` step library và ở Dự án; thẻ kết quả `/export` (`CmdResults.dc.html`); "Xuất riêng chương" trên điện thoại (`MReader.dc.html`) |
| D4 | **Lỗi của editor kèm số mục luật.** Thẻ thẩm duyệt hiện số mục ("Master §7") và đoạn trích; phân biệt `[LỖI]` bắt sửa với `[CẢNH BÁO]` | P0-4 | Chi tiết thẩm duyệt ở `Main.dc.html` (phase gate), `Activity.dc.html`, `MLog.dc.html` |

## Sprint 3

| # | Nhu cầu | Yêu cầu nguồn | Màn đang có bị ảnh hưởng |
|---|---|---|---|
| D5 | **Bản đồ giữ chân trong đại cương.** Mỗi chương có: câu hỏi đang treo, lời hứa, kiểu móc cuối chương, cú lật giữa truyện. Thêm khối cấp truyện xem và sửa được: nhóm người nghe (5 nhóm của Master §1.2), "Điểm khác", hạt giống phần hai. Ba trường này do gói hoặc Architect điền sẵn (Quyết định 11), **màn tạo truyện không đổi** | P1-2 | `Outline.dc.html` tab outline |
| D6 | **Kết quả đo bằng máy:** tỷ lệ thoại, độ dài câu và đoạn, trùng mở đầu với truyện cũ, từ lạm dụng tính trên 1.000 tiếng. Cần quyết định hiện con số hay chỉ hiện vi phạm | P1-1, P1-5 | Rail "Kiểm tra 3 chương gần nhất" ở Quy tắc viết (`Settings.dc.html` rules); `Activity.dc.html` |
| D7 | **Trang nhân vật có bảng xưng hô:** A gọi B là gì, A tự xưng là gì, từ chương nào, lý do đổi; lịch sử quan hệ theo chương; chương xuất hiện | Não W1–W2, P1-3 | `Outline.dc.html` tab cast |
| D8 | **Sửa xưng hô và quan hệ trên web:** khoá khi trợ lý đang viết; xác nhận khi sửa ảnh hưởng chương đã viết | Não W1 (dữ liệu gốc là JSON, sửa qua web) | Mới |
| D9 | **Vi phạm xưng hô** trong Nhật ký và kết quả kiểm. Có thể dùng thẻ vi phạm sẵn có; cần design xác nhận | Não W2 | `Activity.dc.html`, `MLog.dc.html` |
| D10 | **Màn Não truyện (máy tính):** đồ thị thực thể và quan hệ có kiểu, lọc theo loại, thanh chọn chương để xem trạng thái tại chương N, bảng chi tiết khi chọn | Não W1–W2 | Mới. Điện thoại: quyết định có vẽ đồ thị ở 390 px hay chỉ danh sách |
| D11 | **Audio theo gói:** khi gói chọn một giọng kể (gói A, Quyết định 3), màn phân vai hiện trạng thái "theo gói". Nhạc nền không làm trong công cụ (Quyết định 10), không cần vẽ | P1-4 | `MAudio.dc.html`, `AudioExport.dc.html` |
| D12 | **Ngăn Nâng cao (chỉ quản trị):** nút "Mở bằng Obsidian" cho kho ghi chú. Cài đặt embedding trên máy **chưa cần vẽ**, vì W4 đã hoãn sau khi đo W0 (`evals/nao-w0/results.md`) | Não W1 | `Settings.dc.html` mục `adv` |

## P2

| # | Nhu cầu | Yêu cầu nguồn |
|---|---|---|
| D13 | Bảng chất lượng từng truyện: số LỖI và CẢNH BÁO theo chương và theo số mục của gói | P2-4 |
| D14 | Não dự án: sổ cái truyện 10 trục, cảnh báo luật khoảng cách, nhân vật vũ trụ chung, ô tìm kiếm | P2-5 |
| D15 | Não hệ thống: duyệt bài học rút ra (giữ/bỏ, giống luật học từ chat), mục luật hay bị vi phạm theo gói | P2-7 |

## Câu hỏi giao diện còn treo từ các gói A–K

Danh sách từ `19-gap-closure.md` §E. Mỗi câu cần design hoặc PO trả lời.

- **Gói B:** tạo tài khoản quản trị **đầu tiên** trên web, kèm mã thiết lập in lúc khởi động. Hiện việc này vẫn làm trong terminal.
- **Hộp thoại `locked`:** còn chữ "TUI".
- **Lệnh `/simulate`:** câu mô tả nhắc tới thư mục.
- **Esc:** bấm một lần hay hai lần để đóng.
- **`/reopen`:** có ẩn với người không có quyền viết không.
- **Thẻ khôi phục:** khi đã có mã chưa dùng (lỗi 409) thì hiện gì.
- **Lệnh và việc nền:**
  - `--keep` có ở registry nhưng artboard Commands chỉ vẽ `--overwrite`.
  - Gói G đã bỏ "Bỏ đoạn", nút xoá cờ ở Nghe và các ô chương/phụ đề/m4b ở hộp xác nhận xuất audio, vì artboard không vẽ. Có cần vẽ lại không?
- **Điện thoại và máy tính bảng:**
  - khung "chưa chắc" trên điện thoại;
  - trạng thái cocreate trống, mốc giờ của dòng hệ thống cocreate;
  - nền tảng thiết bị trong danh sách push;
  - breakpoint 760 px của máy tính bảng.
- **Quy tắc viết:** lỗi 500 khi "hiểu quy tắc" phải chờ thử lại tới 2 phút mới hạ cấp.
- **Kiểm trên thiết bị thật (không cần vẽ, cần người kiểm):**
  - quét QR bằng iPhone/Android ở 40 cm;
  - "Giảm chuyển động" trên Mac/iPhone thật;
  - SSE trong một phiên đang viết thật.
