# QA3 — Connect / Agents

## Scope and evidence

Independent **live-app** visual QA against the design artboards, not a re-review of implementer screenshots. Built the isolated Go binary with the prescribed `GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4 lockf -k /tmp/verse-heavy.lock nice -n 10 go build` command; **did not rebuild the web**. Migrated and served an isolated database on **18784**, created a temporary administrator, and exercised the embedded application in headless Google Chrome.

Evidence: **17 primary PNG pairs** under `/tmp/viderse-qa3/connect/shots/`, named `art-<state>.png` and `app-<state>.png`, with computed-style/bounding-box JSON alongside them. Supplementary captures include `app-conn-added-dirty.png`, `app-conn-add-bottom.png`, and `app-m-models-two.png`. State shorthand below names these files. The artifact directory is outside the repository; this report is the only repository file written.

Browser execution used one locally launched Chrome from a Node JavaScript evaluation kernel/CDP adapter, one `browser.open` launch, localhost-only network allowlisting, the requested Chrome flags/profile, and deletion of `window.PublicKeyCredential`. The original launch survived kernel recovery; no second Chrome was launched. Desktop viewport: **1440×900**. Phone viewport: **390×844**, mobile/touch enabled. Needed `/api/ai/...` responses were intercepted through one `tab.run` registration, starting from real GET provider/agent/model payloads and the wire DTOs in `internal/web/ai.go`. The application UI, not synthetic DOM, produced the states. No real key, paid API call, or completed external login was used.

**Font-control correction:** the localhost allowlist blocks Google Fonts used by the artboards. Before final comparison, loaded the matching locally bundled Be Vietnam Pro and JetBrains Mono font faces into the reference tab, then **recaptured and read all 17 artboard images**. This changes no artboard layout rules or copy. Fallback-font wrapping is not a product defect: the earlier apparent help-card height, provider-line wrapping, and phone Kiểm định density differences disappear with the correct fonts. Phone-sheet captures were settled after their entrance animation; compositor-damaged intermediate captures were replaced.

Read QA2-Connect and design docs `01`, `03`, `04`, `05`, `06`, artboards README, and hỏi-đáp #18–#26. Accepted pending deviations are excluded. Provider/model identities, counts, quotas, timestamps, prices, and supported capabilities are fixture data, not styling defects. The artboard-only phone status bar is excluded; content alignment accounts for its 44px inset, while bottom sheets are compared bottom-to-bottom.

**Gate verdict: not clean. Seven of the eight round-2 major observations are fixed, but the initial `conn=added` Huỷ/Lưu state is still missing.** Selecting a model afterward produces the strip; that is a different state and must not be substituted for the artboard zero-selected newly-added state.

## 1. Round-2 majors

These are the **eight current-major bullets in QA2**, not its historical 33-item round-1 table.

| Round-2 major | Current disposition | Live evidence |
|---|---|---|
| `conn-added`: missing unsaved-change strip | **still wrong** | Followed Thêm nhà cung cấp → selected OpenRouter → entered a fictional key → Kết nối và lấy model, with only that API request intercepted. The newly added provider is first, Gợi ý is active, all rows are unchecked, and the yellow success/choose-model note is present, but **no `.conn-dirty`, Huỷ, or Lưu exists**. `conn-added` pair shows the page introduction directly followed by the provider. Runtime result: `dirty:false`, `selected:[]`. In contrast, the artboard has its strip even with Đã chọn · 0. `app-conn-added-dirty.png` proves the narrower model-toggle case now works, not that the original case is fixed. |
| `conn-models`: missing unsaved-change strip | **fixed** | Opened the provider catalog and changed a checkbox. `conn-models` pair shows the information icon, `1 thay đổi chưa lưu ở Kết nối AI · dùng cho cả máy này`, Huỷ, and the bright Lưu above the card. |
| `conn-manual`: missing unsaved-change strip | **fixed** | `conn-manual` shows the same full strip above the enclosing provider/catalog card after successful manual addition. |
| `conn-manual`: no demonstrated checked manual row/success result | **fixed** | Tested a name absent from the fixture catalog, `deepseek/deepseek-v3.2`. The new first row is **checked**, says **Thêm theo tên**, and identifies the same model as the input and the green `Đã thêm và chọn` result. All are visible together in `app-conn-manual.png`. The check response supplies capabilities/context, not invented pricing; no finding is inferred from absent manual-model pricing. |
| Desktop `agents`: selected drawing source not demonstrated | **fixed** | `agents` shows ChatGPT with a red selected radio, red outline/tint, and **Mặc định**. Grok/Gemini remain neutral. A paid Gemini source is present too, so its per-image cost, seven-image estimate, and daily self-spend limit are now visible rather than unverified. |
| `m-add`: borderless API-key field | **fixed** | `m-add` contains a genuine full-width rounded outlined password input. Computed input: **358×48px**, **1px solid rgba(255,255,255,.1)** border, **12px** radius, **15px** text. Fictional key is masked; only one 48px connection primary is present. |
| `m-agents`: selected drawing source not demonstrated | **fixed** | `m-agents` shows the selected ChatGPT red radio, border, and tinted card; sources remain separate rounded cards. Paid-source price/estimate/daily-limit fields are also visible. |
| `m-artist`: selected model explicitly marked unable to read images | **fixed** | Desktop and phone artist pickers select `gpt-6.1-sol` with real fixture `text,vision` capabilities. Only the three vision-capable selected-catalog models are enabled. The text-only Ollama row is disabled with **Không đọc được ảnh**; image-generation-only rows are disabled with **Chỉ vẽ ảnh, không viết chữ**. Keeping these disabled rows visible is what the artboard/spec requires; “only image-capable models” means **only image-reading models are selectable**, not removing the explained disabled rows. |

