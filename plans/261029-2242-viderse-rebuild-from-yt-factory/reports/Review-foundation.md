# Foundation production-readiness review

## Code Review Summary

### Scope

- Comparison: **all uncommitted changes against `HEAD`**, including untracked source files as additions. Enumerated with `git status --short`; inspected `git diff HEAD -- <paths>`, not just the staged diff.
- Reviewed: `platform/events/{hub,serve,hub_test}.go`; Noverse `internal/entry/web` SSE adapter, watch/live-frame paths, producers, transport seam and changed tests; `platform/ui/src/data/`, kit additions and their export/build wiring; Noverse data extraction and changed consumers; Viderse `internal/{status,domain,web,persistence,bootstrap}`, `cmd/viderse`, and web router, session/request integration, data layer, shell, components, confirmation flows, route modules and design-rule checks.
- Tracked scope statistics from the initial enumeration: **93 files, +1,498 / -2,434 lines**. These statistics exclude generated `dist/` and do **not** count new untracked files; the new hub, data core, status service, schema, and web modules were also read as source.
- Excluded: generated `dist/` bundles and business implementation in `internal/{youtube,tools,imagegen}`. Persistence receipt SQL/store files were inspected only at their persistence boundary; this is not an end-to-end image-generation review. Design documents/plans were not audited, except the requested intent reports, structure rules, foundation acceptance criteria, and `/api/status` contract used as review authority.
- Protocol: scout first, then blocking trust-boundary/concurrency pass, then informational maintainability/contract pass; applied base, API and web review checklists.
- Scout map: watch changes now cross three separately locked operations; live deltas append rather than replace in the consumer; shared fetch SSE retains Noverse watchdog timing; Viderse session probes do not update session context; reconnect has a second uncancelled status request; icon rendering is duplicated alongside the kit implementation.

### Overall Assessment

**Do not commit this foundation as production-ready yet.** The ordinary SSE frame/replay extraction and content-based ETag implementation look correct by inspection, but the watch-backfill cutover loses authorization atomicity and frame ordering. Viderse also misses its bounded offline-detection requirement and gets stuck after session expiration. The defects below are source-backed; the failure interleavings are **[INFERENCE] from inspected code, not runtime reproductions**.

No source files, plans, task states, database state, or Git state were changed by this review. Only this requested report was written. No builds, tests, linters, formatters, smoke checks or servers were run; implementation-report test claims are not independent review evidence.

## Critical Issues — blocking

### R1 · [P1] Watch backfill can disclose story content after read permission is removed

**Evidence:** `apps/noverse/internal/entry/web/events.go:343-363`; `platform/events/hub.go:212-225`; permission refresh at `apps/noverse/internal/entry/web/events.go:406-416` and `events_producers.go:61-73`.

`SetWatch` checks `watchVisible` while `WithSub` holds the hub lock, then releases that lock, reads the backlog, and sends through `TrySend`. `TrySend` checks only whether the subscriber exists and has buffer capacity; it does not apply the visibility callback. A concurrent permission downgrade can replace the principal through `RefreshUser` between the check and send without removing the subscriber. The subsequent direct send still delivers the now-forbidden story event history and streamed text. This is a regression: the removed `SetWatch` held the same hub mutex through the permission check, backlog read and enqueue.

**Fix:** Make targeted backfill enqueue apply current visibility under the hub lock, and enqueue the backfill as one authorized batch. Do not regard a username match or a still-existing subscriber as proof of current story access. Preserve a consistent lock order when accessing the live backlog.

**Required regression:** Pause backfill after its initial watch check, downgrade that user without disabling the account, then resume. Assert that no forbidden `events`, `clear`, or `delta` frame is enqueued after the downgrade.

## High Priority — blocking

### R2 · [P1] Live frames can interleave inside watch backfill and corrupt streamed text

**Evidence:** `apps/noverse/internal/entry/web/events.go:146-170,341-364`; `platform/events/hub.go:115-118,212-225`; append/clear semantics in `apps/noverse/web/src/api/useStoryEvents.ts:110-148`.

