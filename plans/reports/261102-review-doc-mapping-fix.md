# Review document mapping fix — chapter 5 polish blocker

## Status

Code + regression test + architecture note complete. No deploy, restart, job
cancel/retry, or provider call performed. Parent owns integration run, deploy,
and any user-state decision.

## Root cause (proven)

`save_review` writes in this order:

1. `Progress.ApplyReviewOutcome` (flow + pending rewrites),
2. `World.SaveReview` (typed `reviews` round),
3. `Checkpoints.AppendArtifact(scope, "review", "reviews/NN[-global].json")`.

On Postgres, step 3 reads the artifact through the document surface to compute a
SHA-256. `resolveDoc` had no review mapping, so the read failed with
`postgres document backend has no mapping for "reviews/05-global.json":
unmapped document path`. Steps 1–2 had already committed: the first failure
created review round 13, but the checkpoint (and the tool's success response)
never happened. Later retries then saw “review exists with different content”
because the review itself was already stored.

This is the same shape as the phase-06 accepted-record bug: a file-backend
artifact path read by the checkpoint digest has no Postgres document twin.
Reviews are intentionally typed append-only rows, so the mapping must be a
**read-only derived view**, never a second writable review store.

## Live evidence (2026-11-02, macmini, read-only)

| Fact | Value |
|---|---|
| Project | `dit-dang-audio` (`a11aca7e-c03a-59ea-84a6-9d900f7f6ada`) |
| Story | `01-truyen-moi` (`3606aa16-e8b7-42c8-8df8-17b1de4af6c8`) |
| Failed tool | `save_review`, scope=global, chapter=5, at 03:05:18Z |
| Error | `checkpoint review: digest artifact reviews/05-global.json: ... unmapped document path` |
| Review round | `05-global` round 13, valid=true, created 03:05:18.940Z, chapter=5, scope=global, verdict=polish, affected=[2,3] |
| Progress | flow=`polishing`, completed=[1..10], pending_rewrites=[2,3] |
| Review checkpoints | 0 for this story (171 checkpoints total) |
| Lease | live and renewing, holder `2544be2a-…`, fence 5, expires 03:17:46Z |
| Jobs table | no `jobs` row for this story; the engine run is lease-based, not a queued job |

Interpretation: the review and progress facts are already persisted. Only the
review checkpoint and the tool response are missing; the editor dispatch then
stopped with an attention/arbiter pause, which is the visible “stuck” state.

## Change

`apps/noverse/internal/persistence/postgres/docstore.go`

- New `docReview` class and `DocClassReview`, regex
  `^reviews/(\d+)(-global)?\.json$`.
- `resolveDoc` maps the path to `scope_key` (`05` / `05-global`).
- `readRef` renders the latest non-invalidated `reviews` row through
  `json.MarshalIndent` — the same shape the file backend writes — so the
  checkpoint digest follows review content.
- `Write`/`Remove` return `errReviewReadOnly`; reviews stay append-only through
  `Narrative.SaveReview*`. No silent empty digest and no no-review fallback.
- `Exists` reports the typed row; `ResolveDocument`/`DocumentPath` round-trip
  the new class. Other unmapped review paths (`reviews/rounds/…`) still error.

`apps/noverse/internal/persistence/postgres/docpath_test.go`

- Round-trip coverage for `reviews/01.json` and `reviews/05-global.json`.
- Rejection coverage for non-producible review identities.

`apps/noverse/internal/persistence/postgres/review_doc_test.go`

- Missing review is not-exist.
- Saved global review renders at `reviews/05-global.json` with indented bytes.
- `Checkpoints.AppendArtifact` succeeds and returns the expected digest/artifact.
- Document write/delete of the review path is refused.
- A new review round changes the digest (does not report the old checkpoint as
  current).
- Chapter (`reviews/05.json`) and global (`reviews/05-global.json`) are distinct.

`apps/noverse/docs/architecture.md`

- Phase-06 block now documents both read-only derived views and their gate tests.

## Parent verification

Verified against isolated source tree
`61807f107a38086ee6d01206b4cfcc998ac183ef7`, without the unrelated in-flight
voice/mobile changes:

```sh
go test -p 2 ./apps/noverse/internal/persistence/postgres \
  -run 'TestAppGrantBackfillKeepsExistingAccounts|TestPoolSchemaCompatibility|TestPostgresReviewDocumentView|TestDocumentPath' -count=1
go test -p 2 ./apps/noverse/internal/persistence/postgres \
  ./apps/noverse/internal/tools ./apps/noverse/internal/store \
  ./apps/noverse/internal/host -count=1
go build ./apps/noverse/cmd/ainovel-cli
```

All passed with `REQUIRE_POSTGRES_TEST=1` and a fresh, empty local acceptance
database. An initial broad run against the already-migrated UI fixture database
failed two migration-isolation tests because its `public` schema was populated;
the empty database fixes that test setup, without weakening either test.

A throwaway executable exercised real PostgreSQL review persistence followed
by `Checkpoints.AppendArtifact(..., "reviews/05-global.json")`. It produced a
SHA-256 checkpoint digest, preserved the committed `polish` review and
`pending_rewrites=[2,3]`, and made zero provider calls. The fixture was local
and owned by this verification; production story content was not altered.

No schema or generated-SQL change was needed; `LatestReviewByScope` already
exists and filters `invalidated_at IS NULL`.

## Safe recovery after deploy

### Minimal route: plain Resume, no checkpoint replay

Source proof:

- `host/resume.go:describeResume` checks `PendingRewrites` before the
  reviewing/in-progress branches and returns
  “Khôi phục gọt giũa: 2 chương chờ xử lý”.
- `host/host.go:Resume` only labels, calls `requireCleanChapters`, and starts
  the engine; it never reads a review checkpoint.
- `flow/state.go:LoadState` builds routing facts from `Progress` and the typed
  `World.HasGlobalReview` row. It never touches `Checkpoints`.
- `flow/router.go:Route` priority 3 (`PendingRewrites` non-empty) runs before
  every review/aggregate branch and dispatches `writer` for
  `PendingRewrites[0]` (chapter 2), with “Gọt giũa chương 2”.
- `host/engine.go:275,294` consumes `flow.LoadState` → `flow.Route`.

Therefore, after rollout, ordinary Resume continues polishing chapters 2–3
from the already-committed round 13 and progress. A checkpoint replay is **not
required to unblock the job**.

The missing checkpoint only affects review-history presentation:
`entry/web/review_items.go:roundAt` looks for `step=review` +
`artifact=reviews/05-global.json` (empty timestamp without it). `roundOf` still
derives round 13 from the 12 earlier typed rounds. It does not influence
routing, pending rewrites, or content.

### No checkpoint backfill or review replay

The hotfix does not backfill checkpoints or replay `save_review`. Raw model
arguments are not stored; the normalizer derives fields and validates quotes
against current text, so replaying a stored row as input is not equivalent to
replaying the original call. It can legitimately return `ErrToolConflict`.

Use the normal story Resume endpoint after rollout. It reads the committed
review and pending rewrites, preserving the existing editorial decision rather
than asking a model to recreate it. This continuation performs the user's
pending writing work; it is not a read-only health probe.
