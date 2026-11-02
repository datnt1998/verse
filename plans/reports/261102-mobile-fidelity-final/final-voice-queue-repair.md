# Voice queue repair — stale active job → fresh preparation + waitingMachine

Source complete; **no build/lint/test/formatter was run** (freeze: primary runs the
narrow then full Go before the Noverse deploy). The only generated code touched was
regenerated with `go tool sqlc generate` from the committed queries (codegen, not a
build), and against `HEAD` it changes only `gen/jobs.sql.go`.

## Verdict

F2 (stale active render row silently swallowed a Generate) and F3 (preparation rows
counted/labelled as native-machine work) are fixed. The queue now reuses a row only
when its frozen input is the work the request is asking for, and a stale active row is
rewritten in place, in one transaction, under a bumped fence, with any other active row
of that chapter cancelled. Native claim / preparation protocol and the worker payload
schema are untouched.

## 1. Freshness-aware queue slot (`internal/audio/pg_jobs.go`)

Replaced `chapterAttempts` + the "any active job" `chapterQueueSlot` with:

- `pgQueueIntent` — the exact work a call wants: chapter id/number/volume, kind,
  quality, redo, needsScript, source revision, script revision, ownership fingerprint,
  segment set.
- `pgQueueIntentMatches` — a row is reusable only when every field that changes what
  the worker would synthesize still matches:
  - chapterId + chapter + volume always;
  - kind, quality, redo, needsScript always;
  - preparation rows: `frozen.sourceRevision >= want.sourceRevision` — a prep for a
    later accepted revision already covers the chapter (the runner resolves the
    current source at claim), so a request that raced `AcceptRecord` never downgrades
    it to older prose;
  - render rows: source revision equal, script revision equal, ownership fingerprint
    equal, and the frozen segment set byte-for-field equal (id/text/speaker/voice/tone/
    quality/speed/pitch/engine/model/synthKey/select). A changed cast, voice, settings
    or script therefore can never be masked by a stale queued row.
- `chapterQueueSlot` — lists this story's durable tts rows, counts attempts for the
  idempotency key, and:
  - if the **newest active row matches** the intent → reuse it (resume if paused), and
    fence out older duplicate active rows (legacy/race only);
  - otherwise → return the newest active row for an in-place supersede.
- `enqueueTTSRow` centralizes the decision: reuse → return the same id (no new row, no
  wake); otherwise compute `tts-v<vol>-c<ch>-a<attempt>` and call the repository's
  one-transaction enqueue.

`enqueueTTS` now reads `PlaybackRevision` before the slot (ownership is part of the
match), and `enqueueTTSPrep` states the same intent with `needsScript:true` and no
segments. Both keep kind/redo/quality/source revision and (for renders) ownership in
the frozen input, so user intent survives a supersede.

`Enqueue` (chapter render action) now routes a script whose source hash no longer
matches the accepted chapter through `enqueueTTSPrep`. Before, that action queued a
render of the stale text, whose ACK was refused — the same silent-completion hole.
Callers that pass a current hash (`QueueChapters` non-stale, post-compose, post-lock,
role sweep) keep the render path.

## 2. Atomic supersede (`internal/persistence/postgres`)

New queries (`queries/jobs.sql`, regenerated `gen/jobs.sql.go`):

- `LockChapterJobs` — every tts row of one `(project, story, chapterId)`, `FOR UPDATE`;
  chapterId is the frozen input field, not the chapter number.
- `SupersedeChapterJob` — replaces `input`, `progress_total`, resets
  progress/result/error/timestamps, sets `queued`, clears `worker_id`/`lease_until`,
  bumps `fencing_token`, only while the row is `queued|running|paused`.

New repository method `Jobs.EnqueueChapter` (one transaction):

0. takes a transaction-scoped advisory lock keyed by
   `noverse:tts-enqueue:<project>:<story>:<chapter>` **before reading any job row**:
   two first enqueues otherwise both observe zero rows, so the loser's `InsertJob`
   resolves to the winner's frozen input through the idempotency conflict (same key) or
   inserts a second active row (different keys). The key is a chapter *hash*, not a
   chapter-row lock — preparation completion locks job → chapter, so taking a chapter
   row here would invert that order and deadlock. Every enqueue for the chapter
   serializes; the later transaction's statements see the earlier row;
1. if the caller returned a stale row, supersede it in place (same id/idempotency key);
2. lock the chapter's rows; keep a concurrently inserted row with identical frozen
   input (same request), otherwise supersede the newest active row;
3. if nothing was active, insert under the caller's idempotency key (the existing
   `ON CONFLICT` path resolves a racing identical insert to the same row; if the
   colliding row holds a *different* intent it is superseded when active, and a
   terminal collision returns a visible `ErrJobState` instead of another request's
   frozen input);
4. cancel any remaining active rows of the chapter (`CancelJob`) so one chapter never
   has two active jobs.

Effects, all in the same commit:

- a running render's `worker_id` is cleared and its fence moves → its ACK/heartbeat
  resolves to no row/`ErrJobLeaseLost`, so no stale audio can attach;
- a composing preparation's lease/fence move → `CompletePreparation*` returns
  `ErrJobLeaseLost`, so no stale script is saved;
- a paused row is converted and re-queued by the explicit new request (the previous
  paused row is never resumed into stale work);
- every query is scoped by `project_id + story_id`; supersede is additionally scoped by
  the row id from this story's own list, so another story/chapter is untouched.

The `needsScript` prep row, its claim/lease/publish SQL, the `pgTTSInput` JSON shape and
the worker ACK separation are unchanged.

