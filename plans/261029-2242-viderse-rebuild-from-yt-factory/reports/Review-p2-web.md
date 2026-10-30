# Phase-02 web production-readiness review

## Code Review Summary

### Scope and method
- Compared tracked changes against `HEAD`; treated untracked files as new. Reviewed `apps/viderse/web/src/screens/settings/**`, `screens/setup/**`, `api/**`, the seven requested i18n modules, router/shell/data changes, and the `platform/ui/src/Banner.tsx` diff. References below are repository-relative; `web/` means `apps/viderse/web/`, and `internal/` means `apps/viderse/internal/`.
- Read the structure/performance rules in the rebuild plan, all six implementation reports, design contract §§11/15/23/24/24.1, applicable V7/V9/V17/V22/V23/V24 acceptance criteria, `06-copy.md`, W1/#17 decisions, and Settings/MSettings/Onboard artboard source. Inspected the actual Go handlers and affected services rather than accepting report claims. Consulted the pinned yt-factory `2c201e6` settings-controls source read-only for the port behavior.
- Scouted callers, full-object mutations, task terminal states, query-key dependencies, initial setup state, catalog limits, polling lifecycles, and HTTP middleware before ranking findings.
- **Live-work caveat:** machine setup and shared middleware changed during inspection. The final setup API inspected is `/api/setup/state`, matching `internal/web/setup.go:31-42`, not the account-bootstrap `/api/setup`. The final JSON guard is `platform/webguard/webguard.go:319-327`: POST/PUT always require JSON; bodiless DELETE is allowed. Findings below use those current contracts. No source, plan state, or git state was modified; only this requested report was written.
- No tests, build, typecheck, lint, browser session, network call, or server startup was run. Existing agent verification claims were not rerun. Static findings are not claims of exercised end-to-end behavior.

### Overall assessment
**Do not commit as production-ready.** There are seven high-priority findings in the new web surface, one inherited shared-action blocker newly exercised throughout it, and nine medium-priority findings. No independently confirmed Critical-severity security exploit was found in this scope. The principal failures are real transport incompatibility, an impossible initial setup predicate, lost settings writes, inaccessible catalog selections, and broken storage recovery—not style preferences.

## Critical Issues
None independently confirmed in the inspected web diff. This is not an audit of every backend permission or storage operation.

## High Priority — blocking

### H1 — Bodyless POST mutations are rejected before reaching their Go handlers
**Evidence:** `web/src/api/request.ts:53-60` supplies `Content-Type` only when a body exists. `platform/webapp/server.go:130-133` applies `JSONBodyRequired` before dispatch; `platform/webguard/webguard.go:319-323` requires JSON for every POST. For example, `web/src/api/ai.ts:82-83,95-96,104-105,114-115`, `api/platforms.ts:73-86`, issue bodyless POSTs. The current tools/storage adapters instead use `request(...,{body:{}})` and are not affected.

**Impact:** provider test/login/cancel/logout and platform cookie check/renew/QR creation return HTTP 400 instead of executing. Setup start/retry and mutations already passing an object do not have this defect. Bodiless DELETE is **not** included: the current guard permits it.

**Fix:** make the common mutation transport satisfy the established JSON-write contract, e.g. serialize `{}` and send the JSON header for POST without a supplied body. Consolidate the duplicated request/error machinery rather than fixing each button separately. Do not weaken the origin/content-type security boundary as a workaround.

**Regression test:** execute one bodyless client POST against the actual guarded Go gateway, authenticated, and assert the service is invoked; cover test-provider and QR-create paths, with JSON-bodied tool/storage actions as passing controls. At minimum, fetch-level unit tests must assert method, JSON headers and body, and a server test must enforce the same middleware contract.

### H2 — Fresh-machine setup never starts with the real six-step snapshot
**Evidence:** `web/src/screens/setup/SetupScreen.tsx:102-106` starts only if **every** step is `pending`. `internal/setup/setup.go:143-166` maps the login probe to `needs_user`; `internal/toolsvc/probe.go:222-239` returns `needs_login` until ChatGPT is authenticated. Existing tools can also be `done`. The screen has no other start action (`SetupScreen.tsx:130-165`).

