# Bằng chứng bàn giao các phase 03, 07, 08

Trích nguyên văn từ báo cáo bàn giao của phiên thực thi (`/tmp/lm-reports-bug2710.md`, file tạm trên máy làm việc) ngày 2026-10-28/29, để plan không phải trỏ vào `/tmp`. Đường dẫn và lệnh trong file này tính từ gốc repo cũ, tức `apps/noverse/` trong `verse`; quy ước đường dẫn đầy đủ ở [plan.md](../plan.md).

## DbPhase03

**Đỏ trước sửa (mutation, đã restore sau mỗi lần):**
- Bỏ `ValidateKey` trong `Local.Stat` → `go test ./internal/blobstore -run 'TestLocalConfinement|TestConformanceLocal/invalid_keys'`: FAIL `Stat("") err = statat: empty path, want ErrInvalidKey`.
- Bỏ bound-check trong `resolveRange` → `TestConformanceLocal/range_read`: FAIL `out-of-range err = <nil>, want ErrInvalidRange`.
- `LeavePartsOnError=true` → `TestMultipartAbort` trên MinIO thật: FAIL `orphaned multipart uploads: [{... UploadId ...}]`.
- Bypass `BlobDelivery.authorize` → `TestBlobDelivery/auth_denial` FAIL `denied status = 200, want 403`; `nil_authorizer` FAIL `200, want 500`.

**Xanh sau implement (đã quan sát, `-count=1`):**
- `docker compose -f deploy/compose.test.yml -p ainovel-p3test up -d postgres minio` → cả hai healthy (postgres:17 @55432, minio @59000/59001, tmpfs).
- `go test ./internal/blobstore -run TestConformanceLocal`: PASS — 10 subtest (put/stat/get, range, unicode keys, empty, 9MiB, pagination 12@5, delete idempotent, ExpectedSHA256 mismatch, invalid keys, presign→ErrPresignUnsupported).
- `MINIO_TEST_ENDPOINT=http://127.0.0.1:59000 REQUIRE_MINIO_TEST=1 go test ./internal/blobstore -run TestConformanceMinIO`: PASS — cùng suite trên SDK/network thật; 9MiB multipart (PartSize 8MiB) + presigned GET trả http URL.
- `go test ./internal/blobstore -run 'TestLocalConfinement|TestMultipartAbort|TestCopyResumeHash'`: PASS — symlink escape/`..`/backslash/absolute bị chặn bằng os.Root + ValidateKey; abort sạch (0 multipart upload); resume ledger + verify SHA-256.
- `go test ./internal/entry/web -run TestBlobDelivery`: PASS — 11 subtest: 403 denial, 500 nil-authorizer, private headers (Cache-Control/CSP/Accept-Ranges/X-Content-SHA256), 206 + `Content-Range bytes 4-9/16`, suffix `bytes=-4`→`cdef`, 416 + `bytes */16` (kể cả multi-range), HEAD len=16, 404, sign→presigned/stream fallback (unsupported + lỗi ký).
- `go test ./internal/blobstore ./internal/bootstrap`: PASS (TestS3ConfigValidation, TestBlobConfig 8 subtest, TestCopy*); `go test ./internal/persistence/...` với `TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:55432/ainovel_test`: PASS (phase-02 vẫn xanh).
- `go build ./...`, `go vet` 4 packages, `gofmt -l`: sạch.
- CLI live: `storage copy --from-config local.json --to-config s3.json --manifest m.jsonl` local→MinIO: `copied=3` (300KB, 7MB, 10B), mỗi object re-download verify SHA-256+size; thiếu bucket → exit 1 `NoSuchBucket`; `--resume` bỏ qua theo ledger (có re-Stat); same-size khác nội dung → `conflict`, exit 1, không ghi đè; s3→local chain PASS trong `TestCopyLocalToMinIO`.
- Containers đã `down -v`; `docker ps -a` còn 0.

### Phạm vi tệp
Tạo: `internal/blobstore/{blobstore,internal,local,s3,copy}.go` + 4 test file; `internal/entry/web/blob.go` + `blob_test.go`; `internal/bootstrap/blobstore_test.go`; `deploy/compose.test.yml`. Sửa: `internal/bootstrap/storage.go` (BlobConfig/OpenBlobStore), `cmd/ainovel-cli/storage.go` (subcommand copy), `README.md` (mục BlobStore §9), `go.mod/go.sum` (aws-sdk-go-v2 v1.47.5, credentials, service/s3 v1.114.6, feature/s3/manager v1.23.18 — family đồng nhất; manager là nhánh plan chỉ định, cờ deprecated của upstream về transfermanager chưa áp dụng), `phase-03-blobstore.md` (status done + evidence). Không đụng `internal/store`, audio callers, gateway routes — live giữ file mode như spec.

