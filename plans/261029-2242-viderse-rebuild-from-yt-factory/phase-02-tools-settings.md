---
phase: 2
title: "Công cụ máy, kết nối AI, tài khoản nền tảng, Cài đặt"
status: pending
priority: P1
effort: 48h
dependsOn: [1]
---

# Phase 02 · Công cụ máy, kết nối AI, tài khoản, Cài đặt

## Mục tiêu

Máy có đủ công cụ, tài khoản và thư mục dự án để pipeline chạy được. Cài đặt có đủ 6 mục theo design:

- Công cụ, Kết nối AI, Trợ lý AI;
- Tài khoản nền tảng, Lưu trữ, Nhật ký.

Bước `setup` của Lần đầu mở chạy thật.

Gói design:

- **V7** Cài đặt;
- **V17** Kết nối AI và Trợ lý AI;
- **V22** Thư mục lưu dự án;
- **V23**, phần tài khoản nền tảng;
- **V24** Đăng nhập Bilibili bằng QR;
- **V9**, bước `step=setup`.

## Đầu vào (yt-factory)

| Nguồn | Nội dung cần port |
|---|---|
| `src/server/tools/tool-manifest.ts` (391 dòng), `managed-tools.ts`, `tool-installer.ts`, `tool-process.ts`, `process-lifecycle.ts`, `machine-load.ts` | Phiên bản và SHA-256 cố định, cài và cập nhật, chọn binary theo thứ tự (đường dẫn cấu hình → binary hệ thống → bản app tự quản), giám sát nhóm process |
| `tools/yt-dlp.ts`, `ffmpeg.ts`, `uv.ts`, `transcription.ts`, `chrome.ts`, `codex.ts`, `codex-isolation.ts`, `grok.ts` | Từng công cụ: kiểm tra, phiên bản, đọc tiến độ, xếp loại lỗi |
| `tools/cookies.ts`, `bilibili-cookies.ts`, `backdrop/bilibili-qr-login.ts` | Cookie YouTube, cookie và QR Bilibili |
| `src/lib/setup-steps.ts`, `src/app/setup/*`, `src/app/settings/*` | Các bước setup, các mục Cài đặt |
| `src/server/retention.ts`, `editor/media-gc.ts` | Dọn dữ liệu theo chính sách giữ |

Design: 07 §15 (AI), §23 (thư mục dự án), §24 (nền tảng nguồn, tài khoản), 08 V7, V17, V22, V23, V24; artboard `Settings`, `Onboard`.

## Việc

### Backend

- **`internal/tools`:**
  - manifest cố định cho yt-dlp, ffmpeg/ffprobe, uv cùng môi trường faster-whisper, Codex CLI, Chrome for Testing;
  - tải về, kiểm SHA-256, đổi tên nguyên tử;
  - `Run` khởi tạo process con với `Setpgid`, huỷ thì kill cả nhóm;
  - có timeout, đọc tiến độ từ stdout/stderr, ghi log vào thư mục dự án;
  - xếp loại lỗi theo `tool_missing` và `retryAt` như bản cũ.
- **`internal/accounts`:**
  - cookie YouTube (lấy từ trình duyệt hoặc tệp);
  - Bilibili: cookie và QR login. API trả cả `loginUrl` (hỏi đáp #7);
  - Grok dùng OAuth xAI có sẵn ở `core/auth/xai.go`;
  - mọi bí mật lưu trong `~/.viderse/`, quyền 0600, không bao giờ trả về khoá đầy đủ (chỉ `keyHint`).
- **`internal/aiconn`** (§15):
  - `AiProvider` và `AiModel` dựng trên `core/llmconfig`, `core/auth`, `core/models`;
  - `AgentModels` cho Đạo diễn (`text`) và Hoạ sĩ (`vision` để viết prompt, `image` để vẽ, có dự phòng);
  - đủ các API ở §15, cùng mã lỗi `auth_failed`, `in_use`, `capability_missing`.
- **`internal/imagegen`:**
  - interface `Provider { Draw(ctx, req) (Receipt, error) }`;
  - bản Codex CLI (`gpt-image-2`, cô lập như `codex-isolation.ts`) và bản Grok (`grok-imagine-image`);
  - biên nhận ghi trước và sau mỗi lần vẽ để không trả tiền hai lần khi phục hồi;
  - D7: giới hạn 50 ảnh mỗi ngày cho mỗi nhà cung cấp trả theo ảnh, có thể chỉnh, 0 nghĩa là không tự tiêu.
- **`internal/storage`** (V22):
  - thư mục dự án mặc định `~/Viderse`;
  - API duyệt thư mục phía server để chọn đường dẫn (hỏi đáp #10);
  - báo dung lượng đã dùng;
  - phát hiện ổ biến mất hoặc chỉ đọc thì đưa vào Cần xử lý;
  - dọn dữ liệu theo `retention.ts`.
- **Nhật ký:** `GET /api/tools/logs` lọc theo công cụ và mức. Các bản ghi đã che bí mật.
- **`background_tasks`:** cài và cập nhật công cụ, đăng nhập, dọn dẹp đều ghi một dòng (§11).

### Web

- `screens/settings`: 6 mục theo artboard `Settings`, gồm các tweak:
  - `tools=ok|update|failed|installing`;
  - `conn=list|add|added|models|manual|failed`;
  - `grok=signed|code|out`, `bili=saved|empty`;
  - `clean=idle|preview|done`;
  - `ai=none|director|artist`.
- `screens/onboard`: bước `setup` (`running|login|failed`) chạy cài đặt thật và hiện tiến độ qua SSE.
- Chấm ở bánh răng khi đăng nhập ChatGPT hết hạn; bấm vào mở đúng mục.

## Kiểm thử

- Go:
  - test manifest (SHA-256 sai thì từ chối);
  - test thứ tự chọn binary;
  - test giám sát process: dùng script giả ở `testdata` in tiến độ rồi treo; huỷ phải dọn hết nhóm process;
  - test aiconn với `httptest` cho Gemini, OpenRouter, Ollama, gồm cả trường hợp khoá sai;
  - test D7: vượt giới hạn thì job chờ đến giờ đặt lại;
  - storage: thư mục mất hoặc chỉ đọc.
- Chuyển ca test từ `grok.test.ts`, `image-fallback.test.ts`, `tool-manifest` và `cookies` của yt-factory.
- Web: test các mục Cài đặt theo tweak, test khi Kết nối AI lỗi.
- Smoke thật:
  - cài Chrome for Testing và môi trường whisper vào thư mục tạm;
  - đăng nhập Grok bằng mã thiết bị;
  - quét QR Bilibili, trạng thái đổi trong ≤ 2 s (V24);
  - thêm một khoá API, chọn model, gán cho Đạo diễn.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V7**, **V17**, **V22**, **V24** và phần tài khoản của **V23** (08-dev-prompt).
- Lần đầu mở trên máy chưa có công cụ: hoàn tất bước `setup`, không cần mở terminal.
- Không có bí mật nào trong log, API hay bản xuất chẩn đoán (test che bí mật).

## Rủi ro

- **Codex CLI đổi cờ hoặc định dạng sự kiện.** Ghim phiên bản theo manifest. Chạy kiểm tra trước khi gọi như `text-tool-advertisement` của bản cũ; lệch thì báo lỗi rõ, không chạy tiếp.
- **Tải Chrome và whisper (khoảng 1.3 GB) chậm.** Tải tiếp được khi đứt. Tiến độ hiện ở Việc nền.
