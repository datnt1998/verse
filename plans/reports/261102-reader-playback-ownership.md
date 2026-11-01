# Hosted playback ownership follow-up — 2026-11-02

Status: DONE

## Reviewed causes repaired

1. Missing `ownership` was treated as a wildcard in native ACK attachment and completed-job recovery. It now requires proof from the actual pre-upgrade frozen source revision and the complete segment set. Every required synthesis field must be present/non-null; segment identity, speaker, text, voice, tone, speed, pitch, quality, engine/model and synth key must agree with current script/cast/settings resolution. Unknown or stale provenance is superseded before selected attachment or current chapter publication. Unchanged built-in-voice legacy jobs continue to attach and recover.
2. Whole cast/voice/script/render rows included row versions, timestamps and display metadata in fingerprints. The ownership fingerprint now uses canonical per-chapter audible inputs computed through existing `ResolveVoice`, `pgSegmentKey` and `effectivePauseMs`: ordered segment parameters, referenced voice sample/file identity, script/source identity. Content additionally identifies selected take key/blob/duration/sample rate and latest render attempt/fence. No database row version/timestamp or unrelated voice metadata is hashed.

The existing repository/runner flow remains in place. A small typed proof callback keeps audio semantics in the audio package, with the repository passing its own query surface. In completion/publication/recovery transactions all proof reads use that same transaction connection; there are no nested pooled reads while holding a transaction. Cast decoding is shared with the existing service rather than reimplemented.

Legacy protocol limits are explicit: partial/incomplete snapshots and custom-sample identity not frozen by the old protocol cannot prove currentness and are superseded, not silently upgraded to today's configuration. This does not add a fake voice or fallback provider.

## Focused verification exercised

The final combined gate compiled the three affected Go packages and passed all 12 selected top-level tests, including the existing 10-test gate, six new legacy ACK/recovery subcases and existing cancel/pause subcases. None skipped.

```sh
go test -p 1 ./apps/noverse/internal/audio ./apps/noverse/internal/persistence/postgres ./apps/noverse/internal/entry/web \
  -run 'TestPGLegacyPlaybackOwnership|TestPGPlaybackAudibleOwnershipStable|TestPGNativeCompletionPublishesChapterPlayback|TestPGAudioExportCacheHTTP|TestPGAudioQueuePlaybackHTTP|TestWorkerAPIPostgresPublish|TestPGMinIOExportFormatsReuseSourceAndPeaks|TestPGExportHeartbeatLossPreventsAttachment|TestPGExportRunnerSurvivesJobControl|TestAudioScriptSegmentsRenders|TestAudioScriptReplaceIsCAS|TestExportCacheHashAndExpiry' \
  -count=1 -v
```

The authorized local fixture environment was loaded without printing credentials. HOME was temporary, prescribed Go caches were used, `GOWORK=off`, `REQUIRE_POSTGRES_TEST=1`, package parallelism 1. Final package times: audio 3.522s, postgres 1.713s, web 10.887s.

New evidence:

- Ownership absent, unchanged built-in voice: native ACK attaches real WAVs and durable assembly publishes MP3; a pre-upgrade completed job also recovers.
- Ownership absent, voice or synthesis quality changed before ACK: the old completion cannot select its WAV or publish current playback.
- Ownership absent, voice or quality changed after old-server completion and before recovery: recovery cannot publish stale WAVs under today's settings.
- Missing segment, voice, tone, pitch, speaker or source revision, plus null tone, is rejected rather than passing a subset/zero-value comparison.
- Repeated identical cast saves and another chapter's production `voiceMode` enqueue preserve both published chapter fingerprints/playability and an in-flight native job's ownership; that original job still completes and publishes.
- Unrelated voice edits and referenced-voice display metadata changes preserve ownership. Real referenced sample, effective pause and voice changes invalidate it. Existing primary integration still proves selected-take replacement, source supersession, actual MP3 HTTP bytes/ffmpeg decoding and M4B export.

An initial combined run exposed SQL parameter inference in the new lateral latest-attempt lookup (`uuid = text`). The lookup now explicitly casts the shared chapter parameter as UUID before its text comparison; the final combined gate above passes.

## Modified files

Final file lengths, not diff sizes:

| File | Lines |
| --- | ---: |
| `apps/noverse/internal/audio/pg_cast.go` | 405 |
| `apps/noverse/internal/audio/pg_jobs.go` | 606 |
| `apps/noverse/internal/audio/pg_play.go` | 282 |
| `apps/noverse/internal/audio/pg_export.go` | 1079 |
| `apps/noverse/internal/audio/pg_playback.go` | 280 |
| `apps/noverse/internal/persistence/postgres/audio_playback.go` | 223 |
| `apps/noverse/internal/entry/web/worker_api_pg.go` | 404 |
| `apps/noverse/internal/entry/web/worker_api_pg_test.go` | 330 |
| `apps/noverse/internal/entry/web/audio_playback_pg_test.go` | 438 |
| `apps/noverse/internal/entry/web/audio_playback_ownership_pg_test.go` | 383 |
| `apps/noverse/deploy/README.md` | 1083 |

The existing worker publication fixture now carries a real complete frozen protocol snapshot instead of `segments: 1`; its attachment/fencing assertions remain intact. No UI, platform, Viderse, production state or native provider/pin code was changed.

## Parent-owned gate

The parent owns merge/deployment and the real production native fixture replay. This follow-up does not claim a production replay or independent review closure; both reviewers receive the committed follow-up and focused evidence for their closure verdicts.

## Controller export-download repair

The complete PostgreSQL release gate exposed a real nil-request panic in `/exports/file`: an MP3 blob also had a `chapter-audio:` cache row whose playback manifest was incorrectly decoded as named export metadata. Resolution now applies the same valid, named-export criterion as the export listing, so a playback row cannot hide the actual export filename. The hosted download no longer passes a nil HTTP request to a request-dependent filename helper.

`TestPGAudioExportMusicAndPeaks` failed before with the panic and passes after. It exercises actual PostgreSQL, WAV/MP3 generation, peaks and HTTP download, and now checks that downloaded filename metadata agrees with the listed artifact while allowing the existing transport filename sanitization. Both chapter and volume MP3s are covered; punctuation is not pinned.

The real M4B replay exposed the same manifest-family confusion in aggregate export metadata: a later per-file registry row replaced the summary with a zero duration even though ffprobe reported10.64s. `PGService.ExportInfo` now accepts only the aggregate manifest whose `contentKey` matches its cache-row key. It does not infer family from a permissive JSON decode or a nonempty file list (split exports may legitimately use a different file grouping). The existing real export regression now also checks that its0.2s chapter plus0.3s intro retain a duration of at least0.45s; the focused gate passes.



The actual Listen surface then exposed a readiness ownership error: it displayed “máy chủ đang thiếu công cụ ghép” and disabled volume export although the hosted ffmpeg had produced the M4B successfully. Hosted `/api/tts/status.ffmpeg` now probes the serving gateway, not the synthesis-only worker's heartbeat flag; the unused snapshot copy was removed. The regression covers no connected reader, a reader without ffmpeg on an equipped server, and a reader advertising ffmpeg while the server lacks it. The focused hosted-status gate and the complete Noverse backend gate both pass.

