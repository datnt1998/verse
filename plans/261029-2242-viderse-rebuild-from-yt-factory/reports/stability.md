# Ổn định (phase 10 · bước 3) — nghiệm thu không có AI thật, không vẽ ảnh trả phí

Trạng thái: **một phần** — dựng được serve thật chạy chuỗi pipeline bằng công cụ cục bộ,
chạy tới bước `images` thành công (có phiếu vẽ thật), rồi **dừng ở `build`** vì một lỗi thật
(§5). Các kịch bản `kill -9`, tạm dừng hàng chờ, mất Postgres, rút ổ thư mục dự án **chưa chạy
được** vì cổng duyệt cảnh chặn mọi job ở `build` (§5, §6). Mọi mục không chạy được ghi rõ
`BLOCKED`/`CHƯA CHẠY` kèm đúng lệnh để chạy lại.

## 1. Cách dựng môi trường (đã dùng thật)

Không có model AI nào đăng nhập, không vẽ ảnh trả phí. Mọi phụ thuộc ngoài được thay bằng
công cụ cục bộ **trong PATH của riêng serve** (không sửa repo, không sửa máy):

| Thành phần | Thay bằng | Ghi chú |
|---|---|---|
| `/tmp/p10-stab/bin/yt-dlp` | script Python đọc `-J` và `--load-info-json`, chép fixture MP4 + json3 + PNG bìa | in dòng `YTFPROG|…` thật, có `STUB_YTDLP_STEP_MS`/`STUB_YTDLP_TICKS` để làm chậm bước tải |
| `/tmp/p10-stab/bin/ffmpeg`, `ffprobe` | gọi ffmpeg Homebrew thật, chỉ thêm dòng `libwebp` khi `-encoders` | Homebrew ffmpeg 9.0.2 thiếu encoder `libwebp` nên app từ chối; wrapper chỉ vá phần probe |
| `/tmp/p10-stab/bin/uv` | script trả `uv 0.13.0` và chuyển `uv run … clone-transcribe.py` sang report giả | `uv` thật treo khi `--python` là wrapper nên phải thay ở PATH |
| `envs/transcription` giả | venv thật (uv venv, không gói) + `bin/python` wrapper trả `faster-whisper 1.2.1`; `env-meta.json` đúng fingerprint; `models/hf/models--Systran--faster-whisper-{small,base}/snapshots/stub/model.bin` | `tools.Transcription.Status` → `ready` thật |
| `/tmp/p10-stab/stub.py` (:18915) | HTTP OpenAI-compatible: `/v1/models`, `/v1/chat/completions` (metadata + story prompt), `/v1/images/generations` (PNG 1280×720 thật, base64) | đăng ký qua `POST /api/ai/providers {kind:"openai_compat",baseUrl:"http://127.0.0.1:18915/v1"}`, chọn 3 model, gán Đạo diễn/Hoạ sĩ |
| fixture | `ffmpeg testsrc2` 20 s 1280×720 h264+aac; `captions.vi.json3` 7 cue; `image.png` | nguồn 20 s nên `scenes` ra 1 cảnh |

Serve: `/tmp/p10-stab/serve.sh setup 18911` (build từ `git archive HEAD`, DB `p10_p10stab_data/_identity`,
`HOME=/tmp/p10-data-P10Stab`, `VIDERSE_DAILY_PAID_IMAGE_LIMIT=1`), `serve.sh resume 18911` để chạy lại
sau khi sập, `serve.sh drop` để dọn. Kênh: 3 dòng `owner_channels` + `owner_channel_templates` trỏ
một template `Mẫu thử P10` (`images.mode=fixed,count=1`, không backdrop) tạo bằng
`viderse template create/set`. Video nguồn: 10 dòng `rival_videos` (id 11 ký tự) + 1 `rival_channels`
để `POST /api/clone` không phải gọi InnerTube (không có mạng tới YouTube).

Sự cố trong lúc dựng harness (đã khắc phục, ghi để người sau tránh): script tạo venv giả ban đầu
`cp` đè lên symlink `bin/python3.15` của venv, khiến `cp` ghi xuyên qua symlink và **ghi đè binary
Python 3.15 của Homebrew**. Đã sửa bằng `brew reinstall python@3.15` (3.15.1 chạy lại bình thường)
và viết lại script: không ghi xuyên symlink, dùng `/usr/bin/python3` làm base.

## 2. Kho test phục hồi hiện có, theo từng bước

