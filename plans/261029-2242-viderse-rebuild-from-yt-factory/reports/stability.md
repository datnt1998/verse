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

## 8. Lượt chạy 3 (P10Recovery): sửa gốc §5.2/§5.3, ma trận chạy được một phần

### 8.1 Gốc §5.3 — vì sao `kill -9` giữa publish làm serve không mở lại được

`media.Publish` ghi **đường dẫn cuối** (`publishLocal` → `Link`) **bên trong** transaction công bố, rồi mới `Bind` + commit. Kill giữa hai bước ⇒ tệp bền vững tồn tại mà không có hàng `blob_refs`/`clone_artifacts` nào; lần khởi động sau `ImportExisting → Inventory → RequireKnown` gặp tệp đó và **trả lỗi ⇒ `runServe` thoát**. Đây không phải "hỏng dữ liệu" mà là **byte chưa cam kết**.

Sửa (commit `1ac9ad36`):
- `internal/media/media.go`: byte ghi vào `<đường-dẫn>.part` (inventory đã bỏ qua `*.part`), **chỉ link vào đường dẫn cuối sau khi commit**; rollback thì xoá `.part`; thêm khoá advisory theo `path/` để hai lần công bố cùng đường dẫn không đè nhau.
- `media.RepairLocalPublications` (gọi ở đầu `adoptRoots`): hoàn tất link cho tham chiếu đã commit mà thiếu byte (chỉ khi digest khớp), xoá `.part` không có tham chiếu, giữ lại `.part` lệch digest và báo `Skipped`.
- `serverstorage.BootImport` (khởi động) ≠ `ImportExisting` (chuyển kho/CLI): boot **vẫn nhận root đã biết**, còn tệp không có adapter thì **không thoát** — `storageimport.UnknownPathsError` được trả về, `serve.go` ghi thông báo `needs_attention` ("Cần xử lý: N tệp trong kho dữ liệu chưa thuộc tham chiếu nào (…)" → `#/settings?section=storage`). `Migrate` vẫn fail closed.

### 8.2 Gốc §5.2 — `createdAt` phi tất định

`build/handler.go` không còn ghi `createdAt` (đồng hồ của job) vào `editor/media/media-<sha>/metadata.json`; media địa chỉ-theo-nội-dung nên cùng sha ⇒ cùng byte. Người đọc: `retention/media.go` đã chịu thiếu mốc (rơi về mtime thư mục), `editor/media_gc.go` giữ media qua `held` (snapshot job/template/manifest), UI media lấy `createdAt` từ `project.json` (theo từng job, không đổi). Giữ `pickArtifact` của `749bb8bc` vì dữ liệu **đã công bố trước bản sửa** vẫn có digest khác nhau cho cùng alias.

Test hỏng-trước:
- `TestPublishHidesLogicalPathUntilReferenceCommits` (media) — trước: `logical path exists before the reference committed`.
- `TestBuildMediaMetadataIsContentAddressedAcrossJobs` (build) — trước: hai job khác đồng hồ ⇒ metadata khác nhau.
- `TestBootImportReportsUnreferencedFileAndStillAdoptsRoots`, `TestBootImportCompletesInterruptedLocalPublication`, `TestRepairLocalPublications*` (serverstorage/media, PG).

### 8.3 Ma trận (scenario → kỳ vọng → quan sát → kết luận → bằng chứng)

