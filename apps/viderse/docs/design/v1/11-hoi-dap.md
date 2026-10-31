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
  - Gói V22 và W13 › Lưu trữ: thư mục lưu dự án mặc định là `~/.viderse/projects` (thư mục dữ liệu của bản web), thay cho `~/Library/Application Support/Viderse/data/projects` của Electron. Đổi thư mục không cần khởi động lại app: máy chủ đọc đường dẫn từ cơ sở dữ liệu, không dùng `storage.json`.
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

### #17 · Màn Chuẩn bị máy nằm ở đâu — Mở
- [02-stack-verse.md](02-stack-verse.md) §4 ghi Lần đầu mở là màn `Setup` của `platform/webapp`. Màn đó chỉ tạo tài khoản quản trị (trước khi đăng nhập), không cài công cụ.
- `Onboard` · `step=setup` (W1, 6 việc, Đăng nhập ChatGPT) cần chạy sau khi đăng nhập.
- Code làm: thêm route `#/setup` (Chuẩn bị máy). Sau khi đăng nhập, máy chưa xong 6 việc thì app mở `#/setup`; xong thì sang Tạo kênh. Trên điện thoại, `#/setup` hiện "Mở trên máy tính" như ia-flows §6.2.
- Hỏi design: đồng ý thêm dòng `#/setup` vào bảng route ở §4?

### #18 · Cài đặt › Công cụ: “Máy sẵn sàng · 2/2 luồng” — Mở
- Artboard: `Settings` · `section=tools`.
- Số luồng thuộc phase Hàng chờ (W9/L5). Hiện chưa có cài đặt luồng, nên `/api/status` chưa có `lanes`.
- Code làm: thẻ tổng chỉ ghi “Máy sẵn sàng” (kèm “có n bản cập nhật” nếu có), không bịa số luồng. Phase Hàng chờ thêm `lanes` thì hiện lại đủ câu.
- Hỏi design: đồng ý tạm bỏ “· 2/2 luồng” tới phase Hàng chờ?

### #19 · Cột trái Cài đặt: hàng “Giới hạn từng kênh” — Mở
- Artboard: `Settings` (mọi `section`), nhóm “Ở trang khác”, trỏ sang `Channel`.
- Router chưa có trang giới hạn của kênh (`#/channels/:id?tab=sources` thuộc phase Kênh).
- Code làm: chưa dựng hàng này, không đặt link tới trang chưa có. Phase Kênh có route thì thêm.

### #20 · Báo lỗi: ngày phát hành của bản mới — Mở
- Artboard: `Settings` · `section=logs`, thẻ phiên bản có ngày phát hành.
- `/api/version` chưa trả ngày phát hành, nên code chỉ hiện số bản mới nhất.
- Hỏi design: cần ngày thật không? Nếu cần, máy chủ phải đọc ngày từ nguồn phát hành.

