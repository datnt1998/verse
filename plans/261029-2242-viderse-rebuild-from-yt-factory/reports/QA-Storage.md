# QA · Storage

Independent screenshot fidelity review, 30/10/2026. The implementing report’s blanket “khớp” conclusions are not supported by these captures.

## Evidence and comparison rules

Compared all **18 supplied app PNGs**, individually, with the corresponding rendered design state: **12 desktop states at 1440×900** and **6 phone states at 390×844**. All app captures were read as images, not inferred from DOM/source. Final reference images are saved alongside them under `/tmp/viderse-fidelity/storage/shots/` as `art-store-*` / `art-m-*`.

Reference URLs: `http://127.0.0.1:18700/artboards/Settings.dc.html?section=storage&store=<state>&clean=<state>` and `MSettings.dc.html?screen=storage&store=<state>`. `reclaim=ask` was reached by clicking the artboard’s **Xoá video gốc** control, not by inventing a query tweak. The original storage directory had **no artboard PNGs**; all 18 counterparts were rendered for this review.

The requested JS-eval/browser.open tool is not exposed to this reviewer, so captures used an isolated named agent-browser CLI session with Google Chrome, headless mode, loopback-only domain allowlisting, and the prescribed WebAuthn-disabling init script. No external-site link was clicked; the browser session was closed. The loopback allowlist blocks Google Fonts, so the design’s Be Vietnam Pro 400/500/600 and JetBrains Mono were loaded into the reference browser from existing local built font assets; this avoids mistaking a system-font fallback for an app typography defect. No artboard layout or component CSS was changed.

Read the implementer report, artboard tweak list, binding hỏi-đáp entries, storage IA flow, copy guidance, token sizing, kit reuse rules, and storage artboard markup. Screenshot issues below are visual/capture findings; a mismatched cloud/exFAT fixture is not asserted to prove a backend defect.

