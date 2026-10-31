# Noverse — Lịch sử thiết kế

Mỗi đợt ghi: ngày, gói, thay đổi trên artboard và tài liệu, phiên bản canvas. Mới nhất ở trên. Các gói trước Gói AO ghi trong `21-dev-prompt.md` và `19-gap-closure.md`.

## 31/10/2026 — Gói AO: đăng nhập chung với Viderse <!-- r-ao -->

- `Login`: dòng phụ “Một tài khoản dùng cho Noverse và Viderse”; khối lý do đăng nhập lại (`kicked`: self · other · password · admin · unknown); `state=menu` có nhóm Ứng dụng (Noverse Đang mở, Viderse mở thẻ mới; `viderse=no` ẩn cả nhóm), dòng phụ cho Chuyển tài khoản và Đăng xuất; `state=devices` dùng câu phiên chung và “đổi xong… kể cả máy này”; màn mới `state=denied` (Chưa được cấp quyền vào Noverse: thẻ tài khoản, *Mở Viderse*, *Thử lại*, *Chép lời nhờ quản trị viên cấp quyền*, *Đăng xuất* cả hai app).
- `Members`: mục 3 **Quyền vào ứng dụng** (ô Noverse · Viderse, lưu ngay, câu hệ quả ngay dưới; quản trị khoá cả hai ô) với tweak `app`; hộp Đặt lại mật khẩu, Khoá ghi “đăng xuất khỏi mọi thiết bị ở cả Noverse và Viderse”.
- `MMore`: nhóm Ứng dụng (`viderse`), dòng phụ dưới Đăng xuất, câu phiên chung ở Thiết bị, Đổi mật khẩu, chuyển tài khoản.
- `Phone`: dòng phụ + khối lý do (`kicked`) ở `state=login`.
- Đối chiếu bản thật: đổi mật khẩu thu hồi mọi phiên của tài khoản, kể cả máy đang dùng (`platform/identity/postgres/passwords.go` › `revokeUserAuth`) — câu cũ “thiết bị này giữ nguyên phiên” đã bỏ ở máy tính và điện thoại. `reason`/`app` của 401 và `apps[]` của `GET /api/me` vẫn là GAP (21-dev-prompt Gói AO).
- Tài liệu: `09-auth-onboard.md` §12 (tên board + tweak), `artboards/README.md`, `18-mobile.md` M4, `21-dev-prompt.md` Gói AO.
- Canvas bản 130.

