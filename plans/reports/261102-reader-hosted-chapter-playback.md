# Hosted chapter playback repair — 2026-11-02

Status: DONE

## Outcome and cause

Hosted native rendering attached selected WAVs and completed its durable TTS job, but chapter GET only looked up `chapter-audio:v<V>:c<C>`; only an explicitly requested export published that cache entry. No automatic assembly dispatch connected those two paths. The worker's real synthesis path and public chapter route did not need replacing.

`PGWorkerQueue.Complete` now queues assembly inside `Jobs.CompleteWith`, after validated render attachment and selection. `Audio.EnqueuePlaybackTx` inserts an idempotent `audio_export` child (`chapter-playback-<tts-job-id>`). The existing `PGExportRunner` claims it under its usual lease/heartbeat/fence and invokes `processPlaybackJob`, reusing export scratch materialization, the existing chapter assembler, and immutable blob upload. The MP3 cache attachment lands inside fenced completion. GET remains read-only; neither GET nor the private ACK runs ffmpeg inline. Internal assembly jobs do not appear as duplicate user render/export jobs in the audio queue.

## Ownership and recovery

- Enqueue freezes script/source, cast, synthesis settings and project voice ownership. Stale native completions cannot change selected renders.
- Publication compares script/source, cast/settings/voice, selected-render and latest-native-attempt fingerprints. Chapter GET also checks that fingerprint, so stale cache rows are not presented as current playback.
- Existing mutable configuration rows are share-locked through transactional attachment/publication. A changed accepted source revision supersedes queued assembly.
- Duplicate native ACKs retain the existing 409 fenced-out contract and cannot enqueue duplicate work.
- The runner reconciles completed pre-upgrade TTS rows before claims. A committed ACK/dispatch survives restart; a dead runner's claim is recovered by the existing export lease mechanism. A cancelled child remains terminal and is not recreated.
- Accepting a segment candidate queues its rebuild in the same transaction as selection. Manual MP3/M4B export still works and uses revision-guarded chapter-cache publication.

## Verification exercised

Fail-before evidence: the new real-Postgres/worker-HTTP/real-ffmpeg test was run against a temporary Go overlay of pre-fix HEAD production sources. It failed after successful native-like WAV upload and completion with `durable runner: ran=false err=<nil>`. The temporary overlay was removed.

Pass-after evidence: final focused command compiled all three touched Go packages and passed all 10 selected top-level tests (including cancellation/pause subtests):

```sh
go test -p 1 ./apps/noverse/internal/audio ./apps/noverse/internal/persistence/postgres ./apps/noverse/internal/entry/web \
  -run 'TestPGNativeCompletionPublishesChapterPlayback|TestPGAudioExportCacheHTTP|TestPGAudioQueuePlaybackHTTP|TestWorkerAPIPostgresPublish|TestPGMinIOExportFormatsReuseSourceAndPeaks|TestPGExportHeartbeatLossPreventsAttachment|TestPGExportRunnerSurvivesJobControl|TestAudioScriptSegmentsRenders|TestAudioScriptReplaceIsCAS|TestExportCacheHashAndExpiry' \
  -count=1 -v
```

The run used the authorized local fixture environment without printing secrets, a temporary HOME, the prescribed Go caches, `GOWORK=off` and `REQUIRE_POSTGRES_TEST=1`. All selected tests ran; none skipped. Package times were audio 3.675s, postgres 1.650s, web 6.533s.

The new end-to-end integration exercises native-protocol claim, real PCM WAV HTTP upload, validated ACK, durable runner recreation, automatic MP3 publication, actual chapter HTTP GET and ffmpeg decode. It verifies both segments plus their pause, reread replacement, superseded assembly, candidate selection, pre-upgrade completion recovery, existing M4B export, settings/cast ownership changes, explicit cancellation and accepted-source supersession. The existing MinIO test additionally exercised real S3-backed source materialization, MP3/M4B output and peaks.

The test waveform is an explicit PCM fixture, not a claim of real VieNeu inference. Production native synthesis code, pinning, voice providers and source selection are unchanged; no fake-voice fallback or source bypass was added. The parent owns production deployment and replay of the already-observed real native chapter fixture.

## Files changed

Final file lengths, not diff sizes:

| File | Lines |
| --- | ---: |
| `apps/noverse/internal/audio/pg_jobs.go` | 606 |
| `apps/noverse/internal/audio/pg_play.go` | 282 |
| `apps/noverse/internal/audio/pg_export.go` | 1079 |
| `apps/noverse/internal/audio/pg_playback.go` | 95 |
| `apps/noverse/internal/persistence/postgres/audio_playback.go` | 207 |
| `apps/noverse/internal/entry/web/worker_api_pg.go` | 403 |
| `apps/noverse/internal/entry/web/audio_playback_pg_test.go` | 401 |
| `apps/noverse/deploy/README.md` | 1070 |

No UI, platform, Viderse, production state, browser sessions or protected listeners were modified.

## Concerns / remaining parent-owned gate

No implementation blocker remains. The parent must merge/deploy the backend and retake the real production chapter MP3/M4B proof; this agent did not mutate remote production or claim that replay had happened.
