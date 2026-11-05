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

### #6 · Điện thoại: mở link đăng nhập Bilibili trong app thay vì quét — Đã chốt <!-- r24-biliqr -->
- Màn: `MSettings` · `qr=waiting` · dòng **Bilibili trên chính máy này? → Mở app**.
- Người dùng xem Viderse trên điện thoại thường cũng cài 哔哩哔哩 trên chính máy đó, nên không quét được mã trên màn hình mình.
- Hỏi dev: mở `loginUrl` (link trong mã QR) trên điện thoại có vào app 哔哩哔哩 và hiện màn xác nhận không? Nếu không: bỏ dòng này, thay bằng câu “Quét bằng điện thoại khác, hoặc mở Cài đặt trên máy tính”.
- Dev (29/10/2026): sẽ thử trên điện thoại thật khi làm bản điện thoại và ghi kết quả vào đây. Giữ dòng này tới lúc đó.
- Trả lời (design, 01/11/2026): Giữ dòng *Mở app* tới khi dev thử trên máy thật. Mở được app 哔哩哔哩 và màn xác nhận → giữ; không được → thay bằng “Quét bằng điện thoại khác, hoặc mở Cài đặt trên máy tính” (06-copy). Không cần vẽ lại. <!-- r32 -->

### #7 · `loginUrl` chưa về tới giao diện — Đã chốt <!-- r24-biliqr -->
- yt-factory: `startBilibiliQrLogin()` trả `loginUrl` nhưng `startBilibiliQrLoginAction` chỉ trả `qrcodeKey`, `qrImage`, `expiresAt`.
- Đề nghị: bản verse trả cả `loginUrl` (§24.1) để dùng cho #6. Giao diện không hiện link dạng chữ.
- Trả lời (dev, 29/10/2026): API đăng nhập Bilibili bằng QR của verse trả cả `loginUrl`. Giao diện không hiện link dạng chữ.

### #8 · Cập nhật đẩy thay cho poll 3 giây — Đã chốt
- Màn: đầu trang (§5), Việc nền (§11), Bàn dựng.
- Code làm khác: máy chủ đẩy sự kiện qua `GET /api/events` (SSE). `GET /api/status` và `GET /api/activity` giữ đúng hình dữ liệu, dùng cho lần tải đầu và khi mất kết nối sự kiện; chỉ lúc đó client mới poll 3 giây.
- Lý do: trạng thái lên giao diện trong ≤ 1 giây thay vì ≤ 3 giây, và không còn nhiều vòng poll chạy song song. Giao diện không đổi.
- Hỏi design: đồng ý sửa câu "poll 3 giây khi tab đang hiện" ở §5 và §11 thành "nhận sự kiện đẩy; mất kết nối thì poll 3 giây"?
- Trả lời (design, 01/11/2026): Đồng ý. §5 và §11 đã sửa: “nhận sự kiện đẩy; mất kết nối thì poll 3 giây”. <!-- r32 -->

### #9 · Không nhập dữ liệu yt-factory — Đã chốt
- Chủ sản phẩm chốt (29/10/2026): Viderse bản verse bắt đầu với dữ liệu trắng, không có bộ nhập dữ liệu yt-factory.
- Ảnh hưởng: không có job cũ, nên các chỗ sau không còn việc gì để làm:
  - [01-ia-flows.md](01-ia-flows.md) §0.1, dòng **Bản cũ** (job cũ có MP4 → Sẵn sàng/Đã đăng; job cũ dở → danh sách dọn);
  - [09-migration-yt-factory.md](09-migration-yt-factory.md) §3, dòng nhóm **Bản cũ**;
  - Cài đặt › Lưu trữ: danh sách job cũ dở;
  - Gói V22: migration đổi đường dẫn tuyệt đối cũ sang tương đối.
- Hỏi design: gỡ các chỗ trên khỏi tài liệu gốc?
- Trả lời (design, 01/11/2026): Đồng ý. Đã gỡ dòng Bản cũ ở ia-flows §0.1 và 09-migration §3, gỡ migration đường dẫn cũ ở V22. Lưu trữ không có danh sách job cũ dở. <!-- r32 -->

### #10 · Không còn Electron — Đã chốt
- Chủ sản phẩm chốt (29/10/2026): Viderse chạy dạng web, máy chủ do launchd giữ chạy như Noverse; không có app Electron.
- Code làm khác:
  - Gói V14: `Notification` của Electron và `app.dock.setBadge` thành Notification API của trình duyệt và `navigator.setAppBadge` (cài Viderse dạng PWA).
  - Gói V22, **Chọn thư mục…**: trình duyệt không trả được đường dẫn thật, nên hộp chọn là danh sách thư mục do máy chủ liệt kê (đi từ ổ đang cắm hoặc thư mục nhà). **Mở trong Finder** mở trên máy đang chạy Viderse.
  - Gói V22 và W13 › Lưu trữ: thư mục lưu dự án mặc định là `~/.viderse/projects` (thư mục dữ liệu của bản web), thay cho `~/Library/Application Support/Viderse/data/projects` của Electron. Đổi thư mục không cần khởi động lại app: máy chủ đọc đường dẫn từ cơ sở dữ liệu, không dùng `storage.json`.