| # | Kịch bản | Kỳ vọng | Quan sát | Kết luận | Bằng chứng |
|---|---|---|---|---|---|
| A | Khởi động lại sau `kill -9` giữa publish (trạng thái crash dựng đúng như §5.3) | app mở lại được, tệp lạ hiện "Cần xử lý" | bản **trước sửa**: `exit=1`, `Lỗi: chưa xác định được tham chiếu bền vững cho clone/9c2b5d30…/editor/media/media-6d228769/metadata.json; giữ nguyên kho cũ`, `/healthz=000`. Bản **sau sửa** trên **cùng DB + cùng HOME**: `/healthz=200`, thông báo `needs_attention` "Cần xử lý: 1 tệp trong kho dữ liệu chưa thuộc tham chiếu nào (clone/9c2b5d30…/metadata.json)" | **PASS** | log `/tmp/p10rec/old-restart.log` vs `new-restart.log`; `GET /api/activity` |
| B | `kill -9` từng bước `download…export` (9 bước, job riêng) | chạy tiếp từ checkpoint, không mồ côi, không lặp phiếu/artifact | **CHƯA CHẠY** — bộ harness (stub provider + yt-dlp/uv giả + seed) của lượt 2 đã bị xoá và chưa dựng lại xong trong lượt này | **CHƯA CHẠY** | — |
| C | Mất Postgres 30 s qua proxy riêng | "Cần xử lý"/offline rồi tự chạy tiếp | **CHƯA CHẠY** (phụ thuộc B) | **CHƯA CHẠY** | proxy đã viết: `/tmp/p10rec/pgproxy.py` (đã xoá khi dọn) |
| D | Rút ổ thư mục dự án (disk image) | "Cần xử lý" ≤ 10 s, không hỏng video, cắm lại chạy tiếp | **CHƯA CHẠY** (phụ thuộc B). Ghi chú: watcher thư mục dự án là `storage.Service` với `DefaultWatchEvery = 2 s` (`internal/storage/service.go:19`, có test chặn >10 s) — mốc ≤10 s **không** phụ thuộc `serverstorage/health.go` (50/60 s chỉ là sức khoẻ kho blob) | **CHƯA CHẠY** | `internal/storage/watch_test.go:18` |
| E | `score` > 5 phút trên job 1 cảnh | xác định kẹt hay chi phí thật | **CHƯA ĐO** (phụ thuộc B) | **CHƯA ĐO** | — |
| F | Ngắt mạng khi vẽ ảnh | không vẽ lặp, chờ hạn mức | vẫn cần lượt vẽ trả phí thật | **BLOCKED** | — |

### 8.4 Lệnh chạy nốt (cho người sau)

Harness cần dựng lại: `stub.py` (OpenAI-compatible `chat/completions` trả `{title,description,tags}` cho metadata và `{cast,shots[{id,prompt}]}` cho `clone.story`; `images/generations` trả `b64_json` PNG thật), `yt-dlp` giả (`-J` + `BuildDownloadArgs` → chép MP4 6 s), venv transcription giả (`tools.Transcription` ready; "python" ghi report VAD/ASR theo `internal/pipeline/steps/transcript/speech.go`), `POST /api/tools/ffmpeg/install` + `POST /api/tools/chrome/install`, seed `owner_channels` (pace_per_day ≥ 10) + `rival_videos` bằng SQL, provider `openai_compat` qua `POST /api/ai/providers`.
Serve: `/tmp/p10rec/serve.sh setup|resume|run|drop <Name> <port>` (giữ HOME khi resume; `P10_BIN` ghim binary; `P10_EXTRA_PATH` chèn fake vào PATH; `P10_DB_ADDR` đẩy traffic app qua proxy). Ma trận: mỗi bước một job riêng, đợi `clone_steps.status='running'` của **đúng job đó** rồi `kill -9`, `resume`, `POST /api/clone/{id} {action:"run"}`, đọc `RESULT step=… status=… receipts_before/after=… duplicate_artifacts=… children_after_kill/orphans_after_restart`.

## 9. Lượt chạy 4 (P10Matrix): gốc `score` > 5 phút, ma trận kill theo bước

Harness dựng lại ở `/tmp/p10m` (đã xoá khi dọn): `serve.sh setup|run|drop` (DB riêng `p10_p10matrix_{data,identity}`, build từ `git archive HEAD`), `stub.py` (provider `openai_compat` cổng 18965: `/v1/models`, `chat/completions` trả đúng hợp đồng metadata + casting-director, `images/generations` trả PNG thật, `/ctl?chat_slow&draw_slow` để làm chậm), `bin/yt-dlp` giả (info-json + chép fixture 60 s + dòng `YTFPROG`), `bin/uv` giả (chuyển `uv run --python` sang venv), venv transcription giả (`env-meta.json` khớp `EnvFingerprint`, "python" ghi report VAD/ASR), `hold.sh` (làm chậm ffmpeg/Chrome theo từng bước), `pgproxy.py`, 3 kênh (pace 20), 1 template 1 cảnh (`images fixed 1`), 40 `rival_videos`.

### 9.1 Gốc của "`score` > 5 phút" — bundle web nhúng bị cũ

