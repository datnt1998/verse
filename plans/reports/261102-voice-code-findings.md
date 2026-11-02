# Voice code findings — hosted compose 501 (2026-11-02)

Status: **compose and durable queue preparation implemented; full Noverse Go suite passed with PostgreSQL.** Production deployment and the explicitly authorized short synthetic voice/playback proof remain pending.

## User-visible failure

Exactly: `Tính năng này không khả dụng trên kho dữ liệu hiện tại.`

- Source: `internal/entry/web/httperr.go:77` maps `errs.ErrStorageUnsupported` to HTTP 501 with that generic sentence, discarding the feature label.
- The failure happens **before** any job row: no TTS/audio job was created, so the worker was never involved.

## Observed live evidence (read-only, sanitized)

- `jobs` on the production Postgres: exactly **2** `kind='tts'` rows ever, both `done` at 2026-11-01 21:44Z and 23:26Z (the disposable QA replay). **Zero** failed jobs and **zero** new TTS rows after 23:26Z. A normal generation attempt therefore never reached `Enqueue`.
- `audio_scripts`: exactly **1** row in the whole database (the QA fixture, story `cc23eeb3…`, 1 segment). Real stories have no bản đọc at all. `audio_settings`, `audio_voices`: 0 rows.
- `workers`: subscription alive, `state.state = ready`, `last_seen_at` fresh (~1 s). The worker is healthy — not the cause.
- Production container global config (keys/types only, no secrets): providers `openai-codex` and `xai-oauth`; default `openai-codex/gpt-6-luna`; roles use `gpt-6.1-sol` / `xai-oauth`. **No local/offline provider** (no ollama) is configured.
- HTTP access logs do not exist and mapped 501s are not logged; DB + source trace is the evidence.

## Root cause (source line)

`POST /audio/volumes/:v/scripts` (Transcript "Soạn bản đọc") built the composer only for the file backend:

- `internal/entry/web/audio.go` (old): `var composer audio.Composer; if st.Dir != "" { composer, _ = audioComposer(st) }`
- Hosted stories have `st.Dir == ""`, so composer stayed `nil`.
- `internal/audio/pg_views.go` (old `:491`) `(*PGService).Compose` unconditionally returned `pgUnsupported("soạn bản đọc bằng mô hình")`; old `:496` `Refresh` did the same.
- `pgUnsupported` wraps `errs.ErrStorageUnsupported` → 501 generic message.

The same applies to `POST /audio/volumes/:v/scripts/:ch/refresh` (stale-script recompose). The prior production QA proof preseeded a **locked** script, which bypassed the normal compose step, so it could not catch this gap.

## Fix (real implementation, no fallback)

- `internal/audio/pg_compose.go` (new): real hosted `Compose` / `Refresh`, mirroring `compose_service.go`:
  - synchronous validation exactly like the file backend (nil composer → `ErrNoModel`; unknown chapter; already composing);
  - background model call, sequential, `audio.compose` frames `drafting|done|error` with the same payload shape;
  - `EnsureCast` + `composeSpeakers` + settings/normalization/pronunciations from the story document;
  - `mergeScript` for refresh, locked status/lockedAt preserved, changed segments' selected takes cleared (`changedComposeSegments` → `persistScript(edited)`);
  - CAS persistence against the script revision, then auto-enqueue for a refreshed locked script.
- `internal/audio/pg.go`: per-chapter in-flight set + waitgroup; `Close` waits up to 5 s for compose (same policy as the file backend); capability comment updated.
- `internal/audio/pg_views.go`: removed both 501 stubs; `Volumes` now reports composing status during compose.
- `internal/entry/web/audio.go`:
  - new `(*Gateway).audioComposerForStory` — file story: existing `audioComposer(st)`; hosted: `hostedConfigDocuments()` + `bootstrap.LoadHostedConfigChain(global → project → story, actor)` + `NewModelSet` → `audio.ChatComposer`, preserving the global model/provider scope (`HostedLayerConfig` still strips provider pins from lower layers). Missing model → existing 409 `needsConfig`, never a 501 or fake narration.
  - `serveAudioVolume` / `serveAudioScriptAction` now receive the authenticated `user` and use the resolved composer for both compose and refresh.