### Additional live checks closing QA2 evidence gaps

- **Huỷ/Lưu behavior for model drafts:** after a three-model draft, Huỷ removed the strip and returned to one saved checked row. A subsequent second-row selection followed by Lưu sent the intercepted provider update, removed the strip, and left saved selection `[anthropic/claude-opus-4.6, openai/gpt-5.2]`. This validates UI draft handling, not a paid-provider backend call.
- **Phone catalog summary:** the paid Gemini catalog initially has three checked rows and `Đã chọn 3 model`. Unticking one produces **two checked rows** and `Đã chọn 2 model · giá tính trên 1 triệu token hoặc mỗi ảnh`; see `app-m-models-two.png`. The footer is in frame.
- **Current-row treatment:** both desktop pickers show the current enabled model with the right-edge selected mark; phone pickers show the selected red radio. Artist title is now **Model viết mô tả ảnh** on both sizes; capability pills are restored; both phone sheets use the matching near-black surface.
- **Failed provider:** real rendered 401 warning, orange boundary, correct header Thử kết nối / Đổi khoá, and the three recovery choices are present. The former extra Model header action and boxed Thêm theo tên treatment are gone.
- **Grok:** signed, code, and signed-out states were each captured. Pending status is now warm yellow. The code view has code, copy control, xAI action, cancel, waiting indicator, and expiry; no external authorization action was clicked.
- **Buttons:** desktop provider actions and draft-strip actions measure **32px / r8 / 13px**, matching their artboard `ghost btn` class, not an assumed universal 36px. Phone provider action buttons measure **44px / r12 / 13.5px**; the phone add primary is **48px**. No button-size major remains.

## 2. Remaining differences

Each bullet is counted **once**; shared items are not multiplied by state. Differences below exclude the accepted deviations and corrected fallback-font artifacts.

- [major] **conn-added** · **unsaved-change region**: artboard shows the information/Huỷ/Lưu strip immediately after connecting a provider, even while zero models are selected; app shows no strip until a model checkbox is changed. Reproduced from the actual add UI, not by navigating directly to a fixture-only screenshot. Evidence: `conn-added` pair; compare supplementary `app-conn-added-dirty.png` for the later, now-working state.
- [minor] **desktop overflowing states, especially conn-models / conn-manual** · **top chrome**: artboard header remains **55px** high; app header shrinks to **48px** when this content overflows, moving the heading/sidebar/help baseline upward by about 7px. Evidence: matching `conn-manual` JSON `HEADER` heights and PNGs. This is independent of catalog inventory length and not the artboard phone inset.
- [minor] **conn-added / conn-models / conn-manual** · **desktop catalog checkboxes**: artboard unchecked boxes have white faces and selected pink boxes have white ticks; app unchecked faces are dark/outlined and selected ticks are dark. Evidence: `conn-models` / `conn-manual` pairs. The phone catalog now correctly uses the artboard pink boxes and dark ticks, so this is a desktop-only component mismatch.
- [minor] **agents-artist** · **page framing on opening the picker**: artboard retains Trợ lý AI and its introduction above the agent card; app opening the artist picker scrolls the page enough to remove the heading and move the introduction immediately below the fixed top bar. The popover itself is correctly anchored and has the right title/current row. Evidence: `agents-artist` pair.
- [minor] **m-add** · **sheet vertical spacing**: artboard title starts at **y=295** and primary at **y=758**; settled app title starts around **y=312** and primary at **y=770**, with the same eight tiles and 48px input/primary. App sheet is shorter and starts about 18px lower. Evidence: `m-add` pair and accompanying JSON. This is bottom-sheet spacing, not the simulated 44px status bar.
- [minor] **m-director / m-artist** · **footer icon**: artboard footer is plain **Thêm model ở Kết nối AI** text; app prepends a key icon and shifts the text start right. Evidence: both phone picker pairs. Counted once for the shared footer implementation, not twice.
- [minor] **agents / m-agents** · **paid-source daily-limit copy**: desktop artboard uses **máy tự tiêu tối đa 50 ảnh/ngày** and phone artboard **tự tiêu tối đa 50/ngày**; app uses the longer **máy tự tiêu tối đa 50 ảnh mỗi ngày** wording. On phone this wraps onto an extra line in the paid source card. The actual limit, price, and estimate values are exempt; the repeated non-data wording/formatting is the difference. Evidence: `agents` and `m-agents` pairs. Counted once.

