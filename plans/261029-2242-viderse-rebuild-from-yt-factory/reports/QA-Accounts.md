# Independent visual QA — Accounts

## Scope and evidence

Independent inspection of **every one of the 49 supplied PNGs** in `/tmp/viderse-fidelity/FidelityAccounts/shots/`: 24 complete board/app pairs plus the unmatched desktop YouTube-off app shot. Desktop images are 1800×1125 pixels for a 1440×900 viewport; mobile images are 488×1055 for 390×844 (approximately DPR 1.25), **not mobile @2x as the implementer report claims**. Coordinates/sizes below are CSS pixels unless specified.

Compared structure, wrapping, alignment, component/icon choices, copy, color and density against `01-ia-flows.md` W13, `04-components.md` PlatformCard/QrLoginDialog, `03-tokens.md`, `05-kit.md`, `06-copy.md`, `11-hoi-dap.md` and artboard README/state markup. Live names, cookie masks, QR matrices, countdowns, machine activity, counts and unrelated account configuration are not defects. Baseline findings are listed once and inherited by state sections; no state is declared “Khớp”. This is a visual QA report, not a patch provenance/code-review verdict.

## Independent font verification and weak justifications

- Both real pages declare `getComputedStyle(...).fontFamily` as `"Be Vietnam Pro", ui-sans-serif, system-ui, sans-serif`. That declaration does **not** prove which font supplied the glyphs.
- Artboard `MSettings?...screen=accounts&bili=saved`: `document.fonts` is empty. Chrome DevTools Protocol `CSS.getPlatformFontsForNode` on the heading returns **familyName `.SF NS`, postScriptName `.SFNS-Bold`, isCustomFont=false**. The artboard is using macOS system fallback, **not an installed Be Vietnam Pro**.
- App: the original screenshot service at `127.0.0.1:5193` now refuses connections. The still-running Viderse server at `127.0.0.1:18789` is authentication-gated, so fresh account DOM measurements were not possible without changing authentication. On that real app page, computed body font is the same declared stack, loaded fonts include Be Vietnam Pro 400/600 Latin, Latin-ext and Vietnamese WOFF2 assets, and CDP on its heading returns **Be Vietnam Pro SemiBold / BeVietnamPro-SemiBold / isCustomFont=true**. Account CSS inherits this same stack; this corroborates the webfont side without pretending the authenticated account page was freshly inspected.
- Independent canvas measurement of `Sẵn sàng` at 11.5px with the declared full stack: **artboard 49.96997px; app 51.26692px; delta 1.29695px**. The implementer’s approximately +1.3px advance is reproducible; their explanation of the artboard font source is not. This is a reference-font/environment mismatch, not justification to reduce the app font size. Load the specified font in the reference environment and regenerate the comparison before signing off wrapping as accepted.
- Only the available `agent-browser`/CDP interface could be used; this runtime exposes no JS-eval-kernel `browser.open` tool. Attempting the requested exact Chrome args with the domain allowlist was rejected by the browser tool because `--user-data-dir` can restore pages before containment. Font inspection therefore used isolated session `qa-accounts`, the requested Chrome executable/headless/no-first-run/no-default-browser-check options, the identical credential-disabling init script, and **allowed domains 127.0.0.1/localhost**, omitting only the incompatible user-data-dir. No external-site action was clicked. The session and its tabs were closed.
- The area report’s proposed static error copy is reasonable and is not flagged for hiding raw server errors. However, it does **not** excuse choosing the *rejected-login* copy for the supplied 502 creation failure: `QrLogin.tsx` maps HTTP 502 to `rejected`, exactly matching the incorrect error screenshots. The report’s claim that these shots use network-error copy is contradicted by the images.
- `bili=empty` retaining Sẵn sàng is excluded, as requested by the area proposal and consistent with the artboard. The proposal about the blocked-banner QR action opening the normal dialog only justifies its behavior, not adding a control absent from the blocked artboard.
- Binding answers #1/#2/#7 are respected in this review; #6 says retain same-device opening while phone validation is pending. That does not override the artboard’s explicit hiding of this card after `scanned`.

## Shared visual baseline

Applies wherever the same typography or issue banner is visible; counted once, not once per screenshot.

