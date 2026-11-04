# PR2 · V36 — Template/Library sample viewer acceptance

- **Status: DONE_WITH_CONCERNS.** Sample viewer + explicit “Dùng prompt này” are wired to real API data and pass their oracles on desktop 1440×900; **one PR2-source defect** and **one shared-kit deviation** were observed, and the paid-trial handoff / cover-trial sample viewer could not be exercised (provider prerequisites, named in §8).
- Date: 2026-11-03. Source: `a7341d37` (PR2 changes uncommitted in the working tree at capture time).
- Method: real managed Chromium (Chrome for Testing build 1248, Playwright 1.64.1), isolated API `127.0.0.1:8043`, Vite `127.0.0.1:8044`, reference `127.0.0.1:8041` serving `apps/viderse/docs/design/v1`. Native snapshots/actions only; no mocked responses, no DOM/state fabrication, no source or test edits.
- Evidence: [sample-acceptance/](sample-acceptance/) — `evidence.json` (all measured values), 20 PNGs, 4 side-by-side comparisons, 1 motion recording, the two audit runners.

## 1. Isolation and objects touched

- Stack reused as-is: Vite dev server and `viderse-v36-api` with `HOME` inside `/var/folders/…/verse-foundation-qa-restored-40whjiku`, app DB `verse_workflow_qa_app_*`, identity DB `verse_workflow_qa_identity_*`, local blob root inside that HOME. Session = `workflow.qa` via the private cookie file outside the repo; no credential printed or committed.
- **New object created for mutations** (the only writer): template `b1d74c00-5197-4013-b805-adfb7c8cd7f5` — “V36 QA mẫu · nghiệm thu 225945”, created with `POST /api/templates {name, fromId: 389ad9f0…}`. Revisions: 1 (created) → 2 (a “Dùng prompt này” click in an aborted first attempt) → 3 (PATCH restoring `scenePromptId` to `builtin:scene` so the final run starts from the delivered default) → 4 (the final UI “Dùng prompt này” action). All writes were mine and only on this object. The template is a genuine object served by the real API, not a mock.
- Read-only objects: shared templates `389ad9f0` (Mẫu chuẩn · Tách nền) and `29c3c9ae` (Truyện kể đêm) — recipes unchanged by me (`sceneId=builtin:scene`, `coverId=builtin:cover-clean` before and after); prompts `5483055e…` (Ảnh mẫu QA tự tạo · 1) and `cb991c84…` (· 2) with their authored sample rows; fixture jobs `0 review / 1 paused / 2 review` untouched; no job created, deleted, cancelled or re-queued.
- Provider stores stayed empty; no paid action, no provider login, no production interaction. Sample media used are the authored WebP/FFmpeg QA fixtures (provenance: authored input, `costImages: 0`, `provider: ""`), **not** proof of AI drawing.

## 2. Control ledger — Template › Ảnh cảnh › sample viewer (desktop 1440×900)

