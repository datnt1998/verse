# QA2 — Machine

## Scope and evidence

Independent second-round visual review of **all 15 supplied pairs / 30 PNGs**, each opened with the image reader, from `/tmp/viderse-fidelity/FidelityMachine/shots/`. In this report, a state means `<state>.png` versus `qa-art-<state>.png` in that directory. Desktop comparisons are normalized to 1440×900 CSS proportions; phone comparisons to 390×844. The app phone images are 780×1688, not twice the intended layout size. The artboard phone status bar (44px) and home indicator are device chrome and are excluded.

Read `QA-Machine.md`, `Fidelity-Machine.md`, and the relevant contracts in design/v1 `01-ia-flows.md`, `03-tokens.md`, `04-components.md`, `05-kit.md`, `06-copy.md`, `artboards/README.md`, and `11-hoi-dap.md`. Checked mobile reference markup where necessary to distinguish clipping from different copy. No app code, tests, builds, or browser session was run. No files other than this report were written.

**Verdict: the claim that all majors are fixed is not supported.** Four current tool captures still visibly detach the Advanced row; `frame-tools` retains the old navigation; `frame-agents` is still incomplete; `frame-storage` and `frame-logs` now show the wrong loading screen rather than Settings. Setup has improved substantially: its hero, two-card hierarchy, real login buttons, visible guidance, and readable failure reason are fixed.

### Scoring and boundaries

- Major = wrong hierarchy/destination, missing principal content/action, or unusable state evidence. Minor = localized copy, icon, tone, spacing, or component mismatch.
- Counts are **state-level occurrences**, as in round 1. One root cause repeated in four shots counts four times. The major-status table is a reconciliation, not additional findings to add to the totals.
- Frame states retain round-1 ownership: header/sidebar/footer/right rail, not Connect/Accounts/Storage body styling. Missing or wrong-state captures are still evidence failures for this area. No claim below establishes that the live application always stays loading.
- Live versions, sizes, timestamps, account identities, counts, availability, installed/provided status, and signed-in versus signed-out state are excluded. In particular, ChatGPT needing login in the tool fixtures is not counted as a visual defect against the signed-in reference; the fixture cannot certify the signed-in row.
- Visible typeface/antialiasing differences are **not scored as product defects**: round 1 records locally rendered references without remote Google fonts, while `artboards/README.md` explicitly requires network access for fonts. App lettering is visibly rounder/wider; this changes wrapping, chip widths, and some help-card heights. Those supplied assets cannot establish a product font regression. This also means the implementer cannot honestly claim literal pixel identity from them.
- Small motion-phase differences in spinners/progress fills are not scored. The hero lens finding below concerns the thickness/shape of its colored arcs, not their rotation phase or a slight gradient shade.

## 1. Every round-1 major

Each row names the original issue. Evidence always refers to the supplied second-round pair, not the implementer narrative.

