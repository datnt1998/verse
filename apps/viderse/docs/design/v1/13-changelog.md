# Viderse — Lịch sử thiết kế

Mỗi đợt ghi: ngày, vấn đề, quyết định, thay đổi trên artboard và tài liệu, phiên bản canvas. Mới nhất ở trên. Đợt 1–2 và 10–14 không có mục riêng (ghi trong phần Trạng thái ban đầu và trong các đợt kế tiếp).

## Đợt 29b (31/10/2026) — Chốt hỏi đáp #49 của Gói V25 <!-- r29b -->

- Tên biến đúng như Noverse (`STORAGE_DRIVER`, `S3_*`, `S3_PRESIGN_ENDPOINT` cho ô Địa chỉ kho cho trình duyệt, `BLOB_DELIVERY`, `BLOB_CACHE_*`); bỏ tiền tố `VIDERSE_` tạm.
- `s3nokey`: khối chi tiết ghi đường dẫn `secret.key` và cách cấp quyền ghi thư mục cấu hình; câu banner “…sửa quyền thư mục cấu hình”.
- Dung lượng theo nhóm: Thư viện có nhãn phụ “lớp phủ, video nền, ảnh kênh”; tweak mới `usage=partial` (máy tính và điện thoại) cho bản tạm khi bốn nhóm còn lại trả `null`.
- Huỷ chuyển: câu mới “…lần chuyển sau dùng lại, không phải chép lại”, không có nút xoá phần đã chép. Chỗ cũ: dọn bằng lệnh bảo trì, bỏ gợi ý `rm -rf` trong khối chi tiết.
- Data-contract §23.1 (`presignEndpoint`, `encryptionKey { ok, path, issue }`, `UsageGroup` có `null`, bảng biến môi trường), Gói V25, copy, ia-flows sửa theo. Canvas phiên bản 56.

## Đợt 29 (31/10/2026) — Gói V25: Lưu trữ khi chạy trên máy chủ (kho S3 dùng chung Garage) <!-- r29-s3 -->

- Theo prompt `goi-v25-design-prompt.md` của dev. Luồng, thuật ngữ và cách hiện lỗi giống Noverse Gói AN (đã đối chiếu canvas Noverse › Tuỳ chỉnh › Lưu trữ tệp).
- `Settings`: 13 giá trị `store` mới (`server` … `s3down`) cùng `sprov`, `sfail`, `skey`, `role`. Ổ máy chủ, chọn kho, form theo nhà cung cấp, kiểm tra 6 bước, 7 lỗi kèm cách sửa, xác nhận, đang chuyển (tạm dừng, huỷ hỏi lại tại chỗ), lỗi chuyển, kho đang dùng, do máy chủ đặt, chưa có khoá mã hoá, mất kết nối (banner, dòng đầu trang cho quản trị viên), dọn khi dùng kho S3, màn của thành viên. Bản máy tính (V22) không đổi.
- `MSettings`: `store=server|s3active|s3moving|s3down`; hàng Lưu trữ ghi “Garage · 18,6 GB”; trang chỉ đọc.
- Câu hỏi mở: (1) đang chuyển vẫn cho sửa video; (2) bỏ dòng “Dùng chung máy chủ lưu trữ với Noverse”, ghi GAP.
- Lệch có chủ đích so với prompt: lỗi dùng cam của Viderse (không thêm đỏ); chỗ cũ không tự xoá (khác V22, giống AN).
- Tài liệu: ia-flows W13 › Chạy trên máy chủ; data-contract §23.1; components › Lưu trữ trên máy chủ; copy › Lưu trữ trên máy chủ; dev-prompt Gói V25; hỏi đáp #49 (design hỏi dev).
- Canvas phiên bản 54.

## Đợt 28 (30/10/2026) — Duyệt hỏi đáp Khám phá #31–#37 <!-- r28-explore -->

- #31 khoá kèm một câu lý do tới V4/V6, câu hàng chờ không số: đồng ý. #37 `MExplore` dời V10: đồng ý, kèm yêu cầu cho bản một cột tạm.
- #32, #34: duyệt câu, không vẽ frame; bảng câu chuẩn mới ở [06-copy.md › Khám phá](06-copy.md#copy-explore). Cách tính “Mỗi tuần” ghi vào data-contract §12.
- #33: đệm menu theo kit 6/0 là chuẩn; artboard sửa theo; luật Menu ở components.
- #35: thêm ô **Tìm tiêu đề** (tweak `search=none|match|nomatch`).
- #36: ví dụ một kênh Anh, một kênh Việt (`@hosobian`), một mã UC.
- Sửa lỗi artboard: thẻ đầu lưới vừa có nút “Làm cho Midnight Tales” vừa có trạng thái “Chờ lượt” (từ đợt 21). Video đã làm cho kênh nào thì chỉ hiện trạng thái.
- Canvas phiên bản 53.

## Đợt 27 (29/10/2026) — Editor dễ dùng: mở ra là sửa <!-- r27-editor -->

