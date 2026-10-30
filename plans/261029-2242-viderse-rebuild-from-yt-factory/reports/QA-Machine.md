# QA — Machine

Independent screenshot fidelity review. **The supplied evidence does not support “Khớp”.**

## Evidence and method

- App evidence: all 15 supplied PNGs in `/tmp/viderse-fidelity/FidelityMachine/shots/` were opened individually with the image reader. No app code was modified or executed as a test.
- Reference evidence: rendered every requested state from `http://127.0.0.1:18700/artboards/` at **1440×900 desktop / 390×844 phone**; saved and individually inspected `qa-art-<state>.png` beside the supplied shots. The state-name pairing is exact, not inferred from the implementer’s table.
- The supplied app PNGs are mixed device-pixel ratios: several desktop shots are 1800×1125 and phone shots 488×1055. Comparisons below use CSS-size proportions, not raw PNG pixel dimensions.
- Browser capture used an isolated local-only agent-browser session with Google Chrome, `--headless=new`, `--no-first-run`, `--no-default-browser-check`, a 127.0.0.1/localhost allowlist, and the requested WebAuthn-disabling init script. This toolset does not expose the assignment’s JS-kernel `browser.open`/`tab.run` APIs; the CLI screenshot facility was the available equivalent. No external link was clicked. All QA browser tabs/session were closed after capture.
- The local-only allowlist prevents remote Google-font requests. Font-family/antialiasing differences are therefore **not** scored. Layout, component shape, copy, explicit agent identity colors, and element presence are scored.
- Read the implementer report and the binding/reference sections of `01-ia-flows.md`, `03-tokens.md`, `04-components.md`, `05-kit.md`, `06-copy.md`, `11-hoi-dap.md`, and `artboards/README.md`. Hỏi-đáp #15 is currently **Mở**, not an answered approval; its description of slight gradient variance does not explain exchanging purple/gold/blue agent identities.
- Severity: **major** = wrong hierarchy, missing main action/content, misleading current destination, or unusable evidence; **minor** = localized appearance/copy/spacing/affordance mismatch. Repeated frame defects are listed in every affected state as requested; totals count **state-level occurrences**, not distinct root causes.

## Exclusions and justification audit

Live names, versions, byte/size/time values, dataset-dependent statuses/counts, and corresponding empty states are not counted. Implementer proposals #1 (lane count/queue helper), #2 (per-channel limit link), #3 (system Chrome), #4 (required-space sizes), #5 (version strings), and #7 (release date) are disclosed deviations and are not re-filed as fidelity findings. This is not a claim that their proposals were design-approved. In particular, absence of backend fields is an implementation-scope explanation, not proof that a visual match exists. Proposal #6 is only partially defensible: no keptBytes field can explain a missing number, but the existing error field can provide the reason; that weak portion is explicitly flagged under setup-failed.

Frame sections intentionally review top bar, sidebar, footer and right rail, not the Connect/Accounts/Storage body owned by other QA areas. No non-data footer discrepancy was found; the long development version string explains the app footer’s additional wrapping.

## frame-tools

App: `frame-tools.png` · Artboard: `Settings.dc.html?section=tools` · Reference shot: `qa-art-frame-tools.png`.

Frame-only comparison. The real app has failed/update/login-needed tools, while the default artboard has ready tools; those live statuses are not counted. The separate Advanced card is counted under each tools-* state, not again here.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.

## frame-connect

App: `frame-connect.png` · Artboard: `Settings.dc.html?section=connect` · Reference shot: `qa-art-frame-connect.png`.

Frame/right-rail comparison only. Provider cards belong to the Connect QA area; signed-in versus signed-out provider data is not counted here. The right rail wording matches after ignoring the shared vertical offset listed below.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.

## frame-agents

App: `frame-agents.png` · Artboard: `Settings.dc.html?section=agents` · Reference shot: `qa-art-frame-agents.png`.

The supplied frame-agents.png exists but contains no rendered UI. No region-by-region fidelity conclusion is possible for this state.

- [major] Entire captured state: artboard shows the complete Settings header, left navigation, sidebar footer, Trợ lý AI content, and right-side help rail ; app shows a uniform dark blank image with none of those regions. This is an unusable capture, not evidence that this state matches, and does not by itself establish that the current app always renders blank.

