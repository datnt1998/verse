# Ổn định (phase 10 · bước 3) — nghiệm thu không có AI thật, không vẽ ảnh trả phí

Lượt chạy 2 (2026-11-01). Serve thật dựng từ HEAD, công cụ pin của app, provider giả cục bộ.
Kết quả: **một phần** — 3 lỗi thật (1 đã sửa ở lượt trước, 1 sửa lượt này, 1 chặn khởi động),
các kịch bản `kill -9`/PG/ổ đĩa **chưa chạy xong** vì chính lỗi chặn khởi động (§5).

## 1. Môi trường (đã dùng thật, không AI, không vẽ trả phí)

| Thành phần | Cách làm |
|---|---|
| `ffmpeg` | **công cụ pin của app**, không dùng Homebrew: `POST /api/tools/ffmpeg/install` → `ffmpeg ready 6.0` tại `.viderse/bin/ffmpeg/v-6.0-ffmpeg-static-b6.1.1/ffmpeg`, có `libwebp` (Homebrew 9.0.2 thiếu encoder này nên bước `images` hỏng mã 8 ở lượt trước) |
| Chrome | `POST /api/tools/chrome/install` (192 MB, Chrome for Testing) — bước `score` đòi `Chrome.ManagedPath()`, Chrome hệ thống không đủ |
| AI | provider `openai_compat` trỏ `http://127.0.0.1:18915/v1` (stub: chat → metadata/story JSON, images → PNG 1280×720 thật); Đạo diễn `stub-chat-1`, Hoạ sĩ `stub-vision-4o` + `stub-image-1` |
| `yt-dlp` | script cục bộ: `-J` trả info cố định, `--load-info-json` chép fixture MP4 20 s + json3 + PNG bìa, in dòng `YTFPROG|…` thật; số tick chỉnh qua `/tmp/p10-stab/ytdlp_ticks` |
| `uv` + transcription | `uv` giả ở PATH chuyển `uv run … clone-transcribe.py` sang report giả; venv thật (`uv venv`, không gói) + `env-meta.json` đúng fingerprint + model.bin giả → `tools.Transcription` báo `ready` thật |
| Dữ liệu | 3 `owner_channels` (autonomy `score/0`), 1 template chuẩn `images fixed 1`, 10 `rival_videos` để `POST /api/clone` không gọi InnerTube |

Serve: `serve.sh setup|resume|drop 18911`, DB `p10_p10stab_data/_identity`, `HOME=/tmp/p10-data-P10Stab`.

## 2. Kết quả đã đo được

