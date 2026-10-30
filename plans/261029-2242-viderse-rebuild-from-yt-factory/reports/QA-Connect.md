# Independent visual QA — Connect / Agents

## Scope and evidence

Inspected **all 31 supplied PNGs** in `/tmp/viderse-fidelity/connect/shots/`: 17 artboards and 14 application captures. Desktop files are 1800×1125 and phone files 488×1055, corresponding to the requested 1440×900 and 390×844 logical viewports at approximately 1.25× scale. No product files were changed, no application tests/builds were run, and no additional browser tabs were opened.

Compared all six `conn` values; all three `grok` values (`signed` is represented by `conn=list`); all three `ai` values; and both mobile screens with their relevant `sheet=none|add|models|director|artist` states. Read the supplied design requirements in `01-ia-flows.md`, `03-tokens.md`, `04-components.md`, `05-kit.md`, `06-copy.md`, `11-hoi-dap.md`, the artboard tweak inventory and `Fidelity-Connect.md`.

**Counting:** each severity bullet is one visible difference or one missing-state evidence failure. Shared chrome/layout defects are repeated and counted for each available state in which they are visible; totals are therefore state-observation counts, not unique implementation bug counts. A missing entire state is counted once, not as every unobserved element. Findings describe the supplied captures; they do not imply that an unobserved control cannot exist elsewhere or that a visual symptom has been runtime-reproduced.

**Excluded:** live account/provider/model inventories, names, numeric quota/price/time values, version strings and activity badges; simulated phone system status/home bars; transient hover/focus on individual inputs/buttons; differences already explained by the area’s legitimate hỏi-đáp proposals. In particular, omitted Grok plan, unavailable monthly spend, generated model-description text and real/fallback reset times are not scored. Missing price/limit explanations and internally contradictory selections/results are not merely different live values. Whole-popover blue focus styling is still called out because it visibly differs from both the artboard and the red focus token.

### Proposal/claim audit

- Proposal #1 (Grok plan), #2 (monthly spend), #5 (editorial model notes) and #7 (reset time) are plausible source/data limitations on the evidence supplied; this report does not invent replacement data or count those omissions. Their backend claims were not independently runtime-tested.
- Proposal #3 says the phone catalog has no search/filter, consistent with the artboard. It does **not** justify using a desktop-width row layout that clips the identifier on a phone.
- Proposal #4 concerns a removal action not open in these screenshots; no finding is inferred about that unshown state.
- Proposal #6 is weak: W13 explicitly reserves reasoning/fallback editing for desktop, rather than simply failing to provide a mobile control. The added mobile reasoning control is scored below.
- Hỏi-đáp #15 is open and describes a **slight** difference in orb shading. It does not explain turning the director from purple to saturated red/pink or dimming whole no-model agent rows.
- The implementation report’s repeated Khớp assertions are contradicted by the actual pixels: card surfaces/avatars disappear, `app-conn-failed` is not a failed state, the mobile model summary contradicts its checkboxes, and the single desktop picker image is only Đạo diễn.

## Missing / mismatched screenshot coverage

| State | Evidence gap |
|---|---|
| Desktop `conn=add` | `art-conn-add.png` has no app counterpart. `app-conn-added.png` is not an add form. |
| Desktop `grok=out` | `art-grok-out.png` has no full matching app shot. `app-conn-list.png` shows signed-in Grok, contrary to the implementation report’s mapping. |
| Desktop `ai=director` | No state-named `app-agents-director.png`, but `app-agents-picker.png` is a genuine director-state substitute. This is **not** counted as missing state coverage. |
| Desktop `ai=artist` | `art-agents-artist.png` has no desktop app counterpart. Neither the director popover nor `app-m-artist.png` can stand in for it. |
| Desktop `conn=failed` | A filename exists, but `app-conn-failed.png` contains the normal successful list. The intended failed state is missing from the evidence. |
| Mobile `sheet=models` | Both shots exist, but the artboard is Gemini and the app is ChatGPT; the report incorrectly calls the artboard OpenRouter. Geometry/selection contradictions are assessable, paid-provider state coverage is not established. |
| Mobile add / connect / agents | Both shots exist, but provider/authentication fixtures differ. Missing particular provider rows are not automatically product bugs. |