## frame-accounts

App: `frame-accounts.png` · Artboard: `Settings.dc.html?section=accounts` · Reference shot: `qa-art-frame-accounts.png`.

Frame/right-rail comparison only. Platform account states and account-panel contents are covered by Accounts QA, not counted as Machine differences. The account-safety and help text match apart from the shared frame placement.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.

## frame-storage

App: `frame-storage.png` · Artboard: `Settings.dc.html?section=storage` · Reference shot: `qa-art-frame-storage.png`.

Frame/right-rail comparison only. Storage values, empty datasets, paths, and cleanup panels belong to Storage QA. The help-rail wording matches apart from the shared frame placement.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.

## frame-logs

App: `frame-logs.png` · Artboard: `Settings.dc.html?section=logs` · Reference shot: `qa-art-frame-logs.png`.

The app has zero recent errors, so its empty-state row and the shorter errors card are live-data differences, not defects. There is no populated-error app capture to compare the four-row log list or its action buttons. The missing release date was explicitly disclosed in implementer proposal #7 and is not counted; the real version string is also excluded.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.
- [minor] Export-card heading: artboard shows “Xuất log để báo lỗi” ; app shows “Gói nhật ký để báo lỗi”.
- [minor] Export-card scope copy: artboard shows “Gom nhật ký của app và 5 video gần nhất vào một tệp .zip, khoảng 3 MB. Cookie, tài khoản và đường dẫn cá nhân đã được che.” ; app shows “Một tệp .zip, khoảng 3 MB. Cookie, tài khoản và đường dẫn cá nhân đã được che.”, omitting the app-log and five-video scope and making the card one line shorter.

## tools-ok

App: `tools-ok.png` · Artboard: `Settings.dc.html?section=tools&tools=ok` · Reference shot: `qa-art-tools-ok.png`.

Ready-state tool statuses are present. Dynamic sizes, check times, Chrome provision, membership tier, and disclosed version additions are excluded; see exclusions below.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.
- [major] Advanced-region containment: artboard shows Nâng cao · thư mục dữ liệu và phiên bản as the final row INSIDE the six-tool list card ; app shows it in a separate rounded card with a visible gap below the list.
- [minor] Advanced disclosure icon: artboard shows a right-pointing chevron for the collapsed row ; app shows a downward chevron even while the content is collapsed.
- [minor] Tool icon tiles: artboard shows 34px square icon tiles and text beginning around x=345 ; app shows slightly larger tiles and text beginning around x=347, making the icon column heavier and shifting the row text.

## tools-update

App: `tools-update.png` · Artboard: `Settings.dc.html?section=tools&tools=update` · Reference shot: `qa-art-tools-update.png`.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.
- [major] Advanced-region containment: artboard shows Nâng cao · thư mục dữ liệu và phiên bản as the final row INSIDE the six-tool list card ; app shows it in a separate rounded card with a visible gap below the list.
- [minor] Advanced disclosure icon: artboard shows a right-pointing chevron for the collapsed row ; app shows a downward chevron even while the content is collapsed.
- [minor] Tool icon tiles: artboard shows 34px square icon tiles and text beginning around x=345 ; app shows slightly larger tiles and text beginning around x=347, making the icon column heavier and shifting the row text.
- [minor] Updated-tool icon tone: artboard shows the Tải video và phụ đề download icon in the neutral light icon color ; app shows that icon in yellow in addition to the yellow Có bản mới status pill.

## tools-failed

