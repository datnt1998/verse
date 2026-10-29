---
phase: 7
title: "Audio queue, background jobs, export cache và worker Mac"
status: in-progress
priority: P1
effort: 36h
dependencies: [3, 6]
---

# Phase 07 — Audio/jobs bền vững, TTS kéo việc từ Mac

## Progress evidence (2026-10-29, DbPhase07c)

Hosted audio is wired end to end on Postgres; the tree builds and the suite is
green. File mode is unchanged (`STORE_DRIVER` still selects the backend, Gói AK
semantics preserved on the file backend).

Done this pass:

1. `internal/entry/web/audio_pg_test.go` repaired and rewritten as two real
   gates over HTTP + real worker listener:
   - `TestPGAudioQueuePlaybackHTTP`: enqueue over `POST /audio/queue` writes the
     durable job with the Gói AK snapshot (`kind`, `chapterId`,
     `scriptRevision`, `segments[].synthKey`; `updateEtaSec` absent for a
     create); a fake worker claims/uploads/ACKs over `/internal/worker/*`; the
     segment endpoint streams the blob (bytes + `X-Content-SHA256` + range);
     timeline reports `done`; flags round-trip; settings ETag moves after a
     save; `peaks` answers 501.
   - `TestPGAudioExportCacheHTTP`: `POST /audio/volumes/1/exports` enqueues an
     `audio_export` job, `audio.PGExportRunner` claims it, materializes the
     locked inputs to scratch, reuses the file export pipeline, publishes the
     produced files as blobs + the `export_cache` manifest inside the
     completion transaction; `GET …/exports` lists them with
     `/api/blobs/stream/…` URLs; an identical second request completes with
     `cacheHit: true` without running ffmpeg.
2. `SettingsVersion` is a real token: `postgres.Audio.Settings` now returns the
   row version, `PGService.doc` carries it, and the token is `pgv-<version>`
   (was the constant `pg-doc`).
3. `pgAudioServices` was a package-global `sync.Map` keyed by story slug — two
   gateways on different pools shared a service. It is now per-gateway
   (`g.pgAudioSvcs` under `g.pgAudioMu`, closed by `Gateway.Close`).
4. `ListChapters` call site (`pg_views.go:33`) and `ParseUUID` `.Valid` uses
   (`worker token revoke`) verified against the actual signatures; both were
   already correct, now covered by the build/vet gate.
5. Verified pre-existing wiring: `serve` starts the private listener when
   `WORKER_LISTEN_ADDR` is set (`runWorkerListener`, PG `WorkerAPI`), claims
   are TTS-only, and `worker token create|revoke|list` stores only the hash.
6. Hosted ffmpeg export (`internal/audio/pg_export.go`): content key over
   chapters + script revisions + selected render blob ids + cast + settings +
   voices + options + engine/model; `PGExportRunner` (durable worker row
   `vps-ffmpeg-export`) runs on the VPS in `serve`; scratch layout feeds
   `Service.Export` unchanged; outputs are content-addressed blobs (re-export
   reuses the existing registry row instead of inserting a duplicate);
   `ExportInfo` reads the manifests; `export_cache.kind` is the artifact format
   (`mp3|m4b|wav|zip`) per the schema constraint.
7. `migrations/migrations.go` numbering comment reconciled with the shipped
   files (00004 story documents, 00005 reserved/skipped, 00006 lease key,
   00007 jobs/audio — plan allocation 00004).

Fail-before evidence (mutations reverted):

- PG seam removed from `audioService` (`if false && g.pgAudio != nil`) →
  `TestPGAudioQueuePlaybackHTTP`: `audio_pg_test.go:112: settings: 500 {"error":
  "stat : no such file or directory"}`.
- Export cache lookup disabled (`exportCacheHit` always misses) →
  `TestPGAudioExportCacheHTTP`: `second result must be a cache hit:
  map[cacheHit:false …]` (and it exposed a real bug: re-registering the same
  content-addressed object key violated the `blobs` unique key — fixed by
  reusing the ready registry row).

