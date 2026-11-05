# Library / Template — native interaction smoke (pr-04)

Real-browser smoke against the live app on `http://127.0.0.1:8043` (schema40,
restored QA data: 3 templates, 3 owner channels, 9 image prompts, 0 backdrop
sources, 12 built-in overlays). Runner: standalone `@playwright/test` +
`/Applications/Google Chrome.app` headless. Controller:
`library-native-qa.cjs`; raw ledger: `library-native-qa-results.json`.

- **Mutation fence:** `context.route('**/api/**')` aborts every
  non-GET/HEAD/OPTIONS request. Result: **0 blocked mutations, 165 GET
  reads**. No POST/PATCH/DELETE/upload was ever attempted; every flow was
  opened and cancelled instead, and cancelling was verified against the
  response stream (no write request reached the server).
- **Contexts:** 4 separate browser contexts — 1440×900, 390×844
  (hasTouch+isMobile+CDP `pointer:coarse`), 1440×900 reduced-motion, 390×844
  audit. No shared page state.
- **Fonts/animation:** `document.fonts.ready` + finite `getAnimations()`
  awaited before every screenshot/measurement.

## Control → action → observed

### 1440×900 (desktop)
| Control | Action | Observed |
|---|---|---|
| Tabs Mẫu video/Chỉ dẫn ảnh/Video nền/Lớp phủ | click each | each panel mounts, `?tab=` URL updates, no horizontal overflow |
| Prompt filter Segmented (Tất cả 9/Ảnh cảnh 3/Ảnh bìa 6) | click Ảnh cảnh | `GET /api/image-prompts?kind=scene` fired, grid re-rendered |
| Search field | type "QA" | debounced `GET /api/image-prompts?q=QA`, grid filtered |
| Prompt card | open → "Về danh sách chỉ dẫn" | PromptEditor mounted inline, back returned to grid |
| Overlay tile | click play → click pause | `<video>` played then paused (real media), no API |
| Overlay add tile / remove | — | add tile present; **0 remove buttons** (all 12 overlays built-in — expected) |
| Backdrops add bar | inspect | submit disabled when empty, enabled after input; **submit not clicked** (POST `/api/sources/parse` — mutating, unverified) |
| Template ⋯ menu (in-use "Mẫu chuẩn · Tách nền") | open | popover 250×184 at (447,531), inside viewport |
| Menu → Xoá (locked) | click | `aria-disabled`, reason "Đang là mẫu video mặc định của Midnight Tales, Quiet Hours…"; click = no-op, menu stays open, no dialog |
| Menu | Esc / outside click | both close the popover |
| Menu → Đổi tên | open → Huỷ | Dialog "Đổi tên "…"" centred, cancelled, no PATCH |
| Menu → Đặt làm mặc định cho kênh | open → "Mở <kênh>" | navigates to `#/channels/<id>?tab=templates` (read-only handoff to Channel › Template) |
| Menu → Xoá (free "Kiểm thử QA…") | open → Huỷ | `alertdialog` "Xoá mẫu video "Kiểm thử QA · chọn prompt trong viewer"?" centred, cancelled, no DELETE |
| "Mẫu video mới" | open, fill name, Huỷ | dialog opened, cancelled — no POST /api/templates |
| "Nhập mẫu video" | open → Esc | import dialog rendered, closed; no file/upload |
| Template card link | click | `#/templates/ec7d7b44…` loaded with left nav |
| Template section nav (images/cover/backdrop/output) | click each | all 4 sections mount via `?section=` |
| Template header ⋯ | open → Esc | items [Xuất tệp, Nhân bản, Về Thư viện, Xoá] with delete `aria-disabled` + channel reason |
| Preview source select | open → Esc | **floating listbox** 300×34 inside viewport, 1 source option, no build started |
| Template nav back ("Tất cả mẫu video") | click | returned to `#/library` |