- **Vấn đề:** editor là NLE đầy đủ nhưng người dùng vào chỉ để sửa video chưa đạt (84/100). Không có danh sách cần sửa (chỉ một chip ở timeline); bảng phải hiện X/Y/Tỉ lệ/Keyframe của ảnh trong khi việc cần làm là khớp phụ đề; thanh trái 6 icon không chữ theo kiểu NLE; thanh timeline có “V/T”, “Vùng In–Out”; Xuất luôn là nút chính, không có bước đo lại.
- **Bảng Cần sửa** mở sẵn: điểm, mục chưa đạt với từng đoạn lệch, **Khớp theo giọng cả N đoạn**, Cần đo lại, Đã đạt (gập), **Đo lại độ hoàn thiện** ở chân bảng.
- **Bảng phải theo thứ đang chọn:** đoạn phụ đề (dải Tiếng/Chữ, Khớp theo giọng, dời ±0,1 s, nghe lại, sửa chữ), cảnh (Vẽ lại, Thay ảnh, chuyển động, chuyển cảnh, Nâng cao gập), clip nền, không chọn gì.
- **Thanh trái có chữ** theo việc: Cần sửa · Cảnh · Phụ đề · Nền · Âm thanh · Media. Timeline: Tách · Xoá · Bắt dính · Đoạn lệch n/N.
- **Một nút chính mỗi lúc:** Khớp theo giọng → Đo lại → Xuất. Bấm Xuất khi chưa đạt: hỏi lại (Sửa tiếp / Vẫn xuất).
- Đoạn lệch thứ hai dời về 30:12 để khớp khung xem (phụ đề “I pointed the flashlight into the room.”).
- Tweak mới `fix`, `panel`, `focus`; `state` thêm `export-gate`.
- Spec: ia-flows W15; components › Editor; data-contract §24.2; dev-prompt Gói V13; copy; hỏi đáp #13.
- Canvas phiên bản 51.

## Đợt 26 (29/10/2026) — Siết luật hình: avatar tròn, bỏ vạch màu một bên <!-- r26-rules -->

