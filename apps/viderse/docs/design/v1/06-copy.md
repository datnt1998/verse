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
| Chờ tải nguồn cần xác thực | “Chờ xác thực YouTube”; hướng tới Cài đặt → Tài khoản nền tảng → YouTube, không báo hết hạn mức ảnh |
| Chờ tải lại nguồn / dung lượng / xử lý tạm thời | “Chờ tải lại nguồn” / “Chờ dung lượng lưu trữ” / “Chờ thử lại”, theo nguyên nhân thật |
| Đăng nhập hết hạn | “Phiên đăng nhập ChatGPT đã hết hạn ở cảnh 5. Ảnh đã xong được giữ lại; đăng nhập lại là Hoạ sĩ vẽ tiếp.” |
| Khoá vì đang chạy | “Đợi Hoạ sĩ vẽ xong cảnh 5 rồi sửa” |
| Khoá duyệt | “Chờ Hoạ sĩ vẽ lại xong ảnh đang vẽ” |
| Xoá template đang dùng | “Đang dùng ở 2 kênh. Bỏ khỏi kênh trước rồi xoá.” |
| Xoá kênh đang chạy | “Kênh có 2 video đang làm. Tạm dừng hoặc đợi xong rồi xoá.” |
| Sau khi đánh dấu đã đăng | “Xoá MP4 để lấy lại 172 MB? Giữ project, ảnh và thông tin đăng.” |
| Hàng chờ trống | “Hàng chờ trống” · “Thêm video từ Khám phá hoặc dán link ở Video mới.” |
| Đã duyệt hết | “Đã duyệt hết” · “Kiểm định đang dựng 3 video vừa duyệt. Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.” |
| Nhắn trợ lý không hiểu | “Chưa làm được việc này.” + 2 gợi ý |

**Làm video:** gọi link đầu vào là “video nguồn trên YouTube”, không bắt buộc là đối thủ đã theo dõi. Chưa nhập nguồn thì hướng dẫn dán link hoặc chọn ở Khám phá; đang đọc, đọc lỗi và nguồn đã có là các trạng thái riêng. Không báo “không có video mới” khi chưa đọc được nguồn. Thẻ Đang làm chỉ hiện thanh tiến trình khi có tiến độ đo được; trạng thái chờ không vẽ rãnh trống.

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
| `cookies_failed` | “Không đọc hoặc giải mã được cookie của hồ sơ trình duyệt.” Trên macOS: mở khóa Keychain đăng nhập của người dùng chạy Viderse, tự duyệt quyền Safe Storage nếu được hỏi, rồi chọn lại hồ sơ; không kết luận phiên Google đã hết hạn. | Mở Tài khoản nền tảng |
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

### Lỗi tải và nguồn không còn trong kho

| Chỗ / trạng thái | Câu | Nút |
|---|---|---|
| Công cụ trên máy · yêu cầu thử lại chưa gửi được | “Chưa thử lại được. Bấm Thử lại để gửi lại yêu cầu.”; lý do đã dịch được thì ưu tiên lý do đó, không hiện nguyên văn mã nội bộ | Thử lại |
| Mẫu video · đang đọc nguồn/lớp phủ | “Đang tải nguồn và lớp phủ…” | — |
| Mẫu video · chưa đọc được danh mục Thư viện | “Không tải được thư viện. Thử lại.”; giữ các nguồn đã gắn, không báo chúng bị xoá khi chỉ là lỗi tải | Thử lại |
| Nguồn đã gắn thực sự không có trong danh mục đã đọc xong | “Kênh nền không còn trong kho” / “Video nền không còn trong kho”; không dùng ID thay tên | Chọn video · Bỏ nguồn |
| Lớp phủ đã gắn không có trong danh mục đã đọc xong | “Lớp phủ không còn trong kho” · “Không tìm thấy lớp phủ đã gắn trong Thư viện.”; không dùng ID thay tên hoặc lời báo mất video nền | Bỏ hiệu ứng |
| Chưa tính được cách chia nền | “Chưa tính được nền cho video. Thử lại.”; các nguồn và chỉnh sửa vẫn giữ nguyên | Thử lại |
| Kênh · bỏ liên kết chưa thành công | “Chưa bỏ liên kết được. Thử lại.”; giữ xác nhận và lựa chọn, không lộ nguyên văn lỗi nội bộ | Bỏ liên kết · Huỷ |


## Khám phá <a id="copy-explore"></a> <!-- r28-explore -->

