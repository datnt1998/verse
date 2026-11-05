# Kiểm UI độc lập — Kênh và Thư viện

Parent thực thi controller task bàn giao trên native QA, không gọi AI hay ghi API.

- Bốn tổ hợp: 1440×900 và 390×844, animation bình thường và giảm chuyển động.
- 32 nhóm kiểm qua; không request ghi, không page error. Các nhóm bao gồm discovery và vòng đời controller, không được gọi là 32 artboard state.
- Bốn tab Thư viện cùng nhìn thấy/chuyển được; ba tab Kênh, form mobile đủ vùng chạm, pill sau font ≤2px; chọn người nhận PC phản ánh ngay; form mobile bốn người nhận, trap/restore focus, Esc/scrim, một overlay.
- Lượt đầu có 10 nhóm fail. Giữ nguyên JSON attempt1. Sửa harness: chờ route/ARIA bằng web-first assertion cho Library, không nới kiểm phản hồi ngay của composer; căn giữa control trước hit-test, đo extent 44px và hit disk/cardinal thay vì đòi góc hình vuông của nút bo phải hit. Không sửa ứng dụng để che các lỗi probe. Lượt sau qua toàn bộ.

`ui-acceptance-results.json` là bằng chứng runtime; `HANDOFF.txt` phân biệt assertion đã cài với giới hạn. Không có ảnh/artboard comparison trong controller này. Không đại diện mọi tweak, mọi màn, Safari/thiết bị thật, lưu dữ liệu hay scoring model thật. Luồng lưu thật được ghi riêng trong run004.