Verification runs (this pass): see the DbPhase07c report block in
`/tmp/lm-reports-bug2710.md` (gofmt/vet clean, PG + MinIO suite).

Remaining (precise):

1. Real-provider acceptance (`TestRealVieNeuVolumeOneM4B`, `worker tts` against
   a tailnet URL with a queued job) was not run: it needs a Mac with VieNeu, a
   reachable `AINOVEL_AUDIO_SMOKE_ENDPOINT`, ffmpeg/ffprobe and a minted worker
   token. No NullSynth/mock result is claimed as that acceptance.
2. `text_export` (txt/epub/srt/ZIP) jobs are still not implemented: only
   `audio_export` has a runner. `POST …/exports` for text formats still goes
   through the file-mode export path (`serveExportCreate`), not a durable job.
3. Hosted export ignores `Cover`/music/`bản đọc` companion inputs (the scratch
   layout has no `<assets>/bia` or `<assets>/nhac`), so cover art and intro/outro
   music are only produced when those assets are added to the scratch layout —
   the `cover`/`music` parts of the cache key are therefore constant today.
4. `ExportInfo` reports no peaks (512 KB waveform) on hosted; the peaks endpoint
   remains 501 by design.
5. `queueSnapshot` surfaces `audio_export` rows in the queue view, but the web
   dock has no pause/retry affordance for export jobs yet (only TTS rows are
   actionable in the UI).

## Progress evidence (2026-10-29, DbPhase07)

Done and verified (file mode untouched; `STORE_DRIVER` still selects the backend):

- `internal/persistence/migrations/00007_jobs_audio.sql` (allocation 00004 in the
  schema report; renumbered because 00004–00006 already shipped): `workers`
  (sha256 token, capability `tts`), `jobs` (SKIP-LOCKED queue: state, lease,
  `fencing_token`, per-(story,kind) idempotency key), append-only `job_events`,
  `audio_settings/voices/cast/scripts/segments/renders`, `export_cache`.
  `audio_renders` carries `synth_key` + one-selected-per-segment partial unique
  index; immutability/version triggers reuse 00003's functions.
- `internal/persistence/postgres/jobs.go`: `Enqueue` is idempotent by
  `(project, story, kind, idempotency_key)` (`ON CONFLICT DO NOTHING` + resolve,
  so a duplicate lock does not abort the transaction); `Claim` = one short
  transaction with `FOR UPDATE SKIP LOCKED`, server-clock lease (120 s default),
  `fencing_token+1`, and reclaim of running rows whose lease expired;
  `Heartbeat/Complete/Fail/Release` require `(worker_id, fence, state=running,
  lease_until > now())`; `Cancel/Pause/Retry` bump the fence so an in-flight
  worker is invalidated immediately; `RequeueExpired` only normalises expired
  running rows (a live lease is untouched); `CompleteWith` runs a caller
  publish step in the job-completion transaction. Workers: register/authenticate
  by token hash, touch, revoke.
- `internal/persistence/postgres/audio.go` + `export_cache.go`: settings (per
  story), voices (project-scoped, stable id across updates), cast (role → voice),
  scripts bound to an accepted revision + sha256, segments keyed by script
  revision + stable id, renders keyed by the domain's `audio.SynthKey.Hash()`
  (text+voice+tone+speed+pitch+quality+engine+model), export cache by content
  key with expiry (no path/mtime lookups). Blob registry: `StageBlob` →
  `MarkBlobReady` (verified sha/size) → referenced by renders/export rows.
- `internal/entry/web/worker_api.go` (+ `worker_api_pg.go`): private listener
  protocol `claim → snapshot → upload → ACK` (plus heartbeat/fail/release/input).
  Bearer token resolved by hash, `tts` capability required; the upload key is
  derived server-side (`projects/<project>/jobs/<job>/<sha256>.<ext>`) and the
  streamed hash is verified, so a worker cannot choose a key or overwrite other
  content; the blob row is registered idempotently; completion verifies each
  blob's hash/state and attaches renders + job result in one transaction. A job
  id not held by the caller is 404 (never reveals a foreign job).