### Quyết định trong phạm vi
- Route dạng `/api/blobs/stream/{key}` + `/api/blobs/sign/{key}` thay `{key}/stream` — Go ServeMux chỉ cho `{...}` ở cuối pattern; README + test đồng bộ.
- `BlobInfo.SHA256` bảo đảm ở Put và khi object có user-metadata `sha256` (S3 gắn khi PutOptions.ExpectedSHA256); Stat/Open nói rõ là best-effort vì stream-upload chưa biết hash lúc PUT. Digest chuẩn luôn tự tính, ETag không bao giờ là hash.
- `S3_ALLOW_INSECURE_ENDPOINT=true|false` strict là cờ opt-in duy nhất cho endpoint http:// (MinIO/private); không cờ → http bị từ chối.
- Resume re-Stat đích thay vì tin ledger mù: key đã `copied` mà target biến mất sẽ được copy lại.

### Phòng tái diễn / giới hạn
Chỉ local + MinIO đã qua conformance; 5 provider còn lại là recipe cấu hình trong README, cần smoke trước cutover (đúng phase doc). Delivery seam chưa public-wire: authorizer thật (blobs table + project permission) thuộc 04/05. GCS checksum/presign quirks được mitigate bằng WHEN_REQUIRED + independent SHA-256 nhưng chưa có bucket thật để đo.

Status: DONE
Summary: Phase 03 hoàn tất — BlobStore interface + local driver (os.Root, atomic rename, fsync) + một S3 driver cho 7 recipe, multipart abort sạch, presign+fallback streaming seam có injected authorizer, copy tool verify SHA-256 có ledger resume; conformance suite giống hệt nhau pass trên local và MinIO thật; fail-before chứng minh bằng 4 mutation; build/vet/gofmt sạch; containers đã gỡ.
Concerns: MinIO-only conformance (đúng gate); audio/export callers chưa chuyển (theo phase 07); authorizer production chờ 04/05; transfermanager migration là việc tương lai khi SDK promote nó khỏi preview.

## DbPhase07c

Status: **green build + green full suite; hosted audio (queue/playback/export)
works end to end on Postgres; only real-VieNeu acceptance and `text_export`
remain.**

Summary
-------
1. `internal/entry/web/audio_pg_test.go` rebuilt (it was syntactically broken:
   the test function header was missing and the body referenced APIs that no
   longer exist). It is now two real gates over the real HTTP handlers and the
   real worker listener:
   - `TestPGAudioQueuePlaybackHTTP` — enqueue (`POST /audio/queue`) writes the
     durable job with the Gói AK snapshot (`kind`, `chapterId`,
     `scriptRevision`, `segments[].id/synthKey/text`, no `updateEtaSec` for a
     create); a fake worker claims/uploads/ACKs through `/internal/worker/*`;
     `GET /audio/segments/1-s0001/audio` streams the blob (bytes,
     `X-Content-SHA256`, `Range` → 206); timeline reports `done`; flags
     round-trip; the settings ETag moves after a save; `peaks` is 501.
   - `TestPGAudioExportCacheHTTP` — `POST /audio/volumes/1/exports` enqueues an
     `audio_export` job; `audio.PGExportRunner` claims it, materializes the
     locked inputs into a scratch layout, reuses the file export pipeline,
     publishes outputs as blobs and writes the `export_cache` manifest inside
     the completion transaction; `GET …/exports` lists files with
     `/api/blobs/stream/…` URLs; an identical second request finishes with
     `cacheHit: true` and no ffmpeg run.
