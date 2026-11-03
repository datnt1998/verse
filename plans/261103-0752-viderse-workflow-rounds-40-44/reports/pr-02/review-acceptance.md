# PR2 / V36 Review acceptance — desktop 1440×900 and mobile 390×844

**Historical audit: DONE_WITH_CONCERNS.** Sections below through “Historical capture” preserve the initial pre-fix findings/prerequisites, not current unresolved counts. Parent remediations and later real-provider/replay evidence are recorded at the end and in [current ledger](viewer-and-actions.md). Do not read the historical blocked rows as new-source test failures or as waived acceptance.

## Scope and constraints honoured

- Services reused, never restarted: API `127.0.0.1:8043`, Vite app `127.0.0.1:8044`, reference `127.0.0.1:8041` serving `apps/viderse/docs/design/v1`.
- Isolated QA account `workflow.qa`; fixture job `5f79fcf4-2805-41af-a2a1-ef8e219e4e98` (7 images, `review`, one legacy prompt version each). Fixture jobs 0/1/2 were **not** mutated. No job/project was created, so there are no new Editor-history IDs to hand to the parent (see “Parent request”).
- Real Playwright-managed Chromium `156.0.8078.4`, headless; native role locators, snapshots, focus and clipboard reads. Every page was watched for non-GET verbs; **all scenarios recorded `mutations: []`**. No approve, skip, upload, use-version, redraw or provider action was submitted. No paid image was created.
- Cookies were read from the private QA root and never copied into this report. No DOM/state mocking, no source edits, no artboard edits (reference props were set only through the existing `window.__dcSetProps` tweak mechanism).
- Parent-owned checks were not duplicated: address/reload/back, missing-image fallback and feedback-cancel remain the parent PW suite.

## Evidence index

All paths are relative to `reports/pr-02/`.

| Artifact | Contents |
|---|---|
| One-off capture runner (archived privately after use) | Historical audit: 27 reference captures (18 Review, 9 MReview), 14 app captures, 18 ledger entries, mapping, side-by-side montages. It rejected horizontal overflow, broken images, page errors or any non-GET request. The temporary runner is not shipped as a second regression suite. |
| `review-acceptance/receipt.json` | Machine receipt: counts, source HEAD and hashes, zero provider/cost counters, failures. |
| `review-acceptance/reference/` | Reference board images at each row’s viewport (props read back from `propOverrides`). |
| `review-acceptance/app/` | Actual app screenshots, including each state at canonical `#/review?video=<job0>&img=sc-04`. |
| `review-acceptance/comparison/` | 19 labelled side-by-sides (`Reference · <row id>` left, `App · canonical` right). |
| `review-acceptance/control-ledger.json` | Full control ledger with URL, focus, draft/readback and mutation arrays. |
| `review-acceptance/tweak-mapping.json` | Every Review/MReview matrix row mapped to evidenced/pending/blocked. |
| `review-acceptance/layout-audit.json` | Column/stage/dialog metrics, 360/430/767/768 breakpoints, touch-target scan. |
| `reports/artboard-state-matrix.json` | Unchanged shared matrix; rows consumed as-is. |

## Tweak mapping (27 rows)

