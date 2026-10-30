# QA2 · Storage

Independent second-round visual review of all **18 screenshot pairs / 36 images**, read individually with the image read tool. Evidence directory: `/tmp/viderse-fidelity/storage/shots/`. Compared against the storage IA, component/sheet rules, kit, tokens, copy guidance, artboard inventory, and hỏi-đáp #18–#26; also read `QA-Storage.md` and `Fidelity-Storage.md`.

## Scope and interpretation

- Twelve desktop states and six phone states are paired. No browser was needed or opened; no application code, tests, or builds were changed/run.
- Desktop comparisons are at the supplied 1440×900 framing. `art-store-reclaim-ask.png` is 1800×1125, the same aspect ratio and apparent layout at 1.25×; compare its geometry proportionally, not by raw pixel coordinates. Its app counterpart is 1440×900.
- Phone images are 390×844. Native status bar/home indicator framing is not an app defect. For ordinary phone content, discount the reference-only approximately 44 px status-bar offset. Bottom-sheet positions are compared directly because both sheets are anchored to the same viewport bottom.
- Exclude live numbers, timestamps, volume/path values, readiness dots, avatar, version, and live navigation badges. Do not infer functional/backend defects from screenshots. Numeric date values are live; slash-versus-dash date **format** is a visual difference.
- Accepted pending deviations #18–#26 are **not defects or counted differences**, even though their document entries remain marked Mở. In particular: #19 permits omitting the channel-limit sidebar link; #21 permits a server-side folder browser; #22 permits USB/Thunderbolt/generic external-drive labels; #23 permits omitted video-resumption/static-demo controls and live-task notice variants; #24 permits a 36 px cleanup-preview opener instead of 32 px. These exclusions do not waive unrelated typography, placement, or other button sizes.
- The external-drive shots still show “Ổ ngoài”; per the assignment they can predate the bus-label change. Neither that nor Lexar’s generic external-drive wording is counted. Do not use these captures to claim that the latest “Ổ ngoài USB” label has been verified.
- Counts below are **state-occurrences**, as in round 1, not independent implementation bugs. A shared difference is described once with an ID and expanded across its explicitly named states in the count/evidence table. There are 27 listed difference groups, totaling 93 minor state-occurrences. Regions below the capture are not claimed to have been reviewed.

## 1. Round-1 major disposition

`fixed` means the originally major failure is gone in the supplied pair; it does not mean pixel-perfect. Residual minor differences are listed separately. The one partially corrected low-space sheet layout is deliberately not called fully fixed.

