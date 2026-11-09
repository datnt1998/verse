# Editor object properties — 2026-11-09

**State: deployed to production `cbc06d67f11d39ba8cbd71c27c27fc5fb3b79a3b`; focused smoke passed, not full artboard parity acceptance.**

## Exercised behavior

| Object | Saved and rendered | Boundary checked |
|---|---|---|
| Scene | Position, rotation, grayscale effect; canvas move/resize/rotate | Undo/redo, Escape cancellation, reload; two undos restore resize/rotation to the initial transform |
| Shape | Seven-point star, green fill, rounded corners, 8px outline | Real command history and retained geometry |
| Path | Point position and Bézier handle; subdivision/deletion | Rapid delete → undo restores the fourth point; subdivision preserves the cubic outline |
| Styled text | Per-span content, 72px size, cyan color | Other span retained; color undo does not undo content/size |
| Caption | Global yellow text color | Text and word times retained; no duplicate content editor or unsupported per-span word-caption controls |
| Animation | Loop re-enabled, 1.5× speed, blue fill | Actual JSON animation imported through the real media service and included in scoped export |
| Composition instance | Title and pink color override | Source composition title remains unchanged |

22 distinct property/history controls and canvas move, resize and rotate gestures exercised; 0 unresponsive in that exercised set. This is not an exhaustive control count or a 1:1 artboard audit.

Real export: **30.000s, 1920×1080, H.264 30fps + AAC**. [Exported frame](editor-object-properties-qa.png) is **QA output, not a production screenshot**. No conclusion about a 57-minute export's throughput follows from this short clip.

## Regression evidence

- 29 frontend test files / 374 tests passed; TypeScript and production Vite build passed. Existing dependency annotation/chunk-size warnings remain.
- Six targeted Go packages passed: accounts, native command, domain, pipeline, download and web.
- Reproduced and fixed merged add/delete path history, with failing-before/passing-after regression.
- Reproduced and fixed export rejection when the real timeline serializer omits empty composition transitions/keyframes; malformed supplied values remain rejected.
- Waiting mobile cards do not render an empty progress rail. Video-source dialog exercised at 1430×750, 1440×900 including enlarged root font, and 390×844; desktop template menu stays above the footer and mobile keeps the sheet.
- Visible computed UI monospace count was 0 on the exercised shell/editor surfaces. Video-content font capability is retained by decision #97.
- Production's stored download error classified as `source_auth`; subline becomes “Chờ xác thực YouTube”, not image quota. Retry timing and real quota semantics are unchanged.
- Complete Viderse frontend suite: 449 files, 3,744 passed / 4 skipped. Existing React `act` and localhost network diagnostics remain; the suite exited successfully.
- GitHub Actions repository permission reports `enabled=false`; dispatched runs have no jobs. They are **not** green CI evidence. Local Docker did not answer within 60 seconds, so mandatory Postgres/Garage CI integration coverage was not exercised locally.
- Full local Go suite passed all other packages; the heartbeat-only render test had competing progress/heartbeat deadlines. After isolating that scenario without changing production timeouts, 20 repetitions and the complete render package passed. `go vet ./...` and Viderse `go tool sqlc diff` passed.

## Isolation and remaining production checks

The initial property/export QA used the existing isolated QA database plus private local blob overlay. Original sample project revision stayed unchanged. Production verification then used the real project duplicate API: corner radius 48, green fill and rotation 12° survived save/reload; undo restored rotation 0° while retaining fill, redo restored 12°. The source template revision stayed `3`; the private production copy was deleted through the real API (204). No active production job was edited or retried.

At PR#28 verification, login Keychain was locked and the protected log proved macOS denied Chrome Local State access; no password collection or TCC bypass was attempted. The user subsequently reported YouTube connected. The remaining source-download failure was a separate 4MB process-line cap, repaired and verified on the actual resumed production job in [the follow-up report](source-download-metadata.md). The earlier blocked observation is historical, not a claim that the user's current connection is still broken.

The 517-row / 40-probe production matrix remains pending. No screen group or physical Safari gate is closed by this QA report.

## Production release / focused smoke

- URL: https://vid.aarverse.studio; [merged PR #28](https://github.com/datnt1998/verse/pull/28).
- Installed commit marker and binary checksum match `cbc06d67f11d39ba8cbd71c27c27fc5fb3b79a3b`, SHA-256 `04e7b14b14a270a36989ce64c661839042d6849a02ad6df5d2b586ee95d22b13`; native 8789 and public `/readyz` returned `{"ok":true}`.
- Guarded release confirmed idle leases/receipts, backed up and restored the database, compared invariants, and reopened schema41. Backup: `~/server/apps/viderse/releases/cbc06d67f11d39ba8cbd71c27c27fc5fb3b79a3b/rollout-20261108T205115Z`. Writers have reopened: no blind binary/database rollback.
- Desktop source field is visible; template popover frontmost with bottom 562.94px < viewport 900px after fonts loaded. Mobile sheet opens, selecting Mẫu Joker returns to composer; no job was created.
- Real mobile waiting card shows “Chờ xác thực YouTube”, zero progressbar children and no empty progress rail. Visible computed UI monospace count on production editor: 0.
- Screenshots: [editor properties](editor-properties-production.png), [desktop source dialog](source-dialog-production.png), [mobile waiting card](mobile-wait-production.png), [mobile template sheet](mobile-template-production.png). These are production screenshots, **not** complete artboard pairs. Editor remains desktop ≥1024px, as `Editor.dc.html`'s `state=narrow` specifies; no mobile editor feature claim.
- Focused production controls: **12 exercised / 0 unresponsive**: desktop composer entry + template picker (2); editor shape selection, advanced disclosure, corner radius, fill, rotation, undo and redo (7); mobile composer entry, template sheet and template option (3). Desktop Escape dismissal also passed. This is a partial smoke, not the full screen control ledger.