2. Unverified items fixed:
   - `SettingsVersion` is a real token now: `postgres.Audio.Settings` returns
     `(payload, version, err)`, `PGService.doc` carries the row version, token
     is `pgv-<version>` (was the constant `"pg-doc"`).
   - `ListChapters` (`pg_views.go:33`) and `ParseUUID` `.Valid` (worker token
     revoke by name) were checked against the real signatures — already
     correct, now covered by build/vet.
   - `pgAudioServices` was a package-global `sync.Map` keyed by story slug, so
     two gateways on different pools/schemas shared one service. It is now
     per-gateway (`g.pgAudioSvcs` + `g.pgAudioMu`, closed from
     `Gateway.Close`).
   - `checkIfMatchV` (new helper used by settings/cast/script) returned 412 and
     rejected bare ETags, while the file-mode contract is 409 +
     `{conflict:true, version, current}` and accepts bare/quoted tokens. This
     broke `TestAudioSettingsVersionConflictReturnsCurrent` and
     `TestAudioCastVersionConflictAndChangeEvent` (they were never run after
     the predecessor's edits). Both now share `checkIfMatch` semantics.
3. Hosted ffmpeg export implemented (`internal/audio/pg_export.go`):
   content key over chapters + script revisions + selected render blob ids +
   cast + settings + voices + options + engine/model; durable runner worker
   `vps-ffmpeg-export` started in `serve` (PG mode); scratch layout feeds
   `Service.Export` unchanged; outputs are content-addressed blobs (a re-export
   reuses the existing ready registry row instead of violating the `blobs`
   object-key unique index — this was a real bug the cache-disabled mutation
   exposed); `ExportInfo` reads the manifests; `export_cache.kind` is the
   artifact format (`mp3|m4b|wav|zip`) per the schema check constraint.
4. `migrations/migrations.go` numbering comment reconciled with the shipped
   files (00004 story documents, 00005 reserved/skipped, 00006 lease key,
   00007 workers/jobs/audio — plan allocation 00004).
5. `plans/261028-1340-db-vps-storage/phase-07-audio-jobs.md` gained the
   `DbPhase07c` evidence block; `status` stays `in-progress` (real provider
   acceptance outstanding).

Fail-before (each mutation reverted; build re-run)
--------------------------------------------------
- PG seam removed in `audioService` (`if false && g.pgAudio != nil`) →
  `TestPGAudioQueuePlaybackHTTP`: `audio_pg_test.go:112: settings: 500
  {"error":"stat : no such file or directory"}` (the file backend has no story
  directory in the test).
- Export cache lookup disabled (`exportCacheHit` always misses) →
  `TestPGAudioExportCacheHTTP`: `second result must be a cache hit:
  map[cacheHit:false …]`; the same mutation then failed at blob registration
  (`duplicate key` on the object key), which is what motivated the
  reuse-ready-row guard.
- `checkIfMatchV` restored to the old 412/quoted-only behaviour →
  `TestAudioSettingsVersionConflictReturnsCurrent`: `save settings: 412
  {"error":"Nội dung đã đổi trong lúc bạn xem."}` and
  `TestAudioCastVersionConflictAndChangeEvent`: `matching If-Match: 412 …`.

Verification (this pass)
------------------------
- `go build ./...` → clean; `go vet ./...` → clean; `gofmt -l internal cmd` →
  no output.
- `go test ./internal/entry/web -run 'TestPGAudioQueuePlaybackHTTP|
  TestPGAudioExportCacheHTTP' -count=1` (PG) → ok.
- `REQUIRE_POSTGRES_TEST=1 REQUIRE_MINIO_TEST=1 MINIO_TEST_ENDPOINT=
  http://127.0.0.1:59000 TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:55432/ainovel_test
  go test ./... -count=1 -timeout 1800s` → 183 s, **no FAIL lines** (identical
  DB/MinIO containers as the phase-07 gate run).
- Containers removed: `docker compose -f deploy/compose.test.yml down -v`
  → `docker ps -a` shows none.

Remaining (precise)
-------------------
1. Real-provider acceptance not run: `TestRealVieNeuVolumeOneM4B` and
   `worker tts --api <tailnet-url>` against a queued job need a Mac with
   VieNeu, `AINOVEL_AUDIO_SMOKE_ENDPOINT`, ffmpeg/ffprobe and a minted token.
   Nothing is claimed as a mock/NullSynth pass.
2. `text_export` (txt/epub/srt/ZIP) has no runner; those formats still go
   through the file-mode export path, not a durable job.
3. Hosted export does not materialize cover art (`<assets>/bia`) or music
   (`<assets>/nhac`), so `cover`/`music` inputs of the cache key are constant
   and intro/outro music is never mixed on hosted.
4. Hosted `ExportInfo` carries no waveform peaks; `peaks` stays 501.
5. Export jobs appear in the queue snapshot but the job dock has no
   pause/retry affordance for them (TTS rows only).

Concerns
--------
- `blobs` unique index on (project, object_key) means any producer that
  re-publishes the same key must check for an existing ready row first; the
  export path now does, the TTS worker path gets unique keys by construction.
- Leftover stale spec text: the phase's "Files/ownership" section still names
  `00004_jobs_audio.sql`; the shipped file is `00007_jobs_audio.sql`.

## DbPhase08

Status: **done — phase gate met on real data (0 mismatches, 0-row/0-blob rerun, restore verified)**.

### Delivered
- `internal/persistence/migrations/00005_import_ledger.sql` (new): `library_settings`, `import_runs` (unique `(source_id, manifest_sha256)`, version guard), `import_items` (PK `(source_id, relative_path, record_key)`, immutable trigger, dispositions `imported|blob|derived|excluded|tolerated|quarantined`, raw+semantic hashes), `import_lock_key()` for the advisory lock.
- `internal/storageimport/` (new package, 8 files):
  - `manifest.go` — dispositions, canonical JSON, semantic digest with UTC/µs time normalization, deterministic UUIDv5 ids, BlobObjectKey, content types.
  - `scan.go` — read-only `os.Root` walk; every path classified (facts/blob/derived/scratch/local-only/excluded); symlinks and unreadable paths recorded as quarantined; legacy anonymous project `_` and legacy story dirs discovered; synthesized `project.json`/`story.json` for the legacy layout; unadopted candidate dirs excluded with a reason.
  - `decode.go` — pure decoders for accounts, library/project/story, chapter records + prose, outlines/characters/state/artifacts, comments, reviews, checkpoints/JSONL streams/agent sessions, audio settings/cast/scripts/queue/export requests.
  - `importer.go` — FK-ordered phases, one bounded tx per record, ledger precheck (identical→skip, drifted→loud conflict), advisory lock, run row, dry-run.
  - `rows.go` — single writer switch, `ON CONFLICT DO NOTHING` everywhere (never DO UPDATE), digest re-read on conflict.
  - `reconcile.go` — per-item semantic re-read, per-kind row counts vs ledger, chapter coverage, blob re-download+hash, exactly-one-owner invariant.
  - `export.go` — reverse export into a new dir: library/project/story config, documents, book/format/progress/run/state, outlines, characters, chapter records+prose+drafts, comments, review rounds, audio settings/cast/scripts, artifacts, and every blob at its ledger-recorded relative path; optional session/pairing revocation.
  - `report.go` — manifest read/write and Vietnamese summaries; `importer_test.go` — scan determinism, id determinism, idempotency, live-edit protection, source drift, corrupt-blob detection, export round-trip, manifest round-trip.
- CLI: `storage import|reconcile|export-files` in `cmd/ainovel-cli/storage.go` (credentials only from env).

### Verification (commands run)
- `go test ./internal/storageimport -count=1 -timeout 1800s` (REQUIRE_POSTGRES_TEST=1): **ok**, 8 tests.
- Fail-before (real mutation, not asserted): disabling the ledger precheck + making `BlobObjectKey` non-deterministic made `TestImportIsIdempotent` fail with `lần hai phải tải 0 blob, thực tế 3`; restored.
- Rehearsal on `/tmp` copy of `~/NoverseLive` (981 MiB, 2911 paths) + copy of `~/.ainovel/web-accounts.json`, PG+MinIO from `deploy/compose.test.yml`:
  - dry-run clean (1 account, 1 project, 2 stories, 4 sessions to invalidate);
  - import: 9409 rows, 71 blobs (226.5 MiB), 1996 derived, 47 excluded, 0 quarantined; run `verified`;
  - **rerun: 0 rows added, 0 blobs uploaded**;
  - `reconcile --verify-blobs`: 2911 paths, 9503 records, 71/71 blobs re-hashed — **0 mismatches, 0 owner violations**;
  - `export-files`: 707 files + 71 blobs (462.4 MiB);
  - `pg_dump -Fc` → `pg_restore` into `ainovel_restore`, `mc mirror` into `ainovel-restore`; reconcile against the restored DB+bucket: **0 mismatches, 71/71 blobs verified**.
- `gofmt -l` (import/CLI/migrations): clean; `go vet ./internal/storageimport ./cmd/ainovel-cli`: clean; `go build ./...`: clean.
- `docker compose -f deploy/compose.test.yml down -v`: containers and volumes removed (0 containers left).

### Dispositions on real data (each counted in the ledger with its raw hash)
- derived/excluded (1996 + 47): audio cache (`audio/cache/**`), export cache, `nao/**`, `logs/**`, markdown projections (`book.md`, `characters.md`, `world_rules.md`, `outline.md`, `layered_outline.md`), locks/temp files, root `.ainovel` local config, the 4 stale account sessions (counted, not copied).
- `tolerated`: truncated trailing JSONL lines (recorded, do not block publish; none on real data).
- Blobs: chapter audio, export products, project assets, voice samples, imported `source.txt`, audio cover images. Audio *render cache* is not imported (regenerable from script + voice; segment metadata + render keys stay in `audio_scripts`/`audio_segments`).

### Remaining / concerns
1. **Hosted cover/peaks for audio** and `text_export` (phase 07 leftovers) are untouched by this phase.
2. Reverse export does not write the accounts file (it exports library/project data only) and does not emit `xuat-ban` products other than the imported blobs; new chapter audio rendered after cutover has no file export path yet. Sessions/pairings are only revoked with `--revoke-sessions`.
3. Real-data import showed 92 records skipped on the *first* run: distinct source records that normalize onto the same target row (e.g. two pending-state files of the same kind). They are counted, not silent; reconcile compares the surviving row.
4. `usage.json`-style per-day usage documents are preserved as artifacts, not normalized into `story_events`.
5. `.ainovel/config.json` at the library root (local provider/secret config) is excluded by design and must be reconfigured on the VPS.
6. The rehearsal used the local MinIO from `compose.test.yml`; a production S3 bucket has not been exercised.