- Hỏi design: hộp chọn thư mục phía máy chủ có cần artboard riêng không, hay dùng lại khung sheet chọn ổ của `MSettings` · `store=pick`?
- Trả lời (design, 01/11/2026): Đồng ý. Hộp chọn thư mục phía máy chủ dùng lại khung chọn ổ (`Settings` · `store=pick` là Dialog, `MSettings` · `store=pick` là Sheet), mỗi dòng là thư mục, có đường dẫn mono và nút lên một cấp. Không cần artboard riêng (cùng #21). <!-- r32 -->

### #11 · Truy cập từ điện thoại (D1) — Đã chốt
- Chủ sản phẩm chốt (29/10/2026): §10 chọn cách **(a)**: nghe trên mạng LAN, ghép thiết bị một lần bằng mã QR, sau đó dùng cookie phiên.
- Đề nghị design: bỏ chữ "chờ quyết định D1" ở tiêu đề §10 và ở [09-migration-yt-factory.md](09-migration-yt-factory.md) §4; Gói V10 làm theo (a).
- Trả lời (design, 01/11/2026): Đồng ý. Đã bỏ “chờ quyết định D1” ở 07 §10, 09 §4 và Gói V10. <!-- r32 -->

### #12 · `/api/status` và `/api/activity` cùng tồn tại — Đã chốt
- §11 ghi "Thay `GET /api/status` ở §5 bằng `GET /api/activity`", nhưng hình dữ liệu của `/api/activity` không có `paused`, `reviewCount` và `attention`. Đầu trang cần ba trường này cho viên việc, badge Duyệt và chấm ở bánh răng.
- Dev đề nghị: giữ cả hai. Đầu trang đọc §5; popover Việc nền và thông báo đọc §11; cả hai cùng được làm mới qua sự kiện đẩy (#8).
- Hỏi design: đồng ý, hay gộp ba trường trên vào `/api/activity`?
- Trả lời (design, 01/11/2026): Đồng ý giữ cả hai. 07 §11 đã sửa. <!-- r32 -->

### #13 · “Khớp theo giọng” cho một đoạn phụ đề — Đã chốt <!-- r27-editor -->
- Màn: `Editor` · `focus=caption`, `fix=todo` → bấm **Khớp theo giọng**.
- Code hiện có: checklist `captionAlignment` (“Chỉnh mốc từng từ trong editor theo các cue lệch”) và `captionKaraoke` (“Đo mốc từng từ…”), nhưng sửa vẫn là kéo tay.
- Hỏi dev: dùng lại bước tách tiếng + mốc từng từ đang có trên máy (Trinh sát) để căn lại riêng các cue lệch, trong khoảng ±2 giây quanh cue, được không? Ước lượng thời gian cho 1 và cho 3 đoạn. Nếu chưa làm được trong V13: giữ nút, đổi thành “Dời theo đề xuất +0,6 s” dùng `offsetSec` của checklist.
- Trả lời (design, 01/11/2026): Đồng ý phương án dự phòng: chưa làm được căn ±2 giây thì nút là “Dời theo đề xuất +0,6 s” (dùng `offsetSec`). Làm được thì giữ “Khớp theo giọng”. <!-- r32 -->

### #14 · Lớp `.vv-badge` chưa có định nghĩa — Đã chốt
- Màn: đầu trang › mục **Duyệt** (badge số video chờ duyệt), `Kit`.
- Artboard dùng `.vv-badge`, nhưng `kit.css`, `tokens.css` và các artboard không định nghĩa lớp này nên không có màu chữ trên nền vàng.
- Code tạm dùng mực nâu `#3D2A08` trên nền vàng `warning`, khai báo một chỗ (`--vv-badge-ink` trong `apps/viderse/web/src/styles/viderse.css`).
- Hỏi design: chốt màu chữ, nền, cỡ và bo góc của badge, rồi thêm vào `kit.css`/`03-tokens.md`.
- Trả lời (design, 01/11/2026): Board vẽ badge **nền vàng nhạt, chữ vàng**, không phải nền vàng đặc: nền `rgba(246,203,140,.16)` = token mới `--vv-badge-bg`, chữ `--vv-st-review` (#F6CB8C), cao 18 · tối thiểu 18 · đệm 0 5px · bo tròn · 11px/600 số đều. Đã thêm `.vv-badge` vào kit.css và 03-tokens. Bỏ `--vv-badge-ink` nâu. <!-- r32 -->

### #15 · Màu orb trợ lý: kit `AgentOrb` lệch nhẹ so với board `Brand` — Đã chốt
- Viderse dùng `AgentOrb` của kit như [05-kit.md](05-kit.md) dặn. Sắc độ các cung của orb hơi khác hình vẽ ở board `Brand` › 4 trợ lý; màu chữ của trợ lý vẫn đúng [03-tokens.md](03-tokens.md).
- Hỏi design: giữ `AgentOrb` của kit (Noverse và Viderse cùng một orb), hay cần thông số màu riêng cho Viderse?
- Trả lời (design, 01/11/2026): Giữ `AgentOrb` của kit, Noverse và Viderse dùng chung một orb. Board `Brand` › 4 trợ lý là hình minh hoạ; khi lệch, kit là chuẩn. <!-- r32 -->

### #16 · Mốc đổi sang bản điện thoại: 700px hay 768px — Đã chốt
- [08-dev-prompt.md](08-dev-prompt.md) Gói V10 ghi "Dưới 768px: Thanh dưới 5 mục". Gói V0 và [04-components.md](04-components.md) `AppHeader` lại ghi "Hẹp hơn 700px thì `MainNav` cuộn ngang"; các artboard máy tính cũng có `@media (max-width:700px)` cho nav.
- Nếu dưới 768px đã là bản điện thoại thì không còn bề ngang nào để luật 700px của nav máy tính có tác dụng.
- Code đang làm: dưới 768px dùng khung điện thoại (theo V10). Nav máy tính vẫn giữ CSS cuộn ngang, không rớt dòng.
- Hỏi design: chốt một mốc. Nếu muốn khoảng 700–767px vẫn là bản máy tính thì đổi câu ở V10.
- Trả lời (design, 01/11/2026): Một mốc: **768px**. Dưới 768px là khung điện thoại; từ 768px trở lên là bản máy tính, `MainNav` không đủ chỗ thì cuộn ngang (không đặt mốc 700px). Đã sửa V0, 04-components › AppHeader. <!-- r32 -->

### #17 · Màn Chuẩn bị máy nằm ở đâu — Đã chốt
- [02-stack-verse.md](02-stack-verse.md) §4 ghi Lần đầu mở là màn `Setup` của `platform/webapp`. Màn đó chỉ tạo tài khoản quản trị (trước khi đăng nhập), không cài công cụ.
- `Onboard` · `step=setup` (W1, 6 việc, Đăng nhập ChatGPT) cần chạy sau khi đăng nhập.
- Code làm: thêm route `#/setup` (Chuẩn bị máy). Sau khi đăng nhập, máy chưa xong 6 việc thì app mở `#/setup`; xong thì sang Tạo kênh. Trên điện thoại, `#/setup` hiện "Mở trên máy tính" như ia-flows §6.2.
- Hỏi design: đồng ý thêm dòng `#/setup` vào bảng route ở §4?
- Trả lời (design, 01/11/2026): Đồng ý thêm `#/setup`. Đã thêm dòng vào bảng route ở 02-stack §4. <!-- r32 -->

### #18 · Cài đặt › Công cụ: “Máy sẵn sàng · 2/2 luồng” — Đã chốt
- Artboard: `Settings` · `section=tools`.
- Số luồng thuộc phase Hàng chờ (W9/L5). Hiện chưa có cài đặt luồng, nên `/api/status` chưa có `lanes`.
- Code làm: thẻ tổng chỉ ghi “Máy sẵn sàng” (kèm “có n bản cập nhật” nếu có), không bịa số luồng. Phase Hàng chờ thêm `lanes` thì hiện lại đủ câu.
- Hỏi design: đồng ý tạm bỏ “· 2/2 luồng” tới phase Hàng chờ?
- Trả lời (design, 01/11/2026): Đồng ý tạm bỏ “· 2/2 luồng” tới khi có `lanes`. <!-- r32 -->

### #19 · Cột trái Cài đặt: hàng “Giới hạn từng kênh” — Đã chốt
- Artboard: `Settings` (mọi `section`), nhóm “Ở trang khác”, trỏ sang `Channel`.
- Router chưa có trang giới hạn của kênh (`#/channels/:id?tab=sources` thuộc phase Kênh).
- Code làm: chưa dựng hàng này, không đặt link tới trang chưa có. Phase Kênh có route thì thêm.
- Trả lời (design, 01/11/2026): Đồng ý: chưa dựng hàng này tới khi có route Kênh › Nguồn & tự động. <!-- r32 -->

### #20 · Báo lỗi: ngày phát hành của bản mới — Đã chốt
- Artboard: `Settings` · `section=logs`, thẻ phiên bản có ngày phát hành.
- `/api/version` chưa trả ngày phát hành, nên code chỉ hiện số bản mới nhất.
- Hỏi design: cần ngày thật không? Nếu cần, máy chủ phải đọc ngày từ nguồn phát hành.
- Trả lời (design, 01/11/2026): Không cần ngày. Board đã bỏ: “Viderse 1.4.0 · Bản mới nhất”. <!-- r32 -->

### #21 · Lưu trữ: chọn thư mục bằng danh sách của máy chủ — Đã chốt
- Artboard: `Settings` · `section=storage&store=pick`; `MSettings` · `screen=storage&store=pick`.
- Artboard mở hộp chọn thư mục của hệ điều hành. Web không lấy được đường dẫn thật trên máy chủ (xem #10).
- Code làm: “Chọn thư mục…” mở danh sách thư mục do máy chủ liệt kê (Dialog trên máy tính, Sheet trên điện thoại), chọn xong quay về khối chọn ngay tại chỗ.
- Trả lời (design, 01/11/2026): Đồng ý (xem #10). <!-- r32 -->

### #22 · Lưu trữ: “Ổ ngoài USB-C” — Đã chốt
- Artboard ghi “Ổ ngoài USB-C”. `diskutil` chỉ báo giao tiếp (`USB`, `Thunderbolt`…), không phân biệt đầu cắm USB-C với USB-A.
- Code làm: hiện “Ổ ngoài USB” / “Ổ ngoài Thunderbolt” theo đúng `diskutil`; chưa phân loại được thì chỉ ghi “Ổ ngoài”.
- Hỏi design: đồng ý đổi chữ mẫu trong artboard thành “Ổ ngoài USB”?
- Trả lời (design, 01/11/2026): Đồng ý. Board `Settings`, `MSettings` đã đổi “Ổ ngoài USB · APFS”. <!-- r32 -->

### #23 · Lưu trữ: số video trong banner chuyển thư mục — Đã chốt
- Artboard: `store=done` có “2 video đã chạy tiếp.”; `store=moving` có “Xem khi xong ›”; banner `pick`/`moving` nêu số video đang chạy.
- Chưa có hàng chờ video (phase Hàng chờ), nên chưa đếm được video bị dừng rồi chạy tiếp. “Xem khi xong ›” trong artboard chỉ là nút đổi trạng thái tĩnh.
- Code làm: banner nêu số việc nền đang chạy (`/api/activity`), không có việc nào thì dùng câu không số. Bỏ “2 video đã chạy tiếp.” và “Xem khi xong ›” tới phase Hàng chờ.
- Trả lời (design, 01/11/2026): Đồng ý tạm tới phase Hàng chờ. <!-- r32 -->

### #24 · Nút cao 32px (`ghost btn`) trong Cài đặt — Đã chốt
- Artboard `Settings` vẽ các nút `ghost btn` (ví dụ “Xem trước những gì sẽ dọn”, “Mở trang xAI”) cao 32px, bo 8, chữ 13. Kit lúc đầu chỉ có 30 (`sm`), 36 (mặc định) và 44 (`lg`).
- Trả lời (dev, 30/10/2026): thêm `size="compact"` (32 · bo 8 · chữ 13) vào `Button` của kit. Chỉ thêm, Noverse không đổi. Mỗi nút dùng đúng cỡ theo class artboard tại chỗ đó: `ghost btn` → `compact`, `vv-btn--sm` → `sm`, `vv-btn` → mặc định, điện thoại `vv-btn--m` → `lg`.

### #25 · Tài khoản nền tảng: mốc “từ HH:MM” và “1 video” — Đã chốt
- Artboard: `Settings` · `section=accounts&bili=blocked` ghi “từ 00:10”; `yt=bot` ghi “phụ đề của 1 video”.
- Cơ sở dữ liệu chỉ có `paused_until`, không có mốc bắt đầu bị chặn. Lỗi bot của YouTube cũng chưa đếm số video bị ảnh hưởng.
- Code làm: “từ HH:MM” lấy giờ của lần lỗi cuối (`lastError.at`), tức lần lỗi đã đẩy nền tảng sang tạm chặn. Banner bot ghi “một số video”, không bịa số.
- Hỏi design: chấp nhận? Nếu cần đúng hơn: thêm cột `paused_since`, và đếm `videosAffected` khi có hàng chờ tải.
- Trả lời (design, 01/11/2026): Chấp nhận cả hai, không cần thêm dữ liệu: “từ HH:MM” = lần lỗi cuối; banner bot ghi “một số video” (board đã sửa). <!-- r32 -->

### #26 · Kết nối AI: dữ liệu artboard chưa có nguồn — Đã chốt
- Artboard: `Settings` · `section=connect|agents`; `MSettings` · `screen=connect|agents`.
- **Gói Grok:** xAI không trả tên gói, nên thẻ Grok bỏ “gói …”. ChatGPT vẫn có gói, đọc từ `chatgpt_plan_type` trong token.
- **“Tháng này $1,84” (`spendMonth`):** `image_receipts` không ghi giá, nên chưa hiện. Cần pipeline ghi giá mỗi lần gọi.
- **Ghi chú của model** (“Trong gói · viết và đọc ảnh tốt nhất”): đây là câu biên tập, wire chỉ có khả năng, ngữ cảnh và giá. Code hiện “chữ · đọc ảnh · 272K ngữ cảnh”.
- **Giờ đặt lại hạn mức:** artboard ghi 07:00. Code dùng mốc thật nếu provider báo, nếu không thì dùng nửa đêm giờ máy, vì đó là lúc trần ảnh mỗi ngày của app (D7) đặt lại.
- **Mức suy nghĩ trên điện thoại:** artboard điện thoại không có chỗ chỉnh. Code đặt nó trong sheet Đạo diễn.
- **Nút thêm nhà cung cấp:** bị khoá khi ô khoá API còn trống. Artboard luôn điền sẵn nên không vẽ trạng thái này.
- Hỏi design: chốt từng ý trên. Muốn câu ghi chú theo model thì cần một bảng câu.
- Trả lời (design, 01/11/2026): Chốt từng ý: (1) bỏ “gói …” ở thẻ Grok — board đã sửa; (2) ẩn “tháng này $…” — board đã bỏ; (3) dòng phụ model theo khả năng “chữ · đọc ảnh · 272K ngữ cảnh”, không cần câu biên tập (câu trên board là minh hoạ); (4) giờ đặt lại = mốc thật, không có thì 00:00; (5) điện thoại chỉnh mức suy nghĩ trong sheet Đạo diễn — đồng ý; (6) nút thêm nhà cung cấp khoá khi ô khoá API trống — đồng ý. <!-- r32 -->

### #27 · Kết nối AI: thứ tự thẻ và thẻ trùng loại — Đã chốt
- Artboard: `Settings` · `conn=list` xếp ChatGPT, Grok, Gemini, Ollama (cũ trước); `conn=added` và `conn=failed` đặt thẻ vừa thêm hoặc vừa lỗi lên đầu.
- Code làm: xếp mới nhất trước (`created_at DESC`), khớp `added`/`failed`, lệch `list`.
- Thẻ lỗi vẫn để ô nguồn đó bấm được, nên dán khoá mới sẽ tạo thẻ thứ hai cùng loại thay vì sửa thẻ cũ.
- Hỏi design: giữ “mới nhất trước” hay “lỗi trước, còn lại cũ trước”? Cho thêm thẻ trùng loại, hay bắt dùng **Đổi khoá** trên thẻ cũ?
- Trả lời (design, 01/11/2026): Thứ tự: **thẻ lỗi lên đầu, còn lại theo thứ tự thêm (cũ trước)** — khớp `conn=list`; thẻ vừa thêm đứng đầu tới khi rời trang — khớp `conn=added`. **Không cho thẻ trùng loại**: chọn lại nguồn đã có thì cuộn tới thẻ đó và mở *Đổi khoá* (ngoại lệ: Tương thích OpenAI, Ollama khác địa chỉ). <!-- r32 -->

### #28 · Trợ lý AI: giá mỗi video và nguồn vẽ thứ 4 — Đã chốt
- Artboard: `Settings` · `section=agents`; `MSettings` · `screen=agents`.
- “≈ $0,27 mỗi video” = giá mỗi ảnh × 7. Số 7 lấy từ design (“~7 ảnh” mỗi video), khai báo một chỗ (`IMAGES_PER_VIDEO` trong `AgentsSection.tsx`), chưa lấy từ template của kênh.
- Kho có nguồn vẽ thứ 4 (Tương thích OpenAI), nên lưới 3 cột của thẻ nguồn vẽ xuống dòng 2; artboard chỉ vẽ 3 nguồn.
- Hỏi design: giữ số 7 cố định hay lấy số ảnh của template? Lưới 4 nguồn: xuống dòng (đang làm), cuộn ngang, hay 4 cột?
- Trả lời (design, 01/11/2026): Cài đặt không ghi giá mỗi video (không gắn với kênh nào; board đã bỏ “≈ $0,27 mỗi video”), chỉ ghi giá mỗi ảnh. Giá mỗi video tính theo template ở màn Template (#41) và Kế hoạch. 4 nguồn vẽ: lưới **2 × 2**; 3 nguồn giữ 3 cột. <!-- r32 -->

### #29 · Tài khoản nền tảng trên điện thoại: state chưa vẽ và giãn dòng — Đã chốt
- `MSettings` không có frame cho `yt=bot`, hồ sơ YouTube “Tắt” và `qr=loading`.
- Code làm: banner bot nằm trong thân thẻ YouTube (cao thêm 48px, như banner Bilibili bị chặn/hết hạn); `qr=loading` dùng lại khung chờ của máy tính.
- Thẻ điện thoại trong artboard giãn dòng 1,5; artboard máy tính và phần còn lại của app dùng 1,55. Code chỉ đặt 1,5 trong thẻ tài khoản và sheet trạng thái trên điện thoại.
- Hỏi design: duyệt hai state trên? Giãn dòng 1,5 là chuẩn cho cả `MSettings` hay chỉ thẻ này?
- Trả lời (design, 01/11/2026): Duyệt hai state như code (banner bot trong thân thẻ, `qr=loading` dùng khung chờ của máy tính). Giãn dòng 1,5 chỉ trong thẻ tài khoản và sheet trạng thái điện thoại như code; phần còn lại 1,55. <!-- r32 -->

### #30 · Khung Cài đặt: dải mất kết nối, tông chấm cảnh báo, nhãn pha cài — Đã chốt
- Artboard `Settings` không vẽ dải mất kết nối. Code chèn dải giữa đầu trang và nội dung, nội dung bị đẩy xuống thay vì bị che.
- Chấm cảnh báo ở bánh răng lấy tông theo `kind` trong `/api/status.attention` (`tool_failed`, `login_expired`, `tool_update`, `quota_low`), bảng kind → tông nằm ở web. Thêm kind mới thì cần chốt tông ở đây.
- Tiến trình cài công cụ trả `phaseLabel` là câu tiếng Việt của máy chủ, không phải mã pha; client khác (nếu có) hiện nguyên câu này.
- Pha không biết tổng dung lượng (tải mô hình nhận giọng nói, dựng môi trường Python) trả `pct: null`. Artboard chỉ vẽ pha có phần trăm (“· 62%”). Code chỉ ghi tên pha, cạnh đó là vòng quay của hàng; không vẽ “0%” và thanh đứng yên.
- Hỏi design: dải đẩy hay phủ lên nội dung? Đồng ý bảng tông theo kind? Pha không biết tổng có cần thanh chạy vô định không?
- Trả lời (design, 01/11/2026): Dải mất kết nối **đẩy** nội dung (đồng ý). Bảng tông: `tool_failed`, `login_expired` → cam `--vv-danger`; `tool_update` → xanh `--vv-info`; `quota_low` → vàng `--vv-st-review`; kind mới mặc định cam. Pha không biết tổng: **có** thanh chạy vô định `.vv-progress--working` dưới tên pha, không ghi phần trăm. <!-- r32 -->

### #31 · Khám phá: điều khiển cần kênh của bạn và hàng chờ — Đã chốt <!-- r28-explore -->
- Artboard: `Explore` (xem trước, chọn nhiều, thẻ đối thủ), `MExplore`.
- Kênh của bạn có ở gói Kênh (V4), hàng chờ làm video ở gói Hàng chờ (V6). Tới lúc đó code vẽ các điều khiển sau ở trạng thái khoá, kèm lý do “Chưa có kênh của bạn để làm video — tạo kênh ở mục Kênh.”: “Làm cho <kênh>”, chọn kênh, “Làm video”, nút chọn nhiều “Làm N video” (bỏ “cho <kênh>”), “Liên kết với kênh…”, lọc Trạng thái ngoài “Tất cả”.
- Câu cần số liệu hàng chờ viết không có số: gợi ý “Lên đầu hàng chờ khi bấm Làm video.” (artboard có giờ dự kiến “bắt đầu khoảng 00:40”); hộp Bỏ theo dõi không ghi “2 video trong hàng chờ bị bỏ”.
- Panel Vì sao không có link “Đổi ngưỡng ở Kênh › Nguồn & tự động” vì màn đó chưa có. Panel giữ 3 dòng số liệu (lượt xem, mức thường, ngày đăng); ngưỡng ×3 nằm trong câu giải thích.
- Khối chương của video trong panel xem trước bị ẩn: Trinh sát chưa lưu chương.
- Hỏi design: chấp nhận trạng thái khoá và câu không số tới khi có kênh/hàng chờ? Khi có, các nút mở theo đúng artboard.
- Trả lời (design, 30/10/2026): đồng ý cả bốn ý, tới V4/V6 thì mở đúng artboard. Lý do khoá viết ngắn hơn, một câu, ngay dưới nút hoặc trong tooltip: “Cần một kênh của bạn để làm video. Tạo kênh ở mục Kênh.” Nút khoá vẫn nhận focus và đọc được lý do (`aria-disabled` + `aria-describedby`), không ẩn. “Lên đầu hàng chờ khi bấm Làm video.” và hộp Bỏ theo dõi không có số: được. Vì sao không có link đổi ngưỡng, ẩn khối chương: được; khi Trinh sát lưu chương thì hiện lại (ghi ở 10-gap-closure).

### #32 · Khám phá: trạng thái quét và lỗi thêm đối thủ chưa có artboard — Đã chốt <!-- r28-explore -->
- Artboard `Explore` vẽ quét đang chạy, tạm dừng, bị giới hạn tốc độ, đã xong.
- Code thêm câu cho trạng thái còn lại của việc quét lịch sử: đang chờ (`queued`), chưa quét (`idle`), đủ giới hạn (`capped`), quét hết (`complete`), đã huỷ, lỗi, đang kiểm tra video mới. Câu nằm ở `screens.explore.scanStates` trong `i18n/vi.ts`.
- Lỗi thêm đối thủ: mỗi `code` của `POST /api/rivals` một câu dưới ô nhập (`invalid_input`, `not_found`, `exists` kèm nút Mở, `rate_limited` kèm giờ thử lại, `blocked`, `unexpected_page`, lỗi khác). Câu nằm ở `screens.explore.addErrors`.
- Hỏi design: duyệt các câu này, hoặc vẽ thêm frame.
- Trả lời (design, 30/10/2026): duyệt câu, không vẽ thêm frame (cùng bố cục, chỉ đổi câu). Bảng câu chuẩn nằm ở [06-copy.md › Khám phá](06-copy.md#copy-explore); chỗ sửa so với `vi.ts`: bỏ “—” nối hai mệnh đề, dùng dấu chấm; `idle` và `error` gọi đúng tên nút; `capped` nói rõ giới hạn là bao nhiêu video; `blocked` dẫn sang Tài khoản nền tảng giống `cookies_failed`. Artboard `Explore` · `scan=done` đã đổi theo câu `capped` mới.

### #33 · Menu ⋯ của đối thủ: đệm hàng 6px, cách 0px — Đã chốt <!-- r28-explore -->
- Artboard `Explore` · menu đối thủ: hàng đệm 8px, cách nhau 2px, rộng 272px.
- `ActionMenu` của kit đệm 6/10, cách 0, khớp artboard `Kit` (`.vv-menu__item`) và menu của Noverse. Code đã cho `ActionMenu` prop `width` (đối thủ dùng 272), nên chỉ còn lệch đệm dọc: mỗi hàng thấp hơn ~2,7px.
- Hỏi design: thêm prop đệm hàng cho kit để riêng menu này 8/2, hay coi đệm 6/0 của Kit là chuẩn?
- Trả lời (design, 30/10/2026): đệm 6/10, cách 0 của Kit là chuẩn; không thêm prop đệm. Artboard `Explore` · `rivalMenu` đã sửa theo kit. Luật chung: menu nào cũng dùng `ActionMenu` như kit, chỉ đổi `width`.

### #34 · Lịch đăng: số “Mỗi tuần” và câu khi không có khung — Đã chốt <!-- r28-explore -->
- Artboard `Explore` · thẻ Lịch đăng chỉ vẽ kênh có khung lặp lại.
- “Mỗi tuần” = (số video có giờ − 1) chia độ dài thật của mẫu tính theo tuần, tối thiểu 1 tuần. Cách cũ (số video chia số tuần lịch mẫu chạm tới) cho kênh đăng 3 video/ngày ra 15 vì 30 video trong 9 ngày chạm 2 tuần lịch; cách mới ra ~22. Ô số hẹp (“5,6 video” đã xuống dòng) nên từ 1 video/tuần hiện số nguyên như artboard (“6 video”), dưới 1 giữ một chữ số lẻ (“0,4 video”).
- Không có khung nào, code ghi một trong bốn câu: tạm nghỉ đọc giờ đăng tới HH:MM; đang đọc giờ đăng của video còn lại; chưa đọc được giờ đăng của video nào; “Chưa thấy khung nào lặp lại trong mẫu này.” (đã đọc đủ mẫu, ví dụ kênh đăng giờ lệch nhau mỗi ngày).
- Hỏi design: duyệt cách tính và bốn câu.
- Trả lời (design, 30/10/2026): duyệt cách tính mới và cách hiện số (≥ 1: số nguyên; < 1: một chữ số lẻ). Đã ghi vào [07-data-contract.md §12](07-data-contract.md#posting-slots). Bốn câu duyệt với chỉnh nhỏ ở [06-copy.md › Khám phá](06-copy.md#copy-explore): câu tạm nghỉ nói vì sao (YouTube giới hạn), câu không có khung nói rõ điều kiện “lặp ít nhất 3 lần”.

### #35 · Khám phá: không có ô tìm video — Đã chốt <!-- r28-explore -->
- Artboard `Explore` không có ô tìm. API (`GET /api/rivals/<id>/videos?q=`, tối đa 200 ký tự) và CLI (`viderse rival videos`) tìm được theo tiêu đề; giao diện chưa có ô tìm.
- Hỏi design: cần ô tìm trên thanh lọc không? Nếu có, đặt ở đâu.
- Trả lời (design, 30/10/2026): cần. Ô **Tìm tiêu đề** 240px ở dòng tiêu đề, ngay trước **Chọn nhiều**; phím `/` để vào ô, Esc xoá. Tìm trong đối thủ đang chọn, cộng dồn với lọc và sắp xếp đang bật; không phân biệt hoa thường và dấu. Dòng đếm đổi thành “2 video có “floor” · trong 1.284 đã quét”. Không có kết quả: khối giữa lưới + **Xoá tìm**. Artboard `Explore` · `search=none|match|nomatch`.

### #36 · Khám phá trống: ví dụ là kênh tiếng Anh — Đã chốt <!-- r28-explore -->
- Artboard `Explore` · `state=empty` ghi ví dụ `@nocturnestories`, `youtube.com/@thelatearchive`, `UCx9…Qw`. Code chép nguyên văn.
- Hỏi design: app tiếng Việt, có đổi ví dụ sang kênh Việt không?
- Trả lời (design, 30/10/2026): giữ ví dụ theo ngách chứ không theo ngôn ngữ app: người dùng theo dõi cả kênh Anh lẫn kênh Việt. Đổi thành một kênh Anh, một kênh Việt, một mã UC: `@nocturnestories` · `youtube.com/@hosobian` · `UCx9…Qw`. Artboard `Explore` · `empty` đã đổi.

### #37 · Khám phá trên điện thoại — Đã chốt <!-- r28-explore -->
- Artboard `MExplore`. Gói này chưa làm `MExplore`: dưới 768px, màn máy tính xếp thành một cột (đối thủ, lưới ảo hoá, cột phải), vẫn thêm/bỏ đối thủ, quét, xem lịch đăng được. Làm `MExplore` đúng artboard để gói Điện thoại (V10).
- Hỏi design: đồng ý dời?
- Trả lời (design, 30/10/2026): đồng ý dời `MExplore` sang V10. Trong lúc chờ, bản một cột dưới 768px phải: không cuộn ngang, nút ≥ 44px khi `pointer: coarse`, ô thêm đối thủ ở trên cùng, cột phải (thông tin, quét, lịch đăng) xuống dưới lưới. Không cần artboard cho bản tạm.

### #38 · Template và Prompt ảnh: nút cần một video đã làm xong — Đã chốt
- Artboard: `Template` (`preview=fresh|stale|building`, “Vẽ thử ảnh bìa”), `Library` · `tab=prompts` (“Ảnh mẫu”, “Vẽ thử với…”).
- Hàng chờ và pipeline làm video là gói V6, editor là V13. Chưa có video nào đã làm xong và chưa có câu chuyện đã phân tích, nên chưa dựng được bản xem thử, chưa vẽ thử bìa, chưa vẽ ảnh mẫu.
- Code: trạng thái xem thử vẫn thật (`none|fresh|stale` theo `revision` của template). “Dựng lại bản xem thử”, “Vẽ thử ảnh bìa”, “Vẽ thử với…”, “Mở trong editor” hiện nhưng bị khoá (`aria-disabled`, nhận focus, đọc được lý do). Prompt ảnh hiện “Chưa có mẫu”. `preview=building` chưa xảy ra được.
- Hỏi design: duyệt trạng thái khoá tới V6/V13, và câu lý do trong `i18n/library.ts`.
- Trả lời (design, 01/11/2026): Duyệt khoá tới V6/V13. Câu lý do: Template “Cần ít nhất một video đã làm xong để dựng bản xem thử.”; Prompt ảnh “Cần ít nhất một video đã làm xong để vẽ thử.” Prompt ảnh ghi “Chưa có mẫu”. Không cần artboard thêm. <!-- r31-v27 -->

### #39 · Kênh › Video, số liệu đầu kênh, “Hôm nay x/y”, Dặn trợ lý — Đã chốt
- Artboard: `Channel` · `tab=videos`, đầu kênh, cột phải.
- Bảng video 4 cột cùng `BulkBar`, 4 số liệu đầu kênh, chấm “Chờ bạn”, thanh “Hôm nay x/y” và ô Dặn trợ lý cần job làm video (V6) và trợ lý (V11). Mức tự chủ và Ưu tiên kênh là phase sau (§20).
- Code: tab Video luôn hiện trạng thái kênh mới (`fresh`: “Kênh chưa có video” và hai thẻ dẫn). Không bịa số. Cột phải “Đang áp dụng cho kênh” có Tự lấy video, Template luân phiên, Tiêu đề–mô tả–tag, Trợ lý AI; chưa có Mức tự chủ. Xoá kênh luôn với `runningCount = 0`.
- Hỏi design: duyệt bản tạm này tới V6/V11.
- Trả lời (design, 01/11/2026): Duyệt bản tạm tới V6/V11: tab Video ở trạng thái `fresh`, không bịa số; ẩn số liệu đầu kênh, “Hôm nay x/y” và ô Dặn trợ lý tới khi có dữ liệu. Mức tự chủ, Ưu tiên kênh theo §20. <!-- r31-v27 -->

### #40 · Mẫu chuẩn khi tạo kênh — Đã chốt
- Artboard: `CreateChannel` · `step=done` (“Mẫu chuẩn chạy nền”), `Channel` · `tab=templates`.
- yt-factory dựng Mẫu chuẩn là một project editor (3 cảnh, nút Subscribe, tuyết, sóng âm). Editor là V13.
- Code: tạo kênh tạo ngay một template “Mẫu chuẩn · <tên kênh>” với recipe mặc định (Tách nền, prompt “Theo nhận diện kênh”, bìa “Chữ hook”, lớp phủ Tuyết) và gắn vào vòng luân phiên của kênh. Kênh tạo không tên (để Đạo diễn đặt) thì template tạm tên “Mẫu chuẩn”, rồi tự đổi sang “Mẫu chuẩn · <tên kênh>” khi kênh có tên (Đạo diễn đặt hoặc chủ tự đặt). Template đã đổi tên tay hoặc kênh khác cũng dùng thì giữ nguyên. Phần bố cục editor thêm ở V13.
- Hỏi design: duyệt.
- Trả lời (design, 01/11/2026): Duyệt, kể cả tự đổi tên “Mẫu chuẩn · <tên kênh>” và giữ tên đã đổi tay. <!-- r31-v27 -->

### #41 · Giá ước tính của template — Đã chốt
- Artboard: `Template` (giá mỗi video), 07 §15 `price.perImage`, hỏi đáp #28 (giá = 7 ảnh).
- Code tính theo số ảnh của chính template cộng 1 ảnh bìa: `(ảnh mỗi video + 1) × giá mỗi ảnh`. Template chọn “theo số phút” chưa biết số ảnh khi chưa có video nguồn, nên chỉ ghi cách tính, không ghi giá. Template không có ảnh vẫn tính 1 ảnh bìa. Không biết giá model vẽ thì chỉ ghi số ảnh.
- Hỏi design: duyệt bỏ hằng 7 ở màn Template.
- Trả lời (design, 01/11/2026): Duyệt bỏ hằng 7. Template theo số phút ghi “Số ảnh theo độ dài video nguồn”, không ghi giá. Không biết giá ảnh (`priceUSD: null`) thì chỉ ghi số ảnh. <!-- r31-v27 -->

### #42 · Nhận diện kênh: định dạng ảnh, lỗi và giờ tự thử — Đã chốt
- Artboard: `Channel` · `identity=ready|generating|failed`, `CreateChannel` · `step=done`.
- Avatar 800 × 800 PNG, ảnh bìa 2560 × 1440 JPEG ≤ 6 MB (yt-factory dùng WebP qua ffmpeg + libwebp; ở đây mã hoá thuần Go, không cần libwebp).
- Lỗi được lưu trên kênh, sống qua tải lại trang. Hết hạn mức ảnh thì ghi giờ tự thử từ nguồn vẽ. App tắt giữa chừng thì lần đó thành “Bị gián đoạn”, không tự vẽ lại (vẽ lại tốn tiền).
- Code ghi “Tự thử lại lúc HH:MM” thay cho “Thử lại lúc HH:MM” của artboard, cho rõ là hệ thống tự thử.
- Hỏi design: duyệt định dạng và câu.
- Trả lời (design, 01/11/2026): Duyệt PNG 800 × 800 và JPEG 2560 × 1440 ≤ 6 MB; duyệt “Tự thử lại lúc HH:MM” và “Bị gián đoạn” (kèm *Vẽ lại*, không tự vẽ). 06-copy dùng hai câu này. <!-- r31-v27 -->

### #43 · Bỏ liên kết đối thủ trên máy tính chưa có hộp thoại — Đã chốt
- Artboard: `Channel` · `tab=sources` chỉ có sheet bỏ liên kết ở `MChannel`.
- Code dùng `ConfirmDialog` của kit, tông cam nhạt (`dsoft`), lấy câu của sheet `MChannel`: “Bỏ liên kết <tên>?”, thân nêu hậu quả và đối thủ tham chiếu kế tiếp. Bỏ đối thủ cuối thì Tự lấy tắt, kèm lý do.
- Hỏi design: vẽ hộp thoại cho máy tính, hoặc duyệt bản này.
- Trả lời (design, 01/11/2026): Duyệt `ConfirmDialog` của kit, tông cam nhạt, câu của sheet `MChannel`. Không cần artboard riêng. <!-- r31-v27 -->

### #44 · Video nền: thêm kênh chạy nền, dán link không dò video — Đã chốt
- Artboard: `Library` · `tab=backdrops` (`nguon=adding|addvideo`).
- Thêm kênh nền ghi kênh ngay rồi đọc danh mục trong việc nền; thẻ kênh hiện “Đang đọc” rồi số video (yt-factory đọc trong lúc bấm). Dán link chỉ phân tích link (và mở `b23.tv` một lần), không gọi yt-dlp, nên tên và thời lượng của video lẻ hiện sau khi thêm.
- Chiều cao tải lấy theo nền tảng: Twitch 480, Bilibili 1080 (yt-factory cứng 720). Bilibili: lời gọi API cách nhau 10–20 giây, lượt tải yt-dlp 5–10 giây; hạn mức 20 lời gọi mỗi 10 phút và 200 mỗi ngày. Bị chặn thì chờ 15 → 30 → 60 → 120 phút như dải tạm chặn ở Tài khoản nền tảng.
- Hỏi design: duyệt.
- Trả lời (design, 01/11/2026): Duyệt (đọc danh mục trong việc nền, “Đang đọc” rồi số video; chiều cao tải theo nền tảng; nhịp Bilibili). <!-- r31-v27 -->

### #45 · Prompt ảnh nhập trùng tên — Đã chốt
- Artboard: `Library` · `tab=prompts` (Nhập).
- Design không nói khi nhập một prompt trùng tên. Code tự đổi tên thành “<tên> (nhập)” để nhập cùng tệp hai lần vẫn được, không báo lỗi.
- Hỏi design: duyệt, hoặc chọn hỏi ghi đè.
- Trả lời (design, 01/11/2026): Duyệt tự đổi tên “<tên> (nhập)”, thêm toast “Đã nhập N prompt · 1 trùng tên, đã đặt tên mới”. Không hỏi ghi đè. <!-- r31-v27 -->

### #46 · Thư viện: không có nhập template, thẻ không có ảnh thu nhỏ — Đã chốt
- Artboard: `Library` · `tab=templates` (ô thả tệp template), thẻ template/prompt/lớp phủ có ảnh.
- Chưa có định dạng tệp template (yt-factory nhập cả project editor), nên chưa có ô nhập template; nhập/xuất prompt ảnh thì có. Thẻ chưa có ảnh vì chưa có bản xem thử hay ảnh mẫu (#38): thẻ dùng khối màu trung tính và biểu tượng theo loại, không bịa ảnh. Lớp phủ hiện khung đầu của video.
- Hỏi design: duyệt tới V13.
- Trả lời (design, 01/11/2026): Duyệt tới V13: không ô nhập template, thẻ dùng khối màu trung tính + icon theo loại; lớp phủ dùng khung đầu. <!-- r31-v27 -->
- Chủ sản phẩm (01/11/2026): cần nhập template cũ từ yt-factory. Quyết định V13 ở trên hết hiệu lực cho phần nhập template (thẻ vẫn không có ảnh thu nhỏ).
- Đã làm: nút **Nhập template** trên thanh đầu `tab=templates` (đúng artboard) mở hộp nhập có vùng thả tệp. Nhận `.ytf-template.zip` (một tệp, nhiều tệp, hoặc thư mục `editor-workspace` cũ qua CLI `viderse template import`), kiểm sha256 từng media, nhập media vào kho media, đổi mã media trong project, suy recipe bằng hàm của màn Cài đặt template, đặt tên trùng theo luật #45 (`(nhập)`, `(nhập 2)`). Kết quả trả từng dòng `{name, ok, warnings[]}`; cảnh báo ghi rõ phần không quy đổi được (kênh nền/video ghim chưa có trong thư viện, lớp phủ nằm trong project dưới dạng track). Đường API: `POST /api/templates/import`.

### #47 · Kênh, Tạo kênh trên điện thoại — Đã chốt
- Artboard: `MChannel`, `MCreate` thuộc V10.
- Dưới 1100px, cột trái của Kênh (danh sách kênh, thư viện) xuống dưới nội dung. Dưới 768px (khung điện thoại) cột trái ẩn như `MChannel`: đổi kênh ở viên kênh trên thanh đầu (menu có “Mọi kênh”, “Tạo kênh”), tab dùng tên ngắn (Video · Nguồn · Template · Hồ sơ) và hàng tab tự cuộn ngang. Lần đầu mở là bản máy tính một cột. Trang không cuộn ngang, nút ≥ 44px khi `pointer: coarse`.
- Hỏi design: duyệt bản tạm tới V10.
- Trả lời (design, 01/11/2026): Duyệt bản tạm tới V10. <!-- r31-v27 -->

### #48 · Đổi tên template không làm cũ bản xem thử — Đã chốt
- yt-factory coi mọi lần lưu là làm cũ bản xem thử. Tên không ảnh hưởng bản dựng nên code giữ nguyên `revision` khi chỉ đổi tên.
- Hỏi design: duyệt.
- Trả lời (design, 01/11/2026): Duyệt: chỉ đổi tên không làm cũ bản xem thử. <!-- r31-v27 -->

### #49 · Gói V25: những điểm design cần dev chốt — Đã chốt <!-- r29-s3 --> <!-- r29b -->
- Artboard: `Settings` · `section=storage`, `store=s3envlocked|s3nokey|s3moving`.
- Hỏi dev:
  1. **Tên biến môi trường.** Artboard ghi tạm `VIDERSE_STORAGE_DRIVER`, `VIDERSE_S3_ENDPOINT|REGION|BUCKET|FORCE_PATH_STYLE|ALLOW_INSECURE_ENDPOINT|PUBLIC_BASE_URL|ACCESS_KEY_ID|SECRET_ACCESS_KEY`, `VIDERSE_BLOB_DELIVERY=proxy|redirect` (theo Noverse, thêm tiền tố như `VIDERSE_DATABASE_URL`). Chốt tên để sửa khối “Chi tiết cho người quản trị máy chủ”.
  2. **Khoá mã hoá trên máy chủ.** `secretbox` tự tạo `secret.key` khi ghi được thư mục cấu hình. Khi nào thì `encryptionKey = false` (thư mục chỉ đọc, nhiều bản chạy chung, khoá đặt bằng biến)? Nội dung khối chi tiết của `s3nokey` sửa theo câu trả lời.
  3. **Danh sách nhóm dung lượng.** Artboard dùng 5 nhóm: ảnh đã vẽ, project editor, thư viện, video nguồn, bản xuất MP4. Video nền và lớp phủ nằm trong “Thư viện”? Nhóm nào chưa đếm được thì trả `null` để FE ghi GAP.
  4. **Kiểm tra CORS chạy ở trình duyệt** (`probeUrl` + `probe`, §23.1). Đồng ý cách này hay có cách khác?
  5. **Huỷ chuyển:** tệp đã chép nằm lại kho mới (như Noverse AN), quản trị viên tự xoá. Có cần nút xoá phần đã chép không?
- Trả lời (dev, 31/10/2026):
  1. Không thêm tiền tố, dùng đúng tên của Noverse: hai app đọc chung một bộ đọc cấu hình lưu trữ, mỗi app một tệp cấu hình riêng. Danh sách: `STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_FORCE_PATH_STYLE`, `S3_ALLOW_INSECURE_ENDPOINT`, `S3_PROVIDER`, `BLOB_DELIVERY=proxy|redirect`, `BLOB_CACHE_DIR`, `BLOB_CACHE_MAX_BYTES`. Không có `S3_PUBLIC_BASE_URL`; ô “Địa chỉ kho cho trình duyệt” ứng với `S3_PRESIGN_ENDPOINT`.
  2. Viderse tự tạo khoá máy ở `<thư mục cấu hình>/secret.key` (0600). `encryptionKey = false` chỉ khi không đọc hoặc không tạo được tệp: thư mục cấu hình chỉ đọc, tệp sai quyền, tệp hỏng. Không có biến môi trường đặt khoá; chỉ hỗ trợ một bản chạy.
  3. Thư viện gồm lớp phủ, video nền và ảnh kênh (ảnh đại diện, ảnh bìa), đếm được ngay. Ảnh đã vẽ, project editor, video nguồn, bản xuất MP4 trả `null` tới khi có luồng dựng video.
  4. Đồng ý kiểm tra CORS ở trình duyệt qua `probeUrl`; chỉ trình duyệt mới chứng minh được CORS.
  5. Không cần nút xoá phần đã chép: tệp đặt tên theo nội dung, lần chuyển sau dùng lại, không chép lại. Muốn dọn thì dùng lệnh bảo trì, như Noverse AN.
- Design đã sửa (đợt 29b): khối “Chi tiết cho người quản trị máy chủ” ghi đúng tên biến; khối chi tiết của `s3nokey` ghi đường dẫn tệp khoá và cách cấp quyền ghi; Dung lượng theo nhóm có nhãn phụ “lớp phủ, video nền, ảnh kênh” và tweak `usage=partial` (nhóm `null` ghi “chưa có số”); câu huỷ chuyển nói tệp đã chép được dùng lại; Chỗ cũ dọn bằng lệnh bảo trì (bỏ gợi ý xoá tay). Data-contract §23.1 và Gói V25 sửa theo.

### #50 · Gói V26: lý do phải đăng nhập lại và địa chỉ app kia — Đã trả lời (dev, 31/10) <!-- r30-sso -->
- Artboard: `Auth` · `screen=login`, `kicked=self|other|password|admin|expired|unknown`; `screen=menu|denied`.
- Hỏi dev:
  1. Máy chủ có biết vì sao phiên bị thu hồi không (đăng xuất, đổi mật khẩu, quản trị viên đổi quyền/đặt lại/khoá, hết hạn), và ở app nào? Nếu có thì trả `reason`, `app` trong 401 (§25). Nếu không thì giao diện chỉ dùng câu chung, trừ hai trường hợp client tự biết (`?bye=1`, `?pw=1`).
  2. `apps: AppLink[]` (id, tên, địa chỉ gốc) trả trong `GET /api/me` và 403 `app_access_denied` được không? Địa chỉ lấy từ `VERSE_APP_ORIGINS` hay cấu hình riêng?
  3. Noverse có nhận `#/settings/members?user=<tên>` để mở thẳng người cần cấp quyền không?
- Dev trả lời (đã làm):
  1. Có. Phiên chung ghi `revoked_reason` và `revoked_app`. 401 trả `{ code: "session_ended", reason, app? }`:
     - `reason` là `logout | password_changed | admin_changed | locked | expired | unknown`;
     - `logout` đi kèm `app`, nên giao diện tự so với app đang mở để chọn câu `self` hoặc `other`;
     - cookie lạ hoặc hỏng thì trả `unknown` và không có `app`.
     Thêm hai hành vi:
     - Đăng xuất chung giữ lại cookie đã thu hồi tới lần đăng nhập sau (không gia hạn), để app kia đọc được lý do. Cookie đó không còn xác thực được gì.
     - Đổi mật khẩu thu hồi mọi phiên, kể cả máy đang dùng. Đăng xuất "thiết bị khác" thì giữ phiên máy này.
  2. Có. `AppLink` là `{ id, name, origin }`, không phải `{ label, url }` như §25 đang ghi, nhờ design sửa §25 theo.
     - `apps` có trong `GET /api/me` và 403 `app_access_denied`, chỉ gồm app được cấp quyền (quản trị viên thấy mọi app đã cấu hình).
     - Địa chỉ lấy từ biến riêng `VERSE_APP_LINKS` (`noverse=…,viderse=…`), kiểm cùng luật với `VERSE_APP_ORIGINS`. Không đặt biến thì chỉ có app hiện tại, nên phần chuyển app tự ẩn.
  3. Có. Noverse nhận `#/settings/members?user=<tên>`. Tên không có thì về danh sách.
- Câu hỏi lại cho design:
  - Logo menu: artboard dùng `#18181B`, token gần nhất là `--color-surface-3` = `#18181A`. Code dùng token; nhờ sửa artboard về token.
  - Thẻ app ở Thành viên khi chưa cấu hình `VERSE_APP_LINKS`: code ẩn địa chỉ app kia. Cần chốt cách hiển thị dự phòng.
  - `MAuth` vẽ thanh trạng thái và thanh home của hệ điều hành. Bản web không vẽ hai thanh này, nên toạ độ dọc lệch đúng bằng chiều cao của chúng.
- Trả lời (design, 01/11/2026): ô logo dùng `--vv-surface-3`, board `Auth`/`MAuth` đã sửa. Thiếu `VERSE_APP_LINKS` → ẩn nhóm Ứng dụng và *Mở Noverse*; thẻ quyền app ở Noverse › Thành viên giữ tên + ô quyền, ẩn địa chỉ và biểu tượng mở. Thanh hệ điều hành: theo #57. §25 đã sửa `AppLink { id, name, origin }`, `reason` đủ 6 giá trị. <!-- r31-v27 -->

### #51 · Gói V26: đối chiếu Auth/MAuth và phần artboard còn thiếu — Đã chốt
- Nguồn chuẩn: `artboards/Auth.dc.html`, `artboards/MAuth.dc.html`, chỉ `app=viderse`; không lấy bản Noverse cũ làm mẫu.
- Sai khác chủ đích đã nêu ở #50:
  - Web không vẽ thanh trạng thái 44px và thanh home 26px của hệ điều hành. Khi so cùng viewport, phần neo trên của MAuth có `y(web) = y(board) - 44px`; phần căn giữa có `y(web) = y(board) - 9px`; phần neo đáy có `y(web) = y(board) + 26px`. Không cộng một độ lệch duy nhất cho cả trang.
  - Ô logo menu dùng `--color-surface-3` thay literal của artboard. Nền toàn menu vẫn dùng `--color-surface-4`.
  - Không tự đoán địa chỉ app kia khi thiếu AppLink. Đây là sai khác cấu hình đã ghi ở #50, không phải quyền mới.
- GAP cần bổ sung, không tự vẽ reference:
  - `MAuth` chỉ có `sheet=none|password`, không có tấm thiết bị; app cần danh sách và thao tác thu hồi phiên thực.
  - Dữ liệu mẫu về tên người, ngày tạo, tên thiết bị, hạn phiên, số phiên và domain trong board không phải fixture của backend thực. Không đổi dữ liệu thực thành dữ liệu giả chỉ để khớp ảnh.
  - Lần vào chưa có cookie cũng được backend trả `session_ended/unknown`; chưa có response riêng để phân biệt với cookie lạ/hỏng. Vì vậy `kicked=none` chưa có cặp đo trực tiếp với backend hiện tại.
  - Cổng mật khẩu tạm trên điện thoại cần đường đăng xuất; board không có nút đó. Các lỗi mạng, sao chép thất bại, mã khôi phục không được trả và các thao tác đang chờ cũng chưa có state tương ứng đầy đủ.
- Màu chữ badge nhấn của board chưa có token trùng chính xác: hiện dùng `--vv-tabbar-on`, không thêm literal màu. **Badge ink #FFD3DA chờ design cấp token.** Icon băng thông tin dùng `--kit-info-fg` (sau `defineBrand`: #9AB0D6), trong khi board là #9DB0F7; cũng chờ token được duyệt. Đây **chưa phải xác nhận 1:1**; không thêm token hay màu thô để lách giới hạn màu.
- Đã chốt giữ vùng chạm 48px của nút đổi mật khẩu trên web (31/10). Ở `MAuth screen=password, pwerr=mismatch|wrong`, status/home bar giả chiếm 70px làm flex co nút khai báo 48px xuống **40,844px**. Web không vẽ OS chrome và giữ nút **48px**; residual cao **7,156px**, dòng gợi ý sau nút lệch tương ứng. Không cố co vùng chạm của web để bắt chước lỗi co chiều cao này của board.
- Bằng chứng đo và sai khác còn lại nằm trong báo cáo QA được yêu cầu riêng; mục này không miễn tiêu chí 0,5px hay tự phê duyệt các sai khác chưa chốt.
- Trả lời (design, 01/11/2026): <!-- r31-v27 -->
  - `MAuth` · `sheet=devices`: thiết bị này (nhãn, không nút), *Đăng xuất* từng máy, *Đăng xuất mọi thiết bị khác* có xác nhận tại chỗ, câu “phải đăng nhập lại ở cả Noverse và Viderse”. Dữ liệu thiết bị dùng chung với `Auth` (3 máy).
  - `screen=password` có *Đăng xuất* ở chân màn. `fail=network|copy|norecovery`, `busy=yes` cho cả `MAuth` và `Auth`; câu ở 06-copy › Đăng nhập › trạng thái bổ sung.
  - `kicked=none`: không khối lý do. Máy chủ giữ `unknown`; client phân biệt bằng cờ `vv.signedInBefore` (localStorage) — chưa từng đăng nhập trên trình duyệt này thì không hiện “Phiên đăng nhập đã kết thúc”.
  - Token: chữ huy hiệu `--vv-accent-soft` (#FFD3DA, đã có trong tokens.css), nền `--vv-accent-bg`; icon băng thông tin `--vv-info` (#9DB0F7) — map vào `defineBrand` thay `--kit-info-fg`. Không thêm màu mới.
  - Giữ 48px: board đã cho con của cột cuộn `flex-shrink: 0`; nút không còn co.
  - Tên người, ngày, thiết bị trên board là dữ liệu mẫu; web dùng dữ liệu thật, không đổi cho khớp ảnh.

### #52 · Hàng chờ + Kế hoạch (Gói V6/V2): mã lý do chưa chạy, giá, khoá xếp lại — Đã chốt
- Artboard: `Queue` · `state=running|paused|limited|empty`, `NewVideo`.
- Chốt dev:
  1. `GET /api/queue` trả thêm `pausedAt` (giờ bấm Tạm dừng, cho câu “Đã tạm dừng lúc 00:04”) và mỗi dòng chờ có `waitReason` ngoài `pace_full|no_template` còn `queue_paused|workers_busy|rotation|paused_channel`.
  2. `paused_channel` được hiểu là **Tự lấy tắt** (`owner_channels.auto_pick = false`): chỉ khoá các mục `batch` có `origin = auto` của kênh đó; mục `pick`/`bulk` người dùng chủ động xếp vẫn chạy. Không có cột “tạm dừng kênh” nào khác.
  3. `POST /api/clone/plan` trả thêm `imageCalls` (số ảnh tính theo recipe thật, cộng 10% vẽ lại theo `yt-factory 2c201e6:src/server/clone/queue.ts:72,898`) và `priceUSD` — `null` vì chưa có bảng giá theo ảnh của model nào. `accepted[]` thêm `imageEstimate`, `imageCalls`, `priceUSD`, `viewCount`, `publishedAt`; `duplicates[]` thêm `title` (tên job/video trùng) và nhận cả bản `state: "queued"` với `jobId: null`.
     `accepted[]` còn trả `sourceChannelId` và `rivalLinked` theo liên kết thật của kênh đích. `duplicates[]` có `thumbnailUrl`, `durationSec`, `viewCount`, `hasCaptions`, `sourceChannelTitle`, `publishedAt`, `outlier`: ưu tiên source snapshot của job/hàng chờ, chỉ bổ sung trường lịch sử còn thiếu từ video đối thủ; không gọi lại nguồn ngoài cho preview đã lưu.
  4. `PATCH /api/queue` theo hợp đồng v2: `{paused}`, `{workers, imageWorkers, rescanHours}`, `{id, position}` (vị trí số thực), `{id, action: "front"|"remove"}`. `front` chuyển mục sang `pick` và đặt trước mọi mục `pick` hiện có; nhịp chỉ tính khi mục bắt đầu với ưu tiên `batch`. `items[]` trả `position` thật để kéo thả tính midpoint, `sourceUrl` và `thumbnailUrl` khi có dữ liệu.
  5. `imageQuota[]` lấy số ảnh đã tính hôm nay từ `image_receipts`, giới hạn và giờ reset từ nguồn vẽ thật (không có số thì `null`). `doneToday[]` lấy job `stage_group` `ready|posted|review` cập nhật sau 00:00 giờ máy (`at` = `updated_at` vì chưa có cột giờ xong riêng, `posted` hiện như `ready`).
  6. SSE: một sự kiện `queue.updated` cho mọi thay đổi hàng chờ (xếp, xếp lại, bỏ, tạm dừng, drain bắt đầu/xong) + `clone.job.*` hiện hữu để invalidate; không bắn mỗi lần ETA đổi.
- Hỏi design: duyệt.
- Trả lời (design, 01/11/2026): Duyệt cả 6 điểm. Câu cho từng `waitReason` ở 06-copy › Hàng chờ; `pausedAt` dùng ở viên và băng tạm dừng; `priceUSD: null` → chỉ ghi số ảnh. Data-contract §3, §4 đã ghi theo. <!-- r31-v27 -->

### #53 · Làm video: chọn template riêng và dữ liệu thẻ hàng chờ — Đã chốt
- Artboard: `NewVideo` · `input=pasted|multi`, `Queue` · `state=running|paused|limited`.
- Dropdown ở chip template cho phép chọn khác vòng luân phiên. Bổ sung `templateId?` vào `CloneEnqueueRequest` (lệch §4 `07-data-contract.md`): khi có, kiểm template hợp lệ của kênh và chụp đúng recipe đó; khi vắng, giữ luân phiên hiện hữu. Kế hoạch đọc cùng override.
- Dòng chờ trả `position`, `thumbnailUrl`, `sourceUrl`; thẻ đang chạy trả `pausedAt`. Đây là dữ liệu thực để kéo-thả và mở nguồn, không suy ra số hoặc ảnh ở trình duyệt.
- Dòng “Máy này chạy ổn 2 luồng” thiếu cơ sở đo máy: giữ trống tới khi design chốt; không dùng số tự suy đoán.
- Trả lời (design, 01/11/2026): Duyệt `templateId?`; data-contract §4 đã thêm. Câu đo sức máy bỏ hẳn (#55). <!-- r31-v27 -->

### #54 · Dựng bản nháp trước khi chờ duyệt — Đã chốt
- Artboard: `VideoDetail` · luồng tạo video tới `awaiting_review`; backend Phase 05.
- Dev: Phase 05 dựng project editor schema 15 bằng ảnh đã chọn, hoặc ứng viên hợp lệ mới nhất nếu chủ kênh chưa chọn. Đây chỉ là **bản nháp để duyệt**: không tự ghi thao tác duyệt ảnh, không xuất MP4, không chạy bước kiểm định/xuất.
- Khác yt-factory `build.ts` (chặn trước khi dựng khi ảnh chưa duyệt): điểm chờ được đặt **sau** khi dựng project, đúng yêu cầu Phase 05 “dựng rồi chờ duyệt”. Project và artifact được publish trước khi job đỗ `awaiting_review`, không có `retryAt`; chỉ thao tác duyệt mới mở bước tiếp.
- Trả lời (design, 01/11/2026): Duyệt. <!-- r31-v27 -->

### #55 · Hàng chờ: không suy đoán sức máy — Đã chốt
- Artboard: `Queue` · mọi trạng thái, cột Quy tắc chạy: “Máy này chạy ổn 2 luồng”.
- Backend chưa có phép đo sức máy. Không hiện dòng này; số luồng đã cấu hình vẫn đọc từ `workers.total`, không phải kết quả đo. Không dùng giá trị cấu hình để khẳng định máy chạy ổn.
- Trả lời (design, 01/11/2026): Duyệt. `Queue` đã bỏ câu, thay “Mỗi luồng là một video chạy cùng lúc, từ 1 tới 6. Thêm luồng thì mỗi video chậm hơn và hết hạn mức ảnh sớm hơn.” `MQueue` dùng cùng câu. <!-- r31-v27 -->

### #56 · Viên Hàng chờ khi tạm dừng — Đã chốt
- Artboard: `Main`, `Queue` dùng “Tạm dừng”; tiêu chí nghiệm thu yêu cầu “Đã tạm dừng”.
- Code dùng “Đã tạm dừng”, kể cả khi bước đang làm được chạy nốt. Không hiện chấm đang chạy trong trạng thái này. Chiều rộng viên thay đổi theo câu thật, không ép bằng chiều rộng câu trên artboard.
- Trả lời (design, 01/11/2026): Duyệt. `Main`, `MMain`, `Queue`, `MQueue` đã vẽ “Đã tạm dừng lúc 00:04 · N chờ lượt” với icon tạm dừng. <!-- r31-v27 -->

### #57 · Điện thoại: không vẽ giả thanh hệ điều hành — Đã chốt
- Artboard: `MNew`, `MMain` có thanh trạng thái 44px và vùng home indicator 26px.
- Web không dựng giờ, sóng, pin hoặc home indicator giả. Dùng `env(safe-area-inset-top|bottom)` của thiết bị. Đo phần nội dung theo gốc dưới thanh trạng thái: trên Chromium 390×844 không inset, y của nội dung bằng y artboard trừ 44px; phần dính đáy so theo mép trên vùng home indicator, không thêm 26px giả.
- Quy ước đo tương đối theo safe area giống Noverse; chiều rộng, chiều cao, font, padding và gap của nội dung không được tự đổi.
- Trả lời (design, 01/11/2026): Duyệt; cùng quy ước với #51 (đo theo safe area, không vẽ thanh hệ điều hành). <!-- r31-v27 -->

### #58 · Video mới: tuỳ chọn không cần ảnh — Đã chốt lại (design)
- Gói V2 yêu cầu chip Ảnh và API hỗ trợ `noImages`, nhưng `NewVideo` / `MNew` chưa vẽ điều khiển này.
- Giữ tuỳ chọn “Không cần ảnh” dùng dữ liệu thật. Trên máy tính cùng hàng Kênh/Template; trên điện thoại thêm một hàng 52px trong nhóm tuỳ chọn. Không mặc định bật hoặc sửa recipe gốc của template.
- Khác biệt có chủ ý: nhóm tuỳ chọn điện thoại cao thêm 52px so với artboard; không giấu tính năng chỉ để khớp ảnh.
- Trả lời (design, 01/11/2026): Không giữ phương án tạm: **bỏ “Không cần ảnh” khỏi Video mới cả máy tính và điện thoại**. L14: ngoại lệ ảnh của một video chỉ đặt ở chip Ảnh của Bàn dựng; Gói V16 ghi “Không làm: chip ảnh ở Video mới”. Dòng “Chip Kênh / Template / Ảnh” ở Gói V2 là sót, đã sửa. Video không ảnh: chọn template không ảnh ở chip Template (#53), hoặc chip Ảnh ở Bàn dựng. `noImages` giữ cho CLI. <!-- r31-v27 -->

### #59 · Điện thoại: chỉnh Quy tắc chạy từ hàng chờ — Đã chốt lại (design)
- Artboard `MMain`, sheet Hàng chờ có “Chỉnh” trỏ tới `Queue`; chưa có artboard Queue điện thoại riêng.
- “Chỉnh” mở Dialog của kit, dùng lại chính các control Quy tắc chạy của Queue và cùng PATCH thật. Không thêm tuỳ chọn hoặc lời mới. Đóng Dialog trở về sheet đang xem.
- Trả lời (design, 01/11/2026): Thay Dialog tạm bằng màn `MQueue` (artboard mới) và bottom sheet *Quy tắc chạy* (`MQueue` · `sheet=rules`). Sheet Hàng chờ ở `MMain` dẫn tới `MQueue` qua *Mở cả hàng chờ* và *Quy tắc chạy*. <!-- r31-v27 -->

### #60 · Viên việc nền: thiếu artboard cho máy rảnh, tạm dừng, mất kết nối — Đã chốt
- `Main.dc.html` chỉ vẽ viên “2 đang chạy · 12 chờ lượt”; đổi `stage=paused` chỉ đổi video, không đổi viên. Không có tweak cho `idle|paused|offline`.
- Đối chiếu viên đang chạy trực tiếp với `Main`; ba trạng thái còn lại giữ cùng chiều cao 30px, padding 10px, gap 8px, chữ 12,5px và dùng trạng thái/câu của Gói V0. Không sửa artboard hoặc dựng trạng thái giả để gọi là chứng cứ cặp.
- Hỏi design: bổ sung ba trạng thái trên. Câu “Đã tạm dừng” tiếp tục theo #56.
- Trả lời (design, 01/11/2026): Đã vẽ: `Main` / `MMain` · `jobs=running|idle|paused|offline`. Câu ở 06-copy › Hàng chờ, quy tắc ở components › JobsPill, luật mất kết nối ở data-contract §3. <!-- r31-v27 -->

### #61 · Video mới nhiều link: ưu tiên trên điện thoại — Đã chốt
- `NewVideo` có công tắc “Ưu tiên cả N”, nhưng `MNew` · `input=multi` chỉ có hai hàng Kênh/Template và chưa vẽ công tắc tương ứng.
- Giữ thao tác ưu tiên thật bằng hàng “Ưu tiên lượt này” 52px trong nhóm tuỳ chọn, dùng cùng trạng thái và kế hoạch của máy tính. Nhóm nhiều link cao thêm 104px: 52px cho Không cần ảnh (#58) và 52px cho Ưu tiên. Không coi hai hàng này là sai số bố cục.
- Hỏi design: duyệt hàng công tắc hoặc bổ sung chỗ đặt khác trên `MNew`.
- Trả lời (design, 01/11/2026): Duyệt hàng công tắc 52px “Ưu tiên cả 3” · “Lên đầu hàng chờ, không theo nhịp kênh”, cuối nhóm Kênh/Template (`MNew` · `input=multi`, `prio=on`). Nhóm chỉ cao thêm 52px vì không còn hàng Không cần ảnh (#58). <!-- r31-v27 -->

### #62 · Nút thêm video của thanh dưới: giữ bảng màu đã có — Đã chốt
- `MMain` vẽ cầu đỏ với ba stop `#FF6A80`, `#FF1F45`, `#C80028`; ba stop này chưa có trong token UI đang dùng.
- Theo yêu cầu không thêm màu ở đợt đối chiếu này, giữ màu đỏ nút `--vv-red-button` và bóng hiện có. Kích thước 46px, icon 22px, vị trí giữa năm tab và vùng chạm vẫn khớp artboard.
- Hỏi design: dùng màu nút V0 hiện có hoặc duyệt một token gradient riêng trong đợt bổ sung token; không tăng ratchet.
- Trả lời (design, 01/11/2026): Duyệt token mới `--vv-red-button-fill` (gradient 3 stop của artboard), chỉ cho cầu nút + ở thanh dưới. Đã thêm vào tokens.css, kit.css (`.vv-tabbar__new`) và 03-tokens. <!-- r31-v27 -->

### #63 · Phụ đề nút Làm video ở Khám phá: chữ tối trên nền sáng — Đã chốt
- `Explore` vẽ phụ đề “cho …” bằng trắng trong suốt trên nút primary hiện có nền trắng. Phụ đề không đủ tương phản; đây là lỗi của board, không phải nội dung dữ liệu.
- Main đã duyệt dùng chữ tối theo token: phụ đề lấy `--color-bg` với độ phủ 82%, giữ cỡ 12px, line-height 1,3 và bố cục hai dòng trong nút 46px. Không thêm màu hoặc tăng ratchet.
- Main chuyển lỗi tương phản của board cho design; bản chạy giữ cách hiển thị đã duyệt.
- Trả lời (design, 01/11/2026): Board `Explore` đã sửa phụ đề sang chữ tối `--vv-bg` 82%. <!-- r31-v27 -->

### #64 · Hoàn tác bỏ khỏi hàng chờ bằng chính mục cũ — Đã chốt
- Main duyệt mở rộng duy nhất trong PATCH v2 hiện hữu: `{ id, action: "remove" }` ghi `clone_queue.removed_at`; trả toàn bộ `CloneQueueView` kèm `removed: { id, position, priority }`. Không trì hoãn bỏ ở client.
- Hoàn tác gọi `{ id, action: "restore" }`, xoá dấu bỏ trên cùng dòng, giữ nguyên định danh, vị trí, ưu tiên, nguồn, lựa chọn ảnh và template/recipe. Không dùng URL để enqueue lại; không tạo route mới.
- GET, ETA, bộ đếm, khử trùng và scheduler loại mục đã bỏ. Hai thao tác thành công đều phát `queue.updated`. Restore trả `409` nếu mục đã bị dọn, video đã chạy hoặc nguồn đã có mục/job khác; UI hiển thị lỗi thật, không giả thành công.
- Dùng sweep drain định kỳ hiện có để dọn mục đã bỏ cũ hơn 24 giờ (từ `removed_at`), cả khi hàng chờ tạm dừng/hết luồng; không thêm timer/goroutine. Chi tiết request/response và mã lỗi ở `07-data-contract.md` §3.
- Quyết định Main (01/11/2026): duyệt durable remove/restore theo hợp đồng trên; đây là ngoại lệ có phạm vi rõ ràng của giới hạn không thêm API ở Gói V27.

### #65 · Nút xác nhận đăng xuất khi đang xử lý — Đã chốt
- `MAuth` · `sheet=devices`: lúc xác nhận đang chạy, board chia hai nút rộng 171,234px / 176,766px; ứng dụng chia đều 174px / 174px tại 390px.
- Main duyệt giữ `Button` dùng chung và `aria-busy`; vòng quay của kit nằm ở lớp phủ nên không ảnh hưởng chiều rộng nội dung. Hai nút vẫn cao 48px; ứng dụng khoá thao tác trong lúc xử lý để tránh gửi lặp.

### #66 · Điểm Độ hoàn thiện và vùng phụ đề lệch ở V13 — Đã chốt
- `Editor` V13 dùng điểm 84 và ngưỡng ±0,3 giây làm dữ liệu minh hoạ, không phải điểm cố định hoặc công thức thay thế cho dữ liệu thật.
- App giữ trọng số yt-factory tại commit `2c201e6`, `src/server/clone/score.ts:699–712`: khớp tiếng và karaoke có trọng số 0; khớp tiếng không đạt vẫn là blocker riêng. Vì vậy tổng 100 vẫn có thể chưa được xuất; không sửa điểm thành 84 để giống board.
- Vùng cần sửa chỉ chứa từ có độ lệch tuyệt đối **lớn hơn 120 ms**, theo ngưỡng median của nguồn; từ khớp chính xác hoặc trong ngưỡng không được gắn là lỗi. Quyết định chung vẫn dùng đủ điều kiện nguồn: khớp ít nhất 60%, median ≤120 ms và p95 ≤250 ms. Không dùng ngưỡng minh hoạ 300 ms để bỏ sót lỗi median.
- Giữ nguyên bố cục/copy V13; số điểm, số vùng và offset hiển thị theo kết quả đo thật. Điểm đo cũ sau sửa phụ đề vẫn đóng cổng xuất cho tới khi đo lại hoặc người dùng chọn “Vẫn xuất” theo chính sách đã chốt.
- Lúc xuất, bỏ lời nhắc giữ tab và khẳng định đang dựng ngay trong trình duyệt: Chrome được server quản lý tiếp tục chạy khi tab editor đóng. Chỉ giữ câu “Bạn vẫn xem được timeline nhưng không sửa được.” Không bịa ETA hoặc thời lượng trên thẻ; nhờ design chốt câu thay thế phù hợp với luồng xuất nền thật.

### #67 · Cơ sở điểm nổi trội trong Vì sao? — Đã chốt
- Board `Explore` minh hoạ lượt xem mỗi giờ so với trung vị 30 video; điểm hiện tại được tính bằng tổng lượt xem so với trung vị 150 video gần nhất.
- Main duyệt giữ bố cục thẻ và hiển thị đúng phép tính, nguồn số thật; không dựng số lượt xem mỗi giờ hay trung vị 30 video. Không hiện liên kết Vì sao? khi thiếu dữ kiện.

### #68 · Trạng thái hệ thống của màn Duyệt — Đã chốt
- Board `Review` không vẽ trạng thái đang tải, lỗi hoặc mất kết nối. Main duyệt dùng mẫu trạng thái hệ thống V12 trong vùng nội dung Duyệt, giữ nguyên khung ứng dụng và dải mất kết nối dùng chung; không thay bằng trạng thái duyệt hết khi chưa tải được dữ liệu.

### #69 · Trạng thái hệ thống của bảng bên phải — Đã chốt
- Board `Main` không vẽ trạng thái đang tải, rỗng, lỗi hoặc mất kết nối riêng cho Nguồn và Nền.
- Main duyệt dùng mẫu trạng thái V12 trong nội dung bảng, giữ khung và dải mất kết nối chung; không coi lỗi tải Nền là danh sách clip rỗng.

### #70 · Trạng thái hệ thống của Việc nền và thông báo — Đã chốt
- Board `Activity` không vẽ trạng thái đang tải, lỗi hoặc mất kết nối cho popover và cài đặt. Main duyệt dùng mẫu hệ thống V12 trong nội dung tương ứng, giữ khung ứng dụng và dải mất kết nối dùng chung; không coi lỗi tải là danh sách rỗng.

### #71 · Trạng thái hệ thống của Bàn dựng — Đã chốt
- Board `Main` không vẽ trạng thái đang tải, rỗng, lỗi hoặc mất kết nối cho dữ liệu Bàn dựng.
- Main duyệt dùng mẫu hệ thống V12 trong vùng nội dung, giữ khung ứng dụng và dải mất kết nối chung; không coi lỗi tải là danh sách video rỗng.

### #72 · Trạng thái hệ thống của Bản tin — Đã chốt
- Board `Main` không vẽ trạng thái đang tải, rỗng, lỗi hoặc mất kết nối riêng cho Bản tin.
- Main duyệt dùng mẫu V12 trong vùng nội dung, giữ khung ứng dụng và dải mất kết nối dùng chung; không coi lỗi tải là Bản tin rỗng.

### #73 · Bật lại ảnh khi template không vẽ ảnh — Đã chốt
- Hợp đồng cho phép tắt «Không cần ảnh» riêng cho video, nhưng chưa nêu số ảnh cảnh khi template chọn «Không vẽ ảnh».
- App dùng tối thiểu một ảnh cảnh khi bật lại trong trường hợp này; không sửa template. Các template có số ảnh hoặc nhịp ảnh vẫn giữ cấu hình của template.
- Hỏi design: xác nhận mặc định một ảnh cảnh hay chọn số khác cho trường hợp template không có nhịp ảnh?
- Trả lời (design, 01/11/2026): Bật lại ảnh khi template “Không vẽ ảnh”: mặc định **6 ảnh cảnh** chia đều theo độ dài (như Mẫu chuẩn) + bìa theo template; thẻ Kế hoạch ghi số ảnh tốn trước khi áp dụng. Một ảnh cảnh cho cả video dài là quá ít. <!-- r32 -->

### #74 · Ô Nhắn trợ lý: lỗi, nút Làm khi ra video, phím tắt — Đã chốt
- Board `Main` `agent=plan` và `Dialogs` `assistant-confirm|unknown` không vẽ trạng thái đang đọc câu, lỗi (chưa có model, mất kết nối, kế hoạch hết hạn) và tiến độ sau "Làm". App dùng thẻ cùng khung với `assistant-unknown`, nói đúng lỗi thật và có Thử lại / Sửa câu; sau "Làm" hiện từng bước Đang chờ / Đang làm / Xong.
- Nút **Làm N bước**: 13-changelog đã đổi nút chính từ đỏ sang nút sáng, nên app giữ nút sáng đúng board (`.vv-btn--primary` và `--white` giống hệt) và chỉ đánh dấu `data-enqueues-video` khi có bước đưa video vào làm. Hỏi design: 01-ia-flows §9.7 và 08-dev-prompt vẫn ghi "đỏ khi có bước đưa video vào làm" — giữ sáng hay đổi?
- Hàng phím tắt dưới ô: `Enter gửi`, `Esc tạm dừng` (khi video đang chạy), `[ ] bảng bên` (`[` đóng/mở danh sách video, `]` đóng/mở bảng bên phải; nút ở thanh trên là của khung ứng dụng) và số ảnh trên hạn mức lấy từ `quota` của `/api/ai/providers` (ChatGPT theo ngày, không có thì Grok theo tuần). `/ lệnh` chưa có đặc tả danh sách lệnh trong 01-ia-flows / 04-components / 06-copy nên chưa vẽ cùng ví dụ `/lệnh` trong placeholder. Hỏi design: danh sách lệnh `/` là gì?
- Chip "Bàn với Trinh sát" ở ô Kênh là nhãn người nhận, không phải nút (chưa có hành vi nào được mô tả).
- Trả lời (design, 01/11/2026): Duyệt các state lỗi/tiến độ (câu ở 06-copy › Chốt đợt 32). Nút **Làm N bước giữ sáng** như board; ia-flows §9.7 và 08 V11 đã sửa. Danh sách lệnh `/` ở 04-components › Composer: `/redraw`, `/provider`, `/pause`, `/resume`, `/export-anyway`, `/posted`. Chip “Bàn với Trinh sát” là nhãn người nhận — đúng. <!-- r32 -->

### #75 · Đề xuất, Quy tắc đã học, Ghi chú đã học: phần board không vẽ — Đã chốt
- **Ghi chú đã học (Thư viện › Prompt ảnh)**: `08-dev-prompt` V19 và ia-flows có nêu nhưng `Library` không vẽ khối này. App dùng lại đúng hàng `.vv-rule` của Quy tắc đã học (công tắc, chữ, nguồn, «Xem ví dụ», Xoá) dưới nội dung prompt, tiêu đề «Ghi chú đã học», mô tả «Hoạ sĩ học từ góp ý vẽ lại; áp dụng cho mọi template dùng prompt này.», trống «Chưa có ghi chú nào. Khi bạn góp ý vẽ lại giống nhau 2 lần với prompt này, Hoạ sĩ sẽ hỏi có nên ghi lại.»
- **Activity › Trợ lý chủ động › «Đề xuất của trợ lý»**: board chỉ vẽ hàng đóng. «Chọn loại» mở tại chỗ thành 7 hàng công tắc (Đưa video nổi lên đầu hàng · Template hay chưa đạt điểm · Lưu quy tắc từ việc bạn sửa · Bật Duyệt theo điểm · Đổi nhịp đăng · Áp dụng điều rút ra từ số liệu · Dọn MP4 đã đăng), kèm danh sách loại đã tắt riêng cho kênh (từ «Đừng đề xuất kiểu này») có nút «Bật lại»; nút đổi «Chọn loại» ↔ «Xong».
- **«Xem ví dụ» / «Xem 3 lần sửa»**: board không vẽ nội dung. App mở danh sách «Trước / Sau» ngay dưới hàng hoặc banner (không mở hộp thoại); nút đổi «Ẩn ví dụ».
- **Xác nhận khi nhận đề xuất tốn ảnh**: không có hộp thoại vẽ sẵn, 06-copy chỉ có mẫu «(tốn 7 ảnh)». App dùng hộp xác nhận chung, tiêu đề = nhãn nút chính, nội dung «Tốn N ảnh. Máy sẽ xếp việc này vào hàng chờ ngay sau khi bạn bấm.», nút chính đỏ (đúng quy tắc xếp hàng → đỏ như thẻ kế hoạch), «Huỷ». Dọn MP4: «Xoá MP4 của video đã đăng?» / «Việc này không hoàn tác được.»
- **Toast**: chỉ khi người dùng vừa bấm nhận (câu tóm tắt từ máy chủ, «Hoàn tác» 5 giây nếu việc đảo ngược được); đề xuất mới xuất hiện không bao giờ toast. Trạng thái đang tải / lỗi / mất kết nối dùng chữ «Đang tải đề xuất…», «Không tải được đề xuất.», «Mất kết nối · đề xuất cập nhật lại khi có kết nối», «Không tải được quy tắc đã học.».
- Hỏi design: xác nhận các điểm trên hay vẽ lại.
- Trả lời (design, 01/11/2026): Duyệt tất cả như app đang làm (Ghi chú đã học dùng hàng `.vv-rule`, Chọn loại 7 công tắc, Xem ví dụ mở tại chỗ, hộp xác nhận tốn ảnh nút đỏ, toast chỉ khi bấm nhận, câu tải/lỗi). Không cần vẽ lại. <!-- r32 -->

### #76 · Hiệu quả sau đăng: ô link YouTube, khoá Data API, Kênh › Video: phần board không vẽ — Đã chốt
- **Đánh dấu đã đăng (Bàn dựng ⋯, LiveStudio, hàng Kênh › Video)**: board không vẽ ô nhập link. App dùng hộp xác nhận chung «Đánh dấu đã đăng?» có một ô «Link YouTube (không bắt buộc)»; để trống thì app tự khớp theo link kênh; link không phải youtube.com/watch|shorts|live hoặc youtu.be bị chặn ngay trong hộp. Sau khi đánh dấu vẫn hỏi xoá MP4 như cũ. Đánh dấu hàng loạt (BulkBar) không hỏi link.
- **Kết nối AI › «YouTube Data API»**: `Settings` không vẽ hàng này. App dựng thẻ cùng nhịp thẻ nhà cung cấp (avatar YT, tên, pill «Khoá API» + «Không bắt buộc», dòng phụ trạng thái, nút Thêm khoá/Đổi khoá/Gỡ khoá, ô dán khoá có Lưu khoá/Huỷ); Google từ chối khoá thì báo ngay dưới ô. Không có khoá thì số liệu đọc từ trang công khai (chỉ lượt xem).
- **Trợ lý nhận ra**: thanh dài nhất vẽ 90% (board 90%/80% ở hai nhận xét — app dùng một quy tắc). Nhãn «Sơ bộ · N video có số liệu 7 ngày» theo `counted`; nút theo việc: «Thêm vào quy tắc viết» / «Cho chạy nhiều hơn». Nhận xét đã áp dụng biến mất khỏi danh sách (máy chủ bỏ nó), thông báo bằng toast. Chưa đủ video: «Cần thêm N video có số liệu 7 ngày…». Dòng bằng chứng của thẻ mời xem ghép từ nhóm của nhận xét («Mẫu chuẩn ×1,4 so với Cận cảnh»); tiền tố «Độ dài/Giờ đăng» của hai chiều còn lại là do app đặt.
- **Trạng thái đang tải / lỗi / mất kết nối** của khối và hàng khoá theo mẫu #68–#72.
- **Kênh › Video** (bảng, bộ lọc, Chọn nhiều): board chỉ vẽ nút của hàng; menu ⋯ thêm Làm lại từ đầu / Dừng hẳn / Xoá video dùng chung với Bàn dựng; dòng phụ thẻ số liệu (từ ngày tạo kênh, gần nhất) suy từ dữ liệu thật.
- Hỏi design: xác nhận các điểm trên hay vẽ lại.
- Trả lời (design, 01/11/2026): Duyệt tất cả như app đang làm (ô Link YouTube không bắt buộc trong hộp Đánh dấu đã đăng, thẻ YouTube Data API cùng nhịp thẻ nhà cung cấp, thanh dài nhất 90%, Kênh › Video). Link sai: “Đây không phải link video YouTube.” <!-- r32 -->

### #77 · Gói V29: prompt đã gửi, ảnh tham chiếu và ảnh xem dần — Đã trả lời (dev, 01/11) <!-- r33-img -->
- Artboard: `Review` · `prompt=view|raw`, `state=redrawing|reveal`; data-contract §26.
- Hỏi dev:
  1. Pipeline ảnh hiện ghép prompt ở đâu? Lưu được **nguyên văn đã gửi** theo từng phần (Cảnh · Phong cách · Tham chiếu · Ghi chú đã học · Tránh) cho mỗi lần vẽ không, hay chỉ ghép tại chỗ rồi bỏ?
  2. Ảnh tham chiếu đang gửi kèm gồm những gì (khung đối thủ, ảnh nhân vật…)? Có lưu được nguồn (video, giây, ảnh nào) để hiện lại không?
  3. Nguồn vẽ nào trả **ảnh xem dần** (ví dụ `partial_images` của gpt-image)? Nếu không có thì giao diện chỉ dùng ảnh cũ mờ + vệt sáng, không vẽ ảnh xem dần giả.
  4. Huỷ lần vẽ khi nhà cung cấp đã nhận yêu cầu: có bị tính hạn mức không? Câu “Huỷ lần vẽ” cần nói trước nếu có.
  5. Giữ ảnh của mọi phiên bản cũ có ảnh hưởng dọn dữ liệu (GC media) không? Đề xuất giữ tới khi video được xoá hoặc dọn MP4.
- Trả lời (dev, 01/11/2026):
  1. Prompt ghép ở `internal/pipeline/steps/images/prompt.go` (`BuildParts`), lúc chốt mỗi lần vẽ trong handler bước images. Giờ **lưu đủ**: `Version.Parts` (đúng năm phần Cảnh · Phong cách · Tham chiếu · Ghi chú đã học · Tránh, mỗi phần kèm nguồn `artist|artist_feedback|image_prompt|template|user` + id + nhãn), `Version.Prompt` là **nguyên văn đã gửi** và bằng đúng chuỗi nhà cung cấp nhận (`parts` ghép bằng một dòng trống ra đúng `sentText`; có test đối chiếu với yêu cầu mà nguồn vẽ giả nhận được). Provider không bọc lại prompt: Codex nhận prompt thành một đối số, Grok và nguồn HTTP nhận nguyên `prompt` trong body.
  2. Mặc định mỗi ảnh cảnh gửi **hai** ảnh tham chiếu: khung nguồn của chính cảnh đó (vai trò `layout`, nguồn `rival_frame` kèm `videoId` + `atSec`) và ảnh đang dùng của cảnh đầu (vai trò `character`, nguồn `video_image` kèm `imageId`); ảnh bìa mặc định không gửi kèm. Chủ kênh thêm/bớt được (khung video gốc, ảnh trong video, tệp tải lên). Mỗi ảnh lưu `Version.Refs` với đúng nguồn (`rival_frame` videoId+atSec, `video_image` imageId, `upload`) và đường dẫn artifact để hiện lại ảnh thu nhỏ. Trần theo nguồn vẽ: Codex/ChatGPT 4, Grok 1 (một ảnh mỗi lời gọi `images/edits`), nguồn HTTP khác 0 — quá trần trả 422 `too_many_refs` kèm `limit`.
  3. **Không nguồn nào trả ảnh xem dần.** Codex CLI chỉ trả tệp PNG khi xong; Grok và các nguồn HTTP trả một ảnh cuối. Vì vậy `previewUrl` luôn `null` và giao diện dùng ảnh cũ mờ + vệt sáng như phương án đã chốt — không có ảnh xem dần giả.
  4. Huỷ **trước khi gửi** nhà cung cấp: không tốn ảnh nào (`costImages: 0`, phiên bản `cancelled`, bản đang dùng giữ nguyên). Huỷ **sau khi đã gửi**: yêu cầu đã rời máy, nhà cung cấp có thể vẫn tính — lần vẽ đó vẫn bị tính vào hạn mức ngày (`costImages: 1`) và phiên bản ghi `cancelled`. Nên câu xác nhận “Huỷ lần vẽ” nói trước: “Huỷ khi nhà cung cấp đã nhận yêu cầu vẫn có thể bị tính ảnh.”
  5. Không ảnh hưởng dọn dữ liệu: mọi tệp của mọi phiên bản đều được công bố thành artifact của video và nằm trong gốc giữ của GC (`storage_live_refs` gồm toàn bộ `clone_artifacts`), nên tệp của bản cũ **được giữ** chừng nào video còn; xoá video thì giải phóng theo artifact, dọn MP4 (chính sách dọn tệp nguồn) không đụng tới ảnh. Đúng đề xuất “giữ tới khi video được xoá hoặc dọn MP4”.
- Ghi chú migrate: bản ghi cũ không có `parts` nhưng **có** `prompt` — chính là nguyên văn đã gửi, nên `sentText` của bản cũ đọc lại đúng, không mất dữ liệu. Phần prompt của bản cũ chưa từng tách nên trả về một phần Cảnh duy nhất kèm cờ `partsLegacy: true` (giao diện hiện nguyên văn, không mở sửa theo phần); không cần đánh dấu riêng cho `sentText`.

### #78 · Lưu trữ: in đường dẫn dạng ~/… — Đã chốt <!-- r34 -->
- Hiện trạng: artboard `Settings` · `store=here` in `~/Library/Application Support/Viderse/data/projects`. Dev đã thêm `displayPath` vào API (thay thư mục nhà bằng `~`); UI in `displayPath`, đường dài thì cắt ở giữa.
- Hỏi design: (a) đồng ý cách này hay UI tự rút gọn? (b) đường dài cắt giữa, cắt cuối hay xuống 2 dòng? (c) bề rộng tối đa ở 1440 và 390. Ràng buộc: `--vv-font-mono`, một dòng, không thêm màu.
- Trả lời (design, 01/11/2026):
  - (a) **Đồng ý `displayPath` từ API**; UI không tự đoán thư mục nhà. `title` = `displayPath` đầy đủ; *Chép đường dẫn*, *Mở trong Finder* dùng đường dẫn thật.
  - (b) **Cắt giữa theo thư mục**: giữ phần đầu (`~` hoặc `/Volumes/<tên ổ>`) + 2 thư mục cuối, `…` ở giữa; vẫn dài thì 1 thư mục cuối; tên cuối quá dài mới cắt ký tự đầu tên. Không xuống dòng, không cắt cuối (thư mục cuối là phần người dùng cần thấy nhất).
  - (c) Không đặt số cứng: dòng lấy hết chỗ còn lại trong thẻ. Thực tế 1440 và từ 1600 (khối Cài đặt giữ 760px) ≈ 430px (≈ 60 ký tự mono 11,5px); 390 ≈ 290px (≈ 40 ký tự), nút xuống dòng riêng. Đo theo bề rộng thật.
  - Board sửa: thư mục mặc định `~/.viderse/projects` (theo #10), dữ liệu `~/.viderse`; `store=donenew` dùng đường dài để thấy cách cắt; điện thoại thôi xuống dòng `break-all`, một dòng cắt giữa. Luật ghi ở 04-components › Đường dẫn thư mục và 06-copy › Lưu trữ trên máy.

### #79 · Lưu trữ: "Ổ còn trống" lấy số của ổ nào — Đã chốt <!-- r34 -->
- Hiện trạng: artboard dùng một số 212 GB cho mọi trạng thái. App lấy chỗ trống của ổ đang giữ dự án (ví dụ T7: 1,54 TB); khi ổ ngoài bị rút thì lấy ổ trong máy.
- Hỏi design: (a) duyệt lấy số theo ổ đang giữ dự án? (b) ổ ngoài bị rút thì nhãn ghi gì? (c) `store=offline` có cần dòng cảnh báo riêng không?
- Trả lời (design, 01/11/2026):
  - (a) **Duyệt**, và luôn ghi kèm tên ổ: “Macintosh HD còn trống 212 GB” · “Samsung T7 còn trống 1,54 TB”.
  - (b) Ổ rút: **không** lấy số ổ trong máy (sai nghĩa). Đầu thẻ: “Samsung T7 chưa cắm · chưa đọc được chỗ trống”; dòng thư mục: “Ổ ngoài USB · APFS · chưa cắm”. API trả `null` cho chỗ trống khi ổ không cắm.
  - (c) **Không** thêm dòng riêng: băng cam “Không thấy ổ Samsung T7” đã đủ.
  - Board `Settings`, `MSettings` đã sửa theo `store=here|done|donenew|offline`; bảng câu ở 06-copy › Lưu trữ trên máy.

### #80 · Kit Segmented: nhãn dài bị "…" — Đã chốt <!-- r34 -->
- Hiện trạng: kit chia đều bề rộng nên “Mới nhất” và “VN · New York” ở Explore/MExplore bị cắt “…”. Dev tạm cho riêng hai nhóm Sắp xếp/Vùng tự giãn theo nhãn.
- Hỏi design: (a) kit tự giãn theo nhãn, trên 390 cuộn ngang; hay (b) giữ chia đều và chốt nhãn ngắn.
- Trả lời (design, 01/11/2026): **Chọn (a)** cho cả kit, bỏ ngoại lệ tạm.
  - Mục co theo nhãn, không bao giờ cắt “…”; nhóm hẹp hơn nội dung thì cuộn ngang trong nhóm (ẩn thanh cuộn, mục đang chọn tự cuộn vào tầm nhìn).
  - Chia đều chỉ khi nhóm chiếm hết bề rộng và nhãn ngắn ngang nhau: `.vv-seg--fill` (mới) hoặc `--m`; nhãn dài hơn phần chia vẫn giữ đủ chữ (`min-width: max-content`). Viên trượt `--vv-selected-2` đo theo mục thật.
  - Nhãn giữ đủ: Sắp xếp “Nổi trội · Mới nhất · Lượt xem”; Vùng giờ “VN · New York” (board có 2 vùng, không phải 3).
  - Đã sửa: `kit.css` (và khối kit nhúng trong mọi board), `Kit` › Ô nhập & nút › Nhóm chọn có ví dụ hai nhóm này; `Explore`/`MExplore` vốn vẽ co theo nhãn nên không đổi. Ghi ở 04-components › SegmentedTabs, 05-kit, 06-copy › Nhóm chọn: nhãn.

### #81 · Ảnh mẫu trong tấm chỉnh prompt trên điện thoại — Đã chốt
- Màn / artboard: `MLibrary` · chỉnh prompt / lịch sử ảnh mẫu; `MTemplate` · ảnh bìa. Gói V36, ngoại lệ Trình xem ảnh mobile đợt45 chưa có thiết kế.
- Hiện trạng đã kiểm trên app thật ở390×844: nút ảnh mẫu mở `Dialog` bên dưới `Sheet` đang chỉnh (`z-index40` so với80). Người dùng không thấy ảnh, nhưng focus chuyển vào lớp ẩn. Chỉ nâng z-index sẽ tạo hai lớp modal và vi phạm quy tắc không chồng hộp thoại.
- Hỏi design: **(A)** giữ chức năng xem ảnh mẫu hiện hữu nhưng hiển thị ngay trong cùng tấm đang mở, có Quay lại để trở về bản nháp được giữ nguyên; không thêm viewer đợt45 / không mở lớp thứ hai; hay **(B)** trước đợt45 chỉ hiển thị thumbnail tĩnh trên điện thoại, không dựng nút xem lớn chưa hoạt động? Nếu cần bố cục khác, bổ sung artboard trước khi dev triển khai.
- Trả lời (người dùng, 03/11/2026): **Chọn A — xem ngay trong cùng tấm**. Giữ chức năng xem ảnh lớn, có Quay lại và giữ nguyên bản nháp; chỉ một lớp, không triển khai viewer đợt45. Desktop giữ nguyên.
- Bằng chứng: `plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/sample-acceptance.md` mục7/D1, ảnh và hit-test trong `sample-acceptance/`.

### #82 · Ảnh tách nền từ lượt vẽ thật không cùng kích thước ảnh đã chuẩn hoá — Đã chốt
- Nghiệm thu V36 bằng một ảnh QA thật phát hiện: WebP đã chuẩn hoá1920×1080, nhưng pipeline tách nền từ PNG gốc1672×941; bước dựng đòi PNG tách nền cùng1920×1080 nên dừng. PNG fixture tự dựng và bản xem tách nền sinh từ WebP đều đúng1920×1080.
- Bằng chứng: `plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/real-cutout-normalization-blocker.json`. Diagnostic runtime đã ghi codec/kích thước, không sửa ngưỡng kiểm; dòng log tạm đã gỡ khỏi source.
- Chủ sở hữu: `internal/pipeline/steps/images/handler.go` chuẩn hoá WebP rồi gọi tách nền với PNG gốc; `internal/clone/images/cutout.go` đã dùng ảnh WebP chuẩn hoá cho preview. Đây là lỗi backend ngoài danh sách backend V36 ban đầu.
- Hỏi: **(A)** cho phép vá tối thiểu đường chuẩn hoá tách nền và xử lý derivative đã lưu từ chính ảnh hiện có, không thêm lượt AI/không nới validation; hay **(B)** giữ nguyên giới hạn backend, để nghiệm thu trial bìa và V36 bị chặn? Tối đa2 ảnh QA vẫn giữ nguyên; hiện dùng1.
- Trả lời (người dùng, 03/11/2026): **Chọn A — vá tối thiểu lỗi tách nền**. Chuẩn hoá đúng đường ảnh; xử lý derivative đã lưu từ ảnh hiện có, không thêm lượt AI/không nới validation.

### #83 · Gate baseline toàn cục và thứ tự mở PR — Đã chốt
- Build cuối hiện qua cold entry141,5KiB/150KiB; cold phone còn202,5KiB/200KiB (baseline đã merge202,5KiB). V37 sẽ bỏ các module Bản tin/Kế hoạch hiện đang nằm trong cold phone; chưa tuyên bố việc đó tự bảo đảm qua budget.
- Full Go còn gate latency Khám phá lỗi (`all/outlier/deep` p95=1371,48ms/400ms) trong lúc có các suite/build khác; source query này không thuộc thay đổi V36. Chưa đo riêng khi không có tải kiểm tra khác, không nới threshold hoặc lấy lượt tốt nhất để che lỗi.
- Hỏi: **(A)** cho phép giữ hai gate baseline này mở tới nghiệm thu cuối lộ trình, vẫn đủ nghiệm thu chức năng/UI gói trước khi merge và không deploy production khi gate chưa đạt; hay **(B)** giữ merge gate mọi gate phải xanh ngay PR2, cần xử lý trước khi mở V37? Không thay đổi phạm vi tính năng hoặc bỏ báo cáo lỗi.
- Trả lời (người dùng, 03/11/2026): **Chọn A — giữ tới gate deploy cuối**. Chỉ hai gate dung lượng cold phone và độ trễ Khám phá được giữ mở tới nghiệm thu cuối; UI/chức năng từng gói vẫn phải đủ trước merge. Không deploy khi hai gate chưa đạt.

### #84 · Sửa chỉ dẫn khi đang xem một bản ảnh chưa dùng (V36, đã chốt A)
- **Trạng thái:** Người dùng chọn **“Sửa đúng bản đang xem”** ngày04/11/2026. Cho phép mở rộng tối thiểu backend/callers/worker để giữ đúng prompt và ảnh tham chiếu của phiên bản được chọn; không thêm tính năng khác.
- **Bằng chứng thật:** Lượt50 native trên replay QA có bản1 đang dùng và bản2 đang xem. Cột phải hiển thị prompt và2 ảnh tham chiếu của bản2; bấm Sửa/Thêm lại khởi tạo draft từ bản1, nên hai ảnh tham chiếu biến mất. Sau thêm một ảnh mới, draft chỉ còn1 ảnh. Không gửi vẽ, không tốn thêm hạn mức; source và replay được khôi phục. Ảnh: [bản2/cutout](../../../../../plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/runs/50-review-native-focus-restored/desktop-cutout.png), [draft sau thêm](../../../../../plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/runs/50-review-native-focus-restored/desktop-refs-draft-visible.png).
- **Nguyên nhân:** `ReviewScreen` hiển thị `version` đã chọn nhưng `useImageEditing` lấy `usedVersion`; backend `images/redraw` cũng dựng nền từ `UsedParts/UsedRefs`, chưa nhận phiên bản làm nền. Chỉ đổi draft FE sang bản2 chưa đủ: phần không sửa có thể lại lấy từ bản1 khi máy chủ vẽ.
- **A — Giữ sửa đúng bản đang xem (đã chọn):** Mở rộng tối thiểu contract redraw với phiên bản nền và cập nhật caller/worker liên quan; giữ đúng prompt/tham chiếu đã thấy. Không đổi bản đang dùng khi mở/sửa/gửi; việc dùng ảnh mới tiếp tục theo luật worker/chọn bản hiện hành.
- **B — Chỉ sửa bản đang dùng:** Không mở rộng backend; khi xem bản khác, phải bấm “Dùng bản N cho video” trước khi Sửa/Thêm. Cần chốt khác biệt tương tác với artboard và lời hướng dẫn, không tự khoá nút âm thầm.
- **Giới hạn triển khai:** Chốt A không có nghĩa lỗi đã sửa. Thêm `baseVersion` cho một ảnh, giữ nền qua checkpoint/worker restart, không lấy nhầm bản đang dùng; bản không tồn tại/đang vẽ hoặc gán một nền cho nhiều ảnh phải bị từ chối trước khi ghi. Kiểm những phần không sửa và refs rỗng, đổi bản trong lúc xác nhận, desktop/mobile/Editor/Main. Không gọi thêm ảnh vượt2/2; không coi lượt50/55 control/focus là semantic pass.


### #85 · Trợ lý AI: nguồn vẽ là thẻ đầy đủ hay thanh chọn gọn?
- **Ngày phát hiện:** 04/11/2026.
- **Trạng thái:** **Đã chốt A — người dùng chọn “Giữ thẻ đầy đủ” ngày04/11/2026.**
- **Nguồn thật đang lệch ngay trong artboard:** `Settings.dc.html` dòng1143–1157 chứa radio có tên nhà cung cấp, model, gói, thanh hạn mức và giờ đặt lại; `.opt` dòng112 xếp theo cột. Nhưng quy tắc `.vv-seg > button` dòng459 giới hạn cao30px, làm ảnh artboard render thành ba ô thấp và cắt bớt nội dung. Nguồn artboard không đổi giữa capture và đối chiếu; không phải ảnh chuẩn cũ.
- **Code hiện tại:** radio-card nhiều dòng giữ đủ tên/model/gói/hạn mức. Model và số nguồn khác artboard là dữ liệu thật, không phải lý do tự đổi bố cục.
- **Bằng chứng:** [ảnh chuẩn bên trái / code thật bên phải](../../../../../plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/comparison-main-settings-current/Settings.section.agents.png). Không sửa file artboard trong task triển khai.
- **Cần chốt một phương án:** (A) giữ radio-card đầy đủ và Design vá CSS artboard để phản ánh đúng; (B) theo thanh chọn gọn, Design xác định phần tên/gói/hạn mức nào giữ trên ô và phần nào chuyển ra dòng phụ, không triển khai bằng cách cố ý cắt chữ.
- **Quyết định:** giữ radio-card đầy đủ tên nhà cung cấp, model, gói, hạn mức và giờ đặt lại. Không ép cao30px hoặc cắt nội dung để khớp lỗi render của artboard. Design cập nhật chiều cao artboard riêng; task triển khai không sửa file artboard.
- **Ranh giới:** đây là câu hỏi bố cục nguồn vẽ, không đổi model, quota, suy luận, lưu tức thì hay nguồn dự phòng. Ô chọn model thiếu ngữ cảnh nhà cung cấp là lỗi riêng có thể sửa theo markup đã rõ, không chờ câu trả lời này.

### #86 · V38: giới hạn Video mỗi ngày lệch giữa các artboard — Đã chốt A
- **Ngày phát hiện:** 04/11/2026. Chưa đổi code hoặc dữ liệu theo một giới hạn mới.
- **Bằng chứng:** `Channel.dc.html:1420` giới hạn 1–20 (cùng `08-dev-prompt.md` V38); `CreateChannel.dc.html:1271` và `MCreate.dc.html:1081` vẫn cho 1–50. Đây là handler của cùng trường Video mỗi ngày ở màn tạo và màn chỉnh kênh, không phải khác dữ liệu minh hoạ.
- **Hệ quả:** không thể giữ cả hai giới hạn mà vẫn có cùng tuỳ chọn/hành vi. Kế hoạch hiện ghi 1–20; chưa được coi đây là quyền sửa ngầm board tạo kênh hoặc clamp dữ liệu cũ.
- **Quyết định (người dùng, 04/11/2026): A — tối đa20 video/ngày**, đồng nhất1–20 trên Kênh và Tạo kênh PC/mobile. Design cập nhật màn tạo riêng; task triển khai không sửa artboard. Không tự hạ dữ liệu cũ; ngoài khoảng1–20 phải dừng preflight và báo rõ.
- **Dữ liệu nghiệm thu (người dùng, cùng lượt):** cho tải một clip công khai thật chỉ vào QA để kiểm nền phủ kín; không gọi AI, không ghi production, giữ provenance và dọn dữ liệu kiểm thử sau nghiệm thu.

### #87 · Dặn trợ lý về kênh: thiết kế giữ nhưng chưa có hợp đồng lưu/áp dụng — Đã chốt A
- **Ngày phát hiện:** 04/11/2026, khi đối chiếu ảnh tham chiếu V38.
- **Nguồn chuẩn:** `Workflow.dc.html:979` bỏ ô nhắn toàn cục/Bàn dựng nhưng giữ “Dặn trợ lý về kênh ở Kênh”; `01-ia-flows.md:497` yêu cầu chọn Trinh sát/Đạo diễn/Hoạ sĩ/Kiểm định, gửi có toast và áp dụng video tiếp theo. `Channel.dc.html` vẫn vẽ ô này dưới trang.
- **Code và phạm vi:** `ChannelAside.tsx:6` ghi chưa dựng vì thiếu API; `styleNote` hiện chỉ là Giọng văn, không phải chỉ dẫn riêng cho bốn trợ lý. V37 đã bỏ đường kế hoạch/assistant toàn cục; không khôi phục nó để thay thế. Backend V38 chỉ liệt kê default-template/automation/retire legacy, chưa có hợp đồng lưu và tiêu thụ chỉ dẫn theo trợ lý.
- **Không làm giả:** không thêm ô gửi chỉ hiện toast nhưng không được lưu/áp dụng, không tự ghi mọi chỉ dẫn vào Giọng văn.
- **Quyết định (người dùng, 04/11/2026): A — giữ đúng thiết kế.** Cho phép bổ sung backend tối thiểu lưu chỉ dẫn riêng theo kênh/trợ lý và áp dụng từ lượt tiếp theo/video mới. Lưu phải có readback và consumer thật; không gọi model khi chỉ lưu chỉ dẫn, không thay đổi snapshot video đang chạy, không khôi phục chat/intent/kế hoạch nhiều bước đã bỏ. Các phần V38 còn lại tiếp tục độc lập; chỉ nghiệm thu toàn gói khi phần này có đủ bằng chứng.

### #88 · Chỉ dẫn Kiểm định có bao gồm xử lý nhạc/giọng riêng? — Đã chốt A
- **Ngày phát hiện:** 05/11/2026, khi nối consumer thật theo quyết định#87.
- **Quyết định giữ nguyên:** ô Dặn trợ lý theo kênh, đủ bốn người nhận; lưu/readback không gọi model; áp dụng snapshot video mới. Không đưa chat/kế hoạch toàn cục trở lại.
- **Bằng chứng mới:** ví dụ Kiểm định ở `Channel.dc.html:1468` yêu cầu nhạc nền nhỏ hơn giọng. Consumer Kiểm định hiện là `internal/autonomy/scorer.go:20–35,80–118`, chỉ nhận ảnh/khung tham chiếu và chấm bốn tiêu chí ảnh; không nghe/chỉnh âm thanh. `internal/pipeline/steps/build/audio.go:22–35` trích một track AAC từ nguồn; `build/handler.go:245–261,344,537` đưa chính track đó vào editor. Chưa có stem nhạc/giọng riêng để hạ một thành phần mà giữ nguyên thành phần còn lại. Hạ gain toàn track không đáp ứng ví dụ này.
- **Ranh giới phạm vi:** #87 cho backend tối thiểu lưu/tiêu thụ chỉ dẫn, không xác định pipeline tách stem, model âm thanh, quota mới hay kiểm thử chất lượng tách giọng. Không tự thêm các phần đó và không nối chỉ dẫn âm thanh vào scorer ảnh rồi báo “đã áp dụng”.
- **Phương án A:** giữ bố cục/bốn người nhận; Kiểm định áp dụng chỉ dẫn về ảnh theo khả năng thật, chỉ dẫn vượt khả năng phải báo rõ và không tự duyệt như thể đã đáp ứng. Design đổi ví dụ âm thanh sang kiểm ảnh; cần người dùng chấp thuận giới hạn này.
- **Phương án B không chọn:** bổ sung pipeline tách/mix nhạc–giọng, hợp đồng dữ liệu, quota và nghiệm thu nghe thật; không thuộc đợt này.
- **Quyết định (người dùng, 05/11/2026): A — Kiểm ảnh, báo rõ giới hạn.** Giữ đủ bốn người nhận và bố cục; Kiểm định tiêu thụ chỉ dẫn về ảnh. Chỉ dẫn ngoài khả năng phải báo rõ, không được tự duyệt như thể đã đáp ứng. Design đổi ví dụ âm thanh sang kiểm ảnh; không sửa artboard trong task triển khai. Không thêm xử lý/tách stem âm thanh.

### #89 · Điểm vào Dặn trợ lý theo kênh trên điện thoại — Đã chốt A
- **Ngày phát hiện:** 05/11/2026, khi kiểm chức năng tương đương PC/mobile.
- **Bằng chứng:** PC `Channel.dc.html` giữ hộp Dặn trợ lý; `MChannel.dc.html` không có hộp hoặc điểm vào chỉ dẫn theo kênh. Thẻ “Trợ lý AI · chung mọi kênh” ở Hồ sơ dẫn tới cấu hình model chung, không phải chỉ dẫn riêng. `ChannelScreen.tsx` hiện chỉ render composer khi không phải điện thoại. Vì vậy không thể nhận đủ chức năng tương đương nếu giữ nguyên code này.
- **Không thay đổi:** ba tab Kênh, bốn người nhận, lưu/readback không gọi model, snapshot lượt/video mới, giới hạn Kiểm định theo#88.
- **A:** thêm một hàng “Dặn trợ lý về kênh” trong Hồ sơ điện thoại, mở Sheet Nhập nhanh dùng cùng form/bộ chọn người nhận của PC. Không thêm dock dính đáy cạnh bottom nav.
- **B:** đặt cùng form ngay dưới nội dung ba tab trên điện thoại; luôn thấy nhưng trang dài hơn và cần xử lý bàn phím/đáy màn.
- **Quyết định (người dùng, 05/11/2026): A — Hồ sơ → Sheet Nhập nhanh.** Thêm một hàng trong Hồ sơ, mở cùng form/bộ chọn bốn người nhận của PC; không thêm dock dính đáy. Lưu/readback và giới hạn Kiểm định theo#87/#88 giữ nguyên. Không bỏ chức năng trên điện thoại hoặc thay bằng cấu hình AI toàn app.

## #90. Ảnh minh hoạ thẻ mẫu khi chưa có ảnh mẫu đã vẽ

- Trạng thái: **Đã chốt A — Ảnh thật hoặc hình chờ**.
- Đối chiếu: `Library.dc.html` dùng ảnh minh hoạ cố định trong TH, gắn theo chỉ dẫn của từng mẫu; `Channel.dc.html`/`MChannel.dc.html` cũng có khung ảnh. API hiện hành có `ImagePrompt.sampleUrls` là ảnh mẫu thật đã vẽ, không có poster của template trong `TemplateSummary`.
- Bằng chứng QA: API native có 9 chỉ dẫn, 2 chỉ dẫn có ảnh mẫu thật; có một mẫu video đang dùng ảnh mẫu đó. Hai mẫu mặc định của ba kênh chưa có ảnh mẫu. Renderer đã nối ảnh thật theo đúng `sceneId`, không bịa preview URL hoặc tự vẽ thêm; đang giữ hình chờ trung tính khi thiếu/tải lỗi.
- Câu hỏi: Khi chưa có ảnh mẫu, giữ hình chờ trung tính kèm nghĩa “Chưa có ảnh mẫu” hay dùng hình minh hoạ tĩnh từ thiết kế và ghi rõ đó không phải ảnh do template dựng ra?
- Phương án A — Ảnh thật hoặc hình chờ: không thêm dữ liệu/asset giả và không tốn lượt vẽ. Có khác biệt nội dung ảnh với các ví dụ artboard khi tài khoản chưa vẽ mẫu; không dùng điều đó để tuyên bố khớp từng pixel.
- Phương án B — Ảnh thật ưu tiên, thiếu thì minh hoạ thiết kế: dùng asset thiết kế như đồ hoạ sản phẩm, ghi rõ “Minh hoạ bố cục”, không lưu nó làm ảnh AI hay bản xem thử. Không gọi AI, nhưng cần duyệt chính sách dùng ảnh minh hoạ trong UI dữ liệu thật.
- Không sửa artboard; không tạo thêm lượt suy luận/ảnh để lấp thiếu dữ liệu trong QA.
- Quyết định người dùng: chọn **Ảnh thật hoặc hình chờ** qua câu hỏi#90. Có ảnh mẫu thì hiển thị đúng ảnh của chỉ dẫn đang gắn; thiếu/tải lỗi thì ô trung tính với nghĩa “Chưa có ảnh mẫu”. Không dùng ảnh thiết kế thay dữ liệu thật và không tốn thêm lượt vẽ.