| Round-1 major | Occurrences | Status | Current screenshot evidence |
|---|---:|---|---|
| Desktop sidebar omits “Giới hạn từng kênh” | 12 | still wrong | Every `art-store-*` has the link; every `app-store-*` still omits it. **Accepted #19**, therefore excluded from remaining-defect counts. |
| Desktop pick retains the old location row above the picker | 1 | fixed | `art-store-pick.png` ↔ `app-store-pick.png`: picker replaces location row. |
| Desktop cloud retains the old location row above the picker | 1 | fixed | `art-store-cloud.png` ↔ `app-store-cloud.png`: no extra Finder/location row. |
| Desktop lowspace retains the old location row above the picker | 1 | fixed | `art-store-lowspace.png` ↔ `app-store-lowspace.png`: no extra Finder/location row. |
| Desktop cloud capture selects Samsung rather than Google Drive | 1 | fixed | `app-store-cloud.png` now highlights Google Drive and shows its CloudStorage destination. |
| Desktop cloud omits the full cloud-sync warning | 1 | fixed | Both cloud shots show the complete orange Google Drive/iCloud/Dropbox warning paragraph. |
| Desktop cloud omits writable/filesystem checklist rows | 1 | fixed | `app-store-cloud.png` has cloud rejection, writable, and APFS rows, like its art pair. |
| Desktop lowspace selects APFS Samsung instead of exFAT Lexar | 1 | fixed | `app-store-lowspace.png` now shows Lexar and the exFAT eject warning. |
| Desktop lowspace footer omits both recovery options | 1 | fixed | Lowspace app footer now says “Chọn thư mục khác hoặc “Chỉ video mới lưu ở thư mục mới””. |
| Desktop moving reverses location/progress order | 1 | fixed | `app-store-moving.png`: location precedes blue progress card. |
| Desktop done reverses location/success order | 1 | fixed | `app-store-done.png`: location precedes green success banner. |
| Desktop donenew reverses location/result order | 1 | fixed | `app-store-donenew.png`: location precedes green informational banner. |
| Phone here deletion buttons are only about 30 px high | 1 | fixed | `app-m-here.png`: both Xoá controls are approximately 44 px high. |
| Phone here change-folder button is only about 30 px high | 1 | fixed | `app-m-here.png`: full-width change-folder control is approximately 44 px high. |
| Phone pick primary action is clipped below viewport | 1 | fixed | `app-m-pick.png`: full CTA visible at approximately y=788–836. |
| Phone pick sheet has oversized drive rows/excessive density | 1 | fixed | `app-m-pick.png`: top approximately y=170 versus reference y=168; ordinary drive rows approximately 56 px, not 70 px. Minor row/footer differences remain. |
| Phone lowspace primary action is clipped below viewport | 1 | fixed | `app-m-lowspace.png`: full CTA visible at approximately y=792–836. |
| Phone lowspace sheet density/layout differs | 1 | still wrong | `art-m-lowspace.png` starts approximately y=170; `app-m-lowspace.png` approximately y=196. Rows are corrected to approximately 56 px, but top position and segmented height still differ. **Now minor**, since the primary action is fully visible and phone targets meet 44 px; see M8–M10. |
| Phone lowspace selects APFS Samsung instead of exFAT Lexar | 1 | fixed | `app-m-lowspace.png` now selects Lexar and displays failed-space plus exFAT warning rows. |
| Phone moving deletion buttons are only about 30 px high | 1 | fixed | `app-m-moving.png`: both Xoá controls approximately 44 px high. |
| Phone done deletion buttons are only about 30 px high | 1 | fixed | `app-m-done.png`: both Xoá controls approximately 44 px high. |
| Phone done change-folder button is only about 30 px high | 1 | fixed | `app-m-done.png`: change-folder control approximately 44 px high. |
| Phone done puts change-folder above success result | 1 | fixed | `app-m-done.png`: location → success banner → change-folder, matching reference. |
| Phone done delete-old-copy target is only about 30 px high | 1 | fixed | `app-m-done.png`: “Xoá bản cũ ngay” approximately 44 px high. |
| Phone offline deletion buttons are only about 30 px high | 1 | fixed | `app-m-offline.png`: both Xoá controls approximately 44 px high. |
| Phone offline recovery targets are only about 30 px high | 1 | fixed | `app-m-offline.png`: Thử lại and fallback-storage controls approximately 44 px high. |
| **Total** | **37** | **24 fixed; 13 still wrong; 0 not verifiable** | **12 still-wrong occurrences are accepted #19; the other is partially fixed and downgraded to minor. Zero unaccepted major-severity failures remain visible.** |

The important round-1 structural repairs are real: in-place picker replacement, correct cloud/exFAT states and warnings, location-before-result order, visible phone actions, and 44 px phone targets. The claim that every detail is fixed is not supported: typography, chart gray, button treatment, and some sheet geometry still differ.

## 2. New or remaining visible differences

### Desktop · shared differences

The IDs below are used in the per-state evidence/count table. “All desktop” means all twelve named desktop pairs, including the visible chart strip at the bottom of cloud and the proportionally normalized reclaim reference.