Backlog capture and each individual `TrySend` are now separate critical sections. For example, capture tail `A`, enqueue its `clear`, let the worker broadcast new delta `B`, then enqueue the captured delta `A`. The client appends those deltas and renders `BA`, not `AB`. A broadcast between backlog capture and `clear` can instead be erased by that clear. The former implementation held the hub lock for the entire backlog batch, so a live broadcast could not split it.

**Fix:** Restore an atomic boundary between capturing/enqueuing the complete backfill and live fan-out; use the same batch operation as R1 rather than independent sends. Cover the boundary between snapshot capture and the first enqueue as well as the gap between frames.

**Required regression:** Coordinate a worker delta with the watch-change backfill and assert the final reconstructed text and exact ordering, not just the presence of three backfill frames. Existing `TestSetWatchBackfillsLiveTail` exercises only the sequential case (`events_resume_test.go:204-226`).

### R3 · [P1] Viderse can remain writable and show no offline strip for roughly 45–50 seconds

**Evidence:** `platform/ui/src/data/events.ts:220-246`; `apps/viderse/web/src/data/events.ts:11-21,33-40`; `apps/viderse/web/src/data/status.ts:17-19`; `apps/viderse/web/src/shell/AppShell.tsx:21-26`. Acceptance: `plans/261029-2242-viderse-rebuild-from-yt-factory/phase-01-foundation.md:82-84,131`.

The Viderse client reuses the shared 45-second no-byte watchdog, checked every five seconds. It has no independent browser-offline signal or bounded server-reachability check. If a network change leaves `fetch`/`reader.read()` pending instead of producing an immediate error, SSE remains `open`, fallback polling remains off, `useOnline()` remains true, and the strip stays hidden until the watchdog fires. A stalled initial connection is also hidden while the state is `connecting`. Rendering the strip promptly after an error does not establish the required **≤5 seconds from loss of connectivity**.

**Fix:** Give Viderse an app-owned bounded reachability signal and handle browser offline/online transitions, using it both for the strip and write gating. Keep Noverse transport behavior stable. Simply reducing the shared watchdog to five seconds would be wrong while the server heartbeat interval is fifteen seconds.

**Required regression:** Test a stream that sends `hello` and then never resolves another read, plus a connection whose headers never arrive. Verify the visible strip and disabled write state within five seconds, then recovery when connectivity returns. Existing shared stall tests intentionally permit the longer Noverse timing (`platform/ui/src/data/events.stall.test.ts:24-38`).

### R4 · [P1] An expired/revoked session leaves Viderse in a permanently stopped authenticated shell

**Evidence:** `apps/viderse/web/src/data/events.ts:14-20,33-40`; `platform/ui/src/data/events.ts:144-168,183-185,263-268`; `apps/viderse/web/src/api/request.ts:33-44`; `apps/viderse/web/src/api/session.tsx:40-64`; `apps/viderse/web/src/App.tsx:54-78`.

The SSE 401 callback calls `me()` and discards its rejection. Neither that call nor the reconnect probe updates `SessionProvider.user`; the request helper only throws `ApiError`. The shared client then stops on 401, but Root still sees the old user and continues displaying the authenticated shell. The poll loop continues requesting status because `stopped` counts as down. The strip offers retry, but `retryNow()` explicitly does nothing once the client is no longer running. A page reload is the only existing route back through session bootstrap.

**Fix:** Wire SSE/probe unauthorized results to the app session-expiration path: clear session state and user-scoped cache, stop data work, and return to the login gate. Keep this app policy outside `@noverse/ui/data`; a plain `/api/me` request is not a session-state transition.

**Required regression:** Expire/revoke a session during an open stream, return 401 on reconnect, and assert login is shown, polling stops, and cached authenticated content is cleared without a reload.

## Medium Priority — non-blocking review findings

### R5 · [P2] Reconnect bypasses cache cancellation and can overwrite fresher status

**Evidence:** `apps/viderse/web/src/data/events.ts:58,66-69,75-82`; `platform/ui/src/data/cache.ts:258-266`.