| # | Control (role · name) | State | Action | Observed | Oracle / readback | Verdict |
|---|---|---|---|---|---|---|
| 1 | button · `Xem lớn Ảnh mẫu QA tự tạo · 2` | enabled | click | Dialog `aria-label="Xem ảnh Ảnh mẫu QA tự tạo · 2"`; title `… · 2`, position `· 1 / 2 đã vẽ`, sub `Ảnh cảnh · ảnh mẫu vẽ bằng prompt này`, tag `Mẫu`; stage image `/api/image-prompts/cb991c84…/samples/9ea695ba…/image` (HTTP 200, decoded 1920×1080) | `GET /api/image-prompts` `sampleUrls` contains exactly that URL | PASS |
| 2 | button · `Xem lớn Theo nhận diện kênh` | `disabled` (builtin has no sample) | not clickable (`disabled=true`; Playwright refuses) | No dialog can open; tooltip only, no `aria-describedby`/`title` in the accessibility tree | prompt has `sampleUrls: []` | PASS behaviour / **D6** |
| 3 | button · `Mẫu sau (→)` | enabled | click | Selection moves to QA·1; position `2 / 2 đã vẽ`; image src switches to the QA·1 sample | strip `aria-selected` moves, image URL is the QA·1 `sampleUrls[0]` | PASS |
| 4 | keyboard ←/→ | enabled | `ArrowRight`, `ArrowLeft` | Dialog label toggles QA·2 → QA·1 → QA·2 | same selection oracle | PASS |
| 5 | option · strip item (role=option) | enabled/selected | click `Ảnh mẫu QA tự tạo · 1`, then force-click the disabled builtin option | Enabled option switches the dialog label and `aria-selected` to QA·1 and moves focus into the strip; the disabled item (`aria-disabled="true"`, `tabIndex=-1`, label `… · chưa mở được`, no image) leaves the selection untouched | selection keyed by prompt id; no writes | PASS |
| 6 | button · `Đóng (Esc)` | enabled | click | Dialog unmounted; focus returns to the opening eye button (`Xem lớn Ảnh mẫu QA tự tạo · 1`) | focus oracle | PASS |
| 7 | key `Escape` | enabled | press | Dialog unmounted; **0 non-GET `/api` requests**; selection unchanged (`Theo nhận diện kênh`); `GET /api/templates/<id>` still `prompts.sceneId=builtin:scene`; focus on the eye | no-mutation oracle | PASS |
| 8 | background (scrim) | — | click the background at (30,450) and (5,450) | Dialog **stays open**; the click lands on `Dialog__sx.center` (full-viewport, `pointer-events:auto`) | spec 04-components › Trình xem ảnh: “Đóng: Esc, nút ×, bấm nền” | **FAIL — D2** |
| 9 | button · `Dùng prompt này` (only when the previewed prompt is not the current one) | enabled primary | click | Viewer closes; radio `Ảnh mẫu QA tự tạo · 1` checked; inline `Đã lưu vừa xong`; exactly one `PATCH /api/templates/b1d74c00…`; `GET` readback `prompts.sceneId=5483055e…`, `revision=4`; reload keeps QA·1 | API readback + reload | PASS / **D5** (feedback differs from reference toast) |
| 10 | `Template đang dùng` | current prompt | open viewer for the selected prompt | Footer renders the text `Template đang dùng`, **no** primary button | `preview.id === value` branch | PASS |
| 11 | Tab / Shift+Tab | focus trap | 9× Tab | Cycle stays inside the dialog: prev → next → strip(2) → strip(1) → used-by template link → “Mở trong kho prompt” → close → prev → next | overlay focus trap | PASS |
| 12 | link · `Mở trong kho prompt` | enabled | not clicked | `href="#/library?tab=prompts&prompt=mine"` present | — | observed only (not exercised to avoid cross-route churn) |

## 3. Control ledger — Library › Prompt ảnh › editor sample viewer (desktop)

| # | Control | State | Action | Observed | Oracle | Verdict |
|---|---|---|---|---|---|---|
| 13 | button · `Xem ảnh mẫu` (2 sample thumbs) | enabled | click | Dialog; header `Ảnh mẫu QA tự tạo · không gọi AI · 21:37 · 1 / 2 đã vẽ`, sub `Ảnh cảnh · ảnh mẫu của prompt`, tag `Mẫu`; strip label `Ảnh mẫu của prompt` with 2 real images; side = prompt name, saved prompt text | samples come from `GET /api/image-prompts/5483055e…/samples` history; image 200, 1920×1080 | PASS |
| 14 | button · primary `Vẽ thử · 1 ảnh` | `aria-disabled="true"`, `aria-describedby` → `Chưa có video nào làm xong để vẽ thử · Dựng xong một video rồi thử lại` | force click | Viewer **stays open**, no confirm dialog, **0 POST** | locked action must be a no-op | PASS (locked path) |
| 15 | key `Escape` | enabled | press | Viewer closes, editor remains | — | PASS |
| 16 | provenance isolation | — | type ` [DRAFT-MARKER]` into the prompt textarea, then open the sample viewer | Marker absent from the viewer side; **0 PATCH** | side renders the saved `prompt.prompt`, not the unsaved draft | PASS |

## 4. Contextual labels and “no fabricated versions”