**Impact:** the normal initial snapshot is five pending/done steps plus one needs-user step, so no installation request is sent; the UI nevertheless says installation is running. Mixed preinstalled machines fail the same way. The current test only checks the fabricated all-pending case (`SetupScreen.test.tsx:82-87`).

**Fix:** start/join the idempotent server setup run on the actual initial desktop state, ignoring the separately user-driven login step and already-completed tools; distinguish a never-started state from a failed run that requires explicit retry. Respect disk preflight rather than pretending progress has begun.

**Regression test:** use a real-shaped snapshot with five automatic steps and `codex-login: needs_user`, including one already-done tool; assert one start POST, then SSE-driven progress, and no repeated start on later snapshots.

### H3 — A failed storage move is rendered forever as a running move
**Evidence:** `web/src/screens/settings/storage/StorageSection.tsx:440-441` selects the first `storage_move` task without checking `state`, then disables changing the folder while `moving`. `MoveProgress` in the same file renders moving text and a cancel action. `internal/storage/move.go:284-297` intentionally retains failed moves as `state=failed` with `action.op=retry`. `internal/persistence/postgres/gen/activity.sql.go:253-255` lists all task states; `internal/web/activity.go:144-159` includes them in the historically named `running` array.

**Impact:** after a copy failure, reconnect, or crash reconciliation, storage settings display an operation that is no longer running and offer cancel instead of retry. The retained failed task continues to block the change-folder workflow, including after reload.

**Fix:** interpret `BackgroundTask.state` and `action.op`; only active states lock folder changes. Render a failed-move recovery state, with the appropriate retry/new-attempt behavior and safe dismissal where supported; do not treat the array name as a state guarantee.

**Regression test:** return a failed `storage_move` from `/api/activity`; assert no running/cancel presentation, no indefinite change-folder lock, and a working recovery action. Also test failed-old plus active-new task ordering.

### H4 — Agent auto-save can overwrite an immediately preceding change
**Evidence:** `web/src/screens/settings/agents/AgentsSection.tsx:71-87` accepts overlapping full-object PUTs. Every setter at `:103-109` builds its body from the last fetched `data`. Model picker and drawing buttons remain usable during save (`:119-121,170-176`); only some other controls consume `busy`. Even those controls remain enabled while the awaited request has not yet caused a rerender.

**Impact:** choose a director model, then an artist model before the first refetch: the second PUT contains the old director model and overwrites the first selection. Network completion order can reverse the final result. The saved hint is rendered unconditionally (`:147,211`) despite failure.

**Fix:** serialize full-document saves from one authoritative latest draft, or disable **all** mutation entry points until PUT plus cache refresh settles. Do not construct independent overlapping full documents from an old resource snapshot; show saving/saved/failed truthfully.

**Regression test:** defer the first PUT/refetch, attempt a second change, resolve in both orders, and assert both intended selections persist (or that the second interaction was blocked), with failure feedback instead of a saved claim.

### H5 — Real selected models disappear or acquire fabricated capabilities
**Evidence:** `web/src/screens/settings/agents/AgentsSection.tsx:54-60` calls `listModels(p.id,{})`; `internal/aiconn/aiconn.go:410-412,435-439` defaults that API to 200 entries and caps it at 500. `pickedFrom` in `agents/ModelPicker.tsx:17-29` invents `text` when a selected model is absent from that partial response, then `:65-68` uses those capabilities to decide which image providers exist. `web/src/screens/settings/connect/ModelCatalog.tsx:53-57,70-72` likewise requests only the default page and substitutes returned row count for the provider catalog total.

**Impact:** an already-selected image model outside the first 200 entries vanishes from drawing choices, while an unknown model is wrongly advertised as suitable for text, and a vision model outside the first page is incorrectly disabled for image-description work. The connection catalog reports the truncated page as the entire catalog; its show-more button cannot retrieve model 201. Selected image sources on large OpenRouter catalogs are a normal production case.

**Fix:** expose/reuse authoritative per-selected-model metadata rather than guessing capability; support a bounded, complete search/page flow for large catalogs and preserve the real total. A larger hard-coded limit alone does not solve selected metadata or the 500 cap. Treat unavailable metadata as unknown/loading, not supported capability.

