# Source download metadata — 2026-11-09

State: **deployed and actual source download succeeded** on `ef8bae59cfa6f334b588d717d62059820ad0f2c1` ([PR #30](https://github.com/datnt1998/verse/pull/30)). Production installed commit/hash and native/public readiness verified.

- Reported symptom: YouTube is connected, but restarting the video reports “Tạm thời chưa tải được video nguồn.” Do not equate this generic message with failed account connection.
- Stored failed step was at metadata; yt-dlp log recorded `ERROR: [Errno 32] Broken pipe`. Diagnosis did not retry or edit the job. After the user explicitly approved pause → deploy → resume, the same job was paused/resumed through the app API; no duplicate video was created.
- Initial read-only metadata diagnosis returned valid JSON: **11,655,746 bytes, longest line 11,655,745 bytes, duration 1,823s, 183 formats**. That diagnostic did not create a video, call AI or download the full media; the authorized resumed production job downloaded it afterward. SSH's separate Keychain context emitted decryption warnings; metadata success alone was not treated as proof of authenticated cookies.
- `tools.delivery.write` applied the 4,194,304-byte callback-line limit even when no `OnLine` consumer was requested. It closed stdout on valid large `yt-dlp -J` JSON, producing Broken pipe; the download layer then mapped that to a generic source wait.
- Fix: split/buffer callback lines only when `OnLine` exists. Bounded stdout/stderr tails and process cancellation stay unchanged; actual callback consumers retain the existing line-size guard. No larger global limit or retry-policy change.
- Regression failed before the fix with `4194355 > 4194304`; it passes after the fix. Existing oversized callback-line rejection also passes. Tools/toolsvc/download packages and targeted vet passed.
- Actual runtime smoke passed the real 11,655,746-byte source JSON through `ExecRunner` and decoded the source ID, duration and all 183 formats. After deployment, the actual original job's download step succeeded on attempt4, from `12:08:33.666559+07:00` to `12:09:17.130933+07:00` (~43.5s), with no error. Pipeline then advanced to `transcript`. This proves source download, not final video/render completion.

## Release gate

- Merged artifact tree matched the isolated build sources; native artifact staged on Mac mini with SHA-256 `090b640453189bf8a98f09174e244f1591588fcfcd12d32872303d91c73cff4b`.
- Initial guard blocked on the active job. After the user's approval, app pause returned200; all three lease/receipt counts reached0. Guarded deployment backed up/restored the database, compared invariants and reopened schema41; backup `~/server/apps/viderse/releases/ef8bae59cfa6f334b588d717d62059820ad0f2c1/rollout-20261109T050806Z`.
- Installed marker/hash matched; native8789 and public `/readyz` returned `{"ok":true}`. Resume returned200 for the same job, cleared pausedAt, and it completed download before entering transcript. No forced cutover or snapshot rollback.