**No supplied app PNG lacks an artboard for its actual state:** the generic `app-agents-picker.png` maps to the director artboard. No artboard had to be rendered to inspect these states; the missing evidence is on the application side.

## Desktop · connect / conn=list / grok=signed

**Shots:** `art-conn-list.png` ↔ `app-conn-list.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Provider regions: artboard wraps each provider, its actions, and quota in a distinct dark rounded card with a visible outline; app lays the providers directly on the page background with no enclosing cards, flattening the hierarchy and shortening the list.
- [minor] Provider avatars: artboard shows substantial coloured circular letter avatars (mint ChatGPT, neutral Grok/Ollama, blue Gemini, lavender OpenRouter); app shows bare white letters without the circular coloured backgrounds.
- [minor] Grok status tone: artboard uses muted grey for the signed-in Grok status line; app renders the line green, the same emphasis as the ChatGPT success line.

## Desktop · connect / conn=add

**Shots:** `art-conn-add.png` ↔ `MISSING app-conn-add.png`

- [major] State coverage: artboard shows an expanded Thêm nhà cung cấp card with the eight-tile grid and API-key form; no application screenshot shows the desktop add-provider form. app-conn-added.png instead shows provider cards, a model catalog and an authentication failure, not this state.

No desktop add form can be visually accepted from the supplied evidence. The mobile add sheet is not a desktop counterpart.

## Desktop · connect / conn=added

**Shots:** `art-conn-added.png` ↔ `app-conn-added.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Provider regions: artboard wraps each provider, its actions, and quota in a distinct dark rounded card with a visible outline; app lays the providers directly on the page background with no enclosing cards, flattening the hierarchy and shortening the list.
- [minor] Provider avatars: artboard shows substantial coloured circular letter avatars (mint ChatGPT, neutral Grok/Ollama, blue Gemini, lavender OpenRouter); app shows bare white letters without the circular coloured backgrounds.
- [major] Unsaved-change region: artboard shows the full-width 1 thay đổi chưa lưu ở Kết nối AI · dùng cho cả máy này strip with Huỷ and Lưu above the new provider; app has no corresponding strip in its captured added-provider state.
- [major] New catalog contents: artboard shows recommended model rows directly under the successful connection notice; app’s notice says 4 models were fetched, but the search and footer say 0 models and the catalog is completely empty. The contradiction within the app capture is not a difference in live model names or counts.
- [major] Capture framing: artboard shows the Kết nối AI heading, introduction and newly added provider from the top of the page; app is captured mid-scroll beneath the fixed header, with the heading, upper sidebar and top of the explanatory card cut off and unrelated provider/error rows filling the lower screen.

The provider name and model count may legitimately differ. The missing recommended rows and internally contradictory total cannot be dismissed as that live-data difference. Because the app shot is scrolled, absence of the unsaved strip in this image is a capture/coverage failure, not proof that no off-screen strip exists.

## Desktop · connect / conn=models

**Shots:** `art-conn-models.png` ↔ `app-conn-models.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Provider regions: artboard wraps each provider, its actions, and quota in a distinct dark rounded card with a visible outline; app lays the providers directly on the page background with no enclosing cards, flattening the hierarchy and shortening the list.
- [minor] Provider avatars: artboard shows substantial coloured circular letter avatars (mint ChatGPT, neutral Grok/Ollama, blue Gemini, lavender OpenRouter); app shows bare white letters without the circular coloured backgrounds.
- [major] Unsaved-change region: artboard shows the information icon, 1 thay đổi chưa lưu ở Kết nối AI · dùng cho cả máy này, Huỷ and Lưu between the intro and catalog card; app shows no such region in this top-of-page capture.
- [minor] Catalog checkboxes: artboard’s unchecked boxes are bright white and selected boxes have white ticks on pink; app’s unchecked boxes are dark grey and selected boxes have dark ticks, changing the contrast and selected-state appearance.

Provider/model identities, fetched totals, number of returned rows, prices and timestamps are live data and are not counted. Curated model descriptions versus generated capability/context notes are excluded under Fidelity-Connect proposal #5.

## Desktop · connect / conn=manual

**Shots:** `art-conn-manual.png` ↔ `app-conn-manual.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Provider regions: artboard wraps each provider, its actions, and quota in a distinct dark rounded card with a visible outline; app lays the providers directly on the page background with no enclosing cards, flattening the hierarchy and shortening the list.
- [minor] Provider avatars: artboard shows substantial coloured circular letter avatars (mint ChatGPT, neutral Grok/Ollama, blue Gemini, lavender OpenRouter); app shows bare white letters without the circular coloured backgrounds.
- [major] Unsaved-change region: artboard keeps the Huỷ/Lưu unsaved-change strip above the provider; app omits it.
- [major] Manual-test result association: artboard’s typed model name, newly inserted checked model row and green success result all identify the same model; app’s field contains deepseek/deepseek-v3.2 while its inserted row and success message identify moonshotai/kimi-k3-thinking. The success message therefore does not describe the visible input; this is not just a different live model name.
- [minor] Manually added model annotation: artboard marks the new model row Thêm theo tên; app gives its manually added row only a capability/context description, losing the visible indication that it was added by name. This source annotation is separate from proposal #5’s editorial model descriptions.
- [minor] Catalog checkboxes: artboard shows white unchecked boxes and white selected ticks; app shows dark unchecked boxes and dark ticks on the pink selected boxes.