| # | Kịch bản | Kỳ vọng | Quan sát | Kết luận |
|---|---|---|---|---|
| 1 | 10 video / 3 kênh chạy tới `images` | tất cả qua `download…images` | **10/10 job** `download, backdrop, transcript, metadata, scenes, images` = `succeeded`; **20 dòng `image_receipts`**; cả 10 vào `build` | **PASS** |
| 2 | Tạm dừng hàng chờ giữa lượt | worker dừng, item mới nằm chờ, chạy tiếp sau resume | `paused=true` → `running 2 → 0`; enqueue trong lúc pause → `clone_queue.state='queued'` (1), sau `paused=false` → `started`, `queued=0` | **PASS** |
| 3 | Một video qua `build` | duyệt ảnh kèm “xuất” thì `build` chạy | `POST /api/clone/bulk {action:"approve", expectedRevisions:{job:rev}}` → `build succeeded` (attempt 2); duyệt bằng `POST /api/clone/{id}/images {op:"approveAll"}` **không** đủ (route này không truyền `ApproveExport`) | **PASS** (kèm ghi chú §5.2) |
| 4 | `score` chạy đo bằng Chrome | `score` chạy, không lỗi artefact | Sau khi sửa §5.2: `score` chuyển `running`, sinh request đo trong `.viderse/render/clone-jobs/<job>/measurements/` | **MỘT PHẦN** — chạy > 5 phút chưa xong, chưa có tiến trình Chrome con; chưa xác định xong trong lượt này |
| 5 | `export` ra MP4 | có MP4 | Chưa tới `export` | **CHƯA CHẠY** |
| 6 | `kill -9` từng bước (9 bước) | chạy tiếp từ checkpoint, không mồ côi, không lặp phiếu | Kịch bản tự động (`/tmp/p10-stab/killtest.sh`) chạy nhưng **sai** ở lượt đầu (truy vấn chọn nhầm job của lượt trước → `FAST`; enqueue vượt `pace_per_day` ≤ 50 → `ENQUEUE_FAILED`), rồi dừng khi phát hiện lỗi §5.3. Riêng `kill -9` giữa lúc publish **đã tái hiện** và gây hậu quả ở §5.3 | **CHƯA CHẠY ĐỦ** |
| 7 | Mất Postgres 30 s qua proxy riêng | “Cần xử lý” rồi tự chạy tiếp | Script sẵn (`/tmp/p10-stab/pg-loss.sh`, proxy `/tmp/p10-stab/pgproxy.py`), chưa chạy vì serve không khởi động lại được (§5.3) | **CHƯA CHẠY** |
| 8 | Rút ổ thư mục dự án (disk image) | “Cần xử lý” ≤ 10 s, không hỏng video, cắm lại thì chạy tiếp | Ảnh đĩa đã tạo/attach (`/tmp/p10proj.dmg` → `/Volumes/P10Proj`); script sẵn (`/tmp/p10-stab/disk-loss.sh`), chưa chạy cùng lý do | **CHƯA CHẠY** |
| 9 | Ngắt mạng khi đang vẽ ảnh | không vẽ lặp, chờ hạn mức | Cần lượt vẽ trả phí thật | **BLOCKED** |

Ghi chú đo được thêm: `VIDERSE_DAILY_PAID_IMAGE_LIMIT=2` nhưng provider `openai_compat` (stub) **không bị tính là trả phí** → 20 lượt vẽ không chặn hạn mức; muốn kiểm nhánh “chờ hạn mức” phải dùng provider thật hoặc bật cờ paid cho provider.

## 3. Bộ test phục hồi hiện có (theo bước)

`go test -count=1 -run '(Resume|Recover|Recovery|Crash|Kill|Orphan|Pause)' ./apps/viderse/internal/pipeline/... ./apps/viderse/internal/imagegen/... ./apps/viderse/internal/render/... ./apps/viderse/internal/storage/... ./apps/viderse/internal/activity/... ./apps/viderse/internal/tools/...`
→ **ok, 12.80 s** (2026-11-01).

| Bước | Test phục hồi |
|---|---|
| `download` | `TestHandlerResumesCheckpointWithoutRefetchingMetadata`, `TestHandlerResumesCompletedMediaWithoutDownloadingAgain`, `TestHandlerPauseSavesCheckpointAndKeepsPart`, `TestHandlerRejectsChangedCheckpointMetadata`, `TestHandlerPropagatesProgressAndCheckpointFailures` |
| `backdrop` | `TestCrashResumeKeepsSuccessfulAndPendingWindows`, `TestFillPauseRecoveryAndStoredSpeedGoals`, `TestExistingPauseAvoidsNetworkAndRetriesAtEarliestPause`, `TestPauseAndCheckpointFailuresPreventFurtherFetches`, `TestUnknownOwnerProbeReportsRecovery` |
| `transcript` | `TestTranscriptResumesCompletedPhasesAfterCrash`, `TestTranscriptPauseSavesCheckpointBeforeParking`, `TestSidecarCancelKillsProcessGroup` |
| `metadata`, `scenes`, `build`, `score` | không có test phục hồi riêng |
| `images` | `TestCaptureSurvivesEncodeCrashWithoutDuplicateReceipt` (PG), `TestOwnerRedrawUsesNewReceiptWhileCrashRetryReusesFlight` (PG), `TestWorkerAppliesOverrideBeforeStartingRemainingImages` |
| `export` (internal/render) | `TestRenderRecoveredExportRequeuesInterruptedPipeline`, `TestRenderDuplicateInterruptedIntentRequeuesSameRequest`, `TestExportRecoveryResetsRunAndInvalidatesCapabilities`, `TestRequestRecoveryRotatesExpiredTokens`, `TestSupervisorKill9RecoversSameRequestAndNoOrphanChrome`, `TestSupervisorRestartsEarlyExitExactlyTwiceAndCleansGroup`, `TestSupervisorRestartsUnclaimedHangExactlyTwice`, `TestMeasurementCrashTailReconcilesAndFormatLocks`, `TestMeasurementOrphanSweepDryRunAndLiveGrace`, `TestMeasurementRecoveryChromeAndScopedToken` |
| dùng chung | `imagegen` (4, 1 cần PG), `storage` V22 (6), `activity` `TestReconcileFailsOrphansWithRetryAction`, `tools` `TestGroupIgnoringSigtermKilledAfterGrace` |