- [minor] Typography: artboard shows narrower system-font letterforms, shorter headings and different line breaks; app shows Be Vietnam Pro webfont letterforms throughout. This includes the Twitch description wrapping to two lines on mobile, different desktop introduction/aside wrapping, and wider button labels. The measured advance difference is real, but the claimed installed Be Vietnam Pro on the artboard is false; see font investigation below.
- [minor] Issue-banner palette: artboard shows a lighter, desaturated grey-orange issue surface; app shows a darker, browner/yellower surface with a more saturated amber edge, visible in blocked/expired/Bot banners and rejected/error QR banners.

## Desktop — shared accounts shell

Applies to all unobscured desktop account states, including qr=none. Under QR overlays, only the parts still visible are relevant.

- [minor] Top navigation: artboard shows the five navigation items enclosed together in a rounded dark rail with no filled Bàn dựng selection on Settings; app shows separate uncontained items, a filled/bold Bàn dựng item, and a flag icon for Duyệt instead of the artboard review icon. Badge counts and runtime machine status are excluded as live data.
- [minor] Header/content vertical alignment: artboard shows a 55 CSS-pixel header with the account heading and first card below it; app shows a shorter header and the heading, first card and right aside about 6 CSS pixels higher.
- [major] Settings sidebar: artboard shows a right-hand divider and both Luồng chạy · hàng chờ and Giới hạn từng kênh under Ở trang khác; app omits the divider and the entire Giới hạn từng kênh row. Differences in storage, error counts, versions and unrelated warning dots are excluded as live data.
- [minor] Platform initials: artboard shows YT/TW/BL in the mono typeface (JetBrains Mono/fallback monospace); app renders these letters in the proportional body face, especially evident in TW.
- [minor] YouTube profile selector: artboard shows a small bright native select chevron close to the right edge and more inset value text; app shows a dim outline chevron farther inset and value text closer to the left edge.

## Desktop — bili=saved

Pair: board-desktop-saved.png ↔ app-desktop-saved.png. Inherits shared baseline and desktop shell differences.


## Desktop — bili=blocked

Pair: board-desktop-blocked.png ↔ app-desktop-blocked.png. Inherits shared differences.

- [minor] Bilibili card border: artboard shows the ordinary neutral card border with an orange banner inside; app makes the entire card outline orange. Note: the general PlatformCard prose requests an issue outline, so this is a board/prose conflict, not an instruction to blindly remove the app outline.
- [major] Blocked banner action/layout: artboard shows only the clock, title and one-line explanatory text, with no QR action in this banner; app inserts a Quét mã QR button on the right, constricts the text to two lines and increases the banner/card height.
- [minor] Blocked banner copy: artboard shows Bị chặn 4 lần liên tiếp từ 00:10. App tự quét lại lúc 02:10…; app shows Bị chặn 4 lần liên tiếp. App tự quét lại lúc 02:10… and drops the entire từ <time> clause, not merely a different live timestamp.

## Desktop — bili=manual

Pair: board-desktop-manual.png ↔ app-desktop-manual.png. Inherits shared differences.

- [minor] QR action icon: artboard shows the simple four-corner scan glyph beside Đăng nhập bằng mã QR; app shows the more detailed rounded/face-like scan glyph.
- [minor] Collapsed cookie disclosure: artboard shows a right-pointing chevron beside Dán cookie khác; app shows a downward-pointing chevron even though the form is collapsed.

## Desktop — bili=expired

Pair: board-desktop-expired.png ↔ app-desktop-expired.png. Inherits shared differences.

- [major] Sidebar attention indicator: artboard shows an orange dot on Tài khoản nền tảng when Bilibili login has expired; app has no dot on that row, despite the expired card/banner.
- [minor] Collapsed cookie disclosure: artboard shows a right-pointing chevron beside Dán cookie thủ công; app shows a downward-pointing chevron while the form is collapsed.

## Desktop — bili=empty

Pair: board-desktop-empty.png ↔ app-desktop-empty.png. Inherits shared differences. YouTube being off in this app fixture is independent live account data and is not treated as a Bilibili fidelity defect.

- [minor] Primary QR action size: artboard shows the compact 30 CSS-pixel QR button; app shows a taller 36 CSS-pixel button and a wider action region.
- [minor] Primary QR action icon: artboard shows the simple corner scan glyph; app shows the detailed rounded/face-like scan glyph.
- [minor] Collapsed cookie disclosure: artboard shows a right-pointing chevron beside Dán cookie thủ công; app shows a downward-pointing chevron while the form is collapsed.

## Desktop — yt=bot

Pair: board-desktop-ytbot.png ↔ app-desktop-ytbot.png. Inherits shared differences.

