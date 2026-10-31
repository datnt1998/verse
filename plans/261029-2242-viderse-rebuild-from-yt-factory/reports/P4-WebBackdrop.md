# P4 — Web Video nền (tab Thư viện › Video nền, Template › Nền & hiệu ứng)

**Trạng thái: DONE** — đã sửa xong QA P4 (M1, M4 + toàn bộ minor của web) và kiểm lại trên trình duyệt thật.

## Đã làm

- `web/src/api/backdrop.ts` — client đủ kiểu: `POST /api/sources/parse`; kênh nền (list/get/add/refresh/delete); trang video của kênh theo keyset (`kind`, `cursor`, `limit`); video lẻ (list/add/PATCH đoạn/delete/đường dẫn tệp); `GET /api/templates/{id}/backdrop/mix?lengthSec=`; `POST /api/platforms/requests`; `backdropErrorInfo` (`code`/`recovery`/`retryAt`). Khớp bản sửa wire mới nhất của backend (chỉ gửi nguyên khối `segment`; đọc `retryAt`).
- `web/src/data/keys.ts` — `backdropChannels`, `backdropChannel`, `backdropChannelVideos`, `backdropVideos`, `backdropMix` dưới `['backdrop']`.
- `web/src/data/eventKeys.ts` — thêm `backdrop` vào `SERVER_EVENT_TYPES`; `backdrop` → `['backdrop']`, `['platforms']`; `library.templates` → thêm `['backdrop']` (giữ `chosenBy`/`usedBy` mới).
- `web/src/router.ts` — route Thư viện thêm `nguon: 'channels'|'videos'` và `kenh`; `paths.library(tab, prompt, {nguon, kenh})`; `router.test.ts` cập nhật.
- `web/src/i18n/backdrop.ts` + một khoá trong `vi.ts` — toàn bộ chữ theo artboard.
- `web/src/screens/library/backdrops/`:
  - `BackdropsTab.tsx` — máy trạng thái dán link: parse → kênh / video (kèm chọn đoạn) / nền tảng chưa hỗ trợ (“Gửi yêu cầu hỗ trợ”, mỗi link chỉ gửi một lần) / link sai (câu phục hồi); deep-link `?nguon=&kenh=`.
  - `SourceRail.tsx` — nhóm theo nền tảng, avatar/tint, hàng đang đọc/tạm dừng, nhãn đã gắn.
  - `ChannelPage.tsx` — đầu kênh, dải tạm dừng nền tảng kèm giờ thử lại, chip loại kèm số, lưới ảo hoá keyset 24/trang tự tải tiếp, dấu `chosenBy`, nhãn không còn xem được, Làm mới (khoá khi tạm dừng), Xoá (409 `in_use` hiện tên template).
  - `VideosPage.tsx` — đang tải (tiến độ) / sẵn sàng / hỏng (thử lại = thêm lại), sửa đoạn (PATCH), xoá (409).
  - `SegmentPicker.tsx` — đoạn ≤ 600 giây, thanh kéo khi biết thời lượng.
  - `useChannelVideos.ts` (cache trang 1 + gom cursor, khử trùng), `fmt.ts`, `backdrops.css` (chỉ token, qua design-rules).
- `web/src/screens/library/template/`:
  - `BackdropSection.tsx` — nguồn đã gắn (Ngẫu nhiên / Luôn có), thanh trộn (cố định / từng kênh / lặp) + cảnh báo chạm trần + cảnh báo ít clip, cắt clip tự động/tay 1–40 và 10–600 giây, 8 thanh trượt hình nền + lật, gắn/bỏ lớp phủ qua `listOverlays` (đã thống nhất với LibraryWeb).
  - `BackdropPicker.tsx` — rail kèm nhãn đã gắn, thêm bằng link, chế độ kênh (RadioCard + chip loại + lưới ghim ảo hoá), chi tiết video kèm đoạn + link “Đổi đoạn ở Thư viện”.
- `styles/viderse.css` — token mới `--vv-plat-twitch/bilibili`, `--vv-fixed*`, `--vv-loop-bg`, `--vv-reading*`, `--vv-scrim*`, `--vv-s-17/-45`.

## Kiểm

