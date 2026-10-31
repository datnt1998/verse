# Web Kênh — màn Kênh, Tạo kênh, Onboard, Bộ đổi kênh, mở khoá Khám phá

## Tệp đã làm

**Màn Kênh (`web/src/screens/channel/`)** — `ChannelScreen.tsx`, `ChannelNewScreen.tsx`, `ChannelHeader.tsx`, `ChannelSideRail.tsx`, `ChannelAside.tsx`, `VideosTab.tsx`, `SourcesTab.tsx`, `TemplatesTab.tsx`, `SettingsTab.tsx`, `IdentityBlock.tsx`, `CreateStep.tsx`, `DoneStep.tsx`, `ScheduleCard.tsx`, `PaceStepper.tsx`, `applied.ts`, `schedule.ts`, `templateLine.ts`, `fmt.ts`, `breakpoint.ts`, `channel.css`; test `channels.test.tsx`, `applied.test.ts`, `schedule.test.ts`, `templateLine.test.ts`.

**API / i18n / dữ liệu** — `web/src/api/channels.ts`; `web/src/i18n/channels.ts` (+1 import + khoá `channels` trong `i18n/vi.ts`); khối Kênh trong `data/keys.ts`, `data/eventKeys.ts`; `lib/channelTint.ts`, `lib/lastChannel.ts`, `lib/onboard.ts`.

**Shell** — `shell/AppShell.tsx` (nguồn kênh thật cho Bộ đổi kênh + khung Onboard), `shell/ChannelSwitcher.tsx` (bỏ số liệu giả, chỉ kênh thật), `shell/AppHeader.tsx`, `shell/MainNav.tsx`, `shell/MobileShell.tsx` (đích mục "Kênh" theo kênh đang có).

**Khám phá (chỉ mở khoá)** — `screens/explore/{PreviewPanel,RivalCard,FilterBar,VideoGrid,BulkBar,ExploreScreen}.tsx` + khoá i18n liên quan.

## Quyết định

