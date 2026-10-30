# QA4 — Accounts · focused live re-QA

## Result

**0 majors remaining; 2 minor visual residuals.** The full-width selector major is fixed. QR safety passes the requested two-module minimum on both viewports. The scan-glyph fix is **not** visually complete: changing the icon name selected an asset which still differs from the inline artboard glyph. Completion fills now render correctly, apart from a small amber-alpha mismatch.

No new major or regression of the 17 round-2 majors was found. This report records only fresh evidence from the rebuilt binary, not screenshots supplied by implementers.

## Build, isolation and evidence

- Rebuilt the Go binary with the requested toolchain/CPU/lock/nice command after the announced **14:48:18** distribution rebuild. No web build was run by this reviewer.
- Every retained app capture loads **`/assets/index-CndEvHKK.js`**, recorded in `measurements.json`; this independently confirms the intended embedded bundle.
- Fresh database: `viderse_smoke_qa4accounts`; fresh local administrator created through setup verify/admin; server `127.0.0.1:18783`; own HOME under `/tmp/viderse-qa4/Accounts/home`.
- Fresh Chrome process/profile: `/tmp/viderse-qa4/Accounts/chrome`, using the requested Chrome executable, `--headless=new`, mock keychain/basic password store, and first-run suppression. Exactly two page tabs were used: app and the reference artboard service on `127.0.0.1:18700`.
- Used direct Chrome DevTools Protocol because this worker has no JS-eval browser kernel. The required credential-deletion init script was applied before navigation, and **all 30 app captures record `PublicKeyCredential === undefined`**. Browser HTTP(S) requests outside `127.0.0.1`/`localhost` were denied. No external page/action, QR scan, same-device deep link, or platform login was completed.
- States came from edited real platform/status/profile/QR response shapes. **Waiting alone used real Bilibili QR creation/polling**; all other QR phases used local interception. Sidebar status attention was edited consistently with the corresponding expired/bot platform fixture. This verifies client presentation and routing, not actual platform login/renewal/unpause mutations.
- Viewports: **1440×900 desktop**, **390×844 mobile/touch**, scale 1. Foreground capture waits included document/font readiness, finite animations, icon-mask decoding and rendered frames.

Evidence root: **`/tmp/viderse-qa4/Accounts/shots/`**.

- `art-<state>.png` / `app-<state>.png`: full original screenshots.
- `pair-<state>.png`: comparison crop, **art left / app right**; originals remain unchanged.
- `measurements.json`: text, boxes, colors, masks, loaded bundle and credential status for each state.
- `generated-qr-desktop.svg` / `generated-qr-phone.svg`: actual real-start responses corresponding to the displayed waiting QR.
- `qr-margin-analysis.json`: SVG-module and rendered-raster margin calculations.
- `art-scan-glyph.svg`, `app-scan-glyph.svg`, `scan-glyph-comparison.json`: extracted actual glyph geometry, without the large metadata envelope in the shared asset.

## 1. Every QA3 finding rechecked

| QA3 finding | QA4 disposition | Fresh evidence |
|---|---|---|
| **Major — desktop YouTube selector inline/220px rather than full-width next row** | **Fixed** | App and art selectors are **x=297, width=726, height=36**, below label/help. For blocked/saved/manual/expired/empty/off the measured y coordinate is **337.671875** in both. The following source-card positions now agree instead of losing a control row. Bot also has the full-width next row; its longer accepted live-data wording can make its banner wrap. See `desktop-saved`, all other base pairs, `desktop-yt-bot`, `desktop-yt-off`. |
| **Minor — missing completed/current QR step fills and insufficient completion differentiation** | **Not fully fixed; minor residual only** | Completed green circles are now **rgba(166,237,193,0.12)** and labels are dimmed; both match the board. Current scanned step has a visible amber circle now, but app is **0.14 alpha**, board **0.16**. Both viewports show this measured residual; see §2. No missing fill or step-reset regression remains. |
| **Minor — wrong scan glyph on desktop QR actions/header** | **Still wrong** | Current callsites use `scan`, but the rendered mask **`scan-BEQ-KSAM.svg`** contains an **extra inner rounded rectangle**. The artboard inline `.i-scan` override has only corner brackets plus a horizontal scan line. The difference is visible in `desktop-manual`, `desktop-empty`, and all desktop QR pairs, and is proven by the extracted paths. A name-only check would incorrectly mark this fixed. |
| **Minor — desktop close glyph 16px instead of 14px** | **Fixed** | Actual close mask box is **14×14**, using `close-C9kAW9KH.svg`; see the span/mask measurement in `desktop-qr-scanned`. Button height remains the correct 36px default. The intentional focus ring/tooltip is not a defect. |
| **Minor — reference font fallback** | **Not actionable / not counted**, per assignment | Do not restyle the app to imitate reference fallback. Incidental line-wrap differences attributable to that environment are not charged as new defects. |
| **Minor — QR visible footprint / internal quiet zone** | **Fixed at the requested safe two-module target** | Real generated QR now has exactly **2 modules of intrinsic white border on every side**, without losing that minimum when rendered. Both desktop/phone screenshots match their recorded real-start SVG. The rendered plate margin exceeds 6 module units on every side; detailed proof is below. Live QR bits/density and the required safe quiet zone are not expected to equal the fake artboard bitmap. |