- `npx tsc --noEmit` sạch cho các tệp của mảng.
- `backdrops.test.tsx` 5/5: link chưa hỗ trợ → nút gửi yêu cầu, bấm lần hai không gửi lại; lưới ảo hoá giữ số hàng DOM có giới hạn; tiến độ tải; chọn ghim trong bộ chọn → `pinned` khi lưu; cảnh báo ít clip.
- Cả bộ web 262/263 xanh; 1 lỗi còn lại không thuộc mảng này: `screens/library/library.css` còn `border-radius: 14px` (design-rules, file của LibraryWeb — đã báo).
- Trình duyệt thật: `viderse serve` ở `127.0.0.1:18742` (DB `viderse_p4_bkweb`, HOME riêng), `#/library?tab=backdrops` ở 1440×900 và 390×844 không tràn; thêm kênh Twitch thật `nightdrivefm` qua thanh dán link (1 parse + add + quét).
- Lỗi bắt được khi chạy thật và đã sửa: sau khi thêm kênh, điều hướng sang `kenh=channels` thay vì id kênh mới.

## Sửa theo QA `QA-P4-Backdrops.md`

- **M1 (major) — `.vbd-scroll` không bao giờ cuộn, ảo hoá chết.** Nguyên nhân: `.vbd-tab` cao theo nội dung nên `.vbd-cols`/`.vbd-scroll` không có khung cao hữu hạn. Đã đổi `.vbd-tab { flex: 1 1 0%; min-height: 0 }`, `.vbd-cols { grid-template-rows: minmax(0,1fr); align-items: stretch }`, `.vbd-rail { max-height: 100% }`; mobile chặn thêm `.vbd-scroll { max-height: calc(100vh - 260px); min-height: 320px }`.
  - **Chứng minh trong trình duyệt thật** (DB riêng, 340 video trong một kênh): `document.scrollHeight - clientHeight = 0` (trang không cuộn), `.vbd-scroll` 651/1334 → tự cuộn; cuộn đáy lặp lại nạp đủ 24 → 48 → … → **340/340**, số thẻ DOM cao nhất **32**, hiện tại 20–24 ⇒ ≤ 200 và loadMore bật đúng theo scroll. Mobile 390×844: `.vbd-scroll` 584/… tự cuộn, 5 thẻ DOM, không tràn ngang (`scrollWidth = clientWidth = 390`).
- **M4 (major) — kênh đang bị chặn vẫn hiện trạng thái đang đọc thay vì danh sách.** `channelReading()` giờ trả false khi kênh còn trang chờ nhưng đang bị chặn; `ChannelPage` tính `paused` (kênh hoặc nền tảng) trước và ẩn trạng thái đọc. Kiểm thật (đặt `platform_state.paused_until` cho Twitch): vẫn hiện **24 thẻ + “Đang hiện 24 / 340”** kèm banner “Twitch tạm chặn tới 07:17 · đã đọc 338/340 video”, rail ghi “338 video · chưa gắn”, nút Làm mới bị khoá kèm lý do.
- Minor đã sửa: icon `ActionButton` truyền node (`<Icon name="refresh"/>`) thay vì chuỗi; tiêu đề kênh 18px (`--vv-s-18`); nhãn thời lượng ở góc phải dưới thumb (bỏ khỏi dòng phụ); tag `chosenBy` thành chấm tròn góc phải trên; xoá 2 khoá i18n không dùng (`addFail`, `removeFail`) và thêm gợi ý phục hồi cho link sai; chip loại `flex-shrink: 0` + wrap trên hẹp, nhãn thanh trộn ellipsis; hàng rail mobile `flex: 0 0 auto` (không kéo giãn), spinner rail hiện lại; tag “Ngẫu nhiên/chỉ từ N video” nằm cạnh tên nguồn; nhãn “Cập nhật hôm nay HH:MM”; cảnh báo chạm trần/ít clip có nút **“Tăng mỗi clip lên N giây”** (N = ceil(cần/40), kẹp 10–600); mọi link nội bộ render `#${paths.*}`; thêm `@media (pointer: coarse)` nâng mục chạm ≥ 44px.

## Kiểm lại (sau QA)

- `npx tsc --noEmit`: sạch.
- `backdrops.test.tsx` 5/5; full web 262/263 (lỗi còn lại ở `library.css` của LibraryWeb).
- Trình duyệt thật trên build cuối: 1440×900 và 390×844 không tràn ngang; M1/M4 nêu trên; mở hộp Gắn, chọn kênh → “Chỉ từ video tôi chọn” → ghim 1 video → lưu ⇒ section hiện “chỉ từ 1 video đã chọn”, thanh trộn “1 clip · từ 1 video đã chọn / 41:39 cuối nền lặp lại”, cảnh báo “Cần 42 clip … Tăng mỗi clip lên 63 giây”.
- Dọn: đã `stop` server và `dropdb viderse_p4_bkweb`.

## Còn thiếu

- Không còn trong phạm vi web. Danh mục kênh Twitch thật vẫn có thể trả 0 clip khi quét ẩn danh — theo dõi ở smoke backend (BackdropBackend).