The reconnect callback already calls `reset()`, which invalidates active resources, but then starts a second raw `fetchStatus()` without an abort signal or lifecycle/generation guard. A later SSE invalidation can fetch a newer snapshot; when the older raw request finally resolves, `cache.setData` writes the old snapshot and aborts any newer cache-managed request. With SSE open there is no periodic status refresh to repair it. The same uncancelled callback can populate the cache after logout/cleanup.

**Fix:** Remove the redundant raw request and let the cache own the fetch, cancellation and stale-response ordering. If an imperative fetch is necessary, use the existing cache-managed API and ensure it cannot write after session cleanup.

**Suggested regression:** Resolve reconnect/status requests out of order, and resolve a pending request after cleanup. Assert no stale overwrite or post-session cache write.

### R6 · [P2] Query values are decoded twice and malformed percent input crashes the root router

**Evidence:** `apps/viderse/web/src/router.ts:44,48-49,56-58,68,84`; `apps/viderse/web/src/App.tsx:54-55`.

`URLSearchParams.get()` already returns decoded text; `text()` passes it through `decodeURIComponent` again without a catch. A URL such as `#/?channel=%25` produces a literal percent sign and throws `URIError` during Root rendering. A doubly encoded identifier is also silently changed, so parse/path round trips are not stable. The guarded decoding for path segments does not protect query values.

**Fix:** Return the nonempty value directly from `text()`; decode query strings exactly once. Add cases for `%`, malformed escape sequences, encoded percent signs, Unicode, and double-encoded values to the router tests.

### R7 · [P2] The running-channel deletion guard is explanatory text, not a guard

**Evidence:** `apps/viderse/web/src/components/confirm.tsx:134-165`; kit enable/run conditions in `platform/ui/src/ConfirmDialog.tsx:220-244,291`; test only covers the idle case at `apps/viderse/web/src/components/confirm.test.tsx:89-106`.

`runningCount > 0` only selects the warning paragraph. After typing the channel name, the shared dialog enables its destructive button and calls the supplied `onConfirm` even though the channel still has running videos. That contradicts the helper contract and the V12 behavior claimed by the implementation report. This is not evidence of an existing backend deletion endpoint bypass: those business endpoints are not present in this foundation.

**Fix:** Represent an actual disabled condition/reason in the kit dialog and enforce it in activation/hold/keyboard paths, then pass the running-channel state from the Viderse helper. The future backend must independently enforce the same business invariant.

**Suggested regression:** With `runningCount: 2`, enter the exact name and assert that neither mouse nor keyboard activation invokes the callback. Retain the idle-path test.

### R8 · [P2] Toast cards never receive their card layout

**Evidence:** `platform/ui/src/Toast.tsx:33-39,46-51,151,155-167`; `platform/ui/src/kitAdditions.test.tsx:44-61`.

The new `sx.toastCard` style defines the promised 300px column layout, but `Notification` always composes `sx.toast`, including for `item.card`. Card title, text and action therefore remain children of the ordinary horizontal pill with a 999px radius; the card stylesheet is unused. The test named for the 300px card checks text/buttons/lifetime but never checks its layout.

**Fix:** Select the card or pill base style using `item.card`, retaining the tone/animation composition. Add a style-aware or browser assertion for width, flex direction and shape, not just static text presence.

### R9 · [P2] Channel selection closes the menu without changing the route

**Evidence:** `apps/viderse/web/src/shell/ChannelSwitcher.tsx:23-37,73-85`; `apps/viderse/web/src/shell/AppHeader.tsx:34`; `apps/viderse/web/src/shell/MobileShell.tsx:75`.

`pick()` only invokes an optional `onPick`; neither real shell passes that callback, and there is no default navigation. Choosing a channel, or choosing “Mọi kênh” from a channel-scoped deep link, only closes the menu. This remains a concrete wiring defect even though the channel-list API is intentionally deferred.

**Fix:** Wire both shells to typed route navigation, or make the callback required with a consistent default routing policy. Test the route/hash after selection rather than only rendering the menu.