- [major] YouTube card outline: artboard shows an orange outline around the whole YouTube card; app shows a neutral outline even while the Cần cookie tag and orange error banner are visible.
- [major] Sidebar attention indicator: artboard shows an orange dot on Tài khoản nền tảng for the YouTube cookie problem; app omits it.
- [major] Banner/login region boundary: artboard shows the problem banner directly below the platform header, then a divider above Lấy cookie từ trình duyệt; app places the divider above the banner and none between the banner and cookie controls, changing which region encloses the error.
- [minor] Bot banner explanatory copy: artboard shows Trinh sát chưa tải được phụ đề của 1 video. Chọn hồ sơ trình duyệt có đăng nhập Google phụ rồi thử lại.; app substitutes phụ đề của video bị ảnh hưởng. This is different sentence construction, not just a different live count.

## Desktop — yt=off

Only app-desktop-ytoff.png exists. No matching board shot and no yt=off artboard variant are defined (Settings declares yt=ok|bot). The app-only selector Tắt and grey-dot Đang tắt · app tải video không đăng nhập are recorded, not declared a match. Shared desktop differences remain visible.


## Desktop — qr=none

No separately named qr-none pair. The five bili pairs already show the no-dialog state, so qr=none has baseline visual coverage. Inherits each corresponding bili state and shared desktop differences.


## Desktop — QR dialog shared

Applies to loading, waiting, scanned, expired, rejected, error and success pairs. Timer values, QR matrix contents and account names are excluded as live data/fixtures.

- [major] Modal backdrop: artboard shows a heavily blurred and darkened background, making account controls unreadable; app only darkens the background and leaves the entire account page and navigation sharply readable.
- [minor] Dialog title size: artboard shows the 16px/600 dialog-title token; app explicitly uses 18px/600, making the title appreciably larger/bolder even apart from the font-family difference.
- [minor] Header scan icon: artboard shows the simple corner scan icon in the accent tile; app shows a different rounded/face-like scan glyph.
- [minor] Close affordance: artboard shows a smaller close-circle near the upper right of the header; app shows a larger close-circle shifted inward/left.
- [minor] Security footnote alignment: artboard aligns the shield icon with the first line of the two-line footnote; app vertically centers the icon between the two lines.

## Desktop — qr=loading

Pair: board-desktop-qr-loading.png ↔ app-desktop-qr-loading.png. Inherits QR-shared differences. Contrary to the report summary, the actual label in both screenshots is Đang tạo mã…, not Đang tạo mã QR….

- [minor] Loading status tone: artboard shows muted grey Đang tạo mã…; app shows much brighter near-white status text.

## Desktop — qr=waiting

Pair: board-desktop-qr-waiting.png ↔ app-desktop-qr-waiting.png. Inherits QR-shared differences.

- [minor] Waiting status tone: artboard shows blue Đang chờ quét matching the blue spinner; app keeps the spinner blue but renders the words near-white.

## Desktop — qr=scanned

Pair: board-desktop-qr-scanned.png ↔ app-desktop-qr-scanned.png. Inherits QR-shared differences.

- [minor] Scanned QR treatment: artboard fades the QR while retaining crisp edges; app both fades and blurs it.
- [minor] Scanned status tone: artboard shows the entire confirmation instruction in amber with its amber dot; app has an amber dot but near-white instruction text.
- [minor] Completed-step badges: artboard shows the green completion ticks inside translucent green circular badges; app removes the badge fill and displays larger bare green tick-circle outlines.

## Desktop — qr=expired

Pair: board-desktop-qr-expired.png ↔ app-desktop-qr-expired.png. Inherits QR-shared differences.

- [minor] Expired status tone: artboard shows muted grey Mã đã hết hạn sau 3 phút.; app renders the sentence near-white.

## Desktop — qr=rejected

Pair: board-desktop-qr-rejected.png ↔ app-desktop-qr-rejected.png. Inherits QR-shared differences.

- [major] Step completion: artboard preserves green ticks and muted text for steps 1 and 2 after rejection; app resets both to numbered, unfinished steps 1 and 2.

## Desktop — qr=error

Pair: board-desktop-qr-error.png ↔ app-desktop-qr-error.png. Inherits QR-shared differences. This is the report fixture POST 502 network/upstream failure, not a user rejection.