| Row | Owner | Status | Evidence / reason |
|---|---|---|---|
| Review.confirm.none | PR2 | evidenced | `Review.default.png`; no dialog, URL `#/review?video=<job0>&img=sc-04`. |
| Review.confirm.redrawAll | PR2 | evidenced | `Review.redraw-all.png`; dialog count 7, ChatGPT/Grok 50/50, focus trap 12/12 stops inside. Scrim dismiss fails (B5). |
| Review.state.review | PR2 | evidenced | `Review.default.png`; 7-image queue, selected Cảnh 4. |
| Review.state.redrawing | PR2 | **blocked** | No real `drawing` version; a paid provider draw is not authorized, so it was not simulated. |
| Review.state.reveal | PR2 | **blocked** | Reveal requires a newly finished provider version; no draw authorized/available. |
| Review.state.failed | PR2 | **blocked** | No failed version in the isolated fixture; a failed provider result cannot be fabricated. |
| Review.state.batch | PR2 | evidenced | `Review.batch.png`; shift-select 2, shared feedback draft, URL unchanged. |
| Review.state.blocked | PR2 | **blocked** | No blocked/skipped-refused image; mutating fixture jobs 0/1/2 is prohibited. |
| Review.state.empty | PR2 | evidenced | `Review.empty.png`; filter-empty via unused channel. Not account-empty (jobs must not be removed). |
| Review.view.full | PR2 | evidenced | `Review.default.png`; stage contains a 1920×1080 image at 774×435. |
| Review.view.cutout | PR2 | evidenced | `Review.cutout.png`; control changes state but not the image source (B4). |
| Review.score.false | PR2 | evidenced | `Review.default.png`; no score banner because no image has a real score. Match is by absence of data, not a hide switch. |
| Review.score.true | PR2 | **blocked** | No real Kiểm định scores on any image; score UI was not populated with invented numbers. |
| Review.prompt.view | PR2 | evidenced | `Review.default.png`; structured Cảnh part shown. |
| Review.prompt.edit | PR2 | evidenced | `Review.edit.png`; edit entered via the bottom `Sửa chỉ dẫn` although the rail `Sửa` is disabled for a legacy prompt (B1). |
| Review.prompt.raw | PR2 | evidenced | `Review.raw.png`; raw text equals server `sentText`; toggle `aria-pressed=true`. |
| Review.refs.view | PR2 | evidenced | `Review.default.png`; `0/4`, empty-refs copy, Add enabled. |
| Review.refs.add | PR2 | evidenced | `Review.refs-add.png`; popover, tabs, candidates, roles, disabled→enabled Add. Adding from view mode is silent/unsent (B3). |
| MReview.state.review | PR6 | evidenced | `MReview.default.png`; no dialog and no round45 shared viewer. |
| MReview.state.prompt | PR6 | evidenced | `MReview.prompt.png`; `Prompt đã gửi` sheet; clipboard equals `sentText`. |
| MReview.state.feedback | PR6 | evidenced | `MReview.feedback.png`; sheet opens, draft survives cancel/reopen. Reference also shows a reference strip; app sheet does not (M3). |
| MReview.state.edit | PR6 | evidenced | `MReview.edit.png`; draft equals `sentText`; tab/reopen resets it (M1). |
| MReview.state.redrawing | PR6 | **blocked** | Same provider prerequisite as desktop. |
| MReview.state.reveal | PR6 | **blocked** | Same provider prerequisite as desktop. |
| MReview.state.empty | PR6 | evidenced | `MReview.empty.png`; filter-empty caveat as desktop. |
| MReview.score.false | PR6 | **pending** | App always renders the server autonomy summary banner when present (here “chưa có điểm … bạn vẫn duyệt tay”); the artboard score=false hides the banner. No app-side hide switch was found. |
| MReview.score.true | PR6 | **blocked** | No real scores; mobile indicators cannot be populated with fabricated data. |

Totals: **18 evidenced, 1 pending, 8 blocked**. See `tweak-mapping.json` for paths per row.

## Control ledger (non-paid paths exercised)

All URLs shown are after the action unless stated. Every scenario’s mutation list was empty.