- `internal/ttsworker` + `cmd/ainovel-cli worker.go`: Mac-side pull worker
  (`ainovel-cli worker tts --api <tailnet-url>`), token only from
  `TTS_WORKER_TOKEN`, heartbeat at lease/4 (30 s for the 120 s lease), engine via
  `voice.NewEngine`, sequential synthesis, release on interrupt, stop on
  409/404 (never retry a fenced job).
- Docs: `docs/architecture.md` phase-07 block; `README.md` Mac-worker section
  (env-only token, offline queue behaviour, no VPS GPU).

Tests added (all Postgres, `REQUIRE_POSTGRES_TEST=1`):

- `jobs_test.go`: idempotent enqueue, two concurrent SKIP-LOCKED claims never
  overlap, lease expiry → reclaim with fence+1 and stale heartbeat/complete/
  fail/release rejected, duplicate ACK appends no second event, cancel-during-
  upload fences the worker, restart persistence (fresh pool sees the row;
  expired running → queued, never completed; live lease untouched), dense
  concurrent event sequence, worker credential/revocation, Gói AK round trip
  (kind create/update/redo, redo, updateEtaSec, position order), same-worker
  reclaim with a stale fence.
- `audio_test.go`: settings round trip + missing sentinel, voice/cast identity
  and FK-guarded delete, script/segment/render keys, one-selected invariant with
  the previous take kept as history, invalid synth key rejected, synth-key
  determinism (domain `audio.SynthKey`), export cache hits/misses/expiry.
- `worker_api_test.go` (fakes): 401/403 before any job lookup, 204 empty claim,
  stable JSON, 409 on stale fence for all publish paths, 404 unknown/foreign
  job, upload key derivation + digest mismatch leaves no object, unknown JSON
  field rejected.
- `worker_api_pg_test.go`: real end-to-end HTTP over PG + local BlobStore —
  claim → upload → complete attaches the selected render (blob hash verified) →
  stale worker after reclaim cannot upload or complete and does not change the
  job → new owner completes.
- `internal/ttsworker/worker_test.go`: claim/upload/ACK of two segments,
  409 fence stops the worker without publishing, cancellation releases the lease
  without completing/failing, engine failure reports the job failed, token/API
  URL required.

Fail-before evidence (each mutation reverted; migrations/queries regenerated):

- Complete ignoring the fence (`fencing_token = $4` → `$4::bigint IS NOT NULL`)
  → `TestJobsFenceSameWorkerReclaim`: `old attempt complete = <nil>, want
  ErrJobLeaseLost`. (The cross-worker case is also caught by `worker_id`, which
  is why the same-worker reclaim test exists.)
- `ClearSelectedAudioRenders` becoming a no-op (`SET selected = selected`) →
  `TestAudioScriptSegmentsRenders`: `select new render: ERROR: duplicate key
  value violates unique constraint "audio_one_selected" (SQLSTATE 23505)`.
- Claim no longer reclaiming expired leases → `TestJobsLeaseExpireReclaim`:
  `reclaim after expiry: không có việc nào đang chờ`.

Verification runs (this phase):

- `gofmt -l internal/persistence internal/entry/web internal/ttsworker
  cmd/ainovel-cli internal/audio` → no output; `go vet` on the same packages →
  clean.
- `go test ./internal/audio -count=1` → ok (28.5 s).
- `go test ./internal/persistence/postgres ./internal/persistence/migrate`
  (PG) → ok (24.7 s / 2.5 s).
- `go test ./internal/entry/web -count=1` (PG) → ok (151.4 s, full package).
- `go test ./internal/ttsworker -count=1` → ok (1.0 s).
- `go test ./... -count=1 -timeout 1800s` with `REQUIRE_POSTGRES_TEST=1` →
  see the run recorded in the handover report.

Remaining (precise):

