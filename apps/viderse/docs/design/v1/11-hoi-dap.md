# Hỏi đáp design ↔ dev

Mọi câu hỏi, đề nghị đổi và quyết định giữa design và dev ghi ở đây, trong repo. Không trao đổi qua kênh khác rồi để tài liệu lệch.

## Cách dùng

- **Dev hỏi:** thêm một mục ở cuối, trạng thái `Mở`. Ghi màn / artboard / tweak, câu hỏi, và điều đang chặn (nếu có).
- **Design trả lời:** ghi câu trả lời ngay trong mục, đổi trạng thái `Đã chốt`, và sửa tài liệu gốc (ia-flows, components, artboard…) cùng lúc. Mục này chỉ là dấu vết; tài liệu gốc mới là chuẩn.
- **Code làm khác thiết kế** vì lý do kỹ thuật: ghi ở đây trước khi làm, đợi chốt.
- Gói làm xong: ghi ở [10-gap-closure.md](10-gap-closure.md).

Mẫu:

```md
### #N · <tiêu đề ngắn> — Mở
- Màn / artboard: `Main` · `stage=review`
- Hỏi: …
- Chặn: …
- Trả lời (design, ngày): …
```

## Mục

### #1 · Màu brand trong code khác thiết kế — Đã chốt
- Màn: mọi màn. `apps/viderse/web/src/main.tsx` đang `defineBrand({ primary: '#F43F5E' })`.
- Thiết kế: đỏ nhấn `#FF4D63`, công tắc bật `#E6002E`, lỗi/xoá cam `#FF9A5C`. Bộ đầy đủ ở [02-stack-verse.md](02-stack-verse.md) §2.
- Đề nghị: đổi `defineBrand` theo §2; so với board `Kit` › Nền tảng › Màu.
- Trả lời (dev, 29/10/2026): đổi `defineBrand` đúng bộ §2 (`primary #FF4D63`, success `#A6EDC1`, warning `#F6CB8C`, danger `#FF9A5C`, info `#B9D0F7`, `accentDeep #E6002E`). Trang `#/dev/kit` (chỉ bản dev) dựng lại board `Kit`/`MKit` để so.

### #2 · Nút `quiet` — Đã chốt
- Thiết kế có nút chữ không viền (`.vv-btn--quiet`: Huỷ trong thẻ trợ lý, Bỏ qua ở Đề xuất). `Button` của kit có `ghost` (có viền) và `icon`.
- Hỏi dev: thêm biến thể `quiet` vào kit, hay dùng `ghost` với style bỏ viền trong app?
- Trả lời (dev, 29/10/2026): thêm `variant="quiet"` vào `Button` của kit. Chỉ thêm, Noverse không đổi.

### #3 · Banner màu Đạo diễn — Đã chốt
- Thiết kế có banner tím nhạt khi Đạo diễn giải thích (`.vv-banner--director`, Bàn dựng › Nội dung). `Banner` của kit chỉ có 4 tone trạng thái.
- Hỏi dev: thêm tone `agent` (lấy màu `writer`) hay dựng banner riêng trong app?
- Trả lời (dev, 29/10/2026): thêm `tone="agent"` cho `Banner` của kit, nhận khoá trợ lý của `AgentOrb` (Đạo diễn = `writer`) và lấy màu từ bảng màu trợ lý. Các trợ lý khác dùng được cùng tone.

### #4 · Toast dạng thẻ — Đã chốt
- Thiết kế (components › Toast): kết quả thao tác là toast viên (kit có); **Cần bạn / Cần xử lý** là thẻ 300px có tên video và nút việc (`Activity` · `view=toasts`).
- Hỏi dev: mở rộng `toast()` nhận nội dung thẻ, hay dựng trong app?
- Trả lời (dev, 29/10/2026): mở rộng `toast()` của kit nhận nội dung thẻ 300px (tên video, đúng một nút việc, `role="alert"`, ở lại tới khi người dùng đóng). Toast viên giữ nguyên.

### #5 · Yêu cầu hỗ trợ nền tảng mới gửi về đâu (D8) — Đã chốt
- Thư viện › Video nền › dán link nền tảng chưa hỗ trợ › **Gửi yêu cầu hỗ trợ**.
- Đề xuất: ghi trên máy, gửi kèm khi Báo lỗi. Cần chủ sản phẩm chốt.
- Trả lời (chủ sản phẩm, 29/10/2026): theo đề xuất. Các quyết định D1, D2, D5–D8 đều theo đề xuất của design.

