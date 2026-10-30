# QA3 — Machine

## Result

**Do not pass the final gate yet: 2 major issue groups remain.** All 8 major finding groups listed in QA2-Machine are fixed in the current build. This pass found a genuine fresh-machine setup blocker and a contradictory signed-out description on phone, plus 4 minor groups.

The count table below gives both unique issue groups and affected-state occurrences. A shared sidebar difference is counted once for each of the 6 Settings sections, not counted again for every tools variant.

## Execution and evidence

- Built the Go binary with the requested Go 1.26.0 / GOMAXPROCS=4 / lockf / nice command. Rebuilt and restarted after both embedded-dist notices, including the **14:03:54** update. The final complete primary capture set, including setup login and failed, was taken from that last binary. The focused setup login button now measures **36px / radius 10px / font 13px**; the earlier 32px result is superseded and is not a finding.
- Served the app independently on **127.0.0.1:18781**. Created and migrated an isolated PostgreSQL database and created local administrators through setup verify/admin. An initial database named viderse_smoke_machine disappeared during the run; HTTP confirmed the database did not exist. Those error/loading captures were discarded. The final capture set used the separately named **viderse_smoke_qa3machine**, with healthy provider/agent requests.
- Used desktop **1440×900** and phone **390×844**, with touch/mobile emulation on the phone tabs. Set the artboard tab viewport explicitly as well; an earlier 500px-wide reference capture was discarded. App phone back target is 44×44, update action is 44px high, and the details summary is 48px high.
- Loaded the local Be Vietnam Pro and JetBrains Mono assets into the reference tab to avoid a blocked external-font fallback. Waited for fonts and local mask icons before saving images. Did not use implementer screenshots as evidence.
- Used real GET payloads as the base for request overrides. Setup login/failed and the requested tools variants were produced by overriding only the corresponding API paths; attention state was overridden where needed to inspect dots. The recent-errors example uses the actual errors feed contract populated with the artboard example rows. Fresh setup, a real running installation, and the signed-out phone row were also checked against the real backend without those overrides.
- **Tool-protocol limitation:** this reviewer tool inventory did not expose the JS eval browser kernel, browser.open, tab.run, or tab.box. I used one isolated agent-browser CLI headless Chrome session with two tabs, the localhost allowlist and the PublicKeyCredential deletion init script. It used its CLI-owned ephemeral profile, and interception was configured through its network API rather than inside tab.run. This differs from the prescribed kernel/launch protocol; the observations below are nevertheless from this reviewer driving the live app, not from supplied screenshots. No external login was completed and no external link was clicked.

