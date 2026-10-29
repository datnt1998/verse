# verse

Monorepo for the AI writing apps that share one agent core.

| Path | What it is |
|---|---|
| `apps/noverse` | Noverse: novel/story writing CLI, TUI and web app (binary `ainovel-cli`). See [apps/noverse/README.md](apps/noverse/README.md). |
| `core/` | Shared AI core: `agentcore` (agent runtime), `llm` (model interface and providers), `models`, `llmretry`, `llmcontract`, `auth` (OAuth and credentials), `roles` (role→model config). |
| `platform/` | App-agnostic infrastructure: `identity` (web accounts, sessions, pairing), `pgtest` (isolated Postgres schemas for tests). |
| `archtest/` | Import boundary test: `core/**` imports only the standard library, third-party modules and `core/**`; `platform/**` adds `platform/**`; an app never imports another app. |
| `plans/` | Monorepo-level plans, starting with [the move into this repository](plans/261029-1138-verse-monorepo/plan.md). |

One Go module, `github.com/datnt1998/verse`, rooted here. Each app keeps its own
web package, Dockerfile, migrations and docs inside its directory.

## Build and test

```sh
go build ./...
go test ./...                                   # Postgres/MinIO suites skip unless configured
(cd apps/noverse && go tool sqlc diff)          # generated SQL code is current
(cd apps/noverse/web && npm ci && npm test)     # web app
docker build -f apps/noverse/Dockerfile .       # hosted image; context is the repository root
```

Postgres and MinIO for the full suite: `apps/noverse/deploy/compose.test.yml`
(see `.github/workflows/ci.yml` for the exact environment).

## Origin

Imported on 2026-10-29 from `datnt1998/noverse` at tag `pre-verse` without
history; that repository is frozen and keeps the full history. The module path
changed from `github.com/datnt1998/noverse` to `github.com/datnt1998/verse`
(Noverse code under `github.com/datnt1998/verse/apps/noverse/...`), superseding
the "keep the module path" notes in older design documents. Runtime names are
unchanged: binary `ainovel-cli`, config dir `~/.ainovel`, cookie
`ainovel_session`, database roles `noverse_*`.

Releases (`v*` tags) are Noverse releases, published by GoReleaser from this
repository.

## License

Apache-2.0, see [LICENSE](LICENSE) and [third_party/NOTICE](third_party/NOTICE).