| Viewport | Control / action | Observable readback | Result |
|---|---|---|---|
| 1440 | Open canonical Review route | `#/review?video=<job0>&img=sc-04`; selected Cảnh 4; 0 dialogs; stage `/api/clone/<job>/artifacts/sha256:9c03…` 1920×1080, rendered 774×435; refs `0/4`; rail `Sửa` disabled (`legacyParts`); 0 score banners | pass |
| 1440 | `Chép` prompt | Clipboard equals server `sentText`; button becomes `Đã chép`; URL unchanged | pass |
| 1440 | `Xem nguyên văn gửi máy chủ` | `<pre>` equals `sentText`; toggle pressed; URL unchanged | pass |
| 1440 | `Sửa chỉ dẫn` (bottom) on legacy prompt | Enters edit despite disabled rail `Sửa`; edit bar `Đã sửa 1 chỗ…`; submit `aria-disabled` flips `true → false`; URL unchanged | pass-with-bug (B1) |
| 1440 | `Esc` in edit | Returns to view; textareas and edit bar gone; no request | pass |
| 1440 | Edit → raw → `Xem theo phần` | Draft is discarded; re-entering edit shows the original prompt; no warning | fail (B2) |
| 1440 | `Thêm` → `Huỷ` | Popover closes; refs stay `0/4`; no request; focus falls to document body (not returned to `Thêm`, M4) | pass |
| 1440 | Refs: `Khung video gốc` / `Tải lên` | Frames shows empty copy; upload shows `Kéo ảnh vào đây…` and `PNG, JPG, WebP · tối đa 20 MB`; no upload performed | pass |
| 1440 | Refs: select Cảnh 1 + `Nhân vật` | Add label `Thêm 1 ảnh`, `aria-disabled=false`; candidate list excludes current image | pass |
| 1440 | Refs: `Thêm 1 ảnh` from view mode | Popover closes; header remains `0/4`; no remove control; after entering edit (keyboard `E`) header is `1/4` with one remove button. `redraw()` sends only `{feedback}`, so the staged ref is unsent from the bottom `Vẽ lại` | fail (B3) |
| 1440 | Batch: shift-select Cảnh 1 + Cảnh 2 | URL unchanged; `2 ảnh đã chọn`; label `Vẽ lại 2 ảnh · tốn 2 ảnh`; typed feedback kept | pass |
| 1440 | Batch confirm → `Huỷ` | `alertdialog`: “Vẽ lại 2 ảnh? / Tốn 2 ảnh … ChatGPT còn 50/50 · Grok còn 50/50…” (plus old-image/new-approval copy); dialog closes; draft and selection preserved; focus returns to the batch trigger; no request | pass |
| 1440 | Batch `Esc` | Clears selection and feedback; returns to view; URL unchanged | pass |
| 1440 | `Vẽ lại cả 7 ảnh` open | Dialog: 7 images, quota line, old-image note, shared-feedback field; `Tab` 12/12 stays inside; `Huỷ`, `×` and `Esc` all close and return focus to the trigger; shared draft `Draft chung giữ lại` survives cancel/reopen | pass |
| 1440 | Redraw-all scrim click at (20,450) | Dialog stays open (`dialogCount=1`), background is not dismissed; `Esc` then closes | **fail (B5)** |
| 1440 | `Lớp ảnh · Tách nền` | Radio checked but stage `src` is byte-identical to the full view; no checkerboard/transparent asset; no `Chưa có bản tách nền` message | fail (B4) |
| 1440 | Unused-channel review route | `Đã duyệt hết` / `Không còn video nào chờ duyệt.`; 0 image options; URL keeps the unused channel id | pass-with-caveat (filter-empty) |
| 390 | Open canonical Review | `#/review?video=<job0>&img=sc-04`; `dialogs=0`, shared-viewer dialogs `0`; stage 1920×1080; queue `Đã duyệt 1/7`; server score summary banner visible | pass |
| 390 | `Vẽ lại` → feedback sheet | Sheet opens; typed `Góp ý nháp trên điện thoại`; `Huỷ` closes; reopen preserves the draft; URL unchanged | pass |
| 390 | `Xem prompt đã gửi` → `Chép chỉ dẫn` | Clipboard equals `sentText`; prompt sheet shows Cảnh 04 / bản 1; no request | pass |
| 390 | Prompt sheet → `Sửa chỉ dẫn rồi vẽ lại` | Feedback sheet edit tab opens with draft = `sentText`; type a change, switch `Góp ý` → `Sửa chỉ dẫn`, or close/reopen: draft resets to `sentText` both times | fail (M1, PR6) |
| 390 | Unused-channel review route | `Đã duyệt hết`; 0 options; 0 dialogs | pass-with-caveat |
| 390 | Settings › Kết nối AI; `GET /api/youtube/data-key` | Actual endpoint returns **404**; UI shows only ChatGPT/Grok provider cards; no `YouTube Data API`/`data-key` text | pass |

Reference top-level names used by the app are `Xem prompt đã gửi` (row) and `Prompt đã gửi` (sheet); the harness records those exact labels.

## Source-backed findings (not fixed)

