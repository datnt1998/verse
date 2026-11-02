# Voice fix review — PG Compose/Refresh + GenPickEmpty preparation

Read-only source review. No code changes, no browser, no provider calls, no tests run
(full `./apps/noverse/...` PASS is the primary's, not re-run here). All claims below are
either **PROVEN** (code path traced end to end) or **INFERENCE** (stated as such).

**Integration resolution:** the findings below describe the pre-fix snapshot. Primary repaired all four: batch validation precedes claim acquisition; stale queued/paused/running render jobs are superseded atomically on the same scoped/fenced row; preparation is excluded from native queue counts and Queue/Jobs waiting-machine labels; preparation and direct compose lock/check accepted source revision+digest before publication. A per-chapter transaction advisory lock also closes the first-enqueue zero-row race. The full Noverse suite passed, then the new enqueue race test and all affected PostgreSQL/audio/web/native-worker packages passed on the final source with real isolated PostgreSQL. Production voice/playback proof remains a separate release gate, not a conclusion of this source review.

## Verdict

The core contract holds. I found **one high-impact defect** (in-memory compose claim leak)
and three smaller, code-proven holes. None of them corrupts data or breaks the fence; the
high one needs a server restart to clear.

## F1 — HIGH, PROVEN: a failed multi-chapter Compose permanently sticks the chapters before the bad one

`internal/audio/pg_compose.go:57-87`

```go
for _, ch := range chapters {
    if _, ok := byChapter[ch]; !ok {
        return conflict("Không có chương %d trong tập này.", ch)   // :70-72
    }
    if s.claimComposing(volume, ch) {                            // :73
        list = append(list, ch)                                  // :74
    }
}
...
go func() {
    defer s.compWG.Done()
    defer s.releaseComposing(volume, list)                       // :83  (only the success path)
    ...
```

`claimComposing` writes `s.composing[key]` (`pg_compose.go:31-42`). `releaseComposing` runs
**only** in the background goroutine (`:83`). The `return` at `:71` happens after earlier
chapters were already claimed, so those entries are never released. The file backend does
not have this bug: it only *checks* `isComposing` (`compose_service.go:31`) and sets the
flag per chapter inside the worker (`:32`, `:146-149` of `compose_service.go`), so the PG
claim introduced here is the one that leaks.

Impact chain (all proven):

- `Refresh` (`pg_compose.go:92-122`) and any later `Compose` for that chapter return
  `409 "Chương này đang được soạn rồi."` forever (`:76-78`, `:114-116`).
- `Volumes` reports the chapter as `status: "draft"`:
  `pg_views.go:559` → `DeriveStatus(..., s.isComposing(volume, sc.Chapter), hash)` →
  `script.go:249-254`.
- Frontend consumes that as a live compose: `web/src/screens/audio/genWork.ts:53`
  (`chapter.status === 'draft'` → `composing`), `:103` (`"Đang soạn bản đọc"`), `:163`
  (counted `busy`), `:275`; `Transcript.tsx:124,244,335` makes the script non-editable;
  `mobile/VoiceGenSheet.tsx:200` renders a permanent ring and `:218` dims the row.
- Nothing else clears `s.composing` — `releaseComposing` has no other caller, and the map
  lives in the gateway-cached `*PGService` (`entry/web/audio.go:142-166`), which is only
  dropped by `closePGAudioServices` on gateway shutdown.

Reachability: `POST /api/projects/:p/stories/:s/audio/volumes/:v/scripts` passes the body's
chapter list through `sortedChaptersInput` (`entry/web/audio.go:1420-1431`, positive+dedupe
only), and `audioVolumeSource` membership is only checked inside `Compose`. So
`{"chapters":[1, 999]}` (or a stale tab that still lists a chapter that moved to another
volume) claims chapter 1, then 409s on 999 and leaks chapter 1. The normal UI paths send
chapters of the current volume (`Transcript.tsx:206-208`, `genWork.ts:198-209`), so this is
not reachable by ordinary clicks today — but it is reachable by any client, and the state
is unrecoverable without restarting Noverse. **INFERENCE:** that restart is what would
have been needed if this is ever hit in production; I did not reproduce it at runtime.

Fix (not applied): validate every chapter before the first `claimComposing`, or
`defer s.releaseComposing(volume, list)` on the error path.

## F2 — MEDIUM, PROVEN: a queued *render* row is reused instead of enqueuing preparation, so "generate" on a now-stale chapter is swallowed

`internal/audio/pg_jobs.go:281-296` (`chapterQueueSlot`) returns any `queued|rendering|paused`
job of the chapter, and `enqueueTTSPrep` (`:341-367`) returns that id without looking at
`NeedsScript`:

```go
reuse, attempts, err := s.chapterQueueSlot(ctx, volume, chapter.Number)
if reuse != "" { return reuse, nil }          // :342-345
```

`QueueChapters`' stale branch (`:556-575`) is the fix's auto-compose entry, and it goes
through `enqueueTTSPrep`. If the chapter already has a queued render row created *before*
the chapter text changed, no preparation row is created and the chapter is never re-composed.

The reused row is not dangerous (the ownership/source fence refuses the result), but the
outcome is a silent no-op: `playbackRevision` compares the script's `source_revision` with
the chapter's `current_revision` (`persistence/postgres/audio_playback.go:39-48`), and the
ACK path returns early when they differ — `entry/web/worker_api_pg.go:196-200`
(`if !current { return nil }`) — so the worker can synthesize and upload a full chapter and
every attachment is dropped, while the job still commits as completed.
The worker reports `result {segments: n, kind}` and `progress_done = n` (`ttsworker/worker.go:416-432`),
so the UI will show a finished job with no audio.

**INFERENCE:** the UI path that can hit it is the desktop Transcript render button, which
routes a stale chapter into `enqueueAudioChapters(..., lock:true)` (`Transcript.tsx:195-201`);
the generation sheet cannot, because the chapter's live job makes the row `busy`
(`genWork.ts:20-23, 163`). One wasted click, self-heals on the next attempt because the
completed row is no longer "active".

## F3 — LOW/UX, PROVEN: preparation rows are counted and labelled as work waiting for the reader machine

- `CountQueuedJobs` only filters `kind='tts' AND state='queued'`
  (`persistence/postgres/queries/jobs.sql:436-441`), so `needsScript` rows count as
  "chapters the user loses while the machine is offline" — `entry/web/tts_worker.go:313-320`
  (`queuedVoiceChapters`, used for the reader card warning and `queueWaiting`).
- `entry/web/audio.go:471-487` marks every `JobQueued` timeline job `WaitingMachine` when
  the reader is not ready, including a `needsScript` row that the web process owns.
- `genWork.ts:41-47` then renders `hold` → `Đang chờ máy đọc` instead of the job's
  `step:"compose"` (`pg_jobs.go:193-200`). Cosmetic only; the row does become machine work
  after preparation.

## F4 — LOW, PROVEN (consistency): prep writes `source_revision` read *before* the model call; Compose re-reads it after

- `PrepareClaimed` reads `ref` once at `pg_prepare.go:96` and writes it through
  `scriptBundle(ref, script)` (`pg_views.go:200-228`); the `ref.ID` is also used for the
  publish revision (`pg_prepare.go:196-200`). A chapter revision that lands during the
  model call is therefore not reflected in the script row.
- `composeChapters` deliberately re-reads `chapterRef` *after* the call and before
  `persistScript` (`pg_compose.go:222-225`), with a comment saying so.

With the prep path the ACK's `Current` (`audio_playback.go:322-325`) then fails on
`source_revision != current_revision` and the queued render's attachments are dropped
(same silent-completion shape as F2). Safe, but the two paths disagree; re-reading `ref`
before the publish would make them consistent.

## Verified OK (against the ticket's claims)

- **Native cannot claim before preparation:** `ClaimJob` excludes
  `(input->>'needsScript') IS DISTINCT FROM 'true'` (`jobs.sql:106-133`, filter `:116`);
  string/malformed values stay claimable (`preparation_test.go:53-100`). `PrepareClaimed`
  also re-validates `frozen.NeedsScript` (`pg_prepare.go:77-81`).
- **Jobs stay queued during prep:** `ClaimPreparation` sets `state='queued'`,
  `worker_id=NULL`, a lease and `fence+1` for the exact `(project, story)` and
  `needsScript` row, `FOR UPDATE SKIP LOCKED` (`jobs.sql:218-244`).
- **Exact story/job/fence/lease guard:** `LockPreparation` (`:246-257`), `CompletePreparation`
  (`:296-317`), `CompletePreparationEmpty` (`:319-342`), `FailPreparation` (`:259-277`) and
  `ReleasePreparation` (`:279-294`) all match `project_id+story_id+id+fencing_token`, require
  `worker_id IS NULL`, `state IN ('queued','running')`, a live `lease_until` and
  `needsScript`; zero rows → `ErrJobLeaseLost` (`jobs.go:507-523`, `597-676`).
- **Callbacks atomic:** `CompletePreparationWith` locks the job row first, runs
  `persistScriptTx` + `PlaybackRevisionTx` + the input swap in the *same* tx
  (`jobs.go:615-676`, `pg_prepare.go:190-226`), so a CAS conflict or a moved fence rolls back
  both the script and the job. The worker ACK is likewise one tx with attachments +
  `EnqueuePlaybackTx` (`worker_api_pg.go:170-250`).
- **Same row, same kind, `needsScript` stripped:** `jobs.sql:296-317`; zero-work refresh
  completes the row instead of queueing an empty render (`:319-342`,
  `preparation_test.go:452-505`).
- **Retry re-enters preparation:** `FailPreparation` keeps `needsScript` (`jobs.sql:259-277`)
  and `RetryJob` preserves the input, clears error/result and bumps the fence
  (`:378-397`); `Retry`/`Resume` wake the runner (`pg_jobs.go:646-651`, `681-686`).
- **Runner is actually wired in production:** `cmd/ainovel-cli/serve.go:275` starts it after
  `SetAudioStore` (`:164`) and before `Serve`; `Gateway.Close` stops it before the audio
  services/hubs close (`gateway.go:649-663`). `prepWake` is a size-1 buffered channel
  (`gateway.go:319`) and the hook is installed on the same gateway-cached service whose hub
  serves SSE (`audio.go:159-165`, `:264`, `:1590`).
- **Compose/Refresh are real:** both validate, claim, run on a background goroutine and
  publish the same `audio.compose` frames as the file backend; Refresh keeps lock/`LockedAt`,
  merges by `Source`, detaches only changed segment ids (`pg_compose.go:89-281`).

## Caveats

- `pgComposeChapterTimeout` (2m30) is the ctx also used for `persistScript` + `Enqueue`
  after the provider call (`pg_compose.go:200-240`); the `Enqueue` error is discarded
  (`:238`). **INFERENCE:** only a very slow DB (or a provider call near the 2m ChatComposer
  cap plus slow persistence) leaves a refreshed locked chapter composed but unqueued; the
  file backend likewise ignores that error, so this is not new.
- Multi-process deployments can run one preparation loop per story per web process (claims
  coordinate through the lease, so no double claim, but two processes may compose two
  different chapters of one story in parallel, unlike the per-process sequential comment in
  `audio_prepare.go:27-29`). Deployment topology was not inspected.
