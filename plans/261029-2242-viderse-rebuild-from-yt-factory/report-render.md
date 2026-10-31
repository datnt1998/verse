# Render, measurement, score and export delivery

## Scope and contracts

Implemented `internal/domain/quality`, `internal/render`, score/export pipeline steps, real build scoring manifest, fenced export approval, HTTP request/token endpoints and serve wiring. The existing editor service is constructed once and shared; no second editor workspace/store is created. Managed Chrome for Testing is resolved through `internal/tools`, not a user Chrome profile.

The score API is `render.Service.Quality` / `quality.Score`, with checks, blockers, bound revision/settings/artifact identity and timed fix ranges. The browser uses `POST /api/clone/:id/measure`, `POST /api/clone/:id/export` with `{accept: boolean}`, and the returned request state URL. The token-only pages are `#/measure/<requestId>` and `#/render/<requestId>`; their runtime/snapshot/project-media, nonce claim, fenced heartbeat/progress, PCM or export upload/commit endpoints are request-scoped. These contracts were supplied to EditorApi and EditorWeb and exercised by their real pages.

Approval wakes the existing pipeline rather than rendering in an HTTP handler. `PrepareCloneExport` locks the job, rejects live leases, retains revision fencing, and does not reset already-approved pending work. Durable queued intents resume interrupted pipelines at startup. Chrome retries retain the published HTTP request identity while rotating attempt credentials. Export work uses one sequential lane.

## Source parity and acceptance policy

All source references below are in yt-factory commit **2c201e6**.

- `src/lib/clone-quality.ts:43–55,156–158,181–182`: weighted checklist, minimum 90, duration ±10%, loudness ±1 LUFS, required 1920×1080 at 30 fps. An independent blocker can reject even a total of 100.
- `src/server/clone/score.ts:243–244`: cut tolerance is **max(2 frames, 100 ms)**, therefore 3 frames at 30 fps. The plan's “≤2 frames” shorthand is incorrect; Main explicitly chose source parity.
- `src/server/editor/clone-export-policy.ts:98–109`: template media integrity is checked before an accepted checklist can open export. Lines 111–112 allow accepted checklist decisions; lines 114–129 otherwise reject missing/stale/below-threshold scores. Thus accepted + missing score is allowed, but accepted + template media drift remains refused.
- `src/server/editor/clone-export-policy.ts:171–226`: final template/probe/drift integrity checks precede the checklist waiver; accepted exports retain their freshly computed diagnostics.
- `src/server/editor/projects.ts:1614–1643`: final session drift enforcement; lines 1628–1633 preserve the original accepted-artifact-drift policy. Acceptance is not a general revision or byte-integrity bypass.

Compression uses exactly `libx264 -preset veryfast -crf 28 -maxrate 900k -bufsize 1800k -g 300 -pix_fmt yuv420p -c:a copy -movflags +faststart`, with source ROI coverage and safe smaller-equivalent-output replacement.

### Real build manifest added

Phase 05 did not publish all the evidence consumed by source scoring. `steps/build/manifest.go` now publishes the actual build's version/generatedAt/projectId/generation/baseRevision/committedRevision/recipeFingerprint/itemIds; audio media identity, source LUFS/reference version/duration/hash/target/applied gain; source SHA; envelope hash/sample rate/duration; caption summary; thumbnail media/hash; scene media and frame bounds; and command count. These are captured from the real build, not inferred by a score adapter. The historical approval SQL column was corrected to `output_manifest`.

## Supervisor and token security

Each attempt owns its recorded process group and temporary profile. There are at most two crash restarts for early exit, unclaimed page, heartbeat beyond 45 seconds or stalled progress. Startup sweeps recorded groups/profiles after abrupt owner death and rotates credentials before replaying requests. macOS Chrome includes `--use-mock-keychain --password-store=basic`; a controlled real Chrome run without them hung for 20 seconds, whereas the same fresh-profile probe with them returned its page immediately.

Cookie bypass is limited to valid same-origin render/measure request capabilities. Wrong/expired/other-request credentials, terminal capabilities, cross-site requests and ordinary cookie-auth paths are covered by narrow HTTP tests. Media access uses actual linked media plus explicit document media references; it does not treat the project UUID as a media ID or manufacture metadata. Measurement accepts only PCM f32le stereo 48 kHz, up to 8 GiB, then invokes ffmpeg ebur128 and deletes staged PCM on terminal completion.

## Real smoke evidence

A phase-05 public-service/pipeline fixture built project `clone-47c2bbb3-af91-4de4-aebe-2b211c6ed2f2-g1` from job `47c2bbb3-af91-4de4-aebe-2b211c6ed2f2`. It used isolated PostgreSQL schema and temporary HOME, real generated media and managed tools, not raw SQL project/score seeding.

