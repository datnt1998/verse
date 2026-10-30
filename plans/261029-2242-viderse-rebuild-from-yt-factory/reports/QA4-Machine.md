# QA4 — Machine

## Verdict

**Current-tree result: 0 major issues; 1 minor residual group across the 6 desktop Settings frames.** Both QA3 majors are fixed. Three of the four QA3 minor groups are fixed; the sidebar attention-dot offset is reduced from 11px to 1px. The selected-row rhythm and group-label typography now match, and the top bar stays exactly 55px in every base and expanded state checked.

**Bundle identity caveat:** the assignment named index-DqM0zDuC.js, but that asset was not present in the supplied dist. The current dist/index.html and the independently built/served browser document both reference **index-CndEvHKK.js**. This report verifies that actual current-tree bundle; it does not claim to have exercised the absent DqM0zDuC bundle. See shots/bundle-identity.json and the scripts field in each app capture JSON. No web rebuild or source edit was made by this reviewer.

## Execution

- Built a new Go binary using the requested Go 1.26.0, GOMAXPROCS=4, lockf and nice command. Used isolated **HOME=/tmp/viderse-qa4/Machine/home**, database **viderse_smoke_qa4machine**, and port **18781**. Created the local administrator through setup verify/admin, without an external login.
- Drove a new isolated headless Chrome with mock keychain, basic password store, no-first-run and no-default-browser-check. The session used its own automatically generated --user-data-dir; the owned Chrome process command line is retained in processes-before-cleanup.json. The CLI rejects combining its localhost allowlist with a manually supplied --user-data-dir path, so the requested explicit path was not used. The browser remained limited to **127.0.0.1 and localhost**, and the init script deleted PublicKeyCredential.
- The provided tool inventory still lacked browser.open/tab.run/tab.box. Used the installed agent-browser CLI to control one browser session and two tabs. This is a disclosed tool-protocol substitution, not reuse of implementer screenshots.
- Compared app and artboard at **1440×900**, and **390×844** with touch/mobile emulation. Explicitly set the viewport on both tabs. Injected the local Be Vietnam Pro and JetBrains Mono font assets into the artboard tab and waited for fonts/icons before capturing.
- Checked fresh setup and the signed-out phone row with real backend responses. For otherwise unavailable states, edited saved real API response shapes and intercepted the required local API paths. The CLI matches routes by substring, so storage child paths and provider model paths were registered before their parent endpoints. An early fixture-routing exception was diagnosed as returning the parent storage response to the check request, corrected, and excluded from product findings.