- **Avatar tròn hết:** Thư viện › Video nền (danh sách nguồn, đầu trang kênh), Template (danh sách Lấy nền từ, hộp thoại gắn), Cài đặt › Tài khoản nền tảng và Kết nối AI (máy tính + điện thoại). Ảnh video vẫn chữ nhật.
- **Bỏ vạch trái:** chọn dòng trong danh sách nguồn (Thư viện, hộp thoại) → chỉ nền đậm hơn; vạch màu 3px ở Lấy nền từ → chấm tròn 8px trước tên; khối “Đoạn đầu bản chép lời” (Bàn dựng, Duyệt, điện thoại) và lời nhắc hạn mức trong hộp thoại vẽ lại → khối nền nhạt bo 10px.
- **Luật mới, bắt buộc:** [04-components › Luật hình](04-components.md#c-shape-rules); README nguyên tắc 11; 03-tokens (avatar 50%). Có `artboards/audit_rules.py` quét mọi artboard (0 vi phạm); dev có lệnh `rg` để chạy trong CI.
- Canvas phiên bản 49.

## Đợt 25 (29/10/2026) — Video nền dễ xem, gắn nền vào template dễ hiểu <!-- r25-backdrops -->

- **Vấn đề:** Thư viện › Video nền chỉ hiện kênh thành bảng 4 dòng, video của kênh chỉ thấy 6 ảnh nhỏ ở cột phải (kênh có 312 video); Kênh và Video lẻ tách hai tab, thêm hàng chip lọc nền tảng và banner chung. Template › Nền có 3 khối rời (Lấy từ kênh nền · Số clip mỗi video · Clip cố định), 2 nút gắn mở 2 popover, người dùng phải tự hiểu kênh, ghim, video lẻ, clip cố định khác nhau thế nào và không thấy nền một video gồm gì.
- **Thư viện › Video nền:** danh sách nguồn bên trái (Kênh nền chia theo nền tảng · Video lẻ), bên phải là trang của nguồn đang chọn: Dùng ở (template + cách lấy), lọc loại có số, **lưới 4 cột** video, nhãn tick cho video template đã chọn, video không còn xem được mờ, Hiện thêm. Banner bị chặn chỉ trong trang kênh đó. Tweak mới `kenh`.
- **Template › Nền & hiệu ứng:** một danh sách **Lấy nền từ** (kênh = **Ngẫu nhiên**, video lẻ = **Luôn có**), một nút **Gắn kênh hoặc video** mở hộp thoại (nguồn trái · video phải · Ngẫu nhiên từ mọi video / Chỉ từ video tôi chọn · loại video), thanh **Nền của một video** (42 phút = Luôn có 2:50 + ≈ 36 clip slowroads + 4 clip nightdrivefm), **Cách cắt clip** gập. Tweak `picker=none|add|choose|video` (thay `channels|videos`).
- **Số liệu khớp nhau:** Thư viện có 3 video lẻ (thêm “Phà đêm trong sương”, chưa gắn) → Video nền 7; Bilibili “1 kênh, 2 video lẻ” ở Cài đặt; Bàn dựng › Nền: “2 clip cố định” (trước ghi 3), bỏ clip “nightdrivefm · highlight” và “slowroads · phát lại” (template chỉ lấy Clip của nightdrivefm; slowroads không có phát lại). Ước lượng cũ của template ghi “1 clip cố định” trong khi gắn 2 — nay tính từ danh sách.
- Spec: ia-flows W12; components (SourceRail, BackdropGrid, BackdropSourceList, BackdropMixBar, AttachBackdropDialog); data-contract §24 (video của kênh, `usedBy`, `pinned`, ước lượng thanh nền); dev-prompt Gói V23 (việc FE + nghiệm thu); copy.
- Canvas phiên bản 48.

## Đợt 24 (29/10/2026) — Đăng nhập Bilibili bằng mã QR <!-- r24-biliqr -->

- **Vì sao:** yt-factory đã có đăng nhập Bilibili bằng mã QR và tự gia hạn (`yt-factory`: `src/server/backdrop/bilibili-qr-login.ts`, `bilibili-login.ts`, `src/app/settings/bilibili-cookie-controls.tsx`), thiết kế mới chỉ có ô dán SESSDATA.
- **Cài đặt › Tài khoản nền tảng › Bilibili:** mã QR thành cách đăng nhập chính, cookie dán tay thành lối phụ (gập sẵn). Thẻ có 4 dạng: đã đăng nhập bằng mã (tự gia hạn, **Gia hạn ngay**), cookie dán tay (nhắc quét mã để tự gia hạn), **Cần đăng nhập lại** (gia hạn thất bại, viền cam, chấm cam ở mục trái), chưa đăng nhập. Tweak `bili=blocked|saved|manual|expired|empty`.
- **Hộp thoại quét mã:** mã + đếm ngược 3 phút, 3 bước, ô trạng thái đọc bằng trình đọc màn hình; đang tạo mã · chờ quét · đã quét · hết hạn · bị từ chối · lỗi mạng · xong. Đăng nhập xong bỏ giờ chờ bị chặn. Tweak `qr=none|loading|waiting|scanned|expired|rejected|error|success`. Bấm vào mã trên artboard để xem trạng thái kế tiếp.
- **Điện thoại:** nút trên thẻ mở sheet cùng nội dung; thêm dòng **Mở app** cho trường hợp Bilibili cài trên chính điện thoại (hỏi đáp #6). Tweak `MSettings` · `bili`, `qr`.
- Icon mới `i-scan` (Iconsax Linear “scan”, `icons/scan.svg`).
- Spec: ia-flows W13; data-contract §24.1; dev-prompt **Gói V24**; components › QrLoginDialog; copy; hỏi đáp #6, #7.
- Canvas phiên bản 47.

## Đợt 23 (29/10/2026) — Chuyển về verse/apps/viderse/docs <!-- r23-verse -->

- Bộ thiết kế chuyển từ `yt-factory/docs/vidverse/` về `verse/apps/viderse/docs/design/v1/`, tổ chức như `apps/noverse/docs/design/v1`: tệp đánh số theo thứ tự đọc, README là chỉ mục, lịch sử đợt ở tệp này. Từ nay mọi cập nhật thiết kế và trao đổi với dev ở thư mục này.
- Mới: [02-stack-verse.md](02-stack-verse.md) (dựng trong Go + Postgres + React, brand qua `defineBrand`, component Viderse → `@noverse/ui`, route hash), [11-hoi-dap.md](11-hoi-dap.md) (hỏi đáp design ↔ dev, 5 mục mở), [12-brand.md](12-brand.md) + `brand/` (SVG/PNG xuất từ board `Brand`).
- `migration.md` đổi tên `09-migration-yt-factory.md`: chỉ còn là bản đồ từ giao diện cũ. Quyết định D3, D4 hết hiệu lực trong verse.
- Không đổi artboard. Canvas phiên bản 46.

## Đợt 22 (29/10/2026) — Video nền và Tài khoản nền tảng <!-- r22-sources -->

- **Vấn đề:** chưa có chỗ quản lý các nguồn bên ngoài (Twitch, Bilibili, sau này thêm). Kênh nền ở Thư viện, clip cố định dán riêng trong từng template, đăng nhập ở Cài đặt › Tài khoản phụ; trạng thái bị chặn của từng nền tảng không hiện ở đâu; điện thoại có dòng Tài khoản phụ nhưng không có màn.
- **Quy ước (chủ sản phẩm chốt):** thêm kênh hay video ở **Thư viện** (asset dùng chung); đăng nhập và trạng thái nền tảng ở **Cài đặt** (chỉ nằm trên máy này). Nền tảng mới chỉ thêm vào một danh sách, không đổi bố cục.
- **Thư viện › Video nền** (thay Kênh nền; giữ thuật ngữ, không dùng “Nguồn nền” vì “Nguồn” đã là video đối thủ):
  - Hai nhóm **Kênh** và **Video lẻ**; lọc theo nền tảng; banner khi một nền tảng đang bị chặn.
  - Một ô dán link tự nhận link kênh / link video / nền tảng. Nền tảng chưa hỗ trợ: báo rõ + **Gửi yêu cầu hỗ trợ** (D8).
  - Video lẻ có **đoạn dùng**; gắn vào template thành clip cố định.
  - Tweak `nguon=channels|videos|adding|addvideo|unsupported`.
- **Template › Nền & hiệu ứng:** Gắn kênh và Gắn video lẻ mở bảng chọn từ Video nền (dán link mới ngay trong bảng). Bỏ ô dán clip cố định riêng. Tweak `picker`. Clip cố định của “Mẫu chuẩn · Tách nền” khớp Bàn dựng › Nền (Đường đèo sương mù 0:00–1:30, Mưa trên kính xe 0:00–1:20).
- **Cài đặt › Tài khoản nền tảng** (thay Tài khoản phụ): thẻ YouTube, Twitch, Bilibili cùng một mẫu — dùng cho, trạng thái, chất lượng tải, số nguồn + link, đăng nhập. Tweak `bili=blocked|saved|empty`, `yt=ok|bot`. **Điện thoại** có màn mới (`MSettings` · `screen=accounts`).
- Spec: ia-flows W12, W13; data-contract §24; dev-prompt **Gói V23**; components › PlatformCard · SourceLinkInput · SourcePicker; copy (thuật ngữ, `unsupported_platform`); migration.
- Canvas phiên bản 46.

## Đợt 21 (29/10/2026) — Nền sau hộp thoại, trạng thái trống và dữ liệu mẫu khớp nhau <!-- r21-empty -->

- **Nền phía sau `Dialogs`, `Activity`, `System` nay là `Main` hiện tại.** Trước đó ba board này còn vẽ Bàn dựng cũ (cột trái Đang làm / Chờ bạn / Chờ lượt / Xong, cột phải kiểu cũ). Từ giờ ba board được dựng lại từ `Main` mỗi khi `Main` đổi.
  - `Dialogs` · `channel-switch` dùng chính menu đổi kênh của Bàn dựng; `assistant-confirm|unknown` hiện thẻ trả lời ngay trên ô nhắn, dùng nút của kit.
  - `Activity` · `empty`: nền là Bàn dựng không còn video đang chạy hay chờ lượt.
- **`System` chỉ còn trạng thái của Bàn dựng:** `loading` (khung chờ theo bố cục 3 cột mới), `offline` (banner cam của kit trên Bàn dựng mờ), `not-found`, `empty-workbench` (kênh After Dark Stories vừa tạo: Tự lấy video đang bật, khớp màn Kênh mới).
- **Trạng thái trống về đúng board:**
  - Kênh vừa tạo chưa có video → `Channel` · `fresh=true` (thêm After Dark Stories vào danh sách kênh; số liệu 0; Lịch đăng chỉ của 2 đối thủ liên kết). Chỉ tab Video có nội dung riêng; tab khác vẫn là dữ liệu mẫu của Midnight Tales.
  - Chưa theo dõi đối thủ nào → `Explore` · `empty=true`.
- **Dữ liệu mẫu:**
  - `Activity`: bỏ After Dark Stories (vừa “đang tạo bộ nhận diện 60%” vừa “đã xong” cùng lúc, và không có trong danh sách kênh). Nay là “Vẽ lại ảnh bìa kênh · Quiet Hours” đang chạy và “Ảnh bìa mới của Chuyện Lạ Đêm Khuya đã xong”.
  - Video radio trong hàng chờ của Midnight Tales giữ **tiêu đề gốc** “My Grandfather’s Radio Picks Up a Station That Shut Down in 1962” (trước ghi tiêu đề đã viết lại dù Đạo diễn chưa chạy). Quy tắc ghi ở ia-flows §3.1.
  - `Explore` / `MExplore`: video radio hiện “Midnight Tales · Chờ lượt” (Trinh sát tự lấy lúc 00:05) thay nút Làm; mặc định chọn nhiều đổi sang video chưa làm.
  - `NewVideo` / `MNew`: link dán mẫu là “The Lake House Guestbook Had One More Name” (video radio đã ở hàng chờ nên dán lại sẽ thành trùng).
  - “Video mới nhất của đối thủ” ở `CreateChannel`, `Onboard`, `MCreate`, `Channel` · `fresh`: đúng video đăng gần nhất của Nocturne Stories (Lake House, 1 ngày) thay video 9 ngày trước.
  - Avatar After Dark Stories một màu ở mọi nơi (nền `#1C2433`, viền `#7FB8FF`).
- Canvas phiên bản 45.

## Đợt 20 (29/10/2026) — 23 màn hình vẽ lại theo kit mới <!-- r20-screens -->

- Cả 23 artboard màn hình (15 máy tính: Onboard, CreateChannel, Explore, NewVideo, Main, Queue, Activity, Review, Editor, Channel, Library, Template, Settings, Dialogs, System; 8 điện thoại: MCreate, MExplore, MCompVideo, MNew, MMain, MReview, MChannel, MSettings) nay vẽ theo kit Noverse đổi màu. Không đổi luồng, chữ hay dữ liệu mẫu.
- Đã đổi trên mọi màn, mọi trạng thái tweak:
  - Nút chính đỏ đặc → **nút sáng** (`.vv-btn--primary`), kể cả Làm video, Tạo kênh, Duyệt N video, Làm N bước. Phím tắt trong nút sáng dùng nền xám trong.
  - Công tắc bật xanh lá → **đỏ** `--vv-accent-strong`, núm sáng, cỡ 36×22.
  - Hộp kiểm, radio, dấu tick đang chọn, thẻ lựa chọn đang chọn (viền đỏ .4, nền đỏ .06), chip đang chọn, ô ảnh đang chọn, thanh trượt → **đỏ** `--vv-accent`.
  - Link và chữ nhấn tím ở Kết nối AI → đỏ nhấn (màu tím còn lại chỉ là màu của trợ lý Đạo diễn).
  - Ô nhập 36px, focus viền + vòng đỏ; nút 36px bo 10px.
  - Bộ tăng giảm kiểu Noverse: một ô 148×36 (điện thoại 160×44), − / + hai đầu, số mono có vạch ngăn.
  - Menu nổi bo 14px, mục ≥ 34px bo 9px, bóng Noverse; hộp thoại bo 18px nền `--vv-dialog`; banner bo 12px.
  - Xoá vĩnh viễn trong hộp xác nhận (Xoá video, Xoá kênh) → **cam đặc**; Bỏ liên kết, Đăng xuất, Bỏ thay đổi → cam nhạt.
- Toast giữ hai dạng: viên cho kết quả thao tác, thẻ 300px cho cần bạn / Cần xử lý (components › Toast).
- [05-kit.md](05-kit.md): bảng “Kit cũ → kit mới” nay dùng khi sửa code đang có.
- Canvas phiên bản 42.

## Đợt 19 (29/10/2026) — UI Kit lấy từ Noverse, chỉ đổi theme màu <!-- r19-kit -->

- **Quyết định:** toàn bộ UI Kit của Viderse dùng lại của Noverse (cùng component, cỡ, bo góc, bóng, chuyển động, cách hỏi xác nhận). Viderse chỉ đổi theme màu:
  - Tím Noverse → **đỏ** Viderse (`--vv-accent*`): hộp kiểm, radio, công tắc bật, thanh trượt, focus, dấu tick, link, icon tab đang chọn.
  - Đỏ phá huỷ Noverse → **cam** Viderse (`--vv-danger*`): nút xoá vĩnh viễn, nút nguy hiểm làm lại được, lỗi.
  - Nút chính là nút **sáng**; đỏ không làm nền nút (trừ nút + tròn ở thanh dưới điện thoại).
- [tokens.css](tokens.css) và [kit.css](kit.css) viết lại theo giá trị Kit Noverse: nút, ô nhập, ô chọn 36px bo 10px; menu bo 14px; hộp thoại bo 18px; thẻ bo 16px; banner, toast, tooltip, công tắc, nhóm chọn như Noverse. Tên lớp `.vv-*` cũ giữ khi được; lớp mới: `.vv-select`, `.vv-opt`, `.vv-pop`, `.vv-token`, `.vv-range`, `.vv-checkrow`, `.vv-empty`, `.vv-btn--primary`, `--danger-soft`, `.vv-seg__thumb`. `.vv-btn--red` bỏ; `.vv-btn--danger` nay là cam đặc.
- Board `Kit`: 5 nhóm của Kit Noverse — **Xác nhận** (7 tình huống thật), **Menu** (tài khoản, ⋯ có menu con), **Select & Combobox**, **Ô nhập & nút**, **Gợi ý & trạng thái** — chữ mẫu đổi sang Viderse; cộng 3 nhóm riêng: Nền tảng, Video & thẻ, Trợ lý. Tweak `section`, `confirm`, `open`. `MKit` đổi theme theo.
- `Brand`: luật “mỗi màn một nút đỏ” đổi thành “đỏ là màu nhấn; nút chính sáng”.
- Tài liệu: mọi chỗ ghi “nút đỏ” trong ia-flows, dev-prompt, components nay ghi “nút chính”.
- 23 artboard màn hình chuyển sang kit mới ở đợt 20.
- Canvas phiên bản 41.

## Đợt 18 (28/10/2026) — Đổi tên thương hiệu thành Viderse <!-- r18-rename -->

- Tên hiển thị **Vidverse → Viderse** ở mọi artboard (logo chữ, thanh trên, Cài đặt, thư mục mẫu `~/Library/Application Support/Viderse`, “Viderse Projects”), tài liệu và canvas. Quyết định D3 đổi theo.
- Giữ nguyên để không vỡ đường dẫn và code: thư mục `docs/vidverse/`, tiền tố CSS `--vv-` / `.vv-`, tên tệp artboard, CLI `ytf`.
- Canvas phiên bản 39.

## Đợt 17 (28/10/2026) — Thư mục lưu dự án <!-- r17-storage -->

- Cài đặt › **Lưu trữ** (trước là Dọn dữ liệu) có thẻ **Thư mục lưu dự án**: chọn thư mục tuỳ ý (Chọn thư mục…) hoặc một ổ đang cắm để lưu assets của mọi video (video gốc, ảnh, MP4, project editor) thay cho thư mục mặc định. Cơ sở dữ liệu và công cụ vẫn ở máy này.
- Kiểm tra trước khi đổi (chỗ trống, quyền ghi, thư mục đồng bộ đám mây, exFAT); chọn chuyển dữ liệu cũ hoặc chỉ dùng cho video mới; đang chuyển là việc nền; bản cũ giữ 7 ngày; ổ bị rút thì tạm dừng an toàn và báo Cần xử lý.
- Điện thoại có màn Lưu trữ (trước bấm không có gì). Artboard `Settings` · `section=storage` + `store=…`, `MSettings` · `screen=storage` + `store=…`. Gói V22, data-contract §23.
- Canvas phiên bản 38.

## Đợt 16 (28/10/2026) — Trợ lý chủ động (lớp agentic) <!-- r16-agent -->

- Trước đợt này bốn trợ lý chỉ là tên cho các bước máy; tự động hoá là công tắc và luật cố định. Đợt 16 thêm lớp để trợ lý **giải thích, đề xuất, học, tự làm phần được giao và kể lại**. Luật: [ia-flows §9](01-ia-flows.md); đo thành công và kế hoạch kiểm thử: §10.
- **Vì sao?** cạnh điểm nổi trội, tiêu đề do Đạo diễn viết, điểm ảnh — lý do có số và nguồn (`Explore` · `why`, `Main` · `agent=why`, `Review` · `score`, `MCompVideo` · `sheet=why`).
- **Đề xuất** 7 loại, tối đa 5, không toast, bỏ qua thì 7 ngày không nhắc (`Main` · `agent=suggest`, `MMain` danh sách).
- **Bản tin** khi quay lại: cần bạn, máy đã tự làm (có Hoàn tác), đề xuất, kế hoạch hôm nay (`Main` · `agent=recap`, `MMain` · `screen=recap`).
- **Kế hoạch**: nhắn một câu → tối đa 5 bước bỏ tích được, hỏi lại khi thiếu thông tin (`Main` · `agent=plan`, `MMain` · `screen=plan`).
- **Học từ việc bạn sửa**: gợi ý Lưu quy tắc (viết) và Ghi chú đã học (vẽ); không học ngầm (`Main` · `agent=learn`, `Review` · `state=learn`, `Channel` · Hồ sơ kênh › Quy tắc đã học).
- **Mức tự chủ** thay công tắc Tự duyệt ảnh: Cẩn thận · Cân bằng · Tự chạy + 4 dòng chỉnh riêng, điểm ảnh của Kiểm định, giới hạn ảnh trả phí, năm việc không bao giờ tự làm (`Channel` / `MChannel` · `autonomy`, `Dialogs` · `dialog=autonomy`, Tạo kênh).
- **Hiệu quả sau đăng**: lượt xem 24 giờ / 7 ngày, “Trợ lý nhận ra” (Sơ bộ dưới 20 video), Ưu tiên kênh Đều đặn / Lượt xem (`Channel` · `tab=videos`).
- Rà độc lập sau khi dựng (25 điểm), đã sửa: dự báo Bản tin tính cả nguồn vẽ dự phòng và nói rõ phiên ChatGPT hết hạn chặn Đạo diễn; việc tự làm có trong tab Thông báo (nhãn Tự làm, Hoàn tác); hộp xác nhận Tự chạy một nội dung ở mọi nơi; ngưỡng Duyệt theo điểm chỉnh được 70–95; quy tắc đã học trên điện thoại có Xem ví dụ / Xoá; mọi “Vì sao?” mở được lý do; Khám phá nói vì sao Tự lấy chưa lấy video; số liệu sau đăng chỉ đếm video đủ 7 ngày.
- UI Kit thêm nhóm **Trợ lý** (`Kit`, `MKit` · `section=agent`) và 9 thành phần trong [05-kit.md](05-kit.md). Gói mới V18–V21, V11 cập nhật; data-contract §17–§22; quyết định mới D5–D7.
- Canvas phiên bản 36.

## Đợt 15 (28/10/2026) — UI Kit <!-- r15-kit -->

- Bộ thành phần chuẩn theo cách Noverse làm board “Bộ thành phần”, để dev dựng màn từ cùng một bộ mảnh thay vì chép lại từng artboard.
- **Mã:** [kit.css](kit.css) chứa lớp `.vv-*` cho nút, ô nhập, công tắc, chọn, menu, trạng thái, banner, toast, thẻ, hàng, hộp thoại, sheet, đầu trang và thanh dưới điện thoại. Lớp chỉ dùng biến trong [tokens.css](tokens.css) (thêm token ô nhập, popover, nền trạng thái, bo góc hộp thoại, cỡ điện thoại, vòng focus).
- **Board:** `Kit` (máy tính; tweak Nhóm: nền tảng · nút & ô nhập · chọn & menu · trạng thái · thẻ & hàng · hộp thoại) và `MKit` (điện thoại; đầu trang · điều khiển · hàng · sheet). Hai board dựng bằng chính `kit.css`, mọi mảnh bấm thử được, ma trận 5 kiểu nút × 5 trạng thái.
- **Bảng tra:** [05-kit.md](05-kit.md) ghi component React → lớp → biến thể → ARIA → ô trên board. Gói V0 làm kit trước, có trang `/dev/kit` để so ảnh.
- Canvas phiên bản 35.

## Đợt 9 (28/10/2026) — Kênh, nhận diện, nội dung đăng <!-- r10-channel -->

- **Hồ sơ kênh** làm đúng theo code (`branding.ts`): nhận diện tự sinh từ **đối thủ tham chiếu** (đối thủ liên kết đầu tiên), vẽ lại từng phần (avatar, ảnh bìa, tên) hoặc cả bộ kèm góp ý, mô tả kênh sửa được, trạng thái lỗi. **Phong cách nhận diện** chỉ đọc, dùng qua prompt có sẵn **Theo nhận diện kênh** (thay “Mặc định Viderse”).
- Khối **Khi viết tiêu đề, mô tả, tag**: ngôn ngữ, giọng văn, **đoạn cuối mô tả**, **tag luôn thêm** (mới).
- Đối thủ liên kết: **Bỏ liên kết**, nhãn Tham chiếu nhận diện; Liên kết đối thủ chọn từ kho đối thủ ở Khám phá.
- Bàn dựng › **Nội dung**: sửa tiêu đề, mô tả, tag; **Viết lại** theo phần + góp ý. **Dùng ảnh bìa của bạn** chuyển vào chip Ảnh.
- Điện thoại: Kênh đủ 4 tab (Template luân phiên, Hồ sơ kênh trước đây bấm không được), Tự duyệt ảnh, Nội dung đăng trong chi tiết video.
- Spec: ia-flows W2, W5, W11, L12, §0.2; dev-prompt V1, V4, V16; data-contract §16; components PublishInfo, ChannelProfile.
- **Rà soát đầy đủ** (cùng đợt): <!-- r11-audit -->
  - Nội dung đăng có đủ trạng thái Chưa viết · Đang viết · Chưa viết được, và **Lịch sử** (Dùng lại) trên cả máy tính và điện thoại; điện thoại có sheet Viết lại.
  - Tự duyệt ảnh hỏi xác nhận khi bật, cả hai nền tảng (trước đây công tắc máy tính không bấm được).
  - Kênh điện thoại: trạng thái nhận diện đang vẽ / lỗi, công tắc vùng điện thoại, giá trên nút vẽ lại, sheet ⋯ đối thủ (Chọn video nổi trội, Mở trong Khám phá, Bỏ liên kết có xác nhận), Tải gói đăng, Lịch đăng của đối thủ, lối sang Trợ lý AI.
  - Cột “Đang áp dụng”: Tuỳ chỉnh từng dòng, “riêng” chỉ khi khác mặc định, chữ “Nhịp 5/ngày”.
  - Kênh mới rỗng đúng 4 tab, Tự lấy đang bật; Mẫu chuẩn = Tách nền + Theo nhận diện kênh + Chữ hook.
  - Thuật ngữ **Ảnh bìa kênh** tách khỏi Ảnh bìa của video; quy tắc bỏ liên kết đối thủ; Kênh và Khám phá dùng breadcrumb, không có bộ đổi kênh.
- Canvas phiên bản 30.
- **Rà phần nhỏ** (canvas phiên bản 31): <!-- r12-small -->
  - Nhãn điểm nổi trên thumbnail chuyển xuống góc dưới trái, không che chữ hook (Khám phá, Video mới, Bàn dựng › Nguồn, điện thoại).
  - Video đã chọn: bỏ thẻ “Đã làm cho…” bị xuống dòng, đưa trạng thái vào dòng kênh; nút đỏ hai dòng không cắt tên kênh; chương đúng ngôn ngữ video gốc.
  - Thẻ video giữ chỗ 2 dòng tiêu đề; bộ lọc “Chưa làm cho kênh nào” không còn bật sẵn (lưới đang hiện cả video đã làm); cột trái ghi kênh liên kết thay vì chuỗi bị cắt.
  - **Tạo kênh** thành artboard riêng `CreateChannel` (máy tính, mở từ Kênh và bộ đổi kênh, không qua Chuẩn bị máy) và `MCreate` (điện thoại); theo dõi đối thủ mới ngay trong màn; tweak `rivals=some|none|empty`. Thay quyết định “chỉ máy tính” ở đợt trước. <!-- r13-create -->
  - **Rà luồng điện thoại** (phiên bản 34): <!-- r14-mobile -->
    - Hai loại màn rõ ràng: màn gốc (AppBar + tiêu đề lớn + thanh dưới) và màn con (NavHeader, không thanh dưới). Duyệt có AppBar như các màn gốc khác; Cài đặt thành màn con.
    - Đầu trang màn con thống nhất: ‹ hoặc ×, tiêu đề 16px căn trái (Video mới, Tạo kênh trước căn giữa).
    - Bộ đổi kênh ở Kênh mở được sheet Đổi kênh (trước bấm không có gì); Khám phá và Duyệt bỏ bộ đổi kênh cho giống máy tính.
    - Viên Việc nền ở mọi màn gốc mở sheet Việc nền (trước dẫn sang trang Hàng chờ của máy tính).
    - Tab trang Kênh không còn bị cắt (“Template luân phiê…”): 4 nhãn ngắn chia đều.
    - Nút chính trong sheet: trắng cho xác nhận, đỏ chỉ khi làm video / tạo kênh; tiêu đề sheet cùng cỡ 18px; thumbnail hàng video cùng 80×45.
  - Thanh dưới điện thoại làm lại theo Noverse: viên kính nổi, chỉ báo đỏ trượt, icon đặc khi đang mở, nút + tròn đỏ ở giữa, thu nhỏ khi cuộn (components › MobileTabBar). <!-- r13-tabbar -->
  - Tab ảnh trong chip Ảnh hai dòng (loại ảnh / tên prompt), không tràn khung.
  - Số liệu mẫu khớp nhau: Nocturne Stories đang quét lịch sử 1.284/2.000 (Khám phá, Việc nền, Kênh), video Night Shift 42:10 ở mọi nơi, Video mới dán một video chưa làm (trạng thái `duplicate` mới dùng video đã làm).

## Đợt 8 (28/10/2026) — Quy trình chuẩn: Assets tập trung <!-- r9-assets -->

- Mô hình 4 tầng **Kết nối → Assets → Kênh → Video** và chuẩn tự động hoá: [ia-flows §0.2](01-ia-flows.md). Mỗi thứ tạo một lần ở Thư viện, nơi khác chỉ chọn lại; mỗi việc tự động một công tắc ở đúng nhà.
- Bỏ vì trùng hoặc xung đột: Phong cách ảnh chung của kênh, Cách đặt vào video trong prompt (chuyển về Template thành **Cách ghép ảnh vào video**), Kết hợp / Tự viết, chip ảnh ở Video mới, prompt riêng từng ảnh ở Duyệt, mục “Không tạo ảnh” trong menu ⋯ và thẻ Phong cách kênh ở Bàn dựng.
- Ngoại lệ của một video chỉ còn ở **chip Ảnh** (Bàn dựng). Gói V16 thu gọn; V15 và data-contract §13–§14 cập nhật.
- Kho prompt: 15 prompt (7 ảnh cảnh, 8 ảnh bìa); 1 prompt ảnh cảnh có sẵn (nay là “Theo nhận diện kênh”).
- Canvas phiên bản 28.

## Đợt 7 (28/10/2026) — Prompt theo video/ảnh, Kết nối AI <!-- r8-readme -->

- Prompt ảnh theo video và theo ảnh (Gói **V16**) — đã thu gọn ở Đợt 8: chỉ còn ngoại lệ theo video ở Bàn dựng.
- **Cài đặt › Kết nối AI và Trợ lý AI** (theo mẫu Noverse, Gói **V17**, data-contract §15): nhà cung cấp (đăng nhập gói, khoá API, Ollama trên máy), danh mục model lấy từ nhà cung cấp, model cho Đạo diễn và Hoạ sĩ, nguồn vẽ và chuỗi dự phòng. Thay mục Vẽ ảnh cũ.
- Icon: bỏ ký tự ↶ ↷ − + trong nút, thay bằng `i-undo`, `i-redo`, `i-minus`, `i-add`.
- Canvas phiên bản 27.

## Đợt 6 (28/10/2026) — Kho prompt ảnh <!-- r7-prompts -->

- Cài đặt template không còn Kiểu ảnh / Kiểu ảnh bìa cố định. Template **chọn prompt từ kho**: Thư viện › **Prompt ảnh** (17 prompt mẫu: 9 ảnh cảnh, 8 ảnh bìa; 9 có sẵn của Viderse, 8 của bạn).
- Kho có danh sách, trang sửa (tên, dùng cho, cách đặt vào video / bố cục bìa, cách dùng, prompt, luật luôn giữ, ảnh mẫu, vẽ thử, template đang dùng), prompt có sẵn chỉ đọc, tạo mới.
- Pipeline ảnh giữ nguyên: lúc video bắt đầu, prompt được chép vào các trường recipe hiện có. Spec: Gói **V15**, data-contract §13, ia-flows W12, components PromptCard/PromptPicker, migration §3.
- Canvas phiên bản 26.

## Đợt 5 (28/10/2026) — Ảnh minh hoạ thật <!-- r6-readme -->

- Thay toàn bộ ảnh giả (vệt gradient) trong 22 artboard bằng **91 ảnh minh hoạ** đúng nội dung từng video và từng cảnh: ảnh bìa có chữ hook, 6 cảnh của Night Nurse và của Mall, thumbnail đối thủ, clip nền, các ô Kiểu ảnh / Kiểu ảnh bìa / Lớp phủ.
- Quy tắc dùng ảnh: [components › Ảnh trong giao diện](04-components.md#c-imagery). Ảnh nằm ở `img/`.
- Sửa dữ liệu mẫu: Chuyện Lạ Đêm Khuya có 2 template luân phiên; clip nền “Desert road at sunrise” đổi thành “Snow road at dawn” cho khớp ảnh.
- Canvas phiên bản 25.

## Đợt 4 (28/10/2026) — Mọi kênh <!-- r5-scope -->

- Không thêm trang Tổng quan. **Bàn dựng mở ở phạm vi Mọi kênh**: đầu cột trái có khối Kênh (nhịp hôm nay, chờ bạn, sẵn sàng đăng, template cần sửa); danh sách video gộp 3 kênh, có avatar kênh. Bộ đổi kênh có mục Mọi kênh. Có trên cả máy tính và điện thoại.
- Nhóm ở cột trái đổi thành **Chờ bạn · Đang làm · Sẵn sàng đăng · Chờ lượt** (+ Đã đăng khi xem một kênh). “Chờ bạn” lên đầu vì đó là việc của người dùng.
- Spec: ia-flows W5 và §6.1, dev-prompt V1, data-contract §6 (`channel=all`, `/api/channels/summary`), components `ChannelSwitcher` và `ChannelRow`.
- Dữ liệu mẫu được thống nhất: danh sách Hàng chờ trên điện thoại khớp `Queue`; tên video “Căn nhà cuối hẻm…” khớp `Review`; nhật ký Vẫn xuất ở Cài đặt không còn trùng với video đang Chờ duyệt.
- Canvas phiên bản 24.

## Đợt 3 (28/10/2026) — hành trình A→Z và cắt bớt

- `01-ia-flows.md` §0: hành trình **A Bắt đầu → B Tìm video → C Làm video → D Máy tự chạy → E Duyệt & sửa → F Đăng**; thiết lập (G) và hệ thống (H) chỉ mở khi cần. Canvas xếp đúng theo hàng A→H, artboard đánh số theo bước (A1, B1, C1, D1–D3, E1–E2, F1, G1–G3, H1–H3).
- Đã cắt khỏi YT Factory (lý do ở §0.1): trang Video đối thủ + transcript, chế độ chọn video nổi trội trong Video mới, tạo kênh 4 bước, tab Nhận diện/Cài đặt tách nhau, danh sách Dự án editor, bộ lọc Bản cũ.
- Mới: **L13 Việc nền & thông báo** và Gói **V14** (`Activity`).
- Canvas phiên bản 22: 22 artboard (15 máy tính, 7 điện thoại). Lịch đăng hiện mọi khung đăng lặp lại của đối thủ (data-contract §12).
<!-- r3-readme -->

## Trạng thái ban đầu (28/10/2026)

- 28/10/2026 sáng: thiết kế đợt 1 xong (canvas phiên bản 17).
- 28/10/2026 chiều: đợt 2 bổ sung hộp thoại, thông báo, trạng thái hệ thống, Editor, Thư viện › Dự án editor, Khám phá / Video đối thủ / Cài đặt trên điện thoại, và các trạng thái Duyệt (ảnh gốc/tách nền, bị chặn), Kênh (Bản cũ, vùng điện thoại), Khám phá (bỏ theo dõi). Canvas phiên bản 19. Chưa có gói nào được code.
- Chỉ có trên máy tính (có chủ đích): Editor, Thư viện, Cài đặt template, Lần đầu mở; trong trang Kênh: Chọn nhiều, Dặn trợ lý về kênh này. <!-- r2-status --> <!-- r11-audit -->
