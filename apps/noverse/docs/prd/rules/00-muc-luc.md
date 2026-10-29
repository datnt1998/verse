# MỤC LỤC RESEARCH HUB: TRUYỆN AUDIO GIỌNG AI
Cập nhật 25/10/2026. Đọc file này trước khi dùng các file khác.

## Thứ tự dùng khi sản xuất một truyện
1. **Chọn thể loại** theo báo cáo chấm điểm và lịch xoay trong `he-thong-san-xuat-lien-tuc.md`.
2. **Mở bộ quy tắc của thể loại** (`rules-A`, `rules-B` hoặc `rules-C`), điền mẫu brief ở mục 9 của bộ đó.
3. **Kiểm luật khoảng cách** với sổ cái truyện (`he-thong-san-xuat-lien-tuc.md`, mục 1 và 5).
4. **Viết** theo `master-rules-v3.md` và khung chương của bộ quy tắc.
5. **Kiểm** bằng script trong `dit-dang-van-phong.md` và danh sách kiểm tay ở mục 10 của bộ quy tắc.

## Các file
| File | Vai trò | Dùng khi |
|---|---|---|
| Báo cáo "Thị trường truyện AI tiếng Việt: chấm điểm thể loại" (Docs) | Chấm 18 thể loại, Go/Test/Skip, tâm lý người nghe, công thức tiêu đề, rủi ro, kế hoạch test 30 ngày | Chọn hướng kênh, chọn thể loại |
| `master-rules-v3.md` | Quy tắc lõi cho mọi thể loại: giữ chân, ức chế, công lý, viết cho audio, an toàn kiếm tiền, đóng gói, nhiều phần | Mọi truyện; là chuẩn chấm cao nhất |
| `rules-A-nu-tan-bao-thu.md` | Bộ sản xuất A (Go): nữ tần trọng sinh báo thù, thiên kim, showbiz, hào môn; 8 chương | Truyện thể loại A |
| `rules-B-nguoc-hon-nhan.md` | Bộ sản xuất B (Go): ngược hôn nhân, ly hôn mang con, hối hận muộn; 7 chương | Truyện thể loại B |
| `rules-C-nien-dai-van.md` | Bộ sản xuất C (Go): niên đại văn làm giàu, bản C-Nam và C-Nữ; 10 chương | Truyện thể loại C |
| `phu-luc-the-loai-v3.md` | Module ngắn cho 8 thể loại, gồm 5 thể loại Test chưa có bộ sản xuất riêng | Khi thử thể loại Test |
| `he-thong-san-xuat-lien-tuc.md` | Mười trục biến số, luật khoảng cách, lịch xoay, tuyển tập, sổ cái, dấu hiệu ngán | Lên lịch tuần, duyệt brief |
| `dit-dang-van-phong.md` | Cách đo, mẫu báo cáo chấm, script `kiem_tra_truyen.py` | Kiểm tra trước khi nộp |
| `master-rules-v2.md` | Bản cũ, đã được v3 thay thế | Chỉ để tham khảo lịch sử |

## Lệnh script theo bộ
| Bộ | Bản editor | Bản narration |
|---|---|---|
| A | `python3 kiem_tra_truyen.py ban_editor.txt` | `python3 kiem_tra_truyen.py ban_narration.txt --narration --so-voi <3 truyện gần nhất>` |
| B | `... ban_editor.txt --chuong 7 --thoai 35-50` | `... ban_narration.txt --narration --chuong 7 --thoai 35-50 --so-voi <...>` |
| C | `... ban_editor.txt --chuong 10 --thoai 30-45` | `... ban_narration.txt --narration --chuong 10 --thoai 30-45 --so-voi <...>` |

## Việc còn mở
- vidIQ cấp lại credit ngày 21/11/2026: chạy bổ sung dữ liệu cho các thể loại mới chấm bằng nhận định.
- Lập sổ cái truyện dạng bảng tính.
- Nâng script để tự kiểm trùng nghề, phản diện, tên nhân vật với 10 truyện gần nhất.
- Quyết định có đổi tên `dit-dang-van-phong.md` thành `dinh-dang-van-phong.md` hay không.