### #6 · Điện thoại: mở link đăng nhập Bilibili trong app thay vì quét — Mở <!-- r24-biliqr -->
- Màn: `MSettings` · `qr=waiting` · dòng **Bilibili trên chính máy này? → Mở app**.
- Người dùng xem Viderse trên điện thoại thường cũng cài 哔哩哔哩 trên chính máy đó, nên không quét được mã trên màn hình mình.
- Hỏi dev: mở `loginUrl` (link trong mã QR) trên điện thoại có vào app 哔哩哔哩 và hiện màn xác nhận không? Nếu không: bỏ dòng này, thay bằng câu “Quét bằng điện thoại khác, hoặc mở Cài đặt trên máy tính”.
- Dev (29/10/2026): sẽ thử trên điện thoại thật khi làm bản điện thoại và ghi kết quả vào đây. Giữ dòng này tới lúc đó.

### #7 · `loginUrl` chưa về tới giao diện — Đã chốt <!-- r24-biliqr -->
- yt-factory: `startBilibiliQrLogin()` trả `loginUrl` nhưng `startBilibiliQrLoginAction` chỉ trả `qrcodeKey`, `qrImage`, `expiresAt`.
- Đề nghị: bản verse trả cả `loginUrl` (§24.1) để dùng cho #6. Giao diện không hiện link dạng chữ.
- Trả lời (dev, 29/10/2026): API đăng nhập Bilibili bằng QR của verse trả cả `loginUrl`. Giao diện không hiện link dạng chữ.

### #8 · Cập nhật đẩy thay cho poll 3 giây — Mở
- Màn: đầu trang (§5), Việc nền (§11), Bàn dựng.
- Code làm khác: máy chủ đẩy sự kiện qua `GET /api/events` (SSE). `GET /api/status` và `GET /api/activity` giữ đúng hình dữ liệu, dùng cho lần tải đầu và khi mất kết nối sự kiện; chỉ lúc đó client mới poll 3 giây.
- Lý do: trạng thái lên giao diện trong ≤ 1 giây thay vì ≤ 3 giây, và không còn nhiều vòng poll chạy song song. Giao diện không đổi.
- Hỏi design: đồng ý sửa câu "poll 3 giây khi tab đang hiện" ở §5 và §11 thành "nhận sự kiện đẩy; mất kết nối thì poll 3 giây"?

### #9 · Không nhập dữ liệu yt-factory — Mở
- Chủ sản phẩm chốt (29/10/2026): Viderse bản verse bắt đầu với dữ liệu trắng, không có bộ nhập dữ liệu yt-factory.
- Ảnh hưởng: không có job cũ, nên các chỗ sau không còn việc gì để làm:
  - [01-ia-flows.md](01-ia-flows.md) §0.1, dòng **Bản cũ** (job cũ có MP4 → Sẵn sàng/Đã đăng; job cũ dở → danh sách dọn);
  - [09-migration-yt-factory.md](09-migration-yt-factory.md) §3, dòng nhóm **Bản cũ**;
  - Cài đặt › Lưu trữ: danh sách job cũ dở;
  - Gói V22: migration đổi đường dẫn tuyệt đối cũ sang tương đối.
- Hỏi design: gỡ các chỗ trên khỏi tài liệu gốc?

### #10 · Không còn Electron — Mở
- Chủ sản phẩm chốt (29/10/2026): Viderse chạy dạng web, máy chủ do launchd giữ chạy như Noverse; không có app Electron.
- Code làm khác:
  - Gói V14: `Notification` của Electron và `app.dock.setBadge` thành Notification API của trình duyệt và `navigator.setAppBadge` (cài Viderse dạng PWA).
  - Gói V22, **Chọn thư mục…**: trình duyệt không trả được đường dẫn thật, nên hộp chọn là danh sách thư mục do máy chủ liệt kê (đi từ ổ đang cắm hoặc thư mục nhà). **Mở trong Finder** mở trên máy đang chạy Viderse.