## Desktop · connect / conn=failed

**Shots:** `art-conn-failed.png` ↔ `app-conn-failed.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Provider regions: artboard wraps each provider, its actions, and quota in a distinct dark rounded card with a visible outline; app lays the providers directly on the page background with no enclosing cards, flattening the hierarchy and shortening the list.
- [minor] Provider avatars: artboard shows substantial coloured circular letter avatars (mint ChatGPT, neutral Grok/Ollama, blue Gemini, lavender OpenRouter); app shows bare white letters without the circular coloured backgrounds.
- [major] Failed-state region: artboard shows the failed OpenRouter card first, outlined orange, with the orange 401 explanation and Đổi khoá · Thử lại · Thêm theo tên; app-conn-failed.png shows the ordinary successful four-provider list, with no failed provider, warning panel or recovery actions. It is visually the same state as app-conn-list.png, so the claimed failed-state comparison is not present.

## Desktop · connect / grok=code

**Shots:** `art-grok-code.png` ↔ `app-grok-code.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Provider regions: artboard wraps each provider, its actions, and quota in a distinct dark rounded card with a visible outline; app lays the providers directly on the page background with no enclosing cards, flattening the hierarchy and shortening the list.
- [minor] Provider avatars: artboard shows substantial coloured circular letter avatars (mint ChatGPT, neutral Grok/Ollama, blue Gemini, lavender OpenRouter); app shows bare white letters without the circular coloured backgrounds.
- [minor] Pending-login outline: artboard gives the whole Grok pending-login provider a warm warning outline around its header and code panel; app has only the neutral border around the inner code panel and no enclosing warning outline.

Different device codes are expected live values. The app capture has only ChatGPT and Grok whereas the artboard also includes Gemini/Ollama; provider inventory is not counted as a product styling defect. The code controls and expiry sentence themselves are visible in both.

## Desktop · connect / grok=out

**Shots:** `art-grok-out.png` ↔ `MISSING app-grok-out.png`

- [major] State coverage: artboard shows a logged-out Grok card with Chưa đăng nhập · cần gói SuperGrok hoặc X Premium and only the primary Đăng nhập Grok action; there is no matching app state shot. app-conn-list.png, cited as the substitute in Fidelity-Connect.md, actually shows signed-in Grok with Model, Thử kết nối, Đăng xuất and a weekly quota.

A logged-out Grok row is incidentally visible at the bottom of app-conn-added.png and in the mobile connect shot. Neither establishes the full desktop grok=out layout or makes app-conn-list.png the claimed counterpart.

## Desktop · agents / ai=none

