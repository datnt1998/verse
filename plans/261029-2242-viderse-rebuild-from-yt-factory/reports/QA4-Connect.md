# QA4 — Connect / Agents

## Verdict

**The QA3 major is fixed. Five of six QA3 minors are fixed; the phone add-sheet spacing is not fixed and has overcorrected in the opposite direction. Current remainder: 0 majors, 1 minor.**

This is an independent live check of the new embedded bundle, not reuse of the QA3 application captures. No product source or dist was edited.

## Method and evidence

- Built `/tmp/viderse-qa4/Connect/viderse` using the prescribed Go 1.26.0 / GOMAXPROCS=4 / heavy-lock command. **Did not rebuild the web.** Runtime loads **`/assets/index-BUWsrZiE.js`**, confirming the requested new bundle.
- Migrated a fresh `viderse_smoke_qa4connect` database and served the binary on **127.0.0.1:18784** with a temporary admin and cookie jar.
- Launched one owned headless Chrome with the required flags, fresh profile, localhost-only allowlist, and removal of PublicKeyCredential. Used one browser launch and one AI-interception registration in the local Node JS/CDP evaluation harness. A kernel control-port collision was recovered by attaching to that same owned Chrome, not by launching another browser.
- Intercepted only needed `/api/ai/...` paths using real GET baselines plus the Go wire DTOs. No paid API was called; all keys were explicitly fictional. To test persistence rather than merely echoing a mock, the final zero-model **UI Lưu request was forwarded to the real local backend** against a temporary database provider fixture.
- Captured and inspected **all 17 primary artboard/application pairs** at desktop **1440×900** and phone **390×844**, mobile/touch enabled. Evidence is in **`/tmp/viderse-qa4/Connect/shots/`**, as `art-<state>.png` / `app-<state>.png`, plus computed geometry/style JSON and interaction proofs.
- Loaded the locally bundled Be Vietnam Pro / JetBrains Mono faces into the reference tab, avoiding blocked Google Fonts and fallback-font false positives. Paused finite CSS transitions were advanced to their final frame before captures; no layout or product styles were changed. Phone status-bar-only chrome is excluded; bottom sheets are compared against the same viewport bottom.
- Read the current hỏi-đáp entries through **#29**. Accepted #18–#26/#28 data/editorial deviations and #29 Accounts-only changes are excluded; none excuses the remaining Connect footer spacing.

## 1. Every QA3 item: fixed / not fixed / regressed

| QA3 item | QA4 result | Current live evidence |
|---|---|---|
| **Major — `conn-added` has no initial Huỷ/Lưu strip with zero models** | **FIXED** | Followed the real add UI, entered a fictional key, and accepted an intercepted successful provider creation. Without touching any model checkbox, the new provider has **`selected:[]`** and the full `1 thay đổi chưa lưu ở Kết nối AI · dùng cho cả máy này` / **Huỷ / Lưu** strip. `conn-added` pair and `add-save-cancel.json`. |
| Minor — overflowing desktop content shrinks top chrome | **FIXED** | Header remains **55px** on `conn-models` and `conn-manual`, matching the artboard, instead of shrinking to 48px. `app-conn-manual.json` / `art-conn-manual.json`, `HEADER.h=55`; page introduction and sidebar baseline stay in place. |
| Minor — desktop catalog checkbox colors | **FIXED** | Desktop unchecked native boxes are **white**, checked boxes **pink with white ticks**, as in the artboard. Verified both the changed-selection catalog and manually added row. `conn-models` / `conn-manual` pairs. Phone remains pink with dark ticks as its own design requires. |
| Minor — opening artist picker scrolls away desktop page heading | **FIXED** | Opening the artist picker leaves the Trợ lý AI heading visible and the center scroller unchanged: **heading y=81, scrollTop=0 before AND after**. `artist-scroll.json` and `agents-artist` pair. |
| Minor — phone add-sheet vertical spacing | **REGRESSED / NOT FIXED** | QA3 placed the primary about 12px too low. Current app places it **26px too high**: **y=732**, versus artboard **y=758**, both **48px** high. Current title y≈267.6 versus artboard 295. Captured after the animation ended, including a direct geometry capture. `m-add` pair and `mobile-add-layout.json`. See the actionable diagnosis below. |
| Minor — key icon before phone picker footer | **FIXED** | Both phone director and artist footers now start with plain **Thêm model ở Kết nối AI** text at x=16, with no leading key icon. `m-director` / `m-artist` pairs. Desktop correctly retains its separately designed icon footer. |
| Minor — paid-source daily-limit wording | **FIXED** | Desktop Gemini card says **máy tự tiêu tối đa 50 ảnh/ngày**; phone says **tự tiêu tối đa 50/ngày**, matching the appropriate artboard copy instead of the longer ảnh mỗi ngày wording. Phone limit copy is back on one line. `agents` / `m-agents` pairs. The fixture limit and prices themselves are data and not counted. |