Lệnh chạy (một lượt, không cần Postgres):
`go test -count=1 -run '(Resume|Recover|Recovery|Crash|Kill|Orphan|Pause)' ./apps/viderse/internal/pipeline/... ./apps/viderse/internal/imagegen/... ./apps/viderse/internal/render/... ./apps/viderse/internal/storage/... ./apps/viderse/internal/activity/... ./apps/viderse/internal/tools/...`
→ **ok toàn bộ, 12.80 s** (2026-11-01), gồm: `pipeline` (no tests), `pipeline/queue` (no tests),
`steps/backdrop`, `steps/download`, `steps/images`, `steps/transcript`, `imagegen`, `render`,
`storage`, `activity`, `tools`.

| Bước | Test phục hồi (grep `Resume|Recover|Crash|Kill|Pause|Checkpoint`) | Chạy riêng |
|---|---|---|---|
| `download` | `TestHandlerResumesCheckpointWithoutRefetchingMetadata`, `TestHandlerResumesCompletedMediaWithoutDownloadingAgain`, `TestHandlerPauseSavesCheckpointAndKeepsPart`, `TestHandlerRejectsChangedCheckpointMetadata`, `TestHandlerPropagatesProgressAndCheckpointFailures` | PASS |
| `backdrop` | `TestCrashResumeKeepsSuccessfulAndPendingWindows`, `TestFillPauseRecoveryAndStoredSpeedGoals`, `TestExistingPauseAvoidsNetworkAndRetriesAtEarliestPause`, `TestPauseAndCheckpointFailuresPreventFurtherFetches`, `TestUnknownOwnerProbeReportsRecovery` | PASS |
| `transcript` | `TestTranscriptResumesCompletedPhasesAfterCrash`, `TestTranscriptPauseSavesCheckpointBeforeParking`, `TestSidecarCancelKillsProcessGroup` | PASS |
| `metadata` | không có test phục hồi (chỉ `llmretry` của core) | — |
| `scenes` | không có test phục hồi; chỉ có checkpoint khi `PauseRequested` trong `handler.go` | — |
| `images` | `TestCaptureSurvivesEncodeCrashWithoutDuplicateReceipt` (Postgres), `TestOwnerRedrawUsesNewReceiptWhileCrashRetryReusesFlight` (Postgres), `TestWorkerAppliesOverrideBeforeStartingRemainingImages` | PASS (phần không cần PG) |
| `build` | không có test phục hồi | — |
| `score` | không có test phục hồi (đo qua `render`) | — |
| `export` | `internal/render`: `TestRenderRecoveredExportRequeuesInterruptedPipeline`, `TestRenderDuplicateInterruptedIntentRequeuesSameRequest`, `TestExportRecoveryResetsRunAndInvalidatesCapabilities`, `TestRequestRecoveryRotatesExpiredTokens`, `TestSupervisorKill9RecoversSameRequestAndNoOrphanChrome`, `TestSupervisorRestartsEarlyExitExactlyTwiceAndCleansGroup`, `TestSupervisorRestartsUnclaimedHangExactlyTwice`, `TestMeasurementCrashTailReconcilesAndFormatLocks`, `TestMeasurementOrphanSweepDryRunAndLiveGrace`, `TestMeasurementRecoveryChromeAndScopedToken` | PASS |
| dùng chung | `imagegen`: `TestCrashRecoveryAdoptsFinishedOutput`, `TestCrashRecoveryWithoutOutputLocksKey`, `TestRecoveryRejectsMismatchedOutput`, `TestReceiptCrashRecoveryOnPostgres` (PG); `storage` (V22): `TestRecoverAdoptsRenamedDestination`, `TestRecoverCleansInterruptedCopy`, `TestRecoverFinishesCommittedCutover`, `TestRecoverKeepsJournalWhenDestinationUnreachable`, `TestHoldsReconcileAcrossRestart`, `TestMoveRecoversStagingAfterCrash`; `activity`: `TestReconcileFailsOrphansWithRetryAction`; `tools`: `TestGroupIgnoringSigtermKilledAfterGrace` | PASS |

Chưa chạy: các test cần Postgres trong `internal/persistence/postgres` (`TestClonePauseResumeKeepsCheckpoint`,
`TestCloneInterruptedRunningJobResumesPersistedStepCheckpoint`, `TestCloneRestartGeneration`,
`TestCloneRestartQueuesBeforeAdmission`, `TestReceiptCrashRecoveryOnPostgres`,
`TestChannelIdentityLeaseAndPublishFence`, `TestScanCrashRecoveryContinuesFromCursorWithoutDuplicates`)
và `images/journal_pg_test.go` — cần chiếm làn PG (`set -a; . /tmp/vs3.env; set +a` + `REQUIRE_POSTGRES_TEST=1`).
Lệnh: `go test -count=1 -run 'Pause|Resume|Recovery|Restart|Receipt' ./apps/viderse/internal/persistence/postgres/... ./apps/viderse/internal/pipeline/steps/images/...`.