1. `serve` does not yet start the private listener: no `:8081` bind on the
   Tailscale IP, no token-minting/rotation CLI, no gateway wiring of
   `WorkerAPI` (`PGWorkerQueue` + `PGAudioBlobs` + configured blob driver). 09
   owns the final topology, but the flag/env surface must exist before the Mac
   worker is usable end to end.
2. `audio.Service` still has only the file backend: hosted audio endpoints keep
   answering 501 (phase-06 handover item 6); the durable metadata exists but the
   service does not consume it yet. Gói AK semantics are preserved on the file
   backend and carried through job input on PG (tested), but no PG-backed
   `audio.Service` adapter is wired.
3. Real-provider acceptance (`TestRealVieNeuVolumeOneM4B`, `worker tts` against a
   tailnet URL with a queued job) was not run: it needs a Mac with VieNeu, a
   reachable `AINOVEL_AUDIO_SMOKE_ENDPOINT`, ffmpeg/ffprobe and a minted worker
   token. No NullSynth/mock result is claimed as that acceptance.
4. ffmpeg-side jobs (`audio_export`/`text_export` claim → merge/encode/cover/ZIP
   → export cache) are not implemented; only the queue table, cache table and
   cache repository exist.

## Context / requirements

Bắt đầu sau06 và03, không song song file web/audio ownership. Giả định VieNeu ở Mac, VPS không GPU. `internal/audio/service.go:229` loads file settings/queue; `internal/audio/queue.go:57-79` resets running→pending on restart; `internal/entry/web/audio.go:94,125-189` caches service process-wide by path; export result validation `internal/audio/export_result_cache.go:34-50` dùng size/mtime. `JobsProvider` hiện là interface chung (`internal/entry/web/jobs.go:113-129`); preserve job dock/events/actions, không thêm queue broker hoặc workflow engine mới.

Persist toàn bộ audio metadata: settings/pronunciation/cast/script/segments/candidates/render params/flags, queue/pending export, job history/progress, export-cache manifests và binaries qua BlobStore. TTS worker polling + lease/fence, multipart/big outputs; streaming/range downloads. Existing offline local synthesizer/queue giữ đúng contract. VieNeu live test là acceptance thật, không ghi đạt từ NullSynth/mock.

## Architecture / data flows

1. Browser enqueue với actor/quyền → transaction jobs.input chứa frozen source revision/hash + script/cast/settings/provider/model keys → event dock → queued. Không chỉ cất path tới source.
2. Mac `ainovel-cli worker tts` → Tailscale-only worker listener8081 + hashed random service credential → claim `kind=tts`, bounded batch=1 default, `FOR UPDATE SKIP LOCKED` → lease120s/fence++ → đọc snapshot được cấp cho đúng job → VieNeu segment synth + existing postprocessing → immutable object upload → ACK hash/length/params/fence → DB validates current source/cancel/permission → audio_renders metadata+job result/event atomic. Heartbeat30s dùng server time, no DB access on Mac.
3. VPS ffmpeg CPU handles merge/encode/cover/ZIP/text export jobs, uses private bounded scratch; downloads input blobs/upload outputs, transaction attaches only after verify. Mac TTS never hosts public inbound API, VPS không gọi home IP/port trực tiếp.
4. Crash/lease expired → job có thể render lại nhưng effect idempotent theo synth key/job fence; late ACK rejected. User edits chapter/script/cast mid-render → output cached as historical candidate but not selected current; job reports stale/cancelled, never silently plays wrong revision.

Public port8080 không register worker handlers. Private listener8081 container được publish **chỉ host Tailscale IP**/firewall; Dokploy Traefik không route listener hay `/internal/worker/*`. VPS host Tailscale ACL chỉ Mac worker được reach. Credential least-privilege `tts` capability, không user session/admin token. New worker rotates/revokes credential via admin CLI; revoke stops claim/heartbeat/publish.

## Files / ownership

`R=/Users/dat/Projects/ainovel-cli`.

