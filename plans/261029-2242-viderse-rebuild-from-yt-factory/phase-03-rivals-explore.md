---
phase: 3
title: "Trinh sát: theo dõi đối thủ, quét kênh, Khám phá"
status: in-progress
priority: P1
effort: 44h
dependsOn: [2]
---

# Phase 03 · Trinh sát: đối thủ, quét kênh, Khám phá

## Mục tiêu

- Theo dõi được kênh đối thủ: quét video mới theo nhịp và quét lịch sử có giới hạn.
- Có khung đăng lặp lại (§12).
- Màn Khám phá đi từ "thấy video hay" tới "video vào hàng chờ" chỉ bằng hai lần bấm. Lần bấm thứ hai nối vào `POST /api/clone/plan` ở phase 05; ở phase này nút hiện trạng thái bị khoá và nói rõ lý do.

Gói design: **V5** Khám phá, **§12** khung đăng.

## Đầu vào (yt-factory)

| Nguồn | Nội dung cần port |
|---|---|
| `src/server/youtube/{client,channel,channel-pages,upload-time,parse}.ts` + `fixtures/` | Client InnerTube: trang đầu (home → tab Videos → chip "Latest"), token tiếp theo (≤ 64 KiB), đổi handle thành `UC…`, giờ đăng chính xác, bộ điều tốc yêu cầu toàn cục |
| `src/server/channels/scan-repository.ts` (+ test 3.0k dòng) | Lease có fence, phát lại cursor, việc quét lịch sử và quét mới, mức quét lịch sử 150–10.000 |
| `src/server/videos/{parse,service}.ts` | Xếp loại lỗi yt-dlp (`login_required`, `cookies_failed`), chọn phụ đề |
| `src/app/channels/*`, `components/channels/{video-board,channel-scan-status,posting-schedule}.tsx` | Hành vi UI cũ, dùng để tham chiếu |
| API `channels/[id]/scan` (GET/POST start·pause·resume·cancel/PATCH), `channels/[id]/posting-times` | Hình dữ liệu cũ |

Design: 01-ia-flows (Trinh sát, Khám phá), 07 §12, 08 V5; artboard `Explore` (`preview`, `select`, `rivalMenu`, `scan=running|paused|limited|done`, `empty`), `MExplore` (làm ở phase 09).

## Việc

### Backend

- **`internal/youtube`:**
  - port client InnerTube sang Go, giữ các loại lỗi `blocked`, `cursor_invalid`, `not_found`, `unexpected_page`;
  - bộ điều tốc toàn cục: trong bộ điều tốc không tự thử lại; phải chờ thì trả `retryAt`;
  - mọi đầu vào parser lấy từ fixtures của yt-factory, chép sang `testdata/`.
- **`internal/scan`:**
  - lập lịch trong process theo `rescanHours` (1–24, mặc định 1);
  - quét lịch sử theo trang;
  - lease và fence lưu trong Postgres để phục hồi;
  - mỗi lượt quét ghi một dòng `background_tasks` và phát sự kiện `scan.*`.
- **Migration `0000N_rivals.sql`:**
  - `rival_channels`, `rival_videos` (index `(channel_id, published_at desc)` và tìm kiếm tiêu đề bằng `pg_trgm`), `scan_state`, `scan_schedule`;
  - không có bảng lease riêng: lease là cột của `scan_state`.
- **API:**
  - `GET /api/rivals`, `POST /api/rivals` (dán link hoặc handle; lỗi handle trả mã để UI hiện ngay dưới ô);
  - `GET /api/rivals/:id/videos?cursor=&q=&sort=` (phân trang bằng cursor, mỗi trang ≤ 200);
  - `GET|POST|PATCH /api/rivals/:id/scan`;
  - `GET /api/rivals/:id/posting-slots` (§12, mọi khung lặp lại).
- **CLI:** `viderse rival add|list|scan|posting-slots`, lỗi trả theo khuôn JSON chung (`error-envelope`).

### Web

- `screens/explore`:
  - danh sách video ảo hoá theo cửa sổ cuộn;
  - thumbnail cỡ `mqdefault`, `loading="lazy"`, có `width`/`height` để không giật bố cục;
  - xem trước video ngay tại chỗ, không có trang chi tiết;
  - chọn nhiều, menu đối thủ, trạng thái quét (`running|paused|limited|done`), rỗng;
  - khung đăng hiển thị theo §12.
- Cập nhật qua SSE: video mới chèn vào đầu danh sách, giữ nguyên vị trí cuộn và lựa chọn.

## Quyết định triển khai (30/10/2026)