**Regression test:** a provider with 501+ catalog models, selecting an image-only model beyond row 200, must remain correctly selectable for drawing and never acquire invented text capability; a selected vision model beyond the same boundary must remain usable for image descriptions. Assert search can find later entries and displayed totals are not replaced with page length.

### H6 — Non-subscription provider removal deletes configuration immediately, with a logout label
**Evidence:** `web/src/screens/settings/connect/ProviderCard.tsx:241-257` confirms subscription logout, but directly calls `deleteProvider` for API/local providers. Its footer uses the logout label for both cases (`:379-382`; the API DELETE is `api/ai.ts:90-91`).

**Impact:** a button presented as signing out immediately deletes the provider, stored key/base URL and selected-model configuration when the provider is not currently in use. That violates the explicit orange inline confirmation-before-delete rule; backend in-use rejection does not protect unused configuration.

**Fix:** distinguish logout from deletion in label and behavior, and use the existing orange inline confirmation contract before the DELETE, explicitly naming what is lost. Keep backend in-use enforcement.

**Regression test:** click remove on an unused API provider: no DELETE before confirmation; cancel preserves it; confirm performs one DELETE; 409 in-use leaves the card and explains why.

### H7 — ChatGPT authorization URL is opened outside the user gesture, without a usable blocked-popup fallback
**Evidence:** `web/src/screens/settings/connect/ProviderCard.tsx:93-99` calls `window.open(login.authUrl,...)` from a polling-driven effect and immediately records `authOpened=true`; only the device-code branch renders an explicit open button (`:150-154`). `startLogin` waits for HTTP before opening the flow (`:260-265`).

**Impact:** popup-blocking browsers can prevent the authorization window from opening. There is then no ChatGPT browser-flow link/button exposing `authUrl` to the user, so the page waits for authorization that the user cannot reach. This is a browser-policy-dependent production risk, not an exercised browser reproduction.

**Fix:** display an explicit authorization link/button once the URL is available, or reserve a blank window synchronously in the original click and navigate it after the response. Preserve a visible retry/open fallback even when automatic opening succeeds; detect a null window result.

**Regression test:** mock `window.open` returning null and assert a keyboard-accessible action still opens the exact `authUrl`; validate the actual behavior in a popup-blocking browser before landing.

### D1 — Inherited dependency blocker: the shared async-action controller never becomes idle
**Evidence:** `platform/ui/src/useAsyncAction.ts:24-26` sets `busy=true`; its `finally` at `:35-43` clears/delays the spinner but never restores `busy=false` or `pending=false`. New callers include `StorageSection.tsx:139-155` (recheck when mode changes), `ToolsSection.tsx:25-31`, `LogsSection.tsx:53-57`, and `SetupScreen.tsx:88`.

**Impact:** each mounted action runs at most once. A completed or rejected request prevents every retry; actions lasting over 150 ms can remain visibly pending forever. In the folder picker, changing move mode after the first check clears its result but the next `run()` is ignored, preventing continuation. Stored errors are also invisible in callers that never render `action.error`.

**Attribution:** this helper is outside the requested `platform/ui` diff; this is an inherited blocker exposed by the new screens, not a claim that phase-02 changed the helper.

**Fix:** coordinate with the shared-UI owner to reset state in `finally`, dispose both pending/minimum-display timers safely, and have these callers render or handle errors explicitly.

**Regression test:** invoke one controller successfully twice and reject then retry, both below/above 150 ms; assert idle state and actual second invocation. On the picker, switch from move to new-only after the first check and require a fresh check.

## Medium Priority

### M1 — Agent validation errors use different field paths on Go and web
**Evidence:** `internal/aiconn/agents.go:87-107` emits `director.model`, `director.fallback`, `artist.promptModel`, `artist.draw`, `artist.drawFallback`. `web/src/screens/settings/agents/AgentsSection.tsx:145,164,189` only renders errors for `model`/`director`, `promptModel`/`artist`, or `draw`; neither fallback has its own matching error. `AgentsSection.test.tsx:64-68` fabricates `field: draw` instead of the real response.

