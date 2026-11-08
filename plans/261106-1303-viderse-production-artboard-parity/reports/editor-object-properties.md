# Editor object properties — 2026-11-09

**State: verified on isolated QA; not production parity acceptance.** Production release evidence is added only after cutover.

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
- Full local Go suite exposed competing progress/heartbeat deadlines in the heartbeat-only render test. Keep the progress timeout outside that scenario; production timeout behavior is unchanged.

## Isolation and remaining production checks

All project mutations, media import and exports used the existing isolated QA database plus private local blob overlay. Original sample project revision stayed unchanged; no production project or active job was edited/retried.

Mac mini login Keychain was reported locked; yt-dlp saw 518 Chrome Default cookies and could decrypt none. The user must unlock login Keychain / approve Chrome Safe Storage locally; no password was requested or transmitted. Separately, production profile API returned zero while the same current discovery code run read-only on Mac mini found three; verify the newly deployed process and its sanitized discovery diagnostics rather than claim that mismatch repaired without evidence.

The 517-row / 40-probe production matrix remains pending. No screen group or physical Safari gate is closed by this QA report.