**Shots:** `art-agents.png` ↔ `app-agents.png`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Đạo diễn identity: artboard shows a purple Đạo diễn orb; app shows a saturated red/pink orb. This is a different agent colour, not the slight orb shading discrepancy described in hỏi-đáp #15.
- [minor] Model field content alignment: artboard places ChatGPT · gói Plus immediately after the model identifier; app pushes that provider/plan string to the far right of both the director and artist fields, leaving a conspicuous empty gap.
- [minor] Fallback controls: artboard uses a dashed + Thêm button for director fallbacks and plain ordered model pills; app uses + Thêm nguồn and adds visible × removal affordances to its populated fallback pills.
- [minor] Agent row density: artboard has taller Trinh sát/Đạo diễn/Hoạ sĩ regions and more breathing room between labels, controls and fallbacks; app compresses those regions, moving the Hoạ sĩ start and Kiểm định row markedly upward. Differences in the number of available sources are excluded from this observation.
- [minor] Subscription source copy: artboard labels the ChatGPT drawing source Gói Plus · 50 ảnh mỗi ngày; app says Gói đang trả · 50 ảnh mỗi ngày, although its own model selectors above already identify the plan as gói Plus.
- [minor] Paid drawing-source information: artboard includes the paid API source’s daily self-spend-limit explanation as well as its prices; app’s paid OpenRouter tile shows price estimates but no daily-limit explanation. This is missing explanatory content, not a different price value.
- [major] Information-region boundary: artboard ends the four-agent card after Kiểm định and puts the info-icon explanation below that card; app encloses the explanation inside the same card, with another divider.
- [minor] Information copy duplication: artboard has the single detailed explanation below the agent card; app adds a second standalone Đổi model có hiệu lực từ bước chạy tiếp theo line beneath its already-present detailed explanation.

## Desktop · agents / ai=director

**Shots:** `art-agents-director.png` ↔ `app-agents-picker.png (semantic counterpart; no app-agents-director.png)`

- [minor] Desktop navigation: artboard shows one centred rounded container around all five navigation items, with no Bàn dựng selection while in Settings; app shows an isolated filled Bàn dựng tab and no shared enclosing container.
- [minor] Desktop navigation icons: artboard shows the review face/check icon for Duyệt and the outlined channel/play icon for Kênh; app shows a flag for Duyệt and a different, rounded play icon for Kênh.
- [minor] Settings sidebar: artboard shows Giới hạn từng kênh beneath Luồng chạy · hàng chờ, including its icon and chevron; app omits that entire navigation row.
- [minor] Desktop vertical alignment: artboard starts the page heading and right-hand information card about 6 CSS px lower; app starts both higher, and its shorter header also moves the sidebar group upward.
- [major] Đạo diễn identity: artboard shows a purple Đạo diễn orb; app shows a saturated red/pink orb. This is a different agent colour, not the slight orb shading discrepancy described in hỏi-đáp #15.
- [minor] Model field content alignment: artboard places ChatGPT · gói Plus immediately after the model identifier; app pushes that provider/plan string to the far right of both the director and artist fields, leaving a conspicuous empty gap.
- [minor] Fallback controls: artboard uses a dashed + Thêm button for director fallbacks and plain ordered model pills; app uses + Thêm nguồn and adds visible × removal affordances to its populated fallback pills.
- [minor] Agent row density: artboard has taller Trinh sát/Đạo diễn/Hoạ sĩ regions and more breathing room between labels, controls and fallbacks; app compresses those regions, moving the Hoạ sĩ start and Kiểm định row markedly upward. Differences in the number of available sources are excluded from this observation.
- [minor] Subscription source copy: artboard labels the ChatGPT drawing source Gói Plus · 50 ảnh mỗi ngày; app says Gói đang trả · 50 ảnh mỗi ngày, although its own model selectors above already identify the plan as gói Plus.
- [minor] Paid drawing-source information: artboard includes the paid API source’s daily self-spend-limit explanation as well as its prices; app’s paid OpenRouter tile shows price estimates but no daily-limit explanation. This is missing explanatory content, not a different price value.
- [major] Information-region boundary: artboard ends the four-agent card after Kiểm định and puts the info-icon explanation below that card; app encloses the explanation inside the same card, with another divider.
- [minor] Information copy duplication: artboard has the single detailed explanation below the agent card; app adds a second standalone Đổi model có hiệu lực từ bước chạy tiếp theo line beneath its already-present detailed explanation.
- [minor] Picker selection component: artboard uses a small selected check indicator at the right edge of the selected model row, with no leading radio column; app uses large radio circles at the left of every row and puts the selected marker there.
- [minor] Picker outline: artboard has a subtle neutral popover border and shadow; app has a conspicuous pale-blue outline around the entire open picker, rather than the artboard’s border or the specified red focus treatment.
- [minor] Disabled-model explanation: artboard says Chỉ vẽ ảnh, không viết chữ for an image-only model; app says Không viết được chữ, dropping the explicit explanation that it only draws images.