### 390×844 (mobile, coarse pointer emulated)
| Control | Action | Observed |
|---|---|---|
| Library tab strip (4 radios) | measure + tap each | see **Finding 1** — strip scrolls, 4th tab initially clipped; all taps navigate, tap auto-scrolls tab into view |
| "+" (Thêm vào thư viện) | open | bottom sheet 390×301 bottom-anchored, 4 actions with descriptions |
| Sheet → Chỉ dẫn ảnh mới | open → Esc | PromptEditor `height="92%"` sheet opened, closed, no write |
| Sheet → Mẫu video mới | open → Huỷ | naming Dialog opened, cancelled, no POST |
| Sheet → Nhập mẫu video | open → Esc | TemplateImportDialog opened, closed |
| Sheet → Kênh hoặc video nền | open → Esc | `height="auto"` link sheet 390×202 bottom-anchored (parent's auto-height fix verified) |
| Template card ⋯ (in-use) | open → Esc | ActionSheet rows; "Xoá mẫu video" `disabled`+`aria-disabled` with "Đang là mặc định của Midnight Tales, Quiet Hours · đổi ở Kênh trước" |
| Template ⋯ → Đổi tên | open → Huỷ | Dialog opened, cancelled, no PATCH |
| Template ⋯ → Đặt làm mặc định | open → Mở <kênh> | chooser rendered as centred dialog 358×347; navigated to `#/channels/<id>?tab=templates` |
| Template ⋯ → Xoá mẫu video (free) | open → Huỷ | ConfirmSheet `alertdialog` 390×640, cancelled, no DELETE |
| Template card thumb | tap "Mở <name>" | MTemplate list: 4 section rows + preview card + readiness card |
| MTemplate ⋯ | open → Esc | sheet: Đổi tên / Nhân bản / Xuất tệp mẫu video / Xoá mẫu video (disabled w/ reason) |
| MTemplate "Dựng lại bản xem thử" | open → Esc | source picker sheet opened (1 enabled source); cancelled — **no preview build POST** |
| MTemplate section row → back | tap Ảnh cảnh → "Quay lại" | child screen + return to list |

### Reduced motion (1440×900)
Same desktop flow — every overlay/dialog/menu still opens, positions inside
viewport, closes on Esc/outside/cancel. No behaviour regression.

## Geometry / overflow
- No horizontal document overflow on any surface (scrollWidth == innerWidth
  on every page at both widths; the one intentionally scrolled element is the
  mobile tab strip itself — Finding 1).
- Every overlay measured inside viewport: desktop menus ~250–300px anchored,
  centred dialogs; mobile bottom sheets full-width, bottom-anchored with grab
  handles (height-auto honoured on the link sheet and ActionSheets).

## Findings (real, evidenced)

1. **Mobile tab strip overflows; 4th tab "Lớp phủ" initially clipped.**
   Radios measure 108/119/106/98×40px in a strip of scrollWidth 447px vs
   client 356px — "Lớp phủ" sits at x=362 (only ~28px visible; see
   `native-390-mlibrary-templates.png`). Tapping reveals it (scrollLeft→88,
   `native-390-overlays-after-tab4.png`). Reference
   `MLibrary.tab.templates.png` shows all 4 tabs fully visible because
   artboard labels are shorter ("Template/Prompt ảnh" vs live "Mẫu video/Chỉ
   dẫn ảnh"). Functional but 3-tabs-look presentation; likely worth
   shrinking or equal-width fit.

2. **Touch targets <44px** (under `pointer:coarse` emulation):
   - Tab radios h=40px (Segmented `size="xl"`→40, matches kit token —
     borderline, likely intentional but below the 44px QA rule).
   - Prompt filter radios h=26px (`Segmented size={26}` in MLibrary prompts
     tab — well under 44).
   - Template card name links h=20–39px (`mlib.name` has no coarse-pointer
     min-height; desktop card link does have it). See
     `touch-audit-*` entries in `library-native-qa-results.json`.

3. **Reference/artboard copy drift (source gap, not product bug):** canonical
   reference PNGs and artboard HTML still say "Sửa template / Xoá template /
   Nhập template / Template mới / Prompt mới"; the app intentionally ships
   "…mẫu video / Chỉ dẫn ảnh mới" per `i18n/library.ts` + dev prompt.
   Wording in reference captures is stale relative to current copy.

## Unexercised (mutating — honestly unverified)
Rename/Create **save** (PATCH/POST), delete **confirm** (DELETE), Nhân bản,
Xuất tệp (GET download), import upload, overlay upload/remove, backdrop add
submit (POST `/api/sources/parse`), preview build (POST `/api/templates/:id/
preview`), prompt save/duplicate/delete + "Vẽ thử" trials, prompt import.
All opened where openable and cancelled cleanly; fences confirmed zero write
requests.

## Notes
- Native template names differ from fixture board (expected) — no placeholder
  photographs were produced as "previews"; prompt art tiles render real
  sample URLs only.
- `blockedMutations: []` — the fence was live the whole run and nothing
  attempted a write.
