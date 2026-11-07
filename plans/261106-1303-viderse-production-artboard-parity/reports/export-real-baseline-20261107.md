# Real bounded export measurement · 2026-11-07

## Outcome

Faithful 30-second and 300-second exports did **not reproduce the historical 89-minute export**. The candidate preserves every compared decoded frame, audio sample and caption-coverage record. Its approximately2.7% profiled wall-time difference is a single-run observation, not a proven material improvement or an89-minute-export fix. No extrapolation is justified.

Measurements used an isolated QA database/root without restarting production8789 or mutating production project/job/output records. The parent subsequently deployed the verified artifact separately; the rollout proof is below. No full57-minute export or provider generation was run.

## Measurements

| Metric | Baseline30s | Candidate30s | Baseline300s | Candidate300s |
|---|---:|---:|---:|---:|
| Frames | 900 | 900 | 9000 | 9000 |
| CDP navigation → completed page | 7.726s | 7.519s | 46.361s | 45.124s |
| Peak Chrome process-tree RSS | 1,328,944KiB | 1,206,960KiB | 1,863,424KiB | 1,626,624KiB |
| MP4 bytes | 21,471,028 | 21,471,028 | 228,034,971 | 228,034,971 |

Progress was sampled approximately once per second, not instrumented inside each stage. For the300s samples, baseline frames38@2.434s →9000@43.170s represent approximately220fps; candidate frames51@2.205s →8942@41.940s represent approximately224fps. Upload was then observed at44.203–45.353s and43.099–44.109s respectively. These samples show no substantial slowdown or convincing throughput gain.

**Production-style, fresh-profile direct-launch control:** no debugging port, profiler or CDP attachment; actual URL launched directly using the production Chrome flags. Baseline launch→QA terminal was9.576844709s with peakRSS1,419,632KiB. Candidate was9.181467375s with peakRSS1,556,336KiB. This approximately4.1% single-run difference is within a0.5-second status-sampling interval; candidate RSS was higher. It is not evidence of an order-of-magnitude CDP-throttling effect or a stable memory reduction.

Timing conventions: profiled metrics start at `Page.navigate` and end at the first observed completed-page state; direct metrics start immediately after Chrome process launch and end at the first observed QA terminal record. They are **not interchangeable timing origins**. Startup, queued time and intentional post-completion trace-drain retention are excluded from profiled completion time. RSS is the sampled sum across Chrome descendants, not unique physical memory. Backend RSS was not measured.

## Fidelity and storage limitation

The completed original export binds revision `sha256:2f1f2f6caa120d5651ec9db02c8890a4092d9f69ae1b26d084e817ac327a6894`. Its exact retained raw build blob was acquired and SHA-verified before publishing a pointer only in the QA database. Current editorial revision `32b1fb…` differs. Hashing reserialized refreshed project objects is not a valid raw-revision comparison.

QA database `viderse_export_perf_20261107` was created from a read-only production dump. Root `~/server/qa/viderse-export-perf-20261107/` is mode0700. The native render-only gateway has no background production workers. Blob writes/deletes go only to a private local overlay; original media stores are used only for Open/Stat. Baseline static UI is served through read-only GETs to deployed8789; candidate uses `BENCH_DIST`. Actual candidate execution of `client-render-engine-C5MDeRSQ.js` and worker `canvas-render-orchestrator-McwF3HsF.js` was observed in CPU profiles.

**QA output publication is LOCAL, whereas production publication may use Garage/S3.** Consequently these measurements establish renderer/encoder/caption-path behavior, not the cause or duration attribution of the historical78-minute browser-supervision stage, which also includes production upload/publication. Backend production compression/finalization was not benchmarked. Native browser source media uses real read-only production references, but cache and contention conditions may differ.

Every faithful run enables the real clone frontend branch and caption collector. Native backend validation checks revision, caption instances/bounds,1920×1080/30fps and exactly900/9000/60 rendered frames. Full coverage is persisted. A QA-only gate adapter verifies the original fresh/allowed gate and adjusts only the full-duration score policy for the deliberately trimmed sample; it does not bypass rendering, encoding or native caption validation.