- Template viewer: subtitle `Ảnh cảnh · ảnh mẫu vẽ bằng prompt này` (render-mode kind), strip `role=listbox aria-label="Prompt cùng loại"` = prompts of the same kind (builtin scene prompt included as a striped, non-clickable item with `… · chưa mở được`), footer note `← → xem mẫu khác · Esc đóng · đổi prompt không vẽ lại video đã xong`, current-state text `Template đang dùng`.
- Library editor viewer: subtitle `Ảnh cảnh · ảnh mẫu của prompt`, strip label `Ảnh mẫu của prompt`, item labels `video title · HH:MM` (real sample rows), footer note `Ảnh mẫu của prompt · không thay ảnh trong video`.
- **No fabricated versions**: `mode="sample"` renders no version strip, `versionNodes = 0`, and the viewer text contains no `Phiên bản`/`phiên bản`. The strip is prompt thumbnails, not history entries. There is no `+1`/fake “bản 2”.
- **Real API images only**: every rendered stage/thumb image is `/api/image-prompts/<id>/samples/<sampleId>/image` and returned HTTP 200 (`evidence.json → network.imageResponses`); each decoded to 1920×1080. Auth/ownership is unchanged (same route as before).
- Cancel does not choose: opening the viewer and closing it by Esc (and by ×) produced **zero writes** and left `prompts.sceneId` unchanged (table rows 7, 6).

## 5. Geometry / typography — app vs reference (1440×900)

