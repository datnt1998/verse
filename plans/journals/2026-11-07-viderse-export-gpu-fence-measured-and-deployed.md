---
title: viderse-export-gpu-fence-measured-and-deployed
date: 2026-11-07
summary: Deployed pixel-equivalent GPU-fence removal; historical 89-minute slowdown remains unresolved.
---

# viderse-export-gpu-fence-measured-and-deployed

## What happened
Measured the exact historical export revision in isolated QA on Mac mini: real 1080p30, timed captions/coverage, audio, snow and waveform. Both 30s and 300s samples stayed fast; the historical 89-minute slowdown was not reproduced. QA publication is local, not a production Garage/S3 write benchmark.

## Decision
Removed only the redundant GPU completion fence before synchronous Canvas2D consumption. Every decoded frame, PCM sample and native caption-coverage record matched for 30s, 300s, a real crossfade and no-CDP direct Chrome controls. Single-run 2.7–4.1% differences are not a proven stable speedup; memory results are mixed. No static cache, hardware toggle, quality reduction or full 57-minute rerender.

## Verification and deployment
39 engine tests passed with 4 existing skips; typecheck/build/budgets/native help passed. PR #20 merged and deployed main 2b55c0241ee3c394f09dba89099cb132a1a02c9f; native/public asset hashes and process inode matched. Authenticated production workbench video played/paused without error. User index and unrelated edits preserved; benchmark processes stopped.

## Remaining uncertainty
Historical long-export cause and production publication timing remain unresolved. No claim of 89-minute resolution, complete artboard parity or physical Safari proof. Measurement report: plans/261106-1303-viderse-production-artboard-parity/reports/export-real-baseline-20261107.md.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