| ID | Severity | Finding | Evidence and source |
|---|---|---|---|
| B1 | P1 | **Legacy prompt can be edited through the bottom bar.** Rail `Sửa` is correctly disabled for `partsLegacy`, but bottom `Sửa chỉ dẫn` is gated only by `isDrawing || !online`; it enters edit, enables submit after one change, and would send parts for a prompt the raw-provenance rule says is immutable. | Ledger `desktop-edit` (rail disabled at default; bottom entry; submit `true→false`); `ReviewScreen.tsx:470` vs `ImageRail.tsx:65,225`. |
| B2 | P2 | **Raw toggle silently loses an unsaved prompt draft.** From edit, `Xem nguyên văn gửi máy chủ` switches to raw; `Xem theo phần` routes to `view`, and `setMode('view')` clears edits. Re-entering edit shows the original text with no warning. | Ledger `desktop-edit-raw` (`draftLost=true`); `ImageRail.tsx:292`; `useImageEditing.ts:39-41`. |
| B3 | P1 | **Adding a reference in view mode is silent and not sent by `Vẽ lại`.** The popover closes, the rail stays `0/4`, no remove control appears, and the bottom `redraw()` body contains only feedback. The staged ref becomes visible only after entering edit, which is itself blocked by B1 for this legacy prompt (keyboard `E` used to expose it). The reference `addRef` sets `edit:true` and renders the added ref immediately (`Review.dc.html:1244`). | Ledger `desktop-refs` (`0/4` → edit `1/4`); `ImageRail.tsx:57,78-95`; `ReviewScreen.tsx:246,249-258`. |
| B4 | P1 | **`Tách nền` is a no-op for §26 image versions.** `shownImage` always uses `version.url`; only the legacy fallback calls `imageURL(..., view)`. The cutout state therefore keeps the identical opaque artifact and only switches to `object-fit: contain`. | `Review.cutout.png` vs reference; ledger `desktop-cutout` (`srcBefore === srcAfter`); `ReviewScreen.tsx:126,394`; `review.stylex.ts:34`. |
| B5 | P1 | **Redraw-all scrim click does not dismiss.** `Dialog` renders the scrim with `onClick={onClose}`, but the sibling full-viewport `center` layer (no `pointer-events:none`) is above it, so outside clicks hit `center`. `Huỷ`, `×` and `Esc` work and restore focus. Reference expects scrim dismiss. | Ledger `desktop-redraw-all` (`closedByScrim.dialogCount=1`; `escAfterScrim` closes); `platform/ui/src/Dialog.tsx:23-30,191`. Shared component, surfaced by the PR2 confirmation. |
| M1 | P2 (PR6) | **Mobile edit draft resets on tab switch/reopen.** `onChange` entering the edit tab calls `setDraft(sentPrompt)`, overwriting typing; closing/reopening does the same. Feedback-tab drafts do survive. | Ledger `mobile-prompt-edit` (`draftAfterTabs`/`draftAfterReopen === sentText`); `MReviewScreen.tsx:185,374`. |
| M2 | P2 (PR6) | **Mobile touch targets below 44 px.** Measured: top `Việc nền` 36×36, account 36×36, `Video sau` 36×36, prompt row 358×42; feedback sheet segmented tabs 30 px high, quick chips 34 px high. | `layout-audit.json` `mobile`; `mreview.stylex.ts:22,109,142`. |
| M3 | P3 (PR6) | **Mobile sheet visual delta.** Reference feedback sheet includes the `Ảnh tham chiếu` strip; MReview renders no reference row in its sheets. This is a reference/PR6 gap, not a PR2 source change. | `MReview.feedback.png` comparison; MReview has no refs render path. |
| M4 | P3 (PR2) | **Inline ref popover loses focus on close.** Opening focuses the first control (`Đóng`), but `Huỷ`/close leaves `document.activeElement` on the body rather than returning to `Thêm`. | Ledger `desktop-refs` `afterRefCancel.focus`; `ImageRail.tsx:52` (open focus only). |

No other source-backed failures were found in the PR2 changed paths. The legacy/structured prompt, single-version rail, `usedVersion=0` and `Dùng bản 1` are fixture provenance, not treated as bugs.

## Visual / layout comparison notes

