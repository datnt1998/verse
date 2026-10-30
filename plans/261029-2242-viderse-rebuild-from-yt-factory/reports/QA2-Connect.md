# QA2 — Connect / Agents

## Scope, evidence, and verdict

Independently read **all 34 state-named WebP images (17 artboard/app pairs)** in `/tmp/viderse-fidelity/connect/shots/`, plus `conn-models-openrouter-app.webp` as supplementary evidence. The named newest WebP pairs, not the older PNGs, determine this review. Read `QA-Connect.md`, `Fidelity-Connect.md`, and the relevant requirements in `01-ia-flows.md`, `04-components.md`, `05-kit.md`, `06-copy.md`, `03-tokens.md`, `artboards/README.md`, and hỏi-đáp #18–#26.

**The claim that all round-1 majors are fixed is not supported. Of 33 round-1 major observations, 25 are fixed, 5 are still wrong, and 3 are not verifiable.** There are also remaining visual/state differences detailed below. This is screenshot QA, not a runtime or source-code correctness verdict.

Evidence shorthand: `state` means the exact pair `state-artboard.webp` ↔ `state-app.webp` in the directory above. No browser was needed: every requested artboard has a readable pair. Rendering another artboard cannot recover application content below the captured viewport. No product files were changed and no builds/tests were run.

### Comparison and counting rules

- Compare actual visible layout, wrapping/card boundaries, controls, literal non-data copy, tone, icons, and density. Do not count different provider/model inventories, account authentication fixtures, identifiers, times, quantities, prices, versions, or activity counts as styling bugs.
- Ignore the artboard-only simulated phone system status bar. For phone page-content alignment, account for its approximately 44-CSS-pixel inset; bottom-anchored sheets are compared from the bottom, not by subtracting that inset from their top position.
- Accepted pending deviations #18–#26 are excluded from defect totals even though the source headings still say Mở. In this area, #19 excludes the missing Giới hạn từng kênh sidebar row; #26 excludes absent Grok plan/monthly spend, generated model notes, real/fallback reset times, a disabled empty-key primary, and the proposed phone reasoning control.
- The round-1 table preserves its **33 state observations**, including shared defects repeated in different states. The new-difference count instead counts **each bullet below once**. Shared-desktop bullets have their own count-table row and are not multiplied by eleven screenshots. Do not add the old and new tables together.
- A missing or contradictory captured result is called an evidence/state mismatch, not proof that unseen application behavior is broken. Not-verifiable items are not silently accepted as fixed.

## 1. Round-1 majors — individual disposition

