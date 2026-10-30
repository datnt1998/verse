# QA3 — Storage

Independent live QA of the current Settings and MSettings Storage implementation.

**Verdict: not a clean final gate. One major validation race and eleven minor difference groups remain.** The previously reported major visual corrections are present. The background-task notice correctly says **“việc nền”**, not “video”.

## Evidence and method

- Built and ran my own Go binary against `viderse_smoke_storage` on `127.0.0.1:18782`, with an isolated HOME and administrator created through the real setup endpoints. After Main announced the 13:56:22 embedded-dist update, rebuilt the binary and restarted the service. All retained screenshots use that final build. Did not rebuild the web app or use implementer screenshots as evidence.
- Desktop: 1440×900. Phone: 390×844, mobile emulation and touch enabled. Compared the real app and the artboard in separate tabs of one isolated headless Chrome instance, with the specified keychain/profile arguments and `delete window.PublicKeyCredential` initialization. No external links were clicked and no external login was completed.
- **Harness deviation:** this worker did not have the prescribed browser JS-kernel tool/API. Used a temporary Node/CDP evaluation kernel against that single Chrome instance instead; did not call a nonexistent `browser.open`/`tab.run`. Restricted page requests to localhost/127.0.0.1. Dimensions and computed styles were measured through the live DOM. This is independent live-browser evidence, but not the exact requested browser-tool transport.
- Captured an unmocked fresh-database baseline first. For media/volume/task states unavailable on the test machine, edited real `/api/storage`, `/api/activity`, and `/api/storage/reclaim` response shapes and intercepted only the necessary API endpoints. Cleanup preview/run were also intercepted; the real preview response shape was separately obtained. No real project move, deletion, or cleanup run was executed.
- **Reference-font correction:** Google font requests are outside the allowed origins. Initially the artboard silently used fallback fonts. Before final comparison, supplied the same locally served Be Vietnam Pro/JetBrains font bytes to the reference document as data-font faces and recaptured every artboard. Confirmed loaded Be Vietnam Pro faces. Do not compare older QA screenshots that used fallback metrics against these final captures.
- Evidence directory: `/tmp/viderse-qa3/storage/shots/`. Pairs are `art-store-<state>.png` / `app-store-<state>.png`, and `art-m-<state>.png` / `app-m-<state>.png`. Each has a sibling JSON file with measured element bounds, font sizes, weights, colors, radii, and icon masks. `app-real-here.png` is the unmocked baseline. `app-store-lowspace-race.png` and `validation-race*.json` document the additional interaction defect.
- Cleanup completed: closed Chrome, stopped the service and evaluation kernel, dropped the smoke database, and removed the temporary binary/profile/HOME/scripts. Only `shots/` remains under the isolated QA directory. No product files were changed.

## 1. Round-2 major recheck

QA2 recorded **zero unaccepted remaining majors**. Its first section rechecked inherited round-1 majors, including the partially corrected background-task sentence. Those same entries were independently checked again below; this does not treat accepted deviations as open majors.

| Prior major item recorded in QA2 | Current status | Current evidence |
|---|---|---|
| Desktop project heading 15px instead of 14px | Fixed | Here/pick JSON: app heading 14px/600, matching desktop artboard. |
| Desktop compact actions too large | Fixed | Here and reclaim confirmation: 32px/r8/13px, matching their `ghost btn` artboard classes. |
| Sidebar footer was bottom-aligned incorrectly | Fixed | Here pair: all three footer lines remain visible at the artboard bottom region. |
| Low-space path colored orange | Fixed | Settled lowspace pair: path is neutral; failure/check copy carries warning color. |
| Picker omitted Macintosh HD and split cloud suggestions | Fixed | Pick/cloud pairs show all four destinations in the same suggestions region. |
| Cloud explanation incomplete | Fixed | Cloud pair includes Google Drive, iCloud, Dropbox and the in-use-file risk. |
| Background notice said video instead of background tasks | **Fixed** | Pick/lowspace/cloud DOM says “1 việc nền đang chạy sẽ tạm dừng trong lúc chuyển và tự chạy tiếp khi xong.” Moving/offline also use task language. The unrelated cleanup safety text still correctly refers to videos. |
| Validation warnings used red instead of amber | Fixed in settled state | Settled lowspace/cloud pairs show amber warnings. See new race R0 below: a fast selection can retain a different destination validation. |
| Success banner spacing | Fixed at major level | Done pair has the 12px location-to-banner separation. Icon/alignment residue is minor R5. |
| New-only copy lacked later merge guidance | Fixed | Donenew DOM includes “muốn gom về một chỗ thì bấm Đổi thư mục và chọn Chuyển.” |
| Moving cancel action undersized | Fixed | Desktop compact cancel follows the 32px class; phone cancel is 44px. |
| Destructive action used a modal | Fixed | Reclaim-ask pair shows the amber inline confirmation inside the source-media card, not a modal. |
| Cleanup run CTA was too large | Fixed | Preview pair: “Dọn 2,4 GB” is 32px/r8/13px, not 36px. |
| Phone title/back alignment | Fixed | Phone header title starts at x52; back control retains a 44px target. |
| Phone folder-name tone | Fixed | Phone base/completion/offline use foreground ink rather than a muted drive name. |
| Phone drive icon alignment | Fixed | Here/done/offline pairs show centered folder glyphs in their square tiles. |
| Phone change-folder target | Fixed | Here/done: 44px. |
| Phone destructive/retry targets | Fixed | Xoá, old-copy deletion, and retry are 44px. |
| Phone radio density | Fixed at major level | Four destinations fit in the sheet. Label-size/stroke residue is minor R8. |
| Cloud row used the wrong component | Fixed | Phone pick/lowspace: cloud is a disabled radio destination. |
| Sheet lacked bottom safe padding | Fixed | Phone picker footer keeps 32px bottom padding and the CTA inside the viewport. |
| Move-mode segmented control too short | Fixed | Phone picker segmented control is 48px. |
| Low-space sheet geometry wrong | Fixed | Phone lowspace uses the 80dvh sheet, matching the corresponding artboard state. |
| Phone picker primary action too short | Fixed at major level | 48px in pick/lowspace; remaining font/radius difference is minor R11. |
| Phone old-copy deletion too short | Fixed | Done: 44px, not 32px. |
| Missing “Giới hạn từng kênh” navigation | Accepted, not counted | Still absent; expressly covered by accepted deviation #26. |

