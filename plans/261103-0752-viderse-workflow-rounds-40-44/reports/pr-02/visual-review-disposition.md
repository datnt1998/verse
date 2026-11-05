# V36 · Xử lý kết quả đối chiếu cuối

Trạng thái: controller đã đối chiếu lại bản nhúng `11f1cd36e395205f01e9900856e24ac7e32f07eb`; đây là nghiệm thu phạm vi V36, **không phải V37–V40 hoặc quyết định deploy**. Nguồn ảnh mới: [19 Review/Dialogs](integrated-review-comparisons.json), [3 Main](integrated-main-comparisons.json); [Settings không đổi ở patch cuối](verified-source-main-settings-comparisons.json) giữ đúng nguồn/lượt riêng. [Receipt kiểm tra cuối](integrated-acceptance.json).

| Phát hiện | Xử lý / bằng chứng / owner |
|---|---|
| Nút chuyển ảnh Duyệt thiếu nền tương phản; bố cục composer, border và lời Sửa prompt còn lệch | Đã sửa local theo Review và kiểm ở native140: nền tròn tối40px, quiet controls, nhãn prompt, chỉ một primary theo trạng thái. Không đổi mutation. |
| Hộp Vẽ lại cả bộ: đường chia đầu, nhãn tuỳ chọn, biểu diễn thông tin, icon đóng | Native140 và comparison Dialogs mới có header không divider, quota/style/info tách, nhãn sentence case và icon đóng outlined. Scrim blur3px chỉ theo skin Viderse, không đổi kit dùng chung; close/focus/pending vẫn qua. Quota/tên prompt là dữ liệu thật. Vòng focus chỉ xuất hiện khi có focus bàn phím, không ép trạng thái focus để khớp ảnh mẫu. |
| Dialog bị báo rộng521 thay vì540 | Không sửa theo ước lượng ảnh: [native104](runs/104-final-keyboard-review-states/review-state-replay-results.json) đo sau khi chờ animation: x450, rộng540. Code `width={540}`. Chiều cao còn phải xem theo nội dung thật. |
| Textarea bị báo cao133 thay vì151 | Source Review887/1182 và `ImageRail` đều rows6, font12.5/line1.55/padding8×10/min62. Chưa có bằng chứng DOM trái hợp đồng; không thêm height hack. Border sửa thật từ token20% sang token45% đúng Review889. |
| Ảnh đối chiếu refs.add chưa chọn ứng viên | Native140 chọn ứng viên thật, aria-pressed đúng, Thêm1ảnh bật, Huỷ trả focus và0write. Đã đọc comparison sau sửa; vị trí/chiều cao khác do dữ liệu thật, không dùng ảnh chưa chọn làm cùng trạng thái. |
| Main viewer: caption Prompt đã gửi, số phiên bản bên phải | Đã sửa; native1462/2 kiểm copy nguyên byte, raw/structured, strip/phiên bản/địa chỉ và motion. Header/nút không wrap: nút ở sau nội dung theo Main1318–1320, structured prompt không có khung thừa. Ảnh default đợi trạng thái Đã chép trở về Chép prompt trước capture. |
| Quota/provider/giờ đặt lại, tên ảnh, prompt cũ, số lần thử khác ảnh mẫu | Giữ dữ liệu thật. Không tạo50/50,07:00,2lần thử hay tên template khi API không cung cấp. Prompt/raw lịch sử là provenance bất biến; dịch lại hoặc gán tên provider không có bằng chứng là sai. Lời tĩnh/layout vẫn phải khớp. |
| Review.score.true có Vì sao?/panel tự mở; phần shell vẫn Video mới và thiếu Thư viện | Cutover đúng phase: bỏ Vì sao? theo V40/PR6; điều hướng và shell theo PR3–PR5. Đây là gap có owner, không gọi full artboard đạt ở PR2 và không tự cắt sớm caller đang dùng. |
| Dòng chính sách Duyệt ảnh và dấu màu kênh | Còn kiểm contract/policy thật; không hardcode Luôn hỏi ở3kênh. Chính sách mới phụ thuộc V38 và parity PR6. Chưa gọi đã khớp. |
| Nguồn vẽ: radio-card đầy đủ vs artboard CSS cắt chữ | [Câu hỏi85](../../../../apps/viderse/docs/design/v1/11-hoi-dap.md) đã được người dùng chốt A: giữ thẻ đầy đủ tên/model/gói/hạn mức/giờ đặt lại; không sửa artboard hoặc cắt thông tin theo CSS30px. |

## Kiểm hành vi bổ sung

- [Native115](runs/115-final-native-action-matrix/mutation-matrix-results.json):12/12 thao tác thực sự qua, không skip; HTTP200/202 và readback, cancel0ảnh, chuyển đúng videoB. Trường hợp duyệt chứng minh `clone_bulk_review_intents.image_revision` khớp checkpoint; worker đang standby nên chưa được báo dựng xong. Job/steps/intents/artifacts/bindings khôi phục, source/project gốc giữ nguyên, image_receipts0→0.
- Lượt111 đã bắt race effect địa chỉ sau optimistic approval. Hồi quy desktop/mobile đều lỗi trước và qua sau; focused50/50, full3766qua/4skip và build qua trước sửa caption sau đó. Không bỏ assertion địa chỉ để làm xanh.
- Lượt113/114 giữ nguyên thất bại oracle: dispatcher standby chưa tiêu thụ intent nên stage_group vẫnreview; trạng thái board làpending, không phảiwaiting hay build hoàn tất. Native115 kiểm owner-approved snapshot, revision và durable intent thay cho giả định worker đã chạy.
- [Native112](runs/112-hardened-catalog-manual/owned-catalog-manual-restoration.json):2/2 manual catalog thực sự qua, không skip; manual=true được đọc trước CAS restore. Provider/source giữ nguyên. `ChatGPTAdapter.CheckModel` chỉ đọc bundled catalog, không suy diễn thành phép đo call của provider khác. Access QA được launcher ngoài gỡ sau lượt.
- Nguồn `11f1cd36`: native14212/12 Review actions, native1452/2 Main dùng bản→khôi phục bằng UI, native1462/2 Main viewer, native1418/8 safe controls PC/mobile và2/2 thêm→gỡ refs. Mọi wrapper thao tác có ghi đều khôi phục job/steps/intents/artifacts/bindings; source/project gốc và receipt tính ảnh giữ nguyên.
- Native1382/2 chứng minh model đổi trước response PUT, lưu tức thì và qua reload, model prompt/suy nghĩ có readback. Có guard riêng cho timestamp heartbeat `storage.serverOffline` do monitor ghi lại dù giá trị không đổi; không che thay đổi giá trị hoặc ghi đè heartbeat. Lượt136/137 thất bại được giữ, không gọi là pass.

Không suy ra kiểm browser Chromium thành iPhone/Safari thật. Hai gate toàn cục#83 và PR3–PR6 vẫn mở; chưa deploy.
