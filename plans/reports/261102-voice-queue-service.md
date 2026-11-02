# Gói voice queue — hosted "AI sẽ soạn bản đọc" (GenPickEmpty)

Ngày: 2026-11-02 · Nhánh làm việc: `apps/noverse` (main, uncommitted)

## Vấn đề

Trên kho Postgres, `PGService.QueueChapters` từ chối chương đã nghiệm thu nhưng
chưa có bản đọc ("Chương N chưa có bản đọc — hãy soạn bản đọc trước khi tạo
giọng."). Điện thoại (GenPickEmpty) hứa **tự soạn → chốt → tạo giọng**, nên
người dùng chọn chương trong sheet không có bản đọc thì hàng đợi không nhận.
Bản file (`mobile_queue.go` + `render.go`) đã làm được; hosted thì chưa.

## Thiết kế (đã chốt với slice persistence)

Cùng một kind công việc `tts`; input JSON có `needsScript: true`, `segments: []`,
`progress_total = 0`:

1. `PGService.QueueChapters` nhận chương đã nghiệm thu khi `lock: true` (đúng
   parity file backend). Không có bản đọc hoặc bản đọc cũ (`needsComposition`)
   → ghi hàng đợi dạng **preparation**. `Enqueue` một chương vẫn giữ nguyên.
2. Remote worker **không bao giờ** thấy hàng `needsScript`: `ClaimJob` của
   slice persistence loại trừ bằng so sánh text `input->>'needsScript'`, và các
   truy vấn preparation chuyên dụng giữ `worker_id NULL` (ràng buộc DB cấm
   `running` khi không có worker).
3. Web sở hữu runner (`internal/entry/web/audio_prepare.go`):
   - `ListPreparationTargets` → mỗi story một vòng tuần tự (không gọi model
     song song trong một truyện).
   - `ClaimPreparation(scope, 5m)` → `audio.PGService.PrepareClaimed`.
   - Thức dậy bằng hook `SetPreparationWake` khi enqueue/resume/retry; không
     poll khi rảnh. Hàng đang giữ lease còn sống → hẹn giờ đúng `lease_until`
     (khôi phục sau restart không kẹt 5 phút).
   - `Close` dừng runner trước khi đóng audio service (chờ có giới hạn 5s).
4. `PrepareClaimed` (trong `internal/audio/pg_prepare.go`):
   - Xác nhận đúng scope của service; nhận **đúng nguồn chương mà HTTP queue đã
     kiểm** (runner đọc lại bằng chính `audioVolumeSource`/StoryReader của
     request path ngay trước khi soạn) và chụp `script hiện tại (CAS version)`
     **trước** khi gọi model.
   - Model call ngoài transaction (`pgComposeChapterTimeout`), không có
     provider nào được tự tạo trong audio service.
   - Hợp nhất với bản cũ (`mergeScript`), luôn `StatusLocked`, giữ
     `LockedAt`, áp `applyVoiceFlags`.
   - `CompletePreparationWith` khoá hàng job + kiểm lease/fence trước, rồi
     trong cùng transaction: `SaveScriptBundleTx` (CAS) và thay input của
     **chính hàng đó** bằng input render đã chuẩn bị (revision/segments thật,
     ownership lấy từ `PlaybackRevisionTx`). Tổng 0 đoạn → hàng thành
     `completed` (đã có đủ giọng), không tạo job 0 đoạn cho worker.
   - Pause/cancel trong lúc model chạy: publish trả `ErrJobLeaseLost` → không
     có script, không có claim render; hàng giữ trạng thái điều khiển.
   - Lỗi provider → `FailPreparation` (needsScript giữ nguyên), Retry/Resume
     đưa lại hàng đợi và runner soạn lại.
5. Hợp đồng UI có sẵn (`needsScript`, `step: "compose"`) nay được hosted điền
   thật trong `pgJobView`; hàng preparation không báo số đoạn/ETA giả.

## Sửa lỗi sau cổng PG (đo trên DB thật)

- **`script thiếu chapter/revision`** (mọi test preparation, và compose hosted khi
  ghi bản đọc mới): `scriptBundle` ghi `RevisionNo = script.pgRevision`, nhưng
  script mới soạn có `pgRevision = 0`; `postgres.Audio.validateScriptMeta` đòi
  revision > 0. File backend ghi revision 1 cho lần đầu. Sửa: `scriptBundle`
  mặc định revision 1 khi chưa có (dùng chung cho compose/lock/refresh/prepare,
  một nguồn duy nhất) và prepared input dùng `saved.RevisionNo` thật.
- **`chương chưa có nội dung`** (compose hosted): fixture chỉ có chapter record +
  progress, không có artifact `chapter_text`; `StoryReader.ChapterText` trả lỗi
  nên `sc.Text` rỗng (ChatComposer từ chối). Đây là lỗ chức năng thật: chương đã
  nghiệm thu qua narrative/revision (mobile commit) mà chưa có file prose vẫn
  phải soạn/queue được. Sửa trong `audioVolumeSource`: khi không có artifact,
  đọc **bản ghi đã nghiệm thu** (`sr.Store().ChapterRecords.Load`) làm "chính
  văn" (hash/độ tươi vẫn so với đúng nguồn đó ở queue, compose và timeline).
  Không có store nào bị "đóng" trong đường async: `audioVolumeSource` chạy đồng
  bộ trong handler, composer chỉ giữ model client.
- **`workers_token_hash_check`**: test đăng ký worker với hash 10 byte; dùng
  `sha256` 32 byte như helper worker hiện có. Không nới ràng buộc DB.
- Test preparation **không** seed artifact nữa: chúng đi đúng đường fallback
  chapter record như compose regression (không che lỗi).

## Tệp thay đổi

- `apps/noverse/internal/audio/pg_jobs.go` — `pgTTSInput.needsScript`; tách
  `chapterQueueSlot`/`enqueueTTSRow`; `enqueueTTSPrep`; `QueueChapters` nhận
  chương thiếu/cũ bản đọc; `pgJobView` phát `needsScript`/`step`; wake hook ở
  Resume/Retry.
- `apps/noverse/internal/audio/pg.go` — `prepWake` + `SetPreparationWake` +
  `wakePreparation`.
- `apps/noverse/internal/audio/pg_views.go` — tách `scriptBundle` +
  `persistScriptTx` (dùng `Audio.SaveScriptBundleTx`).
- `apps/noverse/internal/audio/pg_prepare.go` (mới) — `PreparationInput`,
  `PrepareClaimed`, `failPreparation`, `releasePreparation`.
- `apps/noverse/internal/entry/web/audio_prepare.go` (mới) — runner per-story,
  scan/wake, lease-expiry timer, composer nội bộ tin cậy + nhân vật.
- `apps/noverse/internal/entry/web/gateway.go` — state runner + `Close`.
- `apps/noverse/internal/entry/web/audio.go` — `SetPreparationWake` khi tạo
  service; `audioComposerInternal` (chuỗi config hosted, actor nil nội bộ).
- `apps/noverse/cmd/ainovel-cli/serve.go` — `StartAudioPreparation(ctx)` sau
  khi audio deps đã nối (khôi phục queued + lease còn sống).
- Tests: `apps/noverse/internal/audio/pg_prepare_test.go`,
  `apps/noverse/internal/entry/web/pg_audio_prepare_test.go`.

Không sửa: `internal/persistence/postgres/*` (slice A), frontend, migration,
`pg_compose.go`/`pg_compose` gate, remote TTS wire.

## Kiểm chứng (chạy trên máy có Postgres test)

```
cd apps/noverse
go test ./internal/audio/ -run 'TestPGJobViewPreparationContract' -count=1
go test ./internal/entry/web/ -run 'TestPGHosted(QueueComposesMissingScript|Preparation)' -count=1
go test ./internal/entry/web/ -run 'TestPGHostedCompose' -count=1   # 4 ca cũ, không đổi
```

Các test PG dùng `pgtest.Provision…` (tự skip khi không có Postgres). Provider
chỉ là endpoint OpenAI-compatible cục bộ; không gọi nhà cung cấp thật.

## Bất biến đã giữ

- Không gọi provider nếu không có hàng đợi `tts` đã claim.
- Không thêm kind job mới; không đăng ký worker giả; không claim chéo truyện.
- Script CAS snapshot trước model call; pause/cancel thắng bằng fence.
- Không heartbeat giả; lease preparation hết hạn được runner hẹn giờ chính xác.
