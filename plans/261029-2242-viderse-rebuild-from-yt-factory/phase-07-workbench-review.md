---
phase: 7
title: "Bàn dựng, Duyệt, Việc nền và thông báo, Vì sao? và Bản tin"
status: pending
priority: P1
effort: 60h
dependsOn: [5, 6]
---

# Phase 07 · Bàn dựng, Duyệt, Việc nền, Vì sao?

## Mục tiêu

Người dùng mở một màn là biết việc nào đang chờ mình.

- **Duyệt:** nhanh, đủ ngữ cảnh để quyết.
- **Thông báo:** chỉ khi máy cần người.
- **Trợ lý:** giải thích được vì sao nó làm như vậy, và báo lại những gì đã làm khi người dùng vắng.

Gói design:

- **V1** Bàn dựng;
- **V3** Duyệt;
- **V14** Việc nền và thông báo;
- **V16** Ngoại lệ ảnh của một video;
- **V18** Vì sao? và Bản tin;
- §5, §6, §7, §11, §14, §16 (nội dung đăng), §17, §22.

## Đầu vào (yt-factory)

| Nguồn | Nội dung |
|---|---|
| `components/clone/{job-card,clone-board,job-detail-panel,terminal-panel}.tsx` | Tham chiếu hành vi: `terminal-panel` → `ProgressLog` (giữ `cloneTerminal` làm nguồn câu) |
| `components/clone/{review-board,review-row,image-strip}.tsx`, `review-keys.test.tsx` | Phím tắt Duyệt và thao tác ảnh |
| `components/clone/{job-table,bulk-clone-bar,job-notifications}.tsx` | Chọn nhiều, gộp thông báo |
| API `clone/[jobId]/images/[imageId]` (op + `expectedRevision`, `variant=cutout`), `cover`, `backdrop`, `thumbnail`, `bulk`, `download` (zip ≤ 50 id) | Hình dữ liệu cũ |
| `src/server/tools/subject-cutout.ts` (macOS Vision) | Ảnh tách nền cho `view=cutout` |

Design:

- 07 §5, §6, §7, §11, §14, §16, §17, §22;
- 08 V1, V3, V14, V16, V18;
- artboard:
  - `Main` (`scope`, `stage`, `panel=agents|content|source|backdrop`, `promptPop`, `meta`, `agent=recap|why`);
  - `Review` (`state`, `view=full|cutout`);
  - `Activity` (`view=running|inbox|toasts|system|settings|empty`);
  - `Dialogs` (`redraw-all`).

## Việc

### Backend

- **Bàn dựng (§6):**
  - `GET /api/clone?channel=<id>|all` có nhóm `paused` và `subline`;
  - ở Mọi kênh, nhóm `queued` chỉ trả 3 video đầu kèm `queuedTotal`, không trả nhóm `posted`;
  - chọn sẵn video mặc định (`pickDefaultJob`).
- **Nội dung đăng (§16):**
  - `PATCH /api/clone/:id/metadata`;
  - `POST /api/clone/:id/metadata/rewrite` chỉ chạy lại các phần đã chọn, không tốn ảnh;
  - bản cũ lưu vào Lịch sử.
- **Duyệt:**
  - thao tác ảnh có `expectedRevision`: duyệt, vẽ lại kèm góp ý bằng chữ, dùng ảnh bìa của bạn, ảnh tách nền;
  - "Duyệt cả video & dựng" (đổi tên từ "Duyệt & xuất", hành vi giữ nguyên).
- **Ngoại lệ ảnh của video (§14):** hai lớp template → video; vẽ lại tất cả phải qua hộp xác nhận có số ảnh.
- **Chọn nhiều (§7):** `bulkEligibility(jobs)` tính ở server; `POST /api/clone/bulk`.
- **Việc nền và thông báo (§11):**
  - `background_tasks` hợp nhất mọi việc dài;
  - `notifications` có `kind`, `batchKey`; cùng `kind` trong 10 giây thì gộp lại;
  - `GET /api/activity`, `POST /api/notifications/read`;
  - cài đặt thông báo: theo loại × (Trong app / Hệ thống), Không làm phiền.
- **Lý do và Bản tin (§17, §22):**
  - mỗi quyết định của máy ghi một `Reason` (chọn video, xếp hàng, chấm điểm, vẽ lại);
  - Bản tin tổng hợp những gì máy đã làm từ lần mở trước;
  - `insight.Recap` dùng `core/llm` để viết câu, còn dữ kiện lấy từ DB.

### Web

- `screens/workbench` (Bàn dựng):
  - danh sách trái theo nhóm;
  - chi tiết gồm `AgentCard` × 4, `ProgressLog`, `LiveStudio` và các panel `agents|content|source|backdrop`;
  - chip **Ảnh** mở `promptPop` (V16);
  - **Nội dung** với các trạng thái `meta=pending|writing|failed|ai|edited|rewrite|history`.
- `screens/review` (Duyệt):
  - phím tắt theo `review-keys`;
  - **cập nhật lạc quan**: duyệt hoặc vẽ lại thì UI đổi ngay, lỗi thì hoàn lại và báo bằng toast;
  - tải trước ảnh của video kế tiếp;
  - chế độ `full|cutout`.
- `shell/ActivityPopover` (448px) gồm tab Đang chạy và Thông báo:
  - toast viên cho kết quả thao tác;
  - toast dạng thẻ 300px cho Cần bạn và Cần xử lý;
  - không có toast cho tiến độ.
- Thông báo hệ thống: Notification API khi tab ẩn (Chờ duyệt, Cần xử lý, Sẵn sàng), và `navigator.setAppBadge(số việc chờ bạn)` khi chạy dạng PWA (hỏi đáp #10). Manifest PWA và service worker tối giản để cài được như app.
- Vì sao? (`agent=why`) và Bản tin (`agent=recap`) trên Bàn dựng; Vì sao? ở Khám phá (`why=true`).
- Bàn dựng giữ vị trí cuộn, lựa chọn và chữ đang gõ khi dữ liệu cập nhật qua SSE.

## Kiểm thử

- Go:
  - gộp thông báo (3 video chuyển Chờ duyệt trong 10 s thành 1 dòng);
  - Không làm phiền;
  - `bulkEligibility`;
  - viết lại metadata không đụng phần không chọn;
  - `expectedRevision` lệch thì trả 409.
- Web:
  - phím tắt Duyệt;
  - cập nhật lạc quan rồi hoàn lại;
  - toast: tối đa 3, rê chuột thì dừng đếm, Esc đóng;
  - thứ tự ưu tiên trong popover.
- E2E trên trình duyệt: đo thời gian từ lúc job đổi trạng thái tới lúc Bàn dựng đổi, ≤ 1 s có SSE và ≤ 3 s khi đã chặn SSE.
- Smoke thật: một video tới Chờ duyệt → nhận thông báo hệ thống khi tab ẩn → bấm vào mở đúng video → duyệt → dựng → Sẵn sàng.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V1**, **V3**, **V14**, **V16**, **V18** (08-dev-prompt).
- Không có trang nào thiếu trạng thái đang tải, rỗng, lỗi hoặc mất kết nối (V12).

## Rủi ro

- **Cập nhật lạc quan sai khi hai thao tác chồng nhau.** Mọi thao tác ảnh gửi kèm `expectedRevision`. Bị 409 thì tải lại đúng ảnh đó, không tải lại cả trang.
- **Notification API cần người dùng cho phép.** Xin quyền ở Cài đặt thông báo, không hỏi ngay khi mở app. Bị từ chối thì vẫn có trong lịch sử trong app.
