# Setup-context capture handoff

## Status

**PREPARED ONLY — NOT EXECUTED.** Only this private `notes.md` and adjacent `controller.cjs` were written. No browser/API/build/tests/git/artboard/repository edits were performed. Main owns review and execution. No screenshot or React assertion has been observed in this assignment.

Controller entry: `setup-context/controller.cjs` under the private parallel-acceptance directory. Main can execute `node <absolute-controller-path>` sequentially against existing `http://127.0.0.1:8043`. It writes `results.json` and the five specifically named PNGs below to its own directory. Browser closure is in the outer `finally`; each case closes its own context. No throwaway script or source modification is needed.

## Source basis

Repository paths below are relative to `/Users/dat/Projects/verse`:

- `apps/viderse/web/src/api/setup.ts:6–43`: actual SetupStatus / SetupStep / SetupStepProgress DTO; real keys are `yt-dlp`, `ffmpeg`, `transcription`, `chrome`, `codex`, `codex-login`. Numeric progress fields include `bytesDone`, `bytesTotal`, **`pct`**, and `startedAt`; this is the requested current SetupScreen wire contract, not the platform account-setup DTO.
- `apps/viderse/web/src/screens/setup/SetupScreen.tsx:26–35,60–115,120–158,164–188,190–202,213–267`: ETA, ordered row states, numeric row percentage, completed-tool overall percentage, login-focus derivation, retry/cache behavior, desktop/mobile branches.
- `apps/viderse/web/src/i18n/setup.ts:17–20,49–85`: unmodified real task titles, hints, and CTA text. No fixture title renaming.
- `apps/viderse/web/src/screens/settings/fmt.ts:4–12,39–42`: decimal bytes formatted vi-VN (806 MB / 1,3 GB), minute-rounded ETA.
- `apps/viderse/web/src/screens/settings/connect/ChatGPTLogin.tsx:65–80,103–114`: clicking login starts provider flow; therefore never activated.
- `apps/viderse/docs/design/v1/artboards/Onboard.dc.html:925–987,1137,1165–1207`: actual canonical viewport surface, default props, ordered tool states and sizes, 62% model download, failed retained bytes and primary login.
- `plans/reports/orchestrate-261105-095440-v38-acceptance/desktop-tweak-truth.md:177–188`: C3 corrected fixtures and C4 initial-failed versus retry separation.
- `plans/reports/orchestrate-261105-095440-v38-acceptance/mobile-tweak-truth.md:7`: no canonical mobile Onboard.
- Existing private `channel-tweak-closure/controller.cjs:237–278` (`setupState`/`setupProof`, corresponding to requested setupSteps/setupCase baseline): old fixtures made Chrome pending, kept codex pending on failure, and supplied 50% of 1GB; old shared case attached failed/retry stages together. New standalone controller does not import or execute it.

At execution, controller records SHA-256 receipts for the actual source files and truth reports plus exact locally served response DTOs. It never reads/prints cookie values in reports. Setup has no owner/template reference fields, so no invented owner/template IDs or reserved `new` IDs are introduced; unrelated native GET data is untouched.

## Exact comparison bindings

All desktop viewports are **1440×900**, DPR 1. Full canonical props, including hidden defaults, are explicitly recorded:

| Reference ID | Exact PNG filename | Stage / canonical props | Tool states in real key order |
|---|---|---|---|
| `Onboard.setup.running` | `FIXTURE-1440-onboard-setup-running-initial.png` | initial; `{step:"setup",setup:"running",tplMenu:false,identity:"running",rivals:"some"}` | `[done,done,running,done,pending,needs_user]` |
| `Onboard.step.setup` | `FIXTURE-1440-onboard-setup-running-initial.png` | same initial baseline / same full props | same six states |
| `Onboard.setup.failed` | `FIXTURE-1440-onboard-setup-failed-initial.png` | initial; `{step:"setup",setup:"failed",tplMenu:false,identity:"running",rivals:"some"}` | `[done,done,failed,done,done,needs_user]`, derived login focus |

Initial transcription progress: `{phase:"download_model",phaseLabel:"Đang tải mô hình nhận giọng nói",bytesDone:806000000,bytesTotal:1300000000,pct:62,startedAt:<timestamp about 392 seconds before fixture creation>}`. **62% is supported directly by the actual DTO** and consistent with 806/1300; no text/DOM/progress-bar spoof is needed. Timestamp drives genuine ETA math to approximately four minutes. Initial running overall progress remains **3/6 = 50%**, not 62%; failed overall progress is **4/6 = 67%**, matching canonical completed-tool semantics. Step sizes are 125 MB, 90 MB, 1.3 GB, provided Chrome (0 bytes), 300 MB, login 0 bytes. `disk.approxBytes=1800000000` supplies canonical hero “khoảng 1,8 GB”; free/required/enough fields are clearly fixture data, not a machine inventory claim.

