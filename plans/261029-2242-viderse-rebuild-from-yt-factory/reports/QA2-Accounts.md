# QA2 — Accounts · independent visual review

## Scope, evidence, and verdict

Read **all 49 supplied PNGs** directly with the image-displaying read tool in `/tmp/viderse-fidelity/FidelityAccounts/shots/`: **13 desktop pairs, 11 phone pairs, and one unmatched desktop YouTube-off app image**. Desktop PNGs are 1800×1125 for the stated 1440×900 viewport; phone PNGs are 488×1055 for the stated 390×844 viewport (approximately 1.25×, not 2×). Compared each pair region by region, including exact non-live copy, wrapping/enclosure, component/icon choices, tones, alignment, density, typography, and visible phone button heights.

Read `QA-Accounts.md`, `Fidelity-Accounts.md`, W13 in `01-ia-flows.md`, PlatformCard/QrLoginDialog and Banner in `04-components.md`, `03-tokens.md`, `05-kit.md`, `06-copy.md`, the artboard README, and accepted pending deviations #18–#26 in `11-hoi-dap.md`. Also inspected the current shared Banner and account presentation source to assess the explicitly requested post-capture banner cutover risk. No application changes, tests, builds, interaction checks, or fresh screenshots were made. No browser was opened: all defined Accounts reference states have supplied pairs; the missing dedicated states below have no corresponding artboard variant.

**The supplied screenshots do not substantiate the implementer report.** For example, `app-desktop-blocked.png` still contains the reportedly removed QR banner action, `app-mobile-qr-scanned.png` still contains the reportedly removed same-device card, and both `app-*-qr-error.png` still display rejection rather than connection-failure UI. The source currently contains changes for several of these fixes; therefore these are findings about the delivered visual evidence, **not a claim that every screenshot defect still exists in the current source/build**. The images may be stale or from a different build [INFERENCE]. A code assertion or reported computed measurement cannot override the pixels for this visual sign-off.

Counts below exclude live account names, cookie masks, QR bit patterns, countdown values, last-run times, runtime activity, account-profile choices, and unrelated machine configuration. They also exclude the artboard-only simulated phone status bar/home indicator and their approximately 48 CSS-pixel effect on page position. Do not add artificial browser page padding to reproduce native device chrome.

## 1. Every round-1 major

Evidence names below mean the named `board-…png` / `app-…png` pair in the shots directory. Status is the result **visible in the supplied pair**; post-fix build appearance is not established by these captures.

