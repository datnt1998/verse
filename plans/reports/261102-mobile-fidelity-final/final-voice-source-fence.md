# Voice source fence — hosted preparation publish

## Status

Implemented and verified by primary's real PostgreSQL gateway regressions and affected-package suite. Production normal-path voice/playback is a separate release gate.

## Summary

Hosted preparation (`PrepareClaimed`) no longer binds a composed script or the
prepared render input to a chapter revision that no longer names the text the
model was given.

Root cause (proven by source): the runner resolves `in.Source` before
`PrepareClaimed` snapshots `ref := chapterRef(...)`, and the provider call runs
outside any transaction. `CompletePreparationWith`'s closure then wrote
`SourceHash(in.Source.Text)` with `SourceRevision: ref.CurrentRevision` while
`PlaybackRevisionTx` computed ownership from the chapter's *current* revision.
Two wrong pairings were possible: (a) the revision advanced during the provider
call, so the short stamped revision no longer equals the chapter's current one
and the native ACK's `source_revision == current_revision` check dropped every
attachment; (b) the prose was read before a revision landed, so `ref` was newer
than the text and the ACK accepted/attached audio synthesized from prose that
revision does not contain. Re-reading `ref` before the write (as
`pg_compose.go` does) fixes (a) by creating (b), which the ticket forbids.

Fix — all inside the same `CompletePreparationWith` transaction, before any
write:

- `apps/noverse/internal/persistence/postgres/chapter_source.go` (new, narrow):
  `Narrative.LockChapterSourceTx` takes `LockChapterByNumber` (`FOR UPDATE`, the
  same row lock `AcceptRecord` holds via `LockChapterByID` while it appends a
  version and moves `current_revision`), then `GetChapterVersion` for the exact
  revision `current_revision` names. Returns `{ID, Revision, SHA256}`, where
  `SHA256` is the stored `chapter_versions.sha256` (the revision store's
  canonical `domain.ChapterContentSHA256`), with `ok=false` for a missing
  chapter/revision row.
- `apps/noverse/internal/audio/pg_prepare.go:233-240`: the publish closure calls
  the helper first and refuses with a retryable conflict — no
  `persistScriptTx`, no `PlaybackRevisionTx`, no input swap — unless
  `preparationSourceMatches` (`:68-90`) confirms all of:
  1. the locked chapter id equals `ref.ID`,
  2. the locked revision equals `ref.CurrentRevision` (no advance during the
     provider call, no pointer snapshot newer than the text),
  3. `chapter_versions.sha256 == domain.ChapterContentSHA256(in.Source.Text)`
     (the prose that was read is the prose the revision names, catching a read
     that happened before a revision landed). The canonical digest is
     normalization-invariant (BOM/CRLF), and it is the exact identity the
     revision service itself checks when applying edits (`revision/service.go`
     compares `ChapterContentSHA256(prose)` with the record hash), so normal
     engine/mobile/revision commits compare equal.
  Locking the chapter row first means the revision writer cannot commit between
  this check and the script/input commit; after the fence the closure proceeds
  exactly as before (script CAS, pause/cancel job fence, ownership, same-row
  input swap). The script's own `sourceHash` stays `audio.SourceHash(text)` —
  the fingerprint the UI compares — and it is only written together with a
  revision that canonically contains that text.
- `apps/noverse/internal/audio/pg_prepare_test.go:41-87`:
  `TestPreparationSourceFence` covers same source (including CRLF-vs-LF prose),
  revision advanced during compose, prose read before the revision moved,
  missing revision row, re-created chapter id, and uncommitted ref.
- `apps/noverse/internal/entry/web/pg_audio_prepare_source_test.go` (new):
  `TestPGHostedPreparationRejectsSourceChangedDuringModelCall` is the real
  gateway regression: it queues through `postPrepQueue`, blocks the provider,
  calls `nar.AcceptRecord` for a new revision mid-call, and asserts the same row
  ends `failed` with `needsScript` retained, the provider was called exactly
  once (no opaque retry), no script exists, a registered tts worker cannot claim
  the row, and an explicit `svc.Retry` re-resolves the revised accepted record,
  publishes a locked script with `SourceHash(revised)` on the same job id, and
  turns the same row into the queued render input with exactly one more
  provider call. It reuses `pg_audio_prepare_test.go`'s harness/helpers.

Interleavings now handled:

| prose read | `ref` read | revision edit timing | outcome |
|---|---|---|---|
| old | old | during provider call | locked revision ≠ ref → retryable failure, nothing written |
| old | new (edit before `ref`) | before prepare | accepted-content hash ≠ composed text → retryable failure |
| new (artifact written before accept) | old | accept not committed | revision/hash mismatch → retryable failure |
| current | current | none | publishes with `SourceRevision ==` locked revision |

Failure is recorded through the existing `failPreparation` path (same row,
`needsScript` kept, `failed` + message, queue frame published), so the existing
Retry re-enters preparation; `preparationInput` re-resolves the volume source
on that attempt, so the retry composes the new text under the new revision. No
automatic or opaque retry happens inside one generation, and the fence adds no
provider call.

No provider, production, browser, test, build, lint, or formatter command was
run. Full Go suite and the single synthetic production flow remain the
primary's.

## Concerns

1. `pg_compose.go` was fenced by the primary using the same chapter lock:
   `postgres.Audio.SaveComposedScriptBundle` (added to `chapter_source.go`)
   snapshots `chapterRef` before the model call and re-validates
   `LockChapterSourceTx` + `domain.ChapterContentSHA256(sc.Text)` in the script
   transaction, refusing with the generic conflict message instead of stamping
   the pre-model revision on the composed text. Verified by source read; the
   compose regression is `TestPGComposeRejectsSourceChangedDuringModelCall`.
   No action left for me there.
2. The fence treats `chapter_versions.sha256` (canonical, normalization-
   invariant) as the accepted text identity, which tolerates BOM/CRLF/line-ending
   differences between the prose document and the stored revision. Real prose
   differences are refused: that is the same invariant the revision service
   already enforces when applying an edit (`revision/service.go:226-231`
   compares `ChapterContentSHA256(LoadChapterText)` with the record hash).
   Consequence: a chapter whose working prose is ahead of its accepted revision
   (a pending, unsynced edit) now fails preparation retryably until that edit is
   accepted; previously it composed the working prose and stamped the old
   revision (the defect being fixed). Divergent imported prose behaves the same
   way. Intended, but it is a behavior change for those states.
3. Multi-process deployments: the chapter row lock serializes publication with
   the revision writer per database, which is what the fence needs; no
   process-local assumption is added.