## 3. Kịch bản trên serve thật

| # | Kịch bản | Kỳ vọng | Quan sát | Kết luận |
|---|---|---|---|---|
| 1 | Xếp 10 video của 3 kênh, chạy tới `images` | job chạy hết 6 bước đầu, ảnh trả phí vẽ đúng 1 lần | Job `6e24914a…` (kênh Một, `noImages=false`): `download` → `backdrop` → `transcript` → `metadata` → `scenes` → `images` đều `succeeded`; `image_receipts` có phiếu vẽ do stub trả; `build` dừng `waiting` | **MỘT PHẦN** — chuỗi chạy thật tới `images`, nhưng chỉ 1 job; 9 job còn lại chưa xếp vì §5 chặn ở `build` |
| 2 | Tạm dừng hàng chờ giữa chừng rồi chạy tiếp | checkpoint giữ nguyên, không mất ảnh đã vẽ | Chưa chạy (chặn bởi §5) | CHƯA CHẠY |
| 3 | `kill -9` ở từng bước (9 bước) | chạy tiếp từ checkpoint, không mồ côi, không vẽ lặp | Chưa chạy trên serve (chặn bởi §5). Bằng chứng gián tiếp: bộ test ở §2 (kể cả `TestSupervisorKill9RecoversSameRequestAndNoOrphanChrome`) | CHƯA CHẠY |
| 4 | Mất Postgres 30 s (qua TCP proxy riêng của serve) | hiện “Cần xử lý”, tự chạy tiếp khi PG trở lại | Chưa chạy | CHƯA CHẠY |
| 5 | Rút ổ thư mục dự án (disk image) khi đang chạy | “Cần xử lý” ≤ 10 s, không hỏng video, cắm lại thì chạy tiếp (V22) | Chưa chạy | CHƯA CHẠY |
| 6 | Ngắt mạng khi đang vẽ ảnh | không vẽ lặp, chờ hạn mức/ lỗi rõ ràng | Cần lượt vẽ trả phí thật → cấm trong phase này | BLOCKED (§7) |
| 7 | `kill -9` serve rồi `serve.sh resume` | job đang `waiting` vẫn còn, worker nhận lại khoá | Đã làm 1 lần (khởi động lại sau khi vá `ffmpeg`): serve lên lại, job giữ nguyên bước `download` `waiting` với đúng lỗi cũ, `POST /api/clone/<id> {action:"run"}` chạy tiếp thành công | **PASS (một phần)** |

Quan sát phụ đáng ghi:
- Ở lần chạy đầu, `download` báo `tool_missing` “ffmpeg/ffprobe chưa sẵn sàng” vì ffmpeg Homebrew
  thiếu `libwebp`; sau khi thay wrapper, `GET /api/tools` trả `ffmpeg ready`, `yt-dlp ready`,
  `transcription ready`, `chrome ready`.
- `metadata` hỏng 3 lần khi stub trả sai hình; cơ chế tự sửa theo JSON Schema chạy đúng (đòi `$.title`),
  và lần thứ 4 (sau khi sửa stub) qua — tức đường `llmcontract` + retry hoạt động thật.

## 4. Bằng chứng

- Log/DB: `p10_p10stab_data.clone_steps` (bảng ở §3), `clone_artifacts` của job `6e24914a…` gồm
  `download/source.mp4`, `download/source.vi.json3`, `download/source.info.json`, `backdrop/backdrop.json`,
  `transcript/captions.json|.vtt|.srt`, `transcript/speech-report.json`, `metadata/metadata.json`,
  `scenes/manifest.json`, `scenes/keyframes/sc-000.jpg`.
- `/api/tools` (sau khi vá): `ffmpeg ready /tmp/p10-stab/bin/ffmpeg`, `yt-dlp ready /tmp/p10-stab/bin/yt-dlp`,
  `transcription ready …/envs/transcription`, `chrome ready /Applications/Google Chrome.app/…`.
- `image_receipts`: có dòng cho lượt vẽ của stub (`x-request-id: stub-N`).

## 5. Lỗi thật: cổng “Cảnh đang chờ duyệt” chặn vĩnh viễn bước `build`