- At 1440×900 the three-column geometry matches the artboard exactly: queue aside **268 px**, main **812 px**, prompt aside **360 px**, total scroll width **1440**. The stage renders 1920×1080 at **774×435** with `contain` and no upscaling.
- The redraw-all dialog is **521.1 px** wide in app vs **540 px** in the reference (≈3.5 % narrower, no clipping). Both scrims compute to `rgba(5, 5, 7, 0.62)`.
- Breakpoints have no horizontal overflow: 360/430/767 use MReview (`Duyệt ảnh`, no desktop aside); 768 switches to the desktop Review (`aside[aria-label="Video chờ duyệt"]`, stage `Cảnh 4`).
- Data/provenance deltas are intentional fixture limits: one legacy prompt part (no Phong cách/Tránh), one version per image, no refs, no scores, no drawing/failed version. They explain the reference-vs-app text density differences and are not counted as UI bugs.
- Mobile has **no round45 shared viewer**: default `dialogs=0` and shared-viewer dialogs `0`; only the MReview prompt/feedback sheets open.

## Explicitly not exercised (named, not passed)

- `Review.state.redrawing`, `Review.state.reveal`, `Review.state.failed`, `Review.state.blocked`, `MReview.state.redrawing`, `MReview.state.reveal`: blocked by the no-provider / no-fixture-mutation rules.
- `Review.score.true`, `MReview.score.true`: blocked because no image carries real Kiểm định scores; inventing scores was not allowed.
- `MReview.score.false`: pending because the app always shows the server autonomy summary when present.
- Actual redraw/approve/skip/use-version/upload mutations: deliberately not submitted; confirmation cancel paths were exercised instead. `expectedRevision` and `confirmCount` are source-read, not network-observed.
- Parent-owned address/reload/back, missing-image fallback and feedback-cancel tests were not repeated.
- 2-version version-strip/compare/use-version states could not be exercised because the fixture has one version each.

## Parent request — 2-version QA job

No distinct 2-version job was created, so **no jobID/projectID/imageID are available** to hand over. The existing `workflow-qa` seeder asserts a fresh database, starting another stack is prohibited, and adding versions to fixture job0 is prohibited; a real draw is provider-paid and unauthorized. A 2-version Editor-history smoke therefore needs an isolated seeded stack (or a parent-owned fixture) with explicit authored version/parts metadata. No AI generation was attempted.

## Remaining requirements before V36 Review can be called green

1. Fix B1–B5, then re-run this harness and re-inspect `Review.prompt.edit`, `Review.refs.add`, `Review.view.cutout`, and `Review.confirm.redrawAll`.
2. Fix M1–M2 under PR6, and decide whether MReview sheets must display references (M3). Re-check `MReview.score.false` once real scores exist.
3. Produce authorized provider-backed evidence for redrawing/reveal/failed/blocked and score.true, or explicitly accept them as unavailable prerequisites.
4. Provide a real 2-version QA job if version strip/compare/use-version and Editor historical preview are in acceptance scope.

## Historical capture and current regression route

The one-off capture runner was archived outside the repository after this audit. This report and its ledgers describe the pre-fix source hashes in `receipt.json`; they are not a fresh pass claim for the final source.

Maintained regressions live in `tests/viderse-workflow/viewer-audit.pw.ts`. See [parent verification report](viewer-and-actions.md) and [verification commands](../../verification.md) for the private-fixture prerequisites and current results.

The historical capture closed its browser at completion and produced `reference=27, app=14, ledger=18, evidenced=18, pending=1, blocked=8, failures=[]`; individual control failures remain recorded as `result: fail`, not hidden by the runner exit status.

Parent remediation: legacy edit bypass, raw round-trip draft loss, staged references not surfaced, and cutout variant not switching were fixed. Review15/15 unit regressions and the corresponding actual-browser paths passed. Provider-backed states and a genuine two-version QA fixture remain explicitly unproved by this audit.

## Authored state scenarios and late boundary repairs

These persist explicit UI scenarios in owned recorded-source replay A. They are **not claims that the paid provider failed/refused or generated another image**. Ready/history reuse actual saved versions; authored in-progress/error rows carry no invented paid receipt. Browser mutations are blocked; an ownership/CAS guard restores the replay in finally, and the original completed QA source checksum must remain unchanged.