### R10 · [P2] The new app recreates kit icon infrastructure and bypasses its validation

**Evidence:** `apps/viderse/web/src/components/VIcon.tsx:6-10,25-50`; existing implementation at `platform/ui/src/Icon.tsx:11-15,20-25,57-82`; new kit icon names in `platform/ui/src/iconNames.ts:2-3`; structure rule at `plan.md:179-181`.

Viderse builds a second eager SVG registry, name type, mask renderer, fallback renderer and missing-icon logger even though the kit already owns that machinery. Some of the same newly introduced SVGs now exist in both app and kit. This is not an app-specific composition like the assistant-to-orb mapping; it is parallel implementation of a common component that the structure rules prohibit. The kit and app can now disagree on valid names and missing-icon behavior.

**Fix:** Put the required icons in the kit, use its `Icon`/`hasIcon`, migrate Viderse callers and remove the redundant registry/assets/exports. If design-facing names need mapping, keep only a small app name mapping, not a second renderer.

### R11 · [P2] Fallback status polling continues in hidden tabs

**Evidence:** `apps/viderse/web/src/data/events.ts:33-40,71-74`; visibility condition in `/api/status` contract at `apps/viderse/docs/design/v1/07-data-contract.md:131-145`.

The fallback interval is gated only by SSE state, not document visibility. During an outage, every still-open tab continues status requests at the configured interval even while hidden, contrary to the visible-tab polling contract. There is no visibility listener to suspend or resume this loop.

**Fix:** Start fallback polling only while the tab is visible and SSE is down; stop it while hidden and refresh when the tab becomes visible again. Preserve the accepted phase-01 decision to poll only as an SSE fallback: the absence of polling while SSE is healthy is **not** reported as a defect here.

## Low Priority / structure-rule follow-ups

These are source-verified rule/completeness gaps, not additional security blockers:

| Gap | Evidence | Recommended action |
|---|---|---|
| Token discipline is not enforced by the new guard | Hardcoded radii/spacing in `apps/viderse/web/src/styles/components.css:7-8,17,23,28,60,66`; the test at `apps/viderse/web/src/checks/design-rules.test.ts:35-44` only matches camel-case `borderRadius` followed by a `px` string, missing CSS `border-radius` and numeric JSX values. | Move dimensions covered by the structure rule to tokens and make the guard recognize both CSS and JSX forms. This is a real acceptance gap, not a request to restyle the design. |
| Shared route presentation belongs outside a route module | `screens/review/ReviewScreen.tsx:19-24` exports `ScreenEmpty`; queue/channel/editor/library/new/explore/settings import that route module, e.g. `screens/queue/QueueScreen.tsx:6`. | Move the app composition to `components/` so replacing the review screen does not change unrelated route dependencies. Keep using the kit `EmptyState`. |
| Account route remains eager | `apps/viderse/web/src/App.tsx:12,46`, unlike the lazy route imports at lines 18-27. | Lazy-load the account route to meet the stated per-route chunk rule. Auth gate components may remain eager. |
| Obsolete desktop menu remains after cutover | `apps/viderse/web/src/shell/AccountMenu.tsx:1-9` has no remaining import in the source search; the new header ends with the settings link at `AppHeader.tsx:38-49`. | Remove the unused menu if deliberately replaced. Ensure a deliberate desktop account/logout entry remains reachable; the mobile shell still has an explicit account link. |
| Composition root is still split | `apps/viderse/cmd/viderse/serve.go:64-68` constructs hub/store/service/gateway; `apps/viderse/internal/bootstrap/bootstrap.go:23-31` retains process-global auth configuration in `init()`. | Consolidate assembly in bootstrap and invoke auth configuration explicitly from the executable. The `init()` behavior predates this diff, so this is a retained rule violation, not a newly introduced race. |
| Status service and wire shape are still coupled | `apps/viderse/internal/status/status.go:36-54` defines JSON-tagged service types; `apps/viderse/internal/web/gateway.go:63-73` marshals the service snapshot directly. | Keep the status wire DTO/mapping in web, as required by the structure rule, rather than making service evolution change the external contract implicitly. |