| # | Round-1 state / major | Status | Fresh WebP evidence |
|---:|---|---|---|
| 1 | conn-list: missing provider card surfaces/boundaries | fixed | `conn-list`: each provider now has a dark rounded outlined card enclosing its content. |
| 2 | conn-add: no desktop add-state app capture | fixed | `conn-add-app.webp` now exists and shows the expanded eight-tile desktop add card. Its lower form is cropped; see coverage below. |
| 3 | conn-added: missing provider cards | fixed | `conn-added`: enclosing provider/catalog card and subsequent ChatGPT card are visible. |
| 4 | conn-added: absent unsaved-change strip | still wrong | `conn-added-app.webp` shows the page introduction immediately followed by the new provider; no information strip, Huỷ, or Lưu. |
| 5 | conn-added: fetched-model notice contradicted by empty catalog | fixed | `conn-added-app.webp` says 4 fetched models and displays four rows; filter total and source footer also say 4. |
| 6 | conn-added: mid-scroll framing hides heading/new provider | fixed | `conn-added-app.webp` includes Kết nối AI, the introduction, and the newly added provider at the top of the list. |
| 7 | conn-models: missing provider cards | fixed | `conn-models`: OpenRouter header and catalog are enclosed in one outlined rounded provider card. |
| 8 | conn-models: absent unsaved-change strip | still wrong | `conn-models-app.webp` includes the heading/top of the list but no Huỷ/Lưu strip; artboard includes one. |
| 9 | conn-manual: missing provider cards | fixed | `conn-manual`: provider/catalog enclosure is restored. |
| 10 | conn-manual: absent unsaved-change strip | still wrong | `conn-manual-app.webp` has no strip between intro and providers. |
| 11 | conn-manual: typed name, inserted selected row, and success identify different models | not verifiable | The new field and manual row both say `deepseek/deepseek-v3.2`, improving the old mismatch. But that row is unchecked and no successful result is visible; the green result area is outside the capture. The completed manual-add association is not demonstrated. |
| 12 | conn-failed: missing provider cards | fixed | `conn-failed-app.webp` shows enclosing provider surfaces, including an outlined failed OpenRouter card. |
| 13 | conn-failed: ordinary success list substituted for failure | fixed | `conn-failed-app.webp` now contains the 401 warning, failed status, and all three recovery actions. |
| 14 | grok-code: missing provider enclosure | fixed | `grok-code-app.webp` encloses Grok identity, pending status, and device-code panel in one bordered card. |
| 15 | grok-out: missing desktop signed-out-state capture | fixed | `grok-out-app.webp` shows the signed-out explanation and sole primary Đăng nhập Grok action. |
| 16 | agents: red/pink director orb instead of purple | fixed | `agents-app.webp` now has a purple Đạo diễn orb. |
| 17 | agents: info explanation inside four-agent card | fixed | `agents-app.webp`: the card closes after Kiểm định; the info-icon explanation starts below its bottom edge. |
| 18 | agents-director: wrong director orb color | fixed | `agents-director-app.webp` shows the purple orb. |
| 19 | agents-director: info explanation inside four-agent card | fixed | `agents-director-app.webp` shows the separate info line below the closed card. |
| 20 | agents-artist: no desktop artist-picker counterpart | fixed | `agents-artist-app.webp` is now a real popover anchored to the artist field, not a reused director or phone shot. Its title is still wrong; see differences. |
| 21 | m-connect: missing provider cards | fixed | `m-connect-app.webp` has distinct rounded provider cards and colored circular avatars. |
| 22 | m-add: duplicated connection primary | fixed | `m-add-app.webp` has exactly one full-width Kết nối và lấy model primary. |
| 23 | m-add: borderless key entry instead of outlined password field | still wrong | `m-add-app.webp` still presents `sk-…` and the eye icon on a borderless row; the artboard has a full-width rounded outlined field. |
| 24 | m-add: missing dimmed backdrop provider card | fixed | `m-add-app.webp` retains the rounded Tương thích OpenAI card under the scrim. Different provider identity is fixture data. |
| 25 | m-models: huge mostly-empty sheet | fixed | `m-models-app.webp` now uses its tall sheet for an actual 12-model catalog rather than two rows plus blank space. Its height is explained by inventory; requiring three artboard rows would be an invalid live-data comparison. |
| 26 | m-models: desktop grid truncates model names/descriptions | fixed | `m-models-app.webp` shows full identifiers, a second description line, and a right-aligned capability pill; no stray far-right price column or Gợi ý tag crowds the names. |
| 27 | m-models: checked rows but Chưa chọn model nào summary | not verifiable | A checked `gpt-6.1-sol` row is visible, but the summary is below the captured scrolling list. The screenshot cannot establish its count or price-unit explanation. |
| 28 | m-agents: wrong director orb color | fixed | `m-agents-app.webp` shows a purple director orb at normal emphasis. |
| 29 | m-agents: drawing sources merged into one card | fixed | `m-agents-app.webp` has separate outlined rounded ChatGPT and Grok source cards with a gap. |
| 30 | m-agents: selected source lacks selected-card treatment | still wrong | Neither source in `m-agents-app.webp` is visibly selected: both radio rings are empty and both cards neutral. There is no counterpart to the artboard selected ChatGPT radio/red border/tint. This capture cannot substantiate the claimed selected treatment. |
| 31 | m-agents: paid source lacks price/estimate/daily-limit information | not verifiable | `m-agents-app.webp` contains only subscription sources ChatGPT and Grok. There is no paid/API drawing-source tile on which to inspect those fields. |
| 32 | m-director: missing Thêm model ở Kết nối AI footer | fixed | `m-director-app.webp` contains the footer link at the bottom. |
| 33 | m-artist: missing Thêm model ở Kết nối AI footer | fixed | `m-artist-app.webp` contains the footer link at the bottom. |
| **Total** | **33 round-1 majors** | **25 fixed / 5 still wrong / 3 not verifiable** | **No unobserved result is marked fixed.** |