- **Đích mục "Kênh"**: `channelsHref(channels)` — chưa có kênh → `#/channels/new`; có kênh → kênh xem gần nhất còn tồn tại (`localStorage 'viderse:last-channel'`), không thì kênh đầu. Mở kênh ghi lại id. Nhờ vậy lần mở đầu tiên vào đúng Onboard, người đã có kênh không bị đẩy về màn Tạo kênh.
- **Luật Onboard (W1→W2)**: `SetupScreen` tự chuyển `#/channels/new` khi máy cài xong (đã có sẵn `data.complete` + hẹn 1,2 s). Ở `/channels/new?step=create` mà **chưa có kênh nào** và chưa bấm Huỷ lần nào → `AppShell` bỏ nav, `ChannelNewScreen` vẽ đầu Onboard "Viderse · Lần đầu mở" + breadcrumb như artboard. **Huỷ** ghi cờ `onboard-dismissed` (localStorage) rồi về Bàn dựng; mở lại Tạo kênh là màn thường có nav — không lặp người dùng. Không tự redirect từ route khác để tránh vòng.
- **Hộp chọn đối thủ** là popover neo dưới nút đúng artboard `linkOpen` (role=dialog, hàng role=checkbox, Esc đóng bằng `useEscapeLayer`, bấm ngoài đóng, focus trả về nút mở, hàng đầu chọn được tự nhận focus). Đo chỗ trống: không đủ 320 px thì lật lên trên, `max-height` theo viewport trừ thanh tab điện thoại; danh sách cuộn trong, chân "Liên kết N" luôn thấy.
- **Dòng phụ template**: dùng `prompts.summary` backend đã soạn ("Prompt … · Tách nền · bìa …") + "N ảnh"; không tự ghép lại style/bìa để khỏi lệch Thư viện và tránh lặp "Tách nền".
- **Dòng phụ đối thủ đã liên kết**: subscriber · nhịp `perWeek` (posting-slots) · số video đã quét (scan-state) hoặc "đang quét lịch sử x/y" + chip "Đang quét lịch sử" · "xem … trước". Mỗi hàng đọc scan/posting riêng theo id; payload lạ/404 thì rơi về số của danh sách đối thủ, không vỡ.
- **Bỏ đối thủ cuối**: server trả `autoPickPaused` → hiện lý do `sources.pausedReason`; công tắc Tự lấy + nhịp 1–50 lưu ngay khi đổi (`PATCH`), lỗi hiện `role=alert` cạnh khối.
- **Nhận diện**: avatar/banner lấy `/branding/{part}?v=`, tải `?download=1`; đang vẽ khoá nút vẽ lại (kèm câu lý do hiển thị, không chỉ tooltip), tiến độ `%` từ task/SSE; lỗi `role=alert` + "Thử lại" + "Tự thử lại lúc HH:MM" khi quota; gợi ý chi phí "Mỗi lần vẽ dùng ảnh trong hạn mức ChatGPT…" trước khi vẽ lại.
- **Hồ sơ kênh** autosave on blur như Template: giữ chữ đã gõ khi 400, lỗi theo trường; footer ≤1000 có đếm, tag ≤10 dạng chip, ngôn ngữ segmented en|vi|auto; xoá kênh qua `confirmDeleteChannel` với `runningCount: 0`.
- **Khám phá**: đã có ≥1 kênh → chọn kênh trong PreviewPanel, menu đối thủ "Liên kết với kênh…" mở hộp chọn kênh; nút hàng chờ ("Làm video", "Làm N video", BulkBar) vẫn khoá với lý do "cần hàng chờ".
- **Bộ đổi kênh**: chỉ kênh thật (tên, avatar/initials, màu theo id); bỏ dòng "Hôm nay 3/5", pace/waiting vì cần job.
- **Cột trái**: danh sách kênh thật + "Mọi video của N kênh"; Thư viện dùng chung lấy đếm thật (`templates`, `image_prompts`, `overlays`, **Video nền = kênh nền + video lẻ**), chưa tải xong thì ẩn số chứ không hiện 0 giả.
- **Tab Video**: trạng thái `fresh` đúng artboard ("Kênh chưa có video" + hai thẻ dẫn Khám phá/Nguồn), không bảng job, không số liệu giả.
- **Mục đang mở của tab** đồng bộ query `?tab=`; tab có đếm thật (đối thủ liên kết, template).

## Lệch design / khoảng trống artboard

- **Hộp xác nhận bỏ liên kết trên desktop chưa được vẽ** trong `Channel.dc.html`; dùng `confirm` của kit với tông `dsoft` (cam nhạt) + copy sheet MChannel: tiêu đề "Bỏ liên kết <tên>?", thân nêu hậu quả và nguồn tham chiếu kế tiếp. Ghi để bổ sung artboard.
- `Onboard` mobile không vẽ; ở 390 khung Onboard vẫn là bản desktop một cột (không nav), dùng được, chưa khớp artboard di động (ngoài phạm vi đợt).
- Ở ≤700 cột trái Kênh xếp **trên** nội dung (không có artboard mobile): không tràn ngang, song danh sách kênh + thư viện khá dài trước nội dung. Nếu cần, đợt mobile làm bộ chọn kênh gọn.
- Artboard ghi "Thử lại lúc HH:MM"; UI dùng "Tự thử lại lúc HH:MM" cho rõ là hệ thống tự thử (cùng nghĩa, cần chốt copy).
- Trạng thái task nhận diện do seed tay có `owner_pid` cũ: màn Hồ sơ kênh có thể ghép dòng "Bị gián đoạn khi app khởi động lại · 75%" khi state vẫn `generating` — đúng dữ liệu lệch, không phải copy bịa; chạy thật thì task do tiến trình hiện tại sở hữu.

## Khoá + lý do (không bịa số)

- Hàng chờ: "Làm video mới nhất", "Video mới cho kênh", "Làm video" ở Khám phá, BulkBar — lý do "Cần hàng chờ để làm video. Hàng chờ chưa mở." (có kênh cũng vẫn khoá).
- Cột phải Hồ sơ kênh: Tự lấy/Tiêu đề/Mô tả/Template/Trợ lý AI mở "Tùy chỉnh" (cùng màn), không giả lập.
- Mức tự chủ/Ưu tiên (§20) không dựng.