- Hỏi design: hộp chọn thư mục phía máy chủ có cần artboard riêng không, hay dùng lại khung sheet chọn ổ của `MSettings` · `store=pick`?

### #11 · Truy cập từ điện thoại (D1) — Mở
- Chủ sản phẩm chốt (29/10/2026): §10 chọn cách **(a)**: nghe trên mạng LAN, ghép thiết bị một lần bằng mã QR, sau đó dùng cookie phiên.
- Đề nghị design: bỏ chữ "chờ quyết định D1" ở tiêu đề §10 và ở [09-migration-yt-factory.md](09-migration-yt-factory.md) §4; Gói V10 làm theo (a).

### #12 · `/api/status` và `/api/activity` cùng tồn tại — Mở
- §11 ghi "Thay `GET /api/status` ở §5 bằng `GET /api/activity`", nhưng hình dữ liệu của `/api/activity` không có `paused`, `reviewCount` và `attention`. Đầu trang cần ba trường này cho viên việc, badge Duyệt và chấm ở bánh răng.
- Dev đề nghị: giữ cả hai. Đầu trang đọc §5; popover Việc nền và thông báo đọc §11; cả hai cùng được làm mới qua sự kiện đẩy (#8).
- Hỏi design: đồng ý, hay gộp ba trường trên vào `/api/activity`?

### #13 · “Khớp theo giọng” cho một đoạn phụ đề — Mở <!-- r27-editor -->
- Màn: `Editor` · `focus=caption`, `fix=todo` → bấm **Khớp theo giọng**.
- Code hiện có: checklist `captionAlignment` (“Chỉnh mốc từng từ trong editor theo các cue lệch”) và `captionKaraoke` (“Đo mốc từng từ…”), nhưng sửa vẫn là kéo tay.
- Hỏi dev: dùng lại bước tách tiếng + mốc từng từ đang có trên máy (Trinh sát) để căn lại riêng các cue lệch, trong khoảng ±2 giây quanh cue, được không? Ước lượng thời gian cho 1 và cho 3 đoạn. Nếu chưa làm được trong V13: giữ nút, đổi thành “Dời theo đề xuất +0,6 s” dùng `offsetSec` của checklist.

### #14 · Lớp `.vv-badge` chưa có định nghĩa — Mở
- Màn: đầu trang › mục **Duyệt** (badge số video chờ duyệt), `Kit`.
- Artboard dùng `.vv-badge`, nhưng `kit.css`, `tokens.css` và các artboard không định nghĩa lớp này nên không có màu chữ trên nền vàng.
- Code tạm dùng mực nâu `#3D2A08` trên nền vàng `warning`, khai báo một chỗ (`--vv-badge-ink` trong `apps/viderse/web/src/styles/viderse.css`).
- Hỏi design: chốt màu chữ, nền, cỡ và bo góc của badge, rồi thêm vào `kit.css`/`03-tokens.md`.

### #15 · Màu orb trợ lý: kit `AgentOrb` lệch nhẹ so với board `Brand` — Mở
- Viderse dùng `AgentOrb` của kit như [05-kit.md](05-kit.md) dặn. Sắc độ các cung của orb hơi khác hình vẽ ở board `Brand` › 4 trợ lý; màu chữ của trợ lý vẫn đúng [03-tokens.md](03-tokens.md).
- Hỏi design: giữ `AgentOrb` của kit (Noverse và Viderse cùng một orb), hay cần thông số màu riêng cho Viderse?

### #16 · Mốc đổi sang bản điện thoại: 700px hay 768px — Mở
- [08-dev-prompt.md](08-dev-prompt.md) Gói V10 ghi "Dưới 768px: Thanh dưới 5 mục". Gói V0 và [04-components.md](04-components.md) `AppHeader` lại ghi "Hẹp hơn 700px thì `MainNav` cuộn ngang"; các artboard máy tính cũng có `@media (max-width:700px)` cho nav.
- Nếu dưới 768px đã là bản điện thoại thì không còn bề ngang nào để luật 700px của nav máy tính có tác dụng.
- Code đang làm: dưới 768px dùng khung điện thoại (theo V10). Nav máy tính vẫn giữ CSS cuộn ngang, không rớt dòng.
- Hỏi design: chốt một mốc. Nếu muốn khoảng 700–767px vẫn là bản máy tính thì đổi câu ở V10.