Other clear round-1 improvements: shared desktop navigation is enclosed in one rounded group without a selected Bàn dựng tab; Duyệt/Kênh icons now match; circular provider avatars are restored; desktop checkboxes now have white unchecked faces and white selected ticks; the manually added row has Thêm theo tên; agent model/provider labels are adjacent instead of far apart; no-model agents are no longer globally dimmed; the phone artist title and explanation now match; picker footer links are restored; phone primary duplication and oversized mostly-empty picker content are gone. The old mobile title/surface/radio complaints must not simply be copied forward wholesale.

## 2. New or remaining visible differences

### Shared desktop chrome — counted once each

- [minor] all desktop connect states · right information card: artboard body copy fits a shorter Hai cách kết nối card ending around image y=235; app wraps the same explanation into more lines and extends the card to around y=250, pushing An toàn and its bullets lower. This is a text-density/wrapping difference, not different live provider data.
- [minor] all desktop states · sidebar footer: artboard uses a compact standalone Viderse version line; app exposes a long build identifier/commit plus `+dirty`, wrapping the footer into additional lines and making it denser. The actual version value is exempt; the extra build/debug presentation is the visible difference.

### Desktop · conn-list

No additional scored difference beyond shared chrome. Card grouping, avatars, heading alignment, and navigation are substantially corrected. This pair does **not** validate signed-in Grok: the artboard is signed in and the app is signed out. The extra compatible-provider card and OpenRouter-versus-Gemini inventory are live fixtures, not defects.

### Desktop · conn-add

- [minor] conn-add · provider tile surfaces: artboard gives all available provider tiles a discernible dark rounded tile boundary; the app nonselected tiles are much flatter/less outlined, especially the lower-row choices. The selected tile is identifiable, but the grid loses the artboard consistent tile enclosure.
- [minor] conn-add · vertical framing: artboard page heading/right card start at the same baseline as conn-list; app heading, sidebar, and right card are shifted slightly upward compared with its own conn-list shot. This is a small residual capture offset, separate from the legitimate extra-provider height.

The desktop key field, primary, and storage helper are not fully in frame; this is recorded as partial evidence below rather than an invented unseen field-style finding.

### Desktop · conn-added

- [major] conn-added · unsaved-change region: artboard has the full-width 1 thay đổi chưa lưu ở Kết nối AI · dùng cho cả máy này strip with information icon, Huỷ, and Lưu above the new provider; app has no such region despite showing the top of the page.
- [minor] conn-added · initial catalog filter: artboard opens the new provider on Gợi ý; app opens on Tất cả, despite its own success notice saying Viderse lọc sẵn model hợp làm việc ở mục Gợi ý. This is a contradictory initial filter state, not a different returned-model count.
- [minor] conn-added · expanded model trigger: artboard expanded Chọn model trigger retains an upward chevron; app shows plain Chọn model without the expanded-state chevron.

The catalog is no longer empty. Different model totals/identities and a neutral compatible-provider identity instead of purple OpenRouter are not scored.

### Desktop · conn-models

- [major] conn-models · unsaved-change region: artboard keeps the information/Huỷ/Lưu strip above the expanded catalog provider; app omits it.
- [minor] conn-models · capture alignment: artboard heading starts around y=65, matching conn-list; app heading is around y=57, with the sidebar and right card similarly shifted upward. The extra providers explain where the catalog appears, but not this small whole-content offset.

Only the upper catalog is visible in the primary app capture; its final summary/action is not verified. Different provider order and row counts are not defects.

### Desktop · conn-manual

- [major] conn-manual · unsaved-change region: artboard shows Huỷ/Lưu above the provider while app has no strip.
- [major] conn-manual · completed manual-add state: artboard shows the typed identifier as a newly inserted **checked** manual row and an associated green success result; app has the same identifier in an **unchecked** row and the input, with no green result visible. This is not evidence of the requested successful manual-add state. The old cross-name mismatch is improved, but its end-to-end visual acceptance remains unproven.
- [minor] conn-manual · capture alignment: artboard heading/sidebar/right card use the normal top baseline; app shifts them upward by roughly the same small amount as conn-models.

### Desktop · conn-failed

- [minor] conn-failed · failed-provider header actions: artboard failed header has Thử kết nối and Đổi khoá, with the recovery actions inside the warning; app adds a Model · 2 dropdown in that header and substitutes Thử lại for the header test action. This changes the failed-state action hierarchy even though the warning itself is now present.
- [minor] conn-failed · recovery action component: artboard Thêm theo tên is a quiet/text action inside the warning; app gives it a bordered button, making all three recovery choices equally boxed.