- Run37 captured18desktop/mobile normal/reduced screens plus2explicitly inapplicable desktop-edit phone cases. Visual inspection exposed two bugs the initial shallow assertion missed: generic `failed` was classified as content-filter refusal, covering the quota card; mobile displayed drawing version3 when version2 was already reserved.
- `isBlocked` now requires explicit blocked/refused state. Mobile reads state/number from structured `images` already present in the same GET response, with no extra request/backend field; it does not duplicate the reserved drawing row as a completed thumbnail.
- Permanent regressions now combine legacy item failed with structured quota failure and require the **main** failure card, not just a sidebar string. Existing mobile draw→ready coverage asserts the in-flight number and no selectable incomplete version. Focused19/19 and typecheck pass.
- Run38 current Vite source:18scenario screens, visible quota/non-refusal cards and correct drawing number;2phone edit cases remain explicitly outside that desktop tweak. Original source unchanged, replay restored,0new provider calls; corrected screenshots were inspected.
- Run40 adds native offline→online SSE reconnect and real GET readback of actual saved ready version2: **22captured,2inapplicable**, including four reveal transitions. Normal desktop/mobile run the1000ms reveal and1600ms glint; reduced-motion variants have0reveal animations and0grain. No CSS state, provider output or usage ledger was fabricated. Scenario failures themselves remain authored fixtures, distinct from real provider execution.
- The earlier3720pass/4skip run predated the last Review repairs. A subsequent full run (`bg_2888`) on source commit `ee94b6f36db1048beaeeeed640f30a14ca72c2a9` includes these repairs and PromptPicker context: **3720pass/0fail/4skip**. Later local-provider failure copy/action changes are outside that full run; their separate checks are recorded in the provider report.


## Native controls mở rộng · lượt48–50 (04/11/2026)

- Lượt48 dừng do oracle đọc nhầm trường checkpoint `prompt` thành wire `sentText`; chỉ sửa runner dùng GET images thật, không sửa dữ liệu/backend để lấy pass.
- Lượt49 bắt lỗi thật M4 còn tồn tại: Huỷ picker tham chiếu trả focus về `body`. `ImageRail` nay giữ trigger ref và cleanup focus khi popup đóng; không giành focus nếu người dùng đã chuyển sang field ngoài. Regression đó đã qua trong focused Review+Connect32/32; full Viderse hiện hành3723qua/4skip sau sửa.
- [Lượt50](runs/50-review-native-focus-restored/review-state-replay-results.json) có **24 hàng captured,2N/A có tên**. Hai project desktop kiểm native nguyên văn đúng GET server → sửa nháp → raw/view không mất nháp → Huỷ; picker Huỷ trả focus; thêm ref vào draft rồi Huỷ; cutout thật decode → ảnh gốc; Shift chọn2ảnh → Bỏ chọn; xác nhận vẽ lại cả7ảnh → scrim huỷ và trả focus. Normal/reduced đủ. Original source checksum không đổi, checkpoint replay khôi phục,0provider/0POST vẽ.
- **Chưa qua semantic draft:** ảnh thật trong lượt50 cho thấy xem bản2 nhưng draft Sửa/Thêm lấy refs từ bản1 đang dùng. Runner lúc này kiểm controls, chưa có oracle giữ nguyên2refs của bản2. Đã ghi câu hỏi [#84](../../../../apps/viderse/docs/design/v1/11-hoi-dap.md) vì sửa nguyên nhân đầy đủ cần chọn contract phiên bản nền backend hoặc chốt giới hạn tương tác khác artboard. Không coi24capture là pass lỗi này; chưa merge PR2.

- [Lượt55](runs/55-review-integrated-embedded/review-state-replay-results.json) lặp toàn bộ native controls/state/motion trên **binary nhúng nguồn tích hợp**, không phải Vite:24hàng captured/2N/A, original source giữ checksum, replay khôi phục,0provider. Đây xác minh surface build thật sau các sửa provider/focus/Toast, **không đóng #84**. Có thêm6ảnh raw/edit/refs/cutout/batch/confirm đặt cạnh canonical trong [comparison-current](comparison-current/); ảnh chỉ ghép cạnh nhau, không sửa nội dung screenshot.
