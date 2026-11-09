---
title: Viderse large YouTube source metadata repair
date: 2026-11-09
summary: "Deployed ef8bae5: remove callback-only line cap from raw metadata, actual existing source downloaded successfully and continued to transcript."
---

# Viderse large YouTube source metadata repair

User reported connected YouTube but source retry failed. Logs showed yt-dlp Broken pipe; real metadata measured11655746bytes on a single line exceeding the4MiB callback-line limit. ExecRunner applied this limit even with nil OnLine.

Fix buffers/splits lines only for actual callbacks; retain tail bounds, cancellation and callback oversize guard. Regression failed before and passed after; complete tools/toolsvc/download packages and targeted vet passed. Real source JSON traversed fixed ExecRunner and decoded183formats.

Merged PR30 ef8bae59cfa6f334b588d717d62059820ad0f2c1. Initial safe rollout blocked on a production lease. User explicitly approved pause-deploy-resume; paused same job through app API, confirmed all lease/receipt counts0, used guarded backup/restore/invariant comparison and deployed schema41. Installed SHA256090b640453189bf8a98f09174e244f1591588fcfcd12d32872303d91c73cff4b and public/native readiness verified.

Resumed same original video, no duplicate creation. Actual download attempt4 succeeded in43.5seconds with no error; pipeline advanced to transcript. This does not assert final video/export completion or full artboard parity. GitHub Actions disabled: no remote green-CI claim. Removed private metadata fixture and throwaway smoke/build scaffolds after verification.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