- Modify production audio source files enumerated in evidence report: `R/internal/audio/{queue,service,cache,cast,chapter,cover,export,export_cache,export_files,export_pending,export_prefs,export_result_cache,export_selection,ffmpeg,flags,meta,paths,render,script,settings,voice_sample,voices}.go`; keep pure composer/matching behavior.
- Modify `R/internal/entry/web/{audio,audio_jobs,audio_resume,audio_voice_mobile,jobs,jobs_export,worker_jobs,worker_job_registry,worker_results,cover,project_assets,exports_download}.go`; `R/cmd/ainovel-cli/main.go` (command dispatch discovered cmd tree); `R/docs/architecture.md`, `R/README.md`.
- Create `R/internal/persistence/postgres/{jobs,audio,export_cache}.go`, `R/internal/persistence/migrations/00004_jobs_audio.sql`; `R/internal/audio/repository.go`; `R/internal/entry/web/worker_api.go`, `worker_api_test.go`; `R/internal/ttsworker/worker.go`, `worker_test.go`; `R/cmd/ainovel-cli/worker.go`; `R/internal/persistence/postgres/jobs_test.go`, `audio_test.go`.
- Existing tests retained: `R/internal/audio/{service,srt_queue,mobile_queue,staleness,export_result_cache,integration}_test.go`; `R/internal/entry/web/{audio_jobs,audio_followup,audio_partial,jobs}_test.go`. No Delete of file drivers/voice model implementation.

## Steps

1. Add fail-before queue/job/cache parity and crash/fence tests. Map DB states queued/running/paused/waiting/failed/completed/cancelled to current public waiting/running/done/error forms; ID/href/files/download actions retain JSON. Pending export selections/options remain durable, restart does not reset user choices.
2. Implement00004 workers/jobs/events/audio tables/export_cache DDL and queries from schema; scripts bind chapter accepted revision/hash, segments keyed script revision + stable ID, renders key includes voice/tone/speed/pitch/quality/engine/model. Store candidates/selected flags and params, one selected render per segment. Settings/flags/voices samples/pronunciation scoped project/story, not mutable global state.
3. Adapt `audio.Service` to injected repository/blob driver; file implementation retains paths/atomic persistence. PG construction no ensureAudioDirs/readQueue JSON; shared service caches only stable identity/config clients, not user actor/session/context. Remove path Stat requirement on hosted `audioService`; per-request authorization outside cache.
4. Durable claims/heartbeat/ACK/cancel/pause/retry: use server clock, row locks, fencing_token, idempotency key. Every publish validates lease and current job input/source revisions; duplicate ACK returns same result, no duplicate blob/event. Enqueue/retry eligibility uses actor current permission; user disabled/access revoked cancels publication according to job requested_by. worker credential cannot choose different project/blob or arbitrary synthesis request without a lease.
5. Implement private pull endpoints `POST /internal/worker/claim`, `POST /internal/worker/jobs/<id>/heartbeat`, `/complete`, `/fail`, job-scoped source/download/upload capability. Worker upload POST streaming via server BlobStore; avoid handing bucket-wide credentials to Mac. Upload capability bound job/fence/expected key/type/size/hash, cannot attach output after cancel. Multipart internal SDK backend handles large object. Bounded transient network backoff for worker transport preserves cancellation; no unbounded job retry policy added.
6. Implement `worker tts --api <tailnet-url>` reads `TTS_WORKER_TOKEN` secret from env, no tokens in argv/log. Reuse `internal/voice` provider/synthesis contract, current speed/pitch/normalization per segment; cap parallel synth to Mac capacity. Stop graceful lease release; abrupt stop reclaimed by timeout. No inbound Mac listener or Postgres port exposure.
7. Convert all persistent audio binary callers from source inventory to BlobStore. ffmpeg gets leased temp paths only; input spool bounded, disk quota checked and cleanup on cancellation. Export `cache_key` uses exact source/script/cast/settings/options/music/voice/cover hashes; tag-only metadata changes can reuse audio but new output manifest/hash, never mtime. Metadata invalidation removes references not immediately deletes shared objects.
8. Adapt JobsProvider/read lists/replay/progress/SSE to DB jobs/events; retain existing control operations/route permissions. Restart web service rehydrates durable queue once, no duplicate local queue auto-run plus remote claim loop. Local/file mode still works offline without Mac network.
9. Document Mac setup/Tailscale ACL/service-token rotate, provider env, offline queue behavior and no-GPU VPS. 09 owns final topology/deployment; do not rely on current desktop TTS install/power buttons to start model on VPS (hosted capability reports remote worker status instead).