- [major] Error identity and recovery copy: artboard shows a globe icon, Chưa kết nối được Bilibili, and Máy chạy Viderse chưa gọi được Bilibili. Kiểm tra mạng của máy đó rồi tạo mã mới.; app instead shows a warning triangle and the rejected-login title/body about cancelling on the phone or additional account verification.
- [major] Error QR plate: artboard shows a grey globe ring and Chưa có mã on a blank white plate; app shows an orange rejection cross and Chưa đăng nhập được.
- [minor] Error skeleton: artboard shows no skeleton inside the white plate once creation failed; app leaves the pale rounded loading-skeleton region behind the rejection overlay.

## Desktop — qr=success

Pair: board-desktop-qr-success.png ↔ app-desktop-qr-success.png. Inherits QR-shared differences.

- [minor] Success QR treatment: artboard fades a still-sharp QR matrix; app blurs the faded matrix.
- [minor] Success status tone: artboard shows Đã đăng nhập: <account> in green; app shows a green dot but near-white status words.
- [minor] Completed-step badges: artboard shows three small green ticks on translucent green circular fills; app shows three larger unfilled tick-circle outlines.

## Mobile — shared accounts shell

Applies to the five bili pairs and the visible background of QR sheets. Native-device chrome is excluded from defect totals: the artboard draws 9:41/battery/signal and the home indicator; a browser screenshot does not. Their absence accounts for much of the roughly 48 CSS-pixel upward page shift and must not be mistaken for an app-content padding bug.

- [minor] Back/header horizontal alignment: artboard shows the heading farther to the right of the back arrow (about x=53 CSS px); app starts it around x=40, leaving a smaller arrow-to-title gap.

## Mobile — bili=saved

Pair: board-mobile-saved.png ↔ app-mobile-saved.png. Inherits shared baseline/mobile-shell differences; the app Twitch card is one text line taller because of the verified font mismatch.


## Mobile — bili=blocked

Pair: board-mobile-blocked.png ↔ app-mobile-blocked.png. Inherits shared differences.

- [minor] Bilibili card outline: artboard shows a neutral outer border; app outlines the whole blocked card orange. As on desktop, the generic component prose conflicts with this particular board.
- [minor] Blocked banner title: artboard shows Bilibili đang giới hạn yêu cầu; app appends từ mạng này.
- [minor] Banner icon alignment: artboard top-aligns the clock with the title; app vertically centers the clock alongside the whole title/body block.

## Mobile — bili=manual

Pair: board-mobile-manual.png ↔ app-mobile-manual.png. Inherits shared baseline/mobile-shell differences.


## Mobile — bili=expired

Pair: board-mobile-expired.png ↔ app-mobile-expired.png. Inherits shared differences.

- [minor] Expired explanatory copy: artboard says trong lúc chờ, app quét kênh không đăng nhập…; app inserts vẫn before quét.
- [minor] Banner icon alignment: artboard aligns the warning icon beside the title; app vertically centers it beside the multi-line explanatory block.

## Mobile — bili=empty

Pair: board-mobile-empty.png ↔ app-mobile-empty.png. Inherits shared baseline/mobile-shell differences. Chrome · Profile 1 versus Chưa chọn is independent YouTube account fixture data and is excluded.


## Mobile — yt=bot

No board-mobile-ytbot.png or app-mobile-ytbot.png. MSettings does not declare or render a yt tweak; its YouTube card is static. Therefore the implementer has not supplied visual evidence for mobile bot handling, and desktop bot results cannot establish mobile fidelity.


## Mobile — yt=off

No board-mobile-ytoff.png or app-mobile-ytoff.png. MSettings has no off variant. The Chưa chọn row in the empty/terminal-QR app fixtures is not a dedicated yt=off pair.


## Mobile — qr=none

No separately named qr-none pair; the five bili pairs cover the no-sheet baseline. Inherits shared/mobile-shell differences.


## Mobile — QR sheet shared

Applies to waiting, scanned, expired, rejected, error and success pairs, with conditional applicability stated below. QR contents and countdown numbers are excluded.

- [major] Instruction-list alignment: artboard places numbered/ticked steps at the same 16 CSS-pixel left inset as the status banner; app indents the entire list by roughly another 27 CSS pixels, pushing all three step labels substantially right.
- [minor] Sheet heading: artboard shows an 18px sheet title; app sets the title to 16px, making it smaller than the sheet-title token.
- [minor] Body vertical spacing: artboard keeps the code, status/banner and instructions in a tighter stack; app adds visibly larger gaps before the status/banner and before the instructions, moving the lower steps/actions down.
- [major] Terminal-state primary button: artboard shows 48 CSS-pixel tall, full-width Tạo mã mới or Xong buttons in expired/rejected/error/success; app shows approximately 36 CSS-pixel tall buttons, below the documented 44px mobile touch target.
- [minor] Countdown track width: artboard makes the track under waiting/scanned narrower than the 196px QR plate (about 174px); app makes the track the full 196px width.