All retained evidence is under **/tmp/viderse-qa3/Machine/shots/**. Each primary pair has art-<state>.png and app-<state>.png, plus JSON containing visible text, element boxes and computed button styles. Useful additional evidence:

| Evidence | Purpose |
|---|---|
| app-setup-fresh.png; fresh-repro.json | Real final-binary idle setup, with sufficient disk and pending tools |
| app-setup-running-real-current.png; fresh-manual-start.json | The same backend starts after an explicit POST, isolating the missing automatic start |
| app-msettings-tools-real.png; real-mobile.json | Real needs_login account contradicts the phone description |
| art/app-msettings-tools-expanded.png | Version labels present in the design and removed in the app |
| art/app-tools-failed.png; corresponding JSON | Failed-row description tone |
| art/app-logs-populated.png | Populated logs including warning and resolved entries |
| dot-metrics.json; art/app-dot-measure.png | Computed sidebar dot positions, not just a visual estimate |

## 1. Round-2 majors

| QA2 major group | QA3 result | Current evidence |
|---|---|---|
| All 6 Settings frames: the bespoke top bar/navigation did not match the artboard | **Fixed** | frame-tools/connect/agents/accounts/storage/logs pairs show the shared grouped nav with 5 actions, neutral nav selection, machine-settings label, activity indicator, gear and account control. There is no spurious selected Bàn dựng tab. Live badges and activity counts are not treated as copy mismatches. |
| All 6 frames: wrong sidebar group labels/layout and raw build string in the footer | **Fixed** | All frame pairs show Máy này / Dữ liệu / Ở trang khác, the correct section ordering and an anchored footer. The footer is Viderse 0.0.0 in this local build, without timestamp/commit/dirty decorations. The absent Giới hạn từng kênh shortcut is accepted deviation #25, not counted. |
| Tools update/failed/installing: Nâng cao was a detached card rather than the last row of the tools card | **Fixed** | tools-update, tools-failed and tools-installing pairs show a single outer card containing all tool rows and the bottom Nâng cao summary. The separate phone details card remains correct for MSettings. |
| Tools failed: duplicate repair CTA in the summary instead of only the failed row | **Fixed** | tools-failed shows Kiểm tra lại in the summary and one Tải tiếp action next to the failed transcription row. |
| Logs: version-check action incorrectly primary | **Fixed** | frame-logs and logs-populated show outline Kiểm tra bản mới at 32px. Xuất log remains the sole primary button, at 36px. |
| Setup running/login/failed: heading, role dots, status badges/icons and footer treatment differed materially | **Fixed** | setup-running/login/failed pairs show the 720px centered layout, Chuẩn bị máy cho 4 trợ lý, four role chips, six rows, row-end role dots and matching state icon categories. Final login and failed primary actions were rebuilt and re-shot after the size change, and now measure 36/r10/13. |
| Setup failed: an extra download/progress panel remained under the failed row | **Fixed** | setup-failed shows only the orange failure explanation and Thử lại for that row. The row has no extra transfer panel. |
| Agents frame: the page remained in loading rather than settling | **Fixed** | frame-agents was taken after the actual .agt-card appeared. It shows the complete agent configuration, sidebar, top bar and right rail. The discarded database-failure capture is not used. |

## 2. Remaining differences

### Major

- **[major] Onboard fresh machine → setup=running · automatic preparation:** artboard and first-run flow start installation automatically, then show a real running step; app renders Đang cài while the real API remains state=idle, running=false, complete=false and the missing tools stay Chờ lượt. There is no start button on this screen. See the exact final-binary repro below. This is a functional blocker concealed by a plausible static running summary, not a screenshot-only difference.
- **[major] MSettings screen=tools with a real signed-out account · Đăng nhập ChatGPT row:** artboard uses Đã đăng nhập … only for its signed-in example; app unconditionally uses the signed-in description on phone even when its real GET returns state=needs_login and explicitly says the user must sign in. The result is a Cần bạn pill immediately beside Đã đăng nhập · quản lý ở Kết nối AI. This is contradictory state reporting, not merely a different live account or missing Plus tier. Evidence: app-msettings-tools-real.png and real-mobile.json. Cause: hintMobile in ToolsSection.tsx:103–109 substitutes descsMobile for needs_login as well as ready/update; the state-specific server hint is discarded. Preserve the sign-in-required description for that state.

### Minor

- **[minor] Settings section=tools&tools=failed · failed transcription explanation:** artboard uses orange warning ink for the error explanation; app renders the explanation in neutral gray (#A0A0AB). The component emits vv-tools__hint is-err, while tools.css:62–66 styles is-warn and is-note but not is-err. The orange pill/action do not restore the missing explanation tone. Evidence: tools-failed pair and app-tools-failed.json.
- **[minor] Settings section=logs · unresolved recent-error dot and retry status:** artboard uses orange #FF9A5C for both the unresolved dot and Tự thử lại lúc 07:00; app uses yellow #F6CB8C for both. The populated feed was schema-correct and its copy matched the example. LogsSection.tsx:35 and logs.css:41 select the generic warn/yellow token instead of the orange unresolved-error tone. Evidence: logs-populated pair; the reference values are explicit in Settings.dc.html:1407.
- **[minor] Settings frame, all 6 sections · sidebar attention-dot alignment:** artboard places the tools dot at x=210–217; app places it at x=221–228, 11px farther right in the same 248px sidebar. Color and diameter match. The reference retains an empty trailing count span, whereas SettingsScreen.tsx:150–151 omits that span for an empty count, moving the dot into its space. This is a shared layout difference across tools/connect/agents/accounts/storage/logs, not six different causes. Evidence: dot-metrics.json and dot-measure pair; all six frame pairs were also inspected.
- **[minor] MSettings screen=tools, details expanded · tool/version identities:** artboard retains names such as yt-dlp and ffmpeg next to each version; app shows a directory followed by unlabeled values such as 2026.08.19 6.0 — 156.0.8078.74 0.162.1. The values are live data and are not themselves defects, but their identities have been removed. The mobile .vv-tools__adv-k { display: none } rule in tools.css hides every tool label as well as the directory label. Keep the tool names visible. Evidence: msettings-tools-expanded pair.

### Exact setup-blocker reproduction

This was repeated against the last rebuilt binary, after the 14:03:54 embedded-dist update, using a new HOME rather than an intercepted idle response.

1. Start the final binary on 127.0.0.1:18781 with the migrated isolated DB and **HOME=/tmp/viderse-qa3/Machine/fresh-home**, which did not contain an existing machine installation or account store.
2. Create a local administrator using the setup code and POST /api/setup/verify followed by POST /api/setup/admin. This establishes the local browser session; it does not complete any external login.
3. At 1440×900, open **#/setup** with all API interception removed. Wait for the six-step list, then read GET /api/setup/state. Read it again after allowing 1.5 seconds for the mounted effect.
4. Both real responses remain **state=idle, running=false, complete=false**. ffmpeg and transcription are pending; codex-login is needs_user. The disk check is enough=true: freeBytes=7,527,804,928 versus requiredBytes=4,714,397,696 in the second response. The UI nevertheless says Đang cài and both missing tools say Chờ lượt. No automatic preparation starts.
5. Without changing disk, account or installation state, explicitly POST **/api/setup/start** with an empty JSON object. The real response immediately becomes **state=running, running=true**, and the subsequent screen shows a real active install.