### Settled button-rule spot checks

| Location | Artboard | App |
|---|---|---|
| Desktop project change / cleanup preview / reclaim confirmation | 32 / r8 / 13 | 32 / r8 / 13 |
| Desktop picker primary action | 36 / r10 / 13 | 36 / r10 / 13 |
| Phone retry / cancel / old-copy deletion | 44 | 44 |
| Phone picker primary action | 48 | 48 |

## 2. Remaining differences

Counts below are **distinct defect groups**, not the sum of repeated appearances across states. Related typography differences within one region are grouped together.

- [major] **Settings pick → lowspace; shared MSettings picker logic** · destination validation: artboard lowspace shows the selected Lexar destination with “Không đủ chỗ”, exFAT warning, and disabled primary action; app can show Lexar in the path/chip while retaining the Samsung T7 success checklist (“ổ còn 1,62 TB”, APFS) and an **enabled** “Chuyển sang thư mục mới”. **R0 / P2.** Reproduced by delaying the first `/api/storage/check` response 350ms, opening the picker, and selecting Lexar while that check is in flight. The request trace contains only the Samsung path; Lexar was never checked. The stale result and `disabled: false` remained after settlement, not just during one screenshot frame. Evidence: `app-store-lowspace-race.png`, `validation-race.json`, `validation-race-settled.json`; compare the correctly settled lowspace pair. Cause: `StorageSection.tsx:218` reruns `useAsyncAction` on destination/mode changes, but `platform/ui/src/useAsyncAction.ts:27-28` drops `run()` calls while busy; the old closure then writes its result into the new selection. Discrete fix: destination/mode-key validation must keep the latest request and discard obsolete results instead of using the click-deduplication action guard. No move was executed; this finding asserts false validation and enabled UI, not a successful invalid filesystem move.
- [minor] **Settings, all desktop states** · sidebar rhythm: artboard selected row begins at y281 in the here state; app begins at y279, with the category/row rhythm correspondingly slightly higher. Excludes the accepted missing navigation item. Evidence: here pair/JSON. **R1.**
- [minor] **Settings pick, lowspace, cloud, moving, cleanup preview** · global header/main vertical alignment: artboard header stays 55px; app shrinks it to 48px in the three picker states, 51.36px while moving, and 50.92px in cleanup preview. The page title/cards shift upward with it. This is real flex compression, not a live-data text difference. Evidence: corresponding `HEADER` bounds in the JSON files. **R2.**
- [minor] **Settings pick, lowspace, cloud** · existing-data radio cards: artboard cards are about 57px tall; app cards are about 55px, slightly compressing the mode block and following sections. Evidence: picker pairs and choice bounds. **R3.**
- [minor] **Settings cloud** · destination-path tone: artboard colors the rejected cloud path amber; app leaves that path neutral even though the warning text below is correctly amber. Evidence: cloud pair. Lowspace is correctly neutral and is not included. **R4.**
- [minor] **Settings done/donenew; MSettings done** · success glyph/alignment: artboard uses the filled mint tick-circle, vertically centered in the banner; app uses the outline tick-circle aligned at the top. Evidence: final done/donenew pairs and icon-mask metadata; this was checked again after icon assets settled. **R5.**
- [minor] **Settings offline; MSettings offline** · warning typography: artboard title is 14px/700 and body is 14px foreground copy; app title is 13px/500 and body is 12px/muted, reducing warning emphasis and changing wrapping. Retry target heights are now correct. Evidence: offline pairs/JSON. **R6.**
- [minor] **MSettings here, pick, lowspace, moving, done, offline** · project-card hierarchy: artboard project heading is 15px/600 and drive name 14.5px/500; app heading is 14px/600 while the drive name is 16px/600. This reverses the intended emphasis. Excludes path wrapping caused by the real storage-path difference and the accepted native status-bar omission. Evidence: phone here/pick/offline JSON. **R7.**
- [minor] **MSettings pick/lowspace** · destination choices: artboard destination labels are 15px; app labels are 13px. Radio diameter is now correctly 22px, but the app selected ring is heavier than the artboard thin ring. Evidence: phone picker pairs/JSON, e.g. Macintosh HD label at x55. **R8.**
- [minor] **MSettings pick/lowspace** · sheet introduction inset: artboard helper text has x16/358px content width; app helper spans x0/390px, reaching the sheet edge and wrapping differently. Both are 11.5px, so this is missing inset rather than a font-loading artifact. Evidence: corresponding helper bounds in picker JSON. **R9.**
- [minor] **Settings here, fresh real database** · always-local storage size: artboard uses a single unit in “(5,3 GB)”; unmocked app renders **“(0,2 MB GB)”**. `StorageSection.tsx:831-835` removes only a GB suffix from the adaptive formatter before passing the value into copy that adds GB again; sub-GB results retain MB. Evidence: `app-real-here.png`, with real storage data. **R10.**
- [minor] **MSettings pick/lowspace** · primary-action type/radius: artboard 48px CTA uses 15px/600 and r13; app correctly reaches 48px but uses 14px/600 and r12. Evidence: button bounds/styles in the phone picker JSON. **R11.**

