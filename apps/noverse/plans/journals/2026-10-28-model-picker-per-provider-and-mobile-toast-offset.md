---
title: Model picker per provider and mobile toast offset
date: 2026-10-28
summary: Shipped per-provider model lists with picker; fixed key loss on global save and toast offset on mobile tabs
---

# Model picker per provider and mobile toast offset

## What shipped
- `POST /api/config/models/list` lists each provider's models from the provider API (OpenAI-compatible, OpenRouter, Anthropic, Gemini, Ollama, subscription accounts), with a 10-minute cache keyed by provider, type, base URL and key hash. Errors come back in the body as Vietnamese text. `PUT /api/config` returns 409 `model_in_use`.
- Provider cards get an in-card model picker. Trợ lý AI groups models by provider. Nâng cao no longer adds models by hand.
- Mobile toasts: `body:has(.mw-workspace)` matched the hidden Viết pane that MobilePanes keeps mounted, so toasts sat at 214px on Đọc, Nghe, Hoạt động and Thêm. The rule is now scoped to `.m-pane.on`.

## Surprises found in the real-server smoke
- `configForSave` pruned a provider whose only field was the hidden key. The card vanished after reload, and the next global save deleted the key. Fix: keep `{}` entries so the server re-attaches the saved key.
- The fresh provider card closed its key field on mount, because the effect keyed on `ctx.response` also ran at mount.
- OpenCode Go `/models` returns 403 to Python's default User-Agent but works with Go's client.

## Open
- OpenAI-compatible lists carry no context window, so models outside the Noverse catalog fall back to 200K.
- There is no mobile model picker yet, and no artboard for one.
- `docs/design/v1/11-api-contract.md` has the new endpoint documented, but the file also holds the user's uncommitted edits, so it was left uncommitted.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