The real token-only measurement page uploaded **2,304,000 bytes**, 288,000 stereo frames, **6.0 seconds**, at 48 kHz. The server measured **−14.0 LUFS** using ebur128; the PCM file was removed after completion. This measurement was driven in the worker's isolated diagnostic browser while diagnosing Chrome startup. The subsequent MP4 export was claimed and completed by the corrected managed headless Chrome supervisor.

The weighted score was **100/100**, but `passed` remained **false** because the standard template had an independent Subscribe prerequisite: `standard-end-subscribe-backdrop` was missing. The real export deliberately used **accept: true**, preserving this blocker as diagnostics rather than incorrectly claiming a normal gate pass.

Actual export request `e1e88054-4fbb-4f14-a432-c91b2e7b5c11` was claimed at `2026-10-31T19:05:02.728006Z` and finished at `19:05:21.270429Z`. Export ID: `80ba0edd-5636-41de-ad1b-279d9881297f`. Chrome for Testing 155.0.8059.79 rendered all 180 frames, uploaded the output, and the server performed final probing, scoring, gate evaluation and compression. The upload was 706,447 bytes; the final ready MP4 was 622,337 bytes, accepted=true, with final score 100 and the independent blocker retained.

The final MP4 is preserved outside fixture cleanup at **`/tmp/p06-render-smoke.mp4`**.

```text
SHA-256: 66b5e8eb21dfbe5da660fcae7c40fcdd2e33d508832e30941aba72e0f55756d2
ffprobe video: codec_name=h264 width=1920 height=1080 r_frame_rate=30/1
ffprobe audio: codec_name=aac sample_rate=48000 channels=2
ffprobe format: duration=6.000000 size=622337
ffmpeg ebur128 final AAC decode: I=-14.0 LUFS, threshold=-24.0 LUFS, LRA=0.0 LU
```

The decoded video was H.264 High, progressive yuv420p bt709; audio AAC LC stereo. The completed export profile was absent after cleanup.

The initial short-lived fixture expired while a consumer was still using it; it was replaced by a fixture held until explicit `/tmp/p06-fixture.release` or SIGTERM. The replacement's actual `Service.Quality` preflight and authenticated GET quality returned successfully. The old GET 500 did not retain a diagnostic proving its precise cause; attributing that historical response solely to the expired fixture schema is an **inference**, not a claimed proven provider fix.

For subsequent V13 caption-range/alignment acceptance, the fixture owner published an independent **authored golden timing** artifact through the real guarded job-edit/media publication APIs. Its SHA-256 is `461cc59192f66ef5b8103d1a28bb9130fdf552ee1ea62813089c9a4ac94b7b17`; source AAC SHA-256 is `7a695f5074553cfae69f0f8f34a04b55c19866c87b00ee1aa650389378084b90`. The source is a synthetic sine and captions are authored fixture data: this is explicitly **not** a claim of ASR transcribing speech. The helper verified the actual `editor.Anchors` artifact resolver, after normal public fenced recovery of an expired interrupted export. It did not rewrite scores or database rows.

The original real output also exposed a separate post-render pipeline issue: the ready MP4 and render request were complete, but the pipeline export step remained running because replacement retained an input blob read lease while retiring that binding. EditorApi fixed the deadlock and binary durable-resource ownership in `06362ec6`. A real restart then failed closed on the historical original-upload binding; `112bb936` added verified same-export original SHA-state lineage recovery rather than a prefix exception or deletion bypass. The next actual server startup succeeded, preserved the valid compressed MP4 and safely retired the proven superseded binding. This distinguishes the proven original MP4/request completion from the later terminal pipeline acceptance.

EditorWeb's subsequent real V13 interaction used a newly measured, non-overlapping authored-cue baseline. A single actual `cueIds:["c1"]` alignment returned −0.4 seconds; its captured project diff contained only cue/word timings, estimated-word provenance and updatedAt. The UI correctly showed 1/2 fixed, two stale caption checks and nine unaffected passed checks; global export freshness stayed closed. Evidence: `/tmp/p06-editor-actual-alignment-diff.json`. A measured/aligned word is deliberately immutable in the source alignment contract, so authored fixture timings were honestly marked estimated before requesting alignment; no measured anchor or score was fabricated.

### Terminal pipeline smoke after the fixes

The actual later UI export `de9fd69b-b4a1-4809-b423-29a293b7a115` completed the entire pipeline. A read-only owner-side status query observed job **succeeded**, fence 15, with **score succeeded** and **export succeeded**, both attempt 4; the output manifest names this same export. Its ready metadata has accepted=true, score 100, captionAlignment/captionKaraoke passing, and only the independent Subscribe blocker retained. The UI downloaded it with HTTP 200 and decoded a six-second preview.