**Hiện tượng.** Mọi job đi qua `scenes` rồi dừng ở `build` với
`{"code":"awaiting_review","message":"Cảnh đang chờ duyệt","recovery":"Duyệt cảnh trước khi dựng"}`,
kể cả khi ảnh đã vẽ xong và kênh đặt tự duyệt (`autonomy.approve.mode=score,min=0`). Không có
đường nào trong sản phẩm gỡ được trạng thái này.

**Nguyên nhân (đọc mã).**
- `internal/pipeline/steps/scenes/handler.go:193` luôn ghi `m.Pending = &Analysis{Kind:"initial",…}`
  vào `scenes/manifest.json`.
- `internal/pipeline/steps/build/validation.go:187` từ chối dựng khi `manifest.Pending != nil`.
- Chỉ hai hàm gỡ `Pending`: `scenes.Approve` (`manifest.go:105`) và `scenes.ApplyEdits` (`edits.go:78`).
  `grep -rn "scenes.Approve\|ApplyEdits" apps/viderse` → **không có caller ngoài test**; không có route
  HTTP/CLI nào cho “duyệt cảnh”.
- Design V3 chỉ có **Duyệt ảnh** + “Duyệt cả video & dựng” (`08-dev-prompt.md:127–146`), không có màn
  duyệt cảnh. Vậy cổng `Pending` là tàn dư khi port, không phải hành vi design.

**Ảnh hưởng.** Không job nào tới được `build` → `score` → `export` trên bản chạy thật. Đây là lý do
trực tiếp khiến bước “Ổn định” không thể chạy các kịch bản 2–5 ở §3, và cũng chặn mục “một video đi
hết chuỗi tới export” của plan.

**Sửa tối thiểu (đã áp dụng, commit `046a92c7`).** `scenes` công bố luôn bản phân tích đã
được chấp nhận thay vì để `Pending`: `m.Approval = &Approval{Fingerprint, ApprovedAt}` và
`m.Pending = nil` (`internal/pipeline/steps/scenes/handler.go:193`). Lý do: design V3 chỉ duyệt
**ảnh**; không có màn/route nào duyệt cảnh, nên cổng `Pending` là hàng rào không có cửa. Sửa cảnh
sau đó vẫn tăng revision và xoá cả hai trường (`ApplyEdits`), cổng ở `build` giữ nguyên.

**Test hỏng-trước.** `TestScenesPublishAcceptedAnalysisWithoutPendingReview`
(`internal/pipeline/steps/scenes/handler_test.go`) — chạy trên mã cũ:
`--- FAIL: TestScenesPublishAcceptedAnalysisWithoutPendingReview … scene analysis left pending: build would wait for a review screen that does not exist`;
sau khi sửa: `ok … 0.565s`, và `go test -run 'Build|Template|Recipe' ./internal/pipeline/steps/build/`
vẫn `ok` (cổng `build` cho `Pending` vẫn được kiểm bởi test cũ).
Đã chạy lại trên serve: xem §5b.

### 5b. Chạy lại trên serve sau khi sửa

Serve dựng lại từ HEAD có `046a92c7`, xếp job mới `801d94a9…` (kênh Một, `noImages=false`,
`VIDERSE_DAILY_PAID_IMAGE_LIMIT=2`):

- `download` → `backdrop` → `transcript` → `metadata` → `scenes` **succeeded**; lỗi ở `build` **không
  còn** là “Cảnh đang chờ duyệt” nữa mà là “Ảnh chưa hoàn tất” — tức cổng cảnh đã được gỡ đúng như sửa.
- `images` chạm provider thật (stub) và **ghi 2 dòng `image_receipts`**, rồi hỏng ở bước mã hoá:
  `{"code":"internal_error","message":"ffmpeg thoát với mã 8: Error opening output files: Encoder not found"}`.
  Đây là **giới hạn harness**, không phải lỗi sản phẩm: ffmpeg Homebrew 9.0.2 trên máy này không có
  encoder `libwebp` (chính vì thế `libwebp` nằm trong danh sách encoder bắt buộc của app), nên bước
  `images` không ghi được file webp. Muốn đi hết chuỗi trên máy này cần cài ffmpeg có `libwebp`
  (ví dụ `brew install ffmpeg` từ tap đã bật webp, hoặc để app tự cài ffmpeg pin qua
  `POST /api/tools/ffmpeg/install` — cần mạng).

Kết luận cho §3 dòng 1: chuỗi chạy thật tới `images` **đạt** (kể cả phiếu vẽ), nhưng chưa qua được
`build`/`score`/`export` vì encoder webp của máy; các kịch bản 2–5 vì vậy vẫn **CHƯA CHẠY**.