| # | Round-1 major | Status | Evidence and assessment |
|---|---|---|---|
| R1 | Desktop Settings sidebar: divider and Giới hạn từng kênh | still wrong | `desktop-saved`, also all unobscured desktop pairs: the right-hand sidebar divider remains absent. Omission of Giới hạn từng kênh is now an accepted pending deviation (#19), **not a defect**. The residual divider is minor; this compound round-1 finding no longer contributes a major. |
| R2 | Desktop blocked banner has an extra QR action | still wrong | `desktop-blocked`: app still shows Quét mã QR inside the banner, unlike the reference, and pushes the explanation onto two lines. |
| R3 | Desktop expired-login sidebar attention dot | still wrong | `desktop-expired`: reference has an orange dot on Tài khoản nền tảng; app does not. Other rows having dots does not resolve it. |
| R4 | Desktop YouTube-bot card issue outline | still wrong | `desktop-ytbot`: reference outlines the whole YouTube card orange; app card outline is neutral. |
| R5 | Desktop YouTube-bot sidebar attention dot | still wrong | `desktop-ytbot`: app still has no attention dot on Tài khoản nền tảng. |
| R6 | Desktop YouTube banner/login region boundary | still wrong | `desktop-ytbot`: app divider remains above the issue banner, not between that banner and Lấy cookie từ trình duyệt. |
| R7 | Desktop QR backdrop lacks blur | still wrong | Every `desktop-qr-*` pair, especially `desktop-qr-waiting`: app page/navigation remain sharply readable; reference is heavily blurred. |
| R8 | Desktop rejected QR resets completed steps | still wrong | `desktop-qr-rejected`: reference preserves completed steps 1–2; app displays unfinished numbered circles 1–2. |
| R9 | Desktop connection error uses rejection identity/copy | still wrong | `desktop-qr-error`: app warning banner says Bilibili chưa chấp nhận đăng nhập and gives phone-cancellation/account-check advice, not the connection-failure recovery text. |
| R10 | Desktop connection error uses rejection QR plate | still wrong | `desktop-qr-error`: app shows the orange cross and Chưa đăng nhập được, not the grey globe and Chưa có mã. |
| R11 | Phone QR instructions are over-indented | still wrong | Every `mobile-qr-*` pair: reference circles start at approximately 16 CSS px; app circles at approximately 43 CSS px. |
| R12 | Phone terminal-state primary buttons are too short | still wrong | `mobile-qr-expired/rejected/error/success`: app buttons are approximately 36 CSS px high versus reference 48; below the 44px touch-target contract. |
| R13 | Phone same-device action target is too short | still wrong | `mobile-qr-waiting`: app Mở app remains approximately 30 CSS px high versus reference 44. |
| R14 | Phone scanned sheet retains same-device card | still wrong | `mobile-qr-scanned`: app still shows Bilibili trên chính máy này? / Mở app; reference removes the entire card, yielding a much shorter sheet. |
| R15 | Phone rejected QR resets completed steps | still wrong | `mobile-qr-rejected`: app steps 1–2 are numbered/incomplete rather than completed green ticks with muted instruction text. |
| R16 | Phone connection error uses rejection identity/copy | still wrong | `mobile-qr-error`: app gives rejection/cancellation advice instead of Chưa kết nối được Bilibili and checking the Viderse machine network. |
| R17 | Phone connection error uses rejection QR plate | still wrong | `mobile-qr-error`: app orange cross / Chưa đăng nhập được replaces the reference grey globe / Chưa có mã. |
| R18 | Phone success explanation is missing | still wrong | `mobile-qr-success`: App tự gia hạn đăng nhập mỗi ngày. is absent below the account confirmation. |

**Round-1 reconciliation:** 0 fixed, 18 still wrong in the supplied visual evidence, 0 not verifiable for the originally pictured scenarios. Of these 18 original compound findings, **17 remain major**, and R1 is reduced to a minor residual because #19 explicitly accepts the missing route row. No original major is silently omitted from this table.

## 2. New or remaining visible differences

Each bullet is one counted difference. Shared bullets apply to every relevant pair, not once per occurrence. State headings identify the exact filename suffixes; e.g. `desktop-blocked` means `board-desktop-blocked.png` against `app-desktop-blocked.png`. Counts describe delivered screenshots, not independently rendered current-source behavior.

### Shared visual baseline — 0 major / 3 minor

- [minor] desktop + phone, all states · typography: artboard has narrower letterforms and shorter headings/labels; app is visibly wider/heavier, changing introduction/aside line breaks and making the phone Twitch description wrap to two lines. This is one shared font/rendering difference, not a separate defect for every wrap. No fresh font measurement was performed in QA2; round 1 already disproved the claim that the reference was using an installed Be Vietnam Pro.
- [minor] blocked/expired/YouTube-bot and QR rejection/error, both viewports · issue palette: artboard surfaces are lighter, grey-orange and less saturated; supplied app surfaces are darker brown/yellow with more amber edges. The later kit Banner cutover is not pictured; see §3 rather than declaring its palette fixed from an old image.
- [minor] QR waiting and code-bearing terminal/scanned states, both viewports · QR artwork inset: artboard matrix extends close to the plate padding; app matrix occupies a smaller central square with a visibly larger white inset (clear in both waiting pairs). QR **content/density** is live and excluded, but its displayed footprint differs. This is an observation about framing, not a claim that the QR cannot be scanned or an instruction to remove a required quiet zone.

### Desktop shell — 0 major / 5 minor

Evidence: all unobscured desktop pairs, clearest in `desktop-saved`.

- [minor] desktop, all base states · top navigation: artboard encloses all five items in one rounded dark rail with no filled Bàn dựng selection on Settings and uses the review glyph for Duyệt; app has separate uncontained items, a filled/bold Bàn dựng selection, and a flag glyph for Duyệt. Runtime badge counts are excluded.
- [minor] desktop, all base states · vertical alignment: artboard header is approximately 55 CSS px high; app header is shorter and the account heading, first card and safety aside sit approximately 6 CSS px higher.
- [minor] desktop, all base states · sidebar boundary: artboard separates the settings rail from content with a fine vertical right divider; app does not. The separately missing Giới hạn từng kênh row is accepted under #19 and is not counted here.
- [minor] desktop, all base states · platform initials: artboard YT/TW/BL use compact mono letterforms; supplied app initials use proportional-looking body letterforms, especially TW. Current source declaring a mono family does not establish that this capture rendered it.
- [minor] desktop, all YouTube profile selectors · selector affordance/inset: artboard has a small bright native chevron close to the right edge and more inset value text; app has a dim outlined chevron farther inward and value text closer to the left edge. Most evident in `desktop-saved` and `desktop-ytbot`.

### Desktop Bilibili — 2 major / 8 minor

`desktop-saved` has only inherited differences. Other pairs:

- [major] desktop-blocked · issue-banner action/enclosure: artboard has just the clock, title and one-line explanation; app inserts Quét mã QR on the right, constricts the explanation to two lines and enlarges the banner/card.
- [minor] desktop-blocked · outer card border: artboard keeps the blocked Bilibili card neutral with only the inner banner orange; app outlines the entire card orange. General PlatformCard prose conflicts with this specific board; this is the visible board difference, not a mandate to ignore that conflict.
- [minor] desktop-blocked · explanation copy: artboard says Bị chặn 4 lần liên tiếp **từ 00:10**. App tự quét lại lúc 02:10…; app omits the entire từ <time> clause. #25 accepts sourcing that time from lastError.at, not omitting the clause.
- [minor] desktop-manual · QR action glyph: artboard uses the simple scan-corner glyph beside Đăng nhập bằng mã QR; app uses the more detailed rounded/face-like glyph.
- [minor] desktop-manual · collapsed disclosure: artboard shows a right-pointing chevron for Dán cookie khác; app shows a downward chevron although the form is collapsed.
- [major] desktop-expired · sidebar attention: artboard puts an orange dot on Tài khoản nền tảng; app omits it despite the expired-login banner.
- [minor] desktop-expired · collapsed disclosure: artboard shows a right-pointing chevron for Dán cookie thủ công; app shows a downward chevron while collapsed.
- [minor] desktop-empty · primary QR action size: artboard shows the compact 30 CSS-pixel button; app shows a taller approximately 36px button and wider action region.
- [minor] desktop-empty · primary QR action glyph: artboard simple scan-corner glyph is replaced by the rounded/face-like glyph in app.
- [minor] desktop-empty · collapsed disclosure: artboard right-pointing Dán cookie thủ công chevron is downward in app while collapsed.

### Desktop YouTube bot — 3 major / 0 minor

Evidence: `desktop-ytbot`.

- [major] desktop-ytbot · card perimeter: artboard outlines the whole YouTube card orange; app keeps its outer outline neutral despite the Cần cookie tag and issue banner.
- [major] desktop-ytbot · sidebar attention: artboard has an orange dot on Tài khoản nền tảng; app omits it.
- [major] desktop-ytbot · banner/login boundary: artboard places the issue banner immediately after the platform header, then a divider above the browser-cookie controls; app puts the divider above the banner and none between banner and controls, changing which region wraps the issue.

The visible body phrase phụ đề của video bị ảnh hưởng also differs from the reference phụ đề của 1 video and from the implementer report proposal một số video. Because #25 expressly permits non-numeric wording when the count is unavailable, this is recorded as a non-counted accepted-copy variation, not resurrected as a defect.

### Desktop QR, shared — 1 major / 4 minor

Evidence: all seven `desktop-qr-*` pairs.

- [major] desktop-qr-all · modal backdrop: artboard heavily blurs and darkens the account page; app only darkens it and leaves all background controls/navigation sharp and readable.
- [minor] desktop-qr-all · title typography: artboard uses a 16px/600 dialog title; supplied app title is visibly larger (round-1 18px treatment), independently of the shared font difference.
- [minor] desktop-qr-all · header icon: artboard simple scan-corner glyph is replaced by the rounded/face-like scan glyph in app.
- [minor] desktop-qr-all · close affordance: artboard close-circle is smaller and nearer the header right edge; app circle is larger and shifted inward/left.
- [minor] desktop-qr-all · security footnote alignment: artboard shield aligns with the first line of the two-line note; app centers it vertically between the lines.

### Desktop QR, individual states — 3 major / 10 minor

- [minor] desktop-qr-loading · status tone: artboard Đang tạo mã… is muted grey; app status words are much brighter near-white. Both shots say Đang tạo mã…, not Đang tạo mã QR….
- [minor] desktop-qr-waiting · status tone: artboard Đang chờ quét words match the blue spinner; app words are near-white while the spinner remains blue.
- [minor] desktop-qr-scanned · QR treatment: artboard fades a sharp matrix; app fades **and blurs** it.
- [minor] desktop-qr-scanned · status tone: artboard confirmation sentence is amber with its dot; app only colors the dot and leaves words near-white.
- [minor] desktop-qr-scanned · completion badge type: artboard has small green ticks inside translucent green circular fills; app uses larger bare tick-circle outlines without those fills.
- [minor] desktop-qr-expired · status tone: artboard Mã đã hết hạn sau 3 phút. is muted grey; app makes it near-white.
- [major] desktop-qr-rejected · step completion: artboard retains green completed steps 1–2 with muted labels; app resets them to numbered unfinished steps.
- [major] desktop-qr-error · error identity and recovery copy: artboard globe/banner says Chưa kết nối được Bilibili and Máy chạy Viderse chưa gọi được Bilibili. Kiểm tra mạng của máy đó rồi tạo mã mới.; app warning/rejection banner says Bilibili chưa chấp nhận đăng nhập and discusses Huỷ on the phone or additional account checks.
- [major] desktop-qr-error · QR plate: artboard shows a grey globe ring and Chưa có mã on a blank white plate; app shows the orange rejection cross and Chưa đăng nhập được.
- [minor] desktop-qr-error · skeleton persistence: artboard failed-creation plate has no loading skeleton; app leaves a pale rounded skeleton behind the rejection overlay.
- [minor] desktop-qr-success · QR treatment: artboard fades a sharp matrix; app blurs it as well.
- [minor] desktop-qr-success · status tone: artboard Đã đăng nhập: <account> is green; app only colors the dot and keeps words near-white.
- [minor] desktop-qr-success · completion badge type: artboard has three filled translucent green circular tick badges; app has larger unfilled tick-circle outlines.

Desktop success **does contain** App tự gia hạn đăng nhập mỗi ngày khi quét kênh. in the supplied app image, matching the desktop reference sentence; no missing desktop-success explanation is reported.

### Phone account shell and Bilibili — 0 major / 6 minor

Evidence: five `mobile-{saved,blocked,manual,expired,empty}` pairs. Saved/manual/empty have only inherited differences; live YouTube profile choice in empty is excluded.

- [minor] mobile-all-base · back/title horizontal alignment: artboard heading begins approximately x=53 CSS px; app begins approximately x=40, shrinking the gap from the back arrow. The vertical displacement caused by simulated device chrome is excluded.
- [minor] mobile-blocked · Bilibili perimeter: artboard outer card border is neutral; app outlines the whole card orange. The general prose/board conflict is the same as desktop blocked.
- [minor] mobile-blocked · banner title copy: artboard Bilibili đang giới hạn yêu cầu becomes Bilibili đang giới hạn yêu cầu **từ mạng này** in the supplied app image.
- [minor] mobile-blocked · banner icon alignment: artboard clock aligns with the title/first line; app centers it against the whole multi-line title/body block.
- [minor] mobile-expired · explanation copy: artboard says trong lúc chờ, app quét kênh không đăng nhập…; app inserts **vẫn** before quét. Desktop intentionally has vẫn; the phone reference does not.
- [minor] mobile-expired · banner icon alignment: artboard warning icon aligns with the title; app centers it against the full multi-line block.

### Phone QR, shared — 2 major / 2 minor

Evidence: all six `mobile-qr-*` pairs, conditional on the control being present.

- [major] mobile-qr-all · instructions alignment: artboard starts the list circles at the same 16px left inset as status/banner; app indents the list approximately another 27 CSS px and shifts all labels right.
- [major] mobile-qr-expired/rejected/error/success · primary touch target: artboard full-width Tạo mã mới / Xong is 48 CSS px tall; supplied app buttons are approximately 36px, below the documented 44px phone target.
- [minor] mobile-qr-all · sheet heading: artboard uses an 18px heading; supplied app heading is smaller, consistent with the old 16px treatment.
- [minor] mobile-qr-all · body rhythm/density: artboard keeps 12px gaps around the status/banner and list; app leaves visibly larger gaps before status/banner and before instructions, moving the lower content/actions down. Sheet position itself is not separately counted when this or an extra card explains it.

**Correction to round 1:** do not count its purported 174px phone countdown-track discrepancy. The MSettings markup explicitly sets the full track to **196px**, the same width as its 196px plate. The shorter blue **fill** in `board-mobile-qr-waiting.png` is 89% remaining time, not a narrower track. The corresponding scanned reference uses 73%. Comparing these to app countdowns near 3:00 mistakes live data for layout. The implementer report assertion that the phone artboard track is 200px is also wrong (200px is the desktop width), but this does not create a screenshot defect where the visible track matches.

### Phone QR, individual states — 6 major / 15 minor

- [minor] mobile-qr-waiting · status tone: artboard Đang chờ quét is light blue; app words are near-white while its spinner stays blue.
- [major] mobile-qr-waiting · same-device action target: artboard Mở app is approximately 44px high; app uses an approximately 30px compact button, below the phone target contract.
- [minor] mobile-qr-waiting · same-device icon: artboard phone glyph is larger (20px); app phone glyph is visibly smaller (old 16px treatment).
- [major] mobile-qr-scanned · same-device card visibility: artboard removes the entire same-device card after scanning; app retains it, lengthening the scanned sheet nearly to the waiting composition.
- [minor] mobile-qr-scanned · confirmation copy/tone: artboard says Đã quét. Bấm xác nhận trên điện thoại. in amber; app appends **để xong.** and renders words near-white.
- [minor] mobile-qr-scanned · QR treatment: artboard fades without blur; app blurs the faded matrix.
- [minor] mobile-qr-scanned · completion badge type: artboard completed steps 1–2 use translucent filled green circles; app uses larger bare green tick-circle outlines.
- [minor] mobile-qr-expired · status tone: artboard expiry sentence is muted grey; app makes it near-white beside its grey dot.
- [minor] mobile-qr-expired · retry action icon: artboard Tạo mã mới is text-only; app adds a refresh glyph.
- [major] mobile-qr-rejected · step completion: artboard preserves completed green steps 1–2 and muted labels; app resets them to numbered unfinished steps.
- [minor] mobile-qr-rejected · banner icon alignment: artboard warning triangle aligns with the title line; app centers it against the entire multi-line block.
- [minor] mobile-qr-rejected · retry action icon: artboard Tạo mã mới is text-only; app adds a refresh glyph.
- [major] mobile-qr-error · error identity and recovery copy: artboard globe/banner says Chưa kết nối được Bilibili and instructs checking the Viderse machine network; app shows the rejection/warning banner and phone-cancellation/account-check instructions.
- [major] mobile-qr-error · QR plate: artboard grey globe and Chưa có mã on white become app orange cross and Chưa đăng nhập được.
- [minor] mobile-qr-error · skeleton persistence: artboard shows a blank failed-creation white plate; app leaves a pale rounded skeleton under the overlay.
- [minor] mobile-qr-error · banner icon alignment: artboard icon is first-line/top aligned; app warning triangle is centered against the full title/body block.
- [minor] mobile-qr-error · retry action icon: artboard Tạo mã mới is text-only; app adds a refresh glyph.
- [major] mobile-qr-success · follow-on explanation: artboard includes App tự gia hạn đăng nhập mỗi ngày. below the account confirmation; app omits this line entirely.
- [minor] mobile-qr-success · status tone: artboard confirmation words are green; app colors only the dot green and keeps the words near-white.
- [minor] mobile-qr-success · QR treatment: artboard faded matrix remains sharp; app blurs it.
- [minor] mobile-qr-success · completion badge type: artboard uses three filled translucent green tick badges; app uses three larger unfilled tick-circle outlines.

## 3. Post-capture kit Banner cutover: risk, not screenshot approval

The user states that the custom issue banner was replaced **after capture** by kit `Banner tone=error`, with claimed background `rgba(255,154,92,.07)`, border alpha `.22`, icon `#FF9A5C`, padding 10px/12px, radius 12px, title 13px/500, text 12px/fg-3. Those headline metrics agree with the intended issue-banner recipe. They cannot prove that the pictured darker/amber banners are the new rendering, and none of the 49 captures verifies the cutover.

Source evidence: `platform/ui/src/Banner.tsx:45–71` uses a flex-column body **without a gap**, and sets the text font size/color but **no line-height**. The former `.sacc-ibanner` had the user-specified **1px title/body gap** and **1.5 text line-height**. Current app root/body declarations in `apps/viderse/web/src/styles/base.css:14–21` and `styles/tokens.css:30–38` set **1.55**; the mobile overlay has its own inherited typography. Therefore:

1. **Known structural difference from the former custom banner:** the 1px title/body gap is not preserved in the shared Banner. A body row begins immediately after the title line box. This is not automatically a board defect: the generic artboard `.vv-banner__body` also has no gap. Do not recreate the custom gap everywhere merely to preserve an implementation detail.
2. **Unverified density risk:** where the body inherits 1.55, 12px copy has an 18.6px line box rather than the former 18px. With the removed 1px gap, relative block-height change is approximately `0.6 × body-line-count − 1` CSS px if the other line boxes are unchanged. The effect depends on the containing dialog/sheet and wrapping. Multi-line phone rejection/error banners explicitly use 1.5 in the MSettings artboard, so relying on inheritance instead of retaining that local contract can change banner/sheet rhythm. This is a source-backed risk, **not an observed new screenshot defect**; it is excluded from visual totals.
3. **The new boundary props are actually consumed:** Banner dispatches `tone=error` to the error palette, reads the custom `icon`, applies `align=top` to root alignment plus a 2px icon margin, forwards `role`, and conditionally renders `action`. Current Bilibili calls use clock/top on phone, current QR banners use their chosen error glyph/top, and YouTube passes its action. No silent-drop/no-op integration issue is evident at this boundary. The old centred-clock/triangle findings remain visible in the supplied captures, but the source-level top-alignment support means they cannot be extrapolated blindly to the post-cutover build.

Visual sign-off of the latest implementation remains unavailable until screenshots actually covering that implementation exist, especially phone blocked/expired/rejected/error banners with multi-line copy. This report does not replace that missing evidence with a fabricated computed/rendered measurement.

## 4. Accepted pending deviations and weak explanations

- **#19 — missing Giới hạn từng kênh:** accepted and excluded from defect counts. Avoiding a dead route is a credible temporary reason. It does **not** explain removing the sidebar divider, choosing Bàn dựng as selected on Settings, or claiming pixel-level fidelity. It is still incomplete relative to the ultimate design, not a fixed row.
- **#25 — blocked-since timestamp and affected-video count:** accepted and excluded as data-model differences. Using lastError.at as the transition-to-paused time is reasonable only while that remains the error that actually triggered the pause; this is a semantic-risk note, not an assertion of a demonstrated runtime bug. The absent từ <time> clause in the supplied blocked image is not excused by accepting a timestamp source. Replacing the unsupported exact affected-video count with non-numeric wording is reasonable and is not counted as a copy defect.
- **#18, #20–#24, #26:** read and respected; they concern other Settings areas, not an excuse for the visible Accounts discrepancies. No defect count is charged for them.
- **Font excuse:** the implementer report still attributes the Twitch wrap to installed-font versus webfont metrics, although round 1 documented that the reference actually used system fallback. A declared font-family string is not proof of the font supplying glyphs. Treat this as a weak/unresolved reference-environment explanation, not a reason to shrink app type or call these captures identical.
- **Scope excuse:** calling shell differences outside the Accounts implementer remit may assign ownership, but does not make them visually fixed. Shell residuals remain in this area report; the explicitly accepted row omission is the only exemption among them.
- **Evidence quality:** Fidelity-Accounts.md says 25 pairs, but the directory contains 24 pairs plus one unpaired image. Its checked-source/metrics claims and Khớp labels conflict with the images itemized above. This is a sign-off evidence gap, not another manufactured visual defect.

## 5. States missing a pair

| Viewport/state | Missing image(s) | Disposition |
|---|---|---|
| Desktop yt=off | `board-desktop-ytoff.png` | `app-desktop-ytoff.png` was read. Reference Settings declares yt=ok/bot, not off. App-only Tắt selector / grey-dot Đang tắt · app tải video không đăng nhập recorded, not declared matched. |
| Phone yt=bot | `board-mobile-ytbot.png`, `app-mobile-ytbot.png` | MSettings has no yt tweak and a static YouTube card. Desktop evidence does not verify phone bot behavior. |
| Phone yt=off | `board-mobile-ytoff.png`, `app-mobile-ytoff.png` | No corresponding MSettings variant. A Chưa chọn value in another fixture is not dedicated off-state coverage. |
| Phone qr=loading | `board-mobile-qr-loading.png`, `app-mobile-qr-loading.png` | Not a declared MSettings QR state. Transient loading appearance is unevidenced, not a fabricated mismatch. |
| Desktop/phone qr=none | No separately named none pair | **Covered**, not missing baseline: all five base Bilibili pairs per viewport show no modal/sheet. |
| QR success after blocked login, both viewports | No dedicated blocked→success pair | Reference has a special cleared-wait explanation; supplied success pair covers ordinary saved-account success only. This cross-state combination is not verified and is not added to the four dedicated-state coverage-gap count. |
| Post-capture kit Banner rendering | No captures of the acknowledged post-capture change | See §3; no pixel sign-off possible for the changed component from these PNGs. Not a separate missing state or counted visual defect. |

No claim is made about content outside the screenshots or touch hit areas larger than their visible controls. The undersized phone controls are judged against their visible rendered button height and the documented kit target, not an invented hit-test measurement.

## 6. Count tables

Distinct **listed visual differences**; shared differences count once, inherited state occurrences do not multiply them. Accepted deviations, reference corrections, post-capture risks, and coverage gaps are separate from defect totals.

| Section | Major remaining | Minor remaining | Total |
|---|---:|---:|---:|
| Shared visual baseline | 0 | 3 | 3 |
| Desktop shell | 0 | 5 | 5 |
| Desktop Bilibili states | 2 | 8 | 10 |
| Desktop YouTube bot | 3 | 0 | 3 |
| Desktop QR shared | 1 | 4 | 5 |
| Desktop QR individual states | 3 | 10 | 13 |
| Phone shell/Bilibili | 0 | 6 | 6 |
| Phone QR shared | 2 | 2 | 4 |
| Phone QR individual states | 6 | 15 | 21 |
| **Total** | **17** | **53** | **70** |

| Round-1 major reconciliation | Count |
|---|---:|
| Originally listed majors reviewed | 18 |
| Fixed in supplied visual evidence | 0 |
| Still wrong in supplied visual evidence | 18 |
| Not verifiable for original pictured scenario | 0 |
| Still major after accepted-deviation handling | 17 |
| Reduced to minor residual / accepted missing-row portion | 1 |

| Evidence coverage | Count |
|---|---:|
| Images individually read | 49 |
| Complete desktop pairs | 13 |
| Complete phone pairs | 11 |
| Total complete pairs | 24 |
| Unpaired app image read | 1 |
| Dedicated viewport/state gaps, excluding covered none baselines | 4 |

Compared with round 1: remove its compound sidebar major from major totals (accepted missing row; retain divider as a minor), exclude the accepted non-numeric YouTube-copy variation, correct the false track-width finding, and add the visible QR-footprint difference. This reconciliation explains why the new total is 70 rather than simply copying the old 71.

Status: DONE
Summary: **17 majors remaining, 53 minors remaining in the supplied visual evidence.** Worst 3: **(1)** connection failures still masquerade as user-rejected logins on desktop and phone, **(2)** phone QR sheets still have undersized primary/same-device controls, over-indented steps and an extra scanned-state card, **(3)** desktop QR backdrops still lack blur and desktop issue states still omit required attention/outline/boundary cues. No round-1 major is visibly fixed by these captures; the sidebar row omission is accepted, not repaired. Current source and the acknowledged post-capture Banner replacement require new visual evidence before a latest-build fidelity claim can be accepted.