## Test

- `npx tsc --noEmit` sạch; `npx vitest run --maxWorkers=2`: **27 tệp, 272 test xanh** (gồm 29 test kênh/nav).
- Test mới đáng kể: tạo kênh bắt buộc ≥1 đối thủ (`aria-disabled` + lý do, chọn xong mở); Hồ sơ autosave blur giữ chữ khi 400 + lỗi trường; bỏ đối thủ cuối → `autoPickPaused` + lý do; nhận diện đang vẽ khoá vẽ lại, lỗi quota hiện giờ thử lại; popover: `aria-expanded`, Esc đóng + focus về nút, hàng đã dùng kênh khác bị chặn kèm lý do; dòng phụ đối thủ có nhịp + tiến độ quét + chip; `channelsHref`/nav; dòng phụ template.
- Build: `index` 498,44 kB → **149,87 kB gzip** (dưới trần 250 kB); nhắc lại test/tsc đều qua sau lần build cuối.

## Kiểm trên trình duyệt thật (headless Chromium, DB/HOME/cổng riêng 18743, `viderse serve`)

- Seed 3 kênh + 3 đối thủ + nguồn + scan-state + lịch đăng; **không gọi API sinh nhận diện** (không tốn phí). Task `identity`/`channel_scan` chèn thẳng DB để thấy trạng thái đang chạy.
- 1440×900: Kênh tab Nguồn (dòng phụ "1,24 Tr người theo dõi · 1 video/tuần · 1.284 video đã quét · xem 1 ngày trước"), popover neo đúng phải nút (lệch 0 px, cách 6 px, rộng 340), Esc đóng + focus về nút, chặn "đang dùng cho Kênh 1/Kênh 3" kèm title, liên kết Silent House vào Một Phim thành công rồi hiện hàng + huy hiệu tham chiếu, bỏ liên kết cuối hiện lý do tạm dừng + toast; Hồ sơ: đang vẽ khoá nút + 75%, lỗi quota `role=alert` + "Tự thử lại lúc 07:16" + nút Thử lại; Template rỗng đúng trạng thái; nút "Xoá kênh" cao 30 px, `white-space: nowrap`, một dòng.
- 390×844: Kênh (Video/Nguồn/Hồ sơ), Tạo kênh, Khám phá — `documentElement.scrollWidth = 390` (không tràn ngang); popover sau khi sửa nằm trọn 23–363 px, chân "Liên kết 1" trên thanh tab; tiêu đề thẻ xuống dòng thay vì cắt chữ.
- Onboard lần đầu (DB trống riêng 18744): sau khi máy xong, `#/channels/new` mở khung riêng không nav, đầu "Viderse · Lần đầu mở", breadcrumb "Kênh của tôi / Tạo kênh", nút tạo khoá + lý do; bấm **Huỷ** về Bàn dựng, mở lại Tạo kênh là màn thường có nav (không vòng).
- Khám phá: PreviewPanel có "Làm cho kênh" chọn Kênh 1/Kênh 3/Một Phim; menu đối thủ "Liên kết với kênh…" mở hộp chọn kênh; "Làm video" vẫn khoá với lý do hàng chờ.

## Việc còn lại / ngoài phạm vi

- Bảng job + BulkBar hành động (hàng chờ đợt sau); mức tự chủ/Ưu tiên §20.
- Artboard mobile Kênh/MCreate; hộp xác nhận bỏ liên kết desktop chưa có trong artboard.
- Kiểm `pointer: coarse` trên Chromium headless không mô phỏng được (`matchMedia` vẫn fine) nên quy tắc 44 px chỉ đối chiếu CSS/kit, chưa đo bằng trình duyệt.

Status: DONE — màn Kênh 4 tab, Tạo kênh/Onboard, Bộ đổi kênh, mở khoá Khám phá chạy trên server thật đúng dữ liệu; các khoảng trống còn lại là artboard mobile/desktop chưa vẽ và hàng chờ ngoài phạm vi.
