# verse

Monorepo for the AI writing apps that share one agent core.

| Path | What it is |
|---|---|
| `apps/noverse` | Noverse: novel/story writing CLI, TUI and web app (binary `ainovel-cli`). See [apps/noverse/README.md](apps/noverse/README.md). |
| `apps/viderse` | Viderse (Go): video-script writing agent app (binary `viderse`), one Writer agent on the shared core. See [apps/viderse/README.md](apps/viderse/README.md). |
| `core/` | Shared AI core: `agentcore` (agent runtime), `llm` (model interface and providers), `models`, `llmretry`, `llmcontract`, `llmconfig` (shared provider/model config schema), `auth` (OAuth and credentials), `roles` (role→model config). |
| `platform/` | App-agnostic infrastructure: `identity` (web accounts, sessions, pairing), `webguard` (trusted proxies, same-origin guard, client IP, secure-cookie decision), `pgmigrate` (embedded goose migrations), `pgtest` (isolated Postgres schemas for tests). |
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

## Adding an app

`apps/viderse` is the smallest complete example; copy its layout and wiring.

1. **Layout.** Put the code in `apps/<name>/` with `cmd/<binary>/`, `internal/` and a `README.md`. Add `apps/<name>/.gitignore` only if the app writes local artifacts. Everything stays in this Go module; there is no `go.work`.
2. **Imports.** An app may import the standard library, third-party modules, `core/**`, `platform/**` and itself. `archtest` classifies every `apps/<name>` automatically, so `go test ./archtest` covers a new app with no registration.
3. **Isolated runtime state.** Nothing is shared with other apps:
   - config dir: `~/.ainovel`, `~/.viderse`;
   - credentials: `core/auth` configured per app, see `apps/viderse/internal/bootstrap`;
   - session cookie name: `ainovel_session`, `viderse_session`;
   - Postgres database and roles: `noverse_*`; `viderse_go` with `viderse_app`/`viderse_migrate`;
   - fixed default port: Noverse live 8790, Viderse 8789.

   Browsers share cookies across ports of the same host, so cookie names must differ.
4. **Web.** Accounts and sessions come from `platform/identity` (`identity.NewStore`). The gateway goes through a `webguard.Config` from `webguard.ConfigFromEnv()`:
   - `Normalize` and `CrossSiteRequest` reject cross-origin state-changing requests;
   - `SecureRequest` sets the cookie `Secure` flag;
   - `ClientIP` keys the login limiter.

   Reference: `apps/viderse/internal/web/gateway.go`.
5. **Postgres.** Embed the goose migrations and apply them with `platform/pgmigrate`. Integration tests use `platform/pgtest`. sqlc config lives in `apps/<name>/sqlc.yaml`.
6. **CI.** Add the app's `go tool sqlc diff` step and its web job to `.github/workflows/ci.yml`, copying the Viderse entries.
7. **Sharing.** Move code into `core/` or `platform/` only once a second app needs it.

To bring in an existing repository:

- import it at a tag without history, as Noverse was at `pre-verse`;
- rewrite its module path to `github.com/datnt1998/verse/apps/<name>`;
- keep its runtime names (binary, config dir, cookie, database roles) so existing data and sessions keep working.

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
