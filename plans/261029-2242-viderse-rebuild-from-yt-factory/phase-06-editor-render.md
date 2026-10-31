---
phase: 6
title: "Editor, render, đo và chấm điểm, xuất MP4"
status: in-progress
priority: P1
effort: 60h
dependsOn: [5]
---

# Phase 06 · Editor, render, chấm điểm, xuất video

## Mục tiêu

- Chuỗi chạy đủ 9 bước. `score` đo bản dựng và áp cổng ≥ 90. `export` render bằng engine editor trong Chrome headless rồi nén bằng ffmpeg.
- Editor mở được từ video Chưa đạt, có playhead đặt sẵn ở đoạn lệch đầu tiên.
- Editor dùng ngôn ngữ thiết kế Viderse; mật độ giữ như bản cũ.

Gói design: **V13** Editor theo Viderse; bước `score` và `export`.

## Đầu vào (yt-factory)

| Nguồn | Nội dung |
|---|---|
| `src/editor/engine` (159k dòng prod, Freecut `4d62e808` MIT, OpenReel MIT), `src/editor/ui` (8.1k) | Engine và UI editor. Chuyển vào web dưới dạng chunk lazy, giữ `UPSTREAM.md` và hai tệp LICENSE |
| 9 tệp import `next/` trong `src/editor/ui/{shell,template,library,project,clone}` | Đổi sang router hash và API client |
| `src/app/editor/[projectId]/{editor-client,render/render-client,measure/measure-client}.tsx` | Trang editor, trang render, trang đo |
| `src/server/editor/*` (4.8k dòng): `projects`, `workspace`, `media-import`, `media-probe`, `media-background`, `media-from-link`, `media-gc`, `apply-commands`, `clone-export-policy`, `http` | `internal/editor` |
| `src/server/clone/{score.ts 1003, loudness.ts, measurements.ts 485, measurement-launcher.ts, export-launcher.ts 445, export-requests.ts 415, export-gate.ts, export-regions.ts, export-compress.ts, headless-app-server.ts}` | `steps/score`, `internal/render` |
| `src/lib/clone-quality.ts` | `internal/domain/quality`: trọng số, `CLONE_EXPORT_MIN_SCORE=90`, sai lệch thời lượng ±10%, độ lớn âm thanh ±1 LUFS, lệch cắt ≤ max(2 khung, 100 ms) (3 khung ở 30 fps, như `score.ts`), 1920×1080 @ 30 fps |

Design: 08 V13; artboard `Editor` (`state=editing|exporting|export-done|conflict|narrow`); 01-ia-flows (vòng Chưa đạt → sửa → xuất lại).

## Việc

### Backend

- **`internal/editor`:**
  - kho dự án nằm trong thư mục dự án;
  - nhập media, đọc thông tin bằng ffprobe, nhập từ link qua yt-dlp;
  - lệnh sửa có `revision`; ghi lệch phiên bản trả 409 để UI hiện `conflict`;
  - dọn media không còn dùng.
  - Khoảng 35 method API cũ được giữ hình dữ liệu, đặt dưới `/api/editor/…`.
- **`internal/render`:**
  - giám sát Chrome headless: Chrome for Testing lấy từ `tools`, mỗi lượt một thư mục profile và một nhóm process riêng;
  - khởi động lại tối đa 2 lần khi process thoát sớm, heartbeat quá 45 s, trang không nhận yêu cầu, hoặc tiến độ đứng yên;
  - dọn process mồ côi theo nhóm và profile đã ghi lại;
  - một lane xuất chạy tuần tự.
- **Đo:**
  - trang `#/measure/<id>` gửi PCM f32le stereo 48 kHz lên server; server chạy ebur128 bằng ffmpeg;
  - xoá PCM khi yêu cầu kết thúc; tối đa 8 GiB.
- **Nén:** ffmpeg `libx264 -preset veryfast -crf 28 -maxrate 900k -bufsize 1800k -g 300 -pix_fmt yuv420p -c:a copy -movflags +faststart`.
- **`steps/score`:**
  - tính điểm và danh sách đoạn lệch (`export-regions`);
  - lệch cổng thì đo lại và chấm lại một lần, như bản cũ;
  - sau đó chuyển sang `export`, hoặc Chưa đạt kèm danh sách đoạn.
- **Phân quyền cho trang render:**
  - Chrome headless mở `#/render/<requestId>` bằng token ngắn hạn, chỉ dùng cho đúng yêu cầu đó (chỉ đọc media của dự án, chỉ ghi tiến độ của yêu cầu đó);
  - không dùng phiên người dùng;
  - token hết hạn khi yêu cầu kết thúc.

### Web

- `src/editor/`: chép engine và UI từ yt-factory, thay 9 chỗ phụ thuộc Next, đổi giao diện sang token và kit Viderse (V13).
- Ba route lazy: `#/editor/<projectId>`, `#/render/<requestId>`, `#/measure/<requestId>`. Hai route render và đo không có shell.
- Ngân sách: chunk đầu không chứa mã editor (script size kiểm).
- V13: mở từ Chưa đạt thì playhead nằm ở đoạn lệch đầu tiên, chip đếm đúng số đoạn; có trạng thái `exporting`, `export-done`, `conflict`, `narrow`.

## Kiểm thử

- Go:
  - chuyển ca test từ `score`, `export-gate`, `export-regions`, `export-requests`, `measurements`, `loudness`, `export-compress`;
  - chuyển test `src/server/editor/*` (dùng chung `__fixtures__`);
  - supervisor: Chrome giả treo hoặc thoát sớm thì khởi động lại đúng số lần, rồi dọn sạch nhóm process.
- Web: test đang có của engine (`src/editor/engine/test`) chạy được trong vitest của Viderse; thêm test cho 9 chỗ đã thay.
- Đối chiếu với bản cũ: cùng một dự án mẫu (`quality-real-fixture`), xuất ở yt-factory và ở Viderse. So thời lượng, độ phân giải, LUFS và điểm, sai khác phải nằm trong dung sai của `clone-quality`. Ghi thời gian xuất của cả hai.
- Smoke thật: video ở phase 05 đi tiếp `score` → `export` ra MP4. Mở một video Chưa đạt trong editor, sửa, rồi xuất lại.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V13** (08-dev-prompt).
- Cổng ≥ 90 hoạt động như bản cũ: thiếu điểm, có blocker hoặc có mục `unknown` thì không xuất.
- Thời gian xuất của Viderse không chậm hơn yt-factory trên cùng dự án mẫu.
- `kill -9` khi đang xuất: khởi động lại thì xuất lại từ đầu yêu cầu, không còn Chrome mồ côi.

## Rủi ro

- **Engine chạy trong Chrome headless thiếu GPU.** Giữ cách kiểm khả năng như `chrome.ts` (offline audio, AAC, Opus); thiếu thì báo lỗi rõ ở Công cụ.
- **Khối mã lớn làm chậm typecheck và test.** Engine tách thành project TS riêng trong workspace của Viderse; vitest chạy test engine với `--maxWorkers=2`.
