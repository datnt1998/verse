# Viderse — Theo dõi gói đã xong

Dev ghi mỗi gói khi xong: ngày, PR/commit, chỗ lệch so với artboard (nếu có) và lý do. Không xoá dòng cũ.

| Gói | Ngày xong | PR / commit | Lệch so với thiết kế | Ghi chú |
|---|---|---|---|---|
| V0 — Nền tảng | | | Khung điện thoại dưới 768px (hỏi đáp #16). Chữ badge Duyệt tạm dùng `--vv-badge-ink` (hỏi đáp #14). | Đang làm. Đã có: thanh trên, nav, `JobsPill`, badge Duyệt và chấm bánh răng đọc `/api/status` qua sự kiện đẩy; chấm bánh răng theo `attention` (đăng nhập hết hạn, công cụ lỗi/có bản mới, hết hạn mức, thiếu thư mục dự án) mở đúng mục Cài đặt; brand, logo, favicon; đứng yên khi giảm chuyển động. Còn: `JobsPill` theo job thật và “Đã tạm dừng” (cần hàng chờ). |
| V1 — Bàn dựng | | | | |
| V2 — Làm video | | `c26e04a6` (backend) | Kế hoạch trả số ảnh theo recipe thật, số lần vẽ gồm dự phòng 10% của yt-factory, giá `null` khi chưa có giá/ảnh; bản trùng trả tên/trạng thái job hoặc mục hàng chờ (#52). | Đang làm. Backend xong: `POST /api/clone/plan` và `/enqueue`, nhiều link, khử trùng cùng video và job/mục đã có, chọn template của kênh, thời điểm bắt đầu và số ngày rải theo nhịp, metadata thật từ video đối thủ hoặc player. Kiểm thử HTTP + PostgreSQL thật cho nhiều link và link trùng đã đạt. UI và đo artboard do gói giao diện chốt. |
| V3 — Duyệt | | | | |
| V4 — Kênh | | | Kênh › Video chỉ có trạng thái `fresh`; bảng video, `BulkBar`, số liệu đầu kênh, “Hôm nay x/y” và Dặn trợ lý khoá kèm lý do tới khi có job (#39). Nhận diện: avatar PNG 800×800, ảnh bìa JPEG 2560×1440 ≤ 6 MB, “Tự thử lại lúc HH:MM” (#42). Bỏ liên kết đối thủ trên máy tính dùng `confirm` của kit (#43). Điện thoại: cột trái ẩn, tab tên ngắn, hàng tab cuộn ngang (#47). | Đang làm. Đã có: 4 tab (`videos|sources|templates|settings`) với số đếm thật; Nguồn & tự động (liên kết/bỏ đối thủ, Tự lấy + nhịp 1–50 chỉ tính video hàng loạt và tự lấy, bỏ đối thủ cuối thì tạm dừng kèm lý do); Template luân phiên (thêm, bỏ, đổi thứ tự, “Tiếp theo”); Hồ sơ kênh tự lưu khi rời ô (ngôn ngữ, giọng văn, đoạn cuối mô tả ≤ 1.000, tag ≤ 10); nhận diện chạy nền (chữ qua `core/llm`, ảnh qua `imagegen`, vẽ lại từng phần, tải về, tiến độ qua SSE ≤ 4 lần/giây, lỗi ghi rõ bước Đạo diễn/Hoạ sĩ do máy chủ trả); Lịch đăng của đối thủ ở cột phải; Bộ đổi kênh chỉ kênh thật; Khám phá mở chọn kênh và “Liên kết với kênh…”; CLI `viderse channel`. Smoke thật 31/10/2026: kênh 2 đối thủ, Đạo diễn đặt tên “Insanity Inside”, 2 ảnh trả phí. Còn: mức tự chủ; bảng video và số liệu (cần job); Dặn trợ lý (V11). |
| V5 — Khám phá + Video đối thủ | | | `MExplore` dời sang V10; trong lúc chờ, dưới 768px là bản một cột theo #37 (không cuộn ngang, ô thêm đối thủ trên cùng, cột phải xuống dưới lưới, nút ≥ 44px khi `pointer: coarse`). Menu ⋯ chưa có kênh: “Liên kết với kênh…” khoá với lý do #31 thay dòng “Đang liên kết…” của artboard. | Đang làm. Đã có: theo dõi đối thủ bằng @handle, link hoặc mã UC (đọc trang Videos đầu, tính mức thường); quét lịch sử chạy nền, giới hạn 150–10.000, tạm dừng/tiếp tục/huỷ, sống qua khởi động lại; kiểm tra video mới theo `clone.rescanHours` và nút “Kiểm tra video mới”; điểm nổi trội = lượt xem / mức thường; lưới ảo hoá (20.000 video, ≤ 200 thẻ), lọc dài/đăng trong, 3 kiểu sắp, phân trang cursor; xem trước + Vì sao; chọn nhiều; Lịch đăng §12 (khung lặp lại, giờ VN/New York, bảng thứ × giờ, “Mỗi tuần” theo độ dài mẫu); CLI `viderse rival`. QA ảnh cặp 3 vòng xong 30/10/2026. Đợt 28 (#31–#37) xong 30/10/2026: điều khiển cần kênh/hàng chờ khoá bằng `aria-disabled` + `aria-describedby` tới lý do (nút thẻ, nút chính và ô chọn kênh ở xem trước, chip Trạng thái, “Làm N video”, mục menu); câu quét/lỗi thêm/Lịch đăng theo 06-copy; menu ⋯ dùng `ActionMenu` kit (đệm 6/10, khe 0); ô “Tìm tiêu đề” 240px (`q`, không phân biệt hoa thường/dấu, cộng bộ lọc, phím “/”, nút xoá, dòng đếm, khối không có kết quả + “Xoá tìm”); ví dụ `@nocturnestories` · `youtube.com/@hosobian` · `UCx9…Qw`; thẻ chỉ hiện trạng thái hàng chờ hoặc nút làm. Có kênh (31/10/2026): xem trước chọn được kênh, menu ⋯ “Liên kết với kênh…” mở hộp chọn kênh thật. Còn: mở các điều khiển cần Hàng chờ (V6); `MExplore` (V10). |
| V6 — Hàng chờ | | `c26e04a6` (backend) | Mọi video vào một hàng chung; `pick` đứng trước `batch`, Lên đầu chuyển thành `pick`; nhịp chỉ tính mục `batch` đã bắt đầu và reset 00:00 giờ máy. `paused_channel` chỉ khoá mục tự lấy khi tắt Tự lấy. Lý do chờ, vị trí thực và timestamp tạm dừng theo #52. | Đang làm. Backend xong: lane số thực theo kênh, luân phiên công bằng, ETA 7 ngày có nhịp + luồng + export tuần tự, Tự lấy có baseline/rescan/khử trùng/luân phiên đối thủ, giới hạn tài nguyên có huỷ, `GET`/`PATCH /api/queue`, chỉnh luật chạy tại Hàng chờ. Kiểm thử ETA/nhịp/công bằng/luật chạy + HTTP/PostgreSQL đã đạt. UI và đo artboard do gói giao diện chốt. |
| V7 — Cài đặt | | | Thẻ tổng không có số luồng (#18); chưa có hàng “Giới hạn từng kênh” (#19); thẻ phiên bản không có ngày phát hành (#20); dải mất kết nối đẩy nội dung, tông chấm theo `kind`, nhãn pha cài do máy chủ trả (#30). | Đang làm. Đã có: khung Cài đặt (cột trái 3 nhóm, chấm cảnh báo theo mục) và `MSettings`; Công cụ (`tools=ok|update|failed|installing`, cài/cập nhật/tải tiếp, Nâng cao, kích thước đo sau khi cài); Nhật ký (lọc, lỗi 7 ngày, xuất log đã che bí mật, kiểm tra bản mới). QA ảnh cặp 5 vòng xong 30/10/2026, còn 0 lỗi lớn / 0 lỗi nhỏ. Còn: hàng “Giới hạn từng kênh” khi phase Kênh có route (#19). |
| V8 — Thư viện + Template | | | Nút dựng lại xem thử và Vẽ thử khoá tới khi có video đã làm xong (#38). Giá mỗi video theo `price.perImage` × số ảnh (#41). Không có nhập template, thẻ không có ảnh thu nhỏ (#46). Đổi tên template không làm cũ bản xem thử (#48). | Đang làm. Đã có: Thư viện 4 tab (`templates|prompts|backdrops|overlays`) với số đếm, avatar kênh thật; template tự lưu theo `revision` (lệch phiên bản trả 409), Đổi tên/Nhân bản/Thêm vào kênh/Xoá (đang dùng thì khoá kèm tên kênh); Cài đặt template 4 nhóm (`images|cover|backdrop|output`) tự lưu khi rời ô, sức khoẻ, giá ước tính; lớp phủ (12 có sẵn, tải lên, xoá); CLI `viderse template`, `viderse overlay`. QA ảnh cặp xong 31/10/2026. Còn: dựng lại xem thử và vẽ thử bìa (cần video đã làm, V6); bố cục editor (V13). |
| V9 — Lần đầu mở + Tạo kênh | | | Chuẩn bị máy là route `#/setup` sau đăng nhập (#17). Mẫu chuẩn chỉ là recipe, tên “Mẫu chuẩn · <tên kênh>”, đặt lại khi kênh có tên (#40). Onboard điện thoại dùng bản một cột (#47). | Đang làm. Đã có: bước `step=setup` (`running|login|failed`) cài thật 6 việc, tiến độ qua SSE, đăng nhập ChatGPT, kiểm tra dung lượng; điện thoại hiện “Mở trên máy tính”. Tạo kênh `step=create|done`: bắt buộc ≥ 1 đối thủ (thêm ngay trong luồng), để trống tên thì Đạo diễn đặt; màn Xong hiện Đạo diễn · Hoạ sĩ · Kiểm định theo tiến độ thật và bước lỗi. Còn: `MCreate` (V10). |
| V10 — Điện thoại | | | | |
| V11 — Nhắn trợ lý | | | | |
| V12 — Hộp thoại + trạng thái hệ thống | | | Dải mất kết nối trong Cài đặt đẩy nội dung xuống thay vì che (#30). | Đang làm. Đã có: 7 loại `ConfirmDialog` (khoá kèm lý do, cỡ `size` cho hộp thoại rộng), dải mất kết nối (hiện ~0,1 giây sau khi tắt máy chủ, nội dung khoá `inert`, tự tắt khi nối lại), trang Không tìm thấy, Bàn dựng trống. Tắt máy chủ khi còn tab mở thoát ngay, mã 0. Còn: khoá Xoá video đang chạy trên menu ⋯ (cần Bàn dựng có job). |
| Gói V13 — Editor theo Viderse | Runtime đã đối chiếu; chờ gate tích hợp | Nhánh `feat/viderse-p06` | Dữ liệu thật của fixture phase 05, không cố định điểm, thời lượng hay số đoạn theo video minh hoạ. Các khác biệt dữ liệu/copy được ghi chung tại #66 trong `11-hoi-dap.md`. | Đã port engine/UI MIT `2c201e6`, tách project TypeScript, lazy hash routes editor/render/measure và kiểm tra ngân sách entry theo manifest. Đã đo 21 trạng thái desktop và nhánh hẹp 1000px bằng serve 8823/Vite 5323 thật; bảng bên dưới. Đã chạy thật mở lỗi, khớp/⌘Z, khớp tất cả → hai mục cần đo lại, ba hành động xung đột, xuất có xác nhận và xuất khi đủ điều kiện. Bản cuối chờ nén xong mới báo hoàn tất; kích thước UI và MP4 tải về cùng 573436 byte. Test hẹp đã xanh; typecheck/engine suite/budget và design-rules sau ghép các gói phase 05 do Main chạy một lần. | <!-- r2-gap -->
| V14 — Việc nền & thông báo | | | | |
| V15 — Kho prompt ảnh | | | Nhập prompt trùng tên (#45). | Đang làm. Đã có: 7 prompt có sẵn (không Lưu/Xoá), prompt của bạn: tạo, sửa, nhân bản, xoá (đang dùng thì chặn kèm tên template), lọc theo loại, tìm, xuất/nhập JSON; template chọn prompt ảnh cảnh và bìa, `imagePromptMode` tự suy ra. Còn: video đang chạy giữ nội dung cũ, video mới dùng nội dung mới, ảnh tách nền và bìa “Chữ hook” trong video (cần pipeline, V6); Ảnh mẫu/Vẽ thử. |
| V17 — Kết nối AI và Trợ lý AI | | | Dữ liệu artboard chưa có nguồn (#26); thứ tự thẻ, thẻ trùng loại (#27); giá mỗi video = 7 ảnh, lưới 4 nguồn vẽ (#28). | Đang làm. Đã có: §15 đủ API; ChatGPT (một credential trong `core/auth`), Grok mã thiết bị, khoá API niêm phong AES-GCM, Ollama; danh mục model phân trang, thêm model tay; Đạo diễn/Hoạ sĩ chọn model và nguồn vẽ dự phòng; trần ảnh trả phí mỗi ngày (D7). QA ảnh cặp 5 vòng xong 30/10/2026, còn 0 lỗi lớn / 0 lỗi nhỏ. Còn: chủ máy thử đăng nhập Grok và thêm khoá API thật. |
| V22 — Thư mục lưu dự án | 30/10/2026 | `00fd26e` | Chọn thư mục bằng danh sách của máy chủ (#21); “Ổ ngoài USB” theo `diskutil` (#22); banner đếm việc nền, không đếm video (#23). | Xong. §23 (`here|pick|lowspace|cloud|moving|done|donenew|offline`), chuyển thư mục chạy nền có tiến độ và thử lại, phát hiện ổ mất/chỉ đọc, dọn theo chính sách giữ có xem trước. QA ảnh cặp 5 vòng, còn 0 lỗi lớn / 0 lỗi nhỏ. |
| V23 — Video nền và Tài khoản nền tảng | | | Mốc “từ HH:MM” và “một số video” (#25); state điện thoại chưa có artboard, giãn dòng 1,5 (#29). Thêm kênh nền chạy nền, dán link video không dò danh sách (#44). | Đang làm. Đã có: phần Tài khoản nền tảng (YouTube cookie từ trình duyệt hoặc tệp, Bilibili, Twitch, hồ sơ, trạng thái tạm chặn từ `platform_state`). Thư viện › Video nền: dán link (kênh, video lẻ kèm đoạn ≤ 600 giây, nền tảng chưa hỗ trợ lưu yêu cầu trên máy, gửi kèm Báo lỗi); danh mục Twitch và Bilibili (ký WBI), điều tốc theo nền tảng, lưới ảo hoá keyset; ảnh thu nhỏ Twitch cỡ 640×360; Template › Nền & hiệu ứng (Ngẫu nhiên/Luôn có, thanh trộn, cắt clip, chỉnh nền, lớp phủ); CLI `viderse backdrop`. Smoke thật: kênh Twitch 212 video. Còn: lấy clip khi làm video (V6). |
| V24 — Đăng nhập Bilibili bằng mã QR | | | Mở app 哔哩哔哩 trên chính điện thoại (#6). | Đang làm. Đã có: `qr=loading|waiting|scanned|expired|rejected|error|success` trên máy tính và điện thoại, API trả cả `loginUrl` (#7), poll 2 giây, tự gia hạn cookie. Còn: quét thật bằng tài khoản Bilibili. |
| V27 — Lấp chỗ thiếu artboard | 01/11/2026 | `8cf29f38`, `ac1d3876`, `2f7e860a`, `a3a8f801`, `6b54e374`, `9595c703`, `2d68154c`, `df3c54a4` | Hoàn tác bỏ khỏi hàng chờ dùng `PATCH /api/queue { action: "restore" }` trên chính mục cũ, mục bỏ giữ 24 giờ (#64). Nút đang xử lý giữ spinner chồng của nút dùng chung, lệch ~3px bề ngang (#65). | Xong. Viên việc nền 4 trạng thái (Đang chạy · Máy rảnh · Đã tạm dừng lúc · Mất kết nối sau 2 lần `/api/status` lỗi hoặc SSE đứt >10 giây); `MQueue` thay Dialog tạm, Quy tắc chạy bottom sheet (luồng 1–6, ảnh 1–8, đối thủ 1–24 giờ); Ưu tiên cả N ở `MNew`; bỏ Không cần ảnh ở Video mới cả hai bản (`noImages` chỉ còn ở CLI); token `--vv-red-button-fill`, `--vv-accent-soft`, `--vv-info`, `--vv-surface-3`; Tài khoản điện thoại › Thiết bị (đăng xuất từng máy, mọi máy khác có xác nhận), mật khẩu tạm có Đăng xuất, lỗi mạng/chép mã/không có mã khôi phục, `kicked=none` theo `vv.signedInBefore`; `GET /api/me/sessions` trả `{ id, device, browser, lastActiveAt, expiresAt, current }`. Đo board-vs-app ở 1440×900 và 390×844. |

## Backend tạo video và hàng chờ — 31/10/2026

- Luồng máy chủ đăng ký đủ `download → backdrop → transcript → metadata → scenes → images → build`, lịch hàng chờ, quét đối thủ, hạn mức ảnh và semaphore thật. Trạng thái tổng hợp đếm cả job clone; ảnh và bản dựng dùng media publication có fence.
- Project editor schema 15 giữ JSON chuẩn của template cùng binding/revision/media digest; recipe chỉ là phép chiếu cài đặt. Build kiểm manifest hoàn tất, cảnh/tham chiếu/nội dung còn khớp và metadata media đủ trường engine. Bản nháp dùng ứng viên hợp lệ nhưng không tự duyệt ảnh (#54).
- Phiên ảnh dùng journal PostgreSQL: khôi phục lần vẽ đã tính phí trước khi mã hoá, lần vẽ lại có receipt mới, chặn người chờ ngay trong callback khi nguồn lỗi. D7 hết hạn đỗ cùng `retryAt`; `VIDERSE_DAILY_PAID_IMAGE_LIMIT=1..50` chỉ hạ trần mặc định 50 để chạy máy hoặc kiểm tra có giới hạn.
- Khởi động lại nhận diện namespace `clone` và tài nguyên template đi kèm; migration `00014_builtin_media_roots.sql` giữ đúng bốn asset chuẩn khỏi GC. Job bị ngắt còn lease cũ hết hạn được mở lại bước với checkpoint nguyên vẹn, không phát lại bước đã xong.
- Smoke mạng thật với video YouTube 19 giây `jNQXAC9IVRw`: enqueue HTTP, tải video, xử lý nền và chép lời thật. Chưa xác nhận một ảnh trả phí hay trạng thái `awaiting_review` trong môi trường cô lập khi hai nguồn ảnh đều chưa đăng nhập; không đánh dấu toàn bộ gói đã xong vì thiếu chứng cứ ngoài này.

## Đối chiếu Video mới — 31/10/2026

- Đo lại `NewVideo` đủ năm trạng thái và `MNew` đủ ba trạng thái bằng hình học DOM sau khi font và React ổn định, với kế hoạch và metadata thật từ máy chủ cô lập.
- Sửa ô nhiều link bị ẩn bởi selector phần tử đầu; khớp thẻ gợi ý, nhóm trợ lý, thẻ lối đi khác, màu nút khoá, dải lỗi/trùng và nhãn chế độ điện thoại với artboard. Không thêm màu hoặc nâng ratchet.
- Giữ chuẩn hoá safe-area #57, hàng Không cần ảnh #58 và ghi rõ hàng ưu tiên nhiều link điện thoại ở #61. Thiếu ba trạng thái viên việc nền của Main ghi #60; không tự vẽ thay design.

## Đối chiếu Hàng chờ và khung điện thoại — 31/10/2026

- Dùng PATCH thật để chỉnh 2 luồng video, 3 luồng ảnh, quét lại mỗi giờ và dấu giờ tạm dừng; xếp video YouTube thật vào các kênh thật của DB đo riêng. Đo được cả thumbnail đang tải, hai video chạy và hàng luân phiên giữ nguyên khi tạm dừng.
- Khớp chip đếm luồng, đoạn tiến độ 4px, lưới cột chờ lượt, stepper, nút dừng/chạy tiếp và viền cột quy tắc. Trạng thái hết hạn mức chỉ dùng DTO Go thật có sửa số hạn mức trong phiên trình duyệt: hai provider thật chưa đăng nhập, không có API hợp lệ để dùng hết hạn mức.
- Sheet Hàng chờ nằm trên cả header/thanh dưới, có Bàn dựng thật phía sau, giữ hàng 52px và thẻ đang làm 60,25px. Khung điện thoại không giấu số việc nền ở 390px; năm tab chia đều, avatar mở Cài đặt và phần dính đáy chuẩn hoá theo #57. Khác biệt cầu đỏ ghi #62.
- Kiểm tra: TypeScript thành công; QueueScreen và shell 22 kiểm thử thành công. Kiểm thử offline cũ được đổi sang sự kiện mạng thật của hook, thay giả định SSE chưa chạy đồng nghĩa offline.

## Đối chiếu Khám phá và chốt Video mới điện thoại

- Khớp cột preview 316px, phần nội dung 283px, nút quay lại 30px, thumbnail đúng 16:9, hàng kênh 53,95px với avatar/template thật, nút Làm video hai dòng 46px và liên kết YouTube 34px. Phụ đề tối trên nút sáng đã được duyệt ở #63.
- Thanh chọn nhiều cao 54px, nút chọn nhanh 26px, nút hành động 36px. Chế độ chọn nhiều đóng preview như board; mở thumbnail thoát chế độ chọn nhiều và mở lại preview.
- `MNew` rỗng không hiện nhóm tuỳ chọn hoặc dòng kế hoạch; footer còn 63px. Gợi ý và ô nhập khớp sau chuẩn hoá native safe-area #57. Không ẩn textarea nhiều link.
- Kiểm tra cuối: `npx tsc --noEmit -p .` thành công; năm file Vitest tập trung chạy với `--pool=threads --maxWorkers=2`, 43 kiểm thử thành công. Cảnh báo môi trường React `act(...)` đã có vẫn còn, không bị tắt.
- Bằng chứng hình học/kiểu tính được và DTO hạn mức nằm ở `/tmp/p05visual-measurements.json`, `/tmp/p05visual-limited.json`; bảng từng trạng thái được ghi trong mục P05VisualQA của báo cáo bàn giao.

## Gói V13 — bảng đối chiếu runtime Editor

Đo DOM và `getComputedStyle` bằng một session agent-browser riêng, dùng Viderse serve thật tại 8823 và Vite tại 5323. Dự án schema 15 được build bởi phase 05, media thật được nhập qua API/domain. Fixture sáu giây dùng âm sin; mốc lời độc lập là **authored golden, không phải kết quả ASR thật**. Không gán điểm/range giả vào app, không seed nội dung trực tiếp vào DB. Browser/Vite đã đóng; fixture và serve đã được owner thu dọn; tài khoản thử đã xoá.

### Khung chung tại 1440 × 900

| Thành phần | Board | App |
|---|---:|---:|
| Thanh trên | 1440 × 52 | 1440 × 52 |
| Thanh trái có nhãn | 68 × 562 | 68 × 562 |
| Nút trên thanh trái | 56 × 54 | 56 × 54 |
| Bảng trái | 316 × 562 | 316 × 562 |
| Vùng xem | 740 × 562 | 740 × 562 |
| Bảng phải | 316 × 562 | 316 × 562 |
| Timeline | 1440 × 286 | 1440 × 286 |
| Toolbar / ruler / một track | 40 / 24 / 30 | 40 / 24 / 30 |

Các vị trí chung: bảng trái `(68,52)`, bảng phải `(1124,52)`, timeline `(0,614)`. Font UI là Be Vietnam Pro; số thời gian dùng font mono. Border của rail/bảng trái chiếm đúng một pixel, không thay bằng inset shadow.

### Cặp đo cho đủ 21 trạng thái

Giá trị là `rộng × cao`, đơn vị CSS px. Với fix/panel đo bảng trái; focus và nâng cao đo bảng phải; safe-off đo vùng xem; các trạng thái xuất/xung đột đo dialog.

| Trạng thái | Board | App |
|---|---:|---:|
| `fix:todo` | 316 × 562 | 316 × 562 |
| `fix:partial` | 316 × 562 | 316 × 562 |
| `fix:done` | 316 × 562 | 316 × 562 |
| `fix:measuring` | 316 × 562 | 316 × 562 |
| `fix:passed` | 316 × 562 | 316 × 562 |
| `panel:fix` | 316 × 562 | 316 × 562 |
| `panel:scenes` | 316 × 562 | 316 × 562 |
| `panel:subs` | 316 × 562 | 316 × 562 |
| `panel:bg` | 316 × 562 | 316 × 562 |
| `panel:audio` | 316 × 562 | 316 × 562 |
| `panel:media` | 316 × 562 | 316 × 562 |
| `focus:caption` | 316 × 562 | 316 × 562 |
| `focus:scene` | 316 × 562 | 316 × 562 |
| `focus:bg` | 316 × 562 | 316 × 562 |
| `focus:none` | 316 × 562 | 316 × 562 |
| `state:export-gate` | 460 × 263.359375 | 460 × 238.984375 |
| `state:exporting` | 520 × 342 | 520 × 322.625 |
| `state:export-done` | 520 × 479.625 | 520 × 479.625 |
| `state:conflict` | 520 × 344.890625 | 520 × 324.09375 |
| `tweak:advanced` | 316 × 562 | 316 × 562 |
| `tweak:safe-off` | 740 × 562 | 740 × 562 |

Không ép chiều cao dialog bằng khoảng trống giả: gate đo với điểm thật 100 và trạng thái cần đo lại, không phải ví dụ 84 với ba range; exporting bỏ đúng một dòng 19.375px theo quyết định copy của Main; conflict dùng mô tả 409 thật thay vì bịa tác nhân, giờ và số thay đổi, ít hơn một dòng 20.796875px. Các quyết định này nằm trong mục hỏi đáp chung #66, không mở mục hỏi đáp V13 thứ hai.

| Chi tiết kiểm tra | Board | App |
|---|---|---|
| Dòng range | rộng 265, cao 40, padding 5px 8px, gap 8, radius 8 | cùng các giá trị |
| Khớp tất cả | cao 30, padding 0 10px, gap 7, radius 8 | cùng các giá trị; rộng 211.063px với N=2 so với 211.203px với N=3 |
| Gate: icon / gap đầu | 42 × 42 / 14 | 42 × 42 / 14 |
| Gate: mô tả | 13.5px, line-height 20.925px, màu `rgb(180,180,190)` | cùng các giá trị |
| Gate: list | padding 10px 12px 10px 30px, radius 11, nền trắng alpha .03 | cùng các giá trị |
| Nội dung dialog | padding 20, gap 16, font 13 | cùng các giá trị |
| Toast | nền `rgb(28,28,31)`, cao 40, padding 0 8px 0 14px, gap 10, radius 999 | cùng các giá trị, dùng `--kit-surface-toast`, không dùng `--color-surface-1` |

### Nhánh hẹp tại 1000 × 900

Nhánh này thay Editor bằng thông báo màn hình hẹp; không giả lập 21 panel bị ẩn thành 21 bề mặt hẹp khác nhau.

| Thành phần | Board | App |
|---|---:|---:|
| Header | 1000 × 48 | 1000 × 48 |
| Khối thông báo | 420 × 200.28125 | 420 × 200.28125 |
| Nút quay lại trong header | cao 44, radius 8, padding 0 10px 0 6px, gap 6 | cùng các giá trị |
| Nút chính | 153.0625 × 44, radius 11 | 153.0625 × 44, radius 11 |
| Chữ nút chính | 13.5px / 400 | 13.5px / 400 |
| Vị trí thông báo / nút chính | y=373.859375 / 530.140625 | cùng các giá trị |

### Chứng cứ từng tiêu chí V13

| Tiêu chí | Kết quả thực tế |
|---|---|
| Mở từ Chưa đạt | Hash `?fix=1` mở Cần sửa, chọn cue đầu `Xin chào`, playhead frame 12 tại 30fps; chip timeline ghi `Đoạn lệch 1/2`. |
| Khớp theo giọng + ⌘Z | POST thật chỉ gửi `cueIds:['c1']`; response offset `-0.4`; cue/word được ghi về server. ⌘Z khôi phục mốc `.4 → 2.9`, bộ đếm trở lại hai đoạn; redo hoạt động. |
| Khớp cả N đoạn | Hai range authored độc lập, không chồng nhau; khớp tất cả chuyển sang `Đã khớp cả 2 đoạn`, đúng hai check `captionAlignment` và `captionKaraoke` cần đo lại, chín check còn lại không bị đánh stale giả. |
| Xuất ở 84 | Test rendered `ExportGateDialog` kiểm tra 84/100 bị chặn. Runtime không sửa điểm 100 thật thành 84: lỗi timing có weight 0 theo upstream nhưng vẫn là blocker; export thật mở gate và cần xác nhận. |
| Thanh trái với screen reader | Sáu tab có tên, `aria-selected`, `aria-controls`, roving tabindex; ArrowDown, Home, End đổi focus/panel đúng. Axe phạm vi tablist: 19 rule passed, 0 violation. |
| Ba cách giải quyết xung đột | 409 được tạo bằng hai lần ghi CAS thật. Tiếp tục giữ local và tắt tự lưu, revision server không đổi; tải lại bỏ local; lưu bản sao giữ local trong project mới, không ghi đè bản gốc. Copy thử đã xoá bằng API. |
| Đo và xuất thật | Mốc authored golden được đo bằng pipeline thật; đã sửa end-card sáu giây bằng ba item canonical của template qua engine commands, không đổi template hoặc score. Kết quả cuối 100, passed=true, blockers=[], changed=false. |
| Hoàn tất xuất | Poll quan sát `queued → running/claim → running/rendering → running/compressing → done`; không báo xong ở bước upload. Request `8140febf-2520-4e92-867f-0a77bc191668`, MP4 `2a3cc4fb-e307-4265-bf6d-1beaf810ed62`: HTTP 200, UI và tệp cùng **573436 byte**. Owner probe: sáu giây, 1920×1080, H264 30fps, AAC 48kHz stereo, −14 LUFS; pipeline SUCCEEDED. |

### Kiểm thử và phạm vi giữ nguyên upstream

- Các lần chạy hẹp cuối: `quality.test.ts` 7 đạt; `ExportGateDialog.test.tsx` 3 đạt; `ExportDialogs.test.tsx` 5 đạt; `ui/library/library-actions.test.tsx` 12 đạt. Có fail-trước cho thiếu bước xuất thứ ba, nhãn giai đoạn nén, và hai nhánh chèn video không có audio. Video-only không tạo linked-audio và được đánh `embeddedAudioMuted`; audio AAC vẫn giữ nhánh audio thật.
- Route/navigation đã có chín regression fail-trước rồi xanh; headless client đã được chạy với receiver của native fetch và snapshot subtitle thật. Không chạy lại toàn bộ app/engine để tránh tranh CPU với các owner.
- Lượt typecheck app cuối của worker phát hiện token toast không tồn tại; đã thay bằng token toast đúng và đo màu so với board. Không chạy lại tsc sau sửa; Main sở hữu lượt typecheck/gate tích hợp cuối.
- Lượt `design-rules.test.ts` cuối: 10 đạt, 4 lỗi chỉ ở CSS phase 05 (`screens/explore`, `screens/new`, `screens/queue`, gồm `--vv-row-sep`); **không có offender `editor/`**. Không sửa queue ngoài phạm vi được giao. Main ghép các gói StyleX phase 05 trước gate tổng.
- `engine/**` và `vendor/**` giữ nguồn MIT. Các miễn trừ UI là danh sách file cụ thể trong test, không miễn glob `ui/**`: `controls/index.tsx` (control cũ không dùng cho V13); `inspector/canvas.tsx` (bảng canvas cũ); `inspector/text.tsx` (rich text cũ); `inspector/effect-preview.ts` (pixel ảnh mẫu); `library/library-actions.ts` (màu pixel nội dung được tạo); `preview/use-preview-composition.ts` (màu nền nội dung render); `timeline/timeline.css` (CSS NLE upstream). V13 mới dùng StyleX; timeline chrome và track tokens V13 không miễn kiểm tra.

Bằng chứng số liệu gốc: `/tmp/p06-editor-board-1440-final.json`, `/tmp/p06-editor-app-states.json`, `/tmp/p06-editor-{board,app}-1000-final.json`, `/tmp/p06-editor-{board,app}-toast.json`, `/tmp/p06-editor-gate-controls-final.json`, `/tmp/p06-editor-actual-alignment-diff.json`, `/tmp/p06-editor-cmd-z-final.json`, `/tmp/p06-editor-all-aligned-final.json`, `/tmp/p06-editor-conflict-{continue,reload,copy}.json`, `/tmp/p06-editor-finalization-polls.json`, `/tmp/p06-editor-final-export-evidence.json`. MP4 cuối được giữ tại `/tmp/p06-render-smoke-finalized.mp4`.