**Impact:** the server correctly rejects an invalid selection but the UI silently retains the old value while claiming changes apply immediately.

**Fix:** use the actual dotted paths consistently, render a field/fallback-specific message, and preserve the server field for unknown-model errors rather than mapping every failure to director text.

**Regression test:** parameterize real 422 payloads for all five paths; assert the correct nearby error and no saved-success claim.

### M2 — Load-more expands DOM rows past the hard 200-row limit
**Evidence:** `web/src/screens/settings/storage/StorageSection.tsx:55-70,111-113` starts at 100 and increases `windowSize` without a ceiling. Three pages render 300 folder buttons, with later directories inheriting that expanded limit. `connect/ModelCatalog.tsx:49,72,148-151` uses the same expanding-slice technique; manually added models are prepended and can take it past 200 too.

**Impact:** the implementation is incremental rendering, not a bounded window/virtual list; it violates the explicit <=200 DOM-row acceptance criterion on normal 300+ directory listings.

**Fix:** replace-page pagination or actual windowing that keeps <=200 live rows while retaining access to every item. Reset pagination on directory/filter changes; do not just hide or cap unreachable later items.

**Regression test:** return 500 directories, repeatedly load/navigate/search, assert <=200 rendered item rows throughout and that directory 500 remains reachable. Repeat for catalog plus manual entries.

### M3 — Login polling is not bounded by request duration or local expiry
**Evidence:** `web/src/screens/settings/accounts/QrLogin.tsx:59-70` starts a new request every two seconds regardless of an outstanding one; errors are swallowed. The local one-second clock at `:90-96` displays a countdown but does not transition to expired or stop polling when `expiresAt` passes. `connect/ProviderCard.tsx:76-91` has the same overlapping interval pattern and no abort signal.

**Impact:** slow requests accumulate unbounded in-flight work, stale QR waiting responses can overwrite scanned state, and a network failure after QR creation leaves a zero-second waiting UI polling indefinitely. Closing clears future interval ticks, but does not cancel in-flight fetches.

**Fix:** use one request-at-a-time scheduled polling with AbortController cleanup; stop at terminal state/deadline; expose a network/retry state after poll failures rather than swallowing them indefinitely. Keep this specific to the existing login lifecycle, not a new generic polling framework.

**Regression test:** hold one poll promise over several two-second periods and assert one in-flight request; advance beyond `expiresAt` during network failure and assert expired/retry UI and no further polling; close and assert abort/zero later updates.

### M4 — Agent catalog fetching runs during render, with unbounded fan-out and stale private cache
**Evidence:** `web/src/screens/settings/agents/AgentsSection.tsx:54-62` launches HTTP calls from `useMemo` for every provider with a selection. The private Map is retained indefinitely, has no abort lifecycle, and caches an empty list even on failure. SSE invalidation at `web/src/data/eventKeys.ts:18` invalidates the shared `ai` prefix, not this private Map.

**Impact:** render replay/StrictMode can duplicate requests; many providers create an unbounded burst; a temporary error or catalog refresh never updates metadata until remount. This is a second cache convention alongside the existing resource cache and contributes directly to H5.

**Fix:** move side effects out of render; reuse the resource/cache convention, cancel superseded requests, bound any multi-provider loading, and key freshness to authoritative catalog revision/time. Do not retain failed results as permanently loaded catalog data.

**Regression test:** mount under StrictMode, refresh a provider catalog via SSE, and simulate one initial failed load; assert bounded requests, retry recovery, fresh capabilities and abort on unmount.

### M5 — Platform recovery fields/actions exist on the API but are ignored by the cards
**Evidence:** `web/src/screens/settings/accounts/AccountsSection.tsx:172-214` only offers QR/manual replacement for Bilibili. It does not consume `renewable`, `lastRefreshAt` or `refreshError`, and has no clear-login action. The wrappers and handlers exist at `web/src/api/platforms.ts:77-88` and `internal/web/platforms.go:174-221`. YouTube also has no clear/no-login choice (`AccountsSection.tsx:94-130`). MSettings artboard source explicitly defines the renewable saved state and Gia hạn action at `docs/design/v1/artboards/MSettings.dc.html:968-974`.