| Property | Reference `Template.dc.html` (r41f viewer) | App | Δ |
|---|---|---|---|
| Frame | 1240×800 at (100, 50) | 1240×800 at (100, 50) | none |
| Body columns | `898px 340px` | `898px 340px` | none |
| Stage padding / side padding | 16px / 14px 16px | 16px / 14px 16px | none |
| Strip gap / thumb radius | 8px / 7px, `aspect 16/9`, opacity .55→1 | 8px / 7px, `aspect 16/9`, opacity .55→1 | none |
| Header title | 13.5px/600 | 13.5px/600 | none |
| Header position / sub | 13.5px (`.num`) / 13.5px hint | 12.5px / 12.5px hint | −1px |
| Mode tag | 22px high, `rgba(255,255,255,.06)` bg, colour `#C9C9D1` (inline), no border | 20px high, transparent, **`rgb(185,208,247)` (info)**, 1px `--line-2` border, 11.5px | chip styling/colour |
| Side name | 14px/600 | 12.5px/700 | smaller |
| Side captions (`PROMPT`, `ĐANG DÙNG Ở`) | 11px/600 **uppercase**, ls .06em, hint colour | 12.5px/**400**, normal case, fg colour | **D3** |
| Strip content | 7 prompts, no badges, thumbs ≈116.9×65.8 | 3 prompts, numeric badges `1/2/3`, thumbs 284.66×160.11 | **D7** |
| Stage image | `background-size: contain` (upscales) | `<img>` `max-width/height:100%`, objectFit contain, 1920×1080 → 866×487.125 (no upscale) | intentional, better than reference |
| Stage background | `#050506` | `rgb(11,11,13)` (`--color-bg`) | minor |

Side-by-sides: `comparison/01-sample-viewer-1440.png`, `02-use-prompt-1440.png`, `03-mobile-390.png`, `04-picker-and-editor-samples-1440.png`.

## 6. Motion

- App viewer: `kit-dialog` **300 ms** `cubic-bezier(.2,.9,.25,1.08)` on the box + `kit-fade` **200 ms** scrim (kit `Dialog`). Reference: `vv-dlgIn` **220 ms** `var(--vv-ease)` + `vv-fade` **180 ms**. Same open→visible→(close: box 180 ms, scrim 200 ms, then unmount) structure; the durations come from the kit token, not from a new ad-hoc value. Recorded end-to-end in `motion/desktop-template-sample-flow.webm` (open, arrow, Esc, open, use, saved, reload, current state).
- Reduced motion: the kit `Dialog` disables both animations under `prefers-reduced-motion: reduce` (`Dialog.tsx` `NO_MOTION`); the storyboard’s `prefers-reduced-motion` rule is identical in intent. Not re-captured here because this ticket covered the sample path only.
- Focus: Esc/× return focus to the exact eye button; Tab stays trapped; no background shortcut fires while the viewer is open (viewer keydown handler ignores `INPUT/TEXTAREA/SELECT/contenteditable`, and the page behind is inert).

## 7. Deviations (file/range; not fixed here)

| ID | Severity | Finding | Evidence / source |
|---|---|---|---|
| **D1** | **high (mobile)** | On 390×844 the prompt-editor sample viewer is created **underneath the mobile Sheet**: kit `Dialog` overlay `z-index: 40` vs `.m-overlay` (Sheet) `z-index: 80`. The Dialog exists with `aria-labelledby="Ảnh mẫu"` at (16,245,358×354) but is not visible; `elementFromPoint` at its centre returns the Sheet’s `vlb-card-top`. Keyboard focus goes into the invisible dialog (`Đóng`) while the portaled Sheet stays non-inert, and `Escape` closes only that hidden layer while the visible Sheet stays. | `src/screens/library/PromptEditor.tsx:387-393` (phone branch `Dialog`), nested in `src/screens/library/mobile/MLibraryScreen.tsx:434-443` (`Sheet`), `platform/ui/src/kit.css:203` (`.m-overlay{z-index:80}`), `platform/ui/src/Dialog.tsx:16` (`zIndex: 40`). Screenshot `16-prompt-editor-sample-mobile.png`; `evidence.json → geometry.mobileSampleLayering`. Same pattern in `src/screens/library/template/CoverSection.tsx:280-286` (not reachable, see §8). |
| **D2** | medium (shared kit, pre-existing) | Clicking the background does **not** close the viewer. `Dialog` draws the scrim first and then a full-viewport centring wrapper with `pointer-events:auto`, which swallows every outside click, so the scrim `onClick` never fires. Spec 04-components › Trình xem ảnh requires “bấm nền”. Affects every kit dialog, including this viewer. | `platform/ui/src/Dialog.tsx:191-193` (`scrim` then `center`); `evidence.json → geometry.closePaths.scrimClick` — clicks at (30,450) and (5,450) left the dialog open, `elementFromPoint` at (30,450), (10,10) and (1400,880) all return `Dialog__sx.center`. No working-tree diff in `platform/ui` → introduced before PR2. |
| **D3** | low (visual) | Sample-viewer side captions render as body text (`Prompt`, `Đang dùng ở` at 12.5px/400, normal case, fg colour) instead of the reference caption style (11px/600 uppercase, ls .06em, hint colour). The prompt name is 12.5/700 vs reference 14/600. | `src/screens/library/template/PromptPicker.tsx:94`, `src/screens/library/PromptEditor.tsx:384`, `src/screens/library/template/CoverSection.tsx:277`; reference `artboards/Template.dc.html:1387-1389` with `.vv-viewer__cap`. |
| **D4** | low (copy/context) | Template viewer subtitle says `Ảnh cảnh · …` (and for cover prompts the composition name, e.g. `Chữ hook`) where the reference reads `Prompt ảnh cảnh · …`; the prompt’s tag line under the name (“Mỗi kênh một vẻ”) has no equivalent in the app side. | `src/screens/library/template/PromptPicker.tsx:92-94` vs `artboards/Template.dc.html:1379,1387`. |
| **D5** | low (feedback) | After “Dùng prompt này” the app closes the viewer and shows the inline `Đã lưu vừa xong` status; the reference additionally toasts `Template dùng prompt “X” · video mới vẽ theo prompt này` (naming the consequence). No toast was observed (`toasts: []`). | `src/screens/library/template/PromptPicker.tsx:95`, `src/screens/library/TemplateScreen.tsx:193-196`; reference `artboards/Template.dc.html:1429`. |
| **D6** | low (a11y) | Accessible-reason gap: the eye button for a prompt without a sample is `disabled` with only a visual `tooltip`; `aria-describedby`/`title` are `null`, so the reason “Chưa có ảnh mẫu cho prompt này” is unreachable by keyboard/AT. | `src/screens/library/template/PromptPicker.tsx:49`; `evidence.json → geometry.picker.previews[0]`. |
| **D7** | low (data-density) | Sample strip always renders numeric badges `1..n` (reference sample strip has none) and thumbnails grow with `flex:1` + `aspect 16/9`; with 3 prompts the strip occupies 160px of the 688px stage, shrinking the main image versus the 7-prompt reference (66px strip). | `src/components/image-viewer/ImageViewer.tsx:161-178` (badge default `String(index+1)`), `image-viewer.stylex.ts` `strip`/`thumb`; reference `artboards/Template.dc.html:1384`. |
| D8 | note | Locked trial primary label renders `Vẽ thử · 1 ảnh` with an empty provider slot (`copy.tryCost('')` → trailing space, no provider name) while the same panel elsewhere names “ChatGPT”. Cosmetic. | `src/screens/library/PromptEditor.tsx:334,385`. |

Not deviations (checked): no fake versions/history in sample mode; strip placeholders are striped and non-focusable; `←/→` skip placeholders; images are `contain` and never upscaled; frame/columns/padding/radius tokens match; avatar/round rules untouched; mobile Template shows the authored design (no viewer).

## 8. Not exercised — exact prerequisites (not “pass”)

1. **Paid trial handoff (viewer → confirm), and “cancel confirm sends no mutation”.**
   - Runtime state: `GET /api/image-prompts/5483055e…/samples` → `ready:false`, `sources:[]`, `blockedReason:"Chưa có video nào làm xong để vẽ thử · Dựng xong một video rồi thử lại"`, `quota` readable (ChatGPT, 50/50), `cost.images:0`.
   - Backend gate: `covertrial/service.go:878-893` (`sampleBlocked` needs ≥1 preview source, a resolvable drawer and quota) and `preview/service.go:257-273` (`sourceJobs` only returns `CloneSucceeded` jobs with the four source artifacts). The QA fixture has zero such jobs (0/1/2 are review/paused/review and must not be touched).
   - Missing prerequisite: a **succeeded video job** plus a **connected image provider (drawer) and paid quota**; provider stores are deliberately empty and provider login/paid draw is not authorized for this session.
   - Consequence: the confirm dialog (`Vẽ ảnh mẫu?`) never opens, so the close-before-confirm ordering is only code-evident (`PromptEditor.tsx:385`: `flushSync(() => setViewing(null))` then `askSample` → `confirm`; `CoverSection.tsx:278`: same with `draw`). The locked button was verified to be inert (no close, no confirm, no POST).

2. **Template › Ảnh bìa sample viewer (`CoverSection`).**
   - `GET /api/templates/<id>/cover-trials` → `history: []`, `sources: []`, same `blockedReason`. Without a persisted cover-trial row the viewer has nothing to open, and creating one requires the paid draw above.
   - Missing prerequisite: one real cover trial row (`image_prompt_samples`-equivalent `cover_trials` history), i.e. the same provider + finished-video + quota chain. Phone branch also carries **D1**.

3. **Provider-produced sample media.**
   - Only authored QA media exists. The sample route returned HTTP 200 with `content-type: application/octet-stream` for these rows because the QA fixture publisher wrote the blob without `ContentType`; the product path sets `image/png` (`internal/covertrial/run.go:174-175`). Decoding worked (1920×1080); real provider output and its MIME are unverified.

4. **Not re-tested here (parent owns):** workbench/review/editor viewer entrypoints, mobile viewer round 45 (excluded by plan), Vitest/typecheck/build/lint.

## 9. Mobile 390×844 verdict

- Template screen: 0 dialogs, 0 `Xem lớn` preview buttons, prompt radios still render (`Theo nhận diện kênh`, QA·2, QA·1), `scrollWidth = 390` (no overflow), body scroll lock untouched. This matches the authored `MTemplate` artboard, which has **no** prompt eye and **no** sample viewer — the round-45 mobile viewer is not introduced. Reference: `20-reference-mtemplate-390.png`; app: `12-template-images-mobile.png`, `13-…-full.png`; comparison `comparison/03-mobile-390.png`.
- Library › prompt editor (Sheet) does expose “Xem ảnh mẫu”, and that path is **broken at 390×844** (D1): a sample `Dialog` is mounted but sits under the Sheet, invisible and focus-trapping. This is not an accepted state: the control has no honest response on mobile today, and it is new PR2 code.
- Sheet/motion: the mobile Sheet itself is the existing design (`MLibraryScreen`), the sample Dialog is the only layer in question; `comparison/03-mobile-390.png` shows the app without a second modal (because the dialog is hidden).

## 10. Commands and artifacts

The two one-off audit runners were archived outside the repository after use; hardcoded local QA paths/IDs are not shipped as permanent test scaffolds.

Maintained browser regressions: `tests/viderse-workflow/viewer-audit.pw.ts`; prerequisites and current results: [parent report](viewer-and-actions.md), [verification](../../verification.md).

Historical runner hashes: `ea246418a0a6…e69f5f`, `7fc4acac753b…c4906`; motion recording `5eb8438a590e…2cae8`. The screenshots/metrics below retain their pre-remediation source meaning; they are not silently relabelled as current passes.
- Screenshots `01-…` → `20-…`, comparisons in `comparison/`, JSON metrics in `evidence.json`, motion in `motion/desktop-template-sample-flow.webm`.
- Reference was captured from `http://127.0.0.1:8041/artboards/Template.dc.html` and `MTemplate.dc.html` at default props (no tweak overrides), fonts loaded, animations settled; the viewer was opened with the board’s own eye button. No artboard, kit or token file was edited; the artboard’s transient in-memory state was reset by navigation only.
- No build/lint/unit tests were run in this slot (parent owns the final gates).

## 11. Concerns for the merge gate

1. **D1 / design #81 resolved for Prompt mobile:** người dùng chọn A, preview trong cùng Sheet. [Lượt11](runs/11-mobile-same-sheet-focus-fixed/interaction-results.json) actual-browser2/2 normal/reduced: một modal, ảnh decode, Quay lại/Esc giữ draft và focus; Shift+Tab không ra nền. CoverSection đã bỏ Dialog mobile nhưng còn chờ history để kiểm. Lượt10 lỗi focus và ảnh cũ vẫn giữ nguyên, không đổi thành pass.
2. **D2 fixed:** the shared `Dialog` centring layer now dismisses only when the click actually hits its background, not an inner control. Actual browser scrim-close passed in the final focused run. Typography, sample tag, capped strip, contextual heading, saved-selection feedback and missing-sample reason were also corrected; the updated desktop screenshot is linked from the parent report.
3. Paid-trial handoff và Cover sample viewer vẫn **chưa kiểm xong**. Người dùng đã cấp quyền tối đa2 ảnh QA thật; QA nhận access token còn hạn, không nhận refresh token, fallback tắt. Lượt redraw đầu bị chặn trước provider vì fixture thiếu upstream thumbnail pointer; đã sửa metadata QA trỏ tới keyframe thật, đang chạy tiếp cùng draft, không gửi thêm redraw.
4. Screenshot/API evidence uses authored QA sample media (`costImages: 0`); it does not prove provider drawing, provider fallback, or real image MIME.

## 12. Parent update after actual provider and remediation checks

Sections1–10 describe the original audit, not current provider availability or acceptance status.

- User authorized exactly2real QA images; both have now been consumed. Actual scene version2 and a ready cover trial exist. No further image generation is permitted; production credentials remain unchanged and QA receives no refresh token.
- The original finished source now builds and exports an actual decoded H.264/AAC MP4. Cover trial history therefore has a genuine completed source, not a fabricated ready-job flag.
- Actual cover trial: ready PNG1672×941,2300224bytes, costImages1. Run21 generated it once, then failed a harness label assertion; it was not redrawn. Run22 preview parity4/4 passes with exact URL/decoded geometry, one mobile surface, Back preserves source/radio choices and focus.
- D1/#81 now has both Prompt and Cover runtime proof. Run28 adds mobile Cover Esc and desktop Prompt/Cover confirmation cancellation via Huỷ/Esc/scrim; one active modal, same sample and focus restored,0APIwrites. No mobile45 viewer was introduced.
- Quota state is genuine: the existing QA-only daily-limit configuration is2, matching actual two-image consumption. Run31 passes4/4: actual API quota0/2 and exhausted reason, drawing stays locked with0write attempts, existing saved cover remains readable. No synthetic usage rows were inserted. Temporary read-only access is removed by the runner's finally block.
- Production failure feedback: run33/34 proves the lazy confirmation JS fetch can fail visibly and native reload recovers, without revoking the session. Run34's real screenshot found long pill text painting over the recovery action; `Toast` now applies the canonical kit.css `.vv-toast__text` overflow/ellipsis rule. Run35 passes4/4 with text-paint/action boundary assertions and legible native screenshots, normal/reduced motion. Full accessible text is retained.
- Full tweak/control coverage is still tracked by the parent report. These concrete passes do not erase the audit's historical failing images or count unexercised states as passed.