`score`/`export` đo bằng trang headless (`/#/measure/<id>`), nhưng bundle **đã commit** trong `internal/web/dist` không có chunk headless: `grep -c measure assets/index-qdcr4GI-.js` = **0**, không có `headless-page-*.js`. Vì vậy Chrome for Testing được phóng đúng (pid gắn vào `measurements/<id>.json`), nhưng trang không hề gọi `GET /api/measure/<id>` rồi `/claim` → request kẹt `status=queued`; supervisor khởi động lại Chrome 2 lần (`MaxCrashRestarts=2`, "trang đo không nhận yêu cầu") và chỉ dừng ở `MeasurementTimeout = 10 phút`. Đây đúng là triệu chứng "> 5 phút, không thấy tiến độ" của lượt trước.

- Trước sửa: bước `score` `running` > 4 phút, `measurements/<id>.json` `status=queued`, `claimedAt=""`, `chrome.pid` có giá trị → **PASS chẩn đoán**.
- Sau sửa (`da45f46f` + `c8a5c6f2`, bundle dựng lại từ nguồn hiện tại, CI "Embedded web bundle is current"): cùng job, cùng DB → `score` **xong trong 9 s** và trả kết luận thật (`quality_gate: "Video chưa đạt Độ hoàn thiện để xuất"` cho fixture tổng hợp — đúng hành vi). `extract(epoch from finished_at-started_at)` = **9**.

### 9.2 Ma trận (scenario → kỳ vọng → quan sát → kết luận → bằng chứng)

| # | Kịch bản | Kỳ vọng | Quan sát | Kết luận | Bằng chứng |
|---|---|---|---|---|---|
| B1 | `kill -9` giữa `download` | chạy tiếp từ checkpoint, không mồ côi, không lặp | kill ở `status=running` attempt=1 → boot lại **2 s** (`/healthz=200`), `POST {action:"run"}` → `download=succeeded` **attempt=2** sau 14 s; `orphans_same_pgid=0`, `leftover_tool_procs=0`; artifacts `0→10`, `dup_paths=0` | **PASS** | `results/kill-download.log` |
| B2 | `kill -9` giữa `backdrop` | như trên | bước **no-op**: template không có backdrop → `WantedCount=0`, bước xong trước khi kịp thấy `running` (`final=succeeded`) | **N/A** (không có việc để ngắt) | `results/kill-backdrop.log` |
| B3 | `kill -9` giữa `transcript` | như trên | boot 2 s, `transcript=succeeded` attempt=2 sau 11 s, artifacts `5→11`, `dup_paths=0`, orphans 0 | **PASS** | `results/kill-transcript.log` |
| B4 | `kill -9` giữa `metadata` | như trên | boot 2 s, `metadata=succeeded` attempt=2 sau 12 s, artifacts `10→11`, `dup_paths=0` | **PASS** | `results/kill-metadata.log` |
| B5 | `kill -9` giữa `scenes` | như trên | boot 2 s, `scenes=succeeded` attempt=2 sau 12 s, artifacts `11→13`, `dup_paths=0` | **PASS** | `results/kill-scenes.log` |
| B6 | `kill -9` giữa `images` | như trên | boot **TIMEOUT**, serve thoát `exit=1`: `Lỗi: xác minh tham chiếu binary editor trước khi mở kho: editor binary head disagrees with metadata: media/media-8153116c…/source`; bước `images` kẹt `running` attempt=1 | **FAIL — lỗi thật §9.3** | `serve-matrix.log`, `results/kill-images.log` |
| B7 | `kill -9` giữa `score` | như trên | **CHƯA CHẠY** — serve không mở lại được sau B6 | **BLOCKED** | — |
| B8 | `kill -9` giữa `export` | như trên | **CHƯA CHẠY** — cùng lý do | **BLOCKED** | — |
| C | Mất Postgres 30 s qua proxy riêng | "Cần xử lý"/offline rồi tự chạy tiếp, không cần restart | **CHƯA CHẠY** (phụ thuộc B6) | **BLOCKED** | `pgproxy.py` đã viết (đã xoá khi dọn) |
| D | Rút ổ thư mục dự án (`hdiutil`) | "Cần xử lý" ≤ 10 s, không hỏng video, hàng chờ dừng, cắm lại chạy tiếp | **CHƯA CHẠY** (phụ thuộc B6) | **BLOCKED** | `disk_loss.sh` đã viết (đã xoá khi dọn) |
| E | `score` > 5 phút trên job 1 cảnh | xác định kẹt hay chi phí thật | gốc: bundle nhúng cũ (§9.1). Sau khi bundle được dựng lại: **9 s** | **PASS (đã sửa)** | §9.1; `da45f46f`, `c8a5c6f2` |
| F | Ngắt mạng khi vẽ ảnh | không vẽ lặp, chờ hạn mức | vẫn cần lượt vẽ trả phí thật | **BLOCKED** | — |