**Impact:** a saved QR session cannot be manually renewed or cleared from the UI, expiry context is lost, and users cannot disable an existing YouTube cookie source without replacing it. New-login is not equivalent to renew/clear. A failed-refresh banner from the specified states is absent.

**Fix:** render saved/manual/QR/expired states from the existing fields; offer renew only when `renewable`, clear with orange inline confirmation, and retain the explicit unauthenticated YouTube choice. Show refresh failure/time without exposing cookie material.

**Regression test:** real-shaped saved QR, nonrenewable manual cookie, expired saved cookie and refresh-error fixtures; assert exactly the allowed actions/endpoints and no mutation before clear confirmation.

### M6 — SSE invalidation misses visible error/reclaim data and refreshes unrelated data on every task tick
**Evidence:** `web/src/data/eventKeys.ts:18-22` never invalidates `keys.errors()` or the literal `[storage-reclaim]` key. `logs/LogsSection.tsx:19` reads errors once; `storage/StorageSection.tsx:419` reads reclaim estimates once. `internal/activity/errors.go:36-64` derives errors from changing task state. Conversely every activity event invalidates setup and storage whether the task is video, identity, cleanup or storage.

**Impact:** an open log page can continue to say no recent errors after a task fails; post-reclaim/storage changes leave reclaim estimates stale. At up to four progress events per second, unrelated background work needlessly refetches setup/storage. This is named-prefix invalidation, not a global invalidate bug, but its dependency mapping is incomplete and too broad.

**Fix:** add a named reclaim key and map terminal/error-producing and storage-changing events to the actual affected resources, using the event ref/type contract where available. Keep unrelated activity progress away from setup/storage; batch/coalesce only as supported by the existing event layer.

**Regression test:** fail a task while Logs is open and assert errors update; change/reclaim storage and assert estimates refresh; send an unrelated video progress event and assert no setup/storage GET.

### M7 — Product copy and primary-action rules are bypassed at the wire boundary
**Evidence:** `web/src/screens/settings/tools/ToolsSection.tsx:66-80` displays raw `row.hint` instead of the work-oriented local copy. Real hints contain ffmpeg/codex names (`internal/toolsvc/probe.go:129-138,207-217`). `SetupScreen.tsx:55-60` displays `progress.detail/phase` and raw `step.error` outside any Chi tiết disclosure, despite `api/setup.ts:24` documenting the latter as detail-only. `AccountsSection.tsx:218,229-230` renders protocol values `uses/kinds` directly; the actual registry uses values such as `upload`, `rivals`, `backdrops`, `video`, `clip` (`internal/platforms/registry.go:10-30`), not the Vietnamese strings in the account tests. The view can also contain multiple primary actions, e.g. YouTube save plus Bilibili login and manual-cookie save (`AccountsSection.tsx:123,187,205`), and every logged-out provider gets primary styling (`ProviderCard.tsx:321-325`).

**Impact:** the actual backend produces technical names/internal enum copy that the polished mock screenshots omit; diagnostic text escapes the specified detail affordance, and primary-button hierarchy is not enforced per visible view.

**Fix:** map wire enums and known tool states to the approved local vocabulary, keep raw diagnostic names/paths/codes behind Chi tiết, and select one contextual primary action with subordinate alternatives. Explicit storage-location breadcrumbs are required by V22 and its artboard; do not remove those mandated picker/location affordances under a blanket diagnostic-redaction rule.

**Regression test:** render actual handler-shaped tool/platform/setup responses, not translated fixtures; assert no internal use codes/tool diagnostic text in the summary, diagnostics accessible only after disclosure, and at most one enabled primary CTA in representative desktop/mobile states.

### M8 — Several new controls have no accessible name or complete keyboard pattern
**Evidence:** `web/src/screens/settings/agents/AgentsSection.tsx:190-192` puts a text span next to `<Switch>` but supplies neither `label` nor `aria-label`; the shared switch only uses those props (`platform/ui/src/controls.tsx:78`). Model search/manual inputs in `connect/ModelCatalog.tsx:104-109,155-164`, provider edit/code inputs in `connect/ProviderCard.tsx:175-184,333-340`, and AddProvider fields use placeholders/unassociated visible labels. `agents/ModelPicker.tsx:35-74` emits listbox/option roles while focus remains on the trigger, without Arrow/Home/End/selected-option focus handling.