Câu chuẩn cho các trạng thái không vẽ frame riêng (hỏi đáp #31, #32, #34, #35). Một câu nói chuyện gì đang xảy ra, câu sau nói việc tiếp theo; không nối bằng “—”.

| Chỗ | Khoá / trạng thái | Câu | Nút |
|---|---|---|---|
| Quét lịch sử | `running` | “Đang đọc tab Videos, mỗi lượt cách nhau vài giây. Tạm dừng được; app mở lại vẫn chạy tiếp.” | Tạm dừng |
| | `queued` | “Chờ lượt quét. Trinh sát bắt đầu sau ít giây.” | — |
| | `limited` | “YouTube đang giới hạn tốc độ. Trinh sát tự thử lại lúc 01:10.” (không có giờ: “…sau ít phút.”) | Tạm dừng |
| | `paused` | “Đã tạm dừng. Video đã quét vẫn dùng được trong Khám phá.” | Tiếp tục |
| | `capped` | “Đã quét 2.000 video mới nhất, đủ giới hạn đặt cho đối thủ này. Video mới vẫn được kiểm tra mỗi giờ.” | Quét lại |
| | `complete` | “Đã quét hết lịch sử kênh (1.284 video). Video mới vẫn được kiểm tra mỗi giờ.” | — |
| | `idle` | “Chưa quét lịch sử. Bấm Quét lịch sử để Trinh sát đọc tab Videos.” | Quét lịch sử |
| | `cancelled` | “Đã huỷ quét lịch sử. Video đã quét vẫn dùng được trong Khám phá.” | Quét lại |
| | `error` | “Quét lịch sử dừng vì lỗi. Bấm Tiếp tục để thử lại; video đã quét vẫn dùng được.” | Tiếp tục |
| | `discovery` | “Đang kiểm tra video mới trên kênh này.” | — |
| Thêm đối thủ | `invalid_input` | “Chưa nhận ra kênh YouTube. Dán @handle, link kênh hoặc mã UC…” | — |
| | `not_found` | “Không tìm thấy kênh này trên YouTube. Kiểm tra lại @handle hoặc link.” | — |
| | `exists` | “Đã theo dõi đối thủ này.” | Mở |
| | `rate_limited` | “YouTube đang giới hạn tốc độ. Thử lại lúc 01:10.” | — |
| | `blocked` | “YouTube đòi xác minh không phải bot. Chọn hồ sơ trình duyệt có đăng nhập Google ở Tài khoản nền tảng rồi thử lại.” | Mở Tài khoản nền tảng |
| | `unexpected_page` | “YouTube trả về trang không đọc được. Thử lại sau ít phút; lặp lại thì gửi Báo lỗi.” | Thử lại |
| | lỗi khác | “Chưa thêm được đối thủ. Thử lại.” | Thử lại |
| Lịch đăng | đang đọc | “Đang đọc giờ đăng chính xác của các video còn lại.” | — |
| | tạm nghỉ | “YouTube đang giới hạn tốc độ. Đọc giờ đăng tiếp lúc 01:10.” | — |
| | chưa đọc được | “Chưa đọc được giờ đăng chính xác của video nào. Trinh sát đọc khi quét tới.” | — |
| | không có khung | “30 video gần nhất đăng lệch giờ nhau, chưa có khung nào lặp ít nhất 3 lần.” | — |
| Nút cần kênh / hàng chờ (tới V4, V6) | khoá | “Cần một kênh của bạn để làm video. Tạo kênh ở mục Kênh.” | Tạo kênh (link) |
| Tìm tiêu đề | có kết quả | “2 video có “floor” · trong 1.284 đã quét” | — |
| | không có | “Không có video nào có “basement” trong tiêu đề” + dòng phụ nói đang tìm ở đâu, lọc gì | Xoá tìm |

<!-- r3 -->

## Lưu trữ trên máy chủ <a id="copy-storage-server"></a> <!-- r29-s3 -->

Thuật ngữ y như Noverse Gói AN. Tiếng Anh chỉ hiện nhỏ trong hướng dẫn “Lấy ở đâu?” khi cần đối chiếu với trang nhà cung cấp.

| Dùng | Không dùng |
|---|---|
| tên kho | bucket |
| địa chỉ máy chủ lưu trữ (Garage: địa chỉ máy chủ nội bộ) | endpoint |
| mã khoá truy cập · khoá bí mật | access key · secret key |
| link ký | presigned URL |
| Trực tiếp từ kho · Qua máy chủ Viderse | redirect · proxy |
| kiểu đường dẫn máy-chủ/tên-kho | path-style |
| bộ nhớ đệm làm việc | cache, work dir |
| chuyển dữ liệu · chuyển kho | migrate, migration |
| ổ đĩa máy chủ | local disk, filesystem |

| Chỗ | Trạng thái / mã | Câu | Nút |
|---|---|---|---|
| Thẻ nơi lưu | `online` | Nhãn “Hoạt động tốt” · “Kho đang dùng · kiểm tra lúc 21:31” | Kiểm tra kết nối · Đổi kho… |
| | kiểm tra lại đạt | “Kết nối đạt · 6 bước · tổng 0,4 giây · 21:33” | — |
| | `env` | Nhãn “do máy chủ đặt · chỉ đọc”. “Kho tệp do máy chủ đặt sẵn. Ở đây chỉ xem và kiểm tra kết nối được. Muốn đổi kho, đổi khoá hay cách gửi tệp thì sửa cấu hình máy chủ rồi khởi động lại Viderse.” | Chi tiết cho người quản trị máy chủ |
| Mời chuyển | `kind = disk` | “Ổ máy chủ đầy hoặc hỏng là mất tệp. Kho S3 (Garage đang chạy trên Mac mini, hoặc đám mây như Cloudflare R2) bền hơn và dễ mở rộng. Viderse chép đủ tệp, đọc lại từng tệp rồi mới đổi.” | Chọn kho S3 |
| Mất kết nối | `online = false` | “Không kết nối được kho Garage” · “Máy chủ Viderse không mở được kết nối tới garage:3900 (hết thời gian chờ). 2 video đang làm đã tạm dừng, hàng chờ chưa chạy; xem và tải video trong app đang lỗi. Kết nối lại thì tự chạy tiếp, không mất gì.” · “Lần thử gần nhất 21:42 · 40 giây trước · tự thử lại mỗi phút” | Kiểm tra lại |
| | dòng đầu trang (quản trị viên) | “**Không kết nối được kho tệp Garage.** 2 video đang làm đã tạm dừng, hàng chờ chưa chạy. Kết nối lại thì tự chạy tiếp.” | Mở Lưu trữ |
| | thành viên | “Kho tệp đang mất kết nối nên video của bạn tạm dừng. Quản trị viên đã được báo; kết nối lại thì video tự chạy tiếp.” | — |
| Form | chưa có khoá mã hoá | “Máy chủ chưa có khoá mã hoá nên chưa lưu được khoá bí mật” · “Khoá bí mật của kho luôn được mã hoá trước khi lưu, để ai đọc được cơ sở dữ liệu cũng không lấy được khoá. Bạn vẫn kiểm tra kết nối được; lưu thì phải chờ người quản trị máy chủ thêm khoá mã hoá.” | Chi tiết cho người quản trị máy chủ |
| | khối chi tiết `s3nokey` <!-- r29b --> | Dòng đầu “Tệp khoá mã hoá (Viderse tự tạo lần đầu, quyền 0600)” + đường dẫn từ `encryptionKey.path`, rồi theo `issue`: `readonly_dir` “Chưa tạo được: thư mục cấu hình chỉ đọc. Cấp quyền ghi thư mục … cho người dùng chạy Viderse, rồi khởi động lại Viderse.” · `bad_permissions` “Tệp khoá sai quyền. Đặt quyền 0600, chủ là người dùng chạy Viderse, rồi khởi động lại Viderse.” · `corrupt` “Tệp khoá hỏng nên không đọc được. Đừng xoá vội: khoá API đã lưu cũng mã hoá bằng tệp này. Khôi phục tệp từ bản sao lưu rồi khởi động lại Viderse.” Câu banner: “…lưu thì phải chờ người quản trị máy chủ sửa quyền thư mục cấu hình.” | — |
| Dung lượng | nhóm `null` | Cột dung lượng “chưa có số”, cột tệp “—”, tổng “Đã đếm 22 tệp · 400 MB”; dưới bảng: “Ảnh đã vẽ, project editor, video nguồn và bản xuất MP4 có số khi Viderse bắt đầu làm video trên máy chủ.” Thư viện có nhãn phụ “lớp phủ, video nền, ảnh kênh”. | — |
| | http bật | “Đang bật: chỉ an toàn khi Viderse và kho nằm cùng một máy hoặc cùng mạng nội bộ, không đi qua internet.” | — |
| | địa chỉ cho trình duyệt trống | “Để trống thì trình duyệt không tải được tệp, xem video sẽ lỗi. Không có tên miền thì chọn “Qua máy chủ Viderse”.” | — |
| Thanh lưu | idle · run · ok · fail · nokey | “Kiểm tra kết nối đạt rồi mới lưu được” · “Đang kiểm tra kết nối…” · “Kết nối đạt · lưu rồi xác nhận chuyển 412 tệp · 18,6 GB sang Garage” · “Kết nối chưa đạt · sửa rồi kiểm tra lại” · “Máy chủ chưa có khoá mã hoá nên chưa lưu được” | Huỷ · Lưu… |
| Xác nhận | — | “Chuyển 412 tệp · 18,6 GB sang Garage?” · “Từ ổ đĩa máy chủ · kết nối đã kiểm tra đạt · theo dõi trong Việc nền” | Huỷ · Bắt đầu chuyển |
| Đang chuyển | `running` · `paused` | Nhãn “Đang chạy” / “Tạm dừng”. “Bắt đầu lúc 21:06. Video đang làm vẫn chạy, Bàn dựng và Duyệt vẫn sửa được. Tệp mới vẫn ghi vào ổ máy chủ cho tới khi chuyển xong, rồi được chép nốt. Ổ máy chủ không bị xoá.” · “42 MB/giây · còn khoảng 4 phút” | Tạm dừng / Tiếp tục · Huỷ chuyển |
| | hỏi huỷ | “**Huỷ chuyển?** Viderse vẫn dùng ổ máy chủ, không mất tệp nào. 340 tệp đã chép nằm lại trong kho viderse-media; lần chuyển sau dùng lại, không phải chép lại.” | Tiếp tục chuyển · Huỷ chuyển |
| | `error` | Tiêu đề “Chuyển sang Garage chưa xong”, nhãn “3 tệp lỗi”. “Còn tệp lỗi thì chưa đổi kho. Viderse vẫn dùng ổ máy chủ, không mất gì.” | Thử lại 3 tệp lỗi |
| Chỗ cũ | `previous` | “Chỗ cũ: ổ đĩa máy chủ vẫn giữ 18,6 GB” · “Viderse đã thôi đọc từ ổ máy chủ. Khi đã yên tâm (mở vài project trong Bàn dựng, xem vài bản xuất), người quản trị máy chủ dọn bằng lệnh bảo trì để lấy lại chỗ. Viderse không tự xoá.” Khối chi tiết: đường dẫn chỗ cũ, “Dọn bằng lệnh bảo trì của Viderse khi đã yên tâm, như Noverse. Không xoá tay thư mục này khi Viderse đang chạy.” | Chi tiết cho người quản trị máy chủ |
| Điện thoại | dòng cuối | “Đổi kho, đổi khoá và chuyển dữ liệu làm trên máy tính: Cài đặt › Lưu trữ › Đổi kho…” (ổ máy chủ: “Chuyển sang kho S3 làm trên máy tính: Cài đặt › Lưu trữ › Chọn kho S3.”) | — |

**Bước kiểm tra kết nối** (`step`): `write` Ghi thử · Ghi một tệp nhỏ vào kho · `read` Đọc lại · Đọc đúng tệp vừa ghi · `range` Đọc một đoạn · Dùng khi tua video · `presign` Tạo link ký · Link tải ngắn hạn cho trình duyệt (qua máy chủ: “Không cần khi gửi qua máy chủ”, cột thời gian “không cần”) · `cors_probe` Trình duyệt tải được (CORS) · Trình duyệt của bạn tải thử một đoạn bằng link ký · `app_range` Máy chủ gửi được một đoạn cho trình duyệt · Máy chủ đọc từ kho rồi gửi tiếp · `delete` Xoá tệp thử · Dọn tệp thử khỏi kho. Bước chưa chạy vì bước trước lỗi ghi “bỏ qua”.

**Lỗi kiểm tra** (`StorageErrorCode` → tiêu đề · lý do · cách sửa; {P} nhà cung cấp, {B} tên kho, {H} địa chỉ, {A} địa chỉ app):

| Mã | Bước | Tiêu đề | Lý do | Cách sửa |
|---|---|---|---|---|
| `wrong_key` | write | Kho từ chối khoá truy cập | Mã khoá truy cập hoặc khoá bí mật không đúng, hoặc khoá đã bị thu hồi. | Garage: “Chạy “garage key info viderse-app --show-secret” trên máy có Garage rồi chép lại cả hai khoá.” Khác: “Mở trang khoá của {P} và chép lại cả hai ô.” · “Khoá bí mật chỉ hiện một lần lúc tạo. Mất thì tạo khoá mới.” |
| `no_bucket` | write | Không có kho tên “{B}” | Không có kho nào tên như vậy, hoặc khoá này chưa được cấp quyền vào kho. | “Kiểm tra chính tả: chữ thường, không dấu cách.” · Garage: “Tạo kho và cấp quyền bằng lệnh trong “Lấy ở đâu?”, rồi kiểm tra lại.” Khác: “Hoặc tạo kho trên trang của {P} rồi kiểm tra lại.” |
| `no_delete_permission` | delete | Khoá chưa có quyền xoá tệp | Ghi và đọc được, nhưng khoá này không được xoá. Viderse cần xoá để dọn tệp của video đã xoá và tệp thử. | “Cấp quyền đọc, ghi và xoá trong kho này cho khoá.” · “Tệp thử còn trong kho sẽ tự dọn ở lần kiểm tra đạt sau.” |
| `unreachable_endpoint` | write | Không kết nối được địa chỉ kho | Máy chủ Viderse không mở được kết nối tới {H} (hết thời gian chờ). | Garage: “Kiểm tra lại địa chỉ và cổng (thường 3900).” · “Viderse và Garage phải chung mạng Docker, hoặc dùng IP nội bộ của Mac mini.” · “Xem tường lửa có chặn cổng này không.” |
| `clock_skew` | write | Đồng hồ máy chủ bị lệch | Kho từ chối chữ ký vì giờ trên máy chủ Viderse lệch quá xa giờ thật (thường quá 15 phút). | “Bật đồng bộ giờ tự động trên máy chủ, rồi kiểm tra lại.” |
| `cors_blocked` | cors_probe | Trình duyệt bị kho chặn (thiếu CORS) | Máy chủ ghi và đọc được, nhưng kho chưa cho trang {A} tải tệp. Xem video, tua và tải bản xuất trên trình duyệt sẽ lỗi. | Đoạn cấu hình theo nhà cung cấp (R2, AWS: JSON; Garage: `cors.json` + lệnh `aws s3api put-bucket-cors`), câu “Dán xong, chờ chừng một phút rồi bấm Kiểm tra lại.”; nút Kiểm tra lại · Chuyển sang gửi qua máy chủ |
| `browser_unreachable` | cors_probe | Trình duyệt không tới được địa chỉ kho | Địa chỉ {H} chỉ dùng được bên trong mạng của máy chủ. Trình duyệt của bạn không mở được nó, nên không tải thẳng từ kho được. | “Điền Địa chỉ kho cho trình duyệt (tên miền mở ra internet, có https).” · “Hoặc chuyển sang gửi qua máy chủ Viderse, không cần mở kho ra ngoài.” |

**Lỗi chuyển** (`MigrationErrorCode`): `content_mismatch` “Chép xong nhưng đọc lại không khớp. Thường do mạng chập chờn, thử lại là được.” · `unreachable_endpoint` “Mất kết nối tới kho giữa chừng. Thử lại khi mạng ổn định.” · `storage_error` “Kho chưa nhận tệp này. Kiểm tra quyền của khoá và kết nối, rồi thử lại.” · `wrong_key` “Kho mới từ chối khoá. Khoá vừa bị thu hồi hoặc đổi.” Mã khác: “Chưa chép được tệp này. Thử lại; vẫn lỗi thì xuất log ở Báo lỗi.”

## Đăng nhập và tài khoản <a id="copy-auth"></a> <!-- r30-sso -->

Câu nào nói về đăng xuất, đổi mật khẩu hay quyền thì nói rõ ảnh hưởng **cả Noverse và Viderse**. Không hiện cookie, mã lỗi hay tên biến.

| Chỗ | Trạng thái | Câu | Nút |
|---|---|---|---|
| Đăng nhập | — | “Đăng nhập” · “Một tài khoản dùng cho Noverse và Viderse” · “Phiên giữ 30 ngày trên trình duyệt này, dùng chung cho Noverse và Viderse.” | Đăng nhập |
| | sai | “Sai tên đăng nhập hoặc mật khẩu.” | — |
| | thử sai nhiều | “Thử sai quá nhiều lần. Thử lại sau 0:28.” | Đợi 0:28 (khoá) |
| | chân trang | “Quên mật khẩu? Nhờ quản trị viên đặt lại ở Noverse › Thành viên” · “Chưa có tài khoản? Nhờ quản trị viên tạo” | — |
| Vì sao đăng nhập lại | `logout` (chính mình) | **Đã đăng xuất** “Đã đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này.” | — |
| | `logout` ở app kia | **Bạn vừa đăng xuất ở Noverse** “Hai app dùng chung một lần đăng nhập, nên Viderse cũng cần đăng nhập lại.” (ở Noverse: đổi tên hai app) | — |
| | `password_changed` | **Mật khẩu vừa được đổi** “Mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse. Dùng mật khẩu mới.” | — |
| | `admin_changed` | **Quản trị viên vừa thay đổi tài khoản này** “Có thể là quyền vào ứng dụng hoặc mật khẩu đã được đặt lại. Đăng nhập lại để tiếp tục.” | — |
| | `expired` | **Phiên đăng nhập đã hết hạn** “Phiên giữ 30 ngày. Đăng nhập lại để tiếp tục; video đang chạy trên máy chủ không bị dừng.” | — |
| | không rõ | **Phiên đăng nhập đã kết thúc** “Có thể bạn vừa đăng xuất hoặc đổi mật khẩu ở Noverse hay Viderse. Đăng nhập lại để tiếp tục.” | — |
| Chưa được cấp quyền | — | **Chưa được cấp quyền vào Viderse** “Tài khoản này đăng nhập đúng nhưng chưa được cấp quyền dùng Viderse. Nhờ quản trị viên bật ô Viderse cho bạn ở Noverse › Thành viên, rồi bấm Thử lại.” | Mở Noverse · Thử lại · Chép lời nhờ quản trị viên cấp quyền · Đăng xuất |
| | lời nhờ (chép) | “Nhờ bạn cấp quyền Viderse cho tài khoản @linh: Noverse › Thành viên › Linh › Quyền vào ứng dụng › bật Viderse.” | — |
| Đặt mật khẩu tạm | — | **Đặt mật khẩu của bạn** “Bạn đang đăng nhập bằng mật khẩu tạm quản trị viên gửi. Đặt mật khẩu riêng để bắt đầu dùng. Mật khẩu mới dùng cho cả Noverse và Viderse.” · “Đổi xong, bạn đăng nhập lại một lần bằng mật khẩu mới. Thiết bị khác cũng phải đăng nhập lại.” | Đổi mật khẩu |
| | lỗi | “Mật khẩu tạm không đúng.” · “Mật khẩu nhập lại chưa khớp.” | — |
| Thiết lập | — | **Thiết lập Viderse** “Tạo tài khoản quản trị đầu tiên. Tài khoản này dùng được cho cả Noverse và Viderse.” · mã: “Mã in trong nhật ký máy chủ khi Viderse chạy lần đầu. Ai có quyền vào máy chủ mới xem được.” · mã khôi phục: “Mã chỉ hiện một lần này. Mất cả mật khẩu lẫn mã này là mất quyền quản trị cả Noverse và Viderse. Lưu ở nơi an toàn, không lưu chung với mật khẩu.” · “Tôi đã lưu mã khôi phục” | Tiếp tục · Tạo quản trị viên · Chép · Vào Viderse |
| Máy chủ | không trả lời | **Không kết nối được máy chủ Viderse** “Máy chủ không trả lời hoặc đang khởi động lại. Video đang chạy trên máy chủ không bị dừng.” · “Noverse chạy trên máy chủ riêng nên có thể vẫn dùng được.” · “Tự thử lại sau 12 giây” | Thử lại |
| | kho tài khoản lỗi | **Chưa đọc được tài khoản** “Máy chủ Viderse đang chạy nhưng chưa đọc được kho tài khoản dùng chung, nên chưa kiểm được đăng nhập. Bạn vẫn đang đăng nhập; không cần nhập lại mật khẩu.” · “Kho tài khoản dùng chung với Noverse, nên Noverse có thể cũng đang báo lỗi này.” | Thử lại |
| Menu tài khoản | — | nhóm “Ứng dụng” · “Đang mở” · *Tài khoản* “mật khẩu, thiết bị” · *Quản lý thành viên* “Mở Noverse › Thành viên” · *Đăng xuất* “Khỏi cả Noverse và Viderse trên trình duyệt này” | — |
| Trang Tài khoản | — | “Một tài khoản dùng cho Noverse và Viderse. Đổi mật khẩu hay đăng xuất ở đây là cho cả hai ứng dụng.” · Ứng dụng: “Quản trị viên luôn vào được cả hai ứng dụng.” / “Quản trị viên cấp quyền từng ứng dụng ở Noverse › Thành viên.” / “Cần dùng Noverse? Nhờ quản trị viên cấp quyền ở Noverse › Thành viên.” · Thành viên: “Thêm người, cấp quyền vào Noverse và Viderse, đặt lại mật khẩu ở một chỗ duy nhất: Noverse › Thành viên.” · Đổi mật khẩu: “Đổi xong, mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse, kể cả trình duyệt này.” · Thiết bị: “Mỗi thiết bị một phiên chung cho cả hai app” · “Thiết bị bị đăng xuất phải đăng nhập lại ở cả Noverse và Viderse.” | Quản lý thành viên · Đổi mật khẩu · Đăng xuất · Đăng xuất mọi thiết bị khác |

Thay câu cũ trong `i18n/vi.ts`: `loginSub` “Dùng tài khoản quản trị viên đã tạo lúc thiết lập” (sai với thành viên) → “Một tài khoản dùng cho Noverse và Viderse”; `accessDenied` → câu ở bảng; `sharedLogoutNote` → “Đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này.”; `setup.codeHint` bỏ tên lệnh; `serverDownBody` bỏ dấu “—”.

## Hàng chờ và viên việc nền <a id="copy-queue"></a> <!-- r31-v27 -->

**Viên việc nền** (máy tính · điện thoại):

| Trạng thái | Máy tính | Điện thoại | Tooltip / đọc màn hình |
|---|---|---|---|
| Đang chạy | “2 đang chạy · 12 chờ lượt” | số “2” trong vòng tiến độ <!-- r36 --> | “Máy đang chạy 2/2 luồng · 12 video chờ lượt” |
| Máy rảnh | “Máy rảnh” | chỉ icon, không chữ | “Không video nào đang chạy hay chờ lượt” |
| Đã tạm dừng | “Đã tạm dừng lúc 00:04 · 12 chờ lượt” | icon tạm dừng trong vòng xám | “Bạn tạm dừng hàng chờ lúc 00:04 · video đang chạy làm nốt bước hiện tại” |
| Mất kết nối | “Mất kết nối · đang thử lại” | icon cam | “Không kết nối được máy chủ Viderse · số liệu cập nhật lại khi có kết nối” |

Sheet / màn Hàng chờ: băng tạm dừng “Đã tạm dừng lúc 00:04. Video đang chạy làm nốt bước hiện tại rồi dừng; chưa video nào mới được bắt đầu.” · mất kết nối “Không kết nối được máy chủ Viderse. Đang thử lại; danh sách dưới đây là lần cập nhật lúc 00:12.” · hết ảnh “Hết hạn mức vẽ ảnh của cả ChatGPT và Grok. Bước không cần vẽ ảnh vẫn chạy; vẽ ảnh tiếp lúc 07:00.” · máy rảnh “Máy rảnh” + “Không video nào đang chạy hay chờ lượt. Video bạn dán link chạy ngay khi có luồng trống; kênh bật Tự lấy video sẽ thêm vào đây theo nhịp ngày.” + nút *Video mới*.

**Vì sao chưa chạy** (`waitReason`, dòng phụ của từng video chờ; điện thoại ghép sau giờ dự kiến: “00:30 · Chờ tới lượt kênh này”):

| `waitReason` | Câu |
|---|---|
| `workers_busy` | Chờ luồng trống |
| `rotation` | Chờ tới lượt kênh này |
| `pace_full` | Kênh đủ nhịp hôm nay · chạy từ 00:00 |
| `no_template` | Kênh chưa có template dùng được |
| `queue_paused` | Hàng chờ đang tạm dừng |
| `paused_channel` | Tự lấy của kênh đang tắt |
| (template cần sửa, dùng template kế tiếp) | Template cần sửa (màu cam) |

**Quy tắc chạy:** “Luồng chạy cùng lúc” · “Số video chạy cùng lúc · 1 tới 6” · “Mỗi luồng là một video chạy cùng lúc. Thêm luồng thì mỗi video chậm hơn và hết hạn mức ảnh sớm hơn.” · “Ảnh vẽ cùng lúc” · “Dùng chung cho mọi video · 1 tới 8” · “Trinh sát xem đối thủ” · “Cho kênh bật Tự lấy video · 1 tới 24 giờ” · “Bớt luồng không dừng video đang chạy: luồng thừa nghỉ khi video của nó xong. Nhịp từng kênh sửa ở trang kênh.” · thẻ tóm tắt “2 luồng · 3 ảnh · đối thủ mỗi 1 giờ”. Không dùng “Máy này chạy ổn N luồng” (#55).

**Một dòng (⋯):** *Lên đầu hàng chờ* “Chạy khi có luồng trống, trước video tự lấy” (ở đầu rồi: “Đang ở đầu hàng chờ”, khoá) · *Xem video gốc* · *Bỏ khỏi hàng chờ* “Video gốc vẫn ở Khám phá; thêm lại lúc nào cũng được” → “Đã bỏ 1 video khỏi hàng chờ” + *Hoàn tác*.

**Video mới, nhiều link:** công tắc “Ưu tiên cả 3” · “Lên đầu hàng chờ, không theo nhịp kênh”. Bật: kế hoạch “Cả 3 lên đầu hàng chờ, không theo nhịp kênh · bắt đầu từ khoảng 00:40”, dòng phụ “Ưu tiên: cả 3 chạy ngay khi có luồng trống.” Tắt: “Vào cuối hàng chờ, theo nhịp 5 video/ngày của Midnight Tales. Hôm nay còn 2 lượt.” · “Hôm nay kênh còn 2 lượt, video thứ 3 chạy ngày mai.” Giá (`priceUSD`) là `null` → chỉ ghi số ảnh, không ghi tiền.

## Đăng nhập và tài khoản: trạng thái bổ sung <a id="copy-auth-v27"></a> <!-- r31-v27 -->

| Chỗ | Trạng thái | Câu |
|---|---|---|
| Mọi form (đăng nhập, mật khẩu tạm, thiết lập, đổi mật khẩu, thiết bị) | mất mạng / không gọi được máy chủ | băng cam “Không kết nối được máy chủ. Kiểm tra mạng rồi thử lại.”; chip đầu trang “Không kết nối được” |
| Nút chính | đang chờ máy chủ | spinner + “Đang đăng nhập…” · “Đang đổi mật khẩu…” · “Đang kiểm tra mã…” · “Đang tạo…” · “Đang đăng xuất…”; nút khoá (`aria-busy`) |
| Mã khôi phục | chép thất bại | “Không chép được. Chạm giữ vào mã để chọn rồi chép.” (máy tính: “Bấm đúp vào mã để chọn rồi chép bằng Ctrl/⌘ + C.”); mã chọn được cả khối |
| | máy chủ không trả mã | **Chưa nhận được mã khôi phục** “Tài khoản quản trị đã tạo xong nhưng máy chủ không trả mã khôi phục. Vào Viderse rồi tạo mã mới ở Noverse › Tuỳ chỉnh › Nâng cao › Truy cập & khôi phục.” — *Vào Viderse* mở, không có ô “Tôi đã lưu” |
| Mật khẩu tạm | lối ra | *Đăng xuất* (chân màn) + “Không phải tài khoản của bạn? Đăng xuất khỏi cả Noverse và Viderse trên trình duyệt này.” |
| Thiết bị đang đăng nhập | — | “Mỗi thiết bị một phiên chung cho cả Noverse và Viderse.” · dòng: tên máy · trình duyệt, nhãn *Thiết bị này*, “Đang dùng · còn 27 ngày” / “Hoạt động 2 giờ trước · còn 30 ngày” (còn ≤ 3 ngày: chữ vàng) · *Đăng xuất* từng máy · “Đăng xuất điện thoại này bằng nút Đăng xuất ở trang Tài khoản.” |
| | đăng xuất mọi thiết bị khác | xác nhận tại chỗ: “Đăng xuất 2 thiết bị khác? Các thiết bị đó phải đăng nhập lại ở cả Noverse và Viderse.” · *Huỷ* · *Đăng xuất 2 thiết bị* → “Đã đăng xuất mọi thiết bị khác.” |
| | đăng xuất một máy | “Đã đăng xuất Mac · Chrome.” |
| Đăng nhập | `kicked=none` | không có khối lý do. Máy chủ trả `session_ended/unknown` cho cả lần đầu (chưa cookie) và cookie hỏng; client chỉ hiện “Phiên đăng nhập đã kết thúc” khi trình duyệt này đã từng đăng nhập (cờ `vv.signedInBefore` trong localStorage, đặt sau lần đăng nhập thành công, xoá khi bấm Đăng xuất). |

## Chốt đợt 32 <a id="copy-r32"></a> <!-- r32 -->

| Chỗ | Câu |
|---|---|
| Bilibili trên điện thoại, không mở được app (#6) | “Quét bằng điện thoại khác, hoặc mở Cài đặt trên máy tính” |
| YouTube báo bot (#25) | “Trinh sát chưa tải được phụ đề của một số video.” · mốc “từ HH:MM” = lần lỗi cuối |
| Ổ lưu trữ (#22) | “Ổ ngoài USB” · “Ổ ngoài Thunderbolt” · không rõ: “Ổ ngoài” |
| Thẻ Grok (#26) | “Đã đăng nhập · 1 model đã chọn” · “400 ảnh mỗi tuần” (không ghi tên gói) |
| Model (#26) | dòng phụ theo khả năng: “chữ · đọc ảnh · 272K ngữ cảnh”; không viết câu biên tập theo model |
| Hạn mức ảnh (#26) | “đặt lại HH:MM” theo mốc nhà cung cấp báo; không có thì “đặt lại 00:00” |
| Báo lỗi › phiên bản (#20) | “Viderse 1.4.0 · Bản mới nhất” (không ngày) |
| Editor › phụ đề lệch (#13) | chưa có nhận giọng ±2 giây: nút “Dời theo đề xuất +0,6 s” |
| Ô Nhắn trợ lý (#74) | đang đọc: “Đang đọc câu của bạn…” · chưa có model: “Chưa có model viết. Thêm ở Cài đặt › Kết nối AI.” · mất kết nối: “Mất kết nối · gửi lại khi có kết nối” · kế hoạch hết hạn: “Kế hoạch đã cũ vì video vừa đổi. Gửi lại câu để lập kế hoạch mới.” · nút *Thử lại*, *Sửa câu* · bước: Đang chờ · Đang làm · Xong |
| Đề xuất, quy tắc (#75) | theo hỏi đáp #75 (đã đồng ý): “Đang tải đề xuất…” · “Không tải được đề xuất.” · “Mất kết nối · đề xuất cập nhật lại khi có kết nối” · “Tốn N ảnh. Máy sẽ xếp việc này vào hàng chờ ngay sau khi bạn bấm.” |
| Đánh dấu đã đăng (#76) | “Đánh dấu đã đăng?” · ô “Link YouTube (không bắt buộc)” · link sai: “Đây không phải link video YouTube.” |

## Ảnh, prompt và vẽ lại <a id="copy-image"></a> <!-- r33-img -->

| Chỗ | Câu |
|---|---|
| Nhóm ở cột phải | Phiên bản · N · Ảnh tham chiếu · N/4 · Prompt đã gửi · Lời kể trong cảnh |
| Phiên bản | “Bản 2 · Theo góp ý “bảo vệ đứng trên đầu thang, tông lạnh hơn” · 23:58” · “Lần đầu · Hoạ sĩ viết từ lời kể” · *Dùng bản 1 cho video* · huy hiệu ảnh “bản 2” / “bản 1 · chưa dùng” |
| Phần của prompt | Cảnh · Phong cách · Tham chiếu · Tránh (Ghi chú đã học bỏ ở đợt 44); nguồn: “Hoạ sĩ viết từ lời kể” · “Hoạ sĩ sửa theo góp ý” · “prompt “Đêm xanh rêu”” · “template Mẫu chuẩn” · “2 ảnh bên dưới” |
| Dòng dưới prompt | “ChatGPT · gpt-image-2 · 1280×720 · vẽ 44 giây · 23:58 · tốn 1 ảnh” · *Xem nguyên văn gửi máy chủ* / *Xem theo phần* · *Chép* → “Đã chép” |
| Thay đổi | “So với bản 2: Hoạ sĩ đổi 2 chỗ theo góp ý của bạn.” · *Xem thay đổi* / *Ẩn thay đổi* |
| Sửa prompt | thanh dưới “Đã sửa 1 chỗ trong prompt cảnh 3” · “Chỉ áp cho ảnh này · tốn 1 ảnh, ChatGPT còn 7/50 hôm nay” · *Huỷ sửa* · **Vẽ lại với prompt này** · phần đổi: “Đã sửa · Hoàn tác” |
| Ảnh tham chiếu | vai trò Bố cục · Nhân vật · Phong cách · Đồ vật · “Khung đối thủ · Nocturne Stories 14:02” · “Bảo vệ · từ cảnh 1” · trống: “Không dùng ảnh tham chiếu. Thêm ảnh để giữ nhân vật hoặc bố cục giống nhau giữa các cảnh.” · popover: Thêm ảnh tham chiếu · Ảnh trong video · Khung video gốc · Tải lên · “Kéo ảnh vào đây, hoặc chọn tệp” · “PNG, JPG, WebP · tối đa 20 MB” · Dùng làm · “ChatGPT nhận tối đa 4 ảnh tham chiếu” · *Thêm 1 ảnh* / “Chọn ảnh để thêm” |
| Đang vẽ | “Hoạ sĩ đang vẽ bản 3” · “0:24 · thường khoảng 40 giây” · bước Gửi prompt → Đang vẽ → Tải về (xong: “Đã gửi prompt”, “Đã vẽ”) · “Bạn duyệt tiếp ảnh khác được; xong sẽ báo ở đây và trên dải ảnh.” · *Huỷ lần vẽ* · bản: “Đang vẽ bản 3. Bản 2 vẫn dùng cho video tới khi bản mới xong.” |
| Vẽ xong | nhãn dải ảnh “MỚI” · *So với bản 2* / “Đang xem bản 2” |
| Lỗi | “Chưa vẽ được bản 3” + lý do thật: “ChatGPT đã hết hạn mức ảnh hôm nay (50/50), đặt lại lúc 07:00. Bản 2 vẫn được giữ; prompt bạn sửa vẫn còn.” · *Vẽ bằng Grok · còn 190 ảnh* · *Sửa prompt* |
| Chọn nhiều | “3 ảnh đã chọn” · “Góp ý chung, ví dụ: không có chữ trong ảnh, tông lạnh hơn” · *Bỏ chọn* · **Vẽ lại 3 ảnh · tốn 3 ảnh** · “Mỗi ảnh giữ prompt riêng. Góp ý chung được Hoạ sĩ ghép vào phần Cảnh của từng prompt rồi vẽ lần lượt.” · “Shift + bấm ảnh để thêm, bớt · Esc bỏ chọn” |
| Editor | “Hoạ sĩ đang vẽ bản 2” · “Đang vẽ lại ảnh cảnh 5 · bạn chỉnh tiếp được” · toast “Cảnh 5 đã dùng bản 2 · chuyển động, keyframe giữ nguyên” · *Hoàn tác ⌘Z* · trạng thái “Bản mới · cần duyệt lại” · “Ảnh mới chưa duyệt: Độ hoàn thiện kiểm lại ảnh này trước khi xuất. ⌘Z để quay lại bản 1.” · “4 phần còn lại (Phong cách, Tham chiếu, Ghi chú đã học, Tránh) giữ nguyên · sửa đủ ở Duyệt” |
| Điện thoại | tab Góp ý · Sửa prompt · “Hoạ sĩ sửa phần Cảnh của prompt theo góp ý rồi vẽ; xem lại thay đổi ở Prompt đã gửi.” · *Gửi cho Hoạ sĩ* / **Vẽ lại với prompt này** · sheet Prompt đã gửi: *Chép prompt* · **Sửa prompt rồi vẽ lại** |

Prompt hiện **đúng văn bản đã gửi**, giữ ngôn ngữ gốc (thường là tiếng Anh); không dịch, không rút gọn. Mọi nhãn quanh nó là tiếng Việt.

## Lưu trữ trên máy: đường dẫn và chỗ trống <a id="copy-storage-local"></a> <!-- r34 -->

- **Đường dẫn** (#78): in `displayPath` (`~/.viderse/projects`, `/Volumes/Samsung T7/Viderse Projects`), cắt giữa theo thư mục khi dài — luật ở components › Đường dẫn thư mục. Thư mục mặc định của bản web là `~/.viderse/projects` (dữ liệu: `~/.viderse`), không còn `~/Library/Application Support/Viderse/…`.
- **Chỗ trống** (#79) luôn là của **ổ đang giữ thư mục lưu dự án** và ghi tên ổ:

| Trạng thái | Dòng thư mục (sau tên ổ) | Đầu thẻ dung lượng |
|---|---|---|
| `here` · `pick` · `moving` (chưa xong) | “Ổ trong máy · còn 212 GB” | “Macintosh HD còn trống 212 GB” |
| `done` · `donenew` | “Ổ ngoài USB · APFS · còn 1,54 TB” | “Samsung T7 còn trống 1,54 TB” |
| `offline` (ổ ngoài bị rút) | “Ổ ngoài USB · APFS · chưa cắm” | “Samsung T7 chưa cắm · chưa đọc được chỗ trống” |

- Không lấy số của ổ trong máy thay cho ổ ngoài đang rút: con số đó không nói gì về chỗ lưu dự án. `title` của số: “Chỗ trống của ổ đang giữ thư mục lưu dự án”.
- `offline` **không thêm dòng cảnh báo riêng**: băng cam “Không thấy ổ Samsung T7” đã nói việc và cách xử lý; chỉ đổi số thành “chưa cắm”.
- Điện thoại ghi cùng câu, đầu thẻ: “Macintosh HD còn 212 GB” · “Samsung T7 còn 1,54 TB” · “Samsung T7 chưa cắm”.
- `donenew` (video mới ở ổ mới): số theo ổ mới; dữ liệu cũ ở Macintosh HD nói trong băng xanh, không ghi hai số.

## Nhóm chọn: nhãn <a id="copy-seg"></a> <!-- r34 -->
Giữ nhãn đầy đủ, không rút gọn (#80): Khám phá › Sắp xếp “Nổi trội · Mới nhất · Lượt xem”; Lịch đăng › Vùng giờ “VN · New York”. Nhóm co theo nhãn, hẹp thì cuộn ngang.

## Toast: câu chữ <a id="copy-toast"></a> <!-- r35 -->

- **Viên** một câu, bắt đầu bằng việc đã xảy ra: “Đã xoá 2 video” · “Đã thêm 3 video vào hàng chờ” · “Đã chép đường dẫn” · “Không xoá được video đang xuất”. Không chấm câu cuối. Nút: *Hoàn tác* (kèm giây còn lại “· 4s”), *Thử lại*, *Xem*.
- **Thẻ:** tiêu đề nói **việc cần làm hoặc việc đã xong**, ngắn để vừa một dòng; **tên video nằm ở câu phụ**, không ở tiêu đề.
  - Cần xử lý: “Cần đăng nhập lại ChatGPT” — “The Last Bus Doesn’t Stop at Elm Street dừng ở cảnh 5 · ảnh đã vẽ vẫn giữ.” · *Đăng nhập lại* · *Mở video*.
  - Chờ duyệt (gộp): “3 video chờ duyệt ảnh” — “Midnight Tales 2 · Chuyện Lạ Đêm Khuya 1” · *Duyệt ảnh*.
  - Trợ lý xong: “Hoạ sĩ đã vẽ xong 7 ảnh” — “<tên video> · chờ bạn duyệt” · *Duyệt ảnh* · *Mở video*.
- Bỏ các dòng phụ “Tự đóng sau 5 giây · rê chuột để giữ”, “Giữ tới khi bạn đóng”, “Gộp 3 thông báo trong 10 giây”: thời gian nằm ở vòng quanh nút đóng và `title` của nút (“Tự đóng sau N giây” · “Đã dừng tự đóng” · “Ở lại tới khi bạn đóng”); gộp thể hiện ở con số trong tiêu đề.
- `aria-label` nút đóng: “Đóng thông báo”; vùng chứa: “Thông báo”.

## Thanh trên điện thoại và Thông báo <a id="copy-mtop"></a> <!-- r36 -->

- `aria-label` nút Việc nền: “Việc nền: 2 đang chạy, 12 chờ lượt” · “Việc nền: máy rảnh” · “Việc nền: đã tạm dừng lúc 00:04, 12 chờ lượt” · “Việc nền: mất kết nối máy chủ, đang thử lại”; có chấm thì thêm “, 3 thông báo cần bạn”.
- Avatar: “Tài khoản và cài đặt”. Bộ đổi kênh: “Đổi kênh, đang xem Mọi kênh” / “Đổi kênh, đang chọn Midnight Tales”. Chưa có kênh: nút “Tạo kênh”.
- Sheet Việc nền (điện thoại): tiêu đề “Việc nền”; tab “Đang chạy” · “Thông báo” (số chưa đọc trong huy hiệu vàng). Dòng phụ: tab Đang chạy giữ câu cũ; tab Thông báo “3 chưa đọc” · “Đã đọc hết” · “Chưa có thông báo”.
- Danh sách Thông báo dùng đúng câu của popover máy tính (“3 video chờ duyệt”, “Cần xử lý: <tên video>”, “<tên video> sẵn sàng đăng”…), chưa đọc in đậm + chấm vàng trên icon; *Đánh dấu đã đọc*; cuối danh sách *Cài đặt thông báo*.
- Trống: “Chưa có thông báo” — “Khi video chờ bạn duyệt, cần xử lý hay sẵn sàng đăng, Viderse báo ở đây.” · *Cài đặt thông báo*.

## Màn bổ sung đợt 38 <a id="copy-r38"></a> <!-- r38 -->

**Khám phá — danh sách video** (`Explore`/`MExplore` · `list`)
- Lọc rỗng: “Chưa có video nào khớp bộ lọc” — “Đang lọc dài ≥ 40 phút, đăng trong 7 ngày, chưa làm cho kênh nào — trong 1.284 video đã quét của Nocturne Stories. Nới bộ lọc để thấy thêm.” · nút *Bỏ lọc*. Chip đang khác mặc định có viền vàng (`.vv-chip.is-custom`).
- Quét lần đầu (đối thủ vừa thêm): banner “Trinh sát đang lấy video đầu tiên · 12 / 30” — “Đang đọc 30 video mới nhất của Nocturne Stories để tính mức thường. Video hiện dần ở đây, thường xong trong khoảng 1 phút.” + lưới ô xương. Không hiện “Chưa có video nào khớp bộ lọc” lúc này.
- Lỗi tải: “Chưa tải được danh sách video” — “Máy chủ trả lời quá chậm. 1.284 video đã quét vẫn còn nguyên, chỉ là chưa tải về màn này.” · *Thử lại*.
- Mất kết nối: “Mất kết nối máy chủ” — “Tự thử lại sau 8 giây. Bộ lọc, ô tìm và video đang chọn vẫn giữ nguyên.” · *Thử lại ngay*.
- Chưa theo dõi đối thủ (điện thoại): “Chưa theo dõi đối thủ nào” — câu như máy tính · **Theo dõi đối thủ** · Ví dụ `@nocturnestories`, `youtube.com/@hosobian`.

**Cài đặt › Kết nối AI › Số liệu YouTube** (dùng đúng `performanceCopy.dataKey`)
- Nhóm: “Số liệu YouTube” · “Không phải AI · Trinh sát dùng để đọc số liệu video đã đăng”.
- ~~Thẻ: “YouTube Data API” · nhãn “Khoá API”, “Không bắt buộc”. Dòng phụ: chưa có “Chưa có khoá · số liệu sau đăng đọc từ trang công khai”; đã lưu “Đã lưu ••••x9Qw · Trinh sát đọc lượt xem, thích, bình luận qua API” (xanh); không đọc được “Không đọc được trạng thái khoá” (cam) + *Thử lại*; mất kết nối “Mất kết nối máy chủ — mọi nút của hàng này tắt cho tới khi kết nối lại”.~~ (đã bỏ, đợt 44)
- ~~Ô dán: nhãn “Khoá API”; gợi ý dưới ô “Tạo khoá trong Google Cloud Console rồi bật YouTube Data API v3 cho dự án đó. Không có khoá, Trinh sát vẫn đọc số liệu từ trang công khai của video.” Lỗi: “Google không nhận khoá này — kiểm tra lại khoá và bật YouTube Data API v3.”~~ (đã bỏ, đợt 44)

**Tạo kênh — bước Xong khi Đạo diễn lỗi** (`identity=director`)
- Tiêu đề “Đã tạo kênh”; tên “Kênh mới” · “tên tạm, đặt lại ở Hồ sơ kênh”.
- Thẻ “Một việc cần bạn” — “Phần còn lại đã xong hoặc đang chờ.” Dòng: “Đạo diễn chưa đặt được tên và mô tả kênh: ChatGPT không phản hồi” · *Thử lại*; “Hoạ sĩ vẽ avatar và ảnh bìa · chờ Đạo diễn” · Chờ.
- Chú thích: “Kênh làm video được ngay với tên tạm “Kênh mới”. Bấm Thử lại để Đạo diễn đặt tên, hoặc tự đặt tên và mô tả ở Kênh › Hồ sơ kênh — Hoạ sĩ vẽ avatar, ảnh bìa theo tên đó.” Nút thêm: *Đặt tên ở Hồ sơ kênh*.

**Kênh mới chưa có video (điện thoại)** — “Kênh chưa có video” — “Tự lấy video đang bật: video đối thủ đăng từ giờ sẽ tự vào hàng chờ. Muốn có video ngay thì làm video mới nhất của đối thủ.” Đầu trang: “Chưa liên kết YouTube · tạo hôm nay · 2 đối thủ”.
