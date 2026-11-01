# Incremental legacy playback coverage — 2026-11-02

Status: DONE

## Corrected producer contract

The source review of `f84c9c20` correctly identified that ordinary non-redo enqueue intentionally freezes only segments still requiring synthesis. A whole-chapter frozen-count requirement therefore rejected valid pre-upgrade incremental jobs. This report supersedes the earlier ownership report's statement that every partial snapshot is unknown.

The follow-up changes only legacy coverage validation and its focused tests/docs; it adds no public API. Legacy proof now requires:

- A nonempty, unique frozen work set whose size matches the persisted `ProgressTotal`. Missing or inconsistent totals remain unknown, not a wildcard.
- Every frozen segment's required fields and actual source/synthesis parameters/key to match current chapter context, with a consistent frozen engine/model.
- Complete chapter coverage: each omitted segment must already have a selected render with its expected current synthesis key and a ready WAV blob. The blob must belong to the project and its parsed audio-segment key must match the story, WAV extension and registered checksum.
- The same proof and database transaction for native ACK and completed-job recovery. Missing, stale, unready or incorrectly scoped omitted coverage cannot authorize attachment/publication.

The canonical audible fingerprint and whole-chapter legacy voice/settings protections from `f84c9c20` remain intact. Unknown custom sample/reference provenance remains fail-closed.

## Verification exercised

Final combined verification passed all 13 selected top-level tests, including the existing 12-test gate and six new incremental subcases. None skipped. The test run compiled all three affected Go packages.

```sh
go test -p 1 ./apps/noverse/internal/audio ./apps/noverse/internal/persistence/postgres ./apps/noverse/internal/entry/web \
  -run 'TestPGLegacyIncrementalPlaybackCoverage|TestPGLegacyPlaybackOwnership|TestPGPlaybackAudibleOwnershipStable|TestPGNativeCompletionPublishesChapterPlayback|TestPGAudioExportCacheHTTP|TestPGAudioQueuePlaybackHTTP|TestWorkerAPIPostgresPublish|TestPGMinIOExportFormatsReuseSourceAndPeaks|TestPGExportHeartbeatLossPreventsAttachment|TestPGExportRunnerSurvivesJobControl|TestAudioScriptSegmentsRenders|TestAudioScriptReplaceIsCAS|TestExportCacheHashAndExpiry' \
  -count=1 -v
```

The authorized local fixture environment was loaded without printing credentials. HOME was temporary; prescribed Go caches, `GOWORK=off`, `REQUIRE_POSTGRES_TEST=1` and package parallelism 1 were used. Final package times: audio 3.860s, postgres 1.861s, web 16.024s.

`TestPGLegacyIncrementalPlaybackCoverage` uses ordinary `PGService.Enqueue`, not Redo or a fabricated partial input. A two-segment chapter has a real ready selected WAV for its first segment; the producer demonstrably freezes only segment two with persisted total 1. With ownership removed to represent pre-upgrade protocol:

- Unchanged incremental native ACK attaches its real WAV and automatically assembles playable chapter MP3.
- An unchanged completed old-server incremental job recovers and automatically assembles playable MP3.
- Missing or stale omitted selected coverage rejects both ACK selection and recovery publication.
- Missing/inconsistent frozen totals are rejected.

Existing whole-chapter changed-voice/settings cases, canonical no-op cast/other-chapter preservation, sample/pause/selected-take invalidation, public chapter HTTP/real ffmpeg decode and MinIO/M4B paths all pass in the same final run. PCM fixtures exercise real bytes and assembly, not a claim of real voice inference; production synthesis/provider code is unchanged.

## Modified files

Final file lengths, not diff sizes:

| File | Lines |
| --- | ---: |
| `apps/noverse/internal/audio/pg_playback.go` | 330 |
| `apps/noverse/internal/entry/web/audio_playback_pg_test.go` | 438 |
| `apps/noverse/internal/entry/web/audio_playback_ownership_pg_test.go` | 539 |
| `apps/noverse/internal/entry/web/worker_api_pg_test.go` | 331 |
| `apps/noverse/deploy/README.md` | 1085 |

Legacy fixtures now preserve their actual producer work totals rather than creating nullable-total transport placeholders. The primary truncated-snapshot regression still fails closed against its recorded total.

## Remaining gate

No implementation blocker is known after the final focused gate. TTSReview receives the final commit for source closure. The parent still owns deployment and the already-seeded real production native MP3/M4B replay; no production state or session files were touched.
