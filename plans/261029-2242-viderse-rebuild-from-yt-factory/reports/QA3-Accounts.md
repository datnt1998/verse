# QA3 — Accounts · independent live visual QA

## Scope and evidence

**Verdict: the 17 round-2 majors are now visibly fixed. One newly established desktop layout major remains.** This is a review of an independently run current binary, not of implementer screenshots.

- Built the Go binary with the requested `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 lockf -k /tmp/verse-heavy.lock nice -n 10 go build … ./apps/viderse/cmd/viderse` command. After Main announced the final embedded distribution at **13:56:22**, rebuilt the binary and restarted the isolated server **before the retained captures**. Did not build the web distribution.
- Created and migrated `viderse_smoke_accountsqa3`, created a fresh local administrator through setup verify/admin, and ran the app at `127.0.0.1:18783` with its own temporary HOME.
- Used one launched Chrome browser instance and two tabs, app and the live artboard service on `127.0.0.1:18700`. Captures use **1440×900 desktop** and **390×844 phone**, device scale 1, with mobile/touch emulation for phone. `PublicKeyCredential` was deleted by the init script. No external page/action was clicked, no QR was scanned, and no platform login was completed.
- **Tool-method limitation:** this worker has no JS-eval browser kernel or `browser.open`/`tab.run` tool. Used the installed `agent-browser` plus direct Chrome DevTools Protocol instead. The CLI rejected passing `--user-data-dir` through `--args` before launching; the successful single launch used the requested Chrome executable/headless/keychain/password/first-run flags, local-domain allowlist and init script, but its tool-managed profile rather than the requested explicit Chrome profile directory. This is disclosed, not represented as the exact requested kernel invocation.
- Interception was confined to the necessary local `/api/platforms…` and `/api/status` paths. Fixtures started from real platform/status/profile/QR-start/QR-poll responses from this server and were edited to select the states. Only **waiting** used real Bilibili QR creation/polling; every other QR state used intercepted creation/polling. For the sidebar fixture, the corresponding `login_expired` attention item was supplied consistently with the backend attention producer.
- Screenshots and measured DOM/computed-style evidence are retained in **`/tmp/viderse-qa3/accounts/shots/`**. A suffix such as `desktop-qr-error` means `art-desktop-qr-error.png` and `app-desktop-qr-error.png` there. `measurements.json` contains per-state text, boxes and styles; `fontchecks.json` contains actual platform-font evidence; `qa-summary.json` summarizes coverage and touch-target measurements. Captures were brought to the foreground, awaited font/animation readiness, and later captures explicitly decoded icon masks. The desktop expired/scanned pairs were recaptured to eliminate first-load icon-mask artifacts.

Read the round-2 report, IA/W13, PlatformCard/QrLoginDialog, tokens, kit, copy, artboard README, and accepted pending deviations #18–#26. Live names, times, counts, versions, QR bits/countdown fill, and the artboard-only simulated OS status bar/home indicator are not application defects. In particular, no fake OS padding should be added to the web app.

## 1. Round-2 major reconciliation

IDs retain the original report numbering. R1 was already a minor residual, so the 17 major rows are R2–R18.

