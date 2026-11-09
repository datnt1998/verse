# Source download metadata — 2026-11-09

State: source repair verified; production deployment recorded below only after guarded cutover.

- Reported symptom: YouTube is connected, but restarting the video reports “Tạm thời chưa tải được video nguồn.” Do not equate this generic message with failed account connection.
- Stored failed step is still at metadata; yt-dlp log records `ERROR: [Errno 32] Broken pipe`. No original job was retried or edited during diagnosis.
- A read-only metadata request for the actual public source returned valid JSON: **11,655,746 bytes, longest line 11,655,745 bytes, duration 1,823s, 183 formats**. No video creation, AI request or full video download was performed. SSH's separate Keychain context emitted decryption warnings; metadata success is not proof that authenticated cookies work there.
- `tools.delivery.write` applied the 4,194,304-byte callback-line limit even when no `OnLine` consumer was requested. It closed stdout on valid large `yt-dlp -J` JSON, producing Broken pipe; the download layer then mapped that to a generic source wait.
- Fix: split/buffer callback lines only when `OnLine` exists. Bounded stdout/stderr tails and process cancellation stay unchanged; actual callback consumers retain the existing line-size guard. No larger global limit or retry-policy change.
- Regression failed before the fix with `4194355 > 4194304`; it passes after the fix. Existing oversized callback-line rejection also passes. Tools/toolsvc/download packages and targeted vet passed.
- Actual runtime smoke passed the real 11,655,746-byte source JSON through `ExecRunner` and decoded the source ID, duration and all 183 formats. This proves the failing metadata boundary, not full video production completion.