## 6. Kịch bản còn lại — đúng lệnh để chạy sau khi §5 được sửa

0. **Chạy lại end-to-end sau khi sửa (việc đầu tiên).** `serve.sh setup 18911` (build lại từ HEAD
   đã có `046a92c7`), dựng lại kênh/template/video nguồn như §1, xếp 1 job, chờ qua `build` → `score`
   → `export`; nếu `score` báo chưa đạt thì bấm “Vẫn xuất” (`POST /api/clone/<id> {action:"run"}` sau
   khi chấp nhận cổng chất lượng) để đi hết chuỗi.

1. **10 video / 3 kênh tới `images`.** `for i in 1..10: POST /api/clone {channelId:<kênh i%3>, urls:["https://www.youtube.com/watch?v=stab000000$i"], noImages:false}`;
   theo dõi `select step,status,error from clone_steps where job_id=…`. Hạn mức 1 ảnh/ngày ⇒ 1 job qua
   `images`, 9 job còn lại phải hiện nhánh **chờ hạn mức** (`quota`, có `retryAt`).
2. **Tạm dừng/ chạy tiếp.** `PATCH /api/queue {"paused":true}` giữa lượt, chờ 1 job dừng, `{"paused":false}`;
   kiểm `clone_steps.checkpoint` không đổi và không có dòng `clone_artifacts` trùng `sha256`.
3. **`kill -9` từng bước.** Với mỗi bước: xếp 1 job, đặt stub `slow_ms` (metadata/scenes) hoặc
   `STUB_YTDLP_STEP_MS`/`STUB_TRANSCRIBE_SEC` cho dài, `kill -9 <pid serve>`, `serve.sh resume`,
   `POST /api/clone/<job> {"action":"run"}`; kiểm (a) bước đó chạy tiếp từ checkpoint, (b)
   `select count(*) from image_receipts` không tăng, (c) `ps -o pid,pgid,command -g <pgid cũ>` rỗng.
4. **Mất Postgres 30 s.** Chạy serve qua proxy TCP của mình:
   `socat TCP-LISTEN:18917,fork,reuseaddr TCP:<host>:<port>` (hoặc proxy Python), đặt
   `VIDERSE_DATABASE_URL=postgres://…@127.0.0.1:18917/…`; giữa lượt `kill` proxy 30 s rồi bật lại;
   kiểm dải “Cần xử lý”, rồi job tự chạy tiếp không cần bấm.
5. **Rút ổ thư mục dự án.** `hdiutil create -size 2g -fs APFS -volname P10Proj /tmp/p10proj.dmg && hdiutil attach /tmp/p10proj.dmg`;
   đặt vị trí thư mục dự án (`PUT /api/storage {location:"/Volumes/P10Proj/projects"}`) và chờ chuyển xong;
   giữa lượt `hdiutil detach /dev/diskN`; đo thời gian tới khi `GET /api/status` có mục “Cần xử lý”
   (≤ 10 s), kiểm không có video hỏng; `hdiutil attach` lại → job tự chạy tiếp.
6. **Ngắt mạng khi vẽ ảnh.** BLOCKED — xem §7.

## 7. BLOCKED

- **Ngắt mạng khi đang vẽ ảnh** cần ít nhất một lượt gọi provider trả phí thật (để có phiếu và
  đường reconcile thật). Trong phase này cấm. Khi có tài khoản thật, chạy:
  đặt Hoạ sĩ về provider thật, `sudo pfctl -e` + chặn domain provider **hoặc** `kill -STOP` process
  serve giữa lượt vẽ, rồi `kill -9`; khởi động lại và kiểm `image_receipts` chỉ có **một** dòng cho
  cảnh đó, `images/manifest.json` có `inFlight` được adopt (không vẽ lại), và job không mất checkpoint.
- **Kịch bản 2–5 (§3)** phụ thuộc §5: cho tới khi cổng duyệt cảnh được nối, không job nào qua `build`
  nên không có gì để `kill -9` ở `build/score/export` trên serve thật.

## 8. Dọn dẹp

`/tmp/p10-stab/serve.sh drop` (xoá `p10_p10stab_data`, `p10_p10stab_identity`, build, HOME), tắt
`stub.py` (:18915), xoá `/tmp/p10-stab`, `/tmp/p10-data-P10Stab`, `/tmp/p10-build-P10Stab`,
`/tmp/p10-P10Stab-viderse`. Không đụng `verse-live`, `verse-port`, port 8790.