### #21 · Lưu trữ: chọn thư mục bằng danh sách của máy chủ — Mở
- Artboard: `Settings` · `section=storage&store=pick`; `MSettings` · `screen=storage&store=pick`.
- Artboard mở hộp chọn thư mục của hệ điều hành. Web không lấy được đường dẫn thật trên máy chủ (xem #10).
- Code làm: “Chọn thư mục…” mở danh sách thư mục do máy chủ liệt kê (Dialog trên máy tính, Sheet trên điện thoại), chọn xong quay về khối chọn ngay tại chỗ.

### #22 · Lưu trữ: “Ổ ngoài USB-C” — Mở
- Artboard ghi “Ổ ngoài USB-C”. `diskutil` chỉ báo giao tiếp (`USB`, `Thunderbolt`…), không phân biệt đầu cắm USB-C với USB-A.
- Code làm: hiện “Ổ ngoài USB” / “Ổ ngoài Thunderbolt” theo đúng `diskutil`; chưa phân loại được thì chỉ ghi “Ổ ngoài”.
- Hỏi design: đồng ý đổi chữ mẫu trong artboard thành “Ổ ngoài USB”?

### #23 · Lưu trữ: số video trong banner chuyển thư mục — Mở
- Artboard: `store=done` có “2 video đã chạy tiếp.”; `store=moving` có “Xem khi xong ›”; banner `pick`/`moving` nêu số video đang chạy.
- Chưa có hàng chờ video (phase Hàng chờ), nên chưa đếm được video bị dừng rồi chạy tiếp. “Xem khi xong ›” trong artboard chỉ là nút đổi trạng thái tĩnh.
- Code làm: banner nêu số việc nền đang chạy (`/api/activity`), không có việc nào thì dùng câu không số. Bỏ “2 video đã chạy tiếp.” và “Xem khi xong ›” tới phase Hàng chờ.

### #24 · Nút cao 32px (`ghost btn`) trong Cài đặt — Đã chốt
- Artboard `Settings` vẽ các nút `ghost btn` (ví dụ “Xem trước những gì sẽ dọn”, “Mở trang xAI”) cao 32px, bo 8, chữ 13. Kit lúc đầu chỉ có 30 (`sm`), 36 (mặc định) và 44 (`lg`).
- Trả lời (dev, 30/10/2026): thêm `size="compact"` (32 · bo 8 · chữ 13) vào `Button` của kit. Chỉ thêm, Noverse không đổi. Mỗi nút dùng đúng cỡ theo class artboard tại chỗ đó: `ghost btn` → `compact`, `vv-btn--sm` → `sm`, `vv-btn` → mặc định, điện thoại `vv-btn--m` → `lg`.

### #25 · Tài khoản nền tảng: mốc “từ HH:MM” và “1 video” — Mở
- Artboard: `Settings` · `section=accounts&bili=blocked` ghi “từ 00:10”; `yt=bot` ghi “phụ đề của 1 video”.
- Cơ sở dữ liệu chỉ có `paused_until`, không có mốc bắt đầu bị chặn. Lỗi bot của YouTube cũng chưa đếm số video bị ảnh hưởng.
- Code làm: “từ HH:MM” lấy giờ của lần lỗi cuối (`lastError.at`), tức lần lỗi đã đẩy nền tảng sang tạm chặn. Banner bot ghi “một số video”, không bịa số.
- Hỏi design: chấp nhận? Nếu cần đúng hơn: thêm cột `paused_since`, và đếm `videosAffected` khi có hàng chờ tải.

### #26 · Kết nối AI: dữ liệu artboard chưa có nguồn — Mở
- Artboard: `Settings` · `section=connect|agents`; `MSettings` · `screen=connect|agents`.
- **Gói Grok:** xAI không trả tên gói, nên thẻ Grok bỏ “gói …”. ChatGPT vẫn có gói, đọc từ `chatgpt_plan_type` trong token.
- **“Tháng này $1,84” (`spendMonth`):** `image_receipts` không ghi giá, nên chưa hiện. Cần pipeline ghi giá mỗi lần gọi.
- **Ghi chú của model** (“Trong gói · viết và đọc ảnh tốt nhất”): đây là câu biên tập, wire chỉ có khả năng, ngữ cảnh và giá. Code hiện “chữ · đọc ảnh · 272K ngữ cảnh”.
- **Giờ đặt lại hạn mức:** artboard ghi 07:00. Code dùng mốc thật nếu provider báo, nếu không thì dùng nửa đêm giờ máy, vì đó là lúc trần ảnh mỗi ngày của app (D7) đặt lại.
- **Mức suy nghĩ trên điện thoại:** artboard điện thoại không có chỗ chỉnh. Code đặt nó trong sheet Đạo diễn.
- **Nút thêm nhà cung cấp:** bị khoá khi ô khoá API còn trống. Artboard luôn điền sẵn nên không vẽ trạng thái này.
- Hỏi design: chốt từng ý trên. Muốn câu ghi chú theo model thì cần một bảng câu.

### #27 · Kết nối AI: thứ tự thẻ và thẻ trùng loại — Mở
- Artboard: `Settings` · `conn=list` xếp ChatGPT, Grok, Gemini, Ollama (cũ trước); `conn=added` và `conn=failed` đặt thẻ vừa thêm hoặc vừa lỗi lên đầu.
- Code làm: xếp mới nhất trước (`created_at DESC`), khớp `added`/`failed`, lệch `list`.
- Thẻ lỗi vẫn để ô nguồn đó bấm được, nên dán khoá mới sẽ tạo thẻ thứ hai cùng loại thay vì sửa thẻ cũ.
- Hỏi design: giữ “mới nhất trước” hay “lỗi trước, còn lại cũ trước”? Cho thêm thẻ trùng loại, hay bắt dùng **Đổi khoá** trên thẻ cũ?

### #28 · Trợ lý AI: giá mỗi video và nguồn vẽ thứ 4 — Mở
- Artboard: `Settings` · `section=agents`; `MSettings` · `screen=agents`.
- “≈ $0,27 mỗi video” = giá mỗi ảnh × 7. Số 7 lấy từ design (“~7 ảnh” mỗi video), khai báo một chỗ (`IMAGES_PER_VIDEO` trong `AgentsSection.tsx`), chưa lấy từ template của kênh.
- Kho có nguồn vẽ thứ 4 (Tương thích OpenAI), nên lưới 3 cột của thẻ nguồn vẽ xuống dòng 2; artboard chỉ vẽ 3 nguồn.
- Hỏi design: giữ số 7 cố định hay lấy số ảnh của template? Lưới 4 nguồn: xuống dòng (đang làm), cuộn ngang, hay 4 cột?

### #29 · Tài khoản nền tảng trên điện thoại: state chưa vẽ và giãn dòng — Mở
- `MSettings` không có frame cho `yt=bot`, hồ sơ YouTube “Tắt” và `qr=loading`.
- Code làm: banner bot nằm trong thân thẻ YouTube (cao thêm 48px, như banner Bilibili bị chặn/hết hạn); `qr=loading` dùng lại khung chờ của máy tính.
- Thẻ điện thoại trong artboard giãn dòng 1,5; artboard máy tính và phần còn lại của app dùng 1,55. Code chỉ đặt 1,5 trong thẻ tài khoản và sheet trạng thái trên điện thoại.
- Hỏi design: duyệt hai state trên? Giãn dòng 1,5 là chuẩn cho cả `MSettings` hay chỉ thẻ này?

### #30 · Khung Cài đặt: dải mất kết nối, tông chấm cảnh báo, nhãn pha cài — Mở
- Artboard `Settings` không vẽ dải mất kết nối. Code chèn dải giữa đầu trang và nội dung, nội dung bị đẩy xuống thay vì bị che.
- Chấm cảnh báo ở bánh răng lấy tông theo `kind` trong `/api/status.attention` (`tool_failed`, `login_expired`, `tool_update`, `quota_low`), bảng kind → tông nằm ở web. Thêm kind mới thì cần chốt tông ở đây.
- Tiến trình cài công cụ trả `phaseLabel` là câu tiếng Việt của máy chủ, không phải mã pha; client khác (nếu có) hiện nguyên câu này.
- Pha không biết tổng dung lượng (tải mô hình nhận giọng nói, dựng môi trường Python) trả `pct: null`. Artboard chỉ vẽ pha có phần trăm (“· 62%”). Code chỉ ghi tên pha, cạnh đó là vòng quay của hàng; không vẽ “0%” và thanh đứng yên.
- Hỏi design: dải đẩy hay phủ lên nội dung? Đồng ý bảng tông theo kind? Pha không biết tổng có cần thanh chạy vô định không?

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

### #38 · Template và Prompt ảnh: nút cần một video đã làm xong — Mở
- Artboard: `Template` (`preview=fresh|stale|building`, “Vẽ thử ảnh bìa”), `Library` · `tab=prompts` (“Ảnh mẫu”, “Vẽ thử với…”).
- Hàng chờ và pipeline làm video là gói V6, editor là V13. Chưa có video nào đã làm xong và chưa có câu chuyện đã phân tích, nên chưa dựng được bản xem thử, chưa vẽ thử bìa, chưa vẽ ảnh mẫu.
- Code: trạng thái xem thử vẫn thật (`none|fresh|stale` theo `revision` của template). “Dựng lại bản xem thử”, “Vẽ thử ảnh bìa”, “Vẽ thử với…”, “Mở trong editor” hiện nhưng bị khoá (`aria-disabled`, nhận focus, đọc được lý do). Prompt ảnh hiện “Chưa có mẫu”. `preview=building` chưa xảy ra được.
- Hỏi design: duyệt trạng thái khoá tới V6/V13, và câu lý do trong `i18n/library.ts`.

### #39 · Kênh › Video, số liệu đầu kênh, “Hôm nay x/y”, Dặn trợ lý — Mở
- Artboard: `Channel` · `tab=videos`, đầu kênh, cột phải.
- Bảng video 4 cột cùng `BulkBar`, 4 số liệu đầu kênh, chấm “Chờ bạn”, thanh “Hôm nay x/y” và ô Dặn trợ lý cần job làm video (V6) và trợ lý (V11). Mức tự chủ và Ưu tiên kênh là phase sau (§20).
- Code: tab Video luôn hiện trạng thái kênh mới (`fresh`: “Kênh chưa có video” và hai thẻ dẫn). Không bịa số. Cột phải “Đang áp dụng cho kênh” có Tự lấy video, Template luân phiên, Tiêu đề–mô tả–tag, Trợ lý AI; chưa có Mức tự chủ. Xoá kênh luôn với `runningCount = 0`.
- Hỏi design: duyệt bản tạm này tới V6/V11.

### #40 · Mẫu chuẩn khi tạo kênh — Mở
- Artboard: `CreateChannel` · `step=done` (“Mẫu chuẩn chạy nền”), `Channel` · `tab=templates`.
- yt-factory dựng Mẫu chuẩn là một project editor (3 cảnh, nút Subscribe, tuyết, sóng âm). Editor là V13.
- Code: tạo kênh tạo ngay một template “Mẫu chuẩn · <tên kênh>” với recipe mặc định (Tách nền, prompt “Theo nhận diện kênh”, bìa “Chữ hook”, lớp phủ Tuyết) và gắn vào vòng luân phiên của kênh. Kênh tạo không tên (để Đạo diễn đặt) thì template tạm tên “Mẫu chuẩn”, rồi tự đổi sang “Mẫu chuẩn · <tên kênh>” khi kênh có tên (Đạo diễn đặt hoặc chủ tự đặt). Template đã đổi tên tay hoặc kênh khác cũng dùng thì giữ nguyên. Phần bố cục editor thêm ở V13.
- Hỏi design: duyệt.

### #41 · Giá ước tính của template — Mở
- Artboard: `Template` (giá mỗi video), 07 §15 `price.perImage`, hỏi đáp #28 (giá = 7 ảnh).
- Code tính theo số ảnh của chính template cộng 1 ảnh bìa: `(ảnh mỗi video + 1) × giá mỗi ảnh`. Template chọn “theo số phút” chưa biết số ảnh khi chưa có video nguồn, nên chỉ ghi cách tính, không ghi giá. Template không có ảnh vẫn tính 1 ảnh bìa. Không biết giá model vẽ thì chỉ ghi số ảnh.
- Hỏi design: duyệt bỏ hằng 7 ở màn Template.

### #42 · Nhận diện kênh: định dạng ảnh, lỗi và giờ tự thử — Mở
- Artboard: `Channel` · `identity=ready|generating|failed`, `CreateChannel` · `step=done`.
- Avatar 800 × 800 PNG, ảnh bìa 2560 × 1440 JPEG ≤ 6 MB (yt-factory dùng WebP qua ffmpeg + libwebp; ở đây mã hoá thuần Go, không cần libwebp).
- Lỗi được lưu trên kênh, sống qua tải lại trang. Hết hạn mức ảnh thì ghi giờ tự thử từ nguồn vẽ. App tắt giữa chừng thì lần đó thành “Bị gián đoạn”, không tự vẽ lại (vẽ lại tốn tiền).
- Code ghi “Tự thử lại lúc HH:MM” thay cho “Thử lại lúc HH:MM” của artboard, cho rõ là hệ thống tự thử.
- Hỏi design: duyệt định dạng và câu.

### #43 · Bỏ liên kết đối thủ trên máy tính chưa có hộp thoại — Mở
- Artboard: `Channel` · `tab=sources` chỉ có sheet bỏ liên kết ở `MChannel`.
- Code dùng `ConfirmDialog` của kit, tông cam nhạt (`dsoft`), lấy câu của sheet `MChannel`: “Bỏ liên kết <tên>?”, thân nêu hậu quả và đối thủ tham chiếu kế tiếp. Bỏ đối thủ cuối thì Tự lấy tắt, kèm lý do.
- Hỏi design: vẽ hộp thoại cho máy tính, hoặc duyệt bản này.

### #44 · Video nền: thêm kênh chạy nền, dán link không dò video — Mở
- Artboard: `Library` · `tab=backdrops` (`nguon=adding|addvideo`).
- Thêm kênh nền ghi kênh ngay rồi đọc danh mục trong việc nền; thẻ kênh hiện “Đang đọc” rồi số video (yt-factory đọc trong lúc bấm). Dán link chỉ phân tích link (và mở `b23.tv` một lần), không gọi yt-dlp, nên tên và thời lượng của video lẻ hiện sau khi thêm.
- Chiều cao tải lấy theo nền tảng: Twitch 480, Bilibili 1080 (yt-factory cứng 720). Bilibili: lời gọi API cách nhau 10–20 giây, lượt tải yt-dlp 5–10 giây; hạn mức 20 lời gọi mỗi 10 phút và 200 mỗi ngày. Bị chặn thì chờ 15 → 30 → 60 → 120 phút như dải tạm chặn ở Tài khoản nền tảng.
- Hỏi design: duyệt.

### #45 · Prompt ảnh nhập trùng tên — Mở
- Artboard: `Library` · `tab=prompts` (Nhập).
- Design không nói khi nhập một prompt trùng tên. Code tự đổi tên thành “<tên> (nhập)” để nhập cùng tệp hai lần vẫn được, không báo lỗi.
- Hỏi design: duyệt, hoặc chọn hỏi ghi đè.

### #46 · Thư viện: không có nhập template, thẻ không có ảnh thu nhỏ — Mở
- Artboard: `Library` · `tab=templates` (ô thả tệp template), thẻ template/prompt/lớp phủ có ảnh.
- Chưa có định dạng tệp template (yt-factory nhập cả project editor), nên chưa có ô nhập template; nhập/xuất prompt ảnh thì có. Thẻ chưa có ảnh vì chưa có bản xem thử hay ảnh mẫu (#38): thẻ dùng khối màu trung tính và biểu tượng theo loại, không bịa ảnh. Lớp phủ hiện khung đầu của video.
- Hỏi design: duyệt tới V13.

### #47 · Kênh, Tạo kênh trên điện thoại — Mở
- Artboard: `MChannel`, `MCreate` thuộc V10.
- Dưới 1100px, cột trái của Kênh (danh sách kênh, thư viện) xuống dưới nội dung. Dưới 768px (khung điện thoại) cột trái ẩn như `MChannel`: đổi kênh ở viên kênh trên thanh đầu (menu có “Mọi kênh”, “Tạo kênh”), tab dùng tên ngắn (Video · Nguồn · Template · Hồ sơ) và hàng tab tự cuộn ngang. Lần đầu mở là bản máy tính một cột. Trang không cuộn ngang, nút ≥ 44px khi `pointer: coarse`.
- Hỏi design: duyệt bản tạm tới V10.

### #48 · Đổi tên template không làm cũ bản xem thử — Mở
- yt-factory coi mọi lần lưu là làm cũ bản xem thử. Tên không ảnh hưởng bản dựng nên code giữ nguyên `revision` khi chỉ đổi tên.
- Hỏi design: duyệt.

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

### #50 · Gói V26: lý do phải đăng nhập lại và địa chỉ app kia — Mở <!-- r30-sso -->
- Artboard: `Auth` · `screen=login`, `kicked=self|other|password|admin|expired|unknown`; `screen=menu|denied`.
- Hỏi dev:
  1. Máy chủ có biết vì sao phiên bị thu hồi không (đăng xuất, đổi mật khẩu, quản trị viên đổi quyền/đặt lại/khoá, hết hạn), và ở app nào? Nếu có thì trả `reason`, `app` trong 401 (§25). Nếu không thì giao diện chỉ dùng câu chung, trừ hai trường hợp client tự biết (`?bye=1`, `?pw=1`).
  2. `apps: AppLink[]` (id, tên, địa chỉ gốc) trả trong `GET /api/me` và 403 `app_access_denied` được không? Địa chỉ lấy từ `VERSE_APP_ORIGINS` hay cấu hình riêng?
  3. Noverse có nhận `#/settings/members?user=<tên>` để mở thẳng người cần cấp quyền không?

### #52 · Hàng chờ + Kế hoạch (Gói V6/V2): mã lý do chưa chạy, giá, khoá xếp lại — Mở
- Artboard: `Queue` · `state=running|paused|limited|empty`, `NewVideo`.
- Chốt dev:
  1. `GET /api/queue` trả thêm `pausedAt` (giờ bấm Tạm dừng, cho câu “Đã tạm dừng lúc 00:04”) và mỗi dòng chờ có `waitReason` ngoài `pace_full|no_template` còn `queue_paused|workers_busy|rotation|paused_channel`.
  2. `paused_channel` được hiểu là **Tự lấy tắt** (`owner_channels.auto_pick = false`): chỉ khoá các mục `batch` có `origin = auto` của kênh đó; mục `pick`/`bulk` người dùng chủ động xếp vẫn chạy. Không có cột “tạm dừng kênh” nào khác.
  3. `POST /api/clone/plan` trả thêm `imageCalls` (số ảnh tính theo recipe thật, cộng 10% vẽ lại theo `yt-factory 2c201e6:src/server/clone/queue.ts:72,898`) và `priceUSD` — `null` vì chưa có bảng giá theo ảnh của model nào. `accepted[]` thêm `imageEstimate`, `imageCalls`, `priceUSD`, `viewCount`, `publishedAt`; `duplicates[]` thêm `title` (tên job/video trùng) và nhận cả bản `state: "queued"` với `jobId: null`.
  4. `PATCH /api/queue` theo hợp đồng v2: `{paused}`, `{workers, imageWorkers, rescanHours}`, `{id, position}` (vị trí số thực), `{id, action: "front"|"remove"}`. `front` chuyển mục sang `pick` và đặt trước mọi mục `pick` hiện có; nhịp chỉ tính khi mục bắt đầu với ưu tiên `batch`. `items[]` trả `position` thật để kéo thả tính midpoint, `sourceUrl` và `thumbnailUrl` khi có dữ liệu.
  5. `imageQuota[]` lấy số ảnh đã tính hôm nay từ `image_receipts`, giới hạn và giờ reset từ nguồn vẽ thật (không có số thì `null`). `doneToday[]` lấy job `stage_group` `ready|posted|review` cập nhật sau 00:00 giờ máy (`at` = `updated_at` vì chưa có cột giờ xong riêng, `posted` hiện như `ready`).
  6. SSE: một sự kiện `queue.updated` cho mọi thay đổi hàng chờ (xếp, xếp lại, bỏ, tạm dừng, drain bắt đầu/xong) + `clone.job.*` hiện hữu để invalidate; không bắn mỗi lần ETA đổi.
- Hỏi design: duyệt.

### #53 · Làm video: chọn template riêng và dữ liệu thẻ hàng chờ — Đã chốt
- Artboard: `NewVideo` · `input=pasted|multi`, `Queue` · `state=running|paused|limited`.
- Dropdown ở chip template cho phép chọn khác vòng luân phiên. Bổ sung `templateId?` vào `CloneEnqueueRequest` (lệch §4 `07-data-contract.md`): khi có, kiểm template hợp lệ của kênh và chụp đúng recipe đó; khi vắng, giữ luân phiên hiện hữu. Kế hoạch đọc cùng override.
- Dòng chờ trả `position`, `thumbnailUrl`, `sourceUrl`; thẻ đang chạy trả `pausedAt`. Đây là dữ liệu thực để kéo-thả và mở nguồn, không suy ra số hoặc ảnh ở trình duyệt.
- Dòng “Máy này chạy ổn 2 luồng” thiếu cơ sở đo máy: giữ trống tới khi design chốt; không dùng số tự suy đoán.


### #54 · Dựng bản nháp trước khi chờ duyệt — Đã chốt
- Artboard: `VideoDetail` · luồng tạo video tới `awaiting_review`; backend Phase 05.
- Dev: Phase 05 dựng project editor schema 15 bằng ảnh đã chọn, hoặc ứng viên hợp lệ mới nhất nếu chủ kênh chưa chọn. Đây chỉ là **bản nháp để duyệt**: không tự ghi thao tác duyệt ảnh, không xuất MP4, không chạy bước kiểm định/xuất.
- Khác yt-factory `build.ts` (chặn trước khi dựng khi ảnh chưa duyệt): điểm chờ được đặt **sau** khi dựng project, đúng yêu cầu Phase 05 “dựng rồi chờ duyệt”. Project và artifact được publish trước khi job đỗ `awaiting_review`, không có `retryAt`; chỉ thao tác duyệt mới mở bước tiếp.

