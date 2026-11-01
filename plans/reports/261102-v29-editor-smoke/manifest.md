# V29 Editor acceptance — 38499e47

Status: **DONE_WITH_CONCERNS**. Required Editor behavior passed. Desktop Main artboard differences were compared and sent to the owning MMain agent; no mobile/Main screen was changed here.

## Real local fixture and boundary

- Worktree `/Users/dat/Projects/verse-v29`, existing branch `feat/viderse-v29`; main merged through `6ce28cc5` in `c8e87757`.
- Final source `38499e47`. Actual worktree-built serve binary `/tmp/bili-V29Finish2-viderse`; isolated HOME `/tmp/bili-data-V29Finish2`; PostgreSQL `bili_v29finish2_data` and `bili_v29finish2_identity` on `127.0.0.1:64832`.
- One managed browser tab, `http://v29.localhost:8054`, API8053 and canonical artboard9053. Tab closed and all four owned services stopped after acceptance. Temporary Vite hostname/proxy configuration removed. No production, real user HOME, embedded distribution, protected ports, or external credentials touched.
- Job `6eb429d9-fa97-4dc6-8123-80c4a67751b7`; project `clone-6eb429d9-fa97-4dc6-8123-80c4a67751b7-g4`.
- Prerequisite real media/transcript/scene fixture artifacts were seeded; actual images worker, build, publication, supported image use API, Editor GET/PUT If-Match persistence and event/list refresh ran normally. Initial custom motion/keyframes were saved through the normal supported project API. **No direct editor_resources pinning or build-status reset in this final generation.**
- An isolated deterministic OpenAI-compatible upstream on7896 replaced only the external image service and emitted real PNG files, converted/published as real WebP. Thus this proves the real local app pipeline and persistence, not external provider authentication or image aesthetics.

## Final actual UI acceptance

Seven read-only local-store/persisted-project snapshots are recorded in `evidence.json`:

1. Baseline uses version1/media `1ac4a636-9fd4-4d51-9407-e4f9a435ff11`.
2. Click version2 then **Dùng bản 2**: actual canvas and persisted timeline use `46bd85e2-d4d9-4f98-b243-0e1a9f40d389`.
3. Native **Command+Z** restores version1, including persisted project.
4. With the scene selected, use the actual opacity slider to edit92%→88%; enter feedback and click **Vẽ lại**. Real version5 arrives through events/list refresh, changes canvas/timeline to media `1e8af095-8413-4a37-8060-3b03af5fe194`, and saves.
5. Native **Command+Z** restores version1 while preserving the88% edit.
6. Repeat feedback/redraw: real version6 becomes media `5ebe10b6-cacb-4651-9f0f-ff8300889574` and saves.
7. Native **Command+Z** again restores version1; final used badge agrees and Vẽ lại remains available.

Assertions compare the **entire** local clip against the actual persisted clip in every state. All clip fields are unchanged except mediaId/src and the intentional opacity edit. Duration225frames/from0, transform x32/y18/2016×1134/rotation3/cornerRadius12/anchors/flips/aspect lock, complete float-drift modifier (seed42, phase7, amplitude.3, frequency.08, channel gains), and all four keyframes are exact. Keyframes: x0→32, x224→96; rotation0→3, rotation224→9; IDs/easings unchanged. All12 historical consumed media IDs still return200 after both redraws/undo operations. Build/images remain succeeded; no stuck “Video đang xử lý” toast.

Browser diagnostics contain18 aborted GET requests (`net::ERR_ABORTED`) during resource/navigation refresh; no different browser error type was observed. This is not a zero-network-errors claim. Quality measurement reports the isolated HOME lacks Chrome for Testing; successful image binding is retained and Editor redraw/undo remain functional despite that independent measurement failure.

## Screenshot manifest

All final captures are1800×1250 native PNGs from1440×1000 CSS viewport, DPR1.25.

| File | Evidence |
|---|---|
| `05-final-before.png` | Final-source generation4 baseline version1 |
| `06-final-swap.png` | Version2 visible on canvas, timeline, and inspector |
| `07-final-command-z-edit.png` | Command+Z restored image; intentional88% opacity; four keyframes visible |
| `08-final-redraw-1.png` | Real version5 arrival and successful use toast |
| `09-final-redrawing-2.png` | Repeated redraw submitted while editing |
| `10-final-redraw-2.png` | Real version6 arrival and saved project |
| `11-final-undo-used-badge.png` | Second undo restored version1 and matching used badge |
| `12-final-undo-keyframes.png` | Final88% opacity and four preserved keyframes |
| `13-desktop-main.png` | Actual desktop Main live-studio comparison |
| `14-canonical-main.png` | Canonical Main.dc.html selected drawing job at same viewport |

Historical diagnostic captures `01-before.webp`, `02-swap.png`, `03-command-z.png`, and `04-redraw-failure-before-fix.png` are from the earlier generation3 investigation. They are **not final-source acceptance evidence**; the failure capture is superseded by05–12.

## Cause-aligned repairs and evidence

- `d2451c27`: streaming stage-copy while holding shared media lease, release before exclusive Editor import. Real PostgreSQL regression detected the original shared→exclusive deadlock before repair.
- `ec411a8e`: normal Editor save/commands atomically pin consumed resource metadata/sources with project CAS; historical media survives subsequent build alias changes.
- `5981cad9`: persisted redraw `project:false` gives Editor sole ownership of its undoable workspace save; absent flag preserves regular Review/Main backend apply. Distinct image swaps create distinct undo entries.
- `f7ec918b`: live image selection does not invalidate downstream build; own images lease remains writable after completion. Undo refreshes backend used-version badge.
- `38499e47`: explicit rebind/review owner writes ignore only unrelated global job-fence advancement; own step fence/attempt/status and divergent checkpoint CAS remain strict. Concurrent identical build rebind is idempotent only with nonempty, exact checkpoint/artifact metadata after ownership validation. Real Service/media/PostgreSQL test covers concurrency plus foreign paths/steps, empty/extra/changed artifacts, divergent writes and stale own-step tokens.
- Original accepted rebind/measure, old wrapper stripping/user redraw direction, and Editor design edits were committed first (`ada63095`, `e7608749`); wrapper formatting `b4e2626d` retained. No suite was weakened.

Final coherent gates on38499e47: required real PostgreSQL `go test -count=1` passed clone/images0.648s, pipeline/steps/images2.842s, persistence/postgres143.851s, editor45.467s; clone/jobedit has no test files. Web focused scene-images11 + design-rules17 =28passed; complete web typecheck passed. Actual serve binary build passed.

## Desktop Main comparison / remaining owner concern

The drawing tile already uses canonical `--vv-draw-tile-bg`, compact DevelopEffect, artist Orb18 and shimmer. Exact differences sent to **MMain**: current `drawing = !image && status==='running'` hides the drawing effect during redraw when an older version exists; pane lacks canonical2px progress bar/orb/title “Phòng vẽ trực tiếp”/Hoạ sĩ xongN/M metadata; tile lacks overlaid key/label, approved badge, caption/time metadata. Screenshots13/14 document the comparison. This is an owning-screen design concern, not an unpassed Editor acceptance criterion or a claim of Main visual parity.

Temporary fixture DB/HOME and isolated upstream setup are retained for controller inspection; services are stopped. No production deployment or push performed.
