---
title: Viderse editor object properties production release
date: 2026-11-08
summary: "Deploy cbc06d6: real object properties/export, workflow and typography repairs; macOS Chrome data permission and Keychain remain manual blockers."
---

# Viderse editor object properties production release

Implemented contextual transform/effects and object-specific shape/path, styled text, Lottie and composition controls. Verified real isolated 30-second customized export; fixed empty composition-array export schema and distinct topology undo history.

Merged PR28 and deployed cbc06d67f11d39ba8cbd71c27c27fc5fb3b79a3b with idle guard, backup/restore rehearsal and schema41 invariant comparison. Public/native readyz and installed SHA256 matched. Production private project copy preserved corner48/fill/rotation12 across reload, undo/redo, source revision3 unchanged; deleted copy via real API204. Desktop popover and mobile sheet/card checked.

Frontend3744 passed/4skipped. Full Go exposed a competing-deadline test; isolated heartbeat scenario then20 repeats +complete render package passed; vet/sqlc passed. GitHub Actions disabled, no remote CI success; Docker unavailable, mandatory integration CI not exercised.

Cookies remain BLOCKED: protected production platform log proves macOS denies Chrome Local State access; login Keychain also locked. User must grant app-data access and unlock Keychain locally. No password collection or TCC bypass. Full artboard matrix517/40 and physical Safari remain pending.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