| # | State | Round-1 major | Status | Evidence |
|---:|---|---|---|---|
| 1 | frame-tools | Bàn dựng selected while in Settings | still wrong | `frame-tools`: Bàn dựng remains filled/bold; gear is selected too. Other tool captures have a corrected header, but this named shot does not. |
| 2 | frame-connect | Bàn dựng selected while in Settings | fixed | `frame-connect`: outlined grouped navigation; no main tab is filled/bold. |
| 3 | frame-agents | Blank / unusable entire state | still wrong | `frame-agents`: shell and right rail now render, but the entire agent panel is replaced by a spinner; sidebar icons are also absent. Improvement from blank does not establish the completed state. |
| 4 | frame-accounts | Bàn dựng selected while in Settings | fixed | `frame-accounts`: no selected main tab. |
| 5 | frame-storage | Bàn dựng selected while in Settings | fixed | `frame-storage`: header no longer highlights Bàn dựng. This narrowly fixes the original selection issue; the new wrong loading-page capture is a separate major below. |
| 6 | frame-logs | Bàn dựng selected while in Settings | fixed | `frame-logs`: header no longer highlights Bàn dựng. The absent Settings/logs page is separately a new major below. |
| 7 | tools-ok | Bàn dựng selected while in Settings | fixed | `tools-ok`: centered enclosed nav; no selected main destination. |
| 8 | tools-ok | Advanced outside list card | still wrong | `tools-ok`: list ends at y≈675; Advanced starts at y≈691 with a black 16px gap. Reference has a continuous last row ending at y≈718. |
| 9 | tools-update | Bàn dựng selected while in Settings | fixed | `tools-update`: no selected main destination. |
| 10 | tools-update | Advanced outside list card | still wrong | `tools-update`: list ends at y≈694, separate Advanced starts at y≈710; reference divider directly joins the final row. |
| 11 | tools-failed | Bàn dựng selected while in Settings | fixed | `tools-failed`: no selected main destination. |
| 12 | tools-failed | Advanced outside list card | still wrong | `tools-failed`: the same visible black gap and detached disclosure as tools-update. |
| 13 | tools-failed | Failure summary omits the affected step | fixed | `tools-failed`: summary now explicitly says “video mới chờ ở bước Phụ đề.” |
| 14 | tools-installing | Bàn dựng selected while in Settings | fixed | `tools-installing`: no selected main destination. |
| 15 | tools-installing | Advanced outside list card | still wrong | `tools-installing`: six-row list ends at y≈710, separate disclosure begins at y≈726; reference joins it at y≈710. |
| 16 | setup-running | Stacked/centered hero | fixed | `setup-running`: lens left of left-aligned heading/paragraph. |
| 17 | setup-running | Progress and step list merged | fixed | `setup-running`: independent progress card y≈230–326 and step card beginning y≈344. |
| 18 | setup-running | Login action replaced by spinner | fixed | `setup-running`: outlined “Đăng nhập ngay” is visible at right of last row. |
| 19 | setup-running | Below-list guidance clipped | fixed | `setup-running`: full two-line information footnote visible around y≈797–821. |
| 20 | setup-login | Stacked/centered hero | fixed | `setup-login`: lens left, heading/paragraph left-aligned. |
| 21 | setup-login | Progress and step list merged | fixed | `setup-login`: separate progress/list cards with visible gap. |
| 22 | setup-login | Login action replaced by spinner | fixed | `setup-login`: bright “Đăng nhập ChatGPT” button, not a spinner. |
| 23 | setup-login | Below-list guidance clipped | fixed | `setup-login`: complete footnote remains within 900px. |
| 24 | setup-failed | Stacked/centered hero | fixed | `setup-failed`: lens left, heading/paragraph left-aligned. |
| 25 | setup-failed | Progress and step list merged | fixed | `setup-failed`: separate progress/list cards. |
| 26 | setup-failed | Login action replaced by spinner | fixed | `setup-failed`: bright “Đăng nhập ChatGPT” button visible. |
| 27 | setup-failed | Below-list guidance clipped | fixed | `setup-failed`: full footnote visible through y≈840 despite the extra row progress bar noted below. |
| 28 | setup-failed | Generic failed-step reason | fixed | `setup-failed`: “Tải quá chậm nên bị ngắt. Đã giữ 845,2 MB, thử lại sẽ tải tiếp.” gives a specific reason and retained-download explanation. Different real error/bytes are data, not defects. |
| 29 | msettings-tools | Missing introduction | fixed | `msettings-tools`: both lines of the required introduction appear below the header. |
| 30 | msettings-tools | Update button dark/outlined | fixed | `msettings-tools`: bright full-width update action with download icon; reported 44px size agrees with visible phone proportions. |

**Round-1 major reconciliation: 24 fixed · 6 still wrong · 0 not verifiable for those narrowly stated issues.** This does not certify all content in frame-storage/logs: their intended pages are not present at all. There are two additional major capture failures below.

