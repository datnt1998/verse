# Viderse — Lời văn

## Giọng
- **Ngôi xưng:** tiếng Việt, ngôi “bạn”. Trợ lý là người làm việc thật: “Hoạ sĩ đang vẽ cảnh 5”, không viết “Đang xử lý ảnh”.
- **Nói kết quả, không nói cơ chế:** “Ảnh đã xong được giữ lại”, không viết “Artifacts được bảo toàn”.
- **Một khái niệm một tên.** Tên chuẩn ở bảng dưới; không dùng từ đồng nghĩa.
- **Không lộ thông tin kỹ thuật** (tên công cụ, phiên bản, đường dẫn, mã lỗi) ngoài “Chi tiết” và Cài đặt › Nâng cao.
- **Số:**
  - Viết gọn, theo kiểu Việt: `1,24 Tr`, `286 N`, `×4,2`. Tooltip ghi số đầy đủ.
  - Giờ 24h `00:40`; ngày `26/10`.
  - Thời lượng dùng chữ mono `42:10`.

## Tên chuẩn

| Dùng | Không dùng |
|---|---|
| Bàn dựng | Tổng quan, Video, Tất cả video, Clone |
| Video (của bạn) | Job, bản clone |
| Video nguồn / Video đối thủ | Video gốc (trừ “Xem video gốc”) |
| Làm video · Làm N video | Clone, Bắt đầu ngay, Thêm vào hàng chờ |
| Chờ lượt | Hàng chờ (khi là trạng thái), Đang chờ |
| Đang làm | Đang chuẩn bị, Đang dựng & xuất |
| Chờ duyệt | Đã vẽ xong |
| Cần xử lý | Lỗi, Thất bại |
| Tạm dừng / Chạy tiếp | Dừng, Huỷ (Huỷ là thao tác khác) |
| Sẵn sàng | Xong, Hoàn tất |
| Đã đăng | Posted |
| Chờ bạn | Cần chú ý |
| Mọi kênh | Tất cả kênh, Tổng quan |
| Prompt ảnh (kho: Thư viện › Prompt ảnh) | Kiểu ảnh, Style, Mẫu ảnh | <!-- r7-prompts -->
| Cách ghép ảnh vào video (Toàn khung · Tách nền · Cận cảnh) — thuộc template | Kiểu ảnh, Cách đặt vào video | <!-- r9-assets -->
| Bố cục bìa | Kiểu ảnh bìa |
| Theo nhận diện kênh (prompt ảnh cảnh có sẵn) | Mặc định Viderse, Mặc định · Toàn khung / Tách nền / Cận cảnh | <!-- r10-channel -->
| Phong cách nhận diện (app tự rút, chỉ đọc) | Look, phong cách kênh, phong cách ảnh chung |
| Đối thủ tham chiếu · Tham chiếu nhận diện | Đối thủ chính, reference |
| Nội dung đăng (tiêu đề, mô tả, tag) | Metadata, thông tin SEO |
| Đoạn cuối mô tả · Tag luôn thêm | Footer, tag mặc định |
| Dùng ảnh của bạn (ảnh bìa) | Tải ảnh bìa của bạn, upload thumbnail |
| Theo template · riêng video này | Mặc định, Kế thừa, Override, Prompt riêng | <!-- r8-pm -->
| Ảnh (tên chip ngoại lệ ở Bàn dựng) · Ảnh cảnh · Ảnh bìa (tên tab) | Prompt ảnh của video, Kiểu ảnh |
| Kết nối AI · Trợ lý AI | Vẽ ảnh (tên mục cũ), Provider, AI settings | <!-- r8-ai -->
| Nhà cung cấp · Đăng nhập gói · Khoá API · Trên máy này | Provider, Subscription, API key (trừ trong ô nhập) |
| Nguồn vẽ ảnh · Nguồn dự phòng | Image provider, fallback |
| Duyệt cả video & dựng | Duyệt & xuất |
| Tải gói đăng | Tải trọn bộ |
| Nhịp mỗi ngày | Giới hạn mỗi ngày, Tối đa mỗi ngày |
| Ưu tiên | Lên đầu (dùng “Lên đầu” chỉ cho nút trong Hàng chờ) |
| Luồng chạy cùng lúc | Số video chạy cùng lúc, concurrency |
| Trinh sát · Đạo diễn · Hoạ sĩ · Kiểm định | download, scenes, images, build… |
| Độ hoàn thiện | Điểm, score |
| Ảnh bìa (của video) | Thumbnail (trừ “JPG cho YouTube Studio”) |
| Ảnh bìa kênh (banner trang kênh, 2560×1440) | Banner, ảnh bìa (khi nói về kênh) | <!-- r11-audit -->
| Theo dõi đối thủ (kho ở Khám phá) · Liên kết đối thủ (với một kênh) · Bỏ liên kết | Thêm đối thủ, Xoá đối thủ (khi chỉ bỏ khỏi kênh) |
| Video nền (tab Thư viện) · Kênh nền · Video lẻ · Clip cố định (video lẻ đã gắn vào template) | Nguồn nền (trùng nghĩa “Nguồn” = video đối thủ), backdrop, fixed clip | <!-- r22-sources -->
| Cần sửa · Đoạn lệch · Khớp theo giọng · Dời cả đoạn · Nghe lại đoạn này · Đo lại độ hoàn thiện · Sửa tiếp · Vẫn xuất · Nâng cao | Sync, Align, Re-measure, Clip/Transition/Canvas (tên tab cũ), V/T (công cụ) | <!-- r27-editor -->
| Lấy nền từ · Gắn kênh hoặc video · Ngẫu nhiên (cách lấy của kênh) · Luôn có (cách lấy của video lẻ) · Chọn video · Nền của một video · Cách cắt clip | Ghim, pinned, Nguồn video nền, Số clip mỗi video (tên mục cũ) | <!-- r25-backdrops -->
| Tài khoản nền tảng · Nền tảng · Tạm chặn tới HH:MM | Tài khoản phụ (tên mục cũ), Accounts, Rate limit |
| Đăng nhập bằng mã QR · Mã QR (cách đăng nhập) · Cookie dán tay · Dán cookie thủ công (hành động) · Gia hạn ngay · Cần đăng nhập lại | Cookie thủ công (khi nói cách đăng nhập), QR login, Refresh, Renew cookie, Hết phiên | <!-- r24-biliqr -->
| 哔哩哔哩 (app Bilibili bản Trung Quốc) · bilibili.tv (bản quốc tế, không dùng được) | Bilibili app (khi cần phân biệt hai bản) |
| Lớp phủ | Overlay |
| Template luân phiên | Rotation |
| Hồ sơ kênh (tab) · Nhận diện (phần trong tab) | Thiết kế (tab kênh), Cài đặt kênh, Branding |
| Việc nền · Thông báo | Jobs, Hoạt động, Inbox |
| Video đã chọn (panel trong Khám phá) | Trang video đối thủ, Chi tiết video (desktop) |
| Bản tin (khi bạn quay lại) | Tóm tắt, Digest, Báo cáo | <!-- r16-agent -->
| Đề xuất · <Trợ lý> đề xuất · Bỏ qua · Đừng đề xuất kiểu này | Gợi ý AI, Recommendation, Insight (khi là việc nên làm) |
| Vì sao? (link) · <Trợ lý> giải thích | Giải thích AI, Why, Lý do của AI |
| Mức tự chủ: Cẩn thận · Cân bằng · Tự chạy · Tuỳ chỉnh | Autopilot, Chế độ tự động, Tự duyệt ảnh (tên thiết lập cũ) |
| Duyệt theo điểm ≥ N · Tự duyệt hết · Luôn hỏi | Auto-approve, Tự duyệt ảnh bật/tắt |
| Điểm ảnh (của Kiểm định) | Độ hoàn thiện (chỉ dùng cho cả video), score |
| Tự làm (nhãn việc máy tự quyết) | Auto, Tự động hoá |
| Quy tắc đã học (viết) · Ghi chú đã học (prompt ảnh) · Lưu quy tắc | Bộ nhớ, Memory, Học máy |
| Trợ lý nhận ra · Sơ bộ (dưới 20 video) | Insight, Phân tích AI |
| Ưu tiên kênh: Đều đặn · Lượt xem | Mục tiêu kênh, Goal |
| Kế hoạch · Làm N bước · Sửa câu | Plan, Chạy kế hoạch |
| Lưu trữ (mục Cài đặt) · Thư mục lưu dự án · Chọn thư mục… · Chuyển sang thư mục mới | Dọn dữ liệu (tên mục cũ), Storage, Workspace, Data dir | <!-- r17-storage -->