## 2. Explicit Huỷ / Lưu acceptance checks

### Newly added, still zero selected

`app-conn-added.png` was captured **before any checkbox interaction**. It contains the full unsaved strip, yellow connected/choose-model message, Gợi ý tab, unchecked models, and `Chưa chọn model nào.` The major is not being hidden by substituting a later model-dirty state.

### Initial Huỷ, before any model selection

Clicked **Huỷ directly from the newly added zero-selected state**:

- Strip disappeared.
- No PATCH/save request was issued; the request log contained only the completed provider-creation POST.
- The model selection remained empty.
- The connected provider remains in the list: cancellation discards the pending settings draft, not the already completed connection transaction. This is stated explicitly rather than implying provider deletion was tested.

Proof: `initial-cancel-real-zero-save.json` → `initialCancel`, plus `app-added-initial-cancel.png`.

Also tested Huỷ after checking a model in the zero-selected baseline: the strip disappeared, checked rows returned to **0**, and saved selection remained **[]**, with no extra PATCH. Proof: `add-save-cancel.json` → `cancelDraft`, `app-added-cancelled-draft.png`.

### Lưu with zero selected, including real persistence

1. Added a fresh fixture provider through the UI again and left every model unchecked.
2. Clicked **Lưu** immediately. It was enabled; there was no requirement to first select a model.
3. UI emitted **PATCH `/api/ai/providers/openrouter` with `{"selected":[]}`**.
4. For this request only, the interceptor forwarded the actual request to the real local API with the temporary admin cookie. The provider row was separately seeded in the isolated database; no real credential or provider network operation was involved.
5. Real local backend returned **HTTP 200**, `selected:[]`; the UI removed the strip.
6. Database inspection showed **`openrouter | []`** after replacing the prior control value `["qa-before-zero-save"]`. Thus empty selection was actually persisted, not merely supplied by an echoing mock.

Proof: `initial-cancel-real-zero-save.json` → `realSave` / `afterRealSave`, `app-added-real-saved-zero.png`, and the observed PostgreSQL query. The earlier UI fixture reload also retained empty selection without restoring a dirty strip (`add-save-cancel.json` → `reloadAfterSave`).

## 3. Remaining differences and discrete fix

- [minor] **MSettings `screen=connect&sheet=add` · bottom-sheet footer spacing:** artboard primary is **y=758–806** and title **y=295**; app primary is **y=732–780** and title **y≈267.6**. The entire content block is too high and leaves 26px too much blank space below the action. This is a regression of the existing QA3 spacing item, not a new major. **Cause is measured, not inferred:** the app `.conn-sheet-primary` has **padding `12px 16px 38px`**, and the enclosing shared Sheet **also renders its own 26px `.m-sheet-home` row**. The add footer counts the home row twice. In `apps/viderse/web/src/screens/settings/connect/connect.css:305`, return the footer bottom padding to **12px**, leaving the existing 26px Sheet home row to supply its own space; retain the corrected 12px label/input gap. Update the nearby comment so it does not instruct future maintainers to add the home row again. See `mobile-add-layout.json`: `.conn-sheet-primary` y=720, h=98, padding-bottom=38; `.m-sheet-home` y=818, h=26. Both use the same viewport, so the phone status-bar exception does not explain the 26px delta.