App: `tools-failed.png` · Artboard: `Settings.dc.html?section=tools&tools=failed` · Reference shot: `qa-art-tools-failed.png`.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.
- [major] Advanced-region containment: artboard shows Nâng cao · thư mục dữ liệu và phiên bản as the final row INSIDE the six-tool list card ; app shows it in a separate rounded card with a visible gap below the list.
- [minor] Advanced disclosure icon: artboard shows a right-pointing chevron for the collapsed row ; app shows a downward chevron even while the content is collapsed.
- [minor] Tool icon tiles: artboard shows 34px square icon tiles and text beginning around x=345 ; app shows slightly larger tiles and text beginning around x=347, making the icon column heavier and shifting the row text.
- [major] Failure-summary guidance: artboard shows “Video đang làm vẫn giữ nguyên; video mới chờ ở bước Phụ đề.” ; app shows “Video đang làm vẫn giữ nguyên; video mới chờ ở bước bị lỗi.”, removing the named affected step even in the transcription-failure fixture.
- [minor] Failed transcription-row explanation: artboard shows “Mạng bị ngắt khi tải mô hình nhận giọng nói ở 62%. Video mới sẽ dừng ở bước Phụ đề tới khi sửa xong.” ; app shows “Mất kết nối khi tải mô hình. Đã giữ 806 MB, thử lại sẽ tải tiếp.” The recovery explanation is different and the explicit consequence for new videos is missing. The byte and percentage values themselves are not counted.

## tools-installing

App: `tools-installing.png` · Artboard: `Settings.dc.html?section=tools&tools=installing` · Reference shot: `qa-art-tools-installing.png`.

The real fixture has Chrome and drawing tools already installed rather than waiting, so their done labels versus Chờ are not counted. Sidebar 4/6 versus headline 3/6 is recorded as a state-definition/coverage caveat, not scored as a pure numeric mismatch: the screenshot uses different completion/active-position measures. ETA and byte values are excluded.

- [major] Top navigation: artboard shows Settings selected via the gear and no selected main destination ; app shows a filled, bold Bàn dựng tab while the Settings gear is also selected.
- [minor] Top navigation container: artboard shows all five links inside one dark rounded, outlined group ; app shows five free-standing links without the enclosing group.
- [minor] Top navigation alignment: artboard shows the navigation group centered on the viewport ; app shows the group noticeably left of center (its midpoint is approximately x=670 rather than x=720 at 1440 CSS pixels).
- [minor] Duyệt icon: artboard shows the gallery/image icon ; app shows a flag icon.
- [minor] Left-sidebar boundary: artboard shows a faint full-height vertical divider at the right edge of the sidebar ; app shows no corresponding divider.
- [minor] Sidebar group spacing: artboard shows more breathing room before Dữ liệu and Ở trang khác ; app shows compressed gaps, putting Ở trang khác approximately 18 CSS pixels higher.
- [minor] Frame vertical alignment: artboard shows the header bottom around y=54 and the right help card at y=81 ; app shows the header bottom around y=48 and the help card at y=75, moving the page and right rail upward. This records the literal artboard difference; 04-components also says 48px for AppHeader, so that document/artboard conflict should be resolved rather than treating the screenshots as identical.
- [major] Advanced-region containment: artboard shows Nâng cao · thư mục dữ liệu và phiên bản as the final row INSIDE the six-tool list card ; app shows it in a separate rounded card with a visible gap below the list.
- [minor] Advanced disclosure icon: artboard shows a right-pointing chevron for the collapsed row ; app shows a downward chevron even while the content is collapsed.
- [minor] Tool icon tiles: artboard shows 34px square icon tiles and text beginning around x=345 ; app shows slightly larger tiles and text beginning around x=347, making the icon column heavier and shifting the row text.
- [minor] Running-row descriptive tone: artboard shows “Chép lời thoại thành chữ ngay trên máy” in ordinary muted gray, reserving the progress treatment for the bar/phase below ; app shows this description in light blue as well.
- [minor] ChatGPT row action: artboard shows the Cần bạn pill without a neighboring action in this installing state ; app shows an additional outlined “Đăng nhập” button next to that pill and wraps the explanation onto a second line.

## setup-running

App: `setup-running.png` · Artboard: `Onboard.dc.html?step=setup&setup=running` · Reference shot: `qa-art-setup-running.png`.

The missing action is specifically the artboard’s outlined “Đăng nhập ngay” button, not a primary login CTA. ETA, bytes, and total download size are excluded.