## Mobile — qr=loading

No board-mobile-qr-loading.png or app-mobile-qr-loading.png. MSettings declares none|waiting|scanned|expired|rejected|error|success, not loading. A transient mobile skeleton may be intentional but has not been visually evidenced or artboard-approved.


## Mobile — qr=waiting

Pair: board-mobile-qr-waiting.png ↔ app-mobile-qr-waiting.png. Inherits shared/mobile-QR differences.

- [minor] Waiting status tone: artboard renders Đang chờ quét in light blue; app uses near-white text while retaining a blue spinner.
- [major] Same-device action target: artboard shows a large roughly 44px-tall Mở app button; app uses the compact 30px small-button variant.
- [minor] Same-device icon: artboard shows a larger phone glyph beside the same-device explanation; app uses a smaller 16px glyph.

## Mobile — qr=scanned

Pair: board-mobile-qr-scanned.png ↔ app-mobile-qr-scanned.png. Inherits shared/mobile-QR differences.

- [major] Same-device card visibility: artboard removes the entire Bilibili trên chính máy này? / Mở app card after scanning; app still shows it, making the scanned sheet almost as tall as waiting instead of the shorter scanned composition.
- [minor] Confirmation status copy/tone: artboard says Đã quét. Bấm xác nhận trên điện thoại. in amber; app adds để xong. and renders the sentence near-white.
- [minor] Scanned QR treatment: artboard fades but does not blur the QR; app blurs it.
- [minor] Completed-step badges: artboard places the first two ticks on filled translucent green circles; app shows larger bare green outlines.

## Mobile — qr=expired

Pair: board-mobile-qr-expired.png ↔ app-mobile-qr-expired.png. Inherits shared/mobile-QR differences.

- [minor] Expired status tone: artboard shows muted grey expiry text; app shows near-white expiry text beside the grey dot.
- [minor] Retry action icon: artboard shows text-only Tạo mã mới; app adds a refresh icon.

## Mobile — qr=rejected

Pair: board-mobile-qr-rejected.png ↔ app-mobile-qr-rejected.png. Inherits shared/mobile-QR differences.

- [major] Step completion: artboard preserves completed green steps 1 and 2 after rejection; app resets them to unfinished numbered steps.
- [minor] Banner icon alignment: artboard places the warning triangle beside the first/title line; app centers it against the full multi-line banner.
- [minor] Retry action icon: artboard shows text-only Tạo mã mới; app adds a refresh icon.

## Mobile — qr=error

Pair: board-mobile-qr-error.png ↔ app-mobile-qr-error.png. Inherits shared/mobile-QR differences.

- [major] Error identity and recovery copy: artboard shows the network/globe banner Chưa kết nối được Bilibili with instructions to check the Viderse machine network; app shows the rejection/warning banner Bilibili chưa chấp nhận đăng nhập with phone-cancellation/account-verification instructions.
- [major] Error QR plate: artboard shows a grey globe and Chưa có mã on a blank white plate; app shows an orange rejection cross and Chưa đăng nhập được.
- [minor] Error skeleton: artboard has no loading-skeleton inset after failure; app retains a pale rounded skeleton under the overlay.
- [minor] Banner icon alignment: artboard top-aligns its globe icon beside the title; app centers its warning icon against the full text block.
- [minor] Retry action icon: artboard shows text-only Tạo mã mới; app adds a refresh icon.

## Mobile — qr=success

Pair: board-mobile-qr-success.png ↔ app-mobile-qr-success.png. Inherits shared/mobile-QR differences.

- [major] Success follow-on explanation: artboard shows App tự gia hạn đăng nhập mỗi ngày. below the account confirmation (or the explicit cleared-wait explanation when blocked); app omits that second line entirely.
- [minor] Success status tone: artboard renders Đã đăng nhập: <account> in green; app only colors the dot green and keeps the words near-white.
- [minor] Success QR treatment: artboard fades a sharp QR matrix; app blurs the faded matrix.
- [minor] Completed-step badges: artboard uses filled translucent green completion circles; app shows larger bare tick-circle outlines.

## States missing a shot / limits of evidence

