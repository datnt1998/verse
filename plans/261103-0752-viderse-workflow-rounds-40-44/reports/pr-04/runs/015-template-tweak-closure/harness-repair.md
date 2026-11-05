# Template/tweak harness repair

Status: DONE_WITH_CONCERNS

## Changed-line summary

Only `template-tweak-closure/controller.cjs` and this private report were modified. `attempt-1/results.json` and its evidence remain untouched.

- Line 24 and line 439: define `libraryPath(tab, query)` and replace the undefined `library(...)` call for the existing channel/video Library routing case.
- Lines 41, 86–120, 126 and 130: discover existing native cover media using GET-only reads of each existing template's `cover-trials` status and each existing cover prompt's `samples` status, plus existing cover prompt `sampleUrls`. Accept only ready history records and same-origin image URLs; history image paths must match their native record endpoint (with equivalent URL encoding). Decode an actual existing image before selecting it. Report read availability, probed candidates and selected provenance in `native.coverMediaDiscovery`.
- Line 288: scope the selected overlay list and removal controls to `main .vt-sec`, excluding the closing picker duplicate. This repairs the two desktop/mobile strict-locator harness failures.
- Lines 315–322: select a native owner actually present in the dialog, excluding reserved id `new`; use the real accessible name contributed by ChannelAvatar plus `attachGo` (`<name> Mở <name>`) and assert the visible `attachGo` label. This repairs the two desktop/mobile guessed-owner/label failures without writing a default.
- Line 357: use the mobile add-sheet button's complete accessible name, `Mẫu video mới Bắt đầu từ Mẫu chuẩn, sửa từng mục sau`. `MLibraryScreen` supplies the label plus description and `ActionSheet` renders both inside the button.
- Lines 376–393: the existing two cover-ready-history cases now consume discovered, decoded native media. An existing CoverTrial row is preserved in a synthetic GET envelope; an existing PromptSample uses an explicitly synthetic trial wrapper with its real video/prompt identity and existing image URL. Existing prompt artwork remains a last-resort explicitly synthetic history wrapper, never generated or substituted artwork. Report the source kind, read endpoint, record identity, original template when applicable, and media URL. No round45 viewer interaction was added.

## Evidence used

`attempt-1/results.json` records six harness failures: two `.vbd-ovlist` strict-locator failures, two default-picker selection timeouts, the mobile add-sheet guessed accessible name, and the undefined `library` helper. It also records two cover-ready-history UNPROVEN cases caused by only inspecting cover prompt `sampleUrls` rather than existing native histories.

Read source: `AttachChannelDialog.tsx`, `ChannelAvatar.tsx`, `MLibraryScreen.tsx`, shared `platform/ui/src/mobile/sheets.tsx`, library copy, `api/cover-trials.ts`, and the backend `template_trial.go` wire serializers. Native cover/sample histories return `history` rows with ready-state `imageUrl`; GET image endpoints serve existing image bytes. These reads do not authorize a trial start.

## Remaining product versus harness limitations

- Product failures from attempt 1 remain asserted unchanged: catalogue GET errors must render the retryable Library error rather than falsely missing records; missing overlays must show human-readable copy without raw identifiers. Parent reports those source fixes have landed; this task did not rerun them or claim acceptance.
- The six corrected harness failures are not claimed PASS until the parent executes the repaired controller.
- Cover-media discovery is prepared, not executed here. The two cover cases remain UNPROVEN when no same-origin existing cover image decodes, or all relevant native reads are unavailable. The report distinguishes unavailable reads from successful histories containing no eligible media. No fixture creates media to force a PASS.
- A successful cover-ready-history case proves the mounted ready-history consumer displaying existing native image bytes, not that this synthetic envelope is backend-persisted history for the baseline template. Prompt-sample/artwork wrappers are explicitly synthetic. It proves no paid generation, provider call, new trial, successful cancellation or rendered video.
- Existing constraints remain: PATCH is memory-only; generation/cancel/import/source-parser requests are aborted before networking; no real non-GET action or external request is enabled. Import done/partial/native success, archive parsing, named unavailable backdrop identities, rendered preview/playback, pixel-level artboard acceptance, and round45 mobile viewer remain outside this repair.
- No new coverage cases, API mutations, application changes, artboard edits, git operations, credentials, browser/server execution, downloads, tests, build, lint or formatter runs were performed. Parent owns the sequential controller execution and native checks.