This accounts for all six QA3 rows: **3 fixed**, **2 not fully fixed/still wrong at minor level**, **1 explicitly non-actionable**. The formerly missing green backgrounds and the original missing amber background are fixed as behaviors; only the small alpha residual is carried forward.

## 2. Remaining visual differences

- [minor] desktop `bili=manual|empty` and every desktop QR state · **scan glyph**: artboard has simple scan corners and the middle horizontal line; app still adds an inner rounded rectangle. Evidence: `pair-desktop-manual.png`, `pair-desktop-qr-loading.png`, and `scan-glyph-comparison.json`. The actual app path contains **`M17 9.5v5c0 2-1 3-3 3h-4c-2 0-3-1-3-3v-5c0-2 1-3 3-3h4c2 0 3 1 3 3Z`**, which is absent from the inline artboard override at `Settings.dc.html:745`. The consumer really loads `/assets/scan-BEQ-KSAM.svg`; the incorrect geometry is in **`platform/ui/src/icons/scan.svg:1`**, reached by the Accounts `Icon name=scan` callsites. Use the actual artboard mask geometry for these Accounts scan affordances; do not treat the renamed icon token alone as parity.
- [minor] desktop + phone `qr=scanned` · **current step 3 badge fill**: artboard **`rgba(246,203,140,0.16)`**; app **`rgba(246,203,140,0.14)`**. Both badges are now correctly **22×22** and amber, so this is a small tone residual, not a missing-state or integration bug. `accounts.css:276` uses `--frame-warn-14`, whose definition is 14% alpha; `Settings.dc.html:771` and `MSettings.dc.html:480` specify 16%. See the step-number background measurements and `pair-*-qr-scanned.png`.

**No remaining major, no new difference outside these retained QA3 categories, and no new functional blocker were established.** Accepted deviations #18–#26 and #29–#30 were not charged as defects. In particular: accepted last-error timing/nonnumeric affected-video wording, omitted per-channel-limits navigation, current rail dot tone, missing phone artboard states, font fallback and native OS chrome are excluded appropriately. They do not excuse the independently proven wrong SVG path or 14%/16% alpha mismatch.

## 3. Real QR safety check

This is evidence from real, unscanned waiting QR responses, not only from generator source.

1. The displayed image data URI is decoded and compared with the real-start SVG saved for that viewport; the hashes match in both cases.
2. Both SVGs have a **53×53 module viewBox**, containing a **49×49 QR matrix** plus an intrinsic **2-module border on all four sides**. The black-path module extrema prove those borders directly.
3. Rendered image/plate boxes are read from the browser. Black-ink extents are also measured from the actual waiting PNG inside the image rectangle, then related back to the plate edge and module scale. Normal raster rounding is retained rather than pretending a fractional module edge lands on an exact screen pixel.

| Measurement | Desktop | Phone |
|---|---:|---:|
| SVG viewBox / QR matrix | 53 / 49 modules | 53 / 49 modules |
| Intrinsic white border, L/T/R/B | **2 / 2 / 2 / 2 modules** | **2 / 2 / 2 / 2 modules** |
| Rendered image width | **172px** | **168px** |
| Rendered plate width | **200px** | **196px** |
| Rendered module scale | 3.245283px | 3.169811px |
| Raster plate-edge to black-ink margin, L/T/R/B | **21 / 21.1875 / 21 / 20.8125px** | **20 / 20.28125 / 20 / 19.71875px** |
| Smallest rendered plate margin, normalized | **6.413 modules** | **6.221 modules** |
| Displayed SVG matches real-start response | **yes** | **yes** |
| Intrinsic and rendered ≥2-module checks | **PASS** | **PASS** |

The additional white plate padding is not being mistaken for an absent intrinsic border: the SVG itself independently retains two modules. The renderer also does not crop that border. This confirms the requested margin safety, **not** physical camera scannability; scanning was explicitly prohibited and was not attempted.

## 4. Quick regression pass

All requested app states were captured again: five Bilibili account states, YouTube bot/off, all seven QR phases on both viewports, plus blocked→QR-success on both viewports.