This terminal-pipeline MP4 is separately preserved at **`/tmp/p06-render-smoke-final.mp4`**:

```text
SHA-256: 92c9941228225f232d98b242c6b7bbad98b96ef0371cbcfb15af26a3ad1f0766
ffprobe video: h264, 1920x1080, r_frame_rate=30/1
ffprobe audio: aac, sample_rate=48000, channels=2
ffprobe format: duration=6.000000, size=576799
ffmpeg ebur128 final AAC decode: I=-14.0 LUFS, threshold=-24.0 LUFS, LRA=0.0 LU
```

These probe/hash/loudness values were measured directly on the preserved final bytes, not inferred from timeline settings or copied from the first output.

### Genuinely passing checklist export

EditorWeb subsequently restored all three canonical signed-template end-card items through normal engine commands and measured again. The score was genuinely **100, passed=true, blockers=[]**, with all eleven checks fresh and passing. A subsequent actual export `f2ef53ff-2506-47d7-9c57-7c130e18aecf` completed: the read-only owner query observed job succeeded at fence 17, score/export succeeded at attempt 5, and the matching output manifest. The POST body contained no new acceptance; however, metadata still records accepted=true from the earlier durable owner decision. This is not presented as proof that prior acceptance was cleared: the checklist itself independently passes.

Preserved file: **`/tmp/p06-render-smoke-passed.mp4`**. Direct ffprobe: H.264, 1920×1080, 30/1 fps; AAC stereo at 48 kHz; duration **6.000000 s**, size **621,793 bytes**. Direct final AAC ebur128: **−14.0 LUFS**, LRA 0. SHA-256: **`adf7bc39f38d827867739044bed0e3377a1fe62bad502a9216e888a72f6feb9c`**.

### Final HTTP lifecycle and byte-count acceptance

After the finalization fix, real request `8140febf-2520-4e92-867f-0a77bc191668` progressed queued → running/claim → running/rendering → running/compressing with **no export record** → done. Only the final done response exposed the MP4, and its **573,436-byte** record exactly matched the HTTP 200 download. Export ID: `2a3cc4fb-e307-4265-bf6d-1beaf810ed62`. The owner-side query independently confirmed job succeeded at fence 21, score/export succeeded at attempt 7 and the matching output manifest. Final quality: total 100, passed=true, blockers=[], durable prior acceptance still recorded.

Final recommended smoke artifact: **`/tmp/p06-render-smoke-finalized.mp4`**.

```text
SHA-256: 55619efc2ece9501f5b1176b04cf662df0eef2bc5647864bb495b7e6b2a71461
ffprobe video: h264, 1920x1080, r_frame_rate=30/1
ffprobe audio: aac, sample_rate=48000, channels=2
ffprobe format: duration=6.000000, size=573436
ffmpeg ebur128: I=-14.0 LUFS, threshold=-24.0 LUFS, LRA=0.0 LU
```

Evidence: `/tmp/p06-editor-finalization-polls.json` and `/tmp/p06-editor-final-export-evidence.json`. The final restart also exercised EditorApi's tombstone-proven reclamation of a deleted conflict-copy revision (`1bf2e962`); no manual file deletion or inventory bypass was used.

## Exercised verification

Only narrow checks were run; no full-app Go suite, full-app race run or repeated tsc was run by this worker.

