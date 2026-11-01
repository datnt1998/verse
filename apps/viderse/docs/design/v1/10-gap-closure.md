# Viderse — Theo dõi gói đã xong

Dev ghi mỗi gói khi xong: ngày, PR/commit, chỗ lệch so với artboard (nếu có) và lý do. Không xoá dòng cũ.

| Gói | Ngày xong | PR / commit | Lệch so với thiết kế | Ghi chú |
|---|---|---|---|---|
| V0 — Nền tảng | | | Khung điện thoại dưới 768px (hỏi đáp #16). Chữ badge Duyệt tạm dùng `--vv-badge-ink` (hỏi đáp #14). | Đang làm. Đã có: thanh trên, nav, `JobsPill`, badge Duyệt và chấm bánh răng đọc `/api/status` qua sự kiện đẩy; chấm bánh răng theo `attention` (đăng nhập hết hạn, công cụ lỗi/có bản mới, hết hạn mức, thiếu thư mục dự án) mở đúng mục Cài đặt; brand, logo, favicon; đứng yên khi giảm chuyển động. Còn: `JobsPill` theo job thật và “Đã tạm dừng” (cần hàng chờ). |
| V1 — Bàn dựng | Nghiệm thu đã đối chiếu; chờ gate tích hợp | `55587e05`, `09d2acf1`, `81029268`, `f482e336`, `f912ade5`, `84e733c4`, `af68df07`, `4e8cc383`, `f6c9cac4`, `a58b1dbf`, `a9c2115e`, `ee5da46d` | Trạng thái tải/rỗng/lỗi/mất kết nối của Bàn dựng, bảng phải và Bản tin dùng V12 (#69, #71, #72). Bật lại ảnh khi template «Không vẽ ảnh» dùng một ảnh cảnh, chờ design (#73). «Làm lại từ đầu» vào hàng chờ ở đầu, giữ lượt cũ đọc được qua «⋯ › Lượt trước». Đề xuất trợ lý, thẻ hoàn tác Bản tin thuộc phase 08. | Đo 1440×900 với serve thật: 42 tổ hợp phạm vi × bước × menu prompt khớp khung và hộp thoại artboard; bảng Nguồn/Nền/Ảnh/Metadata 0px so với board. V16 đổi prompt ảnh cảnh (vẽ lại ảnh đã vẽ, phần còn lại theo prompt mới), «Không tạo ảnh», chỉ đổi bìa; menu ⋯ theo bước; làm lại từ đầu có xem trước template, số ảnh, giờ bắt đầu; mục hàng chờ được nhận thì màn theo sang job mới. Còn: giá ảnh đang vẽ trả phí chỉ kiểm bằng test máy chủ. |
| V2 — Làm video | | `c26e04a6` (backend) | Kế hoạch trả số ảnh theo recipe thật, số lần vẽ gồm dự phòng 10% của yt-factory, giá `null` khi chưa có giá/ảnh; bản trùng trả tên/trạng thái job hoặc mục hàng chờ (#52). | Đang làm. Backend xong: `POST /api/clone/plan` và `/enqueue`, nhiều link, khử trùng cùng video và job/mục đã có, chọn template của kênh, thời điểm bắt đầu và số ngày rải theo nhịp, metadata thật từ video đối thủ hoặc player. Kiểm thử HTTP + PostgreSQL thật cho nhiều link và link trùng đã đạt. UI và đo artboard do gói giao diện chốt. |
| Gói V3 — Duyệt | Nghiệm thu thao tác đã đối chiếu; chờ gate tích hợp | `774ee394`, `402cfd7a`, `cb3ae25d`, `26d673cf`, `2e40b6b4` | Trạng thái tải/lỗi/mất kết nối dùng V12 tại vùng nội dung (#68); học sở thích (`learn`/`learnSaved`) thuộc V19 và điểm ảnh (`score`) cùng thiết lập mức duyệt thuộc V20, phase 08. Dữ liệu và số video theo máy chủ, không cố định theo video minh hoạ. | Đã đo trên trình duyệt 1440×900 với serve và artifact ảnh đã công bố thật: J/K, ←/→, R/A; không chạy phím khi gõ; ảnh đang vẽ khoá duyệt kèm lý do, ảnh khác nhận lệnh vẽ lại HTTP 202 và trạng thái vẫn còn sau tải lại; duyệt video cuối hiện “Đã duyệt hết”, ẩn badge. Hero, dải ảnh và composer khớp toạ độ/kích thước artboard sau sửa đường viền và line-height. Test hẹp phím, khoá và hoàn tác lạc quan đã chạy. Còn đối chiếu pixel đầy đủ mọi state/view và tích hợp kiểm thử cuối; fixture chạy đồng thời dùng lease/ảnh đang vẽ được chuẩn bị trong DB, không gọi nhà cung cấp ảnh trả phí. |
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
| V14 — Việc nền & thông báo | Nghiệm thu đã đối chiếu; chờ gate tích hợp | `e935953a`, `96688f0c`, `283724da`, `5ecedffe`, `50fc9916`, `a0110417`, `7181809e`, `5f186587`, `50887543` | Trạng thái tải/lỗi/mất kết nối của popover và cài đặt dùng V12 (#70). | Popover Việc nền: trống (giờ quét tiếp nằm trong câu mô tả), đang chạy, thông báo, chi tiết và hành động khớp `Activity` 1440×900 dưới 1px. Thông báo hệ điều hành đi qua worker; cài đặt, không làm phiền, lịch sử phân trang. Còn: chủ máy bấm thử thông báo macOS thật và badge của hệ điều hành. |
| V15 — Kho prompt ảnh | | | Nhập prompt trùng tên (#45). | Đang làm. Đã có: 7 prompt có sẵn (không Lưu/Xoá), prompt của bạn: tạo, sửa, nhân bản, xoá (đang dùng thì chặn kèm tên template), lọc theo loại, tìm, xuất/nhập JSON; template chọn prompt ảnh cảnh và bìa, `imagePromptMode` tự suy ra. Còn: video đang chạy giữ nội dung cũ, video mới dùng nội dung mới, ảnh tách nền và bìa “Chữ hook” trong video (cần pipeline, V6); Ảnh mẫu/Vẽ thử. |
| V17 — Kết nối AI và Trợ lý AI | | | Dữ liệu artboard chưa có nguồn (#26); thứ tự thẻ, thẻ trùng loại (#27); giá mỗi video = 7 ảnh, lưới 4 nguồn vẽ (#28). | Đang làm. Đã có: §15 đủ API; ChatGPT (một credential trong `core/auth`), Grok mã thiết bị, khoá API niêm phong AES-GCM, Ollama; danh mục model phân trang, thêm model tay; Đạo diễn/Hoạ sĩ chọn model và nguồn vẽ dự phòng; trần ảnh trả phí mỗi ngày (D7). QA ảnh cặp 5 vòng xong 30/10/2026, còn 0 lỗi lớn / 0 lỗi nhỏ. Còn: chủ máy thử đăng nhập Grok và thêm khoá API thật. |
| V22 — Thư mục lưu dự án | 30/10/2026 | `00fd26e` | Chọn thư mục bằng danh sách của máy chủ (#21); “Ổ ngoài USB” theo `diskutil` (#22); banner đếm việc nền, không đếm video (#23). | Xong. §23 (`here|pick|lowspace|cloud|moving|done|donenew|offline`), chuyển thư mục chạy nền có tiến độ và thử lại, phát hiện ổ mất/chỉ đọc, dọn theo chính sách giữ có xem trước. QA ảnh cặp 5 vòng, còn 0 lỗi lớn / 0 lỗi nhỏ. |
| V23 — Video nền và Tài khoản nền tảng | | | Mốc “từ HH:MM” và “một số video” (#25); state điện thoại chưa có artboard, giãn dòng 1,5 (#29). Thêm kênh nền chạy nền, dán link video không dò danh sách (#44). | Đang làm. Đã có: phần Tài khoản nền tảng (YouTube cookie từ trình duyệt hoặc tệp, Bilibili, Twitch, hồ sơ, trạng thái tạm chặn từ `platform_state`). Thư viện › Video nền: dán link (kênh, video lẻ kèm đoạn ≤ 600 giây, nền tảng chưa hỗ trợ lưu yêu cầu trên máy, gửi kèm Báo lỗi); danh mục Twitch và Bilibili (ký WBI), điều tốc theo nền tảng, lưới ảo hoá keyset; ảnh thu nhỏ Twitch cỡ 640×360; Template › Nền & hiệu ứng (Ngẫu nhiên/Luôn có, thanh trộn, cắt clip, chỉnh nền, lớp phủ); CLI `viderse backdrop`. Smoke thật: kênh Twitch 212 video. Còn: lấy clip khi làm video (V6). |
| V24 — Đăng nhập Bilibili bằng mã QR | | | Mở app 哔哩哔哩 trên chính điện thoại (#6). | Đang làm. Đã có: `qr=loading|waiting|scanned|expired|rejected|error|success` trên máy tính và điện thoại, API trả cả `loginUrl` (#7), poll 2 giây, tự gia hạn cookie. Còn: quét thật bằng tài khoản Bilibili. |
| V27 — Lấp chỗ thiếu artboard | 01/11/2026 | `8cf29f38`, `ac1d3876`, `2f7e860a`, `a3a8f801`, `6b54e374`, `9595c703`, `2d68154c`, `df3c54a4` | Hoàn tác bỏ khỏi hàng chờ dùng `PATCH /api/queue { action: "restore" }` trên chính mục cũ, mục bỏ giữ 24 giờ (#64). Nút đang xử lý giữ spinner chồng của nút dùng chung, lệch ~3px bề ngang (#65). | Xong. Viên việc nền 4 trạng thái (Đang chạy · Máy rảnh · Đã tạm dừng lúc · Mất kết nối sau 2 lần `/api/status` lỗi hoặc SSE đứt >10 giây); `MQueue` thay Dialog tạm, Quy tắc chạy bottom sheet (luồng 1–6, ảnh 1–8, đối thủ 1–24 giờ); Ưu tiên cả N ở `MNew`; bỏ Không cần ảnh ở Video mới cả hai bản (`noImages` chỉ còn ở CLI); token `--vv-red-button-fill`, `--vv-accent-soft`, `--vv-info`, `--vv-surface-3`; Tài khoản điện thoại › Thiết bị (đăng xuất từng máy, mọi máy khác có xác nhận), mật khẩu tạm có Đăng xuất, lỗi mạng/chép mã/không có mã khôi phục, `kicked=none` theo `vv.signedInBefore`; `GET /api/me/sessions` trả `{ id, device, browser, lastActiveAt, expiresAt, current }`. Đo board-vs-app ở 1440×900 và 390×844. |
| V30 — Đường dẫn thư mục, chỗ trống theo ổ, nhóm chọn | 01/11/2026 | `9d1b714f`, `1667f1e7`, `ca30e27e`, `688e52f3`, `4ee2f09c` | Đường dẫn cắt giữa theo thư mục bằng bề rộng đo thật (`PathText`: canvas `measureText`, `ResizeObserver` + `fonts.ready`), một dòng, `title` = đường đầy đủ; đo trên app chạy thật ở 1440: dòng rộng 410 px, `/Volumes/Samsung T7/Studio 2026/Kênh YouTube/Viderse Projects` hiện `/Volumes/Samsung T7/…/Kênh YouTube/Viderse Projects` (đúng artboard), mặc định `~/.viderse/projects`. Chỗ trống luôn kèm tên ổ (“Samsung T7 còn trống 2,1 GB”), ổ ngoài rút → `freeBytes: null` + “Samsung T7 chưa cắm · chưa đọc được chỗ trống”, không lấy số ổ trong máy (nhớ tên/loại/hệ tệp qua `storage.volume`). Segmented mục co theo nhãn, không “…” , `.kit-seg` cuộn ngang ẩn thanh cuộn, `fit="fill"` (`min-width: max-content`), viên trượt đo theo hộp thật + `scrollLeft`, bỏ ngoại lệ `maxWidth: none` của Explore. Sửa hai lỗi chặn nghiệm thu: cổng thao tác thư mục khoá vô điều kiện (`SetServerStorageMode(true)`) và `validateTarget` chặn mọi path dưới `/Volumes`; chế độ máy chủ nay = có `PUBLIC_URL` HOẶC kho media ở S3, bản desktop trả 404 `/api/storage/server` để giao diện giữ thẻ V22/V30 (01-ia-flows W13). Thiếu artboard: (a) thư mục trong máy biến mất dùng câu suy ra “<Loại ổ> · chưa đọc được chỗ trống”; (b) ổ thử là ảnh đĩa APFS nên cổng nối hiện “Disk Image” thay vì “USB” của artboard; Đã chụp thêm trên serve desktop thật: MSettings 390 `store=here|done` (đường dẫn một dòng cắt giữa `/Volumes/Samsung T7/…/Viderse Projects`), Explore/MExplore 390 và 1440 với 12 video mẫu — nhóm Sắp xếp đủ chữ “Nổi trội / Mới nhất / Lượt xem”, không mục nào bị “…”, cuộn ngang khi chật. Khi rút ổ thật lộ thêm hai lỗi, đã sửa: máy chủ treo vì `AttentionItems` khoá `s.mu` hai lần (không vào được `store=offline`), và nhãn “Không thấy …” dùng tên thư mục thay vì tên ổ; viên trạng thái ở topbar 390 bị bóp chấm về 0px và vùng chạm 44px đè lên chấm. Chưa so px Kit/MKit › Nhóm chọn. |
| V31 — Toast theo Noverse | 01/11/2026 | `b4c715c1` (toast kit + host Viderse), `f25db475` (Editor bỏ sonner — báo cáo của V31Editor) | Thẻ việc nền: một nút sáng chỉ ở thẻ trước; thẻ Cần xử lý không tính vào giới hạn 2 (ở lại tới khi đóng). Chưa chụp trạng thái rê chuột mở rộng chồng thẻ trên app chạy thật (hành vi dừng đếm ngược có test). Màn không có ô Nhắn trợ lý (Editor, cổng) → viên giữa đáy màn. | Đang làm. Nhánh activity của `platform/ui/src/Toast.tsx` (Noverse không đổi): kính `--vv-toast-glass`, đếm ngược = vòng mảnh quanh nút đóng, bỏ thanh đáy và dòng “Tự đóng sau…”/“Gộp N thông báo…”. Viên kết quả thao tác 40 px, neo `[data-toast-anchor]` (ô Nhắn trợ lý) — đo 1440×900: cách ô **12 px**, giữa cột nội dung; Hoàn tác 5 giây kèm “· Ns”; thẻ việc nền **328 px** góc phải, tối đa 2 (điện thoại 1, rộng màn trừ 12 px, cách đáy 80 px), thẻ cũ lùi; tiêu đề nêu việc (“3 video chờ duyệt ảnh”, “Cần đăng nhập lại”, “Sẵn sàng đăng”), tên video ở câu phụ. Test: `Toast.test.tsx`/`Toast.activity.test.tsx` (7 ca fail trên HEAD trước), `activityText.test.ts`, `ActivityBridge.test.tsx`. Ảnh: `/tmp/v31-pill3-1440.png`, `/tmp/v31-stack-1440.png`, `/tmp/v31-card-390.png`. |

| V32 — Thanh trên điện thoại + Thông báo | 01/11/2026 | `0412629f`, `16db2920`, `3da9f5f0` | Số chưa đọc lấy từ `GET /api/activity` `unread` (mọi kind), không phải `status.attention`; sheet mở `initialTab='inbox'` chỉ khi nút có chấm. Điện thoại: nút Việc nền tròn 36px (vòng+số running · icon idle · vòng xám Tạm dừng · cam khi offline), chấm vàng 10px; sheet “Việc nền” hai tab Đang chạy · Thông báo (Segmented fill, huy hiệu vàng đếm chưa đọc), dòng phụ “N chưa đọc / Đã đọc hết / Chưa có thông báo”, dòng chưa đọc in đậm + chấm vàng trên mép icon, “Đánh dấu đã đọc” gọi `POST /api/notifications/read`, chân list “Cài đặt thông báo” và trạng thái rỗng khớp MMain screen=inbox 390. Thanh trên: logo 24 · chọn kênh “Mọi kênh” 3 avatar một chữ xếp chồng (−7px, viền 2px; desktop 20px/−6px) · chấm việc nền · avatar tài khoản 36px. Danh sách thông báo rút gọn thành `shell/notificationList.tsx` dùng chung popover máy tính và sheet (một nguồn, một cách sắp, một cách vẽ — không fork). GAP: màn offline/đang chạy/tạm dừng của nút chỉ đo bằng cách đổi `navigator.onLine`/`useQueue` trong trang; badge tab chỉ thấy khi ≥1 chưa đọc; desktop không đổi (tab popover vẫn dùng nguồn cũ). | Test: `SummaryInbox.test.tsx` (6 ca: mở đúng tab có chấm, đậm+chấm vàng, Đánh dấu đã đọc, trống, Cài đặt thông báo, tab Đang chạy giữ câu cũ) — 6 fail trước, xanh sau; `JobsPill.test.tsx` 8 ca; `ChannelSwitcher.test.tsx` 9 ca; `design-rules.test.ts` 16/16; `typecheck` sạch. |
## Nghiệm thu phase 10 — trạng thái từng gói, 01/11/2026

Bảng trên là lịch sử từng gói. Bảng dưới là **kết luận nghiệm thu** (phase 10): mỗi gói V0–V27 một dòng, trạng thái + bằng chứng chạy lại được, và các mục phải chờ model/ảnh trả phí/tài khoản hay thiết bị thật ghi **BLOCKED** kèm lệnh chạy sau.

Trạng thái: **Xong** = mọi mục nghiệm thu của gói đạt. **Đạt (BLOCKED n)** = phần làm được đã đạt, còn n mục cần điều kiện thật. **Tạm hoãn** = chủ sản phẩm chốt hoãn.

| Gói | Trạng thái | Bằng chứng | Còn lại / BLOCKED | Hỏi đáp |
|---|---|---|---|---|
| V0 — Nền tảng | Xong | Đo 01/11 trên Vite :18924 + serve :18921, Chrome headless 1440×900: thanh trên **1440×55** = board `Main` **1440×55**; viên việc nền cao **30** = board **30**; viên ghi “Máy rảnh”, `PATCH /api/queue {paused:true}` → “Đã tạm dừng lúc 09:42 · 0 chờ lượt”; chấm bánh răng `aria-label="Cài đặt · 5 việc cần xem"`, bấm → `#/settings?section=connect`; chữ phụ `rgb(139,139,151)` = `#8B8B97`; không còn `.vv-rail`; `document.getAnimations()` rỗng khi `prefers-reduced-motion`; `#/dev/kit` render đủ nhóm Kit/MKit/Nền tảng/Xác nhận/Menu; `src/checks/design-rules.test.ts` 15/15 (không hex/bo góc ngoài tokens, ≤1 nút chính mỗi màn). | BLOCKED 1: “tạm dừng một job đang vẽ ảnh” cần job thật — lệnh ở §Lệnh chạy lại. | #14 (mực badge Duyệt) |
| V1 — Bàn dựng | Đạt (BLOCKED 1) | Báo cáo phase 07: 42 tổ hợp phạm vi × bước × menu prompt khớp khung/hộp thoại artboard, bảng Nguồn/Nền/Ảnh/Metadata 0px; `55587e05`, `09d2acf1`, `81029268`, `f482e336`, `f912ade5`, `84e733c4`, `af68df07`, `4e8cc383`, `f6c9cac4`, `a58b1dbf`, `a9c2115e`, `ee5da46d`, `693e8711`, `78b9f37a`; test `OverflowMenu.test.tsx` (mới, fail trước khi khoá), `TestReviewOtherImageWhileRunningLocksDuplicateAndVideoApproval`. | BLOCKED 1: bấm **Duyệt ảnh** ra 7 ảnh thật, chỉ 1 ảnh trả phí — cần model + nguồn vẽ thật. | #73 (bật lại ảnh khi template không vẽ ảnh) |
| V2 — Làm video | Đạt (BLOCKED 1) | `src/screens/new/plan.test.ts` (mới, 4 ca: máy rảnh → “Bắt đầu ngay khi bấm”; máy bận 2/2 → “Lên đầu hàng chờ · bắt đầu khoảng 00:40 khi một trong 2 video đang chạy xong”; 4 link 1 trùng nhịp 2 → “Vào cuối hàng chờ, theo nhịp 2 video/ngày …” + “Hôm nay còn 2 lượt, video thứ 3 chạy ngày mai · 1 đã làm cho kênh này · sẽ bỏ qua”; `pickPriority`), nhãn “Ưu tiên · bạn chọn”, plan/enqueue HTTP + PostgreSQL thật (`c26e04a6`). | BLOCKED 1: dán link thật → video vào **Đang làm ≤ 2 giây** (cần yt-dlp tải video + model). Board không vẽ trạng thái máy rảnh; câu lấy theo 08-dev-prompt V2. | — |
| V3 — Duyệt | Đạt (BLOCKED 1) | `TestReviewRejectsBusyAndStaleVersions`, `TestReviewOtherImageWhileRunningLocksDuplicateAndVideoApproval`, `TestReviewRetainsDirectionAndRejectsNoImageRedraw`, `TestApproveAllCommitsBuildInvalidation`, `TestPromptChangeClearsReviewOrigin`; UI Duyệt đo 1440×900 ở báo cáo phase 07 (bảng đối chiếu V13 cùng khung). | BLOCKED 1: “Duyệt ảnh → 7 ảnh thật, chi phí 1 ảnh” cần nguồn vẽ trả phí thật. | #68 đã chốt |
| V4 — Kênh | Đạt (BLOCKED 1) | `62f501e4` (Hồ sơ hai cột 403), `479e5d58`, `67acd3b8` (kênh mới), `b4f6bb45` (BulkBar một dòng), `f86f8e45` (9 hàng, thứ tự, số tab), `10fa2186`, `6f1463e5`, `30c9f248` (khoá xoá kênh theo số video đang làm), `05223e7c`; test `bulkEligibility`, `channel-videos` (cửa sổ 9 hàng, shift-click, Esc). | BLOCKED 1: đoạn cuối mô tả + 3 tag áp vào **video viết sau** (cần model). | #39, #42, #43 đã chốt |
| V5 — Khám phá + Video đối thủ | Đạt (BLOCKED 1) | Theo dõi @handle/link/UC, quét lịch sử nền 150–10.000, tạm dừng/tiếp tục/huỷ sống qua khởi động lại; test scan/rival + HTTP; UI 42 tổ hợp ở V1; Top 10 → 10 mục cuối hàng chờ (test enqueue `c26e04a6`); smoke Twitch thật đọc 212 video (báo cáo V23). | BLOCKED 1: quét một kênh YouTube thật khi mạng cho phép (cần cookie/không bị chặn). | #31–#37 đã chốt |
| V6 — Hàng chờ | Đạt (BLOCKED 1) | ETA 7 ngày/nhịp/công bằng + HTTP/PostgreSQL (`c26e04a6`); `paceFull: 'Kênh đủ nhịp hôm nay · chạy từ 00:00'`; `MQueue` + Quy tắc chạy (V27); viên “Đã tạm dừng lúc 09:42” đo 01/11; kéo mục lên đầu tính lại ETA (test queue-eta). | BLOCKED 1: “hết hạn mức mọi nguồn vẽ” cần hạn mức thật của provider. | #52, #55, #56 đã chốt |
| V7 — Cài đặt | Đạt (BLOCKED 2) | QA ảnh cặp 5 vòng xong 30/10, 0 lỗi lớn/0 lỗi nhỏ; `Settings` 3 nhóm + chấm cảnh báo, Công cụ (`ok|update|failed|installing`), Nhật ký (lọc, 7 ngày, xuất log đã che), `MSettings`; test tools/logs. | BLOCKED 2: (a) yt-dlp “Có bản mới” + **Cập nhật** (cần mạng, bản release mới); (b) đăng nhập Grok bằng mã (cần tài khoản xAI). | #18, #19, #20, #30 |
| V8 — Thư viện + Template | Đạt (BLOCKED 2) | QA ảnh cặp 31/10; template tự lưu theo `revision` (409 khi lệch), Đổi tên/Nhân bản/Thêm vào kênh/Xoá (khoá khi đang dùng), xem thử `fresh|stale|building`, giá ước tính; `viderse template`, `viderse overlay`; test library/template. | BLOCKED 2: (a) dựng lại xem thử + Vẽ thử bìa (cần video đã làm xong); (b) giá thật từ `price.perImage` của provider. | #38, #41, #46, #48 đã chốt |
| V9 — Lần đầu mở + Tạo kênh | Đạt (BLOCKED 1) | `#/setup` (route mới, #17), 6 việc cài thật + tiến độ SSE, đăng nhập ChatGPT, kiểm tra dung lượng; Tạo kênh bắt buộc ≥ 1 đối thủ, thêm đối thủ ngay trong luồng, Xong → “Làm video mới nhất” (`pick`); test setup/channels. | BLOCKED 1: chạy cài 6 việc thật trên máy mới (~1,4 GB tải). | #17, #40, #47 đã chốt |
| V10 — Điện thoại | Tạm hoãn | `plans/…/phase-09-mobile-lan.md` → `status: paused`; chủ sản phẩm chốt 31/10. Không màn nào của V10 nằm trong bản chạy. | Chưa đạt 2 mục nghiệm thu (390×844 không cuộn ngang; duyệt một video chỉ bằng chạm) vì gói hoãn. | #16, #29 (mốc 700/768, state điện thoại) |
| V11 — Nhắn trợ lý | Đạt (BLOCKED 2) | `TestParseThenRunRedrawPutsSceneIntoDrawing`, `TestEveryIntentPlansThenRunsThroughTheRealService`, `TestMultiStepPlanOrderTotalAndSubsetRun`, `TestDeleteChannelIsRefusedWithTwoSuggestions`, `TestSchemaRejectsUnknownIntentAndNothingRuns`, `TestModelUnavailableIsHonest`, `TestInjectionSentencesStayInPayloadAndYieldValidIntentOrFailure`; UI ô nhắn/thẻ kế hoạch/thẻ xác nhận đo ≤ 1px ở báo cáo phase 08 (`a7fe90e1`, `5a8706b7`, `a502b27b`, `b1727b9c`, `fc9663b6`). | BLOCKED 2: (a) “cảnh 5 cho y tá cầm đèn pin” → thẻ → **Làm** → cảnh 5 đang vẽ (cần model thật); (b) câu “xoá kênh” phải từ chối + 2 gợi ý trên model thật (máy đã test). | #74 |
| V12 — Hộp thoại + trạng thái hệ thống | Xong | Đo 01/11: `set offline on` → dải cam hiện (≤ 6 s; ghi cũ ~0,1 s), ngang hết bề rộng dưới đầu trang (y=55), nội dung `[inert]`; `set offline off` → dải biến mất, `inert` bỏ; `/?video=khong-ton-tai` → **Không tìm thấy** với đúng **một** nút “Về Bàn dựng”; **khoá Xoá video khi đang chạy**: `OverflowMenu.test.tsx` (đỏ trước khi sửa, xanh sau) + `lockedReason` ở menu hàng Kênh › Video; 7 loại `ConfirmDialog` (Esc, gõ tên, cỡ rộng) + toast dùng chung với Activity. | — | #30 |
| V13 — Editor | Đạt (BLOCKED 1) | Bảng đối chiếu runtime ở tài liệu này (mọi dòng 0px/0,01px); ExportGate 84/100, xung đột 3 lựa chọn, ⌘Z, Cần sửa mở đúng đoạn lệch; test editor/quality. | BLOCKED 1: **Khớp theo giọng** + **Đo lại** trên video thật có phụ đề (cần tải video + máy chạy nhận giọng). | #13, #66, #67 đã chốt (trừ #13) |
| V14 — Việc nền & thông báo | Đạt (BLOCKED 1) | Popover Việc nền 1440×900 < 1px với `Activity`; gộp toast 10 s, “Không làm phiền” vẫn ghi lịch sử, phân trang; test activity/notification (`50887543`, `5f186587`, `0a2367f1`). | BLOCKED 1: thông báo **macOS** thật + số trên biểu tượng PWA (chủ máy bấm thử). | #70 đã chốt |
| V15 — Kho prompt ảnh | Đạt (BLOCKED 2) | 7 prompt có sẵn (không sửa/xoá), prompt của bạn CRUD + nhân bản + lọc + tìm + xuất/nhập JSON; prompt đang dùng bị chặn khi xoá kèm tên template; template chọn `coverPromptId`/`imagePromptId`, `imagePromptMode` suy ra; test library/prompts. | BLOCKED 2: (a) video đang chạy giữ prompt cũ / video mới dùng prompt mới (cần pipeline thật); (b) ảnh tách nền + bìa “Chữ hook” (cần vẽ trả phí). | #45 đã chốt |
| V16 — Prompt ảnh khi đang vẽ | Đạt (BLOCKED 1) | Hộp thoại thật ở Bàn dựng (video 4/6 ảnh): “Vẽ lại 4 ảnh cảnh đã vẽ; 2 ảnh còn lại vẽ luôn theo prompt mới. Tốn 4 ảnh”; “Đổi · không tạo ảnh”; đổi prompt bìa chỉ vẽ lại bìa; PATCH trong lúc vẽ được xếp hàng (`4e1f23e1`), đếm ảnh đang vẽ (`a8dd4f29`); test prompt-change/redraw. | BLOCKED 1: chi phí thật của lượt vẽ lại (4 ảnh) cần nguồn vẽ trả phí. | — |
| V17 — Kết nối AI và Trợ lý AI | Đạt (BLOCKED 4) | §15 đủ API; ChatGPT một credential `core/auth`, Grok mã thiết bị, khoá API niêm phong AES-GCM, Ollama; danh mục model phân trang + thêm tay; Đạo diễn/Hoạ sĩ chọn model + nguồn dự phòng; trần ảnh trả phí/ngày (D7); QA ảnh cặp 5 vòng 30/10, 0 lỗi. | BLOCKED 4: (a) OpenRouter khoá đúng → thẻ mới + chọn model; (b) khoá sai → thẻ cam 401; (c) qwen3:14b mờ ở “Model viết mô tả ảnh”; (d) dây dự phòng ChatGPT → Grok → Gemini khi hết hạn mức. Cần khoá/tài khoản thật. | #26, #27, #28 |
| V18 — Vì sao? và Bản tin | Đạt | Bản tin tự mở sau 3 giờ có việc mới, câu đầu = số Chờ bạn; không có gì mới thì không tự mở (đo trên serve thật, báo cáo phase 07); `TestRecapNarratorCannotAlterCounts`; **Vì sao?** chỉ hiện link khi có số — `TestScoreReasonRequiresNumericalBasis`, `f5fe6c7f` (đưa lý do điểm vào xem trước Khám phá); 3 nguồn số (trung vị, nguồn, cửa sổ). | — | #67 đã chốt |
| V19 — Đề xuất và Quy tắc đã học | Đạt (BLOCKED 1) | `TestLearnV19AcceptancePostgres`: sửa tiêu đề lần 1–2 chưa gợi ý, **lần 3** sinh đề xuất “Lưu thành quy tắc”, lưu → prompt metadata **video kế tiếp** có quy tắc, nguồn “Học từ 3 lần bạn sửa”; `TestSuggestionsDismissCooldownLimitPostgres`: bỏ qua → nghỉ **7 ngày**, tối đa **5** đề xuất mở; `TestSuggestionsAreValidForTheirScope`; web: đề xuất không bao giờ toast (`076fc8fc`), hàng Trợ lý chủ động + Quy tắc đã học khớp board (`ee9ae1b6`, `30d697eb`, `f6df86d7`, `e480d483`). | BLOCKED 1: “video kế tiếp không còn phần trong ngoặc” khi chạy thật (cần model). | #75 |
| V20 — Mức tự chủ và điểm ảnh | Đạt | `TestAutonomyAutoApprovesWhenEveryImageMeetsThreshold`, `TestAutonomyKeepsWholeVideoInReviewWhenOneImageIsBelow`, `TestAutonomyBalancedNeverAutoApproves`, `TestAutonomyRetriesSourceDownloadOnceThenNeedsAttention`, `TestAutonomyCarefulDoesNotRetry`, `TestAutonomyNeverRetriesHumanErrors`, `TestAutonomyValidateThreshold`; `domain.Autonomy` chỉ có `pick/approve/onError/belowQuality` — **không có** trường đăng/xoá/template nên không có đường tự làm ba việc đó; thẻ tự chủ khớp board (`14c1afdf`, `0e59931c`, `10fa2186`), hộp xác nhận 520 (`14841256`), banner điểm ảnh (`df30b083`), dòng “Đã tự thử lại lúc HH:MM” do máy chủ dựng (`internal/domain/clone_view.go:381`). | Không chạy thử chấm điểm ảnh bằng model thật: chủ sản phẩm bỏ (01/11/2026). Luồng tự duyệt theo điểm chỉ có chứng cứ bằng bộ chấm giả trong test. | — |
| V21 — Hiệu quả sau đăng | Đạt (BLOCKED 1) | `TestAnalyzeNineVideosAtMostThreePreliminaryInsights`, `TestPerformanceViewShowsAtMostThreeAndVsMedian`, `TestApplyTitleInsightCreatesRuleWithNumericSource`, `TestPerformanceNeedMoreBelowSix`; serve thật seed 9 video đã đăng: 3 nhận xét “Sơ bộ”, `vsMedian` 2,03, áp dụng nhận xét tiêu đề → quy tắc nguồn “Từ số liệu 9 video”, dưới 6 → “Cần thêm N video…”; chữ/nhóm khớp board (`89557031`, `a5d66d07`, `694da767`). | BLOCKED 1: số liệu thật qua YouTube Data API (cần khoá) + `price` cho doanh thu. | #76 |
| V22 — Thư mục lưu dự án | Xong | QA ảnh cặp 5 vòng, 0 lỗi; §23 đủ trạng thái `here|pick|lowspace|cloud|moving|done|donenew|offline`; chặn thư mục đám mây (`internal/storage/paths.go`), chỗ trống ≥ dữ liệu × 1,1 (`service.go:600`), cùng ổ `rename` khác ổ chép có kiểm, bản cũ giữ **7 ngày** (`move.go:28`), hàng chờ tạm dừng khi mất ổ; đường dẫn đọc từ DB (`storage.location`), không còn `storage.json` (#10). | BLOCKED 1 (chủ máy): chuyển 81 GB thật sang `/Volumes/Samsung T7`. | #21, #22, #23 |
| V23 — Video nền và Tài khoản nền tảng | Đạt (BLOCKED 2) | Phần Tài khoản nền tảng (YouTube cookie từ trình duyệt/tệp, Bilibili, Twitch, hồ sơ, `platform_state`); Video nền: dán link kênh/video lẻ (đoạn ≤ 600 s), nền tảng chưa hỗ trợ lưu yêu cầu kèm Báo lỗi (hai lần bấm = một yêu cầu), Twitch + Bilibili (ký WBI), điều tốc theo nền tảng, lưới keyset ảo hoá, ảnh thu nhỏ 640×360; smoke Twitch thật: 212 video, `db.json`/`cli-parity.json` trong báo cáo phase 22. | BLOCKED 2: (a) Bilibili bị chặn thật → “Tạm chặn tới 02:10” rồi tự quét lại; (b) migration template cũ có clip cố định — không áp dụng vì bắt đầu trắng (#9). | #25, #29, #44 đã chốt |
| V24 — Đăng nhập Bilibili bằng mã QR | Đạt (BLOCKED 1) | `qr=loading|waiting|scanned|expired|rejected|error|success` ở cả máy tính và điện thoại, API trả `loginUrl`, poll 2 giây, tự gia hạn cookie, đóng hộp thoại thì dừng hỏi mã, cookie bilibili.tv bị từ chối; test biliqr. | BLOCKED 1: quét mã thật bằng tài khoản Bilibili (kể cả “Tạm chặn → Sẵn sàng”). | #6 |
| V25 — Kho máy chủ (S3) | Đạt (BLOCKED 1) | Báo cáo phase 29: backend (S3Provider, presign proxy/redirect, materialize/gc, khoá niêm phong) + web (Cài đặt › Lưu trữ `driver=s3`, trạng thái, nút Kiểm tra); kho S3 thật của repo là **Garage v2.5.0** trong `deploy/compose.test.yml` (GarageCompat: layout, bucket riêng, kiểm PUT có điều kiện); hỏi đáp #49 đã chốt. Phase 10 **chưa** chạy lại các nhánh lỗi của Viderse trên Garage. | BLOCKED 1: bucket thật của chủ máy trên máy chủ (Garage/MinIO) — cần chủ máy cấp endpoint + khoá. | #49 đã chốt |
| V26 — Đăng nhập chung (SSO) | Đạt | Báo cáo SSO: 19/19 E2E (đăng nhập, cổng cấp quyền app, đổi mật khẩu bắt buộc, session dùng chung cookie `verse_session`, chuyển app `VERSE_APP_LINKS`), ảnh chụp Auth/MAuth + đo bố cục; backend pgtest/httptest. | BLOCKED 1 (chủ máy): bật `VERSE_APP_LINKS` thật trên máy chủ để thấy hàng chuyển app. | #50, #51 đã chốt |
| V27 — Lấp chỗ thiếu artboard | Xong | Viên việc nền 4 trạng thái — đo 01/11: “Máy rảnh” và “Đã tạm dừng lúc 09:42 · 0 chờ lượt”; `MQueue`, Quy tắc chạy bottom sheet (luồng 1–6, ảnh 1–8, đối thủ 1–24 giờ), “Ưu tiên cả N” ở `MNew`; `8cf29f38`, `ac1d3876`, `2f7e860a`, `a3a8f801`, `6b54e374`, `9595c703`, `2d68154c`, `df3c54a4`; hoàn tác bỏ khỏi hàng chờ bằng `PATCH {action:"restore"}` trên chính mục cũ (giữ 24 giờ). | — | #60, #64, #65 đã chốt |

### Lệnh chạy lại các mục BLOCKED

Chỉ cần một lần chủ máy đăng nhập (ChatGPT/Grok/khoá API) và cấp khoá S3; sau đó chạy theo nhóm:

```sh
# Model thật (ChatGPT đã đăng nhập, hoặc Grok/khoá API)
viderse login --status
# 1. V0 tạm dừng job đang vẽ ảnh; V2 dán link → Đang làm ≤ 2 s; V3/V16 duyệt và vẽ lại ảnh trả phí
#    (UI: Bàn dựng › ⋯ › Tạm dừng; Video mới › dán link; Duyệt › Duyệt ảnh; ⋯ › Vẽ lại ảnh)
# 2. V11 nhắn “cảnh 5 cho y tá cầm đèn pin” rồi Làm; nhắn “xoá kênh” → phải từ chối + 2 gợi ý
# 3. V19 sửa tiêu đề 3 lần → “Lưu thành quy tắc”
# 4. V21 thêm khoá YouTube Data API ở Cài đặt › Kết nối AI, đánh dấu 9 video đã đăng
# 5. V17 thêm OpenRouter bằng khoá đúng, rồi khoá sai; chọn model cho Đạo diễn/Hoạ sĩ
viderse clone run <youtube-url> --for <channelId>   # nếu chạy từ CLI

# Thiết bị / tài khoản thật
# V7: Cài đặt › Công cụ › yt-dlp “Cập nhật”; V7: Cài đặt › Kết nối AI › Grok (mã thiết bị)
# V24: Cài đặt › Tài khoản nền tảng › Bilibili › Đăng nhập bằng mã QR (quét bằng app 哔哩哔哩)
# V14: ẩn cửa sổ, tạo một video Cần xử lý → kiểm Notification Center + badge PWA
# V23: dán link kênh Twitch/Bilibili thật; V5: quét một kênh YouTube thật

# Kho S3 và máy chủ (chủ máy)
STORAGE_DRIVER=s3 S3_PROVIDER=garage S3_ENDPOINT=… S3_BUCKET=… viderse serve   # V25
# V26: đặt VERSE_APP_LINKS=noverse=<origin>,viderse=<origin> ở cả hai app
```

Ghi kết quả từng mục vào bảng trên (thay BLOCKED bằng số đo/ảnh chụp) trước khi chốt phase 10.

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


## Gói V29 — ảnh có prompt đã gửi, sửa prompt và vẽ lại đúng ảnh

Kiểm bằng Viderse serve thật (binary của nhánh, DB ném đi trên cluster vs3) + Vite dev tại 5182 trỏ vào API đó, agent-browser 1440 × 900 / 2560 × 1440 / 390 × 844. Không tốn quota: nhà cung cấp OpenAI-compatible nội bộ (`/v1/models`, `/v1/chat/completions`, `/v1/images/generations`) đăng ký qua đúng màn Kết nối AI nên `Paid()` = false; bước `scenes` được seed (manifest + keyframe + thumbnail là blob thật), từ đó bước `images` chạy thật: gọi máy chủ vẽ, ffmpeg mã hoá WebP, publish artifact, ghi manifest §26. Phiên bản 1–4 đều do pipeline vẽ; `use`, `redraw`, `cancel` gọi đúng endpoint của app.

### Đã giao

- Backend: manifest ảnh có `parts`/`wrapper`/`sentText`/`refs`/`provider`/`costImages`/timing, `revision` chống ghi đè, endpoint `redraw` (kèm `feedback`), `use`, `cancel`, `batch`, `refs` add/remove, upload bìa thành phiên bản ready; `providerLimit` theo từng nhà cung cấp; `image.progress` cho pha gửi/nhận.
- Duyệt (desktop): khung ảnh + dải phiên bản + panel "Prompt đã gửi" theo phần, `so với bản N` (ins/del), Sửa prompt theo phần, Góp ý + Vẽ lại, Dùng bản N, Vẽ lại cả N ảnh.
- Điện thoại: `MobileReview` cùng dữ liệu, sheet phiên bản, góp ý/vẽ lại, huỷ lần vẽ.
- Editor: panel cảnh đổi ảnh tại chỗ + ⌘Z; Bàn dựng có ô Phòng vẽ.
- Prompt tách đúng nghĩa: năm phần sửa được theo board (Cảnh · Phong cách · Tham chiếu · Ghi chú đã học · Tránh); hàng rào an toàn và chỉ dẫn công cụ nằm ở `wrapper`, chỉ hiện ở "nguyên văn", không nằm trong phần sửa được và không gửi cho Grok/HTTP (`imagegen.ToolInstructions`).
- Dòng phiên bản `drawing` ghi ngay lúc bắt đầu chuyến vẽ (phần prompt, refs, nguồn vẽ, `startedAt`, `n`), thay bằng ready/failed/cancelled khi xong; lần thử lại dùng lại đúng `n`. Dòng mồ côi (mất `InFlight`) thành `failed` lý do `provider` khi bước chạy lại; huỷ được từ UI với `costImages` 0.

### Sửa trong lúc tích hợp (lỗi thật, có test)

| Lỗi | Tác động | Sửa |
|---|---|---|
| `CommitImageRedraw` trên bước `images` đã đỗ chỉ ghi manifest, không xếp lại bước, không đánh thức worker | "Vẽ lại" ở màn Duyệt (và ở panel cảnh) treo vĩnh viễn: bản mới nằm trong manifest nhưng không ai vẽ, lần bấm sau báo 409 "Ảnh đang tạo" | `jobedit.CommitImageRedraw` đi nhánh `InvalidateFrom=images, Rerun, ImageRedraw` + `Wake`; store chỉ vô hiệu bước `images` (giữ project và bước sau) — `TestCloneImageRedrawRerunsImagesAndWakesJob` |
| `splitWords` lặp theo `range` rune nhưng vẫn tiến theo từ nên mỗi từ bị phát nhiều lần | Panel "So với bản N" hiện chữ nhân đôi ("Thehee shothototot andndd") | quét theo byte, mỗi từ (kèm khoảng trắng trước) đúng một lần — `TestSplitWordsKeepsEveryWordOnce`, `TestDiffTextDoesNotDuplicateUnchangedWords` |
| `MobileReview` GET lại `/images` chỉ để lấy `manifest.revision` | thêm một vòng mạng mỗi lần duyệt/bỏ ảnh, dễ lệch revision | dùng `revision` đã có trong payload của `useJobImages` |
| Hàng rào an toàn và chỉ dẫn công cụ Codex nằm trong phần Cảnh/Phong cách | panel "Prompt đã gửi" hiện `<shot_json>` + "Call image_gen.imagegen exactly ONCE … auth.json" như văn bản Hoạ sĩ viết; chủ kênh sửa phần Cảnh là xoá mất hàng rào, và prompt gửi Grok vẫn mang lệnh gọi tool của Codex | `BuildPrompt` trả `(parts, wrapper)`: guard + `<character_sheet_json>`/`<hook_text_json>` + câu refs vào `wrapper.guard` (mọi nguồn vẽ nhận), lệnh gọi `image_gen.imagegen`/luật đọc tệp/đường dẫn PNG vào `wrapper.tool` chỉ khi nguồn là Codex (`imagegen.ToolInstructions`); `sentText = JoinPrompt(wrapper, parts)` — `TestPartsCarryNoToolOrGuardText`, `TestGrokPromptKeepsGuardWithoutCodexToolInstructions`, `TestPartEditsCannotReachTheWrapper`, `TestWrapperAndPartsJoinToSentText` |
| Chuyến vẽ không có dòng `versions[]` | "Đang vẽ bản N" và **Huỷ lần vẽ** không hiện được ở Duyệt, điện thoại, panel cảnh và Phòng vẽ; `VersionDrawing` là mã chết | ghi dòng `drawing` trong checkpoint lúc bắt đầu (`recordDrawing`), thay khi xong; dọn dòng mồ côi (`recoverDrawings`), huỷ dòng mồ côi ở `CancelVersion`; `sent` chỉ bật ở pha `sending` nên huỷ trước khi gửi tốn 0 ảnh — `TestDrawingVersionIsWrittenWhileTheDrawRuns`, `TestOrphanedDrawingVersionBecomesFailed`, `TestDrawingVersionResumesAfterRestart`, `TestCancelBeforeProviderSendCostsNoImage`, `TestCancelVersionClearsOrphanedDrawingRow` |
| `withDirection` ghép câu "Owner visual direction" vào cuối phần Cảnh | `prompt=view` hiện câu hướng/tool trong phần Cảnh như văn bản Hoạ sĩ viết, dù góp ý đã có trường riêng của bản | câu hướng đi vào `wrapper.guard` (phần không sửa được): vẫn trong `sentText` ngay sau hàng rào, phần Cảnh chỉ giữ lời Hoạ sĩ — `TestDirectionLivesInWrapperNotParts`; `stripDirection` giữ cho prompt cũ chưa tách |
| Nguồn vẽ khoá API ghi `provider` là id thẻ (`openai_compat-61ed93e9`) | dòng meta, dòng hạn mức và footer ô refs ở Duyệt/điện thoại/Hàng chờ hiện id nội bộ thay nhãn thẻ | `imagegen.DisplayLabel` (nhãn thẻ qua `APIDeps.Label` ← `domain.Provider.Label`, fallback nhãn loại); `quota.providers[].label` và `QueueImageQuota.label` gửi nhãn thật; màn Duyệt resolve `version.provider` qua `/api/ai/providers` như Trợ lý AI — `TestDisplayLabelUsesCardLabel`, test `providerName`/`versionMeta` |
| Tab ô Thêm ảnh tham chiếu xuống hai dòng trong popover 327px | lệch artboard Review (nhãn một dòng) | `sx.tab` `whiteSpace:'nowrap'` + `paddingInline` |

### Kiểm lại trên serve thật sau sửa (2560 × 1440 / 390 × 844 / 360)

Cụm kiểm thử riêng (port 7851, DB `bili_v29fix2_*`, fake provider `openai_compat-c463b225` "Tương thích OpenAI" ở 7859, vite dev 5199, serve build từ worktree đã commit). Mặt Duyệt đã đo:

- 2560 × 1440: khung ảnh giữ `min(100cqw, --nat-w, 100cqh×16/9)` — `--nat-w=1920px`, khung 1758 × 988, ảnh 1920 × 1080 **không phóng**; nhãn tab ô Thêm ảnh tham chiếu một dòng.
- `prompt=view` sau Vẽ lại có góp ý: Cảnh chỉ còn lời Hoạ sĩ (nguồn "Hoạ sĩ sửa theo góp ý"); câu "Owner visual direction" chỉ nằm ở `wrapper` trong nguyên văn, đứng sau hàng rào; dòng "Bản 2 · Theo góp ý "…"" đọc đúng trường `feedback`.
- `prompt=raw`: `sentText` đầy đủ guard + direction + parts; meta "Tương thích OpenAI · fake-image · 1920 × 1080 · tốn 1 ảnh" thay id thẻ; footer ô refs "Tương thích OpenAI nhận tối đa 0 ảnh tham chiếu"; quota `{"provider":"openai_compat-c463b225","label":"Tương thích OpenAI"}`.
- Vẽ lại trên job đã đỗ ở cổng duyệt (images `succeeded`, build `awaiting_review`): bước `images` requeue (attempt 2), dòng `drawing` hiện rồi bản 2 `ready` gắn feedback — lỗi "redraw treo" của lần kiểm trước không tái hiện.
- MReview 390 × 844: thẻ ảnh + dải phiên bản + "Prompt đã gửi" (Cảnh sạch, meta nhãn thẻ) + nút Vẽ lại / Bỏ ảnh / Duyệt cả video khớp `MReview.dc.html`; scrollWidth = clientWidth ở 360 (không tràn ngang). Editor / Bàn dựng do V29Shots đo riêng.

| Thành phần | Board | App |
|---|---:|---:|
| Khung ảnh 2560 (1920 gốc) | giữ kích thước gốc | 1758 × 988 ≤ 1920, không phóng |
| MReview prompt sheet | phần Cảnh sạch, meta nhãn | khớp (Cảnh một đoạn, "Tương thích OpenAI") |

### Khoá ảnh khi đưa bản mới vào Editor — 02/11/2026

Lần chạy pipeline ảnh thật trên DB ném đi phát hiện `ProjectBinder.Rebind` giữ
khoá đọc của blob rồi gọi `ImportFileOnce`, trong khi import cần khoá xuất bản
độc quyền trên chính digest đó. PostgreSQL chờ khoá của cùng tiến trình; không
phải lỗi đăng nhập nhà cung cấp. Binder nay chép luồng vào tệp riêng, thả khoá
đọc trước import, giữ tên/định dạng gốc và xoá tệp riêng ở mọi đường thoát.

`TestCloneRebindReleasesReadLeaseBeforeEditorPublication` dùng media/editor
thật cùng PostgreSQL: giữ khoá cũ gây lỗi deadline 5 giây; thả khoá trước import
thì qua. Phép kiểm không thay lease bằng mock và không bỏ qua lỗi timeout.



### Lệch hợp đồng §26 (giữ nguyên, có chủ đích)

- Ảnh tham chiếu theo nhà cung cấp: chatgpt 4, grok 1, còn lại 0 — nhà cung cấp không nhận ref thì `refs` rỗng và nút Thêm báo `too_many_refs` với `limit`.
- Không nhà cung cấp nào stream ảnh dở nên `previewUrl` luôn `null`; tiến trình chỉ có pha qua `image.progress`.
- `wrapper` là trường thêm (chưa có trong §26 gốc): `{guard: string[], tool?: string[], provider?: string}`. `sentText` vẫn là toàn bộ nguyên văn gửi nhà cung cấp và là chuỗi ghép `guard + parts + tool` bằng dòng trống.
- "Xem nguyên văn gửi máy chủ" hiện đúng `sentText` từng byte; dòng tham số (`size · quality · references`) là chú thích riêng dưới khối, không chen vào prompt (board vẽ tham số trong cùng khối) — xem GAP nhãn bên dưới.

### GAP

- **Nhãn cho phần không sửa được.** "Nguyên văn" giờ có thêm khối hàng rào an toàn và chỉ dẫn công cụ mà board không vẽ; chưa có nhãn tiếng Việt cho chúng (đề xuất: một dòng chú thích dưới khối "Phần đầu và phần cuối là hàng rào an toàn và chỉ dẫn công cụ — không sửa được", và badge "chỉ Codex" cho khối tool). Chờ design chốt cách gọi trước khi thêm vào panel.
- Ô chọn ảnh tham chiếu trên điện thoại chưa có thiết kế (Thêm ref ở desktop có).
- Endpoint seed/dev cho job có ảnh chưa giao; muốn dựng lại cảnh kiểm thử phải seed tay (channel + job + bước `scenes` + blob).
- Chưa đo được `Editor` (focus=scene, ảnh đang vẽ/xong) và ô **Phòng vẽ** ở Bàn dựng trên serve thật: job kiểm thử chưa có project đã dựng (bước `build` cần template + video nguồn thật), nên hai mặt này mới có test đơn vị của slice — phần này do V29Shots chốt. Mặt Duyệt/MReview đã chụp lại đầy đủ sau sửa (xem "Kiểm lại trên serve thật sau sửa" ở trên).
- Trạng thái đang vẽ dùng `status === 'running'` của danh sách legacy ở ô Phòng vẽ, không đọc dòng `drawing` của §26; hai nguồn có thể lệch nhau khi bước `images` đỗ vì hạn mức.

### Số đo tại 1440 × 900 (Duyệt)

| Thành phần | Board | App |
|---|---:|---:|
| Khung ảnh (content) | 1280 × 720 | 1278 × 718 (viền 1px) |
| Rail phải | 355 | 355 |
| Ảnh ở 2560 × 1440 | 1280 × 720, không phóng | 1278 × 718, không phóng |
| Dải phiên bản | ô 60 × 34, bản đang xem viền đỏ | ô 60 × 34, bản đang xem viền đỏ |

## Tiêu đề tab, favicon và ảnh chia sẻ — 01/11/2026

**Đã làm**

- `web/public/icons/` (11 tệp) dựng từ bộ brand: favicon lấy đúng bản xuất `brand/svg/viderse-favicon-32.svg` và `brand/png/viderse-favicon-{16,32,48}.png` (luật `12-brand.md`: dưới 32 px bỏ vành kính), `favicon-64.png` dựng từ `viderse-favicon-48.svg`, `favicon.ico` (16/32/48); `apple-touch-icon.png` 180 (nền đặc cho iOS), `icon-192/512.png` (`any`), `icon-maskable-512.png` (nền phủ kín, glyph ~56% khung nằm trong vùng an toàn), `viderse-og-image.png` 1200×630. Xoá `public/favicon.svg`, `app-icon.svg`, `app-icon-dark.svg`, `app-icon-192.png`, `app-icon-512.png`; thông báo hệ điều hành trỏ `/icons/icon-192.png`.
- `index.html` theo mẫu Noverse: `viewport-fit=cover`, `theme-color`, `apple-mobile-web-app-*`, `application-name`, manifest, icon, `description`, `og:type/site_name/title/description/image` (1200×630), `twitter:card summary_large_image`; `<title>Viderse · Xưởng video</title>`.
- Manifest: `id`, `name` "Viderse · Xưởng video YouTube", `short_name`, `description`, `lang`/`dir`, `start_url`/`scope`, `display`, hai màu, icon `any` + `maskable`.
- Tiêu đề tab theo màn (`web/src/shell/labels.ts` + `useNames.ts`, gọi một lần ở `AppShell` cho cả desktop lẫn điện thoại): Bàn dựng / Duyệt / Hàng chờ / Kênh <tên> / Tạo kênh / Khám phá / Video mới / Thư viện / Template <tên> / Cài đặt / Tài khoản / Chuẩn bị máy, mỗi tiêu đề kết bằng "· Viderse"; mặc định `Viderse · Xưởng video`. Editor đặt `<tên project> · Bàn dựng · Viderse`. Tên kênh/template lấy từ danh sách đã tải; chưa có tên thì tiêu đề dừng ở nhãn màn, không lộ mã kênh/template.
- Máy chủ: `platform/webapp` đăng ký MIME `.webmanifest` → `application/manifest+json` và `.ico` → `image/x-icon`. Trước đó manifest bị trả `text/plain` (máy không có `mime.types` hệ thống) nên trình duyệt không cài được PWA và service worker không cache được manifest.

**GAP cho design (cần xác nhận)**

1. **Ảnh chia sẻ 1200×630 chưa có thiết kế.** Bản tạm dựng theo cách của Noverse: nền `#0B0B0D`, quầng `#FF0033` alpha .22 ở 50%/42%, logo ngang `viderse-logo-dark@2x` (bỏ nền thẻ, rộng 600), dòng "XƯỞNG VIDEO" Be Vietnam Pro 500 21px, letter-spacing .34em, màu `#8A8A94`. Cần design vẽ bản chính thức, kèm SVG nguồn trong `brand/svg/` như Noverse.
2. **Khẩu hiệu chưa có bản viết.** Tạm dùng: tiêu đề tab `Viderse · Xưởng video`; manifest `Viderse · Xưởng video YouTube`; mô tả "Bốn trợ lý quét video nổi của đối thủ, viết tiêu đề và chia cảnh, vẽ ảnh, rồi dựng và xuất video cho các kênh YouTube của bạn." Design chốt lại câu chữ.

## Gói V34 — Thư viện điện thoại, trạng thái Khám phá và kênh mới (02/11/2026)

- `MLibrary`/`MTemplate`: màn điện thoại riêng ở ≤767 px; bốn ngăn Template, Prompt ảnh, Video nền, Lớp phủ; sheet hành động/tạo mới; sửa từng mục tự lưu và hiện giờ đã lưu. Trang con ẩn thanh tab, tự quản vùng cuộn; máy tính giữ màn hiện có. Xoá template đang dùng bị khoá kèm lý do.
- Khám phá phân biệt đang quét lần đầu, bộ lọc không có kết quả (**Bỏ lọc**), tải lỗi (**Thử lại**) và mất kết nối (đếm ngược, **Thử lại ngay**). `MChannel` kênh mới có hàng thống kê ngắn, liên kết chọn video trong Khám phá và thẻ tự lấy video.
- `YoutubeDataKeyCard` nằm trong nhóm **Số liệu YouTube**, dưới **Thêm nhà cung cấp**, không còn là một nhà cung cấp AI. Có rỗng/đã lưu/sửa/khoá bị từ chối/tải lỗi/mất kết nối.
- Tạo kênh vẫn tới **Xong** khi Đạo diễn lỗi: dùng tên tạm, thẻ **Một việc cần bạn**, **Thử lại** chỉ bước đặt tên; **Mở kênh** điều hướng tới kênh vừa tạo, không bắt đợi nhận diện.
- Đã kiểm bằng local serve: điện thoại 390×844 với inset 59/34 cho các trạng thái Khám phá/Kênh mới; Thư viện và các mục sửa có ảnh đối chiếu; Số liệu YouTube và Xong có ảnh máy tính/điện thoại. Kiểm tra lại tích hợp bao gồm typecheck và các suite library/explore/channel/settings-connect/shell/checks/i18n. Không coi những ảnh này là chứng cứ toàn bộ màn điện thoại đã khớp artboard.

**Giới hạn hợp đồng / phần còn đối chiếu**

- `preview=building` chưa biểu diễn được: API template chỉ có `none|fresh|stale`. Bộ chọn video xem trước không có trường tương ứng; xuất tệp template hiện khoá kèm lý do. Không tạo trạng thái hoặc ảnh mẫu giả để che thiếu dữ liệu.
- B2 danh sách chương vẫn thiếu dữ liệu API.
- Khung `MExplore` và B2 đã có màn điện thoại riêng, chip/lọc/chọn nhiều và sheet; lượt đối chiếu toàn màn đã tích hợp bên dưới. App giữ bộ lọc ≥8 phút theo API, không đổi thành ví dụ ≥40 phút.
- Vùng cuối Tạo kênh đã kiểm lại trên nguồn tích hợp ở 390/360 px, inset 59/34 và bàn phím CSS 320 px; chưa coi đây là kiểm thử bàn phím thiết bị thật.


## Bộ điều khiển dùng chung và nhập template (02/11/2026)

- Kit dùng chung cung cấp chọn chip, vùng thả tệp/dòng tệp và tab gạch chân; hộp nhập template hiển thị kết quả thật từng tệp, giữ tệp chưa xong khi mất kết nối và không gửi lại tệp đã nhập khi thử lại.
- Đã chạy UI thật với hai gói hợp lệ và một gói hỏng: kết quả hai template được nhập, một tệp lỗi; kiểm tra điện thoại 390/360 px không tràn ngang. Sửa vòng đời hộp thoại để chọn tệp vẫn hoạt động khi React chạy lại effect; ba regression nhập một phần/huỷ/thử lại đều chạy trong StrictMode.
- Sửa menu chọn mục làm mất ảnh đại diện của kênh đang dùng: giữ hình nhận diện và ghi chú, vẫn có dấu chọn. Regression tái hiện ảnh bị mất trước sửa; toàn bộ kit đạt 231 test sau sửa. Màn Bàn dựng sẽ kiểm hình thêm trong lượt tích hợp.
- `MTemplate` đã đối chiếu lại header tên thật, hàng không đóng thẻ, ba cột cách ghép ảnh và hai cột prompt/đầu ra. Pitch hàng 59,625 px bằng canonical; bốn phần desktop không đổi pixel. Tự lưu đổi cách ghép trên template hợp lệ trả 200/revision 2 và giữ sau tải lại; template QA xoá qua API 204. Giới hạn xem trước/nguồn/xuất tệp phía trên vẫn còn, không giả dữ liệu.

## Kênh và tạo kênh điện thoại — tích hợp (02/11/2026)

- Các ngăn Video/Nguồn/Template/Hồ sơ và sheet tự động dùng màn điện thoại riêng; tạo kênh có đầu trang riêng, chọn đối thủ bắt buộc và nút tạo cố định có chừa bàn phím/inset. Dữ liệu nhận diện vẫn lấy thật từ máy chủ, không ghép ảnh minh hoạ thay kết quả.
- Đã chạy tạo kênh qua API thật, mở kênh vừa tạo và xem trường hợp Đạo diễn lỗi; báo cáo `plans/reports/viderse-mobile/mchannel-screenshots.json` ghi kích thước ảnh. Sau chuyển StyleX, lượt nguồn tích hợp đã chụp lại Hồ sơ và Tạo kênh 390/360 px: không cuộn ngang, cuối Hồ sơ không bị tab che, ô tên khi bật bàn phím CSS nằm trên nút tạo.
- Sau tích hợp: bộ kiểm tra kênh có 55 test đạt. Menu hành động đối thủ dùng đúng một nút trigger thay vì lồng hai nút, giữ liên kết nguồn/kênh hoạt động; kiểm tra chức năng liên kết đã đạt.

## Duyệt ảnh điện thoại — đã tích hợp (02/11/2026)

- Màn riêng có chọn phiên bản/so sánh với bản trước, prompt của đúng phiên bản, sửa prompt/góp ý/vẽ lại/bỏ ảnh và duyệt cả video. Dùng API hiện có; không giả ảnh tham chiếu hoặc prompt lịch sử chưa được lưu.
- Chạy UI với API thật đã xác nhận các thao tác nhận 202, trạng thái được lưu và chuyển video kế tiếp. Ảnh đối chiếu có đủ 390/360 px, inset 0 và 59/34; lượt chụp lại vẽ/hiện ảnh mới xác nhận ảnh đã tải và toàn bộ khung ảnh nằm trong vùng nhìn.
- Sửa lỗi polling máy tính/điện thoại bị ngắt liên tục khi danh sách việc làm mới: chu kỳ tải ảnh theo ID ổn định, không theo đối tượng việc thay mới. Regression tái hiện không có lượt tải ảnh, sau sửa đạt chu kỳ mong đợi.
- Sau tích hợp, bộ kiểm tra review và luật thiết kế đạt 27 test. API chấp nhận vẽ lại là thật; hoàn tất ảnh mới trong lượt đối chiếu dùng dữ liệu fixture đi qua polling thật, không phải chứng cứ provider ảnh bên ngoài đã tạo thành công.

## Khám phá và các màn phụ điện thoại — đã tích hợp (02/11/2026)

- `MExplore`/B2 có bố cục riêng: chip đối thủ, bộ lọc, chọn nhiều, sao chép link, gửi hàng loạt và các sheet thông tin/thêm nguồn/chọn kênh/lý do. Bốn sheet dùng lề ngang 16 px; đo ở 390/360 px với inset 0 và 59/34, giữ nút trong khung.
- API thật đã xác nhận thêm và liên kết đối thủ, gửi mười video (chín mới/một trùng), bỏ lọc và hồi phục tải lỗi. Clipboard được đọc lại đúng mười URL. Thiếu dữ liệu chương/preview/direct-video/job-status vẫn là giới hạn hợp đồng, không tạo dữ liệu thay.
- MNew đo chiều cao footer thực và giới hạn vùng cuộn theo inset/bàn phím; textarea nhiều link không bị co, mục cuối cuộn tới được. AuthShell chừa safe-area trên/dưới, nút đăng xuất vẫn tới được. Kiểm tra tích hợp New/Auth/Settings/Queue/design: 194 test và typecheck đạt; chứng cứ 129 PNG dùng bàn phím CSS, không phải bàn phím iOS thật.

