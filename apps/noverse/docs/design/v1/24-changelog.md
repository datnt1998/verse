# Noverse — Lịch sử thiết kế

Mỗi đợt ghi: ngày, gói, thay đổi trên artboard và tài liệu, phiên bản canvas. Mới nhất ở trên. Các gói trước Gói AO ghi trong `21-dev-prompt.md` và `19-gap-closure.md`.

## 01/11/2026 — Gói AR: tạo truyện mới trên điện thoại <!-- r-ar -->

- Mới: `MNewStory` (M11) — Bắt đầu nhanh, Đồng sáng tác, so với truyện trước (lưới 2 cột), Bắt đầu từ tệp, Nhập bản thảo, Mô phỏng văn phong (dán văn mẫu trước).
- `MWork` trống: nút chính *Tạo truyện mới*. `MMore`: hàng *Truyện mới* ở Dự án & truyện; `/start` “Dùng ở Truyện mới”; huy hiệu Quản trị đổi sang token vàng.
- `MOutline`: nền nhãn Đồng phạm 0,06. `tokens.css`: `--role-accomplice-*`.
- Tài liệu: `18-mobile.md` › M11 + vai trò điện thoại; `21-dev-prompt.md` › Gói AR; `06` §AR; `07` Gói AR; `19` Gói AR. Canvas bản 135.

## 01/11/2026 — Gói AQ: lấp chỗ thiếu sau AO, AP, Luồng viết <!-- r-aq -->

- Mới: `MOutline` (M9 tab truyện điện thoại), `MProjBrain` (M10 Não dự án điện thoại), `RunCards` (01c thẻ theo loại thao tác), `MRun` (M1b lịch sử luồng viết điện thoại).
- `Dialogs` newProject: bỏ “Mọi truyện” khỏi quy tắc; *Bắt đầu trống* / *Chép từ dự án khác*; ô chọn kit; sửa biểu tượng thiếu ở cột “Dự án mới có sẵn”.
- `Main`: lối *Lịch sử ›* ở khung Luồng viết trực tiếp. `MWork`: nút Truyện, *Luồng viết ›*. `MMore`: hàng Truyện, Não dự án.
- `Login`: huy hiệu Quản trị theo token mới, tên miền không mono; `Login`/`MMore`/`Members`: nền logo `#18181A`. `Outline`: cột trái đúng 264 px.
- Tài liệu: `21-dev-prompt.md` Gói AQ; `20-settings-rules.md` §10–11; `19-gap-closure.md` Gói AQ; `06-data-contract.md` §AQ; `07-copy.md`; `18-mobile.md`; `artboards/README.md`; `tokens.css`.
- Canvas bản 133.

## 31/10/2026 — Gói AP: quy tắc hai cấp, chống giọng văn máy, nền móng truyện, Não dự án <!-- r-ap -->

- `Settings`: bỏ Quy tắc chung và Cách viết; mở ở Trợ lý AI; thẻ Cách chạy; Thông tin dự án luôn hiện; Não hệ thống ghi “4 bài học chờ duyệt” ở dòng phụ thay huy hiệu.
- `Outline`: tab Thông tin · Tiền đề · Thế giới (dải Nền móng, Khung truyện chuyển từ Dàn ý, Định hướng kết, 10 trục, phiên bản viết); quy tắc một dòng + nhãn loại, thẻ Học từ góp ý riêng, bỏ nhóm Mọi truyện; thẻ Chống giọng văn máy với 16 từ chỉnh − / + tạo quy tắc; ô chọn theo `.cb` của bộ thành phần. Tweak `found`, `aai`.
- `MMore`: bỏ cấp Mọi truyện, nhãn loại, thẻ Chống giọng văn máy (`state=rulesAai`).
- `ProjBrain`: bỏ cột Ngày 7; khối kiểm có kết luận + bảng 4 phép; Tần suất theo trục; bấm ô để soi; tiêu đề cột dính; lọc Vũ trụ chung và Tìm. Tweak `lview`, `uni`, `sfilter`.
- `Members`: thanh bên giống Tuỳ chỉnh (bỏ Quy tắc chung, Cách viết; thêm Lưu trữ tệp); Quyền vào ứng dụng dùng ô chọn của bộ thành phần, thẻ tự xuống một cột khi hẹp, tên miền không tràn.
- `SysBrain`, `Commands`/`MWork`/`Slash`: câu chữ không còn trỏ “Tuỳ chỉnh › Cách viết” / “cấp Mọi truyện”.
- Tài liệu: `20-settings-rules.md` §2, §3, §9; `21-dev-prompt.md` Gói AP; `06-data-contract.md` §AP; `07-copy.md`; `05-screens.md`; `18-mobile.md`; `artboards/README.md`.
- Canvas bản 132.

## 31/10/2026 — Gói AO: đăng nhập chung với Viderse <!-- r-ao -->

- `Login`: dòng phụ “Một tài khoản dùng cho Noverse và Viderse”; khối lý do đăng nhập lại (`kicked`: self · other · password · admin · unknown); `state=menu` có nhóm Ứng dụng (Noverse Đang mở, Viderse mở thẻ mới; `viderse=no` ẩn cả nhóm), dòng phụ cho Chuyển tài khoản và Đăng xuất; `state=devices` dùng câu phiên chung và “đổi xong… kể cả máy này”; màn mới `state=denied` (Chưa được cấp quyền vào Noverse: thẻ tài khoản, *Mở Viderse*, *Thử lại*, *Chép lời nhờ quản trị viên cấp quyền*, *Đăng xuất* cả hai app).
- `Members`: mục 3 **Quyền vào ứng dụng** (ô Noverse · Viderse, lưu ngay, câu hệ quả ngay dưới; quản trị khoá cả hai ô) với tweak `app`; hộp Đặt lại mật khẩu, Khoá ghi “đăng xuất khỏi mọi thiết bị ở cả Noverse và Viderse”.
- `MMore`: nhóm Ứng dụng (`viderse`), dòng phụ dưới Đăng xuất, câu phiên chung ở Thiết bị, Đổi mật khẩu, chuyển tài khoản.
- `Phone`: dòng phụ + khối lý do (`kicked`) ở `state=login`.
- Đối chiếu bản thật: đổi mật khẩu thu hồi mọi phiên của tài khoản, kể cả máy đang dùng (`platform/identity/postgres/passwords.go` › `revokeUserAuth`) — câu cũ “thiết bị này giữ nguyên phiên” đã bỏ ở máy tính và điện thoại. `reason`/`app` của 401 và `apps[]` của `GET /api/me` vẫn là GAP (21-dev-prompt Gói AO).
- Tài liệu: `09-auth-onboard.md` §12 (tên board + tweak), `artboards/README.md`, `18-mobile.md` M4, `21-dev-prompt.md` Gói AO.
- Canvas bản 130.