## Areas that look correct by inspection

- **Noverse SSE frame bytes and ordinary replay/reset:** the extracted serializer preserves `id: N`, `event: TYPE`, `data: JSON`, blank-line framing, no-id hello/reset/live frames, `: ping\n\n`, 500-event ring and 15-second ping. `ResumeID` retains header precedence and query fallback (`platform/events/serve.go:16-38,89-149`; `hub.go:94-103,141-157`). A resume ID beyond the current sequence resets, including after restart. No separate generic Noverse hub/ring implementation remains.
- **Normal visibility/auth filtering:** fan-out and replay use the same visibility callback (`platform/events/hub.go:122-129,148-151`). The Noverse adapter retains disabled-user, private-user/admin, story and project rules (`apps/noverse/internal/entry/web/events.go:56-68,107-115`). R1 is specifically the new direct-backfill exception, not a claim that ordinary Publish/Broadcast/replay is unfiltered.
- **Channel-close and principal synchronization:** sends/removal/close in fan-out, `Update`, `TrySend`, `Unsubscribe` and `Close` share the hub mutex (`platform/events/hub.go:122-134,162-169,196-225,235-242`); no send-versus-close panic path was found in those paths. Watch callbacks run after hub lock release, avoiding re-entry through watcher scans (`apps/noverse/internal/entry/web/events.go:365,393-403`; `watch.go:79-85`). This does not repair R1/R2 atomicity.
- **Noverse session revocation behavior:** the adapter still checks the actual cookie after `session.revoked` and on each ping, and still prunes jobs on the tick (`apps/noverse/internal/entry/web/events.go:503-513,520-528`). Full stream revocation timing was not rerun.
- **Shared TS data cutover:** keyed-cache behavior, SSE backoff/resume and unauthorized callback policy remain in one pure-TS shared subpath; app-specific event/key/watch wiring remains in the app. The removed Noverse cache/transport bodies are not retained as a second implementation. Existing app-facing data exports are intentional composition, not a duplicate transport (`platform/ui/src/data/index.ts:1-9`; `apps/noverse/web/src/data/events.ts:13-24`; `apps/noverse/web/src/data/index.ts:1-11`).
- **ETag/304:** status is rendered before comparison, the strong ETag hashes the exact returned JSON bytes, and matching supports strong, weak, list and wildcard forms. A match returns before writing the body; a restart or an un-bumped setting change cannot reuse an unrelated revision-only ETag (`apps/viderse/internal/web/gateway.go:63-106`). Corresponding content-change/304 tests exist (`gateway_test.go:174-244`), but were not executed in this review.
- **Authentication on the new HTTP endpoints:** both new routes register through authenticated `webapp.Handle`; cookie authentication occurs before application dispatch (`apps/viderse/internal/web/gateway.go:52-53`; `platform/webapp/server.go:140-175`). Viderse SSE revalidates its cookie on each tick (`gateway.go:112-131`). No missing-auth route was found. This does not imply the web client handles expired auth correctly; see R4.
- **Settings persistence/query efficiency:** settings reads use the primary key, writes use a parameterized atomic upsert, missing/wrong-type pause settings follow the documented false default, and DB errors are propagated (`queries/settings.sql:3-12`; `settings.go:30-78`; migration `00001_base.sql:11-15`). No N+1 settings path was found. Empty status attention is an array, not null (`status/status.go:118-135`). Zero running/queued/review counts with no registered sources are the current foundation state, not fabricated jobs.
- **Additive kit contracts:** quiet button and agent banner branches are additive; `ConfirmDialog agent=true` still selects writer, while explicit agent kinds select their orb (`platform/ui/src/Button.tsx:85-99`; `Banner.tsx:44-48`; `ConfirmDialog.tsx:276-279`). Toast-card persistence and action-error handling are present; its layout selection is the distinct R8 defect.
- **Explicitly accepted cutover:** removal of Writer APIs/configuration and replacement of the initial schema are deliberate foundation decisions, not silently introduced compatibility findings. Old Writer identifiers were not found in the scoped runtime/source search. Existing databases still require the planned reset/cutover procedure; that external procedure was not exercised here.