## Mẫu câu theo tình huống

| Tình huống | Câu |
|---|---|
| Kế hoạch, video chọn tay | “Lên đầu hàng chờ · bắt đầu khoảng 00:40, khi một trong 2 video đang chạy xong” |
| Kế hoạch, máy rảnh | “Bắt đầu ngay khi bấm” |
| Kế hoạch, hàng loạt | “Vào cuối hàng chờ, theo nhịp 5 video/ngày của Midnight Tales. Hôm nay còn 2 lượt, video thứ 3 chạy ngày mai.” |
| Trùng link | “Kênh này đã có video từ link này: “<tiêu đề>”, đang vẽ ảnh.” + [Mở video đang làm] |
| Link lỗi | “Không đọc được video từ link này. Kiểm tra lại link YouTube, hoặc video có thể đã bị ẩn.” |
| Chờ hạn mức | Dòng phụ: “Chờ hạn mức ảnh · 07:00” (không là lỗi) |
| Đăng nhập hết hạn | “Phiên đăng nhập ChatGPT đã hết hạn ở cảnh 5. Ảnh đã xong được giữ lại; đăng nhập lại là Hoạ sĩ vẽ tiếp.” |
| Khoá vì đang chạy | “Đợi Hoạ sĩ vẽ xong cảnh 5 rồi sửa” |
| Khoá duyệt | “Chờ Hoạ sĩ vẽ lại xong ảnh đang vẽ” |
| Xoá template đang dùng | “Đang dùng ở 2 kênh. Bỏ khỏi kênh trước rồi xoá.” |
| Xoá kênh đang chạy | “Kênh có 2 video đang làm. Tạm dừng hoặc đợi xong rồi xoá.” |
| Sau khi đánh dấu đã đăng | “Xoá MP4 để lấy lại 172 MB? Giữ project, ảnh và thông tin đăng.” |
| Hàng chờ trống | “Hàng chờ trống” · “Thêm video từ Khám phá hoặc dán link ở Video mới.” |
| Đã duyệt hết | “Đã duyệt hết” · “Kiểm định đang dựng 3 video vừa duyệt. Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.” |
| Nhắn trợ lý không hiểu | “Chưa làm được việc này.” + 2 gợi ý |