**New independent regressions found beyond the carried QA3 item: none.** No remaining major was observed. Inventory-driven row counts, model notes/group ordering, unavailable plan/spend/reset values, and associated content-driven heights are excluded under the accepted deviations rather than counted as fresh styling bugs.

## 4. Regression coverage

All requested states were re-captured and compared, including the fixes that were already correct in QA3.

| Area | States | Result |
|---|---|---|
| Desktop Connect | `conn-list`, `conn-add`, `conn-added`, `conn-models`, `conn-manual`, `conn-failed` | 6/6 pairs inspected. Zero-selected added state, correct full-card/catalog wrapping, manual checked row + Thêm theo tên + green success, failure warning/actions, and button classes checked. |
| Desktop Grok | `signed` in `conn-list`; `grok-code`; `grok-out` | All 3 values checked; 2 additional pairs. Pending code, copy/action/cancel/expiry UI and warm-yellow status still present; no external login clicked. |
| Desktop Agents | `agents` (`ai=none`), `agents-director`, `agents-artist` | 3/3 pairs inspected. Selected default image source, paid-source limit/cost/estimate, both titles/current rows, artist vision eligibility, and unchanged page scroll checked. |
| Phone Connect | `m-connect`, `m-add`, `m-models` | 3/3 pairs inspected. Phone key border remains present; primary 48px and provider actions 44px; native phone catalog checks/summary remain correct. Only remaining scored item is add-sheet spacing. |
| Phone Agents | `m-agents`, `m-director`, `m-artist` | 3/3 pairs inspected. Selected drawing source, paid-source wording, capability pills, plain footers, disabled non-vision artist options, and current selected row retained. |

`artist-eligibility.json` records **3 enabled vision-capable options** and **4 disabled text-only / generation-only options** in the phone artist picker; `gpt-6.1-sol` is the selected enabled row. Disabled options remain visible with explanations as the artboard requires. The previously fixed phone key border and manual success were not lost during the cutover.

**States not checked: none of the requested visual states.** Real external provider login, paid generation, and paid catalog requests were intentionally not exercised. The actual local zero-selection save was exercised as documented above; other provider state payloads remain fixtures and do not claim external-service integration coverage.

## 5. Counts

| QA3 items | Fixed | Not fixed / regressed | Unverified |
|---|---:|---:|---:|
| 1 major | 1 | 0 | 0 |
| 6 minors | 5 | 1 | 0 |
| **Total 7** | **6** | **1** | **0** |

| Current remainder | Count |
|---|---:|
| Majors | **0** |
| Minors | **1** |
| New independent differences beyond QA3 | **0** |
| Primary state pairs checked | **17/17** |
| Requested states skipped | **0** |

Cleanup: owned Chrome closed, round-4 server and kernel stopped, temporary database dropped, and temporary binary/home/profile/cookies/kernel removed. Only `/tmp/viderse-qa4/Connect/shots/` is retained. The shared artboard server was not stopped, other agents environments were not modified, and QA3-Connect.md was left unchanged.

Status: DONE
Summary: **0 majors remaining / 1 minor remaining**. The zero-model added-state strip, initial/model-draft Huỷ behavior, and zero-model Lưu/persistence are verified. Five QA3 minors are fixed. **Worst remaining item (only one): phone add footer double-counts the 26px home row; reduce its bottom padding from 38px to 12px.**


---

## QA5 — focused final recheck

**Verdict: PASS for the requested focused scope. The last QA4 minor is fixed; no new major or minor was observed in the three fresh comparison pairs.** This section supersedes the QA4 remaining-count verdict above; the earlier section remains as the historical record.

### Fresh environment / bundle