**Impact:** assistive-technology users cannot identify the fallback switch and some inputs; the ARIA listbox promises keyboard behavior it does not implement. Pointer-driven tests do not cover these paths.

**Fix:** pass stable names/IDs and associate labels/help/errors; use the existing select/listbox interaction primitive or complete roving/active-descendant keyboard behavior and focus restoration. Existing shared Dialog/Sheet already handle trapping/restoration; reuse rather than introduce another modal manager.

**Regression test:** query every control by accessible role/name, operate model selection without a pointer, verify Escape/focus restoration, and test the fallback switch with its visible name.

### M9 — W1 setup lifecycle does not implement skip/recovery/mobile guarantees
**Evidence:** `web/src/shell/AppShell.tsx:33-45` marks setup checked **before** its fetch succeeds, and permanently suppresses another check in the tab after error or route-change cancellation. It does not track an explicit user skip. `SetupScreen.tsx:93-106` installs completion/start effects before the mobile branch at `:108`; completion redirects mobile to channel creation after 1.2 seconds. The component never consumes `data.disk` and has no persistent unfinished-setup banner after the back/settings links (`:136-142`). The six-step status is retained; the surrounding state behavior is not.

**Impact:** a transient first-fetch error or quick navigation bypasses onboarding for the entire session; a mobile visitor can be redirected out of the instruction-only state or trigger setup if an all-pending snapshot is supplied. Low-disk/retry context and the required skip-but-keep-warning behavior are missing. The completion timer is correctly cleared on unmount, but is not platform-gated.

**Fix:** record successful decision/explicit skip, not attempted fetch; make the check abortable and retryable. Gate all install/auto-advance side effects to desktop, render disk sufficiency and state-specific copy, and implement the specified unfinished-setup warning while allowing work to continue.

**Regression test:** initial setup fetch fails then recovers; navigate while pending; explicitly skip and verify warning; mobile complete/all-pending snapshots must issue no setup POST and no creation redirect; insufficient disk must render an actionable warning rather than a running headline.

## Low Priority / architecture notes

### L1 — Avoid the second request implementation and remove unused settings scaffolding
**Evidence:** `web/src/api/request.ts:20-50` and `:53-81` duplicate credentials, fetch exception mapping, 401 notification, JSON error parsing and 204 behavior. The disconnected old copy subtree in `web/src/i18n/settings.ts:4-48` and several `toolsCopy`/`logsCopy` strings are not consumed by the new sections, while raw transport text is used instead (M7). `SetupScreen.tsx:21-27` turns a missing known login component/export into an empty component via reflective `import.meta.glob` even though that module is part of this change.

**Fix:** one transport implementation with an explicit method and thin convenience wrappers; use the approved copy or remove obsolete exports after checking callers; use a direct lazy import of the known module and let a missing export fail visibly at build time rather than returning a no-op.

**Regression test:** one shared transport contract test covers GET/POST/PUT/PATCH/DELETE, 401, non-JSON errors and 204; a setup render test requires the real lazy login action, not a mock replacement.

No new oversized frontend interface or shared-UI dependency on app/domain code was found in the Banner diff. The important type-boundary failures are trusting wire assertions, rendering protocol enums as copy, and inventing model capabilities—not a justification for introducing a new manager or domain abstraction. The private catalog cache and duplicate fetch code are concrete parallel conventions that should be removed.

## Client / Go contract parity ledger

