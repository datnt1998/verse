# Editor publication and saved-media retention

Status: DONE

## Outcome and source contract

- `postgres/clone_edit.go` now compares the complete build output as well as the checkpoint under the existing job/step transaction locks. Ownership, generation, cancellation, busy-step, attempt, status, and fence checks remain ahead of the exact-retry shortcut. A stale rebind succeeds only if its desired checkpoint and complete reconstructed output are already current; matching only a subset of artifacts is insufficient.
- `ProjectBinder.commitBuild` intentionally keeps `build.Step.Checkpoint` while publishing changed `build/manifest.json` bytes. The new PostgreSQL regression uses that production contract: publish A, reject divergent B from the same snapshot without changing step/artifact/binding rows, then accept exact A. Changes to an unrelated artifact or output-data field also prevent stale success.
- `clone/images/rebind.go` retries only manifest publication, at most three attempts. A conflict causes a fresh read and merge only if generation, project, checkpoint, build fence/attempt/status, and the requested scene/thumbnail entry remain unchanged. It does not repeat media import or the already-committed Editor command, and it does not reapply an older same-target binding. Exhaustion and non-conflict failures propagate. The existing callers continue to expose binder failures as `ProjectError` (`UseVersion`) or `RebindError` (image worker).
- `editor/media_gc.go` recognizes portable IDs in structural saved-document references (`mediaId`, cover/source/backdrop references, and string/object media links), alongside existing UUID handling. Saved project/link ownership remains authoritative; arbitrary portable strings and promoted media heads do not become permanent roots. `project_sources.go` required no change: promotion already captured exact immutable metadata/source bindings; GC's ownership discovery was the missing layer.

## Exercised regression evidence

All PostgreSQL scenarios used an owned disposable PostgreSQL 18.3 cluster on loopback port 65439, a fresh test database, per-test isolated schemas, local temporary blob storage, and `REQUIRE_POSTGRES_TEST=1`. Go ran with temporary HOME, `GOWORK=off`, `-p 1`, and the required existing module/build cache paths. No browser, protected port, real user configuration, production fixture, dependency installation, merge, or deploy was used.

### Fail-before

Original production sources were captured before edits and supplied through Go's `-overlay` option while running the new regression tests.

- Publication: `TestCommitBuildMediaUnchangedCheckpointRejectsLostManifest` failed with `want conflict, got <nil>`.
- Real collection: all three `TestSaveProjectPinsCurrentCloneMedia` variants failed with `saved generated media reclaimed after stale` after normal non-overlay metadata was promoted by SaveProject/RunCommands and current clone aliases disappeared.
- Corrected real concurrent consumer fixture: both real binders returned success, but `TestCloneRebindConcurrentDifferentScenesPreservesBothBindings` failed with `successful scene binding lost`; the published manifest retained the old binding for one scene. This ran real ProjectBinder, Editor commands/imports, PostgreSQL CAS, and blob publication. Only the initial-read boundary was synchronized.
- Portable saved clip/link and cover/source/backdrop ownership cases failed against the UUID-only baseline; unrelated-label, unrelated-ID, and invalid-path cases remained unowned.

Initial concurrent-fixture runs exposed missing old metadata and incomplete timeline identity fields. The fixture was corrected to import genuine old media and provide valid track/label/span fields; production validation was not changed. These fixture failures are not counted as regression proof.

### Pass-after

Coherent final focused check (three packages passed; raw output `artifact://69911`):

```sh
go test -p 1 -count=1 \
  ./apps/viderse/internal/persistence/postgres \
  ./apps/viderse/internal/editor \
  ./apps/viderse/internal/clone/images \
  -run '^(TestCommitBuildMedia|TestCloneImageRebind|TestCloneEdit|TestSaveProjectPin|TestCloneRebind|TestMediaGC|TestProjectMediaTokens|TestRebind)' -v
```

This compiled the three affected packages and exercised publication/fence/ownership failures, exact retries, independent/same-target binder conflicts, bounded retry exhaustion, existing lease/deadlock coverage, media GC grace/shared/overlay/external-reference behavior, fail-closed reference reads, tombstones, and missing-source promotion behavior.

Final narrow real-PG smoke (two packages passed; raw output `artifact://69923`):

```sh
go test -p 1 -count=1 \
  ./apps/viderse/internal/persistence/postgres \
  ./apps/viderse/internal/editor \
  -run '^(TestCommitBuildMediaUnchangedCheckpointRejectsLostManifest|TestCloneRebindConcurrentDifferentScenesPreservesBothBindings|TestSaveProjectPinsCurrentCloneMedia)$' -v
```

The smoke ran after the concurrent test was strengthened to check scene-specific binding loss before its retry-count assertion. It proves:

1. Unchanged-checkpoint divergent publication is rejected and exact retry succeeds without mutation/leaked rows.
2. Two real concurrent different-scene binders preserve each scene's exact returned binding in both saved clips and the build manifest, preserve clip duration, and keep source acquisition usable; precisely one manifest-only CAS retry occurs.
3. SaveProject without source adoption, SaveProject with an adopted source head, and RunCommands without autosave preserve exact non-overlay metadata, source bytes, saved clip/keyframes/transform, and links across actual `ReclaimUnreferencedMedia(false)` after stale/replaced aliases.
4. Removing one saved owner preserves shared media; removing the final saved owner allows the actual collector to reclaim it and remove metadata/source heads. Promotion does not make heads immortal.

Baseline publication/collection output: `artifact://69877`. Corrected consumer baseline failure is recorded in this session's `bg_1009` output.

## Review and scope

`EditorStorageReview` independently reviewed the source and corrected integration fixture and reported no outstanding P1/P2 findings. No new distributed lock, broad GC exemption, generic retry abstraction, public API, or migration was introduced. The parent explicitly authorized the narrowly scoped `rebind.go` caller repair; all other changes are within the assigned storage/GC and focused-test ownership.

No full repository test suite, frontend check, full application binary build, browser test, integration merge, or deployment was performed. Remaining blockers: none within this assignment.
