# MPolish — final mobile evidence and repairs

Status: **DONE_WITH_CONCERNS** — implementation and independent captures complete; parent must inspect the critical native images before merging. External-provider success and real-device keyboard behavior are not claimed.

## Source
- Worktree `/Users/dat/Projects/verse-mobile-polish`, branch `fix/viderse-mobile-polish`.
- Inherited commits `c6a60a32`, `ef1eb556`; final repair `65626836` — `fix(viderse): bound mobile composer and respect auth safe areas`. Three explicitly staged files, private index, no push.
- `NewScreen.tsx`: measure the real sticky footer with `ResizeObserver`, disconnect on cleanup, and bound the phone scrollport using viewport/top-safe/footer/bottom-safe/keyboard geometry. The former fixed clearance missed wrapped plan text and did not constrain the keyboard scrollport. Prevent the multi-link entry from shrinking to zero inside the constrained scrollport.
- `NewScreen.test.tsx`: regression covers footer measurement changes and cleanup of the footer's own observer.
- `auth.stylex.ts`: AuthShell now reserves `--m-safe-top` and `--m-safe-bottom` on phones. Previously the brand remained at approximately 8.75px even with a simulated 59px top inset.
- Existing kit Segmented/Button/Switch controls, Settings body portal and Queue layout are retained. No shell, other-screen, production, embedded-dist or dependency-install changes.

## Evidence provenance — supersedes the former report
The earlier “no screenshots / services stopped / DBs dropped” paragraphs and subsequent `/tmp/mpolish-shots` acceptance claims were contradictory and cannot establish current acceptance. At this handoff, 21 files existed there, but inspected images showed unrelated reader/discover screens and dimensions inconsistent with their filenames. Their origin is not established; **none is accepted as MPolish evidence**.

All current evidence is independently captured from the actual fixture with real login/API state, in explicitly granted one-tab slots. The tab `PolishEvidenceFinish` is CLOSED.

### Authoritative artifacts
- `/tmp/polish-evidence-final/capture-manifest.json` — exact list of **129 native PNGs: 95 app + 34 canonical references**, source versions and URLs; `needsRecapture: []`.
- `/tmp/polish-evidence-final/geometry.json` — per-state bounds and portal facts.
- `/tmp/polish-evidence-final/final-layout-proof.json` — final `65626836` New/Auth assertions, forced-form bottom reachability, and canonical asset HTTP results.
- Only files listed in `capture-manifest.json` are authoritative; earlier probe/intermediate files are not acceptance evidence.

App configurations: **390×844 and 360×780**, each at safe insets **0/0 and 59/34**, plus **1440×900 desktop**, native DPR 1.

- **MNew:** empty, single, multi, priority, single/multi CSS keyboard (`--kb=300px`), keyboard-bottom scroll reachability; desktop single/multi.
- **Queue:** top, bottom, last-row action sheet and rules sheet in all four phone configurations; desktop smoke. Sheets were recaptured after finite entry animations settled.
- **Settings:** list/tools/connect/agents/accounts/storage in all four phone configurations, plus desktop sections. Every phone overlay measured as a direct child of `document.body`; native images show no global-bar obstruction.
- **Auth:** real wrong-password response, real forced-password account and actual missing Viderse grant, at all four phone configurations and desktop. Forced-password bottom-scroll images additionally prove logout/help reachability.
- **Canonical HTTP:** original MNew/MQueue/MSettings/MAuth tweaks at both widths and original desktop NewVideo/Queue/Settings/Auth. The initial static-server root omitted `../img` and `../icons`; all 34 references were replaced after serving the complete `design/v1` root. Collected local images and mask icons returned HTTP 200, with zero recorded failures. Canonical mocked 9:41/status chrome is retained, not fabricated in the app.

### Critical native files for parent inspection
All paths below are inside `/tmp/polish-evidence-final/`:
- `app-360-inset59-34-new-keyboard.png`
- `app-360-inset59-34-new-keyboard-bottom.png`
- `app-360-inset59-34-queue-row.png`
- `app-360-inset59-34-queue-rules.png`
- `app-390-inset59-34-settings-connect.png`
- `app-390-inset59-34-settings-storage.png`
- `app-360-inset59-34-auth-forced.png`
- `app-360-inset59-34-auth-forced-bottom.png`
- `app-360-inset59-34-auth-denied.png`
- `board-360-canonical-MNew-single.png`