**Trợ lý chủ động** <!-- r16-agent -->
- Bản tin, câu đầu: “Trong lúc bạn vắng: 2 video sẵn sàng đăng, 3 chờ bạn duyệt, 1 cần xử lý.” Chỉ đếm; không khen, không “Chào buổi sáng”.
- Đề xuất: câu mệnh lệnh nhẹ có đối tượng + bằng chứng một dòng có số: “Đưa ‘…’ lên đầu hàng của Midnight Tales” / “Nocturne Stories · ×4,2 · đang thứ 6 ở cuối hàng”. Không “Bạn có muốn…?” trừ khi đổi thiết lập (“Quiet Hours: bật Duyệt theo điểm?”).
- Gợi ý học: “<Trợ lý> nhận ra: lần thứ 3 bạn <kiểu sửa>. Lưu thành quy tắc cho <kênh>?” Nút: **Lưu quy tắc** · **Không**.
- Vì sao?: một câu lý do, rồi số và nguồn của số. Không có số thì không viết lý do.
- Việc tự làm: động từ quá khứ + kết quả + nguồn quyết định: “Vẽ lại 2 ảnh lỗi bằng Grok (ChatGPT hết hạn mức lúc 02:14)”, nhãn **Tự làm**.

## Từ mã lỗi tới câu

| `code` | Câu | Nút |
|---|---|---|
| `login_required` | “Cần đăng nhập ChatGPT để Hoạ sĩ vẽ tiếp.” | Đăng nhập lại |
| `cookies_failed` | “YouTube yêu cầu đăng nhập để tải video này.” | Mở Tài khoản nền tảng |
| `unsupported_platform` | “Chưa hỗ trợ <nền tảng>. App lấy nguồn nền từ Twitch và Bilibili.” | Gửi yêu cầu hỗ trợ <nền tảng> | <!-- r22-sources -->
| `platform_paused` | Không hiện lỗi; nhãn “Tạm chặn tới 02:10” + banner một dòng | Tài khoản nền tảng |
| QR `expired` | “Mã đã hết hạn sau 3 phút.” | Tạo mã mới | <!-- r24-biliqr -->
| QR `rejected` | “Bilibili chưa chấp nhận đăng nhập” + “Có thể đã bấm Huỷ trên điện thoại, hoặc Bilibili cần kiểm tra thêm tài khoản này. Tạo mã mới rồi xác nhận lại.” | Tạo mã mới |
| QR lỗi mạng | “Chưa kết nối được Bilibili” + “Máy chạy Viderse chưa gọi được Bilibili. Kiểm tra mạng của máy đó rồi tạo mã mới.” | Tạo mã mới |
| `login_expired` (gia hạn thất bại) | “Đăng nhập Bilibili đã hết hạn” + “App không gia hạn được lúc 08:12. Quét mã QR lại; trong lúc chờ, app vẫn quét kênh không đăng nhập nên dễ bị chặn hơn.” | Quét mã QR |
| Cookie dán sai | “Cookie này không đăng nhập được Bilibili. Dán lại cookie bilibili.com hoặc quét mã QR.” | Đăng nhập bằng mã QR |
| `unavailable` | “Video nguồn không còn xem được (đã ẩn hoặc xoá).” | Bỏ video này |
| `no_subtitles` | “Video không có phụ đề. Trinh sát sẽ tự chép lời từ âm thanh.” | (tự xử lý) |
| `rate_limited` | Không hiện lỗi; dòng phụ “YouTube đang giới hạn · thử lại 14:20” | — |
| `quality_gate` | “Chưa đạt độ hoàn thiện 84/100” + danh sách mục | Sửa trong editor · Vẫn xuất |
| `auth_failed` | “<Nhà cung cấp> từ chối khoá API này. Kiểm tra lại khoá.” | Đổi khoá · Thử lại | <!-- r8-ai -->
| `capability_missing` | “<model> không đọc được ảnh” / “chỉ vẽ ảnh, không viết chữ” | (chọn model khác) |
| `template_changed` | “Template đã đổi số ảnh hoặc prompt ảnh. Đạo diễn cần chia cảnh lại (tốn 7 ảnh).” | Chia cảnh lại |
| `failed` | Một câu từ lý do đã dịch; nguyên văn trong “Chi tiết” | Thử lại |

<!-- r3 -->