## 2. Remaining and new visible differences

### frame-tools

This named app shot is visibly older/different from tools-ok/update/failed/installing; the latter cannot substitute for it.

- [major] frame-tools · current destination: artboard has only Settings selected, with no active main destination; app still fills and bolds “Bàn dựng” while also selecting the gear.
- [minor] frame-tools · main-navigation enclosure: artboard wraps the five destinations in one rounded, outlined dark group; app still shows free-standing destinations.
- [minor] frame-tools · main-navigation horizontal alignment: artboard centers the enclosing group at x≈720; app group remains left of center, around x≈670 after CSS-size normalization.
- [minor] frame-tools · Duyệt icon: artboard uses the gallery/image glyph; app still uses a flag.
- [minor] frame-tools · sidebar boundary: artboard has a faint full-height separator at x≈248; app has no equivalent visible column divider.
- [minor] frame-tools · sidebar group rhythm: artboard leaves more space before “Dữ liệu” and “Ở trang khác”; app compresses these groups, leaving the final heading roughly 18px too high.
- [minor] frame-tools · header/rail vertical placement: artboard header ends around y=55 and help card starts around y=81; app header is approximately 48px and help starts around y=75. The newer tools-* shots correct this, but frame-tools does not.

### frame-connect

No remaining scored frame difference. The enclosing nav, neutral destinations, gallery icon, sidebar divider, group spacing, and header/rail placement are visibly corrected. Right-rail wording matches. Signed-in provider cards versus signed-out data are outside this frame review. Typeface-related wrapping remains an evidence limitation, not a scored defect.

### frame-agents

- [major] frame-agents · completed-state evidence: artboard shows the four-agent settings panel with model/thinking/image-source controls; app shows only a loading spinner under the introduction. The shell is no longer blank, but this is still not evidence of a finished agents state.
- [minor] frame-agents · sidebar icon column: artboard has icons for all six Settings rows plus the queue icon and trailing chevron; app leaves the corresponding icon slots blank, rendering labels only. This is visible even though the top-nav glyphs have loaded.

The recovered shell has corrected navigation selection/enclosure/alignment and the right help rail. Agent-panel fidelity is not verifiable from this capture.

### frame-accounts

No remaining scored frame difference. Header, selected sidebar row, divider, group spacing, footer anchoring, and right-rail copy match structurally. Account-card body and live platform state remain owned by Accounts QA.

### frame-storage

- [major] frame-storage · intended Settings frame: artboard shows the selected Lưu trữ sidebar, storage page title/content, footer, and right help rail; app shows three large skeleton cards and “Đang tải Bàn dựng”, with no Settings sidebar, page title, footer, or help rail. This is the wrong captured page/loading state, not a storage-data difference.

### frame-logs

- [major] frame-logs · entire logs page: artboard shows “Báo lỗi”, the export action/card, recent errors, version card, sidebar/footer and help rail; app instead shows three skeleton cards and “Đang tải Bàn dựng”. Neither the corrected export copy nor any logs-card layout claimed in Fidelity-Machine is visible/verifiable here.
- [minor] frame-logs · header icons: artboard has an icon before each main-navigation label and a visible gear in the Settings button; app shows text-only navigation and an empty rounded Settings button. The absent numerical badge/attention dot is not scored because it depends on data.

### tools-ok

- [major] tools-ok · Advanced containment: artboard makes “Nâng cao · thư mục dữ liệu và phiên bản” the attached final row of the same rounded list card; app still has a detached, separately outlined disclosure with a 16px dark gap. The six-row list also ends with exposed square bottom corners.
- [minor] tools-ok · installed transcription description: artboard says “Chép lời thoại thành chữ ngay trên máy · mô hình giọng nói đã tải”; app stops after “ngay trên máy”, omitting the installed-model confirmation.
- [minor] tools-ok · Chrome purpose copy: artboard says “Dùng Chrome có sẵn trên máy, chạy ngầm để xuất video và đo âm thanh”; app replaces the purpose with “Dùng Chrome có sẵn trên máy, không cần tải thêm” followed by the live version. Provided status/size is excluded; the removed purpose sentence is fixed editorial copy.

