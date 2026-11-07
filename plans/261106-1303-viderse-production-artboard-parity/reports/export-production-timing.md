# Production export timing and duplicate-acquisition fix · 2026-11-07

Status: implementation and bounded real pipeline smoke verified; production rollout pending.

## Delivered change

- Successful compression no longer eagerly acquires/releases the whole original before ReplaceCloneExportFile acquires it again. Originals larger than cache use disposable full-download workspaces; removing this first pass avoids one full read/download, not compression or integrity checks.
- No-compression and fallback paths still acquire/validate the original; template/tool failures preserve the valid original record and original error precedence. Source-handle release before publication remains unchanged.
- Browser requests and export-step output retain cumulative preparation, awaited render/decode/GPU, capture, residual encoder wait, finalization, upload and commit counters plus submitted-frame count. Preflight is a distinct private phase; existing Vietnamese public progress phases remain unchanged.
- Native logs separately time commit, publication, completion acquisition and postprocessing. Mac resource observations sample app/Chrome-tree RSS, swap and scratch free space every30s and terminal transitions; failed probes omit fields. No process arguments, URLs or credentials are logged.
- Full-upload hashing, provider full readback, revision/caption gates, resolution/FPS/bitrate/audio/effects and compression settings are unchanged. The completion acquisition is intentionally preserved and logged outside browser publishMs.

## Before/after real native pipeline

Same frozen real project,900frames at1920×1080/30fps, actual Chrome+Mediabunny+caption collector, native commit and FFmpeg compression; separate QA database/root, local output overlay. An8MiB cache for export-file acquisitions forces the same oversized-workspace branch as the production2GiB cache encountering the historical2,656,758,643-byte original. Input assets use512MiB cache. No production projects/provider objects were written and no full57-minute render ran.

| Observation | Before | After |
|---|---:|---:|
| Original-file acquisitions over complete path | 3 | 2 |
| Original bytes acquired | 64,413,084 | 42,942,056 |
| Postprocess acquisitions | 2 | 1 |
| Whole QA run | 10.561s | 10.831s |

The unchanged completion integrity acquisition accounts for the other read. This proves one21,471,028-byte acquisition removed; **it does not demonstrate local wall-time acceleration or production wire throughput**. No89-minute speedup is extrapolated. One initial QA invocation used an invalid FFmpeg directory and exercised original-file fallback; it was excluded from the successful-compression comparison, then corrected to the installed current-version binaries.

Decoded original video matched for all900frames; decoded PCM and native caption coverage matched. Raw MP4 hashes differed despite identical decoded images. Lossy recompressed outputs were not pixel/bit exact between independent runs (PSNR44.734772dB), so no exact compressed-output claim is made; their encoding configuration was unchanged.

## Actual metrics and final smoke

Initial candidate browser snapshot: prepare458.2ms,render1076.3ms,capture79.0ms,residual encoder wait3366.2ms,finalize107.3ms,upload155.4ms,commit184.3ms,900submitted frames. These are not pure hardware timings or disjoint whole-export phases. Preparation excludes claim/gate/preflight; completion acquisition is separate.

Final exact-source smoke succeeded with2original acquisitions,compressionAttempted/applied=true and actual logs: completion acquire45ms,source preparation73ms,compression2057ms,replacement finalization104ms,postprocess2246ms. App/Chrome/swap/disk observations were emitted; Chrome and the finite QA server exited. Final full QA run11.396s, not a long-video prediction.

Verification:61focused frontend tests passed;4headless tests rerun after final phase-label change; TypeScript/build/size passed (entry142.6/150KiB,phone179.8/200KiB,CSS33.9/40KiB). Go render/export-step/editor/web/cmd packages passed against isolated local test DB; final affected render/export/web tests passed. Native build/help passed. A wrong relative native-build path after successful frontend gates was corrected by building from repository root.

Evidence outside Git: remote `~/server/qa/viderse-export-perf-20261107/pipeline-{duplicate-before,timing-after,final-smoke}-receipt.json`, `pipeline-decoded-comparison.json`, immutable QA media and request records. No raw private snapshots,coverage,large traces or temporary harness source are committed.

## Operational boundary

See [owning operations guide](../../../apps/viderse/deploy/README.md) and [read-only production host/storage evidence](production-export-host-storage.md). Current provider ranges were155–196MiB/s over32MiB only; internal disk/swap pressure was observed, not proven causal. Historical browser78-minute phase attribution remains unavailable. New measurements are automatic on the next normal export, without auto-running that export.