No additional scored difference in `conn-list`, `conn-add`, `conn-failed`, `grok-signed`, `grok-code`, `grok-out`, `agents-director`, `m-connect`, or `m-models` beyond the shared bullets above. The desktop add form can be scrolled to its key/primary/helper region (`app-conn-add-bottom.png`); the former screenshot crop is not retained as a product failure.

### Accepted-deviation / weak-excuse audit

- #18–#26 remain excluded as instructed. In this area, #19 excludes Giới hạn từng kênh; #26 excludes unavailable Grok plan, monthly spend, editorial model notes, fallback reset-time differences, empty-key primary disabling, and proposed phone reasoning placement.
- **No data-source excuse applies to the missing initial Huỷ/Lưu strip.** All required copy, icon, controls, and local newly-added state already exist. A strip only after selecting a checkbox does not match the zero-selected `conn=added` artboard.
- “There are no paid sources to compare” is no longer a valid coverage excuse: both desktop and phone were supplied a wire-shaped paid Gemini source and its pricing/daily-limit presentation was inspected.
- The #26 reasoning-placement rationale remains weak because the desktop-only editing boundary was already identified in QA2. It is nevertheless accepted and **not counted**; no extra reasoning control was visible in the captured phone director sheet.
- Correct font availability matters: do not carry forward the old help-card wrapping or phone Kiểm định-row findings from fallback-font reference screenshots. With the requested fonts, those regions now substantially match.

## 3. Coverage and states not checked

**No requested primary state was skipped: 17/17 matched state pairs captured and inspected.**

| Scope | States / screenshot names | Coverage |
|---|---|---|
| Desktop Connect | `conn-list`, `conn-add`, `conn-added`, `conn-models`, `conn-manual`, `conn-failed` | 6/6; initial added and subsequently dirty added kept distinct; successful manual addition checked |
| Desktop Grok | `signed` in `conn-list`; `grok-code`; `grok-out` | 3/3 values; signed shares the list pair, so two additional pairs |
| Desktop Agents | `agents` (`ai=none`), `agents-director`, `agents-artist` | 3/3; current model, disabled capabilities, selected drawing source, and paid source inspected |
| Phone Connect | `m-connect` (`sheet=none`), `m-add`, `m-models` | 3/3; key field, 44/48px controls, catalog footer, and a live checkbox/summary change inspected |
| Phone Agents | `m-agents` (`sheet=none`), `m-director`, `m-artist` | 3/3; selected source, paid source, enabled/disabled models, capabilities, footer, and surface inspected |

**Intentionally not exercised:** real provider authorization, paid catalog retrieval, paid generation, real-key connection, and an external login completion. These are safety exclusions, not missing visual-state coverage. Intercepted success/error/state payloads validate current UI rendering and interactions, not the corresponding external provider services. The request allowed fixture data for exactly these states.

Cleanup: closed the owned Chrome, stopped the isolated QA services, dropped `viderse_smoke_qa3connect`, and removed the temporary binary/home/profile/kernel/cookies; retained only `shots/` as evidence. No product code, dist, or other reports were changed.

## 4. Counts

| Round-2 current majors | Count |
|---|---:|
| Fixed | 7 |
| Still wrong | 1 |
| Not verifiable | 0 |
| **Total** | **8** |

| Remaining difference scope | Majors | Minors |
|---|---:|---:|
| Initial `conn-added` strip | 1 | 0 |
| Shared overflowing desktop chrome | 0 | 1 |
| Shared desktop catalog checkboxes | 0 | 1 |
| Desktop artist opening/framing | 0 | 1 |
| Phone add-sheet spacing | 0 | 1 |
| Shared phone picker footer | 0 | 1 |
| Shared paid-source limit wording | 0 | 1 |
| **Total distinct bullets** | **1** | **6** |

| Primary visual coverage | Count |
|---|---:|
| Requested state pairs inspected | 17 |
| Requested state pairs skipped | 0 |

Status: DONE
Summary: **1 major remaining / 6 minors remaining**. Worst 3: **(1)** initial newly-added provider still has no Huỷ/Lưu strip; **(2)** desktop artist opening scrolls the page heading out of view; **(3)** overflowing desktop content shrinks the top chrome and moves the page baseline. Artist vision eligibility, model-draft Huỷ/Lưu, selected source treatment, successful manual-add evidence, and the phone key border are now verified fixed.
