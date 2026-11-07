# Export progress bar · 2026-11-07

Status: verified candidate; safe production deployment pending active jobs finishing.

## Cause and repair

The export step's inner fill had a changing width but computed background was transparent/none in the production build. At 10%, the element measured73.8px within a738px track yet painted nothing. The StyleX declaration used the `background` shorthand. Replace it with `backgroundImage`, following the existing setup progress-fill pattern, and move the unchanged green gradient into `--vv-grad-export-progress`.

Only the fill style and its token change. Progress calculation, event subscription, render/encoding quality and jobs are untouched. Existing unrelated working-tree changes in LiveStudio are excluded from the release.

## Verification

TypeScript, production build and bundle size passed. Actual authenticated production job data with locally served candidate assets: caption463→467/1,000, aria value46→47, measured fill/track ratio0.45998→0.47000; computed background is the expected green gradient. The update arrived through the normal application data flow without reloading. Screenshot captured at `/tmp/viderse-progress-bar-fixed.webp`; this is candidate UI against real data, not a deployed or artboard acceptance screenshot.

Initial synthetic replay attempts were discarded: one timed out on a persistent connection, another used the wrong stageGroup. Neither changed server data. Final proof uses real job responses and events, substituting only built static assets in an isolated browser tab. No production job was cancelled, paused, restarted or created for this repair.
