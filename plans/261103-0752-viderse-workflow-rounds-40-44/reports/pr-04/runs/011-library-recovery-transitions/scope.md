# V38 library recovery companion — prepared, not executed

## Owner execution

Run sequentially against the parent-rebuilt read-only QA application:

```sh
node /var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v38-acceptance-xxkx9151/parallel-acceptance-20261104/library-recovery-proof/library-recovery-proof.cjs
```

Prerequisites reuse the original controller: local application at `http://127.0.0.1:8043`, private cookie file already referenced by that controller, installed repository `@playwright/test`, and installed Google Chrome. Cookie contents are never written into the report. No browser, test, build, lint, formatter, or syntax-check command was run during preparation. There are no execution results or claimed passes yet.

The standalone controller writes `results.json` and screenshots only beside itself. It does not import, overwrite, or execute the original main controller. Parent acceptance, not controller preparation, determines product status. Repeated execution replaces this companion's own results/screenshots.

## Planned matrix: 19 cases

One real authenticated template-list/detail prerequisite discovers the native template ID, recipe and revision dynamically. No hard-coded fixture ID or fabricated baseline is used.

Each of desktop 1440×900 and mobile 390×844 then exercises:

1. **Failed save and explicit retry** for synthetic HTTP 500, HTTP 400 without field-specific errors, and network abort. Hold the first recipe PATCH; edit a different placement; queue a name through the actual desktop blur/mobile rename flow; reject the original request. Verify draft name and newest recipe remain, no optimistic success is accepted, and explicit retry sends both the complete newest recipe and name at the original revision. Accept retry only in memory and check restored consumers.
2. **Serialized latest-draft queue.** Hold the first successful PATCH, queue a distinct newest recipe and name (desktop also queues a later name), then hold the second response. Require exactly two PATCHes, no concurrent pending request, newest name/recipe still in the rendered form despite the older response, second request using the newly accepted revision, and final clean save state. The two placement edits differ from each other and the native baseline; this is not a wording-only assertion.
3. **Conflict → native reload**, both immediate success and synthetic failed GET followed by actual GET success. Preserve local name, recipe and conflict lock through failure; require another usable reload action. Successful reload must actually read the native endpoint, replace both local values, clear the conflict queue, and unlock a new synthetic edit using the actual recovery revision without leaking the abandoned name or recipe. Mobile recipe checked state is read under the rename dialog as well as after dismissal; the test does not discard the dialog to manufacture preservation.
4. **Detail transient HTTP 500/network error → actual resource refresh.** Require generic Vietnamese load-failure treatment, no false not-found, no raw internal marker/code, a retry control, and editor restoration after its click produces an actual successful native detail GET. Recovery is never implemented with a controller-injected fetch, page reload, remount, synthetic recovery payload or URL reset. Classification/privacy issues are retained as failures even if recovery succeeds.
5. **Detail synthetic 404.** Require distinct Vietnamese not-found with back-to-library navigation, no transient retry, no internal payload/code and no fallback editor. This intentionally does not claim native 404 backend proof or retry recovery.

The parent reports both screen branches now implement the same intended generic-versus-404 contract; this companion encodes that contract without exempting mobile.

## Native versus synthetic evidence

- Native evidence: initial list/detail GETs and actual conflict/transient recovery detail responses. Response identifiers/revisions and request actions are recorded; full recipes, credentials and response headers are not dumped.
- Synthetic evidence: every PATCH rejection/acceptance; all injected detail/reload errors. Accepted PATCHes update only the controller's in-memory response object. They do not prove backend validation, save persistence, normalized server behavior or another client's state.
- Exact complete PATCH bodies are compared in memory against independently derived expected recipes; request receipts retain only revision/presence/placement summaries, not full recipe payloads.
- Each case records its proof label, assertions, outcome, screenshots and failure reason. Product/runtime errors and route errors remain concerns rather than being swallowed as passes.

## Request and download safety

- Context-wide route fence covers every page and popup; service workers are blocked. Every non-GET is aborted except an exact PATCH to the discovered native template path, which can only be fulfilled or aborted synthetically. No mutation is continued, fetched or forwarded.
- External origins are aborted. WebSocket connections are closed when supported by installed Playwright. Popups close; download events are cancelled and counted; browser downloads are disabled.
- GET API requests use an explicit read allowlist. Export/download/discovery/follow/parse paths, media resource requests and common archive/media extensions are fenced. Unlisted APIs are blocked, including projects/media and provider endpoints. No preview build, provider inference, source parser or metadata download action is clicked.
- The final safety receipt records actual continuation-boundary mutation/external counters plus synthetic PATCH, native detail GET, blocked mutation/external/media/unlisted-API counts. A receipt and browser-close outcome are written in `finally`, including fatal prerequisite failures. Every deliberately held response releases during case cleanup; browser closure runs in outer `finally`.

## Limits

No actual backend mutation success/persisted readback; no paid image generation, actual preview building/inference, source metadata parsing/download, exports, production writes or deployment. No all-four-section save propagation, remount persistence, viewport-breakpoint transition, real multi-client conflict, player continuity, media rendering, visual design score, comprehensive accessibility or cross-browser certification. The local GET allowlist may expose a new application read dependency as a blocked request; inspect the receipt rather than weakening mutation/download fences.