The open title is Model của Đạo diễn and the popover is anchored to the director field. Thus app-agents-picker.png is a director capture only. Different provider groups/model inventories are live data, not defects. Shared underlying-page differences are intentionally counted again for this state.

## Desktop · agents / ai=artist

**Shots:** `art-agents-artist.png` ↔ `MISSING desktop artist-picker app shot`

- [major] State coverage and wrong substitute: artboard shows Model viết mô tả ảnh anchored beneath the Hoạ sĩ field; app-agents-picker.png instead says Model của Đạo diễn and is anchored beneath Đạo diễn. app-m-artist.png is a phone sheet at a different viewport, not a desktop popover, so neither image is a valid counterpart for this artist-picker artboard.

Cannot assess artist-popover placement, clipping or desktop content from a director popover or a mobile sheet. No claim is made about unobserved desktop artist output.

## Mobile · connect / sheet=none

**Shots:** `art-m-connect.png` ↔ `app-m-connect.png`

- [major] Provider regions: artboard uses separate dark, outlined, rounded provider cards enclosing identity, quota and actions; app places all provider blocks directly on the page background, with no card surfaces or outlines.
- [minor] Provider avatars: artboard has coloured circular letter avatars; app has only small bare white letters.
- [minor] Connection-type pill copy: artboard abbreviates the API type to Khoá and local type to Trên máy; app’s visible API-provider pill says Khoá API rather than the compact mobile label.
- [minor] Secondary-action tone: artboard renders Đăng xuất and Đổi khoá in muted grey; app renders these outlined-button labels bright white, with the same emphasis as the model actions.

This is not the same account fixture: artboard has signed-in ChatGPT/Grok plus Gemini/Ollama; app has ChatGPT/OpenRouter and logged-out Grok. Inventory and authentication-state differences are recorded here for coverage, not counted as styling bugs. The supplied shot cannot validate the signed-in mobile Grok quota/actions or local-provider mobile card.

## Mobile · connect / sheet=add

**Shots:** `art-m-add.png` ↔ `app-m-add.png`

- [major] Primary action duplication: artboard has one full-width Kết nối và lấy model button below the key-storage hint; app has one beside the key field and a second full-width copy beneath the hint.
- [major] API-key field component: artboard shows a full-width, clearly outlined, rounded password field on its own row; app shows a borderless sk-… entry and visibility icon squeezed into a row with the extra action button.
- [minor] Sheet placement: artboard starts the add sheet around one-third down the image; app starts it around one-quarter down, covering the provider header much earlier. The difference remains after allowing for the artboard’s simulated status-bar inset.
- [minor] Sheet title typography: artboard’s Thêm nhà cung cấp is smaller and more restrained; app uses a noticeably larger, heavier title.
- [minor] Sheet surface tone: artboard shows a near-black sheet surface clearly darker than the app’s lighter grey sheet.
- [minor] Storage hint component: artboard presents Khoá chỉ lưu trên máy đang chạy Viderse, luôn hiện dạng che. as unadorned helper text; app prepends a key icon and indents the sentence, causing an additional wrapped line.
- [major] Visible backdrop provider card: artboard’s dimmed ChatGPT block still has its rounded card boundary; app’s visible dimmed ChatGPT header/quota floats directly on the page.
- [minor] Visible backdrop avatar: artboard retains a mint circular ChatGPT avatar under the scrim; app retains only the bare C.

Selection/key-value differences are not counted: artboard selects OpenRouter with a filled key; app selects OpenAI with an empty key. Those differences explain disabled actions, but not why the app renders two actions. The app shot does not validate the requested OpenRouter-selected form.

## Mobile · connect / sheet=models

**Shots:** `art-m-models.png` ↔ `app-m-models.png`