- `internal/entry/web/pg_audio_compose_test.go`: seeds only narrative chapter + progress (no script/segment), points the real ChatComposer/provider stack at a local compatible endpoint, then exercises compose → persisted script → lock → `/audio/volumes/1/chapters/1/render` → a durable `tts` job. No external/commercial call.

## Public contract / callers affected

- `POST /audio/volumes/:v/scripts` and `POST /audio/volumes/:v/scripts/:ch/refresh`: success 200 (was 501); request/response bodies unchanged; SSE `audio.compose` unchanged.
- If no model is configured: 409 `{needsConfig:true}` instead of 501 (existing contract from `11-api-contract.md`).
- No migrations, no worker protocol or DB-role changes, no frontend changes. Only internal handler signatures (`serveAudioVolume`, `serveAudioScriptAction`) gained `user`; single caller chain.

## Verification for parent

```
REQUIRE_POSTGRES_TEST=1 go test ./apps/noverse/internal/entry/web -run 'TestPGHostedComposeMakesGenerationReachable|TestPGComposeClaimsChapterExactlyOnce|TestPGComposeDoesNotOverwriteUserEdit|TestPGRefreshRecomposeKeepsLockedScript|TestPGAudioQueuePlaybackHTTP' -count=1
go test ./apps/noverse/internal/audio ./apps/noverse/internal/entry/web -count=1
go build ./apps/noverse/...
```

(No tests/build/formatters were run by this worker, per task policy.)

## Review fixes (post-review pass)

- **Compose slot is claimed atomically** (`claimComposing`, `releaseComposing`): check+set happen in one critical section *before* the goroutine, so two concurrent POSTs cannot both launch a provider call. A duplicate request now gets the existing "Các chương này đang được soạn rồi." conflict.
- **No lost-update window**: the script row + expected revision are snapshotted **before** the provider call; persistence uses that CAS version. A user edit during the model call now fails CAS with the user-visible conflict "Bản đọc vừa được sửa trong lúc soạn — dữ liệu mới đã được giữ, vui lòng thử lại." (published on `audio.compose`, existing error frame).
- **Storage errors propagate**: settings/cast reads in the compose goroutine no longer fall back silently to defaults; a failure publishes the error frame for every chapter in the batch. (The file backend's fallbacks remain untouched; this is the hosted storage-correct behavior.)
- Regressions added (not run): `pg_audio_compose_gate_test.go` — exactly-one model call under concurrent duplicate Compose, user edit during compose preserved + CAS error, and stale Refresh preserving lock/source hash.

## Integrated hosted queue preparation

The early rejected `tts_prepare` attempt was removed. The finished implementation uses the existing `tts` kind with `input.needsScript=true`; detailed ownership and invariants are in [261102-voice-queue-service.md](261102-voice-queue-service.md).

- Preparation stays `queued` with `worker_id NULL` while its exact scoped/fenced lease is held. Native TTS claim excludes `needsScript=true`; no fake worker or new wire protocol.
- The web runner reads the accepted source, snapshots script CAS before the model call, composes and locks, then publishes the script plus prepared render input atomically on the same durable job.
- Pause/cancel wins against publication, failed compose remains retryable, and startup recovers queued work or waits for an existing lease to expire.
- Hosted source loading also accepts the authoritative accepted chapter record when a separate prose artifact has not been published. Script Chapter/Revision metadata is rehydrated from typed row fields on read.
- Empty prepared output (all segments already voiced) completes the same job without creating a zero-segment worker claim.

## Parent verification / consent

- Full `go test -p 2 ./apps/noverse/... -count=1` passed with `REQUIRE_POSTGRES_TEST=1` on a new empty local database; schema isolation was not weakened.
- Hosted compose regression, preparation happy path, cross-story isolation, pause/cancel, retry, restart and existing lease recovery passed. The original hosted-compose test URL was corrected to the real volume-scoped render route; no compatibility endpoint was added.
- The user explicitly authorized one short owned synthetic voice test through the normal compose→native speech→playback path. No provider configuration change is needed or authorized.
- Created only `qa-voice-e2e-f5777b46`, its owned QA story and one synthetic accepted chapter. The guarded helper verified `audioScripts=0`, `providerCalls=0`; it did not preseed a transcript or job.
- The original user story stays paused. No Resume, review replay, checkpoint backfill or content rewrite is part of this voice proof.
- Deployment and actual audio playback evidence must be recorded after rollout, not inferred from a healthy worker or the isolated provider endpoint.