Effective item/project duration and audio sourceEnd are bounded. The300s sample crosses the production >=5-minute streaming-audio branch. H.2641080p30, requested10Mbps VBR, AAC48kHz stereo, audio/effects and hardware no-preference remain unchanged. Hardware video-encoder accelerator calls were observed; no hardware toggle was applied.

## Exact validation

- Profiled30s pair: **900/900 decoded YUV420p frame hashes match**, decoded PCM matches, canonical persisted caption coverage matches.
- Profiled300s pair: **9000/9000 frame hashes match**, PCM and canonical coverage match.
- Fresh direct-launch30s pair: **900/900 frame hashes match**, PCM and canonical coverage match. The direct baseline also matches the profiled baseline exactly.
- Real first-scene crossfade: original frame window20820–20879, with the original15-frame fade centered at local30; **60/60 frame hashes, PCM and coverage match**. Snow source phase and audio/waveform source offsets were preserved. Subtitle cue/word times were shifted and clipped at the window prefix; that prefix adaptation is explicit, and transition-interior timing is unchanged.
- No introduced black frames occurred: every candidate frame equals its baseline counterpart; the additional full-black-frame checks found none.
- FFprobe confirms exact1080p30 frame counts and durations30.000000/300.000000/2.000000 seconds, with AAC48kHz stereo. Contact sheets were inspected for visible scene/subtitles/waveform and sampled snow-wrap positions299→300 and599→600.

Decoded PCM SHA256:30s `5289dee7029a9c728a9ab4981ffee4cc5f7b4a63fd0ce33647eaba42b4a14abc`;300s `2b36494fb0a77bf31f2eaa4f9d9f9c332fcae95b7672e42029a4469a405e5699`;boundary `d1dd1d9408f161982087b1427284c49ec80ad98bfc1cb84eb3845cfe9494d6b2`.

Audio levels for direct candidate30s were mean−17.5dB/peak−1.4dB; boundary mean−18.0dB/peak−2.0dB. These are nonsilent real audio, not substituted silence.

## Profiles, eliminated hypotheses and failures

The faithful30s dedicated render-worker trace showed idle4223.5ms, Mediabunny draw71.8ms, caption observe70.5ms, caption flush40.8/38.9ms, compositor56.9ms and renderFrame16.6ms. The complete candidate300s trace showed idle35.537s, caption observe993ms, Mediabunny draw678ms, compositor503ms, compositeToTexture156ms and renderFrame75+54ms. No dominant static-image CPU raster bottleneck was established. GPU service durations overlap/nest and cannot be summed into a wall-time split.

Evidence narrows—but does not identify—the historical cause:
1. Missing clone coverage was an initial invalid benchmark condition; faithful coverage-enabled samples still finish quickly.
2. The correct historical raw revision was recovered; results are not based on the later editorial project.
3. The >=300s audio branch still sustains approximately220fps, without progressive slowdown within this bound.
4. Direct no-CDP fresh-profile controls remain fast, so CDP does not reproduce the proposed order-of-magnitude difference in this sample.
5. Local QA output storage prevents eliminating production upload/publication latency as a historical contributor.

Two early exploratory samples used the wrong revision and omitted clone coverage; they are excluded. An initial boundary fixture with negative item positions never claimed its request and was cancelled by its180s safety bound; no result from it is used. Rebuilding with nonnegative positions and native source offsets produced the successful real crossfade pair. The helper now stops unclaimed requests after15s. A baseline300s full trace did not finish before its initial15s drain cleanup and is not claimed; complete faithful30s and candidate30s/300s traces are retained. Post-completion drain differences are excluded from render timing/RSS samples.

## Artifacts and exact reuse

Remote private root retains MP4, decoded-video frame hashes, native canonical coverage, measurements and request timings under:
- `baseline-30s/`: export `4c9cdb76-ce94-47c3-97ca-5f081b072cf9`.
- `candidate-30s/`: `705d0b87-27f2-4623-bf4d-68ee3c5b11a5`.
- `baseline-300s/`: `14a7e248-4f9f-4996-a601-a568a847bb09`.
- `candidate-300s/`: `3fa14f75-3721-47df-a053-64f503dd765a`.
- `baseline-direct-30s/`: `2ca4737a-d4ef-4e7d-bb34-8412332ab328`.
- `candidate-direct-30s/`: `ff9b6f4c-4b1f-4990-89a0-96432a44c9f0`.
- `baseline-boundary/`: `13bf7320-2d5f-49c5-9691-fcc1eb400e15`.
- `candidate-boundary/`: `3f86772c-046c-4a33-806d-0a59b32702cd`.