### Accepted deviations reviewed, not counted

- #18–#19: do not count the new-only old-location action policy or the current phone destination/folder-selection policy as missing controls.
- #20: **“việc nền” is required and present**. Do not change it back to the artboard video wording just to match a screenshot; the current task data is not video-only. Do not invent a resumed-video count for completion.
- #21–#22: no finding for the absent synthetic “Xem khi xong ›” state switch, counterparty links, or template-row simulation.
- #23: native status/home-indicator chrome is not reproduced inside the app. The 44px reference status bar is excluded from vertical comparisons on phone.
- #24–#25: no Storage-specific defect is waived by those unrelated deviations.
- #26: the missing per-channel-limits sidebar item is excluded. **Caution:** this is a scope deferral, not visual parity; it would be a weak excuse for unrelated sidebar spacing such as R1, which is still counted.

## 3. Coverage and limitations

| Requested scope | States checked | Evidence |
|---|---|---|
| Settings Storage | here, pick, lowspace, cloud, moving, done, donenew, offline | All eight paired PNG/JSON sets. |
| Settings cleanup | idle, preview, done | Three paired sets; preview/run API outcomes are intercepted. |
| MSettings Storage | here, pick, lowspace, moving, done, offline | All six paired sets, mobile/touch emulation. |
| Additional checks | Inline source-media deletion confirmation; real empty-db here; in-flight selection race | Reclaim pair, unmocked baseline, race screenshot plus request/DOM evidence. |

**States not checked: none of the seventeen requested states.** Additional phone cloud/donenew states were not requested. Real external-volume transfer, physical unplug/replug, background-task pause/resume, and destructive cleanup/deletion were not exercised; their visual scenes used the documented intercepted state shapes. The shared picker race is directly reproduced on desktop and traced to shared code; no separate phone race reproduction is claimed.

## 4. Counts

| Measure | Count |
|---|---:|
| Requested states checked | 17 / 17 |
| Requested states not checked | 0 |
| Extra paired confirmation scene | 1 |
| Inherited major corrections checked and fixed at major level | 25 |
| Inherited accepted missing-navigation occurrences, excluded | 12 |
| **Remaining major defect groups** | **1** |
| **Remaining minor difference groups** | **11** |
| Remaining difference groups total | 12 |

Counts deliberately group repeated defects across states. They are not directly comparable with the repeated-occurrence count of 93 minors in QA2; font-correct reference captures also remove several earlier width/wrapping false differences.

Status: DONE
Summary: **1 major remaining, 11 minor groups remaining.** Worst three: **R0** stale destination validation keeps an invalid-selection action enabled; **R6** offline warning typography is too small/muted; **R8** phone destination labels are 13px instead of the artboard 15px. Background-task wording is corrected to **“việc nền”**. Do not treat this as a clean merge gate until R0 is addressed.