Failure uses top-level `state:"failed",running:false,complete:false`, raw network error, and retained progress bytes. No pending/running rows remain before login, so the shipped `loginFocusOf` produces the real primary login CTA without an invented focus field.

### Separate interaction/supplement captures — NO canonical reference IDs

| Exact PNG filename | Fixture stage |
|---|---|
| `INTERACTION-1440-setup-retry-503.png` | First UI retry receives a **locally fulfilled** 503 typed error. All initial failed DTO fields remain identical, login remains primary, human retry feedback is visible. |
| `INTERACTION-1440-setup-retry-running.png` | Second UI retry receives a **locally fulfilled** 202 SetupStatus. Only global state/running and transcription state/error change. All progress fields and other five tool records are preserved; states are `[done,done,running,done,done,needs_user]`. **Not** canonical initial running, not failed, not installation success. |
| `NONCANONICAL-390-setup-desktop-only-supplement.png` | **390×844, explicitly NONCANONICAL** badge. Native React desktop-only setup notice, zero setup rows, no login/retry/create action. No mobile Onboard reference is claimed. |

The `comparisons` output contains **only the three initial reference bindings**. Retry/supplement images have `referenceIds:[]`, `canonicalComparison:false`, and `props:null`. Filenames are distinct; recovery can never overwrite or inherit the failed comparison label. No success/completion fixture is used.

## Prepared assertions and safety

- All six tool states and unmodified titles in fixed key order; done tool sizes, provided Chrome, pending/done codex 300MB, row 62%, overall count/percentage, 806MB / 1.3GB, retained-byte failure copy and warning progress.
- Primary failed CTA versus ghost running CTA: visible/enabled, actual 36px versus 32px height, Tab/Shift+Tab focusability, no login activation. Incomplete setup must not expose “Tạo kênh đầu tiên”.
- Local retry POST payload exactly `{id:"transcription"}`. First 503 leaves entire DTO unchanged; second 202 preserves all progress fields and the other five steps; running view and alert removal are asserted.
- Per-capture viewport/scroll/column/card/list/row bounds, row text, and CTA sizes are recorded. Horizontal overflow is asserted absent. These measurements are evidence for Main, not a predeclared pixel-parity pass.
- Same-origin GETs to native 8043 are the only continued requests. Provider/inference APIs, external origins, all new image/media acquisition and download/archive/model paths are blocked. Event streams are locally empty; WebSockets closed; service workers blocked; downloads disabled/cancelled.
- Every non-GET defaults to abort. The **only exceptions** are the two locally fulfilled UI-client `/api/setup/retry` responses above; zero native retry/start/check/install/login/provider transport is permitted. No `idle` DTO is used, avoiding automatic setup-start.
- Cookies are reused from the stipulated file, never logged. A conspicuous fixture badge appears on every PNG; the badge is the only DOM addition and does not change source body copy or geometry.
- Page errors, unexpected non-GET attempts, provider attempts, downloads, unexpected popups, and assertion failures produce failing status; local fixture interaction never counts as native persistence/install proof.

## Summary

The prepared controller closes C3/C4 capture selection defects through genuine React response fixtures, preserves all real tool keys/copy, and pins failed comparisons exclusively to their initial state. It deliberately retains 503 and retry-to-running interaction evidence outside canonical labels. There is no execution or native proof claim.

## Concerns

1. **Unachievable exact failed wording via the DTO:** canonical `Onboard.dc.html:1191` says “Mất kết nối khi tải mô hình.”; shipped translation (`SetupScreen.tsx:38–57`, `i18n/setup.ts:69`) says “Mạng bị ngắt khi tải.”. Both retain 806 MB and offer retry, but DTO error parsing cannot produce the canonical first sentence without changing source copy. No fake body text is injected. This source-backed difference may affect wrapping; Main must inspect actual geometry.
2. **Root geometry cannot be fixture-pinned:** canonical shell uses `height:100vh;overflow:hidden`; React setup root uses `minHeight:100vh` (`setup.stylex.ts:7`). Shared inner intent is 720px column, 28px/20px/32px padding, 18px gaps. Actual coordinates are unobserved until Main runs; no exact mismatch in pixel coordinates is asserted from source alone, and there is no DTO knob to repair root geometry.
3. **ETA is computed, not a fixed canonical string:** real timestamp/byte fields yield about four minutes; unusually long capture delays may change rounding. The explicit four-minute initial assertion surfaces that deviation instead of concealing it.
4. **Mobile canonical absent:** supplemental 390 capture is intentionally not an Onboard reference obligation and will not show desktop six-row setup.
5. **CTA verification scope:** login behavior/auth/persistence remain unexercised by design. Main should run this controller after review and inspect `results.json`/initial PNGs before any visual acceptance. No build/test run is required or claimed by this preparation task.
