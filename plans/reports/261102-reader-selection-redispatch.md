# Candidate selection redispatch — 2026-11-02

Status: DONE

## Cause and repair

TTSReview closed the legacy ownership/incremental coverage findings in `3d3dec99` but identified another concrete normal path: select B, assemble B; select A, assemble A; select B again. A canonical-content-only dispatch key collided with B's already-completed child, leaving A in the single chapter cache and causing GET to reject it for B.

The selection transaction now uses chapter ID, segment ID, canonical content and the selected row's resulting version as its durable dispatch identity. The version is propagated explicitly from the existing selection UPDATE through `postgres.Render.Version`; no timestamp or additional public method was introduced. Resource identity prevents ambiguity between row-local versions.

Canonical audible ownership/content fingerprints are unchanged. Row version is used only for dispatch generation. Previously completed/cancelled children remain terminal; an explicit new selection creates a distinct child rather than reviving old work.

## Verification exercised

The primary real-Postgres/worker-HTTP/blobstore/ffmpeg integration now selects **B → A → B → A**, runs the durable runner after each selection, performs actual chapter MP3 GET and ffmpeg decode, and checks the expected assembled durations after every step. Subsequent legacy recovery and M4B export still pass.

The final coherent gate passed all 13 selected top-level tests, including whole/incremental legacy ACK/recovery variants, canonical no-op cast/other-chapter invariance, source/take ownership, existing lease/cancel/pause controls and MinIO MP3/M4B/peaks. None skipped; all three affected Go packages compiled.

```sh
go test -p 1 ./apps/noverse/internal/audio ./apps/noverse/internal/persistence/postgres ./apps/noverse/internal/entry/web \
  -run 'TestPGLegacyIncrementalPlaybackCoverage|TestPGLegacyPlaybackOwnership|TestPGPlaybackAudibleOwnershipStable|TestPGNativeCompletionPublishesChapterPlayback|TestPGAudioExportCacheHTTP|TestPGAudioQueuePlaybackHTTP|TestWorkerAPIPostgresPublish|TestPGMinIOExportFormatsReuseSourceAndPeaks|TestPGExportHeartbeatLossPreventsAttachment|TestPGExportRunnerSurvivesJobControl|TestAudioScriptSegmentsRenders|TestAudioScriptReplaceIsCAS|TestExportCacheHashAndExpiry' \
  -count=1 -v
```

The authorized local fixture environment was loaded without printing credentials. HOME was temporary; prescribed caches, `GOWORK=off`, `REQUIRE_POSTGRES_TEST=1` and package parallelism 1 were used. Final package times: audio 3.914s, postgres 1.704s, web 15.847s. The native-completion/MP3/M4B integration itself passed in 4.12s.

An initial selection-generation run exposed that `SelectRenderTx` returns a `Render` DTO rather than an integer and that its mapper omitted the underlying row version. The final implementation propagates and uses the actual `Version` as described above; the final gate passes.

## Modified files

Final file lengths, not diff sizes:

| File | Lines |
| --- | ---: |
| `apps/noverse/internal/persistence/postgres/audio.go` | 1086 |
| `apps/noverse/internal/persistence/postgres/audio_playback.go` | 227 |
| `apps/noverse/internal/entry/web/audio_playback_pg_test.go` | 451 |
| `apps/noverse/deploy/README.md` | 1088 |

## Delivery and remaining gate

This is a follow-up to `f84c9c20` and `3d3dec99`, not a replacement for them. The parent owns integration, deployment and replay of the existing real native fixture. TTSReview receives the committed delta and B/A/B/A evidence for source closure. No UI/platform/Viderse, native provider/pin, production state or session files were changed.
