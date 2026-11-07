# Export progress bar · 2026-11-07

Status: deployed to https://vid.aarverse.studio at main `ae9d628374bc31d180dfa9c124b9a36611a152ef` (PR#24). No active work was interrupted.

## Cause and repair

The export step's inner fill had a changing width but computed background was transparent/none in the production build. At 10%, the element measured73.8px within a738px track yet painted nothing. The StyleX declaration used the `background` shorthand. Replace it with `backgroundImage`, following the existing setup progress-fill pattern, and move the unchanged green gradient into `--vv-grad-export-progress`.

Only the fill style and its token change. Progress calculation, event subscription, render/encoding quality and jobs are untouched. Existing unrelated working-tree changes in LiveStudio are excluded from the release.

## Verification

TypeScript, production build and bundle size passed. Actual authenticated production job data with locally served candidate assets: caption463→467/1,000, aria value46→47, measured fill/track ratio0.45998→0.47000; computed background is the expected green gradient. The update arrived through the normal application data flow without reloading. Screenshot captured at `/tmp/viderse-progress-bar-fixed.webp`; this is candidate UI against real data, not a deployed or artboard acceptance screenshot.

Initial synthetic replay attempts were discarded: one timed out on a persistent connection, another used the wrong stageGroup. Neither changed server data. Final proof uses real job responses and events, substituting only built static assets in an isolated browser tab. No production job was cancelled, paused, restarted or created for this repair.

## Deployed verification

Eight existing workbench tests passed; native build/help passed. Safe deployment completed at2026-11-07T14:07:24Z with backup/restore rehearsal and schema41 unchanged. Commit marker and installed binary SHA256 `40cf5984a7ae6337c2392f3f41ee935ad1cee75f1a51f8c0e78ff5fd55be2ea4` match. Public readiness200 and served `index-Bqri-NXs.js` SHA256 `4715789ee5556e99073d5a60e0028f118eb05806b18a06ccdc1d83b10327a090` match the tested build.

The real export had already failed at2026-11-07T14:05:59Z, before rollout, with a browser storage-quota error at750/1,000. No automatic retry or storage deletion was performed. Since no running export remained, deployed UI verification used isolated GET/SSE response replay against unmodified production assets:377→600/1,000 changed aria38→60 and measured fill ratio0.38000→0.60000 without reload; computed gradient stayed visible. This is a UI regression smoke, not a successful export or artboard acceptance. Browser tabs and the owned local preview service were closed.