**Excluded deliberately:** live byte counts, times, names/paths, task counts themselves, conditional setup readiness/dots, and user/avatar/version values. The app’s machine-not-ready banner explains its approximately 26 px desktop content offset; that contextual offset is not counted as a storage-layout defect. Artboard phone status/time/battery and home-indicator chrome are OS framing, not app controls, so their absence and the corresponding approximately 48 px top offset are not counted. Differences already reasonably justified in Fidelity-Storage’s proposals are also excluded: web/server-side folder chooser and default path (#10), USB-C bus omission, check wording “Ghi được vào đây”, no cleanup card on phone, omitted static-only “Xem khi xong ›”, and lack of a persisted resumed-video count in the desktop completion banner. Orb palette variance is not counted under #15.

**Justification quality:** “không có số trên wire” is a reasonable limitation for the persisted completion result, but counting every non-storage background task as a video is not equivalent to counting videos. This remains a weak semantic justification; the live-count-only screenshot differences are nevertheless excluded from the findings and totals. The report’s “footer dính đáy” is directly contradicted by both supplied phone picker images. Its “đủ cặp ảnh” claim was also incorrect at handoff: only app images existed. Its claim that the low-space footer recovery copy matches is contradicted by the desktop capture. The source supports the latter: `failureReason()` returns the first failed check, while the screenshot shows that check text where the recovery hint belongs. The cloud image retains Samsung T7 because its selected path is still that drive; the longer cloud explanation is gated on selecting a cloud-sync suggestion.

Major = missing action/region, wrong state or warning coverage, clipped primary action, reversed primary result/action structure, or a phone control visibly below the explicit 44 px target. Minor = other component, color, copy, spacing, or typography mismatch. Counts are **state-occurrences**, not deduplicated implementation bugs: shared shell/chart problems are intentionally listed under every state where visible. Off-screen regions are not claimed to have been compared.

## Desktop · store=here

Evidence: `art-store-here.png` ↔ `app-store-here.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [minor] Posted-MP4 card: artboard reserves more width for the orange action and wraps its explanation onto two lines; app’s smaller neutral action leaves the explanation on one line, making this card about 20 px shorter.

## Desktop · store=pick

Evidence: `art-store-pick.png` ↔ `app-store-pick.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [major] Folder-selection card structure: artboard replaces the current-location row with the “Thư mục mới” picker; app keeps the full Macintosh HD/Finder row above the picker, adding an entire extra region and pushing the form downward.
- [minor] New-folder field: artboard shows a 40 px-high path field; app’s field is visibly shorter. The stylesheet uses the invalid declaration `height: var(40px)` for this field.
- [minor] Suggested-drive selection: artboard visibly highlights the chosen drive pill with a brighter filled background; app’s selected Samsung T7 pill is not visually distinguished from the other ordinary pills.
- [minor] Suggested-drive metadata: artboard formats the external drive free space in TB and gives Google Drive no free-space suffix; app shows “1.620 GB” and adds “còn 0 MB” to Google Drive. These are formatting/presence differences, not differences in the live capacity value.
- [minor] Move notice component: artboard wraps the pause/editing explanation in a bordered, rounded banner; app renders its explanation as an unframed icon-and-text row.
- [minor] Picker footer controls: artboard shows 36 px-high Huỷ and white “Chuyển sang thư mục mới” buttons; app uses visibly smaller 30 px controls.

## Desktop · store=cloud

Evidence: `art-store-cloud.png` ↔ `app-store-cloud.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [major] Folder-selection card structure: artboard replaces the current-location row with the “Thư mục mới” picker; app keeps the full Macintosh HD/Finder row above the picker, adding an entire extra region and pushing the form downward.
- [minor] New-folder field: artboard shows a 40 px-high path field; app’s field is visibly shorter. The stylesheet uses the invalid declaration `height: var(40px)` for this field.
- [minor] Suggested-drive selection: artboard visibly highlights the chosen drive pill with a brighter filled background; app’s selected Samsung T7 pill is not visually distinguished from the other ordinary pills.
- [minor] Suggested-drive metadata: artboard formats the external drive free space in TB and gives Google Drive no free-space suffix; app shows “1.620 GB” and adds “còn 0 MB” to Google Drive. These are formatting/presence differences, not differences in the live capacity value.
- [minor] Move notice component: artboard wraps the pause/editing explanation in a bordered, rounded banner; app renders its explanation as an unframed icon-and-text row.
- [minor] Picker footer controls: artboard shows 36 px-high Huỷ and white “Chuyển sang thư mục mới” buttons; app uses visibly smaller 30 px controls.
- [major] Cloud-state identity: artboard selects Google Drive and displays its cloud-storage destination path; app still selects Samsung T7 and displays `/Volumes/Samsung T7` while claiming that this destination is a cloud-sync folder. The supplied image does not demonstrate the intended Google Drive state.
- [major] Cloud warning: artboard shows the full orange paragraph “Thư mục đồng bộ đám mây (Google Drive, iCloud, Dropbox) không dùng được: tệp đang dựng có thể bị khoá hoặc tải lên giữa chừng.” below the drive pills; app omits that entire explanation.
- [major] Cloud checklist: artboard shows three rows: cloud rejection, writable destination, and APFS large-file support; app shows only cloud rejection and a successful space check, with no writable/filesystem rows.
- [minor] Failed-check text color: artboard colors the cloud-rejection text orange; app colors only its icon yellow and leaves the check text neutral gray.
- [minor] Cloud recovery hint: artboard footer says “Chọn thư mục ngoài Google Drive”; app repeats “Thư mục đồng bộ đám mây: không dùng được” instead of giving the prescribed next action.

## Desktop · store=lowspace

Evidence: `art-store-lowspace.png` ↔ `app-store-lowspace.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [major] Folder-selection card structure: artboard replaces the current-location row with the “Thư mục mới” picker; app keeps the full Macintosh HD/Finder row above the picker, adding an entire extra region and pushing the form downward.
- [minor] New-folder field: artboard shows a 40 px-high path field; app’s field is visibly shorter. The stylesheet uses the invalid declaration `height: var(40px)` for this field.
- [minor] Suggested-drive selection: artboard visibly highlights the chosen drive pill with a brighter filled background; app’s selected Samsung T7 pill is not visually distinguished from the other ordinary pills.
- [minor] Suggested-drive metadata: artboard formats the external drive free space in TB and gives Google Drive no free-space suffix; app shows “1.620 GB” and adds “còn 0 MB” to Google Drive. These are formatting/presence differences, not differences in the live capacity value.
- [minor] Move notice component: artboard wraps the pause/editing explanation in a bordered, rounded banner; app renders its explanation as an unframed icon-and-text row.
- [minor] Picker footer controls: artboard shows 36 px-high Huỷ and white “Chuyển sang thư mục mới” buttons; app uses visibly smaller 30 px controls.
- [major] Low-space state/drive: artboard selects Lexar 128GB and shows its exFAT warning “exFAT dùng được; nhớ đẩy ổ ra trước khi rút”; app still selects Samsung T7 and displays a successful APFS check. This is not just a different free-space number: the warning variant and selected destination are different.
- [major] Low-space recovery hint: artboard footer says “Chọn thư mục khác hoặc “Chỉ video mới lưu ở thư mục mới””; app repeats the failed capacity check and omits both recovery options from that footer.
- [minor] Failed-check text color: artboard colors “Không đủ chỗ…” orange; app leaves that text gray and colors only the warning icon yellow.
- [minor] Existing-data estimate: artboard’s first radio card includes “Khoảng … phút · bản cũ giữ 7 ngày rồi mới xoá”; app omits the entire duration clause in this capture, leaving only the retention explanation.

## Desktop · store=moving

Evidence: `art-store-moving.png` ↔ `app-store-moving.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [major] Moving-card order: artboard shows current folder first, then the blue progress region; app puts progress first and moves the location row below it.
- [minor] Moving-card extra content: artboard does not show the “Luôn ở máy này: cơ sở dữ liệu, mô hình và công cụ…” note during a move; app adds that note after the location row, increasing the enclosing card’s height.
- [minor] Per-group progress colors: artboard colors completed byte totals green and active byte totals blue; app renders all the byte totals neutral gray despite retaining green check icons.

## Desktop · store=done

Evidence: `art-store-done.png` ↔ `app-store-done.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [major] Completion-card order: artboard shows the new location/Finder/change-folder row above the green success banner; app shows the success banner above that location row.
- [minor] Completion-card extra content: artboard ends the folder card after the success banner; app adds the persistent “Luôn ở máy này…” note beneath the location, enlarging the card.
- [minor] Success banner typography: artboard’s completion headline is bold, with the retention explanation at normal body size; app uses a lighter headline and a noticeably smaller, muted retention line.
- [minor] Success icon: artboard uses a filled green check disk; app uses a green outline-circle check.

## Desktop · store=donenew

Evidence: `art-store-donenew.png` ↔ `app-store-donenew.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [minor] Posted-MP4 card: artboard reserves more width for the orange action and wraps its explanation onto two lines; app’s smaller neutral action leaves the explanation on one line, making this card about 20 px shorter.
- [major] New-only completion order: artboard shows the new folder row first and the informational green banner second; app reverses those two regions.
- [minor] New-only completion typography: artboard’s entire “Video mới lưu ở…” explanation is regular body text; app presents the entire paragraph as a smaller, heavier banner title.
- [minor] New-only extra note: artboard has no “Luôn ở máy này…” note in this state; app appends that extra note below the location.
- [minor] Success icon: artboard uses a filled green check disk; app uses a green outline-circle check.

## Desktop · store=offline

Evidence: `art-store-offline.png` ↔ `app-store-offline.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [minor] Posted-MP4 card: artboard reserves more width for the orange action and wraps its explanation onto two lines; app’s smaller neutral action leaves the explanation on one line, making this card about 20 px shorter.
- [minor] Offline banner typography: artboard uses a bold warning title and normal-size body copy; app uses a lighter title and noticeably smaller gray body copy. Both controls remain present.

## Desktop · clean=idle

Evidence: `art-store-clean-idle.png` ↔ `app-store-clean-idle.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [minor] Posted-MP4 card: artboard reserves more width for the orange action and wraps its explanation onto two lines; app’s smaller neutral action leaves the explanation on one line, making this card about 20 px shorter.

## Desktop · clean=preview

Evidence: `art-store-clean-preview.png` ↔ `app-store-clean-preview.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Cleanup preview action row: artboard’s “Dọn 2,4 GB” button is about 101×32 px, with Để sau farther left; app’s button is about 90×30 px, shifts Để sau rightward, and compresses the action row.
- [minor] Abandoned-process outcome tone: artboard makes the right-hand “dừng lại” outcome muted gray; app renders it as bright body text.

## Desktop · clean=done

Evidence: `art-store-clean-done.png` ↔ `app-store-clean-done.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Group-deletion actions: artboard shows orange text/trash icons with orange outlines on “Xoá video gốc” and “Xoá MP4 đã đăng”; app shows neutral white/gray outlined buttons instead.
- [minor] Posted-MP4 card: artboard reserves more width for the orange action and wraps its explanation onto two lines; app’s smaller neutral action leaves the explanation on one line, making this card about 20 px shorter.

## Desktop · reclaim=ask (video gốc)

Evidence: `art-store-reclaim-ask.png` ↔ `app-store-reclaim-ask.png`.

- [minor] Desktop header: artboard shows all five navigation items inside one rounded, bordered dark strip, with no Bàn dựng selection on Settings; app shows free-standing navigation with a separate filled, bold Bàn dựng selection.
- [minor] Desktop header icons: artboard shows the picture/check-style Duyệt icon and the video-camera Kênh icon; app shows a flag for Duyệt and a rounded play-screen icon for Kênh.
- [major] Settings sidebar: artboard shows both “Luồng chạy · hàng chờ” and “Giới hạn từng kênh” under “Ở TRANG KHÁC”; app omits “Giới hạn từng kênh” entirely.
- [minor] Settings sidebar frame: artboard shows a vertical divider at the sidebar’s right edge and generous spacing before “Ở TRANG KHÁC”; app has no divider and compresses that group against “Báo lỗi”.
- [minor] Capacity chart: artboard shows brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the four colored segments and makes the system segment noticeably lighter. Desktop legend dots have the same color mismatch.
- [minor] Capacity heading: artboard shows plain “Ổ còn trống …” text; app prefixes it with a small stopwatch/timer-shaped icon.
- [minor] Cleanup preview button: artboard shows a roughly 215×32 px “Xem trước những gì sẽ dọn” control; app uses a narrower, roughly 200×30 px control with smaller text.
- [minor] Destructive confirmation CTA: artboard shows a saturated orange “Xoá … GB” button; app uses a pale yellow/sand button, weakening the intended danger tone.
- [minor] Remaining MP4 action: artboard still shows an orange-outlined “Xoá MP4 đã đăng” button in the other card; app renders that remaining action neutral white/gray.
- [minor] Posted-MP4 card: artboard reserves more width for the orange action and wraps its explanation onto two lines; app’s smaller neutral action leaves the explanation on one line, making this card about 20 px shorter.

## Phone · store=here

Evidence: `art-m-here.png` ↔ `app-m-here.png`.

- [minor] Mobile navigation header: artboard aligns the back chevron near x=24 and the title near x=52; app places them near x=18 and x=40, making the title start about 12 px farther left. This is independent of the excluded native status-bar height.
- [minor] Capacity chart colors: artboard uses brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the colored segments and lightens the system segment.
- [minor] Capacity free-space label: artboard shows plain small “Ổ còn …” text; app adds a timer icon, uses “Ổ còn trống …”, and makes the label larger.
- [major] Mobile deletion controls: artboard shows 44 px-high orange Xoá buttons with comfortably spaced rows; app uses approximately 30 px-high buttons and compresses both rows. This also falls below the design’s explicit 44 px phone target size.
- [major] Change-folder control: artboard shows a full-width 44 px-high “Đổi thư mục lưu” button; app uses a full-width but only approximately 30 px-high button, below the prescribed phone touch-target height.

## Phone · store=pick

Evidence: `art-m-pick.png` ↔ `app-m-pick.png`.

- [minor] Mobile navigation header: artboard aligns the back chevron near x=24 and the title near x=52; app places them near x=18 and x=40, making the title start about 12 px farther left. This is independent of the excluded native status-bar height.
- [major] Sheet footer visibility: artboard shows the entire 48 px-high primary action and a separated bottom footer within the 844 px viewport; app starts its action around y=822, leaving most of the button and its label below the viewport. The supplied shot contradicts the implementer’s claim that the footer is correctly pinned and visible.
- [major] Sheet density/layout: artboard starts the sheet around y=168–170 and uses roughly 56 px drive rows; app starts around y=205 and uses roughly 70 px drive rows. The excess row height pushes the segmented control and checks about 80–90 px farther down, rather than preserving a compact scroll body above a fixed footer.
- [minor] Sheet surface: artboard uses the darker #141416 sheet surface; app’s large sheet surface is visibly lighter gray.
- [minor] Drive radio styling: artboard uses approximately 22 px fine-stroke radio circles with a small solid accent center; app uses thicker, heavier circles with a different selected-ring treatment.
- [minor] Drive kind labeling: artboard identifies Lexar as “USB · exFAT”; app labels it “Ổ ngoài · exFAT”. The report’s accepted USB-C/bus omission does not explain replacing the separate USB kind with generic external-drive wording.
- [minor] Segmented-control height: artboard’s “Chuyển … sang / Chỉ video mới” control is about 48 px high; app compresses it to about 36 px.
- [minor] Primary-action copy: artboard says “Chuyển sang thư mục mới”; app says “Chuyển 81,1 GB sang”. This is a different label template, not merely a changed live byte value.

## Phone · store=lowspace

Evidence: `art-m-lowspace.png` ↔ `app-m-lowspace.png`.

- [minor] Mobile navigation header: artboard aligns the back chevron near x=24 and the title near x=52; app places them near x=18 and x=40, making the title start about 12 px farther left. This is independent of the excluded native status-bar height.
- [major] Sheet footer visibility: artboard shows the entire 48 px-high primary action and a separated bottom footer within the 844 px viewport; app starts its action around y=822, leaving most of the button and its label below the viewport. The supplied shot contradicts the implementer’s claim that the footer is correctly pinned and visible.
- [major] Sheet density/layout: artboard starts the sheet around y=168–170 and uses roughly 56 px drive rows; app starts around y=205 and uses roughly 70 px drive rows. The excess row height pushes the segmented control and checks about 80–90 px farther down, rather than preserving a compact scroll body above a fixed footer.
- [minor] Sheet surface: artboard uses the darker #141416 sheet surface; app’s large sheet surface is visibly lighter gray.
- [minor] Drive radio styling: artboard uses approximately 22 px fine-stroke radio circles with a small solid accent center; app uses thicker, heavier circles with a different selected-ring treatment.
- [minor] Drive kind labeling: artboard identifies Lexar as “USB · exFAT”; app labels it “Ổ ngoài · exFAT”. The report’s accepted USB-C/bus omission does not explain replacing the separate USB kind with generic external-drive wording.
- [minor] Segmented-control height: artboard’s “Chuyển … sang / Chỉ video mới” control is about 48 px high; app compresses it to about 36 px.
- [minor] Primary-action copy: artboard says “Chuyển sang thư mục mới”; app says “Chuyển 81,1 GB sang”. This is a different label template, not merely a changed live byte value.
- [major] Low-space selected drive and warning: artboard selects Lexar 128GB and shows the exFAT eject warning; app still selects Samsung T7 and shows a green APFS success row. The supplied capture misses the designed low-space/exFAT combination.
- [minor] Low-space check tone: artboard colors the failed-space row orange; app leaves the check text gray and only colors the icon yellow.
- [minor] Recovery-hint copy: artboard ends “Chọn thư mục khác hoặc “Chỉ video mới lưu ở thư mục mới”” without a final period; app adds a final period.

## Phone · store=moving

Evidence: `art-m-moving.png` ↔ `app-m-moving.png`.

- [minor] Mobile navigation header: artboard aligns the back chevron near x=24 and the title near x=52; app places them near x=18 and x=40, making the title start about 12 px farther left. This is independent of the excluded native status-bar height.
- [minor] Capacity chart colors: artboard uses brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the colored segments and lightens the system segment.
- [minor] Capacity free-space label: artboard shows plain small “Ổ còn …” text; app adds a timer icon, uses “Ổ còn trống …”, and makes the label larger.
- [major] Mobile deletion controls: artboard shows 44 px-high orange Xoá buttons with comfortably spaced rows; app uses approximately 30 px-high buttons and compresses both rows. This also falls below the design’s explicit 44 px phone target size.
- [minor] Cancel-move alignment and size: artboard shows the larger “Huỷ chuyển” action left-aligned inside the progress card; app centers a smaller action across the card.

## Phone · store=done

Evidence: `art-m-done.png` ↔ `app-m-done.png`.

- [minor] Mobile navigation header: artboard aligns the back chevron near x=24 and the title near x=52; app places them near x=18 and x=40, making the title start about 12 px farther left. This is independent of the excluded native status-bar height.
- [minor] Capacity chart colors: artboard uses brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the colored segments and lightens the system segment.
- [minor] Capacity free-space label: artboard shows plain small “Ổ còn …” text; app adds a timer icon, uses “Ổ còn trống …”, and makes the label larger.
- [major] Mobile deletion controls: artboard shows 44 px-high orange Xoá buttons with comfortably spaced rows; app uses approximately 30 px-high buttons and compresses both rows. This also falls below the design’s explicit 44 px phone target size.
- [major] Change-folder control: artboard shows a full-width 44 px-high “Đổi thư mục lưu” button; app uses a full-width but only approximately 30 px-high button, below the prescribed phone touch-target height.
- [major] Completion layout: artboard orders location → green success banner → “Đổi thư mục lưu”; app orders location → “Đổi thư mục lưu” → success banner, moving the next action above the result it follows.
- [major] Delete-old-copy touch target: artboard’s orange “Xoá bản cũ ngay” action is 44 px high; app’s is approximately 30 px high.
- [minor] Completion typography: artboard keeps “Đã chuyển … GB.” inline with the normal-size retention sentence; app splits them into a headline and a smaller muted subtitle.
- [minor] Completion punctuation: artboard includes a full stop after “Đã chuyển … GB.”; app’s standalone headline omits it.
- [minor] Success icon: artboard uses a filled green check disk; app uses an outline-circle check.

## Phone · store=offline

Evidence: `art-m-offline.png` ↔ `app-m-offline.png`.

- [minor] Mobile navigation header: artboard aligns the back chevron near x=24 and the title near x=52; app places them near x=18 and x=40, making the title start about 12 px farther left. This is independent of the excluded native status-bar height.
- [minor] Capacity chart colors: artboard uses brighter opaque blue/lilac/sand/mint segments and a dark gray system segment; app dims the colored segments and lightens the system segment.
- [minor] Capacity free-space label: artboard shows plain small “Ổ còn …” text; app adds a timer icon, uses “Ổ còn trống …”, and makes the label larger.
- [major] Mobile deletion controls: artboard shows 44 px-high orange Xoá buttons with comfortably spaced rows; app uses approximately 30 px-high buttons and compresses both rows. This also falls below the design’s explicit 44 px phone target size.
- [major] Offline recovery touch targets: artboard shows 44 px-high Thử lại and fallback-storage controls; app makes both approximately 30 px high and correspondingly narrows the buttons.
- [minor] Offline banner typography: artboard uses larger body copy and a strong warning heading; app uses substantially smaller gray body copy and a lighter heading, shrinking the warning panel.
- [minor] Offline copy: artboard says “Ổ được rút lúc … . 2 video đã tạm dừng, hàng chờ chưa chạy. Cắm lại ổ thì máy tự chạy tiếp.”; app uses “Video đang làm đã tạm dừng, hàng chờ chưa chạy tiếp” and adds “không mất gì.”. Ignoring the live time/count, “đang làm”, “chạy tiếp”, and the reassurance clause are still different copy.

## Missing-shot inventory

All 18 named comparison states now have both app and artboard shots. Originally all 18 artboard counterparts were missing; they have been supplied by this QA pass. No named app state is missing. The provided set has no separate posted-MP4 confirmation capture, no post-deletion result capture, and no arbitrary-folder browser capture; these additional interactive states are not covered or counted. Desktop `store=here` and `clean=idle` are separate supplied captures of the same design combination and are intentionally retained as separate rows.

## Count table

| State | Majors | Minors |
|---|---:|---:|
| Desktop · store=here | 1 | 8 |
| Desktop · store=pick | 2 | 8 |
| Desktop · store=cloud | 5 | 10 |
| Desktop · store=lowspace | 4 | 10 |
| Desktop · store=moving | 2 | 9 |
| Desktop · store=done | 2 | 10 |
| Desktop · store=donenew | 2 | 11 |
| Desktop · store=offline | 1 | 9 |
| Desktop · clean=idle | 1 | 8 |
| Desktop · clean=preview | 1 | 8 |
| Desktop · clean=done | 1 | 7 |
| Desktop · reclaim=ask (video gốc) | 1 | 9 |
| Phone · store=here | 2 | 3 |
| Phone · store=pick | 2 | 6 |
| Phone · store=lowspace | 3 | 8 |
| Phone · store=moving | 1 | 4 |
| Phone · store=done | 4 | 6 |
| Phone · store=offline | 2 | 5 |
| **Total state-occurrences** | **37** | **139** |

Status: DONE
Summary: **37 majors / 139 minors** across 18 states. The three worst problems are **(1)** both phone folder-picker primary actions are mostly below the viewport, **(2)** cloud/low-space captures do not select the designed cloud/exFAT destinations and omit required warning/recovery content, and **(3)** phone change-folder, delete, and offline-recovery controls are only about 30 px high instead of the required 44 px.