The error panel is genuinely present. Round 1 called the outer failed border orange, but the current artboard itself is subtler/provider-toned; do not resurrect an orange-border requirement from that wording. Error copy and warm warning surface are substantially aligned.

### Desktop · grok-out

No additional scored difference beyond shared chrome. The specific signed-out explanation and single primary action now have a matching app capture.

### Desktop · grok-code

- [minor] grok-code · pending status tone: artboard emphasizes Đang đăng nhập · chờ bạn nhập mã trên trang xAI in warm yellow; app renders the line much closer to pale neutral text, reducing the pending-state emphasis. The enclosing pending card and device-code controls are restored.

Code value/expiry and provider inventory are not scored. The device-code panel now has copy, open-page, cancel, waiting indicator, and expiry content in the intended arrangement.

### Desktop · agents

- [major] agents · current drawing-source state: artboard identifies ChatGPT as Mặc định with a selected radio and a red outlined/tinted source card; app shows empty radios for both displayed sources, no selected card, and no Mặc định marker. The claimed selected-source styling is not demonstrated by this app state. This is a visible state/evidence mismatch; it does not prove the styling would fail after selecting a source.
- [minor] agents · director fallback action copy: artboard director fallback control says + Thêm; app says + Thêm nguồn, conflating the model-fallback action wording with the separate drawing-source action.

Model/source inventories are exempt. No paid drawing source is visible, so desktop price/daily-limit explanatory content is also unverified. The restored info boundary and purple director identity are confirmed, not remaining findings.

### Desktop · agents-director

- [minor] agents-director · director fallback action copy: artboard uses + Thêm; app uses + Thêm nguồn.
- [minor] agents-director · visible current selection: artboard shows the current field value in the open list with a selected row and right-edge tick; app field says `openai/gpt-5.2`, but the captured visible list ends with `minimax/minimax-m2.5` and shows no selected row/tick. A scrolled row may exist outside the visible list, so this is a capture/initial-visible-selection difference, **not** a claim that the model is absent from the underlying catalog.

The old leading radio column and blue full-popover focus outline are gone. The group inventories and generated descriptions are permitted differences.

### Desktop · agents-artist

- [minor] agents-artist · picker title copy: artboard says Model viết mô tả ảnh; app still says Model của Hoạ sĩ. The phone artist title has been fixed, but the desktop popover still drops the writing-versus-drawing distinction.
- [minor] agents-artist · director fallback action copy: visible artboard action is + Thêm; app is + Thêm nguồn.
- [minor] agents-artist · visible current selection: artboard current model is visible with a right-edge selection tick; app current field says `openai/gpt-5.2`, but no selected model is visible in its captured popover. As with the director, hidden scroll content cannot be ruled out from a still image.
- [minor] agents-artist · framing/scroll: artboard retains Trợ lý AI and the introduction above the card; app is scrolled enough to remove the page heading and clip the upper page context while preserving its fixed top bar. This complicates direct vertical-position acceptance, although the actual artist popover is now present and correctly anchored to its field.

### Phone · m-connect

- [minor] m-connect · ChatGPT secondary action component: artboard Đăng xuất has the same full outlined button enclosure as the neighboring Model action; app Đăng xuất is a quiet unboxed label in that half of the action row.
- [minor] m-connect · local-provider connection pill: artboard uses the compact Trên máy label; app uses Trên máy này. This is literal UI copy, not the live local address.

The page content aligns substantially after allowing for the simulated status bar. The extra provider legitimately pushes the add row toward/beyond the lower edge. Signed-in mobile Grok quota/actions are not covered by the signed-out app fixture.

### Phone · m-add

- [major] m-add · API-key component: artboard has a clearly bounded full-width rounded password field between its label and helper text; app still has a borderless `sk-…` entry with a visibility icon floating at the right. Removing the duplicate primary did not fix the field enclosure.
- [minor] m-add · provider tile contrast/enclosure: artboard available tiles retain dark rounded surfaces and outlines throughout the grid; app nonselected lower-row tiles are much less enclosed, appearing as text floating on the sheet. Different disabled/connected providers are exempt.
- [minor] m-add · sheet placement: with both sheets bottom-anchored at the same image edge, app top/grabber/title are about 13 image pixels lower than the artboard. This is a small remaining content/spacing difference, not the old oversized-title/high-sheet problem.

