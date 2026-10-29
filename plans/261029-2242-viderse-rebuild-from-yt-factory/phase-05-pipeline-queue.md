---
phase: 5
title: "Lõi pipeline, Làm video, Hàng chờ"
status: pending
priority: P1
effort: 88h
dependsOn: [4]
---

# Phase 05 · Lõi pipeline, Làm video, Hàng chờ

## Mục tiêu

- Một video đi được qua 7 bước đầu: download → backdrop → transcript → metadata → scenes → images → build. Hai bước `score` và `export` cần engine render nên làm ở phase 06.
- Có hàng chờ chung: video chọn tay đi trước, video hàng loạt đi theo nhịp kênh, và có lời giải thích vì sao một video chưa chạy.
- Tạm dừng, chạy tiếp, huỷ và sửa job đều chạy đúng như bản cũ.
- Sập ở bất kỳ đâu thì chạy tiếp được.

Gói design:

- **V2** Làm video (một hành động, có kế hoạch nói trước);
- **V6** Hàng chờ;
- **§1–§4** (`agentView`, nhóm trạng thái và Tạm dừng, hàng chờ, kế hoạch);
- phần backend của **V0** (`paused_at`, pause/resume).

## Đầu vào (yt-factory `src/server/clone`, 15.6k dòng prod)

| Nguồn (số dòng) | Thành phần Go |
|---|---|
| `service.ts` 1827, `repository.ts` 1215, `runner.ts` 706, `step-handlers.ts` 529, `handler-registry.ts`, `launch.ts`, `errors.ts`, `job-edit.ts`, `job-index.ts` 519, `job-recipe.ts`, `manifest-store.ts`, `artifacts.ts` 391 | `internal/pipeline`: máy trạng thái của job và step, các thế hệ (`supersededAt`), checkpoint và thử lại, huỷ, chạy tiếp, sửa job, kho manifest và artifact |
| `queue.ts` 944, `resource-slots.ts`, `auto-pick.ts`, `clone-settings.ts`, `counts.ts` | `internal/pipeline/queue`: lane theo kênh, `position` dạng số thực, xếp lịch công bằng, nhịp mỗi ngày, ETA 7 ngày, tự lấy video |
| `download.ts` 812, `source-cleanup.ts` | `steps/download` (yt-dlp `-J`, tải tiếp từ `.part`, remux bằng ffmpeg, phân loại lỗi kèm `retryAt`) |
| `backdrop.ts` 534, `backdrop-edit.ts`, `backdrop-manifest.ts` | `steps/backdrop` |
| `transcript.ts` 630, `caption-align.ts`, `timed-captions.ts`, `scripts/clone-transcribe.py` | `steps/transcript` (sidecar Python qua `uv`) |
| `metadata.ts` | `steps/metadata` (`core/llm` với structured output, bỏ tag chứa "clone") |
| `scenes.ts` 827, `scene-parser.ts` | `steps/scenes` (ffmpeg `scdet`, khung hình trọn vẹn, ảnh keyframe) |
| `images.ts` 924, `image-prompt.ts`, `image-review.ts` 325, `codex-image-output.ts`, `master-prompt*.ts`, `cover.ts` | `steps/images` (`imagegen`, slot ảnh, chờ khi hết hạn mức, có biên nhận) |
| `build.ts` 724 | `steps/build` (dựng timeline hoặc dự án editor; dừng ở `awaiting_review`) |
| `src/lib/clone-steps.ts`, `clone-retry.ts`, `clone-contract.ts`, `clone-view.ts` | `internal/domain` |
| `cli/clone.ts`, `cli/ytf.ts` | `viderse clone run|batch|pause|resume|cancel|status|wait`, `--priority` |

## Việc

### Backend

- **Migration `0000N_clone.sql`:**
  - `clone_jobs`: `generation`, `superseded_at`, `paused_at`, `cancel_requested_at`, `priority`, `origin`, `model_refs` jsonb, lease và fence;
  - `clone_steps`: `status`, `attempt`, `progress_done/total`, `progress_phase`, lỗi jsonb `{code, message, recovery, retryAt, retryAtSource}`;
  - `clone_queue`: `priority` `pick|batch`, `position`, `state`;
  - `image_receipts`.
  - Read model cho Bàn dựng là view hoặc bảng do code cập nhật trong cùng transaction, không dùng trigger như bản SQLite. Index theo `(channel_id, stage_group, updated_at)`.
- **Worker** (A2):
  - lane bằng `jobConcurrency` (1–6, mặc định 2), slot ảnh bằng `imageConcurrency` (1–8, mặc định 3);
  - khoá advisory để chỉ một process chạy worker;
  - fence được kiểm mỗi lần commit step;
  - tạm dừng: ảnh đang vẽ xong thì dừng, nhả lane, chạy tiếp đúng chỗ (nghiệm thu V0);
  - huỷ: kill nhóm process con của step đang chạy;
  - tắt máy chủ êm: ngừng nhận job mới, cho step đang chạy về checkpoint hoặc chuyển sang `waiting` có `retryAt`.