### 9.3 Lỗi thật mới: `kill -9` để lại staging ⇒ serve không mở lại được (chặn)

Sau `kill -9` giữa `images`, lần boot kế tiếp thoát `exit=1` với `editor binary head disagrees with metadata: media/media-8153116c5fe4ba98132976ee4a18391a/source`. Tệp gây lỗi nằm trong **staging** của job đã bị kill:

```
~/.viderse/clone-staging/ed6132b4-…/gen1/build/editor/media/media-8153116c5fe4ba98132976ee4a18391a/metadata.json
```
(`{"mime":"audio/wav","size":1152044}`, không có trường head).

Cơ chế: bộ nhập lúc boot quét cả thư mục staging còn sót; `adoptBinary` (`internal/editor/binary_resources.go:126-141`) so `existing.Path/Revision` trong DB với digest của metadata và **trả lỗi cứng** khi lệch, thay vì cách ly tệp lạ thành "Cần xử lý". Vì media là content-addressed, hai job có cùng nội dung nguồn (cùng sha) nằm ở hai staging khác nhau là đủ để khoá boot — đúng lớp lỗi của §5.2/§5.3 nhưng ở nhánh *staging*, chưa được sửa.

Hướng sửa nhỏ nhất: khi gặp metadata trong `clone-staging/**` mà `adoptBinary` lệch (hoặc thiếu head), coi là **rác staging**: bỏ qua + xoá thư mục staging của job đó (hoặc đưa vào `needs_attention`), không làm hỏng boot; test trước khi sửa: tạo `clone-staging/<job>/gen1/build/editor/media/media-<sha>/metadata.json` trùng sha với một media đã có đường dẫn khác, boot phải `exit=0` và báo "Cần xử lý".

### 9.4 Ghi chú vận hành

- Đổi binary giữa hai lượt trên cùng HOME làm boot từ chối (`editor binary head disagrees…`) — muốn đổi binary phải `serve.sh setup` lại (DB + HOME mới), hoặc giữ nguyên binary đã ghi kho.
- Bước `backdrop` chỉ có việc khi template có `backdrop` (kênh nền/video cố định); template mặc định không có → không thể ngắt giữa bước này.
- `PATCH /api/templates {recipe.backdrop}` không ghi được backdrop vào template tạo bằng API (recipe được suy ra từ project), nên harness phải chèn `backdrop_channels`/`backdrop_channel_videos` — và kể cả vậy bước vẫn không có việc.

### 9.5 Đã sửa §9.3 (`a1418f0e`)

Gốc thật không phải staging: media là content-addressed, nên hai job (hoặc hai lượt dựng) cùng nội dung ghi cùng id ở hai đường dẫn `clone/<job>/<digest>/editor/media/…`; `adoptBinary` so đường dẫn đã ghi với đường dẫn bản dựng hiện tại và trả lỗi cứng. Nay digest là danh tính: head có revision trùng digest được nhận dù đường dẫn khác; chỉ dời head khi đường dẫn cũ không còn đọc được. Bytes lệch/thiếu → thông báo "Cần xử lý" (giữ nguyên tệp), chỉ lỗi DB/kho còn chặn boot.

Đã chạy thật: trạng thái kill giữa `images` — binary cũ thoát đúng dòng lỗi, binary mới boot `/healthz=200` và chạy tiếp `images` attempt 1→2; `kill -9` thật khi `images` đang chạy → boot lại 200, attempt 3. Đổi binary `main~3` → binary mới trên cùng HOME + DB: boot 200, head không đổi. Test hỏng-trước: `TestEditorAdoptionAcceptsContentAddressedMediaAcrossJobs`. §9.4 dòng đầu không còn đúng.

Chưa chạy lại: B7 (`score`), B8 (`export`), C (mất Postgres 30 s), D (`hdiutil`).