Chưa chạy: nhóm cần Postgres (`internal/persistence/postgres`, `images/journal_pg_test.go`).

## 4. Bằng chứng

- `clone_steps` của 10 job: `download/backdrop/transcript/metadata/scenes/images = succeeded`; `build` 9 `waiting`, 1 `succeeded`.
- `image_receipts` = 20; `/api/tools`: `ffmpeg ready 6.0` (pin), `chrome ready 156.0.8078.74` (Chrome for Testing), `transcription ready`, `yt-dlp ready`.
- `POST /api/clone/bulk` → `{"results":[{"jobId":"33dbdda2…","ok":true}]}` rồi `build succeeded`.
- `.viderse/render/clone-jobs/33dbdda2…/measurements/<id>.json` (request đo, `status:"cancelled"`/mới).
- Log khởi động lỗi §5.3: `Lỗi: chưa xác định được tham chiếu bền vững cho clone/9c2b5d30…/editor/media/media-6d228769…/metadata.json …; giữ nguyên kho cũ`, sau đó **không có tiến trình serve**, `GET /healthz` = `000`.

## 5. Lỗi thật

### 5.1 `scenes` để `Pending` → `build` chặn vĩnh viễn (đã sửa ở lượt trước, commit `046a92c7`)
`scenes/handler.go` luôn ghi `m.Pending`; `build/validation.go:187` từ chối; `scenes.Approve`/`ApplyEdits` **không có caller** ngoài test, không route/CLI nào duyệt cảnh, design V3 chỉ duyệt ảnh. Sửa: công bố `Approval` thay `Pending`. Test hỏng-trước `TestScenesPublishAcceptedAnalysisWithoutPendingReview` (FAIL trước, ok sau). Trên serve: lỗi `build` đổi từ “Cảnh đang chờ duyệt” sang “Ảnh chưa hoàn tất” rồi qua hẳn sau khi duyệt ảnh.

### 5.2 `editor/media/media-<sha>/metadata.json` khác nhau giữa các job → “ambiguous current editor artifact” (sửa đọc: `749bb8bc`)
Cùng một audio (cùng hash) cho cùng `media-<sha>`; `build` ghi `createdAt` = đồng hồ mỗi job nên **bytes khác nhau** (diff hai blob: chỉ khác `createdAt`). `editor_context.go` quét alias trên **mọi** job (`currentEditorArtifactsSQL`) rồi so digest → job thứ hai trở đi không `score`/`export` được:
`{"code":"internal_error","message":"ambiguous current editor artifact: editor/media/media-5c11796…/metadata.json"}`.
Sửa: `pickArtifact` — alias `editor/media/` (địa chỉ theo nội dung) lấy **bản mới nhất**; alias khác giữ kiểm tra xung đột; `legacyArtifacts` sắp `b.updated_at DESC`.
Test: `TestPickArtifactPrefersNewestMediaGeneration` (FAIL trước với đúng thông điệp trên), `TestPickArtifactKeepsStrictConflictForProjectFiles` (giữ hàng rào cho `editor/projects/`).
**Gốc chưa sửa:** `createdAt` phi tất định trong `build/handler.go` (`record["createdAt"] = created.UnixMilli()`) làm **đường publish** cũng đụng nhau (xem §5.3). Sửa gốc: bỏ/ổn định hoá `createdAt` cho media nhập từ clone để cùng `media-<sha>` ⇒ cùng bytes.