- [major] Sheet geometry: artboard uses a compact, content-height sheet occupying roughly the bottom 40% of the image, with the dimmed provider cards still visible; app expands its sheet to roughly 86% of the viewport and leaves most of it empty below two rows.
- [major] Model-row layout and truncation: artboard fits the model identifier, wrapped descriptive/price text and a right-aligned capability pill inside the phone sheet; app keeps a desktop-like grid with an extra far-right price column, truncating even the selected model name to gpt-6.… and the description to chữ · đọc ảnh · 2….
- [major] Selection summary consistency: artboard says Đã chọn 2 model beneath its checked rows; app visibly checks both rows but says Chưa chọn model nào. This is contradictory selection state within one image, not a live-data difference.
- [minor] Checkbox component: artboard shows large rounded pink checkboxes with dark ticks, suitable for touch; app uses much smaller desktop-style square checkboxes.
- [minor] Recommendation tags: artboard’s mobile list has no Gợi ý tags beside model identifiers; app adds green Gợi ý pills that consume the already-constrained identifier space.
- [minor] Footer explanation: artboard’s summary includes giá tính trên 1 triệu token hoặc mỗi ảnh; app’s summary has no price-unit explanation.
- [minor] Sheet title typography: artboard’s Model của … heading is smaller; app’s heading is noticeably larger and heavier.
- [minor] Sheet surface tone: artboard’s compact sheet is near-black; app’s large sheet has a lighter grey surface.

The artboard title is Model của Gemini, not OpenRouter as claimed in Fidelity-Connect.md; the app shot is Model của ChatGPT. Provider names, row counts, prices and source/time values are otherwise excluded as live data. The wrong-provider shot still exposes the layout and contradictory-summary issues above, but it does not validate the paid Gemini catalog state.

## Mobile · agents / sheet=none

**Shots:** `art-m-agents.png` ↔ `app-m-agents.png`

- [major] Đạo diễn identity: artboard uses a purple orb for Đạo diễn; app uses red/pink, a different identity colour rather than a slight gradient variation.
- [minor] No-model agents’ emphasis: artboard keeps Trinh sát and Kiểm định names, explanations and coloured orbs at normal emphasis; app dims both entire rows and orbs, making them look disabled merely because they do not use a model.
- [minor] Agent orb sizing: artboard uses relatively compact orbs alongside all four rows; app uses larger orbs, especially evident on the director and artist rows.
- [minor] Artist-row wrapping: artboard fits Hoạ sĩ · mô tả ảnh on one line and its description on one line; app breaks the title after mô tả and the description onto two lines, making that row substantially taller despite the same model value.
- [minor] Provider-plan labels: artboard shows ChatGPT · gói Plus under the director and artist values; app replaces both with ChatGPT · đăng nhập gói, losing the known plan name that is available in the desktop picker and mobile connect capture.
- [major] Drawing-source region boundaries: artboard gives each drawing source its own separately outlined, rounded card with space between cards; app merges its available sources into one enclosing card with an internal separator.
- [major] Selected drawing-source treatment: artboard gives the selected ChatGPT source a red outline and red-tinted card background; app shows only the red radio marker inside the neutral shared source card, without the selected card outline/tint.
- [minor] Drawing-source plan copy: artboard says Gói Plus · 50 ảnh mỗi ngày; app says Gói đang trả · 50 ảnh mỗi ngày, discarding the actual plan name.
- [major] Paid drawing-source cost information: artboard’s API-source card includes per-image pricing, estimated per-video cost and daily self-spend limit; app’s paid OpenRouter row shows only the model and Khoá API · trả theo ảnh, with no cost estimates or limit text. This is absence of fields, not different numeric prices.

Fewer source/provider choices and different fallback inventories are live-data differences and are not counted. The fallback switch card is visible in the app but falls partly below the artboard’s captured viewport; its visibility alone is not an extra-element defect.

## Mobile · agents / sheet=director

**Shots:** `art-m-director.png` ↔ `app-m-director.png`