- **Desktop base states:** correct card wrappers/order, outer neutral blocked versus issue outlines, saved/manual/empty account controls, disclosures, full-width selector and right-side safety/help rail. Expired Bilibili and YouTube-bot **orange Accounts sidebar dots are both visibly present** in full screenshots, distinct from other sidebar attention markers.
- **Desktop QR:** 620px dialog frame, blurred/dimmed backdrop, correct loading/waiting/scanned/expired/rejected/error/success presentation; rejected retains steps 1–2; network error is grey globe/**Chưa có mã** with network copy, never a rejection plate or stale skeleton; success has its renewal explanation. Blocked success has the cleared-wait explanation instead. The remaining scan glyph is listed above rather than hidden by these passes.
- **Phone base states:** card order, issue tones, concise account-state controls and explanatory rows remain intact. No extra blocked QR banner action was introduced.
- **Phone QR:** 16px list inset, 22px markers, same-device card only while waiting, correct rejection/error distinction, success explanation and blocked-success explanation. **Mở app = 44px**, terminal **Tạo mã mới/Xong = 358×48px**. Retry primaries remain text-only.
- **Round-2 majors:** the previously repaired blocked action, both sidebar markers, YouTube outline/divider, backdrop blur, rejected-step state, error copy/plates, phone list inset/control heights/same-device visibility and success explanation remain fixed. **0 of 17 regressed.**

### Artboard coverage limits

The three app-only phone states **`yt=bot`**, **`yt=off`**, **`qr=loading`** were exercised and captured, but MSettings has no matching frames/enum states. Their `art-` images are the baseline fallback, not proof of parity; this is now accepted under #29. The corresponding phone comparisons are not included among the 27 matching-state pairs.

Desktop **Tắt** was selected through the real reference selector handler before the off screenshot. As in QA3, the extra blocked→success fixture uses a paused/manual saved login so the actual QR action can open the dialog; only the foreground success explanation is compared against the blocked reference. Intercepted success is not claimed to prove server-side unpause.

## 5. Counts and cleanup

| Finding reconciliation | Major | Minor |
|---|---:|---:|
| QA3 actionable major remaining | **0** | — |
| Scan-glyph residual | 0 | **1** |
| Scanned active-badge alpha residual | 0 | **1** |
| Font fallback / accepted deviations / safe QR live geometry | 0 | 0 |
| **Remaining total** | **0** | **2** |

| Coverage | Count |
|---|---:|
| Requested app state/viewport captures | 28 |
| Extra blocked→success captures | 2 |
| App / art original screenshots | 30 / 30 |
| Matching-state reference comparisons | 27 |
| Accepted app-only phone states | 3 |
| Round-2 majors regressed | 0 / 17 |
| Real waiting QR margin checks passed | 2 / 2 viewports |
| External logins/scans completed | 0 |

Cleanup: Chrome closed; isolated service stopped; QA database dropped; binary, HOME, Chrome profile, cookies, API fixtures and temporary harness removed. Only **`/tmp/viderse-qa4/Accounts/shots/`** remains. No application source was edited and no web rebuild/tests/lint were run. The requested Go build/migration and live visual QA are the exercised verification.

Status: DONE
Summary: **0 majors, 2 minors remaining.** Worst residual is the **actual scan SVG still containing the extra inner rectangle**, despite the renamed icon token. Second is **14% versus 16% amber step fill**. There is no third remaining finding. The selector major and close glyph are fixed, all 17 round-2 majors remain fixed, and the real QR retains the required ≥2-module border on both viewports.


---

## QA5 — focused closure of the two QA4 residuals

**Latest verdict: PASS — 0 majors, 0 minors remaining in the requested focused scope.** Both residuals above are now fixed. This QA5 section supersedes the historical QA4 remaining-count verdict, without rewriting its evidence.

### Fresh environment and scope

- Rebuilt an independent Go binary from the current tree after the **15:18:46** distribution rebuild. Every one of the six retained app captures records **`/assets/index-C1swU8kQ.js`**.
- Fresh database **`viderse_smoke_qa5accounts`**, fresh setup-created local administrator, isolated HOME and Chrome profile under `/tmp/viderse-qa5/Accounts/`, and app port **18783**.
- Own headless Chrome process with mock keychain/basic password-store and first-run suppression; app/reference tabs only. Localhost-only request policy and credential-deletion init script were reapplied; **all six app captures confirm `PublicKeyCredential === undefined`**. No external login/deep-link completion or QR scan occurred.
- Captured the requested **desktop `bili=empty|manual`**, **desktop and phone `qr=scanned`**, plus **desktop and phone real `qr=waiting`**. Desktop is 1440×900; phone is 390×844 with mobile/touch emulation. Scanned uses interception of the local QR API with edited real response shapes; waiting uses real, unscanned Bilibili QR creation/polling.
- Final fixtures retain the real wire-shape discriminator fields: the YouTube browser-cookie type/control remains present, and an empty Bilibili login does not receive the manual-cookie warning. These checks are recorded in `fixtureLayoutSanity` to avoid judging artificial fixture-layout differences.

### Finding → disposition

| QA4 residual | QA5 disposition | Fresh observed proof |
|---|---|---|
| Scan glyph still contains an inner rectangle | **Fixed** | The empty/manual account actions render the correct corners-and-horizontal-line glyph at **14×14**; the desktop QR header renders it at **20×20**. The **actual computed browser mask** is an inlined SVG data URI from the served bundle, not an asset filename assumption. Decoding it yields exactly the artboard inline `.i-scan` root/stroke attributes and path; the unwanted inner rectangle is absent. `servedGlyphMatchesArt = true`. |
| Current scanned step uses 14% rather than 16% amber | **Fixed** | The actual dialog step-3 badge is **22×22** and computes to **`rgba(246, 203, 140, 0.16)`** on both desktop and phone, exactly matching the corresponding artboard badge. The comparison selects the QR dialog marker, not the unrelated top-navigation count badge. Both `exactFillMatch` checks are true. |

The runtime mask was decoded into **`qa5-served-scan-line.svg`** and compared with **`qa5-art-scan-line.svg`**, extracted from the reference inline override. Both paths are exactly:

`M2 9V6.5C2 4.01 4.01 2 6.5 2H9M15 2h2.5C19.99 2 22 4.01 22 6.5V9M22 16v1.5c0 2.49-2.01 4.5-4.5 4.5H16M9 22H6.5C4.01 22 2 19.99 2 17.5V15M5 12h14`

Also confirmed **`sharedScanGeometryUnchanged = true`** against the prior extracted `scan.svg` geometry. Thus the Accounts cutover uses the new `scan-line` token without changing the existing shared `scan` shape used elsewhere. The icon consumer is the existing `Icon` name-to-mask lookup, and the rendered mask proves the new token reaches that consumer successfully.

### Waiting sanity and limited regression check

Both real waiting frames display a fully loaded code and **Đang chờ quét**, with the expected timer, instructions and appropriate footer/same-device presentation. The code was displayed only, never scanned.

| Waiting check | Desktop | Phone |
|---|---:|---:|
| Actual image loaded (`complete`) | true | true |
| Image / plate width | **172 / 200px** | **168 / 196px** |
| Intrinsic quiet-zone modules, L/T/R/B | **2 / 2 / 2 / 2** | **2 / 2 / 2 / 2** |
| Minimum-two-module check | **PASS** | **PASS** |

The empty/manual screenshots also retain the full-width YouTube selector and correct account-row/disclosure structure. Scanned frames retain the completed green steps, the active amber third step and the proper scanned sentence. No new regression was found in these six focused comparisons. Live QR patterns/countdowns, reference font fallback, simulated OS chrome, and accepted deviations are not counted as defects. This was **not** another full replay of every QA4 state; the earlier broader regression evidence remains in the historical section.

### Evidence and counts

All new evidence uses the requested **`/tmp/viderse-qa4/Accounts/shots/qa5-*`** prefix:

- `qa5-app-<state>.png` / `qa5-art-<state>.png`: **six fresh full screenshot pairs**.
- `qa5-pair-<state>.png`: corresponding comparison crops, art left/app right.
- `qa5-measurements.json`: actual masks, dialog badge colors/boxes, loaded bundle and other DOM evidence.
- `qa5-verification.json`: decoded-glyph equality, unchanged legacy scan geometry, exact amber matches, fixture-layout guards and real-waiting checks.
- `qa5-served-scan-line.svg` / `qa5-art-scan-line.svg`: extracted actual/reference glyph geometry.
- `qa5-generated-qr-desktop.svg` / `qa5-generated-qr-phone.svg`: the displayed real waiting QR images.

| QA5 closure count | Count |
|---|---:|
| QA4 residuals rechecked | 2 |
| Residuals fixed | **2** |
| Remaining majors / minors | **0 / 0** |
| Fresh app/reference pairs | **6** |
| New differences or regressions established in focused scope | **0** |
| Real waiting quiet-zone checks passed | **2 / 2** |
| Completed platform logins/scans | **0** |

Cleanup completed: owned Chrome closed, app service stopped, QA5 database dropped, and the QA5 binary/HOME/profile/cookies/fixtures/harness removed. Only screenshot/evidence artifacts remain. No application source was edited; no web build, tests or lint were run. The requested Go build/migration and focused live QA are the exercised checks.

Status: DONE
Summary: **QA5 PASS — 0 majors, 0 minors remaining in scope.** The actual rendered scan-line path now matches the artboard exactly; scanned amber fill is exactly 16% on both viewports. Real waiting QR sanity and its two-module quiet zone pass. No new focused regression found.