- **Hàng chờ** (§3):
  - video chọn tay là `pick` và lên đầu; video hàng loạt và tự lấy đi theo nhịp kênh, đặt lại lúc 00:00 giờ máy;
  - `GET /api/queue` có lời giải thích cho từng dòng;
  - Quy tắc chạy (`jobConcurrency`, `imageConcurrency`, `rescanHours`, tạm dừng hàng chờ) sửa được ở Hàng chờ và chỉ ở đó.
- **Kế hoạch** (§4): `POST /api/clone/plan` trả trước giờ bắt đầu dự kiến, số ảnh sẽ vẽ, giá, và các video trùng.
- **View-model** (A9): `agentView`, `subline`, nhóm trạng thái kèm `paused` được tính ở server và có golden JSON cho web.
- **Sự kiện:** `clone.job.*`, `clone.step.progress` (gộp tối đa 4 lần/giây mỗi job), `queue.*`.
- **API:**
  - `GET|POST /api/clone`, `GET|POST|DELETE /api/clone/:id` (`run|pause|resume|cancel`), `GET /api/clone/:id/steps/:step`;
  - `/api/clone/:id/artifacts/:artifactId`: phục vụ tệp hỗ trợ Range, cache `immutable` theo hash;
  - `POST /api/clone/plan`, `GET|PATCH /api/queue`, `GET /api/clone/counts`.

### Web

- `screens/new-video` theo artboard `NewVideo` (`input=empty|pasted|duplicate|error|multi`):
  - dán link xong 300 ms thì gọi kế hoạch;
  - một nút "Làm video".
- `screens/queue` theo artboard `Queue` (`state=running|paused|limited|empty`):
  - Quy tắc chạy;
  - lý do chưa chạy;
  - kéo đổi thứ tự (sửa `position`).
- Nối nút "vào hàng chờ" ở Khám phá (phase 03) vào kế hoạch và "Làm video".
- JobsPill nhận sự kiện thật. Khi tạm dừng hàng chờ, viên ghi "Đã tạm dừng".

## Kiểm thử

- Chuyển ca test từ:
  - `service`, `runner`, `repository` và `runner.integration-fixture`;
  - `queue` (ETA, nhịp, công bằng giữa các kênh), `job-edit`, `clone-retry`;
  - `download`, `transcript`, `scenes`, `images`, `image-review`, `metadata`, `build`, `artifacts`.
- Test phục hồi cho **từng bước**: `kill -9` khi step đang chạy, khởi động lại thì:
  - job chạy tiếp từ checkpoint;
  - không có biên nhận ảnh trùng;
  - không còn process con mồ côi.
  - Kiểm bằng `pgrep` theo nhóm process.
- Test khi hai process `viderse serve` cùng chạy: process thứ hai không lấy job.
- Benchmark `GET /api/clone?channel=all` với 1.000 job: p95 ≤ 100 ms.
- Web: test kế hoạch (debounce, link trùng, nhiều link), test lý do ở Hàng chờ, test kéo đổi thứ tự.
- Smoke thật, đúng 1 ảnh trả phí (giới hạn D7 = 1): chạy 1 video ngắn của đối thủ thật qua các bước; ảnh thứ hai phải dừng ở trạng thái chờ hạn mức. Tạm dừng rồi chạy tiếp, và `kill -9` ở bước `transcript`.

## Nghiệm thu

- Đạt mục Nghiệm thu của **V2** và **V6**, cùng các dòng backend của **V0**: tạm dừng job đang vẽ, JobsPill ≤ 3 s, viên ghi "Đã tạm dừng" (08-dev-prompt).
- Hành vi đổi ở 09 §4 đúng từng mục:
  - mọi video vào hàng chờ;
  - nhịp thay cho giới hạn mỗi ngày;
  - hết hạn mức ảnh thì không chuyển sang Cần xử lý;
  - có Tạm dừng và Chạy tiếp.
- Ngưỡng phục hồi ở plan.md đạt cho 7 bước.

## Rủi ro

- **Port máy trạng thái lệch.** Viết test trước, chuyển nguyên ca biên từ bản cũ (thế hệ, supersede, thử lại tại chỗ, `retryAt` của nhà cung cấp, sửa job). Không thêm trạng thái mới ngoài `paused`.
- **Ổ đĩa đầy giữa chừng.** `storage` kiểm dung lượng trống trước các bước ghi nhiều (download, images, build); thiếu chỗ thì chuyển sang Cần xử lý kèm câu theo 06-copy.