## Fail-before test matrix

| Layer | Cases | Gate |
|---|---|---|
| Unit/contract | queue controls/restart, script/cast/settings/hash variants, candidates/selected, same-size content changed | No path/mtime-dependent false cache hit |
| DB integration | two workers claim, lease expire/reclaim, stale heartbeat/ACK, duplicate ACK, cancel during upload | Single published effect and correct fence; orphan unreferenced, not selected |
| HTTP security | invalid/revoked service token, wrong capability, cross-job source/upload, public mux worker path | 401/403/404; no TTS control or arbitrary object overwrite |
| Integration | kill/restart web+Mac, Mac offline, DB outage, storage upload failure | Jobs survive, no false completed, recover/retry visible |
| Media | ffmpeg real MP3/M4B/SRT/ZIP/cover/metadata, stream range fallback, export cache restore | ffprobe/byte hashes verify actual outputs |
| E2E real provider | enqueue chapter→Mac VieNeu→play→export→restart→download | Real synthesized bytes/hash, no NullSynth substitution |

## Verification commands

```sh
go test ./internal/audio -count=1
go test ./internal/persistence/postgres -run 'TestJobs|TestAudio|TestExportCache|TestLeaseFence' -count=1
go test ./internal/entry/web -run 'TestWorkerAPI|TestAudio|TestJobs|TestExportDownload' -count=1
go test ./internal/ttsworker -run 'TestWorkerClaim|TestWorkerFence|TestWorkerCancellation' -count=1
go test ./internal/audio -run TestRealVieNeuVolumeOneM4B -count=1
go run ./cmd/ainovel-cli worker tts --api "$TTS_WORKER_API_URL"
```

DB/MinIO env explicit, `REQUIRE_POSTGRES_TEST=1`; VieNeu command needs `AINOVEL_AUDIO_SMOKE_ENDPOINT` pointing a real authorized endpoint and installed ffmpeg/ffprobe (`internal/audio/integration_test.go:55-66`). Worker command run on Mac with secret env/Tailscale and a queued test job, terminate after acceptance scenario; it is a long-lived service, not unit-test success by merely starting. Additional new remote E2E in09 uses real web DB job API. None executed during planning.

## Risks / rollback / done

- **High (L cao × I cao):** home Mac unavailable. Queue waits visibly, VPS stays healthy/readable, lease expiration recovers; worker offline does not auto-install GPU engine VPS.
- **High (L vừa × I cao):** split-brain/stale output. Fence/source-revision/permission checks at ACK, immutable keys, cancel semantics and duplicate ACK tests.
- **High (L vừa × I cao):** broad worker access/private content. Tailnet+token/capability/job-bound data, private listener excluded public mux, token rotation/revoke.
- **High (L vừa × I cao):** RAM/disk/ffmpeg exhaustion. Bounded segment uploads, scratch quotas/concurrency, streaming/cleanup and measured large-export test.

Rollback: pause enqueue/claims, expire/revoke leases, stop Mac worker; keep queue/DB/blob objects. Rollback app DB-compatible; if file mode needed08 exports queue/audio manifests/latest binaries first, in-flight jobs normalize to queued not “done”. Delete no buckets. Done: real job survives restart and offline worker, stale worker cannot publish, actual VieNeu/ffmpeg outputs playable, same metadata APIs/file mode preserved, export cache hash semantics proven.08 can import full existing audio state only after this.