The one primary, unadorned storage helper, backdrop card, and circular avatar are now correct. The empty-key disabled action is explicitly exempt under #26. Selecting OpenAI instead of OpenRouter is fixture data; it does not explain the missing field border.

### Phone · m-models

- [minor] m-models · checked checkbox color: artboard checked boxes are pink/red with dark ticks; app checked `gpt-6.1-sol` box is lavender with a dark tick. Size/rounded shape are much improved. Fidelity-Connect claiming that this artboard uses purple is contradicted by the supplied `m-models-artboard.webp` pixels; the red selection token is also documented in `03-tokens.md`.

The sheet contains a genuinely longer catalog, so its tall height is not itself a defect. The app is ChatGPT, not the artboard Gemini; a paid catalog with prices and its footer summary remain unverified. Do not use the old two-checked-rows/zero-summary PNG as evidence of the new hidden footer.

### Phone · m-agents

- [major] m-agents · selected drawing-source state: artboard ChatGPT source is selected with a red radio, red outline/tint, and the other sources unselected; app ChatGPT and Grok both have empty radios and neutral cards. The source cards are now separate, but there is still no visually selected source to validate the claimed selected-card treatment.
- [minor] m-agents · Kiểm định row density: artboard fits Chấm hoàn thiện, đo âm lượng, xuất on one line; app wraps xuất onto a second line, enlarging that row and pushing the drawing-source section down. Other formerly broken artist-title wrapping and no-model row dimming are corrected.

Only subscription sources are shown. Missing Gemini/OpenRouter source inventory is not scored as a product defect, but it prevents accepting paid-source price/estimate/limit presentation.

### Phone · m-director

- [minor] m-director · capability component: artboard uses compact colored pill badges for Chữ · đọc ảnh, Chữ, and Vẽ ảnh; app renders those capabilities as much larger plain text without the pill surface/shape, visually dominating the row right edge.
- [minor] m-director · footer icon: artboard footer is plain Thêm model ở Kết nối AI text; app prepends a key icon, changing its alignment and treatment.

The exact job-description helper is restored, image-only models now say Chỉ vẽ ảnh, không viết chữ, selected radio treatment is substantially corrected, and the footer is present. Different group counts explain much of sheet height/placement; no height defect is inferred solely from those live inventories. No extra reasoning control is visible in this latest shot.

### Phone · m-artist

- [major] m-artist · selected model violates the sheet own capability requirement: the app highlights and selects `openai/gpt-5.2` while dimming that same row and labeling it Không đọc được ảnh. Its subtitle says the artist needs image-reading capability; the artboard selected row is enabled and image-capable. This is an internal selected/disabled contradiction, not a different model name or a generated editorial description.
- [minor] m-artist · capability component: artboard uses compact colored badges; app displays large plain capability text with no pill background, including on disabled rows.
- [minor] m-artist · footer icon: artboard uses plain footer text; app adds a leading key icon.
- [minor] m-artist · sheet surface tone: artboard artist sheet is near-black like its director sheet; app artist sheet is visibly lighter grey than the artboard and than its own director sheet. The common sheet title/copy fixes do not remove this artist-specific surface difference.

The title Model viết mô tả ảnh and concrete vision-requirement helper now match. Disabled image-only rows use the appropriate Chỉ vẽ ảnh, không viết chữ explanation; do not repeat that round-1 copy defect.

## 3. Coverage / states missing a pair

**No complete requested pair is missing: 17/17 WebP pairs exist and were read.** However, a filename pair is not equivalent to complete state acceptance:

| State | Remaining evidence limitation |
|---|---|
| conn-list / signed Grok | App Grok is signed out. `grok-out` is correctly covered separately; signed-in desktop Grok is not validated by the primary list pair. Supplementary `conn-models-openrouter-app.webp` shows signed-in Grok, but not a matching primary list-state capture. |
| conn-add | App image cuts through the lower key/primary region. The desktop key-field enclosure, full primary, and storage helper cannot all be compared. |
| conn-models | OpenRouter is fourth in the app list; the catalog extends below the viewport. Summary and Gán cho trợ lý cannot be accepted from the supplied frame. The supplementary OpenRouter shot also cuts off the bottom and does not resolve this. |
| conn-manual | The successful green result is below the viewport/not visible, and the manual row is unchecked. A completed success-state pair is still not established. |
| agents-director / agents-artist | The selected field value is not visible in the captured portion of either desktop popover; right-edge selected-tick/current-row styling is not verified. Do not assume the rest of a potentially scrolling list is absent. |
| m-connect | App Grok is signed out; the artboard signed-in mobile weekly quota/actions are not demonstrated. |
| m-models | Wrong-provider fixture (ChatGPT versus Gemini); footer summary/price-unit copy is outside the captured list. No supplied phone paid-provider catalog establishes price wrapping or selection-summary consistency. |
| agents / m-agents | App source inventory contains no paid/API drawing-source tile. Paid cost estimates and daily self-spend-limit explanation remain unverified. Neutral/unselected source fixtures also fail to demonstrate the selected-card state. |