| Requested state | Missing evidence | Disposition |
|---|---|---|
| Desktop `yt=off` | `board-desktop-ytoff.png` | App-only image inspected; no dedicated off artboard variant exists. |
| Mobile `yt=bot` | Both board and app shots | MSettings has no yt control; mobile bot appearance remains unverified. |
| Mobile `yt=off` | Both board and app shots | MSettings has no yt control; do not substitute empty Bilibili fixture for this state. |
| Mobile `qr=loading` | Both board and app shots | Not a declared MSettings variant; transient app state lacks evidence. |
| Desktop/mobile `qr=none` | No separately named none shots | Already visually covered by all five base Bilibili pairs on each viewport; not an uncovered baseline. |

The provided Bilibili and QR shots are one-state-at-a-time fixtures, not all cross-products. In particular, QR success after a blocked account has no dedicated pair, so the missing mobile success explanation is visible/proven for the supplied success pair, while its blocked-specific wording is established by the artboard contract rather than a fresh interaction. No layout claim is made for content below a cropped screenshot. No build, lint, unit test or application mutation was performed.

## Count table

Counts are **distinct listed visual differences**, with shared findings counted once. They are not counts of root causes or per-screen multiplied occurrences. A zero in a state row means it inherits shared differences, not a fidelity approval. Missing-shot states are coverage gaps, not manufactured visual defects.

| Section/state | Major | Minor | Total |
|---|---:|---:|---:|
| Shared visual baseline | 0 | 2 | 2 |
| Desktop — shared accounts shell | 1 | 4 | 5 |
| Desktop — bili=saved | 0 | 0 | 0 |
| Desktop — bili=blocked | 1 | 2 | 3 |
| Desktop — bili=manual | 0 | 2 | 2 |
| Desktop — bili=expired | 1 | 1 | 2 |
| Desktop — bili=empty | 0 | 3 | 3 |
| Desktop — yt=bot | 3 | 1 | 4 |
| Desktop — yt=off | 0 | 0 | 0 |
| Desktop — qr=none | 0 | 0 | 0 |
| Desktop — QR dialog shared | 1 | 4 | 5 |
| Desktop — qr=loading | 0 | 1 | 1 |
| Desktop — qr=waiting | 0 | 1 | 1 |
| Desktop — qr=scanned | 0 | 3 | 3 |
| Desktop — qr=expired | 0 | 1 | 1 |
| Desktop — qr=rejected | 1 | 0 | 1 |
| Desktop — qr=error | 2 | 1 | 3 |
| Desktop — qr=success | 0 | 3 | 3 |
| Mobile — shared accounts shell | 0 | 1 | 1 |
| Mobile — bili=saved | 0 | 0 | 0 |
| Mobile — bili=blocked | 0 | 3 | 3 |
| Mobile — bili=manual | 0 | 0 | 0 |
| Mobile — bili=expired | 0 | 2 | 2 |
| Mobile — bili=empty | 0 | 0 | 0 |
| Mobile — yt=bot | 0 | 0 | 0 |
| Mobile — yt=off | 0 | 0 | 0 |
| Mobile — qr=none | 0 | 0 | 0 |
| Mobile — QR sheet shared | 2 | 3 | 5 |
| Mobile — qr=loading | 0 | 0 | 0 |
| Mobile — qr=waiting | 1 | 2 | 3 |
| Mobile — qr=scanned | 1 | 3 | 4 |
| Mobile — qr=expired | 0 | 2 | 2 |
| Mobile — qr=rejected | 1 | 2 | 3 |
| Mobile — qr=error | 2 | 3 | 5 |
| Mobile — qr=success | 1 | 3 | 4 |
| **Total distinct findings** | **18** | **53** | **71** |

| Screenshot coverage | Count |
|---|---:|
| Supplied images read | 49 |
| Complete desktop pairs | 13 |
| Complete mobile pairs | 11 |
| Unmatched app images | 1 |
| Requested variant/viewport rows lacking a dedicated usable pair (excluding none baselines) | 4 |

Status: DONE
Summary: 71 distinct visual differences (18 major, 53 minor), 24 complete pairs inspected, and 4 dedicated-pair coverage gaps documented. The three worst problems are (1) network QR creation failures masquerade as user-rejected logins on both viewports; (2) mobile QR sheets have heavily indented instructions, undersized primary/same-device buttons, and an extra same-device card after scanning; (3) desktop QR overlays omit the reference blur, while the desktop problem states also lose their sidebar attention dots/YouTube issue outline. The claimed installed-artboard-font explanation is disproved even though the +1.3px width difference is real.
