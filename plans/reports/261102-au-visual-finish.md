# Global assistant UI recovery — final visual and interaction evidence

Status: DONE_WITH_CONCERNS (verification limits below; independent review has no remaining source blocker).

## Delivered source

Worktree: `/Users/dat/Projects/verse-provider`, branch `fix/noverse-provider-global`.

Private-index, explicit-path commits, no push:

- `c9c6b3ed` — persist inherited-role reasoning, preserve swap undo after healthy-provider refresh, remove private assistant promise from new projects, align mock thinking/swap contracts.
- `0979e1a8` — classified connection recovery strips, shared kit buttons, actual phone workspace stop/fallback integration, honest mobile acknowledgement/toast timing, obsolete mock model mutation retirement, stable comments.
- `c445996e` — immediate-global-save sidebar copy; classified stop status uses the actual failed/latest active agent, not a hard-coded writer; quota remains a waiting/retry state.
- `5ffd103b` — repeated failure episodes cannot be hidden by stale success; writing success requires running runtime state with no classified error.

No Go, `web/src/api/types.ts`, or `web/src/api/client.ts` edits were staged by this worker. Budget/run-mode scope behavior was intentionally retained.

## Exact artifact locations

Permanent manifest: `/Users/dat/Projects/verse/plans/reports/au-evidence-final/manifest.json`.

Permanent PNG directory: `/Users/dat/Projects/verse/plans/reports/au-evidence-final/`.

Transient matching manifest and PNGs: `/tmp/au-evidence-final/`.

The manifest records the actual URL, expected state, exact transient/permanent paths, and dimensions read from PNG headers. There are **23 actual UI PNGs + 3 canonical reference PNGs**. Desktop images are 1440×900; phone images are 390×844 or 360×844, DPR 1.

| Actual PNG basename | Exercised state |
|---|---|
| `01-settings-admin-1440` | Global immediate save, reasoning choices, corrected sidebar footer |
| `02-save-failure-retry-1440` | Failed actual PUT, visible error, previous selection restored, retry action |
| `03-settings-quota-1440` | Warning/countdown, not expiry |
| `04-settings-expired-1440` | Expired provider, bulk replacement preview and kit actions |
| `05-bulk-swap-undo-warning-1440` | Actual bulk swap, retained undo after healthy refresh, explicit warning |
| `06-connect-expired-1440` | Classified expired connection strip |
| `07-connect-key-entry-1440` | Classified invalid key, real click opens key entry |
| `08-connect-quota-1440` | Classified temporary quota warning |
| `09-work-stop-1440` | Actual desktop writing stop card |
| `10-work-fallback-1440` | Runtime fallback banner |
| `11-command-model-global-1440` | Global command model panel |
| `12-work-stop-390` | Fresh actual phone route; stopped agent, full-width recovery action |
| `13-work-stop-360` | Same fresh stop at narrow width, no old warning toast |
| `14-models-390` | Global model overview |
| `15-model-thinking-390` | Server-provided per-model reasoning choices |
| `16-models-quota-360` | Narrow warning/countdown and affected assistants |
| `17-models-swap-undo-390` | Actual phone swap, healthy refresh, persistent undo and warning |
| `18-model-optimistic-pending-390` | New selection displayed while the actual PUT is held 700 ms |
| `19-member-readonly-1440` | Actual member identity; all four model pickers disabled |
| `20-member-readonly-390` | Phone member read-only controls |
| `21-model-help-scroll-390` | Real mouse-wheel scroll; final help reachable above tabs |
| `22-episode-b-running-1440` | Real resume acknowledgement followed by running B snapshot |
| `23-episode-b-new-key-stop-1440` | Next failure in the same mounted card exposes new B recovery |

All exact final PNGs were opened with native image inspection. Critical 01/12/13 were freshly recaptured after the last source fix. Earlier desktop screenshots retain the earlier sidebar wording; 01 is the authoritative final sidebar evidence.

## Interaction evidence

