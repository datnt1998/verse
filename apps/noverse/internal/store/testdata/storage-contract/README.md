# storage-contract fixtures

Sanitized, synthetic fixtures that pin the on-disk layouts the file backend
must keep supporting while the Postgres backend is added. Every file is a
synthetic sample: no real novel text, no real account data, no secrets.
Passwords used by `accounts/` fixtures are `fixture-mat-khau-123` and
`fixture-mat-khau-456` (Argon2id, same parameters as `internal/webauth/password.go`).

- `manifest.json` — the path classification manifest (Step 1 of phase 01):
  every persistent path class is facts/blob/derived/scratch/local-only/excluded,
  plus the fixture file inventory with SHA-256.
- `library/` — library root with one project (`du-an`), one legacy story dir
  (`truyen-cu`, anonymous project `_`), and one adoptable candidate dir
  (`cu-dir`: `kich_ban/` without `project.json`).
- `library-custom/` — library root whose `.ainovel/library.json` overrides
  `dirs` (stories dir is `truyen`, not `kich_ban`).
- `library-renamed/` — library root where `project.json`'s name was renamed
  after the directory slug `du-an-cu` was created.
- `accounts/` — `web-accounts` file shapes: v3 (current), v2-legacy
  (pre-membership `books[]`/`writer`/`reader`), and a corrupt file to pin the
  absent-vs-corrupt distinction.

The story `du-an/kich_ban/01-dem-trang` intentionally mixes an accepted
chapter record (`meta/chapter_records/000001.json`) with workspace prose that
diverges (`drafts/02.draft.md` edited, not accepted) — revision scan relies on
this asymmetry. `meta/decisions.jsonl` ends with an uncommitted truncated
line (partial-write recovery case).
