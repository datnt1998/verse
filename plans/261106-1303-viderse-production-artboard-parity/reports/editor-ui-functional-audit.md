# Editor UI / functional audit — 2026-11-09

Status: implementation and functional verification in progress; no new production release claimed.

## Scope / fixed brief

- Register: Product. Scene: existing Viderse editor, real project content, mouse/keyboard desktop; narrow viewport retains the canonical handoff rather than claiming a mobile editor.
- Direction: preserve `Editor.dc.html` and the existing Viderse palette/layout; restore broken primitives and real editing behaviors, not redesign the product.
- Color: restrained, existing `--color-bg`, `--color-fg`, accent and surface/border tokens only. Type: existing Be Vietnam Pro UI family. Signature: contextual inspector and red selection/playhead. Dials: low visual variance, state feedback only, existing dense editor.
- User explicitly added panel/component resize and a thorough functional review. This is not merely the Karaoke popup repair.
- Parent integration worktree owns theme and primitive audit; isolated panel/layout and canvas/timeline slices have disjoint source ownership. Original dirty working tree/index are untouched.

## Root evidence

| Finding | Evidence | Change / check |
|---|---|---|
| Transparent menu / white default borders | Actual production select computed background `rgba(0,0,0,0)`; `--popover`, `--input`, `--border`, `--primary` empty; no compiled `bg-popover`/`border-input` rules | Restore lazy Tailwind semantic aliases to existing brand tokens; every opened overlay opaque, frontmost and bounded |
| Resting controls falsely accented after restoring focus color | Existing dense fields used `border-ring/50` for their resting border | Use neutral input token at rest, keep accent focus/selection feedback |
| Caption effect selector has no accessible name | `aria-label` passed to EditorSelect but that component only forwards `label` | Pass the supported label; real menu keyboard/select/dismiss behavior |
| Active shell lacks resize controls | Actual production editor has zero separator handles; v13 uses fixed widths/heights | Real constrained draggable/keyboard panel boundaries, safe persistence/reload |

Evidence: [production before](editor-ui-evidence/select-production-before.png), [isolated after](editor-ui-evidence/select-isolated-after.png), [decoded modified caption](editor-ui-evidence/rendered-caption-frame.png), [decoded resized group](editor-ui-evidence/rendered-group-frame.png). Isolated screenshots and rendered frames do not certify full artboard acceptance.

## DONE contract / required verification

1. All active inspector primitives have resolved semantic colors: menus/popovers/dialogs opaque, neutral default borders, visible enabled/disabled/focus states. No underlying text/slider bleed-through; no clipping at fonts-ready and enlarged text.
2. Caption effect, font/family, number, color, switch, keyframe and effect menus: real click/keyboard changes, dismiss with Escape/outside click, focus returned where appropriate. Inspect actual saved data, not just labels.
3. Left content/preview, preview/inspector and preview/timeline boundaries resize with pointer and keyboard, stay within constraints at 1024/1280/1440/1920 widths, restore after reload; storage errors cannot break editor. Existing narrow surface verified at 768×1024 and 390×844 without claiming full mobile editing.
4. Canvas move/scale/rotate work at varied zoom, rotation and keyframes; preserve unaffected properties, one coherent undo per gesture, Escape cancels. Expression/link ownership must be explicit, not silently corrupt base values. Unsupported rotated/opposite-corner scale snapping must not alter geometry or show false guides; ordinary snapping remains active.
5. Timeline selection, seek/playback, zoom/fit, drag/move, trim, split/delete, track state and history are exercised against the real engine with boundary checks; no decorative no-op handlers.
6. Property edits survive actual save/reload, source project revision unchanged, undo/redo coherent. Real export smoke includes modified content; do not substitute compile/unit tests for runtime proof.
7. Review final screenshots and console/network failures, run focused meaningful regressions then typecheck/build; broaden suite for shared primitive/engine changes. Only report checks actually exercised.
8. Deploy only the verified main artifact through idle-writer/backup/restore guard; verify installed hash/readiness and actual production controls afterward. Never pause/retry a user's active job without approval. Full 517-row/40-probe artboard matrix remains separately pending.

## Coverage ledger

Current isolated preview uses a real duplicated template; only its project PUT is adapted through the loopback test proxy. Initial preview saves were correctly rejected by production CSRF (403), **not** counted as an app bug or successful save. The scoped adapter removes browser-origin metadata only for this private project's PUT; production auth/CSRF are unchanged. No original project was opened in the autosaving editor.