- Held a real mobile `PUT /api/config` for 700 ms. Display switched to `gpt-5.4` with a saving indicator while server writer remained `anthropic / claude-sonnet-4`; no premature success toast. After acknowledgement, global writer became `openai-codex / gpt-5.4`.
- Actual reasoning click on an inherited writer persisted `{provider: 'openai-codex', model: 'gpt-5.4', reasoning_effort: 'low'}` through the global PUT.
- Failed desktop PUT restored the prior selected reasoning radio. Retry cleared the visible failure and server reasoning became `minimal`.
- Actual desktop and phone bulk swaps and undo executed mock API contracts; undo remained visible after the configured primary became healthy.
- Actual phone recovery issued `POST /api/projects/dit-dang-audio/stories/01-tra-xanh-cuop-kich-ban/resume`, HTTP 200, `{ok:true,label:'Viết tiếp chương 4'}`.
- Same-mounted second episode: A expired → real swap to B + resume 200 → running B snapshot → B key failure. React card fiber/current-alternate identity, parent DOM node, and `performance.timeOrigin` remained identical. New failure and enabled recovery were present; stale writing success was absent. This used server fixture snapshot events, with no navigation/unmount during the sequence.
- Phone final help was reached with a real wheel gesture. Its measured top/bottom were 672.27/709.77 in an 844px viewport, above the bottom tabs.

## Canonical HTTP comparison

Raw canonical artboards were served from `/Users/dat/Projects/verse/apps/noverse/docs/design/v1` on owned port 5211. Direct artboard runtime props were applied with `window.__dcSetProps`; no preview iframe manipulation is part of the final reference evidence.

- `canonical-settings-expired-1440.png`: `/artboards/Settings.dc.html`, `{section:'models', prov:'expired'}`; compare actual 04.
- `canonical-work-stop-390.png`: `/artboards/MWork.dc.html`, `{state:'error', perr:'expired'}`; compare actual 12/13.
- `canonical-models-390.png`: `/artboards/MMore.dc.html`, `{state:'model'}`; compare actual 14. This is the ordinary model overview, not a quota reference.

Inspection covered hierarchy, classified severity, replacement preview, selected models, thinking controls, recovery actions, wrapping and phone touch-target layout. Observed plain-text desktop actions were replaced with existing kit buttons; phone recovery is full-width and at least 48px high. Real fixture providers/counts differ from artboard sample data.

## Verification and review

Final focused command ran eight relevant test files (stop-card episodes, desktop roles, mobile model rendering/interactions, new project, desktop live stream, phone rendering and logic): **47 tests passed / 8 files passed**. `tsc --noEmit` passed.

Repeated-episode regression was exercised before repair: **2 failed**, showing the missing B alert and false writing claim. After repair: **2/2 passed** within the final focused run. Failure output: `artifact://69773`.

`AUContractReview` independently inspected committed source and native images 22/23 and explicitly closed P2 at `5ffd103b`: no remaining source blocker, no rerun claimed.

## Limits and cleanup

- Browser evidence uses the normal Vite mock mode, canonical dependency symlinks, and no dedupe workaround. It is not a real upstream model/OAuth/key validation run.
- Phone evidence is Chromium at the stated viewport with environment safe-area insets zero, not physical iOS. Canonical boards include simulated status-bar space; no hard-coded extra iOS inset was added.
- Comparison is not a claim of identical sample content or pixel-exact parity.
- Existing mock live-history load failure is visible below the desktop recovery surfaces; that separate history flow was not changed or claimed to pass.
- Final integration build, combined-repository verification and deployment remain Main's responsibility; this worker did not deploy or push.
- Browser tab `AUVisualFinish` is closed. All worker-owned preview/artboard services on 5210/5211 are stopped. No protected profiles, real config roots or production were touched.

## Controller integration and release

Integrated source, independent AU contract review and combined gates are complete. Final Noverse web suite passed223files/1866tests; the complete backend package gate passed after the subsequent reader export repairs. Paired hosted/native Mac mini deployment completed at `c6f425e7a4c86b8b99d70cfcd5055e5ef63a0cc4`.

Read-only inspection on the public production origin confirmed the app-wide assistant notice, four actual model options in the opened picker, no scope bar and no manual Save action. Current model reasoning choices came from the server; only supported choices were displayed rather than forcing eight choices onto every model. No production assistant/provider selection, key or commercial upstream request was changed for this check.