### Accepted-deviation / weak-excuse audit

- **#19:** excluding Giới hạn từng kênh until its route exists is consistent with the stated staged scope. It remains visibly absent, but is not counted.
- **#26 Grok plan, monthly spend, generated notes, reset times, empty-key disabled primary:** reasonable source/state limitations on this evidence; no fabricated values are demanded and these differences are not counted.
- **#26 phone reasoning placement:** the explanation “artboard has no place to edit it” is weak because W13 explicitly reserves director reasoning/fallback editing for desktop. Per assignment it is still accepted and excluded; moreover no reasoning control is visible in the latest director shot, so the proposed deviation is not demonstrated here.
- **Claims not covered by #26:** a borderless password field, absent Huỷ/Lưu strip, purple selection checkbox, missing capability-pill component, wrong desktop artist title, and selecting a row explicitly marked incapable are not data-source limitations. The entry cannot reasonably excuse them.
- Missing paid-source fixtures are a **coverage limitation**, not proof that the paid cost fields were fixed. Likewise a cropped summary does not establish that contradictory selection counts were fixed.

### Older PNG contradictions

Directly read the older `app-conn-list.png`, `app-conn-failed.png`, `app-m-models.png`, `app-m-add.png`, `app-agents.png`, and `app-m-artist.png` for supersession checks. They contradict the new WebPs in material ways: the old provider list has no provider surfaces/avatars; old failed capture is a success list; old phone add has two primaries; old catalog has two checked clipped rows plus a zero-selection summary and blank space; old agents has a red director orb and encloses the info text; old artist phone picker has the wrong title/helper and no footer. **Those are stale evidence and were not used to retain resolved findings.** Conversely the new phone catalog hides its footer, so the older visible contradiction cannot be used either to condemn or to accept the newest summary. Not every legacy PNG was reread; all required newest pairs were.

## 4. Count tables

### Round-1 major disposition

| Status | Count |
|---|---:|
| fixed | 25 |
| still wrong | 5 |
| not verifiable | 3 |
| **Total round-1 major observations** | **33** |

### Current difference bullets (shared items counted once)

| State / scope | Majors | Minors |
|---|---:|---:|
| Shared desktop chrome | 0 | 2 |
| conn-list | 0 | 0 |
| conn-add | 0 | 2 |
| conn-added | 1 | 2 |
| conn-models | 1 | 1 |
| conn-manual | 2 | 1 |
| conn-failed | 0 | 2 |
| grok-out | 0 | 0 |
| grok-code | 0 | 1 |
| agents | 1 | 1 |
| agents-director | 0 | 2 |
| agents-artist | 0 | 4 |
| m-connect | 0 | 2 |
| m-add | 1 | 2 |
| m-models | 0 | 1 |
| m-agents | 1 | 1 |
| m-director | 0 | 2 |
| m-artist | 1 | 3 |
| **Total current bullets** | **8** | **29** |

The eight major bullets include one **completed-manual-state evidence mismatch**, and two **selected-source evidence/state mismatches**; they are not eight independently runtime-reproduced implementation bugs. Missing/cropped coverage in §3 is not counted again as additional defects. Accepted deviations are not in these totals.

Status: DONE
Summary: **8 majors remaining / 29 minors remaining** under the explicit bullet-count convention; **25/33 round-1 majors fixed, 5 still wrong, 3 not verifiable**. Worst 3: **(1)** missing unsaved-change/Huỷ/Lưu region across added/models/manual; **(2)** artist phone picker selects a model it explicitly labels unable to read images; **(3)** phone add still has a borderless API-key field. Selected-source treatment and completed manual success additionally lack valid matching-state evidence.
