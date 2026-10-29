---
phase: 3
title: "Trinh sát: theo dõi đối thủ, quét kênh, Khám phá"
status: pending
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

## Rủi ro

- **YouTube đổi trang.** Lỗi `unexpected_page` báo to. Fixtures giúp sửa nhanh. Đọc trang công khai vẫn dùng chung client này cho D5 (phase 08).
- **Bị chặn vì quét nhiều.** Bộ điều tốc chung cho mọi yêu cầu; hết hạn mức thì chờ `retryAt`, không thử dồn.