- **Schema** (`00006_rivals.sql`): ngoài 4 bảng đã nêu có thêm `scan_jobs` (việc quét lịch sử và kiểm tra video mới; mỗi kênh tối đa một việc còn sống mỗi loại). Lease toàn cục của luồng quét dùng dòng `youtube` của `platform_state` (có từ 00004); fence từng kênh là cột `lease_*` của `scan_state`. Điểm nổi trội lưu ở `rival_videos.score`, tính lại cả kênh trong cùng giao dịch khi mức thường đổi.
- **Thêm đối thủ** chỉ đọc trang đầu tab Videos (khoảng 30 video) để tính mức thường. Quét lịch sử do người dùng bấm (“quét thêm lịch sử khi cần”), giới hạn 150–10.000, mặc định 2.000.
- **Kiểm tra video mới** theo `clone.rescanHours` (settings, 1–24, mặc định 1) và theo menu ⋯. Chỗ đổi `rescanHours` trên UI đi cùng cài đặt Hàng chờ (§3, phase 05).
- **Luồng quét** là một goroutine trong `serve`; khi rảnh đọc giờ đăng chính xác và cờ phụ đề của 30 video mới nhất mỗi đối thủ (một lời gọi `/player` mỗi video). Khi không có việc đến hạn thì ngủ tới mốc gần nhất; chỉ còn nhịp heartbeat 15 giây gia hạn lease (một lần ghi) và đọc lại mốc kế tiếp, để thấy việc do process khác tạo. Đo khi rảnh: 4 lần ghi DB mỗi phút.
- **Khung đăng §12** là thuật toán mới (yt-factory chỉ có khung nhiều nhất): ≥ 3 video trên ≥ 2 tuần là một khung; độ đều `weekly` (mọi tuần), `most_weeks` (≥ 75 % số tuần), `new` (xuất hiện lần đầu trong 3 tuần gần nhất), còn lại `biweekly`; bảng 7 × 12 theo khối 2 giờ. “Mỗi tuần” = (số video có giờ − 1) / độ dài thật của mẫu tính theo tuần (tối thiểu 1 tuần); chia cho số tuần lịch mẫu chạm tới làm kênh 3 video/ngày ra 15 thay vì ~22 (hỏi đáp #34).
- **Nút cần Kênh/Hàng chờ** (Làm cho kênh, chọn kênh, Làm N video, Liên kết với kênh của tôi, lọc trạng thái) hiện bị khoá kèm lý do cho tới phase 04/05.
- **Điện thoại**: phase này chỉ bảo đảm dùng được (một cột, không tràn); khớp `MExplore` làm ở phase 09.

## Kiểm thử

- Go:
  - parser InnerTube chạy trên mọi fixture;
  - chuyển ca test từ `scan-repository.test.ts`, `channel-pages.test.ts`, `upload-time` và `parse` của videos;
  - fence: hai lượt claim cùng lúc thì chỉ một lượt ghi được;
  - phục hồi: `kill -9` giữa lúc đang quét, chạy lại thì đọc tiếp từ cursor;
  - benchmark `GET /api/rivals/:id/videos` với 20.000 dòng, p95 ≤ 100 ms.
- Web: test danh sách ảo hoá (số hàng DOM có giới hạn), test chọn nhiều, test cách hiện từng loại lỗi.
- Smoke thật: thêm 2 kênh đối thủ thật, quét mới và quét lịch sử 150 video, xem khung đăng, rồi tạm dừng / chạy tiếp / huỷ.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V5** (08-dev-prompt), trừ bước "vào hàng chờ" (bước này nối ở phase 05).
- Kênh bị chặn hoặc trang lạ: hiện ở Việc nền và Cần xử lý kèm câu theo 06-copy, không âm thầm bỏ qua.
- Cuộn 20.000 video: DOM ≤ 200 hàng, không có frame dài trên 50 ms.

## Tiến độ (30/10/2026)

- [x] Backend: `youtube` (tham chiếu kênh, giờ đăng chính xác), `scan` (luồng quét một lease, fence từng kênh, quét lịch sử và kiểm tra video mới, tạm dừng/tiếp tục/huỷ, phục hồi sau sập), `rivals` (điểm nổi trội, phân trang cursor, khung đăng §12), store Postgres, migration 00006; CLI `viderse rival`.
- [x] Web: Khám phá trên máy tính theo artboard `Explore`; QA ảnh cặp 3 vòng, còn 0 lỗi lớn, lỗi nhỏ còn lại là hỏi đáp #33.
- [x] Review code: luồng quét 6 vòng, API đối thủ 3 vòng, còn 0 lỗi mở.
- [x] Smoke trên binary thật với 2 kênh YouTube thật: thêm đối thủ, quét lịch sử, tạm dừng/tiếp tục/huỷ, `kill -9` rồi chạy lại, lịch đăng, đo lúc rảnh.
- [x] Ghi `10-gap-closure.md` (V5) và hỏi đáp #31–#37.
- [ ] Mở các điều khiển đang khoá khi có Kênh (phase 04) và Hàng chờ (phase 05); `MExplore` ở phase 09.

## Rủi ro

- **YouTube đổi trang.** Lỗi `unexpected_page` báo to. Fixtures giúp sửa nhanh. Đọc trang công khai vẫn dùng chung client này cho D5 (phase 08).
- **Bị chặn vì quét nhiều.** Bộ điều tốc chung cho mọi yêu cầu; hết hạn mức thì chờ `retryAt`, không thử dồn.
