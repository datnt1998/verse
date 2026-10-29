---
phase: 10
title: "Nghiệm thu toàn bộ, chạy live, thôi dùng yt-factory"
status: pending
priority: P1
effort: 16h
dependsOn: [1, 2, 3, 4, 5, 6, 7, 8, 9]
---

# Phase 10 · Nghiệm thu, chạy live, thôi dùng yt-factory

## Mục tiêu

- Viderse đạt mọi mục Nghiệm thu từ V0 tới V24 và mọi ngưỡng hiệu năng, ổn định ở plan.md.
- Chạy live bằng launchd.
- Anh chuyển hẳn sang Viderse.
- yt-factory không bị xoá: repo và dữ liệu giữ nguyên để tham chiếu.

## Việc

### 1. Rà nghiệm thu

- Đi qua từng gói V0–V24 ở `08-dev-prompt.md`, ghi bằng chứng (lệnh, ảnh chụp, kết quả test) vào `10-gap-closure.md`.
- So từng artboard với màn thật, chụp cùng khung bằng Chrome headless. Mọi chỗ lệch hoặc sửa ngay, hoặc ghi vào hỏi đáp.

### 2. Hiệu năng

Đo đủ bảng ngưỡng ở plan.md, ghi số vào `reports/performance.md`:

- dữ liệu seed: 1.000 job, 20.000 video đối thủ, 50 template;
- p95 của các API danh sách;
- kích thước chunk đầu;
- thời gian từ lúc đổi trạng thái tới lúc UI đổi;
- trace khi cuộn danh sách;
- RSS lúc rảnh.

### 3. Ổn định

Mỗi lượt chỉ vẽ 1 ảnh trả phí (giới hạn D7 = 1); các video còn lại kiểm tới nhánh chờ hạn mức. Video đi hết chuỗi tới `export` là video chỉ cần 1 cảnh.

- **Chạy liền:** 10 video của 3 kênh chạy tới bước `images` (một video đi hết chuỗi), có tạm dừng hàng chờ và chạy tiếp ở giữa.
- **Tập sập:** `kill -9` ở từng bước (9 bước); tắt Postgres 30 s; rút ổ thư mục dự án rồi cắm lại; ngắt mạng khi đang vẽ ảnh.
- **Kết quả cần đạt:**
  - không hỏng video;
  - không vẽ lặp ảnh đã trả tiền;
  - không còn process mồ côi;
  - Cần xử lý hiện trong ≤ 10 s;
  - tự chạy tiếp khi điều kiện trở lại.

### 4. Live

- `apps/viderse/scripts/run-live.sh` theo mẫu `apps/noverse/scripts/run-live.sh`:
  - build `viderse` từ worktree `/Users/dat/Projects/verse-live` (nhánh `live`, dùng chung với Noverse);
  - chạy migrate;
  - lệnh `update`.
- LaunchAgent `dev.verse.viderse-live` cổng 8789:
  - log trong `~/Library/Logs/viderse-live/`;
  - khởi động lại khi sập.
- Kiểm `/healthz`, rồi `launchctl kickstart -k`. Live Noverse 8790 không bị ảnh hưởng.

### 5. Tài liệu

- `apps/viderse/README.md`: chạy, công cụ máy, thư mục dự án, LAN và ghép thiết bị, live, sao lưu Postgres.
- README gốc: bảng app.
- `third_party/NOTICE`: Freecut và OpenReel (MIT); ghi rõ ffmpeg, Chrome for Testing và yt-dlp là binary ngoài do người dùng tải.

### 6. Chuyển hẳn sang Viderse

Việc của anh. Em chỉ:

- ghi danh sách việc cần tắt ở yt-factory (app desktop, worker, quét đối thủ), để hai bên không cùng quét và cùng vẽ;
- tắt nếu anh đồng ý.

## Nghiệm thu

- `10-gap-closure.md` có đủ V0–V24 kèm bằng chứng.
- Mọi ngưỡng ở plan.md đạt, số đo nằm trong `reports/performance.md`.
- Live chạy qua một lần khởi động lại máy; `/healthz` trả 200.
- Mọi mục hỏi đáp do dev mở đều đã chốt.

## Rủi ro

- **Hai hệ cùng chạy quét và vẽ, tốn gấp đôi hạn mức.** Chỉ bật worker của Viderse sau khi anh tắt worker yt-factory, theo danh sách ở bước 6.