- [major] Hero arrangement: artboard shows the orb on the LEFT of the heading and explanatory paragraph, both left-aligned ; app shows the orb ABOVE the heading with the entire hero centered.
- [major] Card hierarchy: artboard shows a standalone progress card followed by a separate six-step list card with a gap between them ; app shows the progress summary and all six steps enclosed in one card.
- [major] ChatGPT action: artboard shows a usable login button in the ChatGPT row ; app shows only a small spinner at the far right, with no visible login button or action label.
- [major] Below-list guidance: artboard shows the complete information-icon footnote, including where to add other providers and what happens after installation, within the 900px viewport ; app shows that guidance pushed to/below the bottom edge and clips it in the supplied shot.
- [minor] Main-column width: artboard shows 720px-wide cards spanning approximately x=360–1080 ; app shows a reduced usable card width of approximately 680px, x=380–1060, because its 720px main container includes horizontal padding.
- [minor] Agent-chip component: artboard shows compact rounded-rectangle chips (roughly 26px high, 7px corners) aligned to the left edge of the cards ; app shows taller, fully rounded pills centered above the card.
- [minor] Agent-chip palette: artboard shows a purple Đạo diễn dot and a warm gold Hoạ sĩ dot ; app shows a gold Đạo diễn dot and a blue/gray Hoạ sĩ dot. These are different agent identities, not the slight orb-gradient variance described in hỏi-đáp #15.
- [minor] Step-row agent dots: artboard shows gold dots beside Công cụ vẽ ảnh and Đăng nhập ChatGPT ; app shows blue/gray dots beside both rows.
- [minor] Step separators and insets: artboard shows separators extending across the list card and approximately 67px normal rows ; app shows inset separators inside the shared card and additional vertical spacing between rows, making the list visibly taller.
- [minor] Chrome-provided explanation: artboard shows “Dùng Chrome có sẵn trên máy, không cần tải thêm” ; app shows “Dùng Chrome chạy ngầm để xuất video và đo âm thanh” even though the side label says “Có sẵn”. The explanatory copy no longer explains that this step needs no download.
- [minor] Overall-progress palette: artboard shows a red-to-pink overall progress fill ; app shows a red-to-purple fill.
- [minor] Active transcription-step treatment: artboard shows a subtly blue-tinted full row with blue phase text and percentage ; app shows the row on the same background as its neighbors and the phase/percentage in muted gray.

## setup-login

App: `setup-login.png` · Artboard: `Onboard.dc.html?step=setup&setup=login` · Reference shot: `qa-art-setup-login.png`.

The missing action is specifically the artboard’s bright primary “Đăng nhập ChatGPT” button. The app snapshot instead remains a spinner even though the headline says “Còn một bước cần bạn”.

- [major] Hero arrangement: artboard shows the orb on the LEFT of the heading and explanatory paragraph, both left-aligned ; app shows the orb ABOVE the heading with the entire hero centered.
- [major] Card hierarchy: artboard shows a standalone progress card followed by a separate six-step list card with a gap between them ; app shows the progress summary and all six steps enclosed in one card.
- [major] ChatGPT action: artboard shows a usable login button in the ChatGPT row ; app shows only a small spinner at the far right, with no visible login button or action label.
- [major] Below-list guidance: artboard shows the complete information-icon footnote, including where to add other providers and what happens after installation, within the 900px viewport ; app shows that guidance pushed to/below the bottom edge and clips it in the supplied shot.
- [minor] Main-column width: artboard shows 720px-wide cards spanning approximately x=360–1080 ; app shows a reduced usable card width of approximately 680px, x=380–1060, because its 720px main container includes horizontal padding.
- [minor] Agent-chip component: artboard shows compact rounded-rectangle chips (roughly 26px high, 7px corners) aligned to the left edge of the cards ; app shows taller, fully rounded pills centered above the card.
- [minor] Agent-chip palette: artboard shows a purple Đạo diễn dot and a warm gold Hoạ sĩ dot ; app shows a gold Đạo diễn dot and a blue/gray Hoạ sĩ dot. These are different agent identities, not the slight orb-gradient variance described in hỏi-đáp #15.
- [minor] Step-row agent dots: artboard shows gold dots beside Công cụ vẽ ảnh and Đăng nhập ChatGPT ; app shows blue/gray dots beside both rows.
- [minor] Step separators and insets: artboard shows separators extending across the list card and approximately 67px normal rows ; app shows inset separators inside the shared card and additional vertical spacing between rows, making the list visibly taller.
- [minor] Chrome-provided explanation: artboard shows “Dùng Chrome có sẵn trên máy, không cần tải thêm” ; app shows “Dùng Chrome chạy ngầm để xuất video và đo âm thanh” even though the side label says “Có sẵn”. The explanatory copy no longer explains that this step needs no download.
- [minor] Overall-progress palette: artboard shows a red-to-pink overall progress fill ; app shows a red-to-purple fill.
- [minor] ChatGPT focus treatment: artboard shows the entire final row tinted rose/red, with a matching rose flag-icon background ; app shows the row on the ordinary card surface with an orange/brown flag treatment.