## Edge Cases Found by Scout

1. Permission downgrade that keeps the subscriber alive while watch backfill is pending (R1).
2. Worker delta arriving between backfill clear and captured tail (R2).
3. Half-open stream/no-response connection instead of a clean EOF (R3).
4. SSE reconnect gets 401 while React retains an old user object (R4).
5. Old reconnect request completes after a newer event or session cleanup (R5).
6. URL contains a literal/encoded percent sign (R6).
7. Correct deletion phrase while a channel still has running work (R7).
8. Card presence tests pass while the card base style is never selected (R8).
9. Menu selection with the optional shell callback absent (R9).
10. Hidden tabs during an outage (R11).

## Recommended Actions

1. Repair the watch-backfill transaction boundary first; cover both current-principal authorization and batch ordering without introducing a new lock inversion.
2. Finish Viderse connectivity/session lifecycle wiring and add bounded-time, half-open and expired-session scenarios.
3. Remove the uncancelled duplicate reconnect request; fix query decoding and the channel-delete/switcher wiring.
4. Select the actual toast-card layout, consolidate icons into the kit, and close the documented structure-rule gaps.
5. Have the main agent run verification **once after fixes land**, under the repository heavy-work lock/resource limits: focused hub/Noverse watch/live/revocation tests with race coverage; Viderse status/web/settings tests; kit and both apps data/router/session/dialog tests; type/build/architecture checks. Add a real stop/restart and silent-network-loss smoke for the five-second offline claim. Include layout-aware coverage for toast cards. These are recommendations, not checks exercised by this reviewer.

## Plan / acceptance status recommendation

| Area | Source-backed recommendation |
|---|---|
| Shared Go hub extraction | Implemented in the ordinary frame/replay path; do not accept Noverse watch compatibility until R1/R2 are fixed. |
| Shared TS data extraction and kit additions | Extraction is in place; toast-card layout remains incomplete (R8); Viderse should use kit icon infrastructure (R10). |
| Status API and settings foundation | Wire fields, content ETag, 304 and indexed parameterized settings storage are present. Runtime tests not independently repeated. |
| Shell/router/V12 | Present, but offline timing/session expiry, percent-input parsing, running-channel delete guard and channel selection are not complete (R3/R4/R6/R7/R9). |
| Structure rules | Not fully met; see R10 and the specific low-priority rule gaps. |
| Business route content and real pipeline counters | Intentionally deferred by the foundation phase; not counted as missing work in this review. |
| Test/build/size and visual acceptance | Unverified by this review. Existing reports describe prior runs but cannot establish behavior for the missing scenarios above. |

No plan files or task states were mutated.

## Metrics and limitations

- Type coverage: **not measured**; no typecheck was run.
- Test coverage: **not measured**; test source was inspected, no tests were run.
- Linting issues: **not measured**; no linter was run.
- Concurrency/auth/API/backward-compatibility/input-validation/query-efficiency/data-exposure review: applied to the scoped paths above. No current-endpoint SQL injection or missing HTTP authentication was identified; R1 is a concrete data-exposure path.
- No runnable reproductions, DB migrations, production-network checks or generated-bundle checks were performed. Assertions of impact in the failure sequences remain static inferences until the recommended regressions are run.
- The image-generation/YouTube/tool implementations and final whole-product readiness remain outside this review.

## Unresolved Questions

None required to act on the findings. Where plan text differs on normal polling, this review follows the phase-01 fallback-only decision and retains the explicit visible-tab restriction; it does not request an unapproved change to healthy-SSE polling policy.

Status: DONE_WITH_CONCERNS
Summary: Review complete. Four blocking findings cover Noverse backfill authorization/order and Viderse offline/session lifecycle; additional source-backed correctness and structure gaps are documented. Ordinary SSE framing/replay, content ETag/304, endpoint authentication and indexed settings access look correct by inspection; no independent verification was run.
