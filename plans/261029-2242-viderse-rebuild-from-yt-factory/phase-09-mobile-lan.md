---
phase: 9
title: "Điện thoại qua LAN và các màn điện thoại"
status: paused
priority: P2
effort: 20h
dependsOn: [7]
---

# Phase 09 · Điện thoại qua LAN

## Mục tiêu

- Duyệt ảnh và theo dõi được từ điện thoại trong cùng mạng LAN.
- Mỗi thiết bị ghép một lần bằng mã QR (D1, §10 cách a).

Gói design: **V10** Điện thoại; hỏi đáp #6 (Bilibili trên điện thoại).

## Đầu vào

- Design: 07 §10; 08 V10; artboard `MMain` (`screen=list|detail|jobs|switch|recap|plan`), `MReview`, `MChannel`, `MNew`, `MExplore`, `MCompVideo`, `MSettings`, `MCreate`, `MKit`.
- Ghép thiết bị đã có:
  - `platform/identity` (`NewPairing`, `qr.go`);
  - Noverse dùng ở `apps/noverse/internal/entry/web/auth.go:157-210`, gồm QR cho từng địa chỉ truy cập được (`pairingBases`).
- `platform/webguard`: proxy tin cậy, chặn khác origin, cookie Secure.

## Việc

### Backend

- **Nghe trên LAN:**
  - lấy địa chỉ từ cài đặt hoặc `VIDERSE_ADDR` (ví dụ `0.0.0.0:8789`);
  - Cài đặt hiện các địa chỉ LAN truy cập được;
  - mặc định vẫn là `127.0.0.1` cho tới khi anh bật ở Cài đặt.
- **Ghép thiết bị:**
  - chuyển các handler ghép thiết bị dùng chung vào `platform/webapp` (tạo mã một lần, QR cho mỗi địa chỉ, đổi mã lấy phiên);
  - Viderse dùng qua đó;
  - Noverse vẫn giữ gateway riêng, không đổi.
- **Bảo mật:**
  - mọi `/api/*`, tệp media và artifact đều cần phiên;
  - danh sách origin hợp lệ gồm các địa chỉ LAN đang nghe;
  - giới hạn số lần đăng nhập (`identity.limiter`);
  - danh sách thiết bị đã ghép ở Tài khoản, thu hồi được.
- **Bilibili (#6, #7):** kiểm mở `loginUrl` trên điện thoại có đã cài 哔哩哔哩. Mở được thì giữ dòng "Mở app"; không được thì đổi câu theo #6 và ghi trả lời vào hỏi đáp.

### Web

- `MobileShell`:
  - thanh dưới có nút `+` tròn màu đỏ, là chỗ duy nhất nền đỏ cho nút;
  - các màn M* theo artboard;
  - dùng `Sheet` thay `Dialog`.
- Cùng route và cùng dữ liệu với bản máy tính; chỉ đổi bố cục khi màn hẹp hơn 760px.
- Ảnh trên điện thoại dùng biến thể nhỏ (WebP, chiều rộng ≤ 720).

## Kiểm thử

- Go:
  - ghép thiết bị: mã dùng một lần, mã hết hạn;
  - yêu cầu từ origin LAN được chấp nhận, origin lạ bị chặn;
  - thiết bị đã thu hồi thì mất phiên ngay;
  - tệp media không có phiên trả 401.
- Web: test bố cục M* theo các tweak chính, test thanh dưới.
- Smoke trên điện thoại thật cùng Wi-Fi: quét QR → ghép → duyệt 1 video → nhận thông báo trong app.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V10** (08-dev-prompt).
- Máy không ghép hoặc chưa đăng nhập thì không đọc được dữ liệu nào, kể cả ảnh.

## Rủi ro

- **HTTP trên LAN không mã hoá.** Giống cách Noverse đang làm. Cookie đặt `HttpOnly`, `SameSite=Strict`; chỉ dùng trong mạng nhà. Ghi rõ trong README. Nếu cần mở ra ngoài LAN thì làm TLS hoặc qua proxy, ngoài phạm vi plan này.
