# Bộ luật sản xuất truyện audio (bản sao cho dev)

Thư mục này là bản sao của các tài liệu luật trong Project NovelAI của PO, chép ngày 25/10/2026. **Project là bản gốc.** Khi luật thay đổi, PO sửa trong Project rồi chép lại vào đây; dev không sửa trực tiếp các file này.

PRD dẫn chiếu các file này: [`../toi-uu-ainovel-cli-tieng-viet.md`](../toi-uu-ainovel-cli-tieng-viet.md).

## Đọc theo thứ tự

| File | Tầng | Vai trò với dev |
| --- | --- | --- |
| [`00-muc-luc.md`](00-muc-luc.md) | | Mục lục và thứ tự dùng của PO |
| [`master-rules-v3.md`](master-rules-v3.md) | 1. Lõi | Quy tắc chung cho mọi thể loại, §1–§22, nhãn `[LỖI]` / `[CẢNH BÁO]`. Chuẩn chấm cao nhất |
| [`rules-A-nu-tan-bao-thu.md`](rules-A-nu-tan-bao-thu.md) | 2. Gói thể loại | Bộ A, 8 chương. Gói đầu tiên, dùng làm chuẩn nghiệm thu |
| [`rules-B-nguoc-hon-nhan.md`](rules-B-nguoc-hon-nhan.md) | 2. Gói thể loại | Bộ B, 7 chương |
| [`rules-C-nien-dai-van.md`](rules-C-nien-dai-van.md) | 2. Gói thể loại | Bộ C, 10 chương, bản C-Nam và C-Nữ |
| [`phu-luc-the-loai-v3.md`](phu-luc-the-loai-v3.md) | 2. Gói thể loại | Module A–H, gồm các thể loại chưa có bộ sản xuất riêng |
| [`dit-dang-van-phong.md`](dit-dang-van-phong.md) | 3. Định dạng | Cách đo, mẫu báo cáo chấm, script |
| [`kiem_tra_truyen.py`](kiem_tra_truyen.py) | 3. Định dạng | Script tách từ phụ lục của file trên. **Test oracle** khi port các phép đo sang Go (PRD P1-1) |
| [`he-thong-san-xuat-lien-tuc.md`](he-thong-san-xuat-lien-tuc.md) | Vận hành | Mười trục biến số, luật khoảng cách, lịch xoay, sổ cái. Không thuộc phạm vi PRD đợt này, để tham khảo |

`00-muc-luc.md` còn nhắc tới báo cáo "Thị trường truyện AI tiếng Việt: chấm điểm thể loại". Báo cáo đó là tài liệu Docs riêng, không có trong thư mục này.

## Chạy script

```bash
python3 docs/prd/rules/kiem_tra_truyen.py ban_editor.txt
python3 docs/prd/rules/kiem_tra_truyen.py ban_narration.txt --narration
# Bộ B
python3 docs/prd/rules/kiem_tra_truyen.py ban_editor.txt --chuong 7 --thoai 35-50
# Bộ C
python3 docs/prd/rules/kiem_tra_truyen.py ban_editor.txt --chuong 10 --thoai 30-45
```

Script chỉ dùng thư viện chuẩn của Python 3.
