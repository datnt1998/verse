---
phase: 8
title: "Nhắn trợ lý, Đề xuất, Quy tắc đã học, Mức tự chủ, Hiệu quả sau đăng"
status: pending
priority: P2
effort: 40h
dependsOn: [7]
---

# Phase 08 · Trợ lý chủ động

## Mục tiêu

- Người dùng sai khiến trợ lý bằng câu tự nhiên. Mỗi câu thành đúng một thao tác có sẵn, và luôn hỏi xác nhận trước khi chạy.
- Trợ lý tự đề xuất việc nên làm và học từ những gì người dùng đã sửa.
- Mức tự chủ quyết định lúc nào máy tự duyệt.
- Số liệu sau đăng quay lại ảnh hưởng thứ tự ưu tiên.

Gói design:

- **V11** Nhắn trợ lý (D2);
- **V19** Đề xuất và Quy tắc đã học;
- **V20** Mức tự chủ và điểm ảnh (D6, D7);
- **V21** Hiệu quả sau đăng và Ưu tiên lượt xem (D5);
- §9, §18, §19, §20, §21.

## Đầu vào

yt-factory chưa có các tính năng này, nên làm mới hoàn toàn theo design:

- 07 §9, §18–§21;
- 08 V11, V19, V20, V21;
- artboard:
  - `Main` (Composer, `agent=suggest|learn`);
  - `Channel` (Composer kênh, `autonomy=balanced|careful|auto|custom|confirm`, Quy tắc đã học, Trợ lý nhận ra);
  - `Review` (`state=learn`);
  - `Dialogs` (`assistant-confirm`, `assistant-unknown`, `autonomy`).

## Việc

### Backend

- **`internal/assistant`** (§9):
  - `POST /api/assistant/parse` dùng `core/llm` với structured output. Schema chỉ cho đúng 10 intent ở bảng V11 (D2);
  - câu của người dùng đưa vào dưới dạng dữ liệu, không bao giờ là chỉ dẫn hệ thống;
  - không khớp intent nào thì trả `{ ok: false, suggestions }`;
  - `POST /api/assistant/run` gọi đúng service đã có, không có đường riêng.
- **Mức tự chủ và điểm ảnh** (§20):
  - điểm ảnh lấy từ `image-review` (phase 05);
  - D6: ngưỡng mặc định 85; "Tự duyệt hết" chỉ bật được qua Tuỳ chỉnh và luôn hỏi xác nhận;
  - D7 dùng chung giới hạn ảnh với phase 02;
  - mỗi lần tự duyệt ghi một `Reason`.
- **Đề xuất** (§18):
  - bộ sinh chạy cùng chu kỳ quét đối thủ và sau mỗi sự kiện liên quan;
  - khoá trùng theo `kind + channelId + đối tượng`;
  - bỏ qua một đề xuất thì `cooldownUntil` = +7 ngày.
- **Quy tắc đã học** (§19):
  - sinh từ các lần sửa của người dùng (góp ý vẽ lại, sửa metadata);
  - áp từ lần viết hoặc vẽ tiếp theo;
  - xem, tắt và xoá được ở Kênh › Hồ sơ.
- **Số liệu sau đăng** (§21, D5):
  - khoá YouTube Data API tuỳ chọn, cấu hình ở Kết nối AI;
  - không có khoá thì đọc trang công khai bằng client InnerTube (phase 03);
  - đọc lại theo lịch;
  - "Ưu tiên lượt xem" đưa vào cách tự lấy video của hàng chờ.

### Web

- Composer ở Bàn dựng và Kênh: câu đã phân tích hiện trong hộp xác nhận có giá (số ảnh) và danh sách trợ lý sẽ chạy lại.
- Đề xuất (`agent=suggest`): mỗi đề xuất có đúng một hành động chính, kèm Bỏ qua (`quiet`).
- Quy tắc đã học (`agent=learn`, `Review` `state=learn`).
- Mức tự chủ ở Kênh: 4 mức và Tuỳ chỉnh có xác nhận.
- Hiệu quả sau đăng ở Kênh › Video.

## Kiểm thử

- Go:
  - bộ ca câu → intent cho đủ 10 intent;
  - câu có lệnh chèn ("bỏ qua hướng dẫn…") vẫn chỉ ra intent hợp lệ hoặc `ok: false`;
  - schema từ chối intent lạ;
  - ngưỡng D6 và luồng xác nhận "Tự duyệt hết";
  - khoá trùng và `cooldownUntil` của đề xuất;
  - nguồn số liệu: có khoá dùng Data API (`httptest`), không có khoá thì dùng fixture trang công khai.
- Eval nhỏ cho bộ phân loại intent: model gán cho Đạo diễn, khoảng 40 câu tiếng Việt, ngưỡng ≥ 95% đúng intent. Phải hỏi anh trước khi chạy vì gọi model thật.
- Smoke thật: gõ "vẽ lại cảnh 5 cho y tá cầm đèn pin" → hộp xác nhận đúng → chạy → ảnh 5 vẽ lại.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V11**, **V19**, **V20**, **V21** (08-dev-prompt).
- Không thao tác nào của trợ lý chạy khi chưa qua hộp xác nhận.

## Rủi ro

- **Phân loại sai intent.** Luôn hỏi xác nhận, hộp xác nhận nêu rõ hệ quả và giá, và có eval với ngưỡng ở trên.
- **Đọc trang công khai bị chặn.** Dùng chung bộ điều tốc với quét đối thủ. Hiện gợi ý thêm khoá Data API ở Kết nối AI.