Safe comparison/receipt JSON files are at the remote root as `<candidate-label>-comparison.json` and, for finalized controls/boundary, `<candidate-label>-receipt.json`.

Local giant traces and private generated evidence are sealed outside Git worktrees at `/tmp/viderse-export-perf-artifacts-20261107/`, mode0700. Temporary helper source is archived there under `harness-source/`, not retained in the clean source tree. Do not print or commit raw traces, launch credentials, private project snapshots or full coverage.

Serial remote interface:
```sh
BENCH_FRAMES=900 python3 ~/server/qa/viderse-export-perf-20261107/prepare.py
# Use9000 for300s; boundary.py prepares the real60-frame scene window.
BENCH_DIST="$HOME/server/qa/viderse-export-perf-20261107/candidate-dist" \
  ~/server/qa/viderse-export-perf-20261107/run-remote.sh
# Baseline originally proxied then-deployed3f6b assets. After this rollout,
# omitting BENCH_DIST serves the candidate, NOT the historical baseline.
# Repeating a baseline needs static assets from the pinned3f6b artifact.
BENCH_DIRECT=1 BENCH_PROFILE_NAME=unique-owned-profile \
  BENCH_MAX_WALL=180 BENCH_TRACE_DRAIN=0 \
  python3 ~/server/qa/viderse-export-perf-20261107/browser.py
```
For profiling, omit BENCH_DIRECT, retain60s drain, forward local18048→remote8048, privately transfer `launch.private.json` next to the sealed local `profile.cjs`, and run that client concurrently with the browser. `validate.py` probes/decodes completed output. `archive.py <label> <exportId>` retains all frame hashes; `compare.py <baselineLabel> <candidateLabel>` compares every frame/PCM/coverage. `finish.py <candidateLabel> <baselineLabel>` creates a compact receipt. Never overlap actual render measurements.

## Cleanup and remaining uncertainty

All measured Chrome processes exited, including final PID23067. The final remote gateway PID23034 and owned SSH CDP tunnel were stopped;8047/8048/local18048 have no intentionally retained benchmark listeners. Owned Chrome profiles were removed. Private QA artifacts remain outside the repository for evidence. The later production restart belongs only to the pinned rollout below.

The historical long-export root cause remains unresolved. Repeated direct-launch measurements with production-equivalent QA storage and explicit render-versus-publication timing would be needed for causal attribution. The candidate is pixel/audio/coverage-equivalent over all exercised windows, but this work does not establish a meaningful speed improvement or warrant a historical89-minute fix claim.

## Production rollout · 2026-11-07

- PR#20 merged main `2b55c0241ee3c394f09dba89099cb132a1a02c9f`; merged tree exactly matches tested candidate `21982d4220480fbb629028f08d7e9aabbb0f9495`.
- Native SHA256 `353c10e2718a88e924f7785f75e01cfdeb979332c93fbeafb5f8a7f8addde0d8`; installed/process inode3633109,PID23940; schema41 unchanged. Pinned deploy rehearsed migrations against restored backup before writer reopen.
- Public `/readyz` returned `{"ok":true}`. Public `assets/client-render-engine-C5MDeRSQ.js` SHA256 `e8ce83b02fe8add26eb4c23f95d1065bee21fdb881c2955b29eaee1e7d4e8894` equals tested build.
- Actual authenticated production workbench opened; existing1920×1080 video played and paused without media error. [Production surface](gpu-export-production-smoke.webp). This smoke does not rerender57minutes or certify artboard/mobile/Safari parity.
- 39 focused engine tests passed,4 pre-existing skips; TypeScript, production build, size budgets and native build/help passed. GitHub reported no registered PR checks; no CI bypass used.
- Backup: `~/server/apps/viderse/releases/2b55c0241ee3c394f09dba89099cb132a1a02c9f/rollout-20261107T092007Z`; restore DB `viderse_release_20261107092008`. No identity/media migration or quality-setting change.