### tools-update

- [major] tools-update · Advanced containment: artboard attaches the disclosure to the bottom of the tool list; app separates it by approximately 16px and leaves the list bottom square.
- [minor] tools-update · installed transcription description: artboard includes “· mô hình giọng nói đã tải”; app omits that phrase.
- [minor] tools-update · Chrome purpose copy: artboard explains “chạy ngầm để xuất video và đo âm thanh”; app instead says “không cần tải thêm” plus the live version, removing the purpose.

The updated download glyph is now neutral, the desktop update action remains correctly outlined, and update helper wording matches after ignoring version values.

### tools-failed

- [major] tools-failed · Advanced containment: artboard has a continuous list/disclosure card; app still has a detached disclosure and visible black gap.
- [minor] tools-failed · sidebar failure tone: artboard marks “Công cụ trên máy” with an orange attention dot; app uses a pale yellow dot despite the orange “1 công cụ cần sửa” summary and failed transcription row. The failed state itself is matched here, so this is not a different data state.
- [minor] tools-failed · Chrome purpose copy: artboard includes “chạy ngầm để xuất video và đo âm thanh”; app replaces it with “không cần tải thêm” plus the live version.

The summary now names Phụ đề and the row explains the failure, resume action, and consequence for new videos. Different raw reason/resume bytes are not counted as a copy defect.

### tools-installing

- [major] tools-installing · Advanced containment: artboard attaches the disclosure immediately below ChatGPT; app inserts an approximately 16px black gap and a separately outlined disclosure.
- [minor] tools-installing · phase capitalization: artboard says “Đang tải mô hình nhận giọng nói”; app visibly begins the phase “đang tải mô hình nhận giọng nói”. Bytes and ETA are excluded.
- [minor] tools-installing · Chrome purpose copy: artboard explains background video export/audio measurement; app replaces it with the no-download explanation and live version.

The ordinary transcription description is now gray, and the extra ChatGPT login button is removed as requested. Chrome/drawing ready versus queued and sidebar 4/6 versus active-position 3/6 remain the same dataset/measure caveat recorded in round 1, not an additional scored numeric mismatch. They contradict the implementer claim that the recaptured sidebar reads 3/6.

### setup-running

- [minor] setup-running · hero lens shape: artboard shows thick rounded colored arcs around the red sphere; app shows much thinner, nearly hairline arcs. Both occupy the correct left-hand hero position, but their silhouettes/stroke weights do not match.
- [minor] setup-running · row agent identities: artboard has a colored orb dot immediately after each of the six step titles (blue/green/blue/green/gold/gold); app omits all six dots. Correct colors in the four chips above do not replace these row markers.
- [minor] setup-running · active-step phase copy: artboard starts “Đang tải mô hình nhận giọng nói”; app starts “đang tải mô hình nhận giọng nói”. Progress numbers are live data.

Card width, chip shape/left alignment/palette, full-width separators, Chrome provided explanation, blue active-row treatment and red-to-pink overall progress are corrected.

### setup-login

- [minor] setup-login · hero lens shape: artboard has thick rounded arcs; app has visibly thin arcs around the sphere.
- [minor] setup-login · row agent identities: artboard places the six colored agent dots after step names; app omits every row dot.

The rose ChatGPT focus row, rose flag, and bright primary login action are corrected; guidance remains visible.

### setup-failed

- [minor] setup-failed · hero lens shape: artboard has thick rounded arcs; app retains the thin-arc silhouette.
- [minor] setup-failed · row agent identities: artboard shows colored dots after the six step names; app omits them all.
- [minor] setup-failed · failed-step progress/density: artboard has only the failure explanation and retry action in its orange row, with no inner progress track; app adds an orange progress bar below the error. The row becomes about 12px taller, shifting later steps and the footnote down by about 12px. The overall orange progress card above is present in both and is not the extra bar at issue.