- [major] Picker footer navigation: artboard ends the sheet with Thêm model ở Kết nối AI; app has no corresponding footer/link anywhere in the captured sheet, despite ample empty space.
- [minor] Explanatory copy: artboard says Chia cảnh, viết tiêu đề, mô tả và chữ trên ảnh bìa. Mờ là model chỉ vẽ ảnh.; app replaces the job description with Model đã chọn ở Kết nối AI. Mờ là model chỉ vẽ ảnh.
- [minor] Disabled-model explanation: artboard says Chỉ vẽ ảnh, không viết chữ; app uses Không viết được chữ, omitting the first half of the explanation.
- [minor] Selected radio appearance: artboard uses a small centre dot inside the selected pink ring; app uses a much larger filled centre with a second concentric ring.
- [minor] Sheet title typography: artboard uses the smaller Model của Đạo diễn heading; app uses a larger and heavier heading.
- [minor] Sheet surface tone: artboard uses a near-black sheet; app’s surface is lighter grey.
- [minor] Extra reasoning control: artboard has only the model picker and footer; app adds Mức suy nghĩ with Tự động / Thấp / Vừa / Cao inside the phone sheet. Fidelity-Connect proposal #6 calls this an absent-artboard-control workaround, but ia-flows W13 explicitly says reasoning and director fallbacks are edited only on desktop, so that justification conflicts with an existing rule rather than filling an unspecified gap.

Different provider groups/model rows and generated model descriptions are excluded as live data or proposal #5. The extra reasoning control is included because the proposed justification contradicts the explicit desktop-only requirement.

## Mobile · agents / sheet=artist

**Shots:** `art-m-artist.png` ↔ `app-m-artist.png`

- [major] Picker footer navigation: artboard shows Thêm model ở Kết nối AI at the bottom; app omits it, leaving the remainder of the sheet blank.
- [minor] Sheet title copy: artboard says Model viết mô tả ảnh; app says Model của Hoạ sĩ, dropping the distinction between the artist’s description-writing model and drawing source.
- [minor] Helper copy: artboard says Cần đọc được ảnh vì Hoạ sĩ xem khung đối thủ làm mẫu.; app says Model đã chọn ở Kết nối AI. Mờ là model không đọc được ảnh hoặc chỉ vẽ ảnh., replacing the concrete reason for requiring vision with a generic catalog rule.
- [minor] Disabled image-model explanation: artboard labels the image-only model Chỉ vẽ ảnh, không viết chữ; app labels it Không đọc được ảnh. This is a visible explanatory-copy difference, not a different live model identifier.
- [minor] Selected radio appearance: artboard uses a small centre dot in the pink ring; app uses a much larger filled centre with a second concentric ring.
- [minor] Sheet title typography: artboard’s title is smaller; app’s Model của Hoạ sĩ heading is visibly larger and heavier.
- [minor] Sheet surface tone: artboard’s sheet is near-black; app uses the lighter grey sheet surface.
- [minor] Sheet placement: artboard’s artist sheet starts lower than its director sheet and preserves the top of Trinh sát behind the scrim; app places both sheets at the same high top edge, leaving only the card’s upper boundary behind it. This is a smaller remaining offset after accounting for the simulated status-bar inset.

Different available model inventories, prices and generated model notes are excluded. A phone sheet is not evidence for the missing desktop artist picker.

## Count table

| State | Majors | Minors |
|---|---:|---:|
| Desktop · connect / conn=list / grok=signed | 1 | 6 |
| Desktop · connect / conn=add | 1 | 0 |
| Desktop · connect / conn=added | 4 | 5 |
| Desktop · connect / conn=models | 2 | 6 |
| Desktop · connect / conn=manual | 3 | 7 |
| Desktop · connect / conn=failed | 2 | 5 |
| Desktop · connect / grok=code | 1 | 6 |
| Desktop · connect / grok=out | 1 | 0 |
| Desktop · agents / ai=none | 2 | 10 |
| Desktop · agents / ai=director | 2 | 13 |
| Desktop · agents / ai=artist | 1 | 0 |
| Mobile · connect / sheet=none | 1 | 3 |
| Mobile · connect / sheet=add | 3 | 5 |
| Mobile · connect / sheet=models | 3 | 5 |
| Mobile · agents / sheet=none | 4 | 5 |
| Mobile · agents / sheet=director | 1 | 6 |
| Mobile · agents / sheet=artist | 1 | 7 |
| **Total state observations** | **33** | **89** |

Status: DONE
Summary: **33 majors / 89 minors** across 17 states. The three worst problems are: **(1)** provider cards and coloured avatars disappear across desktop/mobile connect screens, destroying the designed grouping; **(2)** the phone model catalog uses a mostly-empty oversized sheet with clipped model rows and says no models are selected while two are checked; **(3)** state coverage is falsely accepted—failed shows a success list, desktop add/logged-out Grok/artist-picker evidence is missing, and one director picker is treated as both desktop pickers.