## Measured final outcomes
- New Segmented group **48px**, items **40px**; single-link clear **44×44**; Dán height **44px**; submit height **48px**. Multi mode intentionally has no clear button, matching the canonical multi state.
- Before repair, 360/59/34 keyboard scrollport bottom was **636.56px** while the bar began at **333.5px**; the ordinary wrapped plan also overlapped by **18.75px**.
- After repair, keyboard scrollport bottom **equals** sticky bar top for 390/0, 390/59, 360/0, 360/59: **405.5 / 397.5 / 341.5 / 333.5px**. Multi textarea height **109.56px**, shrink **0**, textarea bottom before summary/options; last priority row remains reachable in all four configurations. Final native images were inspected after the shrink fix.
- Auth at inset59/34: header top **59px**, brand top **67.75px**, shell bottom padding **34px**. At 360×780 main ends at **746px**; forced-form logout bottom after scrolling **680.05px**, safely inside it. All 15 Auth state/configuration checks pass; all four forced-bottom checks pass.
- No horizontal document overflow in the recorded app configurations.
- Queue uses a 72px **minimum**, not an asserted exact 72px row height: seeded three-line rows measured **75.25px**. The earlier exact-72 claim is withdrawn. Last row and action/rules controls remain reachable in native captures.

## Exercised checks
- `npx tsc --noEmit` — pass after final source edits.
- Initial focused new/queue/settings/auth/design run: 21 files, 210 passed and one newly added observer-test failure. The test incorrectly shared one disconnect spy with kit observers; it was corrected to identify the observer attached to the actual footer, without weakening cleanup assertions.
- Final `NODE_OPTIONS=--no-experimental-webstorage npx vitest run --pool=threads --maxWorkers=2 src/screens/new src/screens/auth src/checks/design-rules.test.ts` — **7 files, 67 tests pass**. Unchanged Queue/Settings tests passed in the initial run. Existing React act-environment warnings were emitted; they were not suppressed.
- No build, lint, formatter, full-repository test run or package installation.

## Live fixture and review handoff
- Existing real backend **8049**, Vite **8050**, fixture DBs and isolated fixture HOME retained. `/tmp/bili-serve.sh` was read only and **never rerun**.
- Canonical app URL: **http://mpolish.localhost:8067/**. `PolishEvidenceProxy8067` forwards assets to existing Vite8050 and `/api` to backend8049, stripping API Origin/Referer only in this fixture. This was necessary because real login through Vite8050 returned HTTP403 under its Host-rewriting proxy. No product auth policy changed; cookies remain isolated to `mpolish.localhost`.
- Canonical references: **http://127.0.0.1:9067/artboards/**, service `PolishArtboards9067Final`, root `/Users/dat/Projects/verse-mobile-polish/apps/viderse/docs/design/v1`.
- Browser released. Backend/Vite/proxy/artboard services remain alive **until parent image review**; this is deliberate, not a claim that cleanup already occurred.

## Honest limits
- Keyboard proof is CSS layout emulation, not an actual iOS/Android keyboard or visualViewport integration test.
- Canonical content and seeded real API content differ; screenshots do not establish pixel identity of text, artwork or dynamic ETA.
- Seeded rival/Queue thumbnails have missing/broken source URLs; product placeholders are visible. No remote-media success is claimed.
- Member-management row is absent because fixture `/api/me` provides no usable Noverse app origin.
- Desktop first-run setup legitimately appeared once; subsequent explicit navigation used the application's existing checked-once behavior, not a fabricated ready-provider response.
- No external ChatGPT/Grok/provider login, generated-image completion, storage migration, password change or Queue mutation is claimed. Auth states were obtained from real login and isolated grant/password-required fixture state; viewer was restored to the forced-password fixture afterward.
- Bilibili PAUSED remains outside this assignment.

## Controller acceptance and cleanup

The parent accepted the critical native proof and integrated the repair in `6b822e19`; subsequent complete Viderse web gates and production deployment include it. Final combined Viderse web verification passed445files/3658tests with4skipped.

Durable native PNGs and metadata are retained in [`polish-evidence-final/`](polish-evidence-final/); use its capture manifest and final-layout proof, not the rejected earlier screenshots. The fixture cleanup receipt in that directory records all four owned ports closed, both owned databases dropped and the isolated HOME removed. The earlier worker handoff's “remain alive” statement describes its handoff time, not the final release state.