Failure colors and row emphasis are corrected. The different timeout/network reason is live error content; it now meets the original requirement to tell users why installation stopped.

### msettings-list

- [minor] msettings-list · back control/title inset and touch target: artboard header uses a 44×44 back-button slot at x=6, placing the title at x≈52; app places the title at x≈40 with a smaller back-control slot. The implementer explicitly reports a 32px back button, consistent with the visible 12px shift, versus the reference `.ib` 44px and the documented ≥44px phone target. This is not the excluded status-bar offset.

The extra paragraph between summary and settings rows is gone. Both supplied reference and app have a left queue/routing icon; **the round-1 minor claiming the reference has no queue icon was mistaken and is withdrawn**, not carried forward. Failure summary, provider/platform counts, attention dots and long development-version wrapping are live-data differences.

### msettings-tools

- [minor] msettings-tools · back control/title inset and touch target: artboard title starts at x≈52 after a 44px back slot; app starts at x≈40 after the implementer-reported 32px slot. The button is smaller than the reference/documented phone target.
- [minor] msettings-tools · update explanation tone: artboard uses warm yellow for “YouTube hay đổi cách tải, nên cập nhật để tải không lỗi.”; app renders its update explanation in ordinary muted gray. The update pill is correctly yellow and the icon is now neutral.
- [minor] msettings-tools · update explanation copy: artboard explicitly ends “để tải không lỗi.”; app ends “nên cập nhật.” after its live version context, omitting that outcome.
- [minor] msettings-tools · transcription description: artboard uses “Chép lời thoại thành chữ ngay trên máy”; app adds the non-version phrase “· mô hình giọng nói đã tải”, importing the longer desktop wording.
- [minor] msettings-tools · Chrome description: artboard uses the compact “Dùng Chrome có sẵn trên máy, chạy ngầm”; app extends this to “để xuất video và đo âm thanh” before the live version, expanding the phone row.
- [minor] msettings-tools · drawing description: artboard uses “Vẽ ảnh cho từng cảnh, chạy tách riêng” (visually clipped in the supplied shot); app uses “Vẽ ảnh cho từng cảnh · chạy tách riêng, không đụng dữ liệu khác” before the live version. The added isolation explanation is fixed desktop copy, not live data.
- [minor] msettings-tools · description density/clamping: artboard constrains the row hint to two lines and keeps a compact six-row list; app allows three-line hints (notably Chrome and drawing), stretching the list and pushing the details card to roughly y≈758 instead of reference y≈635 after removing the 44px device-status offset. Version lengths account for some wrapping, but the lost two-line clamp is independently visible. This bullet counts the different text-layout rule, not each live version value.

The introduction, neutral icons, bright 44px update action with download glyph, phone-specific details label, leading info icon and trailing downward chevron are present. All six rows and the details disclosure are now visible in the phone shot. The signed-in ChatGPT variant and expanded details content still are not represented.

## 3. Missing or unsuitable state pairs

**No filename pair is missing:** all 15 states have both app and artboard PNGs and were read. The following supplied app evidence is unsuitable for the claimed completed state:

| State / variant | Evidence limitation |
|---|---|
| frame-storage | Wrong loading page: “Đang tải Bàn dựng”, no Settings frame. Cannot assess storage sidebar/rail/footer from this shot. |
| frame-logs | Wrong loading page: “Đang tải Bàn dựng”. No logs export/recent-errors/version cards to inspect. Both empty and populated logs fidelity remain unverified here. |
| frame-agents | Shell recovered, but spinner instead of agent panel; sidebar icons not loaded. Not a settled-state screenshot. |
| frame-tools | Pair exists, but app is visibly the pre-fix header/body variant, unlike newer tools-* shots. The named frame evidence does not establish the claimed fix. |
| msettings-list | Error-attention machine versus reference ready machine. Shared structure is comparable; ready-summary data variant is not certified. |
| tools-ok/update/failed and msettings-tools | ChatGPT is signed out in app versus signed in in reference. Signed-in row/pill/copy is not certified; no defect counted for those live differences. |
| tools-installing | Different installed/queued subset and counting measures; shared installing-row structure is comparable, exact reference state coverage is not certified. |
| expanded Advanced/details | No expanded pair was supplied; monospace contents/paths/versions are not visually certified by a closed disclosure. |

No browser was necessary: the problems above are unambiguous in the existing pairs, and only artboards—not the app—are available for browser rendering. Re-rendering a reference cannot repair a missing settled app capture.

## Accepted pending deviations and weak-excuse audit

Entries **#18–#26 are excluded from defect counts exactly as assigned**, even though the document labels them Mở. They are not silently treated as unconditional design approval.

- **#18 lane count / #19 per-channel limits / #23 queue-dependent storage text:** honest pending-feature dependencies, but weak support for a blanket “Khớp” or “all fixed” claim. Not inventing data or a dead link is sensible; it does not visually restore omitted product capability. No defect is counted for these omissions.
- **#20 release date:** a concrete missing backend field is a reasonable pending-data limitation. The supplied logs shot is unusable regardless; that separate capture failure is not excused by a missing date.
- **#21 server directory chooser, #22 USB naming, #24 36px cleanup button, #25 account timestamps/counts:** scoped platform/data/component compromises; no Machine defect counted. This report does not certify their unpictured interactions.
- **#26 AI data/editorial copy:** unavailable plan/spend telemetry is a concrete limitation. The argument that model editorial guidance cannot exist because the wire has no field is weaker: it explains the present implementation, not why curated UI guidance is impossible. The accepted deviation still stays excluded. It cannot excuse an agents loading spinner or absent sidebar icons.
- **Existing #15 lens/orb justification, outside #18–#26:** it describes slight gradient variance from reusing the kit. The observed setup difference is also arc stroke weight/silhouette, so “slight shade variance” is a weak explanation for the visible mismatch. It remains a minor, not a repeated agent-color identity major. Chip identities are now correct.
- **New implementer proposal about fixed-height Settings under an M9 banner:** none of the supplied paired shots demonstrates that scenario; no defect or approval is inferred.

## 4. Counts

| State | Majors remaining | Minors remaining |
|---|---:|---:|
| frame-tools | 1 | 6 |
| frame-connect | 0 | 0 |
| frame-agents | 1 | 1 |
| frame-accounts | 0 | 0 |
| frame-storage | 1 | 0 |
| frame-logs | 1 | 1 |
| tools-ok | 1 | 2 |
| tools-update | 1 | 2 |
| tools-failed | 1 | 2 |
| tools-installing | 1 | 2 |
| setup-running | 0 | 3 |
| setup-login | 0 | 2 |
| setup-failed | 0 | 3 |
| msettings-list | 0 | 1 |
| msettings-tools | 0 | 7 |
| **Total state-level occurrences** | **8** | **32** |

| Round-1 major disposition | Count |
|---|---:|
| fixed | 24 |
| still wrong | 6 |
| not verifiable, for the narrowly stated original issue | 0 |
| **Original majors reconciled** | **30** |
| Additional major capture failures in this evidence set | 2 |

Status: DONE
Summary: **8 majors remaining / 32 minors remaining** across 15 pairs; **24 of 30 round-1 majors fixed, 6 still wrong**. Worst three: **(1)** storage/logs captures show Bàn dựng loading instead of their Settings pages, **(2)** Advanced is still detached in all four tool-state captures despite the claimed containment fix, **(3)** agents remains a spinner/incomplete capture rather than a completed settings panel. Setup login/hierarchy majors are resolved.