**Expected:** mounting desktop setup on an idle, incomplete machine with enough disk issues the automatic start request and begins preparation. **Observed:** it does not; only the explicit diagnostic POST starts the exact same backend.

**Code path:** SetupScreen.tsx:164–168 returns when **!data.complete**, before reaching the state=idle start branch. A fresh machine necessarily has complete=false. GET /api/setup/state only returns Status; it does not start the service. Therefore neither side starts the installation. The comment above this effect already says the server starts only on POST. The fix should allow the incomplete idle machine through that guard, rather than changing the label or adding a misleading fake running state.

## 3. Coverage, exclusions and accepted deviations

| Requested surface | Checked |
|---|---|
| Onboard step=setup, running/login/failed | All 3 artboard/app pairs; real fresh idle and real running behavior additionally checked |
| Settings tools ok/update/failed/installing | All 4 artboard/app pairs; summary, rows, controls and advanced-card boundary checked |
| Settings logs | Empty real feed and populated warning/resolved rows; export/version controls and right rail checked |
| Frame on tools/connect/agents/accounts/storage/logs | All 6 complete frames; top bar, sidebar dots, footer and section-specific right rail checked |
| MSettings list/tools | Both 390×844 touch-emulated pairs; expanded details and real signed-out tools row additionally checked |

**No requested visual state was left unchecked.** A full multi-gigabyte install through completion, external ChatGPT authentication, real video-job error production and clicking the export/version-check endpoints were not exercised: this assignment is the visual/current-state gate, and external login was expressly prohibited. The real installation was started only to verify its running behavior. Details and phone controls were inspected below the fold as applicable; the reference OS status/home bars were not expected to be rendered by the web app.

Entries **#18–#26 in 11-hoi-dap.md are not counted as defects**. In particular: live activity/review/error counts, build semver, transfer sizes, timings/ETA, the omitted lane-capacity fragment, the omitted per-channel shortcut and native phone chrome are excluded. Data differences such as installed Chrome versus the example queued Chrome are not reported as visual bugs.

Two accepted explanations deserve follow-through but do not change the count:

- **#24 is now a weak/outdated explanation:** it says the shared kit lacks a 32px button, but the current compact size demonstrably provides 32/r8/13. The spot-specific artboard class must remain the rule; it cannot justify retaining the wrong size. The final focused setup CTA is now correctly 36/r10/13.
- **#25 is a weak technical justification:** the product already has the main Kênh route, so the missing shortcut is a pending navigation/product decision rather than evidence that navigation is technically unavailable. It remains accepted and is not a QA3 finding.

## 4. Counts

| Area | Major state occurrences | Minor state occurrences |
|---|---:|---:|
| Genuine fresh-machine setup | 1 | 0 |
| Setup running/login/failed visual pairs | 0 | 0 |
| Tools ok/update/failed/installing content | 0 | 1 |
| Logs content | 0 | 1 |
| Shared frame across 6 sections | 0 | 6 |
| MSettings list | 0 | 0 |
| MSettings tools: real signed-out row / expanded details | 1 | 1 |
| **Total affected-state occurrences** | **2** | **9** |
| **Unique issue groups** | **2** | **4** |

All 8 QA2 major groups are fixed. The new functional major is not a claim that the requested running/login/failed visual layouts regressed; it is the real first-run path that must reach those layouts.

## Cleanup

The QA browser was closed, the owned service was stopped, the isolated viderse_smoke_qa3machine database was dropped, and the owned temporary binary, profiles, homes, cookie jar and capture driver were removed. Only shots/ evidence was retained. No application source or design file was edited and the web build was not run by this reviewer.

Status: DONE
Summary: 2 major issue groups remain, 4 minor issue groups remain (2 major / 9 minor affected-state occurrences). Worst 3: automatic first-run installation never starts; signed-out phone ChatGPT row claims signed in; expanded phone details erase tool names from version values.