## 3. waitingMachine / queueWaiting (`internal/entry/web/audio.go`, `CountQueuedJobs`)

- `CountQueuedJobs` now counts only `kind=$1 AND state='queued' AND
  (input->>'needsScript') IS DISTINCT FROM 'true'`. A preparation row is the web
  process composing a bản đọc, not a chapter lost while the reader machine is offline.
  Mirrors the `ClaimJob`/`ListClaimableJobs` predicate, including malformed flags.
- The Queue GET (`/audio/queue`) only sets `WaitingMachine` on queued jobs that are
  **not** composing (`needsScript`/`step=compose` and queued/rendering/paused), and it
  skips the chapter flag for chapters whose live job is that preparation. `NeedsScript`
  and `Step` already on the view are the single source of truth; no new status is
  invented and `Job.State` values are unchanged.

## 4. Regression tests (new file, real Postgres + real runner)

`internal/entry/web/pg_audio_stale_queue_test.go`

`TestPGHostedStaleActiveJobGetsFreshPreparation` — subtests `queued`, `paused`,
`running`:

1. real compose → one queued render row for accepted revision 1;
2. old row left queued / paused through the repo / claimed running by a real worker;
3. `AcceptRecord` lands revision 2;
4. the next Generate must rewrite that **same** row into `needsScript` with
   `sourceRevision=2`, `worker_id` cleared and fence past the old one — exactly one
   active row; the running worker's `Complete` is `ErrJobLeaseLost`;
5. an identical Generate while composition is in flight reuses the row and starts no
   second provider call (call counter = 2);
6. after the runner publishes, the row is the queued render for revision 2 with the
   new segment text; `PlaybackProof.Current` is **true** (the ACK would attach the
   audio, not drop it);
7. an identical Generate after preparation reuses the fresh render row (same id, one
   row);
8. in the `queued` subtest, a second story in the same project DB has its own queued
   row; after all of the above it is still queued, fence 0, same idempotency key
   (scoped fence / no cross-story effect).

`TestPGHostedPreparationIsNotWaitingMachine` — a queued `needsScript` row (no runner)
gives `queueWaiting=0`, `step=compose`, and `waitingMachine` unset on both the job and
its chapter; a real queued render row in the same volume raises it to 1 and is labelled
`waitingMachine`.

The tests drive the HTTP queue handler, the real `PGService`, the real preparation
runner, the real repositories and the real worker claim — no mock of the code under
test. The provider is a local HTTP stub, as in the existing preparation tests.

`internal/persistence/postgres/jobs_enqueue_chapter_test.go` —
`TestJobsEnqueueChapterSerializesPerChapter` (`same key` and `different keys`):

1. with **no row yet**, launch 8 concurrent `EnqueueChapter` calls for the same
   chapter with alternating intents (quality/source revision/segment text) — all
   sharing one idempotency key in the first subtest, eight distinct keys in the
   second;
2. every call must return the one row id, exactly one tts row exists, and the final
   frozen input must be the *last serialized request's* complete intent (the
   max-`version` result's input), never a mix;
3. a native worker claims that row, then a second 8-way concurrent burst runs; it
   must supersede the same row, leave exactly one queued row, move the fence past the
   claim, and the pre-burst claimant's `Complete` must be `ErrJobLeaseLost`.

On the pre-fix code the same-key losers are handed the winner's frozen input through
`GetJobByIdempotency`, and the different-key callers insert parallel active rows; both
assertions catch that.

## Commands for the primary (freeze release)

```
cd apps/noverse
go test ./internal/persistence/postgres -run 'TestJobsEnqueueChapterSerializesPerChapter' -count=1
go test ./internal/entry/web -run 'TestPGHostedStaleActiveJobGetsFreshPreparation|TestPGHostedPreparationIsNotWaitingMachine' -count=1
go test ./internal/audio -count=1
# then the full frozen suite
go test ./...
```

The PG tests need the existing full test environment (`TEST_DATABASE_URL`/pgtest) and an
empty DB; `TestPGHostedPreparationIsNotWaitingMachine` depends on `CountQueued` being
global to the isolated test schema, which pgtest guarantees.

## Residual concerns (explicitly out of this ticket's file scope)

- Primary also changed `entry/web/audio_jobs.go` / `audioHasQueued`: a preparation (`NeedsScript` or `step=compose`) no longer gives the Jobs-centre volume row a native waiting-machine label.
- `PrepareClaimed` snapshots `ref` before the provider call; the primary's new
  `LockChapterSourceTx`/`preparationSourceMatches` fence now rejects a publish when the
  revision pointer or canonical prose digest moved, so that path fails visibly and
  retries instead of dropping audio (covers review item F4).
- Two concurrent Generate requests with *different* intents for the same chapter
  serialize on the per-chapter advisory lock held by `EnqueueChapter`; the later
  request's frozen intent wins, the earlier is visibly superseded (single active row,
  fence moved, old claim rejected). The audio layer's pre-lock read is only a reuse
  hint; the repository re-reads and reconciles inside the lock.

## Parent verification

- New concurrent first-enqueue regression passed with real PostgreSQL (`same key` and `different keys`).
- Full `go test -p 2 ./apps/noverse/... -count=1` passed before the last race hardening; after that hardening, all affected `internal/persistence/postgres`, `internal/audio`, `internal/entry/web` and `internal/ttsworker` packages passed again.
- `REQUIRE_POSTGRES_TEST=1`, empty isolated acceptance database; no provider/commercial calls. Actual normal-path production speech/playback is still a separate authorized release smoke.