| ID | Round-2 major | Current status | Fresh evidence |
|---|---|---|---|
| R2 | Extra QR action in desktop blocked banner | **fixed** | `desktop-blocked`: clock/title/explanation only; no QR action inside the banner. The saved-account actions remain in their own row. |
| R3 | Missing expired-login sidebar attention dot | **fixed** | `desktop-expired`: orange dot is visibly present on **Tài khoản nền tảng**, alongside the expired Bilibili state. |
| R4 | Missing YouTube-bot card issue outline | **fixed** | `desktop-yt-bot`: the entire YouTube card has the orange issue perimeter, not only the banner. |
| R5 | Missing YouTube-bot sidebar attention dot | **fixed** | `desktop-yt-bot`: orange dot appears on **Tài khoản nền tảng**. Other machine/AI attention dots were not mistaken for this marker. |
| R6 | Divider above, rather than below, the YouTube issue | **fixed** | `desktop-yt-bot`: issue is below the platform heading, then a divider separates it from browser-cookie controls. |
| R7 | Desktop QR backdrop lacks blur | **fixed** | All desktop QR pairs: the full page/navigation are blurred and darkened. Current dialog applies `backdrop-filter: blur(3px)`, rather than mere dimming. |
| R8 | Desktop rejected QR resets completed steps | **fixed — major behavior** | `desktop-qr-rejected`: steps 1–2 retain green ticks; step 3 remains pending. The residual missing badge fills are a minor in §2, not another reset bug. |
| R9 | Desktop connection-error identity/copy is rejection | **fixed** | `desktop-qr-error`: **Chưa kết nối được Bilibili** and the Viderse-machine network recovery sentence; no phone-cancellation/account-rejection advice. |
| R10 | Desktop connection-error plate is rejection | **fixed** | `desktop-qr-error`: grey globe and **Chưa có mã** on plain white. No orange cross, rejection label, or retained skeleton. |
| R11 | Phone QR instructions over-indented | **fixed** | All six defined phone QR frames plus app-only loading: list box/markers start at **x=16**, padding 0; markers are **22×22**. |
| R12 | Phone terminal-state primary controls too short | **fixed** | `phone-qr-expired/rejected/error/success`: primary **Tạo mã mới/Xong** is **358×48**, radius 13, text 15/600. |
| R13 | Phone same-device action too short | **fixed** | `phone-qr-waiting`: **Mở app** height is **44**, matching `vv-btn--m`; phone glyph is 20px. |
| R14 | Phone scanned sheet keeps same-device card | **fixed** | `phone-qr-scanned`: the entire same-device card is absent; it is present only in waiting. |
| R15 | Phone rejected QR resets completed steps | **fixed — major behavior** | `phone-qr-rejected`: steps 1–2 remain green ticks, not numbers. Remaining completion styling is the shared minor in §2. |
| R16 | Phone connection-error identity/copy is rejection | **fixed** | `phone-qr-error`: network-failure title/body match the board and copy contract; no rejection advice. |
| R17 | Phone connection-error plate is rejection | **fixed** | `phone-qr-error`: grey globe/**Chưa có mã** on white; no skeleton or rejection cross. |
| R18 | Phone success explanation missing | **fixed** | `phone-qr-success`: **App tự gia hạn đăng nhập mỗi ngày.** appears immediately below the green account confirmation. |

**Additional R1 reconciliation:** the Settings sidebar right divider is now present. The omitted **Giới hạn từng kênh** remains accepted under #19, not a defect. The current navigation also has the shared rounded rail, neutral Settings selection and Duyệt review glyph.

The orange-dot integration was checked beyond the screenshots: `internal/platforms/service.go` produces `login_expired` attention for failed Bilibili renewal and YouTube `cookies_failed`; `serve.go` registers platform attention; `SettingsScreen.tsx` explicitly consumes that kind for the Accounts marker. The captures use a consistent edited status payload, so this is evidence for the consumer/rendering and a source trace of the producer, not a claim that the intercepted fixture exercised a real expired cookie against Bilibili.

## 2. Remaining differences

One bullet is one distinct counted visual difference; shared occurrences are not multiplied by state. Only the first bullet is a remaining major. The first three source-backed fixes are independent of the font/QR reference limitations below.

- [major] desktop · YouTube login-control layout, all base states and `yt=bot|off`: artboard places the browser selector **below** its label/help, full content width (**x=297, width=726, height=36**); app places a **220×36** selector at **x=803**, inline to the right of the label/help. The YouTube card loses a full control row and all following source cards move upward by roughly 48px. Evidence: `desktop-saved`, `desktop-blocked`, `desktop-yt-bot`, `desktop-yt-off`, and their SELECT/SECTION measurements. This is not font fallback: the board selector has `width:100%`, whereas app `.sacc-yt` / `.sacc-select` keep it inline. Restore the rendered artboard flow in `apps/viderse/web/src/screens/settings/accounts/accounts.css:124–126`.
- [minor] desktop + phone · QR step-state styling for `scanned/rejected/success`: artboard has translucent **green filled circles** around completed ticks, a translucent **amber circle** around the current scanned step, and dimmer completed instruction text; app has bare ticks and an unfilled amber number, with less differentiation of completed labels. DOM evidence confirms completed/current app badge backgrounds are **`rgba(0,0,0,0)`**, not the board fills. `.sacc-qr__step--done` and `--now` reference **undefined `--sacc-ok-12` / `--sacc-warn-16`** in `accounts.css:275–276`; the board uses `rgba(166,237,193,.12)` and `rgba(246,203,140,.16)`. The completion state itself is now correct; this residual does not keep R8/R15 major.
- [minor] desktop `bili=manual|empty` and every desktop QR state · scan glyph: artboard uses **`i-scan`**, the simple scan-corner glyph; app uses **`scanner`**, the rounded rectangular scan glyph, both on the account QR action and the dialog header. See `BilibiliLogin.tsx` QR actions and `QrLogin.tsx:400`. The 30px account-action heights are now correct and are not part of this finding.
- [minor] desktop, every QR state · close glyph: artboard close-circle glyph is **14×14**; app explicitly renders it **16×16** in `QrLogin.tsx:405`, making it larger and visually farther inward. The app close button is 36px high; this is **not** a request to incorrectly shrink a default `vv-btn` height to 32px, and the focus ring is not counted as a defect.
- [minor] desktop + phone · shared typography/reference environment: artboard actually renders the inspected Twitch description with **`.SF NS` / `.SFNS-Regular`**, while app uses the embedded **`Be Vietnam Pro` / `BeVietnamPro-Regular`**. Both declare the same Be Vietnam Pro family and the same inspected **12.5px/18.125px** metrics, but the actual glyph provider differs. This explains the narrower reference lettering and the app Twitch-description extra line on phone. Proven by **`CSS.getPlatformFontsForNode`**, not merely a font-family string; see `fontchecks.json`. This is a remaining visual/reference-environment difference, not evidence that the app should abandon the specified font or reduce type sizes.
- [minor] desktop + phone · QR artwork footprint in code-bearing states: artboard matrix reaches the plate padding; the real QR image retains its own quiet zone inside the nominal image box, so its visible ink occupies a smaller central square. The outer geometry is now correct (**desktop 200px plate/172px image; phone 196px plate/168px image**). See `desktop-qr-waiting` and `phone-qr-waiting`. QR bits/density are live and excluded. This is the same framing observation retained from QA2, **not** a claim of unscannability or an instruction to remove a required quiet zone.

### Fixed former minors and reference-sensitive observations

The retained fresh captures also show: neutral outer blocked-card borders on both viewports; the desktop blocked **từ <time>** clause; right-pointing collapsed manual-cookie chevrons; 30px empty/manual desktop QR actions; 16px/600 desktop dialog title and 18px/600 phone sheet heading; top-aligned banner icons/security note; correct muted/blue/amber/green QR status text; sharp-but-faded scanned/success images rather than blur; no failed-creation skeleton; correct phone scanned sentence without the desktop **để xong.**; text-only 48px phone retry controls; and back/title horizontal alignment at approximately x=53. These are not carried forward from stale round-2 images.

The phone terminal sheets have their 12px body gaps and correctly sized controls. Their bottom-position offset relative to a board containing a simulated home-indicator strip is not charged as an additional spacing bug. Likewise, live countdown fill is not mistaken for an incorrect track width.

## 3. Coverage and states not comparable

Every requested **app** state was exercised and captured. The following comparisons cannot honestly be called artboard matches because the reference does not define them:

| State | What was checked | Limitation/reason |
|---|---|---|
| Phone `yt=bot` | Intercepted real platform response into the bot state; captured the actual orange YouTube issue card/banner and local retry control. | MSettings has no `yt` property or bot frame. The `art-phone-yt-bot.png` capture is the unchanged baseline, not a dedicated bot design. Entry #29 documents the gap, but is **outside** the explicitly accepted #18–#26 set; it is not treated as approved visual parity. |
| Phone `yt=off` | Saved=false platform fixture; captured **Chưa chọn · đổi trên máy tính**. | MSettings YouTube profile row is static and has no off frame. The saved-profile baseline capture is retained only to document that absence. |
| Phone `qr=loading` | Intercepted pending QR creation; captured the actual skeleton/status/instructions sheet. | MSettings QR enum omits loading, so the corresponding reference URL yields no QR sheet. The baseline reference capture is not claimed to match. |

**Desktop `yt=off` is no longer a coverage gap:** the Settings artboard has a real **Tắt** selector option and its handler updates the off status. Selected that option in the reference tab, then captured `desktop-yt-off` against the app off fixture; did not rely on an unsupported `yt=off` enum alone.

Also covered **blocked → QR success**, both viewports, in `*-blocked-qr-success`: success has the cleared-wait explanation; ordinary success has the automatic-renewal explanation instead. To reach this legitimately without clicking a refresh action, the app fixture used a **paused/manual saved login**, whose actual QR action opens the dialog. Its intercepted successful poll exercises `pausedUntil` forwarding to the QR body. The background manual-vs-QR method difference is live fixture data; the foreground blocked-success copy matches. This does not claim a real server-side unpause, since success was intercepted.

No QR was scanned, no same-device deep link was opened, and no external login was completed, as required. This review therefore does not certify real platform authentication or server-side login/unpause mutations.

### Accepted deviations and weak excuses

- **#19**: missing per-channel-limits navigation remains accepted. Avoiding a dead route is credible; it does not excuse other shell differences. The old missing divider is actually fixed now.
- **#25**: the timestamp comes from the last error and the bot banner uses **một số video**, not a fabricated affected-video count. Accepted and not counted. The blocked banner now includes the full **từ <time>** clause; acceptance was not used to justify omitting it. The usual semantic limitation of last-error time versus a persisted pause-start time remains a documented limitation, not a newly demonstrated runtime bug.
- **#18, #20–#24, #26**: respected; no Accounts defect was invented from these other-area/data/button decisions. In particular, **per-spot class sizes**, not a blanket compact-button rule, were used throughout.
- The **font explanation would be weak if presented as an installed Be Vietnam Pro in the reference or as proof that the pixels match**. Actual font-provider evidence here establishes system fallback in the reference. It accounts for a remaining visual difference but cannot excuse the independent full-width selector or undefined completion-fill tokens.
- No stale implementer screenshot was used to mark a fix. Local API fixtures prove client presentation for the chosen wire state, not real Bilibili login/renewal outcomes.

## 4. Counts

| Round-2 major reconciliation | Count |
|---|---:|
| Round-2 majors checked (R2–R18) | 17 |
| Original major behavior visibly fixed | **17** |
| Original major behavior still wrong | **0** |
| Original major scenarios unverified | **0** |
| New layout major established by live comparison | **1** |

| Remaining distinct differences | Major | Minor |
|---|---:|---:|
| Desktop YouTube selector layout | 1 | 0 |
| Shared QR step-state styling | 0 | 1 |
| Desktop scan/close glyphs | 0 | 2 |
| Shared actual-font reference difference | 0 | 1 |
| Shared QR artwork footprint | 0 | 1 |
| **Total visual differences** | **1** | **5** |

Of the five minor visual differences, three have direct app-code fixes (step styling, scan glyph, close glyph); font fallback and QR quiet-zone framing are explicitly reference/rendering observations, not a recommendation to damage the specified typography or a valid QR.

| Evidence/coverage | Count |
|---|---:|
| Requested app viewport/state captures | 28 |
| Additional blocked→success app captures | 2 |
| Total app captures / reference captures | 30 / 30 |
| Valid matching-state desktop comparisons | 15 |
| Valid matching-state phone comparisons | 12 |
| App-only states with baseline-only reference | 3 |
| Real external platform logins completed | 0 |

Cleanup: browser closed, isolated server stopped, QA database dropped, and temporary binary/HOME/cookies/fixtures/harness removed; only `shots/` retained. No application source was changed. No build/test/lint commands beyond the explicitly requested Go build/migration and this live visual QA were run.

Status: DONE
Summary: **1 major remaining, 5 minors remaining.** All **17 round-2 majors are fixed** in fresh live evidence. Worst 3 actionable residuals: **(1)** desktop YouTube selector is inline/220px instead of the full-width next row, shifting subsequent cards; **(2)** completed/current QR step fills disappear because their CSS variables are undefined; **(3)** desktop QR entry/header still use the wrong scan glyph. Three phone app states were exercised but lack dedicated artboard counterparts, so no false match is claimed.