Built a new Go binary from the current tree without rebuilding the web; migrated fresh database **`viderse_smoke_qa5connect`**, used a fresh temporary HOME and administrator, and served on **18784**. Live browser script URL was **`/assets/index-C1swU8kQ.js`**, confirming the requested 15:18:46 dist rather than the previous bundle. Used one owned headless Chrome with the required mock-keychain/password-store/profile flags, localhost-only access, and deletion of PublicKeyCredential. The same real-wire-shaped provider/catalog fixtures were used; no paid call, real key, or external login occurred. Reference fonts were loaded from the matching locally bundled faces as in QA4, and screenshots use the settled final animation frame.

### Results

| Focused check | Result | Measured evidence |
|---|---|---|
| **Phone `screen=connect&sheet=add`**, 390×844 | **FIXED** | App primary **y=758, h=48 → bottom=806**, exactly matching artboard **758–806**. App title **y=293.625**, within about 1.4px of artboard **295**, with essentially matching visible title placement. Sheet top is **272.625**, versus reference about **273**. Grid, copy, outlined 48px password field, and primary all fit in frame. |
| Footer no longer double-counts the home row | **FIXED** | `.conn-sheet-primary` is **y=746, h=72, padding `12px 16px`**. Its 48px button starts at 758; the separate **26px `.m-sheet-home`** remains at **y=818**. The former extra 26px footer padding is gone. |
| Phone models sheet sanity | **PASS** | Three fixture models render with the proper pink checkbox/dark-tick treatment, capability pills, readable pricing, and visible footer. Initially **3 checked / Đã chọn 3 model**; after unticking one, **2 checked / Đã chọn 2 model**, with the pricing-unit suffix unchanged. No catalog-summary regression. Inventory, model notes, and associated content-height differences remain accepted fixture/data deviations, not new defects. |
| Desktop `section=connect&conn=list` sanity, 1440×900 | **PASS** | Provider cards, action sizes, help column, main-column wrapping, and sidebar remain intact. Measured sidebar is **248px** wide with padding **`14px 11px 14px 10px`**, confirming the extra right-padding pixel. No new wrapping/clipping or header/frame regression observed. |

### Evidence

All fresh QA5 evidence is under **`/tmp/viderse-qa4/Connect/shots/`**:

- `qa5-art-m-add.png` / `qa5-app-m-add.png`
- `qa5-art-m-models.png` / `qa5-app-m-models.png`
- `qa5-art-conn-list.png` / `qa5-app-conn-list.png`
- Matching per-capture geometry/style `.json` files
- `qa5-app-m-models-two.png` and `.json` for the live two-selected summary
- **`qa5-checks.json`** for exact bundle URL, add-sheet/button/home-row coordinates, before/after selected counts, and sidebar padding
- `qa5-baseline.json` for the real GET baseline used to shape provider fixtures

**Coverage:** 3/3 requested fresh pairs inspected; no focused state skipped. This was intentionally not another 17-state replay: QA4 contains the full regression pass, zero-model save/cancel checks, and real local database-persistence proof. No source files or dist were edited during QA5.

**Cleanup:** owned Chrome closed, QA5 kernel/server stopped, `viderse_smoke_qa5connect` dropped, and `/tmp/viderse-qa5/connect` binary/HOME/profile/cookies/harness removed. Only screenshot/evidence files remain in the requested shared shots directory; the artboard server was left untouched.

| QA5 final count | Count |
|---|---:|
| QA4 remaining minors fixed | **1/1** |
| Known majors remaining | **0** |
| Known minors remaining | **0** |
| New differences in focused recheck | **0** |
| Fresh requested pairs inspected | **3/3** |

Status: DONE
Summary: **QA5 PASS — 0 majors / 0 minors remaining in Connect/Agents across the QA4 baseline plus this focused recheck.** Phone add primary is exactly **758–806**; the 26px double-home-row error is removed. Phone catalog selection/summary and desktop connection/sidebar sanity checks pass. No remaining worst item.