- [minor] **S1 · all desktop** · sidebar spacing: artboard places the selected Lưu trữ row at about y=281 and “Ở TRANG KHÁC” at y=377; app places them about y=279 and y=374, with the subsequent queue link correspondingly a few pixels higher. The divider and substantial group separation are restored; this is only residual spacing, not the old missing-divider issue.
- [minor] **C1 · all desktop** · capacity chart/system legend: artboard ends the bar with a visibly medium-gray system segment and matching dot; app uses a substantially darker charcoal segment/dot. Reference `Settings.dc.html` usage data explicitly sets “Mô hình và công cụ” to `#6E6E78`, not the `#3A3A40` claimed as matching in Fidelity-Storage. The four colored desktop segments now match visibly; the fifth does not.
- [minor] **A1 · here, moving, done, donenew, offline, clean-idle, clean-preview, clean-done, reclaim-ask** · group deletion controls: artboard uses orange text/icons with a fine orange outline on an almost-neutral background, approximately 32 px high; app uses a more visibly orange-filled soft-danger background, approximately 30 px high, and narrower controls (e.g. Xoá video gốc starts around x=893 instead of x=883; Xoá MP4 around x=873 instead of x=861). The old neutral-white tone is fixed, but the treatment and density still differ. In reclaim-ask this applies to the remaining MP4 action, not the correctly orange solid confirmation CTA.
- [minor] **A2 · here, done, donenew, offline, clean-idle, clean-done** · posted-MP4 card: artboard reserves enough width for the larger action that its explanation ends with “MP4.” on a second line; app keeps the same explanation on one line and makes the card about 20 px shorter. This is visible even with matching sample content, so it is not a live-data discrepancy. The reclaim reference itself uses one line and is not included here.
- [minor] **A3 · here, moving, done, donenew, offline, clean-idle, reclaim-ask** · cleanup-preview opener: artboard has a lighter-weight “Xem trước những gì sẽ dọn” label in an approximately 215 px-wide outlined control; app uses a heavier label in an approximately 221 px-wide control, shifting its left edge about 6 px left. The accepted 36-versus-32 px **height** difference (#24) is not counted.
- [minor] **S2 · pick, cloud, lowspace, moving, clean-preview** · sidebar footer: artboard keeps “Mọi cài đặt ở đây áp dụng cho cả máy, mọi kênh dùng chung.” and the version visible at the viewport bottom; app loses the footer below the captured viewport in pick/cloud/lowspace/clean-preview, and only its first line is visible near the bottom in moving. The changing version value/wrapping is excluded; loss of the persistent explanatory footer is the visible difference.

### Desktop · state-specific differences

- [minor] **F1 · pick, cloud, lowspace** · existing-data radio-card/checklist density: artboard radio cards are about 57 px tall with the second card ending around y=550 in pick; app cards are about 55 px, ending around y=546. Checklist, notice, and footer consequently accumulate an approximately 3–5 px upward shift even in pick, where the suggested-drive inventory matches. Correct component types and 36 px footer actions are present now.
- [minor] **F2 · cloud, lowspace** · suggested destinations/layout: artboard retains the Macintosh HD suggestion alongside Samsung, Lexar, and Google Drive, wrapping Google Drive onto a second line; app omits Macintosh HD in these two states and fits the other three on one line. This removes a visible destination choice and lifts the remainder of the form about one pill row (roughly 36 px). In app-store-pick the internal-drive pill is present. This is a screenshot-state/presence discrepancy, not a claim about which drives the real server must discover.
- [minor] **F3 · lowspace** · destination path tone: artboard leaves the Lexar path in the ordinary light mono text while warning about insufficient capacity in the checklist/footer; app also colors the path itself orange, like an invalid cloud destination. Compare `art-store-lowspace.png` and `app-store-lowspace.png`.
- [minor] **B1 · done** · success-banner typography/date format: artboard has a larger bold completion heading and ordinary-size bright retention text; app makes the heading smaller and the retention line smaller/muted. Ignoring the live date value, reference uses slash day/month (`04/11`), whereas app uses a dash (`06–11`). The accepted omission of “2 video đã chạy tiếp.” is not counted.
- [minor] **B2 · donenew** · informational-banner typography: artboard renders the whole “Video mới lưu ở…” paragraph as normal approximately 14 px body text; app now correctly removes the separate heading but renders the paragraph smaller and gray, producing different wrapping and a slightly shorter banner/card. No extra “Luôn ở máy này…” note remains.
- [minor] **B3 · offline** · warning-banner typography: artboard uses a stronger/larger heading and approximately 14 px body copy; app retains a smaller/lighter heading and approximately 12–13 px gray body copy. The banner remains in the correct enclosing card, before location, and both recovery actions are present. Live/task-count wording covered by #23 is excluded.
- [minor] **B4 · done, donenew** · success icon tone: artboard uses a bright mint filled check disk; app now has a filled disk, but its disk is markedly darker/desaturated with a pale check. This is no longer the round-1 outline-circle icon, yet it still differs from the reference.
- [minor] **K1 · clean-preview** · cleanup action row: artboard shows an approximately 32 px-high Dọn button and lighter quiet Để sau control; app uses approximately 36 px controls and heavier labels, extending the preview card about 4 px and moving the following card down. #24 accepts the preview **opener** height, not this separate confirmation row. The “dừng lại” outcome is now correctly muted.

### Phone · shared differences

- [minor] **M1 · here, pick, lowspace, moving, done, offline** · navigation header: artboard back chevron is near x=24 and title starts near x=52; app chevron is near x=18 and title near x=40. This 12 px title alignment error remains after excluding native top chrome, and is also visible through the picker scrims.
- [minor] **M2 · here, moving, done, offline** · capacity-chart colors: artboard has a brighter medium-gray final system segment; app ends in darker charcoal. The sand/editor segment is also slightly duller in app: phone reference explicitly uses `#FFD08A`, unlike the desktop `#F6CB8C`. Blue/lilac/mint are restored. The plain “Ổ còn …” label is restored and is not a remaining finding.
- [minor] **M3 · here, moving, done, offline** · folder icon alignment: artboard centers the 40 px folder tile vertically against the full name/type/path block; app top-aligns it near the name. For example, in here the icon sits above the middle of the multi-line metadata block even after compensating for the status-bar offset. Tile size and icon type themselves are correct.
- [minor] **M4 · here, moving, done, offline** · capacity-card/action density: artboard Xoá controls are about 62 px wide with larger labels; app controls are about 55 px wide with smaller labels (left edge approximately x=303 rather than x=297). Both now meet 44 px height. App deletion rows/padding and the gap above the capacity card also accumulate several extra pixels vertically compared with reference; this is not the excluded whole-screen status-bar offset.

### Phone · picker details

- [minor] **M5 · pick, lowspace** · drive-row radios/type/density: artboard uses approximately 22 px thin radio circles with a small selected center, drive names beginning near x=55 at approximately 14 px, and approximately 8 px inter-row gaps; app uses smaller roughly 18 px circles with a different selected-ring treatment, names near x=51 at approximately 13 px, and tighter gaps. Ordinary row height is corrected to roughly 56 px. USB/generic-kind labels are excluded under #22.
- [minor] **M6 · pick, lowspace** · sheet heading/help spacing and wrapping: artboard helper text wraps with “Projects” on its second line; app fits that word on the first line and leaves only “trong đó.” on the second. In pick the title/helper block is also a few pixels lower within an otherwise nearly matching sheet top, with different spacing before the first drive. The wording is present, not omitted.
- [minor] **M7 · pick, lowspace** · sticky footer bottom inset: artboard primary button ends at about y=812, leaving roughly 32 px to the viewport bottom; app button ends at y=836, only roughly 8 px above the bottom. The separator/footer and CTA are therefore lower. This is a bottom-safe-area/layout mismatch, **not clipping**: both labels and full buttons are visible now.
- [minor] **M8 · lowspace** · sheet top/overall placement: artboard low-space sheet begins around y=170; app begins around y=196, approximately 26 px lower, exposing more of the underlying project card. This residual difference is why the round-1 density/layout entry is “still wrong” rather than fully fixed; it no longer constitutes a major action-visibility failure.
- [minor] **M9 · lowspace** · segmented control: artboard control is approximately 48 px high; app low-space control is roughly 54–56 px, enlarging the selected segment and shifting the checklist. The regular app-m-pick control is much closer to the reference 48 px and is not included in this finding.
- [minor] **M10 · lowspace** · disabled primary-action height: artboard disabled “Chuyển sang thư mục mới” remains approximately 48 px high; app low-space CTA is approximately 44 px (about y=792–836). It meets the minimum phone target but does not match the sheet-primary size. App-m-pick has the full approximately 48 px CTA. Thus the implementer’s blanket “CTA 48 px” claim does not hold for this supplied low-space shot.

### Phone · result/warning details

- [minor] **M11 · done** · success-banner typography/date format: artboard keeps the bold “Đã chuyển … GB.” inline with normal approximately 14 px retention text; app now correctly keeps the inline structure and full stop, but makes the whole paragraph smaller/muted. App uses dash-form day/month rather than the reference slash format, independently of the live date value. Ordering and 44 px actions are fixed.
- [minor] **M12 · done** · success icon tone: artboard check disk is bright mint; app filled disk is darker/desaturated with a pale tick, as in desktop done/donenew. The old outline-circle icon is gone.
- [minor] **M13 · offline** · warning typography and panel density: artboard heading is stronger/larger and its paragraph is approximately 14 px; app has a smaller/lighter heading and smaller gray paragraph, shrinking the warning panel by roughly 15–20 px after excluding native top chrome. Copy now follows the intended warning template apart from accepted live/task-count variation; both 44 px recovery actions and the absence of a change-folder button are correct.

## Accepted-deviation quality notes — excluded from counts

- **#19:** missing future channel-limit route is explicitly accepted for this assignment. The link is still visibly absent, so the round-1 table does not mislabel it “fixed.” No new defect is counted.
- **#21:** server-side folder browsing is a credible web/server boundary constraint; the supplied picker screenshots do not show that extra browser state, so this review makes no fidelity claim about its internal dialog/sheet.
- **#22:** USB versus USB-C is a sound hardware-information distinction. The assignment explicitly warns that the shots may predate the new bus label; generic labels here do not disprove that change, and the updated wording is not verifiable from these images.
- **#23 — weak semantic justification:** using a count of all running background tasks while calling them “video” is not the same as counting affected videos. For example the desktop picker notice now says “1 video đang làm…”, while this review has no evidence that its source task is a video. The accepted limitation explains absent counts/static demo links; it does not make that noun semantically accurate. Flagged as a weak excuse, **not** counted as a screenshot defect or asserted as a proven runtime bug.
- **#24:** reusing the 36 px kit button is a reasonable consistency decision. It accepts only that opener-height discrepancy; it does not establish that every app button’s weight, width, or every other cleanup action is pixel-matched.
- #18, #20, #25, and #26 concern other Settings areas and do not authorize further Storage differences.

## 3. Missing-pair inventory

**None of the 18 assigned states is missing a pair.** Every app-* listed below has its art-* counterpart and both were read.

Additional interactive states without supplied pairs: arbitrary server-side folder browser (#21), posted-MP4 deletion confirmation, and post-group-deletion result. These were also outside the named round-1 paired inventory; they are unverified coverage, not invented defects or part of the 18-state count. No additional app server exists for recapture, and no browser was needed to judge the supplied pairs.

## 4. Count and evidence table

Names in the first column are the exact shared filename suffix: e.g. `store-here` means `art-store-here.png` ↔ `app-store-here.png`; `m-here` means `art-m-here.png` ↔ `app-m-here.png`.

| Pair suffix | Remaining majors | Remaining minors | Difference IDs |
|---|---:|---:|---|
| store-here | 0 | 5 | S1, C1, A1, A2, A3 |
| store-pick | 0 | 4 | S1, C1, S2, F1 |
| store-cloud | 0 | 5 | S1, C1, S2, F1, F2 |
| store-lowspace | 0 | 6 | S1, C1, S2, F1, F2, F3 |
| store-moving | 0 | 5 | S1, C1, A1, A3, S2 |
| store-done | 0 | 7 | S1, C1, A1, A2, A3, B1, B4 |
| store-donenew | 0 | 7 | S1, C1, A1, A2, A3, B2, B4 |
| store-offline | 0 | 6 | S1, C1, A1, A2, A3, B3 |
| store-clean-idle | 0 | 5 | S1, C1, A1, A2, A3 |
| store-clean-preview | 0 | 5 | S1, C1, A1, S2, K1 |
| store-clean-done | 0 | 4 | S1, C1, A1, A2 |
| store-reclaim-ask | 0 | 4 | S1, C1, A1, A3 |
| **Desktop subtotal** | **0** | **63** | 12 pairs |
| m-here | 0 | 4 | M1, M2, M3, M4 |
| m-pick | 0 | 4 | M1, M5, M6, M7 |
| m-lowspace | 0 | 7 | M1, M5, M6, M7, M8, M9, M10 |
| m-moving | 0 | 4 | M1, M2, M3, M4 |
| m-done | 0 | 6 | M1, M2, M3, M4, M11, M12 |
| m-offline | 0 | 5 | M1, M2, M3, M4, M13 |
| **Phone subtotal** | **0** | **30** | 6 pairs |
| **Total state-occurrences** | **0** | **93** | **18 complete pairs; 27 listed difference groups** |

| Round-1 accountability | Major occurrences |
|---|---:|
| Fixed in current shots | 24 |
| Still visually absent but accepted #19, excluded | 12 |
| Partially corrected and downgraded to minor | 1 |
| Not verifiable | 0 |
| **Original total accounted for** | **37** |

Status: DONE
Summary: **0 unaccepted majors remaining; 93 minor state-occurrences remaining (27 listed difference groups).** Worst 3: **(1)** low-space phone sheet still has mismatched placement/segmented/CTA geometry and both picker footers sit too close to the viewport bottom; **(2)** success/offline explanatory text remains smaller and more muted than the references; **(3)** the system capacity segment is still the wrong dark gray throughout, contradicting the implementer’s full-color-match claim. All round-1 action-clipping, sub-44 px phone-target, cloud/exFAT warning, and result-order major failures are resolved; the missing channel-limit link is accepted #19, not fixed.