- Supervisor exact-two-restart early-exit and unclaimed-hang tests, sequential lane, claimed heartbeat/progress stalls, and the actual subprocess kill-9/recover/no-orphan test passed under targeted `-race` runs. Disabling restarts first failed with one start instead of three.
- `TestSupervisorChromeNeverUsesUserKeychain` failed first for missing Chrome arguments, then passed under `-race` after the flags were added.
- `TestSupervisorRetriesKeepPublishedHTTPIntent` failed first because HTTP stateUrl was stranded on an abandoned request, then passed with `go test -race -p 1 ./internal/render -run '^TestSupervisorRetriesKeepPublishedHTTPIntent$'`. Three attempts retain one request ID and use distinct tokens.
- `TestRenderRecoveredExportRequeuesInterruptedPipeline` and `TestRenderDuplicateInterruptedIntentRequeuesSameRequest` failed first with zero preparation calls, then passed targeted `-race` after durable intent reconciliation was wired.
- `TestRenderScopedMediaUsesBuildMediaIdentities` failed against the old implementation (401 instead of 200), then passed with real build media identities and unrelated project UUID refusal.
- Narrow source score/export-gate cases passed, including accepted+template-drift refusal, accepted+nil-score allowance, nonaccepted+nil-score `missing_score`, revision drift refusal, idempotent approval and live-lease rejection.
- Narrow score handler tests and export handler tests passed, covering the one remeasure/rescore retry and failed gate behavior.
- Request/measurement store tests covered nonce/fence/lease, PCM sequencing and duplicate chunks, fixed format/8 GiB cap, crash-tail reconciliation, terminal deletion and recovered token rotation. The recovered-token expiry regression failed before stamping new token issuance, then passed.
- Twenty-five top-level regions/loudness/compression tests plus their subtests passed. The optional `YTF_CLONE_REAL_BINARIES=1` ROI-specific integration test was skipped in that run; the real end-to-end MP4 above independently exercised production compression and tools.
- Scoped HTTP token tests passed for wrong/expired/other-request tokens and path isolation.
- App-local Vite build succeeded after EditorWeb fixed the real worker chunk format failure, native fetch receiver failure, and missing subtitle schema enum. These frontend changes belong to EditorWeb's commits, not this worker's implementation.
- `TestQualityCaptionOnlyEditInvalidatesOnlyCaptionTiming` first failed because all nine unrelated checks were stale; it and `TestQualityStalenessConservativelyHandlesUnknownAndNonCaptionChanges` then passed with targeted `-race`. Score persistence now retains the actual measured project. Cue-time/word-only subtitle edits invalidate alignment and karaoke together; other edits, settings/artifact changes and old records without a measured document remain conservatively stale. The global revision-bound gate is unchanged.
- `TestAlignmentRangesExcludePassingAnchorsAndKeepMedianFailures` failed first with four ranges instead of two, then passed targeted `-race`; `TestScoreRangesFiltersPersistedPassingAnchors` independently failed first on stored JSON ranges and passed after the same outbound filtering. Exact matches and offsets ≤120 ms are not falsely exposed as failures; 150 ms median-failure contributors and signed early/late evidence remain actionable. Main approved source parity over the illustrative V13 84-point/300 ms data; design Q&A #66 records that decision.
- `TestQualityWordAlignmentMetadataKeepsOtherChecksFresh` failed first with nine unrelated stale checks, then passed targeted `-race`; actual alignment's `wordsEstimated` metadata is part of caption timing evidence rather than an unrelated project edit.
- `TestExportStateWaitsForMatchingPipelineFinalization` and `TestExportStateFinalizationFailureIsTerminal` failed first when browser upload completion was incorrectly exposed as final ready output; targeted `-race` passed after owner-facing request state was bound to the matching succeeded pipeline export manifest. Until then the owner sees running/compressing, not the pre-compression record. Finalization failures become terminal failures. Runtime token expiry is unchanged: the token store remains terminal when the browser ends, and the owner-only polling adapter does not reactivate capabilities.

Main owns integration-wide checks. Recommended single integration commands, from `apps/viderse`, when its test lane is free:

```sh
go test -race -p 1 ./internal/render ./internal/domain/quality ./internal/pipeline/steps/score ./internal/pipeline/steps/export ./internal/web
go test -p 1 ./internal/persistence/postgres -run '^TestBulkApprovalDurablyResumesThroughRenderGateButNeverBypassesLowScore$'
```

No credentials, request capabilities or database connection strings are included in this report.

## Commits

- `18d84217` — render measurement/export pipeline and real build evidence.
- `9b46f81a`, `988724c0` — strict project-scoped render media resolution.
- `eb3ba4aa` — headless Chrome macOS keychain isolation.
- `d5f3c732` — resume durable export intents after interruption.
- `64678901` — stable HTTP tracking across Chrome crash retries.
- `7160862e` — actual measured document baseline and caption-only presentation invalidation.
- `cbc92d5f` — persistent real-media smoke fixture, committed by the fixture owner.
- `fe65b7e9` — guarded independent golden caption timing fixture publication.
- `e2b7dfa2`, `b4621bb8` — actionable current/persisted caption ranges, measured-word metadata freshness and approved design Q&A #66.
- `4d4d2e1a` — owner-visible export completion waits for the matching post-compression pipeline output.

Editor API and headless frontend companion fixes were coordinated with their owners. EditorWeb explicitly released the shared fixture after its final successful lifecycle/byte-count check. The shared server exited, both worker browser sessions were closed and the owned smoke executable was removed. The held `TestRenderPhase05SmokeFixture` then **passed** after explicit release (10041.71 s); its registered teardown removed the isolated schema, temporary HOME/fixture directory, private environment handoff and release marker without reported errors. `TestRenderSmokeInstallChrome` and `TestRenderSmokePublishGoldenAnchors` also passed. MP4 evidence is preserved outside that cleanup.