### 5.3 `kill -9` giữa lúc publish ⇒ **serve không khởi động lại được** (chặn, chưa sửa)
Tái hiện: `kill -9` serve khi một job đang chạy `build`/`images` (đúng kịch bản 6), rồi `serve.sh resume`:
```
Lỗi: chưa xác định được tham chiếu bền vững cho clone/9c2b5d30…/editor/media/media-6d228769…/metadata.json, …; giữ nguyên kho cũ
```
rồi tiến trình thoát; `/healthz` = `000`, `pgrep` rỗng — lặp lại nhiều lần. Đường mã: `internal/serverstorage/inventory.go:40` → `storageimport.RequireKnown` (`internal/storageimport/manifest.go:102`) trả lỗi khi gặp đường dẫn durable chưa có root, và **lỗi này làm chết boot** thay vì hạ xuống trạng thái “Cần xử lý”. Hệ quả với người dùng: sau một lần mất điện/`kill -9` đúng lúc đang dựng, app **không mở lại được** cho tới khi can thiệp tay.
Sửa tối thiểu đề xuất: coi lỗi inventory ở boot là **không nghiêm trọng** — log + đặt cờ “Cần xử lý” (attention `storage_path_unavailable`) và vẫn phục vụ, thay vì `return err` từ `Inventory`/`main`. Test hỏng-trước: trong `internal/serverstorage`, dựng cây dữ liệu có một tệp `clone/<job>/<sha>/editor/media/...` không có root, gọi `Service.Inventory` và khẳng định boot-time wrapper trả **nil + attention**, không trả lỗi. Chưa viết vì hết ngân sách lượt chạy.

## 6. Đúng lệnh để chạy nốt (sau khi sửa §5.3 và gốc §5.2)

0. `serve.sh setup 18911` (HEAD đã sửa), dựng lại kênh/template/video như §1.
1. **`kill -9` từng bước:** `/tmp/p10-stab/killtest.sh <step> 240` cho `download backdrop transcript metadata scenes images build score export`; mỗi lượt: xếp job mới (video id riêng), đợi `clone_steps.status='running'` của **đúng job đó**, `kill -9`, `serve.sh resume`, nudge `{action:"run"}`, rồi đọc `RESULT step=… status=… receipts_before/after=… duplicate_artifacts=…` và `children_after_kill`/`orphans_after_restart` (không được còn tiến trình cùng pgid).
   Chỉnh nhịp từng bước: `ytdlp_ticks` (download), `transcribe_delay` (transcript), `stub /ctl?slow=` (metadata), `/ctl?draw_slow=` (images).
2. **PG 30 s:** `/tmp/p10-stab/pg-loss.sh` (proxy `pgproxy.py 18917`, `P10_DB_ADDR=127.0.0.1:18917` trong `serve.env`), đo thời gian tới “Cần xử lý” và thời gian tự hồi.
3. **Rút ổ:** `hdiutil attach /tmp/p10proj.dmg` rồi `/tmp/p10-stab/disk-loss.sh` (chuyển `projects` sang `/Volumes/P10Proj/projects` bằng symlink, detach giữa lượt, đo `notice_after_s` — **lưu ý**: watcher `serverstorage/health.go` poll 50 s khi online / 60 s khi offline, nên mốc ≤ 10 s chỉ đạt nếu có lỗi ghi kích hoạt kiểm tra ngay; cần đo và nếu trượt thì đây là lỗi thứ tư).
4. **Ngắt mạng khi vẽ ảnh:** cần provider thật (BLOCKED).

## 7. Dọn dẹp
`serve.sh drop` (DB `p10_p10stab_*`), tắt `stub.py` (:18915), `hdiutil detach /Volumes/P10Proj` + xoá `/tmp/p10proj.dmg`, xoá `/tmp/p10-stab`, `/tmp/p10-data-P10Stab`, `/tmp/p10-build-P10Stab*`, `/tmp/p10-P10Stab-viderse`. Không đụng `verse-live`, `verse-port`, 8790.
