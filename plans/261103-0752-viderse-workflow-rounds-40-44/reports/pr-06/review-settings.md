# V40 shell/settings bounded source review

Status: DONE_WITH_CONCERNS

## Summary

Reviewed `shell-settings/reports/handoff.json` and all eight mapped changed files against the unchanged primary baseline. Read accepted phase07, Workflow conventions/round44, `01-ia-flows.md` §6.2 r44, `08-dev-prompt.md` V40, and MSettings connect/confirmation layout. This is one bounded source review, by the same model family—not an independent-model audit.

No source edits, builds, tests, formatters, browser/server launches, provider calls, or API mutations were performed. Findings below are source-evidenced; runtime behavior and typecheck were not exercised.

## Concerns

### 1. P1 — Removed logout argument still has an unmigrated caller

- Changed contract: scratch `src/components/confirm.tsx:74-77` removes `fallback` from the accepted object.
- Remaining caller: primary `apps/viderse/web/src/screens/dev/DevKitScreen.tsx:205` still invokes `confirmProviderLogout({ provider: 'ChatGPT', fallback: 'Gemini', ... })`.
- `DevKitScreen.tsx` is absent from the handoff's changed-file map, so applying the eight mapped files preserves that stale object literal.
- Impact: the cutover is not type-contract complete. [INFERENCE] TypeScript excess-property checking will reject this caller when it is included in the web typecheck; no compiler was run.
- Required correction: migrate this existing caller to the new no-fallback contract and include it in integration. No compatibility shim is needed.

### 2. P2 — Logout/removal failures still render raw exception messages inside the confirmation

- Scratch `src/screens/settings/connect/ProviderCard.tsx:391-418` adds Vietnamese mapping in the outer `.catch(guard)`, but `doLogout` and `doDelete` still pass raw rejected API promises into the shared confirmation.
- Primary shared `platform/ui/src/ConfirmDialog.tsx:230-234` catches that action rejection and stores `cause.message`; line325 renders it in the visible alert. Its `useAsyncAction` consumes rejection (`platform/ui/src/useAsyncAction.ts:29-34`), while the confirmation remains open (`ConfirmDialog.tsx:282-289`). Consequently, the provider-level `.catch(guard)` does not translate normal action failures.
- Impact: a failed logout/delete can still display untranslated/raw API errors, contrary to phase07 step6 and the handoff's claim that provider logout failures render Vietnamese product errors. The special `in_use` translation also does not reach the normal confirmation error path.
- Required correction: map/rethrow an appropriate Vietnamese product error within the action handed to the confirmation, or use an existing supported confirmation error-mapping mechanism. Keep failure visible and retryable; do not swallow it and produce success.
- Security scope: no actual secret leak was demonstrated. Raw exception rendering is evidenced, but whether a given server response includes sensitive data was not established.

### 3. P2 — An earlier save can close and discard a newly reopened key/address editor

- Scratch `src/screens/settings/connect/ProviderCard.tsx:374-388` starts an asynchronous update and unconditionally calls `closeEdit()` after update/cache invalidation completes.
- Cancel, scrim, and Escape remain enabled during that request (`:360-367`, `:465-466`, `:553-555`); after the sheet exits, `editExited` clears the previous editor (`:365-367`). The user can then open it again through `secondaryGo` (`:437-443`) and type a different value. There is no edit-session identity check on the earlier completion.
- Source-derived reproduction: start save A on a slow connection → cancel/dismiss the sheet → reopen the same provider editor → type B → let A finish. A's completion closes the B sheet and its exit clears B, even though B was never saved. Save B is also blocked by the shared `editBusy` while A is pending.
- Impact: real unsaved input is lost in the new mobile sheet lifecycle. The older desktop inline flow has the same stale-completion pattern; this is not claimed as a newly introduced desktop regression.
- Required correction: associate completion with the edit session that initiated it, so a completed earlier request cannot close/clear a later session. Dismissal need not pretend to cancel an already-submitted API write.

## Reviewed behavior without an additional source finding

- Mobile key/address editing uses the existing kit Sheet; desktop keeps an inline editor and the same `updateProvider` action.
- Add-existing-provider → key-sheet and edit → removal-confirm transitions wait for `onExited`. Shared `Sheet.tsx:100-113` releases focus/scroll locks before calling the handoff. No ordinary-path simultaneous overlay was found in these two transitions.
- Cancel/Escape/scrim before submission close without initiating a write; the pending-save race is separately reported above.
- Changed mobile buttons use kit `lg` (`Button.tsx:52-53`:44px). The key input explicitly sets46px/16px and reveal44px; the address field uses kit mobile sizing. Actual viewport/keyboard rendering still requires the parent's native UI gate.
- Password entry remains controlled and masked by default through shared PasswordField; the reviewed changes introduce no console logging or persistence of the entered key. Sheet exit clears its local value. This is not a whole-backend credential audit.
- Logout copy now describes machine-scoped credentials, possible completion of already-submitted images, conditional eligible fallback, and preserved outputs; success no longer names or promises an immediate fallback switch. Actual idle/drawing/fallback-unavailable outcomes were not exercised.
- No further desktop regression was found in the inspected diffs beyond the stale logout caller above.

## Coverage limits for parent integration

The added mobile tests stub `matchMedia` to return `matches:true` for every query (`ConnectSection.test.tsx:334-336,354-356`), including reduced motion. Thus they exercise immediate sheet exit, not the normal260ms exit lifecycle (`platform/ui/src/Sheet.tsx:60-70`). They also do not cover a deferred save followed by dismissal/reopen, nor failed logout/delete error localization. These are suggested focused regression scenarios, not claimed test results.

Existing design-location blocker for mobile tools recheck and the non-approving unskip API blocker remain known blockers, not new findings. No invented recheck location, round45 viewer, auth/index/model behavior, or provider lifecycle change is requested by this review.