| Path | Actual observed verification |
|---|---|
| Caption selector | Background now `rgb(30,30,33)`, neutral border; selected Từng từ persisted, undo restored Karaoke, Escape closed and returned focus |
| Enlarged text | Root text24px at1440×900: popup222.67×350 inside viewport, zero clipped option spans, opaque; reset text size afterward |
| Font | Searchable font popup opaque; searched/selected real available Roboto; actual saved subtitle fontFamily=Roboto |
| Typography | Saved size72 and green#33cc88; existing word content/timings unchanged. Stroke toggle off and undo restored stroke without reverting size/color |
| File import | Imported two authentic cues from a completed video's caption source through the real `.srt` file input; actual private project saved2cues/8words |
| Cue drag | Pointer moved first cue9frames:0–1.5s →0.3–1.8s; second cue and its words unchanged |
| Cue trim/history | Pointer trimmed first cue start3frames to0.4s; measured words retained, neighbor unchanged; two undos restored the exact imported cues and words |
| Panel drag | Library316→380px, inspector316→380px, timeline286→366px; preview fitted512×482px; reload retained those sizes |
| Panel keyboard/reset | ArrowLeft changed library380→372px; Home restored default316px; End clamped480px. Visible layout-reset button restored library316/preview740/inspector316/timeline286. Unreliable double-click promise removed; Home and visible reset are verified |
| Playback/seek | Real playback advanced to frame2; pause worked; end sought11:29 in12s timeline; start restored0 |
| Preview audio controls | Keyboard volume Home reached0, End restored1; mute/unmute toggled actual pressed state. No claim of acoustic measurement from a headless browser |
| Effects | Added actual GPU grayscale; disabled it and set amount0.4; saved canonical data confirmed enabled=false/amount0.4. Delete and undo restored that same effect; removed it before render proof |
| Timeline editing | Actual image split at frame2 produced contiguous2/178-frame pieces; delete removed the selected178-frame piece, two undos restored the original. Zoom keyboard End reached2; Fit restored1.06 |
| Track state | Snow visibility changed tofalse in saved track data; undo restored visibility. Audio mute later verified pressed=true and canonical saved muted=true, then undo restored it |
| Single canvas | Actual subtitle southeast resize1560×160→1053×108 preserved ratio; undo restored. Move changed x0/y324→81/378; undo restored. Escape during another drag restored the exact starting transform |
| Viewports | Actual widths1024/1280/1440/1920: no document overflow.768×1024 and390×844 retain the explicit ≥1024px editing handoff, no document overflow; not full mobile editing parity |
| Real render | Actual live sequence, first180frames, worker renderer:3.18s,2,021,638bytes. ffprobe confirms1920×1080 H.26430fps/6s and6.016s PCM audio; decoded frame0.5s shows modified green Roboto caption, waveform and real background. This is engine render smoke, not a clone-bound publish/quality-gate acceptance |
| Advanced image properties | Set actual scene opacity65%; canonical save stores0.65. Reload and reopen advanced inspector shows65; restored100 afterward. Original source-template revision remains3 |
| Rotation/keyframes | Actual caption rotation gesture produced30°, undo restored. Added width keyframe through Tỉ lệ X; resize updated keyed width1560→1053 while base width stayed1560, then two undos removed gesture and added keyframe |
| Linked group | Real Subscribe selection retains3ids. Pointer uniform resize changes backdrop/icon/text together, including font35→57.006; one undo restores35, redo restores57.006. Escape clears all previews and retains selection. A second visible-frame resize atframe150 produced font48.0696; decoded exportframe5s preserves icon/text spacing and scaled backdrop |
| Animation semantics | Existing animated lanes continue editing at the current frame, as the canonical auto-key helper specifies; this is not a whole-animation rewrite. Resizing an invisible entry key atframe120 does not promise the same layout at later keyed positions. Visible-frame proof is atframe150 |
| Regression run | Full frontend suite451files:3859passed/4skipped after group fixes. Integration found and repaired real fontSize resolver misuse and off-center-anchor rotation drift; floating geometry checks use numerical tolerance rather than exact IEEE equality |
| Review | Theme/layout review found no actionable defect. Canvas/timeline review found trailing-edge snapping discarded and selected trim-neighbor preview exclusions; both corrected using canonical snapping/trim contracts. Two meaningful consumer regressions added; final focused timeline run20passed |

## Release verification

- Final typecheck and production build passed after timeline review corrections; embedded assets regenerated. Existing Rollup annotation, mixed dynamic/static import and >500kB chunk warnings remain visible. Real browser trailing-edge snap: first cue moved15requested frames and snapped17frames so its end meets the next cue; undo restored the original.
- Deployed main`7f90abe51faf55aeb6c2b532e6487dbe1d4ead94` through the guarded Mac mini script. Installed SHA256`127a27e0438e15cd4885ee450eb0d2e466b05db9b4acfffd1de713a7069e5eb8` matches the tested native artifact; PR source tree equals merged main tree. Restored rehearsal DB and backup kept under release rollout`20261109T174219Z`; schema41unchanged. Native/public`/readyz` both returned`{"ok":true}` after transient startup retries. No user job paused/retried.
- Full artboard matrix, physical Safari and full mobile editing are not implied by this scoped desktop editor repair.
- Actual authenticated production smoke: opaque selector`rgb(30,30,33)`, selected Từng từ saved as`word-by-word`, panel keyboard316→324px, reset, linked3-member resize saved matching width/font factor1.16952, undo.390×844 has no horizontal overflow and retains the desktop-editing handoff. Screenshots: [menu](editor-ui-evidence/production-menu.png), [group resize](editor-ui-evidence/production-group-resize.png).
- Production smoke exercised10distinct controls: caption clip selection, Nâng cao disclosure, caption selector, Từng từ option, content-width separator, reset button, group clip selection, seek ruler, southeast resize handle, Undo. Zero non-responsive controls **within those10**, not an exhaustive whole-editor certification.
- Source template revision stayed3. Private QA project deleted through actual API(204); loopback adapter removed, owned preview service and browser stopped, private SRT excerpt removed. No provider request/new user job was created.
- Remaining visual limits: dense/off-canvas transform handles can overlap/clip within the existing frame model; numeric advanced inspector remains available. Full artboard1:1 certification and whole-editor control coverage remain open; this release is not labelled complete parity.