Evidence root: **/tmp/viderse-qa4/Machine/shots/**. Pairs use art-<state>.png and app-<state>.png, with JSON recording visible text, viewport, served script URLs, boxes and computed styles.

## 1. QA3 findings rechecked

| QA3 finding | QA4 result | Independent current evidence |
|---|---|---|
| **Major:** fresh setup displays installing but never starts | **Fixed** | fresh-autostart.json records a real GET before mounting with state=idle, followed by real responses with state=running and running=true. No diagnostic/manual POST was made. The request record includes the automatic /api/setup/start request. app-setup-running-real.png shows actual ffmpeg download progress while transcription is queued. |
| **Major:** signed-out phone ChatGPT row says signed in | **Fixed** | real-mobile.json still reports codex-login state=needs_login. app-msettings-tools-real.png shows Cần bạn and the sign-in-required explanation, rather than Đã đăng nhập. The long explanation is clamped to the normal two phone lines; the row title itself is Đăng nhập ChatGPT. |
| **Minor:** failed tool explanation rendered neutral gray | **Fixed** | tools-failed pair and JSON show the failed transcription explanation in orange, rgb(255,192,154), matching the artboard warning explanation tone. The summary retains Kiểm tra lại and the row has one Tải tiếp CTA. |
| **Minor:** unresolved logs dot/status used yellow instead of orange | **Fixed** | frame-logs pair uses the populated real errors-feed contract. The unresolved dot background and retry-status text are both rgb(255,154,92), the artboard #FF9A5C. Resolved rows remain muted. |
| **Minor:** sidebar attention dot was 11px too far right | **Partially fixed; 1px residual** | dot-metrics.json shows app x=211–218 versus artboard x=210–217, with the same y, 7px diameter and rgb(246,203,140). The app sidebar inner content is 228px wide versus the artboard 227px; its right-aligned dot/row edge retains a 1px displacement. This is one shared minor group, not a new major. |
| **Minor:** expanded phone details hide tool names | **Fixed** | msettings-tools-expanded pair shows yt-dlp, ffmpeg · ffprobe, faster-whisper, Chrome and Codex labels next to their values. Missing or varying version values are live installation data, not missing labels. |

### Real fresh-machine check

1. Start the new binary with a newly created HOME and the isolated migrated database.
2. Create the local administrator, then read /api/setup/state before opening setup. The real response is idle and incomplete, with sufficient disk.
3. Open #/setup and wait for the step list. The real backend becomes running without an explicit test POST; it remains running in a subsequent response after 1.5 seconds.
4. The real screenshot shows ffmpeg at 46%, including received/total bytes, and the following model step queued. This is not an intercepted running response.

This closes the exact QA3 functional blocker rather than merely changing its summary label. The intercepted setup-running/login/failed pairs were additionally compared for the requested visual states. Focused login and failed CTAs remain **36/r10/13**; the non-focused running login action remains the 32px ghost treatment.

## 2. Additional frame checks

### Header height

Both the app and corresponding artboard header measured **y=0, height=55px** in every row below. The complete computed boxes are in frame-metrics.json and expanded-header-metrics.json.

| Surface/state | App | Artboard | Result |
|---|---:|---:|---|
| Tools | 55 | 55 | Matches |
| Connect | 55 | 55 | Matches |
| Agents, after the real .agt-card loaded | 55 | 55 | Matches |
| Accounts | 55 | 55 | Matches |
| Storage | 55 | 55 | Matches |
| Logs, populated | 55 | 55 | Matches |
| Storage picker | 55 | 55 | Matches |
| Storage low-space picker | 55 | 55 | Matches |
| Storage cloud-folder picker | 55 | 55 | Matches |
| Storage moving | 55 | 55 | Matches |
| Storage cleanup preview | 55 | 55 | Matches |
| Connect model chooser expanded | 55 | 55 | Matches |
| Connect manual-model entry expanded | 55 | 55 | Matches |

The Storage and Connect expansion captures exercise shell geometry under their larger bodies; they are not a replacement for the separately owned content QA of those areas. No actual storage move, cleanup deletion, external authentication or model mutation was performed.

### Sidebar row rhythm and typography

All six base frames now have exactly the same row coordinates and heights as their artboards, without the earlier extra 2px-per-row drift:

| Sidebar row | App and artboard y | App and artboard height |
|---|---:|---:|
| Công cụ trên máy | 96.8125 | 34 |
| Kết nối AI | 132.8125 | 34 |
| Trợ lý AI | 168.8125 | 34 |
| Tài khoản nền tảng | 204.8125 | 34 |
| Lưu trữ | 280.625 | 34 |
| Báo lỗi | 316.625 | 34 |

All three group labels are **11.5px**, with **17.825px line height**, matching the artboard. Their y positions also match. See sidebar-rhythm.json and group-font-metrics.json. The remaining 1px horizontal inner-width difference is isolated below; it is not a vertical-rhythm regression.

## 3. Remaining difference

- **[minor] Settings frame on tools/connect/agents/accounts/storage/logs · sidebar inner right edge and attention dot:** artboard sidebar content is 227px wide and its empty-count attention dot occupies x=210–217; app content is 228px wide and the dot occupies x=211–218. The prior 11px error is mostly removed, but a 1px horizontal inset remains. Vertical position, color, diameter, selected-row height, row spacing and group-label typography match. Evidence: dot-metrics.json, group-font-metrics.json and dot-measure pair. Preserve the artboard inner border/content width instead of compensating by moving only the dot.

**No new major or functional regression was found in the checked current-tree states.**

## 4. Coverage and exclusions

| Check set | Coverage |
|---|---|
| QA3 majors/minors | All 6 finding groups rechecked |
| Onboard setup | Real fresh automatic start, plus running/login/failed pairs |
| Tools | ok/update/failed/installing pairs; advanced row remains inside desktop tool card |
| Logs | Populated unresolved/resolved rows; primary export and ghost version-check roles |
| Base desktop frame | All 6 sections; top bar, selected sidebar row, dots, footer and right rail |
| Phone | list/tools pairs, real signed-out tools row, expanded details |
| Extra frame stress states | Storage pick/lowspace/cloud/moving/cleanup and Connect models/manual |

The exact named **index-DqM0zDuC.js** bundle was not checked because it was absent. The live **index-CndEvHKK.js** bundle was checked instead and is the only index JS asset referenced by the supplied dist. Full install completion, external login, real model edits, storage moves and destructive cleanup were not exercised.

Current accepted deviations **#18–#30** were excluded from findings. This includes tool versions/sizes, built-in Chrome behavior, ETA/live counts, missing fixed capacity text, local semver, ghost size differences already accepted, the channel-limits shortcut, native phone status/home bars, provider/account empty defaults, assistant guidance labels and the gear-dot color behavior in #30. Live provider count and selected model differences were not promoted into visual defects.

## 5. Counts

| Measure | Major | Minor |
|---|---:|---:|
| QA3 issue groups fully fixed | 2 | 3 |
| QA3 issue groups partially fixed | 0 | 1 |
| New issue groups | 0 | 0 |
| **Remaining issue groups** | **0** | **1** |
| **Remaining affected-state occurrences** | **0** | **6** |

The single remaining group is a 1px sidebar inset, not a release-blocking functional problem. The bundle-identity caveat is reported separately and is not a fabricated UI finding.

## Cleanup

Closed the owned browser, stopped the owned server and installer children, dropped viderse_smoke_qa4machine, and removed the owned homes, binary, profile/init scaffolding and capture drivers. Only shots/ evidence was retained. The owned process-tree snapshots and final remaining-process check are retained with the evidence; no owned uv/pip installer process remained.

Status: DONE
Summary: 0 major groups, 1 minor residual group across 6 frames. Both QA3 majors are fixed; all 13 checked base/expanded headers are 55px; sidebar row rhythm and group typography match. This verdict applies to the actually served index-CndEvHKK.js, not the absent requested index-DqM0zDuC.js.


---

## QA5 — Focused sidebar closure

### Verdict

**PASS — 0 major issues and 0 minor issues in the focused QA5 checks.** The remaining QA4 1px sidebar inset is fixed in all 6 desktop sections. The right-side padding correction did not move selected-row text or compress the header. The quick setup=running sanity pair also passed.

The independently built and served app loaded **index-C1swU8kQ.js**, matching the 15:18:46 dist named for this pass. The earlier QA4 bundle caveat is historical and does not apply to this QA5 build. The served script URL is recorded in every qa5-app-*.json capture.

### Environment and method

- Built a new binary from the current tree with Go 1.26.0, GOMAXPROCS=4, lockf and nice; did not rebuild the web app or edit source.
- Created/migrated fresh **viderse_smoke_qa5machine**, used port **18781**, and a new HOME under /tmp/viderse-qa4/Machine/qa5-runtime/home. Created the local administrator through setup verify/admin.
- Used a fresh named headless Chrome session with mock keychain, basic password store, no-first-run, no-default-browser-check, a separately owned ephemeral --user-data-dir, the localhost-only allowlist, and the PublicKeyCredential deletion init script. The CLI/tool-protocol substitution disclosed for QA4 was unchanged.
- Compared two independently driven tabs at **1440×900**, loading local reference fonts and waiting for fonts/icons. Captured actual DOM ranges for the selected label text, client-width-minus-padding for inner content width, dot boxes, and header boxes.
- After the real setup sanity capture, edited the real tools/status response shapes to present one update dot without an installation count. This keeps the same empty-count attention-dot case visible in every section. No external login, model edit, storage move or destructive cleanup action was performed.

### Measured result

Every cell below is **app = artboard**, not an estimate from the images:

| Desktop section | Sidebar inner width | Attention-dot x range | Selected text x | Header height |
|---|---:|---:|---:|---:|
| tools | 227px | 210–217px | 47px | 55px |
| connect | 227px | 210–217px | 47px | 55px |
| agents | 227px | 210–217px | 47px | 55px |
| accounts | 227px | 210–217px | 47px | 55px |
| storage | 227px | 210–217px | 47px | 55px |
| logs | 227px | 210–217px | 47px | 55px |

The app retains its **inset-shadow divider and no physical one-sided border**. Its 10px left / 11px right padding yields the same 227px content width as the reference 10px / 10px padding plus 1px border. The selected label text stays at x=47px; narrowing the right edge did not shift the icon/text group. Dot diameter, y position and color also match in the retained metrics.

### Setup sanity

The real backend was **idle, incomplete, with enough disk** before visiting setup. After the screen mounted it became **running=true, state=running**, without a diagnostic/manual start request. The real screenshot shows the active ffmpeg transfer row and following queued model row in the expected setup layout. The artboard/app pair was reviewed with live tool/transfer values excluded as accepted deviations. This was a running-state sanity check, not an assertion that the full multi-gigabyte installation completed.

### Evidence

All new evidence is retained under **/tmp/viderse-qa4/Machine/shots/** with a qa5- prefix:

- **qa5-metrics.json**: complete app/art measurements for all 6 sections, including label DOM-range boxes, outer/inner sidebar dimensions, padding, divider style, dot bounds and header boxes.
- **qa5-app-{tools,connect,agents,accounts,storage,logs}.png** and matching **qa5-art-*.png**: six fresh desktop comparison pairs, with corresponding JSON metadata.
- **qa5-app-setup-running.png / qa5-art-setup-running.png** and **qa5-setup-real.json**: quick running sanity pair and real pre/post backend states.
- **qa5-baselines.json**: real API responses used as the base for the attention-dot fixture.
- **qa5-processes-before.json**, **qa5-processes-at-cleanup.json**, **qa5-installer-processes-at-stop.json**, **qa5-profile-cleanup.json**, **qa5-processes-after-cleanup.json**: ownership and cleanup evidence.

### Remaining differences and cleanup

**No remaining difference was found in the requested QA5 checks.** Accepted deviations #18–#30 were not reopened. This focused pass closes the one QA4 residual; it does not pretend to repeat every unrelated content/interaction check from QA4.

Closed the owned browser, stopped the server and any still-owned installer descendants, dropped viderse_smoke_qa5machine, removed the owned profile and qa5-runtime directory, and retained only shots/ evidence. The final process snapshot reports **remainingOwnedProcesses=[]**, including no surviving owned uv/pip installer process.

Status: DONE
Summary: QA5 passes on index-C1swU8kQ.js. 0 majors, 0 minors in scope; the QA4 1px residual is closed. All 6 sidebar inner widths are 227px, dots occupy x=210–217, selected text remains x=47, headers remain 55px, and the real setup=running sanity check passes.