| Client surface | Actual handler contract | Review result |
|---|---|---|
| `api/setup.ts` | GET `/api/setup/state`; POST `/api/setup/start` `{}`; POST `/api/setup/retry` `{id}`; 202 + snapshot | Current URLs and mutation bodies match. Initial-state interpretation is wrong (H2). `SetupStep.error` is declared required nullable on web but omitted for an empty Go string (`internal/web/setup.go:68-72`); declare optional. No present crash relies on this difference. |
| `api/tools.ts` | GET list; POST check; POST per-tool install/cancel | Route names, JSON bodies and snapshot/progress fields match; these actions explicitly pass `{}` and do not have H1. Tool display copy must not treat `hint` as an approved summary (M7). |
| `api/ai.ts` providers/models/login | Provider array; selected string IDs; optional quota/catalog timestamp; models return a limited array, default 200/max 500; login phases and optional URL/code fields | Provider/model/login wire shapes generally match. Bodyless actions fail H1; callers assume a complete catalog/selected metadata that the array does not guarantee (H5). Delete wrapper claims `void` but handler returns `{ok:true}` (`internal/web/ai.go:284-287`); currently ignored, so harmless but inaccurately typed. |
| `api/ai.ts` agents | GET/PUT full document; 422 error includes dotted `field`, `need`, `model` | Full-document shape matches. Dotted field mismatch M1; concurrent full-object semantics H4. |
| `api/platforms.ts` | GET registry; GET profiles; PUT login `{browser,profile}` / `{cookie}`; POST check/refresh/QR; DELETE login; GET QR-key poll | Route/body/QR field names match the handlers. POST middleware mismatch H1. Uses/kinds are protocol values, not display copy (M7). Renewable/saved/refresh metadata is declared but UI does not implement corresponding states/actions (M5). Bodyless DELETE is currently permitted. |
| `api/storage.ts` | GET snapshot/reclaim/browse; POST check/move with path/mode; 202 task or done; POST cancel body; POST open/recheck/use-default/previous-delete; POST clean with groups | Reviewed request and response names match. Control POSTs explicitly pass JSON objects and do not have H1. Activity state interpretation breaks failed-move recovery (H3). Reclaim cache is not refreshed on related SSE changes (M6). |
| `api/activity.ts` | `running` contains active **and failed** retained tasks; state/action determine behavior | DTO includes `state` and `action` correctly; StorageSection ignores them (H3). Do not filter by the misleading array name. |
| `api/logs.ts` | limit 200, before cursor, `{lines,nextBefore}`; export zip | Names/filters/cursor match. Page replacement keeps rendered log rows at <=200. No evidence of a new log pagination mismatch. |
| `api/errors.ts` | `{items,count7d}` with human summary fields and optional link | Current declared subset matches. Task changes never invalidate this resource (M6); an empty/error GET is also presented as the no-errors state (`LogsSection.tsx:19-32`) instead of an unavailable state. Fix by rendering the resource error and test a failing errors GET. |
| `api/version.ts` | GET/check `{version,latest,checkedAt}`, POST check `{}` | Current paths/body/fields match; no transport-body defect here. Shared action D1 prevents repeated checks. |

## Edge cases found during scouting / test quality
- Real initial setup has a user-required login step and may contain already-installed tools; the all-pending fixture is not representative (H2).
- A retained failed task is not a running task even if its API array is called `running` (H3).
- Two valid full-document saves are not independent patches (H4).
- Selected models need not occur on the first catalog page; missing metadata is not permission to synthesize capabilities (H5).
- Clearing an interval does not abort an already-running request; one interval is not one in-flight request (M3).
- A namespaced cache prefix is better than wildcard invalidation, but does not prove complete or precise dependencies (M6).
- UI fetch stubs, for example `ToolsSection.test.tsx:30-40`, mock success from URL/method without applying the shared middleware. Acceptance by that mock pattern is not evidence that the Go JSON guard accepts a request; H1 is established specifically by the AI/platform callers and actual middleware. The current tools/storage adapters do send JSON objects. Account tests use Vietnamese `uses` fixtures (`AccountsSection.test.tsx:15-30`) although the server sends protocol codes. Agent tests inject a nonexistent `field:draw`. These are contract-shaped happy-path illusions, not cross-boundary verification.
- The tests are not all phantom: connection tests assert request bodies and preserve selected draft across reload, storage tests assert move bodies and cleanup confirmation ordering, log tests assert cursors/page replacement, and QR tests count poll calls and close cleanup. Their useful coverage does not establish middleware parity, race safety, or real schema fidelity.