## setup-failed

App: `setup-failed.png` · Artboard: `Onboard.dc.html?step=setup&setup=failed` · Reference shot: `qa-art-setup-failed.png`.

The missing action is the bright primary “Đăng nhập ChatGPT” button. The summary changes “bước 3” to “bước lỗi”; that dynamic reference is not scored separately.

- [major] Hero arrangement: artboard shows the orb on the LEFT of the heading and explanatory paragraph, both left-aligned ; app shows the orb ABOVE the heading with the entire hero centered.
- [major] Card hierarchy: artboard shows a standalone progress card followed by a separate six-step list card with a gap between them ; app shows the progress summary and all six steps enclosed in one card.
- [major] ChatGPT action: artboard shows a usable login button in the ChatGPT row ; app shows only a small spinner at the far right, with no visible login button or action label.
- [major] Below-list guidance: artboard shows the complete information-icon footnote, including where to add other providers and what happens after installation, within the 900px viewport ; app shows that guidance pushed to/below the bottom edge and clips it in the supplied shot.
- [minor] Main-column width: artboard shows 720px-wide cards spanning approximately x=360–1080 ; app shows a reduced usable card width of approximately 680px, x=380–1060, because its 720px main container includes horizontal padding.
- [minor] Agent-chip component: artboard shows compact rounded-rectangle chips (roughly 26px high, 7px corners) aligned to the left edge of the cards ; app shows taller, fully rounded pills centered above the card.
- [minor] Agent-chip palette: artboard shows a purple Đạo diễn dot and a warm gold Hoạ sĩ dot ; app shows a gold Đạo diễn dot and a blue/gray Hoạ sĩ dot. These are different agent identities, not the slight orb-gradient variance described in hỏi-đáp #15.
- [minor] Step-row agent dots: artboard shows gold dots beside Công cụ vẽ ảnh and Đăng nhập ChatGPT ; app shows blue/gray dots beside both rows.
- [minor] Step separators and insets: artboard shows separators extending across the list card and approximately 67px normal rows ; app shows inset separators inside the shared card and additional vertical spacing between rows, making the list visibly taller.
- [minor] Chrome-provided explanation: artboard shows “Dùng Chrome có sẵn trên máy, không cần tải thêm” ; app shows “Dùng Chrome chạy ngầm để xuất video và đo âm thanh” even though the side label says “Có sẵn”. The explanatory copy no longer explains that this step needs no download.
- [minor] Failure-progress color: artboard shows an orange progress bar for a failed installation ; app shows a pale yellow bar.
- [minor] Failed-row emphasis: artboard shows a warm orange-tinted background across the failed transcription row and an orange warning glyph ; app shows the unchanged row background and a yellow warning glyph.
- [minor] ChatGPT focus treatment: artboard shows the final row tinted rose/red and a rose flag-icon background ; app shows the ordinary card background and an orange/brown flag treatment.
- [major] Failed-step reason (weak implementer justification #6): artboard shows a specific network failure and retained-download explanation ; app shows the replacement reason “Chưa cài xong. Thử lại sẽ cài tiếp từ chỗ dừng.” The lack of a dedicated keptBytes field can justify omitting the byte count, but not discarding every failure reason: SetupStep already has error and progress.bytesDone, and StepRow unconditionally chooses the generic failedHint without reading error. The report also claims the raw error is in a title, but the rendered row code has no such title. Users are not told why the step failed.

## msettings-list

App: `msettings-list.png` · Artboard: `MSettings.dc.html?screen=list` · Reference shot: `qa-art-msettings-list.png`.

The supplied app is an error-attention machine (2 tools need repair), while the artboard list is a ready machine with one update. Summary status/icon, provider/platform counts, storage size, and blocked-platform values are live data, not scored. Proposal #1 already discloses the omitted lane count and the different queue-helper wording; those are not scored. Native OS status bar/home indicator in the phone artboard are device chrome, not missing app components.

- [minor] Between summary and settings list: artboard goes directly from the summary card to the six-row settings card ; app shows the additional copy “Máy này, dữ liệu và báo lỗi. Model, giọng đọc và tài khoản nền tảng cũng nằm đây.” across two lines, substantially increasing the gap and pushing the list downward.
- [minor] Queue-navigation card: artboard shows its title/helper copy and right chevron without a left icon ; app shows an additional flow/queue icon at the left, shifting the text column.

## msettings-tools

App: `msettings-tools.png` · Artboard: `MSettings.dc.html?screen=tools` · Reference shot: `qa-art-msettings-tools.png`.

The app shot is not a matching ready-tools fixture: it has two failed tools and ChatGPT still needing login. The larger failed rows and their retry buttons are not scored as missing/extra elements against a ready-state reference. They push the final ChatGPT content out of the visible list area; the screenshot therefore cannot substantiate the implementer’s claim that all six ready rows match. The closed details row proves neither the claimed monospace expanded content nor its absence. Version additions are disclosed by proposal #5 and excluded. Device status bar/home indicator are excluded.

- [major] Introductory region: artboard shows “Viderse tự tải, cài và cập nhật các công cụ này. Mọi việc xử lý video đều chạy trên máy của bạn.” between the header and list ; app shows the first tool card immediately after the header, omitting that whole explanation.
- [major] Update action variant: artboard shows a full-width BRIGHT primary “Cập nhật · <size>” button ; app shows a dark outlined button, removing the visual primary action.
- [minor] Update action icon: artboard includes a download icon before Cập nhật ; app shows text only.
- [minor] Update explanation tone: artboard colors the “YouTube hay đổi cách tải, nên cập nhật để tải không lỗi” explanation warm yellow ; app shows its update explanation in muted gray.
- [minor] Tool icon tone: artboard shows a neutral light download icon for the update row ; app shows a yellow download icon.
- [minor] Advanced-region label: artboard shows “Chi tiết · phiên bản và thư mục” ; app shows the desktop wording “Nâng cao · thư mục dữ liệu và phiên bản”.
- [minor] Advanced disclosure affordance: artboard shows a downward chevron at the right of the Chi tiết row ; app shows no right-side chevron.

## Missing or unusable state evidence

All 15 app filenames exist. All 15 reference states were initially missing artboard shots in this area; all have now been rendered as `qa-art-*` and visually read. No artboard shot remains missing. **frame-agents has no usable app UI shot** (the supplied PNG is blank), so that state remains unverified beyond the recorded major capture failure. **msettings-list/tools are not matching ready-state datasets**, and **frame-logs has no populated-error app state**; data-dependent content in those reference states cannot be certified by these shots. The implementer report’s statement that setup running/failed could not be captured is stale: both supplied PNGs exist and were reviewed here.

## Count table

| State | Majors | Minors |
|---|---:|---:|
| frame-tools | 1 | 6 |
| frame-connect | 1 | 6 |
| frame-agents | 1 | 0 |
| frame-accounts | 1 | 6 |
| frame-storage | 1 | 6 |
| frame-logs | 1 | 8 |
| tools-ok | 2 | 8 |
| tools-update | 2 | 9 |
| tools-failed | 3 | 9 |
| tools-installing | 2 | 10 |
| setup-running | 4 | 8 |
| setup-login | 4 | 8 |
| setup-failed | 5 | 9 |
| msettings-list | 0 | 2 |
| msettings-tools | 2 | 5 |
| **Total (state-level occurrences)** | **30** | **100** |

Status: DONE
Summary: **30 majors / 100 minors** across 15 reviewed states. The three worst problems are **(1)** setup replaces the login action with a spinner in all three states, including the login-only state; **(2)** setup centers/stacks the hero and merges the progress/list cards instead of the artboard’s left-orb, two-card hierarchy, pushing guidance off-screen; **(3)** frame-agents is entirely blank, leaving that complete state without usable app evidence.
