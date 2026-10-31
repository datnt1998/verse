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
| | http bật | “Đang bật: chỉ an toàn khi Viderse và kho nằm cùng một máy hoặc cùng mạng nội bộ, không đi qua internet.” | — |
| | địa chỉ cho trình duyệt trống | “Để trống thì trình duyệt không tải được tệp, xem video sẽ lỗi. Không có tên miền thì chọn “Qua máy chủ Viderse”.” | — |
| Thanh lưu | idle · run · ok · fail · nokey | “Kiểm tra kết nối đạt rồi mới lưu được” · “Đang kiểm tra kết nối…” · “Kết nối đạt · lưu rồi xác nhận chuyển 412 tệp · 18,6 GB sang Garage” · “Kết nối chưa đạt · sửa rồi kiểm tra lại” · “Máy chủ chưa có khoá mã hoá nên chưa lưu được” | Huỷ · Lưu… |
| Xác nhận | — | “Chuyển 412 tệp · 18,6 GB sang Garage?” · “Từ ổ đĩa máy chủ · kết nối đã kiểm tra đạt · theo dõi trong Việc nền” | Huỷ · Bắt đầu chuyển |
| Đang chuyển | `running` · `paused` | Nhãn “Đang chạy” / “Tạm dừng”. “Bắt đầu lúc 21:06. Video đang làm vẫn chạy, Bàn dựng và Duyệt vẫn sửa được. Tệp mới vẫn ghi vào ổ máy chủ cho tới khi chuyển xong, rồi được chép nốt. Ổ máy chủ không bị xoá.” · “42 MB/giây · còn khoảng 4 phút” | Tạm dừng / Tiếp tục · Huỷ chuyển |
| | hỏi huỷ | “**Huỷ chuyển?** Viderse vẫn dùng ổ máy chủ, không mất tệp nào. 340 tệp đã chép sang Garage vẫn nằm trong kho viderse-media, bạn tự xoá nếu muốn.” | Tiếp tục chuyển · Huỷ chuyển |
| | `error` | Tiêu đề “Chuyển sang Garage chưa xong”, nhãn “3 tệp lỗi”. “Còn tệp lỗi thì chưa đổi kho. Viderse vẫn dùng ổ máy chủ, không mất gì.” | Thử lại 3 tệp lỗi |
| Chỗ cũ | `previous` | “Chỗ cũ: ổ đĩa máy chủ vẫn giữ 18,6 GB” · “Viderse đã thôi đọc từ ổ máy chủ. Khi đã yên tâm (mở vài project trong Bàn dựng, xem vài bản xuất), người quản trị máy chủ xoá thư mục cũ để lấy lại chỗ. Viderse không tự xoá.” | Chi tiết cho người quản trị máy chủ |
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