## Areas that look correct in the inspected state
- Current setup state URL is separated from account bootstrap. Start/retry use JSON objects and the current handler accepts those route shapes.
- Settings sections are lazy-loaded; setup lazily loads its login UI; App routes are lazy (`web/src/App.tsx:11-22`, `SettingsScreen.tsx:20-27`). No eager settings-to-initial-entry import was found.
- Resource fetches generally propagate AbortSignal through the standard resource adapter; same-origin credentials and centralized 401/session-expiry signaling are retained. No new raw HTML sink was found in these screens. API-key creation uses PasswordField in the final inspected implementation.
- Model/catalog search and directory search debounce at 250 ms and clear their timeout on dependency change/unmount. Log pages replace rather than append, maintaining <=200 log lines. The folder/catalog load-more cases are the exceptions described above.
- Login/QR interval callbacks are cleared on close/unmount; setup completion timer also has cleanup. This is positive lifecycle evidence, not proof that in-flight work is bounded or aborted.
- Logs export uses a real zip endpoint/link. Storage cleanup and previous-copy deletion include the orange inline confirmation and only send the destructive request after confirmation (`StorageSection.tsx:294-320,389-415`). The API-provider removal exception is H6.
- The Banner change remains app-agnostic, keeps the legacy single-action prop usable, provides optional multiple actions, and preserves error alert/status roles. No concrete defect was found in that shared-UI diff itself.
- No unrelated architecture rewrite or new oversized manager was found within this assignment. Component boundaries generally follow existing app/resource/UI patterns except the private catalog loader and duplicated transport noted above.

## Verification / metrics / acceptance status
- **Exercised:** read-only diff/source/spec inspection and gzip byte count of the available prebuilt entry `apps/viderse/internal/web/dist/assets/index-CUksQcy_.js`: **127,196 bytes** using `gzip -c ... | wc -c` (default compression). Its inspected manifest entry has no static JS imports; settings/setup are dynamic chunks. This is below 250 kB for the available JS entry, **not** proof that a fresh build of the moving working tree meets the entire first-load budget or that CSS/network load is included.
- **Not exercised:** unit/integration tests, browser popup behavior, keyboard/a11y audit, build, fresh bundle size, typecheck, lint, coverage. Type/test coverage percentages and lint issue counts are **not measured**, not zero.
- **V7/W1:** six-step presentation/routes are present, but H2 and M9 block the setup behavior acceptance criteria.
- **V9:** connection/agent/settings controls are substantially present, but transport, save ordering, model completeness and error parity block completion.
- **V17:** logs/export/pagination are present; live error freshness, true error boundaries and repeated version checks are not complete.
- **V22:** folder/move/cleanup/offline UI exists; failed-move recovery, repeated checks, DOM-row bounds and stale reclaim state block completion. Backend copy correctness remains the storage reviewer responsibility.
- **V23/V24:** registry-driven cards and QR states exist; actual vocabulary, renew/clear/expiry states, POST parity and bounded polling block completion.
- Plan/checklist state was intentionally not edited. Recommend that the lead retain the affected phase-02 tasks as incomplete until the above behavioral criteria are proven, rather than relying on component-presence or mock-based pass counts.

## Recommended actions
1. Correct the shared POST transport contract and coordinate the inherited async-action reset first; otherwise most UI testing observes mocks rather than real actions.
2. Fix initial setup and failed storage task semantics; serialize agent saves and obtain authoritative selected-model metadata.
3. Add precise schema/guard integration tests, slow/overlapping mutation tests, and terminal-state recovery tests using actual Go wire payloads.
4. Finish renew/clear/expiry UI, bounded list/poll behavior, resource invalidation and accessibility/copy requirements; only then perform the focused tests, guarded integration checks, fresh build/size and browser checks as the main agent.

## Status / Summary
**Status: REVIEW COMPLETE — BLOCK COMMIT.** Seven new-surface High findings, one inherited High dependency blocker, nine Medium findings, and one Low architecture/cleanup note. Correctness and performance claims are limited to the source evidence and the explicitly exercised artifact-size measurement above; the implementation remains in flux. No source or git-state changes were made.
